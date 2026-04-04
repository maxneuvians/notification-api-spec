# Validation Report: business-rules/templates.md
Date: 2026-04-04

## Summary
- **DAO Functions Checked**: 22 (11 templates_dao + 6 template_categories_dao + 5 template_folder_dao)
- **Functions Present**: 22/22
- **Discrepancies**: 5 (2 critical, 2 moderate, 1 spec-documented anomaly)
- **Missing from spec**: 3 (folder movement, emptiness validation, permission model for folders)
- **Risk items**: 5 (3 critical/moderate)

## Verdict
**CONDITIONAL PASS** — All 22 DAO functions exist with correct signatures. However, 2 critical runtime bugs must be fixed before Go rewrite: `process_type` hybrid property crashes on null category, and template history queries may raise `MultipleResultsFound` after a process_type update.

---

## Confirmed (Brief List)

- All 11 template DAO functions: `dao_create_template`, `dao_update_template`, `dao_update_template_reply_to`, `dao_update_template_process_type`, `dao_update_template_category`, `dao_redact_template`, `dao_get_template_by_id_and_service_id`, `dao_get_template_by_id`, `dao_get_all_templates_for_service`, `dao_get_template_versions`, `get_precompiled_letter_template` ✅
- All 6 category DAO functions ✅
- All 5 folder DAO functions ✅
- Template versioning via `@version_class` decorator with automatic version increment ✅
- `TemplateRedacted` one-to-one relationship created at template creation ✅
- Pre-compiled letter template singleton with `hidden=True` ✅
- Archive and hidden flags respected in `dao_get_all_templates_for_service` ✅
- Redis caching for `dao_get_template_by_id` ✅
- Cascade delete with default category reassignment ✅

---

## Discrepancies

### 1. `process_type` Hybrid Property Crashes on Null Category — CRITICAL

**Location**: `app/models.py`, `process_type` hybrid property

```python
@hybrid_property
def process_type(self):
    if self.template_type == SMS_TYPE:
        return self.process_type_column if self.process_type_column else self.template_category.sms_process_type
    elif self.template_type == EMAIL_TYPE:
        return self.process_type_column if self.process_type_column else self.template_category.email_process_type
```

**Problem**: `template_category_id` is nullable (spec confirms). If `template_category` is `None`, accessing `.sms_process_type` raises `AttributeError`.

**Impact**: Any template with `process_type_column=None` and `template_category_id=NULL` will crash on serialization, REST responses, and history construction.

**Fix**:
```python
return self.process_type_column if self.process_type_column else (
    self.template_category.sms_process_type if self.template_category else None
)
```

---

### 2. Template History Version Collision — CRITICAL

**Location**: `app/dao/templates_dao.py`, `dao_update_template_process_type`

**Problem**: This function writes a history row WITHOUT incrementing `version`. If a template at version N has process_type updated, two history rows share `(id, version N)`. Querying `dao_get_template_by_id(id, version=N)` with `.one()` raises `MultipleResultsFound`.

**Note**: Spec documents this as a known anomaly. Risk is that Go implementors may not carry this quirk forward, causing incorrect behavior.

**Fix options**: Either increment version in this function, or change `.one()` to `.first()` in version queries and document the constraint.

---

### 3. Template History Missing `template_category_id` — MODERATE

**Locations**: `dao_update_template_reply_to`, `dao_update_template_process_type`, `dao_update_template_category`

**Problem**: Manually-constructed `TemplateHistory` dictionaries omit `template_category_id`. `TemplateBase` inherits this field, so history rows store NULL even when a category is assigned.

**Impact**: Cannot reconstruct category assignment at any historical version. Incomplete audit trail.

**Fix**: Add `"template_category_id": template.template_category_id` to all three history construction dicts.

---

### 4. Cascade Delete Has Double-Commit — MODERATE

**Location**: `app/dao/template_categories_dao.py`, `dao_delete_template_category_by_id`

**Problem**: Function is wrapped with `@transactional` but contains explicit `db.session.commit()` calls inside the reassignment loop. If reassignment succeeds but deletion fails, the earlier commits cannot be rolled back.

**Fix**: Remove explicit `db.session.commit()` calls; rely on `@transactional` to commit once at end.

---

### 5. `dao_update_template_process_type` Version Anomaly (spec-documented)
**Spec explicitly notes** this function does not increment version. ✅ Confirmed as-designed. Go implementors must replicate this behavior.

---

## Missing from Spec

1. **Template folder movement / content reassignment** (`move_to_template_folder` in REST layer) — allows reassigning templates and subfolders to different parents; no DAO spec entry
2. **Folder emptiness validation before delete** — REST layer enforces non-empty folders cannot be deleted; not documented in business rules
3. **User permission model for folder access** — `user_folder_permissions` junction table enforces per-user folder access; not documented in spec

---

## RISK Items for Go Implementors

### 🔴 CRITICAL
**1. Fix `process_type` null-safety before Go rewrite**
- Any template may lack a category; Go code reading process_type must handle nil category pointer

**2. Fix template history version collision**
- Go version of `dao_update_template_process_type` must explicitly NOT increment version to preserve behavior
- OR change `.one()` to `.first()` in all version-based queries

### 🟡 MODERATE
**3. Populate `template_category_id` in manual history rows**
- Audit trail incomplete without it; fix in Python before Go spec is finalized

**4. Remove double-commits in category cascade delete**
- Partial commit risk; fix before Go implements transaction boundaries

### ℹ️ INFORMATIONAL
**5. Stale relationship access in history construction**
- After `db.session.execute(UPDATE)`, lazy-loaded relationships are stale
- Recommend `db.session.refresh(template)` after any UPDATE in history construction paths
