# Validation Report: behavioral-spec/templates.md
Date: 2026-04-04

## Summary
- **Contracts in spec**: ~32
- **Contracts with test coverage**: ~30
- **CONFIRMED**: 30
- **DISCREPANCIES**: 0
- **UNCOVERED**: 2 edge cases
- **EXTRA BEHAVIORS in tests**: 1 (implementation detail)
- **Risk items**: 2

## Verdict
**CONFIRMED** — No discrepancies. All major contracts covered (creation, update, versioning, redaction, categories, folders, v2 endpoints). Two low-risk uncovered edge cases.

---

## Confirmed (concise)

- POST /service/{id}/template: 201, version=1, process_type from category, TemplateHistory v1 + TemplateRedacted created ✅
- POST /service/{id}/template/{id}: 200, version increments on change, redaction → no version increment ✅
- GET /service/{id}/template, /{id}, /preview, /versions, /version/{n}: all response shapes ✅
- GET /service/{id}/template/precompiled: lazy creation, hidden=True, singleton ✅
- Template category CRUD: name uniqueness (en + fr separately), cascade delete reassigns to DEFAULT_TEMPLATE_CATEGORY_MEDIUM ✅
- Template folder CRUD: root → all active users permissions, child → parent's permissions, 400 on non-empty delete ✅
- POST /template_folder.move_to_template_folder: moves do NOT increment version, null target → top-level ✅
- GET /v2/template/{id}: optional version param, personalisation dict with `{field: {required: true}}` ✅
- POST /v2/template/{id}/preview: body+subject substituted, html for email, missing placeholder → 400 ✅
- GET /v2/templates: optional type filter, excludes hidden/archived ✅
- Versioning rules: create → v1, content change → version++, redaction → no change, archival alone → no change, reply-to change → version++, category change → version++ ✅
- process_type resolution: column if set, else category default (sms_process_type/email_process_type) ✅
- Redaction: one-way, idempotent (second True→True is no-op, updated_at NOT changed again) ✅
- Folder permissions: root = all active users, update replaces list entirely ✅

---

## Discrepancies
None found.

---

## Uncovered Contracts

1. **Archival + other field change in same request**: spec says archival alone does not increment version, but if name also changes in the same `{archived: true, name: "new"}` request, it is unclear whether version increments — no explicit test
2. **POST /template-category/{id} with non-existent category ID**: no explicit 404 test for updating a category that doesn't exist

---

## RISK Items for Go Implementors

### 🔴
**1. Redaction is truly idempotent (second call must NOT update `updated_at`)**
- First `redact_personalisation: true` → TemplateRedacted.updated_at set to now
- Second call → complete no-op (updated_at NOT changed)
- Go implementation must check current state before writing; many ORM update patterns will write unconditionally

### 🟡
**2. Precompiled template lazy creation has no name-collision guard**
- GET lazy-creates a template named "Pre-compiled PDF" if none exists
- If a user manually creates a template with this exact name first, behavior is unspecified
- Verify whether "Pre-compiled PDF" should be a reserved name; tests assume no collision
