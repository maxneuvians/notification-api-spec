# Business Rules: Notifications

## Overview

The **Notifications** domain is the core of the system. A notification is a single email, SMS, or letter sent (or to be sent) to one recipient on behalf of a service. The domain covers:

- **Creation**: generating notification records (one-off via REST, bulk via job, or internally via Celery tasks)
- **Delivery lifecycle**: status progression from `created` through terminal states
- **Rate enforcement**: per-minute API rate limits and daily / annual send quotas
- **Data retention and archival**: tiered retention policies per service / notification type
- **Administrative queries**: recipient search, performance stats, slow-delivery detection, bounce-rate reporting

Notifications are stored in the `notifications` table while they are live. Once they age past the service's retention period (default 7 days) they are upserted into `notification_history` and then deleted from `notifications`.

---

## Data Access Patterns

### `dao_get_last_template_usage(template_id, template_type, service_id)`
- **Purpose**: Return the most recently created notification that used a given template (used to show "last used" in the admin UI).
- **Query type**: SELECT
- **Key filters/conditions**:
  - `template_id = ?`
  - `key_type != 'test'` — test-key sends are excluded
  - `notification_type = template_type`
  - `service_id = ?`
- **Returns**: Single `Notification` or `None` (`.first()`)
- **Side effects**: None
- **Notes**: `ORDER BY created_at DESC` — a `MAX(created_at)` would be more efficient but was not changed because the caller uses the full object.

---

### `dao_create_notification(notification)`
- **Purpose**: Insert a new notification row with guaranteed `id` (UUIDv4) and default status.
- **Query type**: INSERT
- **Key filters/conditions**: N/A
- **Returns**: `None` (side-effectful)
- **Side effects**: Defaults `notification.id` to a new UUID if absent; defaults `notification.status` to `NOTIFICATION_CREATED` if absent; calls `db.session.add(notification)`.
- **Notes**: Wrapped in `@transactional`.

---

### `bulk_insert_notifications(notifications)`
- **Purpose**: Insert or update a list of `Notification` objects in one batch.
- **Query type**: INSERT (bulk)
- **Key filters/conditions**: N/A
- **Returns**: `None`
- **Side effects**: Assigns UUID and `NOTIFICATION_CREATED` status for each item missing them; uses `db.session.bulk_save_objects`.
- **Notes**: No individual-row error handling — a failed bulk write silently drops all rows in the batch (a Redis-backed queue fallback is mentioned as a TODO in the code).

---

### `update_notification_status_by_id(notification_id, status, sent_by=None, feedback_reason=None)`
- **Purpose**: Update a notification's delivery status by its primary key, used when a delivery receipt is received.
- **Query type**: SELECT FOR UPDATE → UPDATE
- **Key filters/conditions**:
  - `id = notification_id` with `WITH FOR UPDATE` row lock
  - current `status` must be one of: `created`, `sending`, `pending`, `sent`, `pending-virus-check`; otherwise the update is silently discarded as a duplicate callback
  - For international SMS: if `country_records_delivery(phone_prefix)` returns `False`, the update is skipped
- **Returns**: Updated `Notification` or `None`
- **Side effects**: Sets `sent_by` if not already populated; calls `_update_notification_status` which writes all delivery metadata columns.
- **Notes**: Guard against duplicate callbacks via `_duplicate_update_warning` (log only, no error raised).

---

### `update_notification_status_by_reference(reference, status)`
- **Purpose**: Update the status of a letter or email notification identified by its provider reference string.
- **Query type**: SELECT → UPDATE
- **Key filters/conditions**:
  - `reference = ?`
  - current `status` must be `sending` or `pending`
- **Returns**: Updated `Notification` or `None`
- **Side effects**: Calls `_update_notification_status`.
- **Notes**: Used for callbacks from letter/email providers where only the provider reference is known.

---

### `dao_update_notification(notification)`
- **Purpose**: Mark an already-loaded notification as updated (sets `updated_at = utcnow()`) then flushes to the DB.
- **Query type**: UPDATE
- **Returns**: `None`
- **Side effects**: Sets `notification.updated_at`.
- **Notes**: Used internally by `_update_notification_status`.

---

### `dao_update_notifications_by_reference(references, update_dict)`
- **Purpose**: Batch-update fields (via dict) on multiple notifications identified by their reference column. Falls back to `NotificationHistory` if the live table count is less than the number of references supplied.
- **Query type**: UPDATE (bulk)
- **Returns**: Tuple `(updated_count, updated_history_count)`
- **Side effects**: May write to `notification_history` as a secondary target.
- **Notes**: Used when a provider reports a batch result and the notification may already have been archived.

---

### `_update_notification_statuses(updates)` / `update_notification_statuses(notifications)`
- **Purpose**: Apply a prepared list of status + bounce-response mutations atomically.
- **Query type**: UPDATE (bulk)
- **Returns**: `None`
- **Side effects**: `_update_notification_statuses` calls `_decide_permanent_temporary_failure` for each entry before writing; then delegates to `update_notification_statuses` which calls `db.session.bulk_save_objects`.
- **Notes**: `@transactional` on `_update_notification_statuses`.

---

### `get_notification_for_job(service_id, job_id, notification_id)`
- **Purpose**: Fetch a single notification scoped to a specific job (used by the admin UI).
- **Query type**: SELECT ONE
- **Key filters/conditions**: `service_id`, `job_id`, `id` — will raise `NoResultFound` if not matched.
- **Returns**: `Notification`
- **Notes**: `.one()` — hard error on missing row.

---

### `get_notifications_for_job(service_id, job_id, filter_dict=None, page=1, page_size=None)`
- **Purpose**: Return paginated notifications for a job, in row-order.
- **Query type**: SELECT (paginated)
- **Key filters/conditions**: `service_id`, `job_id`; optional status / type filters via `_filter_query`
- **Returns**: Flask-SQLAlchemy `Pagination` object
- **Side effects**: None
- **Notes**: `ORDER BY job_row_number ASC`; `page_size` defaults to `PAGE_SIZE` config.

---

### `get_notification_count_for_job(service_id, job_id)`
- **Purpose**: Count rows for a job (used for progress indicators).
- **Query type**: COUNT
- **Returns**: `int`

---

### `get_notification_with_personalisation(service_id, notification_id, key_type)`
- **Purpose**: Load a notification and eagerly join its template (used when rendering or returning full notification data).
- **Query type**: SELECT ONE
- **Key filters/conditions**: `service_id`, `id`, optional `key_type`
- **Returns**: `Notification` with `template` loaded, or `None` (`.one()` → warning log → `None` on match failure)
- **Notes**: Uses `joinedload("template")`.

---

### `get_notification_by_id(notification_id, service_id=None, _raise=False)`
- **Purpose**: Simple lookup by primary key, optionally scoped to a service.
- **Query type**: SELECT (on reader replica via `db.on_reader()`)
- **Returns**: `Notification` or `None`; raises if `_raise=True` and no row found.

---

### `get_notifications(filter_dict=None)`
- **Purpose**: Unscoped notification query with optional status/type filtering (used internally).
- **Query type**: SELECT
- **Returns**: SQLAlchemy query object (not yet executed)

---

### `get_notifications_for_service(service_id, ...)`
- **Purpose**: Primary listing query for a service's notification history; powers all admin/API list endpoints.
- **Query type**: SELECT (paginated)
- **Key filters/conditions**:
  - `service_id = ?` (always)
  - `created_at > now - limit_days` if `limit_days` set (respects retention period)
  - `created_at < older_than_notification.created_at` for cursor-based "older than" pagination
  - `job_id IS NULL` if `include_jobs=False` (excludes bulk-job notifications)
  - `created_by_id IS NULL` if `include_one_off=False` (excludes manually-sent one-offs)
  - `key_type = ?` if explicit; else `key_type != 'test'` unless `include_from_test_key=True`
  - `client_reference = ?` if provided
  - Optional `status` and `template_type` from `filter_dict` via `_filter_query`
- **Returns**: `Pagination` ordered by `created_at DESC`
- **Side effects**: None
- **Notes**:
  - `personalisation=True` adds `joinedload("template")`; otherwise `_personalisation` column is fully deferred to avoid deserialising large blobs
  - `format_for_csv=True` eagerly loads `template`, `job`, and `created_by` to avoid N+1 queries during CSV generation
  - `count_pages=False` skips the COUNT query for faster paginated fetches when the total isn't needed

---

### `delete_notifications_older_than_retention_by_type(notification_type, qry_limit=10000)`
- **Purpose**: Age-out live notification rows for a given type, respecting per-service retention policies.
- **Query type**: SELECT (retention config), INSERT-ON-CONFLICT (archive), DELETE (batched)
- **Key filters/conditions**: Reads `ServiceDataRetention` per type; applies per-service retention days; remaining services get the global 7-day default.
- **Returns**: Total rows deleted (`int`)
- **Side effects**:
  1. Calls `insert_update_notification_history` — upserts non-test notifications into `notification_history` before deletion
  2. Deletes in batches of `qry_limit` until no more qualify
  3. Test-key notifications are deleted directly (no history row)
- **Notes**: `qry_limit=10000` per query; loops until zero rows remain.

---

### `insert_update_notification_history(notification_type, date_to_delete_from, service_id)`
- **Purpose**: Copy notifications that are about to be purged into the long-term `notification_history` table.
- **Query type**: INSERT … FROM SELECT … ON CONFLICT DO UPDATE
- **Key filters/conditions**: `notification_type`, `service_id`, `created_at < date_to_delete_from`, `key_type != 'test'`
- **Returns**: `None`
- **Side effects**: On primary-key conflict updates `notification_status`, `reference`, `billable_units`, `updated_at`, `sent_at`, `sent_by` on the existing history row.

---

### `dao_delete_notifications_by_id(notification_id)`
- **Purpose**: Hard-delete a single notification by ID (used by tests and admin tools).
- **Query type**: DELETE
- **Notes**: `@transactional`; no history archival.

---

### `dao_timeout_notifications(timeout_period_in_seconds)`
- **Purpose**: Mark stale undelivered notifications as timed out; runs as a periodic Celery beat task.
- **Query type**: SELECT → UPDATE (two passes)
- **Key filters/conditions**:
  - `created_at < (utcnow - timeout_period)` AND
  - `notification_type != LETTER_TYPE` (letters are never timed out this way)
  - Pass 1: `status IN ('created')` → `technical-failure`
  - Pass 2: `status IN ('sending', 'pending')` → `temporary-failure`
- **Returns**: Tuple of `(technical_failure_notifications, temporary_failure_notifications)` lists
- **Notes**: See [Status Transition Rules](#status-transitions) for business meaning.

---

### `is_delivery_slow_for_provider(created_at, provider, threshold, delivery_time)`
- **Purpose**: Determine whether a provider is experiencing slow delivery (used for automatic provider failover logic).
- **Query type**: SELECT (aggregate)
- **Key filters/conditions**: `created_at >= ?`, `sent_at IS NOT NULL`, `status IN (delivered, pending, sending)`, `sent_by = provider`, `key_type != 'test'`
- **Returns**: `True` if `slow_count / total >= threshold`, `False` otherwise (also `False` if no qualifying notifications).
- **Notes**: A notification is "slow" if `updated_at - sent_at >= delivery_time` (when delivered) or `utcnow - sent_at >= delivery_time` (still in-flight).

---

### `dao_get_notifications_by_to_field(service_id, search_term, notification_type=None, statuses=None)`
- **Purpose**: Recipient-based notification search (admin "search by recipient" feature).
- **Query type**: SELECT
- **Key filters/conditions**:
  - `service_id = ?`
  - `normalised_to LIKE '%<normalised>%'`
  - `key_type != 'test'`
  - Optional `status IN (?)`, `notification_type = ?`
- **Returns**: All matching `Notification` rows ordered by `created_at DESC`
- **Side effects**: None
- **Notes**:
  - If `notification_type` is not supplied, `guess_notification_type` infers it: EMAIL if the term contains any ASCII letter or `@`, otherwise SMS.
  - SMS normalisation: validate + format phone number, strip `(`, `)`, ` `, `-`, strip leading `+` and `0`.
  - Email normalisation: validate + format; falls back to `lower()` on invalid addresses.
  - Special characters in the search term are escaped for the LIKE operator via `escape_special_characters`.
  - Raises `InvalidRequest(400)` if type resolves to LETTER.

---

### `dao_get_notification_by_reference(reference)`
- **Purpose**: Exact-match lookup on the `reference` column (read replica).
- **Query type**: SELECT ONE
- **Returns**: `Notification` (raises `NoResultFound` if absent)

---

### `dao_get_notification_history_by_reference(reference)`
- **Purpose**: Reference lookup that checks the live table first, then falls back to history (test/research-mode notifications are never in history).
- **Query type**: SELECT ONE × 2
- **Returns**: `Notification` or `NotificationHistory`

---

### `dao_get_notifications_by_references(references)`
- **Purpose**: Bulk reference lookup.
- **Query type**: SELECT (IN list)
- **Returns**: List of `Notification`
- **Notes**: Raises `NoResultFound` with the reference list if the result is empty.

---

### `dao_created_scheduled_notification(scheduled_notification)`
- **Purpose**: Persist a new `ScheduledNotification` row.
- **Query type**: INSERT
- **Notes**: Committed immediately (`db.session.commit()`).

---

### `dao_get_scheduled_notifications()`
- **Purpose**: Fetch all scheduled sends whose execution time has arrived.
- **Query type**: SELECT (JOIN `scheduled_notifications`)
- **Key filters/conditions**: `scheduled_for < utcnow()` AND `pending = True`
- **Returns**: List of `Notification`

---

### `set_scheduled_notification_to_processed(notification_id)`
- **Purpose**: Mark a processed scheduled notification so it is not re-processed.
- **Query type**: UPDATE `scheduled_notifications.pending = False`
- **Notes**: Committed immediately.

---

### `dao_get_total_notifications_sent_per_day_for_performance_platform(start_date, end_date)`
- **Purpose**: Performance Platform reporting — total notifications sent and how many arrived within 10 seconds.
- **Query type**: SELECT (aggregate)
- **Key filters/conditions**: `api_key_id IS NOT NULL`, `key_type != 'test'`, `notification_type != 'letter'`, `created_at` between `start_date` and `end_date`
- **Returns**: Named-tuple with `messages_total` and `messages_within_10_secs`

---

### `get_latest_sent_notification_for_job(job_id)`
- **Purpose**: Retrieve the notification most recently updated in a job (used to produce job completion timestamps).
- **Query type**: SELECT ORDER BY `updated_at DESC` LIMIT 1

---

### `dao_get_last_notification_added_for_job_id(job_id)`
- **Purpose**: Retrieve the last notification row added to a job (highest `job_row_number`).
- **Query type**: SELECT ORDER BY `job_row_number DESC` LIMIT 1

---

### `notifications_not_yet_sent(should_be_sending_after_seconds, notification_type)`
- **Purpose**: Find notifications that have been in `created` status longer than the expected queuing time (monitoring / alerting).
- **Query type**: SELECT
- **Key filters/conditions**: `created_at <= (utcnow - threshold)`, `notification_type = ?`, `status = 'created'`
- **Returns**: List of `Notification`

---

### `dao_old_letters_with_created_status()`
- **Purpose**: Identify letters that were never picked up for printing (past the 5:30 PM BST same-day cutoff of the previous day).
- **Query type**: SELECT
- **Key filters/conditions**: `notification_type = 'letter'`, `status = 'created'`, `updated_at < yesterday 17:30 BST → UTC`
- **Returns**: List of `Notification`, ordered by `updated_at`

---

### `dao_precompiled_letters_still_pending_virus_check()`
- **Purpose**: Find precompiled letters stuck in `pending-virus-check` for more than 90 minutes (monitoring).
- **Query type**: SELECT
- **Key filters/conditions**: `status = 'pending-virus-check'`, `created_at < (utcnow - 90 min)`
- **Returns**: List of `Notification`, ordered by `created_at`

---

### `send_method_stats_by_service(start_time, end_time)`
- **Purpose**: Produce per-service, per-type, per-send-method delivery counts (used by reporting).
- **Query type**: SELECT (aggregate, JOIN `services`, `organisations`)
- **Key filters/conditions**: `status IN ('delivered', 'sent')`, `key_type != 'test'`, `created_at` between `start_time` and `end_time`
- **Returns**: List of named rows: `(service_id, service_name, organisation_name, notification_type, send_method, total_notifications)`
- **Notes**: `send_method` is `'api'` when `api_key_id IS NOT NULL`, else `'admin'`.

---

### `overall_bounce_rate_for_day(min_emails_sent=1000, default_time=utcnow())`
- **Purpose**: Cross-service hard bounce rate in the last 24 hours (used for platform-wide bounce alerts).
- **Query type**: SELECT (aggregate subquery)
- **Key filters/conditions**: `created_at BETWEEN (default_time - 24h) AND default_time`; `HAVING COUNT(*) >= min_emails_sent`
- **Returns**: List of `(service_id, total_emails, hard_bounces, bounce_rate%)` for services that clear the minimum threshold.

---

### `service_bounce_rate_for_day(service_id, min_emails_sent=1000, default_time=utcnow())`
- **Purpose**: Same as above but scoped to one service.
- **Query type**: SELECT (aggregate subquery)
- **Returns**: Single row `(total_emails, hard_bounces, bounce_rate%)` or `None` if below threshold.

---

### `total_notifications_grouped_by_hour(service_id, default_time=utcnow(), interval=24)`
- **Purpose**: Time-series email volume for a service over `interval` hours (used for bounce dashboard charting).
- **Query type**: SELECT (aggregate)
- **Key filters/conditions**: `notification_type = 'email'`, `service_id = ?`, `created_at` within interval
- **Returns**: List of `(hour, total_notifications)`

---

### `total_hard_bounces_grouped_by_hour(service_id, default_time=utcnow(), interval=24)`
- **Purpose**: Time-series hard-bounce count per hour for a service.
- **Key filters/conditions**: Same as above plus `feedback_type = 'hard-bounce'` (the `NOTIFICATION_HARD_BOUNCE` constant)
- **Returns**: List of `(hour, total_notifications)`

---

### `resign_notifications(chunk_size, resign, unsafe=False)` / `_resign_notifications_chunk(...)`
- **Purpose**: Maintenance operation — re-encrypt the `_personalisation` column for all notification rows with a (potentially rotated) signing key.
- **Query type**: SELECT (chunked by `slice`) → conditional UPDATE (bulk)
- **Returns**: Count of notifications resigned or needing re-signing.
- **Notes**: `resign=False` is a dry-run (counts but does not write). `unsafe=True` forces re-sign even for rows with bad signatures.

---

## Domain Rules & Invariants

### Status Transitions

Valid notification statuses and their meaning:

| Status | Meaning |
|---|---|
| `created` | Record saved; not yet dispatched to the provider queue |
| `sending` | Handed off to the provider |
| `sent` | Sent; no delivery receipt expected (some international SMS) |
| `pending` | Provider acknowledged; awaiting delivery receipt |
| `pending-virus-check` | Precompiled letter uploaded; awaiting virus scan |
| `delivered` | Provider confirmed delivery |
| `temporary-failure` | Delivery failed — may be retried |
| `permanent-failure` | Delivery permanently failed (hard bounce, invalid number) |
| `technical-failure` | Internal failure (never reached provider) |

**Allowed inbound transitions** (enforced by guards in the DAO):

| Incoming event | Guard (required current status) | Resulting status |
|---|---|---|
| Delivery receipt (by ID) | `created`, `sending`, `pending`, `sent`, `pending-virus-check` | Any valid terminal/in-progress status |
| Delivery receipt (by reference) | `sending`, `pending` | Any valid status |
| Timeout — no provider dispatch | `created` | `technical-failure` |
| Timeout — no delivery receipt | `sending`, `pending` | `temporary-failure` |

**Special rule — Firetext `permanent-failure` correction:**
If `current_status == 'pending'` and the incoming `status == 'permanent-failure'`, the status is silently downgraded to `temporary-failure` (function `_decide_permanent_temporary_failure`). Firetext sends `pending` before the terminal status; a `permanent-failure` arriving this way indicates a transient failure, not a definitive one.

**International SMS — delivery receipt skip:**
If the notification is marked `international=True` and the destination country's `dlr` attribute in `INTERNATIONAL_BILLING_RATES` is not `"yes"`, any status update via `update_notification_status_by_id` is silently ignored.

---

### Rate Limiting (Per-Minute API Throughput)

- Controlled by `check_service_over_api_rate_limit_and_update_rate` / `check_rate_limiting`.
- Requires **both** `API_RATE_LIMIT_ENABLED` and `REDIS_ENABLED` to be in effect.
- Uses a Redis sorted-set keyed `rate_limit:{service_id}:{key_type}`.
- Window: **60 seconds**; limit: `service.rate_limit` requests.
- Raises: `RateLimitError(rate_limit, interval=60, key_type)` → HTTP 429.

---

### Daily Send Limits

Enforced at request time before the notification is persisted.

#### Email daily limit
- Counter: Redis key `email_daily_count:{service_id}` (2-hour TTL); seeded lazily from `fetch_todays_total_email_count` if cache miss.
- Limit field: `service.message_limit`
- Threshold: `(emails_sent_today + requested) > service.message_limit`
- Trial service raises: `TrialServiceTooManyEmailRequestsError`
- Live service raises: `LiveServiceTooManyEmailRequestsError`

#### SMS daily limit (standard mode — `FF_USE_BILLABLE_UNITS = False`)
- Counter: Redis key `sms_daily_count:{service_id}` (2-hour TTL); seeded lazily from `fetch_todays_total_sms_count`.
- Limit field: `service.sms_daily_limit`
- Threshold: `(sms_sent_today + requested) > service.sms_daily_limit`
- Trial raises: `TrialServiceTooManySMSRequestsError`
- Live raises: `LiveServiceTooManySMSRequestsError`

#### SMS daily limit (billable-unit mode — `FF_USE_BILLABLE_UNITS = True`)
- Counter: Redis key `billable_units_sms_daily_count:{service_id}` (2-hour TTL); seeded from `fetch_todays_total_sms_billable_units`.
- `requested_sms` is the **fragment count** for the notification (not always 1).
- Otherwise identical to standard mode.

#### Warning notifications
Both limits send warning emails through the service's own users when:
- **≥ 80% used** (`NEAR_DAILY_LIMIT_PERCENTAGE = 0.80`): uses Redis key `near_*_daily_limit:{service_id}` with expiry = seconds until midnight.
- **≥ 100% used**: uses Redis key `over_*_daily_limit:{service_id}` with the same expiry.
- Redis keys prevent duplicate emails within the same calendar day.

Counters are incremented **at request time** (not at delivery) via `increment_sms_daily_count_send_warnings_if_needed` / `increment_email_daily_count_send_warnings_if_needed`.

---

### Annual Send Limits

Enforced at request time, after the daily-limit check.

#### Data source
Annual totals are stored in Redis in a hash keyed `annual_limit_notifications_v2:{service_id}` with fields:
- `TOTAL_EMAIL_FISCAL_YEAR_TO_YESTERDAY`
- `TOTAL_SMS_FISCAL_YEAR_TO_YESTERDAY`
- `TOTAL_SMS_BILLABLE_UNITS_FISCAL_YEAR_TO_YESTERDAY` (only populated when `FF_USE_BILLABLE_UNITS` is enabled)

The hash is seeded on first access each day (`was_seeded_today` check). Seeding queries `fact_notification_status` via:
- `fetch_notification_status_totals_for_service_by_fiscal_year` for notification counts
- `fetch_billable_units_totals_for_service_by_fiscal_year` for billable units

Today's counts are obtained from the daily Redis counters (not from the DB) and added to the fiscal-year-to-yesterday total at check time.

**Zero-count guard**: if all seeded values are zero, `set_seeded_at()` is called manually to prevent an infinite re-seeding loop on every API call.

#### Email annual limit
- Total used: `emails_sent_today (Redis) + TOTAL_EMAIL_FISCAL_YEAR_TO_YESTERDAY (Redis hash)`
- Limit field: `service.email_annual_limit`
- Threshold: `total_used + requested > service.email_annual_limit`
- Trial raises: `TrialServiceRequestExceedsEmailAnnualLimitError`
- Live raises: `LiveServiceRequestExceedsEmailAnnualLimitError`

#### SMS annual limit (standard / billable-unit modes)
- Standard: uses `TOTAL_SMS_FISCAL_YEAR_TO_YESTERDAY`
- Billable-unit: uses `TOTAL_SMS_BILLABLE_UNITS_FISCAL_YEAR_TO_YESTERDAY`
- Limit field: `service.sms_annual_limit`
- Trial raises: `TrialServiceRequestExceedsSMSAnnualLimitError`
- Live raises: `LiveServiceRequestExceedsSMSAnnualLimitError`

#### Warning notifications (annual)
- **≥ 80% used** (`NEAR_ANNUAL_LIMIT_PERCENTAGE = 0.80`): a "nearing annual limit" warning email is sent **once** per fiscal year (deduplication via `annual_limit_client.check_has_warning_been_sent` / `set_nearing_*_limit`).
- **Exactly at limit (==)**: a "reached annual limit" email is sent **once** (deduplication via `check_has_over_limit_been_sent` / `set_over_*_limit`).

---

### SMS Fragment Counting

- Function: `number_of_sms_fragments(template, personalisation)` in `process_notifications.py`
- Delegates to `create_content_for_notification(template, personalisation).fragment_count` which is computed by the `notifications_utils` library based on the rendered content length and encoding (GSM-7 vs Unicode).
- Returns `0` for non-SMS templates.
- **SMS character-count limit** (`SMS_CHAR_COUNT_LIMIT`): validated against the rendered content length (after personalisation substitution) before persisting. Rest v1 checks `content_count > SMS_CHAR_COUNT_LIMIT`; v2 validators check `content_count + prefix_length > SMS_CHAR_COUNT_LIMIT` (adds `": "` + service name when `prefix_sms=True`).
- `billable_units` on the `Notification` row is set to the fragment count only when `FF_USE_BILLABLE_UNITS` is enabled; otherwise it may remain `None` or be populated by the Celery deliver task.

---

### Personalisation Security

- The `_personalisation` column is stored **signed** using `signer_personalisation` (itsdangerous HMAC).
- The property accessor `notification.personalisation` transparently unsigns on read.
- Setting `notification.personalisation = value` transparently re-signs on write.
- Key rotation is handled by `resign_notifications` / `_resign_notifications_chunk`, which iterate all `Notification` rows in configurable chunk sizes, unsign with the old key, and re-sign with the new key.
- `unsafe=True` instructs the resigner to use `verify_unsafe` (accepts any valid-format signature) to recover rows signed with an unknown-but-still-readable key.

---

### Personalisation Validation

Applied in `validate_personalisation_and_decode_files` (validators.py):

1. **Size limit** (`PERSONALISATION_SIZE_LIMIT` config): total length of all non-file personalisation values concatenated; raises `ValidationError` with bytes count.
2. **File attachment count** (`ATTACHMENT_NUM_LIMIT`): number of dict values with a `"file"` key; raises `ValidationError`.
3. **Per-file decode and size**: each file value is base64-decoded; decoded size checked against `ATTACHMENT_SIZE_LIMIT`; base64 decode errors returned as `ValidationError` per key.

---

### Data Retention and Archival

- **Per-service retention** is configured in the `service_data_retention` table: `(service_id, notification_type, days_of_retention)`.
- **Default retention**: 7 days (applied to services without a `ServiceDataRetention` row for the relevant type).
- The `delete_notifications_older_than_retention_by_type` function runs per notification type and:
  1. Reads all `ServiceDataRetention` rows for the type.
  2. For each: calls `insert_update_notification_history` then `_delete_notifications` (batched).
  3. For services not in `ServiceDataRetention`: applies the 7-day default.
- **Test-key notifications are never archived** — only deleted.
- **Non-test notifications** are upserted into `notification_history` before deletion (conflict key = PK; on conflict update: status, reference, billable_units, updated_at, sent_at, sent_by).

---

### Bounce Rate Rules

- `feedback_type = 'hard-bounce'` is the database value that identifies a hard bounce (stored on `Notification.feedback_type`, a separate column from `status`).
- Bounce-rate window: **last 24 hours** from `default_time`.
- Minimum volume before reporting: `min_emails_sent` (default **1 000**) — services below this threshold are excluded.
- Rate formula: `(hard_bounces / total_emails) × 100` reported as integer percentage.

---

### Template Validation

Enforced before notification is persisted:

| Rule | Error |
|---|---|
| Template type ≠ notification type | `BadRequestError` — "X template is not suitable for Y notification" |
| Template is archived | `BadRequestError` — "Template {id} has been deleted" |
| Template has missing personalisation placeholders | `BadRequestError` (v1) / `BadRequestError` (v2) — "Missing personalisation: …" |
| SMS content (with prefix) exceeds `SMS_CHAR_COUNT_LIMIT` | `BadRequestError` — "Content … greater than the limit of {N}" |
| Rendered message body is blank/whitespace | `BadRequestError` — "Message is empty or just whitespace" |

---

### Recipient Validation

| Rule | Error |
|---|---|
| Recipient is `None` or empty | `BadRequestError` — "Recipient can't be empty" |
| Team-only API key, recipient not in service safelist | `BadRequestError` — "Can't send … using a team-only API key" |
| Trial-mode service, recipient not in safelist | `BadRequestError` — "Can't send … when service is in trial mode" |
| International SMS, service lacks `INTERNATIONAL_SMS_TYPE` permission | `BadRequestError` — "Cannot send to international mobile numbers" |
| Service lacks permission for notification type | `BadRequestError` — "Service is not allowed to send …" |

---

### Scheduled Notifications

- Stored in the `scheduled_notifications` join table: `(notification_id, scheduled_for, pending)`.
- A service must have the `SCHEDULE_NOTIFICATIONS` permission; attempting to schedule without it raises `BadRequestError` — "Cannot schedule notifications (this feature is invite-only)".
- `dao_get_scheduled_notifications` returns all rows where `scheduled_for < utcnow()` AND `pending = True`.
- After a scheduled notification is dispatched, `set_scheduled_notification_to_processed` marks `pending = False`.

---

## Error Conditions

| Exception | Condition | HTTP |
|---|---|---|
| `RateLimitError` | API-rate-limit exceeded (>N requests / 60s for the API key type) | 429 |
| `TrialServiceTooManySMSRequestsError` | Trial service: SMS daily count + requested > `sms_daily_limit` | 429 |
| `LiveServiceTooManySMSRequestsError` | Live service: SMS daily count + requested > `sms_daily_limit` | 429 |
| `TrialServiceTooManyEmailRequestsError` | Trial service: email daily count + requested > `message_limit` | 429 |
| `LiveServiceTooManyEmailRequestsError` | Live service: email daily count + requested > `message_limit` | 429 |
| `TrialServiceRequestExceedsSMSAnnualLimitError` | Trial service: SMS fiscal-year total + today + requested > `sms_annual_limit` | 429 |
| `LiveServiceRequestExceedsSMSAnnualLimitError` | Live service: same condition | 429 |
| `TrialServiceRequestExceedsEmailAnnualLimitError` | Trial service: email fiscal-year total + today + requested > `email_annual_limit` | 429 |
| `LiveServiceRequestExceedsEmailAnnualLimitError` | Live service: same condition | 429 |
| `BadRequestError` — template type mismatch | `notification_type != template.template_type` | 400 |
| `BadRequestError` — template archived | `template.archived == True` | 400 |
| `BadRequestError` — missing personalisation | Template has unfilled placeholders at send time | 400 |
| `BadRequestError` — SMS content too long (v2) | `content_count + prefix_length > SMS_CHAR_COUNT_LIMIT` | 400 |
| `InvalidRequest` — SMS content too long (v1) | `content_count > SMS_CHAR_COUNT_LIMIT` | 400 |
| `BadRequestError` — blank message | Rendered body is empty or whitespace-only | 400 |
| `BadRequestError` — empty recipient | `recipient` is `None` | 400 |
| `BadRequestError` — team-only key, recipient not safelisted | Recipient not in service safelist when using team API key | 400 |
| `BadRequestError` — trial mode, recipient not safelisted | Recipient not in safelist when service is restricted | 400 |
| `BadRequestError` — international SMS not permitted | `international=True` but service lacks `INTERNATIONAL_SMS_TYPE` permission | 400 |
| `BadRequestError` — no permission for type | Service permission list doesn't include notification type | 400 |
| `BadRequestError` — scheduling not permitted | `scheduled_for` set but service lacks `SCHEDULE_NOTIFICATIONS` permission | 400 |
| `BadRequestError` — unknown reply-to / SMS sender / letter contact | Supplied ID not found in DB for the service | 400 |
| `BadRequestError` — template not found | Template ID not found for service | 400 |
| `ValidationError` — personalisation size | Concat of all personalisation values exceeds `PERSONALISATION_SIZE_LIMIT` | 400 |
| `ValidationError` — too many file attachments | File count > `ATTACHMENT_NUM_LIMIT` | 400 |
| `ValidationError` — file too large | Decoded file size > `ATTACHMENT_SIZE_LIMIT` | 400 |
| `ValidationError` — bad base64 | File personalisation value is not valid base64 | 400 |
| `InvalidRequest` — search by recipient letter type | `dao_get_notifications_by_to_field` called with letter search term | 400 |
| `NoResultFound` | `dao_get_notifications_by_references` called with references matching no rows | 500 / caller-handled |
| `BadSignature` | `_resign_notifications_chunk` fails to unsign a row and `unsafe=False` | internal |

---

## Query Inventory (for sqlc)

| Query name | Type | Tables | Description |
|---|---|---|---|
| `GetLastTemplateUsage` | SELECT | `notifications` | Most recent non-test notification by template, type, and service; ORDER BY created_at DESC LIMIT 1 |
| `CreateNotification` | INSERT | `notifications` | Insert a single notification row |
| `BulkInsertNotifications` | INSERT | `notifications` | Batch insert notifications |
| `UpdateNotificationStatusByID` | SELECT FOR UPDATE + UPDATE | `notifications` | Lock and update status + delivery metadata by PK |
| `UpdateNotificationStatusByReference` | SELECT + UPDATE | `notifications` | Update status by provider reference |
| `UpdateNotificationByID` | UPDATE | `notifications` | Set `updated_at` and any changed fields |
| `BulkUpdateNotificationStatuses` | UPDATE (batch) | `notifications` | Batch update status + bounce fields |
| `UpdateNotificationsByReference` | UPDATE | `notifications`, `notification_history` | Batch update by reference list; fall through to history |
| `GetNotificationForJob` | SELECT | `notifications` | Single notification scoped to service + job + ID |
| `ListNotificationsForJob` | SELECT | `notifications` | Paginated, ordered by `job_row_number` ASC |
| `CountNotificationsForJob` | COUNT | `notifications` | Row count for a job |
| `GetNotificationWithPersonalisation` | SELECT + JOIN | `notifications`, `templates` | Single notification with template eagerly loaded |
| `GetNotificationByID` | SELECT | `notifications` | Simple PK lookup (reader replica) |
| `ListNotificationsForService` | SELECT | `notifications` | Paginated, multi-filter listing with deferred personalisation |
| `ListNotificationsForServiceWithCSV` | SELECT + JOINs | `notifications`, `templates`, `jobs`, `users` | Same as above with all related data for CSV export |
| `DeleteNotificationsOlderThan` | DELETE | `notifications` | Batched soft-retention delete (up to `qry_limit` rows) |
| `DeleteTestKeyNotificationsOlderThan` | DELETE | `notifications` | Batched delete for test-key rows only |
| `UpsertNotificationHistory` | INSERT … ON CONFLICT DO UPDATE | `notification_history` | Archive notifications before deletion |
| `DeleteNotificationByID` | DELETE | `notifications` | Single-row hard delete |
| `TimeoutNotificationsToTechnicalFailure` | SELECT + UPDATE | `notifications` | Bulk created→technical-failure for old non-letter notifications |
| `TimeoutNotificationsToTemporaryFailure` | SELECT + UPDATE | `notifications` | Bulk sending/pending→temporary-failure for old non-letter notifications |
| `IsDeliverySlowForProvider` | SELECT (aggregate) | `notifications` | Slow/total ratio for a provider since a given time |
| `SearchNotificationsByRecipient` | SELECT | `notifications` | LIKE search on `normalised_to`, exclude test keys |
| `GetNotificationByReference` | SELECT | `notifications` | Exact reference lookup (reader replica) |
| `GetNotificationHistoryByReference` | SELECT | `notifications`, `notification_history` | Reference lookup; falls back to history |
| `GetNotificationsByReferences` | SELECT (IN) | `notifications` | Bulk reference lookup |
| `CreateScheduledNotification` | INSERT | `scheduled_notifications` | Insert pending scheduled send |
| `GetDueScheduledNotifications` | SELECT + JOIN | `notifications`, `scheduled_notifications` | All pending scheduled sends past their `scheduled_for` |
| `MarkScheduledNotificationProcessed` | UPDATE | `scheduled_notifications` | Set `pending = False` by notification ID |
| `GetPerformancePlatformStats` | SELECT (aggregate) | `notifications` | Total sent + within-10s count for a date range |
| `GetLatestSentNotificationForJob` | SELECT | `notifications` | Most-recently-updated notification in a job |
| `GetLastNotificationForJob` | SELECT | `notifications` | Highest `job_row_number` in a job |
| `GetUnsentNotifications` | SELECT | `notifications` | Stuck-in-created notifications older than threshold |
| `GetOldLettersWithCreatedStatus` | SELECT | `notifications` | Letters past the 5:30 PM BST cutoff still in created |
| `GetLettersPendingVirusCheck` | SELECT | `notifications` | Letters stuck in pending-virus-check > 90 min |
| `GetSendMethodStatsByService` | SELECT (aggregate) | `notification_history`, `services`, `organisations` | Delivered counts grouped by service / type / send method |
| `GetOverallBounceRateForDay` | SELECT (aggregate) | `notifications` | 24h hard-bounce rate across all services |
| `GetServiceBounceRateForDay` | SELECT (aggregate) | `notifications` | 24h hard-bounce rate for one service |
| `GetNotificationVolumeByHour` | SELECT (aggregate) | `notifications` | Hourly email volume for a service |
| `GetHardBouncesByHour` | SELECT (aggregate) | `notifications` | Hourly hard-bounce email count for a service |
| `CountNotificationsInChunk` | COUNT | `notifications` | Total row count for resign pagination |
| `GetNotificationsChunkForResign` | SELECT | `notifications` | Ordered slice for personalisation re-signing |
