## Context

Templates are the core reusable content definitions. Every notification references a template. A template belongs to a service, has a type (`sms`, `email`, `letter`), carries versioned content, an optional folder placement, and an optional category that determines the default `process_type`. Every mutating operation snapshots the current state into `templates_history` before returning. Two Python bugs—C4 (nil-pointer panic on `process_type`) and C5 (missing version increment before history write on `process_type` update)—are fixed in this Go implementation.

## Goals / Non-Goals

**Goals:** Full template CRUD, version history API, folder management, category management, template redaction, pre-compiled letter template, v2 GET endpoints, C4 and C5 bug fixes.

**Non-Goals:** Letter template rendering at send time (covered by `letter-stub-endpoints`); template content substitution at send time (covered by `send-email-notifications` / `send-sms-notifications`).

## Decisions

### D1 — process_type resolution with null guard (C4 fix)

`effectiveProcessType(template)` returns values in this priority order:
1. `process_type_column` if non-null → return it.
2. Category exists and template is SMS → return `category.sms_process_type`.
3. Category exists and template is email or letter → return `category.email_process_type`.
4. Category is nil (no category assigned) → return nil; callers default to `"normal"`.

This replaces the Python hybrid property that panics on `self.template_category.sms_process_type` when `template_category` is `None`. The null guard makes case 4 explicit and safe.

### D2 — version increment on every mutation path (C5 fix)

The Python `dao_update_template_process_type` writes a `templates_history` row without incrementing `version`, causing two rows with the same `(id, version)` composite PK — which breaks the `.one()` query used by `dao_get_template_by_id(id, version=N)`.

Go MUST increment `version` on every code path that writes a history row:
- `InsertTemplate` (version = 1)
- `UpdateTemplate` (version += 1)
- `UpdateTemplateReplyTo` (version += 1)
- `UpdateTemplateProcessType` (version += 1) ← **this is the C5 fix**
- `UpdateTemplateCategory` (version += 1)

Redaction and archival-only updates do NOT write a history row and therefore do NOT increment version.

### D3 — history row always includes template_category_id

The Python manual history row constructors for `dao_update_template_reply_to`, `dao_update_template_process_type`, and `dao_update_template_category` all omit `template_category_id`. Go must populate `template_category_id` in every `templates_history` INSERT so the category audit trail is accurate.

### D4 — version-history lookup uses LIMIT 1, not strict single-row assertion

Because legacy Python data may contain duplicate `(id, version)` rows in `templates_history` (from C5), Go must use `LIMIT 1` — not a strict single-row assertion — when querying history by `(id, version)`. This is a forward-compatibility guard, not a data-migration task.

### D5 — folder permissions at creation time

Root-level folders (no `parent_id`) → grant permission to all active (non-pending) service users.
Child folders (`parent_id` set) → inherit only users who have permission on the parent folder.

`users_with_permission` updates fully replace the existing list (not additive).

### D6 — v2 endpoints use API key auth; internal endpoints use internal auth

`GET /v2/template/{id}` and `GET /v2/templates` are authenticated with service-scoped API keys. All other template endpoints use the internal auth header. These are two distinct middleware checks with different claims.

### D7 — pre-compiled letter template is a lazy singleton

`GET /service/{id}/template/precompiled` checks for a hidden letter template with `name = "Pre-compiled PDF"` for the service. If absent, creates it within the same handler within a transaction. The hidden flag (`hidden = true`) prevents the row from appearing in standard template list or version-history queries.

## Risks / Trade-offs

- **C4 forward-only**: Callers that previously panicked on nil process_type will now receive nil and must handle it. Go handlers default nil to `"normal"` in the response serialiser.
- **C5 forward-only**: Duplicate version rows left by Python will not be cleaned up. The LIMIT 1 guard in D4 prevents runtime errors but does not repair data consistency.
- **Category double-commit (Python anomaly)**: `dao_update_template_category` in Python issues two commits. Go must issue exactly one commit per transaction; the second commit is not replicated.
- **Folder-not-empty enforcement**: The REST layer (not the DB cascade) enforces "folder is not empty" checks before calling `dao_delete_template_folder`. Go must replicate this pre-delete check exactly.
