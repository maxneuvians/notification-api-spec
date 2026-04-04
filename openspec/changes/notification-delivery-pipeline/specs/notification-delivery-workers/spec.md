## Requirements

### Requirement: save-smss and save-emails workers — HMAC verification
Workers consuming the DB-persistence queues (`-priority-database-tasks.fifo`, `-normal-database-tasks`, `-bulk-database-tasks`) SHALL verify the `itsdangerous` HMAC signature on each notification blob using `pkg/signing.Verify` before any DB write. A `BadSignature` error SHALL cause the SQS message to be deleted immediately with no retry.

#### Scenario: Valid signed blob proceeds to DB insert
- **WHEN** a save worker receives a notification blob with a valid HMAC signature
- **THEN** the notification row is inserted into the DB and a delivery task is enqueued

#### Scenario: Invalid signature drops message immediately without retry
- **WHEN** a save worker receives a notification blob with a tampered or invalid HMAC signature
- **THEN** the SQS message is deleted and no DB insert is attempted; no retry is queued

---

### Requirement: save-smss and save-emails workers — bulk insert and delivery enqueue
After signature verification, workers SHALL bulk-insert all valid notifications via `BulkInsertNotifications`. After insert, each notification SHALL be enqueued to its appropriate delivery queue via `send_notification_to_queue`. If a Redis `receipt` token is present the worker SHALL call `acknowledge_receipt` after insert.

#### Scenario: Persisted SMS notification is enqueued to deliver-sms queue
- **WHEN** an SMS notification is successfully persisted
- **THEN** a `deliver-sms` task is enqueued to the correct priority send queue (`send-sms-high`, `send-sms-medium`, or `send-sms-low`)

#### Scenario: Persisted email notification is enqueued to deliver-email queue
- **WHEN** an email notification is successfully persisted
- **THEN** a `deliver-email` task is enqueued to the correct priority send queue

#### Scenario: Daily/annual SMS limit exceeded skips delivery enqueue with warning log
- **WHEN** `LiveServiceTooManySMSRequestsError` or `TrialServiceTooManySMSRequestsError` is raised
- **THEN** delivery enqueue is skipped and a warning is logged; the notification row is still persisted

#### Scenario: SQLAlchemyError triggers retry up to 5 times
- **WHEN** `BulkInsertNotifications` returns a `SQLAlchemyError`
- **THEN** `handle_batch_error_and_forward` re-queues individual notifications; max retries = 5, back-off = 300 s

#### Scenario: Redis receipt acknowledged after successful insert
- **WHEN** a save task has a non-nil `receipt` token
- **THEN** `acknowledge_receipt` is called after `BulkInsertNotifications` succeeds

#### Scenario: Redis receipt acknowledged even on batch failure
- **WHEN** `BulkInsertNotifications` fails and the task has a receipt token
- **THEN** `acknowledge_receipt` is still called to purge the inflight key and prevent re-delivery

---

### Requirement: deliver-sms worker — normal flow and pre-conditions
The `deliver-sms` worker SHALL consume `send-sms-high`, `send-sms-medium`, and `send-sms-low`. It SHALL call `send_sms_to_provider(notification)` after evaluating all pre-conditions in order.

#### Scenario: Service inactive causes technical-failure without provider call
- **WHEN** the notification's service has `active = false`
- **THEN** `notification.status = "technical-failure"`, `NotificationTechnicalFailureException` is raised, and the provider is not called

#### Scenario: Empty SMS body after personalisation causes technical-failure
- **WHEN** the SMS body is empty or whitespace-only after personalisation substitution
- **THEN** `notification.status = "technical-failure"` and the provider is not called

#### Scenario: Notification status not "created" is skipped silently
- **WHEN** the notification's current status is not `"created"` (e.g., already `"sending"`)
- **THEN** the worker returns without calling the provider and without setting a new status

#### Scenario: Internal test number calls send_sms_response instead of provider
- **WHEN** the notification recipient matches `Config.INTERNAL_TEST_NUMBER`
- **THEN** `send_sms_response` is called; no real provider call is made; status → `"sent"`

---

### Requirement: deliver-sms worker — research mode and test key
When `research_mode = true` OR `key_type = KEY_TYPE_TEST`, the deliver-sms worker SHALL call `send_sms_response` instead of a real provider.

#### Scenario: Research mode uses send_sms_response and does not call real provider
- **WHEN** the notification's service has `research_mode = true`
- **THEN** `send_sms_response("sns", to, reference)` is called; `billable_units = 0`

#### Scenario: Test key uses send_sms_response regardless of research mode
- **WHEN** `notification.key_type = KEY_TYPE_TEST`
- **THEN** `send_sms_response` is called instead of the real provider

#### Scenario: Exception in send_sms_response in research mode leaves billable_units at 0
- **WHEN** `send_sms_response` raises an exception in research mode
- **THEN** `notification.billable_units` remains 0 and the exception propagates

---

### Requirement: deliver-sms worker — opted-out phone number
The `aws_pinpoint_client.send_sms` function returns the string `"opted_out"` for opted-out recipients. The deliver-sms worker SHALL detect this return value and set the notification to permanent-failure without raising an exception.

#### Scenario: Opted-out return value from Pinpoint sets permanent-failure
- **WHEN** `aws_pinpoint_client.send_sms` returns `"opted_out"`
- **THEN** `notification.status = "permanent-failure"` and `notification.provider_response = "Phone number is opted out"`; no exception raised

---

### Requirement: deliver-sms worker — error handling and retry
The deliver-sms worker SHALL apply process-type–aware retry countdown (PRIORITY: 25 s, NORMAL/BULK: 300 s). Max retries: 48.

#### Scenario: Notification not found triggers 25 s retry
- **WHEN** the notification row is not found in the DB
- **THEN** the task is retried via `retry-tasks` with `countdown = 25`

#### Scenario: Generic provider exception triggers 300 s retry
- **WHEN** a generic exception is raised during `send_sms_to_provider`
- **THEN** the task is retried via `retry-tasks` with `countdown = 300`

#### Scenario: MaxRetriesExceeded sets technical-failure and enqueues callback
- **WHEN** 48 retry attempts are all exhausted
- **THEN** `notification.status = "technical-failure"`, a delivery-status callback is enqueued, and `NotificationTechnicalFailureException` is raised

#### Scenario: PinpointValidationException sets provider-failure without retry
- **WHEN** Pinpoint raises `PinpointValidationException` (e.g., NO_ORIGINATION_IDENTITIES_FOUND)
- **THEN** `notification.status = "provider-failure"`, `notification.feedback_reason` set from exception Reason field, callback enqueued, no retry

#### Scenario: PinpointConflictException sets provider-failure without retry
- **WHEN** Pinpoint raises `PinpointConflictException`
- **THEN** `notification.status = "provider-failure"` and callback enqueued; no retry

#### Scenario: PinpointValidationException does not call dao_toggle_sms_provider
- **WHEN** `PinpointValidationException` is raised
- **THEN** `dao_toggle_sms_provider` is NOT called

#### Scenario: Generic provider exception sets billable_units=1 and triggers failover
- **WHEN** a generic SMS provider exception is raised
- **THEN** `notification.billable_units = 1` and `dao_toggle_sms_provider` is called

---

### Requirement: deliver-sms worker — statsd metrics
The deliver-sms worker SHALL emit statsd metrics after each successful send.

#### Scenario: SMS statsd metrics emitted on success
- **WHEN** `send_sms_to_provider` returns successfully
- **THEN** `statsd_client.timing_with_dates("sms.total-time", sent_at, created_at)` and `statsd_client.incr("sms.process_type-<type>")` are called

---

### Requirement: deliver-throttled-sms worker — rate-limited single goroutine
The `deliver-throttled-sms` worker SHALL run with exactly 1 goroutine and a `golang.org/x/time/rate` token-bucket limiter at 0.5 tokens/s (30 messages per minute). All other behaviour is identical to `deliver-sms`.

#### Scenario: Throttled worker processes at most 30 messages per minute
- **WHEN** 60 messages are enqueued to `send-throttled-sms-tasks`
- **THEN** they are processed at a rate not exceeding 30 per minute (0.5/s)

#### Scenario: Throttled worker pool has exactly 1 goroutine
- **WHEN** `WorkerManager.Start` is called
- **THEN** exactly 1 goroutine is consuming `send-throttled-sms-tasks`

---

### Requirement: deliver-email worker — normal flow and pre-conditions
The `deliver-email` worker SHALL consume `send-email-high`, `send-email-medium`, `send-email-low`. It SHALL call `send_email_to_provider(notification)` after evaluating all pre-conditions.

#### Scenario: Service inactive causes technical-failure without SES call
- **WHEN** the notification's service has `active = false`
- **THEN** `notification.status = "technical-failure"` and SES is not called

#### Scenario: Notification status not "created" is skipped silently
- **WHEN** the email notification's current status is not `"created"`
- **THEN** the worker returns without calling SES and without modifying notification status

#### Scenario: Internal test email calls send_email_response instead of SES
- **WHEN** the notification recipient matches `Config.INTERNAL_TEST_EMAIL_ADDRESS`
- **THEN** `send_email_response(to_address)` is called; status → `"sending"`, `sent_by = "ses"`; no SES call

#### Scenario: Research mode or test key uses send_email_response
- **WHEN** `research_mode = true` OR `key_type = KEY_TYPE_TEST`
- **THEN** `send_email_response(to_address)` is called; no real SES send

---

### Requirement: deliver-email worker — PII scan (FF_SCAN_FOR_PII)
When `FF_SCAN_FOR_PII = true`, the deliver-email worker SHALL scan the notification body for Social Insurance Numbers. A SIN that passes the Luhn algorithm SHALL cause immediate technical failure without calling SES. A SIN that fails Luhn or a phone-number-format pattern SHALL be allowed through.

#### Scenario: SIN passing Luhn scan causes pii-check-failed
- **WHEN** `FF_SCAN_FOR_PII = true` and the notification body contains a SIN that passes the Luhn algorithm
- **THEN** `notification.status = "pii-check-failed"` and SES is not called

#### Scenario: SIN failing Luhn is allowed through
- **WHEN** `FF_SCAN_FOR_PII = true` and the notification body contains a number that fails Luhn
- **THEN** the PII scan passes and SES is called normally

---

### Requirement: deliver-email worker — file attachment handling
The deliver-email worker SHALL handle file attachments before calling SES: run the malware scan first, then download for `sending_method = "attach"` or embed URL for `sending_method = "link"`.

#### Scenario: sending_method=attach downloads file and attaches to SES call
- **WHEN** `sending_method = "attach"` and the malware scan returns HTTP 200 (clean)
- **THEN** the file is fetched from `direct_file_url` and passed as an attachment to `AwsSesClient.send_email`

#### Scenario: sending_method=attach retries up to 5 times on HTTP 5xx from file server
- **WHEN** the file server returns HTTP 503 on the first 4 attempts and succeeds on the 5th
- **THEN** the file is successfully downloaded at the 5th attempt with no error

#### Scenario: sending_method=attach logs error after all 5 retries fail
- **WHEN** the file server returns HTTP 5xx on all 5 attempts
- **THEN** an error containing `"Max retries exceeded"` is logged

#### Scenario: sending_method=link embeds URL in HTML body without downloading
- **WHEN** `sending_method = "link"`
- **THEN** the `url` is embedded in the email HTML; `direct_file_url` is not fetched

#### Scenario: sending_method=link with file:// URL raises InvalidUrlException
- **WHEN** `sending_method = "link"` and `direct_file_url` has `file://` scheme
- **THEN** `InvalidUrlException` is raised

#### Scenario: Malware scan 423 (THREATS_FOUND) sets virus-scan-failed
- **WHEN** `document_download_client.check_scan_verdict` returns HTTP 423
- **THEN** `MalwareDetectedException` is raised and `notification.status = "virus-scan-failed"`

#### Scenario: Malware scan 428 (in progress) keeps status created and triggers exponential backoff
- **WHEN** `check_scan_verdict` returns HTTP 428 on first attempt
- **THEN** `MalwareScanInProgressException` is raised; status stays `"created"`; retry countdown = `SCAN_RETRY_BACKOFF × (retries + 1)` s

#### Scenario: Malware scan exponential backoff capped at SCAN_MAX_BACKOFF_RETRIES
- **WHEN** `check_scan_verdict` returns HTTP 428 on attempts 1 through 5 (`SCAN_MAX_BACKOFF_RETRIES = 5`)
- **THEN** after 5 scan-in-progress retries the worker falls back to the default countdown

#### Scenario: Malware scan 404 sets technical-failure
- **WHEN** `check_scan_verdict` returns HTTP 404
- **THEN** `DocumentDownloadException` is raised and `notification.status = "technical-failure"`

---

### Requirement: deliver-email worker — bounce rate tracking
After every successful SES send, the deliver-email worker SHALL call `bounce_rate_client.set_sliding_notifications` and then evaluate `check_bounce_rate_status`.

#### Scenario: Every email send updates the bounce-rate sliding window
- **WHEN** `send_email_to_provider` completes successfully
- **THEN** `bounce_rate_client.set_sliding_notifications(service_id, notification_id)` is called

#### Scenario: CRITICAL bounce rate logs a warning
- **WHEN** `check_bounce_rate_status` returns `CRITICAL` (≥10%)
- **THEN** a warning is logged containing `"critical bounce rate threshold of 10%"`

#### Scenario: WARNING bounce rate logs a warning
- **WHEN** `check_bounce_rate_status` returns `WARNING` (≥5%)
- **THEN** a warning is logged with the 5% threshold

#### Scenario: NORMAL bounce rate produces no log output
- **WHEN** `check_bounce_rate_status` returns `NORMAL`
- **THEN** no log message is emitted and `None` is returned

---

### Requirement: deliver-email worker — error handling and retry
The deliver-email worker applies the same process-type–aware retry policy as deliver-sms. Max retries: 48.

#### Scenario: InvalidEmailError causes immediate technical-failure with log message
- **WHEN** `InvalidEmailError` is raised by SES
- **THEN** `notification.status = "technical-failure"`, callback enqueued, info logged: `"Cannot send notification <id>, got an invalid email address: <msg>."`; no retry

#### Scenario: InvalidUrlException causes immediate technical-failure
- **WHEN** `InvalidUrlException` is raised
- **THEN** `notification.status = "technical-failure"` and callback enqueued; no retry

#### Scenario: AwsSesClientException retries; status stays created
- **WHEN** `AwsSesClientException` is raised
- **THEN** the task is retried; `notification.status` remains `"created"`

#### Scenario: MaxRetriesExceeded on email sets technical-failure and enqueues callback
- **WHEN** all 48 retry attempts are exhausted for deliver-email
- **THEN** `notification.status = "technical-failure"` and delivery-status callback enqueued

---

### Requirement: deliver-email worker — statsd metrics
The deliver-email worker SHALL emit statsd metrics after each successful send, distinguishing between emails with and without attachments.

#### Scenario: Email without attachments emits no-attachments metric key
- **WHEN** an email with no attachments is sent successfully
- **THEN** `statsd_client.timing_with_dates("email.no-attachments.process_type-normal", ...)` is called

#### Scenario: Email with attachments emits with-attachments metric key
- **WHEN** an email with file attachments is sent successfully
- **THEN** `statsd_client.timing_with_dates("email.with-attachments.process_type-normal", ...)` is called

#### Scenario: Email total-time metric is always emitted
- **WHEN** any email is sent successfully
- **THEN** `statsd_client.timing_with_dates("email.total-time", sent_at, created_at)` is called

---

### Requirement: WorkerManager starts and stops all delivery pools
`WorkerManager.Start(ctx)` SHALL launch goroutine pools for all delivery queues. `Stop()` SHALL cancel the context and wait for all goroutines to exit within the configured shutdown timeout.

#### Scenario: All delivery queue pools start on WorkerManager.Start
- **WHEN** `WorkerManager.Start(ctx)` is called
- **THEN** goroutine pools consuming all delivery queues (save-smss×3, save-emails×3, deliver-sms×3, deliver-email×3, deliver-throttled-sms×1, research-mode×1) are active

#### Scenario: Context cancellation stops all pools cleanly
- **WHEN** the context passed to `Start` is cancelled
- **THEN** all goroutine pools stop polling SQS and exit within the shutdown timeout

#### Scenario: WorkerManager.Stop waits for in-flight message processing
- **WHEN** `Stop` is called while a delivery worker is mid-processing
- **THEN** the in-flight message completes before the goroutine exits
