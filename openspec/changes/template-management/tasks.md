## 1. Template Service Layer (CRUD + C4/C5 fixes)

- [ ] 1.1 Implement `internal/service/templates/process_type.go` — `effectiveProcessType(template) *string`: return `process_type_column` if non-null; else return category `sms_process_type` or `email_process_type` by template type; return nil when category is nil and column is nil (C4 fix); cover all 4 branches in unit tests
- [ ] 1.2 Implement `internal/service/templates/create.go` — `CreateTemplate`: assign UUID, set version=1, insert `templates` row, insert `templates_history` row at version 1 (include `template_category_id`), create `template_redacted` row; wrap in single transaction; write tests for rollback on partial failure
- [ ] 1.3 Implement `internal/service/templates/update.go` — `UpdateTemplate`: apply partial mutations, increment version, insert `templates_history` row with `template_category_id` (C5 and history completeness); skip history write if content unchanged; write tests confirming unchanged content produces no new history row
- [ ] 1.4 Implement `internal/service/templates/update_process_type.go` — `UpdateTemplateProcessType`: increment `version` before writing `templates_history` row (C5 fix); include `template_category_id` in history row; unit test: version increments from 2 → 3; no duplicate version rows in history
- [ ] 1.5 Implement `internal/service/templates/redact.go` — `RedactTemplate`: update `template_redacted.redact_personalisation=true`, `updated_at`, `updated_by_id`; do NOT increment version; idempotent (second call is no-op); write tests confirming version unchanged after redact

## 2. Template Handlers (v1 internal)

- [ ] 2.1 Implement `internal/handler/services/templates.go` — `POST /service/{id}/template`, `GET /service/{id}/template`, `GET /service/{id}/template/{tid}`, `POST /service/{id}/template/{tid}` (update/archive/redact), `GET /service/{id}/template/precompiled`; validate content/name length limits; check service permission for template type; write handler tests for 400/403/404 error paths
- [ ] 2.2 Implement `internal/handler/services/template_versions.go` — `GET /service/{id}/template/{tid}/versions` (ordered version DESC), `GET /service/{id}/template/{tid}/version/{version}` (use LIMIT 1, not strict ONE, to tolerate legacy duplicate versions per D4); write test: non-existent version returns 404
- [ ] 2.3 Implement `internal/handler/services/template_preview.go` — `GET /service/{id}/template/{tid}/preview` (personalisation via query params); substitute `((placeholder))` tokens; return 400 for missing required placeholders; extra query params ignored
- [ ] 2.4 Implement `GET /service/{id}/template/{tid}/notification/{nid}/preview` — proxy to template preview service for regular letters; fetch from S3 for precompiled; handle `file_type=pdf|png`; return 400 for invalid file_type; return 500 on upstream/S3 error

## 3. v2 Template Handlers

- [ ] 3.1 Implement `internal/handler/v2/templates/get_template.go` — `GET /v2/template/{id}` with optional `?version=` param; return v2 shape: `body`, `subject`, `personalisation` dict (placeholder → `{"required": true}`) extracted from both body and subject; letter templates always include `"contact block"`; use service API key middleware (not internal auth); write tests for personalisation extraction, letter contact block, non-existent UUID 404 shape
- [ ] 3.2 Implement `internal/handler/v2/templates/list_templates.go` — `GET /v2/templates` with optional `?type=` filter; exclude hidden and archived; return `{"templates": [...]}` with `id`, `type`, `body`, `subject` (email only); validate type enum; write tests: invalid type returns 400 with v2 error shape

## 4. v2 Preview Handler

- [ ] 4.1 Implement `POST /v2/template/{id}/preview` — accept `personalisation` from JSON body; substitute body and subject; return `id`, `body`, `subject`, `html` (rendered HTML for email, null for SMS/letter); return 400 with v2 error shape for missing personalisation; write test for email html rendering, SMS html=null

## 5. Folder Management

- [ ] 5.1 Implement `internal/service/templates/folders.go` — `CreateFolder` (validate parent_id belongs to same service; assign permissions: root→all active users, child→parent's permitted users), `UpdateFolder` (replace users_with_permission entirely), `DeleteFolder` (pre-check: empty folder only; cascade delete user_folder_permissions rows), `MoveContentsToFolder`; write tests for wrong-service parent_id rejection, cascade-delete of permissions
- [ ] 5.2 Implement folder REST handlers at `GET/POST /service/{id}/template-folder`, `POST/DELETE /service/{id}/template-folder/{fid}`, `POST template_folder/move_to_template_folder`; write tests: non-empty folder delete returns 400; move to null target sets parent=null; archived templates in move payload silently skipped; loop detection returns 400

## 6. Template Category Management

- [ ] 6.1 Implement `internal/service/templates/categories.go` — `CreateCategory` (assign UUID, insert, default `sms_sending_vehicle="long_code"`), `UpdateCategory` (single commit — do NOT double-commit per D7 in design), `GetCategoryByID`, `GetCategoryByTemplateID`, `ListCategories` (filter by template_type and/or hidden), `DeleteCategory`; write tests: cascade=true reassigns to default category by process_type mapping; cascade=false with templates returns 400
- [ ] 6.2 Implement category REST handlers at `POST/GET /template-category`, `POST/GET /template-category/{id}`, `GET /template-categories`, `DELETE /template-category/{id}?cascade=`; write tests: duplicate name_en returns 400; invalid template_type filter returns 400; cascade delete with no templates returns 204

## 7. Template Statistics

- [ ] 7.1 Implement `GET template_statistics/service/{id}/day` (`whole_days` or `limit_days` param 1–7): query notifications grouped by template for the window; return `template_id`, `count`, `template_name`, `template_type`, `status`, `is_precompiled_letter`; emit `billable_units` when `FF_USE_BILLABLE_UNITS=true`; write tests: `whole_days < 1` or `> 7` returns 400; float/non-integer returns 400
- [ ] 7.2 Implement `GET template_statistics/template/{id}`: return most recent notification for the template; return `{"data": null}` when none in live table; return 404 for non-existent template_id

## 8. Integration & Contract Tests

- [ ] 8.1 Write integration tests: `CreateTemplate` → history row at version 1 with category_id; `UpdateTemplate` same content → no new history row; `UpdateTemplateProcessType` → version incremented, no duplicate history versions (C5); `RedactTemplate` → version unchanged
- [ ] 8.2 Write e2e test: create template, update twice, fetch `/versions` → 3 history rows newest-first; fetch `/version/1` → original content; fetch `/version/99` → 404
