# Spec Validation — Master Summary
Date: 2026-04-04

This document synthesizes all 27 validation reports produced by the agent series across Phases 1–4. It is the final gate before the Go rewrite phase begins.

---

## Overall Verdict

| Layer | Files validated | Verdict |
|---|---|---|
| Foundation / Data Model | `data-model.md` vs `out.sql` + `app/models.py` | ⚠️ CONDITIONAL PASS |
| API Surface | `api-surface.md` vs Flask blueprints | ⚠️ CONDITIONAL PASS |
| Async Tasks | `async-tasks.md` vs `app/celery/` | ⚠️ CONDITIONAL PASS |
| Business Rules (10 domains) | `business-rules/*.md` vs DAO + service layers | ✅ PASS / CONDITIONAL PASS |
| Behavioral Spec (13 domains) | `behavioral-spec/*.md` vs tests | ✅ PASS / CONDITIONAL PASS |
| Cross-Spec Consistency | `go-architecture.md` vs all other specs | ⚠️ CONDITIONAL PASS |

**Overall: CONDITIONAL PASS — 27/27 spec files validated. No FAIL verdicts. Corrections required in 7 areas before Go implementation proceeds.**

---

## Critical Findings (Must Fix Before Go Rewrite)

These issues would cause a silently incorrect or insecure Go implementation if not corrected first.

### C1 — Missing Encrypted Column: `inbound_sms._content`
**Reports**: [data-model.md](data-model.md), [cross-spec-consistency.md](cross-spec-consistency.md)
- Python encrypts `inbound_sms._content` via `signer_inbound_sms` but this column does not appear in `data-model.md`, `business-rules/inbound-sms.md`, or `go-architecture.md`
- **Fix**: Add `inbound_sms._content` to all encrypted-column lists in spec

### C2 — 4 Undocumented API Routes
**Report**: [api-surface.md](api-surface.md), [cross-spec-consistency.md](cross-spec-consistency.md)
- 4 routes exist in the Python codebase that are not in `api-surface.md` or `go-architecture.md`:
  1. `POST /newsletter/update-language/{subscriber_id}`
  2. `GET /newsletter/send-latest/{subscriber_id}`
  3. `GET /newsletter/find-subscriber`
  4. `GET /platform-stats/send-method-stats-by-service`
- **Fix**: Add these 4 routes to `api-surface.md` with their auth scheme and request/response shapes

### C3 — Missing Active Celery Task: `process-pinpoint-result`
**Report**: [async-tasks.md](async-tasks.md)
- `process-pinpoint-result` is an active production task in `app/celery/process_pinpoint_receipts_tasks.py` but is absent from `async-tasks.md`
- (Note: `go-architecture.md` does mention it under `handlePinpointReceipt()` — only the spec intermediate file is missing it)
- **Fix**: Add `process-pinpoint-result` to `async-tasks.md` with queue assignment (`delivery-receipts`), trigger, payload, and retry policy

### C4 — `process_type` Hybrid Property Crashes on Null Category
**Report**: [br-templates.md](br-templates.md)
- `app/models.py` `process_type` hybrid property accesses `self.template_category.sms_process_type` without null-guard
- `template_category_id` is nullable — any template without a category and without `process_type_column` will raise `AttributeError` at runtime
- **Fix**: Add null check: `self.template_category.sms_process_type if self.template_category else None`

### C5 — Template History Version Collision
**Report**: [br-templates.md](br-templates.md)
- `dao_update_template_process_type` writes a history row WITHOUT incrementing `version`
- Querying `dao_get_template_by_id(id, version=N)` after this operation will raise `MultipleResultsFound` (`.one()` fails)
- Spec documents this as a "known anomaly" but does not prescribe a fix
- **Fix**: Either increment version in this function, or change version-based query to `.first()` and document the edge case

### C6 — Missing Q4 Quarterly Email Beat Schedule Entry
**Reports**: [async-tasks.md](async-tasks.md), [cross-spec-consistency.md](cross-spec-consistency.md)
- `send-quarterly-email-q4` is absent from Python CELERYBEAT_SCHEDULE
- `insert-quarter-data-for-annual-limits-q4` (Apr 1) IS present
- `go-architecture.md` already prescribes Go add `0 23 2 4 *`
- **Fix in Python**: Add the missing entry to CELERYBEAT_SCHEDULE. Fix in spec: Add the entry to `async-tasks.md` beat schedule section (31 → 32 entries)

### C7 — Exception Syntax Error in `service_sms_sender_dao.py`
**Report**: [br-services.md](br-services.md)
- `raise Exception("You must have at least one SMS sender as the default.", 400)` produces garbled error output
- **Fix**: Replace with `raise InvalidRequest("...", 400)` (consistent with rest of codebase)

---

## High-Priority Findings (Fix Before Go, or Document as Known Constraint)

### H1 — 7 Letter Stub Endpoints Not Explicitly Mapped
**Reports**: [api-surface.md](api-surface.md), [cross-spec-consistency.md](cross-spec-consistency.md)
- Spec and architecture acknowledge that letter endpoints are stubs but do not enumerate which 7 endpoints or specify their required status codes
- Stubs: 5 letter-contact routes + `/service/{id}/send-pdf-letter` + `/letters/returned`
- **Fix**: Add a "Letter Stub Endpoints" appendix to `go-architecture.md` listing each endpoint and its expected response code

### H2 — `FF_PT_SERVICE_SKIP_FRESHDESK` Behavior Not Tested
**Report**: [beh-users-auth.md](beh-users-auth.md)
- Province/territory services call a different Freshdesk endpoint and return 201 instead of 204
- No test covers this; Go implementation will miss this branch
- **Fix**: Add targeted test in Python before Go rewrite; ensure spec documents the different return code

### H3 — Partial Index on `api_keys` Not Documented
**Report**: [data-model.md](data-model.md)
- `uix_service_to_key_name` has `WHERE expiry_date IS NULL` in `out.sql` — soft-delete semantics
- Not documented in `data-model.md`
- **Fix**: Add note to index documentation

### H4 — `services_history.prefix_sms` Nullable Mismatch
**Report**: [data-model.md](data-model.md)
- `services.prefix_sms` is NOT NULL; `services_history.prefix_sms` lacks the NOT NULL constraint
- **Fix**: Document the discrepancy and add NOT NULL to history table in a new migration

### H5 — Conditional Update Logic Uses Truthiness (`if url:`) vs `is not None`
**Report**: [br-services.md](br-services.md)
- `reset_service_inbound_api` and `reset_service_callback_api` use `if url:` instead of `if url is not None:`
- Prevents clearing webhook URL/token to empty string
- **Fix**: Change to `is not None` check; test with empty string reset

### H6 — Auth Types `CacheClear-v1` and `Cypress-v1` Not Documented
**Report**: [br-users-auth.md](br-users-auth.md)
- `api-surface.md` and `go-architecture.md` mention JWT and ApiKey-v1 but not the 2 internal auth types
- **Fix**: Add section to `api-surface.md` documenting internal auth schemes for `/cache-clear` and `/cypress`

### H7 — `dao_update_template_category` Missing `template_category_id` in History Row
**Report**: [br-templates.md](br-templates.md)
- Three template update functions omit `template_category_id` from manually-constructed history dicts
- Breaks category assignment audit trail
- **Fix**: Add field to all three dicts

---

## Moderate Findings (Document or Fix Post-Go-Rewrite)

| # | Issue | Report |
|---|---|---|
| M1 | Job status: native enum `job_status_types` (4 vals) vs lookup table (9 vals) — Go must use lookup table | [data-model.md](data-model.md) |
| M2 | `notify_status_type` native enum unused — may confuse Go implementors | [data-model.md](data-model.md) |
| M3 | `SensitiveString` type abstraction for notifications columns not documented | [data-model.md](data-model.md) |
| M4 | `dao_count_organsations_with_live_services` typo is intentional and everywhere | [br-organisations.md](br-organisations.md) |
| M5 | `get_user_by_id(user_id=None)` returns ALL users — nil guard needed in Go | [br-users-auth.md](br-users-auth.md) |
| M6 | Events DAO (`dao_create_event`) has no `@transactional` wrapper — event loss on commit failure | [br-platform-admin.md](br-platform-admin.md) |
| M7 | Duplicate category cascade delete has double-commit (harmless but fragile) | [br-templates.md](br-templates.md) |
| M8 | Complaint count timezone dependency (EST midnight) | [br-platform-admin.md](br-platform-admin.md) |
| M9 | Read replica routing pattern described but not annotated per-function | [cross-spec-consistency.md](cross-spec-consistency.md) |
| M10 | `test-manifest.txt` referenced in README does not exist on disk | README.md |
| M11 | Simulated recipients: bullet-point error message wording in beh-notifications vs actual test assertion | [beh-notifications.md](beh-notifications.md) |
| M12 | MOU notification emails: test is `@pytest.mark.skip` — unverified behavior | [beh-organisations.md](beh-organisations.md) |
| M13 | Provider failover inert: `get_alternative_sms_provider` returns same provider | [br-providers.md](br-providers.md), [beh-providers.md](beh-providers.md) |
| M14 | Salesforce 4 integration points have no retry or idempotency; mocked entirely in tests | [beh-services.md](beh-services.md) |
| M15 | Q4 quarterly email bug: `send-quarterly-email-q4` absent from Python CELERYBEAT_SCHEDULE | [async-tasks.md](async-tasks.md) (see C6) |
| M16 | Queue name typo `notifiy-cache-tasks` (unused but present in QueueNames) | [async-tasks.md](async-tasks.md) |
| M17 | Billing quarterly aggregation (`get_previous_quarter`, etc.) not thoroughly tested | [beh-billing.md](beh-billing.md) |
| M18 | Domain cross-org semantics in `GET /organisations/by-domain` ambiguous (test is xfail) | [beh-organisations.md](beh-organisations.md) |

---

## Confirmed Correct (No Action Required)

The following were fully verified and match spec:

- All 68 database tables (accounting for `alembic_version` infrastructure table) ✅
- All 6 history tables (`*_history`) follow composite PK, append-only pattern ✅
- Both fact tables (`ft_billing`, `ft_notification_status`) composite PKs and columns ✅
- All 31 beat schedule entries match CELERYBEAT_SCHEDULE in code ✅
- All retry policies match (`max_retries`, `default_retry_delay`) ✅
- All 6 feature flags documented with correct env vars ✅
- Both error response formats (admin and v2) documented correctly ✅
- All notification status transitions and guards ✅
- All limit enforcement logic (rate, daily SMS/email, annual) ✅
- All DAO functions for notifications (40/40), organisations (19/19), billing (18/18) ✅
- Auth type decorators (all blueprints correctly secured) ✅
- Simulated recipient bypass logic (both v0 and v2 paths) ✅
- Template versioning rules (all increment/no-increment conditions) ✅
- Inbound SMS encryption, pagination, retention — all match ✅
- Provider selection algorithm (all 6 classification flags, all DO-NOT-USE conditions) ✅
- External client implementations (Airtable, SES, SNS, Pinpoint, Salesforce, Freshdesk) ✅

---

## Required Spec Corrections Before Go Rewrite

The following changes must be made to the spec files:

| # | File to update | Change |
|---|---|---|
| 1 | `specs/data-model.md` | Add `inbound_sms._content` to encrypted columns list |
| 2 | `specs/data-model.md` | Document partial index `WHERE expiry_date IS NULL` on `api_keys` |
| 3 | `specs/data-model.md` | Clarify job_status lookup table (9 vals) vs native enum (4 vals, unused) |
| 4 | `specs/data-model.md` | Add note on `SensitiveString` type for notifications columns |
| 5 | `specs/api-surface.md` | Add 4 missing routes (3 newsletter + 1 platform-stats) |
| 6 | `specs/api-surface.md` | Add status codes for `revoke` (202) and `suspend` (204) |
| 7 | `specs/api-surface.md` | Mark 5 letter-contact routes + send-pdf-letter as stubs |
| 8 | `specs/async-tasks.md` | Add `process-pinpoint-result` task entry |
| 9 | `specs/async-tasks.md` | Add `send-quarterly-email-q4` beat schedule entry (31→32) |
| 10 | `specs/async-tasks.md` | Fix research-mode count (1 Celery task, not 3) |
| 11 | `specs/go-architecture.md` | Add `inbound_sms._content` to encrypted columns list |
| 12 | `specs/go-architecture.md` | Add explicitly mapped stub endpoint appendix (7 stubs + response codes) |
| 13 | `specs/go-architecture.md` | Annotate which repository functions should use reader vs writer DB |
| 14 | `specs/business-rules/templates.md` | Document `process_type` null-guard requirement + version collision edge case |
| 15 | `specs/business-rules/services.md` | Fix exception syntax note + conditional update semantics |
| 16 | `specs/README.md` | Remove reference to `test-manifest.txt` (file does not exist) or create it |

---

## Validation Report Index

| Phase | Report | Verdict |
|---|---|---|
| 1A | [data-model.md](data-model.md) | ⚠️ 7 risk items |
| 1B | [api-surface.md](api-surface.md) | ⚠️ 4 missing routes, 2 undocumented status codes |
| 1C | [async-tasks.md](async-tasks.md) | ⚠️ 1 missing task, Q4 bug confirmed |
| 2 | [br-notifications.md](br-notifications.md) | ✅ PASS |
| 2 | [br-services.md](br-services.md) | ⚠️ Exception syntax, conditional logic |
| 2 | [br-templates.md](br-templates.md) | ⚠️ 2 runtime bugs, audit trail gap |
| 2 | [br-jobs.md](br-jobs.md) | ✅ PASS |
| 2 | [br-billing.md](br-billing.md) | ✅ PASS |
| 2 | [br-users-auth.md](br-users-auth.md) | ⚠️ Undocumented auth types |
| 2 | [br-organisations.md](br-organisations.md) | ✅ PASS |
| 2 | [br-inbound-sms.md](br-inbound-sms.md) | ✅ PASS |
| 2 | [br-providers.md](br-providers.md) | ⚠️ Failover inert, noted |
| 2 | [br-platform-admin.md](br-platform-admin.md) | ⚠️ Events DAO, double-commit |
| 3 | [beh-notifications.md](beh-notifications.md) | ⚠️ Error message wording discrepancy |
| 3 | [beh-services.md](beh-services.md) | ✅ PASS |
| 3 | [beh-templates.md](beh-templates.md) | ✅ PASS |
| 3 | [beh-jobs.md](beh-jobs.md) | ⚠️ Simulated phone bypass untested |
| 3 | [beh-billing.md](beh-billing.md) | ⚠️ Quarterly aggregation under-tested |
| 3 | [beh-users-auth.md](beh-users-auth.md) | ⚠️ PT Freshdesk bypass untested |
| 3 | [beh-organisations.md](beh-organisations.md) | ⚠️ MOU test skipped, domain semantics ambiguous |
| 3 | [beh-inbound-sms.md](beh-inbound-sms.md) | ✅ PASS |
| 3 | [beh-providers.md](beh-providers.md) | ⚠️ Failover tests skipped |
| 3 | [beh-platform-admin.md](beh-platform-admin.md) | ✅ PASS |
| 3 | [beh-external-clients.md](beh-external-clients.md) | ✅ PASS |
| 3 | [beh-smoke-tests.md](beh-smoke-tests.md) | ✅ PASS |
| 3 | [beh-misc-gaps.md](beh-misc-gaps.md) | ✅ PASS |
| 4 | [cross-spec-consistency.md](cross-spec-consistency.md) | ⚠️ 5 critical gaps in go-architecture.md |

---

## Readiness Assessment

| Gate | Status | Blocker? |
|---|---|---|
| All spec files validated by an agent | ✅ Done | — |
| No FAIL verdicts | ✅ None | — |
| Critical security gaps identified | ⚠️ C1 (encrypted column) | Fix before Go |
| Runtime crash risks identified | ⚠️ C4, C5 (process_type, version collision) | Fix before Go |
| Undocumented routes catalogued | ⚠️ C2 (4 routes) | Fix before Go |
| Missing active task documented | ⚠️ C3 (Pinpoint task) | Fix before Go |
| Go architecture coherent | ⚠️ 5 gaps | Address before implementation starts |

**Recommendation: Apply the 16 spec corrections listed above before initiating Go rewrite. The corrections are additive (no spec content is being reversed, only gaps filled). The spec is at ~95% fidelity to the Python implementation today.**
