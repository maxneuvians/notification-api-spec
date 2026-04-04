## Requirements

### Requirement: SNS envelope routing to receipt handler
The `delivery-receipts` queue consumer SHALL inspect each SQS message for the SNS source topic ARN or `MessageAttributes` provider-type field and dispatch to `processSESResult`, `processSNSResult`, or `processPinpointResult`. Messages with an unrecognised source SHALL be logged and deleted without retry.

#### Scenario: SES envelope routes to SES handler
- **WHEN** an SQS message arrives with an SNS source topic matching the SES receipt topic ARN
- **THEN** `processSESResult` is invoked with the parsed SES notification body

#### Scenario: Pinpoint envelope routes to Pinpoint handler
- **WHEN** an SQS message arrives with provider-type attribute indicating Pinpoint
- **THEN** `processPinpointResult` is invoked

#### Scenario: Unrecognised source is dropped with log
- **WHEN** an SQS message arrives with no recognisable source identifier
- **THEN** the message is deleted from the queue and an error is logged; no retry is attempted

---

### Requirement: SES delivery receipt sets status to delivered
The `process-ses-result` worker SHALL parse the SES notification batch, identify delivery events by `notificationType = "Delivery"`, look up each notification by provider reference, and set `notification.status = "delivered"`. A `send-delivery-status` callback SHALL be enqueued for each updated notification.

#### Scenario: SES delivery receipt updates notification to delivered
- **WHEN** an SES `Delivery` receipt arrives for a notification in `sending` status
- **THEN** `notification.status` is updated to `"delivered"` and a `send-delivery-status` task is enqueued

#### Scenario: SES delivery receipt does not set provider_response
- **WHEN** an SES `Delivery` receipt is processed
- **THEN** `notification.provider_response` is not modified

#### Scenario: Annual limit incremented on email delivery
- **WHEN** an SES delivery receipt is processed successfully
- **THEN** `annual_limit_client.increment_email_delivered(service_id)` is called

---

### Requirement: SES hard bounce — subtype-specific status and provider_response
The `process-ses-result` worker SHALL handle SES hard bounces by setting `notification.status = "permanent-failure"`, setting `feedback_type = NOTIFICATION_HARD_BOUNCE`, and applying the subtype-specific `provider_response` value.

#### Scenario: Hard bounce subtype General yields permanent-failure with null provider_response
- **WHEN** an SES hard bounce with `BounceSubType = "General"` arrives
- **THEN** `notification.status = "permanent-failure"`, `notification.feedback_type = NOTIFICATION_HARD_BOUNCE`, `notification.provider_response = null`

#### Scenario: Hard bounce subtype Suppressed yields permanent-failure with suppression list response
- **WHEN** an SES hard bounce with `BounceSubType = "Suppressed"` arrives
- **THEN** `notification.status = "permanent-failure"` and `notification.provider_response = "The email address is on our email provider suppression list"`

#### Scenario: Hard bounce subtype OnAccountSuppressionList yields GC Notify suppression response
- **WHEN** an SES hard bounce with `BounceSubType = "OnAccountSuppressionList"` arrives
- **THEN** `notification.status = "permanent-failure"` and `notification.provider_response = "The email address is on the GC Notify suppression list"`

#### Scenario: Hard bounce updates bounce-rate sliding window
- **WHEN** any SES hard bounce is processed
- **THEN** `bounce_rate_client.set_sliding_hard_bounce(service_id, notification_id)` is called and `set_sliding_notifications` is NOT called

#### Scenario: Annual limit incremented on hard bounce
- **WHEN** an SES hard bounce is processed
- **THEN** `annual_limit_client.increment_email_failed(service_id)` is called

#### Scenario: Hard bounce callback enqueued
- **WHEN** an SES hard bounce receipt arrives for a service with a registered callback URL
- **THEN** a `send-delivery-status` task is enqueued to `service-callbacks`

---

### Requirement: SES soft bounce — subtype-specific status and provider_response
The `process-ses-result` worker SHALL handle SES soft bounces by setting `notification.status = "temporary-failure"` and `feedback_type = NOTIFICATION_SOFT_BOUNCE`. Soft bounces SHALL NOT update the Redis bounce-rate sliding window.

#### Scenario: Soft bounce subtype General yields temporary-failure with null provider_response
- **WHEN** an SES soft bounce with `BounceSubType = "General"` arrives
- **THEN** `notification.status = "temporary-failure"`, `notification.feedback_type = NOTIFICATION_SOFT_BOUNCE`, `notification.provider_response = null`

#### Scenario: Soft bounce subtype AttachmentRejected yields attachment-rejection response
- **WHEN** an SES soft bounce with `BounceSubType = "AttachmentRejected"` arrives
- **THEN** `notification.status = "temporary-failure"` and `notification.provider_response = "The email was rejected because of its attachments"`

#### Scenario: Soft bounce does not update bounce-rate Redis sliding window
- **WHEN** an SES soft bounce is processed
- **THEN** `bounce_rate_client.set_sliding_hard_bounce` is NOT called

---

### Requirement: SES status transition guard — permanent-failure is sticky
The `process-ses-result` worker SHALL check the notification's current status before updating. If the notification is already at `permanent-failure`, a subsequent delivery receipt SHALL be ignored. However, a notification at `delivered` SHALL be updatable to `permanent-failure` by a hard bounce receipt.

#### Scenario: Delivery receipt ignored for already-permanent-failure notification
- **WHEN** an SES delivery receipt arrives for a notification already in `permanent-failure` status
- **THEN** no status update is made and no callback is enqueued

#### Scenario: Delivered notification can be downgraded to permanent-failure
- **WHEN** an SES hard bounce receipt arrives for a notification currently in `delivered` status
- **THEN** `notification.status` is updated to `permanent-failure`

---

### Requirement: SES complaint receipt — create Complaint record, no status change, PII scrub
The `process-ses-result` worker SHALL handle SES `Complaint` events by creating a `Complaint` DB record linked to the notification, WITHOUT updating notification status. Recipient email addresses in the complaint payload SHALL be scrubbed by `remove_emails_from_complaint`. If the notification is not in the main `notifications` table it SHALL be fetched from `notification_history`.

#### Scenario: Complaint receipt creates Complaint row without status change
- **WHEN** an SES `Complaint` event arrives for a notification
- **THEN** a row is inserted into `complaints` and `notification.status` is unchanged

#### Scenario: Complaint receipt scrubs PII from stored JSON
- **WHEN** a complaint receipt containing recipient email addresses is processed
- **THEN** `remove_emails_from_complaint` is called; stored complaint JSON contains no email addresses

#### Scenario: Complaint receipt for notification in history falls back to notification_history
- **WHEN** the notification referenced by a complaint is not in the main `notifications` table
- **THEN** it is fetched from `notification_history` and the `Complaint` row is created referencing that ID

#### Scenario: Complaint callback enqueued
- **WHEN** a complaint receipt is processed for a service with a registered complaint callback
- **THEN** a `send-complaint` task is enqueued to `service-callbacks`

---

### Requirement: SES partial batch retry
When processing a batch of SES receipts, notifications not yet present in the DB SHALL be re-queued individually to `retry-tasks`. Successfully resolved notifications SHALL have their callbacks dispatched normally in the same batch processing cycle.

#### Scenario: Not-found notification in batch is re-queued individually
- **WHEN** a batch of 3 SES receipts is processed and 1 notification is not found
- **THEN** the 1 not-found receipt is re-queued to `retry-tasks` and the other 2 notifications are updated and their callbacks dispatched

#### Scenario: MaxRetriesExceeded for missing notation logs and drops
- **WHEN** a `process-ses-result` retry reaches `MaxRetriesExceeded` for missing notifications
- **THEN** an error is logged: `"notifications not found for SES references: <refs>. Giving up."` and the message is dropped

---

### Requirement: SES annual limit seeding on first outcome of the day
When the Redis annual-limit key for a service has not been seeded today, the receipt worker SHALL call `seed_annual_limit_notifications(service_id, data)` with current DB counts and SHALL NOT call a separate `increment_*` function for that outcome (to avoid double-counting).

#### Scenario: First email delivery of the day seeds annual limits and skips increment
- **WHEN** `process-ses-result` processes a delivery receipt and Redis annual-limit key is not seeded for today
- **THEN** `seed_annual_limit_notifications` is called and `increment_email_delivered` is NOT called for that outcome

#### Scenario: Subsequent deliveries on same day use increment only
- **WHEN** `process-ses-result` processes a delivery receipt and Redis annual-limit key is already seeded for today
- **THEN** `annual_limit_client.increment_email_delivered(service_id)` is called without seeding

---

### Requirement: process-pinpoint-result worker (C3 fix)
The `process-pinpoint-result` worker SHALL be implemented as a first-class consumer of the `delivery-receipts` queue. This worker was present in Python production code (`app/celery/process_pinpoint_receipts_tasks.py`) but was missing from `spec/async-tasks.md`. It SHALL parse Pinpoint SMS V2 delivery events, map the outcome to a Notify status, store optional SMS metadata fields, and enqueue delivery-status callbacks.

#### Scenario: Pinpoint delivered receipt updates status and stores 7 metadata fields
- **WHEN** a Pinpoint delivery event with `messageStatus = DELIVERED` arrives
- **THEN** `notification.status = NOTIFICATION_DELIVERED` and up to 7 SMS metadata fields are stored: `provider_response`, `sms_total_message_price`, `sms_total_carrier_fee`, `sms_iso_country_code`, `sms_carrier_name`, `sms_message_encoding`, `sms_origination_phone_number`

#### Scenario: Pinpoint shortcode delivery stores short code as origination number
- **WHEN** a Pinpoint delivery event indicates a shortcode origination (e.g., `"555555"`)
- **THEN** `sms_origination_phone_number = "555555"` and `provider_response = "Message has been accepted by phone carrier"`

#### Scenario: Pinpoint delivered with missing SMS metadata still sets delivered status
- **WHEN** a Pinpoint delivery event is missing carrier and country fields
- **THEN** `notification.status = NOTIFICATION_DELIVERED`; absent fields remain `null`

#### Scenario: Pinpoint annual limit incremented on SMS delivery
- **WHEN** a Pinpoint delivered receipt is processed
- **THEN** `annual_limit_client.increment_sms_delivered(service_id)` is called

#### Scenario: Pinpoint successful (non-delivered, isFinal=false) leaves status unchanged
- **WHEN** a Pinpoint event arrives with `messageStatus = SUCCESSFUL` and `isFinal = false`
- **THEN** no status update is made and no callback is enqueued

---

### Requirement: Pinpoint failure — 13-entry provider_response to status mapping
The `process-pinpoint-result` worker SHALL map the `messageStatusDescription` (provider_response) string to a Notify status using the following exhaustive table.

#### Scenario: "Blocked as spam by phone carrier" maps to permanent-failure
- **WHEN** `provider_response = "Blocked as spam by phone carrier"`
- **THEN** `notification.status = PERMANENT_FAILURE`

#### Scenario: "Destination is on a blocked list" maps to permanent-failure
- **WHEN** `provider_response = "Destination is on a blocked list"`
- **THEN** `notification.status = PERMANENT_FAILURE`

#### Scenario: "Invalid phone number" maps to permanent-failure
- **WHEN** `provider_response = "Invalid phone number"`
- **THEN** `notification.status = PERMANENT_FAILURE`

#### Scenario: "Message body is invalid" maps to permanent-failure
- **WHEN** `provider_response = "Message body is invalid"`
- **THEN** `notification.status = PERMANENT_FAILURE`

#### Scenario: "Phone is currently unreachable/unavailable" maps to permanent-failure
- **WHEN** `provider_response = "Phone is currently unreachable/unavailable"`
- **THEN** `notification.status = PERMANENT_FAILURE`

#### Scenario: "Unknown error attempting to reach phone" maps to permanent-failure
- **WHEN** `provider_response = "Unknown error attempting to reach phone"`
- **THEN** `notification.status = PERMANENT_FAILURE`

#### Scenario: "Unhandled provider" maps to permanent-failure
- **WHEN** `provider_response = "Unhandled provider"`
- **THEN** `notification.status = PERMANENT_FAILURE`

#### Scenario: "Phone carrier has blocked this message" maps to temporary-failure
- **WHEN** `provider_response = "Phone carrier has blocked this message"`
- **THEN** `notification.status = TEMPORARY_FAILURE`

#### Scenario: "Phone carrier is currently unreachable/unavailable" maps to temporary-failure
- **WHEN** `provider_response = "Phone carrier is currently unreachable/unavailable"`
- **THEN** `notification.status = TEMPORARY_FAILURE`

#### Scenario: "Phone has blocked SMS" maps to temporary-failure
- **WHEN** `provider_response = "Phone has blocked SMS"`
- **THEN** `notification.status = TEMPORARY_FAILURE`

#### Scenario: "Phone is on a blocked list" maps to temporary-failure
- **WHEN** `provider_response = "Phone is on a blocked list"`
- **THEN** `notification.status = TEMPORARY_FAILURE`

#### Scenario: "This delivery would exceed max price" maps to temporary-failure
- **WHEN** `provider_response = "This delivery would exceed max price"`
- **THEN** `notification.status = TEMPORARY_FAILURE`

#### Scenario: "Phone number is opted out" maps to technical-failure
- **WHEN** `provider_response = "Phone number is opted out"`
- **THEN** `notification.status = TECHNICAL_FAILURE`

#### Scenario: Unrecognised provider_response string maps to technical-failure with warning
- **WHEN** `provider_response` does not match any known string
- **THEN** `notification.status = TECHNICAL_FAILURE` and a warning is logged

#### Scenario: All Pinpoint failure cases save provider_response and enqueue callback
- **WHEN** any Pinpoint failure receipt is processed
- **THEN** `notification.provider_response` is saved and a `send-delivery-status` callback is enqueued

#### Scenario: Pinpoint annual limit incremented on SMS failure
- **WHEN** any Pinpoint failure receipt is processed
- **THEN** `annual_limit_client.increment_sms_failed(service_id)` is called

---

### Requirement: Pinpoint wrong provider guard
If the notification found by provider reference has `sent_by` set to a provider other than `"pinpoint"`, the `process-pinpoint-result` worker SHALL log an exception and return without updating the notification status.

#### Scenario: Wrong provider logs exception and skips update
- **WHEN** a Pinpoint receipt arrives but the matched notification has `sent_by = "sns"`
- **THEN** an exception is logged, `notification.status` is unchanged, and no callback is enqueued

---

### Requirement: Pinpoint retry and max-retries behaviour
The `process-pinpoint-result` worker SHALL retry on notification-not-found and on DB update errors. On `MaxRetriesExceeded`, it SHALL log a warning and drop the message without queuing a callback.

#### Scenario: Notification not found triggers retry
- **WHEN** `process-pinpoint-result` cannot find the notification by provider reference
- **THEN** the task is retried via `retry-tasks`

#### Scenario: MaxRetriesExceeded logs warning without callback
- **WHEN** all 5 retries are exhausted for a missing notification
- **THEN** a warning is logged: `"notification not found for Pinpoint reference: <ref> (update to <status>). Giving up."` and no callback is enqueued

---

### Requirement: Pinpoint annual limit seeding on first outcome of the day
Same seeding semantics as SES: when Redis annual-limit key is not yet seeded for today, call `seed_annual_limit_notifications` and suppress the separate `increment_*` call.

#### Scenario: First SMS outcome of the day seeds annual limits
- **WHEN** a Pinpoint receipt is processed and Redis key is not seeded for today
- **THEN** `seed_annual_limit_notifications(service_id, data)` is called and `increment_sms_delivered`/`increment_sms_failed` is NOT called for that outcome

#### Scenario: FF_USE_BILLABLE_UNITS seed payload includes SMS billable unit fields
- **WHEN** `FF_USE_BILLABLE_UNITS` is enabled and seeding runs
- **THEN** the seed payload includes `total_sms_billable_units_fiscal_year_to_yesterday`, `sms_billable_units_failed_today`, `sms_billable_units_delivered_today`
