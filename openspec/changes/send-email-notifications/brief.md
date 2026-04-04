## Source Files

- `spec/behavioral-spec/notifications.md` — email sections (POST /notifications/email, POST /v2/notifications/email, GET /v2/notifications/{id}, GET /v2/notifications, GET /notifications/{id}, GET /notifications, DAO behaviour contracts)
- `spec/business-rules/notifications.md` — rate limiting, daily limits, annual limits, personalisation validation, data retention, status transitions

## Requirements

### R1: POST /v2/notifications/email — Create and Enqueue

- Required fields: `email_address` (string), `template_id` (UUID string)
- Optional fields: `reference` (string), `personalisation` (object), `email_reply_to_id` (UUID), `scheduled_for` (ISO8601 string), `one_click_unsubscribe_url` (string)
- Response 201: `{id, reference, content: {body, subject, from_email}, uri, template: {id, version, uri}, scheduled_for}`
- Notification persisted to `notifications` table; signed blob published to Redis queue based on template `process_type`
- `one_click_unsubscribe_url` if provided: stored on notification row, passed through in response
- v2 error format: `{"errors": [{"error": "ValidationError", "message": "..."}], "status_code": 400}`

### R2: POST /notifications/email — Legacy v0 Create

- Required fields: `to` (email address string), `template` (UUID string)
- Optional fields: `personalisation` (object)
- Response 201: `{"data": {"notification": {"id": "<uuid>"}, "body": "<rendered>", "subject": "<rendered>", "template_version": N}}`
- Body is plain text (no HTML); newlines preserved as-is; `content_char_count` is `null` for email
- Same service layer as v2; different request/response schema
- v0 error format: `{"result": "error", "message": {...}}`

### R3: Email Address Validation

- `email_address` must be a string type (not int, null, array) → 400 ValidationError `"email_address {val} is not of type string"`
- Must be valid RFC 5322 address; parse via `net/mail.ParseAddress`
- Bracketed display names (e.g. `"Name <addr>"`) are rejected → 400 ValidationError
- Invalid format → 400 ValidationError `"Not a valid email address"` (v0) / ValidationError (v2)

### R4: Template Validation

- `template_id` must be a valid UUID → ValidationError `"template_id is not a valid UUID"`
- Template must exist and belong to the authenticated service → 400 BadRequestError `"Template not found"`
- Template type must be `email`; mismatch (e.g. SMS template on email endpoint) → BadRequestError `"{type} template is not suitable for {type} notification"`
- Template must not be archived → BadRequestError `"Template {id} has been deleted"`
- All `((placeholder))` variables declared in template body must be present in `personalisation` → BadRequestError `{"template": ["Missing personalisation: <Name>"]}`
- Extra `personalisation` keys beyond declared placeholders: silently ignored
- Rendered body must not be blank/whitespace → BadRequestError `"Message is empty or just whitespace"`

### R5: Personalisation Validation

- `personalisation` must be a JSON object (not string, not array) → ValidationError `"personalisation {val} is not of type object"`
- Total JSON-serialised size of non-file personalisation values ≤ 51,200 bytes → ValidationError `"Personalisation variables size of {N} bytes is greater than allowed limit of 51200 bytes"`
- Size check performed at parse time, before template rendering

### R6: Email Reply-To Override

- If `email_reply_to_id` provided: must reference an active (non-archived) `service_email_reply_to` record for the service
  - Archived or not found → 400 BadRequestError `"email_reply_to_id {id} does not exist in database for service id {id}"`
  - `reply_to_text` stored on `notifications` row at creation time
- If not provided: service's default email reply-to address is fetched and stored
- Stored `reply_to_text` value is immutable; later service reply-to changes do NOT retroactively update stored notifications

### R7: Document Attachment Validation

- Service must have `upload_document` permission; without it → error
- Total document count per notification ≤ 10 → ValidationError `"File number exceed allowed limits of 10 with number of {N}."`
- Each document must have `sending_method` in `["attach", "link"]` → ValidationError `"personalisation {method} is not one of [attach, link]"`
- `filename`: required for `attach` method; optional for `link`; valid length 2–255 chars
- `file`: must be valid base64 → 400 with decode error message per key
- Decoded file size ≤ 10 MB → ValidationError `"and greater than allowed limit of"`
- All document errors accumulated and returned as a list of ValidationErrors

### R8: Simulated Email Addresses

- Three addresses that bypass all processing:
  - `simulate-delivered@notification.canada.ca`
  - `simulate-delivered-2@notification.canada.ca`
  - `simulate-delivered-3@notification.canada.ca`
- Checked BEFORE template lookup, personalisation validation, limit checks, DB insert
- Return HTTP 201 with mock response body; NO `notifications` row created; NO queue publish; NO limit function calls
- For `link` documents in simulated requests: return a simulated document URL in response

### R9: Service Email Permission Check

- Authenticated service must have `email` permission → 400 BadRequestError `"Service is not allowed to send emails"`
- Check is performed before limit enforcement

### R10: Suspended Service

- If service `active = false` (suspended) → HTTP 403 before any other processing

### R11: Scheduled Send

- Requires service to have `SCHEDULE_NOTIFICATIONS` permission → BadRequestError `"Cannot schedule notifications (this feature is invite-only)"`
- `scheduled_for` must be string → ValidationError `"scheduled_for {N} is not of type string, null"`
- `scheduled_for` must not be in the past
- `scheduled_for` max 24 h in the future (single sends; bulk allows 96 h)
- Scheduled notifications: persisted to `notifications` + `scheduled_notifications` table; NOT dispatched to queue at creation; `pending = true` in `scheduled_notifications`

### R12: Rate and Limit Enforcement (Email)

- Enforcement order (first breached stops processing): per-minute rate → annual email → daily email
- **Rate limit** (requires `API_RATE_LIMIT_ENABLED` AND `REDIS_ENABLED`): Redis sorted-set `rate_limit:{service_id}:{key_type}`; window 60 s; limit `service.rate_limit`; raises RateLimitError → 429 `"Exceeded rate limit for key type {TYPE} of {N} requests per {INTERVAL} seconds"`
- **Annual email limit**: Redis hash `annual_limit_notifications_v2:{service_id}` field `TOTAL_EMAIL_FISCAL_YEAR_TO_YESTERDAY` + today's Redis daily counter; threshold `total_used + 1 > service.email_annual_limit`; trial → `TrialServiceRequestExceedsEmailAnnualLimitError` (429); live → `LiveServiceRequestExceedsEmailAnnualLimitError` (429)
- **Daily email limit**: Redis key `email_daily_count:{service_id}` (2-hour TTL); seeded lazily from `fetch_todays_total_email_count` on cache miss; threshold `(emails_sent_today + 1) > service.message_limit`; trial → `TrialServiceTooManyEmailRequestsError` 429; live → `LiveServiceTooManyEmailRequestsError` 429
- **Warning emails**: sent once at ≥ 80% of daily limit and once at ≥ 100%; deduplicated via Redis keys with TTL = seconds until midnight
- **Annual seeding zero-count guard**: if all seeded annual values are zero, `set_seeded_at()` called to prevent infinite re-seeding loop
- `test` key type: skip ALL limit checks; notification IS persisted normally
- Simulated addresses: skip ALL limit checks
- `billable_units` for email is always `1` (email is not fragment-counted)

### R13: Queue Routing (Email)

- template `process_type = "priority"` → `email_priority_publish` (high)
- template `process_type = "bulk"` → `email_bulk_publish` (low)
- default / `process_type = "normal"` → `email_normal_publish`
- Queue receives a signed notification blob (not a direct SES delivery call)
- If queue publish fails after DB insert: delete the notification row and return error to caller

### R14: GET /v2/notifications/{notification_id} — v2 Single

- Returns HTTP 200 with full notification object for authenticated service
- Response fields: `{id, reference, email_address, phone_number, type, status, status_description, provider_response, template: {id, version, uri}, created_at, created_by_name, body, subject, sent_at, completed_at, scheduled_for, postage}`
- `subject` present for email; `phone_number` null for email
- Invalid UUID in path → 400 ValidationError `"notification_id is not a valid UUID"`
- Notification not found → 404 `{"message": "Notification not found in database", "result": "error"}`
- Notification belongs to a different service → 404

### R15: GET /v2/notifications — v2 List

- Returns 200 `{"notifications": [...], "links": {"current": "...", "next": "..."}}` for authenticated service
- Excludes job-created notifications by default; ordered newest-first
- Query params and validation:
  - `template_type`: sms / email / letter → invalid value: 400 `"template_type {val} is not one of [sms, email, letter]"`
  - `status`: see valid status list → invalid: 400 `"status {val} is not one of [...]"`
  - `older_than`: UUID cursor → invalid UUID: 400 `"older_than is not a valid UUID"`; non-existent UUID: 200 empty list
  - `reference`: string, AND filter
  - `include_jobs`: "true" — include job-created notifications
- `status=failed` expands to: `technical-failure, temporary-failure, permanent-failure`
- Multiple filters combined with AND semantics
- Each key type only sees notifications created by that same key type (normal sees normal, test sees test)

### R16: GET /notifications/{id} — Legacy v0 Single

- Returns 200 `{"data": {"notification": {id, status, template, to, service, body, subject?, content_char_count}}}`
- `subject` present for email only; `content_char_count` is null for email
- Template body rendered using stored personalisation at the template version active at creation
- Not found → 404 `"Notification not found in database"`
- Malformed UUID in path → 405

### R17: GET /notifications — Legacy v0 List

- Returns 200 `{"notifications": [...], "total": N, "page_size": N, "links": {last?, prev?, next?}}`
- Default: excludes job notifications; excludes test-key notifications; ordered newest-first
- Query params: `template_type` (sms/email/letter), `status`, `page`, `page_size`, `include_jobs` (bool)
- Invalid `page` / `page_size` → 400 `"Not a valid integer."`
- Normal key with `include_jobs=true` returns job + API notifications
- Team/test keys with `include_jobs=true` still return only their own key-type notifications

## Error Conditions

| Condition | HTTP Status | Error |
|---|---|---|
| Missing `email_address` | 400 | `ValidationError: email_address is a required property` |
| Missing `template_id` | 400 | `ValidationError: template_id is a required property` |
| `email_address` not a string | 400 | ValidationError: `"email_address {val} is not of type string"` |
| Invalid email format | 400 | ValidationError |
| `template_id` invalid UUID | 400 | `"template_id is not a valid UUID"` |
| Template not found / not owned by service | 400 | `BadRequestError: Template not found` |
| Template type mismatch | 400 | BadRequestError |
| Archived template | 400 | `BadRequestError: Template {id} has been deleted` |
| Missing personalisation placeholder | 400 | `{"template": ["Missing personalisation: {Name}"]}` |
| Personalisation too large (> 51,200 bytes) | 400 | ValidationError |
| `email_reply_to_id` not found or archived | 400 | BadRequestError |
| Document count > 10 | 400 | ValidationError |
| Invalid `sending_method` | 400 | ValidationError |
| Base64 decode failure | 400 | ValidationError |
| File > 10 MB | 400 | ValidationError |
| Service lacks email permission | 400 | `BadRequestError: Service is not allowed to send emails` |
| Schedule permission missing | 400 | BadRequestError |
| Rate limit exceeded | 429 | RateLimitError |
| Annual limit exceeded | 429 | Annual limit error |
| Daily limit exceeded | 429 | Daily limit error (trial or live variant) |
| Service suspended | 403 | — |
| No auth token | 401 | AuthError |

## Business Rules

- Simulated address check is earliest possible short-circuit — before template fetch, limits, DB write
- Limit check order (rate → annual → daily) must match Python; different order changes edge-case behaviour
- `reply_to_text` stored at creation; immutable thereafter
- `test` key: skips all limits; notification IS persisted (visible in GET /v2/notifications)
- `key_type` stored on notification row from the API key's `key_type` field
- Data retention: notifications older than `service_data_retention` days (default 7) archived to `notification_history` then deleted from `notifications`; test-key notifications deleted only (never archived)
- Annual limit Redis hash seeded once per day; today's count from daily Redis counter (not DB query); zero-guard prevents infinite re-seeding
- Queue publish failure → notification row deleted (no orphaned `created` status notifications)
- `billable_units` for email is always 1; no fragment counting
