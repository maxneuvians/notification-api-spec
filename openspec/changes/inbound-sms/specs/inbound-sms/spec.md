## ADDED Requirements

### Requirement INB-1: Inbound SMS SNS Ingestion
`POST /inbound-sms` (no auth, SNS-triggered) SHALL validate the SNS message signature, parse the sender number and message body, normalise the sender to E.164 (alphanumeric IDs stored verbatim), encrypt the body with `pkg/crypto.Encrypt` (C1 fix), persist an `inbound_sms` row, and enqueue a `send-inbound-sms` task only when the service has a configured `service_inbound_api` URL.

#### Scenario: Valid SNS notification creates row
- **WHEN** a correctly signed SNS notification is delivered to `POST /inbound-sms`
- **THEN** a new `inbound_sms` row is created, the raw `content` column contains ciphertext, and the handler returns HTTP 200

#### Scenario: Service has webhook URL — task enqueued
- **WHEN** a valid SNS notification arrives for a service that has a `service_inbound_api` record
- **THEN** a `send-inbound-sms` task is enqueued for that service

#### Scenario: Service has no webhook URL — no task enqueued
- **WHEN** a valid SNS notification arrives for a service with no `service_inbound_api` record
- **THEN** the `inbound_sms` row is created but no `send-inbound-sms` task is enqueued

#### Scenario: Invalid SNS signature rejected
- **WHEN** a message arrives at `POST /inbound-sms` with an invalid or missing SNS signature
- **THEN** the handler returns HTTP 403 and no `inbound_sms` row is created

---

### Requirement INB-2: Inbound SMS Encrypted Content Column (C1 fix)
The `content` column of `inbound_sms` (physical column; maps to Python `_content`) SHALL store ciphertext produced by `pkg/crypto.Encrypt`. The service layer SHALL call `pkg/crypto.Decrypt` on every read. There is no transparent ORM property in Go — encrypt/decrypt is explicit at every call site.

#### Scenario: Stored raw column value is ciphertext
- **WHEN** an inbound SMS is created via `CreateInboundSms`
- **THEN** querying the raw `content` column returns bytes that are not equal to the original plaintext message

#### Scenario: Retrieved content is decrypted plaintext
- **WHEN** an inbound SMS is fetched via `GET /service/{id}/inbound-sms/{sms_id}`
- **THEN** the `content` field in the response equals the original plaintext sent by the sender

#### Scenario: Key rotation re-sign preserves plaintext
- **WHEN** `ResignInboundSms(resign=true)` is called after a signing-key rotation
- **THEN** all `content` column values are re-encrypted with the new key and decrypting any row returns the same plaintext as before

---

### Requirement INB-3: Admin Inbox List
`GET /service/{service_id}/inbound-sms` (admin auth) SHALL return all `inbound_sms` rows for the service within the retention window, ordered `created_at DESC`. An optional `phone_number` query parameter filters by sender; the value is normalised to E.164 before comparison (alphanumeric IDs matched verbatim).

#### Scenario: No filter returns all messages within default retention
- **WHEN** `GET /service/{id}/inbound-sms` is called with no query params and the service has no custom SMS retention
- **THEN** all messages created within the last 7 days are returned in a `data` array, newest-first

#### Scenario: Phone number filter returns only matching sender
- **WHEN** `?phone_number=%2B1+650+253-2222` is passed
- **THEN** only messages whose `user_number` equals `+16502532222` are returned

#### Scenario: Custom retention extends the window
- **WHEN** the service has a `ServiceDataRetention` record for SMS with `days_of_retention=14`
- **THEN** messages created up to 14 days ago are included in the results

#### Scenario: EST midnight boundary excludes messages before cutoff
- **WHEN** a message was created at 03:59 UTC on the boundary day (EST midnight)
- **THEN** the message is excluded from the results; a message created at 04:00 UTC on the same day is included

---

### Requirement INB-4: Admin Most-Recent Inbox
`GET /service/{service_id}/inbound-sms/most-recent` (admin auth) SHALL return a deduplicated list — one message per unique sender (`user_number`) — using page-based pagination. The `page` query param is 1-based; page size is controlled by `PAGE_SIZE` config. The retention window applies with the same EST midnight boundary.

#### Scenario: Full first page with more results available
- **WHEN** there are 60 unique senders and `PAGE_SIZE=50`
- **THEN** page 1 returns 50 rows and the response includes `has_next: true`

#### Scenario: Last page returns remainder
- **WHEN** page 2 is requested for the 60-sender scenario above
- **THEN** 10 rows are returned with `has_next: false`

#### Scenario: Retention window filters out inactive senders
- **WHEN** a service has 5-day SMS retention and a sender's most-recent message is 6 days ago
- **THEN** that sender does not appear in the most-recent list

---

### Requirement INB-5: Admin Summary
`GET /service/{service_id}/inbound-sms/summary` (admin auth) SHALL return `{ "count": <int>, "most_recent": "<ISO datetime or null>" }`. The `count` is always computed over a **hard-coded 7-day** window regardless of any custom retention setting. `most_recent` is `null` when no messages exist.

#### Scenario: Messages exist within 7-day window
- **WHEN** the service has inbound SMS created within the last 7 days
- **THEN** the response is `{ "count": N, "most_recent": "<ISO datetime of newest message>" }`

#### Scenario: No messages for the service
- **WHEN** no inbound SMS exist for the service
- **THEN** the response is `{ "count": 0, "most_recent": null }`

---

### Requirement INB-6: Admin Get by ID
`GET /service/{service_id}/inbound-sms/{inbound_sms_id}` (admin auth) SHALL return a single `inbound_sms` record scoped to the service. A non-UUID path parameter or a cross-service lookup returns 404.

#### Scenario: Record found for the service
- **WHEN** `inbound_sms_id` is a valid UUID belonging to the given `service_id`
- **THEN** HTTP 200 with fields: `id`, `service_id`, `user_number`, `notify_number`, `content`, `created_at`

#### Scenario: Non-UUID path parameter
- **WHEN** `inbound_sms_id` is not a valid UUID
- **THEN** HTTP 404

#### Scenario: Record belongs to a different service
- **WHEN** `inbound_sms_id` is a valid UUID but the row's `service_id` does not match the path `service_id`
- **THEN** HTTP 404

---

### Requirement INB-7: v2 Received Text Messages
`GET /v2/received-text-messages` (service API key auth) SHALL return cursor-paginated inbound SMS newest-first. The only permitted query parameter is `older_than` (UUID of the last-seen message). Any additional parameter returns 400. Response: `{ "received_text_messages": [...], "links": { "current": "<url>", "next": "<url>" } }`. Each item's `created_at` MUST carry a UTC `Z` suffix.

#### Scenario: First page includes links.next when more results exist
- **WHEN** the service has more messages than `API_PAGE_SIZE` and no `older_than` param is provided
- **THEN** `received_text_messages` contains `API_PAGE_SIZE` items (newest-first) and `links.next` is present

#### Scenario: older_than cursor walks to next page
- **WHEN** `?older_than=<uuid>` is provided
- **THEN** only messages with `created_at` strictly older than the cursor record are returned, newest-first

#### Scenario: Cursor exhausted returns empty list without links.next
- **WHEN** `older_than` references the oldest record in the service's inbox
- **THEN** `received_text_messages` is empty and `links.next` is absent from the response

#### Scenario: Extra query parameter rejected
- **WHEN** any query parameter other than `older_than` is present (e.g., `?user_number=foo`)
- **THEN** HTTP 400 with body containing `ValidationError: Additional properties are not allowed`

---

### Requirement INB-8: send-inbound-sms Webhook Worker
The `send-inbound-sms` worker SHALL read the `inbound_sms` row and the service's `service_inbound_api` URL, apply an SSRF guard to the callback URL, and POST a JSON payload with a signed bearer token. Retry policy: maximum 5 attempts, 300 s total timeout.

#### Scenario: Successful delivery to service webhook
- **WHEN** the service has a valid (non-SSRF-blocked) webhook URL and the server returns 2xx
- **THEN** the JSON payload is POSTed exactly once with a signed bearer token and the task completes successfully

#### Scenario: SSRF-blocked URL skips POST
- **WHEN** the callback URL resolves to a private or loopback address that fails the SSRF guard
- **THEN** no HTTP request is made and an error is logged; the task does not retry

#### Scenario: Transient HTTP failure triggers retry
- **WHEN** the first POST attempt returns a 5xx response
- **THEN** the worker retries up to 4 more times (5 total) before marking the delivery as failed

---

### Requirement INB-9: Nightly Retention Sweep
A nightly beat task `delete-inbound-sms-older-than-retention` SHALL delete `inbound_sms` rows older than each service's configured retention window. Services with an SMS `ServiceDataRetention` record **and** an assigned inbound number use `days_of_retention`; all other services use a 7-day default. Deletion is batched in 10 000-row chunks.

#### Scenario: Custom-retention service is purged at the correct boundary
- **WHEN** a service has a 30-day SMS retention and has messages from 25 days ago and 35 days ago
- **THEN** only the 35-day-old message is deleted; the 25-day-old message is retained

#### Scenario: Default-retention service purged at 7 days
- **WHEN** a service has no custom SMS retention and has messages from 5 days ago and 10 days ago
- **THEN** only the 10-day-old message is deleted; the 5-day-old message is retained

#### Scenario: Sweep returns total deleted count across all services
- **WHEN** the nightly task runs across three services with 3-day, 7-day (default), and 30-day retention
- **THEN** it returns the sum of all rows deleted across every service (verified: 4 + 2 + 1 = 7 in the reference scenario)
