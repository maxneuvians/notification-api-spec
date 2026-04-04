# Behavioral Spec: Jobs & Celery Tasks

## Processed Files

- [x] `tests/app/job/test_rest.py`
- [x] `tests/app/dao/test_jobs_dao.py`
- [x] `tests/app/celery/test_tasks.py`
- [x] `tests/app/celery/test_research_mode_tasks.py`
- [x] `tests/app/celery/test_scheduled_tasks.py`
- [x] `tests/app/celery/test_nightly_tasks.py`
- [x] `tests/app/celery/test_ftp_update_tasks.py`

---

## Endpoint Contracts (Job REST API)

### GET /service/{service_id}/job

- **Happy path**: Returns a paginated list of all jobs for the service. Response body contains `data`, `page_size`, `total`, and `links` fields. Jobs are ordered by `processing_started` descending then `created_at` descending. Statistics are embedded per job as `[{status, count}]` arrays.
- **Filtering by status**: Accepts a `statuses` query parameter (comma-separated list). Returns only jobs matching those statuses. Invalid/unknown statuses return an empty list rather than an error.
- **Filtering by age**: Accepts a `limit_days` query parameter. Returns only jobs whose `created_at` falls within the last N full calendar days (i.e., today plus the previous N-1 days). Jobs created exactly on day N are included; jobs created before the start of day N are excluded.
- **Pagination**: Accepts `page` parameter. Uses server-side `PAGE_SIZE` config. Response contains `links.next`, `links.last` (and `links.prev` on pages > 1).
- **Statistics source**: For jobs whose `processing_started` is within the last 3 days, statistics come from the live `notifications` table. For older jobs, statistics come from the `ft_notification_status` fact table. Jobs with `processing_started IS NULL` return `statistics = []`.
- **Validation rules**: `service_id` must match an existing service; mismatched or unknown service returns an empty list (not a 404 from this endpoint, since it's a list query).
- **Auth requirements**: Requires authorization header.

### GET /service/{service_id}/job/{job_id}

- **Happy path**: Returns `{ data: { id, statistics, created_by: { name }, ... } }`. Statistics summed from live notifications table.
- **Error cases**:
  - Non-UUID `job_id`: 404 `{ result: "error", message: "No result found" }`.
  - UUID that does not exist: 404 `{ result: "error", message: "Job not found in database" }`.
- **Auth requirements**: Requires authorization header.

### POST /service/{service_id}/job

- **Happy path (unscheduled)**: Creates a job with `job_status = "pending"`, sets `template`, `original_file_name`, `notification_count`, `statistics = []`, `scheduled_for = null`. Enqueues `process_job.apply_async([job_id], queue="job-tasks")` immediately. Returns 201.
- **Happy path (scheduled)**: When `scheduled_for` is provided and within the allowed window, creates a job with `job_status = "scheduled"`, stores `scheduled_for` as UTC. Does **not** enqueue `process_job`. Returns 201.
- **Sender ID**: If the S3 metadata contains a `sender_id`, that value is stored on the job and returned in the response. If no `sender_id` in metadata but the service has an inbound number, the service's default SMS sender ID is used.
- **Daily count increment**: For unscheduled email jobs, `increment_email_daily_count_send_warnings_if_needed` is called with the notification count. For jobs scheduled on a future calendar day (> ~24–36 hours out), this call is **not** made.
- **Billable units feature flag (`FF_USE_BILLABLE_UNITS`)**:
  - When **enabled**: `check_sms_annual_limit`, `check_sms_daily_limit`, and `increment_sms_daily_count_send_warnings_if_needed` are called with `RecipientCSV.sms_fragment_count` (total fragments across all recipients).
  - When **disabled**: The same functions are called with `csv_length` (one unit per recipient, regardless of message length).
  - Simulated phone numbers (e.g., `+1613...`) skip all limit checks entirely when the flag is enabled.
- **Validation rules**:
  - `id` field is required: 400 `{ message: { id: ["Missing data for required field."] } }`.
  - S3 metadata must contain `original_file_name` and `notification_count`: 400 with `message.original_file_name` / `message.notification_count` errors.
  - S3 metadata must have `valid = "True"` (exact string): 400 `{ message: "File is not valid, can't create job" }`.
  - Template must exist: 404.
  - Template must not be archived: 400 `{ message: { template: ["Template has been deleted"] } }`.
  - `scheduled_for` must not be in the past: 400 `{ message: { scheduled_for: ["Date cannot be in the past"] } }`.
  - `scheduled_for` must be ≤ 96 hours in the future: 400 `{ message: { scheduled_for: ["Date cannot be more than 96 hours in the future"] } }`.
- **Error cases**:
  - Inactive service: 403 `{ result: "error", message: "Create job is not allowed: service is inactive " }`. DAO `dao_create_job` is **not** called.
  - Annual limit exceeded (SMS or email, live or trial): 429 `{ message: "Exceeded annual {TYPE} sending limit of {N} messages" }`.
  - Service not found: 404.
- **Auth requirements**: Requires authorization header.

### POST /service/{service_id}/job/{job_id}/cancel

- **Happy path**: Cancels a scheduled job (one with `job_status = "scheduled"`). Returns 200 with `{ data: { id, job_status: "cancelled" } }`.
- **Error cases**: Cannot cancel a non-scheduled job (e.g., `pending`, `in progress`, `finished`): returns 404. The `dao_update_job` DAO function is **not** called.
- **Auth requirements**: Requires authorization header.

### GET /service/{service_id}/job/{job_id}/notifications

- **Happy path**: Returns `{ notifications: [...] }` ordered by `job_row_number` ascending. Each item includes `id`, `to`, `job_row_number`, `status`.
- **Filtering**: Accepts `status` query parameter (array). Only notifications matching those statuses are returned.
- **CSV format**: When `format_for_csv=true`, each notification item has the keys: `created_at`, `created_by_name`, `created_by_email_address`, `template_type`, `template_name`, `job_name`, `status`, `row_number`, `recipient`. No other keys.
- **Isolation**: Returns only notifications belonging to the specified job; notifications from other jobs on the same template are excluded.
- **Auth requirements**: Requires authorization header (admin request pattern).

### GET /service/{service_id}/job/has_jobs

- **Happy path**: Returns 200 `{ data: { has_jobs: true } }` when at least one job exists for the service; `{ data: { has_jobs: false } }` otherwise.
- **Auth requirements**: Requires authorization header.

---

## DAO Behavior Contracts

### `dao_create_job`

- **Expected behavior**: Persists a `Job` model to the database. New jobs initialize `notifications_delivered = 0` and `notifications_failed = 0`. The record is retrievable by ID immediately after creation.
- **Edge cases verified**: None beyond basic persistence.

### `dao_get_job_by_service_id_and_job_id`

- **Expected behavior**: Returns the `Job` matching both `service_id` and `job_id`. Returns the exact same object as created by the tests.

### `dao_get_jobs_by_service_id`

- **Expected behavior**: Returns a paginated result. Results are ordered by `processing_started DESC` then `created_at DESC` (jobs with no `processing_started` sort lower than jobs with a `processing_started`).
- **`limit_days` parameter**: When provided, excludes jobs with `created_at` before midnight at the start of the `(today - limit_days + 1)`-th day. Jobs created exactly at midnight of the boundary day are included.
- **Edge cases verified**:
  - Results from one service do not bleed into another service's results.
  - Pagination: `per_page`, `total`, and item slicing work correctly. Page 1 returns the newest jobs first.
  - Jobs from exactly 7 days ago with `created_at = datetime(date, 0, 0, 0)` are included in a `limit_days=7` query.

### `dao_get_jobs_older_than_data_retention`

- **Expected behavior**: Returns jobs older than the data retention period for the given `notification_types`. Default retention is 7 days. Only returns jobs in a two-day sliding window (between 7 and 9 days old). Already-archived jobs are excluded.
- **`scheduled_for` behaviour**: If a job has a `scheduled_for` date, that date overrides `created_at` for the retention age calculation. A job created 8 days ago but `scheduled_for` 6 days ago is **not** returned.
- **Type filter**: Only returns jobs whose template type is in the provided `notification_types` list.
- **Custom retention**: Services with a `service_data_retention` override use that retention period instead of 7 days.
- **`limit` parameter**: When provided, caps total results returned across all services; default is no limit.

### `dao_get_notification_outcomes_for_job`

- **Expected behavior**: Returns a list of `(status, count)` aggregates for notifications belonging to `(service_id, job_id)`. Counts both live `notifications` and `notification_history` rows.
- **Edge cases verified**:
  - Returns an empty list when no notifications exist.
  - Only counts notifications for the specified job, not other jobs on the same service.
  - Only counts notifications for the specified service; cross-service isolation confirmed.
  - Merges counts from both `notifications` and `notification_history` tables.
- **Decorator**: Function is wrapped (verified via `__wrapped__`).

### `dao_update_job`

- **Expected behavior**: Persists in-memory mutations to a `Job` object (e.g., `job_status` changes) to the database.

### `dao_set_scheduled_jobs_to_pending`

- **Expected behavior**: Finds all jobs with `job_status = "scheduled"` whose `scheduled_for` is in the past (≤ now), sets their status to `"pending"`, and returns the updated list ordered by `scheduled_for` ascending (oldest first).
- **Edge cases verified**:
  - Jobs scheduled in the future are **not** touched.
  - Jobs with any status other than `"scheduled"` are ignored.

### `dao_get_future_scheduled_job_by_id_and_service_id`

- **Expected behavior**: Returns a scheduled job that has not yet been triggered (i.e., `scheduled_for` is in the future).

### `dao_cancel_letter_job`

- **Expected behavior**: Sets `job.job_status = "cancelled"` and sets all associated notifications with status `"created"` to `"cancelled"`. Returns the count of cancelled notifications.

### `can_letter_job_be_cancelled`

- **Expected behavior**: Returns `(True, None)` when all conditions for cancellation are met.
- **Returns `(False, error_message)` in these cases**:
  - Any notification for the job is in `"sending"` status → `"It's too late to cancel sending, these letters have already been sent."`
  - The job's template is not a letter type → `"Only letter jobs can be cancelled through this endpoint. This is not a letter job."`
  - The job's `job_status` is not `"finished"` (e.g., still `"in progress"`) → `"We are still processing these letters, please try again in a minute."`
  - The number of notifications in the DB does not match the job's `notification_count` → `"We are still processing these letters, please try again in a minute."` (not all notifications have been created yet).

### `dao_service_has_jobs`

- **Expected behavior**: Returns `True` if any job exists for the given `service_id`; `False` otherwise.

---

## Task Behavior Contracts

### `process_job`

- **What the task does**: Reads the recipient CSV from S3, builds a signed notification payload for each row, and dispatches them to `save_smss` or `save_emails` as a single batched `apply_async` call on the appropriate database queue. Sets `job.job_status = "in progress"` and records `processing_started` timestamp.
- **Input conditions and expected outcomes**:
  - SMS template → calls `save_smss.apply_async(..., queue=NORMAL_DATABASE)`.
  - Email template → calls `save_emails.apply_async(..., queue=NORMAL_DATABASE)`.
  - Each signed notification includes: `to`, `template`, `template_version`, `personalisation`, `row_number`, `job`, `queue`, `sender_id`.
  - Jobs with a `sender_id` pass it through to the signed payload.
  - Empty CSV file: job is set to `"in progress"` but no `save_*` task is dispatched.
  - Processing exactly at send limit: allowed (job processes normally).
- **Side effects verified**:
  - `statsd_client.timing_with_dates("job.processing-start-delay", processing_started, created_at)` is called once.
  - `job.job_status` transitions to `"in progress"`.
  - `job.processing_started` is set.
- **Retry/failure behavior verified**:
  - If the service is inactive when the task runs, the job is set to `"cancelled"` and no S3 read or row processing occurs.
- **Decorator**: Function is wrapped (verified via `__wrapped__`).

### `process_rows`

- **What the task does**: Taking a list of pre-parsed CSV rows plus template/job/service objects, signs each row as a notification and dispatches `save_smss` or `save_emails`. Handles restricted-service validation inline.
- **Input conditions and expected outcomes**:
  - Research-mode service → queue is `"research-mode-tasks"`.
  - Non-research SMS → `"-normal-database-tasks"` (NORMAL_DATABASE).
  - Non-research email → `"-normal-database-tasks"`.
  - Template `process_type = PRIORITY` and notification count ≥ 200 → routed to `BULK_DATABASE`.
  - Template `process_type = NORMAL` and notification count ≥ 200 → routed to `BULK_DATABASE`.
  - `CSV_BULK_REDIRECT_THRESHOLD` config: when the row count exceeds the threshold, SMS is routed to `SEND_SMS_LOW`, email to `SEND_EMAIL_LOW`, regardless of template process type. Priority templates switch to bulk if threshold is met.
  - Restricted service + non-whitelisted recipient phone number → notification is **not** sent; `save_smss` is not called.
  - Job has no API key (empty dict `api_key`) → `key_type` defaults to `KEY_TYPE_NORMAL`.
- **Side effects verified**:
  - Signed payload contains: `api_key`, `key_type`, `template`, `template_version`, `job`, `to`, `row_number`, `personalisation`, `queue`, `sender_id`, `client_reference`.

### `save_smss`

- **What the task does**: Batch-persists a list of signed SMS notification payloads and dispatches `deliver_sms.apply_async` for each. Calls `acknowledge` on the receipt after persisting.
- **Input conditions and expected outcomes**:
  - Standard service → `deliver_sms.apply_async([id], queue=SEND_SMS_MEDIUM)`.
  - Research-mode service → `deliver_sms.apply_async([id], queue="research-mode-tasks")`.
  - Template `process_type = "priority"` → `deliver_sms.apply_async([id], queue=SEND_SMS_HIGH)`.
  - Template `process_type = "bulk"` → `deliver_sms.apply_async([id], queue=SEND_SMS_LOW)`.
  - Notification carries `queue = SEND_SMS_LOW` (set by bulk redirect) → `deliver_sms.apply_async([id], queue=SEND_SMS_LOW)`.
  - Service has a custom SMS sender (non-default) → `deliver_throttled_sms.apply_async([id], queue="send-throttled-sms-tasks")` instead; `reply_to_text` is set to the sender value.
  - Notification has an explicit `sender_id` → `deliver_throttled_sms` is used; `reply_to_text` is looked up via `dao_get_service_sms_senders_by_id`.
  - Restricted service + valid whitelisted number → saved normally.
  - `reply_to_text` defaults to the service's default SMS sender value.
  - Non-default sender via `sender_id` → `reply_to_text` is the non-default sender's value.
  - Job association: `job_id`, `job_row_number`, `api_key_id = None`, `key_type = KEY_TYPE_NORMAL` are persisted correctly.
  - Personalisation is stored encrypted via `signer_personalisation.sign`.
- **Retry/failure behavior verified**:
  - `SQLAlchemyError` during `bulk_insert_notifications` → `save_smss.retry(exc=exception, queue="retry-tasks")`. No delivery task dispatched. Notification count remains 0.
  - Duplicate `IntegrityError` (notification ID already exists) → delivery task **not** called; retry **not** called; receipt is still acknowledged.
- **Side effects verified**:
  - `acknowledge` is called once with the receipt UUID after processing.
  - `put_batch_saving_bulk_created` metric is emitted.
- **Decorator**: Function is wrapped (verified via `__wrapped__`).

### `save_emails`

- **What the task does**: Batch-persists signed email notification payloads and dispatches `deliver_email.apply_async` for each. Calls `acknowledge` on the receipt.
- **Input conditions and expected outcomes**:
  - Standard service → `deliver_email.apply_async([id], queue=SEND_EMAIL_MEDIUM)`.
  - Research-mode service → `deliver_email.apply_async([id], queue="research-mode-tasks")`.
  - Template `process_type = "priority"` → `SEND_EMAIL_HIGH`.
  - Template `process_type = "bulk"` → `SEND_EMAIL_LOW`.
  - Notification carries `queue = SEND_EMAIL_LOW` (bulk redirect) → `SEND_EMAIL_LOW`.
  - `reply_to_text`: set from service default reply-to if `sender_id` is null; set from explicit reply-to address if `sender_id` is provided.
  - Template version on the notification is frozen at the time of job creation; later template updates do **not** change existing notification records.
  - Personalisation is stored encrypted.
  - Redis cache is used to retrieve service and template data when available (avoids DB lookups).
- **Retry/failure behavior verified**:
  - `SQLAlchemyError` → `save_emails.retry(exc=exception, queue="retry-tasks")`. No delivery task dispatched.
  - Duplicate `IntegrityError` → delivery task not called; retry not called; receipt still acknowledged.
- **Side effects verified**: `acknowledge` called once.
- **Decorator**: Function is wrapped (verified via `__wrapped__`).

### `handle_batch_error_and_forward`

- **What the task does**: Error handler for batch save failures. On error for a batch of N notifications: if N=1, calls `save_smss.retry`; if N>1, re-dispatches each notification individually as separate `save_smss.apply_async` calls, each with a `None` receipt.
- **Input conditions and expected outcomes**:
  - Single notification error → `retry(exc=exception, queue="retry-tasks")` called once.
  - Three-notification error → `save_smss.apply_async` called 3 times, each with `(service_id, [signed_notification], None)` and `queue="-normal-database-tasks"`.

### `process_incomplete_job`

- **What the task does**: For a job in `JOB_STATUS_ERROR`, re-reads the CSV from S3 and re-dispatches only the rows that have no existing notification in the DB (identified by row number). The already-processed rows are skipped.
- **Input conditions and expected outcomes**:
  - Job with 10 notifications total, 2 already saved → `save_smss.apply_async` called once with 8 unsent notifications.
  - Job with all 10 already saved → `save_smss` is **not** called.
  - Job with 0 notifications saved → all 10 are dispatched in one call.
  - Nonexistent job ID → raises `Exception`; `save_smss` not called.
  - Email job → uses `save_emails.apply_async`.
- **Edge cases verified**: Row-number awareness prevents re-sending already-processed rows.

### `process_incomplete_jobs`

- **What the task does**: Iterates over a list of job IDs and calls `process_incomplete_job` for each. Resets `processing_started` to now before processing each job (but does **not** change `job_status`).
- **Input conditions and expected outcomes**:
  - Empty list → no dispatches, no error.
  - Two jobs → `process_incomplete_job` called twice; `job.processing_started` reset to current time for both; `job.job_status` remains `JOB_STATUS_ERROR`.

### `update_in_progress_jobs`

- **What the task does**: Queries for all in-progress jobs and updates each job's `updated_at` to the `updated_at` of the latest sent notification for that job.
- **Input conditions and expected outcomes**:
  - In-progress job with sent notifications → `job.updated_at` is set to latest notification's `updated_at`.
  - In-progress job with no sent notifications → `dao_update_job` is **not** called.

### `acknowledge_receipt`

- **What the task does**: Acknowledges a message receipt on the correct SQS-backed queue based on notification type and priority.
- **Input conditions and expected outcomes**:
  - `(SMS_TYPE, NORMAL, receipt)` → calls `sms_normal.acknowledge(receipt)`.
  - `(EMAIL_TYPE, NORMAL, receipt)` → calls `sms_bulk.acknowledge(receipt)` (bulk queue handles email normal in this mapping).
  - `(None, None, receipt)` → raises `ValueError`.

### `choose_database_queue`

- **What the task does**: Selects the appropriate Celery database queue based on template process type, service research mode, and notification count.
- **Routing table verified**:
  | research_mode | process_type | notification_count | queue |
  |---|---|---|---|
  | True | any | any | `RESEARCH_MODE` |
  | False | PRIORITY | < 200 | `PRIORITY_DATABASE` |
  | False | PRIORITY | ≥ 200 | `BULK_DATABASE` |
  | False | NORMAL | < 200 | `NORMAL_DATABASE` |
  | False | NORMAL | ≥ 200 | `BULK_DATABASE` |
  | False | BULK | any | `BULK_DATABASE` |

### `send_inbound_sms_to_service`

- **What the task does**: POSTs inbound SMS data to the service's registered inbound API URL using HTTPS with Bearer token authentication.
- **Input conditions and expected outcomes**:
  - Sends JSON payload containing `id`, `source_number`, `destination_number`, `message`, `date_received`.
  - Uses `Content-Type: application/json` and `Authorization: Bearer {token}` headers.
- **Retry/failure behavior verified**:
  - HTTP 500 response → `retry(queue="retry-tasks")` called once.
  - `RequestException` → `retry(queue="retry-tasks")` called once.
  - HTTP 404 response → **no** retry.
  - Inbound SMS not found in DB → raises `SQLAlchemyError`; no HTTP request made.
  - No inbound API configured → no HTTP request made (no error).

### `send_notify_no_reply`

- **What the task does**: Persists a notification back to the original sender of an email that hit a no-reply address.
- **Input conditions and expected outcomes**:
  - Payload `{ sender, recipients }` → persists notification with `recipient = sender`, personalisation `{ sending_email_address: recipients[0] }`, `reply_to_text = None`.
  - Enqueues via `send_notification_to_queue` with `queue = QueueNames.NOTIFY`.
- **Retry/failure behavior verified**: `send_notification_to_queue` failure → `retry(queue=QueueNames.RETRY)` with `Retry` exception.

### `seed_bounce_rate_in_redis`

- **What the task does**: Populates Redis with historical bounce rate data for a service (total notifications and hard bounces grouped by hour).
- **Input conditions and expected outcomes**:
  - When seeding not yet started (`get_seeding_started = False`) → writes data; bounce rate is computed as `total_hard_bounces / total_notifications`.
  - When seeding already started (`get_seeding_started = True`) → skips seeding; `set_notifications_seeded` and `set_hard_bounce_seeded` are **not** called.

### `generate_report`

- **What the task does**: Generates a CSV report from notifications, uploads it to S3, and stores a presigned URL on the `Report` record.
- **Input conditions and expected outcomes**:
  - Happy path → `update_report` called twice (once to mark as `generating`, once to mark as `READY`). `Report.url` is set to the presigned URL. `generated_at` is set to now, `expires_at` is set to now + 3 days.
  - Error → `update_report` called twice (once for `generating`, once for error state). Exception is re-raised.

### `check_billable_units`

- **What the task does**: Validates that the `billable_units` on a notification matches the `page_count` from a DVLA response.
- **Input conditions and expected outcomes**:
  - `billable_units == page_count` → no error log.
  - `billable_units != page_count` → logs an error with the notification ID, the DB billable_units, and the DVLA page count.

### `get_billing_date_in_est_from_filename`

- **What the task does**: Parses the date portion of a DVLA response filename and returns the billing date in EST.
- **Input conditions and expected outcomes**:
  - `"NOTIFY-20170820230000-RSP.TXT"` → `date(2017, 8, 20)`.
  - `"NOTIFY-20170120230000-RSP.TXT"` → `date(2017, 1, 20)`.

---

## Research Mode Task Contracts

### `send_sms_response`

- **What the task does**: Generates a fake SNS or Pinpoint delivery callback and queues it on the `RESEARCH_MODE` queue.
- **Input conditions and expected outcomes**:
  - Provider `"sns"` with numbers ending in `30`/`31` → `sns_success_callback` payload dispatched to `process_sns_results.apply_async`.
  - Provider `"sns"` with numbers ending in `32`/`33` → `sns_failed_callback` payload with appropriate `provider_response` string.
  - Provider `"pinpoint"` → `pinpoint_delivered_callback` or `pinpoint_failed_callback` dispatched to `process_pinpoint_results.apply_async`.
  - Payload includes `reference`, `destination`, and UTC `timestamp` matching the current time.
  - All dispatches use `queue=QueueNames.RESEARCH_MODE`.

### `send_email_response`

- **What the task does**: Generates a fake SES delivery callback and queues it.
- **Input conditions and expected outcomes**:
  - Dispatches `ses_notification_callback(reference)` to `process_ses_results.apply_async` on `QueueNames.RESEARCH_MODE`.

### `create_fake_letter_response_file`

- **What the task does**: Uploads a fake DVLA response file to S3 with content `"{reference}|Sent|0|Sorted"`, using a timestamp-based filename. In development environment, also dispatches an SNS mock callback.
- **Input conditions and expected outcomes**:
  - File does not exist → uploads once to the DVLA response bucket.
  - File already exists → retries with a new filename (seconds incremented). Retries up to 30 times; raises `ValueError` after 30 failed attempts.
  - Environment `"development"` → additionally calls `process_sns_results.apply_async` on `QueueNames.RESEARCH_MODE`.
  - Environment `"preview"` → does **not** make the DVLA callback HTTP request.
  - Filename format: `"NOTIFY-{YYYYMMDDHHmmss}-RSP.TXT"` within a 30-second window of the current time.

---

## Scheduled Task Contracts

### `run_scheduled_jobs`

- **What the task does**: Transitions all past-due `"scheduled"` jobs to `"pending"` and enqueues each via `process_job.apply_async([job_id], queue="job-tasks")`.
- **Input conditions and expected outcomes**:
  - Single past-due job → status becomes `"pending"`; `process_job` enqueued once.
  - Multiple past-due jobs (ordered by `scheduled_for` ascending) → all updated; `process_job` called for each in chronological order.
  - No jobs past due → no calls.

### `check_job_status`

- **What the task does**: Checks for in-progress jobs that have been stuck for more than 30 minutes. Raises `JobIncompleteError` listing the stuck job IDs and enqueues `PROCESS_INCOMPLETE_JOBS` on `QueueNames.JOBS`. Sets stuck jobs to `JOB_STATUS_ERROR`.
- **Input conditions and expected outcomes**:
  - Job `updated_at` > 30 minutes ago and `job_status = IN_PROGRESS` → included; `job.job_status` set to `JOB_STATUS_ERROR`.
  - Job `updated_at` < 30 minutes ago → excluded regardless of status.
  - Multiple stuck jobs → all IDs passed in a single `send_task` call; all listed in `JobIncompleteError.message`.
  - Finished jobs → no error raised.

### `send_scheduled_notifications`

- **What the task does**: Finds scheduled notifications (status `"created"`, `scheduled_for` ≤ now) and dispatches each to the delivery queue.
- **Input conditions and expected outcomes**:
  - Notification `scheduled_for` in past with status `"created"` → `deliver_sms.apply_async([id], queue=SEND_SMS_MEDIUM)`.
  - Notification `scheduled_for` in future → not dispatched.
  - Notification with status `"delivered"` and past `scheduled_for` → not dispatched.
  - After dispatch, the scheduled notification list becomes empty (notifications cleared from scheduled state).

### `replay_created_notifications`

- **What the task does**: Re-queues notifications that are older than ~4 hours 15 minutes and still in `"created"` status (missed delivery).
- **Input conditions and expected outcomes**:
  - Old `"created"` SMS → `deliver_sms.apply_async([id], queue=SEND_SMS_MEDIUM)`.
  - Old `"created"` email → `deliver_email.apply_async([id], queue=SEND_EMAIL_MEDIUM)`.
  - `"sending"`, `"delivered"` statuses → not replayed.
  - Recently created notifications → not replayed.

### `mark_jobs_complete`

- **What the task does**: Checks in-progress or error-status jobs and marks them `"finished"` if all notifications have been created.
- **Transition logic verified**:
  | notification_count_in_job | notification_count_in_db | initial_status | expected_status |
  |---|---|---|---|
  | 3 | 0 | IN_PROGRESS | IN_PROGRESS (no change) |
  | 3 | 1 | IN_PROGRESS | IN_PROGRESS (no change) |
  | 3 | 1 | ERROR | ERROR (no change) |
  | 3 | 3 | ERROR | FINISHED |
  | 3 | 3 | IN_PROGRESS | FINISHED |
  | 3 | 10 | IN_PROGRESS | FINISHED (≥ count triggers finish) |

### `recover_expired_notifications`

- **What the task does**: Calls `expire_inflights` on all six SQS queues (sms_bulk, sms_normal, sms_priority, email_bulk, email_normal, email_priority).
- **Side effects verified**: Each queue's `expire_inflights` is called exactly once.

### `delete_verify_codes`

- **What the task does**: Delegates to `delete_codes_older_created_more_than_a_day_ago`. Verified that it is called exactly once.

### `delete_invitations`

- **What the task does**: Delegates to `delete_invitations_created_more_than_two_days_ago`. Verified that it is called exactly once.

### `beat_inbox_sms_normal` / `beat_inbox_sms_bulk` / `beat_inbox_sms_priority`

- **What the task does**: Polls the corresponding SQS inbox queue (`sms_normal`, `sms_bulk`, `sms_priority`) and, if messages are returned, dispatches them via `save_smss.apply_async`. Continues polling until the queue returns an empty batch.
- **Queue routing**:
  - `beat_inbox_sms_normal` → `queue="-normal-database-tasks"`.
  - `beat_inbox_sms_bulk` → `queue="-bulk-database-tasks"`.
  - `beat_inbox_sms_priority` → `queue="-priority-database-tasks.fifo"`.

### `beat_inbox_email_normal` / `beat_inbox_email_bulk` / `beat_inbox_email_priority`

- **What the task does**: Same pattern as SMS inbox beats but for email queues, dispatching via `save_emails.apply_async`.
- **Queue routing**:
  - `beat_inbox_email_normal` → `queue="-normal-database-tasks"`.
  - `beat_inbox_email_bulk` → `queue="-bulk-database-tasks"`.
  - `beat_inbox_email_priority` → `queue="-priority-database-tasks.fifo"`.

---

## Nightly Task Contracts

### `remove_sms_email_jobs` / `remove_letter_jobs`

- **What the task does**: Archives jobs older than the data retention window and removes their files from S3 via `s3.remove_jobs_from_s3`.
- **Default retention**: 7 days; only jobs within the 7–9 day window are deleted in each run (2-day sliding window). Already-archived jobs are excluded.
- **Custom retention**: Services with a `service_data_retention` record use that value instead of 7 days.
- **Type filter**: `remove_sms_email_jobs` targets SMS and email jobs; `remove_letter_jobs` targets letter jobs only.
- **Side effects verified**: `job.archived = True` is set on deleted jobs; jobs at exactly the 7-day boundary are **not** deleted.

### `delete_sms_notifications_older_than_retention`

- **What the task does**: Delegates to `delete_notifications_older_than_retention_by_type("sms")`. Verified called once with `"sms"`.

### `delete_email_notifications_older_than_retention`

- **What the task does**: Delegates to `delete_notifications_older_than_retention_by_type("email")`. Verified called once with `"email"`.

### `delete_letter_notifications_older_than_retention`

- **What the task does**: Delegates to `delete_notifications_older_than_retention_by_type("letter")`. Verified called once with `"letter"`.

### `timeout_notifications`

- **What the task does**: Sets notifications older than `SENDING_NOTIFICATIONS_TIMEOUT_PERIOD` still in `"sending"` or `"created"` or `"pending"` status to a failure terminal status.
- **Status transitions verified**:
  - `"sending"` → `"temporary-failure"`.
  - `"created"` → `"technical-failure"`.
  - `"pending"` → `"temporary-failure"`.
- **After timeout**: Raises `NotificationTechnicalFailureException` listing notification IDs that transitioned to `"technical-failure"`.
- **Edge cases**:
  - Notifications within the timeout period → **not** updated.
  - Letter notifications (`LETTER_TYPE`) → **never** timed out regardless of status or age.
- **Side effects verified**: If a service callback API is registered, `send_delivery_status_to_service.apply_async` is called with the signed callback data on `QueueNames.CALLBACKS`.

### `send_daily_performance_platform_stats`

- **What the task does**: Sends total send counts by notification type to the Performance Platform API.
- **Input conditions and expected outcomes**:
  - Performance Platform client is **inactive** → no stats sent.
  - Client **active** → calls `send_total_notifications_sent_for_day_stats` for `sms`, `email`, and `letter` with the correct counts for the target date.

### `delete_inbound_sms`

- **What the task does**: Delegates to `delete_inbound_sms_older_than_retention`. Verified called once.

### `remove_transformed_dvla_files`

- **What the task does**: Removes transformed DVLA files from S3 for letter jobs in the 7–9 day retention window.
- **Edge cases verified**: Jobs outside the 2-day sliding window (< 7 days old or > 9 days old) are not touched.

### `delete_dvla_response_files_older_than_seven_days`

- **What the task does**: Deletes DVLA response files from S3 that were last modified between 7 and 9 days ago.
- **Edge cases verified**: Files at exactly 7 days old, older than 9 days, or newer than 7 days are **not** deleted.

### `raise_alert_if_letter_notifications_still_sending`

- **What the task does**: Alerts via Zendesk if letters remain in `"sending"` status for more than 2 days (business days).
- **Edge cases verified**:
  - Letters `"sending"` for only 1 day → no alert.
  - On weekends → no alert (task is a no-op on Saturdays and Sundays).

### `letter_raise_alert_if_no_ack_file_for_zip`

- **What the task does**: Cross-references ZIP files sent to DVLA against ACK files received. Raises a Zendesk alert if any ZIP file is missing an ACK.
- **Edge cases verified**: If both lists are empty (no files) → no error raised; `get_list_of_files_by_suffix` called twice.

---

## Business Rules Verified

### Job status transitions verified by tests

- `"scheduled"` → `"pending"` (via `run_scheduled_jobs` or explicit cancel path reversal)
- `"pending"` → `"in progress"` (via `process_job`)
- `"in progress"` → `"finished"` (via `mark_jobs_complete` when all notifications created)
- `"in progress"` → `"error"` (via `check_job_status` after 30-minute timeout)
- `"error"` → `"finished"` (via `mark_jobs_complete` when all notifications are confirmed created)
- Any job → `"cancelled"` (via cancel endpoint, only allowed from `"scheduled"`)
- Inactive-service job → `"cancelled"` (inside `process_job` when service.active is False)

All valid job statuses (referenced in filter tests): `pending`, `in progress`, `finished`, `sending limits exceeded`, `scheduled`, `cancelled`, `ready to send`, `sent to dvla`, `error`.

### Scheduling logic verified

- A job is only directly enqueued to `process_job` when it has no `scheduled_for`, or when `scheduled_for` is on the same calendar day. Future-day scheduled jobs are stored with status `"scheduled"` and not enqueued.
- Scheduling limit: `scheduled_for` must be ≤ 96 hours in the future and must not be in the past.
- `dao_set_scheduled_jobs_to_pending` converts past-due `"scheduled"` jobs to `"pending"`; `run_scheduled_jobs` then enqueues them.
- `scheduled_for` is stored in UTC (confirmed by timezone-aware ISO format in response).

### Research mode behavior verified

- All Celery task dispatches from research-mode services route to `"research-mode-tasks"` queue.
- `send_sms_response` and `send_email_response` generate realistic-looking fake provider callbacks (SNS, Pinpoint, SES) with correct timestamps and reference values.
- Phone numbers with specific suffixes (`32`, `33`) generate failure callbacks; others generate success callbacks.
- `create_fake_letter_response_file` retries filename selection up to 30 times before giving up with `ValueError`.

### Nightly cleanup tasks verified

- Data retention default: 7 days for all notification types.
- Deletion window: jobs/notifications are only deleted within the last 7–9 day band per run (prevents bulk deletion).
- Custom per-service, per-type data retention is honoured.
- Job archival sets `job.archived = True` and removes S3 files.
- Notification timeout period is configurable via `SENDING_NOTIFICATIONS_TIMEOUT_PERIOD`. Only `"sending"`, `"created"`, and `"pending"` statuses are timed out; letter notifications are exempt.
- Performance Platform stats submission is gated on the client's `active` property.

### Queue priority routing verified

- Three delivery priority lanes: HIGH (priority templates), MEDIUM (normal templates), LOW (bulk templates).
- `CSV_BULK_REDIRECT_THRESHOLD` config causes automatic downgrade to LOW queue when the job row count exceeds the threshold, even for PRIORITY templates.
- Custom SMS senders always route to the throttled delivery queue regardless of process type.
- SQS inbox beats drain their respective queues and forward batches to the correct database-write queue (normal / bulk / priority.fifo).
