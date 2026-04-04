## ADDED Requirements

### Requirement PAF-1: Cache-Clear Endpoint
`POST /cache/clear` SHALL delete all Redis keys matching every pattern in `CACHE_KEYS_ALL` and return `201 {"result": "ok"}`. Auth SHALL use the dedicated cache-clear JWT header (distinct from admin JWT). Any Redis exception during key deletion SHALL return `500 {"error": "Unable to clear the cache"}`.

#### Scenario: Cache clear succeeds
- **WHEN** `POST /cache/clear` is called with a valid cache-clear JWT
- **THEN** `redis_store.delete_cache_keys_by_pattern` is called for each pattern in `CACHE_KEYS_ALL` and the response is `201 {"result": "ok"}`

#### Scenario: Redis exception returns 500
- **WHEN** `POST /cache/clear` is called and the Redis client raises an exception
- **THEN** the response is `500 {"error": "Unable to clear the cache"}`

#### Scenario: Admin JWT is rejected on cache-clear endpoint
- **WHEN** `POST /cache/clear` is called with a standard admin JWT instead of the cache-clear JWT
- **THEN** the response is `401`

---

### Requirement PAF-2: Healthcheck Endpoints
`GET /` and `GET /_status` SHALL both return `200 {"status": "ok", "db_version": "<str>", "commit_sha": "<str>", "build_time": "<str>", "current_time_utc": "<str>"}` without requiring any authentication.

#### Scenario: GET / returns status object
- **WHEN** `GET /` is called with no auth header
- **THEN** the response is `200` with a JSON body containing `status`, `db_version`, `commit_sha`, `build_time`, and `current_time_utc`

#### Scenario: GET /_status is equivalent to GET /
- **WHEN** `GET /_status` is called with no auth header
- **THEN** the response body is identical to the `GET /` response

---

### Requirement PAF-3: Live Service and Organisation Counts (Admin)
`GET /status/live-service-and-organisation-counts` SHALL return `{"organisations": <int>, "services": <int>}`. A service is counted as live when `active=True`, `restricted=False`, and `count_as_live=True`. An organisation is counted only when it has at least one qualifying live service.

#### Scenario: Only qualifying services and their orgs are counted
- **WHEN** `GET /status/live-service-and-organisation-counts` is called
- **THEN** the response contains counts for only active, unrestricted, count-as-live services and their parent organisations

#### Scenario: Service with no org is counted in service total
- **WHEN** a live service has no parent organisation
- **THEN** it is included in the `services` count and does not affect `organisations` count

#### Scenario: Trial and inactive services are excluded
- **WHEN** services exist with `restricted=True` or `count_as_live=False` or `active=False`
- **THEN** they are excluded from both counts

---

### Requirement PAF-4: SRE Tool Endpoints (SRE JWT)
Endpoints under `/sre-tools/` SHALL require the SRE JWT (`RequireSRE` middleware). `GET /sre-tools/live-service-and-organisation-counts` SHALL return the same counts as `GET /status/live-service-and-organisation-counts`.

#### Scenario: SRE JWT grants access to SRE endpoint
- **WHEN** `GET /sre-tools/live-service-and-organisation-counts` is called with a valid SRE JWT
- **THEN** the response is `200 {"organisations": <int>, "services": <int>}`

#### Scenario: Admin JWT is rejected on SRE endpoint
- **WHEN** `GET /sre-tools/live-service-and-organisation-counts` is called with a standard admin JWT
- **THEN** the response is `401`

---

### Requirement PAF-5: Cypress Create User (Non-Production)
`POST /cypress/create_user/<email_suffix>` SHALL create two users (`notify-ui-tests+ag_<suffix>@cds-snc.ca` regular and `notify-ui-tests+ag_<suffix>_admin@cds-snc.ca` admin) and return `201 {"regular": {...}, "admin": {...}}`. The endpoint SHALL return `403` when `NOTIFY_ENVIRONMENT == "production"`, checked before auth.

#### Scenario: Valid suffix creates both users in non-production
- **WHEN** `POST /cypress/create_user/testsuffix` is called with a valid Cypress JWT in a non-production environment
- **THEN** both `notify-ui-tests+ag_testsuffix@cds-snc.ca` and `notify-ui-tests+ag_testsuffix_admin@cds-snc.ca` are created in the database and the response is `201 {"regular": {...}, "admin": {...}}`

#### Scenario: Non-alphanumeric suffix is rejected
- **WHEN** `POST /cypress/create_user/test-suffix` is called (suffix contains a dash)
- **THEN** the response is `400`

#### Scenario: Production environment blocks regardless of auth
- **WHEN** `POST /cypress/create_user/testsuffix` is called in production (`NOTIFY_ENVIRONMENT == "production"`)
- **THEN** the response is `403` before the Cypress JWT is verified and no users are created

#### Scenario: Cypress JWT is required in non-production
- **WHEN** `POST /cypress/create_user/testsuffix` is called in non-production without a Cypress JWT
- **THEN** the response is `401`

---

### Requirement PAF-6: Cypress Cleanup (Non-Production)
`GET /cypress/cleanup` SHALL delete all test users with `created_at` older than 30 days and return `201 {"message": "Clean up complete"}`. The same production guard and Cypress JWT auth requirement apply.

#### Scenario: Cleanup removes stale test users
- **WHEN** `GET /cypress/cleanup` is called in a non-production environment
- **THEN** all test users with `created_at` older than 30 days are deleted and the response is `201 {"message": "Clean up complete"}`

#### Scenario: Production blocks cleanup
- **WHEN** `GET /cypress/cleanup` is called in production
- **THEN** the response is `403` and no users are deleted

---

### Requirement PAF-7: Events Endpoint
`POST /events` SHALL accept `{"event_type": "<str>", "data": {<arbitrary JSON>}}`, persist the event record, and return `201 {"data": {"event_type": ..., "data": {...}}}`. All fields in `data` SHALL be stored and echoed without modification.

#### Scenario: Event is persisted and echoed
- **WHEN** `POST /events` is called with `{"event_type": "foo", "data": {"key": "value"}}`
- **THEN** the response is `201 {"data": {"event_type": "foo", "data": {"key": "value"}}}` and the event row is retrievable from the database

#### Scenario: Arbitrary nested data is stored verbatim
- **WHEN** `POST /events` is called with a deeply nested `data` object
- **THEN** the response `data` field equals the input `data` object exactly

---

### Requirement PAF-8: Email Branding List
`GET /email-branding` SHALL return `{"email_branding": [...]}` with all branding records. An optional `?organisation_id=<uuid>` filter restricts results to brandings whose `organisation_id` matches. `organisation_id` SHALL be serialised as empty string `""` when null.

#### Scenario: All brandings returned when no filter
- **WHEN** `GET /email-branding` is called with no query params
- **THEN** all `EmailBranding` records are returned in `{"email_branding": [...]}`

#### Scenario: Organisation filter returns scoped brandings only
- **WHEN** `GET /email-branding?organisation_id=<uuid>` is called
- **THEN** only brandings whose `organisation_id` equals that UUID are returned

#### Scenario: Null organisation_id serialised as empty string
- **WHEN** a branding record has `organisation_id = null`
- **THEN** the response includes `"organisation_id": ""`

---

### Requirement PAF-9: Email Branding Get by ID
`GET /email-branding/<email_branding_id>` SHALL return `{"email_branding": {colour, logo, name, id, text, brand_type, organisation_id, alt_text_en, alt_text_fr, created_by_id, updated_at, created_at, updated_by_id}}`. `organisation_id` is empty string when null; `alt_text_fr` is `null` when absent.

#### Scenario: Known branding returned with all fields
- **WHEN** `GET /email-branding/<id>` is called for an existing record
- **THEN** HTTP 200 with the full branding object including `organisation_id` (empty string if null) and `alt_text_fr` (null if absent)

#### Scenario: Unknown ID returns 404
- **WHEN** `GET /email-branding/<id>` is called with an ID that does not exist
- **THEN** HTTP 404

---

### Requirement PAF-10: Email Branding Create
`POST /email-branding` SHALL create an `EmailBranding` record with `name` and `created_by_id` required. `brand_type` defaults to `custom_logo`; `text` defaults to `name` when not supplied. Duplicate `name` returns `400`. Missing `name` returns `400`. Invalid `brand_type` returns `400`.

#### Scenario: Valid creation returns 201
- **WHEN** `POST /email-branding` is called with `{"name": "My Brand", "created_by_id": "<uuid>"}`
- **THEN** the response is `201 {"data": {...}}` and the new branding is retrievable

#### Scenario: Missing name returns 400
- **WHEN** `POST /email-branding` is called without a `name` field
- **THEN** the response is `400 {"errors": [{"message": "name is a required property"}]}`

#### Scenario: Duplicate name returns 400
- **WHEN** `POST /email-branding` is called with a `name` that already exists
- **THEN** the response is `400 {"message": "Email branding already exists, name must be unique."}`

#### Scenario: Invalid brand_type returns 400
- **WHEN** `POST /email-branding` is called with an unrecognised `brand_type` value
- **THEN** the response is `400 {"errors": [{"message": "brand_type <val> is not one of [custom_logo, both_english, both_french, custom_logo_with_background_colour, no_branding]"}]}`

#### Scenario: text defaults to name when not supplied
- **WHEN** `POST /email-branding` is called without a `text` field
- **THEN** the created record has `text` equal to the value of `name`

---

### Requirement PAF-11: Email Branding Update
`POST /email-branding/<email_branding_id>` SHALL perform a partial update; any subset of fields may be provided; `updated_by_id` is required. Duplicate `name` returns `400`. Falsy field values are persisted as `NULL`.

#### Scenario: Partial update succeeds
- **WHEN** `POST /email-branding/<id>` is called with `{"colour": "#FF0000", "updated_by_id": "<uuid>"}`
- **THEN** the response is `200` and the branding record's `colour` field is updated

#### Scenario: Duplicate name on update returns 400
- **WHEN** `POST /email-branding/<id>` is called with a `name` that belongs to a different branding record
- **THEN** the response is `400 {"message": "Email branding already exists, name must be unique."}`

#### Scenario: Falsy field value is stored as NULL
- **WHEN** `POST /email-branding/<id>` is called with `{"colour": "", "updated_by_id": "<uuid>"}`
- **THEN** the branding record's `colour` column is set to `NULL`

---

### Requirement PAF-12: Letter Branding List
`GET /letter-branding` SHALL return a JSON array of all letter branding records ordered `name ASC`. An empty array is returned when no records exist.

#### Scenario: All letter brandings returned ordered by name
- **WHEN** `GET /letter-branding` is called and multiple records exist
- **THEN** the response is a JSON array of all records serialised via `.serialize()`, ordered `name ASC`

#### Scenario: Empty array when no records exist
- **WHEN** `GET /letter-branding` is called and no letter branding records exist
- **THEN** the response is `[]`

---

### Requirement PAF-13: Letter Branding Get by ID
`GET /letter-branding/<letter_branding_id>` SHALL return a single serialised branding record. An unknown ID returns `404`.

#### Scenario: Known branding returned
- **WHEN** `GET /letter-branding/<id>` is called for an existing record
- **THEN** HTTP 200 with the serialised branding object

#### Scenario: Unknown ID returns 404
- **WHEN** `GET /letter-branding/<id>` is called with an ID that does not exist
- **THEN** HTTP 404

---

### Requirement PAF-14: Letter Branding Create
`POST /letter-branding` SHALL create a `LetterBranding` record with `name` and `filename` required and return `201 {"id": "<uuid>", ...}`.

#### Scenario: Valid creation returns 201
- **WHEN** `POST /letter-branding` is called with `{"name": "Crown", "filename": "crown.svg"}`
- **THEN** the response is `201` with the new record including its generated `id`, and the record is retrievable via `GET /letter-branding/<id>`

#### Scenario: Created record is immediately retrievable
- **WHEN** a letter branding is created via `POST /letter-branding`
- **THEN** `GET /letter-branding/<returned_id>` returns `200` with the same record

---

### Requirement PAF-15: Letter Branding Update
`POST /letter-branding/<letter_branding_id>` SHALL update `name` and/or `filename` and return `200`. A name collision SHALL return `400 {"message": {"name": ["Name already in use"]}}`.

#### Scenario: Name update succeeds
- **WHEN** `POST /letter-branding/<id>` is called with `{"name": "New Name"}`
- **THEN** the response is `200` and the record's `name` is updated

#### Scenario: Filename update succeeds
- **WHEN** `POST /letter-branding/<id>` is called with `{"filename": "new.svg"}`
- **THEN** the response is `200` and the record's `filename` is updated

#### Scenario: Name collision returns 400
- **WHEN** `POST /letter-branding/<id>` is called with a `name` already used by another record
- **THEN** the response is `400 {"message": {"name": ["Name already in use"]}}`

---

### Requirement PAF-16: Complaint List
`GET /complaint` SHALL return `{"complaints": [...]}` sorted descending by `created_at`. Page-based pagination with optional `?page=N`; when results exceed `PAGE_SIZE`, the response SHALL include `{"links": {"prev", "next", "last"}}` with absolute paths. The associated service object SHALL be eager-loaded to avoid N+1.

#### Scenario: Complaints returned sorted descending
- **WHEN** `GET /complaint` is called and complaints exist across multiple services
- **THEN** the response is `{"complaints": [...]}` with items sorted `created_at DESC`

#### Scenario: Empty state returns empty list
- **WHEN** no complaints exist
- **THEN** the response is `{"complaints": []}`

#### Scenario: Pagination links present when page_size exceeded
- **WHEN** the total number of complaints exceeds `PAGE_SIZE` and `?page=1` is requested
- **THEN** the response includes `{"links": {"prev": ..., "next": "/complaint?page=2", "last": ...}}`

---

### Requirement PAF-17: Complaint Count
`GET /complaint/count` SHALL return an integer count of complaints within the given date range (not wrapped in an object). Both dates default to today when omitted. Date boundaries use `America/Toronto` timezone midnight. Invalid date format returns `400`.

#### Scenario: Count within date range returned as integer
- **WHEN** `GET /complaint/count?start_date=2025-01-01&end_date=2025-01-31` is called
- **THEN** the response body is an integer count (not a JSON object)

#### Scenario: Default to today when dates omitted
- **WHEN** `GET /complaint/count` is called with no query params
- **THEN** both `start_date` and `end_date` default to today and the count reflects complaints from today

#### Scenario: Count is inclusive on both day boundaries
- **WHEN** complaints exist at 22:00 and 23:00 on day N and at 00:00 and 13:00 on day N+1 (UTC), and the query is `[N+1, N+1]`
- **THEN** the count is 2 (midnight and 13:00 entries on the America/Toronto date N+1)

#### Scenario: Invalid date format returns 400
- **WHEN** `GET /complaint/count?start_date=not-a-date` is called
- **THEN** the response is `400 {"errors": [{"message": "start_date time data not-a-date does not match format %Y-%m-%d"}]}`

---

### Requirement PAF-18: Platform Stats Overview
`GET /platform-stats` SHALL return notification status totals grouped by channel (`email`, `letter`, `sms`). Both `start_date` and `end_date` default to today when omitted. Invalid date format returns `400`.

#### Scenario: Stats returned grouped by channel
- **WHEN** `GET /platform-stats?start_date=2025-01-01&end_date=2025-01-31` is called
- **THEN** the response is `{email: {failures: {virus-scan-failed, temporary-failure, permanent-failure, technical-failure}, total, test-key}, letter: {...}, sms: {...}}`

#### Scenario: Default to today when dates omitted
- **WHEN** `GET /platform-stats` is called with no query params
- **THEN** both dates default to today and stats reflect today's notifications

#### Scenario: Invalid date format returns 400
- **WHEN** `GET /platform-stats?start_date=baddate` is called
- **THEN** the response is `400 {"errors": [{"message": "start_date time data baddate does not match format %Y-%m-%d"}]}`

---

### Requirement PAF-19: Platform Stats Usage for All Services
`GET /platform-stats/usage-for-all-services` SHALL return per-service usage data for a given date range. The range SHALL fall within a single financial year (April 1 → March 31 local time); cross-year ranges return `400`. Services present only in letter data SHALL be included. Sorting: blank org last, then org name, then service name.

#### Scenario: Per-service usage returned
- **WHEN** `GET /platform-stats/usage-for-all-services?start_date=2025-04-01&end_date=2025-04-30` is called
- **THEN** the response is an array of objects with `organisation_id`, `service_id`, `sms_cost`, `sms_fragments`, `letter_cost`, `letter_breakdown`; `organisation_id` is `""` when the service has no org

#### Scenario: Cross-financial-year range returns 400
- **WHEN** the query spans April 1 (new FY start) — e.g. `start_date=2025-03-01&end_date=2025-04-30`
- **THEN** the response is `400 {"message": "Date must be in a single financial year.", "status_code": 400}`

#### Scenario: start_date > end_date returns 400
- **WHEN** `start_date=2025-05-01&end_date=2025-04-01` is supplied
- **THEN** the response is `400 {"message": "Start date must be before end date", "status_code": 400}`

#### Scenario: Non-date string returns 400
- **WHEN** `start_date=invalid` is supplied
- **THEN** the response is `400 {"message": "Input must be a date in the format: YYYY-MM-DD", "status_code": 400}`

#### Scenario: Letter-only service is included in response
- **WHEN** a service has letter costs but no SMS costs in the given period
- **THEN** that service still appears in the response array with `sms_cost=0` and non-zero `letter_cost`

---

### Requirement PAF-20: Platform Stats Trial Services
`GET /platform-stats/usage-for-trial-services` SHALL return notification stats for trial-mode services only. No date parameters are accepted. The result may be an empty array.

#### Scenario: Trial service stats returned
- **WHEN** `GET /platform-stats/usage-for-trial-services` is called and trial services have notification activity
- **THEN** the response is a non-empty array containing stats for trial services only

#### Scenario: Empty array when no trial services have activity
- **WHEN** no trial services have sent notifications
- **THEN** the response is `[]`

---

### Requirement PAF-21: Platform Stats Send-Method Breakdown
`GET /platform-stats/send-methods-stats-by-service` SHALL accept `?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD` and return an array of per-service send-method stats.

#### Scenario: Send-method stats returned for date range
- **WHEN** `GET /platform-stats/send-methods-stats-by-service?start_date=2025-01-01&end_date=2025-01-31` is called
- **THEN** the response is an array of objects with per-service send-method counts

#### Scenario: Invalid date format returns 400
- **WHEN** an invalid date string is passed as `start_date` or `end_date`
- **THEN** the response is `400` with a date format error message

---

### Requirement PAF-22: Support Find-ID Lookup
`GET /support/find-id` SHALL accept `?ids=<uuid>[,<uuid>...]` and return a JSON array with one resolution entry per ID. Entity resolution order: user → service → template → job → notification; first match wins. Non-UUID tokens return `{"type": "not a uuid"}` inline. Unknown UUIDs return `{"type": "no result found"}`. Absent or empty `ids` parameter returns `400`.

#### Scenario: User UUID resolved
- **WHEN** `GET /support/find-id?ids=<user-uuid>` is called
- **THEN** the response contains `[{"type": "user", "id": "<uuid>", "user_name": "..."}]`

#### Scenario: Service UUID resolved
- **WHEN** `GET /support/find-id?ids=<service-uuid>` is called
- **THEN** the response contains `[{"type": "service", "id": "<uuid>", "service_name": "..."}]`

#### Scenario: Notification UUID resolved with null api_key_id
- **WHEN** `GET /support/find-id?ids=<notification-uuid>` is called and the notification has no API key
- **THEN** the response contains an entry with `"type": "notification"` and `"api_key_id": null`

#### Scenario: Non-UUID token returned inline
- **WHEN** `GET /support/find-id?ids=not-a-uuid` is called
- **THEN** the response contains `[{"type": "not a uuid"}]` and HTTP status is 200

#### Scenario: Multiple IDs return one entry per ID in order
- **WHEN** `GET /support/find-id?ids=<uuid1>,<uuid2>` is called
- **THEN** the response array has exactly two entries in the same order as the input IDs

#### Scenario: Absent ids parameter returns 400
- **WHEN** `GET /support/find-id` is called with no `ids` query param
- **THEN** the response is `400 {"error": "no ids provided"}`
