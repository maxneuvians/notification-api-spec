# Brief: notification-receipt-callbacks

## Source Files Analysed

- `spec/behavioral-spec/providers.md` — sections: process_ses_results, process_pinpoint_results, process_complaint_receipts
- `spec/async-tasks.md` — tasks: process-ses-result, process-sns-result, process-pinpoint-result, send-delivery-status, send-complaint
- `openspec/changes/notification-receipt-callbacks/proposal.md`

---

## Workers Covered

| Worker | Queue | Trigger |
|--------|-------|---------|
| `process-ses-result` | `delivery-receipts` | SES event via SNS → SQS |
| `process-sns-result` | `delivery-receipts` | SNS SMS delivery report |
| `process-pinpoint-result` | `delivery-receipts` | Pinpoint SMS V2 event **(C3 fix)** |
| `send-delivery-status` | `service-callbacks` / `service-callbacks-retry` | Chained from receipt workers |
| `send-complaint` | `service-callbacks` / `service-callbacks-retry` | Chained from `process-ses-result` |

---

## C3 Fix: process-pinpoint-result was Missing

`process-pinpoint-result` (singular noun) exists in the Python production codebase (`app/celery/process_pinpoint_receipts_tasks.py`) and is tested, but was absent from `spec/async-tasks.md`. It **must** be included in the Go implementation as a first-class receipt worker consuming `delivery-receipts`.

---

## process-ses-result — Exhaustive Behaviors

### Input / Queue

- Consumed from `delivery-receipts` queue; retries via `retry-tasks`.
- Input is a batch: `response` dict with `Messages` list or single `Message` key, each containing a JSON-encoded `SESReceipt` object wrapped in an SNS envelope.
- `separate_complaint_and_non_complaint_receipts` splits the batch by `notificationType` into complaint receipts and delivery/bounce receipts before processing.

### Delivery Receipt

- Sets `notification.status = "delivered"`.
- Does **not** set `provider_response`.
- Queues `send-delivery-status` callback if the service has a registered callback API.
- Annual limit: calls `annual_limit_client.increment_email_delivered(service_id)`.

### Hard Bounce Receipt — per subtype

| Subtype | Status | `provider_response` |
|---------|--------|---------------------|
| `General` | `permanent-failure` | `None` |
| `Suppressed` | `permanent-failure` | `"The email address is on our email provider suppression list"` |
| `OnAccountSuppressionList` | `permanent-failure` | `"The email address is on the GC Notify suppression list"` |

- All hard bounces: set `feedback_type = NOTIFICATION_HARD_BOUNCE` and `feedback_subtype` to the matching subtype constant.
- Redis: calls `bounce_rate_client.set_sliding_hard_bounce(service_id, notification_id)`. Does **not** call `set_sliding_notifications`.
- Annual limit: calls `annual_limit_client.increment_email_failed(service_id)`.
- Queues `send-delivery-status` callback.

### Soft Bounce Receipt — per subtype

| Subtype | Status | `provider_response` |
|---------|--------|---------------------|
| `General` | `temporary-failure` | `None` |
| `AttachmentRejected` | `temporary-failure` | `"The email was rejected because of its attachments"` |

- All soft bounces: set `feedback_type = NOTIFICATION_SOFT_BOUNCE` and `feedback_subtype`.
- Redis: bounce rate **not** updated (soft bounces excluded from sliding window).
- Annual limit: calls `annual_limit_client.increment_email_failed(service_id)`.
- Queues `send-delivery-status` callback.

### Status Transition Guard

- A notification already at `permanent-failure` is **NOT** updated to `delivered` by a delivery receipt; it stays `permanent-failure`.
- However, a notification at `delivered` **CAN** be overwritten to `permanent-failure` by a subsequent hard bounce receipt.

### Complaint Receipt

- Does **NOT** update notification status.
- Creates a `Complaint` DB record referencing the notification.
- Calls `remove_emails_from_complaint` to scrub PII (recipient email addresses) from the stored JSON.
- If the notification is not in the main `notifications` table, falls back to `notification_history` (handles notifications past data-retention cutoff).
- Queues `send-complaint` callback via `send_complaint_to_service.apply_async`.

### Partial Batch Retry

- In a batch, individual notifications that are **not found** in the DB are re-queued to `retry-tasks` individually (not the whole batch).
- Successfully updated notifications have their callbacks dispatched normally regardless of other failures in the same batch.

### Annual Limit — Seeding

- First terminal outcome of the day (Redis not yet seeded for today): calls `seed_annual_limit_notifications(service_id, data)` with current counts and does **not** additionally call `increment_*` (avoids double-count).
- For email, billable unit fields in the seed payload are always 0.

### StatsD Metrics

- `statsd_client.incr("callback.ses.<status>")` for each processed receipt.

### Error Handling

- Notification not found: retries via `retry-tasks`. On `MaxRetriesExceededError`, logs error: `"notifications not found for SES references: <refs>. Giving up."`. Returns `None`.
- DB update exception: retries (`retry.call_count != 0` verified in test).
- Return value: `True` on success; `None` if retried.

---

## process-pinpoint-result — Exhaustive Behaviors

### Input / Queue

- Consumed from `delivery-receipts` queue; retries via `retry-tasks`.
- Input: `response` dict with `Message` key containing a JSON-encoded Pinpoint SMS V2 delivery event.
- Processes **one notification per message** (unlike SES batch).
- Skips update if `isFinal=false` and status is `SUCCESSFUL` (early return, no callback).

### Delivered Receipt

- Sets `notification.status = NOTIFICATION_DELIVERED`.
- Stores **7 optional SMS metadata fields** (each nullable if absent in the event):
  - `provider_response`
  - `sms_total_message_price`
  - `sms_total_carrier_fee`
  - `sms_iso_country_code`
  - `sms_carrier_name`
  - `sms_message_encoding`
  - `sms_origination_phone_number`
- Shortcode delivery: `sms_origination_phone_number` is stored as the short code string (e.g., `"555555"`); `provider_response = "Message has been accepted by phone carrier"`.
- Missing SMS data in callback: status still updated to `DELIVERED`; carrier/country fields remain `None`.
- Queues `send-delivery-status` callback task.
- Annual limit: calls `annual_limit_client.increment_sms_delivered(service_id)`.

### Successful (Non-Delivered) Receipt

- `isFinal=false` and `messageStatus = SUCCESSFUL`: stays `NOTIFICATION_SENT`.
- Callback task **NOT** queued.

### Failure Receipt — provider_response → status mapping

| `provider_response` string | Status |
|----------------------------|--------|
| `"Blocked as spam by phone carrier"` | `PERMANENT_FAILURE` |
| `"Destination is on a blocked list"` | `PERMANENT_FAILURE` |
| `"Invalid phone number"` | `PERMANENT_FAILURE` |
| `"Message body is invalid"` | `PERMANENT_FAILURE` |
| `"Phone is currently unreachable/unavailable"` | `PERMANENT_FAILURE` |
| `"Unknown error attempting to reach phone"` | `PERMANENT_FAILURE` |
| `"Unhandled provider"` | `PERMANENT_FAILURE` |
| `"Phone carrier has blocked this message"` | `TEMPORARY_FAILURE` |
| `"Phone carrier is currently unreachable/unavailable"` | `TEMPORARY_FAILURE` |
| `"Phone has blocked SMS"` | `TEMPORARY_FAILURE` |
| `"Phone is on a blocked list"` | `TEMPORARY_FAILURE` |
| `"This delivery would exceed max price"` | `TEMPORARY_FAILURE` |
| `"Phone number is opted out"` | `TECHNICAL_FAILURE` |
| Any unrecognized string | `TECHNICAL_FAILURE` (logged as warning) |

- All failure cases: `provider_response` saved; `send-delivery-status` callback queued.
- Annual limit: calls `annual_limit_client.increment_sms_failed(service_id)`.

### StatsD Metrics

- `statsd_client.timing_with_dates("callback.pinpoint.elapsed-time", ...)`.
- `statsd_client.incr("callback.pinpoint.delivered")` on delivered.

### Wrong Provider Guard

- If the notification found by reference has `sent_by != "pinpoint"` (e.g., `"sns"`): logs an exception, does **not** update status, returns.

### Error Handling

- Notification not found: retries. On `MaxRetriesExceededError`, logs warning: `"notification not found for Pinpoint reference: <ref> (update to <status>). Giving up."`. No callback queued.
- DB update exception: retries.

### Annual Limit — Seeding

- Same first-of-day seeding pattern as SES: `seed_annual_limit_notifications` called, increment suppressed.
- With `FF_USE_BILLABLE_UNITS` enabled: seed payload includes `total_sms_billable_units_fiscal_year_to_yesterday`, `sms_billable_units_failed_today`, `sms_billable_units_delivered_today`.

---

## process-complaint-receipts Helper

- Called within `process-ses-result` for complaint-type receipts.
- Input: list of complaint receipts + list of found notifications.
- Calls `handle_complaint` for each matched complaint; queues `send-complaint` callback.
- Returns list of complaints with no matching notification (caller re-queues these for retry).

---

## send-delivery-status — Exhaustive Behaviors

### Queue / Retry

- Consumes `service-callbacks`; retries via `service-callbacks-retry`.
- Max retries: 5. Back-off: 300 s.
- Retries on: connection errors, HTTP 5xx, HTTP 429.
- Does **not** retry on: HTTP 4xx (other than 429) — drops with log.

### Input

- `notification_id` (UUID str), `signed_status_update` (signed/encrypted blob), `service_id` (UUID str).
- Verifies `itsdangerous` HMAC signature on `signed_status_update` before use.

### HTTP Call

- Looks up the service's registered delivery-status callback URL and bearer token (bearer token stored encrypted; decrypted before use).
- `POST` to callback URL with `Authorization: Bearer <signed_token>` header.
- JSON body: `{id, reference, to, status, status_description, provider_response, created_at, completed_at, sent_at, notification_type}`.
- Request timeout: 5 s.

### SSRF Guard

- Callback URL must be HTTPS (not HTTP).
- Callback URL hostname must not resolve to private IP ranges (RFC 1918: 10/8, 172.16/12, 192.168/16) or loopback.
- Private IP → reject before TCP connection is established; log error.

### Queue URL Validation

- Queue URL is validated before any outbound call (per callback API registration).

---

## send-complaint — Exhaustive Behaviors

### Queue / Retry

- Consumes `service-callbacks`; retries via `service-callbacks-retry`.
- Same retry policy as `send-delivery-status` (max 5, 300 s, no retry on 4xx except 429).

### Input

- `complaint_data` (signed blob), `service_id` (UUID str).
- Verifies signature on `complaint_data` before use.

### HTTP Call

- `POST` to the service's registered callback URL.
- JSON body: `{notification_id, complaint_id, reference, to, complaint_date}`.
- Request timeout: 5 s.
- Same SSRF guard as `send-delivery-status`.

---

## Retry Policies Summary

| Worker | Max Retries | Back-off | Queue |
|--------|-------------|----------|-------|
| `process-ses-result` | 5 | 300 s | `retry-tasks` |
| `process-pinpoint-result` | 5 | 300 s | `retry-tasks` |
| `send-delivery-status` | 5 | 300 s | `service-callbacks-retry` |
| `send-complaint` | 5 | 300 s | `service-callbacks-retry` |
