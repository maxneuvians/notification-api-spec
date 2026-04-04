## ADDED Requirements

### Requirement: GET /v2/notifications/{notification_id} — v2 single notification
The endpoint SHALL return HTTP 200 with the full notification object for a notification belonging to the authenticated service. The response SHALL include: `id`, `reference`, `email_address`, `phone_number`, `type`, `status`, `status_description`, `provider_response`, `template: {id, version, uri}`, `created_at`, `created_by_name`, `body`, `subject`, `sent_at`, `completed_at`, `scheduled_for`, `postage`. For email notifications, `subject` SHALL be populated and `phone_number` SHALL be null.

#### Scenario: Returns notification for authenticated service
- **WHEN** GET /v2/notifications/{id} is called with a notification ID belonging to the authenticated service
- **THEN** HTTP 200 is returned with all notification fields populated including rendered `body` and `subject`

#### Scenario: Email notification has subject, null phone_number
- **WHEN** the notification is an email notification
- **THEN** `subject` is populated and `phone_number` is null in the response

#### Scenario: Notification of another service returns 404
- **WHEN** the notification exists in the DB but belongs to a different service
- **THEN** HTTP 404 is returned

#### Scenario: Non-existent notification returns 404
- **WHEN** the notification ID does not exist in the `notifications` table
- **THEN** HTTP 404 `{"message": "Notification not found in database", "result": "error"}`

#### Scenario: Invalid UUID in path returns 400
- **WHEN** the notification_id path parameter is `"not-a-uuid"`
- **THEN** HTTP 400 ValidationError `{"errors": [{"error": "ValidationError", "message": "notification_id is not a valid UUID"}], "status_code": 400}`

---

### Requirement: GET /v2/notifications — v2 notification list
The endpoint SHALL return HTTP 200 with `{"notifications": [...], "links": {"current": "...", "next": "..."}}` for the authenticated service. The list SHALL exclude job-created notifications by default and SHALL be ordered newest-first. Each key type (normal/team/test) SHALL only see notifications created by that same key type.

#### Scenario: Returns notifications for authenticated service
- **WHEN** GET /v2/notifications is called with no query params
- **THEN** HTTP 200 is returned with an array of notification objects (excluding job notifications, newest-first)

#### Scenario: List filtered by status
- **WHEN** `status=delivered` query param is provided
- **THEN** only notifications with `status = "delivered"` are returned

#### Scenario: List filtered by template_type
- **WHEN** `template_type=email` query param is provided
- **THEN** only email notifications are returned

#### Scenario: Paginated response includes next cursor
- **WHEN** there are more notifications than the page size
- **THEN** the response includes a `links.next` URL with an `older_than` cursor

#### Scenario: older_than cursor paginates correctly
- **WHEN** `older_than=<uuid>` of the last notification on page 1 is provided
- **THEN** only notifications created before that notification are returned

#### Scenario: older_than with non-existent UUID returns empty list
- **WHEN** `older_than` is a valid UUID that does not match any notification
- **THEN** HTTP 200 with empty `notifications` array

#### Scenario: status=failed expands to three statuses
- **WHEN** `status=failed` is provided
- **THEN** notifications with status `technical-failure`, `temporary-failure`, or `permanent-failure` are all returned

#### Scenario: Multiple filter params combine with AND semantics
- **WHEN** both `template_type=email` and `status=delivered` are provided
- **THEN** only email notifications with `delivered` status are returned

#### Scenario: Invalid status value returns 400
- **WHEN** `status=unknown-status` is provided
- **THEN** HTTP 400 `"status unknown-status is not one of [...]"`

#### Scenario: Invalid template_type value returns 400
- **WHEN** `template_type=fax` is provided
- **THEN** HTTP 400 `"template_type fax is not one of [sms, email, letter]"`

#### Scenario: Invalid older_than UUID returns 400
- **WHEN** `older_than=not-a-uuid` is provided
- **THEN** HTTP 400 `"older_than is not a valid UUID"`

#### Scenario: Normal key only sees normal-key notifications
- **WHEN** a normal-key request calls GET /v2/notifications
- **THEN** only notifications created with `key_type = 'normal'` are returned (not test-key notifications)

#### Scenario: include_jobs returns job-created notifications
- **WHEN** `include_jobs=true` is provided with a normal key
- **THEN** job-created notifications are included in the results

---

### Requirement: GET /notifications/{id} — legacy v0 single notification
The legacy endpoint SHALL return HTTP 200 with `{"data": {"notification": {id, status, template, to, service, body, subject?, content_char_count}}}`. For email, `subject` SHALL be present and `content_char_count` SHALL be null. Template body is rendered using stored personalisation at the template version that was active at creation time.

#### Scenario: Legacy v0 returns notification with correct schema
- **WHEN** GET /notifications/{id} is called for a valid notification
- **THEN** HTTP 200 with `data.notification` containing `id`, `status`, `body`, `subject` (email only), `content_char_count` (null for email)

#### Scenario: Email notification has subject in legacy response
- **WHEN** the notification is an email
- **THEN** `data.notification.subject` is present and populated

#### Scenario: Email content_char_count is null in legacy response
- **WHEN** the notification is an email
- **THEN** `data.notification.content_char_count` is null

#### Scenario: Not found returns 404 in legacy format
- **WHEN** the notification ID does not exist
- **THEN** HTTP 404 `"Notification not found in database"`

#### Scenario: Malformed UUID in legacy path returns 405
- **WHEN** the notification_id path parameter is not a valid UUID
- **THEN** HTTP 405

---

### Requirement: GET /notifications — legacy v0 notification list
The legacy endpoint SHALL return HTTP 200 with `{"notifications": [...], "total": N, "page_size": N, "links": {last?, prev?, next?}}`. By default it excludes job-created and test-key notifications.

#### Scenario: Legacy list returns correct shape
- **WHEN** GET /notifications is called
- **THEN** HTTP 200 with `notifications` array, `total`, `page_size`, and `links`

#### Scenario: Legacy list filtered by template_type
- **WHEN** `template_type=sms` is provided
- **THEN** only SMS notifications are returned

#### Scenario: Legacy list paginates with page and page_size
- **WHEN** `page=2&page_size=5` is provided
- **THEN** the second page of 5 notifications is returned

#### Scenario: Invalid page returns 400
- **WHEN** `page=abc` is provided
- **THEN** HTTP 400 `"Not a valid integer."`

#### Scenario: Normal key with include_jobs=true includes job notifications
- **WHEN** `include_jobs=true` is provided with a normal key
- **THEN** job-created notifications are included, returning one per both API and job origins

#### Scenario: Test key with include_jobs=true still shows only test-key notifications
- **WHEN** `include_jobs=true` is provided with a test key
- **THEN** only test-key notifications are returned (include_jobs has no effect for test keys)
