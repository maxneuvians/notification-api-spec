## Context

Notifications are persisted to the DB by the send endpoints (via Redis inbox → `save-smss`/`save-emails`) but are not yet dispatched to a provider. This change implements the goroutine-based worker pools that carry a notification from a DB row to an actual AWS API call, replacing Python Celery tasks `save-smss`, `save-emails`, `deliver-sms`, `deliver-email`, and `deliver-throttled-sms`. It also wires the three AWS provider clients (SES, SNS, Pinpoint) and replaces the `WorkerManager` stub from `go-project-setup` with the real implementation.

## Goals / Non-Goals

**Goals:**
- Implement `save-smss` and `save-emails` workers: HMAC signature verification, bulk DB insert, delivery-queue enqueue
- Implement `deliver-sms`, `deliver-throttled-sms`, and `deliver-email` workers: pre-conditions, research mode, PII scan, provider dispatch, process-type retry
- Implement `AwsPinpointClient`: phone normalisation, pool selection, dedicated sender, opted-out handling, error mapping
- Implement `AwsSesClient`: MIME construction (multipart/alternative and multipart/mixed), IDN punycode, error mapping
- Implement `WorkerManager.Start/Stop` replacing the stub

**Non-Goals:**
- Receipt/callback processing (`notification-receipt-callbacks` change)
- Provider selection and failover (`provider-integration` change)
- Job batch processing path (`bulk-send-jobs` change)
- Reporting and maintenance beat tasks (`billing-tracking` / `platform-admin-features`)
- Bounce rate enforcement (`billing-tracking`)

## Decisions

### D1: Worker pools map 1:1 to SQS queue names
Each worker goroutine pool receives a dedicated SQS queue URL configured at startup. Pool concurrency is configurable per queue. This replaces Celery's per-pod `CELERY_CONCURRENCY` setting and allows independent scaling of each queue's consumers without redeploying.

### D2: Save workers verify HMAC before any DB write
Each signed notification blob from the Redis inbox is verified with `pkg/signing.Verify` (itsdangerous-compatible HMAC) before any DB operation is attempted. A `BadSignature` error causes the SQS message to be dropped immediately with no retry — matching the Python `BadSignature` raise-without-retry behaviour. This prevents malformed or tampered payloads from polluting the DB.

### D3: Process-type-aware retry back-off
Delivery workers implement three retry countdown tiers:
- PRIORITY: 25 s (matches Python `countdown=25` for not-found)
- NORMAL / BULK: 300 s (matches Python `countdown=300`)
Max retries: 48. After max retries the notification is set to `technical_failure` and a delivery-status callback is enqueued. This mirrors the Python `MaxRetriesExceededError` handler.

### D4: deliver-throttled-sms — single goroutine with token-bucket rate limiter
The throttled pool runs with concurrency = 1 and a `golang.org/x/time/rate` token-bucket limiter at 0.5 tokens/s (30/min). This replicates Celery `rate_limit="30/m"` on a single dedicated pod with concurrency=1. Global cap is guaranteed as long as exactly one throttled-sms goroutine runs, which is enforced in `WorkerManager`.

### D5: PII scan gate (FF_SCAN_FOR_PII)
Before dispatching an email to SES, the worker checks the notification body for Social Insurance Numbers that pass the Luhn algorithm. A positive hit raises `NotificationTechnicalFailureException` with status `pii-check-failed`. The Luhn check is the authoritative gate — phone-number-format patterns and Luhn-failing SINs are allowed through. This matches the Python `SCAN_FOR_PII` feature-flag path.

### D6: Malware scan before file attachment download
For `sending_method=attach`, the worker calls `document_download_client.check_scan_verdict` before fetching the file. 423 → immediate `virus-scan-failed`; 428 → exponential backoff retry (keeps status `created`); 404 → immediate `technical-failure`. This ordering ensures infected files are rejected before any S3 download is attempted.

### D7: Bounce rate tracking after every email send
After a successful SES dispatch, the worker calls `bounce_rate_client.set_sliding_notifications` and then checks `bounce_rate_client.check_bounce_rate_status`. CRITICAL threshold ≥10% and WARNING threshold ≥5% emit structured log warnings. This check is informational only at dispatch time — enforcement (suspension) is handled separately in `billing-tracking`.

### D8: AWS clients wrapped behind interfaces
All three AWS clients (SES, SNS, Pinpoint) implement narrow Go interfaces so they can be replaced with mocks in unit tests without any network calls. The interfaces expose only the operations actually used (`SendTextMessage`, `SendEmail`, `SendRawEmail`, `Publish`).

## Risks / Trade-offs

- **Duplicate email delivery on retry** — If `deliver-email` calls SES successfully but SQS acknowledgement fails, a retry will call SES a second time. Mitigated by the pre-condition check: skip if notification status is not `"created"` (SES sets it to `"sending"` on the first successful dispatch). Accepts at-most-twice delivery for the rare SQS ACK failure.
- **Bulk insert partial failure** — If `BulkInsertNotifications` fails mid-batch, individual notifications already inserted may be re-inserted on retry. The `handle_batch_error_and_forward` pattern re-queues individually; the DB upsert on `notification_id` primary key prevents true duplicates.
- **Pinpoint pool misconfiguration** — If `AWS_PINPOINT_DEFAULT_POOL_ID` is empty but a template is listed in `AWS_PINPOINT_SC_TEMPLATE_IDS`, the system falls back to SNS rather than failing. Operators must ensure both pool IDs are configured for full Pinpoint routing to be active.
- **International SMS without OriginationIdentity** — Pinpoint requires no `OriginationIdentity` for non-`+1` numbers. If this field is accidentally set for international numbers, Pinpoint will reject the send. The client must apply the international-number check before constructing the boto3 payload.
