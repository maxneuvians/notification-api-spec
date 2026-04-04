# Behavioral Spec: Billing

## Processed Files

- [x] `tests/app/billing/test_billing.py`
- [x] `tests/app/dao/test_annual_billing_dao.py`
- [x] `tests/app/dao/test_annual_limits_data_dao.py`
- [x] `tests/app/dao/test_fact_notification_status_dao.py`
- [x] `tests/app/dao/test_ft_billing_dao.py`
- [x] `tests/app/dao/test_provider_rates_dao.py`
- [x] `tests/app/celery/test_int_annual_limit_seeding.py`
- [x] `tests/app/celery/test_reporting_tasks.py`

---

## Endpoint Contracts

### POST /service/{service_id}/billing/free-sms-fragment-limit

Route name: `billing.create_or_update_free_sms_fragment_limit`

- **Happy path**: Creates or updates the `annual_billing` record for the given service and year. Returns HTTP 201.
- **Current-year update cascades forward**: When no `financial_year_start` is supplied (defaults to current year), all future-year records already in the table are also updated to the new limit. Records for past years are untouched.
- **Past-year update is isolated**: When `financial_year_start` is explicitly set to a past year, only that year's record is updated; current-year and future-year records are unaffected.
- **Upsert semantics**: If a record already exists for the requested year it is updated in place; no duplicate is created.
- **Validation rules**: Request body must be valid JSON containing `free_sms_fragment_limit`. A missing or empty body returns HTTP 400 with `"JSON"` in the error message.
- **Auth requirements**: Authorization header required on all requests.

### GET /service/{service_id}/billing/free-sms-fragment-limit

Optional query param: `financial_year_start` (integer year).

- **Happy path**: Returns HTTP 200 with `{ "financial_year_start": <year>, "free_sms_fragment_limit": <n> }`.
- **Current year, no record exists**: System auto-creates a record by copying the limit from the most recent prior year that does have a record. Returns the limit of that prior year under the current year's key.
- **Requested past year not found**: Falls back to the most recent year before the requested year that does have a record. Returns that year's limit.
- **Requested future year not found**: Falls back to the most recent year before (or equal to) the requested year. Returns that year's limit.
- **No record exists at all for the requested year**: Returns `None` from the DAO; the REST layer applies the fallback described above.
- **Auth requirements**: Authorization header required.

### GET /service/{service_id}/billing/ft-monthly-usage?year=YYYY

- **Happy path**: Returns an array of monthly billing objects, each with fields `month` (full month name), `notification_type`, `billing_units`, `rate`, `postage`. Returns HTTP 200.
- **Auto-populates FactBilling**: If the `FactBilling` table has no rows for the requested year, the endpoint triggers a delta-population from live notifications before querying. The resulting `FactBilling` rows are persisted.
- **Email exclusion**: Email rows (rate=0, billable_unit=0) are excluded from the response array.
- **Grouping**: Results are grouped by `(month, notification_type, rate, postage)`. Days within the same month are summed.
- **Letter differentiation**: Letters with different rates (e.g., 0.33 vs. 0.36) or different postage classes appear as separate rows.
- **SMS rate multiplier**: For SMS, `billing_units` = sum of `billable_unit × rate_multiplier` across all records in the month.

### GET /service/{service_id}/billing/ft-yearly-usage-summary?year=YYYY

- **Missing year param**: Returns HTTP 400 with `{ "message": "No valid year provided", "result": "error" }`.
- **No billing data**: Returns HTTP 200 with `[]`.
- **Happy path**: Returns HTTP 200 with an array sorted by `notification_type`. Each entry has fields `notification_type`, `billing_units`, `rate`, `letter_total`.
  - `letter_total` = `billing_units × rate` for letters; `0` for all other types.
  - Multiple rate rows per `notification_type` (e.g., two different letter rates) appear as separate entries.
  - SMS `billing_units` sum accounts for `rate_multiplier`.
  - Email appears with `rate=0` and `letter_total=0`.
- **Auth requirements**: Authorization header required.

---

## DAO Behavior Contracts

### `dao_create_or_update_annual_billing_for_year`

- **Expected behavior**: Creates a new `AnnualBilling` row if none exists for the given `(service_id, financial_year_start)`, or updates the existing row's `free_sms_fragment_limit`.
- **Edge cases**: Row is fully replaced in-place on update; the same object reference reflects the new value immediately.

### `dao_get_free_sms_fragment_limit_for_year`

- **Expected behavior**: Returns the `AnnualBilling` row for the exact `(service_id, year)` pair.
- **Edge cases**: Returns `None` when no record exists for the requested year.

### `dao_update_annual_billing_for_future_years`

- **Expected behavior**: Updates the `free_sms_fragment_limit` for every row where `financial_year_start > current_year` for the given service.
- **Boundary conditions**:
  - Records with `financial_year_start < current_year` are unmodified.
  - If no row exists for `current_year` itself, none is created.
  - Only already-existing future rows are updated; missing future years are not auto-created.

### `get_previous_quarter`

- **Expected behavior**: Given a date, returns a `(quarter_label, (start_dt, end_dt))` tuple for the calendar quarter that immediately preceded the current one.
- **Quarter mapping** (fiscal year starts April 1):

  | Date falls in   | Returns quarter | Date range                         |
  |-----------------|-----------------|-------------------------------------|
  | Apr 1 – Jun 30  | Q4-{prev FY}    | Jan 1 00:00 – Mar 31 23:59:59      |
  | Jul 1 – Sep 30  | Q1-{cur FY}     | Apr 1 00:00 – Jun 30 23:59:59      |
  | Oct 1 – Dec 31  | Q2-{cur FY}     | Jul 1 00:00 – Sep 30 23:59:59      |
  | Jan 1 – Mar 31  | Q3-{prev FY}    | Oct 1 00:00 – Dec 31 23:59:59      |

- **Label format**: `"Q{n}-{fiscal_year_start_year}"` (e.g., `"Q4-2020"`, `"Q1-2021"`).

### `insert_quarter_data`

- **Expected behavior**: Inserts `AnnualLimitsData` rows for `(service_id, notification_type, time_period)` with the provided `notification_count`.
- **Upsert on conflict**: If a row already exists for the same primary key, the `notification_count` is overwritten with the new value (not added).
- **Input shape**: List of namedtuples `(service_id, notification_type, notification_count)` plus a `service_info` dict mapping `service_id → (email_annual_limit, sms_annual_limit)`.

### `fetch_quarter_cummulative_stats`

- **Expected behavior**: Accepts a list of quarter labels and a list of service IDs. Returns an iterable of `(service_id, counts_dict)` pairs where `counts_dict` maps `notification_type → total_count` summed across all specified quarters.
- **Aggregation rule**: Same `(service_id, notification_type)` across different time periods → counts summed.
- **Filtering**: Only returns data for the provided service IDs and quarter labels.

### `update_fact_notification_status`

- **Expected behavior**: For the given `process_day` (local-timezone date), deletes all existing `FactNotificationStatus` rows matching that date and the specified `service_ids`, then inserts fresh aggregate counts derived from current notifications.
- **Data sources**: Queries both the `notifications` table and the `notification_history` table (used when a service has data retention and notifications have been moved).
- **Row shape**: One row per `(bst_date, service_id, template_id, notification_type, notification_status)`. `job_id` is `00000000-0000-0000-0000-000000000000` when no job.
- **Scope of deletion**:
  - With `service_ids` supplied: only deletes rows for those services; other services' data for the same day is preserved.
  - Without `service_ids`: deletes all rows for that day regardless of service.
- **Empty result**: If no notifications exist in the source for that day, no rows are inserted (deleted rows are not re-inserted).
- **Idempotent**: Running twice with the same data produces the same final row count and values.

### `fetch_notification_status_for_service_by_month`

- **Expected behavior**: Returns counts grouped by `(month, notification_type, notification_status)` for a service within a date range.
- **Date range**: Inclusive; `bst_date` must fall within `[start_date, end_date]`.
- **Exclusions**: Test key (`key_type='test'`) notifications excluded. Records for other services excluded.
- **Aggregation**: Rows with the same `(month, type, status)` sum their counts.

### `fetch_notification_status_for_service_for_day`

- **Expected behavior**: Returns counts by `(notification_type, notification_status)` for a given service on a single local-timezone day.
- **Day boundary**: Based on local timezone. For EST/BST services a UTC day does not align to a calendar day — the local midnight is the boundary.
- **Key type**: Includes `normal` and `team` key types; excludes `test`.
- **Service scoping**: Only the requested service's notifications are counted.

### `fetch_notification_status_for_service_for_today_and_7_previous_days`

- **Expected behavior**: Combines fact table data (8 days ago to yesterday) with live notification table data (today), returning aggregated counts per `(notification_type, status)`.
- **7-day exclusion**: Notifications older than the 8-day window are excluded.
- **`by_template=True`**: Breaks results down by template in addition to type/status.
- **`limit_days=1`**: Restricts to today only (midnight UTC boundary).
- **Midnight UTC handling**: Only notifications created on or after local midnight (UTC midnight) count as "today"; earlier notifications on the calendar day are excluded.

### `_timing_notification_table`

- **Expected behavior**: Returns the cutoff datetime for switching from live notifications to the fact table. Equals `(most recent bst_date in fact table for this service) + 1 day` at 00:00:00.

### `fetch_notification_status_totals_for_all_services`

- **Expected behavior**: Aggregates counts per `(notification_type, status)` across all services for a given date range, combining fact table and today's live data.
- **Exclusions**: Test key notifications excluded. Notifications beyond the date range excluded.

### `fetch_notification_statuses_for_job`

- **Expected behavior**: Returns `{status: count}` aggregated across all dates for the specified `job_id`. Rows from the same job on different days are summed.

### `fetch_stats_for_all_services_by_date_range`

- **Expected behavior**: Returns per-`(service_id, notification_type, status)` counts for a date range, covering all services.
- **Services with no data**: Still appear in results with `None` for `notification_type`, `status`, and `count`.

### `fetch_monthly_template_usage_for_service`

- **Expected behavior**: Returns per-template usage counts broken down by `(month, year)` within the specified date range.
- **Today in range**: If the end of the date range is today or later, the function supplements fact table data with live notifications from the notifications table.
- **Today not in range**: Only queries the fact table.
- **Exclusions**: `cancelled` status excluded. Test key notifications excluded.
- **Result shape**: `(template_id, name, is_precompiled_letter, template_type, month, year, count)`.

### `fetch_delivered_notification_stats_by_month`

- **Expected behavior**: Returns `(month, notification_type, count)` rows from the `MonthlyNotificationStatsSummary` table, sorted by month descending. Aggregates all services together.
- **Historical cutoff**: Excludes data with `month < 2019-11-01` (before GC Notify launched).
- **Heartbeat filtering**: `filter_heartbeats=True` excludes all records belonging to `NOTIFY_SERVICE_ID`.
- **Empty case**: Returns `[]` with no data.

### `fetch_notification_stats_for_trial_services`

- **Expected behavior**: Returns stats only for `restricted=True` (trial) services. Excludes live services and failed notifications.
- **Result shape**: `(service_id, service_name, creation_date, user_name, user_email, notification_type, notification_sum)`.

### `fetch_notification_status_totals_for_service_by_fiscal_year`

- **Expected behavior**: Counts all `FactNotificationStatus` rows for a service within the fiscal year (`Apr 1` of `year` through `Mar 31` of `year+1`).
- **Notification type filter**: When `notification_type` is provided, only counts rows of that type. When `None`, counts all types.
- **Returns**: Integer total.

### `fetch_quarter_data`

- **Expected behavior**: Returns `(service_id, notification_type, total_count)` tuples for a specified date range and list of service IDs. Aggregates counts from the fact table.

### `fetch_billable_units_for_service_for_day`

- **Expected behavior**: Returns `(notification_type, notification_status, aggregated_billable_units)` for a single local-timezone day. Groups by type and status, summing `billable_units`.
- **Date scope**: Only today's notifications; yesterday and earlier excluded.

### `fetch_billable_units_totals_for_service_by_fiscal_year`

- **Expected behavior**: Sums `billable_units` from the fact table for a service within the fiscal year, filtered by `notification_type`.
- **Returns**: Integer (or 0 when no data).

### `fetch_notification_billable_units_for_service_for_today_and_7_previous_days`

- **Expected behavior**: Combines today's live notification billable_units with the fact table entries for the previous 7 days. Aggregates as `(notification_type, status, billable_units, count)`.
- **`by_template=True`**: Adds `template_id` grouping.

### `get_total_notifications_sent_for_api_key`

- **Expected behavior**: Returns `[(notification_type, count)]` for the given API key ID. Returns `[]` when the key has sent nothing.

### `get_last_send_for_api_key`

- **Expected behavior**: Returns the `last_used_timestamp` of the API key when set. Returns `[]` when unset.

### `get_api_key_ranked_by_notifications_created`

- **Expected behavior**: Returns top-N API keys ranked by total notifications created. Each row has 9 fields, including key name, service name, email count, SMS count, and grand total.

### `get_total_sent_notifications_for_day_and_type`

- **Expected behavior**: Returns the total `notification_count` for a specific `(day, notification_type)` from the fact table.
- **Returns**: `0` when no records exist.

### `fetch_billing_data_for_day`

- **Expected behavior**: Queries `notifications` (and `notification_history` for older dates) to build per-row billing aggregates for a given local-timezone day.
- **Billable statuses**: `delivered`, `sending`, `temporary-failure` — counted.
- **Non-billable statuses**: `created`, `technical-failure` — excluded.
- **Key type filtering**: `test` key excluded; `normal` and `team` included.
- **Grouping dimensions**: `(template, service, sent_by/provider, rate_multiplier, international, notification_type, sms_sending_vehicle)`.
- **SMS sending vehicle determination**:
  - `sms_origination_phone_number` matching `+1XXXXXXXXXX` (E.164 +1 + 10 digits) → `long_code`.
  - Any other non-null value → `short_code`.
  - `NULL` origination → falls back to the template's category `sms_sending_vehicle` (default `long_code`).
  - International SMS → always classified as `long_code` (rate-multiplier accounts for destination cost).
- **Historical data**: When `notification_history` is the source (data retention services), results are identical.
- **`service_id` parameter**: Optional. When provided, results are scoped to that service only.

### `get_rates_for_billing`

- **Expected behavior**: Returns `(non_letter_rates, letter_rates)` tuple. Non-letter rates include separate SMS entries keyed by `sms_sending_vehicle`. Letter rates are separate.

### `get_rate`

- **Expected behavior**: Picks the rate whose `start_date` is the most recent date not exceeding `process_date`.
- **SMS**: Differentiates by `sms_sending_vehicle` (`long_code` vs `short_code`).
- **Letters**: Filters by `post_class` and `sheet_count` (page count maps to sheet count).
- **Zero page count**: Returns `0` for letters with `letter_page_count=0`.
- **Missing rate**: Logs an error with tag `[error-sms-rates]` (including the vehicle name) and raises `ValueError`.

### `create_billing_record`

- **Expected behavior**: Constructs a `FactBilling` ORM object from notification data and a rate.
- **Calculation rule**: `billing_total = billable_units × rate_multiplier × rate`.

### `update_fact_billing`

- **Expected behavior**: Persists the `FactBilling` record to the database. If an existing record already exists for that `(service, date, template, …)` key, the existing record is preserved unchanged (idempotent rerun — rate does not change).

### `fetch_monthly_billing_for_year`

- **Expected behavior**: Returns rows grouped by `(month, notification_type, rate, postage)` for the financial year `year` (Apr `year` – Mar `year+1`).
- **Auto-population**: When called for the current year, if today's billing row is missing it is automatically computed and inserted before the query runs.
- **`billable_units`**: `sum(billable_unit × rate_multiplier)` within the group.
- **Result count**: Financial year with data for all 12 months and 4 types/rates = 52 rows (4 rows/month × 13 months in a leap/split year).

### `fetch_billing_totals_for_year`

- **Expected behavior**: Returns annual totals per `(notification_type, rate)` combination. Same fields as monthly but summed over the full year `(notifications_sent, billable_units, rate)`.

### `delete_billing_data_for_service_for_day`

- **Expected behavior**: Deletes all `ft_billing` rows for the specified `(service_id, utc_date)`. Rows for other services or other dates are untouched.

### `fetch_sms_free_allowance_remainder`

- **Expected behavior**: Returns `(service_id, free_sms_fragment_limit, used_units, remaining_units)` for each service as of the given date.
- **`remaining`**: `max(0, free_sms_fragment_limit - used_units)`.
- **`used_units`**: Sum of `billable_units` for SMS ft_billing rows from the start of the financial year through the given date.

### `fetch_sms_billing_for_all_services`

- **Expected behavior**: Returns per-service SMS billing for a date range with fields: `(org_name, org_id, service_name, service_id, free_sms_fragment_limit, sms_rate, sms_remainder, sms_billable_units, chargeable_billable_units, sms_cost)`.
- **No organisation**: Services with no linked org appear with `org_name=None`, `org_id=None`.
- **Email-only services**: Excluded (no SMS billing rows → not returned).
- **Cost formula**: `sms_cost = chargeable_billable_units × sms_rate`.

### `fetch_letter_costs_for_all_services`

- **Expected behavior**: Returns `(org_name, org_id, service_name, service_id, total_letter_cost)` per service for a date range. No-org services appear with `None` in org fields.

### `fetch_letter_line_items_for_all_services`

- **Expected behavior**: Returns a per-rate-postage breakdown: `(org_name, org_id, service_name, service_id, rate, postage, count)`. Multiple letter-rate rows per service appear as separate items.

### `create_provider_rates`

- **Expected behavior**: Creates a single `ProviderRates` record, looked up by provider identifier. Stores `rate` (Decimal) and `valid_from` (datetime) linked to the `ProviderDetails.id`.

---

## Task Behavior Contracts

### `create_nightly_billing`

- **What it does**: Dispatches `create_nightly_billing_for_day` as an async Celery task for 4 consecutive days.
- **Without `day_start`**: Processes yesterday, 2 days ago, 3 days ago, 4 days ago (relative to now).
- **With `day_start`**: Processes the given date and the 3 days before it.
- **Verified behavior**: Exactly 4 `apply_async` calls are issued, each with `kwargs={"process_day": "YYYY-MM-DD"}`.

### `create_nightly_billing_for_day`

- **What it does**: Reads billing-eligible notifications for a date string (`YYYY-MM-DD`) and writes `FactBilling` rows.
- **Rate multiplier merging**: Notifications with identical rate_multiplier on the same template/service/provider are merged into one row; different multipliers produce separate rows.
- **BST boundary**: Day is interpreted in local (BST/EST) timezone.
- **Null provider**: Stored as `provider='unknown'`.
- **Idempotency (rate preservation)**: Re-running after a new rate row is added for a later date does not change the rate stored in the existing `FactBilling` record. The rate in the fact table is fixed at the value that applied on the original process day.
- **`billing_total` stored**: `billable_units × rate_multiplier × rate` is persisted.

### `create_nightly_notification_status`

- **What it does**: Dispatches `create_nightly_notification_status_for_day` for 4 days. Scheduling is identical to `create_nightly_billing`.
- **Verified behavior**: 4 `apply_async` calls with correct `process_day` kwargs.

### `create_nightly_notification_status_for_day`

- **What it does**: Aggregates notification statuses for a given date into `FactNotificationStatus`. After persistence, clears Redis annual limit counts for all affected services.
- **BST timezone**: Day boundary respected; only notifications in the local day are counted.
- **`billable_units` copied**: The raw `billable_units` field from the notification is copied to the fact row.
- **Both tables sourced**: Reads from `notifications` AND `notification_history`.
- **Redis clear**: After the fact table is updated, all annual limit hash values for affected services are set to `0` in Redis (`annual_limit_client`). Handles batches larger than the chunk size (tested with 39 services).

### `insert_quarter_data_for_annual_limits`

- **What it does**: Given a datetime, determines the previous quarter using `get_previous_quarter`, then aggregates notification counts from `FactNotificationStatus` and upserts them into `AnnualLimitsData`.
- **Upsert**: Existing rows for the same `(service_id, notification_type, time_period)` have their count replaced, not added.
- **Verified with real dates**: Q4-2017 (Jan–Mar 2018) and Q1-2018 (Apr–Jun 2018) both produce correct labels and counts.

### `send_quarter_email`

- **What it does**: For each service user, sends a quarterly usage email summarising per-service notification counts vs annual limits.
- **Email content (Markdown)**: Contains `## {service_name}` headings, sent count, annual limit, percentage used. Bilingual (EN/FR).
- **Delegates to**: `send_annual_usage_data(user_id, fy_start, fy_end, markdown_en, markdown_fr)`.

### `create_monthly_notification_stats_summary`

- **What it does**: Aggregates `FactNotificationStatus` data for the current and previous month into `MonthlyNotificationStatsSummary` (upsert).
- **Date scope**: Current month + previous month only. Data older than 2 months is excluded.
- **Included statuses**: Only `delivered` and `sent`. Failed, created, temporary-failure, etc. are excluded.
- **Test key exclusion**: `key_type='test'` notifications excluded.
- **Aggregation**: Multiple days in the same month, same service, same type → single summed row.
- **Upsert / re-run**: Running again with updated source data overwrites the existing count and updates `updated_at`.

### Integration: Annual limit seeding and incrementation (`test_int_annual_limit_seeding.py`)

- **Flow tested**:
  1. At end-of-day: `create_nightly_notification_status_for_day` clears all Redis `annual_limit` hash values to `0` for every affected service.
  2. Next day, first delivery receipt (`process_sns_results`): detects Redis is unseeded; reads DB for notifications from start of fiscal year to yesterday and seeds Redis with cumulative totals + today's partial counts.
  3. Same day, subsequent delivery receipts: Redis is already seeded; only today's per-type counters are incremented (no DB read).
- **Redis keys populated on seeding** (26 services tested in parallel, 25 for first phase):
  - `sms_failed_today`, `sms_delivered_today`, `email_failed_today`, `email_delivered_today`
  - `total_email_fiscal_year_to_yesterday`, `total_sms_fiscal_year_to_yesterday`
  - (with `FF_USE_BILLABLE_UNITS=True`): `sms_billable_units_delivered_today`, `sms_billable_units_failed_today`, `total_sms_billable_units_fiscal_year_to_yesterday`
- **`seeded_at` key**: Set by `annual_limit_client.set_seeded_at()` when seeding occurs; checked by `annual_limit_client.was_seeded_today()` to gate increment-vs-seed logic.

---

## Business Rules Verified

- **Financial year boundary**: The financial year runs from April 1 of year N through March 31 of year N+1. `financial_year_start` is always the April year (e.g., 2024 = Apr 2024 – Mar 2025).
- **Annual billing limit cascade**: Setting the current year's free SMS limit automatically propagates to all future-year records that already exist. Past-year changes are isolated.
- **Free SMS fallback on missing year**: When no record exists for a queried year, the most recent year before it is used.
- **Billable status rule**: Only `delivered`, `sending`, and `temporary-failure` count for billing. `created` and `technical-failure` do not.
- **Test key exclusion**: All billing, stats, and fact-table queries exclude `key_type='test'` notifications universally.
- **SMS sending vehicle classification**: Origination number `+1XXXXXXXXXX` → `long_code`; any other value → `short_code`; `NULL` → template category default (usually `long_code`); international SMS → always `long_code` for rate-lookup purposes.
- **billing_total formula**: `billable_units × rate_multiplier × rate` (stored in `FactBilling.billing_total`).
- **Rate lookup rule**: Most recent rate whose `start_date ≤ process_date`. Missing rate → error log `[error-sms-rates]` + `ValueError`.
- **Rate preservation on rerun**: Once a `FactBilling` record is created and persisted, re-running nightly billing for the same day does not change the stored `rate` or `billing_total`, even if a newer rate row has since been inserted.
- **Quarter label format**: `"Q{1|2|3|4}-{fiscal_year}"` where fiscal year is the April-start year for that quarter.
- **Monthly stats summary scope**: Only `delivered` and `sent` statuses; only current and previous month; test keys excluded; multiple days in a month aggregated into one row per `(service, type)`.
- **Annual limit Redis lifecycle**: Redis keys are seeded on first delivery of each day; cleared to `0` at end-of-day by `create_nightly_notification_status_for_day`; subsequent deliveries within the day only increment already-seeded keys.
- **Fiscal year notification totals**: `fetch_notification_status_totals_for_service_by_fiscal_year` counts only notifications with `bst_date` in `[Apr 1 of year, Mar 31 of year+1]`.
- **FactBilling grouping dimensions**: Rows are unique per `(bst_date, service, template, provider, rate_multiplier, international, notification_type, sms_sending_vehicle, postage)`.
- **Letter rate differentiation**: Letters at different rates or postage classes are separate billing rows, separate monthly summary rows, and separate yearly summary rows.
