## ADDED Requirements

### Requirement: POST /v2/notifications/email creates and enqueues email notification
The endpoint SHALL accept `{"email_address": "<email>", "template_id": "<uuid>"}` with optional `reference`, `personalisation`, `email_reply_to_id`, `scheduled_for`, and `one_click_unsubscribe_url`, and return HTTP 201 with `{id, reference, content: {body, subject, from_email}, uri, template: {id, version, uri}, scheduled_for}`. The notification SHALL be persisted to the `notifications` table and a signed notification blob SHALL be published to the appropriate Redis queue based on template `process_type`.

#### Scenario: Valid email request returns 201
- **WHEN** a valid POST /v2/notifications/email is made with required fields
- **THEN** HTTP 201 is returned with `id`, `content.body`, `content.subject`, and `content.from_email` populated

#### Scenario: Template process_type priority routes to high-priority queue
- **WHEN** the template has `process_type = "priority"`
- **THEN** the notification blob is published to `email_priority_publish` queue

#### Scenario: Template process_type bulk routes to low-priority queue
- **WHEN** the template has `process_type = "bulk"`
- **THEN** the notification blob is published to `email_bulk_publish` queue

#### Scenario: Default template routes to normal queue
- **WHEN** the template has `process_type = "normal"` or no process_type override
- **THEN** the notification blob is published to `email_normal_publish` queue

#### Scenario: Scheduled notification not immediately dispatched
- **WHEN** `scheduled_for` is provided and is valid (not past, ≤ 24 h in future)
- **THEN** the notification is persisted with `scheduled_for` set and NOT dispatched to the queue

#### Scenario: Queue publish failure rolls back notification
- **WHEN** the Redis queue publish call fails after the DB insert
- **THEN** the notification row is deleted from the DB and an error is returned to the caller

#### Scenario: `one_click_unsubscribe_url` stored and returned
- **WHEN** `one_click_unsubscribe_url` is provided in the request
- **THEN** it is stored on the notification row and included in the 201 response

---

### Requirement: POST /notifications/email — legacy v0 create
The legacy endpoint SHALL accept `{"to": "<email>", "template": "<uuid>"}` with optional `personalisation` and return HTTP 201 with `{"data": {"notification": {"id": "<uuid>"}, "body": "<rendered>", "subject": "<rendered>", "template_version": N}}`. Body is plain text; `content_char_count` is `null` for email. Same service-layer call as v2 with translated parameters.

#### Scenario: Legacy v0 creates notification and returns id
- **WHEN** a valid POST /notifications/email is made with `to` and `template`
- **THEN** HTTP 201 is returned with `data.notification.id`, `data.body`, `data.subject`, and `data.template_version`

#### Scenario: Legacy v0 uses v0 error format
- **WHEN** `to` is missing
- **THEN** HTTP 400 is returned with body `{"result": "error", "message": {"to": ["Missing data for required field."]}}`

---

### Requirement: Email address validation
`email_address` SHALL be validated as a properly formatted RFC 5322 email address. Non-string types (int, null, array) SHALL return HTTP 400 ValidationError. Invalid format SHALL return HTTP 400 ValidationError. Bracketed display names SHALL be rejected.

#### Scenario: Invalid email address rejected
- **WHEN** `email_address` is `"not-an-email"`
- **THEN** HTTP 400 is returned with a validation error on `email_address`

#### Scenario: Integer email_address rejected
- **WHEN** `email_address` is the integer `12345`
- **THEN** HTTP 400 ValidationError `"email_address 12345 is not of type string"`

#### Scenario: Bracketed display name rejected
- **WHEN** `email_address` is `"Name <user@example.com>"`
- **THEN** HTTP 400 ValidationError

---

### Requirement: Template validation
The template SHALL exist for the authenticated service, SHALL NOT be archived, and its type SHALL match the endpoint type (`email`).

#### Scenario: Template not found returns error
- **WHEN** `template_id` is a valid UUID but does not exist for the service
- **THEN** HTTP 400 BadRequestError `"Template not found"`

#### Scenario: Archived template rejected
- **WHEN** `template_id` references a template that has been archived
- **THEN** HTTP 400 BadRequestError `"Template {id} has been deleted"`

#### Scenario: SMS template on email endpoint rejected
- **WHEN** `template_id` references an SMS template
- **THEN** HTTP 400 BadRequestError `"email template is not suitable for sms notification"` (or matching variant)

#### Scenario: Invalid template_id UUID rejected
- **WHEN** `template_id` is `"not-a-uuid"`
- **THEN** HTTP 400 ValidationError `"template_id is not a valid UUID"`

#### Scenario: Blank rendered body rejected
- **WHEN** the template body renders to whitespace only (e.g. template body is `"  "`)
- **THEN** HTTP 400 BadRequestError `"Message is empty or just whitespace"`

---

### Requirement: Email personalisation validation
If `personalisation` is provided it SHALL be validated as a JSON object (not string, array, etc.) and the total serialised size SHALL be ≤ 51,200 bytes. All placeholders declared in the template SHALL be present in `personalisation`; missing → HTTP 400 `{"template": ["Missing personalisation: <Name>"]}`. Extra keys are silently ignored.

#### Scenario: Personalisation exceeds 51,200 bytes
- **WHEN** the serialised personalisation object is 51,201 bytes
- **THEN** HTTP 400 ValidationError `"Personalisation variables size of 51201 bytes is greater than allowed limit of 51200 bytes"`

#### Scenario: Missing required placeholder
- **WHEN** the template declares `((name))` but `personalisation` has no `name` key
- **THEN** HTTP 400 `{"template": ["Missing personalisation: name"]}`

#### Scenario: Personalisation not an object
- **WHEN** `personalisation` is the string `"hello"`
- **THEN** HTTP 400 ValidationError `"personalisation hello is not of type object"`

---

### Requirement: Email reply-to override
If `email_reply_to_id` is provided it SHALL reference a non-archived `service_email_reply_to` record belonging to the service. If not provided, the service's default reply-to address SHALL be used. The reply-to value is stored on the notification at creation and does not change if the service's default later changes.

#### Scenario: Valid email_reply_to_id accepted
- **WHEN** `email_reply_to_id` references an active reply-to record for the service
- **THEN** that address is stored as `reply_to_text` on the notification

#### Scenario: Archived reply-to rejected
- **WHEN** `email_reply_to_id` references an archived reply-to record
- **THEN** HTTP 400 BadRequestError `"email_reply_to_id {id} does not exist in database for service id {id}"`

#### Scenario: No reply-to id uses service default
- **WHEN** `email_reply_to_id` is not provided
- **THEN** the service's default email reply-to address is stored on the notification

---

### Requirement: Document attachment validation
Documents in `personalisation` SHALL be validated: (1) service SHALL have `upload_document` permission; (2) total document count SHALL be ≤ 10; (3) each document SHALL have `sending_method` in `["attach", "link"]`; (4) `filename` SHALL be 2–255 characters (required for `attach`); (5) `file` SHALL be valid base64; (6) decoded file size SHALL be ≤ 10 MB.

#### Scenario: More than 10 documents rejected
- **WHEN** personalisation contains 11 document objects
- **THEN** HTTP 400 ValidationError `"File number exceed allowed limits of 10 with number of 11."`

#### Scenario: Invalid sending_method rejected
- **WHEN** a document has `sending_method: "fax"`
- **THEN** HTTP 400 ValidationError `"personalisation fax is not one of [attach, link]"`

#### Scenario: Oversized file rejected
- **WHEN** the decoded file is 10 MB + 1 byte
- **THEN** HTTP 400 ValidationError containing `"greater than allowed limit of"`

---

### Requirement: Simulated email addresses
Requests to the three simulated addresses (`simulate-delivered@notification.canada.ca`, `simulate-delivered-2@notification.canada.ca`, `simulate-delivered-3@notification.canada.ca`) SHALL return HTTP 201 without persisting a notification row, without publishing to any queue, and without calling limit-check functions. The simulated address check occurs before template lookup, personalisation validation, and DB insert.

#### Scenario: Simulated address returns 201 without DB write
- **WHEN** POST /v2/notifications/email is sent to `simulate-delivered@notification.canada.ca`
- **THEN** HTTP 201 is returned and no row exists in the `notifications` table

#### Scenario: All three simulated addresses bypass processing
- **WHEN** POST /v2/notifications/email is sent to `simulate-delivered-2@notification.canada.ca` or `simulate-delivered-3@notification.canada.ca`
- **THEN** HTTP 201 is returned without limit checks and without queue publish

---

### Requirement: Email send permission check
The authenticated service SHALL have the `email` service permission. If not → HTTP 400 `"Service is not allowed to send emails"`.

#### Scenario: Service without email permission rejected
- **WHEN** the service does not have the `email` permission
- **THEN** HTTP 400 BadRequestError `"Service is not allowed to send emails"`

---

### Requirement: Suspended service blocked
If the service `active = false`, the request SHALL be rejected with HTTP 403 before any other processing.

#### Scenario: Suspended service returns 403
- **WHEN** the authenticated service has `active = false`
- **THEN** HTTP 403 is returned regardless of request validity

---

### Requirement: Scheduled send permission and validation
`scheduled_for` requires the `SCHEDULE_NOTIFICATIONS` service permission and an ISO8601 datetime that is in the future and no more than 24 hours from now.

#### Scenario: Schedule without permission rejected
- **WHEN** `scheduled_for` is provided but the service lacks `SCHEDULE_NOTIFICATIONS` permission
- **THEN** HTTP 400 BadRequestError `"Cannot schedule notifications (this feature is invite-only)"`

#### Scenario: scheduled_for in the past rejected
- **WHEN** `scheduled_for` is an ISO8601 datetime 1 minute in the past
- **THEN** HTTP 400 ValidationError

#### Scenario: Non-string scheduled_for rejected
- **WHEN** `scheduled_for` is the integer `1234567890`
- **THEN** HTTP 400 ValidationError `"scheduled_for 1234567890 is not of type string, null"`

---

### Requirement: Email rate and limit enforcement
Email send requests SHALL enforce, in order: (1) per-minute API rate limit by key type; (2) annual email limit; (3) daily email limit. Any exceeded limit SHALL return HTTP 429. Test-key sends SHALL skip all limit checks. Simulated addresses SHALL skip all limit checks.

#### Scenario: Annual limit exceeded returns 429
- **WHEN** the service has exhausted its annual email allocation
- **THEN** HTTP 429 is returned and the notification is not persisted

#### Scenario: Daily limit exceeded returns 429
- **WHEN** the service has exhausted its daily email message limit
- **THEN** HTTP 429 is returned and the notification is not persisted

#### Scenario: Rate limit exceeded returns 429
- **WHEN** the service has exceeded its per-minute API rate limit
- **THEN** HTTP 429 is returned with `"Exceeded rate limit for key type {TYPE} of {N} requests per {INTERVAL} seconds"`

#### Scenario: Test key bypasses limits
- **WHEN** a notification is sent using a `test` key type
- **THEN** no limit functions are called and the notification is persisted normally

#### Scenario: Limit check order: rate before annual before daily
- **WHEN** both the rate limit and annual limit are exceeded simultaneously
- **THEN** the rate limit error (not the annual limit error) is returned
