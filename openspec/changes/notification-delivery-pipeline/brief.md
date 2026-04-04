# Brief: notification-delivery-pipeline

## Source Files Analysed

- `spec/behavioral-spec/providers.md` — sections: deliver_sms, deliver_throttled_sms, deliver_email, provider selection, SMS/email pre-conditions, personalisation, file attachments, bounce rate tracking, statsd metrics, AwsPinpointClient, AwsSesClient
- `spec/async-tasks.md` — tasks: save-smss, save-emails, deliver-sms, deliver-throttled-sms, deliver-email
- `openspec/changes/notification-delivery-pipeline/proposal.md`

---

## Workers Covered

| Worker / Component | Queue(s) | Description |
|---------------------|----------|-------------|
| `save-smss` | `*-database-tasks` (3 queues) | Verify HMAC, bulk-insert, enqueue deliver-sms |
| `save-emails` | `*-database-tasks` (3 queues) | Same for email |
| `deliver-sms` | `send-sms-{high,medium,low}` | AWS SNS / Pinpoint dispatch |
| `deliver-throttled-sms` | `send-throttled-sms-tasks` | Rate-limited dedicated-number SMS |
| `deliver-email` | `send-email-{high,medium,low}` | AWS SES dispatch |
| `AwsPinpointClient` | — | Phone normalisation, pool selection, error mapping |
| `AwsSesClient` | — | MIME construction, error mapping |

---

## save-smss / save-emails — Exhaustive Behaviors

### HMAC Verification

- Each signed notification blob is verified with `pkg/signing.Verify` (itsdangerous HMAC).
- `BadSignature`: drop message immediately, **no retry**, do not attempt DB insert.

### Bulk Insert

- Valid notifications bulk-inserted via `BulkInsertNotifications`.
- If `receipt` present: calls `acknowledge_receipt()` to remove the Redis inflight key after insert.
- Emits CloudWatch metric `put_batch_saving_bulk_processed` when not on receipt path.

### Delivery Enqueue

- For each persisted notification: calls `send_notification_to_queue` → enqueues `deliver-sms`/`deliver-throttled-sms` (SMS) or `deliver-email` (email) on the appropriate priority send queue.
- Skips delivery enqueue and logs warning when `LiveServiceTooManySMSRequestsError` or `TrialServiceTooManySMSRequestsError` (or email equivalents) encountered.

### Reply-To (save-emails only)

- Resolution order: notification blob → `service_email_reply_to` lookup → template default → service default.

### Error Handling

- `SQLAlchemyError`: calls `handle_batch_error_and_forward` — re-queues individual notifications or retries the task via `QueueNames.RETRY`.
- Max retries: 5. Default retry delay: 300 s.
- On batch failure: Redis receipt acknowledged anyway (purges inflight) to avoid re-delivery.

---

## deliver-sms — Exhaustive Behaviors

### Normal Flow

- Reads `notifications` row by ID.
- Calls `send_to_providers.send_sms_to_provider(notification)` → AWS SNS `publish` or Pinpoint SMS V2 `send_text_message`.

### SMS Pre-Conditions (checked before provider call)

1. **Service inactive**: raises `NotificationTechnicalFailureException(str(notification.id))`; status → `"technical-failure"`; provider not called.
2. **SMS body empty or whitespace-only after personalisation**: status → `"technical-failure"`; provider not called.
3. **Notification status is not `"created"`**: skip silently (no send, no response mock).
4. **Internal test number** (`Config.INTERNAL_TEST_NUMBER`): calls `send_sms_response` instead of provider; status → `"sent"`.

### Research Mode / Test Key (SMS)

- `research_mode=True` **OR** `key_type=KEY_TYPE_TEST`: calls `send_sms_response("sns", to, reference)` instead of real provider; status → `"sent"`.
- `research_mode=True` + `KEY_TYPE_NORMAL` or `KEY_TYPE_TEAM`: `billable_units=0`.
- Exception during `send_sms_response` in research mode: `billable_units` unchanged (0), exception propagates.

### Provider Opted-Out Return Value

- `aws_pinpoint_client.send_sms` returns the string `"opted_out"` for opted-out numbers.
- `send_sms_to_provider` checks return value; if `"opted_out"`: sets `notification.status = "permanent-failure"` and `notification.provider_response = "Phone number is opted out"`. No exception raised.

### Error Handling

- **Notification not found**: retries with `queue="retry-tasks"`, `countdown=25`.
- **Generic exception**: retries with `queue="retry-tasks"`, `countdown=300`. On `MaxRetriesExceededError`: sets `notification.status = "technical-failure"`, queues callback, raises `NotificationTechnicalFailureException`.
- **`PinpointValidationException`** (`NO_ORIGINATION_IDENTITIES_FOUND`, `DESTINATION_COUNTRY_BLOCKED`, etc.): **no retry**; sets `notification.status = "provider-failure"` and `notification.feedback_reason = <Reason from exception response>`; queues callback.
- **`PinpointConflictException`**: sets `notification.status = "provider-failure"`, queues callback (no retry).
- **Non-provider exceptions**: do **NOT** call `dao_toggle_sms_provider`.
- **On any SMS provider exception**: `billable_units` is set to 1; `dao_toggle_sms_provider` called for provider failover. This is explicitly **not** called for `PinpointConflictException`/`PinpointValidationException`.

### Retry Policy

- `max_retries`: 48. `default_retry_delay`: process-type–aware via `CeleryParams.retry()`:
  - PRIORITY: 25 s countdown.
  - NORMAL / BULK: 300 s countdown.
  - Retries via `QueueNames.RETRY`.

### StatsD Metrics

- `statsd_client.timing_with_dates("sms.total-time", sent_at, created_at)`.
- `statsd_client.timing_with_dates("sms.process_type-normal", ...)`.
- `statsd_client.incr("sms.process_type-normal")`.

---

## deliver-throttled-sms — Exhaustive Behaviors

- Identical to `deliver-sms` (shares `_deliver_sms` helper internally).
- Queue: `send-throttled-sms-tasks`.
- Pool: exactly **1 goroutine** (concurrency = 1).
- Rate limiter: `golang.org/x/time/rate` at **0.5 tokens/s** (= 30/min), replicating Celery `rate_limit="30/m"`.
- Single consumer pod with concurrency=1 ensures rate limit is a true global cap.

---

## deliver-email — Exhaustive Behaviors

### Normal Flow

- Reads `notifications` row. Calls `send_to_providers.send_email_to_provider(notification)` → AWS SES.

### Email Pre-Conditions (checked before provider call)

1. **Service inactive**: raises `NotificationTechnicalFailureException`; status → `"technical-failure"`; provider not called.
2. **Notification status is not `"created"`**: skip silently.
3. **Internal test email** (`Config.INTERNAL_TEST_EMAIL_ADDRESS`): calls `send_email_response(to_address)`; no SES send; status → `"sending"`, `sent_by = "ses"`.
4. **PII scan** (`FF_SCAN_FOR_PII=True`): SIN that passes Luhn algorithm → raises `NotificationTechnicalFailureException`; status → `"pii-check-failed"`; provider not called. SIN failing Luhn or phone number format → allowed through.

### Research Mode / Test Key (email)

- `research_mode=True` **OR** `key_type=KEY_TYPE_TEST`: calls `send_email_response(to_address)` instead of SES; status → `"sending"`, `sent_by = "ses"`.

### File Attachments

- **`sending_method = "attach"`**: fetches file from `direct_file_url` via urllib3 `GET`; attaches as `{name, data, mime_type}` to SES call; up to **5 retries** on HTTP 5xx; after all retries exhausted logs error containing `"Max retries exceeded"`.
- **`sending_method = "link"`**: embeds `url` in HTML body; no attachment; `direct_file_url` with `file://` scheme → raises `InvalidUrlException`.
- **Malware scan** (runs before download/attach):
  - HTTP 423 (`THREATS_FOUND`) → raises `MalwareDetectedException` → status = `"virus-scan-failed"`.
  - HTTP 428 (scan in progress) → raises `MalwareScanInProgressException` → status stays `"created"`; exponential backoff retry: countdown = `SCAN_RETRY_BACKOFF × (retries + 1)` s; up to `SCAN_MAX_BACKOFF_RETRIES` (5) retries before falling back to default countdown.
  - HTTP 200 / 408 / 422 (clean / timed-out / unsupported) → proceed with send.
  - HTTP 404 → raises `DocumentDownloadException` → status = `"technical-failure"`.

### Bounce Rate Tracking

- Every email sent: calls `bounce_rate_client.set_sliding_notifications(service_id, notification_id)`.
- After sending, calls `check_service_over_bounce_rate` → evaluates `bounce_rate_client.check_bounce_rate_status`:
  - `CRITICAL` (≥10%): logs warning `"Service: <id> has met or exceeded a critical bounce rate threshold of 10%..."`.
  - `WARNING` (≥5%): logs warning with 5% threshold.
  - `NORMAL`: no log, returns `None`.

### Error Handling

- **Notification not found**: retries `queue="retry-tasks"`, `countdown=25`.
- **Generic exception**: retries `countdown=300`; on `MaxRetriesExceededError`: status `"technical-failure"`, callback queued.
- **`InvalidEmailError`**: **no retry**; status `"technical-failure"`; callback queued; logs info: `"Cannot send notification <id>, got an invalid email address: <msg>."`.
- **`InvalidUrlException`**: status `"technical-failure"`, callback queued.
- **`AwsSesClientException`**: retries; notification status remains `"created"`.
- **`MalwareDetectedException`**: callback queued (status already set upstream).

### StatsD Metrics

- `statsd_client.timing_with_dates("email.total-time", sent_at, created_at)`.
- Email with no attachments: `"email.no-attachments.process_type-normal"`.
- Email with attachments: `"email.with-attachments.process_type-normal"`.

---

## Personalisation / Content Transformation

### SMS

- Personalisation values substituted; HTML tags in content left as-is (not escaped) in the SMS string.
- Non-GSM characters (grapes emoji, tabs, zero-width space, ellipsis) downgraded to GSM equivalents (`?`, space, empty string, `...`).
- Phone number normalised: strip spaces, add country code.
- Uses template version snapshotted on the notification row, not the current live template version.
- `prefix_sms=True`: prepends service name (e.g., `"Sample service: "`).

### Email

- Personalisation substituted; HTML in template body escaped in plain-text part, preserved in HTML part.
- From address encoded as MIME encoded-word syntax (Base64/UTF-8): `"=?utf-8?B?<base64>?= <localpart@domain>"`.

---

## Provider Selection for SMS — Full Table

Rules evaluated in order:

| Condition | Selected Provider |
|-----------|------------------|
| Both pool IDs configured + Canadian number + not dedicated sender | Pinpoint |
| `AWS_PINPOINT_DEFAULT_POOL_ID` empty AND template not in `AWS_PINPOINT_SC_TEMPLATE_IDS` | SNS |
| Template ID in `AWS_PINPOINT_SC_TEMPLATE_IDS` AND SC pool configured | Pinpoint |
| Dedicated sender + `FF_USE_PINPOINT_FOR_DEDICATED = false` | SNS |
| Dedicated sender + `FF_USE_PINPOINT_FOR_DEDICATED = true` | Pinpoint |
| US number (e.g., `+1706…`) | SNS |
| Non-zone-1 international number (e.g., `+44…`) with `international=True` | Pinpoint |
| Zone-1 non-Canada number (e.g., `+1671…` Guam) | SNS |
| Phone number fails regex matching | SNS |
| Either pool ID empty (only one configured) | SNS |

For email: only SES used (single provider).

---

## Email Branding Options

| Branding type | `fip_banner_english` | `fip_banner_french` | `logo_with_background_colour` |
|---|---|---|---|
| No branding (default service) | true | false | false |
| `BRANDING_ORG_NEW` | false | false | false |
| `BRANDING_BOTH_EN` | true | false | false |
| `BRANDING_BOTH_FR` | false | true | false |
| `BRANDING_ORG_BANNER_NEW` | false | false | true |

When branding is set: response includes `brand_colour`, `brand_text`, `brand_name`, `brand_logo` (URL prefixed with `https://assets.notification.canada.ca/`). If logo is `None` for `ORG_BANNER_NEW`, `brand_logo` is `None`.

---

## AwsPinpointClient — Exhaustive Behaviors

### `send_sms(to, content, reference, template_id, service_id, sender, sending_vehicle)`

- **Phone normalisation**: 10-digit number gets `+1` prepended; empty string → raises `ValueError("No valid numbers found for SMS delivery")`.
- **Pool selection** for `OriginationIdentity`:
  - Default case: `DEFAULT_POOL_ID`.
  - Template in `AWS_PINPOINT_SC_TEMPLATE_IDS` AND `service_id == NOTIFY_SERVICE_ID`: `SC_POOL_ID`.
  - `sending_vehicle = SmsSendingVehicles("short_code")`: `SC_POOL_ID`.
  - `sending_vehicle = SmsSendingVehicles("long_code")` or `None`: `DEFAULT_POOL_ID`.
  - International numbers (non-`+1`): **no** `OriginationIdentity` field and **no** `DryRun` field in the boto call.
- **Dry run**: if `to` matches `EXTERNAL_TEST_NUMBER` config value → sets `DryRun=True`.
- **Dedicated sender**: if `sender` is a long-code number and `FF_USE_PINPOINT_FOR_DEDICATED=True` → uses `_dedicated_client.send_text_message` with `OriginationIdentity=sender`. Default `_client` is **NOT** called.
- Always sets `MessageType="TRANSACTIONAL"` and `ConfigurationSetName=<config_set_name>`.
- **Error mapping**:
  - `ConflictException` with `Reason="DESTINATION_PHONE_NUMBER_OPTED_OUT"` → returns string `"opted_out"` (no exception).
  - Any other `ConflictException` → raises `PinpointConflictException` wrapping the original.
  - `ValidationException` → raises `PinpointValidationException` wrapping the original.

---

## AwsSesClient — Exhaustive Behaviors

### `send_email(source, to_addresses, subject, body, html_body, reply_to_address, attachments)`

- Constructs a raw MIME email; calls `boto3_client.send_raw_email(RawMessage={"Data": raw_message})`.
- **No attachments**: `multipart/alternative` — outer boundary wraps `text/plain` and `text/html` parts.
- **With attachments**: `multipart/mixed` — outer boundary wraps `multipart/alternative` sub-part and one attachment part per file; attachment data base64-encoded with `Content-Disposition: attachment`.
- **Reply-to**: if `None`, header absent; if a string, included; IDN domains are punycode-encoded, then the whole address base64-encoded.
- **To address**: IDN domains punycode-encoded and base64-encoded.
- **Error mapping**:
  - `ClientError` with `Code="InvalidParameterValue"` → raises `InvalidEmailError` with the AWS message.
  - Any other `ClientError` → raises `AwsSesClientException` with the AWS message.

### `punycode_encode_email(address)`

- Converts international domain names to ASCII-compatible punycode encoding.
- ASCII-only domains returned unchanged.

---

## WorkerManager

- `Start(ctx)`: launches goroutine pools for all delivery queues: save-smss (3 × N), save-emails (3 × N), deliver-sms (3 × N), deliver-email (3 × N), deliver-throttled-sms (1 goroutine), research-mode-tasks (1 pool).
- `Stop()`: cancels context; waits for all goroutines to exit within shutdown timeout.
- Replaces the stub `WorkerManager` from `go-project-setup` change.

---

## Retry Policies Summary

| Worker | Max Retries | Countdown / Back-off | Queue |
|--------|-------------|----------------------|-------|
| `save-smss` / `save-emails` | 5 | 300 s | `retry-tasks` |
| `deliver-sms` | 48 | PRIORITY: 25 s / NORMAL+BULK: 300 s | `retry-tasks` |
| `deliver-throttled-sms` | 48 | 300 s | `retry-tasks` |
| `deliver-email` | 48 | PRIORITY: 25 s / NORMAL+BULK: 300 s | `retry-tasks` |
