# Validation Report: business-rules/billing.md
Date: 2026-04-04

## Summary
- **Spec DAO functions**: 18 across 3 files
- **Code functions**: All 18 confirmed present
- **Confirmed**: 18/18
- **Discrepancies**: 0
- **Missing from spec**: 0 core functions
- **Extra in code**: 8 performance/analytics helpers (non-core, acceptable)
- **Risk items**: 3

## Verdict
**PASS** — All annual billing, limits, and fact-billing DAOs implemented as specified. Nightly aggregation tasks present and correct.

---

## Confirmed

**annual_billing_dao.py (5/5)**: `dao_create_or_update_annual_billing_for_year` (upsert), `dao_get_annual_billing`, `dao_update_annual_billing_for_future_years`, `dao_get_free_sms_fragment_limit_for_year`, `dao_get_all_free_sms_fragment_limit` ✅

**annual_limits_data_dao.py (4/4)**: `get_previous_quarter`, `get_all_quarters`, `insert_quarter_data` (INSERT ON CONFLICT), `fetch_quarter_cummulative_stats` ✅

**fact_billing_dao.py (9/9)**: `fetch_sms_free_allowance_remainder`, `fetch_billing_totals_for_year`, `fetch_monthly_billing_for_year`, `delete_billing_data_for_service_for_day`, `fetch_billing_data_for_day`, `get_rate`, `update_fact_billing` (INSERT ON CONFLICT), `create_billing_record`, `_query_for_billing_data` ✅

**Celery (reporting_tasks.py)**: `create_nightly_billing`, `create_nightly_notification_status`, `insert_quarter_data_for_annual_limits` ✅

---

## Discrepancies
None.

---

## Missing from Spec
None of the core functions. Additional analytics/admin query helpers in code are not part of the billing business rules domain.

---

## RISK Items for Go Implementors

1. **Rate lookup complexity**: `get_rate()` uses nested conditionals for letter postage class, SMS vehicle type, and crown status. Verify all combinations during Go implementation to avoid incorrect rate application.

2. **Financial year boundary**: Annual billing uses April 1 start date (fiscal year). Code uses `get_current_financial_year_start_year()`. Ensure Go date logic correctly handles April 1 transitions and timezone offsets.

3. **SMS vehicle type inference**: `sms_sending_vehicle` is computed via a SQL CASE expression in `_query_for_billing_data`. Verify Go SQL dialect handles this correctly before migration.
