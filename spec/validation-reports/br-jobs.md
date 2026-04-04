# Validation Report: business-rules/jobs.md
Date: 2026-04-04

## Summary
- **Spec DAO functions**: 15
- **Code functions**: All 15 present + 2 Celery tasks
- **Confirmed**: 15/15
- **Discrepancies**: 0
- **Missing from spec**: 1 helper (`update_in_progress_jobs`)
- **Risk items**: 4

## Verdict
**PASS** — All critical DAOs implemented as specified. Job lifecycle (creation → S3 → processing → archival), status transitions, and letter cancellation all match spec.

---

## Confirmed

**DAO Layer (15/15)**:
- Query functions (outcome counts, job retrieval by service/template/id): ✅ 6
- Job CRUD (create, update, archive): ✅ 3
- Scheduled job flow (`dao_set_scheduled_jobs_to_pending` with `FOR UPDATE`): ✅
- Cancellation (letter jobs, scheduled jobs): ✅
- Archival & retention: ✅ 2
- Existence checks: ✅

**Celery Tasks**:
- `process_job` in `app/celery/tasks.py`: ✅ Checks pending status, service.active, sets timestamps, processes CSV in chunks
- `run_scheduled_jobs` in `app/celery/scheduled_tasks.py`: ✅ Calls `dao_set_scheduled_jobs_to_pending()`, enqueues each to task queue

**All 11 documented error conditions matched in code.**

---

## Discrepancies
None significant.

---

## Missing from Spec

| Function | Location | Purpose |
|---|---|---|
| `update_in_progress_jobs()` | `app/celery/scheduled_tasks.py` | Monitoring task that updates `job.updated_at` from the latest sent notification — stall detection helper |

---

## RISK Items for Go Implementors

1. **Job status enum mismatch**: Spec claims 9 statuses (`pending`, `in progress`, `finished`, `scheduled`, `cancelled`, `sending limits exceeded`, `ready to send`, `sent to dvla`, `error`). Verify lookup table has all 9 during migration — see data-model report for native enum vs lookup table distinction.

2. **Letter cancellation window**: `letter_can_be_cancelled()` is imported from `notifications_utils.letter_timings` (external Python package). Verify the timing rules match business requirements before Go port.

3. **CSV file validation metadata**: S3 metadata must contain `valid="True"` for the job to be processed. Confirm the S3 metadata contract with the file upload service — not detailed in spec.

4. **API key vs user-initiated jobs**: Jobs can originate from either `api_key_id` (API-driven) or `created_by_id` (user-driven). Ensure signing payload correctly reflects origin in both paths.
