# Validation Report: behavioral-spec/notifications.md
Date: 2026-04-04

## Summary
- **Contracts in spec**: ~42
- **Contracts with test coverage**: ~38
- **CONFIRMED**: 38
- **DISCREPANCIES**: 1 (service permission error message wording)
- **UNCOVERED**: 3 edge cases
- **EXTRA BEHAVIORS in tests**: 2 (implementation details, not gaps)
- **Risk items**: 2

## Verdict
**MOSTLY CONFIRMED** — Well-tested spec. One error message wording discrepancy that affects Go clients parsing error strings. Three low/medium risk uncovered edge cases.

---

## Confirmed (concise)

- POST /notifications/sms and /email (v0): 201 shape, validation, queue selection by process_type ✅
- POST/GET /v2/notifications (email, sms, bulk): full contract including personalisation size limit (51,200 bytes), document count limit (10), scheduled_for max 96h ✅
- GET /v2/notifications/{id}: null fields present, status_description remapping ✅
- GET /v2/notifications: cursor-based pagination (older_than), status composite expansion ✅
- Annual limit contracts: 80% warning, 100% blocks (429), fiscal year April 1→March 31, TrialService vs LiveService error types ✅
- Daily limits (SMS): `FF_USE_BILLABLE_UNITS` controls fragment vs message counting, test key bypass, simulated numbers excluded ✅
- Rate limiting: 60-second window per `{service_id}-{key_type}` ✅
- Status transitions: all documented transitions confirmed in tests ✅
- Bounce rate: slow threshold, test-key exclusion, TEMPORARY_FAILURE exclusion ✅
- Callback dispatch: delivery status queue, complaint → permanent-failure for OnAccountSuppressionList ✅
- Schema validation: UUID format, personalisation type, ISO8601 scheduled_for ✅
- SES bounce subtype mapping ✅
- Simulated recipients (v0 and v2): 201, not persisted, not enqueued ✅
- Template archival 400, missing personalisation 400 ✅
- Trial mode restriction, team key safelist rules ✅
- Content char limits (SMS ≤ 612) ✅
- Bulk request: exactly one of `rows` or `csv` ✅

---

## Discrepancies

### 1. Service Permission Error Message Wording — MODERATE
**Contract**: POST /v2/notifications/sms when service lacks SMS permission

**Spec says**: `"Cannot send text messages"` and `"Cannot send emails"`

**Test asserts**:
```python
assert error_json["errors"][0]["message"] == "Service is not allowed to send text messages"
assert error_json["errors"][0]["message"] == "Service is not allowed to send emails"
```

**Impact**: Go API clients parsing error messages against spec wording will fail. Use test assertions as source of truth.

---

## Uncovered Contracts

1. `dao_get_notification_history_by_reference` — history table reference lookup: no explicit test found (low risk, mirrors active table behavior)
2. `notifications_not_yet_sent` — stale `created` status filtering: not found in test searches (medium risk, affects timeout task eligibility)
3. SMS 612-char boundary with personalisation substitution: tests check pre-rendered + placeholder, but no test at the exact 612-char fragment boundary

---

## Extra Behaviors in Tests (not in spec)
1. Mock annual limit seeding: tests reveal Redis expects pre-initialized hash structure — spec documents the contract, not initialization path
2. `fetch_todays_requested_sms_billable_units_count` mock structure — implementation detail, not a contract gap

---

## RISK Items for Go Implementors

### 🔴
**1. Error message wording mismatch** — Use tests as source of truth: `"Service is not allowed to send {medium}"`, not the truncated spec wording.

### 🟡
**2. Simulated recipient list is hard-coded** — Spec lists 3 SMS numbers and 3 emails; no dynamic configuration tested. Verify with stakeholders if this list is static or should be config-driven.
