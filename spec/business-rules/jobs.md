# Business Rules: Jobs

## Overview

A **job** represents a bulk notification send initiated via CSV file upload. The caller uploads a CSV to S3,
then POSTs job metadata to the API. The API validates the file, checks rate limits, creates the `jobs` row,
and (for non-scheduled jobs) immediately enqueues a Celery task that reads the CSV back from S3, splits
recipients into batches, and creates one `Notification` per row. Letter jobs have an additional
post-processing cancellation window.

---

## Data Model

Table: `jobs`

| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| `id` | UUID | No | `uuid4()` | Primary key |
| `original_file_name` | String | No | — | Name of uploaded CSV |
| `service_id` | UUID FK → `services.id` | No | — | Owning service |
| `template_id` | UUID FK → `templates.id` | No | — | Template used for all rows |
| `template_version` | Integer | No | — | Snapshot of template version at creation time |
| `created_at` | DateTime (UTC) | No | `utcnow()` | Indexed |
| `updated_at` | DateTime (UTC) | Yes | auto-update | |
| `notification_count` | Integer | No | — | Total recipient rows in the CSV |
| `notifications_sent` | Integer | No | `0` | Running counter |
| `notifications_delivered` | Integer | No | `0` | Running counter |
| `notifications_failed` | Integer | No | `0` | Running counter |
| `processing_started` | DateTime (UTC) | Yes | `null` | Set when Celery task begins; indexed |
| `processing_finished` | DateTime (UTC) | Yes | `null` | Set when Celery task completes |
| `created_by_id` | UUID FK → `users.id` | Yes | — | Null for API-key-initiated jobs |
| `api_key_id` | UUID FK → `api_keys.id` | Yes | — | Null for user-initiated jobs |
| `scheduled_for` | DateTime (UTC) | Yes | `null` | Future send time; indexed |
| `job_status` | String FK → `job_status.name` | No | `"pending"` | Indexed |
| `archived` | Boolean | No | `false` | Soft-delete flag |
| `sender_id` | UUID | Yes | — | Override SMS sender; auto-resolved to service default if absent |

---

## Job Statuses

All valid values are stored in the `job_status` lookup table.

| Status | Constant | Meaning |
|---|---|---|
| `pending` | `JOB_STATUS_PENDING` | Created and queued for immediate processing |
| `in progress` | `JOB_STATUS_IN_PROGRESS` | Celery task is actively creating notifications |
| `finished` | `JOB_STATUS_FINISHED` | All rows consumed; processing complete |
| `scheduled` | `JOB_STATUS_SCHEDULED` | Will not be processed until `scheduled_for` is in the past |
| `cancelled` | `JOB_STATUS_CANCELLED` | Cancelled before or during processing |
| `sending limits exceeded` | `JOB_STATUS_SENDING_LIMITS_EXCEEDED` | Rate-limit check failed at dispatch time |
| `ready to send` | `JOB_STATUS_READY_TO_SEND` | Letter pipeline intermediate state |
| `sent to dvla` | `JOB_STATUS_SENT_TO_DVLA` | Letter job forwarded to DVLA |
| `error` | `JOB_STATUS_ERROR` | Unrecoverable processing error |

### Valid Status Transitions

```
                  ┌─────────────────────────────────────────────┐
                  │  POST /job (scheduled_for set)               │
                  ▼                                              │
             scheduled ──── scheduled_for passes (cron) ──► pending
                  │                                              │
                  │ cancel endpoint                              │ process_job Celery task
                  ▼                                              ▼
             cancelled                               in progress
                                                          │
                                         ┌────────────────┼────────────────┐
                                         ▼                ▼                ▼
                                      finished          error       cancelled
                                         │
                              (letter only, within cancellation window)
                                         ▼
                                      cancelled
```

Additional terminal-ish statuses reachable from `pending` or `in progress` at the Celery dispatch layer:
`sending limits exceeded`, `ready to send`, `sent to dvla`.

---

## Data Access Patterns

### `jobs_dao.py`

#### `dao_get_notification_outcomes_for_job_batch(service_id, job_ids)`
- **Purpose**: Fetch per-status notification counts for multiple jobs in a single query, used in the list-jobs response.
- **Query type**: SELECT with GROUP BY on (`job_id`, `status`); operates on the live `notifications` table only.
- **Key filters**: `service_id == service_id`, `job_id IN job_ids`.
- **Returns**: List of `(job_id, status, count)` named-tuple rows.
- **Notes**: Used only for **recent** jobs (processed within the last 3 days). For older jobs, see `fetch_notification_statuses_for_job_batch` which reads from `ft_notification_status`.

---

#### `dao_get_notification_outcomes_for_job(service_id, job_id)`
- **Purpose**: Fetch per-status notification counts for a single job, used in the single-job GET response.
- **Query type**: UNION of two GROUP-BY SELECTs: one on `notifications`, one on `notification_history`, grouped by `status`.
- **Key filters**: `service_id == service_id`, `job_id == job_id` in both branches.
- **Returns**: List of `(count, status)` rows.
- **Notes**: Queries both live and archived notification tables so the result is complete regardless of how old the job is.

---

#### `dao_get_job_by_service_id_and_job_id(service_id, job_id)`
- **Purpose**: Retrieve a single job scoped to a service, for user-facing reads.
- **Query type**: SELECT with `filter_by(service_id, id)`.
- **Returns**: `Job` or `None` (uses `.first()`).
- **Notes**: Used by GET, cancel, and cancel-letter-job endpoints. Does **not** raise on missing — callers handle `None` by returning 404.

---

#### `dao_get_jobs_by_service_id(service_id, limit_days, page, page_size, statuses)`
- **Purpose**: Paginated list of jobs for a service, used in the list-jobs endpoint.
- **Query type**: SELECT with dynamic filter list + `paginate()`.
- **Key filters**:
  - `service_id == service_id` (always)
  - `created_at > get_query_date_based_on_retention_period(limit_days)` (when `limit_days` provided)
  - `job_status IN statuses` (when `statuses` is non-empty and not `[""]`)
- **Ordering**: `created_at DESC`
- **Returns**: SQLAlchemy `Pagination` object (`.items`, `.total`, `.per_page`).
- **Notes**: `page_size` defaults to `current_app.config["PAGE_SIZE"]`. Empty `statuses` list or `[""]` means no status filter.

---

#### `dao_get_job_by_id(job_id)`
- **Purpose**: Internal lookup by job UUID without service scoping.
- **Query type**: SELECT with `filter_by(id)`.
- **Returns**: `Job` (uses `.one()` — raises `NoResultFound` if missing).
- **Notes**: Used by the Celery `process_job` task. Not service-scoped; callers are internal.

---

#### `dao_archive_jobs(jobs)`
- **Purpose**: Soft-delete a collection of jobs by setting `archived = True`.
- **Query type**: Iterated UPDATE (one `session.add()` per job) then a single `session.commit()`.
- **Returns**: Nothing.
- **Notes**: Driven by the data-retention cron task. Archived jobs remain in the database but are excluded from all active queries (all active queries filter `archived == False`).

---

#### `dao_get_in_progress_jobs()`
- **Purpose**: Fetch all jobs currently being processed, used by cron/monitoring tasks.
- **Query type**: SELECT filtered by `job_status == JOB_STATUS_IN_PROGRESS`.
- **Returns**: List of `Job`.
- **Notes**: No service scoping; cross-service query.

---

#### `dao_service_has_jobs(service_id)`
- **Purpose**: Efficient existence check — does this service have **any** job record?
- **Query type**: `EXISTS` sub-select.
- **Returns**: Boolean scalar.
- **Notes**: Used by the `GET /has_jobs` endpoint to avoid loading full job rows.

---

#### `dao_set_scheduled_jobs_to_pending()`
- **Purpose**: Atomically promote all past-due scheduled jobs to `pending` so they can be picked up by the job processor.
- **Query type**: SELECT … FOR UPDATE, then batch UPDATE.
- **Key filters**: `job_status == JOB_STATUS_SCHEDULED` AND `scheduled_for < utcnow()`.
- **Ordering**: `scheduled_for ASC` (process oldest scheduled jobs first).
- **Returns**: List of `Job` rows that were updated.
- **Notes**: The `FOR UPDATE` lock prevents double-processing if the cron task runs concurrently. Called by the `run_scheduled_jobs` periodic Celery beat task.

---

#### `dao_get_future_scheduled_job_by_id_and_service_id(job_id, service_id)`
- **Purpose**: Retrieve a scheduled job that has not yet fired, for the cancel-scheduled-job endpoint.
- **Query type**: SELECT with `one()`.
- **Key filters**: `service_id`, `id`, `job_status == JOB_STATUS_SCHEDULED`, `scheduled_for > utcnow()`.
- **Returns**: `Job` (raises `NoResultFound` if not found or already promoted to pending).
- **Notes**: The `scheduled_for > utcnow()` guard ensures only genuinely future jobs can be cancelled this way.

---

#### `dao_create_job(job)`
- **Purpose**: Persist a newly built `Job` instance.
- **Query type**: INSERT.
- **Returns**: Nothing (mutates the passed object).
- **Notes**: Auto-assigns a UUID if `job.id` is falsy. Called after all validation passes in the create-job endpoint.

---

#### `dao_update_job(job)`
- **Purpose**: Persist state changes to an existing `Job`.
- **Query type**: UPDATE (via `session.add` + `session.commit`).
- **Returns**: Nothing.
- **Notes**: Used by the Celery task, REST endpoints, and archiving routines. No diff-checking; entire object is re-saved.

---

#### `dao_get_jobs_older_than_data_retention(notification_types, limit)`
- **Purpose**: Identify jobs eligible for archiving based on per-service data retention policies.
- **Query type**: Two nested loops producing UNION of results:
  1. For each `ServiceDataRetention` row matching the given notification types: select jobs for that service where `COALESCE(scheduled_for, created_at) < (today − retention_days)`, `archived == False`.
  2. For each notification type: select jobs for services **without** a custom policy where `COALESCE(scheduled_for, created_at) < (today − 7 days)`, `archived == False`.
- **Returns**: Combined list of `Job` objects (deduplicated by construction — pass 1 handles configured services, pass 2 handles the rest).
- **Notes**: `limit` is a global cap across all results; each sub-query's `.limit()` accounts for jobs already gathered. Jobs are joined to `templates` to filter by `template_type`.

---

#### `dao_cancel_letter_job(job)`
- **Purpose**: Cancel all notifications in a letter job and mark the job cancelled, atomically.
- **Query type**: Bulk UPDATE on `notifications` (set `status = cancelled`, `billable_units = 0`, `updated_at = utcnow()`), then UPDATE job.
- **Returns**: Integer — number of notifications cancelled.
- **Decorator**: `@transactional` — rolls back on any exception.
- **Notes**: Only callable after `can_letter_job_be_cancelled` returns `(True, None)`.

---

#### `can_letter_job_be_cancelled(job)` (validation helper)
- **Purpose**: Check whether a letter job's notifications are still within the cancellation window.
- **Returns**: `(True, None)` on success; `(False, error_string)` on failure.
- **Failure conditions**:
  1. Template type is not `letter` → `"Only letter jobs can be cancelled through this endpoint."`
  2. `job.job_status != JOB_STATUS_FINISHED` OR `len(notifications) != job.notification_count` → `"We are still processing these letters, please try again in a minute."`
  3. Any notification is not in `CANCELLABLE_JOB_LETTER_STATUSES` OR `letter_can_be_cancelled(NOTIFICATION_CREATED, job.created_at)` returns `False` → `"It's too late to cancel sending, these letters have already been sent."`

---

## Domain Rules & Invariants

### Job Statuses and Transitions

See the transition diagram above. Key invariants:

- A job in any status other than `pending` is **skipped** by `process_job`; the Celery task returns immediately if `job.job_status != JOB_STATUS_PENDING`.
- A job whose service is **inactive** when `process_job` runs is set to `cancelled` and the task exits.
- `dao_set_scheduled_jobs_to_pending` is the **only** mechanism that promotes `scheduled → pending`; it uses `FOR UPDATE` to guarantee exactly-once promotion under concurrent execution.

---

### Job Scheduling

- If `scheduled_for` is set on the incoming POST body, the REST layer sets `job_status = JOB_STATUS_SCHEDULED` before creating the record; the Celery task is **not** enqueued at creation time.
- The `run_scheduled_jobs` Celery beat task calls `dao_set_scheduled_jobs_to_pending()` periodically. It collects all jobs whose `scheduled_for < utcnow()`, promotes them to `pending`, and then each is dispatched to the `process_job` task queue.
- A scheduled job can only be cancelled while `scheduled_for > utcnow()`. Once the beat task promotes it to `pending`, the cancel endpoint will raise `NoResultFound` (404).

---

### Job Creation Validation (REST layer)

The `POST /service/{service_id}/job` endpoint enforces the following gates in order:

1. **Service active check**: service must have `active == True`; otherwise `403 InvalidRequest("service is inactive")`.
2. **S3 metadata presence**: `data["id"]` must map to an existing S3 object; missing key → `400 {"id": ["Missing data for required field."]}`.
3. **File validity flag**: S3 object metadata must contain `valid == "True"`; otherwise `400 "File is not valid"`.
4. **Template not archived**: `unarchived_template_schema.validate` on the template; archived templates are rejected.
5. **SMS-specific**:
   - Mixed simulated / non-simulated recipients in the same CSV → `400 "Bulk sending to testing and non-testing numbers is not supported"`.
   - Non-simulated (real) recipients: SMS annual limit checked, then SMS daily limit checked; counters incremented with warning thresholds.
   - If `FF_USE_BILLABLE_UNITS` feature flag is enabled, limit checks use `sms_fragment_count` (total fragments across all rows) rather than row count.
6. **Email-specific**:
   - `notification_count` is taken from S3 metadata when available; falls back to `len(recipient_csv)` (with a warning log).
   - Email annual limit checked, then daily limit checked.
   - Counter is incremented **only if** the job is not scheduled for a future date. Scheduling defers the increment.

---

### CSV Processing

- The CSV file is uploaded to S3 by the client **before** calling `POST /job`.
- At creation time the API fetches the raw CSV from S3 and parses it with `RecipientCSV` (from `notifications_utils`) passing:
  - `template_type` — controls which recipient column is expected (`phone_number` for SMS, `email_address` for email).
  - `placeholders` — list of personalisation variable names declared in the template.
  - `template` — the full template object for fragment-count calculation.
- The optional `reference` column per row becomes `client_reference` on the generated notification.
- At processing time the Celery task re-fetches the CSV from S3 and creates notifications in chunks of `Config.BATCH_INSERTION_CHUNK_SIZE` rows.
- Each row produces one signed `Notification` record. The signing payload includes: `api_key`, `key_type`, `template`, `template_version`, `job` (id), `to` (recipient), `row_number`, `personalisation`, `queue`, `sender_id`, `client_reference`.

---

### Job Cancellation

Two distinct cancel flows exist:

**Scheduled-job cancellation** (`POST /job/{id}/cancel`):
- Requires job to be in status `scheduled` with `scheduled_for > utcnow()` (enforced by `dao_get_future_scheduled_job_by_id_and_service_id` which uses `.one()`).
- Sets `job_status = cancelled`, saves, then decrements today's email send counter by `job.notification_count`.
- Only applies to email/SMS jobs — there is no guard against letter jobs but the email-count decrement implies it is email-oriented.

**Letter-job cancellation** (`POST /job/{id}/cancel-letter-job`):
- Job must exist for the service (any status).
- `can_letter_job_be_cancelled` must return `(True, None)` (see above for full conditions).
- Atomically cancels all notifications and the job via `dao_cancel_letter_job`.

---

### Statistics Tracking

Per-job notification statistics are computed at read time, not written incrementally (aside from the
`notifications_sent`, `notifications_delivered`, `notifications_failed` counters on the job row itself).

**Single-job read** (`GET /job/{id}`):
- Always queries `notifications` UNION `notification_history`, grouped by `(service_id, job_id, status)`.
- Result shape: `[{"status": "<status>", "count": <n>}, …]`.

**Job-list read** (`GET /job`):
- Jobs split into **recent** (processing_started ≥ midnight 3 days ago) and **old** (before that cutoff).
- **Recent**: batch query against live `notifications` table via `dao_get_notification_outcomes_for_job_batch`.
- **Old**: batch query against `ft_notification_status` fact table via `fetch_notification_statuses_for_job_batch`.
- Jobs with `processing_started == null` get `statistics = []`.
- Both approaches reduce N+1 queries by collecting all job IDs first, then fetching stats in one round trip per category.

---

### Pagination

Job listings use SQLAlchemy's `paginate()` helper:
- Pagination parameters come from the `GET /job` query string: `page` (default `1`), `page_size` not exposed directly (server uses `PAGE_SIZE` config).
- Response envelope: `{"data": […], "page_size": <n>, "total": <n>, "links": {…}}`.
- `pagination_links()` generates `next` / `prev` / `last` link URLs based on the pagination object and the view function name.

---

### Data Retention & Archiving

- Default retention: **7 days** from `COALESCE(scheduled_for, created_at)`.
- Override: per `(service_id, notification_type)` row in `ServiceDataRetention`.
- Archiving sets `archived = True`; it does not delete rows.
- `dao_get_jobs_older_than_data_retention` uses the template join to match `template_type` against the retention policy's `notification_type`.
- The `limit` parameter caps total results across all policies and types, accounting for results already gathered.

---

## Error Conditions

| Condition | Raised by | Error |
|---|---|---|
| Service is inactive | `create_job` REST handler | `InvalidRequest("service is inactive", 403)` |
| S3 metadata key missing | `create_job` REST handler | `InvalidRequest({"id": ["Missing data for required field."]}, 400)` |
| CSV file flagged invalid in S3 metadata | `create_job` REST handler | `InvalidRequest("File is not valid, can't create job", 400)` |
| Template is archived | `create_job` REST handler | `InvalidRequest(template_errors, 400)` |
| Mixed simulated + real SMS recipients | `create_job` REST handler | `InvalidRequest("Bulk sending to testing and non-testing numbers is not supported", 400)` |
| SMS annual limit exceeded | `check_sms_annual_limit` validator | `InvalidRequest` (raised inside validator) |
| SMS daily limit exceeded | `check_sms_daily_limit` validator | `InvalidRequest` (raised inside validator) |
| Email annual limit exceeded | `check_email_annual_limit` validator | `InvalidRequest` (raised inside validator) |
| Email daily limit exceeded | `check_email_daily_limit` validator | `InvalidRequest` (raised inside validator) |
| `limit_days` query param is not an integer | `get_jobs_by_service` REST handler | `InvalidRequest({"limit_days": ["… is not an integer"]}, 400)` |
| Job not found (single GET) | `get_job_by_service_and_job_id` REST handler | `jsonify(result="error", message="Job not found…"), 404` |
| Scheduled job not found / no longer future | `cancel_job` REST handler | `NoResultFound` → 404 (via `.one()`) |
| Letter job not found | `cancel_letter_job` REST handler | `jsonify(result="error", message="Job not found…"), 404` |
| Letter job cancellation guard fails | `cancel_letter_job` REST handler | `jsonify(message=<errors>), 400` |

---

## Query Inventory (for sqlc)

| Query name | Type | Tables | Description |
|---|---|---|---|
| `GetNotificationOutcomesForJobBatch` | SELECT | `notifications` | Status + count grouped by (job_id, status) for a set of job IDs within a service |
| `GetNotificationOutcomesForJob` | SELECT (UNION) | `notifications`, `notification_history` | Status + count grouped by status for a single job; union covers full history |
| `GetJobByServiceAndJobId` | SELECT ONE | `jobs` | Look up a job by (service_id, id); returns null if missing |
| `GetJobById` | SELECT ONE | `jobs` | Internal look up of a job by id; raises not-found |
| `GetJobsByServiceId` | SELECT + paginate | `jobs` | Paginated, filtered listing sorted by created_at DESC |
| `ServiceHasJobs` | EXISTS | `jobs` | Boolean existence check for a service |
| `GetInProgressJobs` | SELECT | `jobs` | All jobs with status = 'in progress' |
| `GetFutureScheduledJob` | SELECT ONE | `jobs` | Single scheduled job with scheduled_for > now for a service |
| `SetScheduledJobsToPending` | SELECT FOR UPDATE + UPDATE | `jobs` | Atomically promote past-due scheduled jobs to pending; return promoted rows |
| `CreateJob` | INSERT | `jobs` | Insert a new job row |
| `UpdateJob` | UPDATE | `jobs` | Persist state changes to an existing job |
| `ArchiveJobs` | UPDATE (batch) | `jobs` | Set archived = true on a collection of jobs |
| `CancelLetterJobNotifications` | UPDATE (bulk) | `notifications` | Set status = cancelled, billable_units = 0 for all notifications of a job |
| `GetJobsOlderThanDataRetention` | SELECT | `jobs`, `templates`, `service_data_retention` | Find non-archived jobs whose effective date is before the retention threshold |
| `GetNotificationStatusesForJobBatch` | SELECT | `ft_notification_status` | Status + count for a set of old job IDs (from fact table) |
