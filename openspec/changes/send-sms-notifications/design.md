## Context

SMS and email differ substantially in validation (phone vs email), encoding (GSM-7/UCS-2 vs UTF-8), billing (fragment counting vs flat rate), and queue routing (five SMS queues vs three email queues). The throttled SMS path introduces a special route for a specific sender number used for rate-limited delivery.

## Goals / Non-Goals

**Goals:**
- Implement SMS send with phone normalisation, international permission check, fragment counting, queue routing
- Implement `pkg/smsutil.FragmentCount` replacing the Python `sms_fragment_utils.py`
- Enforce `FF_USE_BILLABLE_UNITS` feature flag for limit check billing unit calculation
- Handle all five queue routing cases including the throttled path

**Non-Goals:**
- GET notification endpoints (covered in `send-email-notifications`)
- Inbound SMS (covered in `inbound-sms`)
- Delivery workers (covered in `notification-delivery-pipeline`)
- Provider selection and failover (covered in `provider-integration`)

## Decisions

### Phone normalisation: libphonenumber
Use `github.com/nyaruka/phonenumbers` (Go port of libphonenumber) for E.164 normalisation and validation. The normalised form is stored in `normalised_to`; the original input (encrypted) is stored in `to`.

### GSM-7 fragment counting
`pkg/smsutil.FragmentCount` must replicate `app/sms_fragment_utils.py` exactly:
- Single-part GSM-7: ≤ 160 chars → 1 fragment
- Multi-part GSM-7: ≤ 153 chars per fragment (6-char UDH overhead per part)
- Single-part UCS-2: ≤ 70 chars → 1 fragment
- Multi-part UCS-2: ≤ 67 chars per fragment
- A message is GSM-7 if every character is in the GSM-7 basic charset + extension table; otherwise UCS-2.

### `FF_USE_BILLABLE_UNITS` feature flag
When enabled: `billable_units = FragmentCount(rendered_body)`. Limit checks use `billable_units`.
When disabled: `billable_units = 0` (stored); limit checks always use count `1`.
Test-key sends: `billable_units` is still calculated for storage, but limit functions are never called.

### Queue selection priority
The throttled-sender check (`reply_to_text == "+14383898585"`) precedes the template `process_type` check. A priority template sent from the throttled sender still goes to `send-throttled-sms-tasks`.

### Simulated numbers: early exit before any DB write
Identical to the email simulated address pattern — checked before template lookup, limit checks, and DB write.

### SMS prefix prepended before character limit check
When `service.prefix_sms = true`, `"{service_name}: "` is prepended to the rendered body before the 612-character check and before fragment counting. The v2 validator adds prefix length to the content character count. The prefix is NOT part of the template body; it is applied at send time.

### Research mode: two independent detection paths
A notification routes to `research-mode-tasks` if EITHER `key_type == "test"` (test API key) OR `service.research_mode == true` (service-level flag). Either condition alone is sufficient. This allows services to enter research mode without switching API keys, and allows test keys to always route safely regardless of `research_mode`.

### `to` vs `normalised_to` — dual storage
The original `phone_number` input (encrypted) is stored in `to` exactly as supplied. The E.164-normalised form is stored in `normalised_to` (unencrypted; used for deduplication, search, and delivery). Both written at creation time; neither updated after that.

### v1/v2 error response schema differentiation
Identical rationale to email: v2 uses `{"errors": [...], "status_code": 400}`; v0 uses `{"result": "error", "message": {...}}`. Service layer returns typed errors; each handler translates independently.

### Invalid legacy path segment handling
The v0 router handles `POST /notifications/{type}` with an explicit type switch. Known-but-unsupported types (e.g. `letter`) return HTTP 400 `"letter notification type is not supported, please use the latest version of the client"`. Unknown types (e.g. `apple`) return HTTP 400 `"apple notification type is not supported"`.

## Risks / Trade-offs

- **GSM-7 charset divergence** → The smsutil implementation must be byte-for-byte equivalent to the Python version; a divergence causes fragment count mismatches and therefore billing discrepancies. Write a cross-reference test suite using Python-generated fixture data.
- **International number detection** → libphonenumber may differ from the Python implementation for edge cases. Run the existing Python test fixtures through the Go implementation to catch divergences.
