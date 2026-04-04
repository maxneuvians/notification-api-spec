# Behavioral Spec: Notifications

## Processed Files

- [x] `tests/app/notifications/rest/test_callbacks.py`
- [x] `tests/app/notifications/rest/test_send_notification.py`
- [x] `tests/app/notifications/test_callbacks.py`
- [x] `tests/app/notifications/test_notifications_ses_callback.py`
- [x] `tests/app/notifications/test_process_notification.py`
- [x] `tests/app/notifications/test_rest.py`
- [x] `tests/app/notifications/test_validators.py`
- [x] `tests/app/dao/notification_dao/test_notification_dao.py`
- [x] `tests/app/dao/notification_dao/test_notification_dao_bounce_rate.py`
- [x] `tests/app/dao/notification_dao/test_notification_dao_delete_notifications.py`
- [x] `tests/app/dao/notification_dao/test_notification_dao_performance_platform.py`
- [x] `tests/app/dao/notification_dao/test_notification_dao_template_usage.py`
- [x] `tests/app/v2/notifications/test_get_notifications.py`
- [x] `tests/app/v2/notifications/test_notification_schemas.py`
- [x] `tests/app/v2/notifications/test_post_notifications.py`
- [x] `tests/app/public_contracts/test_GET_notification.py`
- [x] `tests/app/public_contracts/test_POST_notification.py`
- [x] `tests/app/test_annual_limit_utils.py`
- [x] `tests/app/test_email_limit_utils.py`

---

## Endpoint Contracts

### POST /notifications/sms  (v0 / legacy)

**Happy path**
- Input: `{"to": "<phone>", "template": "<uuid>"}` plus optional `personalisation` dict.
- Response 201: `{"data": {"notification": {"id": "<uuid>"}, "body": "<rendered>", "template_version": N}}`
- Queues Celery task `deliver_sms` on the queue determined by template process type (see queue rules below).

**Validation rules**
- `to` and `template` are both required; missing either returns 400 with `"Missing data for required field."` per field.
- `to` must be a valid phone number; invalid → 400 `"Invalid phone number: Not a valid international number"`.
- International numbers require the `INTERNATIONAL_SMS_TYPE` service permission; without it → 400 `"Cannot send to international mobile numbers"`.
- `template` UUID must exist and belong to the requesting service; unknown → 404 `"No result found"`.
- Template type must match the endpoint type; mismatching → 400 `"{sms} template is not suitable for {email} notification"`.
- Template must not be archived; archived → 400 `"Template {id} has been deleted"`.
- Required personalisation fields must be supplied; missing → 400 `{"template": ["Missing personalisation: <Name>"]}`.
- Extra personalisation keys beyond what the template needs are silently ignored.
- Rendered SMS body must be ≤ SMS_CHAR_COUNT_LIMIT (612) characters; exceeded → 400 `"Content has a character count greater than the limit of 612"`.
- Service must have SMS permission; if not → 400 `"Cannot send text messages"`.

**Error cases**

| Condition | Status | Message |
|---|---|---|
| Missing `to` or `template` | 400 | `"Missing data for required field."` |
| Invalid phone number | 400 | `"Invalid phone number: Not a valid international number"` |
| Unknown template UUID | 404 | `"No result found"` |
| Template type mismatch | 400 | `"{type} template is not suitable for {type} notification"` |
| Archived template | 400 | `"Template {id} has been deleted"` |
| Missing personalisation | 400 | `{"template": ["Missing personalisation: <Name>"]}` |
| SMS content too long | 400 | `"Content has a character count greater than the limit of 612"` |
| Service lacks SMS permission | 400 | `"Cannot send text messages"` |
| Rate limit exceeded | 429 | `"Exceeded rate limit for key type {TYPE} of {N} requests per {INTERVAL} seconds"` |
| Annual limit exceeded | 429 | `"Exceeded annual SMS sending limit of {N} messages"` |
| Daily limit exceeded (live service) | 429 | (generic over-limit error) |
| Daily limit exceeded (trial service) | 429 | (trial-specific over-limit error) |
| Invalid notification type in path (e.g. `/notifications/letter`) | 400 | `"letter notification type is not supported, please use the latest version of the client"` |
| Completely unknown path segment (e.g. `/notifications/apple`) | 400 | `"apple notification type is not supported"` |
| SQS queue failure | exception propagated; notification is deleted from DB | — |

**Auth requirements**
- JWT Bearer token scoped to the service's API key.
- Three key types: `normal`, `team`, `test`.

**Trial-mode / restriction rules**
- `normal` key on restricted service: can only send to team members; non-member → 400 `"Can't send to this recipient when service is in trial mode – see <docs>"`.
- `team` key: can only send to team members or safelisted recipients → 400 `"Can't send to this recipient using a team-only API key (service {id}) - see <docs>"`.
- `test` key: bypasses all restrictions and limit checks; routed to `research-mode-tasks`.
- Safelist recipients: allowed for both `normal` and `team` keys regardless of service restriction setting.

**Queue selection**
- `test` key → `research-mode-tasks`
- Research mode service → `research-mode-tasks`
- Reply-to number matches throttle pattern (+14383898585) → `send-throttled-sms-tasks` (task: `deliver_throttled_sms`)
- Template `process_type = "priority"` → `send-sms-high` (`SEND_SMS_HIGH`)
- Template `process_type = "bulk"` → `send-sms-low` (`SEND_SMS_LOW`)
- Default (normal) → `send-sms-medium` (`SEND_SMS_MEDIUM`)

**Simulated recipients**
- SMS numbers 6132532222, +16132532222, +16132532223 (from `SIMULATED_SMS_NUMBERS`) return 201 but are NOT persisted and delivery task is NOT called.

**Notable edge cases**
- `reply_to_text` is stored from the default SMS sender at creation time; later changes to the sender do not retroactively alter the stored value.
- When `FF_USE_BILLABLE_UNITS` is enabled: SMS fragment count is used for daily/annual limit checks. With a 200-char message (2 fragments), `check_sms_annual_limit` and `check_sms_daily_limit` are called with `billable_units=2`.
- When `FF_USE_BILLABLE_UNITS` is disabled: limit checks always use count `1` regardless of message length.
- Test key notifications: limit check functions are never called.
- Simulated recipients: limit check functions are never called.
- The original `to` value (e.g. `+16502532222`) is stored verbatim; `normalised_to` stores the E.164-normalised form.

---

### POST /notifications/email  (v0 / legacy)

**Happy path**
- Input: `{"to": "<email>", "template": "<uuid>"}` plus optional `personalisation`.
- Response 201: `{"data": {"notification": {"id": "<uuid>"}, "body": "<rendered>", "subject": "<rendered>", "template_version": N}}`
- Response body is **plain text** (no HTML); newlines preserved as-is.
- Queues `deliver_email` on the queue determined by template process type.

**Validation rules**
- `to` must be a valid email address; invalid → 400 `"Not a valid email address"`.
- All template-level rules identical to SMS (template existence, type match, archive, personalisation).
- Email has no character count limit for content.
- Service must have email permission; if not → 400 `"Cannot send emails"`.

**Notable edge cases**
- `subject` field is included in response; `content_char_count` is `null` for email.
- `reply_to_text` is set from the service's default email reply-to address at creation time.

---

### GET /notifications/{id}  (v0 / legacy)

**Happy path**
- Returns 200 with `{"data": {"notification": {id, status, template, to, service, body, subject?, content_char_count}}}`.
- Template body is rendered using stored personalisation.
- Notifications created by any key type are accessible by any other key type for the same service.

**Error cases**

| Condition | Status | Message |
|---|---|---|
| Notification not found | 404 | `"Notification not found in database"` |
| Malformed UUID in path | 405 | — |

**Notable edge cases**
- `subject` is only present for email notifications.
- `content_char_count` is present for SMS (reflects rendered content), `null` for email.
- Response uses the template version that was active when the notification was created (via `template_version`).

---

### GET /notifications  (v0 / legacy)

**Happy path**
- Returns 200 with `{"notifications": [...], "total": N, "page_size": N, "links": {...}}`.
- Default: excludes job notifications, excludes test-key notifications.
- Ordered newest-first.

**Query parameters**

| Param | Type | Effect |
|---|---|---|
| `template_type` | enum (sms/email/letter) | Filter by type; multiple values supported |
| `status` | enum | Filter by status; multiple values supported |
| `page` | int | Page number; invalid → 400 |
| `page_size` | int | Items per page; invalid → 400 |
| `include_jobs` | bool | Include job-created notifications (only effective with `normal` key) |

**Error cases**
- Invalid `page` or `page_size` → 400, `"Not a valid integer."`.

**Auth requirements** — same JWT approach. Each key type only sees notifications created by that key type (or the same type).

**Notable edge cases**
- Normal key with `include_jobs=true` returns job + API notifications.
- Team/test keys with `include_jobs=true`: still return only their own-type notifications (1 each).
- Pagination links contain `last`, `prev`, `next` as applicable.

---

### POST /v2/notifications/sms

**Happy path**
- Input: `{"phone_number": "<e164>", "template_id": "<uuid>"}` plus optional fields.
- Response 201: validated against `post_sms_response` schema.
- Response shape: `{id, reference, content: {body, from_number}, uri, template: {id, version, uri}, scheduled_for}`.
- Publish signed notification to Redis queue (sms_normal_publish / sms_bulk_publish / sms_priority_publish based on template process type).

**Optional request fields**

| Field | Type | Notes |
|---|---|---|
| `reference` | string | Client-supplied idempotency reference |
| `personalisation` | object | Template substitutions |
| `sms_sender_id` | UUID | Overrides default SMS sender |
| `scheduled_for` | datetime string | ISO8601; not in past; max 24 h in future |

**Validation rules**
- `phone_number` and `template_id` required; missing either → 400 with ValidationError.
- `template_id` must be a valid UUID (no whitespace, no truncation) → ValidationError `"template_id is not a valid UUID"`.
- `phone_number` must be a string (not int, not null, not array) → ValidationError `"phone_number {val} is not of type string"`.
- `phone_number` must be a valid international phone number → ValidationError `"phone_number Not a valid international number"`.
- `personalisation` must be an object (not string, etc.) → ValidationError `"personalisation {val} is not of type object"`.
- Additional properties (e.g. `email_reply_to_id` on SMS) → 400 ValidationError `"Additional properties are not allowed ({field} was unexpected)"`.
- `sms_sender_id` must reference a non-archived sender; archived → 400 BadRequestError `"sms_sender_id {id} does not exist in database for service id {id}"`.
- SMS content (after personalisation substitution) must be ≤ SMS_CHAR_COUNT_LIMIT; exceeded → 400 `"has a character count greater than"`.
- Service must have SMS permission → 400 BadRequestError `"Service is not allowed to send text messages"`.
- International numbers require `INTERNATIONAL_SMS_TYPE` permission → 400 BadRequestError `"Cannot send to international mobile numbers"`.
- Team-key with non-safelist recipient → 400 BadRequestError `"Can't send to this recipient using a team-only API key..."`.

**Error cases**

| Condition | Status | Error type |
|---|---|---|
| Missing template_id | 400 | `ValidationError: template_id is a required property` |
| Missing phone_number | 400 | `ValidationError: phone_number is a required property` |
| Template not found | 400 | `BadRequestError: Template not found` |
| Rate limit exceeded | 429 | `RateLimitError` |
| Annual limit exceeded | 429 | `BadRequestError / Annual limit error` |
| Daily limit exceeded | 429 | (TrialService or LiveService error) |
| No auth token | 401 | `AuthError: Unauthorized, authentication token must be provided` |
| Unknown URL path segment | 404 | `"The requested URL was not found on the server."` |
| Suspended service | 403 | — |

**Auth requirements** — JWT Bearer. All key types supported; test key bypasses limits.

**Queue selection** (v2)
- `sms_bulk_publish`, `sms_normal_publish`, or `sms_priority_publish` based on `template.process_type`.
- All publish signed notification blobs to Redis (not directly to Celery).

**Simulated recipients** — return 201, no persist, no publish.

**Billable units (FF_USE_BILLABLE_UNITS)**
- When enabled: `billable_units` calculated at persist time (GSM-7 fragment count). `increment_sms_daily_count_send_warnings_if_needed` called with the fragment count.
- When disabled: `billable_units = 0`; increment always called with `1`.
- Test key → increment never called.
- Simulated recipient → increment never called.

---

### POST /v2/notifications/email

**Happy path**
- Input: `{"email_address": "<email>", "template_id": "<uuid>"}` plus optional fields.
- Response 201: validated against `post_email_response` schema.
- Response shape: `{id, reference, content: {body, subject, from_email}, uri, template: {id, version, uri}, scheduled_for}`.

**Optional request fields**

| Field | Type | Notes |
|---|---|---|
| `reference` | string | Client reference |
| `personalisation` | object | Including document objects for attachments |
| `email_reply_to_id` | UUID | Must exist and not be archived |
| `scheduled_for` | datetime string | ISO8601; max 24 h in future |

**Validation rules**
- `email_address` must be valid email string (no brackets, not int/null/array) → ValidationError.
- `email_reply_to_id` must exist and not be archived → 400 BadRequestError `"email_reply_to_id {id} does not exist in database for service id {id}"`.
- `personalisation` total size must be ≤ 51,200 bytes → 400 ValidationError `"Personalisation variables size of {N} bytes is greater than allowed limit of 51200 bytes"`.
- Documents (attachment/link) in personalisation require service `UPLOAD_DOCUMENT` permission.
- Document `filename`: required (unless link method); min 2 chars; max 255 chars.
- Document `sending_method`: required; must be `"attach"` or `"link"` → ValidationError `"personalisation {method} is not one of [attach, link]"`.
- Document file (`file`): must be valid base64; invalid → 400 with decode error message.
- Document file size: must be ≤ 10 MB; exceeded → 400 ValidationError with `"and greater than allowed limit of"`.
- Document count per notification: max 10; exceeded → 400 ValidationError `"File number exceed allowed limits of 10 with number of {N}."`.
- Service must have email permission → 400 BadRequestError `"Service is not allowed to send emails"`.

**Simulated email addresses**
- simulate-delivered@notification.canada.ca
- simulate-delivered-2@notification.canada.ca
- simulate-delivered-3@notification.canada.ca
- Return 201, no persist, no publish, but document simulated URL returned for link documents.

---

### POST /v2/notifications/bulk

**Happy path**
- Input: `{"name": "<job name>", "template_id": "<uuid>", "rows": [[header,...],[val,...]] | "csv": "<csv string>"}`.
- Response 201 with job object: `{data: {id, job_status, notification_count, original_file_name, template, template_version, template_type, service, service_name, created_at, created_by, api_key, scheduled_for, sender_id, ...}}`.
- Uploads CSV to S3, creates a Job, dispatches `process_job` Celery task (unless scheduled).

**Validation rules**
- `name` required → 400 ValidationError `"name is a required property"`.
- Must supply exactly one of `rows` or `csv` (not both, not neither) → 400 BadRequestError `"You should specify either rows or csv"`.
- `template_id` must exist for the service and not be archived.
- Service must have permission to send the template type.
- `reply_to_id` must be a valid UUID → ValidationError `"reply_to_id is not a valid UUID"`.
- `reply_to_id` must exist for the service → 400 BadRequestError with `"email_reply_to_id {id} does not exist..."` or `"sms_sender_id {id} does not exist..."`.
- CSV must have required column headers (email address / phone number, plus all template placeholders) → 400 `"Missing column headers: {cols}"`.
- No duplicate recipient column headers → 400 `"Duplicate column headers: {cols}"`.
- Row count ≤ `CSV_MAX_ROWS` (configurable) → 400 `"Too many rows. Maximum number of rows allowed is {N}"`.
- Row-level errors (bad email, blank required field) → 400 `"Some rows have errors. Row N - \`field\`: message."`.
- SMS content (after personalisation) must be ≤ 612 chars per row → 400 `"has a character count greater than"`; reports the failing row number.
- Annual limit checked first (before daily limit).
- If annual limit would be exceeded by the batch size → 400 `"You only have {N} remaining messages before you reach your annual limit. You've tried to send {M} messages."`.
- If daily limit would be exceeded by the batch size → 400 `"You only have {N} remaining messages before you reach your daily limit. You've tried to send {M} messages."`.
- Team-key bulk submission: all recipients must be team/safelist members → 400 `"You cannot send to these recipients because you used a team and safelist API key."`.
- Trial service (restricted) bulk: all recipients must be team/safelist → 400 `"You cannot send to these recipients because your service is in trial mode..."`.
- Mixing simulated and real numbers in bulk with LIVE or TEAM key → 400 BadRequestError; TEST key allows mixing.
- Rate limit exceeded → 429.
- Suspended service → 403.

**`scheduled_for` rules (bulk)**
- Must be valid ISO8601.
- Cannot be in the past.
- Max 96 hours in the future (vs 24 hours for single sends).
- Non-string type (e.g. int) → ValidationError `"scheduled_for {N} is not of type string, null"`.

---

### GET /v2/notifications/{notification_id}

**Happy path**
- Returns 200 with full notification object.

**Response shape**
```
{
  id, reference,
  email_address,   // null for SMS
  phone_number,    // null for email
  line_1..6, postcode,  // null for non-letters
  type,            // "sms" | "email" | "letter"
  status,
  status_description,  // human-readable formatted status
  provider_response,
  template: {id, version, uri},
  created_at,      // DATETIME_FORMAT
  created_by_name, // user name if created_by_id set, else null
  body,            // rendered content
  subject,         // null for SMS
  sent_at,
  completed_at,
  scheduled_for,   // ISO8601 UTC if scheduled, else null
  postage          // null for non-letters
}
```

**Error cases**

| Condition | Status | Response |
|---|---|---|
| Not found | 404 | `{"message": "Notification not found in database", "result": "error"}` |
| Invalid UUID in path | 400 | `{"errors": [{"error": "ValidationError", "message": "notification_id is not a valid UUID"}], "status_code": 400}` |

**Letter status remapping** — not relevant for SMS/email but: `created/sending → accepted`, `delivered → received` in API output.

---

### GET /v2/notifications

**Happy path**
- Returns 200 with `{"notifications": [...], "links": {"current": "...", "next": "..."}}`.
- Excludes job notifications by default.
- Ordered newest-first.

**Query parameters**

| Param | Allowed values | Error if invalid |
|---|---|---|
| `template_type` | sms, email, letter | 400 `"template_type {val} is not one of [sms, email, letter]"` |
| `status` | See list below | 400 `"status {val} is not one of [...]"` |
| `older_than` | UUID | 400 `"older_than is not a valid UUID"` |
| `reference` | string | — |
| `include_jobs` | "true" | — |

**Valid status values**: `cancelled, created, sending, sent, delivered, pending, failed, technical-failure, temporary-failure, permanent-failure, provider-failure, pending-virus-check, validation-failed, virus-scan-failed, returned-letter, pii-check-failed, accepted, received`

**`status=failed`** is a composite that expands to: `technical-failure, temporary-failure, permanent-failure`.

**Notable edge cases**
- `older_than` with non-existent UUID → 200 with empty list.
- `older_than` with last notification's own ID → 200 with empty list.
- Multiple filters combined: AND semantics.

---

## DAO Behavior Contracts

### `dao_create_notification`

- **Expected behavior**: Persists a `Notification` row. Does not create a `NotificationHistory` row.
- **Constraints**: `template_id` and `template_version` are required; missing → `SQLAlchemyError`, no row created.
- **Edge cases**: Can save multiple notifications from same template (auto-generated IDs). Commit failure (e.g. bad `job_id`) rolls back; no partial save.

---

### `update_notification_status_by_id`

- **Expected behavior**: Updates `status` and `updated_at`. Returns the updated notification or `None`.
- **Allowed source states**: `created`, `sending`, `pending`, `pending-virus-check`.
- **Terminal states blocked**: `delivered`, `failed`, `permanent-failure`, `temporary-failure` cannot be changed.
- **Special transition**: `pending → permanent-failure` is silently coerced to `temporary-failure`.
- **`sent` status (international SMS)**: Only allows transition if the phone's country has full delivery receipts (e.g. Russia prefix "7"). Countries with no receipts (Taiwan "886") or carrier-only (USA "1") reject the transition; `SENT` stays `SENT`.
- **`feedback_reason`**: Can be set alongside status (e.g. for `provider-failure`).
- **Edge cases**: Non-existent notification ID returns `None`.
- **Can set `sent_by`**: provider name stored when supplying `sent_by` kwarg.

---

### `update_notification_status_by_reference`

- **Expected behavior**: Updates status by the provider reference string.
- **Allowed source state**: `sending` only.
- **Multiple matches**: Not an error here; updates all.
- **`sent` status blocked**: Same country-specific logic as by-ID.
- **Edge cases**: Non-existent reference returns `None`.

---

### `get_notification_by_id`

- **Expected behavior**: Returns notification or `None`; if `service_id` provided and mismatches, raises `NoResultFound` when `_raise=True`.
- **Edge cases**: Logs warning if not found.

---

### `get_notification_with_personalisation`

- **Expected behavior**: Returns notification with `scheduled_notification` eagerly loaded.
- **Edge cases**: Returns `None` if not found (logs warning).

---

### `get_notifications_for_service`

- **Expected behavior**: Paginated query scoped to service.
- **Filters available**: `filter_dict` (status, key_type), `limit_days`, `include_jobs` (default False), `include_one_off` (default True), `include_from_test_key` (default False), `key_type`, `count_pages`, `page_size`, `client_reference`.
- **Ordering**: newest-first.
- **`count_pages=False`**: Returns `total=None` for performance.
- **Eager loading**: `scheduled_notification` is joinedloaded to prevent N+1.
- **Constraints**: `limit_days` limits by `created_at` date only (not time), so a 1-day window includes today from midnight.

---

### `dao_get_notifications_by_to_field`

- **Expected behavior**: Partial-match search on `normalised_to`.
- **Email**: Case-insensitive partial match (`LIKE %term%`); special chars (`%`, `_`, `\`) are escaped.
- **SMS phone**: Spaces stripped before comparison; country code variants matched by normalisation.
- **`notification_type`**: Default infers from search term (phone → sms, email-like → email).
- **`statuses`**: Optional list filter; `None` returns all.
- **Ordering**: `created_at DESC`.
- **Edge cases**: Invalid phone/email search terms return empty result (no error).

---

### `dao_timeout_notifications`

- **Expected behavior**: Times out stale notifications older than `N` minutes.
  - `created` → `technical-failure`
  - `sending` → `temporary-failure`
  - `pending` → `temporary-failure`
  - `delivered` → unchanged
- **Constraints**: Does NOT affect letter notifications.
- **Edge cases**: Only affects notifications older than the threshold; future-dated notifications untouched.

---

### `delete_notifications_older_than_retention_by_type`

- **Expected behavior**: Deletes notifications past their retention period (default 7 days; service-specific override via `service_data_retention`).
- **Process**: Inserts/updates `NotificationHistory` first, then deletes from `notifications`. If history insert fails, notification is NOT deleted.
- **Scope**: One notification type per call.
- **Test key notifications**: Also deleted.
- **Returns**: Total count of deleted notifications.
- **`qry_limit` param**: Controls batch size for chunked deletion.
- **Edge cases**: One direction of retention extension (longer retention) keeps more rows.

---

### `insert_update_notification_history`

- **Expected behavior**: Inserts rows into `NotificationHistory` for notifications older than a cutoff date within a service.
- **Upsert behaviour**: Updates existing history records with latest status.
- **Constraints**: Scoped to a single service and notification type.
- **Edge cases**: Notifications not yet past the cutoff are excluded; other services' notifications excluded.

---

### `dao_update_notifications_by_reference`

- **Expected behavior**: Bulk-updates `notifications` (and `notification_history`) matching a list of reference strings.
- **Returns**: `(updated_notification_count, updated_history_count)`.
- **Edge cases**: Works when some references are in active table and others in history; returns (0, 0) if no matches.

---

### `dao_delete_notifications_by_id`

- **Expected behavior**: Hard-deletes a single notification by ID. No history created.
- **Edge cases**: Non-existent ID is a no-op.

---

### `dao_get_last_template_usage`

- **Expected behavior**: Returns the most recent (by `created_at`) `Notification` for a given template/type/service combination.
- **Ignores test-key notifications**.
- **Returns**: `None` if no usage found.

---

### `bulk_insert_notifications`

- **Expected behavior**: Batch-inserts list of `Notification` objects.
- **Constraints**: Duplicate IDs raise an exception; entire batch rolls back (no partial insert).

---

### `dao_get_scheduled_notifications`

- **Expected behavior**: Returns only notifications with a pending `scheduled_notification` (not delivered/processed).

---

### `dao_get_notification_by_reference`

- **Expected behavior**: Returns single notification by provider reference; raises `SQLAlchemyError` on zero or multiple matches.

---

### `dao_get_notification_history_by_reference`

- **Same semantics** as `dao_get_notification_by_reference` but queries `NotificationHistory`.

---

### `notifications_not_yet_sent`

- **Expected behavior**: Returns `status=created` notifications older than `N` seconds for a given type.
- **Edge cases**: `status=sending` or recent `status=created` are excluded.

---

### `is_delivery_slow_for_provider`

- **Expected behavior**: Returns `True` if the fraction of slow notifications (sent > `time_threshold` ago) to all recent notifications from the provider exceeds `threshold`.
- **Excludes**: test-key notifications, notifications with `sent_at=None`, wrong provider, `TEMPORARY_FAILURE` status.
- **Only counts**: `DELIVERED`, `PENDING`, `SENDING`.

---

### `dao_get_total_notifications_sent_per_day_for_performance_platform`

- **Expected behavior**: Returns `messages_total` (count in date range) and `messages_within_10_secs` (sent ≤ 10 s after creation).
- **Filtering**: Counts only API notifications (`api_key` set); excludes test-key, letter notifications.
- **Zero result**: Returns `(0, 0)` when no data.

---

### `send_method_stats_by_service`

- **Expected behavior**: Returns tuples of `(service_id, service_name, organisation_name, template_type, "admin"|"api", count)` for a date range.
- **Excludes**: `created` status notifications and notifications outside the time window.

---

### `overall_bounce_rate_for_day` / `service_bounce_rate_for_day`

- **Expected behavior**: Calculates `total_emails`, `hard_bounces`, and `bounce_rate` (percentage integer) within a time window.
- **`service_bounce_rate_for_day`**: Returns `None` if service has no matching data.

---

### `total_notifications_grouped_by_hour` / `total_hard_bounces_grouped_by_hour`

- **Expected behavior**: Returns list of `(hour: datetime, total_notifications: int)` tuples.
- **`total_hard_bounces_grouped_by_hour`**: Returns `[]` when no hard bounces exist.

---

### `resign_notifications`

- **Expected behavior**: Re-signs encrypted `_personalisation` fields with a new key set.
- **`resign=False`**: Preview mode — reads and verifies but does not write.
- **`resign=True, unsafe=False`**: Raises `BadSignature` if existing signature cannot be verified with current keys.
- **`resign=True, unsafe=True`**: Forces re-sign even if old key unrecognised.
- **`chunk_size`**: Controls batch size for memory-bounded operation.

---

## Business Rule Verification

### Annual / fiscal-year limits

- **Fiscal year**: April 1 → March 31.
- **Count sources**: Redis (today's intraday count) + DB `ft_notification_status` (fiscal year to yesterday).
- **80% threshold** → warning email sent once (idempotent via Redis flag `check_has_warning_been_sent`).
- **100% threshold** → over-limit email sent once + sends blocked (429).
- **Error types**:
  - Trial service: `TrialServiceRequestExceedsEmailAnnualLimitError` / `TrialServiceRequestExceedsSMSAnnualLimitError`
  - Live service: `LiveServiceRequestExceedsEmailAnnualLimitError` / `LiveServiceRequestExceedsSMSAnnualLimitError`
- **Message**: `"Exceeded annual email/SMS sending limit of {N} messages"` (HTTP 429).
- **FF_USE_BILLABLE_UNITS**: SMS annual limit uses `total_sms_billable_units_fiscal_year_to_yesterday` key; email is unaffected.
- **Warning email personalisation keys**: `service_name`, `count_en/fr`, `remaining_en/fr`, `message_limit_en/fr`, `contact_url`, `limit_reset_time_et_12hr/24hr`. SMS adds `message_type_en/fr`.

### Daily limits (SMS, FF_USE_BILLABLE_UNITS)

- SMS daily limit is compared against **billable units** (fragment count) when `FF_USE_BILLABLE_UNITS=True`, otherwise against message count.
- Near-limit email trigger: `≥ 80%` of `sms_daily_limit`.
- Over-limit blocks all subsequent sends.
- Test-key sends and simulated-number sends **do not count** toward daily limits.
- Admin one-off sends to simulated numbers do not count.
- Admin CSV sends: allowed only if **all** numbers in the batch are simulated; mixing simulated with real numbers → 400 error.
- Fragment count rules (GSM-7): 1–160 chars = 1 fragment; 161–306 = 2; 307–459 = 3; multipart max per part = 153 chars.

### Status transitions (tested)

```
created  ──►  sending  ──►  delivered      (normal)
created  ──►  sending  ──►  pending  ──►  delivered
                                    └──►  temporary-failure   (permanent-failure in pending→  becomes temporary-failure)
created  ──►  sending  ──►  permanent-failure
created  ──►  failed
created  ──►  technical-failure            (via dao_timeout_notifications)
sending  ──►  temporary-failure            (via dao_timeout_notifications)
pending  ──►  temporary-failure            (via dao_timeout_notifications)
pending-virus-check  ──►  cancelled
```

- `delivered` is a **terminal state** — no further transitions allowed.
- International SMS `sent` status: transition to `delivered` only if the country has **full** delivery receipts (phone_prefix not in no-receipt or carrier-only lists).

### Bounce rate

- Hard bounce threshold tested at: 0%, 10%, 50%, 55%, with parametrised slow vs normal notification counts.
- `is_delivery_slow_for_provider` returns `True` when `slow_count / total_count > threshold`.
- Test-key, wrong-provider, `sent_at=None`, and `TEMPORARY_FAILURE` notifications are excluded.

### Rate limiting

- Checked per `{service_id}-{key_type}` compound key in Redis.
- Rate: `service.rate_limit` requests per 60 seconds (hardcoded interval).
- `API_RATE_LIMIT_ENABLED=False` → check skipped entirely.
- Error response (v0): `{"result": "error", "message": "Exceeded rate limit for key type {TYPE} of {N} requests per {INTERVAL} seconds"}` HTTP 429.
- Error response (v2): `{"errors": [{"error": "RateLimitError", "message": "..."}], "status_code": 429}` HTTP 429.

### Deletion / data retention

- Default retention: 7 days.
- Service-specific retention: configured in `service_data_retention` table.
- Notifications deleted only after successful `NotificationHistory` upsert (no data loss guard).
- Test-key notifications subject to same retention deletion.
- `NotificationHistory` is never deleted by this process.

### Callback dispatch

- Delivery status callbacks: queued to `service-callbacks` queue via `send_delivery_status_to_service.apply_async`.
- Not dispatched if: service callback API is suspended; notification object is `None`.
- Complaint callbacks: queued similarly; `OnAccountSuppressionList` complaint also sets notification status to `permanent-failure`; regular complaints do not change notification status.

---

## Schema Contracts

### `post_sms_request` schema

**Required**
- `phone_number` (string) — valid international number
- `template_id` (string UUID)

**Optional**
- `reference` (string)
- `personalisation` (object)
- `sms_sender_id` (UUID string)
- `scheduled_for` (datetime string, ISO8601)

**Validation**
- `template_id` must be a UUID with no surrounding whitespace and correct hyphen structure.
- `personalisation` must be type `object`; strings, arrays, ints rejected.
- `scheduled_for`: must be parseable as ISO8601; not in the past; max 24 hours in future.

---

### `post_email_request` schema

**Required**
- `email_address` (string) — valid email; no brackets; ASCII
- `template_id` (string UUID)

**Optional**
- `reference` (string)
- `personalisation` (object)
- `email_reply_to_id` (UUID string)
- `scheduled_for` (datetime string, ISO8601)

---

### `get_notifications_request` schema

**Optional fields and valid values**
- `reference` (string)
- `status` (array of strings) — each element must be one of: `cancelled, created, sending, sent, delivered, pending, failed, technical-failure, temporary-failure, permanent-failure, provider-failure, pending-virus-check, validation-failed, virus-scan-failed, returned-letter, pii-check-failed, accepted, received`
- `template_type` (array of strings) — each element must be one of: `sms, email, letter`
- `include_jobs` (string `"true"`)
- `older_than` (UUID string)

---

### `get_notification_response` schema (v2)

Full response shape (all fields present, nulled when not applicable):
```json
{
  "id": "<uuid>",
  "reference": null,
  "email_address": null,
  "phone_number": "<e164>",
  "line_1": null, "line_2": null, "line_3": null,
  "line_4": null, "line_5": null, "line_6": null,
  "postcode": null,
  "type": "sms",
  "status": "created",
  "status_description": "In transit",
  "provider_response": null,
  "template": {"id": "<uuid>", "version": 1, "uri": "<url>"},
  "created_at": "2024-01-01T00:00:00.000000Z",
  "created_by_name": null,
  "body": "<rendered content>",
  "subject": null,
  "sent_at": null,
  "completed_at": null,
  "scheduled_for": null,
  "postage": null
}
```

---

### `get_notifications_response` schema (v2)

```json
{
  "notifications": [ /* array of get_notification_response */ ],
  "links": {
    "current": "/v2/notifications",
    "next": "/v2/notifications?older_than=<uuid>"
  }
}
```

---

### Delivery status callback payload (signed)

Fields verified after `signer_delivery_status.verify(...)`:
- `notification_client_reference`
- `notification_created_at` (DATETIME_FORMAT)
- `notification_id`
- `notification_provider_response`
- `notification_sent_at` (DATETIME_FORMAT)
- `notification_status`
- `notification_status_description`
- `notification_to`
- `notification_type`
- `notification_updated_at`
- `service_callback_api_bearer_token`
- `service_callback_api_url`

---

### Complaint callback payload (signed)

Fields verified after `signer_complaint.verify(...)`:
- `complaint_id`
- `notification_id`
- `reference`
- `to`
- `complaint_date` (DATETIME_FORMAT)
- `service_callback_api_url`
- `service_callback_api_bearer_token`

---

## Public API Contracts (v2)

### Public contract test coverage

The `public_contracts` tests validate two layers:

**V2 endpoints (`/v2/notifications/...`)** — validated against programmatic JSON schemas:
- `GET /v2/notifications/{id}` (SMS) → against `get_notification_response`
- `GET /v2/notifications/{id}` (email) → against `get_notification_response`
- `GET /v2/notifications` → against `get_notifications_response`

**V0 endpoints (`/notifications/...`)** — validated against fixture JSON files:
- `GET /notifications/{id}` (SMS) → `GET_notification_return_sms.json`
- `GET /notifications/{id}` (email) → `GET_notification_return_email.json`
- `GET /notifications` → `GET_notifications_return.json`
- `POST /notifications/sms` → `POST_notification_return_sms.json`
- `POST /notifications/email` → `POST_notification_return_email.json`

### Error response shape — v2

All v2 errors return JSON with this structure:
```json
{
  "status_code": 400,
  "errors": [
    {"error": "ValidationError", "message": "template_id is not a valid UUID"}
  ]
}
```

Error type strings used: `ValidationError`, `BadRequestError`, `RateLimitError`, `AuthError`, `PDFNotReadyError`.

### Error response shape — v0

```json
{"result": "error", "message": "..."}
```

Or for field-level errors:
```json
{"result": "error", "message": {"field_name": ["error message"]}}
```

### SES bounce/complaint callback processing

`get_aws_responses` return shape:
```json
{
  "message": "Delivered | Hard bounced | Soft bounced | Complaint",
  "success": true | false,
  "notification_status": "delivered | permanent-failure | temporary-failure",
  "provider_response": null | "<human readable>",
  "bounce_response": {
    "feedback_type": "NOTIFICATION_HARD_BOUNCE | NOTIFICATION_SOFT_BOUNCE",
    "feedback_subtype": "NOTIFICATION_HARD_GENERAL | NOTIFICATION_HARD_NOEMAIL | ...",
    "ses_feedback_id": "<feedback id>",
    "ses_feedback_date": "<timestamp>"
  }
}
```

**Bounce sub-type → classification mapping**:

| SES type | SES subtype | feedback_type | feedback_subtype |
|---|---|---|---|
| Permanent | General | HARD_BOUNCE | HARD_GENERAL |
| Permanent | NoEmail | HARD_BOUNCE | HARD_NOEMAIL |
| Permanent | Suppressed | HARD_BOUNCE | HARD_SUPPRESSED |
| Permanent | OnAccountSuppressionList | HARD_BOUNCE | HARD_ONACCOUNTSUPPRESSIONLIST |
| Transient | General | SOFT_BOUNCE | SOFT_GENERAL |
| Transient | MailboxFull | SOFT_BOUNCE | SOFT_MAILBOXFULL |
| Transient | MessageTooLarge | SOFT_BOUNCE | SOFT_MESSAGETOOLARGE |
| Transient | ContentRejected | SOFT_BOUNCE | SOFT_CONTENTREJECTED |
| Transient | AttachmentRejected | SOFT_BOUNCE | SOFT_ATTACHMENTREJECTED |
| Undetermined | Undetermined | SOFT_BOUNCE | SOFT_GENERAL |

**Provider response strings** (populated only for specific sub-types):
- `Suppressed` → `"The email address is on our email provider suppression list"`
- `OnAccountSuppressionList` → `"The email address is on the GC Notify suppression list"`
- `AttachmentRejected` → `"The email was rejected because of its attachments"`

**Unrecognised notification type** → `KeyError` raised.

### Annual limit utility contract

`get_annual_limit_notifications_v2(service_id)` returns:
```python
{
    "email_delivered_today": int,
    "email_failed_today": int,
    "sms_failed_today": int,
    "sms_delivered_today": int,
    "total_sms_fiscal_year_to_yesterday": int,
    "total_email_fiscal_year_to_yesterday": int,
    # When FF_USE_BILLABLE_UNITS:
    "total_sms_billable_units_fiscal_year_to_yesterday": int,
    "sms_billable_units_failed_today": int,
    "sms_billable_units_delivered_today": int,
}
```

Seeds Redis from DB if not already seeded today (`annual_limit_client.was_seeded_today` → False).

`seed_data_in_redis(service_id)`:
- Always calls `seed_annual_limit_notifications`.
- If all counts are zero (short-circuit): calls `set_seeded_at` itself to prevent infinite re-seeding loop.
- If counts non-zero: `set_seeded_at` is called internally by `seed_annual_limit_notifications`; `seed_data_in_redis` must NOT call it again.

### Email daily count cache contract

`fetch_todays_email_count(service_id)`:
- Redis key: `email_daily_count_cache_key(service_id)` (format from `notifications_utils`).
- Cache miss: fetches from DB (`fetch_todays_total_email_count`), caches with TTL = 7200 s.
- Cache hit: returns cached value directly (no DB call, no `set` called).

`increment_todays_email_count(service_id, increment_by)`:
- Uses `redis_store.incrby(cache_key, increment_by)`.
