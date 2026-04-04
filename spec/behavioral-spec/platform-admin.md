# Behavioral Spec: Platform Admin & Cross-Cutting

## Processed Files

- [x] tests/app/complaint/test_complaint_rest.py
- [x] tests/app/dao/test_complaint_dao.py
- [x] tests/app/report/test_rest.py
- [x] tests/app/report/test_utils.py
- [x] tests/app/events/test_rest.py
- [x] tests/app/dao/test_events_dao.py
- [x] tests/app/email_branding/test_rest.py
- [x] tests/app/dao/test_email_branding_dao.py
- [x] tests/app/letter_branding/test_letter_branding_rest.py
- [x] tests/app/dao/test_letter_branding_dao.py
- [x] tests/app/platform_stats/test_rest.py
- [x] tests/app/dao/test_daily_sorted_letter_dao.py
- [x] tests/app/dao/test_date_utils.py
- [x] tests/app/newsletter/test_rest.py
- [x] tests/app/support/test_rest.py
- [x] tests/app/cache/test_cache_rest.py
- [x] tests/app/status/test_status.py
- [x] tests/app/cypress/test_rest.py
- [x] tests/app/test_model.py
- [x] tests/app/test_errors.py
- [x] tests/app/test_schemas.py
- [x] tests/app/test_queue.py
- [x] tests/app/test_config.py
- [x] tests/app/test_utils.py
- [x] tests/app/test_annotations.py
- [x] tests/app/test_cors_headers.py
- [x] tests/app/test_json_provider.py
- [x] tests/app/test_user_agent_processing.py
- [x] tests/app/test_cronitor.py
- [x] tests/app/celery/test_celery_error_classification.py
- [x] tests/app/celery/test_service_callback_tasks.py
- [x] tests/app/commands/test_performance_platform_commands.py
- [x] tests/app/aws/test_metric_logger.py
- [x] tests/app/aws/test_metrics.py
- [x] tests/app/aws/test_s3.py
- [x] tests/app/v2/test_errors.py
- [x] tests/app/v2/api_spec/test_get_api_spec.py

---

## Endpoint Contracts

### Complaint

#### GET /complaint
- **Happy path**: returns `{"complaints": [...]}` sorted descending by `created_at`; complaints span multiple services
- **Empty state**: returns `{"complaints": []}`
- **Pagination**: optional `?page=N`; when `PAGE_SIZE` is exceeded, response includes `{"links": {"prev", "next", "last"}}` with absolute paths (`/complaint?page=N`)
- **Auth**: requires internal JWT auth header

#### GET /complaint/count
- **Happy path**: `?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD`; delegates to `fetch_count_of_complaints`; returns an integer directly (not wrapped in an object)
- **Defaults**: if `start_date` or `end_date` are omitted, both default to `date.today()` (UTC wall-clock date at request time)
- **Validation**: date strings must match `%Y-%m-%d`; invalid value →  `400 {"errors": [{"message": "start_date time data <val> does not match format %Y-%m-%d"}]}`
- **Auth**: requires internal JWT auth header

---

### Report

#### POST /service/`<service_id>`/report
- **Happy path**: creates a `Report` record in status `REQUESTED`, fires `generate_report.apply_async([report_id, notification_statuses], queue="generate-reports")`, returns `201 {"data": {id, report_type, service_id, language, status, [job_id]}}`
- **Defaults**: `notification_statuses` defaults to `[]` when omitted; `language` must be supplied explicitly
- **Optional fields**: `notification_statuses` (list of status strings), `job_id` (UUID); both can coexist
- **Validation**: `report_type` must be a valid `ReportType` enum value; invalid → `400 {"result": "error", "message": "Invalid report type ..."}`
- **Error cases**: non-existent `service_id` → `404`
- **Auth**: requires internal JWT auth header

#### GET /service/`<service_id>`/report
- **Happy path**: returns `{"data": [...]}` of reports for the service, using `get_reports_for_service(service_id, days_limit)`
- **Defaults**: `days_limit` defaults to `7` when `?limit_days` is not provided
- **Custom limit**: `?limit_days=N` overrides the default
- **Empty state**: returns `{"data": []}`
- **Error cases**: non-existent `service_id` → `404`
- **Auth**: requires internal JWT auth header

---

### Events

#### POST /events
- **Happy path**: body `{"event_type": "<str>", "data": {<arbitrary JSON>}}`; returns `201 {"data": {"event_type": ..., "data": {...}}}`; all fields in `data` are stored and echoed
- **Auth**: requires internal JWT auth header

---

### Email Branding

#### GET /email-branding
- **Happy path**: returns `{"email_branding": [...]}` with all branding records; each record includes `id`, `organisation_id` (empty string `""` when null)
- **Filter**: `?organisation_id=<uuid>` restricts results to brandings whose `organisation_id` matches; without the filter all records are returned regardless of org membership
- **Auth**: internal admin auth (no public API key required)

#### GET /email-branding/`<email_branding_id>`
- **Happy path**: returns `{"email_branding": {colour, logo, name, id, text, brand_type, organisation_id, alt_text_en, alt_text_fr, created_by_id, updated_at, created_at, updated_by_id}}`; `organisation_id` is empty string when null, `alt_text_fr` is `null` when absent
- **Auth**: internal admin auth

#### POST /email-branding (create)
- **Required fields**: `name`, `created_by_id`
- **Optional fields**: `colour` (null if omitted), `logo` (null if omitted), `brand_type`, `alt_text_en`, `alt_text_fr`, `text`
- **Defaults**: `brand_type` defaults to `BRANDING_ORG_NEW` (`custom_logo`); `text` defaults to `name` when not supplied; explicit `null` for `text` is persisted as `null`
- **Happy path**: returns `201 {"data": {...}}`
- **Validation**:
  - Missing `name` → `400 {"errors": [{"message": "name is a required property"}]}`
  - Duplicate `name` → `400 {"message": "Email branding already exists, name must be unique."}`
  - Invalid `brand_type` → `400 {"errors": [{"message": "brand_type <val> is not one of [custom_logo, both_english, both_french, custom_logo_with_background_colour, no_branding]"}]}`
- **Auth**: internal admin auth

#### POST /email-branding/`<email_branding_id>` (update)
- **Happy path**: partial update; any subset of fields may be provided; `updated_by_id` required; returns `200`
- **Validation**:
  - Duplicate `name` (clashes with another branding) → `400 {"message": "Email branding already exists, name must be unique."}`
  - Invalid `brand_type` → `400` with same message as create
- **Auth**: internal admin auth

---

### Letter Branding

#### GET /letter-branding
- **Happy path**: returns JSON array of all letter branding records (serialised via `.serialize()`); empty when none exist
- **Auth**: requires internal JWT auth header

#### GET /letter-branding/`<letter_branding_id>`
- **Happy path**: returns single serialised branding record; `200`
- **Error cases**: unknown id → `404`
- **Auth**: requires internal JWT auth header

#### POST /letter-branding (create)
- **Required fields**: `name`, `filename`
- **Happy path**: inserts record, returns `201 {"id": "<uuid>", ...}`; entity is retrievable by `LetterBranding.query.get(id)`
- **Auth**: requires internal JWT auth header

#### POST /letter-branding/`<letter_branding_id>` (update)
- **Happy path**: updates `name` and/or `filename` for the given brand; `200`
- **Validation**: name collision (IntegrityError) → `400 {"message": {"name": ["Name already in use"]}}`
- **Auth**: requires internal JWT auth header

---

### Platform Stats

#### GET /platform-stats
- **Happy path**: returns notification status totals grouped by channel: `{email: {failures: {virus-scan-failed, temporary-failure, permanent-failure, technical-failure}, total, test-key}, letter: {...}, sms: {...}}`
- **Defaults**: both `start_date` and `end_date` default to today when omitted
- **Filtering**: `?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD`; delegates to `fetch_notification_status_totals_for_all_services`
- **Validation**: invalid date format → `400 {"errors": [{"message": "start_date time data <val> does not match format %Y-%m-%d"}]}`
- **Auth**: internal admin auth

#### GET /platform-stats/usage-for-all-services
- **Happy path**: `?start_date=...&end_date=...`; returns array of per-service objects: `{organisation_id, service_id, sms_cost, sms_fragments, letter_cost, letter_breakdown}`; `organisation_id` is empty string when service has no org; `letter_breakdown` is human-readable (e.g. `"6 second class letters at 45p\n"`)
- **Date constraint**: range must be within a single financial year (April 1 → March 31)
- **Auth**: internal admin auth

#### GET /platform-stats/usage-for-trial-services
- **Happy path**: delegates to `fetch_notification_stats_for_trial_services`; returns array (may be empty)
- **Auth**: internal admin auth

#### GET /platform-stats/send-methods-stats-by-service
- **Happy path**: `?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD`; delegates to `send_method_stats_by_service(start_date, end_date)`; returns array
- **Auth**: internal admin auth

#### Financial-year date validation (`validate_date_range_is_within_a_financial_year`)
- Valid: any range that stays within a single April 1 – March 31 window
- Invalid (crosses FY boundary) → `400 {"message": "Date must be in a single financial year.", "status_code": 400}`
- Invalid (start > end) → `400 {"message": "Start date must be before end date", "status_code": 400}`
- Invalid (non-date string) → `400 {"message": "Input must be a date in the format: YYYY-MM-DD", "status_code": 400}`

---

### Newsletter

#### POST /newsletter/subscribe
- **Happy path (new subscriber)**: body `{"email": "<addr>", "language": "en|fr"}`; creates `NewsletterSubscriber`, sends confirmation email, returns `201 {"result": "success", "subscriber": {...}}`
- **Language default**: omitting `language` defaults to `"en"`
- **Existing subscriber**: updates `language` on the existing record, resends confirmation email, returns `200 {"result": "success", "message": "A subscriber with this email already exists", "subscriber": {...}}`
- **Validation**: missing `email` → `400 {"result": "error", "message": "Email is required"}`
- **Error cases**:
  - Airtable save fails → `500 {"result": "error", "message": "Failed to create unconfirmed mailing list subscriber."}`;  confirmation email NOT sent
  - Airtable API error (non-404 HTTP error) → `500 {"result": "error", "message": "Error fetching existing subscriber ..."}`
- **Auth**: internal admin auth

#### GET /newsletter/confirm/`<subscriber_id>`
- **Happy path**: confirms subscription; returns `200 {"result": "success", "message": "Subscription confirmed", "subscriber": {...}}`
- **Already confirmed**: returns `200 {"result": "success", "message": "Subscription already confirmed", "subscriber": {...}}` without re-calling confirm
- **Resubscribe**: if subscriber status is `UNSUBSCRIBED`, calls `confirm_subscription(has_resubscribed=True)` and returns `200 {"result": "success", "message": "Subscription confirmed"}`
- **Error cases**:
  - Subscriber not found (Airtable 404) → `404 {"result": "error", "message": "Subscriber not found"}`
  - Save fails → `500 {"result": "error", "message": "Subscription confirmation failed"}`
- **Auth**: internal admin auth

#### GET /newsletter/unsubscribe/`<subscriber_id>`
- **Happy path**: returns `200 {"result": "success", "message": "Unsubscribed successfully", "subscriber": {...}}`
- **Already unsubscribed**: returns `200 {"result": "success", "message": "Subscriber has already unsubscribed", "subscriber": {...}}`
- **Error cases**:
  - Subscriber not found → `404 {"result": "error", "message": "Subscriber not found"}`
  - Save fails → `500 {"result": "error", "message": "Unsubscription failed"}`
- **Auth**: internal admin auth

#### POST /newsletter/subscriber/`<subscriber_id>`/language
- **Happy path**: body `{"language": "en|fr"}`; calls `subscriber.update_language(language)`; returns `200 {"result": "success", "message": "Language updated successfully", "subscriber": {...}}`
- **Validation**: missing `language` → `400 {"result": "error", "message": "New language is required"}`
- **Error cases**:
  - Subscriber not found → `404 {"result": "error", "message": "Subscriber not found"}`
  - Save fails → `500 {"result": "error", "message": "Language update failed"}`
- **Auth**: internal admin auth

#### GET /newsletter/send-latest/`<subscriber_id>`
- **Happy path**: subscriber must have status `SUBSCRIBED`; sends latest newsletter; returns `200 {"result": "success", "subscriber": {...}}`
- **Validation**: non-subscribed status → `400 {"result": "error", "message": "Cannot send to subscribers with status: ..."}`
- **Error cases**:
  - Subscriber not found → `404 {"result": "error", "message": "Subscriber not found"}`
  - Airtable API error → `500 {"result": "error", "message": "Failed to fetch subscriber ..."}`
- **Auth**: internal admin auth

#### GET /newsletter/subscriber/`<subscriber_id>`
- **Happy path**: returns subscriber data via `NewsletterSubscriber.from_id`
- **Auth**: internal admin auth

---

### Support

#### GET /support/find-id
- **Required**: `?ids=<uuid>[,<uuid>...]` — comma and/or whitespace-separated UUIDs
- **Validation**:
  - `ids` absent or empty list → `400 {"error": "no ids provided"}`
  - Value that is not a UUID → returns `[{"type": "not a uuid"}]` entry in result array (no 400)
- **Happy path**: returns a JSON array with one entry per ID:
  - User: `{type: "user", id, user_name}`
  - Service: `{type: "service", id, service_name}`
  - Template: `{type: "template", id, template_name, service_id, service_name}`
  - Job: `{type: "job", id, original_file_name, created_by_id, created_by_name, notification_count, job_status, service_id, service_name, template_id, template_name}`
  - Notification: `{type: "notification", id, notification_type, status, to, service_id, service_name, template_id, template_name, job_id, job_row_number, api_key_id}` (`api_key_id` is `null` when not set)
  - Unknown UUID: `{type: "no result found"}`
- **Multiple IDs**: multiple comma/space-separated IDs return one result per ID in order
- **Auth**: internal admin auth

---

### Cache

#### POST /cache/clear
- **Happy path**: calls `redis_store.delete_cache_keys_by_pattern` for every key pattern in `CACHE_KEYS_ALL`; returns `201 {"result": "ok"}`
- **Error cases**: any Redis exception → `500 {"error": "Unable to clear the cache"}`
- **Auth**: requires dedicated cache-clear auth header (distinct from standard JWT)

---

### Status / Healthcheck

#### GET / and GET /_status
- **Happy path**: returns `200 {"status": "ok", "db_version": "<str>", "commit_sha": "<str>", "build_time": "<str>", "current_time_utc": "<str>"}`
- **No auth required**

#### GET /status/live-service-and-organisation-counts
- **Happy path**: returns `{"organisations": <int>, "services": <int>}`
- **Counting rules**:
  - A service is counted as live when: `active=True`, `restricted=False`, `count_as_live=True`
  - Services not attached to any org still count toward the service total
  - An organisation is counted only if it has at least one qualifying live service
  - Trial services, inactive services, and services with `count_as_live=False` are excluded from both counts
- **Auth**: internal admin auth

---

### Cypress (Non-Production Only)

#### POST /cypress/create_user/`<email_suffix>`
- **Happy path**: creates two users with emails `notify-ui-tests+ag_<suffix>@cds-snc.ca` (regular) and `notify-ui-tests+ag_<suffix>_admin@cds-snc.ca` (admin); returns `201 {"regular": {...}, "admin": {...}}`; users are persisted in the database
- **Validation**: `email_suffix` containing non-alphanumeric characters (e.g. dashes) → `400`
- **Environment guard**: `NOTIFY_ENVIRONMENT == "production"` → `403`
- **Auth**: requires dedicated Cypress auth header

#### GET /cypress/cleanup
- **Happy path**: deletes all test users with `created_at` older than 30 days; returns `201 {"message": "Clean up complete"}`; deleted users are no longer retrievable from the database
- **Auth**: requires dedicated Cypress auth header

---

### v2 API Spec

#### GET /v2/openapi-en
- **Happy path**: returns `200`, `Content-Type: application/yaml`, body contains `openapi:`
- **No auth required**

#### GET /v2/openapi-fr
- **Happy path**: returns `200`, `Content-Type: application/yaml`, body contains `openapi:` and `API de Notifications`
- **No auth required**

#### GET /v2/openapi-`<other>`
- Returns `404`

---

## DAO Behavior Contracts

### Complaint DAO

#### `fetch_paginated_complaints(page)`
- Returns a paginated result object (`.items` list, pagination metadata)
- Items sorted descending by `created_at`
- Page size governed by `app.config["PAGE_SIZE"]`

#### `fetch_complaints_by_service(service_id)`
- Returns all complaints for a service sorted descending by `created_at`
- Returns `[]` when no complaints exist

#### `fetch_count_of_complaints(start_date, end_date)`
- Counts complaints whose `created_at` falls within `[start_date midnight, end_date midnight)` (UTC)
- Example: complaints at 22:00 and 23:00 on day N and 00:00 and 13:00 on day N+1, with range `[N+1, N+1]`, returns 2 (the midnight and 13:00 entries)

#### `save_complaint(complaint)`
- Persists a `Complaint` record to the database

#### Service eager-loading regression
- `fetch_paginated_complaints` must joinedload `service` to avoid `DetachedInstanceError` during serialisation of items outside the session

---

### Email Branding DAO

#### `dao_get_email_branding_options(filter_by_organisation_id=None)`
- Without filter: returns all email branding
- With filter: returns only brandings whose `organisation_id` matches

#### `dao_get_email_branding_by_id(id)` → single record by primary key
#### `dao_get_email_branding_by_name(name)` → single record by exact name match
#### `dao_update_email_branding(branding, **fields)` → applies field updates to the record
- `EmailBranding` model has no `domain` attribute

---

### Letter Branding DAO

#### `dao_get_letter_branding_by_id(id)`
- Returns matching record; raises `SQLAlchemyError` if not found

#### `dao_get_all_letter_branding()` → all records; `[]` when table is empty

#### `dao_create_letter_branding(LetterBranding)` → inserts record; `filename` defaults to `name` when only `name` is supplied

#### `dao_update_letter_branding(id, **fields)` → updates by id; change is immediately visible via query

---

### Daily Sorted Letter DAO

#### `dao_get_daily_sorted_letter_by_billing_day(billing_day)`
- Returns record or `None`

#### `dao_create_or_update_daily_sorted_letter(dsl)`
- **Insert**: if no record exists for `billing_day`, creates a new one; `updated_at` is `None`
- **Upsert**: if a record already exists for the same `billing_day` and `file_name`, updates `unsorted_count` and `sorted_count`; sets `updated_at`

---

### Date Utilities

#### `get_financial_year(year)` → `(datetime, datetime)`
- Start: `<year>-04-01 05:00:00` UTC (i.e., midnight Eastern Standard)
- End: `<year+1>-04-01 04:59:59.999999` UTC

#### `get_april_fools(year)` → naive `datetime` at `<year>-04-01 04:00:00`

#### `get_month_start_and_end_date_in_utc(dt)` → `(start, end)` as UTC datetimes
- Accounts for EST (UTC-5) in winter and EDT (UTC-4) in summer

#### `get_financial_year_for_datetime(dt)` → `int`
- `2018-04-01 05:00:00 UTC` → FY 2018 (first moment of new fiscal year)
- `2019-03-31 22:59:59 UTC` → FY 2018 (last moment of old fiscal year)
- Accepts both `date` and `datetime`

#### `get_midnight(dt)` → `datetime`
- Returns midnight of the same calendar date in the same timezone

#### `get_query_date_based_on_retention_period(period_days)` → `datetime`
- Returns `(today - period_days).date` at `23:59:59.999999` (end of the cutoff day)

---

### Events DAO

#### `dao_create_event(event)` → inserts `Event` record; idempotently verifiable via `Event.query.count()`

---

## Cross-Cutting Behavior Verified

### Error Handling

#### Internal API errors (`app.errors`)
- `DuplicateEntityError` base: `status_code=400`, `message="Entity already exists."`
- `CannotSaveDuplicateEmailBrandingError`: `message="Email branding already exists, name must be unique."`
- `CannotSaveDuplicateTemplateCategoryError`: `message="Template category already exists, name_en and name_fr must be unique."`
- Multi-field form: `"<entity> already exists, <f1>, <f2>, ..., and <fN> must be unique."`

#### v2 API error format
All errors from v2 blueprints use `{"status_code": <int>, "errors": [{"error": "<ExcClass>", "message": "<text>"}]}`:

| Exception | HTTP Status | `error` field | `message` field |
|---|---|---|---|
| `AuthError` | 403 | `AuthError` | original message |
| `BadRequestError` | 400 | `BadRequestError` | original message |
| `TooManyRequestsError` | 429 | `TooManyRequestsError` | `"Exceeded send limits (<limit>) for today"` |
| `ValidationError` | 400 | `ValidationError` | per-field messages (multiple entries) |
| `DataError` (SQLAlchemy) | 404 | `DataError` | `"No result found"` |
| `JobIncompleteError` | 500 | `JobIncompleteError` | original message |
| Unhandled exception | 500 | `<ExcClass>` | `"Internal server error"` |
| Wrong HTTP method | 405 | — | `{"message": "The method is not allowed for the requested URL.", "result": "error"}` |

---

### CORS Headers

- **Allowed origins**: whitelisted origins (verified: `https://documentation.notification.canada.ca`); other origins receive no CORS headers
- **On valid origin**: every response (including 401) includes:
  - `Access-Control-Allow-Origin: <origin>`
  - `Access-Control-Allow-Headers: Content-Type,Authorization`
  - `Access-Control-Allow-Methods: GET,PUT,POST,DELETE`
- **OPTIONS preflight**: returns `200` with full CORS headers without requiring authentication
- **Invalid origin**: no `Access-Control-Allow-Origin` header is set; request proceeds normally (may return 401)

---

### Authentication

- Standard internal routes: require JWT signed with the internal API key (`create_authorization_header()`)
- Cache-clear route: uses a dedicated signed header (`create_cache_clear_authorization_header()`)
- Cypress routes: use a dedicated Cypress signed header (`create_cypress_authorization_header()`)
- Health check routes (`/`, `/_status`): no auth required
- v2 public routes: use Bearer token API key auth; unauthorized → `403 AuthError`
- OPTIONS preflight: no authentication required on any route

---

### Configuration

#### Queue Names (`QueueNames.all_queues()`)
Exactly 21 queues:

| Name | Constant |
|---|---|
| `priority` | `PRIORITY` |
| `bulk` | `BULK` |
| `periodic` | `PERIODIC` |
| `priority-database` | `PRIORITY_DATABASE` |
| `normal-database` | `NORMAL_DATABASE` |
| `bulk-database` | `BULK_DATABASE` |
| `send-sms-high` | `SEND_SMS_HIGH` |
| `send-sms-medium` | `SEND_SMS_MEDIUM` |
| `send-sms-low` | `SEND_SMS_LOW` |
| `send-throttled-sms` | `SEND_THROTTLED_SMS` |
| `send-email-high` | `SEND_EMAIL_HIGH` |
| `send-email-medium` | `SEND_EMAIL_MEDIUM` |
| `send-email-low` | `SEND_EMAIL_LOW` |
| `research-mode` | `RESEARCH_MODE` |
| `reporting` | `REPORTING` |
| `jobs` | `JOBS` |
| `retry` | `RETRY` |
| `service-callbacks-retry` | `CALLBACKS_RETRY` |
| `notify-internal-tasks` | `NOTIFY` |
| `service-callbacks` | `CALLBACKS` |
| `delivery-receipts` | `DELIVERY_RECEIPTS` |

Note: `CREATE_LETTERS_PDF` and `LETTERS` are commented out and not included in `all_queues()`.

#### Delivery Queue Routing
| Channel | Process Type | Queue |
|---|---|---|
| SMS | `normal` | `SEND_SMS_MEDIUM` |
| SMS | `priority` | `SEND_SMS_HIGH` |
| SMS | `bulk` | `SEND_SMS_LOW` |
| email | `normal` | `SEND_EMAIL_MEDIUM` |
| email | `priority` | `SEND_EMAIL_HIGH` |
| email | `bulk` | `SEND_EMAIL_LOW` |

#### SQLAlchemy Pool
- `SQLALCHEMY_DISABLE_POOL=true` (env var) → `poolclass=NullPool`; `SQLALCHEMY_POOL_SIZE=None`, `SQLALCHEMY_POOL_TIMEOUT=None`, `SQLALCHEMY_ENGINE_OPTIONS` contains `poolclass`
- Default (env var absent): `SQLALCHEMY_DISABLE_POOL=False`, `SQLALCHEMY_ENGINE_OPTIONS={}`

#### Safe/Sensitive Config
- `Config.get_safe_config()` calls `get_class_attrs` and `get_sensitive_config`
- `Config.get_sensitive_config()` returns a non-empty dict where every key is truthy

---

### Queue (Redis-backed Buffer)

#### `Buffer` naming
- `Buffer.INBOX.inbox_name()` → `"inbox"`
- `Buffer.INBOX.inbox_name("sfx")` → `"inbox:sfx"`
- `Buffer.INBOX.inbox_name("sfx", "normal")` → `"inbox:sfx:normal"`
- `Buffer.INBOX.inflight_name(receipt)` → `"in-flight:<receipt>"`
- `Buffer.INBOX.inflight_name(receipt, "sfx")` → `"in-flight:sfx:<receipt>"`
- `Buffer.INBOX.inflight_name(receipt, "sfx", "normal")` → `"in-flight:sfx:normal:<receipt>"`

#### `RedisQueue` operations
- `publish(element)` → LPUSH to inbox; emits `batch_saving_published` CloudWatch metric
- `poll(count)` → atomically RPOPLPUSH up to `count` items from inbox → new inflight list; returns `(receipt_uuid, elements)`;  `count ≤ 0` returns `(receipt, [])` without consuming
- `acknowledge(receipt)` → deletes inflight list; returns `True` on success, `False` if receipt unknown
- `expire_inflights()` → returns any inflight items whose TTL has expired back to the inbox; non-inflight keys are not affected
- Messages are stored and retrieved as plain strings (no serialisation wrapper added by the queue)

#### `MockQueue` (test double)
- `poll(n)` always returns `n` freshly generated elements regardless of prior `publish` calls
- `publish()` and `acknowledge()` are no-ops

---

### Healthcheck (Status) Details

- `GET /` and `GET /_status` are equivalent
- Response always includes `db_version` (confirms DB connectivity), `commit_sha`, `build_time`, `current_time_utc`

---

### Celery Error Classification

#### Categories (`CeleryErrorCategory`)
| Category | Classification Trigger |
|---|---|
| `THROTTLING` | Exception class name contains `ThrottlingException`, or message contains throttling keyword |
| `DUPLICATE_RECORD` | `IntegrityError`, class name `UniqueViolation`, or message contains `"duplicate key value violates unique constraint"` |
| `JOB_INCOMPLETE` | Exception class name is `JobIncompleteError` |
| `NOTIFICATION_NOT_FOUND` | Class name `NoResultFound`, or message contains `"notifications not found for SES references:"` |
| `SHUTDOWN` | Message contains `"SIGKILL"` |
| `TIMEOUT` | Message contains `"timeout-sending-notifications"` |
| `TASK_RETRY` | `celery.exceptions.Retry`, or message contains `"Retry in"` |
| `UNKNOWN` | No other rule matches, or exception is `None` |

#### Chain-walking rules
- Walks `__cause__` first, then `__context__`; deepest matching exception wins
- Circular exception chains are detected via a visited-set; loop is broken without infinite recursion
- When the deepest exception matches, that exception is returned as `root_exc`

#### Classification precedence within one exception
- Class-name patterns take priority over message-substring patterns

#### Signal handlers
- `task_retry(sender, reason, request)`:
  - If `reason` is an `Exception`: classifies it and logs `CELERY_KNOWN_ERROR::<CATEGORY>` with `task_name` and `task_id`
  - If `reason` is not an `Exception`: logs `CELERY_UNKNOWN_ERROR`
  - `None` sender/request: logs with `task_name=unknown`
- `task_failure`, `task_internal_error`, `task_unknown` — signal handlers also registered (implementation mirrors `task_retry`)

---

### Service Callback Tasks

#### `send_delivery_status_to_service(notification_id, signed_status_update, service_id)`
- Calls `POST <callback_url>` with:
  - Body: `{id, reference, to, status, status_description, provider_response, created_at, completed_at, sent_at, notification_type}`
  - `Content-type: application/json`
  - `Authorization: Bearer <token>`
- Retries on HTTP `429`, `500`, `503` → retry queued on `service-callbacks-retry`
- Does **not** retry on HTTP `404`
- `sent_at` may be `null` (e.g., for `technical-failure` notifications)
- Supports `email`, `letter`, `sms` notification types

#### `send_complaint_to_service(complaint_data, service_id)`
- Calls `POST <callback_url>` with:
  - Body: `{notification_id, complaint_id, reference, to, complaint_date}`
  - `Content-type: application/json`
  - `Authorization: Bearer <token>`

Both tasks receive *signed* payloads; the signing uses `signer_delivery_status` / `signer_complaint` respectively.

---

### AWS Clients

#### S3 (`app.aws.s3`)

| Function | Behavior |
|---|---|
| `get_s3_file(bucket, key)` | Delegates to `get_s3_object(bucket, key)` |
| `remove_transformed_dvla_file(uuid)` | Calls `get_s3_object(DVLA_BUCKETS["job"], "<uuid>-dvla-job.text").delete()` |
| `get_s3_bucket_objects(bucket, subfolder)` | Paginates via `client().get_paginator().paginate(Bucket=bucket, Prefix=subfolder)`; returns list of `{ETag, Key, LastModified}` |
| `filter_s3_bucket_objects_within_date_range(objects)` | Excludes folder stub objects (key ending in `/`); includes only items where `start_date < LastModified < end_date` (exclusive boundaries, window = last 7–9 days from now) |
| `get_list_of_files_by_suffix(bucket, subfolder, suffix, last_modified)` | Case-insensitive suffix filter; `last_modified` restricts to items younger than that datetime |
| `upload_job_to_s3(service_id, csv_data)` | Uploads to `CSV_UPLOAD_BUCKET_NAME` at path `service-<service_id>-notify/<upload_id>.csv`; returns `upload_id` |
| `remove_jobs_from_s3(jobs, batch_size)` | Batch-deletes `service-<service_id>-notify/<job_id>.csv` objects in chunks of `batch_size` |
| `upload_report_to_s3(service_id, report_id, csv_data)` | Uploads to `REPORTS_BUCKET_NAME` at `service-<service_id>/<report_id>.csv`; generates presigned URL with `expiration=259200` s (3 days) |
| `generate_presigned_url(bucket, key, expiration)` | Returns presigned URL string; returns `None` (falsy) on `ClientError` |
| `stream_to_s3(bucket, key, copy_command, cursor)` | Uses PostgreSQL `cursor.copy_expert(copy_command, buffer)` to stream result set; uploads via `s3_client.upload_fileobj` |

#### Metrics Logger (`app.aws.metrics_logger.MetricsLogger`)
- Default environment: `EC2Environment`
- When `AWS_EXECUTION_ENV` env var is set: `LambdaEnvironment`
- `"local"` environment: `flush()` writes to stdout

#### CloudWatch Batch Metrics (`app.aws.metrics`)
All functions are no-ops when `metrics_logger.metrics_config.disable_metric_extraction = True`.
`ClientError` during `flush()` is caught and logged as a `warning`.

| Function | Dimension keys | Metric name |
|---|---|---|
| `put_batch_saving_metric(logger, queue, count)` | `{list_name: queue._inbox}` | `batch_saving_published` |
| `put_batch_saving_inflight_metric(logger, queue, count)` | `{created: "True", notification_type: queue._suffix, priority: queue._process_type}` | `batch_saving_inflight` |
| `put_batch_saving_inflight_processed(logger, queue, count)` | `{acknowledged: "True", notification_type, priority}` | `batch_saving_inflight` |
| `put_batch_saving_expiry_metric(logger, queue, count)` | two calls: `{expired: "True", notification_type, priority}` then `{expired: "True", notification_type: "any", priority: "any"}` | `batch_saving_inflight` |
| `put_batch_saving_bulk_created(logger, count, type, priority)` | `{created: "True", notification_type, priority}` | `batch_saving_bulk` |
| `put_batch_saving_bulk_processed(logger, count, type, priority)` | `{acknowledged: "True", notification_type, priority}` | `batch_saving_bulk` |

---

### Cypress Test Routes

- Only callable in non-production environments; `NOTIFY_ENVIRONMENT == "production"` returns `403`
- Managed user email prefix: `notify-ui-tests+ag_`
- Users created in the Cypress service (fixture `sample_service_cypress`)
- Cleanup removes users with `created_at` > 30 days ago

---

### JSON Provider

`NotifyJSONProvider` serialises `sqlalchemy.engine.row.Row` objects by calling `._asdict()` and encoding the resulting dict as JSON.

---

### User-Agent Processing (`process_user_agent`)

| Input | Output |
|---|---|
| `"NOTIFY-API-PYTHON-CLIENT/3.0.0"` | `"notify-api-python-client.3-0-0"` |
| `None` | `"unknown"` |
| Any other string (browser UA, non-matching) | `"non-notify-user-agent"` |

Matching rule: input must exactly follow `^NOTIFY-API-PYTHON-CLIENT/\d+\.\d+\.\d+$`; version dots converted to hyphens.

---

### Cronitor Monitoring

- Decorator: `@cronitor("task_name")`
- `CRONITOR_ENABLED=True` + `CRONITOR_KEYS={"task_name": "secret"}`:
  - Before execution: `GET https://cronitor.link/<key>/run?host=<NOTIFY_HOST>`
  - On success: `GET https://cronitor.link/<key>/complete?host=...`
  - On exception: `GET https://cronitor.link/<key>/fail?host=...`; original exception is re-raised
- `CRONITOR_ENABLED=False`: decorator is transparent; no HTTP calls made

---

### Annotations (`app.annotations`)

#### `@unsign_params`
- Inspects type hints of each parameter using `typing.get_type_hints`
- `SignedNotification` typed parameter → calls `signer_notification.verify(value)`; raises `BadSignature` if signed with a different key
- `SignedNotifications` (list) typed parameter → verifies each element
- Parameters without a `Signed*` type hint → passed through unchanged

#### `@sign_return`
- Signs the return value with `signer_notification.sign()`
- List return → signs each element individually
- Empty list → returns `[]`
- Non-string return (e.g. integer) → returned unchanged

---

### Schemas (`app.schemas`)

| Schema | Verified Behaviors |
|---|---|
| `job_schema` | Does not serialise the `notifications` relationship |
| `notification_with_template_schema` | Includes `key_name` (`null` when no API key attached) |
| `notification_schema` / `notification_with_personalisation_schema` | Always includes `status` field |
| `user_update_schema_load_json` | Accepts: `name`, `email_address`, `mobile_number`, `blocked`; rejects `None`/empty `name`, malformed email, short phone; rejects disallowed fields with `"Unknown field name <field>"` error (disallowed: `id`, `updated_at`, `created_at`, `user_to_service`, `_password`, `verify_codes`, `logged_in_at`, `password_changed_at`, `failed_login_count`, `state`, `platform_admin`) |
| `provider_details_schema` | Nested `created_by` returns `{id, email_address, name}` |
| `service_schema` | Includes `sms_annual_limit` and `email_annual_limit` |

---

### Model Behaviors (`app.models`)

#### `Notification.substitute_status(statuses)`
- `"failed"` (or `["failed"]`) expands to all `NOTIFICATION_STATUS_TYPES_FAILED` (deduplicates)
- `NOTIFICATION_STATUS_LETTER_ACCEPTED` → `[sending, created]`
- `NOTIFICATION_STATUS_LETTER_RECEIVED` → `[delivered]`
- Mixed lists are expanded and deduplicated

#### `Notification.serialize_for_csv()`
- `row_number` = `job_row_number + 1`
- `created_at` converted to Eastern time, formatted `YYYY-MM-DD HH:MM:SS`
- `status` field is human-readable:

| Channel | Raw status | Human status |
|---|---|---|
| email | `failed` | `"Failed"` |
| email | `technical-failure` | `"Tech issue"` |
| email | `temporary-failure` | `"Content or inbox issue"` |
| email | `permanent-failure` | `"No such address"` |
| email | `permanent-failure` + subtype `suppressed` or `on-account-suppression-list` | `"Blocked"` |
| sms | `temporary-failure` | `"Carrier issue"` |
| sms | `permanent-failure` | `"No such number"` |
| sms | `provider-failure` + reason `DESTINATION_COUNTRY_BLOCKED` or `NO_ORIGINATION_IDENTITIES_FOUND` | `"Can't send to this international number"` |
| sms | `sent` | `"Sent"` |
| letter | `created` or `sending` | `"Accepted"` |
| letter | `technical-failure` | `"Technical failure"` |
| letter | `delivered` | `"Received"` |

#### `Notification.personalisation`
- Getter: returns `{}` when internal value is `None` or `{}`
- Setter: always stores signed empty dict when input is `None` or `{}`

#### `Notification.subject`
- SMS: always `None`
- Email / letter: fills personalisation placeholders in the template subject

#### `Notification` requires a valid template version; mismatched version raises `IntegrityError`

#### `ServiceSafelist.from_string(service_id, type, contact)`
- `EMAIL_TYPE`: validates email address format; invalid → `ValueError`
- `MOBILE_TYPE`: validates phone number format; invalid → `ValueError`

#### `Service.get_inbound_number()` → returns inbound number string or `None`
#### `Service.get_default_reply_to_email_address()` → returns default reply-to email
#### `Service.get_default_letter_contact()` → returns default letter contact block

---

### Utils (`app.utils`)

| Function | Behavior |
|---|---|
| `get_local_timezone_midnight(dt)` | Converts UTC datetime to the midnight of that local Eastern calendar day |
| `get_local_timezone_midnight_in_utc(dt)` | Returns the UTC equivalent of local Eastern midnight for the given date |
| `get_midnight_for_day_before(dt)` | Returns local midnight of the day before `dt`, expressed as UTC |
| `midnight_n_days_ago(n)` | Returns local midnight `n` days ago, expressed as UTC |
| `get_logo_url(filename)` | `https://assets.notification.canada.ca/<filename>` |
| `get_document_url(language, path)` | `https://documentation.notification.canada.ca/<language>/<path>` |
| `get_limit_reset_time_et()` | Returns `{12hr: "8PM", 24hr: "20"}` during EDT; `{12hr: "7PM", 24hr: "19"}` during EST |
| `get_fiscal_year(dt)` | Returns fiscal year int; April 1 is start of new FY; `None` → current FY |
| `get_fiscal_dates(current_date, year)` | Returns `(FY_start, FY_end)` as `datetime`; raises `ValueError` if both `current_date` and `year` are provided |
| `update_dct_to_str(dct, lang)` | Formats update dict as human-readable change list; supports `EN` and `FR` |
| `rate_limit_db_calls(prefix, period_seconds)` | Decorator; checks Redis key `<prefix>:<id>`; returns `None` if key exists (rate-limited); sets key on first call; transparent when `REDIS_ENABLED=False` |
| `prepare_billable_units_counts_for_seeding(data)` | Aggregates SMS-only records by status into `{sms_billable_units_delivered_today, sms_billable_units_failed_today}`; non-SMS rows are ignored |

---

### Performance Platform Commands

#### `backfill_processing_time(start_date, end_date)` (Click command)
- Calls `send_processing_time_for_start_and_end(day_start_utc, day_end_utc)` once per calendar day in `[start_date, end_date]`
- Day boundaries shifted to 04:00 UTC (Eastern midnight equivalent)

#### `backfill_performance_platform_totals(start_date, end_date)` (Click command)
- Calls `send_total_sent_notifications_to_performance_platform(day)` once per calendar day in `[start_date, end_date]`

---

### Report Utils (`app.report.utils`)

#### `generate_csv_from_notifications(service_id, notification_type, language, notification_statuses, job_id, days_limit, s3_bucket, s3_key)`
- Calls `build_notifications_query(...)` then `compile_query_for_copy(query)` then `stream_query_to_s3(copy_command, s3_bucket, s3_key)`

#### `build_notifications_query(service_id, notification_type, language, notification_statuses, job_id, days_limit)`
- With non-empty `notification_statuses`: adds `WHERE notification_status IN (...)` using expanded status set (e.g. `"failed"` → all derived failure statuses)
- With empty `notification_statuses=[]`: no status filter
- With `job_id`: adds `WHERE job_id = '<id>'`

#### `Translate(language).translate(key)`
- `language="en"`: returns key unchanged
- `language="fr"`: maps known keys (e.g. `Recipient → Destinataire`, `Template → Gabarit`); unknown keys returned unchanged

#### `send_requested_report_ready(report)`
- Looks up the report template and service; persists a notification; calls `send_notification_to_queue(notification, False, queue="notify-internal-tasks")`
