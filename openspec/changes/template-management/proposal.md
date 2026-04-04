## Why

Templates are the reusable message definitions that every notification references. This change implements the full template CRUD, versioning, folder management, category management, and admin endpoints, plus the v2 GET template endpoints. Includes fixes C4 and C5 from the validation reports.

## What Changes

- `internal/handler/` template family: POST/GET/PUT for service templates, template versions, template statistics, template redaction, template folders, template categories
- `internal/handler/v2/templates/` — GET /v2/template/{id}, GET /v2/templates
- `internal/service/templates/` — create/update/archive with history writes, `process_type` resolution from category, redaction, folder moves
- **C4 fix**: `process_type` hybrid property null-guard — when `template_category_id` is set but `template_category` is nil, return `nil` rather than panicking
- **C5 fix**: `dao_update_template_process_type` increments `version` before writing a history row, preventing `MultipleResultsFound` on version-based lookups

## Capabilities

### New Capabilities

- `template-management`: Template CRUD with versioning, folder organisation, category assignment, personalisation redaction, and v2 read endpoints

### Modified Capabilities

## Non-goals

- Template rendering at send time (covered in `send-email-notifications` and `send-sms-notifications`)
- Letter templates beyond stub behaviour (covered in `letter-stub-endpoints`)

## Impact

- Requires `authentication-middleware`, `data-model-migrations` (templates, template_folders, template_categories repositories)
- `send-email-notifications` and `send-sms-notifications` depend on the template lookup implemented here
