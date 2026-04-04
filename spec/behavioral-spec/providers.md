# Behavioral Spec: Providers & Delivery

## Processed Files

- [x] `tests/app/provider_details/test_rest.py`
- [x] `tests/app/dao/test_provider_details_dao.py`
- [x] `tests/app/dao/test_provider_rates_dao.py`
- [x] `tests/app/celery/test_provider_tasks.py`
- [x] `tests/app/delivery/test_send_to_providers.py`
- [x] `tests/app/celery/test_process_pinpoint_receipts_tasks.py`
- [x] `tests/app/celery/test_process_ses_receipts_tasks.py`
- [x] `tests/app/clients/test_aws_pinpoint.py`
- [x] `tests/app/clients/test_aws_ses.py`

---

## Endpoint Contracts

### GET /provider-details

- **Happy path**: Returns all 7 providers in a fixed ordering — sorted first by `notification_type`, then by `priority` within each type. The stable ordering verified by tests with Pinpoint priority bumped to 50: `ses`, `sns`, `mmg`, `firetext`, `loadtesting`, `pinpoint`, `dvla`.
- **Response fields**: Each object contains exactly: `id`, `created_by_name`, `display_name`, `identifier`, `priority`, `notification_type`, `active`, `updated_at`, `supports_international`, `current_month_billable_sms`. `current_month_billable_sms` counts units billed in the current calendar month from `ft_billing`.
- **Auth requirements**: Requires internal authorization header (`create_authorization_header`). Unauthenticated requests are rejected.

### GET /provider-details/{id}

- **Happy path**: Returns a single provider keyed on `provider_details` in the response envelope. `identifier` matches the record from the list endpoint.
- **Auth requirements**: Requires authorization header.

### POST /provider-details/{id}

- **Happy path — update priority**: Accepts `{"priority": <int>}`. Returns 200 with updated `provider_details` object. Both the JSON response and the live DB object reflect the new priority value.
- **Happy path — update active status**: Accepts `{"active": false}`. Returns 200; both response and DB object show `active = false`.
- **Happy path — update with created_by**: Accepts `{"created_by": "<user_id>", "active": false}`. User ID stored on the provider record; returned in the response.
- **Validation rules — disallowed fields**: Attempts to update `identifier`, `version`, or `updated_at` return HTTP 400 with `{"result": "error", "message": {"<field>": ["Not permitted to be updated"]}}`.
- **Auth requirements**: Requires authorization header plus `Content-Type: application/json`.

### GET /provider-details/{id}/versions

- **Happy path**: Returns version history in a `data` array. Each object contains exactly: `id`, `created_by`, `display_name`, `identifier`, `priority`, `notification_type`, `active`, `version`, `updated_at`, `supports_international`. Note: `current_month_billable_sms` is absent from version history records (present only on live records).
- **Auth requirements**: Requires authorization header.

---

## DAO Behavior Contracts

### get_provider_details_by_notification_type

- **Expected behavior**: Filters `provider_details` by `notification_type`. For `"sms"` (non-international), returns 5 providers. For `"sms"` + `international=True`, returns 3 providers all with `supports_international=True`. For `"email"`, returns 1 provider (`ses`), which has the lowest priority value (sorted ascending by priority).
- **Edge cases verified**: Function is called with type `"sms"` or `"email"` only. Priority ordering among SMS providers is tested but currently skipped (single active SMS provider in production config).

### get_current_provider

- **Expected behavior**: Returns the active provider with the lowest priority for the given `notification_type`. For `"sms"`, returns `sns` under current configuration.
- **Edge cases verified**: If all providers of a type are set inactive, returns `None`.

### get_provider_details_by_identifier

- **Expected behavior**: Returns the single `ProviderDetails` row matching the given string identifier (e.g., `"ses"`, `"pinpoint"`, `"sns"`).

### get_alternative_sms_provider

- **Status**: Currently skipped (`pytest.mark.skip`) — only 1 SMS provider active. Intended to return the non-current active SMS provider.

### dao_update_provider_details

- **Expected behavior**: Persists changes to `provider_details`. Automatically bumps `version` by 1. Writes the previous state as a new row in `provider_details_history` with the incremented version. Sets `updated_at` to the current UTC time.
- **Edge cases verified**: After update, both old and new history rows exist. Old row retains `active=True`, `version=1`, `updated_at=None`; new row reflects changed values.

### dao_switch_sms_provider_to_provider_with_identifier

- **Expected behavior**: Switches the active SMS provider to the named one.
- **Edge cases verified**: Calling with the identifier of the currently-active provider is a no-op — the same provider remains current. Switching to an inactive provider is also a no-op (skipped test).

### dao_toggle_sms_provider

- **Status**: Skipped — intended to promote the alternative provider and demote the current one, recording user ID in history.

### dao_get_provider_versions

- **Expected behavior**: Returns all `ProviderDetailsHistory` rows for a given provider ID. Fresh provider has exactly 1 history row.

### dao_get_provider_stats

- **Expected behavior**: Returns one stat row per provider, ordered by priority. Each row includes: `identifier`, `display_name`, `created_by_name`, `notification_type`, `supports_international`, `active`, `current_month_billable_sms`. `current_month_billable_sms` sums `billable_unit` from `ft_billing` for the current month only (rows from prior months excluded). Providers with no billing rows for the current month show 0.

### dao_get_sms_provider_with_equal_priority

- **Status**: Skipped — returns the conflicting provider when two SMS providers share the same priority value.

### create_provider_rates

- **Expected behavior**: Inserts a row into `provider_rates` linked to the provider by identifier (resolved to `provider_id`). Stores `rate` (Decimal) and `valid_from` (datetime) exactly as supplied.
- **Edge cases verified**: Exactly one row created; `provider_id` matches the resolved `ProviderDetails.id`.

---

## Task Behavior Contracts

### deliver_sms / deliver_throttled_sms

- **What they do**: Both are Celery task wrappers around the same internal `_deliver_sms` function. They load the notification by ID and call `send_to_providers.send_sms_to_provider(notification)`.
- **Notification not found**: Retries with `queue="retry-tasks"`, `countdown=25`.
- **Generic exception from provider**: Retries with `queue="retry-tasks"`, `countdown=300`. On `MaxRetriesExceededError`, sets `notification.status = "technical-failure"` and queues the callback task. Raises `NotificationTechnicalFailureException` containing the notification ID.
- **PinpointValidationException** (e.g., `NO_ORIGINATION_IDENTITIES_FOUND`, `DESTINATION_COUNTRY_BLOCKED`): No retry. Sets `notification.status = "provider-failure"` and `notification.feedback_reason = <Reason from exception response>`. Queues callback task.
- **Non-provider exceptions**: Do NOT trigger `dao_toggle_sms_provider`.
- **Decoration verified**: Both tasks have `__wrapped__.__name__` equal to their function name, confirming the `@celery.task` decorator wraps a correctly named function.

### deliver_email

- **What it does**: Loads the notification by ID and calls `send_to_providers.send_email_to_provider(notification)`.
- **Notification not found**: Retries with `queue="retry-tasks"`, `countdown=25`. No call to `send_email_to_provider` made.
- **Generic exception**: Retries with `queue="retry-tasks"`, `countdown=300`. On `MaxRetriesExceededError`, sets `notification.status = "technical-failure"`, queues callback task, raises `NotificationTechnicalFailureException`.
- **InvalidEmailError**: No retry. Sets `notification.status = "technical-failure"`, queues callback. Logs info: `"Cannot send notification <id>, got an invalid email address: <msg>."`.
- **AwsSesClientException**: Retries. Notification status remains `"created"`.

### process_pinpoint_results

- **What it does**: Processes receipt callbacks from AWS Pinpoint for SMS delivery outcomes. Looks up the notification by provider reference, updates status, stores SMS metadata, queues callbacks.
- **Delivered receipt flow**:
  - Sets `status = DELIVERED`.
  - Stores: `provider_response`, `sms_total_message_price`, `sms_total_carrier_fee`, `sms_iso_country_code`, `sms_carrier_name`, `sms_message_encoding`, `sms_origination_phone_number`.
  - Shortcode deliveries: origination stored as the short code (e.g., `"555555"`), provider_response = `"Message has been accepted by phone carrier"`.
  - Missing SMS data in callback: status still updated to DELIVERED; carrier/country fields remain `None`.
  - Queues delivery callback task.
- **Successful (non-delivered) receipt**: Stays `NOTIFICATION_SENT`. Callback task NOT queued.
- **Failed receipt flows** — provider_response → notification status mapping:
  - `"Blocked as spam by phone carrier"` → `PERMANENT_FAILURE`
  - `"Destination is on a blocked list"` → `PERMANENT_FAILURE`
  - `"Invalid phone number"` → `PERMANENT_FAILURE`
  - `"Message body is invalid"` → `PERMANENT_FAILURE`
  - `"Phone is currently unreachable/unavailable"` → `PERMANENT_FAILURE`
  - `"Unknown error attempting to reach phone"` → `PERMANENT_FAILURE`
  - `"Unhandled provider"` → `PERMANENT_FAILURE`
  - `"Phone carrier has blocked this message"` → `TEMPORARY_FAILURE`
  - `"Phone carrier is currently unreachable/unavailable"` → `TEMPORARY_FAILURE`
  - `"Phone has blocked SMS"` → `TEMPORARY_FAILURE`
  - `"Phone is on a blocked list"` → `TEMPORARY_FAILURE`
  - `"This delivery would exceed max price"` → `TEMPORARY_FAILURE`
  - `"Phone number is opted out"` → `TECHNICAL_FAILURE`
  - Any unrecognized string → `TECHNICAL_FAILURE` (logged as warning)
  - All failure cases: `provider_response` is saved; callback task queued.
- **Wrong provider**: If the notification found by reference was sent by a different provider (e.g., `sns`), logs an exception and does not update status.
- **Notification not found**: Retries. On `MaxRetriesExceededError`, logs warning: `"notification not found for Pinpoint reference: <ref> (update to <status>). Giving up."`. No callback queued.
- **DB update exception**: Retries.
- **Annual limit integration**: On successful delivery, calls `annual_limit_client.increment_sms_delivered(service_id)`. On any failure, calls `annual_limit_client.increment_sms_failed(service_id)`. When Redis is enabled and annual limits not yet seeded for today, calls `seed_annual_limit_notifications(service_id, data)` with current counts and does NOT separately increment.
- **Service callback**: Queues `send_delivery_status_to_service.apply_async([notification_id, signed_data, service_id], queue="service-callbacks")` when a service callback API is configured. Emits `statsd_client.timing_with_dates("callback.pinpoint.elapsed-time", ...)` and `statsd_client.incr("callback.pinpoint.delivered")`.

### process_ses_results

- **What it does**: Processes receipt callbacks from SES for email delivery, bounce, and complaint outcomes. Supports batch processing (multiple references per invocation).
- **Delivery receipt flow**: Sets notification status to `"delivered"`. Does not set `provider_response`. Queues service callback if configured.
- **Hard bounce flows** — subtype → status and provider_response:
  - `"General"` → `permanent-failure`, `provider_response = None`
  - `"Suppressed"` → `permanent-failure`, `provider_response = "The email address is on our email provider suppression list"`
  - `"OnAccountSuppressionList"` → `permanent-failure`, `provider_response = "The email address is on the GC Notify suppression list"`
  - Any hard bounce also sets `feedback_type = NOTIFICATION_HARD_BOUNCE` and `feedback_subtype` to the matching constant.
  - With Redis enabled: calls `bounce_rate_client.set_sliding_hard_bounce(service_id, notification_id)`. Does NOT call `set_sliding_notifications`.
- **Soft bounce flows** — subtype → status and provider_response:
  - `"General"` → `temporary-failure`, `provider_response = None`
  - `"AttachmentRejected"` → `temporary-failure`, `provider_response = "The email was rejected because of its attachments"`
  - Sets `feedback_type = NOTIFICATION_SOFT_BOUNCE` and `feedback_subtype` to matching constant.
  - With Redis enabled: does NOT call `bounce_rate_client` methods (soft bounces are not tracked in sliding window).
- **Status transition guard**: A notification already at `permanent-failure` is NOT updated to `delivered` by a delivery receipt; it stays `permanent-failure`. However, a delivered notification CAN be overwritten by a new hard bounce receipt to `permanent-failure`.
- **Complaint receipt flow**: Does NOT update notification status. Creates a `Complaint` record referencing the notification. PII scrubbed: `remove_emails_from_complaint` removes complained recipient email addresses from the stored JSON. If the notification is only in `notification_history` (not main table), it is fetched from history. Complaint callback queued via `send_complaint_to_service.apply_async`.
- **Separation logic**: `separate_complaint_and_non_complaint_receipts` splits a batch into complaint and non-complaint lists by `notificationType`.
- **Partial retry**: In a batch, notifications that are not yet found are re-queued individually with `queue="retry-tasks"`. Successfully updated notifications have callbacks dispatched normally.
- **Notification not found**: Retries. On `MaxRetriesExceededError`, logs error: `"notifications not found for SES references: <refs>. Giving up."`. Returns `None`.
- **DB update exception**: Retries; `retry.call_count != 0` verified.
- **Annual limit integration**: On delivery, calls `annual_limit_client.increment_email_delivered(service_id)`. On hard, soft, or unknown bounce, calls `annual_limit_client.increment_email_failed(service_id)`. When Redis enabled and not yet seeded today, calls `seed_annual_limit_notifications` and does NOT separately increment. Billable unit fields for email are always 0 in the seed payload.

### process_complaint_receipts (helper)

- **What it does**: Accepts a list of complaint receipts and a list of found notifications. Returns any complaint records for which no notification was found (for retry). Complaints with a matching notification are handled inline (handle_complaint called, callback queued).

---

## Delivery Logic Verified

### Provider selection (`provider_to_use`)

Rules evaluated in order for SMS:

| Condition | Selected Provider |
|-----------|------------------|
| Both `AWS_PINPOINT_SC_POOL_ID` and `AWS_PINPOINT_DEFAULT_POOL_ID` configured, Canadian number, not dedicated sender | Pinpoint |
| `AWS_PINPOINT_DEFAULT_POOL_ID` empty AND template not in `AWS_PINPOINT_SC_TEMPLATE_IDS` | SNS |
| Template ID matches `AWS_PINPOINT_SC_TEMPLATE_IDS` and SC pool configured (even without default pool) | Pinpoint |
| Dedicated sender number AND `FF_USE_PINPOINT_FOR_DEDICATED = False` | SNS |
| Dedicated sender number AND `FF_USE_PINPOINT_FOR_DEDICATED = True` | Pinpoint |
| US number (e.g., `+1706…`) | SNS |
| Non-zone-1 international number (e.g., `+44…`) with `international=True` | Pinpoint |
| Zone-1 non-Canada number (e.g., `+1671…` Guam) | SNS |
| Phone number fails regex matching | SNS |
| Either pool ID empty (only one configured) | SNS |

For email, only SES is used (single provider).

### Research mode / test key scenarios (SMS)

- `research_mode=True` **or** `key_type=KEY_TYPE_TEST`: calls `send_sms_response("sns", to, reference)` instead of real provider; no actual SMS send; status → `"sent"`.
- `research_mode=True` + `KEY_TYPE_NORMAL` or `KEY_TYPE_TEAM`: `billable_units=0`, status typically → `"delivered"` (determined by `send_sms_response` mock side-effect).
- Exception during `send_sms_response` in research mode: notification `billable_units` unchanged (0), exception propagates.

### Research mode / test key scenarios (email)

- `research_mode=True` **or** `key_type=KEY_TYPE_TEST`: calls `send_email_response(to_address)` instead of real SES; no actual email sent; status → `"sending"`, `sent_by = "ses"`.

### SMS sending pre-conditions

- Service inactive: raises `NotificationTechnicalFailureException(str(notification.id))`; status → `"technical-failure"`; provider not called.
- SMS body empty or whitespace only (after personalisation): status → `"technical-failure"`; provider not called.
- Notification status is not `"created"`: skip silently (no send, no response mock).
- Internal test number (`Config.INTERNAL_TEST_NUMBER`): calls `send_sms_response`, not provider; status → `"sent"`.

### Email sending pre-conditions

- Service inactive: raises `NotificationTechnicalFailureException`; provider not called; status → `"technical-failure"`.
- Notification status is not `"created"`: skip silently.
- Internal test email (`Config.INTERNAL_TEST_EMAIL_ADDRESS`): calls `send_email_response`; no SES send; status → `"sending"`.
- PII scan (`SCAN_FOR_PII=True`): SIN that passes Luhn algorithm → raises `NotificationTechnicalFailureException`, status → `"pii-check-failed"`, provider not called. SIN failing Luhn or phone number format → allowed through.

### Personalisation / content

- SMS: personalisation values substituted; HTML tags in content left as-is (not escaped) in the SMS string sent to provider. Non-GSM characters (grapes emoji, tabs, zero-width space, ellipsis) downgraded to GSM equivalents (`?`, space, empty, `...`). Phone number normalised (strip spaces, add country code).
- SMS: uses template version snapshotted on the notification, not the current template version.
- SMS: `prefix_sms=True` prepends service name (`"Sample service: "`).
- Email: personalisation substituted; HTML in template body escaped in plain-text part, preserved in HTML part. From address encoded as MIME encoded-word syntax (Base64/UTF-8): `"=?utf-8?B?<base64>?=" <localpart@domain>`.

### SMS failure recovery

- On any exception from the SMS provider: `billable_units` is still set to 1; `dao_toggle_sms_provider` is called (provider failover triggered).

### File attachments (email)

- Sending method `"attach"`: fetches file from `direct_file_url` via urllib3 `GET`; attaches as `{name, data, mime_type}` to SES call. Applies up to 5 retries on HTTP 5xx; after all retries exhausted logs error containing `"Max retries exceeded"`.
- Sending method `"link"`: embeds `url` in HTML body; no attachment. `direct_file_url` with `file://` scheme → raises `InvalidUrlException`.
- Malware scan: calls `document_download_client.check_scan_verdict` first:
  - HTTP 423 (`THREATS_FOUND`) → raises `MalwareDetectedException`; status → `"virus-scan-failed"`.
  - HTTP 428 (scan in progress) → raises `MalwareScanInProgressException`; status remains `"created"`.
  - HTTP 200, 408, 422 (clean/timed-out/unsupported/failed) → proceed with send.
  - HTTP 404 → raises `DocumentDownloadException`; status → `"technical-failure"`.

### Opted-out phone numbers (Pinpoint)

- `aws_pinpoint_client.send_sms` returns string `"opted_out"` for opted-out numbers.
- `send_sms_to_provider` checks the return value; if `"opted_out"`, sets `notification.status = "permanent-failure"` and `notification.provider_response = "Phone number is opted out"`. No exception raised.

### Bounce rate tracking (email)

- Every email sent: calls `bounce_rate_client.set_sliding_notifications(service_id, notification_id)`.
- `check_service_over_bounce_rate` evaluates `bounce_rate_client.check_bounce_rate_status`:
  - `CRITICAL` (≥10%): logs warning `"Service: <id> has met or exceeded a critical bounce rate threshold of 10%..."`.
  - `WARNING` (≥5%): logs warning with 5% threshold.
  - `NORMAL`: no log, returns `None`.

### Email branding options (`get_html_email_options`)

| Branding type | `fip_banner_english` | `fip_banner_french` | `logo_with_background_colour` |
|---|---|---|---|
| No branding (default service) | True | False | False |
| `BRANDING_ORG_NEW` | False | False | False |
| `BRANDING_BOTH_EN` | True | False | False |
| `BRANDING_BOTH_FR` | False | True | False |
| `BRANDING_ORG_BANNER_NEW` | False | False | True |

Additional fields present when branding is set: `brand_colour`, `brand_text`, `brand_name`, `brand_logo` (prefixed with `https://assets.notification.canada.ca/`). If logo is `None` for `ORG_BANNER_NEW`, `brand_logo` is `None`.

### Statsd metrics

- SMS sent: `statsd_client.timing_with_dates("sms.total-time", sent_at, created_at)`, `timing_with_dates("sms.process_type-normal", …)`, `incr("sms.process_type-normal")`.
- Email sent (no attachments): key `"email.no-attachments.process_type-normal"`.
- Email sent (with attachments): key `"email.with-attachments.process_type-normal"`.
- Both email and SMS: `timing_with_dates("email.total-time", …)` / `timing_with_dates("sms.total-time", …)`.

---

## Client-Level Behavior

### AwsPinpointClient (`app.aws_pinpoint_client`)

**`send_sms(to, content, reference, template_id, service_id, sender, sending_vehicle)`**

- Normalises `to` by prepending `+1` for 10-digit numbers; empty string raises `ValueError("No valid numbers found for SMS delivery")`.
- Pool selection for OriginationIdentity:
  - Default case: `DEFAULT_POOL_ID`.
  - Template in `AWS_PINPOINT_SC_TEMPLATE_IDS` and `service_id == NOTIFY_SERVICE_ID`: `SC_POOL_ID`.
  - `sending_vehicle = SmsSendingVehicles("short_code")`: `SC_POOL_ID`.
  - `sending_vehicle = SmsSendingVehicles("long_code")` or `None`: `DEFAULT_POOL_ID`.
  - International numbers (non-`+1`): no `OriginationIdentity` or `DryRun` fields in the boto call.
- Dry run: if `to` matches `EXTERNAL_TEST_NUMBER` config value, sets `DryRun=True`.
- Dedicated sender: if `sender` is a long-code number and `FF_USE_PINPOINT_FOR_DEDICATED=True`, uses `_dedicated_client.send_text_message` with `OriginationIdentity=sender`. The default `_client` is NOT called.
- Always sets `MessageType="TRANSACTIONAL"` and `ConfigurationSetName=<config_set_name>`.
- Error handling:
  - `ConflictException` with `Reason="DESTINATION_PHONE_NUMBER_OPTED_OUT"` → returns `"opted_out"`.
  - Any other `ConflictException` → raises `PinpointConflictException` wrapping the original exception.
  - `ValidationException` → raises `PinpointValidationException` wrapping the original exception.

### AwsSesClient (`app.aws_ses_client`)

**`send_email(source, to_addresses, subject, body, html_body, reply_to_address, attachments)`**

- Constructs a raw MIME email and calls `boto3_client.send_raw_email(RawMessage={"Data": raw_message})`.
- **Multipart/alternative** (text + HTML, no attachments): outer boundary wraps text/plain and text/html parts.
- **Multipart/mixed** (with attachments): outer boundary wraps multipart/alternative sub-part and attachment parts. Attachment data base64-encoded with `Content-Disposition: attachment`.
- **Reply-to**: if `None`, header is absent; if a string, included as `reply-to: <value>`; IDN domains are punycode-encoded and the whole address base64-encoded.
- **To address**: IDN domains punycode-encoded and base64-encoded.
- Error handling:
  - `ClientError` with `Code="InvalidParameterValue"` → raises `InvalidEmailError` with the AWS message.
  - Any other `ClientError` → raises `AwsSesClientException` with the AWS message.

**`punycode_encode_email(address)`**: Converts international domain names to punycode (ASCII-compatible encoding). ASCII domains are returned unchanged.

---

## Business Rules Verified

### Provider registry

- Exactly **7 providers** exist in the system: `ses` (email), `sns` (sms, international), `mmg` (sms, international, inactive), `firetext` (sms, inactive), `loadtesting` (sms, not-international, inactive), `pinpoint` (sms, international), `dvla` (type not shown in available tests).
- Providers support types `"email"` and `"sms"`. International SMS providers: `sns`, `mmg`, `pinpoint`.
- Removing a provider from the DB does not break the application — `clients.get_sms_client("sns")` succeeds even without a DB row.

### Provider failover

- Multi-provider SMS failover (toggle) is currently **disabled** (all toggle/switch tests are `pytest.mark.skip`). The system operates with a single active SMS path at any time.
- SMS provider switching via `dao_switch_sms_provider_to_provider_with_identifier` is a no-op if the target is already current.
- On any unhandled SMS send exception, `dao_toggle_sms_provider` is called to initiate failover. This is explicitly NOT called for non-provider exceptions (verified: `assert switch_provider_mock.called is False` for generic non-provider exceptions).
- **⚠️ Skipped tests**: failover toggle/switch tests are marked `pytest.mark.skip` because only one SMS provider is active. Go must implement the full failover infrastructure and add tests once a second SMS provider is configured.

### Receipt / bounce / complaint processing flows

- **SES receipts**: A delivered notification CAN be downgraded to `permanent-failure` on receipt of a subsequent hard bounce. The reverse is blocked: a hard-bounced notification stays at `permanent-failure` even if a delivery receipt arrives later.
- **Pinpoint receipts**: Only processed if `sent_by == "pinpoint"`. Mismatch raises a logged exception with no status change.
- **Complaint processing**: Stored as `Complaint` objects; no notification status change. Email addresses scrubbed from stored JSON payloads. Notifications found only in `notification_history` (soft-deleted / archived) are still used for complaint linkage.
- **Retry semantics**: Missing notifications trigger retries. All providers give up after `MaxRetriesExceeded` and log a warning. SES supports partial-batch retry: failed references are re-enqueued while successful ones are committed.

### Annual limit tracking

- Both Pinpoint (SMS) and SES (email) receipt tasks call `annual_limit_client.increment_*` on every terminal outcome (delivered or failed).
- First outcome of the day (when not yet seeded): `seed_annual_limit_notifications` is called with full current counts; increment calls are suppressed during seeding to avoid double-counting.
- When `FF_USE_BILLABLE_UNITS` is enabled, the seed payload includes additional billable unit fields (`total_sms_billable_units_fiscal_year_to_yesterday`, `sms_billable_units_failed_today`, `sms_billable_units_delivered_today`); for email these are always 0.

### Rate limiting (provider rates)

- `create_provider_rates(identifier, valid_from, rate)` records a rate with `valid_from` timestamp linked to the provider. Rate is a `Decimal` value (e.g., `1.00000`). No rate limiting logic tested at the task or delivery layer; rates are informational billing records only.
