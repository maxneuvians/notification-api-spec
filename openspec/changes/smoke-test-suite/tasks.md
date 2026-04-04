## 1. Smoke Test Infrastructure

- [ ] 1.1 Create `tests/smoke/main_test.go` with `//go:build smoke` tag; implement `TestMain` that reads all 9 required env vars (`NOTIFY_API_URL`, `SMOKE_TEST_API_KEY`, `SMOKE_SERVICE_ID`, `SMOKE_EMAIL_ADDRESS`, `SMOKE_SMS_NUMBER`, `SMOKE_TEMPLATE_EMAIL_ID`, `SMOKE_TEMPLATE_SMS_NORMAL_ID`, `SMOKE_TEMPLATE_SMS_BULK_ID`, `SMOKE_TEMPLATE_SMS_PRIORITY_ID`) and calls `m.Run()` after skipping all tests with a named-variable message if any are missing (exits 0)
- [ ] 1.2 Implement `smokeHTTPClient` with `Authorization: ApiKey-v1 {key}` header injection; implement `postNotification(notifType, templateID, recipient string) (string, error)` helper (returns notification ID from 201 response) and `getNotificationStatus(id string) (string, error)` helper; implement `pollUntilTerminal(t *testing.T, id string, timeout time.Duration) error` with 1s ticker, early exit on `permanent-failure`/`technical-failure`, and timeout error return

## 2. Four Delivery Path Tests

- [ ] 2.1 Implement `TestSmokeEmail` in `tests/smoke/email_test.go` (build tag `smoke`) — `POST /v2/notifications/email` with `SMOKE_EMAIL_ADDRESS` + `SMOKE_TEMPLATE_EMAIL_ID`, extract notification ID, call `pollUntilTerminal` with 60 s timeout, assert no error
- [ ] 2.2 Implement `TestSmokeSMSNormal` in `tests/smoke/sms_test.go` — `POST /v2/notifications/sms` with `SMOKE_SMS_NUMBER` + `SMOKE_TEMPLATE_SMS_NORMAL_ID`, poll, assert `delivered`
- [ ] 2.3 Implement `TestSmokeSMSBulk` — `POST /v2/notifications/sms` with `SMOKE_TEMPLATE_SMS_BULK_ID`, poll, assert `delivered`
- [ ] 2.4 Implement `TestSmokeSMSPriority` — `POST /v2/notifications/sms` with `SMOKE_TEMPLATE_SMS_PRIORITY_ID`, poll, assert `delivered`

## 3. Makefile Target

- [ ] 3.1 Add `make smoke` target to `Makefile`: `go test -tags smoke -v -timeout 300s ./tests/smoke/...` with a comment block listing all 9 required env vars and example values; ensure `NOTIFY_API_URL` defaults to `http://localhost:6011` if not set

## 4. Documentation

- [ ] 4.1 Add `tests/smoke/README.md` documenting: purpose, all required env vars with descriptions, how to provision templates in a staging environment (3 SMS templates with distinct process types), and example `make smoke` invocation with exported env vars
