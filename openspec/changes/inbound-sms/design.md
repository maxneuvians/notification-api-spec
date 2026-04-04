## Context
Inbound SMS receives replies to service-owned numbers via AWS SNS push. The system validates the SNS signature, stores messages as `inbound_sms` rows with an encrypted `content` column (C1 fix), and optionally delivers them to a registered service webhook. Admins and service consumers retrieve messages through separate inbox endpoints.

## Goals / Non-Goals
**Goals:** SNS ingestion handler, encrypted content storage (C1 fix), admin and v2 inbox endpoints, `send-inbound-sms` webhook worker, nightly retention sweep.
**Non-Goals:** Inbound number CRUD (managed in `service-management`), outbound SMS (in `send-sms-notifications`).

## Decisions

### C1 fix: explicit encrypt/decrypt for inbound_sms.content
The Python version transparently encrypts via the `signer_inbound_sms` SQLAlchemy hybrid property. In Go there is no ORM-level property; the service layer calls `pkg/crypto.Encrypt` before every write and `pkg/crypto.Decrypt` after every read of the `content` column. The repository layer treats `content` as raw `[]byte` — it never touches the plaintext.

### SNS ingestion flow
`POST /inbound-sms` (no application auth) validates the SNS message signature against the SNS certificate URL before processing. On a valid message: normalise sender to E.164 → encrypt body → `CreateInboundSms` → conditionally enqueue `send-inbound-sms`. Invalid signature → 403, no row written. This mirrors the Python behaviour while making the encryption step explicit.

### Admin vs v2 inbox endpoints: same data, different auth and pagination
- Admin (`GET /service/{id}/inbound-sms` and sub-routes): JWT admin auth, page-based pagination for the most-recent view, optional `phone_number` filter normalised to E.164.
- v2 (`GET /v2/received-text-messages`): service API-key auth, cursor-based pagination via `older_than` UUID. The only permitted query param is `older_than`; any other triggers a 400 ValidationError.

In the Python API the admin inbox uses `POST` to carry a filter body; the Go equivalent uses `GET /service/{id}/inbound-sms?phone_number=…` to stay RESTful.

### Retention: nightly beat task, two-pass sweep
Inbound SMS are **not** deleted at query time. A nightly beat task `delete-inbound-sms-older-than-retention` runs two passes: (1) services with an SMS `ServiceDataRetention` record **and** an assigned inbound number → use `days_of_retention`; (2) all remaining services → 7-day default. Deletes are batched in 10 000-row chunks. The dashboard summary endpoint hard-codes 7 days regardless of custom retention.

### Webhook delivery: SSRF guard + retry policy
The `send-inbound-sms` worker applies the same SSRF denylist check as service callbacks before POSTing to the `service_inbound_api` URL. Retry policy: max 5 attempts, 300 s total timeout. No POST is made if the URL is SSRF-blocked; an error is logged instead.
