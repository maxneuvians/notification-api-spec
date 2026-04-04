## Why

Services can receive inbound SMS replies via dedicated numbers. This change implements the inbound SMS ingestion endpoint (called by SNS on receipt), the inbox retrieval endpoints for services, and the `send-inbound-sms` webhook worker. Includes C1 fix: the encrypted `_content` column on `inbound_sms`.

## What Changes

- `internal/handler/inbound/` — POST inbound SMS (SNS-triggered), GET /service/{id}/inbound-sms list, GET /v2/received-text-messages
- `internal/service/inbound/` — phone normalisation, inbound_sms row creation with encrypted `_content` via `pkg/crypto` (**C1 fix**), `send-inbound-sms` task enqueue to notify registered service webhook
- `internal/worker/scheduled/` — inbox drain tickers (6 tickers at 10 s intervals for 3 SMS + 3 email priorities, already stubbed in delivery pipeline)
- Retention: inbound SMS older than the service's configured retention window (default 7 days) are deleted by the nightly maintenance task

## Capabilities

### New Capabilities

- `inbound-sms`: Inbound SMS ingestion, encrypted content storage (C1 fix), service inbox retrieval (admin and v2), send-inbound-sms webhook worker

### Modified Capabilities

## Non-goals

- Outbound SMS (covered in `send-sms-notifications`)
- Inbound number CRUD — managed via `service-management`

## Impact

- Requires `authentication-middleware`, `data-model-migrations` (inbound_sms repository)
- **C1 fix**: `inbound_sms._content` (physical column `content`) is encrypted using `signer_inbound_sms`; the repository layer treats it as raw bytes and the service layer calls `pkg/crypto.Decrypt`/`Encrypt` explicitly
