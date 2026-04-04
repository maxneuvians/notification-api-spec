## Source Files

- `spec/behavioral-spec/notifications.md` — SMS sections (POST /notifications/sms, POST /v2/notifications/sms, queue rules, simulated numbers, trial restrictions, billable units, DAO contracts)
- `spec/business-rules/notifications.md` — rate limiting, daily limits (standard and billable-unit modes), annual limits, SMS fragment counting, personalisation validation, status transitions

## Requirements

### R1: POST /v2/notifications/sms — Create and Enqueue

- Required fields: `phone_number` (string), `template_id` (UUID string)
- Optional fields: `reference` (string), `personalisation` (object), `sms_sender_id` (UUID), `scheduled_for` (ISO8601 string)
- Response 201: `{id, reference, content: {body, from_number}, uri, template: {id, version, uri}, scheduled_for}`
- Notification persisted; signed blob published to Redis queue determined by key type, sender, and template `process_type`
- v2 error format: `{"errors": [{"error": "ValidationError", "message": "..."}], "status_code": 400}`
- Additional unknown properties (e.g. `email_reply_to_id` on SMS) → 400 `"Additional properties are not allowed ({field} was unexpected)"`

### R2: POST /notifications/sms — Legacy v0 Create

- Required fields: `to` (phone string), `template` (UUID)
- Optional fields: `personalisation` (object)
- Response 201: `{"data": {"notification": {"id": "<uuid>"}, "body": "<rendered>", "template_version": N}}`
- `to` and `template` both required; missing either → 400 `"Missing data for required field."` per field
- Same service layer as v2; different request/response schema
- v0 error format: `{"result": "error", "message": {...}}`
- Unknown path segment `/notifications/letter` → 400 `"letter notification type is not supported, please use the latest version of the client"`
- Unknown path segment `/notifications/apple` → 400 `"apple notification type is not supported"`

### R3: Phone Number Validation and Normalisation

- `phone_number` must be a string type (not int, null, array) → ValidationError `"phone_number {val} is not of type string"`
- Must be parseable as an international phone number using libphonenumber; default region: Canada (CA) → ValidationError `"phone_number Not a valid international number"` (v2) / `"Invalid phone number: Not a valid international number"` (v0)
- E.164 normalised form stored in `normalised_to` (e.g. `+16132532222`)
- Original encrypted input stored verbatim in `to`
- `6132532222` (no country code) → normalised to `+16132532222` using CA as default country

### R4: International SMS Permission Check

- Phone number with country code ≠ +1 requires service `international_sms` permission
- Without permission → 400 BadRequestError `"Cannot send to international mobile numbers"`
- With permission → proceed normally

### R5: SMS Character Limit

- Rendered SMS body (after personalisation substitution) must be ≤ 612 characters
- When `service.prefix_sms = true`: prepend `"{service_name}: "` to body before character check (v2 validators add prefix length to count)
- Exceeded → 400 `"Content has a character count greater than the limit of 612"`

### R6: Template Validation (SMS)

- `template_id` must be a valid UUID → ValidationError `"template_id is not a valid UUID"`
- Template must exist and belong to the authenticated service → 400 BadRequestError
- Template type must be `sms`; mismatch → BadRequestError `"{sms} template is not suitable for {email} notification"`
- Template must not be archived → BadRequestError `"Template {id} has been deleted"`
- Required personalisation placeholders must be present → BadRequestError `{"template": ["Missing personalisation: <Name>"]}`
- Extra keys silently ignored
- Rendered body must not be blank/whitespace → BadRequestError

### R7: SMS Sender Override

- If `sms_sender_id` provided: must reference a non-archived `service_sms_senders` record for the service
  - Archived or not found → 400 BadRequestError `"sms_sender_id {id} does not exist in database for service id {id}"`
  - `reply_to_text` value stored on `notifications` row at creation time
- If not provided: service's default SMS sender fetched; `reply_to_text` set from default sender's number
- Stored `reply_to_text` immutable; later sender changes do NOT retroactively update stored notifications

### R8: SMS Queue Routing — 5 Queues, Strict Precedence

Evaluation stops at first match (order is critical):

1. `key_type == "test"` OR `service.research_mode == true` → `research-mode-tasks`
2. `reply_to_text == "+14383898585"` (throttled sender) → `send-throttled-sms-tasks` (task: `deliver_throttled_sms`)
3. template `process_type == "priority"` → `send-sms-high` (`SEND_SMS_HIGH`)
4. template `process_type == "bulk"` → `send-sms-low` (`SEND_SMS_LOW`)
5. default → `send-sms-medium` (`SEND_SMS_MEDIUM`)

Note: throttled sender (rule 2) defeats priority template (rule 3). A priority-process-type template sent via the throttled sender still goes to `send-throttled-sms-tasks`.

### R9: Simulated SMS Numbers

- Three simulated numbers bypass all processing: `6132532222`, `+16132532222`, `+16132532223`
- Checked BEFORE template lookup, limit checks, DB write
- Return HTTP 201; NO `notifications` row created; NO queue publish; NO limit function calls

### R10: Trial Mode and Team-Key Restrictions

- `test` key: bypasses ALL restrictions and limit checks; routed to `research-mode-tasks`
- `normal` key on restricted (trial) service: recipient must be team member or safelisted → 400 BadRequestError `"Can't send to this recipient when service is in trial mode – see <docs>"`
- `team` key: recipient must be team member or safelisted regardless of service mode → 400 BadRequestError `"Can't send to this recipient using a team-only API key (service {id}) - see <docs>"`
- Safelist members (both normal and team keys): allowed for any service mode

### R11: Service SMS Permission Check

- Authenticated service must have `sms` permission → 400 BadRequestError `"Service is not allowed to send text messages"`

### R12: Suspended Service

- If service `active = false` → HTTP 403 before any other processing

### R13: Billable Units and `FF_USE_BILLABLE_UNITS`

- When `FF_USE_BILLABLE_UNITS = true`:
  - `billable_units = FragmentCount(rendered_body)` stored on `notifications` row
  - Daily SMS limit check called with `billable_units` (not 1)
  - Redis counter key: `billable_units_sms_daily_count:{service_id}`
  - Annual limit check uses `TOTAL_SMS_BILLABLE_UNITS_FISCAL_YEAR_TO_YESTERDAY`
- When `FF_USE_BILLABLE_UNITS = false`:
  - `billable_units` may remain 0 / populated later by delivery task
  - Limit checks always use count `1`
  - Redis counter key: `sms_daily_count:{service_id}`
- `test` key: `billable_units` still calculated and stored; limit functions NOT called
- Simulated numbers: limit functions NOT called; `billable_units` not calculated

### R14: Rate and Limit Enforcement (SMS)

- Same enforcement order as email: rate → annual → daily
- **Rate limit**: identical mechanism to email (Redis sorted-set, 60 s window, `service.rate_limit`)
- **Annual SMS limit** (standard): Redis hash field `TOTAL_SMS_FISCAL_YEAR_TO_YESTERDAY`; threshold `total + 1 > service.sms_annual_limit`
- **Annual SMS limit** (billable-unit): hash field `TOTAL_SMS_BILLABLE_UNITS_FISCAL_YEAR_TO_YESTERDAY`; threshold `total + billable_units > service.sms_annual_limit`
- **Daily SMS limit** (standard): Redis key `sms_daily_count:{service_id}` (2-hour TTL); threshold `sms_sent_today + 1 > service.sms_daily_limit`
- **Daily SMS limit** (billable-unit): Redis key `billable_units_sms_daily_count:{service_id}`; threshold `sms_sent_today + billable_units > service.sms_daily_limit`
- Warning emails: same 80%/100% mechanism as email; uses SMS-specific Redis dedup keys

### R15: `pkg/smsutil.FragmentCount` — GSM-7 / UCS-2 Encoding

- `FragmentCount(message string) int` in `pkg/smsutil/fragment.go`
- Classify: GSM-7 if every rune is in GSM-7 basic charset OR extension table; otherwise UCS-2
- **GSM-7 basic charset**: 128 characters (standard 7-bit GSM set)
- **GSM-7 extension table** (each counts as 2 chars): `{`, `}`, `[`, `]`, `\`, `~`, `|`, `€`, `^`, `` ` ``
- **GSM-7 fragment limits**: single-part ≤ 160 chars → 1 fragment; multi-part: `ceil(len / 153)` fragments
- **UCS-2 fragment limits**: single-part ≤ 70 chars → 1 fragment; multi-part: `ceil(len / 67)` fragments
- `IsGSM7(r rune) bool` helper function exported for testing and fixture validation
- Implementation must match Python `notifications_utils` fragment count exactly (byte-for-byte parity)

## Error Conditions

| Condition | HTTP Status | Error |
|---|---|---|
| Missing `phone_number` | 400 | `ValidationError: phone_number is a required property` |
| Missing `template_id` | 400 | `ValidationError: template_id is a required property` |
| `phone_number` not a string | 400 | ValidationError `"phone_number {val} is not of type string"` |
| Invalid phone number format | 400 | ValidationError `"Not a valid international number"` |
| `template_id` invalid UUID | 400 | `"template_id is not a valid UUID"` |
| Template not found | 400 | BadRequestError |
| Template type mismatch | 400 | BadRequestError |
| Archived template | 400 | BadRequestError |
| Missing personalisation | 400 | BadRequestError |
| SMS body > 612 chars | 400 | BadRequestError |
| `sms_sender_id` archived | 400 | BadRequestError |
| International number, no permission | 400 | BadRequestError |
| Trial mode, non-team recipient | 400 | BadRequestError |
| Team key, non-member recipient | 400 | BadRequestError |
| Service lacks SMS permission | 400 | `BadRequestError: Service is not allowed to send text messages` |
| Rate limit | 429 | RateLimitError |
| Annual limit (trial) | 429 | `TrialServiceRequestExceedsSMSAnnualLimitError` |
| Annual limit (live) | 429 | `LiveServiceRequestExceedsSMSAnnualLimitError` |
| Daily limit (trial) | 429 | `TrialServiceTooManySMSRequestsError` |
| Daily limit (live) | 429 | `LiveServiceTooManySMSRequestsError` |
| Suspended service | 403 | — |
| No auth token | 401 | AuthError |

## Business Rules

- Throttled sender check (rule 2 in queue routing) MUST precede template `process_type` check (rule 3) — this is the critical Python compliance requirement
- `to` (encrypted, original) and `normalised_to` (E.164) stored separately; normalised form used for deduplication and recipient search
- `FF_USE_BILLABLE_UNITS`: single feature flag controls both `billable_units` storage and limit-check semantics; Go implementation reads from config
- Fragment count computed on rendered body (after personalisation substitution), not on template body
- Character limit check also uses rendered body (after personalisation)
- Research mode: triggered by `key_type == "test"` OR `service.research_mode == true`; either condition alone is sufficient to route to `research-mode-tasks`
- Extended GSM-7 chars (e.g. `€`) count as 2 characters in fragment calculation (escape + char in UDH scheme)
- International SMS delivery receipt skip: if `INTERNATIONAL_BILLING_RATES[phone_prefix].dlr != "yes"`, status updates via delivery receipt are silently ignored for that notification
- GSM-7 fragment parity with Python is a hard requirement; test with cross-generated Python fixture data
