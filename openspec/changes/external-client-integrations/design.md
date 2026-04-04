## Context
GC Notify integrates with four external SaaS systems: Airtable (newsletter subscriber storage), Salesforce (service account and engagement tracking), Freshdesk (support ticket creation), and Cronitor (nightly task heartbeat monitoring). This change implements all four HTTP client packages and wires them to the handlers and workers that call them.

## Goals
- Implement all four external client packages with full behavioral fidelity to the Python reference
- Keep each client independently mockable for unit testing
- Wire three newsletter handler routes (`POST /newsletter/unconfirmed-subscriber`, `GET /newsletter/confirm/{id}`, `GET /newsletter/unsubscribe/{id}`) to the Airtable client

## Non-Goals
- The 4 undocumented newsletter routes (C2 fix) — covered in `newsletter-endpoints`
- AWS SNS client — covered in `send-sms-notifications`
- Document Download client — covered in `notification-delivery-pipeline`
- Performance Platform stats collection — deferred to a future analytics change
- Inbound email / no-reply worker — covered in `notification-delivery-pipeline`

## Decisions

### All clients: interface-based for testability
Each client is defined as a Go interface; the production implementation uses real HTTP (or AWS SDK). Tests inject a mock implementation. No concrete client types are constructed outside of their `internal/client/*/` package.

### Salesforce: SOAP + REST hybrid
Salesforce engagement data (Opportunity, OpportunityContactRole) uses SOAP XML; Contact and Account data uses REST. Both sub-clients live in `internal/client/salesforce/`. The `SalesforceClient` facade opens a session, delegates to module functions, and always closes the session in a `defer` — never leaks an authenticated session.

### Airtable: create-if-absent table provisioning
The `AirtableTableMixin` pattern auto-provisions required tables on first use. The Go client must replicate this: on any 404 from the Airtable API for a table operation, attempt table creation before retrying. This avoids manual setup steps in staging environments.

### Freshdesk: silent fallback on failure
HTTP errors from Freshdesk must never surface to the caller. On failure, the client queues an email notification to `CONTACT_FORM_EMAIL_ADDRESS` via the internal Notify pipeline and returns 201 (matching the Python `email_freshdesk_ticket()` fallback). When `FRESH_DESK_ENABLED = false`, the client returns 201 immediately with no HTTP call.

### Cronitor: fire-and-forget, failures do not propagate
Cronitor heartbeat failures (network errors, non-2xx) are logged and discarded. They must never fail the nightly task that triggered the ping. The ping is a simple GET with no retry logic.

### Config injection via struct
All credentials (`SALESFORCE_*`, `AIRTABLE_*`, `FRESH_DESK_*`, `CRONITOR_*`) are read from the application `Config` struct and injected into each client at construction time (not read from environment variables directly inside client code).

## Risks
- Salesforce session lifecycle is stateful; a leaked session consumes API quota. Verify `defer end_session` is called even when a panic occurs inside the operation.
- Airtable create-if-absent is a two-step operation: two concurrent requests could both attempt table creation. Treat the second HTTP 409 as success (idempotent).
- Freshdesk rate limits are not publicly documented; ticket creation bursts (e.g. concurrent go-live requests) could be throttled. Consider a per-request timeout.
