## Source Files
- spec/behavioral-spec/misc-gaps.md (H1 fix)
- openspec/changes/letter-stub-endpoints/proposal.md

---

## Context
Validation finding H1: 7 letter-related endpoints exist in Python production as stubs — they accept requests and return fixed responses but trigger no letter generation. The Go implementation must expose these same routes with correct HTTP status codes so API clients and integration tests never receive 404s. Full letter implementation is explicitly out of scope.

---

## Stubs Required

### Group 1: Letter Contact Routes (5 endpoints)

| Method | Path | Status | Response body |
|--------|------|--------|---------------|
| GET | `/service/{id}/letter-contact` | 200 | `{"data": []}` |
| POST | `/service/{id}/letter-contact` | 200 | stub letter contact object |
| GET | `/service/{id}/letter-contact/{contact_id}` | 200 | stub letter contact object |
| PUT | `/service/{id}/letter-contact/{contact_id}` | 200 | stub letter contact object |
| POST | `/service/{id}/letter-contact/{contact_id}/archive` | 200 | stub letter contact object |

**Stub letter contact object** (minimal shape):
```json
{
  "data": {
    "id": "<contact_id>",
    "service_id": "<service_id>",
    "contact_block": ""
  }
}
```
For `GET /service/{id}/letter-contact` (list), the stub returns `{"data": []}` (empty list) rather than a single object.

---

### Group 2: Send PDF Letter (1 endpoint)

| Method | Path | Status | Response body |
|--------|------|--------|---------------|
| POST | `/service/{id}/send-pdf-letter` | 400 | `{"result": "error", "message": "Letters as PDF feature is not enabled"}` |

The 400 message text must match Python verbatim: `"Letters as PDF feature is not enabled"`.

---

### Group 3: Returned Letters (1 endpoint)

| Method | Path | Status | Response body |
|--------|------|--------|---------------|
| GET | `/letters/returned` | 200 | `{"data": []}` |

---

## Auth Requirements
All 7 stub endpoints require admin JWT (`requires_admin_auth`). Unauthenticated requests → 401.

---

## Implementation Notes
- All stub handlers must include a code comment: `// STUB: letter implementation not in scope`
- No database queries; no worker integration; no S3 interaction; no external API calls
- Handlers must satisfy the same router registration pattern as non-stub handlers
- Routes are in the admin surface; register under the existing admin route group

---

## Business Rules
- None of the 7 routes trigger any real letter generation
- HTTP status codes must match Python production behaviour exactly (H1 fix)
- PDF letter 400 message must match Python verbatim (client error handling depends on exact text)
- Stub responses must be stable across all calls (same input always produces same output)
