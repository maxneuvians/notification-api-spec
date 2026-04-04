# Inbound SMS — Change Brief

## Overview

Inbound SMS allows service recipients to reply to a dedicated phone number. When a reply arrives via AWS SNS, the system validates the SNS signature, normalises the sender number, encrypts the message body (**C1 fix**: `content` column stores ciphertext via `pkg/crypto`), persists an `inbound_sms` row, and optionally enqueues a `send-inbound-sms` webhook task. Admins and services retrieve messages through inbox endpoints. A nightly retention sweep deletes records older than each service's configured window (default 7 days).

---

## Source Files

- `spec/behavioral-spec/inbound-sms.md`
- `spec/business-rules/inbound-sms.md`
- `openspec/changes/inbound-sms/proposal.md`

---

## Endpoints

### POST /inbound-sms (SNS ingestion, no auth)

- SNS-triggered; no application auth required.
- Validates SNS message signature before processing; rejects invalid signatures with 403.
- Parses sender number (`user_number`) and message body from SNS payload.
- Normalises `user_number` to E.164 international format; alphanumeric sender IDs are stored verbatim.
- Encrypts message body with `pkg/crypto.Encrypt` (C1 fix) before writing the `inbound_sms` row.
- If the service has a `service_inbound_api` record, enqueues `send-inbound-sms` task.
- If no `service_inbound_api` record, row is created and no task is enqueued.
- Returns 200 OK on success.

### GET /service/{service_id}/inbound-sms (admin)

- Auth: admin JWT.
- Optional `phone_number` query param: filters to messages from that sender.
  - Normalised to E.164 before comparison (strips spaces, dashes, parentheses, country-code prefix).
  - `"6502532222"` → matches `"+16502532222"`; `"+1 (650) 253-2222"` → normalised then matched.
  - Alphanumeric sender IDs (e.g., `"ALPHANUM3R1C"`) matched verbatim without normalisation.
- Results filtered to the service's retention window (default 7 days; overridden by SMS `ServiceDataRetention` record).
  - Day boundary: EST midnight (UTC 04:00). Message at 03:59 UTC on boundary day is excluded; at 04:00 UTC is included.
- Returns all matching rows ordered `created_at DESC`.
- Response: `{ "data": [{ "id", "created_at", "service_id", "notify_number", "user_number", "content" }] }`.

### GET /service/{service_id}/inbound-sms/most-recent (admin)

- Auth: admin JWT.
- Deduplicated: returns the single most-recent message per unique `user_number`.
- Page-based pagination via `page` query param (1-based); page size = `PAGE_SIZE` config (default 50).
- Response includes `items`, `has_next`, `per_page`.
- Retention window applies with same EST midnight boundary.

### GET /service/{service_id}/inbound-sms/summary (admin)

- Auth: admin JWT.
- Returns `{ "count": <int>, "most_recent": "<ISO datetime or null>" }`.
- `count` always computed over a **hard-coded 7-day** window, regardless of any custom retention setting.
- `most_recent` is `null` when no messages exist for the service.

### GET /service/{service_id}/inbound-sms/{inbound_sms_id} (admin)

- Auth: admin JWT.
- Returns single `inbound_sms` record scoped to the service.
- Non-UUID `inbound_sms_id` or `service_id` → 404.
- Record not found for that `(id, service_id)` pair → 404.
- Response fields: `id`, `service_id`, `user_number`, `notify_number`, `content`, `created_at`.

### GET /v2/received-text-messages (public v2, service API key)

- Auth: service-scoped API key (`Authorization` header).
- Response: `{ "received_text_messages": [...], "links": { "current": "<url>", "next": "<url>" } }`.
- Messages ordered newest-first; page size = `API_PAGE_SIZE` config.
- Cursor-based pagination via `older_than` query param (UUID of last seen message).
  - Returns messages with `created_at` strictly older than the cursor record, newest-first.
  - Cursor at or past the oldest record → empty `received_text_messages`, `links.next` absent.
- `older_than` is the **only** permitted query parameter; any other parameter → 400 (`ValidationError: Additional properties are not allowed`).
- Each item: `id` (UUID), `service_id` (UUID), `user_number`, `notify_number`, `content`, `created_at` (ISO 8601, **UTC `Z` suffix required**; timezone-naive timestamps fail schema validation).

---

## Worker Behaviors

### send-inbound-sms worker

- Reads the `inbound_sms` row and the service's `service_inbound_api` webhook URL.
- Applies SSRF guard to the callback URL (same guard as service callbacks); skips POST and logs error if blocked.
- POSTs a JSON payload to the URL with a signed bearer token.
- Retry policy: max 5 attempts, 300 s total timeout (same as service callbacks).

### delete-inbound-sms-older-than-retention (nightly beat)

- Runs nightly; deletes all `inbound_sms` rows older than each service's configured retention window.
- Two-pass approach:
  1. Services that have an SMS `ServiceDataRetention` record **and** an assigned inbound number → use `days_of_retention`.
  2. All remaining services → 7-day hard-coded cutoff.
- Only services with an assigned inbound number participate in the flexible-retention pass.
- Deletion batched in **10 000-row** chunks to limit lock contention and transaction size.
- Returns total count of deleted rows.
- Verified scenario (freeze at 2017-06-08 12:00 UTC, three services: 3-day, 7-day default, 30-day):
  - 3-day service: 4 deleted; 7-day default: 2 deleted; 30-day: 1 deleted; total = 7.

---

## DAO Functions

### `inbound_sms_dao`

| Function | Type | Transactional |
|---|---|---|
| `dao_create_inbound_sms(inbound_sms)` | INSERT | Yes |
| `dao_get_inbound_sms_for_service(service_id, user_number, limit_days, limit)` | SELECT `created_at DESC` | No |
| `dao_get_paginated_inbound_sms_for_service_for_public_api(service_id, older_than, page_size)` | SELECT cursor-paginated | No |
| `dao_count_inbound_sms_for_service(service_id, limit_days)` | SELECT COUNT | No |
| `dao_get_inbound_sms_by_id(service_id, inbound_id)` | SELECT `.one()` | No |
| `dao_get_paginated_most_recent_inbound_sms_by_user_number_for_service(service_id, page, limit_days)` | SELECT self-join, page-paginated | No |
| `delete_inbound_sms_older_than_retention()` | DELETE batched | Yes |
| `resign_inbound_sms(resign, unsafe)` | SELECT + bulk UPDATE | Yes |

**Key query notes:**

- All queries filter by `service_id` to prevent cross-service data leakage.
- EST midnight (UTC 04:00) boundary used for `created_at >= midnight_n_days_ago(limit_days)` in every date-windowed query.
- `dao_get_paginated_inbound_sms_for_service_for_public_api`: cursor keyed on `created_at` of the `older_than` UUID via scalar subquery — stable under concurrent inserts.
- `dao_get_paginated_most_recent_inbound_sms_by_user_number_for_service`: self-join outer-join anti-pattern — a second alias of `inbound_sms` joined on `(user_number, service_id)` where the newer row's `id IS NULL` ensures only the latest row per sender survives.
- `dao_count_inbound_sms_for_service`: dashboard summary endpoint always passes `limit_days=7` regardless of service's retention setting.
- `delete_inbound_sms_older_than_retention`: inner DELETE loop repeats until 0 rows deleted per iteration; each iteration limited to 10 000 rows.
- `resign_inbound_sms(resign=False)`: dry-run — counts rows needing re-signing but reverts `_content` to original value before transaction commits. `unsafe=True` falls back to `signer_inbound_sms.verify_unsafe()` when old signature cannot be verified.

### `inbound_numbers_dao`

| Function | Type | Notes |
|---|---|---|
| `dao_get_inbound_numbers()` | SELECT all, `joinedload(service)`, ordered `updated_at ASC` | |
| `dao_get_available_inbound_numbers()` | SELECT `active=True AND service_id IS NULL` | |
| `dao_get_inbound_number_for_service(service_id)` | SELECT `.first()` | Returns `None` if none assigned |
| `dao_get_inbound_number(inbound_number_id)` | SELECT `.first()` by PK | |
| `dao_set_inbound_number_to_service(service_id, inbound_number)` | UPDATE via ORM add | Transactional; caller validates preconditions |
| `dao_set_inbound_number_active_flag(service_id, active)` | SELECT then UPDATE | Transactional |
| `dao_allocate_number_for_service(service_id, inbound_number_id)` | Conditional UPDATE (`active=True AND service_id IS NULL AND id=…`) | Transactional; raises `Exception("Inbound number: <id> is not available")` on 0 rows updated |
| `dao_add_inbound_number(inbound_number)` | INSERT + eager `db.session.commit()` | Hardcodes `provider="pinpoint"`, `active=True` |

> **Note:** Inbound number CRUD is managed via the `service-management` change. These DAOs are listed here for completeness; new handlers for number assignment/deactivation are out of scope for `inbound-sms`.

---

## Business Rules

1. **C1 fix — explicit encrypt/decrypt**: `inbound_sms.content` (physical column) stores ciphertext from `pkg/crypto.Encrypt`. Service layer decrypts on every read with `pkg/crypto.Decrypt`. No transparent ORM property in Go — encrypt/decrypt is explicit at every call site.

2. **Inbound number assignment**: Each service holds at most one inbound number (unique constraint on `service_id`). Attempting a second assignment raises `IntegrityError` ("duplicate key value violates unique constraint"). `dao_allocate_number_for_service` uses an atomic conditional UPDATE; any failure condition raises `Exception("Inbound number: <id> is not available")`. All numbers registered under `pinpoint` provider.

3. **Data retention default**: 7 days. Overridden per service by a `ServiceDataRetention` record for `SMS_TYPE` (not email). Only services with an assigned inbound number are included in the custom-retention pass of the nightly sweep. Dashboard summary always uses 7-day hard-coded window.

4. **EST midnight boundary**: All `created_at >= midnight_n_days_ago(N)` conditions use EST midnight (UTC 04:00) as the day boundary. Messages at exactly the cutoff are **kept**; messages 1 second before are **deleted**.

5. **Sender normalisation**: `user_number` stored in E.164. Admin `phone_number` filter normalised via `try_validate_and_format_phone_number(..., international=True)` before comparison. Alphanumeric IDs not transformed.

6. **Two pagination strategies**:
   - Admin most-recent: page-based (`page` param, 1-based, `PAGE_SIZE` config, `has_next` boolean).
   - v2 public: cursor-based (`older_than` UUID, `API_PAGE_SIZE` config, `links.next` absent when exhausted). Cursor strategy is stable under concurrent inserts; page strategy is not.

7. **v2 schema validation**: `created_at` must have UTC `Z` suffix. `older_than` is the only allowed query param. Missing `content` field or non-`Z` timestamp on any item fails full-list validation.

8. **SSRF guard**: `send-inbound-sms` worker applies the same SSRF denylist check as service callbacks before POSTing to the webhook URL.

---

## Error Conditions

| Location | Condition | Outcome |
|---|---|---|
| `POST /inbound-sms` | Invalid or missing SNS signature | 403; no row created |
| `GET /service/{id}/inbound-sms/{sms_id}` | Non-UUID path param | 404 |
| `GET /service/{id}/inbound-sms/{sms_id}` | No `(id, service_id)` match | 404 |
| `GET /v2/received-text-messages` | Query param other than `older_than` | 400 (`ValidationError: Additional properties are not allowed`) |
| `dao_allocate_number_for_service` | Number inactive, already assigned, or not found | `Exception("Inbound number: <id> is not available")` |
| `dao_get_inbound_sms_by_id` | No matching row | `sqlalchemy.orm.exc.NoResultFound` (raised by `.one()`) |
| `resign_inbound_sms` | `BadSignature` and `unsafe=False` | `itsdangerous.BadSignature` re-raised |

---

## sqlc Query Inventory

| Query name | Type | Tables | Description |
|---|---|---|---|
| `CreateInboundSms` | INSERT | `inbound_sms` | Persist a new inbound SMS (content is pre-encrypted ciphertext) |
| `GetInboundSmsForService` | SELECT | `inbound_sms` | All messages for a service, optional sender/date/limit filters, `created_at DESC` |
| `GetPaginatedInboundSmsForPublicApi` | SELECT | `inbound_sms` | Cursor-paginated via `created_at` scalar subquery on `older_than` ID |
| `CountInboundSmsForService` | SELECT (agg) | `inbound_sms` | COUNT within EST midnight date window |
| `GetInboundSmsById` | SELECT | `inbound_sms` | Single row by `(id, service_id)` |
| `GetMostRecentInboundSmsByUserNumber` | SELECT | `inbound_sms` (self-join) | Latest row per sender, inside retention window, page-paginated |
| `DeleteInboundSmsBatch` | DELETE | `inbound_sms` | Batch delete ≤10 000 rows older than datetime + extra filter |
| `GetFlexibleRetentionPoliciesWithInboundNumber` | SELECT | `service_data_retention`, `services`, `inbound_numbers` | SMS retention policies for services with assigned inbound number |
| `ResignInboundSmsAll` | SELECT + UPDATE | `inbound_sms` | Full-table read for key-rotation re-signing |
| `GetAllInboundNumbers` | SELECT | `inbound_numbers`, `services` | All numbers with eagerly joined service |
| `GetAvailableInboundNumbers` | SELECT | `inbound_numbers` | Active, unassigned numbers |
| `GetInboundNumberForService` | SELECT | `inbound_numbers` | Number assigned to a service |
| `GetInboundNumberById` | SELECT | `inbound_numbers` | Single number by PK |
| `AllocateInboundNumberForService` | UPDATE | `inbound_numbers` | Atomic conditional UPDATE to claim a number |
| `SetInboundNumberServiceId` | UPDATE | `inbound_numbers` | Assign number to service |
| `SetInboundNumberActiveFlag` | UPDATE | `inbound_numbers` | Toggle `active` flag |
| `InsertInboundNumber` | INSERT | `inbound_numbers` | Register new number (provider=pinpoint, active=true) |
