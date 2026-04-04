# Validation Report: go-architecture.md Cross-Spec Consistency
Date: 2026-04-04

## Summary
- **Checks performed**: 9 (A–I)
- **CONFIRMED**: 4 checks with no issues
- **DISCREPANCIES**: 5 checks with findings
- **Critical RISK items**: 5
- **Moderate RISK items**: 1

## Verdict
**MOSTLY CONSISTENT** — go-architecture.md correctly documents all feature flags, error formats, Q4 bug, and worker pools. However, 5 gaps would require Go implementors to research beyond the architecture document to produce a complete, secure implementation.

---

## Check A: Endpoint Coverage

**Status**: ✅ Main groups confirmed / ❌ 4 undocumented routes missing

All 32+ principal route groups are present in the Go router structure. However, the api-surface validation found 4 routes in code not in spec, and these are ALSO absent from go-architecture.md:

1. `POST /newsletter/update-language/{subscriber_id}` (admin JWT) — updates language preference
2. `GET /newsletter/send-latest/{subscriber_id}` (admin JWT) — sends latest newsletter
3. `GET /newsletter/find-subscriber` (admin JWT) — finds subscriber
4. `GET /platform-stats/send-method-stats-by-service` (admin JWT, query: start_date, end_date)

From go-architecture.md:
```
├── /newsletter     → handler/newsletter     (admin JWT)
├── /platform-stats → handler/platform_stats (admin JWT)
```
Packages exist but full route coverage not detailed.

---

## Check B: Worker Coverage

**Status**: ✅ All documented tasks covered / ✅ Pinpoint mentioned / ✅ Q4 acknowledged / ⚠️ Count stated as 53

- All 53 documented tasks are accounted for across worker pools
- `process-pinpoint-result` IS mentioned under `receiptWorkerPool` with `handlePinpointReceipt()` handler ✅
- Q4 email bug explicitly acknowledged in go-architecture.md: `"The Go implementation should add 0 23 2 4 * for consistency"` ✅
- Task count says "all 53 Celery tasks" but Python has 58 — minor, since Pinpoint is covered

---

## Check C: Encrypted Columns

**Status**: ❌ DISCREPANCY — Missing 6th encrypted column

go-architecture.md lists 5 encrypted columns:
```
notifications._personalisation, service_callback_api.bearer_token,
service_inbound_api.bearer_token, verify_codes._code, users._password
```

**Missing**: `inbound_sms._content` — encrypted in Python `app/models.py` but absent from spec and architecture.

**Impact**: Go implementors will store inbound SMS content in plaintext, creating a security gap (inbound SMS may contain PII).

---

## Check D: Feature Flags

**Status**: ✅ CONFIRMED — All 6 flags documented

1. `FF_USE_BILLABLE_UNITS` ✅
2. `FF_SALESFORCE_CONTACT` ✅
3. `FF_USE_PINPOINT_FOR_DEDICATED` ✅
4. `FF_BOUNCE_RATE_SEED_EPOCH_MS` ✅
5. `FF_PT_SERVICE_SKIP_FRESHDESK` ✅
6. `FF_ENABLE_OTEL` ✅

---

## Check E: Q4 Quarterly Email

**Status**: ✅ CONFIRMED AND EXPLICITLY ACKNOWLEDGED

go-architecture.md:
```
> Note: send-quarterly-email-q4 (April) is missing from the Python beat schedule —
> this appears to be a bug. The Go implementation should add 0 23 2 4 * for consistency.
```

---

## Check F: Error Response Formats

**Status**: ✅ CONFIRMED — Both formats documented with complete status code mapping

- Admin format: `{"result": "error", "message": "..."}` ✅
- V2 format: `{"status_code": N, "errors": [...]}` ✅
- Status code mapping table (404, 400, 403, 409, 500) ✅
- Go error types (`APIError` vs `V2Error`) defined ✅

---

## Check G: Read Replica Routing

**Status**: ⚠️ PATTERN DESCRIBED BUT IMPLEMENTATION DETAILS VAGUE

Pattern clearly described: two `*sql.DB` instances injected (writer + reader). Example mentions `GetServiceByIDWithAPIKeys` as a reader-path function.

**Gap**: Actual function signatures don't indicate which DB (`*sql.DB`) they receive. Go implementors must consult Python `app/dao/*.py` to determine which functions require read replica vs writer.

---

## Check H: Dead/Stub Endpoints

**Status**: ⚠️ ACKNOWLEDGED BUT NOT EXPLICITLY MAPPED

go-architecture.md:
```
retaining only the stub endpoints required to return correct HTTP status codes
for any clients that call them.
```

**Gap**: Does not enumerate which 7 specific endpoints are stubs (5 letter-contact + `/send-pdf-letter` + `/letters/returned`) or what status codes each should return.

**Impact**: Go implementor won't know exactly which endpoints to stub or whether to use 501, 204, or 404.

---

## Check I: Missing Routes from api-surface Validation

**Status**: ❌ These 4 routes not accounted for in go-architecture.md

Same routes as Check A. Handler packages exist in architecture (`handler/newsletter/`, `handler/platform_stats/`) but the specific route list is not detailed, so the 4 undocumented routes will be omitted.

---

## RISK Items for Go Implementors

### 🔴 CRITICAL (5)

1. **4 undocumented routes will be omitted** — newsletter (3) + platform-stats (1) routes exist in Python but are not in spec or architecture

2. **`inbound_sms._content` not encrypted** — Go will store inbound SMS in plaintext; security gap; PII at risk

3. **Letter stub endpoints not mapped** — 7 specific stub endpoints and their required response codes are not listed

4. **Read replica routing details absent** — implementors must research Python DAOs to determine which functions need the reader DB

5. **Newsletter/platform-stats routes incomplete** — handler package references exist but route-level detail is missing

### 🟡 MODERATE (1)

6. **Task count "53" is misleading** — Python has 58 actual tasks; Pinpoint is covered but count in introductory text is inaccurate

---

## Summary Table

| Check | Item | Status | Risk |
|---|---|---|---|
| A | Main route groups | ✅ CONFIRMED | — |
| A | 4 undocumented routes | ❌ MISSING | 🔴 |
| B | Worker pool coverage | ✅ CONFIRMED | — |
| B | Q4 quarterly email | ✅ ACKNOWLEDGED | — |
| B | Task count accuracy | ⚠️ UNDERSTATED | 🟡 |
| C | 5 of 6 encrypted columns | ❌ MISSING 1 | 🔴 |
| D | All 6 feature flags | ✅ CONFIRMED | — |
| E | Q4 email prescribed | ✅ CONFIRMED | — |
| F | Both error formats | ✅ CONFIRMED | — |
| G | Read replica pattern | ⚠️ VAGUE | 🔴 |
| H | Letter stubs mapping | ⚠️ NOT EXPLICIT | 🔴 |
| I | Missing routes accounted | ❌ NOT ACCOUNTED | 🔴 |

---

## Recommendations

1. Add `inbound_sms._content` to the encrypted columns list in go-architecture.md
2. Document the 4 missing routes in api-surface.md and go-architecture.md's newsletter/platform-stats handler sections
3. Create a "stub endpoint appendix" listing all 7 stub endpoints with their required response codes
4. Annotate repository function signatures with `// uses reader DB` comments
5. Update task count from "53" to "54 active tasks" (53 documented + process-pinpoint-result)
