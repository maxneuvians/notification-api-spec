## Why

Bulk sending allows government services to send the same message to thousands of recipients via a CSV file upload. This change implements the job REST API, the `process-job` worker, the `process-incomplete-jobs` recovery path, the scheduled-job beat tasks, and the daily/annual limit enforcement for bulk sends.

## What Changes

- `internal/handler/` job family: POST/GET job, cancel job, per-job notifications list, has_jobs
- `internal/service/jobs/` — create job (validate CSV metadata from S3, check limits, enqueue or schedule), cancel job
- `internal/worker/jobs/` — `process_job.go` (read CSV from S3, batch-build notifications, enqueue save tasks), `process_incomplete_jobs.go` (detect and resume stalled jobs)
- Beat scheduler jobs: `run-scheduled-jobs` (every minute), `mark-jobs-complete` (every minute), `check-job-status` (every minute), `process-incomplete-jobs` (triggered by check-job-status)
- `internal/client/s3/` — S3 get/put/presign wrappers for CSV upload/download

## Capabilities

### New Capabilities

- `bulk-send-jobs`: Job REST API, process-job worker, scheduled-job beat tasks, S3 CSV integration, per-bulk-send limit enforcement

### Modified Capabilities

## Non-goals

- Single notification send (covered in `send-email-notifications` and `send-sms-notifications`)
- Notification list for a job beyond the job notifications endpoint (part of the general notifications list)

## Impact

- Requires `send-email-notifications` and `send-sms-notifications` (save workers must exist), `notification-delivery-pipeline` (save workers are precondition), `template-management` (template lookup)
- Introduces `internal/client/s3/` used also by `platform-admin-features` and `billing-tracking`
