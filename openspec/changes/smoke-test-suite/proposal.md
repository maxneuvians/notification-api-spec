## Why

After all domain changes are implemented, the system needs end-to-end verification across all 4 delivery paths (email, SMS Normal, SMS bulk/low, SMS priority/high). This change implements the smoke test suite that exercises each path against a live instance, polling for delivery confirmation.

## What Changes

- `tests/smoke/` — smoke test suite using `go test -tags smoke` build tag; 4 test functions each sending a real notification via the v2 API and polling `GET /v2/notifications/{id}` until `status` reaches a terminal state or timeout (default 60 s)
- Environment-variable-driven: test API key, service ID, email address, phone numbers sourced from env vars
- Makefile target `make smoke` that runs the suite pointing at a configurable `NOTIFY_API_URL`

## Capabilities

### New Capabilities

- `smoke-test-suite`: Executable smoke tests for all 4 delivery paths (email, SMS normal, SMS bulk, SMS priority) with polling, configurable timeout, and environment variable config

### Modified Capabilities

## Non-goals

- Load testing or performance benchmarks
- Testing of admin-only endpoints
- Automated CI smoke runs against production (manual execution only)

## Impact

- Requires all preceding changes to be implemented
- References `behavioral-spec/smoke-tests.md` for the 4 delivery path definitions and expected terminal statuses
