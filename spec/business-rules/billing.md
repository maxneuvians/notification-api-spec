# Business Rules: Billing

## Overview

The billing domain spans five tables (`annual_billing`, `annual_limits_data`, `ft_billing`,
`ft_notification_status`, `monthly_notification_stats_summary`) and one supporting table
(`provider_rates`). Together they track:

- **Annual SMS free allowances** per service per financial year (`annual_billing`).
- **Quarterly notification counts** used for annual limit enforcement
  (`annual_limits_data`).
- **Daily billable-unit aggregations** with cost calculations for SMS, email, and letters
  (`ft_billing` / `FactBilling`).
- **Daily notification-status aggregations** for reporting and near-real-time dashboards
  (`ft_notification_status` / `FactNotificationStatus`).
- **Monthly delivered/sent summary** for high-performance platform stats
  (`monthly_notification_stats_summary`).
- **Provider-level rates** tied to specific providers and effective dates
  (`provider_rates`).

All date/time comparisons use BST dates (local timezone) stored as plain `DATE` values,
while `created_at` timestamps are stored as UTC.

---

## Data Access Patterns

### `annual_billing_dao.py`

#### `dao_create_or_update_annual_billing_for_year(service_id, free_sms_fragment_limit, financial_year_start)`

- **Purpose**: Upsert the free SMS fragment limit for a service in a given financial year.
- **Query type**: SELECT then UPDATE or INSERT (within a transaction).
- **Key filters/conditions**: `service_id` + `financial_year_start` exact match.
- **Returns**: The `AnnualBilling` ORM instance (new or updated).
- **Notes**: Decorated `@transactional`. Delegates to
  `dao_get_free_sms_fragment_limit_for_year` for the read step; if no row exists it
  creates a new `AnnualBilling` object and adds it to the session.

---

#### `dao_get_annual_billing(service_id)`

- **Purpose**: Retrieve all annual billing records for a service, across all financial years.
- **Query type**: SELECT (no mutation).
- **Key filters/conditions**: `service_id` only.
- **Returns**: List of `AnnualBilling`, ordered by `financial_year_start` ascending.
- **Notes**: Returns every year on record; callers must index into the list for a
  specific year.

---

#### `dao_update_annual_billing_for_future_years(service_id, free_sms_fragment_limit, financial_year_start)`

- **Purpose**: Propagate a new SMS limit forward to all future financial years already
  stored for a service.
- **Query type**: Bulk UPDATE (within a transaction).
- **Key filters/conditions**: `service_id` match and
  `financial_year_start > financial_year_start` (strictly greater than the supplied year).
- **Returns**: Nothing (SQLAlchemy row count discarded).
- **Notes**: Decorated `@transactional`. Only affects rows that already exist — future
  years not yet created are unaffected until they are created.

---

#### `dao_get_free_sms_fragment_limit_for_year(service_id, financial_year_start=None)`

- **Purpose**: Fetch the free SMS fragment limit for one service/year combination.
- **Query type**: SELECT (no mutation).
- **Key filters/conditions**: `service_id` + `financial_year_start`; defaults to current
  financial year when `financial_year_start` is falsy.
- **Returns**: Single `AnnualBilling` row or `None`.
- **Notes**: Used both as a direct API-layer lookup and as the read step inside
  `dao_create_or_update_annual_billing_for_year`.

---

#### `dao_get_all_free_sms_fragment_limit(service_id)`

- **Purpose**: Fetch all annual billing records for a service (alias of
  `dao_get_annual_billing` with the same ordering).
- **Query type**: SELECT.
- **Key filters/conditions**: `service_id` only.
- **Returns**: List of `AnnualBilling`, ordered by `financial_year_start` ascending.
- **Notes**: Used in the REST layer when a year-specific record is absent; callers
  access `sms_list[0]` (oldest) or `sms_list[-1]` (newest) by index.

---

### `annual_limits_data_dao.py`

#### `get_previous_quarter(date_to_check)`

- **Purpose**: Compute the quarter label and calendar date range for the quarter that
  ended before `date_to_check`.
- **Query type**: Pure computation (no DB access).
- **Key filters/conditions**: Based on `date_to_check.month`:
  - Jan–Mar → Q3 of previous calendar year (Oct 1 – Dec 31).
  - Apr–Jun → Q4 of previous fiscal year (Jan 1 – Mar 31, stored under `year - 1`).
  - Jul–Sep → Q1 of current year (Apr 1 – Jun 30).
  - Oct–Dec → Q2 of current year (Jul 1 – Sep 30).
- **Returns**: `(quarter_name: str, (start_date, end_date))`, e.g. `("Q1-2025", (...))`.
- **Notes**: Quarter labels follow the fiscal year (April start): Q1=Apr–Jun,
  Q2=Jul–Sep, Q3=Oct–Dec, Q4=Jan–Mar.

---

#### `get_all_quarters(process_day)`

- **Purpose**: Build a list of all completed quarter names up to and including the
  previous quarter for a given processing date.
- **Query type**: Pure computation.
- **Key filters/conditions**: Delegates to `get_previous_quarter`; uses a static
  mapping keyed on the quarter label.
- **Returns**: List of quarter-name strings, e.g. `["Q1-2025", "Q2-2025"]`.
- **Notes**: Used to determine which quarters to include when building a cumulative
  annual-limit report.

---

#### `insert_quarter_data(data, quarter, service_info)`

- **Purpose**: Upsert notification counts for a given quarter into `annual_limits_data`.
- **Query type**: PostgreSQL `INSERT … ON CONFLICT DO UPDATE` (one statement per row,
  committed immediately).
- **Key filters/conditions**: Conflict target is the unique index on
  `(service_id, time_period, notification_type)`.
- **Returns**: Nothing.
- **Notes**: `service_info` is a dict keyed by `service_id` with a 2-tuple
  `(annual_email_limit, annual_sms_limit)` — limits are snapshot-stored alongside
  counts so historical limit values are preserved. Each row commits immediately; not
  wrapped in a single outer transaction.

---

#### `fetch_quarter_cummulative_stats(quarters, service_ids)`

- **Purpose**: Return cumulative notification counts per service across a list of
  quarters, pivoted by notification type into a JSON object.
- **Query type**: SELECT with subquery + aggregation.
- **Key filters/conditions**: `service_id IN service_ids` and
  `time_period IN quarters`.
- **Returns**: List of rows with `(service_id, notification_counts)` where
  `notification_counts` is a JSON object mapping type → count.
- **Notes**: Uses `func.json_object_agg` (PostgreSQL-specific). The subquery groups by
  `(service_id, notification_type)`; the outer query aggregates further to produce one
  row per service.

---

### `fact_billing_dao.py`

#### `fetch_sms_free_allowance_remainder(start_date)`

- **Purpose**: For every service, compute how many free SMS fragments remain at the
  start of a given date within its financial year.
- **Query type**: SELECT with OUTER JOIN + aggregation; returns a SQLAlchemy query
  object (not yet executed).
- **Key filters/conditions**: `AnnualBilling.financial_year_start == billing_year`;
  `ft_billing` rows joined on `service_id`, `bst_date >= start_of_year`,
  `bst_date < start_date`, `notification_type == SMS_TYPE`.
- **Returns**: Un-executed query yielding rows of
  `(service_id, free_sms_fragment_limit, billable_units, sms_remainder)` where
  `sms_remainder = max(limit − used, 0)`.
- **Notes**: OUTER JOIN ensures services with zero ft_billing rows are still returned.
  Used as a subquery by `fetch_sms_billing_for_all_services`.

---

#### `fetch_sms_billing_for_all_services(start_date, end_date)`

- **Purpose**: Administrative report — SMS costs for every service in the period.
- **Query type**: SELECT with multiple joins + aggregation; executed immediately.
- **Key filters/conditions**: `ft_billing.bst_date` in `[start_date, end_date]`;
  `notification_type == SMS_TYPE`.
- **Returns**: List of rows:
  `(organisation_name, organisation_id, service_name, service_id,
  free_sms_fragment_limit, sms_rate, sms_remainder, sms_billable_units,
  chargeable_billable_sms, sms_cost)`.
- **Notes**: `chargeable_sms = max(total_billable_units − sms_remainder, 0)`;
  `sms_cost = chargeable_sms × rate`. The free-allowance remainder is pre-computed
  from `fetch_sms_free_allowance_remainder` as a subquery.

---

#### `fetch_letter_costs_for_all_services(start_date, end_date)`

- **Purpose**: Administrative report — total letter spend per service in the period.
- **Query type**: SELECT with joins + aggregation; executed immediately.
- **Key filters/conditions**: `ft_billing.bst_date` in `[start_date, end_date]`;
  `notification_type == LETTER_TYPE`.
- **Returns**: List of rows:
  `(organisation_name, organisation_id, service_name, service_id, letter_cost)` where
  `letter_cost = sum(notifications_sent × rate)`.

---

#### `fetch_letter_line_items_for_all_services(start_date, end_date)`

- **Purpose**: Administrative report — letter line items per service, rate, and postage
  class.
- **Query type**: SELECT with joins + aggregation; executed immediately.
- **Key filters/conditions**: Same date/type filters as
  `fetch_letter_costs_for_all_services`.
- **Returns**: List of rows:
  `(organisation_name, organisation_id, service_name, service_id, letter_rate, postage,
  letters_sent)`.
- **Notes**: Ordered by `organisation_name, service_name, postage DESC, rate`.

---

#### `fetch_billing_totals_for_year(service_id, year)`

- **Purpose**: Annual billing summary for a single service, split by notification type.
- **Query type**: Two queries (email+letter UNION ALL sms) executed immediately.
- **Key filters/conditions**: `service_id`, `bst_date >= year_start.strftime("%Y-%m-%d")`,
  `bst_date < year_end.strftime("%Y-%m-%d")`.
- **Returns**: List of rows:
  `(notifications_sent, billable_units, rate, notification_type)` ordered by type, rate.
- **Notes**: For email/letter, `billable_units = notifications_sent` (rate multiplier not
  applicable). For SMS, `billable_units = sum(billable_units × rate_multiplier)`.

---

#### `fetch_monthly_billing_for_year(service_id, year)`

- **Purpose**: Month-by-month billing breakdown for a service across a financial year.
- **Query type**: Two queries (email+letter UNION ALL sms) executed immediately; if the
  financial year is still in progress, today and yesterday are refreshed via
  `fetch_billing_data_for_day` + `update_fact_billing` before querying.
- **Key filters/conditions**: `service_id`, `bst_date` in financial year bounds
  (inclusive end for monthly).
- **Returns**: List of rows:
  `(month, notifications_sent, billable_units, rate, notification_type, postage)`.

---

#### `delete_billing_data_for_service_for_day(process_day, service_id)`

- **Purpose**: Remove all ft_billing rows for a service on a specific BST date (used
  before a rebuild).
- **Query type**: DELETE.
- **Key filters/conditions**: `bst_date == process_day` AND `service_id == service_id`.
- **Returns**: Integer row count deleted.

---

#### `fetch_billing_data_for_day(process_day, service_id=None)`

- **Purpose**: Read raw notification data for a calendar day and transform it into
  `FactBilling`-shaped transit objects.
- **Query type**: SELECT (read-only); iterates over all services if `service_id` is
  None, otherwise scopes to one service.
- **Key filters/conditions**: `created_at` in `[start_of_bst_day_UTC, end_of_bst_day_UTC)`;
  only `SMS_TYPE` is processed; falls back from `Notification` to `NotificationHistory`
  when the primary query returns no rows.
- **Returns**: List of result namedtuples (one per aggregation group).
- **Notes**: Only SMS notifications are currently aggregated; email and letter types are
  omitted from this function.

---

#### `_query_for_billing_data(table, notification_type, start_date, end_date, service_id)` _(internal)_

- **Purpose**: Shared query logic for `fetch_billing_data_for_day` against either
  `Notification` or `NotificationHistory`.
- **Query type**: SELECT + aggregation.
- **Key filters/conditions**: `status IN NOTIFICATION_STATUS_TYPES_BILLABLE`;
  `key_type != KEY_TYPE_TEST`; `created_at` window; `notification_type` match; `service_id`.
- **Returns**: Aggregated rows grouped by
  `(template_id, service_id, notification_type, sent_by, letter_page_count,
  rate_multiplier, international, crown, postage, sms_sending_vehicle)`.
- **Notes**: Computes `sms_sending_vehicle` via a CASE expression (see
  [SMS Sending Vehicle](#sms-sending-vehicle) under Domain Rules).

---

#### `get_rates_for_billing()`

- **Purpose**: Load entire `Rate` and `LetterRate` tables for in-memory rate lookup.
- **Query type**: SELECT.
- **Returns**: `(non_letter_rates: list[Rate], letter_rates: list[LetterRate])`, each
  sorted descending by `valid_from` / `start_date` so `next()` iteration yields the
  most recent applicable rate first.

---

#### `get_service_ids_that_need_billing_populated(start_date, end_date)`

- **Purpose**: Identify services that have at least one historically billable
  notification in a date range.
- **Query type**: SELECT DISTINCT.
- **Key filters/conditions**: `NotificationHistory.created_at` in range;
  `notification_type IN [SMS, EMAIL, LETTER]`; `billable_units != 0`.
- **Returns**: List of `(service_id,)` tuples.

---

#### `get_rate(non_letter_rates, letter_rates, notification_type, date, crown, letter_page_count, post_class, sms_sending_vehicle)`

- **Purpose**: Look up the applicable rate for a notification from pre-loaded rate
  lists (pure in-memory, no DB call).
- **Key filters/conditions**:
  - Letter: `start_of_day >= r.start_date AND crown == r.crown AND
    letter_page_count == r.sheet_count AND post_class == r.post_class`.
  - SMS: `notification_type match AND start_of_day >= r.valid_from AND
    sms_sending_vehicle == r.sms_sending_vehicle`.
  - Email: always returns `0`.
- **Returns**: Decimal/float rate or `0`.
- **Notes**: If `letter_page_count == 0`, returns `0` immediately. If no SMS rate
  matches, raises `ValueError` with prefix `[error-sms-rates]`.

---

#### `update_fact_billing(data, process_day)`

- **Purpose**: Compute the rate for a transit-data row and upsert it into `ft_billing`.
- **Query type**: PostgreSQL `INSERT … ON CONFLICT DO UPDATE` on `ft_billing_pkey`.
- **Key filters/conditions**: Conflict key is `ft_billing_pkey` (composite primary key).
  On conflict: updates `notifications_sent`, `billable_units`, `billing_total`,
  `updated_at`.
- **Returns**: Nothing (commits immediately).
- **Notes**: Calls `get_rates_for_billing()` + `get_rate()` on every invocation;
  delegates record construction to `create_billing_record`.

---

#### `create_billing_record(data, rate, process_day)`

- **Purpose**: Construct a `FactBilling` ORM instance with a pre-computed
  `billing_total`.
- **Query type**: No DB access.
- **Returns**: `FactBilling` instance (not persisted).
- **Notes**: `billing_total` formula for SMS: `billable_units × rate_multiplier × rate`.
  Converts float rates through `str()` before `Decimal()` conversion to avoid
  floating-point precision drift. If `rate is None`, `billing_total = 0`.

---

### `fact_notification_status_dao.py`

#### `fetch_notification_status_for_day(process_day, service_ids=None)`

- **Purpose**: Collect raw notification status data for a calendar day, ready to be
  written to `ft_notification_status`.
- **Query type**: SELECT (read-only); iterates all services × all 3 notification types.
- **Key filters/conditions**: `created_at` in `[start_of_day, end_of_day)`; falls back
  from `Notification` to `NotificationHistory` when primary query returns nothing.
- **Returns**: Combined list of aggregated rows across all services and types.

---

#### `query_for_fact_status_data(table, start_date, end_date, notification_type, service_id)` _(internal)_

- **Purpose**: Shared aggregation query for `fetch_notification_status_for_day`.
- **Query type**: SELECT + group by.
- **Key filters/conditions**: `created_at` window; `notification_type`; `service_id`;
  `key_type != KEY_TYPE_TEST`.
- **Returns**: Rows of
  `(template_id, service_id, job_id, notification_type, key_type, status,
  notification_count, billable_units)`. `job_id` defaults to the nil UUID when NULL.

---

#### `update_fact_notification_status(data, process_day, service_ids=None)`

- **Purpose**: Replace `ft_notification_status` rows for a given day (and optionally a
  subset of services) with freshly computed data.
- **Query type**: DELETE then bulk INSERT; committed in one operation.
- **Key filters/conditions**: DELETE: `bst_date == process_day`; if `service_ids`
  supplied, also filters `service_id IN service_ids`.
- **Returns**: Nothing.
- **Notes**: Unlike `update_fact_billing`, this is **not** an upsert — it deletes all
  rows for the day first, then bulk-inserts. On `IntegrityError` the session is rolled
  back and the error re-raised. On any other exception the session is also rolled back
  and re-raised.

---

#### `fetch_notification_status_for_service_by_month(start_date, end_date, service_id)`

- **Purpose**: Monthly notification status counts from `ft_notification_status` for a
  service.
- **Query type**: SELECT + aggregation.
- **Key filters/conditions**: `service_id`; `bst_date` range (exclusive of today's date);
  `key_type != KEY_TYPE_TEST`.
- **Returns**: Rows of `(month, notification_type, notification_status, count)`.

---

#### `fetch_delivered_notification_stats_by_month(filter_heartbeats=None)`

- **Purpose**: Platform-wide delivered/sent counts aggregated by month and type (used
  for public dashboard stats).
- **Query type**: SELECT from `monthly_notification_stats_summary`.
- **Key filters/conditions**: `month >= "2019-11-01"` (GC Notify launch date);
  optionally excludes `NOTIFY_SERVICE_ID` and `HEARTBEAT_SERVICE_ID`.
- **Returns**: Rows of `(month, notification_type, count)` ordered by month desc.
- **Notes**: This query was migrated from `ft_notification_status` (28M+ rows) to the
  pre-aggregated `monthly_notification_stats_summary` table for performance. Only
  `DELIVERED` and `SENT` statuses are stored in the summary table.

---

#### `fetch_notification_stats_for_trial_services()`

- **Purpose**: Report on all delivered/sent notifications from services still in trial
  mode, with creator details.
- **Query type**: SELECT with multiple joins.
- **Key filters/conditions**: `ServiceHistory.version == 1` (creation record);
  `Service.restricted == True`; `notification_status IN [DELIVERED, SENT]`.
- **Returns**: Rows of
  `(service_id, service_name, creation_date, user_name, user_email,
  notification_type, notification_sum)`.

---

#### `fetch_notification_status_for_service_for_day(bst_day, service_id)`

- **Purpose**: Intraday notification counts from the live `Notification` table (today
  only).
- **Query type**: SELECT from `notifications`.
- **Key filters/conditions**: `created_at` in BST day window; `service_id`;
  `key_type != KEY_TYPE_TEST`.
- **Returns**: Rows of `(month, notification_type, notification_status, count)` (month
  is always the first of the current month as a `DateTime` literal).

---

#### `fetch_billable_units_for_service_for_day(bst_day, service_id)`

- **Purpose**: Intraday SMS billable-unit totals from the live `Notification` table.
- **Query type**: SELECT from `notifications`.
- **Key filters/conditions**: Same day window + service + key type; additionally
  `notification_type == SMS_TYPE`.
- **Returns**: Rows of `(month, notification_type, notification_status, count)` where
  `count` is `sum(billable_units)`.

---

#### `fetch_notification_status_for_emails_service_for_day(bst_day, service_id)`

- **Purpose**: Intraday email notification counts from the live `Notification` table.
- **Query type**: SELECT from `notifications`.
- **Key filters/conditions**: Same day window + service + key type;
  `notification_type == EMAIL_TYPE`.
- **Returns**: Rows of `(month, notification_type, notification_status, count)`.

---

#### `fetch_notification_status_for_service_for_today_and_7_previous_days(service_id, by_template, limit_days, notification_type)`

- **Purpose**: Dashboard view — notification status counts for up to 7 days, combining
  historical facts with live data for the current partial day.
- **Query type**: `ft_notification_status` for historical days UNION ALL `notifications`
  for today; results aggregated in subquery.
- **Key filters/conditions**: Historical cutoff determined by retention period;
  live-table cutoff is `max(bst_date in ft_notification_status) + 1 day`. Test keys
  excluded.
- **Returns**: Rows of `(notification_type, status, count)` optionally with
  `(template_name, is_precompiled_letter, template_id)` when `by_template=True`.
- **Notes**: The split point between the two data sources is computed dynamically by
  `_timing_notification_table`; only the last 10 days of `ft_notification_status` are
  scanned for the max-date calculation.

---

#### `fetch_notification_billable_units_for_service_for_today_and_7_previous_days(service_id, by_template, limit_days)`

- **Purpose**: Same hybrid approach as above but also returns `billable_units` per row.
- **Query type**: UNION ALL of `ft_notification_status` (with SUM aggregation) and live
  `notifications`.
- **Returns**: Rows of `(notification_type, status, count, billable_units)`.

---

#### `get_total_notifications_sent_for_api_key(api_key_id)`

- **Purpose**: Count all notifications ever sent via a specific API key, by type.
- **Query type**: SELECT from live `notifications` table only.
- **Key filters/conditions**: `api_key_id` match.
- **Returns**: Rows of `(notification_type, total_send_attempts)`.

---

#### `get_last_send_for_api_key(api_key_id)`

- **Purpose**: Return the timestamp of the most recent notification sent via an API key.
- **Query type**: SELECT from `api_keys` table (`last_used_timestamp`); no fallback to `notifications`.
- **Key filters/conditions**: `api_key_id` match.
- **Returns**: Non-empty list with a single row `[(last_used_timestamp,)]` if `last_used_timestamp` is set; empty list if `null`.
- **Notes**: Does not query the notifications table; relies on the `api_keys.last_used_timestamp` column being kept up to date.

---

#### `get_api_key_ranked_by_notifications_created(n_days_back)`

- **Purpose**: Rank API keys by total notification volume over the last `n_days_back` days (top 50).
- **Query type**: Two-level subquery aggregation then JOIN to `api_keys` and `services`.
- **Key filters/conditions**: `created_at >= (utcnow - n_days_back days)`; `api_key_id IS NOT NULL`; `key_type = KEY_TYPE_NORMAL`.
- **Returns**: Rows of `(api_key_name, key_type, service_name, api_key_id, service_id, last_notification_created, email_notifications, sms_notifications, total_notifications)`, ordered by `total_notifications DESC`, limited to 50.

---

#### `fetch_notification_status_totals_for_all_services(start_date, end_date)`

- **Purpose**: Cross-service notification status totals for a date range; used by platform stats endpoints.
- **Query type**: SELECT from `ft_notification_status` aggregated by `(notification_type, status, key_type)`. If today falls within the range, adds a live UNION ALL from `notifications` for the current partial day.
- **Key filters/conditions**: `bst_date` between `start_date` and `end_date`.
- **Returns**: Rows of `(notification_type, status, key_type, count)` ordered by `notification_type`.

---

#### `fetch_stats_for_all_services_by_date_range(start_date, end_date, include_from_test_key=True)`

- **Purpose**: Per-service, per-type, per-status notification counts for a date range; includes service metadata.
- **Query type**: SELECT from `ft_notification_status` JOIN `services`; if today is in range, UNION ALL with live `notifications` for today.
- **Key filters/conditions**: `bst_date` between `start_date` and `end_date`. `include_from_test_key=False` adds `key_type != TEST` filters to both branches.
- **Returns**: Rows of `(service_id, name, restricted, research_mode, active, created_at, notification_type, status, count)` ordered by `(name, notification_type, status)`.

---

#### `fetch_notification_statuses_for_job(job_id)`

- **Purpose**: Per-status notification counts for a single job from the fact table.
- **Query type**: SELECT from `ft_notification_status` grouped by `notification_status`.
- **Key filters/conditions**: `job_id` match.
- **Returns**: Rows of `(status, count)`.

---

#### `fetch_notification_statuses_for_job_batch(service_id, job_ids)`

- **Purpose**: Per-status notification counts for multiple jobs from the fact table (used for old job-list stats).
- **Query type**: SELECT from `ft_notification_status` grouped by `(job_id, notification_status)`.
- **Key filters/conditions**: `service_id` + `job_id IN job_ids`.
- **Returns**: Rows of `(job_id, status, count)`.
- **Notes**: Counterpart to `dao_get_notification_outcomes_for_job_batch` which reads from the live `notifications` table. This function is used for jobs older than ~3 days.

---

#### `fetch_monthly_template_usage_for_service(start_date, end_date, service_id)`

- **Purpose**: Monthly template usage counts for a service, used by the "monthly statistics" admin view.
- **Query type**: SELECT from `ft_notification_status` JOIN `templates`; if today is in range, UNION ALL with live `notifications`.
- **Key filters/conditions**: `service_id`; `bst_date` range; `key_type != TEST`; `notification_status != CANCELLED`.
- **Returns**: Rows of `(template_id, name, template_type, is_precompiled_letter, month, year, count)` ordered by `(year, month, name)`.

---

#### `fetch_monthly_notification_statuses_per_service(start_date, end_date)`

- **Purpose**: Cross-service monthly breakdown by status category (sending, delivered, failures) for the "live service report".
- **Query type**: SELECT from `ft_notification_status` JOIN `services` with conditional SUM aggregations.
- **Key filters/conditions**: `bst_date` range; `notification_status != CREATED`; `Service.active`; `key_type != TEST`; `research_mode == False`; `restricted == False`.
- **Returns**: Rows of `(date_created, service_id, service_name, notification_type, count_sending, count_delivered, count_technical_failure, count_temporary_failure, count_permanent_failure, count_sent)` ordered by `(date_created, service_id, notification_type)`.

---

#### `fetch_quarter_data(start_date, end_date, service_ids)`

- **Purpose**: Total notification counts per service per type within a quarter window.
- **Query type**: SELECT from `ft_notification_status` grouped by `(service_id, notification_type)`.
- **Key filters/conditions**: `service_id IN service_ids`; `bst_date` between `start_date` and `end_date`.
- **Returns**: Rows of `(service_id, notification_type, notification_count)`.

---

#### `fetch_notification_status_totals_for_service_by_fiscal_year(service_id, fiscal_year, notification_type=None)`

- **Purpose**: Total notification count for a service over a full fiscal year.
- **Query type**: SELECT SUM from `ft_notification_status` for fiscal year date range.
- **Key filters/conditions**: `service_id`; `bst_date` in `[start_date, end_date]` from `get_fiscal_dates(fiscal_year)`; optional `notification_type` filter.
- **Returns**: Integer count (0 if no rows).

---

#### `fetch_billable_units_totals_for_service_by_fiscal_year(service_id, fiscal_year, notification_type=None)`

- **Purpose**: Total billable units for a service over a full fiscal year (SMS-focused).
- **Query type**: SELECT SUM(`billable_units`) from `ft_notification_status` for fiscal year date range.
- **Key filters/conditions**: Same as `fetch_notification_status_totals_for_service_by_fiscal_year`.
- **Returns**: Integer billable unit total (0 if no rows).

---

#### `get_total_sent_notifications_for_day_and_type(day, notification_type)`

- **Purpose**: Total notification count for a given day and type (non-test keys only).
- **Query type**: SELECT SUM from `ft_notification_status`.
- **Key filters/conditions**: `notification_type`; `key_type != TEST`; `bst_date == day`.
- **Returns**: Integer count (0 if no rows / `null` sum).

---

### `provider_rates_dao.py`

#### `create_provider_rates(provider_identifier, valid_from, rate)`

- **Purpose**: Record a new provider-level rate effective from a given timestamp.
- **Query type**: INSERT (within a transaction).
- **Key filters/conditions**: Looks up the `ProviderDetails` row by `identifier`
  using `.one()` (raises if not found or ambiguous).
- **Returns**: Nothing (the new `ProviderRates` instance is added to the session).
- **Notes**: Decorated `@transactional`. This rate is stored in `provider_rates` and is
  distinct from the `rates` table used in `get_rates_for_billing()`.

---

## Domain Rules & Invariants

### Financial Year

- The financial year runs **April 1 → March 31** (inclusive).
- The start boundary is April 1 at 00:00 in the configured local timezone
  (`TIMEZONE` env var, defaulting to `America/Toronto`), converted to UTC.  
  Example: April 1, 00:00 EDT = April 1, 04:00 UTC.
- `get_current_financial_year_start_year()` returns the **calendar year** in which the
  current financial year started (e.g. during Jan 2026 → returns 2025).
- Date comparisons in `ft_billing` and `ft_notification_status` queries use
  `bst_date` (DATE column, local timezone) not UTC timestamps.

### Annual Billing (`annual_billing` table)

- Tracks one row per `(service_id, financial_year_start)` containing
  `free_sms_fragment_limit`.
- Only SMS has a free annual allowance; email and letter costs are not offset here.
- **Back-fill rule**: If no record exists for the requested year, the REST layer:
  - For a **past** year → returns the oldest entry on record (no new row created).
  - For the **current or future** year → copies the newest entry's limit, persists it,
    and returns it.
- **Forward-propagation rule**: When the limit is updated for the current year or later,
  `dao_update_annual_billing_for_future_years` updates all *already-existing* future-year
  rows with the new limit.  Years not yet created are unaffected until they are
  first created.

### Annual Limits (`annual_limits_data` table)

- Quarters follow the fiscal year: Q1=Apr–Jun, Q2=Jul–Sep, Q3=Oct–Dec, Q4=Jan–Mar.
- Each row stores a snapshot of `annual_email_limit` and `annual_sms_limit` alongside
  the count, preserving the limit values that were in effect when the quarter was
  processed.
- Upsert conflict key: `(service_id, time_period, notification_type)`.
- Cumulative limit checks are performed by `fetch_quarter_cummulative_stats` which sums
  counts across all completed quarters of the fiscal year.

### Fact Billing (`ft_billing` / `FactBilling`)

- One row per `(bst_date, template_id, service_id, provider, rate_multiplier,
  notification_type, international, postage, sms_sending_vehicle)`.
- **Only SMS** notifications are sourced from `fetch_billing_data_for_day`; email and
  letter rows must be populated by other means (the billing report queries do read email
  and letter rows when they exist).
- Notifications sent with `KEY_TYPE_TEST` keys are **never** included in billable data.
- Notifications with status not in `NOTIFICATION_STATUS_TYPES_BILLABLE` are excluded.
- Populated via day-level processing: data is read from `Notification` first; if empty
  (purge after ~7 days), `NotificationHistory` is used.
- `billing_total` (SMS) = `billable_units × rate_multiplier × rate`.
- Upsert conflict key: `ft_billing_pkey`; on conflict the counts and total are
  overwritten, `updated_at` is refreshed.

### SMS Sending Vehicle

Determines which rate row applies for an SMS notification:

| Condition | Vehicle |
|---|---|
| `international == True` | `long_code` |
| Origination number present and matches `LONG_CODE_REGEX` | `long_code` |
| Origination number present and does **not** match `LONG_CODE_REGEX` | `short_code` |
| No origination number | `TemplateCategory.sms_sending_vehicle` (fallback: `long_code`) |
| Default (none matched) | `long_code` |

### Billable Units by Channel

| Channel | Billable unit | Cost formula |
|---|---|---|
| SMS | Fragment count × rate_multiplier | `billable_units × rate_multiplier × rate` |
| Letter | Sheet count (page pairs) | `notifications_sent × rate` (rate varies by sheets + postage class + crown) |
| Email | Not applicable | Always `0` |

### Fact Notification Status (`ft_notification_status` / `FactNotificationStatus`)

- One row per `(bst_date, template_id, service_id, job_id, notification_type,
  key_type, notification_status)` holding `notification_count` and `billable_units`.
- `job_id` is stored as the nil UUID (`00000000-0000-0000-0000-000000000000`) when
  the notification has no associated job.
- Populated by **delete-then-bulk-insert** (not upsert): all rows for the target
  `bst_date` (and optional service scope) are deleted before new rows are inserted.
- The `monthly_notification_stats_summary` table caches the aggregate of delivered/sent
  statuses by month for performance (replaces direct `ft_notification_status` scans for
  platform-wide stats).
- The live-data hybrid queries (7-day dashboard) use a dynamic cutover point: the
  highest `bst_date` in `ft_notification_status` within the last 10 days + 1 day
  determines from when the live `Notification` table is queried.

### Provider Rates (`provider_rates` table)

- Provider rates are linked to specific `ProviderDetails` records (by identifier
  string) and have an effective `valid_from` timestamp.
- These are separate from the `rates` table used for billing calculations.
  The `rates` table is used by `get_rates_for_billing()` / `get_rate()` for SMS/email;
  `LetterRate` is used for letters. `provider_rates` appears to be tracked for
  auditing/reference but is not consumed by the active billing path.

---

## Error Conditions

| Raised by | Exception type | Condition |
|---|---|---|
| `billing/rest.py: get_yearly_usage_by_monthly_from_ft_billing` | HTTP 400 (`TypeError`) | `year` query param is absent or not castable to `int` |
| `billing/rest.py: get_yearly_billing_usage_summary_from_ft_billing` | HTTP 400 (`TypeError`) | Same as above |
| `billing/rest.py: get_free_sms_fragment_limit` | `InvalidRequest` (→ HTTP 404) | No `annual_billing` rows at all for the service |
| `fact_billing_dao.get_rate` | `ValueError` with prefix `[error-sms-rates]` | No SMS rate matches `(sms_sending_vehicle, valid_from ≤ date)` |
| `fact_billing_dao.get_rate` | `StopIteration` (implicit, via `next()`) | No letter rate matches `(crown, sheet_count, post_class, start_date ≤ date)` |
| `fact_notification_status_dao.update_fact_notification_status` | `IntegrityError` (re-raised after rollback) | Duplicate key or constraint violation during bulk insert |
| `fact_notification_status_dao.update_fact_notification_status` | Any `Exception` (re-raised after rollback) | Any other DB error during bulk insert |
| `provider_rates_dao.create_provider_rates` | `NoResultFound` / `MultipleResultsFound` (via `.one()`) | Provider identifier not found or matches multiple rows in `provider_details` |

---

## Query Inventory (for sqlc)

| Query name | Type | Tables | Description |
|---|---|---|---|
| `GetFreeSmsFragmentLimitForYear` | SELECT ONE | `annual_billing` | Free SMS limit for a service/year; defaults to current year |
| `GetAllFreeSmsFragmentLimits` | SELECT MANY | `annual_billing` | All annual billing rows for a service, ordered by year |
| `UpsertAnnualBillingForYear` | UPSERT | `annual_billing` | Create or update free SMS limit for a service/year |
| `UpdateAnnualBillingForFutureYears` | UPDATE | `annual_billing` | Set new SMS limit on all rows where year > given year |
| `InsertQuarterData` | UPSERT | `annual_limits_data` | Insert/update notification count for a quarter (conflict on service/period/type) |
| `FetchQuarterCumulativeStats` | SELECT MANY | `annual_limits_data` | Cumulative notification counts per service across a list of quarters |
| `FetchSmsAllowanceRemainder` | SELECT MANY | `annual_billing`, `ft_billing` | Per-service free SMS fragment remainder at a given date |
| `FetchSmsBillingForAllServices` | SELECT MANY | `services`, `annual_billing`, `ft_billing`, `organisations` | SMS costs per service for an admin date range report |
| `FetchLetterCostsForAllServices` | SELECT MANY | `services`, `ft_billing`, `organisations` | Letter total costs per service for a date range |
| `FetchLetterLineItemsForAllServices` | SELECT MANY | `services`, `ft_billing`, `organisations` | Letter line items (by rate + postage) per service for a date range |
| `FetchBillingTotalsForYear` | SELECT MANY | `ft_billing` | Annual billing totals for a service, split by type and rate |
| `FetchMonthlyBillingForYear` | SELECT MANY | `ft_billing` | Month-by-month billing for a service within a financial year |
| `DeleteBillingDataForServiceForDay` | DELETE | `ft_billing` | Remove all ft_billing rows for a service on a bst_date |
| `FetchBillingDataForDay` | SELECT MANY | `notifications` / `notification_history`, `services`, `templates`, `template_categories` | Raw SMS billing aggregates for a BST day (with NotificationHistory fallback) |
| `GetServiceIdsNeedingBillingPopulated` | SELECT DISTINCT | `notification_history` | Services with non-zero billable units in a date range |
| `GetAllRatesForBilling` | SELECT MANY | `rates`, `letter_rates` | Full rate tables loaded for in-memory lookup |
| `UpsertFactBilling` | UPSERT | `ft_billing` | Insert or update a billing row; conflict on `ft_billing_pkey` |
| `FetchNotificationStatusForDay` | SELECT MANY | `notifications` / `notification_history` | All notification status counts for a BST day (all types, all services) |
| `DeleteFactNotificationStatusForDay` | DELETE | `ft_notification_status` | Remove rows for a date (and optional service list) before re-population |
| `BulkInsertFactNotificationStatus` | INSERT MANY | `ft_notification_status` | Bulk-insert replacement rows |
| `FetchNotificationStatusByMonth` | SELECT MANY | `ft_notification_status` | Monthly status counts for a service in a date range |
| `FetchDeliveredStatsByMonth` | SELECT MANY | `monthly_notification_stats_summary` | Platform-wide delivered/sent counts per month |
| `FetchTrialServiceNotificationStats` | SELECT MANY | `ft_notification_status`, `services`, `service_history`, `users` | Delivered/sent totals for restricted (trial) services with creator info |
| `FetchNotificationStatusForServiceToday` | SELECT MANY | `notifications` | Intraday notification counts for a service (all types) |
| `FetchSmsUnitsForServiceToday` | SELECT MANY | `notifications` | Intraday SMS billable units for a service |
| `FetchEmailStatusForServiceToday` | SELECT MANY | `notifications` | Intraday email notification counts for a service |
| `FetchNotificationStatusLast7Days` | SELECT MANY | `ft_notification_status` (UNION ALL `notifications`) | 7-day status counts hybrid query for service dashboard |
| `FetchBillableUnitsLast7Days` | SELECT MANY | `ft_notification_status` (UNION ALL `notifications`) | 7-day billable unit counts hybrid query |
| `GetTotalSentByApiKey` | SELECT MANY | `notifications` | Total send attempts per notification type for an API key |
| `CreateProviderRate` | INSERT | `provider_rates`, `provider_details` | Record a new rate for a provider effective from a given timestamp |
