## 1. Save-Notifications Workers

- [x] 1.1 Implement `internal/worker/savenotify/save_smss.go` — consume `-priority-database-tasks.fifo`, `-normal-database-tasks`, `-bulk-database-tasks`; verify HMAC signature per blob via `pkg/signing.Verify`; `BadSignature` → drop immediately; bulk-insert via `BulkInsertNotifications`; enqueue `deliver-sms` / `deliver-throttled-sms` per notification via `send_notification_to_queue`; acknowledge Redis receipt if present; on SQLAlchemyError call `handle_batch_error_and_forward`
- [x] 1.2 Implement `internal/worker/savenotify/save_emails.go` — same as above for email type; add reply-to resolution order (notification blob → `service_email_reply_to` → template default → service default)
- [x] 1.3 Write unit tests: valid signed blob → DB insert + delivery enqueue; BadSignature → drop + no insert; SQLAlchemyError → retry up to 5; receipt acknowledgement after success; receipt acknowledged even on batch failure; daily/annual limit exceeded → enqueue skipped with warning log

## 2. deliver-sms Worker

- [x] 2.1 Implement `internal/worker/delivery/deliver_sms.go` — consume `send-sms-{high,medium,low}`; evaluate pre-conditions (service inactive, empty body, wrong status); call `send_sms_to_provider`; handle opted-out return value (`"opted_out"` → permanent-failure); process-type retry countdown (PRIORITY: 25 s, NORMAL/BULK: 300 s); max retries 48
- [x] 2.2 Implement research mode / test key path: `research_mode=true` or `KEY_TYPE_TEST` → call `send_sms_response`; no real provider call; `billable_units = 0`; internal test number → `send_sms_response`
- [x] 2.3 Implement `PinpointValidationException` handler: no retry; set `provider-failure`; set `feedback_reason` from exception Reason field; enqueue callback; no `dao_toggle_sms_provider`
- [x] 2.4 Implement `PinpointConflictException` handler: no retry; set `provider-failure`; enqueue callback
- [x] 2.5 Implement `MaxRetriesExceeded` handler: set `technical-failure`; enqueue callback; raise `NotificationTechnicalFailureException`; on generic provider exception set `billable_units=1` and call `dao_toggle_sms_provider`
- [x] 2.6 Write unit tests: service inactive → technical-failure; empty body → technical-failure; wrong status → silent skip; PinpointValidationException → provider-failure + no retry + no toggle; PinpointConflictException → provider-failure; MaxRetriesExceeded → technical-failure + callback; generic exception → retry 300 s + billable_units=1 + toggle; not found → retry 25 s; opted-out → permanent-failure; research mode → send_sms_response; statsd metrics emitted

## 3. deliver-throttled-sms Worker

- [x] 3.1 Implement `internal/worker/delivery/deliver_throttled_sms.go` — consume `send-throttled-sms-tasks`; configure pool concurrency = 1; attach `golang.org/x/time/rate` limiter at 0.5 tokens/s; share `_deliver_sms` logic with `deliver-sms`
- [x] 3.2 Write unit tests: pool concurrency = 1 verified; rate limiter blocks at > 30/min; all error paths same as deliver-sms

## 4. deliver-email Worker

- [x] 4.1 Implement `internal/worker/delivery/deliver_email.go` — consume `send-email-{high,medium,low}`; evaluate pre-conditions (service inactive, wrong status, internal test email, research mode); call `send_email_to_provider`; after success call `bounce_rate_client.set_sliding_notifications` and `check_bounce_rate_status`
- [x] 4.2 Implement PII scan (`FF_SCAN_FOR_PII`): scan body for SINs; run Luhn algorithm; passing SIN → `pii-check-failed` + no SES call; failing Luhn or phone format → allowed through
- [x] 4.3 Implement file attachment flow: call `check_scan_verdict` first; 200/408/422 → proceed; 423 → `virus-scan-failed`; 428 → exponential-backoff retry (SCAN_RETRY_BACKOFF × (retries+1)); 404 → `technical-failure`; `sending_method=attach`: GET file with 5 HTTP retries on 5xx, pass as `{name, data, mime_type}`; `sending_method=link`: embed URL in HTML; `file://` scheme → `InvalidUrlException`
- [x] 4.4 Implement error handlers: `InvalidEmailError` → `technical-failure` + callback + info log; `InvalidUrlException` → `technical-failure` + callback; `AwsSesClientException` → retry; `MaxRetriesExceeded` → `technical-failure` + callback; not found → retry 25 s; generic exception → retry 300 s
- [x] 4.5 Write unit tests: service inactive → technical-failure; wrong status → silent skip; internal test email → send_email_response; research mode/test key → send_email_response; PII SIN passes Luhn → pii-check-failed; PII SIN fails Luhn → allowed; malware 423 → virus-scan-failed; malware 428 → exponential backoff + status created; malware 404 → technical-failure; attach with 5xx retries; link embed; file:// → InvalidUrlException; InvalidEmailError → technical-failure + no retry + log; AwsSesClientException → retry; bounce rate CRITICAL/WARNING/NORMAL log behaviour; statsd metrics (with/without attachments)

## 5. AwsPinpointClient

- [x] 5.1 Implement `internal/client/pinpoint/client.go` — `send_sms(to, content, reference, template_id, service_id, sender, sending_vehicle)`; phone normalisation (+1 prefix for 10-digit, ValueError for empty); pool selection logic (4 conditions); omit OriginationIdentity/DryRun for international numbers; DryRun=True for EXTERNAL_TEST_NUMBER; MessageType=TRANSACTIONAL; ConfigurationSetName from config; dedicated-sender path using `_dedicated_client`
- [x] 5.2 Implement error mapping: `ConflictException(OPTED_OUT)` → return `"opted_out"`; other `ConflictException` → `PinpointConflictException`; `ValidationException` → `PinpointValidationException` (preserve Reason)
- [x] 5.3 Write unit tests: 10-digit normalisation; empty → ValueError; DEFAULT pool; SC pool via template+service; SC pool via short_code vehicle; international → no OriginationIdentity; dry run; dedicated sender → _dedicated_client; OPTED_OUT conflict → "opted_out"; other conflict → PinpointConflictException; ValidationException → PinpointValidationException with Reason

## 6. AwsSesClient

- [x] 6.1 Implement `internal/client/ses/client.go` — `send_email(source, to_addresses, subject, body, html_body, reply_to_address, attachments)`; multipart/alternative for no attachments (text/plain + text/html); multipart/mixed for with attachments (sub-part + base64 attachment parts with Content-Disposition: attachment); call `send_raw_email`
- [x] 6.2 Implement reply-to handling: omit header if nil; include as-is if ASCII; punycode-encode IDN domain then base64-encode full address
- [x] 6.3 Implement `punycode_encode_email`: Unicode domain → punycode ACE; ASCII unchanged
- [x] 6.4 Implement error mapping: `ClientError(InvalidParameterValue)` → `InvalidEmailError`; any other `ClientError` → `AwsSesClientException`
- [x] 6.5 Write unit tests: no-attachment → multipart/alternative; with-attachment → multipart/mixed + base64 + Content-Disposition; nil reply-to → header absent; IDN reply-to → punycode; IDN to-address → punycode; InvalidParameterValue → InvalidEmailError; other ClientError → AwsSesClientException; punycode_encode_email ASCII unchanged; punycode_encode_email Unicode converted

## 7. Provider Client Interfaces

- [x] 7.1 Define `SMSSender` interface in `internal/client/sms_sender.go` with `SendSMS` method; define `EmailSender` interface in `internal/client/email_sender.go` with `SendEmail` method; verify `AwsPinpointClient` and `AwsSNSClient` satisfy `SMSSender`; verify `AwsSesClient` satisfies `EmailSender`

## 8. WorkerManager — Full Implementation

- [x] 8.1 Replace `WorkerManager` stub in `internal/worker/manager.go` with real implementation: `Start(ctx)` launches pools for all queues with configured concurrency; register: save-smss (×3 queues), save-emails (×3), deliver-sms (×3), deliver-email (×3), deliver-throttled-sms (1 goroutine, rate limiter), research-mode-tasks (1 pool)
- [x] 8.2 Implement `Stop()`: cancel context; use `sync.WaitGroup` to wait for all goroutines to exit within `WORKER_SHUTDOWN_TIMEOUT`
- [x] 8.3 Write unit tests: Start starts all configured queue consumers; Stop cancels context and all goroutines exit; in-flight processing completes before goroutine exits on cancellation

## 9. SMS Personalisation and Content

- [x] 9.1 Implement non-GSM character downgrade: map emoji/tabs/zero-width space/ellipsis to GSM equivalents; HTML tags preserved as-is in SMS body; `prefix_sms=True` prepends service name
- [x] 9.2 Write unit tests: grapes emoji → `?`; tab → space; zero-width space → empty; ellipsis → `...`; HTML tags preserved; prefix applied

## 10. Email Branding Options

- [x] 10.1 Implement `get_html_email_options(branding_type)` returning `{fip_banner_english, fip_banner_french, logo_with_background_colour, brand_colour, brand_text, brand_name, brand_logo}` per the 5-type matrix; `brand_logo` URL prefixed with `https://assets.notification.canada.ca/`; null logo for ORG_BANNER_NEW with nil logo
- [x] 10.2 Write unit tests: all 5 branding types produce correct flag combinations; brand_logo URL prefixed; nil logo → brand_logo nil
