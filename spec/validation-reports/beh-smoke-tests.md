# Validation Report: behavioral-spec/smoke-tests.md
Date: 2026-04-04

## Summary
- **Flows validated**: 4 (admin one-off, admin CSV, API one-off, API bulk)
- **CONFIRMED**: All
- **DISCREPANCIES**: 0
- **UNCOVERED**: 5 edge cases
- **Risk items**: 5

## Verdict
**FULLY CONFIRMED** — Specification aligns with smoke test implementation.

---

## Confirmed

- Config defaults: `API_HOST_NAME=localhost:6011`, `POLL_TIMEOUT=120s`, `AWS_REGION=ca-central-1`, `JOB_SIZE=2` ✅
- OIDC assumed if `AWS_ACCESS_KEY_ID`/`SECRET` absent ✅
- Flow 1 (admin one-off): POST /service/{id}/send-notification (JWT), GET polled ✅
- Flow 2 (admin CSV): S3 upload (AES-256), copy-to-self metadata, POST /service/{id}/job ✅
- Flow 3 (API one-off): POST /v2/notifications/{email|sms} (ApiKey-v1), GET polled ✅
- Flow 4 (API bulk): POST /v2/notifications/bulk (ApiKey-v1), GET /service/{id}/job polled ✅
- Polling success: `status=="delivered"` OR (local AND `"fail"` not in status) ✅
- Polling failure: immediate on `permanent-failure` OR timeout after `POLL_TIMEOUT` ✅
- SMS + attachment: rejected with error + early return ✅
- Local mode (`IS_LOCAL=True` if `"localhost"` in host): skips strict status check ✅

---

## Discrepancies
None.

---

## Uncovered Contracts

1. Delivery polling under network failure (500 or timeout) — assumed framework raises exception
2. CSV row count max — no test for overflow
3. Attachment file fetch retry if document-download temporarily unavailable
4. S3 metadata copy validation from job processor's perspective
5. `send_many.py` behavior under concurrent load

---

## RISK Items for Go Implementors

1. **Polling timeout hard-coded at 120s**: Long queues in production can cause smoke test false-negatives. Make `POLL_TIMEOUT` configurable per flow.

2. **S3 AES-256 metadata contract**: CSV metadata in S3 custom headers. Go job processor must read S3 metadata correctly or job creation silently fails. Add explicit integration test.

3. **Local mode false-pass risk**: Local mode accepts any non-failure status — a stuck `"pending"` notification passes locally but fails in production. Log a warning when local mode accepts non-delivered status.

4. **JWT expiration during long poll**: Admin JWT may expire if polling takes >120s in slow environments. Re-request JWT for each poll attempt or use extended TTL.

5. **Personalisation incrementing format**: Tests use `"{prefix} {n}"`. If Go changes format, smoke tests still pass but personalisation substitution may not be tested correctly.
