## Context

The billing-tracking change implements the full billing aggregation pipeline for the Notification API:

- Annual SMS fragment limits per service per financial year (REST endpoints + DAO)
- Nightly fact-table writes for billing (`ft_billing`) and notification status (`ft_notification_status`)
- Quarterly reporting workers with a corrected beat schedule (**C6 fix**: missing Q4 entry)
- Monthly notification stats summary (`MonthlyNotificationStatsSummary`)
- REST endpoints exposing aggregated billing data to service dashboards

The Python codebase has an existing implementation. This change ports and specifies it for Go while fixing C6 (the `send-quarterly-email-q4` beat entry was never added in Python, so Q4 quarterly emails were never sent automatically).

---

## Goals / Non-Goals

**Goals**
- Specify all billing REST endpoints with precise HTTP contracts and error conditions
- Define DAO function signatures and idempotency guarantees
- Specify all nightly, quarterly, and monthly worker behaviour
- Correct the beat schedule to include `send-quarterly-email-q4` at `0 23 2 4 *`
- Specify SMS sending vehicle classification algorithm

**Non-Goals**
- Changing the financial year definition or billing rate model
- Modifying email template HTML
- Altering Redis data structures beyond clearing annual limit hash fields

---

## Decisions

### C6 fix: add send-quarterly-email-q4 beat entry

The Python worker beat schedule omits `send-quarterly-email-q4`. This means Q4 quarterly emails (covering Jan–Mar usage) were never sent automatically. The Go implementation **must** register all 4 entries:

| Entry | Cron | Fires on |
|---|---|---|
| send-quarterly-email-q1 | `0 23 2 7 *` | July 2 |
| send-quarterly-email-q2 | `0 23 2 10 *` | October 2 |
| send-quarterly-email-q3 | `0 23 2 1 *` | January 2 |
| **send-quarterly-email-q4** | **`0 23 2 4 *`** | **April 2 ← add this** |

This is a bug fix, not a behaviour change. Tests must assert all 4 cron expressions are registered in the scheduler.

### Rate preservation on FactBilling rerun (idempotent writes)

`update_fact_billing` (and its Go equivalent) performs an **INSERT … ON CONFLICT DO NOTHING**. If a record already exists for the billing tuple `(service_id, template_id, sent_by, rate_multiplier, notification_type, bst_date, …)`, it is left entirely unchanged, even if rates have since been updated. This guarantees that historical billing totals computed at the time of first aggregation are never silently altered by a later rerun.

Consequence: updating rates does not retroactively modify stored `FactBilling` records. Queries must use the stored `rate` column, not re-derive it from a current rates lookup.

### Billing data sources: live notifications vs notification_history

`fetch_billing_data_for_day` queries the `notifications` table for recent data and `notification_history` for older data — services with data-retention policies remove rows from `notifications` after N days, but those rows are preserved in `notification_history`. Both tables share the same schema for billing-relevant columns. The Go DAO must UNION both sources.

`update_fact_notification_status` likewise reads from both tables; the same UNION approach applies.

### BST timezone boundaries for all date operations

Workers receive a date string (`"YYYY-MM-DD"`) representing a calendar day, but must resolve the **BST-equivalent day boundary** (`Europe/London`) when querying `notifications` and `notification_history`. The Go implementation must convert the process day to a BST-aware interval (midnight–midnight London time) before issuing queries.

### SMS sending vehicle classification algorithm

The sending vehicle is determined by the `sms_origination_phone_number` column on the notification, evaluated in order:

1. Value matches `+1` followed by exactly 10 digits → **`long_code`**
2. Value is any other non-null string → **`short_code`**
3. Value is NULL → look up the template category default (typically `long_code`)
4. Notification is flagged as international → always **`long_code`** regardless of origination number

Vehicle classification affects the billing-tuple grouping key and the rate lookup via `get_rates_for_billing()`.

### Quarterly data upsert replaces, not accumulates

`insert_quarter_data_for_annual_limits` performs an upsert on `AnnualLimitsData`. On conflict the `count` column is **replaced** with the newly aggregated value (not added to the existing value). This makes the worker safe to re-run without inflating historical counts.

### Annual limit cascade on current-year POST

When `POST /billing/free-sms-fragment-limit` is called without an explicit `financial_year_start` (defaulting to the current fiscal year), the handler additionally calls `dao_update_annual_billing_for_future_years` to overwrite the limit on any future-year rows already present in `annual_billing`. This keeps pre-created future records consistent with the operator's intent. Explicit past-year updates do **not** trigger the cascade.

---

## Risks / Trade-offs

- **UNION of notifications + notification_history**: both tables must be queried on every billing run. If `notification_history` grows very large, query performance may degrade. Mitigation: ensure indexes on `(created_at, notification_type, key_type)` on both tables.
- **Rate preservation prevents corrections**: if a billing rate was recorded incorrectly on the first insert, a rerun will not fix it — a manual data migration is required. This is an accepted trade-off for historical immutability.
- **BST boundary edge case**: notifications created within the BST/UTC overlap hour (01:00–02:00 UTC during clock-change) may appear in two different billing days depending on timezone. Current behaviour mirrors the Python implementation; consistency is more important than philosophical correctness here.
- **C6 fix side-effect**: enabling Q4 emails means the first Go deployment will send Q4 quarterly emails that were never sent by the Python service. Operators should be warned before the initial deployment.
