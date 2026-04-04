## Context

Bulk sending is the primary high-volume use case: a caller uploads a CSV to S3, then POSTs job metadata. The API validates the file, enforces rate limits, creates the `jobs` row, and (for non-scheduled jobs) immediately enqueues a `process_job` Celery-equivalent task. The Celery worker reads the CSV back from S3, splits recipients into notification batches, and enqueues `save_smss` or `save_emails`. A recovery path (`process_incomplete_jobs`) re-triggers stalled jobs without re-sending already-created notifications.

## Goals / Non-Goals

**Goals:** Job REST API, `process_job` worker, `process_incomplete_jobs` recovery worker, scheduled-job beat tasks (`run_scheduled_jobs`, `mark_jobs_complete`, `check_job_status`), S3 client wrapper, per-bulk-send SMS/email limit enforcement.

**Non-Goals:** Single notification send (covered by `send-email-notifications` / `send-sms-notifications`); notification list beyond the per-job endpoint.

## Decisions

### D1 — statistics source: live table vs fact table

Per-job notification statistics are computed at read time in two modes:

- **Single job GET**: always UNION live `notifications` + `notification_history`.
- **Job list GET**: split by recency.
  - `processing_started` within the last 3 days → batch query on live `notifications`.
  - Older jobs → batch query on `ft_notification_status` fact table.
  - `processing_started IS NULL` → return `statistics = []`.

This replicates Python's `get_job_with_statistics` logic and avoids expensive full-table scans on old jobs. The 3-day cutoff is measured from midnight of 3 days ago (not a rolling 72-hour window).

### D2 — process_job is idempotent via status guard

Before any processing, `process_job` checks `job.job_status == "pending"`. If the job is already past pending (in progress, finished, cancelled, error), the task returns immediately without reading S3 or creating notifications. This prevents double-processing on retry or accidental re-queue.

### D3 — S3 CSV streaming

`internal/client/s3` implements `GetObject` using the AWS SDK v2 streaming response — the CSV body is streamed row-by-row via `encoding/csv.NewReader`, never fully buffered in memory. This is required for large recipient lists that exceed available memory.

For `PutObject`, the caller provides an `io.Reader`; the client streams it directly to S3.

### D4 — process_incomplete_jobs uses row-number awareness

When recovering a stalled job, `process_incomplete_jobs` fetches all existing notification row numbers for the job from the DB. Only rows whose `job_row_number` is absent from that set are re-dispatched. This prevents duplicate notifications when a job partially completed before stalling.

### D5 — billable units flag determines limit check unit

When `FF_USE_BILLABLE_UNITS` is enabled:
- SMS limit checks use `RecipientCSV.sms_fragment_count` (total message fragments across all rows).
- Simulated phone numbers bypass all limit checks.

When disabled, one unit per recipient row regardless of message length.

### D6 — CSV_BULK_REDIRECT_THRESHOLD downgrade

When the job's `notification_count` exceeds `CSV_BULK_REDIRECT_THRESHOLD`, delivery queue routing is forced to the LOW (bulk) queue regardless of template `process_type`. This cap applies even to PRIORITY templates. The threshold is read from app config at dispatch time.

### D7 — scheduled job cancel decrements daily email counter

When a scheduled email job is cancelled, today's email send counter is decremented by `job.notification_count`. This counter was incremented at the time the job was scheduled for the same calendar day. Counter is NOT decremented for SMS jobs.

## Risks / Trade-offs

- **CSV metadata exact match**: S3 metadata key `valid` must equal the string `"True"` (capital T). Any deviation returns 400. This is preserved from Python by design.
- **Atomic scheduled-job promotion**: `dao_set_scheduled_jobs_to_pending` uses `SELECT … FOR UPDATE` to prevent concurrent beat runs from double-promoting the same job. Go must replicate the row-lock.
- **check_job_status timeout**: 30-minute threshold is hardcoded in the Python scheduled task. Go may read this from config but must default to 30 minutes for compatibility.
- **process_incomplete_jobs vs process_incomplete_job**: `process_incomplete_jobs` (plural) is the beat task entry point that iterates over stuck job IDs and resets `processing_started` on each. `process_incomplete_job` (singular) is the per-job worker. Both must be implemented as separate callables.
