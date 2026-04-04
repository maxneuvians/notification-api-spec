## Requirements

### Requirement: Email delivery smoke test
`TestSmokeEmail` SHALL send a `POST /v2/notifications/email` using a test API key, then poll `GET /v2/notifications/{id}` every second until status is `delivered` or timeout (60 s).

#### Scenario: Email reaches delivered status within 60 seconds
- **WHEN** the smoke test sends an email notification with `Authorization: ApiKey-v1 {SMOKE_TEST_API_KEY}`
- **THEN** the POST returns HTTP 201 and the notification status reaches `"delivered"` before the 60-second timeout

#### Scenario: Email notification POST returns 201
- **WHEN** `TestSmokeEmail` sends `POST /v2/notifications/email` with valid template_id and email_address
- **THEN** the API returns HTTP 201 with a JSON body containing a notification `id`

---

### Requirement: SMS normal delivery smoke test
`TestSmokeSMSNormal` SHALL send `POST /v2/notifications/sms` with a normal-process-type template.

#### Scenario: SMS normal reaches delivered within 60 seconds
- **WHEN** the smoke test sends an SMS with `SMOKE_TEMPLATE_SMS_NORMAL_ID`
- **THEN** the notification status reaches `"delivered"` within 60 seconds

---

### Requirement: SMS bulk delivery smoke test
`TestSmokeSMSBulk` SHALL send `POST /v2/notifications/sms` with a bulk-process-type template.

#### Scenario: SMS bulk delivers within 60 seconds
- **WHEN** the smoke test sends an SMS with `SMOKE_TEMPLATE_SMS_BULK_ID`
- **THEN** status reaches `"delivered"` within 60 seconds

---

### Requirement: SMS priority delivery smoke test
`TestSmokeSMSPriority` SHALL send `POST /v2/notifications/sms` with a priority-process-type template.

#### Scenario: SMS priority delivers within 60 seconds
- **WHEN** the smoke test sends an SMS with `SMOKE_TEMPLATE_SMS_PRIORITY_ID`
- **THEN** status reaches `"delivered"` within 60 seconds

---

### Requirement: Polling exits early on terminal failure status
`pollUntilTerminal` SHALL return an error immediately upon receiving `permanent-failure` or `technical-failure` without waiting for the full timeout.

#### Scenario: Permanent failure causes immediate poll exit
- **WHEN** `GET /v2/notifications/{id}` returns `status = "permanent-failure"`
- **THEN** `pollUntilTerminal` returns an error immediately (does not wait the remaining timeout)

#### Scenario: Technical failure causes immediate poll exit
- **WHEN** `GET /v2/notifications/{id}` returns `status = "technical-failure"`
- **THEN** `pollUntilTerminal` returns an error immediately

#### Scenario: Timeout error returned after 60 seconds without terminal status
- **WHEN** 60 seconds elapse and the notification has not reached a terminal status
- **THEN** `pollUntilTerminal` returns a timeout error

---

### Requirement: Smoke tests require all env vars; skip gracefully if missing
If any required environment variable is missing, `TestMain` SHALL skip all smoke tests with a descriptive message rather than failing.

#### Scenario: Missing env var skips all tests gracefully
- **WHEN** `SMOKE_TEST_API_KEY` is not set when the test binary starts
- **THEN** all smoke tests are skipped with a message naming the missing variable, and the process exits with code 0

#### Scenario: Missing NOTIFY_API_URL skips tests
- **WHEN** `NOTIFY_API_URL` is not set
- **THEN** all smoke tests are skipped (not failed)

#### Scenario: All env vars present enables test execution
- **WHEN** all 9 required env vars are set
- **THEN** `TestMain` calls `m.Run()` and allows all 4 test functions to execute

---

### Requirement: Build tag prevents smoke tests from running in normal CI
All files under `tests/smoke/` SHALL carry `//go:build smoke` so that `go test ./...` (without `-tags smoke`) never picks them up.

#### Scenario: Smoke tests excluded from normal go test run
- **WHEN** `go test ./...` is executed without `-tags smoke`
- **THEN** no tests from `tests/smoke/` are compiled or run

#### Scenario: Smoke tests included with explicit tag
- **WHEN** `go test -tags smoke ./tests/smoke/...` is executed
- **THEN** all 4 test functions are compiled and eligible to run

---

### Requirement: make smoke target provides single-command entry point
`make smoke` SHALL run the full smoke suite with verbosity, a 300-second hard timeout, and the `smoke` build tag; required env vars SHALL be documented in the Makefile target comment.

#### Scenario: make smoke runs all 4 tests
- **WHEN** all env vars are set and `make smoke` is executed
- **THEN** runs `go test -tags smoke -v -timeout 300s ./tests/smoke/...` and all 4 test functions execute
