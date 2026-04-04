## 1. Receipt Queue Consumer and Routing

- [ ] 1.1 Implement `internal/worker/receipts/handler.go` — SQS consumer for `delivery-receipts`; inspect SNS envelope `TopicArn` / `MessageAttributes` to determine provider type; route to `processSESResult`, `processSNSResult`, or `processPinpointResult`; drop and log unrecognised messages
- [ ] 1.2 Write unit tests with fixture SNS envelope payloads for SES, SNS, and Pinpoint routing; assert correct handler called per provider type; assert unrecognised source is dropped without panic

## 2. SES Receipt Processing

- [ ] 2.1 Implement `internal/worker/receipts/process_ses_result.go` — parse SES notification batch; call `separate_complaint_and_non_complaint_receipts`; handle delivery, hard bounce (3 subtypes), soft bounce (2 subtypes); set feedback_type/feedback_subtype; call `UpdateNotificationStatusByReference`
- [ ] 2.2 Implement status-transition guard: skip status update if current status is already `permanent-failure` and incoming event is a delivery; allow `delivered → permanent-failure` transition on hard bounce
- [ ] 2.3 Implement partial batch retry: re-queue not-found notifications individually to `retry-tasks`; dispatch callbacks for successfully resolved notifications in same pass; log and drop on MaxRetriesExceeded
- [ ] 2.4 Implement bounce-rate Redis calls: `set_sliding_hard_bounce` on hard bounce; no Redis call on soft bounce
- [ ] 2.5 Implement annual limit calls: `increment_email_delivered` on delivery; `increment_email_failed` on any bounce; first-of-day seeding path via `seed_annual_limit_notifications`
- [ ] 2.6 Write unit tests: delivery → delivered; General hard bounce → permanent-failure + null provider_response; Suppressed hard bounce → correct provider_response string; OnAccountSuppressionList hard bounce → GC Notify suppression string; General soft bounce → temporary-failure; AttachmentRejected soft bounce → correct provider_response; already-permanent-failure guard; delivered-to-permanent-failure transition; bounce rate Redis calls; partial batch retry; annual limit seeding

## 3. SES Complaint Processing

- [ ] 3.1 Implement `handle_complaint` helper: create `Complaint` row linked to notification; call `remove_emails_from_complaint` to scrub PII from stored JSON; fall back to `notification_history` lookup if notification not in main table
- [ ] 3.2 Enqueue `send-complaint` task after complaint is persisted (only if service has a registered complaint callback)
- [ ] 3.3 Write unit tests: complaint receipt → Complaint row created + notification status unchanged; PII scrub applied; notification_history fallback works; send-complaint task enqueued

## 4. Pinpoint Receipt Processing (C3 fix)

- [ ] 4.1 Implement `internal/worker/receipts/process_pinpoint_result.go` — parse Pinpoint SMS V2 event; check `isFinal` flag (SUCCESSFUL + isFinal=false → early return, no callback); extract `messageStatusDescription`; apply 13-entry status mapping table; store up to 7 SMS metadata fields on delivered outcome
- [ ] 4.2 Implement shortcode delivery path: detect short-code origination; store short code as `sms_origination_phone_number`; set `provider_response = "Message has been accepted by phone carrier"`
- [ ] 4.3 Implement wrong-provider guard: if `notification.sent_by != "pinpoint"` log exception and return without status update
- [ ] 4.4 Implement annual limit calls: `increment_sms_delivered` on DELIVERED; `increment_sms_failed` on failures; first-of-day seeding path; `FF_USE_BILLABLE_UNITS` billable-unit fields in seed payload
- [ ] 4.5 Write unit tests: all 13 provider_response → status mappings (one test per string); SUCCESSFUL isFinal=false → no update + no callback; DELIVERED → 7 metadata fields stored; shortcode origination; missing SMS fields → null on non-fatal fields; wrong provider → no update; MaxRetriesExceeded log; annual limit seeding

## 5. SNS SMS Receipt Processing

- [ ] 5.1 Implement `internal/worker/receipts/process_sns_result.go` — parse SNS SMS delivery report; extract `messageId`, `status`, `providerResponse`; apply status mapping; call `UpdateNotificationStatusByReference`; emit annual limit increment; enqueue delivery-status callback
- [ ] 5.2 Write unit tests: successful delivery → delivered; failure status mapping; wrong provider guard

## 6. send-delivery-status Callback Worker

- [ ] 6.1 Implement `internal/worker/callbacks/send_delivery_status.go` — consume `service-callbacks` queue; verify itsdangerous HMAC signature on `signed_status_update`; look up service callback URL and decrypt bearer token; POST with `Authorization: Bearer <signed_token>` and 5 s timeout; delete SQS message on 2xx; re-queue to `service-callbacks-retry` on 5xx/429/connection error; drop on 4xx (except 429)
- [ ] 6.2 Implement POST body schema: `{id, reference, to, status, status_description, provider_response, created_at, completed_at, sent_at, notification_type}`
- [ ] 6.3 Write unit tests: 200 → SQS delete; 503 → re-enqueue to retry queue; 404 → drop; 429 → retry; connection error → retry; invalid signature → drop without HTTP call; correct Bearer header present

## 7. SSRF Guard

- [ ] 7.1 Implement `internal/worker/callbacks/ssrf_guard.go` — validate callback URL before any outbound call: reject non-HTTPS scheme; resolve hostname; reject if resolved IP in RFC 1918 (10/8, 172.16/12, 192.168/16) or loopback (127/8, ::1); return error without opening TCP connection
- [ ] 7.2 Write unit tests: http:// URL → rejected; 10.0.0.1 → rejected; 172.16.0.1 → rejected; 192.168.1.1 → rejected; 127.0.0.1 → rejected; ::1 → rejected; valid public HTTPS URL → passes guard

## 8. send-complaint Callback Worker

- [ ] 8.1 Implement `internal/worker/callbacks/send_complaint.go` — consume `service-callbacks` queue for complaint type; verify signature on `complaint_data`; POST to service complaint callback URL; body: `{notification_id, complaint_id, reference, to, complaint_date}`; same retry policy as send-delivery-status; same SSRF guard
- [ ] 8.2 Write unit tests: correct body fields; 200 → delete; 503 → retry; 400 → drop; private IP → rejected; signed Bearer header present

## 9. Worker Manager Integration

- [ ] 9.1 Register `delivery-receipts` consumer pool, `service-callbacks` pool, and `service-callbacks-retry` pool in `WorkerManager.Start(ctx)`; configure per-queue concurrency from config
- [ ] 9.2 Write integration test: publish SES delivery receipt to mock SQS → receipt worker updates notification status → `send-delivery-status` enqueued → callback worker POSTs to mock HTTP server → assert notification status = delivered and callback received correct body
