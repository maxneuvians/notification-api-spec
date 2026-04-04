# Behavioral Spec: Miscellaneous (Gap Fill)

## Processed Files
- [x] tests/app/api_key/test_rest.py
- [x] tests/app/dao/test_api_key_dao.py
- [x] tests/app/dao/test_reports_dao.py
- [x] tests/app/performance_platform/test_total_sent_notifications.py
- [x] tests/app/test_sms_fragment_utils.py

---

## API Key Endpoint Contracts (test_rest.py)

### GET /api_key/stats/`<api_key_id>`

- **Happy path (with sends):** Returns `api_key_id` (UUID string), `email_sends`, `sms_sends`, `total_sends` counts computed from persisted notifications linked to that key.
- **Happy path (no sends):** Returns all count fields as `0`; `last_send` is `null`.
- **Response shape:** `{ "data": { "api_key_id": str, "email_sends": int, "sms_sends": int, "total_sends": int, "last_send": datetime|null } }`
- **Auth:** Admin request (internal API).

### GET /api_key/ranked?n_days_back=`<N>`

- Returns a list of API keys ordered descending by notification count.
- Each item includes: `api_key_name`, `service_name`, `sms_notifications`, `email_notifications`, `total_notifications`, `last_notification_created`.
- `n_days_back` controls the lookback window for the ranking.
- The key with the highest total count appears at index 0; ties broken by insertion order of results.
- Counts are broken down by channel (sms vs email).

### POST /sre_tools/revoke_api_keys

Route lives on the `sre_tools` blueprint, not the `api_key` blueprint.

- **Auth:** Requires SRE authorization header. Missing header → `401`.
- **Request body (all fields required):**
  | Field    | Type   | Notes                          |
  |----------|--------|--------------------------------|
  | `token`  | string | Full prefixed key secret (see format below) |
  | `type`   | string | Source type label (e.g. `"cds-tester"`) |
  | `url`    | string | URL where the key was found    |
  | `source` | string | Scanner/source identifier      |

- **Token format:** `gcntfy-keyname-{service_id}-{unsigned_secret}` — parsed to look up the key.
- **Happy path (201):**
  - Sets `expiry_date` on the matched `ApiKey` to effectively revoke it.
  - Stores `compromised_key_info` JSON containing at minimum `type`, `url`, `source`, and `time_of_revocation`.
  - Calls `send_api_key_revocation_email` exactly once (notifies the service owner).
- **Missing any required field → 400.**
- **All fields present but token does not resolve to a known key → 200** (not an error; key may already be absent or token format is invalid without a parseable service ID).
- **No auth header → 401.**

---

## API Key DAO Contracts (test_api_key_dao.py)

### save_model_api_key

- Persists a new `ApiKey`; initial `version` is `1`.
- `last_used_timestamp` is `None` at creation time.
- Creates exactly one history record on the first save.
- Does **not** create or increment `Service` history records.
- **Duplicate name constraint:** Two active keys for the same service with the same name → `IntegrityError`.
- **Name reuse allowed if previous key is expired:** Creating a new key with the same name as an expired key succeeds; both rows coexist in the table.

### expire_api_key

- Sets `expiry_date` to a value ≤ `utcnow()`.
- Preserves `secret` and `id`.
- Creates a second history record; history versions become 1 and 2.

### update_last_used_api_key

- Writes `last_used_timestamp` to the provided datetime value.
- Does **not** create a new history record (version stays at 1).

### update_compromised_api_key_info

- Writes the supplied dict to `compromised_key_info`.
- Preserves `secret`, `id`, `service_id`.
- Creates a new history record; versions become 1 and 2.

### get_model_api_keys

- **By `service_id` only:** Returns all keys for that service that are either active or expired within the last 7 days.
- **Revoked-and-old filter:** API keys whose `expiry_date` is more than 7 days in the past are excluded from the results. Keys expired ≤ 7 days ago are still returned.
  - `days_old=5` → key is returned (length 1).
  - `days_old=8` → key is excluded (length 0).
- **By `service_id` + `id`:** Returns the specific key.
- **Non-existent `id` → `NoResultFound`.**

### get_unsigned_secrets

- Returns a list of unsigned (plaintext) secret values for all keys belonging to a service.
- The stored `_secret` column (HMAC-signed) differs from the returned value; the returned value equals `key.secret`.

### get_unsigned_secret

- Returns the single unsigned secret for a given key ID.
- Same contract: returned value equals `key.secret`, not `key._secret`.

### get_api_key_by_secret

- **Expected token format:** `gcntfy-keyname-{service_id}-{unsigned_secret}`.
- **Valid token → returns matching `ApiKey`.**
- **No `gcntfy-keyname-` prefix → `ValueError`.**
- **Prefix present but service ID portion is not a valid UUID → `ValueError`.**
- **Prefix & valid structure but service ID in prefix does not match any service → `ValueError`.**
- **Prefix & parseable service ID & valid structure but wrong secret value → `NoResultFound`.**

### resign_api_keys

- **`resign=False` (preview):** Reads and verifies all keys; makes no changes. Unsigned value and `_secret` are unchanged.
- **`resign=True`:** Re-signs all keys with the current active signing key. Unsigned `secret` value is preserved; `_secret` column changes.
- **Key rotation safety:** Requires the old signing key to still be present in the signer's accepted-key list. If the old key is not present → `BadSignature`.
- **`resign=True, unsafe=True`:** Forces re-sign even when the existing signature cannot be verified with any current key (e.g., old key fully removed). Unsigned value preserved; `_secret` changes.

---

## Reports DAO Contracts (test_reports_dao.py)

### create_report

- Persists a `Report` row with `id`, `report_type` (`ReportType.SMS` or `ReportType.EMAIL`), `service_id`, `status` (`ReportStatus.REQUESTED`).
- Returns the created object with identity fields intact.

### get_reports_for_service

- **Parameters:** `service_id` (UUID), `limit_days` (int).
- **Date filter:** Excludes reports where `requested_at` < `utcnow() - timedelta(days=limit_days)`. A 35-day-old report is excluded when `limit_days=7`.
- **Sorting:** Results are ordered by `requested_at` descending (newest first).
- **Empty result:** Returns `[]` when no reports exist for the service within the time window.
- **Multi-type:** The query returns reports of all `report_type` values (SMS, EMAIL) without channel filtering unless the caller provides additional filters.
- **Count example (7-day window, 5 reports at 1–5 days old + 1 report at 35 days old):** Returns exactly 5 records.

---

## Performance Platform — Total Sent Notifications (test_total_sent_notifications.py)

### send_total_notifications_sent_for_day_stats

- Sends exactly one call to `performance_platform_client.send_stats_to_performance_platform`.
- **Payload shape:**

  | Field        | Value / Rule                                                            |
  |--------------|-------------------------------------------------------------------------|
  | `dataType`   | `"notifications"` (literal)                                             |
  | `service`    | `"govuk-notify"` (literal)                                              |
  | `period`     | `"day"` (literal)                                                       |
  | `channel`    | `notification_type` arg (e.g. `"sms"`, `"email"`)                       |
  | `_timestamp` | Local midnight of the calendar date derived from `start_time` (UTC→EST) |
  | `count`      | `count` arg (integer)                                                   |
  | `_id`        | Base64 encoding of `"{date}govuk-notify{channel}notificationsday"` — unique identifier for idempotency |

- **Timezone note:** `start_time` is UTC; the `_timestamp` field is the **local** (EST) midnight of the resulting calendar date. E.g. `datetime(2016, 10, 16, 4, 0, 0)` UTC → `"2016-10-16T00:00:00"` EST midnight.

### get_total_sent_notifications_for_day

- Queries `ft_notification_status` (fact-table) for a specific UTC date.
- Returns a dict keyed by notification channel: `{"email": int, "sms": int, "letter": int}`.
- The target date must be before "today"; today's data is excluded (only prior-day rows in the fact table are counted).
- Counts reflect the `count` column of the fact table, summed per channel.

---

## SMS Fragment Utils (test_sms_fragment_utils.py)

### fetch_todays_requested_sms_count

- **Cache key:** `sms_daily_count_cache_key(service_id)` (from `notifications_utils`).
- **Redis hit (value present):** Returns the cached integer value; does **not** write back to Redis.
- **Redis miss (key absent / `None`):** Calls `fetch_todays_total_sms_count` (DB query), writes the result to Redis with `ex=7200` (2-hour TTL), returns the DB value.
- **Parametric examples:**
  | redis_value | db_value | expected_result |
  |-------------|----------|-----------------|
  | `None`      | `5`      | `5` (from DB, cached) |
  | `"3"`       | `5`      | `3` (from Redis) |

### increment_todays_requested_sms_count

- Calls `redis.incrby(cache_key, increment_by)` unconditionally (regardless of whether the key was already populated).
- Does not set or seed the cache key before incrementing.

### fetch_todays_requested_sms_billable_units_count

- Same Redis-or-DB caching logic as `fetch_todays_requested_sms_count`, using cache key `billable_units_sms_daily_count_cache_key(service_id)`.
- **Redis hit:** Returns cached integer; no Redis write.
- **Redis miss:** Reads DB via `fetch_todays_total_sms_billable_units`, writes to Redis with `ex=7200`, returns DB value.
- **When `REDIS_ENABLED=False`:** Reads directly from `fetch_todays_total_sms_billable_units` (DB); no Redis interaction whatsoever.
- **Parametric examples:**
  | redis_value | db_value | expected_result |
  |-------------|----------|-----------------|
  | `None`      | `10`     | `10` (from DB, cached) |
  | `"15"`      | `10`     | `15` (from Redis) |
- **Feature flag:** Protected by `FF_USE_BILLABLE_UNITS` (TODO: remove after go-live).

### increment_todays_requested_sms_billable_units_count

- **When `REDIS_ENABLED=True`:** Calls `redis.incrby(billable_units_cache_key, increment_by)`.
- **When `REDIS_ENABLED=False`:** Does nothing; no Redis call is made.
- **Parametric examples:**
  | redis_value | db_value | increment_by |
  |-------------|----------|--------------|
  | `None`      | `10`     | `5`          |
  | `"20"`      | `10`     | `3`          |
