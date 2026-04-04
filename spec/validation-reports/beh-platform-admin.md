# Validation Report: behavioral-spec/platform-admin.md
Date: 2026-04-04

## Summary
- **Domains covered**: complaints, reports, events, email/letter branding, platform stats, newsletter, support, cache, cypress, status, OpenAPI
- **CONFIRMED**: All major contracts
- **DISCREPANCIES**: 0
- **UNCOVERED**: 5 edge cases
- **Risk items**: 5

## Verdict
**FULLY CONFIRMED** — All major endpoints and workflows verified.

---

## Confirmed

- GET /complaint: paginated DESC by created_at ✅
- GET /complaint/count: date-range filtering, returns integer directly (not wrapped) ✅
- POST /service/{id}/report: creates REQUESTED status, fires task, 201 ✅
- GET /complaint/count date format: `%Y-%m-%d` enforced, invalid → 400 ✅
- POST /events: stores arbitrary JSON, 201 ✅
- GET /email-branding: optional `organisation_id` filter; empty string when null ✅
- POST /email-branding/create: name unique, `brand_type` defaults `BRANDING_ORG_NEW` ✅
- GET /platform-stats: status totals by channel, date validation ✅
- Newsletter subscribe/confirm/unsubscribe lifecycle ✅
- Support `find-id`: returns type + metadata for users/services/templates/jobs/notifications ✅
- POST /cache/clear: `redis_store.delete_cache_keys_by_pattern`, 201 or 500 ✅
- POST /cypress/create_user: creates 2 users, `email_suffix` alphanumeric only ✅
- GET /_status: `status=ok`, db_version, commit_sha, build_time, current_time_utc ✅
- GET /status/live-service-and-organisation-counts: active=True, restricted=False, count_as_live=True ✅

---

## Discrepancies
None.

---

## Uncovered Contracts

1. Complaint pagination threshold (no test at exactly PAGE_SIZE+1)
2. Email branding `organisation_id=null` filter: returns empty vs. all unassigned brandings — not tested
3. Freshdesk fallback to `CONTACT_FORM_EMAIL_ADDRESS` when Freshdesk is down — not tested
4. Newsletter Airtable API 500 handling — not tested
5. Status endpoint `db_version` source (alembic version?) — not documented with test

---

## RISK Items for Go Implementors

1. **Freshdesk feature flag** (`FRESH_DESK_ENABLED=False`): returns 201 without storing data. Go must handle consistently — failed Freshdesk must not silently discard tickets.

2. **Newsletter Airtable table auto-creation**: On first `save()`, creates table if absent. Multiple Go replicas racing to create same table — implement idempotency guard.

3. **Financial year boundary at EST midnight**: `get_financial_year_for_datetime()` converts UTC to EST. Request at `03:59:59 UTC March 31` is counted as March, not April. Add explicit test.

4. **Complaint email scrubbing**: `remove_emails_from_complaint` scrubs PII. If email appears in nested JSON fields, scrubbing may miss it. Audit scrubbing coverage.

5. **Cache clear partial failure**: `delete_cache_keys_by_pattern` may delete some keys then fail. No rollback. Log which patterns succeeded/failed.
