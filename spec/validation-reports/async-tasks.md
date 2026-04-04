# Validation Report: async-tasks.md
Date: 2026-04-04

## Summary
- **Tasks in spec**: 53 (claimed)
- **Tasks in code**: 58 (actual)
- **Beat schedule entries in spec**: 31 (claimed)
- **Beat schedule entries in code**: 31 ✅ CONFIRMED
- **Queues in spec**: 22 (claimed)
- **Queues in code**: 25 (defined in QueueNames)
- **CONFIRMED**: 46 core tasks + all 31 beat entries verified
- **DISCREPANCIES**: 5 tasks in code not in spec
- **MISSING FROM SPEC**: 1 active production task (`process-pinpoint-result`)
- **EXTRA IN SPEC**: 0 (all 53 documented tasks exist in code)
- **RISK items**: 5

---

## Task Count Reconciliation

### Tasks Found in Code: 58 Total

| Module | Count | Status |
|---|---|---|
| tasks.py | 8 | ✅ All in spec |
| provider_tasks.py | 3 | ✅ All in spec |
| scheduled_tasks.py | 17 | ✅ All in spec (some unscheduled — see below) |
| nightly_tasks.py | 12 | ✅ 10 in spec; 2 marked dead code |
| reporting_tasks.py | 7 | ✅ All in spec |
| process_ses_receipts_tasks.py | 1 | ✅ In spec |
| process_sns_receipts_tasks.py | 1 | ✅ In spec |
| process_pinpoint_receipts_tasks.py | 1 | ⚠️ **NOT IN SPEC** |
| service_callback_tasks.py | 2 | ✅ Both in spec |
| letters_pdf_tasks.py | 5 | ✅ All 5 stubs in spec |
| research_mode_tasks.py | 1 | ✅ In spec (but spec over-counts helpers) |
| **TOTAL** | **58** | **53 in spec + 5 gap** |

---

## Beat Schedule Discrepancies

### ✅ All 31 Entries Confirmed

Every beat schedule entry from spec verified in `app/config.py` CELERYBEAT_SCHEDULE (intervals, cron schedules, queue assignments all match).

### ⚠️ CONFIRMED BUG: Missing Q4 Quarterly Email

- `insert-quarter-data-for-annual-limits-q4` — ✅ **present** (Apr 1, 23:00 UTC)
- `send-quarterly-email-q4` — ❌ **MISSING** (should be ~Apr 2-3)
- **Impact**: April quarterly usage emails are never sent automatically
- **Note**: This bug is mentioned in `specs/README.md` "Notable Constraints" — confirmed real

---

## Queue Name Discrepancies

Code defines **25** queue names vs spec's **22**. All 22 active queues from spec confirmed. The 3 extras are:

1. `create-letters-pdf-tasks` — present but unused (letters are stubs)
2. `notifiy-cache-tasks` — **TYPO** (`notifiy` not `notify`) in `app/config.py`; appears to be unused
3. Confirmed 22 active queues otherwise match spec exactly

---

## Retry Policy Discrepancies

### ✅ ALL VERIFIED — Matches Spec Exactly

| Task | max_retries | default_retry_delay | Status |
|---|---|---|---|
| save-smss | 5 | 300s | ✅ |
| save-emails | 5 | 300s | ✅ |
| deliver-sms | 48 | 300s | ✅ |
| deliver-throttled-sms | 48 | rate_limit 30/m | ✅ |
| deliver-email | 48 | MalwareScan backoff | ✅ |
| send-inbound-sms | 5 | — | ✅ |
| send-delivery-status | 5 | — | ✅ |
| send-complaint | 5 | — | ✅ |
| process-ses-result | 5 | 300s | ✅ |
| process-sns-result | 5 | 300s | ✅ |
| process-pinpoint-result | 5 | 300s | ✅ (not in spec but policy matches SNS pattern) |

---

## Tasks Missing From Spec

### 1. `process-pinpoint-result` — ACTIVE PRODUCTION TASK

- **File**: `app/celery/process_pinpoint_receipts_tasks.py`
- **Queue**: `delivery-receipts`
- **Trigger**: Pinpoint SMS delivery receipts via SQS
- **Payload**: `response` dict (same structure as SNS result)
- **Retry**: max_retries=5, default_retry_delay=300
- **Status**: Active, used when `FF_USE_PINPOINT_FOR_DEDICATED` is enabled
- **RISK**: Go implementor will not implement Pinpoint receipt processing

---

## Unscheduled / Dead Tasks (in code, not in CELERYBEAT_SCHEDULE)

These tasks exist as registered Celery tasks but are not in the beat schedule and appear to be legacy/dead code. Spec does not document most of them (correctly):

1. `switch-current-sms-provider-on-slow-delivery` — unscheduled, legacy
2. `check-precompiled-letter-state` — unscheduled, letter dead code
3. `check-templated-letter-state` — unscheduled, letter dead code
4. `raise-alert-if-letter-notifications-still-sending` — unscheduled
5. `raise-alert-if-no-letter-ack-file` — unscheduled
6. `remove_letter_jobs` — unscheduled, letter dead code
7. `delete_dvla_response_files` — dead code, marked "TODO: remove me" in code

---

## Research-Mode Documentation

Spec claims "3 research-mode helpers". Reality:
- `create-fake-letter-response-file` — 1 registered Celery task ✅
- `send_sms_response()`, `send_email_response()`, `aws_sns_callback()`, `aws_pinpoint_callback()` — utility functions, NOT Celery tasks

Spec over-counts research-mode tasks. Only 1 is a real Celery task.

---

## Letter Stubs — CONFIRMED

All 5 letter PDF tasks are stubs (empty body):
1. `create-letters-pdf` ✅
2. `collate-letter-pdfs-for-day` ✅
3. `process-virus-scan-passed` ✅
4. `process-virus-scan-failed` ✅
5. `process-virus-scan-error` ✅

---

## RISK Items for Go Implementors

### 🔴 CRITICAL

1. **Missing `process-pinpoint-result` task in spec**
   - Active production task processing Pinpoint SMS delivery receipts
   - Required when `FF_USE_PINPOINT_FOR_DEDICATED=true`
   - Go rewrite must implement this goroutine worker

2. **Q4 quarterly email never fires (confirmed bug)**
   - `send-quarterly-email-q4` beat entry is absent from Python code
   - Go rewrite must add it (README already flags this)

### 🟡 MODERATE

3. **Queue name typo `notifiy-cache-tasks`**
   - Appears unused but if any code references this typo, Go must match exactly or the queue won't be consumed

4. **7 unscheduled legacy tasks in code**
   - Spec correctly omits most, but Go implementors reading the code may be confused about which tasks to port
   - Recommend adding a "dead/legacy tasks" list to spec

5. **Research-mode count mismatch (3 in spec, 1 actual task)**
   - Minor, only affects test/research mode workers
   - Go implementors should implement only `create-fake-letter-response-file` as a worker

---

## Recommendations

1. Add `process-pinpoint-result` to `async-tasks.md` (queue, trigger, payload, retry policy)
2. Add `send-quarterly-email-q4` to beat schedule spec section (already noted in README)
3. Add a "dead/legacy tasks" appendix listing the 7 unscheduled tasks for clarity
4. Fix research-mode count: 1 Celery task, 4 utility functions (not "3 helpers")
5. Document queue name typo `notifiy-cache-tasks` — flag for Go to standardize
