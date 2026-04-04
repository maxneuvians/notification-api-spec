## ADDED Requirements

### Requirement: Free SMS fragment limit — create/update

`POST /service/{service_id}/billing/free-sms-fragment-limit` SHALL create or update the annual SMS fragment limit for a service and return HTTP 201.

#### Scenario: Create limit for current year (no financial_year_start)
- **WHEN** POST body contains `{"free_sms_fragment_limit": 50000}` with no `financial_year_start`
- **THEN** HTTP 201 is returned
- **AND** an `annual_billing` row is upserted for the current financial year with limit 50000
- **AND** all existing `annual_billing` rows for future years for that service are updated to 50000

#### Scenario: Create limit for an explicit past year
- **WHEN** POST body contains `{"free_sms_fragment_limit": 10000, "financial_year_start": 2022}`
- **THEN** HTTP 201 is returned
- **AND** only the 2022 `annual_billing` row is updated
- **AND** future-year rows are NOT modified

#### Scenario: Upsert — no duplicate row created
- **WHEN** POST is called twice for the same service and same financial year
- **THEN** exactly one `annual_billing` row exists for that service+year after both calls

#### Scenario: Missing or empty body returns 400
- **WHEN** POST is called with a missing or empty body
- **THEN** HTTP 400 is returned
- **AND** the error message contains the string `"JSON"`

#### Scenario: Auth required
- **WHEN** POST is called without an authorization header
- **THEN** HTTP 401 is returned

---

### Requirement: Free SMS fragment limit — retrieve

`GET /service/{service_id}/billing/free-sms-fragment-limit` SHALL return the annual SMS fragment limit for a service.

#### Scenario: Get current year limit
- **WHEN** GET is called without `financial_year_start`
- **THEN** HTTP 200 is returned with `{"financial_year_start": <current_year>, "free_sms_fragment_limit": <n>}`

#### Scenario: Auto-create for current year when no record exists
- **WHEN** GET is called for the current year and no `annual_billing` row exists
- **THEN** a new row is auto-created by copying the limit from the most recent prior-year record
- **AND** HTTP 200 is returned with the copied limit

#### Scenario: Year not found falls back to most recent prior year
- **WHEN** GET is called with `financial_year_start=2025` and no 2025 row exists
- **THEN** HTTP 200 is returned with data from the most recent `annual_billing` row whose year is before 2025

#### Scenario: Auth required
- **WHEN** GET is called without an authorization header
- **THEN** HTTP 401 is returned

---

### Requirement: Monthly billing usage endpoint

`GET /service/{service_id}/billing/ft-monthly-usage?year=YYYY` SHALL return monthly aggregated billing data for the service.

#### Scenario: Happy path — returns monthly breakdown
- **WHEN** GET is called with a valid `year` for a service that has billing data
- **THEN** HTTP 200 is returned
- **AND** the response is an array of objects each with fields: `month`, `notification_type`, `billing_units`, `rate`, `postage`
- **AND** rows are grouped by `(month, notification_type, rate, postage)`
- **AND** billing across days within the same month are summed into a single row

#### Scenario: Email notifications excluded
- **WHEN** the service has email notifications for the year
- **THEN** those rows are excluded from the response (email has rate=0 and billable_unit=0)

#### Scenario: Letters with different rates appear as separate rows
- **WHEN** a service sent letters at two different rates in the same month
- **THEN** two separate rows appear for that month with distinct `rate` and `postage` values

#### Scenario: SMS billing_units accounts for rate_multiplier
- **WHEN** SMS notifications have a rate_multiplier > 1
- **THEN** `billing_units` for those rows equals sum(billable_unit × rate_multiplier)

#### Scenario: Auto-populate FactBilling if no rows for year
- **WHEN** GET is called for a year with no existing `ft_billing` rows
- **THEN** `ft_billing` is populated from raw notification data before the response is constructed
- **AND** the populated rows are persisted to the database (not computed transiently)

---

### Requirement: Yearly billing usage summary endpoint

`GET /service/{service_id}/billing/ft-yearly-usage-summary?year=YYYY` SHALL return annual aggregated billing totals.

#### Scenario: Missing year parameter returns 400
- **WHEN** GET is called without a `year` query parameter
- **THEN** HTTP 400 is returned with body `{"message": "No valid year provided", "result": "error"}`

#### Scenario: No data returns empty array
- **WHEN** GET is called for a year that has no billing data
- **THEN** HTTP 200 is returned with `[]`

#### Scenario: Happy path — returns yearly summary sorted by notification_type
- **WHEN** GET is called and data is present for the year
- **THEN** HTTP 200 is returned
- **AND** results are sorted ascending by `notification_type`
- **AND** each row contains: `notification_type`, `billing_units`, `rate`, `letter_total`

#### Scenario: letter_total computed for letters, zero for others
- **WHEN** the result includes letter rows
- **THEN** `letter_total` = `billing_units × rate` for those rows
- **AND** `letter_total` = `0` for SMS and email rows

#### Scenario: Multiple rates per type appear as separate rows
- **WHEN** a service used two different SMS rates in a year
- **THEN** two rows appear for SMS, each with its distinct `rate`

#### Scenario: Auth required
- **WHEN** GET is called without an authorization header
- **THEN** HTTP 401 is returned

---

### Requirement: Nightly billing worker

`create_nightly_billing` SHALL fan out per-day billing aggregation across a 4-day window. `create_nightly_billing_for_day` SHALL write idempotent `FactBilling` rows using BST date boundaries.

#### Scenario: 4-day fan-out without day_start
- **WHEN** `create_nightly_billing` is dispatched without `day_start`
- **THEN** exactly 4 subtasks are dispatched for: yesterday, 2 days ago, 3 days ago, 4 days ago
- **AND** each carries `kwargs={"process_day": "YYYY-MM-DD"}`

#### Scenario: 4-day fan-out with explicit day_start
- **WHEN** `create_nightly_billing` is dispatched with a specific `day_start`
- **THEN** exactly 4 subtasks are dispatched for `day_start` and the 3 preceding days

#### Scenario: Rate multiplier merging — same multiplier collapses to one row
- **WHEN** multiple notifications share the same `(template, service, provider, rate_multiplier)` tuple on the same day
- **THEN** they are collapsed into a single `FactBilling` row

#### Scenario: Different rate multipliers produce separate rows
- **WHEN** notifications for the same service+template on the same day have different `rate_multiplier` values
- **THEN** separate `FactBilling` rows are created, one per distinct multiplier

#### Scenario: Null provider stored as 'unknown'
- **WHEN** a notification has no `sent_by` provider value
- **THEN** the `FactBilling` row stores `'unknown'` as the provider field

#### Scenario: Rate preserved on re-run (idempotent FactBilling)
- **WHEN** `create_nightly_billing_for_day` is re-run after a rate change for the same date
- **THEN** the existing `FactBilling` row retains the original rate (INSERT … ON CONFLICT DO NOTHING)
- **AND** `billing_total` remains unchanged

#### Scenario: BST timezone boundary applied
- **WHEN** the `process_day` falls during BST offset
- **THEN** notifications are queried within the BST-local midnight-to-midnight window for that date, not UTC midnight

---

### Requirement: Nightly notification status worker

`create_nightly_notification_status` SHALL fan out per-day notification status aggregation. `create_nightly_notification_status_for_day` SHALL write idempotent `FactNotificationStatus` rows, read from both notification tables, and clear Redis annual limit counters.

#### Scenario: 4-day fan-out
- **WHEN** `create_nightly_notification_status` is dispatched
- **THEN** exactly 4 subtasks are dispatched using the same 4-day window logic as `create_nightly_billing`

#### Scenario: Reads from both notifications and notification_history
- **WHEN** `create_nightly_notification_status_for_day` runs for a date
- **THEN** it queries both the `notifications` table and the `notification_history` table

#### Scenario: billable_units copied from notification
- **WHEN** a notification is aggregated into `FactNotificationStatus`
- **THEN** the `billable_units` field equals the source notification's own `billable_units` value

#### Scenario: Redis annual limit cleared after persistence
- **WHEN** `FactNotificationStatus` rows have been written for a set of services
- **THEN** for each affected service, all fields in its Redis annual limit hash are set to `0`

#### Scenario: Redis clear handles large batches
- **WHEN** more than the configured chunk size of services are affected (e.g. 39 services)
- **THEN** the Redis clear is applied to all services across multiple batches with no omissions

#### Scenario: BST timezone respected
- **WHEN** the `process_day` spans the BST/UTC boundary
- **THEN** aggregation uses BST-local date boundaries (Europe/London)

---

### Requirement: Quarterly data insert worker

`insert_quarter_data_for_annual_limits` SHALL aggregate `FactNotificationStatus` data for the previous quarter and upsert into `AnnualLimitsData` with replace semantics.

#### Scenario: Determines previous quarter from input datetime
- **WHEN** the worker is given a datetime in Jul–Sep
- **THEN** it computes Q1 of the current fiscal year (Apr 1 – Jun 30)

#### Scenario: Full quarter mapping is correct
- **WHEN** the input datetime is in Jan–Mar
- **THEN** the previous quarter is Q3-{prev_year} covering Oct 1 – Dec 31
- **WHEN** the input datetime is in Apr–Jun
- **THEN** the previous quarter is Q4-{prev_fy} covering Jan 1 – Mar 31
- **WHEN** the input datetime is in Oct–Dec
- **THEN** the previous quarter is Q2-{cur_year} covering Jul 1 – Sep 30

#### Scenario: Upsert replaces existing count
- **WHEN** the worker is run twice for the same quarter
- **THEN** the second run REPLACES the `count` in `AnnualLimitsData` (not adds to it)
- **AND** the resulting count equals the aggregate from the second run only

---

### Requirement: Quarterly email worker and beat schedule (C6 fix)

`send_quarter_email` SHALL send per-service usage emails in English and French. The beat schedule SHALL include all 4 quarterly entries, including `send-quarterly-email-q4` which was missing in the Python implementation.

#### Scenario: Email contains required fields
- **WHEN** `send_quarter_email` is called for a service
- **THEN** the email body contains: sent count, annual limit, and percentage used
- **AND** the body is produced in both English and French
- **AND** service names are formatted as `## {service_name}` markdown headings

#### Scenario: All 4 quarterly email beat entries registered (C6 fix)
- **WHEN** the beat schedule is initialized
- **THEN** all 4 `send-quarterly-email` entries are registered:
  - `send-quarterly-email-q1` at `0 23 2 7 *`
  - `send-quarterly-email-q2` at `0 23 2 10 *`
  - `send-quarterly-email-q3` at `0 23 2 1 *`
  - `send-quarterly-email-q4` at `0 23 2 4 *` ← was missing in Python
- **AND** a test asserts all 4 cron expressions are present in the scheduler

#### Scenario: Q4 entry fires on correct date
- **WHEN** the cron expression `0 23 2 4 *` triggers (April 2 at 23:00 UTC)
- **THEN** the `send-quarterly-email-q4` handler function is invoked

---

### Requirement: Monthly notification stats summary worker

`create_monthly_notification_stats_summary` SHALL aggregate delivered and sent notifications for the current and previous month into `MonthlyNotificationStatsSummary`.

#### Scenario: Only delivered and sent statuses included
- **WHEN** the worker runs and a service has notifications with various statuses
- **THEN** only notifications with status `delivered` or `sent` are counted in the summary

#### Scenario: Test keys excluded
- **WHEN** a service has notifications sent via test keys
- **THEN** those notifications are excluded from the summary counts

#### Scenario: Covers current and previous month only
- **WHEN** the worker runs in April
- **THEN** it aggregates data for April and March
- **AND** February data is not included

#### Scenario: Re-run overwrites existing rows
- **WHEN** the worker is run twice for the same months
- **THEN** the second run overwrites the count in `MonthlyNotificationStatsSummary`
- **AND** `updated_at` is set to the time of the second run

---

### Requirement: SMS sending vehicle classification

The system SHALL classify each SMS notification's sending vehicle based on `sms_origination_phone_number` using a defined priority order.

#### Scenario: +1 with 10-digit number → long_code
- **WHEN** `sms_origination_phone_number` matches `+1` followed by exactly 10 digits
- **THEN** the sending vehicle is classified as `long_code`

#### Scenario: Other non-null value → short_code
- **WHEN** `sms_origination_phone_number` is a non-null string that does not match the +1/10-digit pattern
- **THEN** the sending vehicle is classified as `short_code`

#### Scenario: Null value → template category default
- **WHEN** `sms_origination_phone_number` is NULL
- **THEN** the sending vehicle is determined by the template's category default (typically `long_code`)

#### Scenario: International notifications always → long_code
- **WHEN** the notification is flagged as international
- **THEN** the sending vehicle is `long_code` regardless of `sms_origination_phone_number`

---

### Requirement: Financial year definition and current-year limit cascade

The system SHALL use April 1 – March 31 as the financial year boundary. Updating the current-year SMS fragment limit via POST SHALL cascade to all future-year `annual_billing` rows.

#### Scenario: Financial year for a May date
- **WHEN** the system computes the current financial year for a date in May 2025
- **THEN** the financial year is identified as 2025 (April 1, 2025 – March 31, 2026)

#### Scenario: Financial year for a January date
- **WHEN** the system computes the current financial year for a date in January 2026
- **THEN** the financial year is identified as 2025 (April 1, 2025 – March 31, 2026)

#### Scenario: Current-year POST cascades to future rows
- **WHEN** POST is called without `financial_year_start` (defaults to current year)
- **THEN** `dao_update_annual_billing_for_future_years` is called with the new limit
- **AND** all existing `annual_billing` rows with `financial_year_start > current_year` for that service are updated

#### Scenario: Past-year explicit POST does not cascade
- **WHEN** POST is called with `financial_year_start=2022`
- **THEN** `dao_update_annual_billing_for_future_years` is NOT called
- **AND** only the 2022 `annual_billing` row is modified
