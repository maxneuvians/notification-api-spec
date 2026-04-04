## Context
The Python application exposes 7 letter-related endpoints as stubs (they exist and return fixed responses, but no actual letter generation occurs). The Go rewrite must expose these same routes returning the correct HTTP status codes so that API clients and integration tests do not receive 404s. Full letter implementation is explicitly out of scope for this project.

## Goals
- Mount all 7 stub routes in the Go router with correct HTTP methods and path shapes
- Return the exact HTTP status codes and response bodies that Python stubs return
- Never touch the database or external services inside any of the 7 stub handlers

## Non-Goals
- Actual letter generation, printing, or delivery
- DVLA integration
- Letter PDF rendering or storage

## Decisions

### Predetermined responses only — no DB queries
All 7 stub handlers return hardcoded JSON without touching the database, message queues, or any external service. This ensures they can never fail due to data-layer errors, matching Python stub behaviour exactly.

### Handler comment marks as STUB
Every stub handler is annotated with `// STUB: letter implementation not in scope`. This makes the intent explicit during code review and prevents accidental completion of stubs before the full letter feature is planned.

### send-pdf-letter returns 400 (not 501 or 404)
Python production returns HTTP 400 with `{"result": "error", "message": "Letters as PDF feature is not enabled"}`. Using 400 rather than 501 Not Implemented matches the Python behaviour exactly so that existing client error-handling code is not broken (H1 fix).

### Admin JWT required on all 7 endpoints
All letter routes are in the admin surface and register under the existing admin route group with `requires_admin_auth`. An unauthenticated request → 401.

### Stub letter-contact object is minimal
For `POST /service/{id}/letter-contact`, `GET/PUT /service/{id}/letter-contact/{contact_id}`, and the archive endpoint, the stub returns `{"data": {"id": "<contact_id>", "service_id": "<service_id>", "contact_block": ""}}`. This is the smallest valid shape that won't break clients expecting a letter contact object.

## Risks
- If Python stub response shapes evolve (new fields added), Go stubs will diverge silently. A future validation pass should re-verify these shapes with the Python reference.
- If admin JWT middleware is accidentally omitted from the route registration, stubs become publicly accessible. Routes must be registered inside the existing admin route group — not at the top level.
