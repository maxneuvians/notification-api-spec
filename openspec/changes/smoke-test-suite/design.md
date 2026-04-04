## Context
Smoke tests verify a live Notify API instance (staging) end-to-end across all delivery paths. They use real API calls, push real notifications through the system, and poll until delivery is confirmed. Tests are gated by the `//go:build smoke` tag so they never run in normal CI. They are modelled after the Python `tests_smoke/` suite but scoped to the 4 v2 API delivery paths.

## Goals
- Verify all 4 v2 delivery paths (email, SMS normal, SMS bulk, SMS priority) against a live instance
- Skip cleanly (not fail) when env vars are absent, so CI is never accidentally broken
- Provide a single `make smoke` entry point with all env var requirements documented

## Non-Goals
- Admin one-off or admin CSV delivery paths (internal JWT auth — out of scope for this change)
- File attachment variants (`ATTACHED` / `LINK`) — out of initial scope
- Load testing or performance benchmarks
- Automated CI smoke runs against production (manual execution only)

## Decisions

### 60 s polling timeout with 1 s interval
Each smoke test polls `GET /v2/notifications/{id}` every second for up to 60 seconds waiting for a terminal status (`delivered`, `permanent-failure`, `technical-failure`). Timeout exits with an error. The outer `go test -timeout 300s` provides a hard ceiling for the entire suite run.

### Polling algorithm: ticker-based with early exit on failure
```
ticker := time.NewTicker(1 * time.Second)
deadline := time.Now().Add(timeout)
for range ticker.C {
    status := GET /v2/notifications/{id}
    if status == "delivered"                         { return nil }
    if status in {"permanent-failure",
                  "technical-failure"}               { return error }
    if time.Now().After(deadline)                    { return timeout error }
}
```
Early exit on terminal failure statuses avoids waiting the full 60 s when a notification has definitively failed.

### Environment-variable driven
All credentials and targets sourced from env vars: `NOTIFY_API_URL`, `SMOKE_TEST_API_KEY`, `SMOKE_SERVICE_ID`, `SMOKE_EMAIL_ADDRESS`, `SMOKE_SMS_NUMBER`, `SMOKE_TEMPLATE_EMAIL_ID`, `SMOKE_TEMPLATE_SMS_NORMAL_ID`, `SMOKE_TEMPLATE_SMS_BULK_ID`, `SMOKE_TEMPLATE_SMS_PRIORITY_ID`.

### TestMain skips (not fails) on missing env vars
If any required env var is absent, `TestMain` calls `m.Run()` with all tests pre-marked as skipped (using a `testing.Short()` guard or equivalent) and exits 0. This prevents CI failures on branches that don't have smoke-test credentials configured. The skip message names the specific missing variable.

### Build tag: `//go:build smoke`
Every file under `tests/smoke/` carries `//go:build smoke` to ensure `go test ./...` never picks up the smoke tests. They are explicitly opt-in via `-tags smoke`.

### Process types map to separate templates
The 3 SMS paths use distinct pre-provisioned templates each with a different process type configuration. This ensures all 3 worker queues (normal, bulk, priority) are independently exercised.

## Risks
- Smoke tests depend on external provider availability (SES, Pinpoint); `permanent-failure` indicates a provider issue, not a code bug
- Template IDs must be pre-provisioned in the target environment; missing template → 400 from the API before polling even begins
- Polling at 1 s with 60 s timeout: if staging delivery takes longer than 60 s under load, tests will false-fail; `SMOKE_POLL_TIMEOUT` could be made configurable in a future iteration
