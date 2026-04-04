# Validation Report: behavioral-spec/misc-gaps.md
Date: 2026-04-04

## Summary
- **Domains covered**: API key analytics/revocation, reports DAO, performance platform, SMS fragment utils, daily sorted letters
- **CONFIRMED**: All core contracts
- **DISCREPANCIES**: 0
- **UNCOVERED**: 5 edge cases
- **Risk items**: 5

## Verdict
**FULLY CONFIRMED** — All gap-fill contracts validated. No critical uncovered scenarios.

---

## Confirmed

- GET /api_key/stats/{id}: returns counts (0 if no sends) ✅
- GET /api_key/ranked?n_days_back=N: sorted by total DESC, includes api_key_name, service_name ✅
- POST /sre_tools/revoke_api_keys: format `gcntfy-keyname-{service_id}-{unsigned_secret}`; 201 on success, 200 if unresolvable (not error), 400 missing field, 401 no auth ✅
- `save_model_api_key()`: version=1, `last_used_timestamp=None`, creates 1 history record ✅
- `expire_api_key()`: sets `expiry_date`, creates history record (version becomes 2) ✅
- `update_last_used_api_key()`: writes timestamp, does NOT create history record ✅
- `get_model_api_keys(service_id, id=None)`: returns active OR expired ≤7 days; excludes >7-day-old expired ✅
- `get_api_key_by_secret()`: parses `gcntfy-keyname` prefix, validates service_id UUID ✅
- `resign_api_keys(resign, unsafe)`: preview mode read-only; `resign=True` re-signs; `unsafe=True` forces without verify ✅
- `create_report()`: status=REQUESTED ✅
- `get_reports_for_service(service_id, limit_days)`: ≥ (now - limit_days), DESC, SMS+EMAIL ✅
- `send_total_notifications_sent_for_day_stats()`: `_id` = base64(`{date}govuk-notify{channel}notificationsday`) ✅
- `fetch_todays_requested_sms_count()`: Redis-or-DB, 7200s TTL ✅
- `dao_create_or_update_daily_sorted_letter()`: INSERT then upsert (updates `unsorted_count`, `sorted_count`, `updated_at`) ✅

---

## Discrepancies
None.

---

## Uncovered Contracts

1. **Revocation notification email**: "send_api_key_revocation_email exactly once" — no test verifies content or recipient
2. **`FF_USE_BILLABLE_UNITS=False` path**: billable_units DAO calls skipped entirely — not tested
3. **Performance platform DST transition**: UTC-to-EST conversion near spring/fall clock change — no test
4. **API key signing rotation**: `resign_api_keys unsafe=True` — no test verifies what happens with incompatible old-format key
5. **Reports retention edge case**: report created at 23:59:59 on day-7 vs 00:00:00 — boundary not explicitly tested

---

## RISK Items for Go Implementors

1. **API key signing scheme compatibility**: If Go uses different HMAC algorithm, verification fails on migrated keys. Use reference Python signing implementation for cross-validation tests.

2. **Redis cache TTL hard-coded at 7200s**: Make TTL configurable in app config to allow ops tuning without code change.

3. **Performance platform `_id` generation**: String concat order is `"{date}govuk-notify{channel}notificationsday"`. Different Go concatenation breaks idempotency key.

4. **Report retention off-by-one**: `limit_days=7` boundary includes/excludes exactly-7-day-old reports depending on sub-second timing. Add explicit boundary test.

5. **Daily sorted letter upsert atomicity**: Two replicas inserting same `billing_day` simultaneously = race condition. Use `INSERT ... ON CONFLICT (billing_day) DO UPDATE` with explicit constraint name.
