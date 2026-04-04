## Why

External service teams need to send emails to their users via the GC Notify API. This change implements the email-send endpoints for both the legacy v0 API and the public v2 API, plus the shared GET endpoints for retrieving individual and listed notifications.

## What Changes

- `internal/handler/v2/notifications/` — `POST /v2/notifications/email`, `GET /v2/notifications/{id}`, `GET /v2/notifications`
- `internal/handler/notifications/` — `POST /notifications/email` (legacy), `GET /notifications/{id}` (legacy), `GET /notifications` (legacy)
- `internal/service/notifications/email.go` — email validation, personalisation rendering, document attachment handling, reply-to resolution, simulated-address short-circuit, SQS/Redis queue dispatch
- `internal/service/notifications/limits.go` — per-minute rate check, daily email limit check, annual email limit check
- Email-specific validation: address format, `email_reply_to_id` existence, personalisation size ≤ 51,200 bytes, per-document constraints (size, count ≤ 10, sending_method, base64)
- Simulated email address list: `simulate-delivered@notification.canada.ca` (and -2, -3) — return 201 without persisting or publishing

## Capabilities

### New Capabilities

- `send-email`: POST /v2/notifications/email and POST /notifications/email — create, validate, persist, and enqueue email notifications
- `get-notifications`: GET /v2/notifications/{id}, GET /notifications/{id}, GET /v2/notifications, GET /notifications — retrieve single or listed notifications with pagination and filtering

### Modified Capabilities

## Non-goals

- SMS send endpoints (covered in `send-sms-notifications`)
- Bulk CSV send (`POST /v2/notifications/bulk`) — covered in `bulk-send-jobs`
- Delivery workers (Celery → goroutine replacement) — covered in `notification-delivery-pipeline`
- Email delivery via AWS SES — covered in `notification-delivery-pipeline`
- Template rendering beyond variable substitution (letter templates, HTML generation) — not in scope

## Impact

- Requires `authentication-middleware` (service auth on all routes), `data-model-migrations` (notifications + templates + services repository), `template-management` (template lookup)
- Establishes the request/response contract used by all external GC Notify client SDKs
