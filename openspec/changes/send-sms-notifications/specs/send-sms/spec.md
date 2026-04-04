## ADDED Requirements

### Requirement: POST /v2/notifications/sms creates and enqueues SMS notification
The endpoint SHALL accept `{"phone_number": "<e164>", "template_id": "<uuid>"}` with optional `reference`, `personalisation`, `sms_sender_id`, `scheduled_for` and return HTTP 201 with `{id, reference, content: {body, from_number}, uri, template: {id, version, uri}, scheduled_for}`. The notification SHALL be persisted and a signed blob SHALL be published to the queue determined by key type, sender, and template `process_type`. Additional unknown properties SHALL return HTTP 400 ValidationError.

#### Scenario: Valid SMS request returns 201
- **WHEN** a valid POST /v2/notifications/sms is made with a valid phone number and template ID
- **THEN** HTTP 201 is returned with `id`, `content.body`, and `content.from_number` populated

#### Scenario: Test key routes to research-mode queue
- **WHEN** the request uses a `test` API key
- **THEN** the notification is published to `research-mode-tasks`

#### Scenario: Throttled sender routes to throttled queue
- **WHEN** the resolved sender number is `+14383898585`
- **THEN** the notification is published to `send-throttled-sms-tasks`

#### Scenario: Priority template routes to high-priority queue
- **WHEN** the template has `process_type = "priority"` and the sender is NOT the throttled number
- **THEN** the notification is published to `send-sms-high`

#### Scenario: Bulk template routes to low-priority queue
- **WHEN** the template has `process_type = "bulk"` and the sender is not throttled
- **THEN** the notification is published to `send-sms-low`

#### Scenario: Default template routes to normal queue
- **WHEN** the template has `process_type = "normal"` and no special conditions apply
- **THEN** the notification is published to `send-sms-medium`

#### Scenario: Priority template with throttled sender uses throttled queue
- **WHEN** a priority-process-type template is sent from the throttled sender number (+14383898585)
- **THEN** the notification goes to `send-throttled-sms-tasks`, NOT `send-sms-high`

#### Scenario: service.research_mode routes to research-mode queue
- **WHEN** the service has `research_mode = true` (regardless of key type)
- **THEN** the notification is published to `research-mode-tasks`

#### Scenario: Additional unknown property rejected
- **WHEN** the request body includes `"email_reply_to_id": "<uuid>"` (an email-only field)
- **THEN** HTTP 400 ValidationError `"Additional properties are not allowed (email_reply_to_id was unexpected)"`

---

### Requirement: POST /notifications/sms — legacy v0 create
The legacy endpoint SHALL accept `{"to": "<phone>", "template": "<uuid>"}` with optional `personalisation` and return HTTP 201 with `{"data": {"notification": {"id": "<uuid>"}, "body": "<rendered>", "template_version": N}}`. Both `to` and `template` are required.

#### Scenario: Legacy v0 creates notification and returns id
- **WHEN** a valid POST /notifications/sms is made with `to` and `template`
- **THEN** HTTP 201 is returned with `data.notification.id`, `data.body`, and `data.template_version`

#### Scenario: Legacy v0 missing field returns 400
- **WHEN** `to` is missing from the legacy request
- **THEN** HTTP 400 `{"to": ["Missing data for required field."]}`

#### Scenario: Invalid notification type path rejected
- **WHEN** POST /notifications/letter is called
- **THEN** HTTP 400 `"letter notification type is not supported, please use the latest version of the client"`

#### Scenario: Unknown notification type path rejected
- **WHEN** POST /notifications/apple is called
- **THEN** HTTP 400 `"apple notification type is not supported"`

---

### Requirement: Phone number validation and normalisation
`phone_number` SHALL be validated as a string. It SHALL be parsed as an international phone number using libphonenumber (default region: CA). The E.164 normalised form SHALL be stored in `normalised_to`; the original encrypted input in `to`.

#### Scenario: Non-string phone_number rejected
- **WHEN** `phone_number` is the integer `16132532222`
- **THEN** HTTP 400 ValidationError `"phone_number 16132532222 is not of type string"`

#### Scenario: Invalid format rejected
- **WHEN** `phone_number` is `"not-a-number"`
- **THEN** HTTP 400 ValidationError `"phone_number Not a valid international number"`

#### Scenario: Local number normalised to E.164 with CA country code
- **WHEN** `phone_number` is `"6132532222"` (no country code)
- **THEN** `normalised_to` stores `"+16132532222"` and `to` stores the original encrypted input

#### Scenario: Full E.164 number stored verbatim and normalised
- **WHEN** `phone_number` is `"+14165551234"`
- **THEN** `normalised_to` stores `"+14165551234"` and `to` stores the encrypted original

---

### Requirement: International SMS permission check
If the resolved phone number has a country code other than +1, the service SHALL have the `international_sms` permission.

#### Scenario: International number without permission rejected
- **WHEN** `phone_number` is a UK number (`+447911123456`) and service lacks `international_sms` permission
- **THEN** HTTP 400 BadRequestError `"Cannot send to international mobile numbers"`

#### Scenario: International number with permission accepted
- **WHEN** `phone_number` is a UK number and service has `international_sms` permission
- **THEN** HTTP 201 is returned

---

### Requirement: SMS character limit validation
The rendered SMS body (after personalisation substitution) SHALL be ≤ 612 characters. When `service.prefix_sms = true`, the prefix `"{service_name}: "` is prepended before the character count check.

#### Scenario: SMS body at limit accepted
- **WHEN** the rendered body is exactly 612 characters
- **THEN** HTTP 201 is returned

#### Scenario: SMS body over limit rejected
- **WHEN** the rendered body is 613 characters
- **THEN** HTTP 400 BadRequestError `"Content has a character count greater than the limit of 612"`

#### Scenario: SMS prefix adds to character count
- **WHEN** `service.prefix_sms = true` and the rendered body plus prefix exceeds 612 characters
- **THEN** HTTP 400 BadRequestError `"Content has a character count greater than the limit of 612"`

---

### Requirement: Template validation (SMS)
The template SHALL exist for the service, SHALL NOT be archived, and its type SHALL be `sms`. Required personalisation placeholders SHALL be present.

#### Scenario: SMS template type mismatch rejected
- **WHEN** `template_id` references an email template
- **THEN** HTTP 400 BadRequestError `"sms template is not suitable for email notification"` (or matching variant)

#### Scenario: Archived template rejected
- **WHEN** `template_id` references an archived template
- **THEN** HTTP 400 BadRequestError `"Template {id} has been deleted"`

#### Scenario: Missing personalisation placeholder rejected
- **WHEN** the template declares `((name))` and `personalisation` has no `name` key
- **THEN** HTTP 400 `{"template": ["Missing personalisation: name"]}`

---

### Requirement: SMS sender override
If `sms_sender_id` is provided it SHALL reference a non-archived `service_sms_senders` record for the service. The `reply_to_text` value is stored at creation and does not change if the sender is later modified.

#### Scenario: Archived sms_sender_id rejected
- **WHEN** `sms_sender_id` references an archived sender record
- **THEN** HTTP 400 BadRequestError `"sms_sender_id {id} does not exist in database for service id {id}"`

#### Scenario: No sender id uses service default
- **WHEN** `sms_sender_id` is not provided
- **THEN** the service's default SMS sender `reply_to_text` is stored on the notification

---

### Requirement: Simulated SMS numbers
Requests to the three simulated SMS numbers (`6132532222`, `+16132532222`, `+16132532223`) SHALL return HTTP 201 without persisting a notification row, without publishing to any queue, and without calling limit-check functions.

#### Scenario: Simulated number returns 201 without DB write
- **WHEN** POST /v2/notifications/sms is sent to `+16132532222`
- **THEN** HTTP 201 is returned and no row exists in the `notifications` table

#### Scenario: All three simulated numbers bypass processing
- **WHEN** POST is sent to `6132532222`, `+16132532222`, or `+16132532223`
- **THEN** HTTP 201 is returned; no limit functions called; no queue publish

---

### Requirement: Trial mode and team-key restrictions
`normal` key on restricted service: can only reach team members or safelisted recipients. `team` key: can only reach team members or safelisted recipients regardless of service mode.

#### Scenario: Normal key on restricted service rejects non-team recipient
- **WHEN** the service is restricted and the phone number is not a team member or safelisted
- **THEN** HTTP 400 BadRequestError `"Can't send to this recipient when service is in trial mode – see <docs>"`

#### Scenario: Team key rejects non-member regardless of service mode
- **WHEN** the API key is a `team` key and the recipient is not a team member or safelisted
- **THEN** HTTP 400 BadRequestError `"Can't send to this recipient using a team-only API key (service {id}) - see <docs>"`

#### Scenario: Test key bypasses all restrictions
- **WHEN** the API key is a `test` key
- **THEN** no restriction checks are performed and the notification is created

---

### Requirement: SMS permission check
The authenticated service SHALL have the `sms` permission.

#### Scenario: Service without SMS permission rejected
- **WHEN** the service does not have the `sms` permission
- **THEN** HTTP 400 BadRequestError `"Service is not allowed to send text messages"`

---

### Requirement: Suspended service blocked
If the service `active = false`, the request SHALL be rejected with HTTP 403.

#### Scenario: Suspended service returns 403
- **WHEN** the authenticated service has `active = false`
- **THEN** HTTP 403 is returned before any other processing

---

### Requirement: Billable units calculation and limit checks
When `FF_USE_BILLABLE_UNITS` is enabled, `billable_units` SHALL be set to `FragmentCount(rendered_body)` and daily/annual SMS limit checks SHALL be called with that value. When disabled, `billable_units = 0` and limit checks SHALL use `1`.

#### Scenario: Multi-fragment message consumes multiple daily limit units
- **WHEN** `FF_USE_BILLABLE_UNITS` is enabled and the rendered body produces 3 fragments
- **THEN** the daily SMS limit counter is decremented by 3

#### Scenario: Feature flag off uses 1 unit regardless of length
- **WHEN** `FF_USE_BILLABLE_UNITS` is disabled and the message has 4 fragments
- **THEN** the daily SMS limit counter is decremented by 1

#### Scenario: Test key stores billable_units but skips limit checks
- **WHEN** `FF_USE_BILLABLE_UNITS` is enabled and a `test` key is used
- **THEN** `billable_units` is calculated and stored on the notification but no limit function is called

---

### Requirement: SMS rate and limit enforcement
SMS send requests SHALL enforce, in order: (1) per-minute API rate limit; (2) annual SMS limit; (3) daily SMS limit. Any exceeded limit returns HTTP 429. Test-key sends skip all limit checks.

#### Scenario: Annual limit exceeded returns 429
- **WHEN** the service has exhausted its annual SMS allocation
- **THEN** HTTP 429 is returned and the notification is not persisted

#### Scenario: Daily limit exceeded returns 429
- **WHEN** the service has exhausted its daily SMS limit
- **THEN** HTTP 429 is returned and the notification is not persisted

#### Scenario: Simulated numbers skip all limit checks
- **WHEN** POST is sent to a simulated number and the service is at its daily limit
- **THEN** HTTP 201 is returned (limit check is never called)
