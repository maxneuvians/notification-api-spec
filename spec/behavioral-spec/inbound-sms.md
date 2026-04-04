# Behavioral Spec: Inbound SMS

## Processed Files

- [x] `tests/app/inbound_sms/test_rest.py`
- [x] `tests/app/inbound_number/test_rest.py`
- [x] `tests/app/dao/test_inbound_sms_dao.py`
- [x] `tests/app/dao/test_inbound_numbers_dao.py`
- [x] `tests/app/v2/inbound_sms/test_get_inbound_sms.py`
- [x] `tests/app/v2/inbound_sms/test_inbound_sms_schemas.py`

---

## Endpoint Contracts

### POST /inbound-sms (admin: `inbound_sms.post_inbound_sms_for_service`)

Retrieves inbound SMS messages for a specific service. Uses POST to allow a filter body.

- **Happy path**
  - Body `{}`: returns all inbound SMS for the service within the retention window.
  - Each item in `data` array contains exactly: `id`, `created_at`, `service_id`, `notify_number`, `user_number`, `content`.
  - `content` defaults to `"Hello"` in test fixtures (shows real content is stored and returned).

- **Validation rules / filtering**
  - Optional body field `phone_number`: filters results to messages from that sender.
  - Phone number filter normalises to E.164 before comparison:
    - `"6502532222"` → matches stored `"+16502532222"`.
    - `"+16502532222"` → matches stored `"+16502532222"`.
    - `"+1 (650) 253-2223"` → normalised and matched against `"+16502532223"`.
  - Alphanumeric sender IDs (e.g., `"ALPHANUM3R1C"`) are passed through without normalisation and matched verbatim.
  - Without an explicit data-retention setting the window defaults to **7 days** (calculated using local timezone midnight; EST boundary = UTC 04:00).
  - If the service has an SMS data-retention record, that `days_of_retention` value overrides the 7-day default (tested with 5 days and 14 days).

- **Error cases** — none explicitly tested for this endpoint beyond the retention/filter logic above.

- **Auth requirements** — admin request (internal service); JWT/admin-auth enforced by the `admin_request` fixture.

---

### GET /inbound-sms/most-recent (admin: `inbound_sms.get_most_recent_inbound_sms_for_service`)

Returns deduplicated most-recent messages (one per sender number), page-based.

- **Happy path**
  - Default (page=1): returns up to **50** rows with `has_next: true` when total > 50.
  - Explicit `page=2`: returns **10** rows (remainder) with `has_next: false`.
  - Results are filtered to the retention window (same rules as above).
  - When a service has a data-retention > 7 days (e.g., 14 days), messages older than 7 days but within the custom window are still returned.

- **Data retention**
  - With 5-day retention and `freeze_time("2017-04-10 12:00")`, only messages from 2017-04-05 through 2017-04-10 are returned (6 days inclusive, using EST midnight boundary).
  - Retention beyond 7 days extends the window correspondingly.

- **Auth requirements** — admin.

---

### GET /inbound-sms/summary (admin: `inbound_sms.get_inbound_sms_summary_for_service`)

Returns aggregate stats for the service.

- **Happy path**
  - Response: `{ "count": <int>, "most_recent": "<ISO datetime>" }`.
  - `count` reflects only messages belonging to the requesting service.
  - `most_recent` is the `created_at` of the newest message for that service.

- **Edge cases**
  - When no messages exist: `{ "count": 0, "most_recent": null }`.

- **Auth requirements** — admin.

---

### GET /inbound-sms/`<inbound_sms_id>` (admin: `inbound_sms.get_inbound_by_id`)

Fetches a single inbound SMS record by ID, scoped to a service.

- **Happy path**
  - Returns `user_number` (E.164 normalised) and `service_id` among other fields.

- **Error cases**
  - Non-UUID `inbound_sms_id` → **404**.
  - Non-UUID `service_id` → **404**.

- **Auth requirements** — admin.

---

### GET /inbound-number (admin: `inbound_number.get_inbound_numbers`)

Returns the full list of inbound numbers in the system.

- **Happy path** — Returns serialised list of all `InboundNumber` records.
- **Edge case** — Returns `{ "data": [] }` when no numbers have been provisioned.
- **Auth requirements** — admin.

---

### GET /inbound-number/`<service_id>` (admin: `inbound_number.get_inbound_number_for_service`)

Returns the inbound number assigned to a specific service.

- **Happy path** — `{ "data": <serialised InboundNumber> }` for a service that has an assigned number.
- **Edge case** — `{ "data": {} }` (empty dict, not `null`) when the service has no assigned number.
- **Auth requirements** — admin.

---

### POST /inbound-number/`<service_id>`/off (admin: `inbound_number.post_set_inbound_number_off`)

Deactivates the inbound number for a service.

- **Happy path** — Sets `active = false` on the `InboundNumber` row; responds **204 No Content**.
- **Auth requirements** — admin.

---

### GET /inbound-number/available (admin: `inbound_number.get_available_inbound_numbers`)

Lists numbers not yet assigned to any service.

- **Happy path** — Returns only numbers where `service_id IS NULL`.
- **Edge case** — `{ "data": [] }` when all numbers are assigned or none exist.
- **Auth requirements** — admin.

---

### GET /v2/received-text-messages (public v2: `v2_inbound_sms.get_inbound_sms`)

Public-facing endpoint for service owners to pull their received text messages.

- **Happy path**
  - `200 OK`, `Content-Type: application/json`.
  - Response shape: `{ "received_text_messages": [...], "links": { "current": "<url>", "next": "<url>" } }`.
  - Messages ordered **newest-first** (`created_at` descending).
  - Page size controlled by `API_PAGE_SIZE` config value.
  - `links.current` always points to the base URL with no cursor.
  - `links.next` points to `?older_than=<last_id_on_page>` for the next page.

- **Cursor-based pagination (`older_than` query param)**
  - `?older_than=<uuid>` returns messages with `created_at` older than the identified record, newest-first within that window.
  - Successive `older_than` calls walk backward through history.
  - When the cursor is at or past the oldest record the `received_text_messages` array is **empty** and `links.next` is **absent** from the response.

- **Validation rules**
  - Only the `older_than` query parameter is permitted.
  - Any other query parameter (e.g., `user_number`) → **400** with `ValidationError: Additional properties are not allowed`.

- **Error cases**
  - No messages → `200` with empty `received_text_messages` array.
  - `older_than` past last record → `200` with empty array and no `next` link.

- **Auth requirements** — Service-scoped API key (`Authorization` header created via `create_authorization_header(service_id=...)`).

---

## DAO Behavior Contracts

### `dao_get_inbound_sms_for_service(service_id, limit=None, limit_days=None)`

- **Expected behavior**
  - Returns all `InboundSms` rows for the given service.
  - Ordered by `created_at` **descending** (newest first).
  - `limit` caps the number of returned rows.
  - `limit_days` restricts to messages created within the last N calendar days; the day boundary uses local timezone midnight (tests assume EST → UTC 04:00 cutoff).

- **Edge cases verified**
  - Returns empty list when no messages exist.
  - Strictly filters by `service_id` (messages from other services are excluded).
  - With `limit_days=7`, a message created at 03:59 UTC on day-7 boundary is excluded; 04:00 UTC on the same day is included.

---

### `dao_count_inbound_sms_for_service(service_id, limit_days)`

- **Expected behavior** — Returns the integer count of messages within the retention window for the given service.
- **Edge cases verified**
  - Counts are per-service only.
  - Uses the same EST midnight boundary as `dao_get_inbound_sms_for_service`.

---

### `delete_inbound_sms_older_than_retention()`

- **Expected behavior**
  - Iterates all services; for each service removes `InboundSms` rows older than that service's SMS retention period.
  - Returns total count of deleted rows.

- **Retention lookup rules**
  - If a service has an SMS `ServiceDataRetention` record → use its `days_of_retention`.
  - If only email retention is configured → ignored; SMS default applies.
  - Default (no SMS retention record): **7 days**.

- **Precision**
  - Day boundary is EST midnight (UTC 04:00); messages at exactly the cutoff boundary are kept; messages 1 second before are deleted.

- **Verified scenario** (freeze at 2017-06-08 12:00 UTC, three services: 3-day, 7-day default, 30-day):
  - 3-day service: 4 messages deleted (keeps only the 1 within 3 days).
  - 7-day default service: 2 messages deleted (keeps 3 within 7 days).
  - 30-day service: 1 message deleted (keeps 4 within 30 days).
  - Total deleted = 7.

---

### `dao_get_inbound_sms_by_id(service_id, id)`

- **Expected behavior** — Fetches a single `InboundSms` by composite key (service_id, id).
- **Edge cases verified** — Returns the exact same object as the one created.

---

### `dao_get_paginated_inbound_sms_for_service_for_public_api(service_id, older_than=None, page_size=<API_PAGE_SIZE>)`

- **Expected behavior**
  - Returns a list of `InboundSms` for the service, newest-first.
  - Strictly filters by `service_id`.
  - Returns `[]` when no messages exist.

- **Cursor (`older_than`) behavior**
  - `older_than=<id>` skips all rows at or newer than that ID.
  - Returns up to `page_size` records older than the cursor, newest-first among those.
  - When the cursor points to the oldest record → returns `[]`.

- **Edge cases verified**
  - Messages from other services are excluded even when `older_than` cursor spans them.

---

### `dao_get_paginated_most_recent_inbound_sms_by_user_number_for_service(service_id, limit_days, page)`

- **Expected behavior**
  - De-duplicates messages by `user_number`: returns **only the single most-recent message** per sender number.
  - Results are page-based (not cursor-based); page size driven by `PAGE_SIZE` config.
  - Returns a pagination object with `items`, `has_next`, `per_page`.
  - `limit_days` uses the same EST midnight boundary as other DAO functions.

- **Edge cases verified**
  - With 5 sender numbers and `PAGE_SIZE=2`, page 1 returns the 2 most-recently-active senders' latest messages; page 2 returns the next 2; `has_next` reflects remaining pages correctly.
  - The 7-day window excludes senders whose most-recent message is outside the window (boundary: 03:59 UTC excluded, 04:00 UTC included).

---

### `dao_get_inbound_numbers()` / `dao_get_available_inbound_numbers()`

- **`dao_get_inbound_numbers`** — Returns all `InboundNumber` rows regardless of assignment status.
- **`dao_get_available_inbound_numbers`** — Returns only rows where `service_id IS NULL`.

---

### `dao_set_inbound_number_to_service(service_id, inbound_number)`

- **Expected behavior** — Sets `service_id` on the given `InboundNumber` row.
- **Post-condition** — The number no longer appears in the available pool.
- **Error cases**
  - Assigning a second number to a service that already has one raises `IntegrityError` (unique constraint on `service_id`). Error message contains `"duplicate key value violates unique constraint"`.

---

### `dao_set_inbound_number_active_flag(service_id, active: bool)`

- **Expected behavior** — Sets `InboundNumber.active` for the number assigned to the given service. Works for both `True` and `False`.

---

### `dao_allocate_number_for_service(service_id, inbound_number_id)`

- **Expected behavior**
  - Assigns the specified `InboundNumber` to the service; returns the updated object.
  - `service.get_inbound_number()` reflects the new number immediately.
- **Error cases**
  - If the requested `inbound_number_id` is already assigned to another service → raises an exception with `"is not available"` in the message.

---

### `resign_inbound_sms(resign: bool, unsafe: bool = False)`

- **Expected behavior**
  - Iterates all `InboundSms` rows; re-signs the `_content` field using the current active signing key.
  - `resign=False` (preview): reads and verifies signatures but **does not write** — `_content` is unchanged.
  - `resign=True`: verifies with old keys then re-signs with the new key; `_content` changes but decrypted `content` is identical.
  - `resign=True, unsafe=True`: skips verification of old signatures and signs directly — allows recovery when old key is irretrievably lost.

- **Error cases**
  - `resign=True` with an incompatible key set (no overlap between old and current keys) → raises `itsdangerous.BadSignature`.

---

## Business Rules Verified

### Number Assignment Scenarios

- An inbound number starts as unassigned (`service_id IS NULL`).
- Calling `dao_set_inbound_number_to_service` or `dao_allocate_number_for_service` sets `service_id` on the number.
- Once assigned, the number is removed from the available pool (enforced at both DAO and DB constraint levels).
- A service may hold **at most one** inbound number (unique constraint on `service_id`); attempting a second assignment raises `IntegrityError`.
- A number can be deactivated (`active=false`) independently of re-assignment.

### SMS Content Storage and Retrieval

- Inbound SMS content is stored **encrypted** in the `_content` column via `signer_inbound_sms`; the ORM property `content` transparently decrypts.
- Content is scoped per-service: queries on service A never return messages belonging to service B.
- Sender number (`user_number`) is stored in normalised E.164 format. Non-E.164 alphanumeric senders are stored and returned verbatim.
- The admin POST filter normalises the supplied `phone_number` before comparison (strips country-code prefixes, spaces, parentheses, dashes).

### Pagination Behavior

| API surface | Style | Param | Default page size | Direction |
|---|---|---|---|---|
| Admin `GET /inbound-sms/most-recent` | Page-number | `page` | 50 | Newest-first, one row per sender |
| Admin `POST /inbound-sms` | None (full list) | — | No limit | Newest-first, all messages |
| Public `GET /v2/received-text-messages` | Cursor | `older_than` (UUID) | `API_PAGE_SIZE` config | Newest-first, all messages |
| DAO `dao_get_paginated_inbound_sms_for_service_for_public_api` | Cursor | `older_than` (UUID) | `API_PAGE_SIZE` config | Newest-first |
| DAO `dao_get_paginated_most_recent_inbound_sms_by_user_number_for_service` | Page-number | `page` | `PAGE_SIZE` config | Newest-first, deduplicated by sender |

- **Cursor pages**: a `links.next` URL is provided when more records exist; the key is absent from `links` when the current page is the last.
- **Cursor at end**: query returns an empty array.

### Data Retention Behavior

- Each service may have an SMS-specific `ServiceDataRetention` record specifying `days_of_retention`.
- Retention for other notification types (e.g., email) does not affect inbound SMS retention.
- Default when no SMS retention record exists: **7 days**.
- The cutoff time is localised to EST midnight (UTC 04:00); messages created before that moment on the boundary day are considered expired.
- `delete_inbound_sms_older_than_retention()` enforces retention across all services in a single pass and returns the total deleted count.
- The admin UI endpoints (`post_inbound_sms_for_service`, `get_most_recent_inbound_sms_for_service`) also honour retention when filtering results.

### Schema Validation Rules (from `test_inbound_sms_schemas.py`)

**Request schema (`get_inbound_sms_request`)**

- Valid inputs: `{}` (no params) or `{ "older_than": "<UUID>" }`.
- **Invalid**: any additional property (e.g., `user_number`) → `ValidationError`.

**Single-item response schema (`get_inbound_sms_single_response`)**

Required fields: `id` (UUID), `service_id` (UUID), `user_number`, `notify_number`, `content`, `created_at`.

- `created_at` must be ISO 8601 with **UTC `Z` suffix** (e.g., `"2017-11-02T15:07:57.197546Z"`). A timezone-naive datetime (no `Z`) fails validation.
- Missing `content` field → `ValidationError`.

**List response schema (`get_inbound_sms_response`)**

- Required top-level keys: `received_text_messages` (array), `links` (object with at least `current`).
- Each element of `received_text_messages` must satisfy the single-item schema.
- A list containing an invalid item (missing `content`, non-`Z` timestamp) → `ValidationError` on the whole list.
