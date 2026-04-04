# Platform Admin Features — Change Brief

## Overview

Platform admin features implement cross-cutting operational endpoints that serve administrators and SRE teams: complaint management, email and letter branding CRUD, system event logging, platform usage statistics, SRE inspection tools, Redis cache-clear, Cypress test helpers (non-production only), healthcheck, and the support entity-lookup utility.

---

## Source Files

- `spec/behavioral-spec/platform-admin.md`
- `spec/business-rules/platform-admin.md`
- `openspec/changes/platform-admin-features/proposal.md`

---

## Endpoints

### Cache

#### POST /cache/clear
- Calls `redis_store.delete_cache_keys_by_pattern` for every key pattern in `CACHE_KEYS_ALL`.
- Returns `201 {"result": "ok"}` on success.
- On Redis exception → `500 {"error": "Unable to clear the cache"}`.
- Auth: dedicated cache-clear JWT (distinct from standard admin JWT).

---

### Status / Healthcheck

#### GET / and GET /_status (no auth)
- Returns `200 {"status": "ok", "db_version": "<str>", "commit_sha": "<str>", "build_time": "<str>", "current_time_utc": "<str>"}`.
- No authentication required on either route.

#### GET /status/live-service-and-organisation-counts
- Returns `{"organisations": <int>, "services": <int>}`.
- A service is counted as live when: `active=True`, `restricted=False`, `count_as_live=True`.
- Services not attached to any org still count toward the service total.
- An organisation is counted only if it has at least one qualifying live service.
- Trial services, inactive services, and services with `count_as_live=False` are excluded from both counts.
- Auth: internal admin JWT.

---

### SRE Tools

SRE endpoints require SRE JWT (distinct from standard admin JWT); admin JWT on an SRE endpoint returns 401.

#### GET /sre-tools/live-service-and-organisation-counts
- Delegates to same DB counting logic as `/status/live-service-and-organisation-counts`.
- Returns `{"organisations": <int>, "services": <int>}`.
- Auth: SRE JWT.

---

### Cypress (Non-Production Only)

#### POST /cypress/create_user/<email_suffix>
- Creates two users: `notify-ui-tests+ag_<suffix>@cds-snc.ca` (regular) and `notify-ui-tests+ag_<suffix>_admin@cds-snc.ca` (admin).
- Returns `201 {"regular": {...}, "admin": {...}}`; both users are persisted in the database.
- Validation: `email_suffix` containing non-alphanumeric characters (e.g. dashes) → `400`.
- Environment guard: `NOTIFY_ENVIRONMENT == "production"` → `403` before auth is checked.
- Auth: dedicated Cypress JWT.

#### GET /cypress/cleanup
- Deletes all test users with `created_at` older than 30 days.
- Returns `201 {"message": "Clean up complete"}`; deleted users are no longer retrievable.
- Auth: dedicated Cypress JWT.

---

### Events

#### POST /events
- Body: `{"event_type": "<str>", "data": {<arbitrary JSON>}}`.
- Returns `201 {"data": {"event_type": ..., "data": {...}}}`.
- All fields in `data` are stored and echoed without modification.
- Auth: internal admin JWT.

---

### Email Branding

#### GET /email-branding
- Returns `{"email_branding": [...]}` with all branding records.
- Each record includes `id`, `organisation_id` (empty string `""` when null).
- Filter: `?organisation_id=<uuid>` restricts results to brandings whose `organisation_id` matches; without the filter all records are returned.
- Auth: internal admin JWT.

#### GET /email-branding/<email_branding_id>
- Returns `{"email_branding": {colour, logo, name, id, text, brand_type, organisation_id, alt_text_en, alt_text_fr, created_by_id, updated_at, created_at, updated_by_id}}`.
- `organisation_id` is empty string `""` when null; `alt_text_fr` is `null` when absent.
- Auth: internal admin JWT.

#### POST /email-branding (create)
- Required fields: `name`, `created_by_id`.
- Optional fields: `colour` (null if omitted), `logo` (null if omitted), `brand_type`, `alt_text_en`, `alt_text_fr`, `text`.
- Defaults: `brand_type` defaults to `BRANDING_ORG_NEW` (`custom_logo`); `text` defaults to `name` when not supplied; explicit `null` for `text` is persisted as `null`.
- Returns `201 {"data": {...}}` on success.
- Missing `name` → `400 {"errors": [{"message": "name is a required property"}]}`.
- Duplicate `name` → `400 {"message": "Email branding already exists, name must be unique."}`.
- Invalid `brand_type` → `400 {"errors": [{"message": "brand_type <val> is not one of [custom_logo, both_english, both_french, custom_logo_with_background_colour, no_branding]"}]}`.
- Auth: internal admin JWT.

#### POST /email-branding/<email_branding_id> (update)
- Partial update; any subset of fields may be provided; `updated_by_id` required.
- Returns `200` on success.
- Duplicate `name` → `400 {"message": "Email branding already exists, name must be unique."}`.
- Invalid `brand_type` → same 400 message as create.
- Auth: internal admin JWT.

---

### Letter Branding

#### GET /letter-branding
- Returns JSON array of all letter branding records (serialised via `.serialize()`); empty array `[]` when none exist.
- Auth: internal admin JWT.

#### GET /letter-branding/<letter_branding_id>
- Returns single serialised branding record; `200`.
- Unknown id → `404`.
- Auth: internal admin JWT.

#### POST /letter-branding (create)
- Required fields: `name`, `filename`.
- Inserts record, returns `201 {"id": "<uuid>", ...}`; entity is retrievable immediately after creation.
- Auth: internal admin JWT.

#### POST /letter-branding/<letter_branding_id> (update)
- Updates `name` and/or `filename` for the given brand; returns `200`.
- Name collision (IntegrityError) → `400 {"message": {"name": ["Name already in use"]}}`.
- Auth: internal admin JWT.

---

### Complaint

#### GET /complaint
- Returns `{"complaints": [...]}` sorted descending by `created_at`.
- Empty state: returns `{"complaints": []}`.
- Optional `?page=N`; when `PAGE_SIZE` is exceeded, response includes `{"links": {"prev", "next", "last"}}` with absolute paths (`/complaint?page=N`).
- Auth: internal admin JWT.

#### GET /complaint/count
- `?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD`; delegates to `fetch_count_of_complaints`; returns integer directly (not wrapped in an object).
- Defaults: if either date is omitted, both default to `date.today()` (UTC wall-clock date at request time).
- Invalid date format → `400 {"errors": [{"message": "start_date time data <val> does not match format %Y-%m-%d"}]}`.
- Auth: internal admin JWT.

---

### Platform Stats

#### GET /platform-stats
- Returns notification status totals grouped by channel: `{email: {failures: {virus-scan-failed, temporary-failure, permanent-failure, technical-failure}, total, test-key}, letter: {...}, sms: {...}}`.
- `start_date` and `end_date` both default to today when omitted.
- Filter: `?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD`.
- Invalid date format → `400 {"errors": [{"message": "start_date time data <val> does not match format %Y-%m-%d"}]}`.
- Auth: internal admin JWT.

#### GET /platform-stats/usage-for-all-services
- `?start_date=...&end_date=...`; returns array of per-service objects: `{organisation_id, service_id, sms_cost, sms_fragments, letter_cost, letter_breakdown}`.
- `organisation_id` is empty string `""` when service has no org.
- `letter_breakdown` is human-readable (e.g. `"6 second class letters at 45p\n"`).
- Date range must be within a single financial year (April 1 → March 31, local timezone); cross-year → `400`.
- Services sorted: blank `organisation_name` last, then alphabetically by org name, then by service name.
- Services present only in letter data (not SMS) are still included in the result.
- Auth: internal admin JWT.

#### GET /platform-stats/usage-for-trial-services
- Delegates to `fetch_notification_stats_for_trial_services`; returns array (may be empty).
- No date parameters.
- Auth: internal admin JWT.

#### GET /platform-stats/send-methods-stats-by-service
- `?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD`; delegates to `send_method_stats_by_service(start_date, end_date)`; returns array.
- Auth: internal admin JWT.

**Financial-year date validation (`validate_date_range_is_within_a_financial_year`)**:
- Non-date string → `400 {"message": "Input must be a date in the format: YYYY-MM-DD", "status_code": 400}`.
- start > end → `400 {"message": "Start date must be before end date", "status_code": 400}`.
- Spans two financial years → `400 {"message": "Date must be in a single financial year.", "status_code": 400}`.

---

### Support

#### GET /support/find-id
- Required: `?ids=<uuid>[,<uuid>...]` — comma and/or whitespace-separated UUIDs.
- `ids` absent or empty list → `400 {"error": "no ids provided"}`.
- Non-UUID value in list: returned inline as `{"type": "not a uuid"}` (no 400).
- Returns JSON array with one entry per submitted ID in order:
  - User: `{type: "user", id, user_name}`
  - Service: `{type: "service", id, service_name}`
  - Template: `{type: "template", id, template_name, service_id, service_name}`
  - Job: `{type: "job", id, original_file_name, created_by_id, created_by_name, notification_count, job_status, service_id, service_name, template_id, template_name}`
  - Notification: `{type: "notification", id, notification_type, status, to, service_id, service_name, template_id, template_name, job_id, job_row_number, api_key_id}` (`api_key_id` is `null` when not set)
  - Unknown UUID: `{type: "no result found"}`
- Entity resolution order: user → service → template → job → notification; first match per UUID wins.
- Not paginated.
- Auth: internal admin JWT.

---

## DAO Functions

### complaint_dao

#### `save_complaint(complaint)`
- INSERT via `db.session.add`; decorated `@transactional`.
- Returns None.

#### `fetch_paginated_complaints(page=1)`
- SELECT all complaints, `created_at DESC`, page-based via `PAGE_SIZE` config.
- Returns SQLAlchemy `Pagination` object.
- `joinedload(Complaint.service)` to avoid N+1 during serialisation outside the session.

#### `fetch_complaints_by_service(service_id)`
- SELECT filtered by `service_id`, `created_at DESC`; no pagination; returns full list.

#### `fetch_count_of_complaints(start_date, end_date)`
- SELECT COUNT where `created_at >= midnight(start_date) AND created_at < midnight(end_date + 1 day)` (UTC, using `America/Toronto` timezone — TIMEZONE env var — for boundary conversion).
- Returns integer count.
- ⚠️ Go must replicate `America/Toronto` timezone boundary, not UTC midnight.

### events_dao

#### `dao_create_event(event)`
- INSERT + immediate `db.session.commit()` (no `@transactional` decorator).
- ⚠️ No automatic rollback on failure; Go must insert events within the caller's transaction to prevent orphan audit rows.

### email_branding_dao

#### `dao_get_email_branding_options(filter_by_organisation_id=None)`
- SELECT; without filter returns all rows; with filter restricts by `organisation_id = :org_id`.

#### `dao_get_email_branding_by_id(email_branding_id)`
- SELECT by PK with `.one()`; raises `NoResultFound` on miss.

#### `dao_get_email_branding_by_name(email_branding_name)`
- SELECT by name with `.first()`; returns `None` on miss.

#### `dao_create_email_branding(email_branding)`
- INSERT; `@transactional`.

#### `dao_update_email_branding(email_branding, **kwargs)`
- UPDATE via `setattr`; any falsy kwarg value coerced to `NULL` (empty string → NULL, 0 → NULL).
- `@transactional`.

### letter_branding_dao

#### `dao_get_letter_branding_by_id(letter_branding_id)`
- SELECT by PK with `.one()`; raises `SQLAlchemyError` on miss.

#### `dao_get_letter_branding_by_name(letter_branding_name)`
- SELECT by name with `.first()`; returns `None` on miss.

#### `dao_get_all_letter_branding()`
- SELECT all rows; ordered `name ASC`; returns `[]` when table is empty.

#### `dao_create_letter_branding(letter_branding)`
- INSERT; `@transactional`; `filename` defaults to `name` when only `name` is supplied.

#### `dao_update_letter_branding(letter_branding_id, **kwargs)`
- SELECT record by `letter_branding_id`, then UPDATE; falsy values coerced to `NULL`.
- Returns updated `LetterBranding` object (unlike email branding update, which returns None).

### daily_sorted_letter_dao

#### `dao_get_daily_sorted_letter_by_billing_day(billing_day)`
- SELECT with `.first()`; returns `DailySortedLetter` or `None`.

#### `dao_create_or_update_daily_sorted_letter(new_daily_sorted_letter)`
- PostgreSQL `INSERT … ON CONFLICT DO UPDATE` (conflict target: composite `(billing_day, file_name)`).
- On conflict: updates `unsorted_count`, `sorted_count`, `updated_at`.
- Uses raw SQL via `db.session.connection().execute(stmt)` to handle concurrent workers on the same billing-day file.

### dao_utils

#### `transactional` (decorator)
- Calls `db.session.commit()` on success; on exception logs, rolls back, re-raises.

#### `version_class(*version_options)` (decorator)
- After wrapped function returns, snapshots new/dirty model instances into history tables via `create_history`.
- Raises `RuntimeError` when `must_write_history=True` and no matching objects found (programming error, not runtime condition).

#### `dao_rollback()`
- Explicit rollback; used in error recovery outside `@transactional`.

---

## Business Rules & Invariants

### Complaints
- No deduplication at DAO layer; duplicate provider callbacks create duplicate rows.
- Count query is inclusive on both ends at day level: complaints at midnight and at 13:00 on day N are both included in a query for `[N, N]`.
- Paginated listing always joinloads `.service` to avoid `DetachedInstanceError` during serialisation.
- Complaint date boundaries use `America/Toronto` timezone (TIMEZONE env var); Go must replicate this conversion.

### Email Branding
- `organisation_id` null = globally available branding; non-null = scoped to that org.
- Updating any field with a falsy value (`""`, `0`, `False`) stores `NULL`, not the falsy value. Callers must pass the existing value or `None` explicitly.
- Name uniqueness is enforced at the API layer via `dao_get_email_branding_by_name` check before write.

### Letter Branding
- All letter branding records are globally visible (no organisation scoping in the DAO).
- Alphabetical sort (`name ASC`) is required for all list results.
- Same falsy-to-`NULL` coercion applies on update as for email branding.
- `dao_update_letter_branding` is the only branding update function that returns the modified record.

### Platform Statistics
- `usage-for-all-services` enforces that the date range falls within a single financial year; cross-year → 400.
- Response merges three datasets (SMS billing, letter cost totals, letter line items) per `service_id`; services in letter data only are still included.
- Services sorted: blank `organisation_name` last, then org name alphabetically, then service name alphabetically.
- Financial year boundary: April 1 local time (`America/Toronto`), expressed as UTC, via `get_april_fools(year)`.

### Cypress Environment Guard
- Production guard is evaluated at request handling time, **before** the Cypress auth header is verified.
- If `NOTIFY_ENVIRONMENT == "production"`, the handler returns `403` regardless of auth.
- Non-alphanumeric `email_suffix` (e.g. containing dashes) returns 400 in all environments.

### Cache Clear
- A single `POST /cache/clear` clears all keys matching every pattern in `CACHE_KEYS_ALL`.
- Redis exceptions during key deletion are surfaced as `500`.

### Events
- `dao_create_event` bypasses `@transactional` and commits directly; failures produce no automatic rollback.
- Go must insert events within the caller's DB transaction to maintain rollback safety.

### Support
- Entity resolution order is fixed: user → service → template → job → notification.
- Non-UUID tokens are returned inline as `{"type": "not a uuid"}` (not rejected with 400).
- UUIDs matching no entity return `{"type": "no result found"}`.
- The endpoint is not paginated.

---

## Error Conditions

| Location | Condition | Response |
|---|---|---|
| `POST /cache/clear` | Redis exception during key deletion | `500 {"error": "Unable to clear the cache"}` |
| `GET /complaint/count` | Invalid date format | `400 {"errors": [{"message": "start_date time data <val> does not match format %Y-%m-%d"}]}` |
| `POST /email-branding` | Missing `name` | `400 {"errors": [{"message": "name is a required property"}]}` |
| `POST /email-branding` | Duplicate `name` | `400 {"message": "Email branding already exists, name must be unique."}` |
| `POST /email-branding` | Invalid `brand_type` | `400 {"errors": [{"message": "brand_type <val> is not one of [custom_logo, both_english, both_french, custom_logo_with_background_colour, no_branding]"}]}` |
| `POST /email-branding/<id>` | Duplicate `name` | `400 {"message": "Email branding already exists, name must be unique."}` |
| `GET /email-branding/<id>` | Unknown id | SQLAlchemy `NoResultFound` → 404 |
| `GET /letter-branding/<id>` | Unknown id | `404` |
| `POST /letter-branding/<id>` | Name collision | `400 {"message": {"name": ["Name already in use"]}}` |
| `GET /platform-stats` | Invalid date format | `400 {"errors": [{"message": "start_date time data <val> does not match format %Y-%m-%d"}]}` |
| `GET /platform-stats/usage-for-all-services` | Non-date string | `400 {"message": "Input must be a date in the format: YYYY-MM-DD", "status_code": 400}` |
| `GET /platform-stats/usage-for-all-services` | start > end | `400 {"message": "Start date must be before end date", "status_code": 400}` |
| `GET /platform-stats/usage-for-all-services` | Cross-financial-year range | `400 {"message": "Date must be in a single financial year.", "status_code": 400}` |
| `GET /support/find-id` | `ids` absent or empty list | `400 {"error": "no ids provided"}` |
| `POST /cypress/create_user/<suffix>` | Non-alphanumeric `email_suffix` | `400` |
| `POST /cypress/create_user/<suffix>` | `NOTIFY_ENVIRONMENT == "production"` | `403` |
| `dao_utils.transactional` | Any exception inside wrapped function | Logs error, rolls back, re-raises |
| `dao_utils.version_class` | `must_write_history=True` but no matching dirty/new objects | `RuntimeError("Can't record history for ...")` |
