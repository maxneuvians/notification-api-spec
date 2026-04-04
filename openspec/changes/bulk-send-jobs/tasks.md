## 1. S3 Client

- [ ] 1.1 Implement `internal/client/s3/client.go` — interface `S3Client` with `GetObject(ctx, bucket, key) (io.ReadCloser, error)` (streaming, no full-buffer), `PutObject(ctx, bucket, key string, body io.Reader, metadata map[string]string) error`, `GeneratePresignedURL(ctx, bucket, key string, expiry time.Duration) (string, error)`; wrap AWS SDK v2; implement `MockS3Client` for tests; write unit tests for all three methods

## 2. Job Service Layer

- [ ] 2.1 Implement `internal/service/jobs/create.go` — `CreateJob`: validate service active (403); fetch S3 metadata and check `valid == "True"` (400); validate `original_file_name` and `notification_count` present; check template not archived; for SMS check mixed-simulated (400), then annual then daily limits (`FF_USE_BILLABLE_UNITS` aware); for email check annual then daily limits; defer daily count increment for future-day scheduled jobs; call `dao_create_job`; if no `scheduled_for` enqueue `process_job`; write unit tests for each gate in isolation
- [ ] 2.2 Implement `internal/service/jobs/cancel.go` — `CancelScheduledJob`: fetch via `dao_get_future_scheduled_job_by_id_and_service_id` (uses `.one()` — returns 404 if not found or already promoted); set `job_status = cancelled`; decrement email daily counter by `job.notification_count` (email jobs only); write tests: pending job returns 404; finished job returns 404

## 3. Job REST Handlers

- [ ] 3.1 Implement `internal/handler/services/jobs.go` — `POST /service/{id}/job` (delegating to CreateJob service), `GET /service/{id}/job` (paginated, statuses + limit_days filters, statistics source selection), `GET /service/{id}/job/{jid}` (with statistics UNION), `POST /service/{id}/job/{jid}/cancel`, `GET /service/{id}/job/has_jobs`; write handler tests for all documented error shapes
- [ ] 3.2 Implement `internal/handler/services/job_notifications.go` — `GET /service/{id}/job/{jid}/notifications` ordered by `job_row_number` ASC; support `status[]` filter; support `format_for_csv=true` returning exactly the CSV keys; write tests: status filter applied; CSV format keys verified; cross-job isolation

## 4. Job Statistics

- [ ] 4.1 Implement statistics source selection in the job list handler: partition job IDs into recent (`processing_started >= midnight 3 days ago`) and old; query live `notifications` via `GetNotificationOutcomesForJobBatch` for recent; query `ft_notification_status` via `GetNotificationStatusesForJobBatch` for old; return `statistics: []` for jobs with `processing_started = null`; write tests for both branches and null case

## 5. process_job Worker

- [ ] 5.1 Implement `internal/worker/jobs/process_job.go` — idempotent guard (`job_status != pending` → return); fetch service, check `service.active` (set cancelled and return if false); set `job_status = "in progress"`, set `processing_started`; read CSV via `S3Client.GetObject` streaming row-by-row; build signed notification per row (include `to`, `template`, `template_version`, `personalisation`, `row_number`, `job`, `queue`, `sender_id`, `client_reference`); batch to `BATCH_INSERTION_CHUNK_SIZE`; select queue considering `CSV_BULK_REDIRECT_THRESHOLD` downgrade (even for priority templates); dispatch `save_smss` or `save_emails`; record `statsd_client.timing_with_dates` once; write tests for all branches
- [ ] 5.2 Implement inactive-service and limit-exceeded guards within `process_job`: set `job_status = "cancelled"` on inactive service; set `job_status = "sending_limits_exceeded"` with `processing_finished = now` on limit failure; write tests confirming no S3 read on inactive service

## 6. process_incomplete_jobs Recovery Worker

- [ ] 6.1 Implement `internal/worker/jobs/process_incomplete_job.go` — `ProcessIncompleteJob(job_id)`: fetch job (raise on not found); re-read CSV from S3; fetch existing notification row numbers for this job; dispatch only rows whose `job_row_number` is absent from existing set; use same queue routing as `process_job`; write tests: 3 existing rows → 7 dispatched; all rows existing → no dispatch; email job → save_emails
- [ ] 6.2 Implement `internal/worker/jobs/process_incomplete_jobs.go` — `ProcessIncompleteJobs(job_ids []string)`: for each ID reset `job.processing_started = now()` (do NOT change `job_status`); call `ProcessIncompleteJob(id)`; write tests: two IDs → both processing_started reset; empty list → no-op

## 7. Scheduled Beat Tasks

- [ ] 7.1 Implement `internal/worker/scheduled/run_scheduled_jobs.go` — every minute: call `dao_set_scheduled_jobs_to_pending()` with `SELECT FOR UPDATE` row lock; for each returned job call `process_job.apply_async([job_id], queue="job-tasks")`; write tests: past-due job promoted and enqueued; future job not touched; multiple jobs enqueued in `scheduled_for` ascending order
- [ ] 7.2 Implement `internal/worker/scheduled/mark_jobs_complete.go` — every minute: fetch in-progress and error-status jobs; for each job count notifications in DB; if count >= `job.notification_count` set `job_status = "finished"`; write tests using the transition table from the spec (0/3, 1/3, 3/3 cases for both IN_PROGRESS and ERROR statuses)
- [ ] 7.3 Implement `internal/worker/scheduled/check_job_status.go` — every minute: fetch all in-progress jobs with `updated_at > 30 minutes ago`; set each to `JOB_STATUS_ERROR`; raise `JobIncompleteError` listing their IDs; send a single `PROCESS_INCOMPLETE_JOBS` task to `QueueNames.JOBS`; write tests: recently-updated job not flagged; multiple stuck jobs batched in one send_task

## 8. Integration Tests

- [ ] 8.1 Write integration test: create a 300-row SMS job, verify `process_job` dispatches 3 batches of 100 (using `BATCH_INSERTION_CHUNK_SIZE=100`); verify `job_status` transitions `pending → in progress`
- [ ] 8.2 Write integration test: set `scheduled_for` 1 minute in future, advance mock clock, verify `run_scheduled_jobs` promotes job to pending and enqueues `process_job`
- [ ] 8.3 Write integration test: create job with 10 rows, simulate 7 saved notifications, call `process_incomplete_job`, verify exactly 3 new notifications dispatched (row-number aware)
