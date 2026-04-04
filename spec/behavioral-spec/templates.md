# Behavioral Spec: Templates

## Processed Files

- [x] tests/app/template/test_rest.py
- [x] tests/app/template/test_rest_history.py
- [x] tests/app/template/test_template_category_rest.py
- [x] tests/app/template_folder/test_template_folder_rest.py
- [x] tests/app/template_statistics/test_rest.py
- [x] tests/app/dao/test_templates_dao.py
- [x] tests/app/dao/test_template_categories_dao.py
- [x] tests/app/dao/test_template_folder_dao.py
- [x] tests/app/v2/template/test_get_template.py
- [x] tests/app/v2/template/test_post_template.py
- [x] tests/app/v2/template/test_template_schemas.py
- [x] tests/app/v2/templates/test_get_templates.py
- [x] tests/app/v2/templates/test_templates_schemas.py

---

## Endpoint Contracts

### POST /service/{service_id}/template

**Happy path**
- Returns `201` with full template object.
- Response must include: `id`, `name`, `template_type`, `content`, `service`, `version` (= 1), `process_type` (= `"normal"` if no explicit override), `created_by`, `template_category_id`.
- `subject` is present for `email` and `letter` types; absent (null) for `sms`.
- `postage` is present only for `letter` type (defaults to `"second"`); null for `sms` and `email`.
- Serialised response must exactly match `template_schema.dump(template)`.

**Validation rules**
- `created_by` is required; omitting it returns `400` with `"created_by is a required property"`.
- `subject` is required for `email` and `letter` types; omitting it returns `400` with `"subject is a required property"`.
- Template name must not exceed `TEMPLATE_NAME_CHAR_COUNT_LIMIT` characters; violation returns `400` with `"Template name must be less than {limit} characters"`.
- Template content must not exceed `SMS_CHAR_COUNT_LIMIT` (SMS) or `EMAIL_CHAR_COUNT_LIMIT` (email) characters; violation returns `400` with `"Content has a character count greater than the limit of {limit}"`.
- `parent_folder_id`, if supplied, must belong to the same service; violation returns `400` with `"parent_folder_id not found"`.
- A non-existent `parent_folder_id` UUID returns `400` with `"parent_folder_id not found"`.

**Error cases**
- Non-existent `service_id`: `404` `"No result found"`.
- Service lacks permission for the requested `template_type`: `403` with messages such as `"Creating text message templates is not allowed"` / `"Creating email templates is not allowed"` / `"Creating letter templates is not allowed"`.

**Auth requirements**
- Requires an internal authorization header (`create_authorization_header()`).

**Notable edge cases**
- `parent_folder_id` creates a folder relationship; the `Template.folder` FK is set.
- `template_category_id` sets the category; `process_type` defaults to the category's process type when the `process_type_column` is `null`.
- When both `template_category_id` and `process_type` are supplied, the explicit `process_type` is stored in `process_type_column` and returned as-is; `process_type` reported in response equals the explicit override.
- When only `template_category_id` is supplied (no `process_type`), the returned `process_type` equals the category's default process type. `process_type_column` is `null`.
- `text_direction_rtl` defaults to `False` when omitted; can be set to `True` or `False` explicitly.
- Creation also creates a `TemplateRedacted` row (with `redact_personalisation = false`) and a `TemplateHistory` row at version 1.

---

### POST /service/{service_id}/template/{template_id}

**Happy path**
- Returns `200` with updated template object; `version` increments by 1.
- Partial updates are supported (only supplied fields are changed).

**Validation rules**
- Template name length and content length limits apply identically to creation.
- `postage` may be updated only for `letter` templates; valid values are `"first"` and `"second"`.

**Error cases**
- Non-existent `service_id` or `template_id`: `404` `"No result found"`.
- Service lacks permission for the template's type: `403` with messages such as `"Updating text message templates is not allowed"`.
- Setting `reply_to` to a `letter_contact_id` that belongs to a different service: `400` with `"letter_contact_id {id} does not exist in database for service id {service_id}"`.
- Setting `redact_personalisation: true` without `created_by`: `400` `{"created_by": ["Field is required"]}`.

**Auth requirements**
- Requires internal authorization header.

**Redaction behaviour**
- When `redact_personalisation: true` is submitted: records `updated_at` and `updated_by_id` on `template_redacted`; ignores all other payload fields (e.g. `name` is not updated); does not increment `version`.
- If already redacted, a subsequent `redact_personalisation: true` call is a no-op (updated_at is not changed).
- Redaction is one-way; no rollback endpoint is tested.

**Category / process-type interaction on update**
- Changing `template_category_id` while omitting `process_type` resets `process_type_column` to `null`; the effective `process_type` becomes the new category's default.
- Supplying both `template_category_id` and an explicit `process_type` stores the override in `process_type_column`; effective `process_type` equals the explicit value regardless of the category's default.

**Archival behaviour**
- Setting `archived: true` marks the template as archived and clears its folder relationship (`Template.folder = null`).
- No new version is created from archival alone when other fields are unchanged.

**No-op on unchanged content**
- Submitting the same `template_type` and `content` as already stored does **not** create a new `TemplateHistory` row; version stays at 1.

**Reply-to (letter templates)**
- Can set, change, or clear (`null`) `reply_to`; each change creates a new `TemplateHistory` row with the updated `service_letter_contact_id`.
- `reply_to` in the API response maps to the `service_letter_contact_id` UUID.
- `reply_to_text` in the response contains the `contact_block` string of the contact.
- The raw column name `service_letter_contact_id` is never exposed in the response.

**RTL text direction**
- `text_direction_rtl: true` can be set on update; `text_direction_rtl: false` reverts it; null/omitted also result in `false` in the response.

---

### GET /service/{service_id}/template

**Happy path**
- Returns `200` with `{"data": [...]}`.
- Returns only templates belonging to the specified service.
- Each entry includes `name`, `version`, `created_at`.

**Error cases**
- Returns `200` with `{"data": []}` when service has no templates.

**Auth requirements**
- Internal authorization header required.

---

### GET /service/{service_id}/template/{template_id}

**Happy path**
- Returns `200` with full template including `content`, `subject`, `process_type`, `redact_personalisation`, `reply_to`, and `reply_to_text`.
- `service_letter_contact_id` is not present in the response.

**Error cases**
- Non-existent `template_id`: `404` `{"result": "error", "message": "No result found"}`.

**Auth requirements**
- Internal authorization header required.

---

### GET /service/{service_id}/template/{template_id}/preview

**Happy path**
- Returns `200` with `content` and `subject` fields with personalisation placeholders substituted.
- Personalisation values are passed as query parameters (e.g. `?name=Alice&thing=document`).
- Extra query parameters that are not placeholders are silently ignored.

**Error cases**
- Missing required personalisation placeholders: `400` with `{"template": ["Missing personalisation: {names}"]}`.
- Template without any placeholders renders without query params.

**Auth requirements**
- Internal authorization header required.

---

### GET /service/{service_id}/template/precompiled

**Happy path**
- Returns `200` with the service's pre-compiled PDF template object.
- Fields: `name = "Pre-compiled PDF"`, `hidden = true`.
- If no precompiled template exists, one is created on-demand (lazy creation).
- If one already exists (any hidden letter template), it is returned without creating a duplicate.

**Auth requirements**
- Internal authorization header required.

---

### GET /service/{service_id}/template/{template_id}/versions

**Happy path**
- Returns `200` with `{"data": [...]}` where each entry represents a historical version.
- Three versions exist after two content updates; each version has the correct `content` for that version.

**Auth requirements**
- Internal authorization header required.

---

### GET /service/{service_id}/template/{template_id}/version/{version}

**Happy path**
- Returns `200` with version data: `id`, `content`, `version`, `process_type`, `created_by` (as nested object with `name`), `created_at`.
- Fetching an old version returns the content as it was at that version, not the current content.

**Error cases**
- Non-existent `version` number: `404`.

**Auth requirements**
- Internal authorization header required.

---

### GET /service/{service_id}/template/{template_id}/notification/{notification_id}/preview (letter preview by notification id)

**Happy path**
- Accepts `file_type` query param: `pdf` or `png`; returns `200` with `{"content": "<base64>"}`.
- For regular letter templates: proxies request to the template preview service; forwards template id, personalisation values (address fields), date, and filename.
- For pre-compiled letter templates: fetches the PDF from S3 via `get_letter_pdf`; for `png` requests, calls the extract-page and preview overlay endpoints.
- PNG preview for page 1 includes `hide_notify=true` query param; pages > 1 do not.

**Error cases**
- `file_type` other than `pdf` or `png`: `400` `["file_type must be pdf or png"]`.
- Template preview service returns non-2xx: `500` with descriptive message including notification id.
- S3 returns a `botocore.ClientError` (e.g. 403): `500`.
- PDF read error (`PdfReadError`) during page extraction: `500`.

**Auth requirements**
- Internal authorization header required.

---

### POST /template-category

**Happy path**
- Returns `201` with `{"template_category": {...}}`.
- Response includes all fields: `name_en`, `name_fr`, `description_en`, `description_fr`, `sms_process_type`, `email_process_type`, `hidden`, `created_by_id`.

**Error cases**
- Duplicate `name_en` or duplicate `name_fr` (even if only one is duplicated): `400` `"Template category already exists, name_en and name_fr must be unique."`.

**Auth requirements**
- Internal authorization header required.

---

### POST /template-category/{template_category_id}

**Happy path**
- Updates mutable fields on the category.

**Error cases**
- Updating to a `name_en` or `name_fr` that already exists on another category: `400` `"Template category already exists, name_en and name_fr must be unique."`.

---

### GET /template-category/{template_category_id}

**Happy path**
- Returns `200` `{"template_category": {...}}` with all persisted fields.

---

### GET /template-category (by template id variant)

**Happy path**
- Returns `200` `{"template_category": {...}}` for the category associated with the specified template.

---

### GET /template-categories

**Happy path**
- Returns `200` with all categories.
- Filterable by `template_type` (`sms` or `email`) and `hidden` (`True` / `False`).
- Non-boolean `hidden` value (e.g. `"not_a_boolean"`) returns `200` (tolerant parsing).

**Error cases**
- Invalid `template_type` (not `sms` or `email`): `400` `"Invalid filter 'template_type', valid template_types: 'sms', 'email'"`.

**Auth requirements**
- Internal authorization header required.

---

### DELETE /template-category/{template_category_id}

**Happy path**
- `cascade=True`: returns `204`; deletes the category and reassigns all associated templates to the default medium-priority category (`DEFAULT_TEMPLATE_CATEGORY_MEDIUM`).
- No associated templates: returns `204` unconditionally.

**Error cases**
- `cascade=False` (default) with associated templates: `400` `"Cannot delete categories associated with templates. Dissociate the category from templates first."`.

---

### POST /template.update_templates_category (update single template's category)

**Happy path**
- Returns `200`; sets `template_category_id` on the template.

---

### GET /service/{service_id}/template-folder (get folders)

**Happy path**
- Returns `200` `{"template_folders": [...]}`.
- Each folder entry: `id`, `name`, `service_id`, `parent_id` (null if root), `users_with_permission` (list of user ids).
- Returns only folders for the requested service.

**Auth requirements**
- Internal authorization header required.

---

### POST /service/{service_id}/template-folder (create folder)

**Happy path**
- Returns `201` `{"data": {...}}`.
- Fields: `name`, `service_id`, `parent_id`.
- If `parent_id` is supplied: inherits permissions only from those users who already have access to the parent folder (i.e. a subset of service users).
- If `parent_id` is null: all active (non-pending) service users receive permission.

**Error cases**
- Missing `name` or missing `parent_id` from the request body: `400` `"ValidationError: {field} is a required property"`.
- `parent_id` UUID not found: `400` `{"result": "error", "message": "parent_id not found"}`.
- `parent_id` belongs to a different service: `400` `{"result": "error", "message": "parent_id not found"}`.

---

### POST /service/{service_id}/template-folder/{folder_id} (update folder)

**Happy path**
- Can update `name` and `users_with_permission`.
- `users_with_permission` replaces the existing list entirely (not additive).

**Error cases**
- Missing `name`: `400` `"name is a required property"`.
- `name` is `null`: `400` `"name None is not of type string"`.
- `name` is empty string: `400` `"name  is too short"`.

---

### DELETE /service/{service_id}/template-folder/{folder_id}

**Happy path**
- Returns `204`; removes the folder.

**Error cases**
- Folder contains sub-folders: `400` `{"result": "error", "message": "Folder is not empty"}`.
- Folder contains templates: `400` `{"result": "error", "message": "Folder is not empty"}`.

---

### POST template_folder.move_to_template_folder

**Happy path**
- Accepts `{"templates": [...], "folders": [...]}` and `target_template_folder_id`.
- Folders in the payload become children of the target folder.
- Templates in the payload move directly into the target folder (not into a moved folder even if both are in the same request).
- `target_template_folder_id = null` moves items to the top level (no parent).
- Moving does **not** increment template versions.
- Archived templates in the payload are silently skipped (not moved).

**Validation rules**
- `templates` key is required; if missing the request returns `400`.
- `folders` key is required; if missing returns `400`.
- Folder ids must be valid UUIDs; passing `null` or a non-UUID value returns `400`.

**Error cases**
- Folder from a different service: `400` `"No folder found with id {id} for service {service_id}"`.
- Template from a different service: `400` `"Could not move to folder: No template found with id {id} for service {service_id}"`.
- Moving a folder to one of its own descendant folders (loop): `400` `"You cannot move a folder to one of its subfolders"`.
- Moving a folder to itself: `400` `"You cannot move a folder to itself"`.

---

### GET template_statistics.get_template_statistics_for_service_by_day

**Happy path**
- Returns `200` `{"data": [...]}` with per-template counts for the requested window.
- Each entry: `template_id`, `count`, `template_name`, `template_type`, `status`, `is_precompiled_letter`.
- With feature flag `FF_USE_BILLABLE_UNITS = True`: also includes `billable_units`.

**Validation rules**
- `whole_days` (or legacy `limit_days`) must be a positive integer in the range 1–7.
- Returns `400` for: missing parameter, value < 0 or > 7, float, or non-integer string.

**Error cases**
- No templates for service: returns `200` with `{"data": []}`.

**Auth requirements**
- Internal authorization header required.

---

### GET template_statistics.get_template_statistics_for_template_id

**Happy path**
- Returns `200` `{"data": {...}}` with the most recent notification object for the template.
- Returns `{"data": null}` when no current notifications exist.
- Old notifications stored only in `notification_history` (not in live table) return `{"data": null}`.

**Error cases**
- Non-existent `template_id`: `404` `{"result": "error", "message": "No result found"}`.

---

### GET /v2/template/{template_id}

**Happy path**
- Returns `200` with: `id`, `type`, `created_at` (ISO 8601), `updated_at` (null if never updated), `version`, `created_by` (email address string), `body`, `subject` (null for SMS), `name`, `personalisation` (dict), `postage` (null for non-letter).
- `personalisation` is a dict keyed by placeholder name; each value is `{"required": true}`.
- Placeholders are extracted from both `body` and `subject`.
- For `letter` type: `"contact block"` is always included as a required personalisation entry.

**Validation rules**
- Version param is optional; passing no version returns the current version. Both behave identically for v1 templates.

**Error cases**
- Non-existent `template_id`: `404` `{"errors": [{"error": "NoResultFound", "message": "No result found"}], "status_code": 404}`.
- Non-existent version number: same `404` shape.

**Auth requirements**
- Service-scoped API key authorization header.

---

### POST /v2/template/{template_id}/preview

**Happy path**
- Returns `200` with: `id`, `body` (personalisation substituted), `subject` (substituted, null for SMS), `html` (rendered HTML for email, null otherwise).
- Personalisation values passed in the JSON body under `"personalisation"` key.
- Substitution applies to both `body` and `subject`.

**Error cases**
- Missing required personalisation: `400` `{"errors": [{"error": "BadRequestError", "message": "Missing personalisation: {name}"}]}`.
- Non-existent `template_id`: `404`.

**Auth requirements**
- Service-scoped API key authorization header.

---

### GET /v2/templates

**Happy path**
- Returns `200` `{"templates": [...]}` with all non-hidden, non-archived templates for the authenticated service.
- Each entry: `id`, `type`, `body`, `subject` (only present for email), plus standard metadata fields.
- Optional `?type=` query parameter filters by template type.

**Error cases**
- Invalid `type` value: `400` `{"errors": [{"error": "ValidationError", "message": "type {value} is not one of [sms, email, letter]"}], "status_code": 400}`.

**Auth requirements**
- Service-scoped API key authorization header.

---

## DAO Behavior Contracts

### dao_create_template

**Expected behavior**
- Persists a `Template` row with `version = 1`.
- Simultaneously creates a `TemplateHistory` row (same `id`, `version = 1`, `created_by_id`).
- Creates a `TemplateRedacted` row with `redact_personalisation = false` and `updated_by_id = service.created_by_id`.
- Accepts optional `redact_personalisation` kwarg; sets the `TemplateRedacted` value accordingly at creation.
- If `reply_to` (letter contact id) is provided, associates the contact.
- Default `process_type` for a newly created template (with no `process_type_column` set) is `"normal"`.

**Edge cases verified**
- `postage` values for letters are constrained to `"first"` or `"second"` at the DB level; `"third"` raises `SQLAlchemyError` on insert.
- A non-letter template with a `postage` value also raises `SQLAlchemyError`.

---

### dao_update_template

**Expected behavior**
- Applies mutations to the `Template` row.
- Creates a new `TemplateHistory` row at `version + 1`.
- Does not create new history if submitted values are unchanged relative to current state (idempotent at the API layer; DB-level behaviour may differ).

**Edge cases verified**
- Setting `process_type = None` (with a `template_category_id` set) causes the effective `process_type` to fall back to the category's `email_process_type`; `process_type_column` is stored as `null`.
- Setting `process_type = "priority"` stores `"priority"` in `process_type_column`.
- `postage = "third"` on an existing letter template raises `SQLAlchemyError` on update.

---

### dao_update_template_reply_to

**Expected behavior**
- Accepts `template_id` and `reply_to` (UUID or `None`).
- Updates `service_letter_contact_id` on the `Template` row.
- Increments `version` and sets `updated_at`.
- Creates a new `TemplateHistory` row reflecting the change.
- All three transitions tested: `None → some`, `some → different`, `some → None`.

---

### dao_redact_template

**Expected behavior**
- Sets `TemplateRedacted.redact_personalisation = True` for the template.
- Records `updated_at` (current time) and `updated_by_id` (supplied user id).
- Does **not** increment the template version.

---

### dao_get_all_templates_for_service

**Expected behavior**
- Returns only templates for the given `service_id`.
- Excludes archived templates.
- Excludes hidden templates.
- Results are alphabetically ordered by template name.
- Eager-loads `TemplateRedacted` to avoid N+1 queries during serialisation.

---

### dao_get_template_by_id

**Expected behavior**
- Fetches current `Template` row by `id`.
- Accepts optional `version` parameter; when provided, returns a `TemplateHistory` instance instead of a `Template`.
- Accepts optional `use_cache` parameter; when `True`, checks Redis for a cached serialised copy before hitting the database.

---

### dao_get_template_by_id_and_service_id

**Expected behavior**
- Fetches `Template` (or `TemplateHistory` if version supplied) matching both `template_id` and `service_id`.
- Raises `NoResultFound` for hidden templates.
- Raises `NoResultFound` for templates belonging to a different service.

---

### dao_get_template_versions

**Expected behavior**
- Returns all `TemplateHistory` rows for the given `(service_id, template_id)` pair.
- Version 1 has `updated_at = None`; later versions have `updated_at` set.
- Returns an empty list for hidden templates.

---

### dao_update_template_category

**Expected behavior**
- Updates `template_category_id` on the `Template` row.
- Sets `updated_at`.
- Increments version.
- The new `TemplateHistory` row does **not** carry `template_category_id` (history captures previous state, not the updated one).

---

### dao_create_template_category

**Expected behavior**
- Persists a `TemplateCategory` row.
- `sms_sending_vehicle` defaults to `"long_code"` when not provided.
- `sms_sending_vehicle` stores explicitly provided values (e.g. `"short_code"`).

---

### dao_update_template_category

**Expected behavior**
- Updates all provided mutable fields (`name_en`, `name_fr`, `description_en`, `description_fr`, `sms_process_type`, `email_process_type`, `hidden`).
- Persists `updated_by_id`.

---

### dao_get_template_category_by_id

**Expected behavior**
- Returns the `TemplateCategory` row matching the given UUID.

---

### dao_get_template_category_by_template_id

**Expected behavior**
- Returns the `TemplateCategory` associated with the given template.

---

### dao_get_all_template_categories

**Expected behavior**
- Returns all categories when no filters supplied.
- Filters by `template_type` (`"sms"` or `"email"`): each category is returned when it has associated templates of that type.
- Filters by `hidden` (`True` / `False`): restricts to matching categories.
- Filters can be combined (e.g. `template_type="sms"` + `hidden=False`).

---

### dao_delete_template_category_by_id

**Expected behavior**
- Deletes the category when no templates reference it.
- Raises `InvalidRequest` when templates are associated and `cascade=False` (default).
- With `cascade=True`: deletes the category and reassigns all previously associated templates to `DEFAULT_TEMPLATE_CATEGORY_MEDIUM`; the three generic default categories survive.

---

### dao_delete_template_folder

**Expected behavior**
- Deletes the `TemplateFolder` row.
- Cascades deletion to associated `user_folder_permissions` rows (junction table).

---

### dao_update_template_folder

**Expected behavior**
- Updates mutable fields on `TemplateFolder` (e.g. `users` list, `name`).

---

## Business Rules Verified

### Template versioning behavior
- Every `dao_create_template` call produces `Template` version 1 plus a matching `TemplateHistory` row.
- Every `dao_update_template` call producing a change increments the version and appends a `TemplateHistory` row.
- Submitting unchanged content via the API does **not** create a new version.
- Redaction (`redact_personalisation`) does **not** increment the version.
- Archival alone does **not** increment the version.
- Reply-to changes (setting, changing, or clearing) do increment the version and produce a new history row.
- Category changes via `dao_update_template_category` increment the version.

### Process type resolution
- `process_type` is a computed property: returned value is `process_type_column` when set; otherwise falls back to the template category's process type (`email_process_type` for email/letter, `sms_process_type` for SMS).
- Changing the template category while omitting `process_type` in an update **resets** `process_type_column` to `null`; effective `process_type` becomes the new category's default.
- Supplying an explicit `process_type` alongside a category change stores the override and returns it as the effective process type.

### Category behavior
- Categories carry separate EN/FR names and descriptions, distinct SMS and email process types, a hidden flag, and an `sms_sending_vehicle` (`"long_code"` default, `"short_code"` optional).
- `name_en` and `name_fr` must each be globally unique; duplicate on either field prevents create or update.
- Deletion is blocked when templates reference the category unless `cascade=True`.
- Cascade deletion reassigns affected templates to the system default medium-priority category.

### Folder permissions
- Folders carry a `users_with_permission` list of service-user UUIDs.
- When creating a root-level folder (no parent), all active (non-pending) service users are granted permission.
- When creating a child folder, only the parent folder's permitted users inherit permission.
- Updating `users_with_permission` fully replaces the existing list.
- Folder deletion cascades to the `user_folder_permissions` junction table.

### Redaction behavior
- Redaction is triggered by submitting `redact_personalisation: true` on a template update.
- When redacted, only `updated_at` and `updated_by_id` on `TemplateRedacted` are mutated; all other template fields (including `name`, `content`, `version`) are unchanged.
- An already-redacted template is idempotent: a second redaction request is a no-op.
- `should_template_be_redacted(org)` returns `True` only when the organisation type is `"province_or_territory"`.

### v2 preview endpoint: personalisation substitution rules
- The `GET /v2/template/{id}` response includes a `personalisation` dict. Each key is a placeholder name from the template body or subject; each value is `{"required": true}`.
- Letter templates always add `"contact block"` as a required personalisation entry regardless of template content.
- `POST /v2/template/{id}/preview` performs substitution: `((placeholder))` tokens in body and subject are replaced with values from the `personalisation` request object.
- If any required placeholder is missing from the preview request, a `400 BadRequestError` is returned naming the missing keys.
- Extra keys in the preview request `personalisation` are silently ignored.
- The preview response includes `html` (rendered HTML) for email templates; `html` is `null` for SMS and letter types.
