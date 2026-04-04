## Source Files
- spec/behavioral-spec/smoke-tests.md
- openspec/changes/smoke-test-suite/proposal.md

---

## Overview
Smoke tests verify a live Notify API instance (staging) end-to-end across all delivery paths. They make real HTTP calls, push real notifications through the system, and poll until delivery is confirmed or a timeout is reached. Tests are gated by `//go:build smoke` so they never run in normal CI.

---

## Python Reference: 4 Delivery Paths

The Python `tests_smoke/` suite covers:

| Path | Auth | Delivery granularity |
|------|------|----------------------|
| Admin one-off | JWT (`notify-admin`) | 1 notification at a time |
| Admin CSV | JWT + S3 (AES-256) | batch job via `/service/{id}/job` |
| API one-off | `ApiKey-v1` | `POST /v2/notifications/{type}` |
| API bulk (v2) | `ApiKey-v1` | `POST /v2/notifications/bulk` |

**The Go smoke suite scopes to the two v2 API paths** (paths 3 and 4), further split by SMS process type:

| Go test function | Python equivalent | Notes |
|------------------|-------------------|-------|
| `TestSmokeEmail` | API one-off (email) | `POST /v2/notifications/email` |
| `TestSmokeSMSNormal` | API one-off (SMS, normal template) | Normal process type queue |
| `TestSmokeSMSBulk` | API one-off (SMS, bulk template) | Bulk process type queue |
| `TestSmokeSMSPriority` | API one-off (SMS, priority template) | Priority process type queue |

---

## Python Common Infrastructure (reference)

### Polling
- `single_succeeded(uri, use_jwt)`: polls notification status endpoint once per second for up to `POLL_TIMEOUT` seconds; returns `True` on `"delivered"`, `False` on `"permanent-failure"` or timeout
- Poll URL is the `uri` field from the POST response body

### Config env vars (Python reference)
`SMOKE_API_HOST_NAME`, `SMOKE_ADMIN_CLIENT_SECRET`, `SMOKE_POLL_TIMEOUT` (default 120s), `SMOKE_SERVICE_ID` (required), `SMOKE_USER_ID` (required for admin paths), `SMOKE_EMAIL_TO`, `SMOKE_SMS_TO`, `SMOKE_EMAIL_TEMPLATE_ID`, `SMOKE_SMS_TEMPLATE_ID`, `SMOKE_API_KEY` (required for API paths), `SMOKE_JOB_SIZE` (default 2)

---

## Go Implementation Requirements

### Required environment variables

| Env var | Purpose |
|---------|---------|
| `NOTIFY_API_URL` | Base URL of the target API instance |
| `SMOKE_TEST_API_KEY` | API key for v2 endpoint auth (`ApiKey-v1 {key}`) |
| `SMOKE_SERVICE_ID` | Service ID owning the test templates |
| `SMOKE_EMAIL_ADDRESS` | Recipient email address |
| `SMOKE_SMS_NUMBER` | Recipient phone number |
| `SMOKE_TEMPLATE_EMAIL_ID` | Email template UUID |
| `SMOKE_TEMPLATE_SMS_NORMAL_ID` | SMS template (normal process type) |
| `SMOKE_TEMPLATE_SMS_BULK_ID` | SMS template (bulk process type) |
| `SMOKE_TEMPLATE_SMS_PRIORITY_ID` | SMS template (priority process type) |

### `TestMain` contract
- Reads all required env vars at startup
- If any env var is missing: logs `"SMOKE_TEST_API_KEY not set — skipping smoke tests"` (naming the missing var) and calls `m.Run()` after marking skip; exits 0
- If all vars present: proceeds normally via `m.Run()`
- Never calls `os.Exit(1)` when env vars are absent

### `pollUntilTerminal` helper
- Signature: `func pollUntilTerminal(t *testing.T, client HTTPClient, notificationID string, timeout time.Duration) error`
- Polls `GET /v2/notifications/{id}` (with `Authorization: ApiKey-v1 {key}`) every 1 second
- Terminal statuses: `"delivered"`, `"permanent-failure"`, `"technical-failure"`
- On `"delivered"` → return `nil`
- On `"permanent-failure"` or `"technical-failure"` → return error immediately (do not wait for timeout)
- On timeout (elapsed ≥ `timeout`) → return timeout error

### Polling algorithm (ticker-based)
```
ticker := time.NewTicker(1 * time.Second)
deadline := time.Now().Add(timeout)
for range ticker.C {
    resp := GET /v2/notifications/{id}
    switch resp.status {
    case "delivered":          return nil
    case "permanent-failure",
         "technical-failure":  return fmt.Errorf("terminal failure: %s", resp.status)
    }
    if time.Now().After(deadline) {
        return fmt.Errorf("timeout after %s", timeout)
    }
}
```

### Test function flow (each of the 4 tests)
1. Read required env vars (already validated in TestMain)
2. Build POST payload: `{email_address / phone_number, template_id, personalisation: {"var": "smoke test"}}`
3. POST to `/v2/notifications/email` or `/v2/notifications/sms` with `Authorization: ApiKey-v1 {key}`
4. Assert response HTTP 201
5. Extract notification ID from response body (`response.id`)
6. Call `pollUntilTerminal(t, client, id, 60*time.Second)`
7. Assert no error

### Makefile target
```make
smoke:
    # Required env vars: NOTIFY_API_URL SMOKE_TEST_API_KEY SMOKE_SERVICE_ID
    # SMOKE_EMAIL_ADDRESS SMOKE_SMS_NUMBER SMOKE_TEMPLATE_EMAIL_ID
    # SMOKE_TEMPLATE_SMS_NORMAL_ID SMOKE_TEMPLATE_SMS_BULK_ID SMOKE_TEMPLATE_SMS_PRIORITY_ID
    go test -tags smoke -v -timeout 300s ./tests/smoke/...
```
- `NOTIFY_API_URL` configurable via env; defaults to `http://localhost:6011` if not set
- The outer 300s timeout provides a hard ceiling encompassing all 4 × 60s polls plus HTTP overhead

### Build tag
Every file under `tests/smoke/` carries `//go:build smoke` to ensure `go test ./...` never picks them up. Tests are explicitly opt-in via `-tags smoke`.

---

## Business Rules
- Tests must NEVER run in regular CI (build tag enforces this)
- Missing env vars must produce a skip (exit 0), not a failure (exit 1)
- Each test is independent; failure of one must not skip the others
- Polling interval: 1 second (matches Python reference)
- Hard timeout per notification: 60 seconds
- Hard timeout for the full test run: 300 seconds (`-timeout 300s`)
- The 3 SMS templates must each have a distinct process type pre-provisioned in the target environment so that all 3 worker queues are independently exercised
