# Business Rules: Platform Admin & Cross-Cutting

## Overview

This document covers the platform-level features that support operational administration, reporting,
and system-wide cross-cutting concerns:

- **Complaints** — SES/provider bounce/complaint callbacks stored and queryable per-service or globally.
- **Reports** — Per-service generated-report tracking (status, retention window).
- **Events** — Append-only audit log for system events.
- **Email branding** — Named branding assets attached to organisations and services.
- **Letter branding** — Named PDF branding assets for letter rendering.
- **Daily sorted letter** — Billing-day–scoped letter sort counts from the print provider.
- **Platform statistics** — Admin aggregate views across all services (notification counts, SMS/letter costs).
- **DAO utilities** — Shared transaction and versioning decorators used across all DAOs.
- **Date utilities** — Financial-year boundaries, UTC-midnight helpers, and retention-period date calculations.
- **Support tooling** — UUID-based entity lookup across notifications/jobs/templates/services/users.
- **Newsletter** — Airtable-backed mailing list: subscribe, confirm, unsubscribe, language preference, send.

---

## Data Access Patterns

### complaint_dao.py

#### `save_complaint(complaint)`
- **Purpose** — Persist a new `Complaint` record received from a provider callback.
- **Query type** — INSERT (via `db.session.add`).
- **Key filters/conditions** — None; caller constructs the object.
- **Returns** — None (committed by `@transactional`).
- **Notes** — Decorated with `@transactional`; commit/rollback is automatic.

#### `fetch_paginated_complaints(page=1)`
- **Purpose** — Return a page of all complaints for admin review.
- **Query type** — SELECT with pagination.
- **Key filters/conditions** — None; all complaints, ordered `created_at DESC`.
- **Returns** — SQLAlchemy `Pagination` object (page size from `PAGE_SIZE` config).
- **Notes** — Uses `joinedload(Complaint.service)` to avoid N+1 on the related service name.

#### `fetch_complaints_by_service(service_id)`
- **Purpose** — Return all complaints for a specific service.
- **Query type** — SELECT filtered by `service_id`.
- **Key filters/conditions** — `service_id = :service_id`, ordered `created_at DESC`.
- **Returns** — List of `Complaint` objects.
- **Notes** — No pagination; returns the full list.

#### `fetch_count_of_complaints(start_date, end_date)`
- **Purpose** — Count complaints in an inclusive date range (used for stats dashboards).
- **Query type** — SELECT COUNT.
- **Key filters/conditions** — `created_at >= midnight(start_date) AND created_at < midnight(end_date + 1 day)`, both expressed as UTC.
- **Returns** — Integer count.
- **Notes** — Dates are converted from local timezone to UTC midnight via `get_local_timezone_midnight_in_utc`; `end_date` is made exclusive by adding one day.
  - **⚠️ Timezone dependency**: the boundary is EST/EDT midnight (America/Toronto, or the value of the `TIMEZONE` env var), not UTC midnight. A complaint created at 23:30 UTC on a given date will fall on a different calendar day depending on the active DST offset. Go must replicate the same `America/Toronto` timezone conversion when computing the `start_date`/`end_date` boundaries for this query.

---

### reports_dao.py

#### `create_report(report: Report) -> Report`
- **Purpose** — Persist a new `Report` record.
- **Query type** — INSERT + explicit `commit()`.
- **Key filters/conditions** — None.
- **Returns** — The persisted `Report` object.
- **Notes** — Decorated `@transactional` but also calls `db.session.commit()` explicitly inside the body; the outer decorator will call commit again — effectively a double-commit pattern (harmless but redundant).

#### `get_reports_for_service(service_id: str, limit_days: int) -> List[Report]`
- **Purpose** — Retrieve reports for a service within a rolling window.
- **Query type** — SELECT with optional date filter.
- **Key filters/conditions** — `service_id = :service_id`; if `limit_days` is not None, additionally `requested_at >= (utcnow - limit_days days)`.
- **Returns** — List of `Report` objects ordered `requested_at DESC`.
- **Notes** — Passing `limit_days=None` disables the date filter and returns all reports for the service.

#### `get_report_by_id(report_id) -> Report`
- **Purpose** — Fetch a single report by primary key.
- **Query type** — SELECT with `.one()`.
- **Key filters/conditions** — `id = :report_id`.
- **Returns** — Single `Report` object.
- **Notes** — Raises `NoResultFound` if no matching record exists.

#### `update_report(report: Report)`
- **Purpose** — Persist changes to an existing report.
- **Query type** — UPDATE (via `db.session.add` on dirty object) + explicit commit.
- **Key filters/conditions** — None; caller is responsible for mutating the object.
- **Returns** — None.
- **Notes** — Same double-commit pattern as `create_report`.

---

### events_dao.py

#### `dao_create_event(event)`
- **Purpose** — Append a system event to the audit log.
- **Query type** — INSERT + immediate commit.
- **Key filters/conditions** — None.
- **Returns** — None.
- **Notes** — Does **not** use the `@transactional` decorator; commits directly. This means there is no automatic rollback on failure.
  - **⚠️ No transaction safety**: if the INSERT succeeds but the calling function’s surrounding transaction is later rolled back, the event row will still be committed and cannot be undone (orphan audit row). Conversely, if the DB commit inside `dao_create_event` fails, the caller receives no exception (the event is silently lost). Go must wrap event inserts in the caller’s transaction rather than issuing a separate commit.

---

### email_branding_dao.py

#### `dao_get_email_branding_options(filter_by_organisation_id=None)`
- **Purpose** — List available email branding records, optionally scoped to an organisation.
- **Query type** — SELECT (with or without filter).
- **Key filters/conditions** — If `filter_by_organisation_id` is truthy: `organisation_id = :org_id`; otherwise returns all rows.
- **Returns** — List of `EmailBranding` objects.
- **Notes** — Returns the full table when no filter is passed.

#### `dao_get_email_branding_by_id(email_branding_id)`
- **Purpose** — Fetch a single email branding record by primary key.
- **Query type** — SELECT with `.one()`.
- **Key filters/conditions** — `id = :email_branding_id`.
- **Returns** — Single `EmailBranding` object.
- **Notes** — Raises an exception if not found.

#### `dao_get_email_branding_by_name(email_branding_name)`
- **Purpose** — Lookup branding by its display name.
- **Query type** — SELECT with `.first()`.
- **Key filters/conditions** — `name = :email_branding_name`.
- **Returns** — `EmailBranding` or `None`.
- **Notes** — Name uniqueness is not enforced here; first match is returned.

#### `dao_create_email_branding(email_branding)`
- **Purpose** — Persist a new email branding record.
- **Query type** — INSERT.
- **Key filters/conditions** — None.
- **Returns** — None (committed by `@transactional`).

#### `dao_update_email_branding(email_branding, **kwargs)`
- **Purpose** — Update fields on an existing email branding record.
- **Query type** — UPDATE.
- **Key filters/conditions** — Iterates `kwargs`; sets each attribute with `setattr`; coerces any falsy value to `None`.
- **Returns** — None (committed by `@transactional`).
- **Notes** — Falsy-to-`None` coercion means empty strings and zero values are stored as `NULL`.

---

### letter_branding_dao.py

#### `dao_get_letter_branding_by_id(letter_branding_id)`
- **Purpose** — Fetch a single letter branding record by primary key.
- **Query type** — SELECT with `.one()`.
- **Key filters/conditions** — `id = :letter_branding_id`.
- **Returns** — Single `LetterBranding` object.
- **Notes** — Raises an exception if not found.

#### `dao_get_letter_branding_by_name(letter_branding_name)`
- **Purpose** — Lookup letter branding by display name.
- **Query type** — SELECT with `.first()`.
- **Key filters/conditions** — `name = :letter_branding_name`.
- **Returns** — `LetterBranding` or `None`.

#### `dao_get_all_letter_branding()`
- **Purpose** — Return the complete catalogue of letter branding options.
- **Query type** — SELECT, all rows.
- **Key filters/conditions** — None; ordered `name ASC`.
- **Returns** — List of `LetterBranding` objects.

#### `dao_create_letter_branding(letter_branding)`
- **Purpose** — Persist a new letter branding record.
- **Query type** — INSERT.
- **Key filters/conditions** — None.
- **Returns** — None (committed by `@transactional`).

#### `dao_update_letter_branding(letter_branding_id, **kwargs)`
- **Purpose** — Update fields on an existing letter branding record.
- **Query type** — SELECT then UPDATE.
- **Key filters/conditions** — Looks up by `letter_branding_id` first; iterates `kwargs`; coerces falsy values to `None`.
- **Returns** — Updated `LetterBranding` object.
- **Notes** — Unlike `dao_update_email_branding`, this function fetches the record internally and returns it. Falsy-to-`None` coercion applies here too.

---

### daily_sorted_letter_dao.py

#### `dao_get_daily_sorted_letter_by_billing_day(billing_day)`
- **Purpose** — Retrieve the sorted-letter count record for a specific billing day.
- **Query type** — SELECT with `.first()`.
- **Key filters/conditions** — `billing_day = :billing_day`.
- **Returns** — `DailySortedLetter` or `None`.

#### `dao_create_or_update_daily_sorted_letter(new_daily_sorted_letter)`
- **Purpose** — Upsert sorted/unsorted letter counts for a (billing_day, file_name) pair.
- **Query type** — PostgreSQL `INSERT … ON CONFLICT DO UPDATE`.
- **Key filters/conditions** — Conflict target is the composite unique index on `(billing_day, file_name)`. On conflict: updates `unsorted_count`, `sorted_count`, and `updated_at`.
- **Returns** — None.
- **Notes** — Uses raw SQL (`db.session.connection().execute(stmt)`) to avoid race conditions when multiple threads process the same billing-day file. The `excluded` pseudo-table references the values that were rejected by the constraint.

---

### dao_utils.py

#### `transactional` (decorator)
- **Purpose** — Wrap a DAO function with automatic commit/rollback.
- **Behaviour** — Calls `db.session.commit()` on success; on any exception logs the error, calls `db.session.rollback()`, and re-raises.
- **Notes** — Used by the majority of mutating DAO functions across all domains.

#### `VersionOptions`
- **Purpose** — Configuration object passed to `version_class`.
- **Fields** — `model_class` (required), `history_class` (optional — if `None` the `create_history` helper infers it), `must_write_history` (default `True`).

#### `version_class(*version_options)` (decorator)
- **Purpose** — After a DAO mutation, snapshot all new/dirty session objects of the specified model classes into corresponding history tables.
- **Behaviour** — After the wrapped function returns (but before commit), inspects `db.session.new` and `db.session.dirty` for matching model instances and calls `create_history` on each. If `must_write_history=True` and no matching objects were found, raises `RuntimeError` (guards against premature session flushes).
- **Notes** — Supports multiple `VersionOptions` in a single decorator. Typically combined with `@transactional`.

#### `dao_rollback()`
- **Purpose** — Explicitly roll back the current database session.
- **Returns** — None.
- **Notes** — Used in error recovery paths outside of `@transactional`-decorated functions.

---

### date_util.py

#### `get_months_for_financial_year(year)`
- **Purpose** — Return UTC datetime objects for each past month in the financial year starting in `year`.
- **Logic** — Combines months Apr–Dec of `year` with Jan–Mar of `year+1`; filters out any month whose UTC representation is in the future.
- **Returns** — List of `datetime` (UTC, no tzinfo).

#### `get_months_for_year(start, end, year)`
- **Purpose** — Generate month-start datetimes for a range of months within a calendar year.
- **Returns** — List of `datetime(year, month, 1)` for `month in range(start, end)`.

#### `get_financial_year(year)`
- **Purpose** — Return the UTC start and end of a financial year.
- **Returns** — `(start, end)` tuple where `start = get_april_fools(year)` and `end = get_april_fools(year+1) - 1 microsecond`.

#### `get_current_financial_year()`
- **Purpose** — Return the UTC start/end of the financial year that contains the current moment.
- **Logic** — If current UTC month ≤ 3 (January–March), FY start year = current year − 1; otherwise = current year.
- **Returns** — `(start, end)` tuple (same shape as `get_financial_year`).

#### `get_april_fools(year)`
- **Purpose** — Return April 1 00:00:00 local time (configured timezone, default `America/Toronto`) expressed in UTC with tzinfo stripped.
- **Notes** — This is the canonical financial-year start boundary. The timezone is read from the `TIMEZONE` environment variable; absence defaults to `America/Toronto`.

#### `get_month_start_and_end_date_in_utc(month_year)`
- **Purpose** — Return UTC bounds for a complete calendar month.
- **Returns** — `(first_day_utc, last_day_utc)` where last day is `23:59:59.099999` of the final day, converted to UTC.

#### `get_current_financial_year_start_year()`
- **Purpose** — Return the integer start year of the current financial year.
- **Returns** — Integer (e.g. `2025` for the 2025–2026 FY).

#### `get_financial_year_for_datetime(start_date)`
- **Purpose** — Determine which financial year a given date or datetime belongs to.
- **Logic** — If `start_date < get_april_fools(year)`, returns `year - 1`; otherwise returns `year`.
- **Returns** — Integer year.

#### `get_midnight(datetime: datetime) -> datetime`
- **Purpose** — Zero out the time portion of a datetime.
- **Returns** — `datetime` with `hour=minute=second=microsecond=0`.

#### `tz_aware_utc_now() -> datetime`
- **Purpose** — Return the current UTC time as a pytz-aware datetime.
- **Returns** — `datetime` localized to `pytz.utc`.

#### `tz_aware_midnight_n_days_ago(days_ago: int = 1) -> datetime`
- **Purpose** — Return UTC midnight (EST/EDT-aware) for a date`days_ago` days before today.
- **Returns** — EST/EDT pytz-aware `datetime`.

#### `utc_midnight_n_days_ago(number_of_days) -> datetime`
- **Purpose** — Return UTC midnight for a date `number_of_days` before today.
- **Returns** — `datetime` with `timezone.utc`, time zeroed out.

#### `get_query_date_based_on_retention_period(retention_period) -> datetime`
- **Purpose** — Compute the oldest date still within a notification retention window.
- **Logic** — `now(UTC) - retention_period days` combined with `time.max` (23:59:59.999999).
- **Returns** — `datetime` (timezone-aware, UTC).

---

## Domain Rules & Invariants

### Complaints
- Complaints are received from provider callback webhooks and persisted directly via `save_complaint`.
- No deduplication is enforced at the DAO layer; duplicate callbacks would create duplicate rows.
- The complaint count query is **inclusive on both ends** at the day level: start-date midnight ≤ `created_at` < (end-date + 1 day) midnight — this makes it a [start, end] closed interval when measured in whole days.
- Paginated global listing always includes the related service object (eager-loaded) to avoid N+1 queries on admin UIs.

### Reports
- Reports are scoped to a single service and tracked with a `requested_at` timestamp.
- A rolling `limit_days` window is the standard access pattern; passing `None` bypasses the window.
- Report status transitions (pending → ready/failed) are performed via `update_report`.
- The `reports_dao` double-commits (explicit `commit()` inside a `@transactional` body); this is safe but means the decorator's commit is a no-op.

### Events
- The events table functions as an append-only audit log.
- `dao_create_event` bypasses `@transactional`, so failures are not automatically rolled back.

### Email Branding
- Branding can be scoped to an organisation (`organisation_id` foreign key) or global (no org filter).
- Updating any field with a falsy value (`""`, `0`, `False`) stores `NULL`, not the falsy value. Callers must pass `None` explicitly if they want to preserve the existing value.
- Name lookup (`dao_get_email_branding_by_name`) returns the first match and does not enforce uniqueness.

### Letter Branding
- All letter branding records are globally visible (no organisation scoping in this DAO).
- The sorted list (`dao_get_all_letter_branding`) is alphabetical by name — suitable for display in dropdowns.
- Same falsy-to-`None` coercion rule applies on update as for email branding.
- `dao_update_letter_branding` is the only branding update function that returns the modified record.

### Daily Sorted Letter
- The `daily_sorted_letters` table records how many letters were sorted/unsorted per billing day per file.
- The composite key `(billing_day, file_name)` is the conflict target. The same file processed twice in the same day overwrites the counts rather than appending.
- `updated_at` is set to `utcnow()` on every upsert conflict.
- The PostgreSQL upsert pattern via `sqlalchemy.dialects.postgresql.insert` is intentional to handle concurrent workers processing the same billing-day file.

### Platform Statistics
- `GET /platform-stats` aggregates notification status totals across all services for a date window (defaults to today if parameters omitted).
- `GET /platform-stats/usage-for-all-services` enforces that the queried date range falls **within a single financial year**; cross-year queries are rejected with HTTP 400.
- The usage-for-all-services response merges three independent datasets (SMS billing, letter cost totals, letter line-item breakdown) into a per-service dictionary keyed by `service_id`. Services present only in letter data and not SMS data are still included.
- Services are sorted: blank `organisation_name` last, then alphabetically by `organisation_name`, then by `service_name`.
- Trial-service stats are a separate endpoint with no date parameters.

### DAO Utilities
- The `@transactional` decorator is the standard commit/rollback guard for all mutating DAO functions. Any DAO that modifies data without it (e.g. `dao_create_event`) manages its own commit.
- The `@version_class` decorator integrates with `history_meta.create_history` to snapshot model state into audit history tables. It must be applied **in addition to** `@transactional` (they are independent).
- If a session is flushed before `@version_class` inspects `db.session.new/dirty`, the decorator raises `RuntimeError` with a diagnostic message. This is a programming error, not a runtime condition.

### Date Utilities
- The financial year is defined as **April 1 local time (TIMEZONE env var, default `America/Toronto`) through March 31 of the following year**.
- All financial-year boundaries are stored and compared as UTC timestamps with tzinfo stripped (naïve datetimes).
- `get_april_fools` is the single source of truth for the FY start boundary; all other FY functions delegate to it.
- `utc_midnight_n_days_ago` uses `datetime.now(timezone.utc)` (standard library); `tz_aware_utc_now` uses `pytz.utc.localize` — both produce UTC-aware objects but via different mechanisms.

### Support
- The `/support/find-ids` endpoint is an administrative lookup tool; it is not paginated.
- Entity resolution order is fixed: user → service → template → job → notification. The first match for a given UUID wins; only one entity type is returned per UUID.
- Non-UUID tokens are not rejected with 400; they are returned inline as `{"id": "...", "type": "not a uuid"}`.
- UUIDs that match no entity are returned as `{"id": "...", "type": "no result found"}`.

### Newsletter
- Subscriber state is managed externally in **Airtable** (not the Postgres database); all subscriber CRUD calls the Airtable client.
- Subscription flow: `POST /unconfirmed-subscriber` → confirmation email → `GET /confirm/<id>`.
- If a subscriber already exists on `POST /unconfirmed-subscriber`, the language preference is updated and the confirmation email is resent (idempotent re-subscribe).
- A subscriber with status `UNSUBSCRIBED` who confirms is handled as a re-subscription (`has_resubscribed=True`).
- The latest newsletter (`send-latest`) can only be sent to subscribers with status `SUBSCRIBED`; other statuses are rejected with HTTP 400.
- Newsletter emails are sent via the system notify service (`NOTIFY_SERVICE_ID`) using `KEY_TYPE_NORMAL` and enqueued to `QueueNames.NOTIFY`.
- Confirmation email template IDs are language-keyed config values (`NEWSLETTER_CONFIRMATION_EMAIL_TEMPLATE_ID_EN` / `_FR`); latest newsletter templates are fetched dynamically from Airtable at send time.
- The confirmation link is constructed as `{ADMIN_BASE_URL}/newsletter/confirm/{subscriber_id}`.

---

## Error Conditions

| Location | Condition | Exception / Response |
|---|---|---|
| `dao_utils.transactional` | Any exception inside wrapped function | Logs error, rolls back session, re-raises original exception |
| `dao_utils.version_class` | `must_write_history=True` but session has no matching new/dirty objects | `RuntimeError("Can't record history for <ModelClass> ...")` |
| `reports_dao.get_report_by_id` | No `Report` row with given `id` | SQLAlchemy `NoResultFound` (unhandled — propagates to caller) |
| `email_branding_dao.dao_get_email_branding_by_id` | No `EmailBranding` row with given `id` | SQLAlchemy `.one()` raises `NoResultFound` |
| `letter_branding_dao.dao_get_letter_branding_by_id` | No `LetterBranding` row with given `id` | SQLAlchemy `.one()` raises `NoResultFound` |
| `platform_stats.validate_date_range_is_within_a_financial_year` | Date not parseable as `YYYY-MM-DD` | `InvalidRequest(400, "Input must be a date in the format: YYYY-MM-DD")` |
| `platform_stats.validate_date_range_is_within_a_financial_year` | `end_date < start_date` | `InvalidRequest(400, "Start date must be before end date")` |
| `platform_stats.validate_date_range_is_within_a_financial_year` | Dates span two different financial years | `InvalidRequest(400, "Date must be in a single financial year.")` |
| `support.find_ids` | `ids` query param absent | `{"error": "no ids provided"}` with HTTP 400 |
| `support.*_query` functions | SQLAlchemy `NoResultFound` | Caught internally; returns `None` (entity reported as not found) |
| `newsletter.create_unconfirmed_subscription` | `email` field missing in body | `InvalidRequest(400, "Email is required")` |
| `newsletter.create_unconfirmed_subscription` | Airtable error fetching existing subscriber (non-404) | `InvalidRequest(<airtable_status_code>, "Error fetching existing subscriber: ...")` |
| `newsletter.create_unconfirmed_subscription` | Airtable save failed | `InvalidRequest(500, "Failed to create unconfirmed mailing list subscriber.")` |
| `newsletter.confirm_subscription` | Subscriber not found in Airtable | `InvalidRequest(404, "Subscriber not found")` |
| `newsletter.confirm_subscription` | Airtable error (non-404) fetching subscriber | `InvalidRequest(<airtable_status_code>, "Failed to fetch subscriber: ...")` |
| `newsletter.confirm_subscription` | Airtable save of confirmation failed | `InvalidRequest(500, "Subscription confirmation failed")` |
| `newsletter.unsubscribe` | Subscriber not found | `InvalidRequest(404, "Subscriber not found")` |
| `newsletter.unsubscribe` | Airtable save failed | `InvalidRequest(500, "Unsubscription failed")` |
| `newsletter.update_language_preferences` | `language` field missing in body | `InvalidRequest(400, "New language is required")` |
| `newsletter.update_language_preferences` | Subscriber not found | `InvalidRequest(404, "Subscriber not found")` |
| `newsletter.update_language_preferences` | Airtable save failed | `InvalidRequest(500, "Language update failed")` |
| `newsletter.send_latest_newsletter` | Subscriber not found | `InvalidRequest(404, "Subscriber not found")` |
| `newsletter.send_latest_newsletter` | Subscriber status ≠ `SUBSCRIBED` | `InvalidRequest(400, "Cannot send to subscribers with status: <status>")` |
| `newsletter._send_latest_newsletter` | No current templates in Airtable | `InvalidRequest(404, "No current newsletter templates found")` |
| `newsletter.get_subscriber` | Neither `subscriber_id` nor `email` provided | `InvalidRequest(400, "Subscriber ID or email is required")` |
| `newsletter.get_subscriber` | Subscriber not found | `InvalidRequest(404, "Subscriber not found")` |

---

## Query Inventory (for sqlc)

| Query name | Type | Tables | Description |
|---|---|---|---|
| `InsertComplaint` | INSERT | `complaints` | Persist a new complaint record |
| `GetComplaintsPaginated` | SELECT | `complaints`, `services` | All complaints ordered `created_at DESC`, with service join, paginated |
| `GetComplaintsByService` | SELECT | `complaints` | Complaints for a service, ordered `created_at DESC` |
| `CountComplaintsByDateRange` | SELECT COUNT | `complaints` | Count complaints with `created_at` in UTC date window |
| `InsertReport` | INSERT | `reports` | Create a new report record |
| `GetReportsByService` | SELECT | `reports` | Reports for a service within a rolling day window, ordered `requested_at DESC` |
| `GetReportById` | SELECT | `reports` | Single report by primary key |
| `UpdateReport` | UPDATE | `reports` | Persist changed fields on an existing report |
| `InsertEvent` | INSERT | `events` | Append a new audit event |
| `GetEmailBrandingOptions` | SELECT | `email_branding` | All email branding, optionally filtered by `organisation_id` |
| `GetEmailBrandingById` | SELECT | `email_branding` | Single email branding by PK |
| `GetEmailBrandingByName` | SELECT | `email_branding` | First email branding matching a name |
| `InsertEmailBranding` | INSERT | `email_branding` | Create email branding record |
| `UpdateEmailBranding` | UPDATE | `email_branding` | Update arbitrary fields on email branding |
| `GetLetterBrandingById` | SELECT | `letter_branding` | Single letter branding by PK |
| `GetLetterBrandingByName` | SELECT | `letter_branding` | First letter branding matching a name |
| `GetAllLetterBranding` | SELECT | `letter_branding` | All letter branding ordered `name ASC` |
| `InsertLetterBranding` | INSERT | `letter_branding` | Create letter branding record |
| `UpdateLetterBranding` | UPDATE | `letter_branding` | Update arbitrary fields on letter branding by PK |
| `GetDailySortedLetterByBillingDay` | SELECT | `daily_sorted_letters` | Sorted-letter counts for a billing day |
| `UpsertDailySortedLetter` | INSERT ON CONFLICT DO UPDATE | `daily_sorted_letters` | Upsert sorted/unsorted counts on `(billing_day, file_name)` conflict |
| `GetNotificationStatusTotalsForAllServices` | SELECT (complex) | `ft_notification_status` | Aggregated notification stats per service/status for date window |
| `GetNotificationStatsForTrialServices` | SELECT (complex) | `ft_notification_status`, `services` | Notification stats filtered to trial-mode services |
| `GetSmsBillingForAllServices` | SELECT (complex) | `ft_billing`, `services`, `organisations` | SMS billing totals per service for date range |
| `GetLetterCostsForAllServices` | SELECT (complex) | `ft_billing`, `services`, `organisations` | Letter billing totals per service |
| `GetLetterLineItemsForAllServices` | SELECT (complex) | `ft_billing` | Per-postage letter line items for breakdown display |
| `GetSendMethodStatsByService` | SELECT (complex) | `notifications` (or `ft_*`) | Send-method (API vs CSV job) counts per service for date range |
