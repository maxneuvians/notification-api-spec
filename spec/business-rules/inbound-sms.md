# Business Rules: Inbound SMS

## Overview

Inbound SMS allows recipients of notifications to reply to a service's dedicated phone number. When a reply arrives at the provider, it is stored as an `InboundSms` record linked to the service that owns the receiving number. Services may optionally configure a callback URL to receive these messages in near-real-time. The admin API exposes inbox-style views (most-recent per sender, full history), and the v2 public API exposes a cursor-paginated list for service consumers.

---

## Data Access Patterns

### `app/dao/inbound_sms_dao.py`

#### `resign_inbound_sms(resign, unsafe=False)`
- **Purpose**: Bulk re-sign (or dry-run verify) the `_content` column of every `InboundSms` row after a key rotation.
- **Query type**: Full table scan — `InboundSms.query.all()`, then conditional `db.session.bulk_save_objects()`.
- **Key filters/conditions**: None; processes all rows. Skips rows whose signature did not change. The `unsafe` flag allows ignoring `BadSignature` errors by falling back to `signer_inbound_sms.verify_unsafe()`.
- **Returns**: Nothing (side-effects only).
- **Notes**: Wrapped in `@transactional`. When `resign=False` the function runs in dry-run mode: it counts rows that need re-signing but resets signatures to their original values before the transaction commits.

#### `dao_create_inbound_sms(inbound_sms)`
- **Purpose**: Persist a new inbound SMS record.
- **Query type**: INSERT — `db.session.add()`.
- **Key filters/conditions**: None; caller is responsible for constructing a valid `InboundSms` object.
- **Returns**: Nothing (side-effects only).
- **Notes**: Wrapped in `@transactional`.

#### `dao_get_inbound_sms_for_service(service_id, user_number=None, *, limit_days=None, limit=None)`
- **Purpose**: Retrieve inbound SMS messages for a service, with optional sender-number and date-range filters.
- **Query type**: SELECT with optional WHERE clauses; ordered `created_at DESC`.
- **Key filters/conditions**:
  - Always filters `InboundSms.service_id == service_id`.
  - If `limit_days` is provided: `created_at >= midnight_n_days_ago(limit_days)`.
  - If `user_number` is provided: `user_number == user_number`.
  - If `limit` is provided: SQL `LIMIT`.
- **Returns**: List of `InboundSms` ORM objects.
- **Notes**: Used by the internal admin REST API for both the inbox search (POST with phone number) and the dashboard summary (limit=1 to find most-recent timestamp).

#### `dao_get_paginated_inbound_sms_for_service_for_public_api(service_id, older_than=None, page_size=None)`
- **Purpose**: Cursor-based pagination of inbound SMS for the v2 public API.
- **Query type**: SELECT with scalar subquery for cursor position; ordered `created_at DESC`; SQLAlchemy `.paginate()`.
- **Key filters/conditions**:
  - Always filters `InboundSms.service_id == service_id`.
  - If `older_than` (an `InboundSms.id`) is provided: a scalar subquery looks up `created_at` for that ID, then filters `created_at < <cursor_value>`.
  - `page_size` defaults to `app.config["PAGE_SIZE"]`.
- **Returns**: List of `InboundSms` ORM objects (the `.items` slice from the paginator).
- **Notes**: Cursor is keyed on `created_at` of the `older_than` record, not on a row offset, making it stable under concurrent inserts.

#### `dao_count_inbound_sms_for_service(service_id, limit_days)`
- **Purpose**: Count inbound SMS records within retention window — used by the dashboard summary endpoint.
- **Query type**: `COUNT(*)` with two WHERE predicates.
- **Key filters/conditions**: `service_id == service_id` AND `created_at >= midnight_n_days_ago(limit_days)`.
- **Returns**: Integer count.
- **Notes**: The summary endpoint always passes `limit_days=7` regardless of the service's configured data-retention period.

#### `_delete_inbound_sms(datetime_to_delete_from, query_filter)` *(internal)*
- **Purpose**: Batch-delete inbound SMS rows older than a given datetime matching an extra filter predicate.
- **Query type**: Repeated DELETE via subquery loop; `LIMIT 10000` per iteration.
- **Key filters/conditions**: `created_at < datetime_to_delete_from` combined with caller-supplied `query_filter`. Loop continues until zero rows are deleted.
- **Returns**: Total integer count of deleted rows.
- **Notes**: Batching in 10 000-row chunks avoids long-running transactions and excessive lock contention.

#### `delete_inbound_sms_older_than_retention()`
- **Purpose**: Scheduled retention sweep — delete inbound SMS older than each service's configured SMS retention period, defaulting to 7 days for services without a custom policy.
- **Query type**: SELECT of `ServiceDataRetention` (joined to `Service` → `InboundNumber`) then batched DELETEs via `_delete_inbound_sms`.
- **Key filters/conditions**:
  - First pass: services that have a `ServiceDataRetention` record for `SMS_TYPE` AND have an inbound number. Uses `days_of_retention` from the policy.
  - Second pass: all remaining services (not in the first-pass set), uses a hard-coded 7-day cutoff.
- **Returns**: Total integer count of deleted rows.
- **Notes**: Decorated with `@statsd(namespace="dao")` and `@transactional`. Only services that have an inbound number assigned are included in the flexible-retention pass.

#### `dao_get_inbound_sms_by_id(service_id, inbound_id)`
- **Purpose**: Fetch a single inbound SMS record by its own ID, scoped to a service.
- **Query type**: SELECT with two equality filters; `.one()` — raises if not found.
- **Key filters/conditions**: `id == inbound_id` AND `service_id == service_id`.
- **Returns**: A single `InboundSms` ORM object; raises `sqlalchemy.orm.exc.NoResultFound` if not found.
- **Notes**: Service-scoping prevents cross-service data leakage.

#### `dao_get_paginated_most_recent_inbound_sms_by_user_number_for_service(service_id, page, limit_days)`
- **Purpose**: Inbox view — return the single most-recent inbound SMS per unique sender (`user_number`) for a service, page-paginated.
- **Query type**: Self-join outer join on `inbound_sms` to eliminate all but the latest row per `(user_number, service_id)` pair; ordered `created_at DESC`; SQLAlchemy `.paginate()`.
- **Key filters/conditions**:
  - `InboundSms.service_id == service_id`.
  - `created_at >= midnight_n_days_ago(limit_days)` on the driving table.
  - The anti-join predicate `t2.id IS NULL` ensures only the most-recent row per sender passes.
- **Returns**: SQLAlchemy `Pagination` object; callers access `.items` and `.has_next`.
- **Notes**: Equivalent SQL is documented inline in the source. The `page` parameter is 1-based; `per_page` comes from `app.config["PAGE_SIZE"]`.

---

### `app/dao/inbound_numbers_dao.py`

#### `dao_get_inbound_numbers()`
- **Purpose**: List all inbound numbers registered in the system, with their associated service eagerly loaded.
- **Query type**: SELECT with `joinedload(InboundNumber.service)`; ordered `updated_at ASC`.
- **Key filters/conditions**: None — returns all rows.
- **Returns**: List of `InboundNumber` ORM objects.

#### `dao_get_available_inbound_numbers()`
- **Purpose**: List numbers that can be assigned to a new service.
- **Query type**: SELECT with two WHERE predicates.
- **Key filters/conditions**: `active == True` AND `service_id IS NULL`.
- **Returns**: List of `InboundNumber` ORM objects.

#### `dao_get_inbound_number_for_service(service_id)`
- **Purpose**: Retrieve the inbound number currently assigned to a service.
- **Query type**: SELECT; `.first()` — returns `None` if not found.
- **Key filters/conditions**: `service_id == service_id`.
- **Returns**: Single `InboundNumber` ORM object or `None`.
- **Notes**: A service may have at most one inbound number; the `.first()` call encodes this one-to-one cardinality assumption.

#### `dao_get_inbound_number(inbound_number_id)`
- **Purpose**: Fetch a single inbound number by its primary key.
- **Query type**: SELECT; `.first()` — returns `None` if not found.
- **Key filters/conditions**: `id == inbound_number_id`.
- **Returns**: Single `InboundNumber` ORM object or `None`.

#### `dao_set_inbound_number_to_service(service_id, inbound_number)`
- **Purpose**: Assign an `InboundNumber` object to a service by setting its `service_id`.
- **Query type**: UPDATE via ORM `db.session.add()`.
- **Key filters/conditions**: Caller passes the `InboundNumber` object; the function mutates and persists it.
- **Returns**: Nothing.
- **Notes**: Wrapped in `@transactional`. Does not check whether the number is currently unassigned or active — caller must validate preconditions.

#### `dao_set_inbound_number_active_flag(service_id, active)`
- **Purpose**: Toggle the `active` flag on the inbound number belonging to a service.
- **Query type**: SELECT then UPDATE via ORM.
- **Key filters/conditions**: `service_id == service_id` (`.first()`).
- **Returns**: Nothing.
- **Notes**: Wrapped in `@transactional`. Used by the `/service/<id>/off` REST endpoint to disable inbound SMS for a service without unassigning the number.

#### `dao_allocate_number_for_service(service_id, inbound_number_id)`
- **Purpose**: Atomically claim a specific available inbound number for a service.
- **Query type**: UPDATE with conditional filter (`id`, `active=True`, `service_id=None`); raises if no row was updated.
- **Key filters/conditions**: `id == inbound_number_id` AND `active == True` AND `service_id IS NULL`.
- **Returns**: The updated `InboundNumber` ORM object (re-fetched after update).
- **Notes**: Wrapped in `@transactional`. The single-statement conditional update (`filter_by(...).update(...)`) acts as an optimistic lock: if any condition fails (number inactive, already assigned, or wrong ID) exactly 0 rows are updated, triggering the `Exception`.

#### `dao_add_inbound_number(inbound_number)`
- **Purpose**: Register a new phone number in the inbound numbers pool.
- **Query type**: INSERT via `db.session.add()` + `db.session.commit()`.
- **Key filters/conditions**: None.
- **Returns**: Nothing.
- **Notes**: Hardcodes `provider="pinpoint"` and `active=True`. A new UUID is generated automatically. This function calls `db.session.commit()` directly rather than using `@transactional`, making it an eager commit.

---

## Domain Rules & Invariants

### Inbound Number Assignment
- Each service is limited to **one** inbound number (`dao_get_inbound_number_for_service` uses `.first()`).
- A number is "available" when `active=True` AND `service_id IS NULL`.
- Allocation is **atomic**: `dao_allocate_number_for_service` performs a single conditional UPDATE; if the number is already assigned or inactive, it raises `Exception("Inbound number: <id> is not available")`.
- All inbound numbers are registered under the `pinpoint` provider; the provider field is not configurable at registration time via the API.
- A number can be **deactivated** (`active=False`) for a service without being unassigned, via `dao_set_inbound_number_active_flag`.

### Inbound SMS Storage & Content Signing
- The `_content` column is **signed at rest** using `signer_inbound_sms`. The ORM property `content` transparently unsigns on read and re-signs on write.
- After a signing-key rotation, `resign_inbound_sms(resign=True)` must be run to re-sign all rows. `resign=False` is a dry-run that logs the count of rows needing re-signing without persisting changes.
- If a row's signature cannot be verified with the current key, `resign_inbound_sms` raises `BadSignature` unless `unsafe=True`, in which case it falls back to `signer_inbound_sms.verify_unsafe()`.

### Data Retention
- The default retention period is **7 days** for services that have not configured a custom `ServiceDataRetention` record for `SMS_TYPE`.
- Services with a custom `ServiceDataRetention` for SMS use `days_of_retention` from that record; only services that also have an inbound number assigned are subject to the custom policy in the retention sweep.
- The **dashboard summary** endpoint (`/summary`) always uses a hard-coded 7-day window for the count, regardless of the service's retention setting.
- Deletion is performed in **batches of 10 000** rows per iteration to limit transaction size and lock contention.

### Pagination Models
Two distinct pagination strategies are in use:

| Context | Strategy | Key parameter |
|---|---|---|
| v2 public API (`GET /v2/received-text-messages`) | Cursor-based (keyed on `created_at` of the `older_than` record ID) | `older_than` (UUID) |
| Admin inbox most-recent view (`GET /most-recent`) | Offset/page-based | `page` (integer, 1-based) |

- The cursor strategy is stable under concurrent inserts; the page strategy is not.
- Both strategies use `app.config["PAGE_SIZE"]` as the default page size; the v2 API can also be influenced by `app.config["API_PAGE_SIZE"]`.

### Inbox View (Most-Recent per Sender)
- The `GET /most-recent` endpoint deduplicates by `user_number` using a **self-join anti-pattern**: an outer join of `inbound_sms` on itself identifies and drops all rows that are not the latest for a given (`user_number`, `service_id`) pair.
- Only messages within the data-retention window (`limit_days`) are considered.

### User Number Normalisation
- When the admin API receives a `phone_number` filter (POST to the inbound-sms endpoint), it attempts to normalise the number to E.164 international format via `try_validate_and_format_phone_number(..., international=True)`.
- Normalisation is best-effort: if the input is alphanumeric (a short code or sender ID), the function may not transform it.

---

## Error Conditions

| Location | Condition | Raised |
|---|---|---|
| `dao_allocate_number_for_service` | The target number is not active, is already assigned, or does not exist | `Exception("Inbound number: <id> is not available")` |
| `resign_inbound_sms` | A row's `_content` cannot be verified with the current key and `unsafe=False` | `itsdangerous.BadSignature` (re-raised after logging) |
| `dao_get_inbound_sms_by_id` | No row matches `(inbound_id, service_id)` | `sqlalchemy.orm.exc.NoResultFound` (raised by `.one()`) |

---

## Query Inventory (for sqlc)

| Query name | Type | Tables | Description |
|---|---|---|---|
| `GetInboundSmsForService` | SELECT | `inbound_sms` | All messages for a service, optionally filtered by sender number and/or date window; ordered by `created_at DESC` |
| `GetPaginatedInboundSmsForPublicApi` | SELECT | `inbound_sms` | Cursor-paginated list for v2 API; cursor resolved via scalar subquery on `id` |
| `CountInboundSmsForService` | SELECT (aggregate) | `inbound_sms` | COUNT within a date window for a service |
| `GetInboundSmsById` | SELECT | `inbound_sms` | Single row by `(id, service_id)` |
| `GetMostRecentInboundSmsByUserNumber` | SELECT | `inbound_sms` (self-join) | Latest message per unique sender for a service, inside the retention window; page-paginated |
| `DeleteInboundSmsBatch` | DELETE | `inbound_sms` | Batch delete (up to 10 000 rows) older than a datetime matching an extra filter |
| `GetFlexibleRetentionPoliciesWithInboundNumber` | SELECT | `service_data_retention`, `services`, `inbound_numbers` | Fetch SMS retention policies for services that have an inbound number, for the retention sweep |
| `CreateInboundSms` | INSERT | `inbound_sms` | Persist a newly received inbound SMS record |
| `ResignInboundSmsAll` | SELECT + UPDATE | `inbound_sms` | Full-table read for key-rotation re-signing |
| `GetAllInboundNumbers` | SELECT | `inbound_numbers`, `services` | All numbers with their assigned service (eager join) |
| `GetAvailableInboundNumbers` | SELECT | `inbound_numbers` | Active, unassigned numbers |
| `GetInboundNumberForService` | SELECT | `inbound_numbers` | The number currently assigned to a service |
| `GetInboundNumberById` | SELECT | `inbound_numbers` | Single number record by primary key |
| `AllocateInboundNumberForService` | UPDATE | `inbound_numbers` | Conditional atomic UPDATE to claim an available number (active=True, service_id=NULL) |
| `SetInboundNumberServiceId` | UPDATE | `inbound_numbers` | Assign a number to a service by setting `service_id` |
| `SetInboundNumberActiveFlag` | UPDATE | `inbound_numbers` | Toggle `active` flag on the number belonging to a service |
| `InsertInboundNumber` | INSERT | `inbound_numbers` | Add a new phone number to the pool (provider=pinpoint, active=true) |
