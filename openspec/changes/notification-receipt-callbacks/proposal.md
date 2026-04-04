## Why

Delivering a notification generates a receipt (delivery confirmation or bounce) from the provider. This change implements the goroutine workers that process inbound SES, SNS, and Pinpoint delivery receipts, updating notification statuses and triggering service webhooks. Includes the missing `process-pinpoint-result` task (validation finding C3).

## What Changes

- `internal/worker/receipts/` — `process_ses_result.go`, `process_sns_result.go`, `process_pinpoint_result.go`: consume `delivery-receipts` queue, parse SNS envelope, update notification status, emit complaint records
- `internal/worker/callbacks/` — `send_delivery_status.go`, `send_complaint.go`: consume `service-callbacks` and `service-callbacks-retry` queues, make outbound HTTPS webhook calls to service-registered callback URLs with HMAC-signed bearer tokens
- HTTP POST callback payload format: `{id, reference, to, status, created_at, completed_at, sent_at, inbound_number, cost_in_millicents_per_sms}`
- Retry logic for callback failures: non-4xx errors trigger re-queue to `service-callbacks-retry` (up to 5 retries, 300 s back-off)
- `process-pinpoint-result` task added (**C3 fix** from validation findings): processes Pinpoint SMS V2 delivery receipts on the `delivery-receipts` queue

## Capabilities

### New Capabilities

- `receipt-processing-workers`: goroutine pools for processing SES, SNS, and Pinpoint delivery receipts; update notification status; emit complaints
- `service-callback-workers`: goroutine pools for sending delivery-status and complaint webhooks to service-registered callback endpoints

### Modified Capabilities

## Non-goals

- Inbound SMS receipt (completely separate from delivery receipts — covered in `inbound-sms`)
- Provider selection logic — covered in `provider-integration`
- Bounce rate tracking — covered in `billing-tracking`

## Impact

- Requires `notification-delivery-pipeline` (delivery workers must exist first), `data-model-migrations` (notifications and complaints repositories)
- **C3 fix**: `process-pinpoint-result` is present in production Python code but was missing from the intermediate async-tasks.md spec; included here
