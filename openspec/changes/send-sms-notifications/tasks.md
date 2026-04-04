## 1. SMS Fragment Counting Package

- [ ] 1.1 Implement `pkg/smsutil/fragment.go` — `IsGSM7(r rune) bool`: build the GSM-7 basic charset byte-for-byte from the spec (128 chars) and extension table (10 chars: `{`, `}`, `[`, `]`, `\`, `~`, `|`, `€`, `^`, backtick); return true if rune is in either table
- [ ] 1.2 Implement `FragmentCount(message string) int`: iterate runes; detect encoding (any non-GSM7 rune → UCS-2); accumulate effective character count (extension chars count as 2 in GSM-7 mode); apply limits: GSM-7 single ≤ 160 → 1, multi: ceil(len/153); UCS-2 single ≤ 70 → 1, multi: ceil(len/67); empty string → 0
- [ ] 1.3 Write table-driven unit tests: 160-char ASCII = 1, 161-char ASCII = 2, 306-char ASCII = 2, 307-char ASCII = 3; 70-char UCS-2 = 1, 71-char UCS-2 = 2, 134-char UCS-2 = 2, 135-char UCS-2 = 3; 80 `{` chars = 1, 81 `{` chars = 2; single emoji = 1; empty = 0; all 10 extension chars individually correct
- [ ] 1.4 Write cross-language fixture tests: generate 20+ fixture pairs `(message, expected_count)` from the Python `notifications_utils` library; embed as a table in `pkg/smsutil/fragment_fixture_test.go` and verify all match

## 2. Phone Validation & Normalisation

- [ ] 2.1 Implement `internal/service/notifications/sms_phone.go` — `validateAndNormalisePhone(raw string) (e164 string, isInternational bool, err error)` using `github.com/nyaruka/phonenumbers`; default country code CA; detect non-CA country codes for international flag; return typed ValidationError on invalid format
- [ ] 2.2 Write unit tests: local Canadian number `"6132532222"` normalises to `"+16132532222"` (not international); UK number flagged as international; E.164 `"+16132532222"` accepted as-is; integer input caught before this layer returns type error; empty string returns validation error; `"not-a-number"` returns validation error

## 3. SMS Send Service Layer

- [ ] 3.1 Implement `internal/service/notifications/sms.go` — `SendSMS(ctx, serviceID, req SMSRequest) (*Notification, error)`: check service suspended (403), validate phone type (string check), validate + normalise phone (libphonenumber), check international permission, check service SMS permission, resolve SMS sender, validate personalisation (object type, placeholder completeness), render template body, check character limit ≤ 612 (including prefix if `prefix_sms=true`), check simulated numbers (early return 201), calculate `billable_units` per `FF_USE_BILLABLE_UNITS`, enforce limits (rate → annual → daily), check trial/team restrictions, persist notification, select queue and publish
- [ ] 3.2 Implement `resolveSMSSender(ctx, serviceID, smsSenderID *uuid.UUID) (senderRecord ServiceSMSSender, err error)` — if ID provided: validate not archived; else: fetch service default; write unit tests: valid override used, archived override returns 400, no override uses default
- [ ] 3.3 Implement `checkTrialRestrictions(ctx, service *Service, phone string, keyType string) error` — check recipient is team member or safelisted for normal/team keys on restricted services or for team keys on any service; write unit tests: team member passes, safelisted passes, unknown recipient fails for normal key on restricted service, team key fails for unknown recipient on any service

## 4. Queue Selection Logic

- [ ] 4.1 Implement `selectSMSQueue(keyType string, researchMode bool, replyTo string, processType string) string` — apply five-rule precedence: test key or research mode → `research-mode-tasks`; `replyTo == "+14383898585"` → `send-throttled-sms-tasks`; `processType == "priority"` → `send-sms-high`; `processType == "bulk"` → `send-sms-low`; default → `send-sms-medium`
- [ ] 4.2 Write table-driven unit tests for all five branches including: test key → research-mode; research_mode=true with normal key → research-mode; throttled sender with priority template → throttled (NOT high); priority template not throttled → high; bulk template → low; default → medium

## 5. Billable Units & Limit Checks

- [ ] 5.1 Implement `calcBillableUnits(renderedBody string, featureFlag bool) int` — when flag true: return `FragmentCount(renderedBody)`; when false: return 0; write unit tests: flag on returns fragment count, flag off always returns 0
- [ ] 5.2 Implement `CheckSMSRateLimit`, `CheckAnnualSMSLimit(ctx, serviceID, billableUnits int)`, `CheckDailySMSLimit(ctx, serviceID, billableUnits int)` in `internal/service/notifications/limits.go`; for annual/daily: use correct Redis key based on `FF_USE_BILLABLE_UNITS`; write unit tests: below limit passes, exceeded 429; test key bypasses all three; simulated number caller bypasses all three; billable_units=3 decrements counter by 3

## 6. v2 Handler

- [ ] 6.1 Implement `internal/handler/v2/notifications/sms.go` — POST /v2/notifications/sms: parse request, validate field types and reject additional properties, call service layer, map errors to v2 format, return 201 `{id, reference, content: {body, from_number}, uri, template: {id, version, uri}, scheduled_for}`
- [ ] 6.2 Write httptest integration tests for POST /v2/notifications/sms: valid 201, non-string phone 400, invalid phone 400, international without permission 400, body too long 400, archived template 400, type mismatch 400, SMS template on SMS endpoint type-match 201, team key non-member 400, annual limit 429, daily limit 429, billable-units limit deducts fragment count, simulated number 201 no-DB, test key bypasses limits, throttled sender queue verified, suspended service 403

## 7. Legacy v0 Handler

- [ ] 7.1 Implement `internal/handler/notifications/sms.go` — POST /notifications/sms: request `{to, template, personalisation?}`; call same service layer with schema translation; response `{"data": {"notification": {"id": "<uuid>"}, "body": "<rendered>", "template_version": N}}`; error format `{"result": "error", "message": {...}}`
- [ ] 7.2 Implement path-segment type dispatch in the v0 router: explicit match on `sms`, `email`; "supported but deprecated" match on `letter` → 400 with migration message; catch-all for unknown type → 400 with type-not-supported message; write unit tests for both 400 cases
