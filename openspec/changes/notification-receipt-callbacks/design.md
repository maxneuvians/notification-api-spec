## Context

Delivering a notification generates a receipt (delivery confirmation, bounce, or complaint) from the provider. AWS SES, SNS, and Pinpoint each push receipts into the `delivery-receipts` SQS queue wrapped in an SNS envelope. Go receipt workers parse the envelope, look up the originating notification by provider reference, update its status, record complaint objects, and enqueue outbound service webhook calls. Service callback workers then POST those status updates and complaints to the service's registered HTTPS webhook endpoints.

This change also adds the `process-pinpoint-result` worker as a **C3 fix**: the task exists in the Python production codebase (`app/celery/process_pinpoint_receipts_tasks.py`) and is fully tested, but was absent from the intermediate `spec/async-tasks.md` specification.

## Goals / Non-Goals

**Goals:**
- Implement `process-ses-result` worker: batch SES delivery/bounce/complaint processing with partial-batch retry
- Implement `process-sns-result` worker: SNS SMS delivery report processing
- Implement `process-pinpoint-result` worker (C3 fix): Pinpoint SMS V2 event processing with 13-entry status mapping
- Implement `send-delivery-status` worker: signed webhook POST to service-registered callback URLs
- Implement `send-complaint` worker: complaint webhook POST with PII-scrubbed payload
- Enforce status-transition guard (permanent-failure stays permanent-failure on late delivery receipt)
- Enforce SSRF guard on all outbound callback URLs
- Track annual limit increments on every terminal receipt outcome

**Non-Goals:**
- Inbound SMS (entirely separate flow — `inbound-sms` change)
- Provider selection and failover logic (`provider-integration` change)
- Bounce rate tracking/enforcement (`billing-tracking` change)
- Receipt processing for letter/DVLA notifications

## Decisions

### D1: Three receipt workers share one SQS queue (`delivery-receipts`)
All provider receipts arrive at a single SQS queue wrapped in an SNS envelope. The dispatcher inspects the SNS source topic ARN or `MessageAttributes` (provider type field) and routes to the appropriate handler (`processSESResult`, `processSNSResult`, `processPinpointResult`). Unrecognised source → logged and dropped. This matches the Python `ProcessNotificationService` routing pattern.

### D2: Status transition guard prevents delivery from overwriting permanent-failure
Before updating status, the worker checks the notification's current status. If it is already `permanent-failure`, a delivery receipt is ignored (status stays). A `delivered` notification CAN be overwritten to `permanent-failure` by a subsequent hard bounce. This one-way guard matches the Python DAO `update_notification_status_by_id` semantics and prevents infinite callback loops on late-arriving receipts.

### D3: SES batch receipt — partial retry (not whole-batch retry)
When a batch of SES receipts contains some notifications not yet found in the DB, those records are re-queued individually to `retry-tasks` while successfully resolved notifications proceed to callback dispatch. Whole-batch retry would delay all notifications in the batch. This matches Python's `_update_notification_statuses` partial-retry pattern.

### D4: Complaint PII scrub happens at write time
`remove_emails_from_complaint` strips recipient email addresses from the complaint payload before the `Complaint` row is persisted. This matches the Python production behaviour and minimises PII retention surface.

### D5: Callback worker uses signed bearer token (not HMAC body signing)
Service callback requests carry `Authorization: Bearer <signed_token>` where the token is `pkg/signing.Sign(service_callback_api.bearer_token, notification_id)`. Timeout: 5 s. The signed blob on the SQS message allows the worker to reconstruct the token without a DB lookup on the hot path.

### D6: SSRF guard enforced before TCP connection
Before making any outbound HTTP call in `send-delivery-status` or `send-complaint`, the callback URL's hostname is resolved and the resulting IP is checked against RFC 1918 ranges and loopback. Only HTTPS scheme is accepted. Private-IP or HTTP URLs are rejected with a logged error and the SQS message is deleted (not retried — this is a misconfiguration, not a transient failure).

### D7: Annual limit seeding on first outcome of the day
When the Redis annual-limit key for a service has not yet been seeded today, the receipt worker calls `seed_annual_limit_notifications` with current DB counts and suppresses the separate `increment_*` call to avoid double-counting. Subsequent outcomes within the same day call only `increment_*`.

## Risks / Trade-offs

- **Pinpoint receipt format divergence** — Pinpoint SMS V2 events have a different schema from SNS SMS delivery reports. The `process-pinpoint-result` worker must parse the Pinpoint-specific fields (`messageStatus`, `messageStatusDescription`, `isFinal`, SMS pricing fields). Reference: `app/celery/process_pinpoint_receipts_tasks.py` fixture payloads.
- **Duplicate receipt for the same notification** — A notification may receive a receipt more than once (e.g., AWS retries an SNS delivery). The status-transition guard (D2) provides idempotency for delivery/bounce. For Pinpoint, the `isFinal=false` + `SUCCESSFUL` early-return prevents spurious intermediate-state updates.
- **Callback URL availability** — Service webhook endpoints may be temporarily unavailable. Retry max is 5 with 300 s back-off. After 5 retries the message is dead-lettered. Services that consistently fail callbacks will accumulate DLQ messages — operators must monitor DLQ depth.
- **notification_history fallback (SES complaints)** — Complaints may arrive for notifications older than the data-retention window, which have been moved to `notification_history`. The complaint worker must handle the fallback query without performance degradation (indexed by provider reference).
