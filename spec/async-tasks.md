# Async Tasks

## Overview

- **Total registered task count**: 53 tasks across 8 modules (5 letter PDF stubs, 3 research-mode helpers, 45 production tasks)
- **Queue count**: 22 named queues (see Queues table)
- **Beat scheduler**: 31 entries in `CELERYBEAT_SCHEDULE`; processes run in a dedicated `celery beat` pod
- **Broker**: AWS SQS (`sqs://`), prefixed with `NOTIFICATION_QUEUE_PREFIX`, visibility timeout 310 s, polling interval 1 s
- **Tracing**: AWS X-Ray (default) or OpenTelemetry (when `FF_ENABLE_OTEL=true`); custom `NotifyTask` base class wraps every task in a Flask app context and records wall-clock time
- **Error classification**: `app/celery/error_registry.py` walks the exception chain on every retry/failure and emits a structured `CELERY_KNOWN_ERROR::*` or `CELERY_UNKNOWN_ERROR` log token for CloudWatch metric filters

### Worker process breakdown

| Script | Queues consumed | Typical concurrency |
|--------|----------------|---------------------|
| `scripts/run_celery_beat.sh` | Beat scheduler only (no worker) | 1 |
| `scripts/run_celery.sh` | All except `send-throttled-sms-tasks` and `send-email-*` | `CELERY_CONCURRENCY` (default 4) |
| `scripts/run_celery_no_sms_sending.sh` | All except `send-throttled-sms-tasks`, `send-sms-*` | `CELERY_CONCURRENCY` (default 4) |
| `scripts/run_celery_core_tasks.sh` | All except `send-throttled-sms-tasks`, `send-sms-*`, `send-email-*` | `CELERY_CONCURRENCY` (default 4) |
| `scripts/run_celery_send_sms.sh` | `send-sms-high`, `send-sms-medium`, `send-sms-low` | `CELERY_CONCURRENCY` (default 4) |
| `scripts/run_celery_sms.sh` | `send-throttled-sms-tasks` | **1** (rate-limited dedicated pod) |
| `scripts/run_celery_send_email.sh` | `send-email-high`, `send-email-medium`, `send-email-low` | `CELERY_CONCURRENCY` (default 4) |
| `scripts/run_celery_delivery.sh` / `run_celery_receipts.sh` | `delivery-receipts` | `CELERY_CONCURRENCY` (default 4) |
| `scripts/run_celery_report_tasks.sh` | `generate-reports` | `CELERY_CONCURRENCY` (default 4) |
| `scripts/run_celery_local.sh` | ALL queues + embedded beat | `CELERY_CONCURRENCY` (default 4) |

### Beat / scheduled task summary

31 beat entries are registered (some map to the same task with different schedules for quarterly runs):
- **Every 10 s**: 6 batch-inbox drain tasks (3 SMS + 3 email priorities)
- **Every 1 min** (`crontab()`): `run-scheduled-jobs`, `mark-jobs-complete`, `check-job-status`
- **Every 15 min**: `replay-created-notifications`
- **Every 60 s** (interval): `in-flight-to-inbox`
- **Every 63 min** (interval): `delete-verify-codes`
- **Every 66 min** (interval): `delete-invitations`
- **Nightly** (UTC, ~EST midnight to 5 AM window): 10 maintenance/reporting jobs
- **Quarterly** (4 × 2 = 8 entries): annual-limit data insertion + quarterly usage email

---

## Queues

| Queue name | Purpose | Primary worker script |
|-----------|---------|----------------------|
| `periodic-tasks` | Beat-dispatched maintenance tasks | `run_celery.sh`, `run_celery_core_tasks.sh` |
| `priority-tasks` | High-priority single notifications | `run_celery.sh` |
| `normal-tasks` | Normal-priority single notifications | `run_celery.sh` |
| `bulk-tasks` | Bulk/low-priority single notifications | `run_celery.sh` |
| `-priority-database-tasks.fifo` | DB persistence for priority notifications (SQS FIFO) | `run_celery.sh` |
| `-normal-database-tasks` | DB persistence for normal notifications | `run_celery.sh` |
| `-bulk-database-tasks` | DB persistence for bulk notifications | `run_celery.sh` |
| `job-tasks` | CSV job processing and resumption | `run_celery.sh` |
| `notify-internal-tasks` | Internal Notify service notifications (e.g. no-reply) | `run_celery.sh` |
| `send-sms-high` | SMS delivery (priority/high) via SNS/Pinpoint | `run_celery_send_sms.sh` |
| `send-sms-medium` | SMS delivery (normal) via SNS/Pinpoint | `run_celery_send_sms.sh` |
| `send-sms-low` | SMS delivery (bulk) via SNS/Pinpoint | `run_celery_send_sms.sh` |
| `send-throttled-sms-tasks` | Throttled long-dedicated-number SMS (≤30/min) | `run_celery_sms.sh` |
| `send-email-high` | Email delivery (priority) via SES | `run_celery_send_email.sh` |
| `send-email-medium` | Email delivery (normal) via SES | `run_celery_send_email.sh` |
| `send-email-low` | Email delivery (bulk) via SES | `run_celery_send_email.sh` |
| `research-mode-tasks` | Simulated delivery for research/trial services | `run_celery.sh` |
| `reporting-tasks` | Nightly billing and notification-status aggregation | `run_celery.sh` |
| `generate-reports` | On-demand user-requested CSV reports | `run_celery_report_tasks.sh` |
| `service-callbacks` | Outbound delivery-status/complaint webhooks to services | `run_celery.sh` |
| `service-callbacks-retry` | Retry queue for failed service callbacks | `run_celery.sh` |
| `delivery-receipts` | Inbound SNS/SES/Pinpoint receipt processing | `run_celery_delivery.sh` |
| `retry-tasks` | General retry queue for transient failures | `run_celery.sh` |
| `notifiy-cache-tasks` | Batch-saving Redis cache operations (defined but not in `all_queues()`) | — |

---

## Tasks

### process-job

- **Module**: `app/celery/tasks.py`
- **Queue**: `job-tasks` (dispatched there by `run-scheduled-jobs` and `check-job-status`)
- **Trigger**: Beat task `run-scheduled-jobs` calls `process_job.apply_async([job_id], queue=QueueNames.JOBS)` once per scheduled job; also re-queued by `process-incomplete-jobs`
- **Input payload**: `job_id` (str UUID)
- **Side effects**:
  - Reads CSV from S3 (`CSV_UPLOAD_BUCKET_NAME`) via `get_job_from_s3`
  - Writes `jobs` table: sets `job_status = in_progress`, `processing_started`
  - Updates `api_keys.last_used_at` once per job
  - Enqueues `save-smss` or `save-emails` tasks in batches of `BATCH_INSERTION_CHUNK_SIZE` (default 500)
  - Emits CloudWatch metric `put_batch_saving_bulk_created` per batch
  - Writes `jobs.job_status = cancelled` if service is inactive
  - Writes `jobs.job_status = sending_limits_exceeded` + `processing_finished` if daily/annual SMS or email limits would be breached
- **Output / return value**: None; chains into `save-smss` / `save-emails` tasks
- **Retry policy**: No `@celery.task(bind=True)` — no automatic retry; if the job is incomplete it will be re-detected by `check-job-status` and re-queued via `process-incomplete-jobs`
- **Notes**:
  - Guards against processing a job already past `JOB_STATUS_PENDING` (idempotent guard)
  - Routing logic (`choose_database_queue`, `choose_sending_queue`) selects queue based on template `process_type` (bulk/normal/priority) and CSV row count vs `CSV_BULK_REDIRECT_THRESHOLD`
  - Research-mode services always route rows to `research-mode-tasks`

---

### save-smss

- **Module**: `app/celery/tasks.py`
- **Queue**: `-priority-database-tasks.fifo`, `-normal-database-tasks`, or `-bulk-database-tasks` (chosen by `choose_database_queue`)
- **Trigger**:
  1. `process_job` → `save_smss.apply_async(...)` in bulk batches
  2. `beat-inbox-sms-*` beat tasks → `save_smss.apply_async(...)` from Redis inbox drain
  3. `handle_batch_error_and_forward` re-queues individual notifications on DB error
- **Input payload**: `service_id` (str UUID or None), `signed_notifications` (list of signed/encrypted notification blobs), `receipt` (UUID or None — present when draining Redis inbox)
- **Side effects**:
  - Verifies `itsdangerous` HMAC signature on each notification blob
  - Fetches service (with cache) and template (with cache) from DB
  - Inserts rows into `notifications` table via `persist_notifications`
  - If `receipt` is set: calls `acknowledge_receipt()` to remove inflight Redis key
  - Emits CloudWatch metric `put_batch_saving_bulk_processed` if no receipt path
  - Enqueues `deliver_sms` or `deliver_throttled_sms` via `send_notification_to_queue` for each persisted notification
  - Skips delivery enqueue and logs warning when `LiveServiceTooManySMSRequestsError` or `TrialServiceTooManySMSRequestsError`
- **Output / return value**: None; chains to `deliver_sms` / `deliver_throttled_sms`
- **Retry policy**: `bind=True`, `max_retries=5`, `default_retry_delay=300` s; `SQLAlchemyError` triggers `handle_batch_error_and_forward` which either re-queues individual notifications or retries the task (via `QueueNames.RETRY`); `BadSignature` raises immediately without retry
- **Notes**:
  - Partially idempotent: `handle_batch_error_and_forward` checks if notification already exists before re-forwarding
  - On batch failure it acknowledges the Redis receipt anyway (purges inflight) to avoid re-delivery

---

### save-emails

- **Module**: `app/celery/tasks.py`
- **Queue**: `-priority-database-tasks.fifo`, `-normal-database-tasks`, or `-bulk-database-tasks`
- **Trigger**: Same three paths as `save-smss` (job processing, Redis inbox drain, error retry)
- **Input payload**: `_service_id` (str UUID or None), `signed_notifications` (list), `receipt` (UUID or None)
- **Side effects**:
  - Same as `save-smss` but for email type
  - Resolves reply-to address: checks notification blob → `service_email_reply_to` lookup → template default → service default
  - Inserts rows into `notifications`; acknowledges Redis receipt or emits batch metric
  - Calls `try_to_send_notifications_to_queue` → enqueues `deliver_email` tasks
  - Skips delivery and logs when `LiveServiceTooManyRequestsError` / `TrialServiceTooManyRequestsError`
- **Output / return value**: None; chains to `deliver_email`
- **Retry policy**: `bind=True`, `max_retries=5`, `default_retry_delay=300` s; same error routing as `save-smss`
- **Notes**:
  - Known bug documented in code: `process_type` and `service` used in `acknowledge_receipt` / rate-limit checks are the last iteration values from the loop, not per-notification (multi-service batch edge case)

---

### deliver-sms

- **Module**: `app/celery/provider_tasks.py`
- **Queue**: `send-sms-high`, `send-sms-medium`, or `send-sms-low` (selected by `send_notification_to_queue`)
- **Trigger**: Chained from `save-smss` via `send_notification_to_queue`
- **Input payload**: `notification_id` (UUID str)
- **Side effects**:
  - Reads `notifications` row from DB
  - Calls `send_to_providers.send_sms_to_provider(notification)` → AWS SNS `publish` or AWS Pinpoint SMS V2 `send_text_message`
  - On `PinpointConflictException` / `PinpointValidationException`: updates `notifications.status = provider_failure`, sets `feedback_reason`, enqueues delivery-status callback
  - On `MaxRetriesExceededError`: updates `notifications.status = technical_failure`, enqueues callback
- **Output / return value**: None
- **Retry policy**: `bind=True`, `max_retries=48`, `default_retry_delay` is **process-type–aware** via `CeleryParams.retry()`:
  - PRIORITY: 25 s countdown
  - NORMAL / BULK: 300 s countdown
  - Retries via `QueueNames.RETRY`
- **Notes**:
  - Rate-limited: `CELERY_DELIVER_SMS_RATE_LIMIT` (env var, default `"1/s"`) — in production 3 primary pods + up to 20 scalable pods → up to ~92 SMS/s aggregate
  - External calls: AWS SNS or AWS Pinpoint SMS V2

---

### deliver-throttled-sms

- **Module**: `app/celery/provider_tasks.py`
- **Queue**: `send-throttled-sms-tasks`
- **Trigger**: Chained from `send_notification_to_queue` for dedicated long numbers with `FF_USE_PINPOINT_FOR_DEDICATED=true`; only 1 worker with concurrency=1
- **Input payload**: `notification_id` (UUID str)
- **Side effects**: Identical to `deliver-sms` — calls `_deliver_sms` helper
- **Output / return value**: None
- **Retry policy**: `bind=True`, `max_retries=48`, `default_retry_delay=300` s; same process-type countdown logic
- **Notes**:
  - Celery `rate_limit="30/m"` → effectively ≤1 task per 2 s
  - Single consumer pod with concurrency=1 ensures the rate limit is a true global cap
  - External calls: AWS Pinpoint SMS V2 (dedicated long numbers from `us-west-2`)

---

### deliver-email

- **Module**: `app/celery/provider_tasks.py`
- **Queue**: `send-email-high`, `send-email-medium`, or `send-email-low`
- **Trigger**: Chained from `save-emails` via `send_notification_to_queue`
- **Input payload**: `notification_id` (UUID str)
- **Side effects**:
  - Reads `notifications` row
  - Calls `send_to_providers.send_email_to_provider(notification)` → AWS SES `send_raw_message` or `send_email`
  - On `InvalidEmailError`: updates `notifications.status = technical_failure`, enqueues callback
  - On `InvalidUrlException`: same as above
  - On `MalwareDetectedException`: enqueues callback (no status update here; status set upstream)
  - On `MalwareScanInProgressException`: exponential backoff retry up to `SCAN_MAX_BACKOFF_RETRIES` (5), then falls back to default countdown; countdown = `SCAN_RETRY_BACKOFF` × (retries + 1) seconds
  - On `MaxRetriesExceededError`: updates `notifications.status = technical_failure`, enqueues callback
- **Output / return value**: None
- **Retry policy**: `bind=True`, `max_retries=48`, `default_retry_delay=300` s; retries via `QueueNames.RETRY` with process-type countdown
- **Notes**:
  - External calls: AWS SES (`us-east-1` by default, configurable via `AWS_SES_REGION`)

---

### send-inbound-sms

- **Module**: `app/celery/tasks.py`
- **Queue**: `QueueNames.RETRY` on retry; initial dispatch queue not specified (defaults to Celery default or defined by caller)
- **Trigger**: Enqueued by inbound SMS handler when an inbound SMS is received and the service has a registered inbound API endpoint
- **Input payload**: `inbound_sms_id` (UUID str), `service_id` (UUID str)
- **Side effects**:
  - Reads `inbound_sms` row and `service_inbound_api` row
  - Makes outbound HTTP `POST` to the service's registered inbound webhook URL with a signed bearer token
  - Payload: `{id, source_number, destination_number, message, date_received}`
  - Request timeout: 60 s
- **Output / return value**: None; returns early if no inbound API configured
- **Retry policy**: `bind=True`, `max_retries=5`, `default_retry_delay=300` s; retries only on non-4xx HTTP errors or connection errors
- **Notes**:
  - External call: arbitrary service-registered HTTPS endpoint

---

### process-incomplete-jobs

- **Module**: `app/celery/tasks.py`
- **Queue**: `job-tasks` (dispatched by `check-job-status`)
- **Trigger**: `check-job-status` beat task via `notify_celery.send_task(TaskNames.PROCESS_INCOMPLETE_JOBS, args=(job_ids,), queue=QueueNames.JOBS)`
- **Input payload**: `job_ids` (list of UUID str)
- **Side effects**:
  - Resets `jobs.processing_started` to now (prevents re-detection by `check-job-status`)
  - For each job: reads CSV from S3, determines last successfully added row, enqueues remaining rows via `process_rows` → `save-smss` / `save-emails`
  - Emits `put_batch_saving_bulk_created` metric per chunk
- **Output / return value**: None; raises `JobIncompleteError` after dispatching (caught by beat task error handler)
- **Retry policy**: None (no `bind=True`)
- **Notes**:
  - Idempotent: skips rows already persisted by checking `dao_get_last_notification_added_for_job_id`

---

### send-notify-no-reply

- **Module**: `app/celery/tasks.py`
- **Queue**: `notify-internal-tasks` (for delivery); retries via `QueueNames.RETRY`
- **Trigger**: Fed by AWS Lambda `ses_receiving_emails` via SQS when someone replies to a GC Notify–sent email
- **Input payload**: `data` (JSON string with keys `sender`, `recipients`)
- **Side effects**:
  - Fetches `NOTIFY_SERVICE_ID` service and `NO_REPLY_TEMPLATE_ID` template
  - Calls `persist_notifications` → inserts into `notifications`
  - Enqueues `deliver_email` via `send_notification_to_queue` on `notify-internal-tasks`
- **Output / return value**: None
- **Retry policy**: `bind=True`, `max_retries=5`; retries via `QueueNames.RETRY`
- **Notes**:
  - `reply_to_text` is explicitly set to `None` to avoid reply loops

---

### seed-bounce-rate-in-redis

- **Module**: `app/celery/tasks.py`
- **Queue**: Not in beat schedule; called programmatically (e.g., on first request for a service's bounce rate)
- **Trigger**: Application logic (likely service dashboard load or first bounce-rate check)
- **Input payload**: `service_id` (str UUID), `interval` (int, default 24 hours)
- **Side effects**:
  - Checks `bounce_rate_client.get_seeding_started(service_id)` Redis key; no-ops if already seeded
  - Sets seeding-in-progress flag in Redis
  - Queries `notifications` table: aggregated total notifications and hard bounces grouped by hour over `interval` hours
  - Writes time-series data into Redis sorted sets for total notifications and hard bounces
- **Output / return value**: None
- **Retry policy**: None (no `bind=True`)
- **Notes**:
  - Idempotent: guarded by `seeding_started` flag in Redis

---

### generate-report

- **Module**: `app/celery/tasks.py`
- **Queue**: `generate-reports`
- **Trigger**: HTTP `POST /service/{service_id}/report` creates a `reports` DB row with status `pending`, then enqueues this task
- **Input payload**: `report_id` (str UUID), `notification_statuses` (list of str, default `[]`)
- **Side effects**:
  - Reads `reports` row from DB
  - Sets `reports.status = generating`
  - Streams CSV directly to S3 (`REPORTS_BUCKET_NAME`) at key `reports/{service_id}/{report_id}.csv` via PostgreSQL `COPY TO` (streaming)
  - Generates a presigned S3 URL (expiry = `DAYS_BEFORE_REPORTS_EXPIRE` × 24 h = 3 days)
  - Updates `reports`: `url`, `generated_at`, `expires_at`, `status = ready`
  - Sends email notification to requesting user via `send_requested_report_ready`
  - On exception: sets `reports.status = error`
- **Output / return value**: None
- **Retry policy**: None (no `bind=True`); exceptions are caught, status set to error, then re-raised
- **Notes**:
  - External calls: S3 (`REPORTS_BUCKET_NAME`), AWS SES (via `send_requested_report_ready`)
  - Filtered by `notification_statuses` and up to `LIMIT_DAYS = 7` days of data

---

### run-scheduled-jobs

- **Module**: `app/celery/scheduled_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — every minute (`crontab()`)
- **Input payload**: None
- **Side effects**:
  - Calls `dao_set_scheduled_jobs_to_pending()` — finds jobs with `scheduled_for <= now`, updates `job_status = pending`
  - Enqueues `process-job` on `job-tasks` for each
- **Output / return value**: None
- **Retry policy**: None; `SQLAlchemyError` logged and re-raised
- **Notes**: No deduplication; concurrent beats are prevented by Celery beat's single-process design

---

### mark-jobs-complete

- **Module**: `app/celery/scheduled_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — every minute (`crontab()`)
- **Input payload**: None
- **Side effects**:
  - Queries in-progress/error jobs
  - Compares `notification_count` vs persisted notifications; calls `job_complete()` → sets `job_status = finished`, `processing_finished`
- **Output / return value**: None
- **Retry policy**: None

---

### send-scheduled-notifications

- **Module**: `app/celery/scheduled_tasks.py`
- **Queue**: `periodic-tasks` (defined but **not currently in `CELERYBEAT_SCHEDULE`**)
- **Trigger**: Was formerly a beat task; now only invocable manually
- **Input payload**: None
- **Side effects**:
  - Fetches notifications with `scheduled_for <= now` from `scheduled_notifications` table
  - Calls `send_notification_to_queue` for each → enqueues `deliver_sms` or `deliver_email`
  - Sets `scheduled_notification.status = processed`
- **Output / return value**: None
- **Notes**: Not registered in the current beat schedule; may be legacy

---

### delete-verify-codes

- **Module**: `app/celery/scheduled_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — every 63 minutes (`timedelta(minutes=63)`)
- **Input payload**: None
- **Side effects**: Deletes `verify_codes` rows older than 24 h
- **Retry policy**: None

---

### delete-invitations

- **Module**: `app/celery/scheduled_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — every 66 minutes (`timedelta(minutes=66)`)
- **Input payload**: None
- **Side effects**: Deletes `invited_users` and `invited_organisation_users` rows older than 2 days
- **Retry policy**: None

---

### switch-current-sms-provider-on-slow-delivery

- **Module**: `app/celery/scheduled_tasks.py`
- **Queue**: `periodic-tasks` (defined but **not currently in `CELERYBEAT_SCHEDULE`**)
- **Trigger**: Formerly beat; now only invocable manually
- **Input payload**: None
- **Side effects**:
  - Checks if current SMS provider was switched in the last 10 minutes; returns early if so
  - Queries for slow-delivery notifications (>30% took >4 min in last 10 min)
  - Calls `dao_toggle_sms_provider` → updates `provider_details` table
- **Retry policy**: None
- **Notes**: Not registered in the current beat schedule; may be legacy

---

### check-job-status

- **Module**: `app/celery/scheduled_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — every minute (`crontab()`)
- **Input payload**: None
- **Side effects**:
  - Calls `update_in_progress_jobs()` to refresh `jobs.updated_at` from latest sent notification
  - Finds in-progress jobs with `updated_at` between 30–35 minutes ago
  - Marks those jobs `job_status = error`
  - Enqueues `process-incomplete-jobs` on `job-tasks`
- **Output / return value**: Raises `JobIncompleteError` if any stalled jobs found (for monitoring)
- **Retry policy**: None

---

### replay-created-notifications

- **Module**: `app/celery/scheduled_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — every 15 minutes (`crontab(minute="0, 15, 30, 45")`)
- **Input payload**: None
- **Side effects**:
  - Finds notifications still in `created` status older than 4 h 15 min
  - Re-enqueues each via `send_notification_to_queue`
- **Retry policy**: None
- **Notes**: Safety net for notifications that slipped through without being dispatched

---

### check-precompiled-letter-state

- **Module**: `app/celery/scheduled_tasks.py`
- **Queue**: `periodic-tasks` (**not in current beat schedule**)
- **Trigger**: Formerly beat; now only invocable manually
- **Input payload**: None
- **Side effects**:
  - Finds letters in `pending-virus-check` for >90 minutes
  - Creates a Zendesk incident ticket in live/production environments
- **Retry policy**: None

---

### check-templated-letter-state

- **Module**: `app/celery/scheduled_tasks.py`
- **Queue**: `periodic-tasks` (**not in current beat schedule**)
- **Trigger**: Formerly beat; now only invocable manually
- **Input payload**: None
- **Side effects**:
  - Finds letters with `created` status created before 17:30 yesterday
  - Creates a Zendesk incident ticket in live/production
- **Retry policy**: None

---

### in-flight-to-inbox (recover_expired_notifications)

- **Module**: `app/celery/scheduled_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — every 60 seconds (interval)
- **Input payload**: None
- **Side effects**:
  - Calls `expire_inflights()` on all 6 Redis queue objects (`sms_priority`, `sms_normal`, `sms_bulk`, `email_priority`, `email_normal`, `email_bulk`)
  - Moves any in-flight notifications whose visibility TTL has expired back into the inbox
- **Retry policy**: None
- **Notes**: Provides at-least-once delivery guarantee for the Redis batch-save queues

---

### beat-inbox-sms-normal

- **Module**: `app/celery/scheduled_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — every 10 seconds
- **Input payload**: None
- **Side effects**: Polls `sms_normal` Redis inbox; for each non-empty batch dispatches `save-smss.apply_async((None, batch, receipt_id), queue=NORMAL_DATABASE)`; loop continues until inbox is empty
- **Retry policy**: None

---

### beat-inbox-sms-bulk

- **Module**: `app/celery/scheduled_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — every 10 seconds
- **Side effects**: Same as `beat-inbox-sms-normal` but for `sms_bulk` → `BULK_DATABASE`

---

### beat-inbox-sms-priority

- **Module**: `app/celery/scheduled_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — every 10 seconds
- **Side effects**: Same pattern for `sms_priority` → `PRIORITY_DATABASE`

---

### beat-inbox-email-normal

- **Module**: `app/celery/scheduled_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — every 10 seconds
- **Side effects**: Polls `email_normal` Redis inbox; dispatches `save-emails.apply_async` to `NORMAL_DATABASE`

---

### beat-inbox-email-bulk

- **Module**: `app/celery/scheduled_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — every 10 seconds
- **Side effects**: `email_bulk` → `BULK_DATABASE`

---

### beat-inbox-email-priority

- **Module**: `app/celery/scheduled_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — every 10 seconds
- **Side effects**: `email_priority` → `PRIORITY_DATABASE`

---

### process-ses-result

- **Module**: `app/celery/process_ses_receipts_tasks.py`
- **Queue**: `delivery-receipts` (initial); retries via `retry-tasks`
- **Trigger**: AWS Lambda (or SQS/SNS) pushes SES delivery/bounce/complaint events into `delivery-receipts` queue
- **Input payload**: `response` (dict with `Messages` list or single `Message` key containing JSON-encoded `SESReceipt` objects)
- **Side effects**:
  - Parses batch of SES receipts; separates complaints from delivery/bounce events
  - For complaints: looks up notification in `notifications` table; falls back to `notification_history` if not found (handles old notifications past data-retention)
  - Calls `handle_complaint()` → inserts/updates `complaints` table; enqueues `send-complaint` callback
  - For delivery/bounce: bulk-updates `notifications.status` via `_update_notification_statuses`
  - Updates Redis annual-limit counters (`annual_limit_client.increment_email_delivered` / `increment_email_failed`) if not already seeded
  - Updates Redis bounce-rate sliding window (`bounce_rate_client.set_sliding_hard_bounce`) on permanent failure
  - Emits StatsD metric `callback.ses.<status>`
  - Enqueues `send-delivery-status` callback for each notification
- **Output / return value**: `True` on success; `None` if retried
- **Retry policy**: `bind=True`, `max_retries=5`, `default_retry_delay=300` s; receipts with no matching notification in DB are retried via `QueueNames.RETRY`; entire batch retried on unexpected exception
- **Notes**:
  - Idempotent guard: skips status downgrade from permanent failure → delivered
  - Duplicate-update guard: `_duplicate_update_warning` logs but does not error

---

### process-sns-result

- **Module**: `app/celery/process_sns_receipts_tasks.py`
- **Queue**: `delivery-receipts`; retries via `retry-tasks`
- **Trigger**: SNS delivery receipt pushed to `delivery-receipts` queue
- **Input payload**: `response` (dict with `Message` key containing JSON-encoded SNS delivery receipt)
- **Side effects**:
  - Parses SNS delivery receipt; extracts `messageId`, `status`, `providerResponse`
  - Calls `_update_notification_status` → updates `notifications.status`, `provider_response`
  - Updates Redis annual-limit counters for SMS
  - Emits StatsD metric `callback.sns.<status>`
  - Enqueues `send-delivery-status` callback
- **Output / return value**: `True` on success
- **Retry policy**: `bind=True`, `max_retries=5`, `default_retry_delay=300` s; notification-not-found → retry via `QueueNames.RETRY`; unexpected → retry
- **Notes**:
  - Only processes if `notification.sent_by == SNS_PROVIDER`; logs exception otherwise
  - Duplicate guard: checks `notification.status != NOTIFICATION_SENT` before updating
  - Status mapping via `determine_status()`: maps SNS provider response strings to Notify statuses

---

### process-pinpoint-result

- **Module**: `app/celery/process_pinpoint_receipts_tasks.py`
- **Queue**: `delivery-receipts`; retries via `retry-tasks`
- **Trigger**: AWS Pinpoint SMS event pushed to `delivery-receipts` queue
- **Input payload**: `response` (dict with `Message` key containing JSON-encoded Pinpoint SMS event)
- **Side effects**:
  - Parses Pinpoint receipt: `messageId`, `messageStatus`, `messageStatusDescription`, `isFinal`, pricing and carrier fields
  - Updates `notifications` with status, `provider_response`, and optional fields: `sms_total_message_price`, `sms_total_carrier_fee`, `sms_iso_country_code`, `sms_carrier_name`, `sms_message_encoding`, `sms_origination_phone_number`
  - Updates Redis annual-limit counters (supports optional `FF_USE_BILLABLE_UNITS` flag to count billable units instead of messages)
  - Emits StatsD metric `callback.pinpoint.<status>`
  - Enqueues `send-delivery-status` callback
- **Output / return value**: None on success
- **Retry policy**: `bind=True`, `max_retries=5`, `default_retry_delay=300` s
- **Notes**:
  - Skips update if `isFinal=false` and status is `SUCCESSFUL` (returns early)
  - Status mapping via `determine_pinpoint_status()`: multi-condition logic on status + provider response string

---

### send-delivery-status

- **Module**: `app/celery/service_callback_tasks.py`
- **Queue**: `service-callbacks`; retries via `service-callbacks-retry`
- **Trigger**: Chained from `process-ses-result`, `process-sns-result`, `process-pinpoint-result`, and `timeout-sending-notifications` via `_check_and_queue_callback_task`
- **Input payload**: `notification_id` (UUID str), `signed_status_update` (signed/encrypted blob), `service_id` (UUID str)
- **Side effects**:
  - Verifies `itsdangerous` signature on `signed_status_update`
  - Makes outbound HTTP `POST` to service's registered callback URL with Bearer token
  - JSON body: `{id, reference, to, status, status_description, provider_response, created_at, completed_at, sent_at, notification_type}`
  - Request timeout: 5 s
- **Output / return value**: None
- **Retry policy**: `bind=True`, `max_retries=5`, `default_retry_delay=300` s; retries on connection errors, 5xx responses, and 429; does NOT retry on 4xx (except 429)
- **Notes**:
  - External call: arbitrary service-registered HTTPS endpoint

---

### send-complaint

- **Module**: `app/celery/service_callback_tasks.py`
- **Queue**: `service-callbacks`; retries via `service-callbacks-retry`
- **Trigger**: Chained from `process-ses-result` via `_check_and_queue_complaint_callback_task`
- **Input payload**: `complaint_data` (signed blob), `service_id` (UUID str)
- **Side effects**:
  - Verifies signature on `complaint_data`
  - Makes outbound HTTP `POST` to service's callback URL
  - JSON body: `{notification_id, complaint_id, reference, to, complaint_date}`
  - Request timeout: 5 s
- **Output / return value**: None
- **Retry policy**: Same as `send-delivery-status`
- **Notes**: External call to service-registered HTTPS endpoint

---

### timeout-sending-notifications

- **Module**: `app/celery/nightly_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — daily at UTC 05:05 (00:05 EST)
- **Input payload**: None
- **Side effects**:
  - Calls `dao_timeout_notifications(SENDING_NOTIFICATIONS_TIMEOUT_PERIOD)` (default 259,200 s = 3 days)
  - Updates `notifications.status` to `technical_failure` or `temporary_failure`
  - For each timed-out notification that has a service callback configured: enqueues `send-delivery-status`
- **Output / return value**: raises `NotificationTechnicalFailureException` if any technical failures occurred (for monitoring)
- **Retry policy**: None

---

### delete-sms-notifications-older-than-retention (delete-sms-notifications)

- **Module**: `app/celery/nightly_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — daily at UTC 09:15 (04:15 EST)
- **Input payload**: None
- **Side effects**: Deletes `notifications` rows of type `sms` older than service-configured retention (falls back to default); writes info log with count
- **Retry policy**: None

---

### delete-email-notifications-older-than-retention (delete-email-notifications)

- **Module**: `app/celery/nightly_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — daily at UTC 09:30 (04:30 EST)
- **Side effects**: Same as above for `email` type
- **Retry policy**: None

---

### delete-letter-notifications-older-than-retention (delete-letter-notifications)

- **Module**: `app/celery/nightly_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — daily at UTC 09:45 (04:45 EST)
- **Side effects**: Same as above for `letter` type
- **Retry policy**: None

---

### delete-inbound-sms

- **Module**: `app/celery/nightly_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — daily at UTC 06:40 (01:40 EST)
- **Input payload**: None
- **Side effects**: Deletes `inbound_sms` rows older than configured retention
- **Retry policy**: None

---

### send-daily-performance-platform-stats

- **Module**: `app/celery/nightly_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — daily at UTC 07:00 (02:00 EST)
- **Input payload**: `date` (optional str `"YYYY-MM-DD"`, defaults to yesterday)
- **Side effects**:
  - If `PERFORMANCE_PLATFORM_ENABLED`: sends total SMS, email, letter counts to external Performance Platform API
  - Calls `processing_time.send_processing_time_to_performance_platform`
- **Retry policy**: None
- **Notes**: External call to `PERFORMANCE_PLATFORM_URL` (UK government platform, rarely active)

---

### remove_transformed_dvla_files

- **Module**: `app/celery/nightly_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — daily at UTC 08:40 (03:40 EST)
- **Input payload**: None
- **Side effects**:
  - Fetches letter jobs older than data retention
  - Removes transformed DVLA files from S3 for each job
- **Retry policy**: None

---

### remove_sms_email_jobs

- **Module**: `app/celery/nightly_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — daily at UTC 09:00 (04:00 EST)
- **Input payload**: None
- **Side effects**:
  - In a loop (batches of 100): fetches email and SMS jobs older than data retention
  - Removes CSV files from S3 (`CSV_UPLOAD_BUCKET_NAME`)
  - Archives jobs in DB (moves to `jobs_history` or sets archived flag)
- **Retry policy**: None
- **Notes**: External call: S3

---

### remove_letter_jobs

- **Module**: `app/celery/nightly_tasks.py`
- **Queue**: `periodic-tasks` (**not in current beat schedule**)
- **Trigger**: Formerly beat; now only invocable manually
- **Input payload**: None
- **Side effects**: Same as `remove_sms_email_jobs` but for `letter` type
- **Notes**: Appears to be superseded by `remove_sms_email_jobs` batch handling or not yet re-registered

---

### delete_dvla_response_files (delete_dvla_response_files_older_than_seven_days)

- **Module**: `app/celery/nightly_tasks.py`
- **Queue**: `periodic-tasks` (**explicitly commented: "TODO: remove me, i'm not being run by anything"**)
- **Trigger**: None (dead code)
- **Side effects**: Would delete DVLA response files from S3 older than 7 days
- **Notes**: Legacy; safe to delete in Go rewrite

---

### raise-alert-if-letter-notifications-still-sending

- **Module**: `app/celery/nightly_tasks.py`
- **Queue**: `periodic-tasks` (**not in current beat schedule**)
- **Trigger**: Formerly beat
- **Input payload**: None
- **Side effects**:
  - Counts letters with `sending` status older than 2–4 business days
  - Skips weekends
  - Creates Zendesk incident ticket in production environments
- **Notes**: Not currently scheduled; letter feature may be dormant in Canadian deployment

---

### raise-alert-if-no-letter-ack-file (letter_raise_alert_if_no_ack_file_for_zip)

- **Module**: `app/celery/nightly_tasks.py`
- **Queue**: `periodic-tasks` (**not in current beat schedule**)
- **Trigger**: Formerly beat
- **Side effects**:
  - Lists `.TXT` ZIP manifest files in S3 `LETTERS_PDF_BUCKET_NAME` for today
  - Lists `.ACK.txt` acknowledgement files in S3 `DVLA_RESPONSE_BUCKET_NAME` from yesterday
  - Diffs the two sets; creates Zendesk ticket if mismatches
- **Notes**: External call: S3; not currently scheduled

---

### create-nightly-billing

- **Module**: `app/celery/reporting_tasks.py`
- **Queue**: `reporting-tasks`
- **Trigger**: Beat — daily at UTC 05:15 (00:15 EST)
- **Input payload**: `day_start` (optional str `"YYYY-MM-DD"`)
- **Side effects**: Fans out to `create-nightly-billing-for-day` for today and the preceding 3 days (4 days total) via `apply_async` on `reporting-tasks`
- **Retry policy**: None

---

### create-nightly-billing-for-day

- **Module**: `app/celery/reporting_tasks.py`
- **Queue**: `reporting-tasks` (dispatched by `create-nightly-billing`)
- **Trigger**: Chained from `create-nightly-billing`
- **Input payload**: `process_day` (str `"YYYY-MM-DD"`)
- **Side effects**:
  - Calls `fetch_billing_data_for_day(process_day)` — aggregates from `notifications`
  - Calls `update_fact_billing(data, process_day)` — upserts rows in `fact_billing` table
- **Retry policy**: None

---

### create-nightly-notification-status

- **Module**: `app/celery/reporting_tasks.py`
- **Queue**: `reporting-tasks`
- **Trigger**: Beat — daily at UTC 05:30 (00:30 EST)
- **Input payload**: `day_start` (optional str `"YYYY-MM-DD"`)
- **Side effects**: Fans out to `create-nightly-notification-status-for-day` for today and preceding 3 days
- **Retry policy**: None

---

### create-nightly-notification-status-for-day

- **Module**: `app/celery/reporting_tasks.py`
- **Queue**: `reporting-tasks`
- **Trigger**: Chained from `create-nightly-notification-status`
- **Input payload**: `process_day` (str `"YYYY-MM-DD"`)
- **Side effects**:
  - Fetches all service IDs
  - Processes in chunks of 10 service IDs
  - Calls `fetch_notification_status_for_day` → `update_fact_notification_status` → upserts `fact_notification_status` table
  - Calls `annual_limit_client.reset_all_notification_counts(chunk)` → resets Redis counters for the chunk after the DB reflects truth
- **Retry policy**: None per chunk (individual chunk failures logged and continue)

---

### create-monthly-notification-stats-summary

- **Module**: `app/celery/reporting_tasks.py`
- **Queue**: `reporting-tasks`
- **Trigger**: Beat — daily at UTC 06:30 (01:30 EST), after `create-nightly-notification-status`
- **Input payload**: None
- **Side effects**:
  - PostgreSQL `INSERT ... ON CONFLICT DO UPDATE` upsert for the current and previous calendar months into `monthly_notification_stats_summary`
  - Aggregates from `fact_notification_status` filtered to `delivered`/`sent`, non-test keys; covers last 2 months only
- **Retry policy**: None; DB transaction rolled back on exception

---

### insert-quarter-data-for-annual-limits

- **Module**: `app/celery/reporting_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — 4 quarterly schedules: Jul 1, Oct 1, Jan 1, Apr 1 at UTC 23:00 (start of new quarter in Canada)
- **Input payload**: `process_day` (optional datetime, defaults to `datetime.now()`)
- **Side effects**:
  - Fetches all services' annual limits
  - Determines previous quarter date range via `get_previous_quarter(process_day)`
  - Processes in chunks of 20 service IDs
  - Calls `fetch_quarter_data` → `insert_quarter_data` → inserts into `annual_limits_data` table
- **Retry policy**: None per chunk

---

### send-quarterly-email

- **Module**: `app/celery/reporting_tasks.py`
- **Queue**: `periodic-tasks`
- **Trigger**: Beat — 3 quarterly schedules (Jul 2, Oct 2, Jan 3 at UTC 23:00); Q4 (Apr) entry absent from beat schedule
- **Input payload**: `process_date` (optional datetime)
- **Side effects**:
  - Fetches all services with their annual limits
  - Determines all historical quarters
  - For each user (chunks of 50), fetches cumulative quarterly usage from `annual_limits_data`
  - Calls `send_annual_usage_data(user_id, ...)` → sends bilingual (EN/FR) email via `ANNUAL_LIMIT_QUARTERLY_USAGE_TEMPLATE_ID`
- **Retry policy**: None per chunk; individual email failures logged, loop continues

---

### create-letters-pdf (stub)

- **Module**: `app/celery/letters_pdf_tasks.py`
- **Queue**: `create-letters-pdf-tasks`
- **Trigger**: Not active in current deployment
- **Input payload**: `notification_id`
- **Side effects**: `pass` (empty stub)
- **Retry policy**: `bind=True`, `max_retries=15`, `default_retry_delay=300` (declared but not used)
- **Notes**: Letter PDF generation is not deployed; entire `letters_pdf_tasks.py` is a stub

---

### collate-letter-pdfs-for-day (stub)

- **Module**: `app/celery/letters_pdf_tasks.py`
- **Queue**: `create-letters-pdf-tasks`
- **Trigger**: `@cronitor("collate-letter-pdfs-for-day")` decorator present but not in beat schedule
- **Side effects**: `pass` (stub)

---

### process-virus-scan-passed / process-virus-scan-failed / process-virus-scan-error (stubs)

- **Module**: `app/celery/letters_pdf_tasks.py`
- **Side effects**: `pass` (stubs)
- **Notes**: All three are empty stubs; virus scanning pipeline not active

---

### create-fake-letter-response-file

- **Module**: `app/celery/research_mode_tasks.py`
- **Queue**: `research-mode-tasks`
- **Trigger**: Called by research-mode SMS/email response helpers during simulated delivery
- **Input payload**: `reference` (str)
- **Side effects**:
  - Generates DVLA-format response string `"{reference}|Sent|0|Sorted"`
  - Finds an unused filename by trying random timestamps in the last 30 seconds
  - Uploads to S3 `DVLA_RESPONSE_BUCKET_NAME`
  - In `development` environment only: directly calls `process_sns_results.apply_async` via `QueueNames.RESEARCH_MODE`
- **Output / return value**: None
- **Retry policy**: `bind=True`, `max_retries=5`, `default_retry_delay=300` s
- **Notes**:
  - External call: S3 (`DVLA_RESPONSE_BUCKET_NAME`)
  - Used only in research/development to simulate DVLA letter delivery receipt

---

## Scheduled Tasks (Beat)

All beat tasks are dispatched using Celery beat's single-process scheduler. The beat process itself is `scripts/run_celery_beat.sh`.

| Beat entry name | Task function | Module | Schedule (UTC) | Queue | Purpose |
|-----------------|--------------|--------|---------------|-------|---------|
| `beat-inbox-sms-normal` | `beat_inbox_sms_normal` | scheduled_tasks | every 10 s | periodic-tasks | Drain normal-priority SMS Redis inbox → DB |
| `beat-inbox-sms-bulk` | `beat_inbox_sms_bulk` | scheduled_tasks | every 10 s | periodic-tasks | Drain bulk SMS Redis inbox → DB |
| `beat-inbox-sms-priority` | `beat_inbox_sms_priority` | scheduled_tasks | every 10 s | periodic-tasks | Drain priority SMS Redis inbox → DB |
| `beat-inbox-email-normal` | `beat_inbox_email_normal` | scheduled_tasks | every 10 s | periodic-tasks | Drain normal email Redis inbox → DB |
| `beat-inbox-email-bulk` | `beat_inbox_email_bulk` | scheduled_tasks | every 10 s | periodic-tasks | Drain bulk email Redis inbox → DB |
| `beat-inbox-email-priority` | `beat_inbox_email_priority` | scheduled_tasks | every 10 s | periodic-tasks | Drain priority email Redis inbox → DB |
| `in-flight-to-inbox` | `recover_expired_notifications` | scheduled_tasks | every 60 s | periodic-tasks | Recover stale Redis in-flight items |
| `run-scheduled-jobs` | `run_scheduled_jobs` | scheduled_tasks | every 1 min | periodic-tasks | Dispatch scheduled jobs to processing |
| `mark-jobs-complete` | `mark_jobs_complete` | scheduled_tasks | every 1 min | periodic-tasks | Close out finished jobs |
| `check-job-status` | `check_job_status` | scheduled_tasks | every 1 min | periodic-tasks | Detect and resume stalled jobs |
| `replay-created-notifications` | `replay_created_notifications` | scheduled_tasks | 0,15,30,45 min | periodic-tasks | Re-dispatch stuck notifications |
| `delete-verify-codes` | `delete_verify_codes` | scheduled_tasks | every 63 min | periodic-tasks | Purge expired 2FA codes |
| `delete-invitations` | `delete_invitations` | scheduled_tasks | every 66 min | periodic-tasks | Purge expired invitations |
| `timeout-sending-notifications` | `timeout_notifications` | nightly_tasks | 05:05 (00:05 EST) | periodic-tasks | Mark timed-out notifications as failed |
| `delete-inbound-sms` | `delete_inbound_sms` | nightly_tasks | 06:40 (01:40 EST) | periodic-tasks | Purge old inbound SMS |
| `create-monthly-notification-stats-summary` | `create_monthly_notification_stats_summary` | reporting_tasks | 06:30 (01:30 EST) | reporting-tasks | Refresh monthly stats table |
| `send-daily-performance-platform-stats` | `send_daily_performance_platform_stats` | nightly_tasks | 07:00 (02:00 EST) | periodic-tasks | Push stats to gov performance platform |
| `remove_transformed_dvla_files` | `remove_transformed_dvla_files` | nightly_tasks | 08:40 (03:40 EST) | periodic-tasks | Remove old DVLA files from S3 |
| `remove_sms_email_jobs` | `remove_sms_email_jobs` | nightly_tasks | 09:00 (04:00 EST) | periodic-tasks | Archive old SMS/email jobs, remove S3 CSVs |
| `delete-sms-notifications` | `delete_sms_notifications_older_than_retention` | nightly_tasks | 09:15 (04:15 EST) | periodic-tasks | Delete old SMS notifications |
| `delete-email-notifications` | `delete_email_notifications_older_than_retention` | nightly_tasks | 09:30 (04:30 EST) | periodic-tasks | Delete old email notifications |
| `delete-letter-notifications` | `delete_letter_notifications_older_than_retention` | nightly_tasks | 09:45 (04:45 EST) | periodic-tasks | Delete old letter notifications |
| `create-nightly-billing` | `create_nightly_billing` | reporting_tasks | 05:15 (00:15 EST) | reporting-tasks | Fan-out billing computation for 4 days |
| `create-nightly-notification-status` | `create_nightly_notification_status` | reporting_tasks | 05:30 (00:30 EST) | reporting-tasks | Fan-out notification status aggregation |
| `insert-quarter-data-for-annual-limits-q1` | `insert_quarter_data_for_annual_limits` | reporting_tasks | Jul 1 23:00 UTC | periodic-tasks | Insert Q1 (Apr–Jun) annual limit data |
| `insert-quarter-data-for-annual-limits-q2` | `insert_quarter_data_for_annual_limits` | reporting_tasks | Oct 1 23:00 UTC | periodic-tasks | Insert Q2 (Jul–Sep) data |
| `insert-quarter-data-for-annual-limits-q3` | `insert_quarter_data_for_annual_limits` | reporting_tasks | Jan 1 23:00 UTC | periodic-tasks | Insert Q3 (Oct–Dec) data |
| `insert-quarter-data-for-annual-limits-q4` | `insert_quarter_data_for_annual_limits` | reporting_tasks | Apr 1 23:00 UTC | periodic-tasks | Insert Q4 (Jan–Mar) data |
| `send-quarterly-email-q1` | `send_quarter_email` | reporting_tasks | Jul 2 23:00 UTC | periodic-tasks | Send quarterly usage email Q1 |
| `send-quarterly-email-q2` | `send_quarter_email` | reporting_tasks | Oct 2 23:00 UTC | periodic-tasks | Send quarterly usage email Q2 |
| `send-quarterly-email-q3` | `send_quarter_email` | reporting_tasks | Jan 3 23:00 UTC | periodic-tasks | Send quarterly usage email Q3 |

> **Note**: The Q4 quarterly email (April) is missing from `CELERYBEAT_SCHEDULE`; the corresponding `insert-quarter-data-for-annual-limits-q4` entry IS present, but the `send-quarterly-email-q4` entry is absent — this appears to be a bug.

---

## Go Goroutine Mapping

### Scheduled tasks → `time.Ticker` or cron library

**Pattern**: Use `github.com/robfin/cron/v3` (or `time.NewTicker` for sub-minute intervals) in a dedicated scheduler goroutine.

```
Sub-minute (10 s):  6 × ticker goroutines — one per inbox type (sms/email × bulk/normal/priority)
                    Each polls Redis INBOX, atomically moves batch to IN_FLIGHT, enqueues a DB-write job
1-minute tasks:     cron.AddFunc("* * * * *", runScheduledJobs)
                    cron.AddFunc("* * * * *", markJobsComplete)
                    cron.AddFunc("* * * * *", checkJobStatus)
15-minute tasks:    cron.AddFunc("0,15,30,45 * * * *", replayCreatedNotifications)
60-second interval: time.NewTicker(60 * time.Second) → recoverExpiredNotifications()
63-min interval:    cron.AddFunc("@every 63m", deleteVerifyCodes)
66-min interval:    cron.AddFunc("@every 66m", deleteInvitations)
Nightly:            Standard cron expressions matching UTC times above
Quarterly:          cron.AddFunc("0 23 1 7,10,1,4 *", insertQuarterData)
                    cron.AddFunc("0 23 2 7,10 *", sendQuarterlyEmail)
                    cron.AddFunc("0 23 3 1 *", sendQuarterlyEmail)
```

**In-flight recovery** for the Redis batch queues requires a goroutine that periodically scans all `in-flight:*` keys and moves expired ones back to `inbox`. This is the `in-flight-to-inbox` replacement.

### Queue-consumer tasks → goroutine pool reading from SQS

Each SQS queue type maps to a worker pool:

| SQS queue(s) | Go worker pool | Task handler |
|-------------|---------------|-------------|
| `send-sms-high`, `send-sms-medium`, `send-sms-low` | `smsWorkerPool` (N goroutines, N controlled by HPA equivalent) | `handleDeliverSMS()` → AWS SNS / Pinpoint SDK |
| `send-throttled-sms-tasks` | `throttledSMSWorkerPool` (1 goroutine) | Same handler, controlled by `rate.NewLimiter(0.5, 1)` (≤30/min) |
| `send-email-high`, `send-email-medium`, `send-email-low` | `emailWorkerPool` | `handleDeliverEmail()` → AWS SES SDK |
| `-priority-database-tasks.fifo`, `-normal-database-tasks`, `-bulk-database-tasks` | `dbSaveWorkerPool` | `handleSaveNotifications()` — insert batch + enqueue delivery |
| `job-tasks` | `jobWorkerPool` | `handleProcessJob()` — stream-read S3 CSV, bulk-insert rows |
| `delivery-receipts` | `receiptWorkerPool` | `handleSESReceipt()`, `handleSNSReceipt()`, `handlePinpointReceipt()` |
| `service-callbacks` | `callbackWorkerPool` | `handleDeliveryStatusCallback()`, `handleComplaintCallback()` |
| `service-callbacks-retry` | same pool | retry with exponential backoff |
| `retry-tasks` | shared with appropriate pool or dedicated `retryWorkerPool` | dispatch based on task name in message |
| `periodic-tasks` | `periodicWorkerPool` | maintenance handlers (delete, archive, report) |
| `reporting-tasks` | `reportingWorkerPool` | billing/stats aggregation |
| `generate-reports` | `reportWorkerPool` | stream-generate CSV to S3 |
| `notify-internal-tasks` | shared `emailWorkerPool` | internal no-reply email |
| `research-mode-tasks` | `researchWorkerPool` | simulated delivery callbacks |

**SQS consumption pattern** (per pool):
```go
for {
    msgs, _ := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
        QueueUrl:            &queueURL,
        MaxNumberOfMessages: 10,
        WaitTimeSeconds:     20,  // long polling
        VisibilityTimeout:   310,
    })
    for _, msg := range msgs.Messages {
        sem <- struct{}{}
        go func(m types.Message) {
            defer func() { <-sem }()
            if err := handler(ctx, m); err != nil {
                // exponential backoff via SQS visibility timeout extension
            } else {
                sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{...})
            }
        }(msg)
    }
}
```

### Chained tasks → goroutine pipelines with channels

**Notification send pipeline** (replaces `process-job` → `save-smss/save-emails` → `deliver-sms/deliver-email`):

```
S3 CSV stream ──► rowParser goroutine
                       │ channel [[]SignedRow]
                       ▼
               batchBuilder goroutine (chunks of BATCH_INSERTION_CHUNK_SIZE = 500)
                       │ channel [NotificationBatch]
                       ▼
               dbPersist goroutine (bulk INSERT into notifications)
                       │ channel [[]NotificationID]
                       ▼
               deliveryDispatch goroutine (enqueue to SQS send-sms-*/send-email-* by priority)
```

**Redis inbox drain pipeline** (replaces 6 `beat-inbox-*` tasks):

```
time.Ticker(10s) ──► inboxDrainer goroutine
                          │ polls Redis INBOX key
                          │ moves batch to IN_FLIGHT key
                          │ channel [InboxBatch]
                          ▼
                     dbSaveWorkerPool ─► deliveryDispatch
```

**SES/SNS/Pinpoint receipt pipeline** (replaces 3 `process-*-result` tasks + callback tasks):

```
SQS delivery-receipts ──► receiptParser goroutine
                               │
                     ┌─────────┴──────────┐
                     ▼                    ▼
             complaintsHandler     deliveryHandler
                     │                    │
              DB complaint update   bulk DB status update
                     │              annual limit Redis update
                     └──────┬───────┘
                             ▼
                    callbackDispatch ──► SQS service-callbacks
```

### Key retry/backoff replacements

| Celery mechanism | Go equivalent |
|-----------------|--------------|
| `max_retries=5, default_retry_delay=300` | SQS visibility timeout extension + dead-letter queue after 5 receives |
| Priority-aware countdown (25 s vs 300 s) | `sqs.ChangeMessageVisibility` with countdown based on notification process type |
| `QueueNames.RETRY` | Separate SQS retry queue; worker re-reads and re-attempts |
| `MaxRetriesExceededError` → status=technical_failure | DLQ consumer updates DB status + enqueues callback |
| `rate_limit="30/m"` on `deliver_throttled_sms` | `golang.org/x/time/rate.NewLimiter(rate.Every(2*time.Second), 1)` |
