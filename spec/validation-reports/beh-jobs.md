# Validation Report: behavioral-spec/jobs.md
Date: 2026-04-04

## Summary
- **Contracts in spec**: ~28
- **CONFIRMED**: 25
- **DISCREPANCIES**: 0
- **UNCOVERED**: 3 edge cases
- **Risk items**: 4

## Verdict
**PASS** — All major endpoint and task contracts backed by tests. 3 edge cases uncovered.

---

## Confirmed

- GET /service/{id}/job: paginated list, statistics, status/limit_days filter, processing_started ordering ✅
- Statistics source: jobs with processing_started within 3 days → live table; older → ft_notification_status ✅
- processing_started IS NULL → empty statistics list ✅
- POST /service/{id}/job: creates pending job, enqueues process_job immediately ✅
- Scheduled job creation: status=scheduled, NOT enqueued, 96h limit, past date → 400 ✅
- Sender ID: S3 metadata; falls back to service inbound SMS sender if absent ✅
- POST .../cancel: cancels scheduled only; non-scheduled → 404 ✅
- FF_USE_BILLABLE_UNITS: controls SMS limit check (billable_units vs csv_length) ✅
- Annual limit errors: 429 `"Exceeded annual {TYPE} sending limit of {N} messages"` ✅
- Inactive service: 403 `"Create job is not allowed: service is inactive"` ✅
- process_job task: sets job_status=in progress, records processing_started, emits statsd metric ✅
- save_smss/save_emails: metadata persisted (job_id, row_number, sender_id), personalisation encrypted ✅
- Retry on SQLAlchemyError → "retry-tasks" queue ✅
- IntegrityError (duplicate): no retry, no delivery task, receipt still acknowledged ✅

---

## Discrepancies
None.

---

## Uncovered Contracts

1. **Simulated phone number bypass**: `+1613...` numbers should skip limit checks when `FF_USE_BILLABLE_UNITS` enabled — no test found (**medium risk**)
2. **Email daily count increment for unscheduled jobs only**: edge case for jobs scheduled >36 hours out not tested
3. **process_rows restricted service validation**: all recipient edge cases not exhaustively covered

---

## RISK Items for Go Implementors

### 🔴 CRITICAL
**1. Simulated phone number limit bypass** — In spec, NOT tested. Implement and add tests before Go deployment.

### 🟡 MEDIUM
**2. 3-day fact table cutoff logic** — Uses `processing_started`, not `created_at`, with NULL handling. Ensure datetime arithmetic matches spec exactly.

**3. Restricted service whitelist validation** — SMS recipient phone checks have basic coverage only; implement carefully.

**4. Scheduled job timing** — 96-hour limit, 24-36 hour threshold for daily count increment. Precise datetime handling required.
