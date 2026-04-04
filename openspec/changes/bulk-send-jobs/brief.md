# Brief: bulk-send-jobs

## Source Files Analysed

- `spec/behavioral-spec/jobs.md`
- `spec/business-rules/jobs.md`
- `openspec/changes/bulk-send-jobs/proposal.md`

---

## Endpoints Covered

### Job REST API
| Method | Path | Description |
|--------|------|-------------|
| POST | `/service/{service_id}/job` | Create job (immediate or scheduled) |
| GET | `/service/{service_id}/job` | List jobs (paginated, filterable) |
| GET | `/service/{service_id}/job/{job_id}` | Get single job with statistics |
| POST | `/service/{service_id}/job/{job_id}/cancel` | Cancel a scheduled job |
| GET | `/service/{service_id}/job/{job_id}/notifications` | List notifications for job |
| GET | `/service/{service_id}/job/has_jobs` | Boolean existence check |

---

## Key Data Model Facts

### `jobs` table columns
- `id`, `original_file_name`, `service_id`, `template_id`, `template_version`
- `notification_count`, `notifications_sent`, `notifications_delivered`, `notifications_failed`
- `processing_started` (nullable, indexed), `processing_finished` (nullable)
- `created_by_id` (nullable — null for API-key jobs), `api_key_id` (nullable)
- `scheduled_for` (nullable UTC, indexed), `job_status` (FK lookup), `archived`, `sender_id`

### Valid job statuses
`pending`, `in progress`, `finished`, `scheduled`, `cancelled`, `sending limits exceeded`, `ready to send`, `sent to dvla`, `error`

### Status transitions
```
scheduled → pending (run_scheduled_jobs beat) OR cancel
pending → in progress (process_job worker)
in progress → finished (mark_jobs_complete)
in progress → error (check_job_status after 30 min timeout)
error → finished (mark_jobs_complete when all notifications confirmed)
any → cancelled (inactive service guard inside process_job)
```

---

## Business Logic Invariants

### Job Creation Gates (in order)
1. Service must be `active=true`: 403 `"service is inactive"`.
2. `data["id"]` must map to existing S3 object.
3. S3 metadata `valid == "True"` (exact string): 400 `"File is not valid"`.
4. Template must not be archived: 400 `"Template has been deleted"`.
5. SMS: mixed simulated/non-simulated recipients: 400.
6. SMS annual limit, then daily limit.
7. Email annual limit, then daily limit.

### Scheduling
- `scheduled_for` must be ≤ 96 hours in future and not in the past.
- Scheduled jobs get status `scheduled`; `process_job` NOT enqueued at creation.
- `run_scheduled_jobs` beat calls `dao_set_scheduled_jobs_to_pending()` with `FOR UPDATE` lock.
- Cancel only possible while `scheduled_for > utcnow()`.
- Email daily count decremented on cancel of scheduled job.

### process-job Worker (idempotent)
- Guard: if `job_status != "pending"` → return immediately (prevents double-processing).
- If service is `active=false` when task runs → set status `cancelled`, exit.
- Reads CSV from S3 via streaming (do NOT load entire file into memory).
- Builds notifications in chunks of `BATCH_INSERTION_CHUNK_SIZE`.
- Sets `job_status = "in progress"` and `processing_started` timestamp before processing.
- Selects delivery queue: `CSV_BULK_REDIRECT_THRESHOLD` triggers downgrades to LOW queue even for PRIORITY templates.
- Dispatches `save_smss` or `save_emails` per batch.

### CSV Validation
- Empty CSV file: job processes normally but no save tasks dispatched.
- Every row must have all required personalisation keys.
- `reference` column per row → `client_reference` on notification.
- At creation time: parse CSV with `RecipientCSV` to count rows and fragments.

### S3 CSV Client (`internal/client/s3`)
- Operations: `GetObject`, `PutObject`, `GeneratePresignedURL`.
- Use AWS SDK v2; stream `GetObject` — do not buffer full file.

### Statistics Source Selection
- Single job GET (`GET /job/{id}`): always UNION live `notifications` + `notification_history`.
- Job list GET (`GET /job`):
  - `processing_started` within last 3 days → live `notifications` table batch query.
  - Older jobs → `ft_notification_status` fact table batch query.
  - `processing_started IS NULL` → `statistics = []`.

### process-incomplete-jobs Recovery
- Triggered by `check_job_status` after detecting a job stuck > 30 min in `IN_PROGRESS`.
- Sets job to `JOB_STATUS_ERROR`.
- `process_incomplete_jobs` resets `processing_started` and re-dispatches only rows with no existing notification (row-number awareness).

### Billable Units Feature Flag (`FF_USE_BILLABLE_UNITS`)
- Enabled: limit checks use `sms_fragment_count` (total fragments across all rows).
- Disabled: limit checks use `csv_length` (one unit per recipient).
- Simulated phone numbers skip all limit checks when flag is enabled.

### Per-bulk-send Notification Limit
- Annual limit checked first, then daily.
- 429 response for annual limit: `"Exceeded annual {TYPE} sending limit of N messages"`.
- Daily limit increment deferred for jobs scheduled on a future calendar day.

### Job Notifications (GET /job/{id}/notifications)
- Ordered by `job_row_number` ascending.
- `format_for_csv=true`: returns specific CSV keys only.
- Status filter via `status` query parameter (array).

### Pagination
- Response envelope: `{"data": [], "page_size": N, "total": N, "links": {…}}`.
- `links.prev` only present on pages > 1.
- Server controls `PAGE_SIZE` from config; client only sends `page`.

### Scheduled Beat Tasks
| Task | Period | Description |
|------|--------|-------------|
| `run-scheduled-jobs` | Every minute | Promote past-due scheduled jobs to pending, enqueue `process_job` |
| `mark-jobs-complete` | Every minute | Finish jobs where all notifications are created |
| `check-job-status` | Every minute | Detect stuck in-progress jobs (>30 min), set to error, trigger incomplete-jobs |

---

## Known Errors Handled
| Condition | Response |
|-----------|----------|
| Inactive service at create | 403 |
| S3 file invalid | 400 |
| Archived template | 400 |
| Mixed simulated/real | 400 |
| Annual limit exceeded | 429 |
| Daily limit exceeded | 429 |
| Scheduled job not found / already promoted | 404 |
| Non-UUID job_id | 404 |

---

## Auth Requirements
- All job REST endpoints: internal authorization header.

---

## Dependency Notes
- Requires: `send-email-notifications`, `send-sms-notifications` (save workers), `template-management` (template lookup).
- Introduces: `internal/client/s3/` (also used by `platform-admin-features`, `billing-tracking`).
- Non-goals: single notification send; general notifications list.
