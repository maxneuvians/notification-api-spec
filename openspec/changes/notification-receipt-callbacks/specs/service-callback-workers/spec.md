## Requirements

### Requirement: send-delivery-status worker consumes service-callbacks queue
The `send-delivery-status` worker SHALL consume the `service-callbacks` queue. For each message it SHALL verify the `itsdangerous` HMAC signature on the signed payload, look up the service's registered delivery-status callback URL and encrypted bearer token, and POST the notification status payload to that URL.

#### Scenario: Signed payload verification before processing
- **WHEN** a `send-delivery-status` task is received
- **THEN** the `itsdangerous` HMAC signature on `signed_status_update` is verified before the HTTP call is made

#### Scenario: Tampered signature is dropped without HTTP call
- **WHEN** the HMAC signature on `signed_status_update` is invalid
- **THEN** the message is deleted from the queue and no HTTP call is attempted

---

### Requirement: Delivery-status callback POST payload
The `send-delivery-status` worker SHALL POST a JSON body to the service's registered callback URL with the following fields: `id`, `reference`, `to`, `status`, `status_description`, `provider_response`, `created_at`, `completed_at`, `sent_at`, `notification_type`.

#### Scenario: HTTP POST uses Authorization Bearer with signed token
- **WHEN** a delivery-status callback is dispatched
- **THEN** the request carries an `Authorization: Bearer <signed_token>` header where the token is derived from the service's decrypted callback bearer token

#### Scenario: POST body contains all required status fields
- **WHEN** a delivery-status callback is dispatched for an email notification
- **THEN** the POST JSON body contains `id`, `reference`, `to`, `status`, `status_description`, `provider_response`, `created_at`, `completed_at`, `sent_at`, `notification_type`

#### Scenario: Request timeout is 5 seconds
- **WHEN** a delivery-status callback POST is made
- **THEN** the HTTP client enforces a 5 s request timeout

---

### Requirement: Delivery-status callback retry on non-4xx failures
The `send-delivery-status` worker SHALL retry on HTTP 5xx responses, HTTP 429, and connection errors. It SHALL NOT retry on HTTP 4xx responses (except 429). Max retries: 5. Back-off: 300 s. Retries go to `service-callbacks-retry`.

#### Scenario: HTTP 200 response deletes the SQS message
- **WHEN** the service callback endpoint returns HTTP 200
- **THEN** the SQS message is deleted from `service-callbacks`

#### Scenario: HTTP 503 re-queues to service-callbacks-retry
- **WHEN** the service callback endpoint returns HTTP 503
- **THEN** the message is re-enqueued to `service-callbacks-retry` with 300 s back-off

#### Scenario: HTTP 429 triggers retry
- **WHEN** the service callback endpoint returns HTTP 429
- **THEN** the message is retried with back-off

#### Scenario: HTTP 404 drops the message without retry
- **WHEN** the service callback endpoint returns HTTP 404
- **THEN** the message is deleted from the queue without retry and an error is logged

#### Scenario: Connection error triggers retry
- **WHEN** a TCP connection error occurs reaching the callback endpoint
- **THEN** the message is retried with 300 s back-off

#### Scenario: MaxRetriesExceeded moves message to dead-letter queue
- **WHEN** all 5 retry attempts have been exhausted
- **THEN** the message is deleted from `service-callbacks-retry` (dead-lettered) and a warning is logged

---

### Requirement: SSRF guard on delivery-status callback URL
Before making any outbound HTTP call, the `send-delivery-status` worker SHALL validate the callback URL: only HTTPS scheme is allowed, and the hostname MUST NOT resolve to an RFC 1918 private IP address or loopback address. Validation failures SHALL drop the message with an error log.

#### Scenario: HTTP (non-TLS) callback URL is rejected
- **WHEN** the service's registered callback URL uses `http://` scheme
- **THEN** no TCP connection is established and an error is logged; the SQS message is deleted

#### Scenario: Callback URL resolving to RFC 1918 address is rejected
- **WHEN** the callback URL hostname resolves to a private IP (e.g., 10.0.0.1, 192.168.1.1, 172.16.0.1)
- **THEN** no TCP connection is established, an error is logged, and the SQS message is deleted (not retried)

#### Scenario: Callback URL resolving to loopback is rejected
- **WHEN** the callback URL hostname resolves to 127.0.0.1 or ::1
- **THEN** no TCP connection is established and an error is logged

#### Scenario: Valid HTTPS callback URL to public IP proceeds
- **WHEN** the callback URL is a valid HTTPS URL resolving to a public IP
- **THEN** the HTTP POST is made normally

---

### Requirement: send-complaint worker consumes service-callbacks queue
The `send-complaint` worker SHALL consume the `service-callbacks` queue for complaint-type callbacks. It SHALL verify the HMAC signature on `complaint_data`, look up the service's complaint callback URL, and POST the complaint payload.

#### Scenario: Complaint callback POST contains required fields
- **WHEN** a `send-complaint` task is dispatched
- **THEN** the POST JSON body contains `notification_id`, `complaint_id`, `reference`, `to`, `complaint_date`

#### Scenario: Complaint callback uses Authorization Bearer header
- **WHEN** a complaint callback POST is made
- **THEN** the request carries an `Authorization: Bearer <signed_token>` header

#### Scenario: Complaint callback timeout is 5 seconds
- **WHEN** a complaint callback POST is made
- **THEN** the HTTP client enforces a 5 s timeout

---

### Requirement: send-complaint retry policy mirrors send-delivery-status
The `send-complaint` worker SHALL apply the same retry policy as `send-delivery-status`: max 5 retries, 300 s back-off, retry on 5xx/429/connection errors, drop on 4xx (except 429), retries via `service-callbacks-retry`.

#### Scenario: Complaint callback HTTP 503 re-queues to retry queue
- **WHEN** the complaint callback endpoint returns HTTP 503
- **THEN** the message is re-enqueued to `service-callbacks-retry` with 300 s back-off

#### Scenario: Complaint callback HTTP 400 drops without retry
- **WHEN** the complaint callback endpoint returns HTTP 400
- **THEN** the message is deleted without retry

---

### Requirement: SSRF guard on complaint callback URL
The `send-complaint` worker SHALL apply the same SSRF guard as `send-delivery-status`: HTTPS only, no RFC 1918 or loopback resolution.

#### Scenario: Private IP complaint callback URL is rejected
- **WHEN** the complaint callback URL hostname resolves to a private IP
- **THEN** no TCP connection is established and the message is dropped with an error log

---

### Requirement: Callback worker only fires on terminal notification status
The receipt workers SHALL only enqueue `send-delivery-status` after setting the notification to a terminal status (`delivered`, `permanent-failure`, `temporary-failure`, `technical-failure`, `provider-failure`, `virus-scan-failed`, `pii-check-failed`). Non-terminal intermediate statuses (e.g., `sending`, `created`) SHALL NOT trigger a callback.

#### Scenario: Delivery-status callback enqueued only after terminal status is set
- **WHEN** a receipt worker sets `notification.status = "delivered"`
- **THEN** a `send-delivery-status` task is enqueued to `service-callbacks`

#### Scenario: Pinpoint SUCCESSFUL (non-final) receipt does not enqueue callback
- **WHEN** a Pinpoint receipt with `messageStatus = SUCCESSFUL` and `isFinal = false` is received
- **THEN** no `send-delivery-status` task is enqueued
