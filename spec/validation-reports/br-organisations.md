# Validation Report: business-rules/organisations.md
Date: 2026-04-04

## Summary
- **Spec DAO functions**: 19 across 2 files
- **Code functions**: All 19 present
- **Confirmed**: 19/19
- **Discrepancies**: 0 (function name typo is intentional)
- **Missing from spec**: 0
- **Risk items**: 2

## Verdict
**PASS** — All org CRUD, service linkage, domain management, and invitation DAOs implemented as specified.

---

## Confirmed

**organisation_dao.py (13/13)**:
- `dao_get_organisations` (ordered active DESC, name ASC) ✅
- `dao_count_organsations_with_live_services` (typo intentional, see Discrepancies) ✅
- `dao_get_organisation_services` ✅
- `dao_get_organisation_by_id` ✅
- `dao_get_organisation_by_email_address` (longest-domain-first matching, `.gsi.gov.uk` → `.gov.uk` rewrite) ✅
- `dao_get_organisation_by_service_id` ✅
- `dao_create_organisation` ✅
- `dao_update_organisation` (scalar updates, email branding refresh, org type cascade to services) ✅
- `_update_org_type_for_organisation_services` (via `@version_class`) ✅
- `dao_add_service_to_organisation` (cascades type + crown) ✅
- `dao_get_invited_organisation_user` ✅
- `dao_get_users_for_organisation` (active only, ordered by created_at) ✅
- `dao_add_user_to_organisation` (many-to-many append) ✅

**invited_org_user_dao.py (6/6)**: All invitation CRUD + 2-day cleanup ✅

**Domain rules**: Organisation types (9 values), crown status, domain uniqueness (global PK), atomic domain replacement, email branding relationship, MOU metadata, invitation 2-day expiry — all ✅

---

## Discrepancies
None. The function name `dao_count_organsations_with_live_services` contains a typo ("organsations") that is embedded throughout the codebase. This is consistent between spec and code.

---

## Missing from Spec
None.

---

## RISK Items for Go Implementors

1. **Function name typo is stable** — `dao_count_organsations_with_live_services` must be referenced with the typo for any callers that compare function strings or generate code. Decide: either replicate the typo in Go or accept a deliberate fork from the Python naming.

2. **Domain longest-match collision** — Domains have a globally unique PK (`domain.domain`). Two orgs cannot register overlapping entries, but Go schema migrations must enforce this constraint. Test that subdomain matching (`example.gov.uk` matching `gov.uk`) does not cause a false positive for a different org.
