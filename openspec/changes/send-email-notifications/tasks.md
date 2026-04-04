## 1. Service Layer: Email Validation & Business Logic

- [ ] 1.1 Implement `internal/service/notifications/email.go` — `SendEmail(ctx, serviceID, req EmailRequest) (*Notification, error)`: check service suspended (403), validate email address (type + RFC 5322), check service email permission, check `SCHEDULE_NOTIFICATIONS` permission if `scheduled_for` provided, resolve email reply-to, validate personalisation (object type, size ≤ 51,200 bytes, placeholder completeness), validate documents, check simulated addresses (early-return 201), enforce limits (rate → annual → daily), persist notification + optional `scheduled_notifications` row, publish to Redis queue (or skip if scheduled); delete notification and return error if queue publish fails
- [ ] 1.2 Implement simulated address check helper in `internal/service/notifications/simulate.go`; write unit tests: each of the 3 addresses returns mock 201 response, does not call DB insert or queue publish or any limit function
- [ ] 1.3 Implement `internal/service/notifications/limits.go` — `CheckRateLimit(ctx, serviceID, keyType)`, `CheckAnnualEmailLimit(ctx, serviceID)`, `CheckDailyEmailLimit(ctx, serviceID)`; write unit tests: below limit passes, exactly at limit passes, exceeded returns 429-category error; test key skips all three; simulated address caller skips all three

## 2. Template Rendering & Reply-To Resolution

- [ ] 2.1 Implement `internal/service/notifications/render.go` — `RenderTemplate(body string, personalisation map[string]string) (string, error)`: replace `((key))` placeholders using exact string replacement; return error listing all missing required placeholders; extra keys silently ignored; return error if rendered body is blank/whitespace; write unit tests covering: all placeholders substituted, missing required placeholder error, extra key ignored, blank rendered body error
- [ ] 2.2 Implement `resolveEmailReplyTo(ctx, serviceID, emailReplyToID *uuid.UUID) (replyTo string, err error)` — if ID provided: validate it exists and is not archived for the service; else: fetch service default; write unit tests: valid override used, archived override returns 400 BadRequestError, no override uses service default
- [ ] 2.3 Implement `validatePersonalisation(templateBody string, p map[string]any) error` — check all `((key))` placeholders in body are present in map and total JSON size ≤ 51,200 bytes; write unit tests: all present passes, one missing fails with correct message, exactly 51,200 bytes passes, 51,201 bytes fails

## 3. Document Attachment Validation & Upload

- [ ] 3.1 Implement document validation in `internal/service/notifications/documents.go` — `validateDocuments(service *Service, docs []Document) []ValidationError`: check service `upload_document` permission, count ≤ 10, per-document: `sending_method` in `[attach, link]`, filename length (2–255, required for attach), base64 valid, decoded size ≤ 10 MB; accumulate all errors regardless of which document fails; write table-driven unit tests for each error type and multi-error accumulation
- [ ] 3.2 Implement S3 upload path for `attach` documents: decode base64, upload to S3 key derived from notification ID, replace document object in personalisation with signed URL before persisting; write unit test using mock S3 client: successful upload replaces document, S3 error propagates

## 4. v2 API Handlers

- [ ] 4.1 Implement `internal/handler/v2/notifications/email.go` — POST /v2/notifications/email: parse and validate JSON request `{email_address, template_id, reference?, personalisation?, email_reply_to_id?, scheduled_for?, one_click_unsubscribe_url?}`; call service layer; map typed errors to v2 format `{"errors": [{"error": "...", "message": "..."}], "status_code": 400}`; return 201 `{id, reference, content: {body, subject, from_email}, uri, template: {id, version, uri}, scheduled_for}`
- [ ] 4.2 Implement GET /v2/notifications/{notification_id}: validate UUID path param (400 on invalid), fetch by id scoped to service, return 200 with full schema including all fields; 404 if not found or belongs to another service
- [ ] 4.3 Implement GET /v2/notifications: parse and validate query params (`template_type`, `status`, `older_than`, `reference`, `include_jobs`); validate enum values (400 on invalid); expand `status=failed` to three statuses; call repository list with cursor pagination; return `{"notifications": [...], "links": {"current": "...", "next": "..."}}`

## 5. Legacy v0 API Handlers

- [ ] 5.1 Implement `internal/handler/notifications/email.go` — POST /notifications/email: request `{to, template, personalisation?}`; call same service layer with schema translation (`to` → `email_address`, `template` → `template_id`); response `{"data": {"notification": {"id": "<uuid>"}, "body": "<rendered>", "subject": "<rendered>", "template_version": N}}`; map errors to v0 format `{"result": "error", "message": {...}}`
- [ ] 5.2 Implement GET /notifications/{id} legacy handler — response `{"data": {"notification": {id, status, template, to, service, body, subject?, content_char_count}}}`; render template from stored personalisation; `subject` for email, null for SMS; `content_char_count` null for email; 404 body `"Notification not found in database"`; 405 on malformed UUID path
- [ ] 5.3 Implement GET /notifications legacy list handler — support `template_type`, `status`, `page`, `page_size`, `include_jobs`; response `{"notifications": [...], "total": N, "page_size": N, "links": {last?, prev?, next?}}`; invalid page/page_size → 400 `"Not a valid integer."`; normal key with `include_jobs=true` returns job and API notifications; team/test keys with `include_jobs=true` return only own key-type notifications

## 6. Notification Repository

- [ ] 6.1 Implement `internal/repository/notifications/create.go` — `CreateNotification(ctx, n *Notification) error`: insert row to `notifications`, assign UUID if absent, default status to `created`; write unit test: row inserted with UUID and `created` status
- [ ] 6.2 Implement `internal/repository/notifications/get.go` — `GetNotificationByID(ctx, serviceID, notificationID uuid.UUID) (*Notification, error)`: SELECT on read replica scoped to `service_id`; return `ErrNotFound` if not found or service mismatch
- [ ] 6.3 Implement `GetNotificationsForService(ctx, serviceID uuid.UUID, filter NotificationFilter) (*Page[Notification], error)`: cursor pagination via `older_than`, filters for `status`, `template_type`, `key_type`, `reference`, `include_jobs`; ORDER BY `created_at DESC`; write unit tests for each filter combination

## 7. Error Mapping & Integration Tests

- [ ] 7.1 Define typed error hierarchy in `internal/service/notifications/errors.go`: `ValidationError` → 400, `BadRequestError` → 400, `RateLimitError` → 429, `AnnualLimitError` → 429, `DailyLimitError` → 429, `ForbiddenError` → 403, `NotFoundError` → 404; write unit test confirming correct HTTP status code for each
- [ ] 7.2 Write httptest integration tests for POST /v2/notifications/email: valid 201, invalid email 400, missing template_id 400, archived template 400, type mismatch 400, missing personalisation placeholder 400, oversized personalisation 400, simulated address 201 (verify no DB row), annual limit 429, daily limit 429, rate limit 429, service lacks email permission 400, suspended service 403, no auth header 401, schedule without permission 400
- [ ] 7.3 Write httptest integration tests for GET /v2/notifications/{id}: found 200 with correct schema, not found 404, other-service ID 404, invalid UUID 400; for GET /v2/notifications: unfiltered 200, filtered by status 200, filtered by template_type 200, `status=failed` expands correctly, invalid status 400, invalid older_than 400, cursor pagination works
