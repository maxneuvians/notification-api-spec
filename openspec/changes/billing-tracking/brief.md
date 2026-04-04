# Billing Tracking — Change Brief

## Overview

Implements the full billing aggregation pipeline for the Notification API: annual SMS fragment limits per service, nightly fact-table writes (`ft_billing`, `ft_notification_status`), quarterly reporting workers, monthly notification stats, and the REST endpoints that expose billing data. Also fixes **C6**: the Python beat schedule was missing the `send-quarterly-email-q4` entry; the Go implementation must add it.

---

## REST Endpoints

### POST /service/{service_id}/billing/free-sms-fragment-limit

- Creates or updates an `annual_billing` record for the given service and year; returns **HTTP 201**
- No `financial_year_start` in body → defaults to current financial year **and** cascades to all future-year `annual_billing` rows already in the table
- Explicit past-year `financial_year_start` → updates only that year (no cascade)
- Upsert semantics: no duplicate row is created on repeated calls
- Missing or empty body → **HTTP 400** with `"JSON"` in the error message
- Authorization header required (**HTTP 401** when absent)

### GET /service/{service_id}/billing/free-sms-fragment-limit

- Optional query param: `financial_year_start` (integer year)
- Returns **HTTP 200** `{"financial_year_start": <year>, "free_sms_fragment_limit": <n>}`
- Current year, no record in DB → auto-creates by copying limit from the most recent prior-year record
- Requested year not found → falls back to the most recent year before the requested year
- Authorization header required

### GET /service/{service_id}/billing/ft-monthly-usage?year=YYYY

- Returns an array of monthly billing objects; **HTTP 200**
- Fields per object: `month` (full name), `notification_type`, `billing_units`, `rate`, `postage`
- Auto-populates `FactBilling` if no rows exist for the requested year before querying (result is persisted, not transient)
- Email rows excluded (rate=0, billable_unit=0)
- Grouped by `(month, notification_type, rate, postage)`; billing days within the same month are summed
- Letters with different rates or postage appear as separate rows
- SMS `billing_units` = sum(billable_unit × rate_multiplier)
- Authorization header required

### GET /service/{service_id}/billing/ft-yearly-usage-summary?year=YYYY

- Missing `year` param → **HTTP 400** `{"message": "No valid year provided", "result": "error"}`
- No data for year → **HTTP 200** `[]`
- Happy path → **HTTP 200**, sorted by `notification_type`
- Fields: `notification_type`, `billing_units`, `rate`, `letter_total`
- `letter_total` = `billing_units × rate` for letters; `0` for SMS and email rows
- Multiple rates per notification type appear as separate rows
- SMS `billing_units` accounts for rate_multiplier
- Authorization header required

---

## Background Workers

### create_nightly_billing

- Dispatches `create_nightly_billing_for_day` for 4 consecutive days
- Without `day_start`: yesterday, 2 days ago, 3 days ago, 4 days ago
- With `day_start`: given date + 3 days prior
- Produces exactly 4 `apply_async` calls with `kwargs={"process_day": "YYYY-MM-DD"}`

### create_nightly_billing_for_day

- Reads billing-eligible notifications for a date string; writes `FactBilling` rows
- Rate multiplier merging: same multiplier on same template/service/provider → one row; different multipliers → separate rows
- Uses **BST timezone** boundary for date resolution
- Null provider stored as `'unknown'`
- **Idempotent**: re-running after a new rate change does NOT update the stored rate in existing `FactBilling` records (INSERT … ON CONFLICT DO NOTHING)
- `billing_total` = `billable_units × rate_multiplier × rate` (persisted to DB)

### create_nightly_notification_status

- Dispatches `create_nightly_notification_status_for_day` for 4 days (same scheduling logic as billing)

### create_nightly_notification_status_for_day

- Aggregates notification statuses for the given date into `FactNotificationStatus`
- After persistence, clears Redis annual limit counts for all affected services (sets all hash values to 0)
- Respects **BST timezone** boundaries
- `billable_units` copied from the notification record
- Reads from **both** `notifications` **and** `notification_history` tables
- Redis clear: `annual_limit_client` sets all hash values for each affected service to 0; handles batches exceeding chunk size (tested with 39 services)

### insert_quarter_data_for_annual_limits

- Given a datetime, determines the previous quarter via `get_previous_quarter`
- Aggregates from `FactNotificationStatus`, upserts into `AnnualLimitsData`
- Upsert semantics: existing rows have count **REPLACED** (not accumulated)

### send_quarter_email

- Per-service usage email: sent count, annual limit, percentage used
- Bilingual (EN/FR)
- Markdown format: `## {service_name}` headings
- Delegates to `send_annual_usage_data(user_id, fy_start, fy_end, markdown_en, markdown_fr)`

**C6 fix (critical):** The Python beat schedule is missing `send-quarterly-email-q4`. The Go implementation must include **all 4** entries:

| Beat entry | Cron | Date |
|---|---|---|
| send-quarterly-email-q1 | `0 23 2 7 *` | July 2 |
| send-quarterly-email-q2 | `0 23 2 10 *` | October 2 |
| send-quarterly-email-q3 | `0 23 2 1 *` | January 2 |
| **send-quarterly-email-q4** | **`0 23 2 4 *`** | **April 2 — MISSING IN PYTHON** |

### create_monthly_notification_stats_summary

- Aggregates `FactNotificationStatus` for current + previous month into `MonthlyNotificationStatsSummary` (upsert)
- Only `delivered` and `sent` statuses included
- Test key exclusion
- Re-run overwrites existing count and updates `updated_at`

---

## DAO Layer

| Function | Behaviour |
|---|---|
| `dao_create_or_update_annual_billing_for_year(service_id, free_sms_fragment_limit, financial_year_start)` | Upsert; `@transactional` |
| `dao_update_annual_billing_for_future_years(service_id, limit, year)` | Bulk UPDATE `annual_billing` where `financial_year_start > year`; only updates existing rows |
| `dao_get_free_sms_fragment_limit_for_year(service_id, year)` | Returns `AnnualBilling` or None |
| `update_fact_notification_status(process_day, service_ids)` | DELETE existing rows for day+services, INSERT fresh aggregates; reads both `notifications` and `notification_history`; idempotent |
| `fetch_billing_data_for_day(date, service_id)` | UNION `notifications` + `notification_history`; BST boundary; grouped by billing tuple; billable statuses filter; test key exclusion |
| `update_fact_billing(ft_billing)` | INSERT … ON CONFLICT DO NOTHING — existing record preserved unchanged on rerun |
| `fetch_monthly_billing_for_year(year)` | Grouped by (month, notification_type, rate, postage) |
| `fetch_billing_totals_for_year(year)` | Annual totals per (notification_type, rate) |
| `get_rates_for_billing()` | Returns (non_letter_rates, letter_rates); SMS differentiated by `sms_sending_vehicle` |
| `get_rate(process_date, vehicle, …)` | Most recent rate where `start_date <= process_date`; missing rate → log `[error-sms-rates]` + ValueError |

### fetch_billing_data_for_day — grouping key

`(template, service, sent_by, rate_multiplier, international, notification_type, sms_sending_vehicle)`

Billable statuses: `delivered`, `sending`, `temporary-failure`
Excluded statuses: `created`, `technical-failure`
Test keys: always excluded

---

## SMS Sending Vehicle Classification

| `sms_origination_phone_number` value | Vehicle |
|---|---|
| `+1` followed by exactly 10 digits | `long_code` |
| Any other non-null string | `short_code` |
| NULL → template category default | Usually `long_code` |
| International notification (any value) | Always `long_code` |

---

## Business Rules

- **Financial year**: April 1 of year N → March 31 of year N+1
- **Current-year cascade**: updating current year calls `dao_update_annual_billing_for_future_years` to overwrite all future-year rows already present in `annual_billing`
- **Billable statuses**: `delivered`, `sending`, `temporary-failure` only
- **Test key exclusion**: universal across all billing and stats queries
- **`billing_total`**: `billable_units × rate_multiplier × rate` (stored in `FactBilling`)
- **Rate preservation**: existing `FactBilling` record rate is NEVER changed after first write; reruns are no-ops for existing rows
- **Quarter labels**: `Q{1|2|3|4}-{fiscal_year}` e.g. `Q1-2025`

### get_previous_quarter mapping

| Month of input datetime | Quarter returned | Date range |
|---|---|---|
| Jan–Mar | Q3-{prev_year} | Oct 1 – Dec 31 |
| Apr–Jun | Q4-{prev_fy} | Jan 1 – Mar 31 |
| Jul–Sep | Q1-{cur_year} | Apr 1 – Jun 30 |
| Oct–Dec | Q2-{cur_year} | Jul 1 – Sep 30 |

- **Monthly stats**: `delivered` + `sent` only; current + previous month only; test keys excluded
- **Annual limit Redis lifecycle**: seeded on first delivery each day; cleared to 0 by the nightly notification status task
