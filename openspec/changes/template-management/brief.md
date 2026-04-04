# Brief: template-management

## Source Files Analysed

- `spec/behavioral-spec/templates.md`
- `spec/business-rules/templates.md`
- `openspec/changes/template-management/proposal.md`

---

## Endpoints Covered

### Template CRUD (v1 internal)
| Method | Path | Status |
|--------|------|--------|
| POST | `/service/{service_id}/template` | Create template |
| GET | `/service/{service_id}/template` | List templates for service |
| GET | `/service/{service_id}/template/{template_id}` | Fetch single template |
| POST | `/service/{service_id}/template/{template_id}` | Update template (partial) |
| GET | `/service/{service_id}/template/precompiled` | Fetch pre-compiled PDF template |

### Template Versioning & History
| Method | Path |
|--------|------|
| GET | `/service/{service_id}/template/{template_id}/versions` |
| GET | `/service/{service_id}/template/{template_id}/version/{version}` |

### Template Previews
| Method | Path |
|--------|------|
| GET | `/service/{service_id}/template/{template_id}/preview` |
| GET | `/service/{service_id}/template/{template_id}/notification/{notification_id}/preview` |
| POST | `/v2/template/{template_id}/preview` |

### v2 Read Endpoints
| Method | Path |
|--------|------|
| GET | `/v2/template/{template_id}` |
| GET | `/v2/templates` |

### Template Folders
| Method | Path |
|--------|------|
| GET | `/service/{service_id}/template-folder` |
| POST | `/service/{service_id}/template-folder` |
| POST | `/service/{service_id}/template-folder/{folder_id}` |
| DELETE | `/service/{service_id}/template-folder/{folder_id}` |
| POST | `template_folder.move_to_template_folder` |

### Template Categories
| Method | Path |
|--------|------|
| POST | `/template-category` |
| POST | `/template-category/{template_category_id}` |
| GET | `/template-category/{template_category_id}` |
| GET | `/template-category` (by template id) |
| GET | `/template-categories` |
| DELETE | `/template-category/{template_category_id}` |
| POST | `/template.update_templates_category` |

### Statistics
| Method | Path |
|--------|------|
| GET | `template_statistics.get_template_statistics_for_service_by_day` |
| GET | `template_statistics.get_template_statistics_for_template_id` |

---

## Key Data Model Facts

- `templates` table: primary state; `templates_history` table: every snapshot.
- `templates_history` has composite PK `(id, version)`.
- `template_redacted`: one-to-one with every template; `redact_personalisation = false` by default.
- `template_folder` self-references via `parent_id`; permissions junction: `user_folder_permissions`.
- `template_categories`: bilingual (`name_en`, `name_fr`), separate `sms_process_type` / `email_process_type`, `sms_sending_vehicle`, `hidden` flag.
- `process_type` is a hybrid derived property:
  1. `process_type_column` if set.
  2. Otherwise `template_category.sms_process_type` (SMS) or `email_process_type` (email/letter).
  3. **C4**: if category is nil and column is nil → return nil (do not panic).
- Valid `process_type` values: `bulk`, `normal`, `priority`.
- Valid `template_type` values: `sms`, `email`, `letter`.

---

## Business Logic Invariants

### Versioning
- `dao_create_template`: sets `version = 1`, writes `TemplateHistory` row at version 1.
- `dao_update_template`: increments version, writes `TemplateHistory` row.
- `dao_update_template_reply_to`: manually increments version, writes history.
- `dao_update_template_category`: manually increments version, writes history.
- **C5 fix**: `dao_update_template_process_type` MUST increment `version` before writing history. The Python implementation does not — this causes duplicate version rows and `MultipleResultsFound` errors on version lookups.
- Redaction (`redact_personalisation = true`) does NOT increment version.
- Archival alone does NOT increment version.
- Unchanged content submission does NOT create a new history row.

### Process Type (C4 fix)
- If `template_category_id` is NULL and `process_type_column` is NULL → return nil, fallback to `"normal"` for callers.
- Letter templates: return `process_type_column` directly (no category resolution).

### Category Cascade Delete
- `cascade=False` (default): raises 400 if templates reference the category.
- `cascade=True`: reassigns templates to default category based on deleted category's process type:
  - `bulk` → `DEFAULT_TEMPLATE_CATEGORY_LOW`
  - `normal` → `DEFAULT_TEMPLATE_CATEGORY_MEDIUM`
  - `priority` → `DEFAULT_TEMPLATE_CATEGORY_HIGH`

### Folder Permissions
- Root-level folder: all active (non-pending) service users get permission.
- Child folder: only inherits parent folder's permitted users.
- `users_with_permission` update fully replaces the list.
- Folder delete cascades to `user_folder_permissions` junction rows.
- Cannot delete folder with sub-folders or templates: 400 `"Folder is not empty"`.

### Move-to-folder
- Archived templates in the payload are silently skipped.
- Moving does not increment template versions.
- `target_template_folder_id = null` moves to top level.
- Cannot move folder to a descendant or itself.

### v2 Endpoints
- `GET /v2/template/{id}`: returns `personalisation` dict where each key = placeholder name, value = `{"required": true}`. Letter templates always include `"contact block"`.
- `POST /v2/template/{id}/preview`: returns `html` (rendered HTML) for email; null for SMS/letter.
- `GET /v2/templates`: filters by `type`; excludes hidden and archived.

### Pre-compiled Letter Template
- Singleton per service: `hidden=True`, `name="Pre-compiled PDF"`, `postage="second"`.
- Created on first access if absent.

### Redaction
- One-way: no unredact operation.
- Second redact request is a no-op (idempotent).

---

## Known Bugs Fixed (from validation reports)

| ID | Location | Description |
|----|----------|-------------|
| C4 | `process_type` hybrid property | Nil-pointer panic when `template_category_id` is set but object is nil; must guard |
| C5 | `dao_update_template_process_type` | Missing version increment before history row; produces duplicate version rows |

---

## Auth Requirements
- Internal endpoints: `create_authorization_header()` (internal JWT).
- v2 endpoints: service-scoped API key authorization.

---

## Validation Rules Summary
- `created_by` required on create: 400.
- `subject` required for email/letter: 400.
- Template name ≤ `TEMPLATE_NAME_CHAR_COUNT_LIMIT` chars.
- SMS content ≤ `SMS_CHAR_COUNT_LIMIT` chars; email ≤ `EMAIL_CHAR_COUNT_LIMIT` chars.
- `parent_folder_id` must belong to same service: 400 `"parent_folder_id not found"`.
- Service must have permission for template type: 403.
- `postage` constrained to `"first"` or `"second"` for letter templates only.
- Category `name_en` and `name_fr` must be globally unique.

---

## Dependency Notes
- Requires: `authentication-middleware`, `data-model-migrations`.
- Downstream consumers: `send-email-notifications`, `send-sms-notifications` (template lookup).
- Non-goals: letter rendering (letter-stub-endpoints), send-time rendering (send-* changes).
