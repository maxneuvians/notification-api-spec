## 1. Billing Repository Layer (DAO)

- [ ] 1.1 Implement `dao_create_or_update_annual_billing_for_year(service_id, free_sms_fragment_limit, financial_year_start)`: upsert `annual_billing`; mark `@transactional`; write unit test for insert and for update on existing row
- [ ] 1.2 Implement `dao_update_annual_billing_for_future_years(service_id, limit, year)`: bulk UPDATE rows where `financial_year_start > year`; write unit test confirming only future rows are modified, current-year row unchanged
- [ ] 1.3 Implement `dao_get_free_sms_fragment_limit_for_year(service_id, year)`: return `AnnualBilling` or None; test exact-match, test no-record-returns-None, test fallback lookup (caller responsibility)
- [ ] 1.4 Implement `fetch_billing_data_for_day(date, service_id)`: UNION `notifications` + `notification_history`; apply BST timezone window; filter to billable statuses (`delivered`, `sending`, `temporary-failure`); exclude test keys; group by `(template, service, sent_by, rate_multiplier, international, notification_type, sms_sending_vehicle)`
- [ ] 1.5 Implement `update_fact_billing(ft_billing)`: INSERT … ON CONFLICT DO NOTHING; write idempotency test confirming existing row `rate` and `billing_total` are unchanged after rerun with a different rate
- [ ] 1.6 Implement SMS sending vehicle classification helper: `+1` + 10 digits → `long_code`; other non-null → `short_code`; NULL → template category default; international flag → `long_code`; write unit tests covering all 4 branches
- [ ] 1.7 Implement `get_rates_for_billing()` returning `(non_letter_rates, letter_rates)` differentiated by `sms_sending_vehicle`; implement `get_rate(process_date, vehicle, …)` selecting most recent rate where `start_date <= process_date`; log `[error-sms-rates]` and raise ValueError on missing rate
- [ ] 1.8 Implement `fetch_monthly_billing_for_year(year)` grouped by `(month, notification_type, rate, postage)`; write unit test with multi-month data
- [ ] 1.9 Implement `fetch_billing_totals_for_year(year)` grouped by `(notification_type, rate)`; write unit test

## 2. Notification Status Repository Layer (DAO)

- [ ] 2.1 Implement `update_fact_notification_status(process_day, service_ids)`: DELETE existing rows for `(day, service_ids)`; INSERT fresh aggregates from UNION of `notifications` + `notification_history`; write idempotency test (run twice, same row count)
- [ ] 2.2 Write test confirming `update_fact_notification_status` reads from both `notifications` and `notification_history` (mock both sources independently)
- [ ] 2.3 Implement BST timezone boundary conversion helper (`Europe/London` midnight-to-midnight interval from date string); write unit tests for a UTC date, a BST date, and a clock-change boundary date

## 3. REST Endpoints

- [ ] 3.1 Implement `POST /service/{service_id}/billing/free-sms-fragment-limit`: parse body; if no `financial_year_start` default to current fiscal year and call `dao_update_annual_billing_for_future_years`; call `dao_create_or_update_annual_billing_for_year`; return 201; test missing body → 400 with `"JSON"` in message
- [ ] 3.2 Implement `GET /service/{service_id}/billing/free-sms-fragment-limit`: accept optional `financial_year_start` query param; auto-create from most recent prior year when current year has no record; fall back to most recent prior year when requested year not found; return 200 `{financial_year_start, free_sms_fragment_limit}`
- [ ] 3.3 Implement `GET /service/{service_id}/billing/ft-monthly-usage?year=YYYY`: auto-populate `FactBilling` if no rows for year (persisted); call `fetch_monthly_billing_for_year`; exclude email rows; return 200 array; test email exclusion and SMS rate_multiplier billing_units
- [ ] 3.4 Implement `GET /service/{service_id}/billing/ft-yearly-usage-summary?year=YYYY`: validate `year` param (400 with `{"message": "No valid year provided", "result": "error"}` when absent); call `fetch_billing_totals_for_year`; sort by `notification_type`; compute `letter_total`; return 200 or 200 `[]` on no data
- [ ] 3.5 Write auth tests for all 4 billing endpoints: each returns 401 when authorization header is absent

## 4. Nightly Billing Worker

- [ ] 4.1 Implement `create_nightly_billing`: dispatch 4 `create_nightly_billing_for_day` subtasks; without `day_start` use yesterday/−2/−3/−4; with `day_start` use given date/−1/−2/−3; test that exactly 4 `apply_async` calls are made with correct `process_day` kwargs for both paths
- [ ] 4.2 Implement `create_nightly_billing_for_day`: call `fetch_billing_data_for_day`; merge rows sharing the same rate_multiplier tuple; call `update_fact_billing` per row; store null provider as `'unknown'`; compute and persist `billing_total = billable_units × rate_multiplier × rate`
- [ ] 4.3 Write rate-multiplier merging test: same multiplier on same service+template+provider → 1 `FactBilling` row; two distinct multipliers → 2 separate rows
- [ ] 4.4 Write idempotency test for `create_nightly_billing_for_day`: insert a `FactBilling` row, then change the rate, re-run; verify original rate and `billing_total` are unchanged in the stored record

## 5. Nightly Notification Status Worker

- [ ] 5.1 Implement `create_nightly_notification_status`: dispatch 4 `create_nightly_notification_status_for_day` subtasks using identical 4-day window logic as `create_nightly_billing`; write fan-out count test
- [ ] 5.2 Implement `create_nightly_notification_status_for_day`: call `update_fact_notification_status`; copy `billable_units` from source notification; for each affected service, clear all Redis annual limit hash fields to 0 via `annual_limit_client`
- [ ] 5.3 Write Redis clear test: after task run, verify all hash values for affected services are set to 0 (mock `annual_limit_client`)
- [ ] 5.4 Write large-batch Redis clear test: 39 services (exceeds chunk size); verify all 39 services are cleared without omission across multiple batches
- [ ] 5.5 Write test confirming both `notifications` and `notification_history` are queried by the per-day worker

## 6. Quarterly Workers and Beat Schedule (C6 Fix)

- [ ] 6.1 Implement `insert_quarter_data_for_annual_limits`: call `get_previous_quarter(datetime)` to get quarter date range; aggregate `FactNotificationStatus` for that range; upsert into `AnnualLimitsData` with REPLACE (not accumulate) semantics on conflict
- [ ] 6.2 Implement `get_previous_quarter` with all 4 mappings: Jan–Mar → Q3-{prev_year} (Oct–Dec); Apr–Jun → Q4-{prev_fy} (Jan–Mar); Jul–Sep → Q1-{cur_year} (Apr–Jun); Oct–Dec → Q2-{cur_year} (Jul–Sep); write unit tests for one date in each of the 4 cases
- [ ] 6.3 Implement `send_quarter_email`: build bilingual (EN/FR) markdown body with sent count, annual limit, percentage used, and `## {service_name}` headings; delegate to `send_annual_usage_data(user_id, fy_start, fy_end, markdown_en, markdown_fr)`
- [ ] 6.4 Register all 4 `insert-quarter-data` beat entries in the cron scheduler (Q1 Jul, Q2 Oct, Q3 Jan, Q4 Apr)
- [ ] 6.5 Register all 4 `send-quarterly-email` beat entries: `0 23 2 7 *` (Q1), `0 23 2 10 *` (Q2), `0 23 2 1 *` (Q3), **`0 23 2 4 *` (Q4 — C6 fix, was missing in Python)**; write test asserting all 4 cron expressions are present in the registered scheduler entries
- [ ] 6.6 Write upsert-replace test for `insert_quarter_data_for_annual_limits`: run twice for the same quarter with different aggregated counts; verify `AnnualLimitsData.count` equals the second run's value, not the sum of both

## 7. Monthly Notification Stats Summary Worker

- [ ] 7.1 Implement `create_monthly_notification_stats_summary`: query `FactNotificationStatus` for current + previous month; filter to `delivered` and `sent` statuses; exclude test keys; upsert into `MonthlyNotificationStatsSummary` (overwrite count, update `updated_at`)
- [ ] 7.2 Write status filter test: insert rows with `failed`, `pending`, `technical-failure` statuses; verify none appear in `MonthlyNotificationStatsSummary` after worker run
- [ ] 7.3 Write re-run test: run worker twice for the same months with different underlying counts; verify `MonthlyNotificationStatsSummary.count` reflects the second run and `updated_at` is advanced
