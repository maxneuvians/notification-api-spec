## Requirements

### Requirement: Create job (immediate)
`POST /service/{service_id}/job` SHALL create a job with `job_status="pending"`, validate S3 metadata, enforce rate limits, and immediately enqueue `process_job`.

#### Scenario: Successful unscheduled job creation returns 201
- **GIVEN** an active service with a valid non-archived template; S3 metadata has `valid="True"`, `original_file_name`, `notification_count`
- **WHEN** `POST /service/{id}/job` is called with a valid `id` mapping to the S3 object
- **THEN** HTTP 201; response includes `id`, `job_status: "pending"`, `template`, `original_file_name`, `notification_count`, `statistics: []`, `scheduled_for: null`; `process_job` is enqueued on the `"job-tasks"` queue

#### Scenario: Sender ID from S3 metadata stored on job
- **GIVEN** S3 object metadata contains `sender_id = "some-uuid"`
- **WHEN** `POST /service/{id}/job` is called
- **THEN** job `sender_id` equals the value from S3 metadata and is returned in the response

#### Scenario: Inactive service returns 403
- **GIVEN** a service with `active = false`
- **WHEN** `POST /service/{id}/job` is called
- **THEN** HTTP 403, `{"result": "error", "message": "Create job is not allowed: service is inactive "}`; `dao_create_job` is NOT called

#### Scenario: S3 metadata valid flag must be exact string "True"
- **WHEN** S3 metadata has `valid = "true"` (lowercase) or `valid = "false"`
- **THEN** HTTP 400, `{"message": "File is not valid, can't create job"}`

#### Scenario: Archived template rejected
- **WHEN** the job references a template with `archived = true`
- **THEN** HTTP 400, `{"message": {"template": ["Template has been deleted"]}}`

#### Scenario: SMS annual limit exceeded returns 429
- **WHEN** the service has already hit its annual SMS limit
- **THEN** HTTP 429, `{"message": "Exceeded annual sms sending limit of N messages"}`

---

### Requirement: Create scheduled job
`POST /service/{service_id}/job` with `scheduled_for` in an allowed future window SHALL create a job with `job_status="scheduled"` without enqueuing `process_job`.

#### Scenario: Scheduled job creation returns 201 without enqueue
- **GIVEN** `scheduled_for` is 2 hours in the future
- **WHEN** `POST /service/{id}/job` is called
- **THEN** HTTP 201; `job_status = "scheduled"`; `scheduled_for` stored in UTC; `process_job` is NOT enqueued; email daily count NOT incremented

#### Scenario: scheduled_for in the past returns 400
- **WHEN** `scheduled_for` is set to a timestamp in the past
- **THEN** HTTP 400, `{"message": {"scheduled_for": ["Date cannot be in the past"]}}`

#### Scenario: scheduled_for more than 96 hours in future returns 400
- **WHEN** `scheduled_for` is set to 97 hours from now
- **THEN** HTTP 400, `{"message": {"scheduled_for": ["Date cannot be more than 96 hours in the future"]}}`

---

### Requirement: Cancel scheduled job
`POST /service/{service_id}/job/{job_id}/cancel` SHALL cancel a job that is still in `scheduled` status with `scheduled_for` in the future.

#### Scenario: Cancel scheduled job returns 200
- **GIVEN** a job with `job_status = "scheduled"` and `scheduled_for` in the future
- **WHEN** `POST /service/{id}/job/{jid}/cancel` is called
- **THEN** HTTP 200, `{"data": {"id": ..., "job_status": "cancelled"}}`

#### Scenario: Cannot cancel non-scheduled job
- **GIVEN** a job with `job_status = "pending"` or `"in progress"`
- **WHEN** `POST /service/{id}/job/{jid}/cancel` is called
- **THEN** HTTP 404; `dao_update_job` is NOT called

#### Scenario: Email daily count decremented on cancel
- **GIVEN** a scheduled email job with `notification_count = 500`
- **WHEN** the job is cancelled
- **THEN** today's email send counter is decremented by 500

---

### Requirement: Get single job with statistics
`GET /service/{service_id}/job/{job_id}` SHALL return the job object with embedded per-status notification statistics as a UNION of live and history tables.

#### Scenario: Returns job with statistics
- **GIVEN** a job with 10 delivered and 2 failed notifications
- **WHEN** `GET /service/{id}/job/{jid}` is called
- **THEN** HTTP 200, `{"data": {"id": ..., "statistics": [{"status": "delivered", "count": 10}, {"status": "failed", "count": 2}], "created_by": {"name": ...}}}`

#### Scenario: Non-existent job UUID returns 404
- **WHEN** `GET /service/{id}/job/{jid}` is called with a valid UUID that does not exist
- **THEN** HTTP 404, `{"result": "error", "message": "Job not found in database"}`

#### Scenario: Non-UUID job_id returns 404
- **WHEN** `GET /service/{id}/job/{jid}` is called with a non-UUID string as `job_id`
- **THEN** HTTP 404, `{"result": "error", "message": "No result found"}`

#### Scenario: Statistics include both live and history notifications
- **GIVEN** a job where some notifications have been moved to `notification_history`
- **WHEN** `GET /service/{id}/job/{jid}` is called
- **THEN** statistics include counts from both `notifications` and `notification_history` tables

---

### Requirement: List jobs with pagination and filters
`GET /service/{service_id}/job` SHALL return a paginated list of jobs ordered by `processing_started DESC` then `created_at DESC`, supporting `statuses` and `limit_days` filters.

#### Scenario: Returns paginated jobs with statistics and links
- **GIVEN** a service with 15 jobs and `PAGE_SIZE = 10`
- **WHEN** `GET /service/{id}/job` is called without params (page 1)
- **THEN** HTTP 200; body has `data`, `page_size`, `total`, `links.next`, `links.last`; 10 jobs returned; no `links.prev`

#### Scenario: Filter by statuses
- **GIVEN** a service with both finished and pending jobs
- **WHEN** `GET /service/{id}/job?statuses=finished` is called
- **THEN** HTTP 200; only jobs with `job_status = "finished"` returned

#### Scenario: limit_days excludes old jobs
- **GIVEN** jobs created 1 day ago and 10 days ago
- **WHEN** `GET /service/{id}/job?limit_days=7` is called
- **THEN** only the job created 1 day ago is returned

#### Scenario: Statistics from live table for recent jobs
- **GIVEN** a job with `processing_started` 1 day ago
- **WHEN** `GET /service/{id}/job` is called
- **THEN** statistics for that job come from the live `notifications` table

#### Scenario: Statistics from fact table for old jobs
- **GIVEN** a job with `processing_started` 4 days ago
- **WHEN** `GET /service/{id}/job` is called
- **THEN** statistics for that job come from `ft_notification_status`

#### Scenario: Jobs with null processing_started have empty statistics
- **GIVEN** a scheduled job that has not yet been processed
- **WHEN** `GET /service/{id}/job` is called
- **THEN** that job has `statistics: []`

---

### Requirement: Job has_jobs existence check
`GET /service/{service_id}/job/has_jobs` SHALL return a boolean indicating whether any job exists for the service.

#### Scenario: Returns true when jobs exist
- **GIVEN** at least one job for the service
- **WHEN** `GET /service/{id}/job/has_jobs` is called
- **THEN** HTTP 200, `{"data": {"has_jobs": true}}`

#### Scenario: Returns false when no jobs exist
- **GIVEN** a service with no jobs
- **WHEN** `GET /service/{id}/job/has_jobs` is called
- **THEN** HTTP 200, `{"data": {"has_jobs": false}}`

---

### Requirement: Get job notifications
`GET /service/{service_id}/job/{job_id}/notifications` SHALL return notifications for the job ordered by `job_row_number` ascending, supporting status filter and CSV format.

#### Scenario: Returns notifications ordered by row number
- **GIVEN** a job with 5 notifications
- **WHEN** `GET /service/{id}/job/{jid}/notifications` is called
- **THEN** HTTP 200, `{"notifications": [...]}` with items in ascending `job_row_number` order; each item includes `id`, `to`, `job_row_number`, `status`

#### Scenario: Status filter applied
- **WHEN** `GET /service/{id}/job/{jid}/notifications?status=delivered` is called
- **THEN** only `delivered` notifications returned

#### Scenario: CSV format returns specific keys only
- **WHEN** `GET /service/{id}/job/{jid}/notifications?format_for_csv=true` is called
- **THEN** each item has exactly: `created_at`, `created_by_name`, `created_by_email_address`, `template_type`, `template_name`, `job_name`, `status`, `row_number`, `recipient`; no other keys

#### Scenario: Notifications isolated to this job
- **GIVEN** two jobs on the same service and template
- **WHEN** `GET /service/{id}/job/{jid}/notifications` is called for job 1
- **THEN** only job 1's notifications are returned; job 2's notifications absent

---

### Requirement: process_job worker — idempotent CSV processing
`process_job` SHALL set `job_status = "in progress"`, stream the CSV from S3, and dispatch batched save tasks. If `job_status != "pending"` at run time, the task SHALL exit immediately.

#### Scenario: Processes pending job and transitions status
- **GIVEN** a job with `job_status = "pending"` referencing an SMS template and a 10-row CSV
- **WHEN** `process_job` runs
- **THEN** `job_status = "in progress"`; `processing_started` is set; `save_smss.apply_async` is called; each signed notification includes `to`, `template`, `template_version`, `personalisation`, `row_number`, `job`, `queue`, `sender_id`

#### Scenario: Idempotent — already in-progress job skipped
- **GIVEN** a job with `job_status = "in progress"` (already past pending)
- **WHEN** `process_job` runs
- **THEN** task returns immediately; no S3 read; no save task dispatched

#### Scenario: Inactive service at run time cancels job
- **GIVEN** the service was deactivated after the job was created
- **WHEN** `process_job` runs
- **THEN** `job_status = "cancelled"`; no S3 read; no notifications created

#### Scenario: Empty CSV dispatches no save tasks
- **GIVEN** a job with an empty CSV file on S3
- **WHEN** `process_job` runs
- **THEN** `job_status = "in progress"`; `processing_started` set; no `save_smss` or `save_emails` call

#### Scenario: CSV bulk redirect threshold downgrades queue
- **GIVEN** a job whose `notification_count` exceeds `CSV_BULK_REDIRECT_THRESHOLD` and uses a priority template
- **WHEN** `process_job` runs
- **THEN** delivery tasks are dispatched to the LOW (bulk) queue, not the priority queue

---

### Requirement: process_incomplete_jobs recovery worker
`process_incomplete_jobs` SHALL reset `processing_started` for stalled jobs and re-dispatch only rows without existing notifications, using row-number awareness to avoid duplicates.

#### Scenario: Re-dispatches only missing rows
- **GIVEN** a stalled job with 10 total rows; 3 notifications already exist for row numbers 1, 2, 3
- **WHEN** `process_incomplete_job` is called for this job
- **THEN** `save_smss.apply_async` is called once with 7 notifications (rows 4–10 only); rows 1–3 not re-sent

#### Scenario: Fully processed job not resent
- **GIVEN** a stalled job where all 10 rows already have notifications
- **WHEN** `process_incomplete_job` is called
- **THEN** no `save_smss` or `save_emails` call

#### Scenario: processing_started reset before re-dispatch
- **GIVEN** a list of two stalled job IDs
- **WHEN** `process_incomplete_jobs` runs
- **THEN** `processing_started` reset to now for both jobs; `job_status` remains `JOB_STATUS_ERROR`

#### Scenario: Non-existent job ID raises exception
- **WHEN** `process_incomplete_job` is called with a job ID that does not exist
- **THEN** raises an exception; no save task dispatched

---

### Requirement: run_scheduled_jobs beat task
`run_scheduled_jobs` SHALL atomically promote all past-due scheduled jobs to pending and enqueue each via `process_job`.

#### Scenario: Past-due scheduled job promoted and enqueued
- **GIVEN** a scheduled job whose `scheduled_for` is in the past
- **WHEN** `run_scheduled_jobs` runs
- **THEN** job `job_status = "pending"`; `process_job.apply_async([job_id], queue="job-tasks")` called

#### Scenario: Future scheduled job not promoted
- **GIVEN** a scheduled job whose `scheduled_for` is 2 hours in the future
- **WHEN** `run_scheduled_jobs` runs
- **THEN** job remains with `job_status = "scheduled"`; `process_job` not called

#### Scenario: Multiple past-due jobs ordered by scheduled_for ascending
- **GIVEN** two past-due jobs scheduled 6 hours ago and 2 hours ago respectively
- **WHEN** `run_scheduled_jobs` runs
- **THEN** both promoted; process_job enqueued for both in chronological order (oldest first)

---

### Requirement: check_job_status beat task and stuck-job detection
`check_job_status` SHALL identify in-progress jobs stuck for more than 30 minutes, set them to `JOB_STATUS_ERROR`, raise `JobIncompleteError`, and enqueue `PROCESS_INCOMPLETE_JOBS`.

#### Scenario: Stuck job detected and set to error
- **GIVEN** a job with `job_status = "in progress"` and `updated_at` > 30 minutes ago
- **WHEN** `check_job_status` runs
- **THEN** `job_status = JOB_STATUS_ERROR`; `JobIncompleteError` raised listing the job ID; `PROCESS_INCOMPLETE_JOBS` sent to `QueueNames.JOBS`

#### Scenario: Recently updated in-progress job not flagged
- **GIVEN** a job with `job_status = "in progress"` and `updated_at` < 30 minutes ago
- **WHEN** `check_job_status` runs
- **THEN** job unchanged; no error raised

#### Scenario: Multiple stuck jobs batched in single send_task call
- **GIVEN** three jobs all stuck > 30 minutes
- **WHEN** `check_job_status` runs
- **THEN** all three IDs passed in a single `send_task` call; all three listed in `JobIncompleteError.message`

---

### Requirement: mark_jobs_complete beat task
`mark_jobs_complete` SHALL finish jobs where the number of notifications in the DB equals or exceeds `notification_count`, regardless of prior error status.

#### Scenario: In-progress job transitions to finished when all rows created
- **GIVEN** a job with `notification_count = 3` and 3 notification rows in the DB
- **WHEN** `mark_jobs_complete` runs
- **THEN** `job_status = "finished"`

#### Scenario: Error-status job also transitions to finished on completion
- **GIVEN** a job with `job_status = "error"` and `notification_count = 3` with 3 DB rows
- **WHEN** `mark_jobs_complete` runs
- **THEN** `job_status = "finished"`

#### Scenario: Partially completed job stays in current status
- **GIVEN** a job with `notification_count = 3` and only 1 notification row in the DB
- **WHEN** `mark_jobs_complete` runs
- **THEN** `job_status` unchanged

---

### Requirement: S3 client streaming
`internal/client/s3` SHALL stream CSV content via `GetObject` without buffering the entire file in memory, and provide `PutObject` and `GeneratePresignedURL`.

#### Scenario: GetObject streams row-by-row
- **GIVEN** a large CSV in S3 (100 000 rows)
- **WHEN** `GetObject` is called and the caller reads row-by-row via `csv.NewReader`
- **THEN** no full-file buffer is allocated; rows are processed as they arrive from the AWS SDK stream

#### Scenario: PutObject accepts io.Reader
- **GIVEN** an `io.Reader` source of CSV data
- **WHEN** `PutObject(ctx, bucket, key, reader, metadata)` is called
- **THEN** the file appears in S3 with the supplied metadata; no intermediate buffer required

#### Scenario: GeneratePresignedURL returns a signed URL valid for the requested duration
- **WHEN** `GeneratePresignedURL(ctx, bucket, key, 60*time.Minute)` is called
- **THEN** returns a URL that grants temporary GET access to the object for approximately 60 minutes

---

### Requirement: Per-bulk-send notification limit enforcement
Job creation SHALL enforce SMS and email annual and daily limits before creating the job record.

#### Scenario: FF_USE_BILLABLE_UNITS uses fragment count for SMS limits
- **GIVEN** `FF_USE_BILLABLE_UNITS = true` and a 100-row CSV where each row produces 2 SMS fragments
- **WHEN** `POST /service/{id}/job` is called
- **THEN** limit checks are performed with value 200 (fragments), not 100 (rows)

#### Scenario: Simulated phone numbers skip limits when FF enabled
- **GIVEN** `FF_USE_BILLABLE_UNITS = true` and all recipients are simulated numbers
- **WHEN** `POST /service/{id}/job` is called regardless of limit state
- **THEN** all limit checks bypassed; job created successfully

#### Scenario: Mixed simulated and real numbers rejected
- **GIVEN** a CSV with both simulated and real phone numbers
- **WHEN** `POST /service/{id}/job` is called
- **THEN** HTTP 400, `"Bulk sending to testing and non-testing numbers is not supported"`
