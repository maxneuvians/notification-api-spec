## Why

Notifications are persisted and queued by the send endpoints but not yet delivered. This change implements the goroutine-based worker pools that replace Celery's `save-smss`, `save-emails`, `deliver-sms`, `deliver-email`, `deliver-throttled-sms` tasks, establishing the delivery half of the notification pipeline from SQS queue to AWS SES/SNS/Pinpoint.

## What Changes

- `internal/worker/savenotify/` — `save_smss.go`, `save_emails.go`: consume DB-persistence queues, verify HMAC signatures, bulk-insert notifications, enqueue delivery tasks
- `internal/worker/delivery/` — `deliver_sms.go`, `deliver_email.go`, `deliver_throttled_sms.go`: consume send queues, call AWS clients, handle provider errors, retry with process-type-aware back-off
- `internal/client/ses/` — AWS SES client wrapping `send_raw_message`/`send_email`
- `internal/client/sns/` — AWS SNS SMS client
- `internal/client/pinpoint/` — AWS Pinpoint SMS V2 client (`us-west-2`)
- `internal/worker/manager.go` — `WorkerManager` replacing the stub from `go-project-setup`: starts/stops goroutine pools for all delivery queues
- Beat scheduler stubs for `run-scheduled-jobs`, `mark-jobs-complete`, `check-job-status`, `replay-created-notifications`, `in-flight-to-inbox`, `beat-inbox-sms-*`, `beat-inbox-email-*` (actual implementations in later changes)

## Capabilities

### New Capabilities

- `notification-delivery-workers`: goroutine pools consuming all 7 delivery-related SQS queues (save-smss, save-emails, deliver-sms, deliver-email, deliver-throttled-sms, research-mode), with signature verification, bulk DB insertion, provider dispatch, and process-type retry back-off
- `aws-provider-clients`: SES, SNS, and Pinpoint client wrappers used by delivery workers

### Modified Capabilities

- `go-project-scaffold`: `WorkerManager` stub is replaced with the real implementation

## Non-goals

- Receipt/callback processing (separate change: `notification-receipt-callbacks`)
- Job processing workers (`process-job`, `save-smss` from jobs path) — covered in `bulk-send-jobs`
- Reporting and maintenance beat tasks — covered in `billing-tracking` and `platform-admin-features`
- Provider selection and failover logic — covered in `provider-integration`

## Impact

- Requires `go-project-setup` (SQS consumer, worker manager), `data-model-migrations` (notifications repository), `send-email-notifications` and `send-sms-notifications` (notification row schema)
- Delivers the first end-to-end notification flow (send → persist → deliver)
