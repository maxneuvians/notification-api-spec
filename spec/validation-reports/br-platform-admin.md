# Validation Report: business-rules/platform-admin.md
Date: 2026-04-04

## Summary
- **DAO functions validated**: 24/24 present and match spec
- **Confirmed**: 24/24
- **Discrepancies**: 0
- **Missing from spec**: 4 (event ordering, complaint webhook ingestion, report lifecycle, cypress cleanup)
- **Risk items**: 6 (mostly informational)

## Verdict
**PASS** — All specified business rules correctly implemented. Minor code quality issues noted (redundant commits, missing transaction wrapper) but no functional defects.

---

## Confirmed

- `save_complaint` / `fetch_*_complaints` (paginated, by-service, by-count with UTC timezone conversion) ✅
- `create_report` / `get_reports_for_service` / `update_report`: Retention window filtering; double-commit pattern (safe) ✅
- `dao_create_event`: Direct `db.session.add()` + `commit()` without `@transactional` — by design for audit trail ✅
- `dao_get_email_branding_options`: Optional org_id filter ✅
- `dao_update_email_branding` / `dao_update_letter_branding`: Falsy-to-None coercion via `value or None` ✅
- `dao_create_or_update_daily_sorted_letter`: PostgreSQL `INSERT … ON CONFLICT DO UPDATE` on `(billing_day, file_name)` ✅
- `@transactional` decorator: commit on success, rollback on exception ✅
- Cache clear: `redis_store.delete_cache_keys_by_pattern()` over `CACHE_KEYS_ALL` patterns ✅

---

## Discrepancies
None.

---

## Missing from Spec

1. **Event ordering**: No specification of whether `created_at` is used as a total-order key for audit purposes
2. **Complaint webhook ingestion**: SES complaint callback path and verification logic not documented
3. **Reports lifecycle**: No TTL/archival/deletion policy specified for old report records
4. **Cypress test data cleanup**: Spec references `app/cypress/` helpers but does not enumerate what test data is created or how it is cleaned up post-test

---

## RISK Items for Go Implementors

### 🔴 CRITICAL
**1. Events DAO has no transaction wrapper**
- `dao_create_event` does NOT use `@transactional`; if `db.session.commit()` fails, event is silently lost
- Recommend: wrap in `@transactional` or add explicit try/except with rollback

### 🟠 HIGH
**2. Complaint count timezone risk**
- `fetch_count_of_complaints` converts dates via `get_local_timezone_midnight_in_utc`
- If local timezone ≠ UTC, count range shifts by a day
- Recommend: accept UTC dates only or explicitly document timezone assumption

### 🟡 MODERATE
**3. Reports double-commit**
- `create_report` calls `db.session.commit()` inside body AND `@transactional` commits again on exit
- Harmless but wasteful; remove explicit commit to rely on decorator

**4. Email branding name uniqueness not enforced**
- `dao_get_email_branding_by_name` uses `.first()` — multiple records with same name return arbitrary result
- Recommend: add unique DB constraint or document non-unique lookup behavior

**5. Missing ON DELETE cascade verification**
- If service/org/user is deleted, related complaints/reports/branding may become orphaned
- Verify all FK relationships have appropriate cascade delete semantics

### ℹ️ INFORMATIONAL
**6. Inbound SMS dashboard always 7-day window**
- Summary endpoint hardcodes 7-day window regardless of per-service retention setting
- Document or align with retention config
