## Why

External service teams need to send SMS messages via the GC Notify API. This change implements the SMS-only send endpoints, which differ from email in their validation (phone number format, international permission, character limits), billable unit counting (SMS fragments), and queue selection (throttled SMS path).

## What Changes

- `internal/handler/v2/notifications/sms.go` — `POST /v2/notifications/sms`
- `internal/handler/notifications/sms.go` — `POST /notifications/sms` (legacy v0)
- `internal/service/notifications/sms.go` — phone normalisation, international permission check, character limit (≤ 612), SMS sender resolution, simulated-number short-circuit, billable unit calculation (`FF_USE_BILLABLE_UNITS`), SQS/Redis queue dispatch
- `pkg/smsutil/` — `FragmentCount(message string) int` — GSM-7 encoding and fragment counting (replaces `app/sms_fragment_utils.py`)
- Queue selection: `test` key / research-mode → `research-mode-tasks`; throttled number (+14383898585) → `send-throttled-sms-tasks`; priority template → `send-sms-high`; bulk template → `send-sms-low`; default → `send-sms-medium`
- Simulated SMS numbers: 6132532222, +16132532222, +16132532223 — return 201 without persisting or publishing

## Capabilities

### New Capabilities

- `send-sms`: POST /v2/notifications/sms and POST /notifications/sms — create, validate, persist, and enqueue SMS notifications with fragment-based billing and queue routing
- `sms-fragment-counting`: GSM-7 and UCS-2 character encoding detection and per-message fragment count calculation

### Modified Capabilities

## Non-goals

- Email send endpoints (covered in `send-email-notifications`)
- GET notification endpoints (covered in `send-email-notifications`)
- Bulk CSV send — covered in `bulk-send-jobs`
- Delivery workers — covered in `notification-delivery-pipeline`
- Inbound SMS receipt — covered in `inbound-sms`

## Impact

- Requires `authentication-middleware`, `data-model-migrations`, `template-management`
- `pkg/smsutil.FragmentCount` stub from `go-project-setup` is replaced here with the real implementation
- Annual SMS limit checks directly affect billing; must correctly count fragments when `FF_USE_BILLABLE_UNITS` is enabled
