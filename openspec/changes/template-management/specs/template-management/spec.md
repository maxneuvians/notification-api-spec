## Requirements

### Requirement: Create template
`POST /service/{service_id}/template` SHALL create a template with version 1, a matching `templates_history` snapshot, and a `template_redacted` record, returning 201 with the full template object.

#### Scenario: Successful creation returns 201 with version 1
- **GIVEN** a service that has permission to create email templates
- **WHEN** `POST /service/{id}/template` is called with valid `name`, `template_type=email`, `content`, `subject`, `created_by`
- **THEN** HTTP 201; response includes `id`, `version: 1`, `process_type`, `created_by`, `template_category_id`; a `templates_history` row at version 1 exists; a `template_redacted` row with `redact_personalisation=false` exists

#### Scenario: process_type defaults to category default when not overridden
- **GIVEN** a create request that supplies only `template_category_id` (no explicit `process_type`)
- **WHEN** `POST /service/{id}/template` is called
- **THEN** returned `process_type` equals the category's `email_process_type` (or `sms_process_type` for SMS); `process_type_column` is stored as null

#### Scenario: Explicit process_type overrides category default
- **GIVEN** a create request with both `template_category_id` and `process_type: "priority"`
- **WHEN** `POST /service/{id}/template` is called
- **THEN** returned `process_type = "priority"`; `process_type_column` is stored as `"priority"`

#### Scenario: Missing created_by returns 400
- **WHEN** `POST /service/{id}/template` body omits `created_by`
- **THEN** HTTP 400, body contains `"created_by is a required property"`

#### Scenario: Missing subject for email returns 400
- **WHEN** `POST /service/{id}/template` creates an email template without `subject`
- **THEN** HTTP 400, body contains `"subject is a required property"`

#### Scenario: Service lacks permission for template type returns 403
- **WHEN** `POST /service/{id}/template` requests `template_type=sms` and the service does not have SMS permission
- **THEN** HTTP 403, body contains `"Creating text message templates is not allowed"`

#### Scenario: parent_folder_id from another service returns 400
- **WHEN** `POST /service/{id}/template` supplies a `parent_folder_id` that belongs to a different service
- **THEN** HTTP 400, body contains `"parent_folder_id not found"`

---

### Requirement: List templates for service
`GET /service/{service_id}/template` SHALL return all active, visible templates for the service ordered by name, excluding archived and hidden templates.

#### Scenario: Returns service templates only
- **GIVEN** two services each with templates
- **WHEN** `GET /service/{service_id}/template` is called for service A
- **THEN** HTTP 200, `{"data": [...]}` contains only service A's templates

#### Scenario: Empty service returns empty list
- **WHEN** `GET /service/{service_id}/template` is called for a service with no templates
- **THEN** HTTP 200, `{"data": []}`

#### Scenario: Archived and hidden templates excluded
- **GIVEN** a service with one active template, one archived template, and one hidden template
- **WHEN** `GET /service/{service_id}/template` is called
- **THEN** only the active template is returned

---

### Requirement: Fetch single template
`GET /service/{service_id}/template/{template_id}` SHALL return the current version of the template with full fields including `process_type`, `redact_personalisation`, `reply_to`, and `reply_to_text`.

#### Scenario: Returns full template object
- **WHEN** `GET /service/{id}/template/{tid}` is called for an existing template
- **THEN** HTTP 200; response includes `content`, `subject`, `process_type`, `redact_personalisation`, `reply_to_text`; `service_letter_contact_id` is absent from the response

#### Scenario: Non-existent template returns 404
- **WHEN** `GET /service/{id}/template/{tid}` is called with a UUID that does not exist
- **THEN** HTTP 404, `{"result": "error", "message": "No result found"}`

---

### Requirement: Update template
`POST /service/{service_id}/template/{template_id}` SHALL apply partial updates, increment version, and write a `templates_history` snapshot for any changed content.

#### Scenario: Successful update increments version
- **GIVEN** a template at version 1
- **WHEN** `POST /service/{id}/template/{tid}` supplies a new `name`
- **THEN** HTTP 200; `version = 2`; a new `templates_history` row exists at version 2

#### Scenario: Unchanged content does not create new history row
- **WHEN** `POST /service/{id}/template/{tid}` submits identical `content` and `template_type`
- **THEN** HTTP 200; version stays at 1; no new `templates_history` row

#### Scenario: Category change without explicit process_type resets process_type_column
- **GIVEN** a template with `process_type_column = "priority"` and category A
- **WHEN** `POST /service/{id}/template/{tid}` sets `template_category_id` to category B and omits `process_type`
- **THEN** `process_type_column` becomes null; effective `process_type` equals category B's default

#### Scenario: Archive clears folder relationship
- **WHEN** `POST /service/{id}/template/{tid}` sets `archived: true`
- **THEN** template is marked archived; `folder` FK is null; no new service-permission error

#### Scenario: Redaction-only update does not increment version
- **WHEN** `POST /service/{id}/template/{tid}` sets `redact_personalisation: true`
- **THEN** HTTP 200; `version` unchanged; only `template_redacted.updated_at` and `updated_by_id` changed; second redaction call is a no-op

---

### Requirement: Template versioning — version 1 on create, increment on every mutation (C5)
Every mutating operation that changes template content MUST increment `version` and write a `templates_history` row in the same transaction. `dao_update_template_process_type` MUST increment version, fixing the Python C5 bug.

#### Scenario: Three successive updates produce versions 1, 2, 3
- **GIVEN** a newly created template at version 1
- **WHEN** content is updated twice
- **THEN** `templates_history` has rows at versions 1, 2, 3; each has the content from that point in time

#### Scenario: process_type update increments version (C5 fix)
- **GIVEN** a template at version 2
- **WHEN** `dao_update_template_process_type` is called (e.g., category changes the process type)
- **THEN** template `version` becomes 3; a single `templates_history` row at version 3 exists; no duplicate version rows

#### Scenario: Reply-to change increments version
- **GIVEN** a letter template at version 1 with `reply_to = null`
- **WHEN** `reply_to` is set to a valid letter contact UUID
- **THEN** version becomes 2; `templates_history` at version 2 has the new `service_letter_contact_id`

---

### Requirement: Template version history endpoints
`GET /service/{id}/template/{tid}/versions` SHALL return all history rows ordered newest-first. `GET /service/{id}/template/{tid}/version/{version}` SHALL return the snapshot at that specific version.

#### Scenario: Versions list returns all snapshots newest-first
- **GIVEN** a template that has been updated twice
- **WHEN** `GET /service/{id}/template/{tid}/versions` is called
- **THEN** HTTP 200, `{"data": [...]}` with 3 entries (versions 3, 2, 1 in that order)

#### Scenario: Fetch specific version returns correct content
- **GIVEN** a template at version 3 with content C3
- **WHEN** `GET /service/{id}/template/{tid}/version/1` is called
- **THEN** HTTP 200; `content` equals C1 (the original), `version = 1`, `created_by` is a nested object with `name`

#### Scenario: Non-existent version returns 404
- **WHEN** `GET /service/{id}/template/{tid}/version/99` is called and version 99 does not exist
- **THEN** HTTP 404

#### Scenario: Hidden templates excluded from version history
- **GIVEN** a hidden pre-compiled letter template
- **WHEN** `GET /service/{id}/template/{tid}/versions` is called
- **THEN** HTTP 200, `{"data": []}` (or 404 depending on auth path)

---

### Requirement: Template preview endpoints
`GET /service/{id}/template/{tid}/preview` substitutes personalisation placeholders. `POST /v2/template/{id}/preview` returns rendered body and optional HTML.

#### Scenario: v1 preview substitutes query param personalisation
- **GIVEN** a template with content `"Hello ((name))"`
- **WHEN** `GET /service/{id}/template/{tid}/preview?name=Alice` is called
- **THEN** HTTP 200; `content` equals `"Hello Alice"` and `subject` placeholders also substituted

#### Scenario: Missing required placeholder returns 400
- **WHEN** `GET /service/{id}/template/{tid}/preview` is called without required personalisation keys
- **THEN** HTTP 400, `{"template": ["Missing personalisation: {names}"]}`

#### Scenario: Extra query params silently ignored
- **WHEN** `GET /service/{id}/template/{tid}/preview?name=Alice&extra=ignored` is called and only `name` is a placeholder
- **THEN** HTTP 200; no error for `extra`

#### Scenario: v2 preview returns html for email
- **GIVEN** an email template
- **WHEN** `POST /v2/template/{id}/preview` is called with personalisation values
- **THEN** HTTP 200; response includes `id`, `body` (substituted), `subject` (substituted), `html` (rendered HTML, non-null)

#### Scenario: v2 preview html is null for SMS
- **GIVEN** an SMS template
- **WHEN** `POST /v2/template/{id}/preview` is called
- **THEN** HTTP 200; `html` is null

---

### Requirement: v2 GET template endpoint
`GET /v2/template/{template_id}` SHALL return the template in v2 shape with `personalisation` dict where each placeholder maps to `{"required": true}`.

#### Scenario: Returns v2 shape with personalisation map
- **GIVEN** an email template with placeholders `((first_name))` in body and `((ref))` in subject
- **WHEN** `GET /v2/template/{id}` is called with a service API key
- **THEN** HTTP 200; `personalisation = {"first_name": {"required": true}, "ref": {"required": true}}`; `body` and `subject` present; `created_by` is the email address string (not nested object)

#### Scenario: Letter template always includes contact block personalisation
- **GIVEN** a letter template with no explicit personalisation placeholders
- **WHEN** `GET /v2/template/{id}` is called
- **THEN** `personalisation` contains `"contact block": {"required": true}`

#### Scenario: Non-existent template returns v2 error shape
- **WHEN** `GET /v2/template/{id}` is called with a non-existent UUID
- **THEN** HTTP 404, `{"errors": [{"error": "NoResultFound", "message": "No result found"}], "status_code": 404}`

#### Scenario: Optional version param works identically to current
- **WHEN** `GET /v2/template/{id}` is called with and without a `version` query param pointing to the current version
- **THEN** both responses are identical

---

### Requirement: v2 GET templates list
`GET /v2/templates` SHALL return all non-hidden, non-archived templates for the authenticated service, with optional type filter.

#### Scenario: Returns all active templates for service
- **GIVEN** a service with 3 active templates and 1 archived template
- **WHEN** `GET /v2/templates` is called with a service API key
- **THEN** HTTP 200, `{"templates": [...]}` with exactly 3 entries

#### Scenario: Type filter restricts results
- **GIVEN** a service with 2 SMS templates and 1 email template
- **WHEN** `GET /v2/templates?type=sms` is called
- **THEN** HTTP 200, only the 2 SMS templates returned

#### Scenario: Invalid type filter returns 400
- **WHEN** `GET /v2/templates?type=fax` is called
- **THEN** HTTP 400, `{"errors": [{"error": "ValidationError", "message": "type fax is not one of [sms, email, letter]"}], "status_code": 400}`

---

### Requirement: Template folders — list and create
`GET /service/{id}/template-folder` and `POST /service/{id}/template-folder` SHALL manage service-scoped folders with per-user permissions.

#### Scenario: List returns folders for service only
- **GIVEN** two services with folders
- **WHEN** `GET /service/{id}/template-folder` is called for service A
- **THEN** HTTP 200, `{"template_folders": [...]}` contains only service A's folders with `id`, `name`, `service_id`, `parent_id`, `users_with_permission`

#### Scenario: Create root folder grants permission to all active users
- **GIVEN** a service with 3 active users and 1 pending user
- **WHEN** `POST /service/{id}/template-folder` is called with `name` and `parent_id: null`
- **THEN** HTTP 201; `users_with_permission` contains the 3 active user IDs; pending user excluded

#### Scenario: Create child folder inherits only parent's permitted users
- **GIVEN** a parent folder with permissions for user A and user B (not user C)
- **WHEN** `POST /service/{id}/template-folder` is called with `parent_id` set to that folder
- **THEN** HTTP 201; new folder's `users_with_permission` contains only user A and user B

#### Scenario: Invalid parent_id returns 400
- **WHEN** `POST /service/{id}/template-folder` is called with a `parent_id` that belongs to a different service
- **THEN** HTTP 400, `{"result": "error", "message": "parent_id not found"}`

---

### Requirement: Template folders — update and delete
`POST /service/{id}/template-folder/{folder_id}` SHALL update name and permissions. `DELETE /service/{id}/template-folder/{folder_id}` SHALL delete an empty folder.

#### Scenario: Update replaces users_with_permission entirely
- **GIVEN** a folder with permissions for users A, B, C
- **WHEN** `POST /service/{id}/template-folder/{fid}` sets `users_with_permission: [A]`
- **THEN** HTTP 200; folder now only has user A in permissions

#### Scenario: Update with null name returns 400
- **WHEN** `POST /service/{id}/template-folder/{fid}` sets `name: null`
- **THEN** HTTP 400, `"name None is not of type string"`

#### Scenario: Delete empty folder returns 204
- **GIVEN** a folder with no sub-folders and no templates
- **WHEN** `DELETE /service/{id}/template-folder/{fid}` is called
- **THEN** HTTP 204; folder row and all `user_folder_permissions` rows deleted

#### Scenario: Delete non-empty folder returns 400
- **GIVEN** a folder containing at least one template
- **WHEN** `DELETE /service/{id}/template-folder/{fid}` is called
- **THEN** HTTP 400, `{"result": "error", "message": "Folder is not empty"}`

---

### Requirement: Move templates and folders to a target folder
`POST template_folder.move_to_template_folder` SHALL move templates and sub-folders to a target folder atomically, skipping archived templates without error.

#### Scenario: Templates move into target folder
- **GIVEN** two templates in folder A
- **WHEN** move is called with `{"templates": [t1, t2], "folders": [], "target_template_folder_id": B_id}`
- **THEN** both templates are in folder B; template versions are unchanged

#### Scenario: Move to root with null target
- **WHEN** move is called with `target_template_folder_id: null`
- **THEN** templates and folders moved to top level (no parent)

#### Scenario: Archived templates silently skipped
- **GIVEN** a payload containing one active and one archived template
- **WHEN** move is called
- **THEN** HTTP 200; active template moved; archived template unchanged; no error

#### Scenario: Moving folder to its own descendant returns 400
- **WHEN** move is called with a folder whose target is one of its own sub-folders
- **THEN** HTTP 400, `"You cannot move a folder to one of its subfolders"`

#### Scenario: Cross-service folder rejected
- **WHEN** a folder ID belonging to a different service is included in the move payload
- **THEN** HTTP 400, `"No folder found with id {id} for service {service_id}"`

---

### Requirement: Template category CRUD
Category endpoints SHALL allow create, read, update, cascade-delete, and reassignment of templates to default categories.

#### Scenario: Create category returns 201
- **WHEN** `POST /template-category` is called with unique `name_en`, `name_fr`, `sms_process_type`, `email_process_type`
- **THEN** HTTP 201, `{"template_category": {...}}` with all fields including `sms_sending_vehicle` defaulting to `"long_code"` if not supplied

#### Scenario: Duplicate name_en or name_fr returns 400
- **WHEN** `POST /template-category` is called with a `name_en` that already exists on another category
- **THEN** HTTP 400, `"Template category already exists, name_en and name_fr must be unique."`

#### Scenario: Get category by id returns 200
- **WHEN** `GET /template-category/{id}` is called for an existing category
- **THEN** HTTP 200, `{"template_category": {...}}` with all persisted fields

#### Scenario: List categories filtered by template_type
- **GIVEN** categories used by SMS and email templates
- **WHEN** `GET /template-categories?template_type=sms` is called
- **THEN** HTTP 200; only categories used by at least one SMS template are returned

#### Scenario: Invalid template_type filter returns 400
- **WHEN** `GET /template-categories?template_type=fax` is called
- **THEN** HTTP 400, `"Invalid filter 'template_type', valid template_types: 'sms', 'email'"`

#### Scenario: Delete category with cascade=true reassigns templates to default
- **GIVEN** a category with process_type "normal" associated with 3 templates
- **WHEN** `DELETE /template-category/{id}?cascade=True` is called
- **THEN** HTTP 204; all 3 templates reassigned to `DEFAULT_TEMPLATE_CATEGORY_MEDIUM`; category deleted

#### Scenario: Delete category with cascade=false (default) when templates exist returns 400
- **GIVEN** a category associated with at least one template
- **WHEN** `DELETE /template-category/{id}` is called without cascade
- **THEN** HTTP 400, `"Cannot delete categories associated with templates. Dissociate the category from templates first."`

---

### Requirement: process_type null guard (C4 fix)
When a template has `template_category_id = null` AND `process_type_column = null`, `effectiveProcessType` SHALL return nil rather than panic.

#### Scenario: Nil category and nil column returns nil process_type
- **GIVEN** a template with no category and no explicit `process_type_column`
- **WHEN** the template is serialised for an API response
- **THEN** `process_type` in the response defaults to `"normal"` (caller fallback); no panic; no 500 error

#### Scenario: Nil category but explicit process_type_column returns column value
- **GIVEN** a template with no category but `process_type_column = "priority"`
- **WHEN** the template is fetched
- **THEN** `process_type = "priority"` in response

#### Scenario: Category present, column absent — uses category default
- **GIVEN** an email template with category having `email_process_type = "bulk"` and `process_type_column = null`
- **WHEN** the template is fetched
- **THEN** `process_type = "bulk"`

---

### Requirement: Pre-compiled letter template lazy creation
`GET /service/{id}/template/precompiled` SHALL return the singleton hidden letter template, creating it on first access.

#### Scenario: First access creates and returns the template
- **GIVEN** a service with no pre-compiled letter template
- **WHEN** `GET /service/{id}/template/precompiled` is called
- **THEN** HTTP 200; template created with `name="Pre-compiled PDF"`, `hidden=true`, `postage="second"`; subsequent call returns the same template without creating a duplicate

#### Scenario: Existing template returned without duplication
- **GIVEN** a service that already has a hidden letter template
- **WHEN** `GET /service/{id}/template/precompiled` is called
- **THEN** HTTP 200; same template returned; no INSERT to DB

---

### Requirement: Template statistics by day
`GET template_statistics.get_template_statistics_for_service_by_day` SHALL return per-template notification counts for a given window with a `whole_days` parameter of 1–7.

#### Scenario: Returns counts with valid whole_days
- **GIVEN** a service that has sent notifications via multiple templates in the last 3 days
- **WHEN** the endpoint is called with `whole_days=3`
- **THEN** HTTP 200, `{"data": [...]}` with entries including `template_id`, `count`, `template_name`, `template_type`, `status`, `is_precompiled_letter`

#### Scenario: Invalid whole_days values return 400
- **WHEN** `whole_days=0`, `whole_days=8`, `whole_days=-1`, or `whole_days=3.5` is supplied
- **THEN** HTTP 400

#### Scenario: Service with no templates returns empty data
- **WHEN** the endpoint is called for a service with no notifications in the window
- **THEN** HTTP 200, `{"data": []}`
