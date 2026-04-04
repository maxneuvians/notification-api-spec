# Validation Report: behavioral-spec/billing.md
Date: 2026-04-04

## Summary
- **Contracts in spec**: ~35
- **CONFIRMED**: 28
- **DISCREPANCIES**: 0
- **UNCOVERED**: 6 (quarterly aggregation, letter rates, SMS rate_multiplier, historical cutoff)
- **Risk items**: 4

## Verdict
**CONDITIONAL PASS** — Core billing endpoints tested. Quarterly data aggregation specified but not thoroughly tested.

---

## Confirmed

- POST .../free-sms-fragment-limit: creates/updates AnnualBilling, current-year update cascades to future years ✅
- GET .../free-sms-fragment-limit: falls back to most recent prior year if record missing ✅
- GET .../ft-monthly-usage: auto-populates FactBilling delta if missing, excludes email rate=0, groups by (month, type, rate, postage) ✅
- GET .../ft-yearly-usage-summary: sorted by notification_type, includes letter_total ✅
- Missing year parameter → 400 `{"message": "No valid year provided", "result": "error"}` ✅
- `dao_create_or_update_annual_billing_for_year`: upsert, in-place replacement ✅
- `dao_update_annual_billing_for_future_years`: only rows where financial_year_start > current_year ✅
- `fetch_notification_status_for_service_by_month`: groups by (month, type, status), excludes test keys ✅
- `fetch_billing_data_for_day`: `sms_sending_vehicle` by E.164 format (+1XXXXXXXXXX → long_code, other → short_code, NULL → template category default) ✅

---

## Discrepancies
None.

---

## Uncovered Contracts

1. `get_previous_quarter` — fiscal quarter mapping and label generation not explicitly tested
2. `insert_quarter_data` — upsert conflict resolution not explicitly tested
3. `fetch_quarter_cummulative_stats` — multi-quarter aggregation not explicitly tested
4. Letter rate/postage differentiation — multiple rates per letter type (e.g., 0.33 vs 0.36) not fully verified
5. SMS `rate_multiplier` application — exact multiplier values and edge cases not verified
6. Historical data cutoff `2019-11-01` — `fetch_delivered_notification_stats_by_month` excludes pre-launch data; cutoff not tested

---

## RISK Items for Go Implementors

### 🔴 CRITICAL
**1. Quarterly aggregation (3 functions) is specified but not tested** — Implement with comprehensive test suite before deploying Go billing.

### 🟡 MEDIUM
**2. Letter rate/postage grouping** — Multiple rates per notification_type; verify grouping logic matches spec.

**3. Current-year FactBilling auto-population around date boundaries** — Today vs yesterday datetime arithmetic requires careful handling.

**4. SMS rate_multiplier calculation** — Depends on vehicle type and international flag; ensure correct multiplier is applied.
