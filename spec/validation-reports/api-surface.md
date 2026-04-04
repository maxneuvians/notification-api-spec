# Validation Report: api-surface.md
Date: 2026-04-04

## Summary
- **Endpoints in spec**: ~210 (as claimed)
- **Endpoints in codebase**: ~215 (accounting for stubs and undocumented routes)
- **CONFIRMED**: ~205 (HTTP methods, paths, auth match)
- **DISCREPANCIES**: 4 (routes in code not documented in spec)
- **MISSING FROM SPEC**: 4 (routes exist in code but not documented)
- **EXTRA IN SPEC**: 0 (all documented routes exist in code)
- **RISK ITEMS**: 4

---

## Blueprint-by-Blueprint Findings

### ✅ CONFIRMED BLUEPRINTS (All routes match spec)
- Status (3 routes) ✅
- Accept Invite (1 route) ✅
- API Key Stats (2 routes) ✅
- SRE Tools (1 route) ✅
- Cache Clear (1 route) ✅
- Complaints (2 routes) ✅
- Cypress (2 routes) ✅
- Email Branding (4 routes) ✅
- Events (1 route) ✅
- Inbound Numbers (5 routes) ✅
- Inbound SMS (4 routes) ✅
- Service Invitations (3 routes) ✅
- Jobs (7 routes) ✅
- Letter Branding (4 routes) ✅
- Letters (1 stub) ✅
- Notifications v1 (3 routes) ✅
- Org Invites (3 routes) ✅
- Organisations (11 routes) ✅
- Provider Details (4 routes) ✅
- Reports (2 routes) ✅
- Service Callbacks (9 routes) ✅
- Services (41+ routes, including stubs) ✅
- Support (1 route) ✅
- Templates (9+ routes) ✅
- Template Categories (6 routes) ✅
- Template Folders (6 routes) ✅
- Template Statistics (2 routes) ✅
- Users (33+ routes) ✅
- Billing (6 routes, with aliases) ✅
- V2 Notifications (5-7 routes) ✅
- V2 Template (2-3 routes) ✅
- V2 Templates (1 route) ✅
- V2 Inbound SMS (1 route) ✅
- V2 OpenAPI (2 routes) ✅

---

## Missing from Spec (Routes in code, not documented)

### NEWSLETTER — 3 undocumented routes

Spec documents 3 newsletter routes. Code has 6 total:

1. **`POST /newsletter/update-language/<subscriber_id>`**
   - Auth: `requires_admin_auth`
   - Response: 200 — Updates subscriber language preference
   - **RISK**: Go implementor won't know this exists

2. **`GET /newsletter/send-latest/<subscriber_id>`**
   - Auth: `requires_admin_auth`
   - Response: 200 — Sends latest newsletter to subscriber
   - **RISK**: Go implementor won't know this exists

3. **`GET /newsletter/find-subscriber`**
   - Auth: `requires_admin_auth`
   - Response: 200 — Finds subscriber by query param
   - **RISK**: Go implementor won't know this exists

### PLATFORM STATS — 1 undocumented route

Spec documents 3 platform-stats routes. Code has 4:

1. **`GET /platform-stats/send-method-stats-by-service`**
   - Auth: `requires_admin_auth`
   - Query params: `start_date` (required), `end_date` (required) — YYYY-MM-DD
   - Response: 200 — Returns send method statistics by service
   - **RISK**: Go implementor won't implement this used endpoint

---

## Extra in Spec
**NONE** — All ~210 documented routes exist in the codebase.

---

## Auth Scheme Discrepancies
**NONE FOUND** — All decorators (`@requires_admin_auth`, `@requires_auth`, `@requires_sre_auth`, `@requires_no_auth`) are correctly applied and match spec claims.

| Decorator | Route Prefix | Enforcement | Spec Match |
|---|---|---|---|
| `requires_admin_auth` | Most `/service/*` | JWT via ADMIN_CLIENT_USER_NAME | ✅ |
| `requires_sre_auth` | `/sre-tools/*` | JWT via SRE_USER_NAME | ✅ |
| `requires_cache_clear_auth` | `/cache-clear` | JWT via CACHE_CLEAR_USER_NAME | ✅ |
| `requires_cypress_auth` | `/cypress` | JWT via CYPRESS_AUTH_USER_NAME, blocks on production | ✅ |
| `requires_auth` | `/v2/notifications/*` | JWT (service key) or API key | ✅ |
| `requires_no_auth` | `/_status`, `/v2/openapi-*` | None | ✅ |

---

## Status Code Discrepancies

Two minor gaps where spec omits the status code:

1. **`POST /service/{service_id}/api-key/revoke/{api_key_id}`**
   - Spec: status not documented
   - Code: returns **202** (Accepted)
   - **Impact**: Go implementor may return 200 instead of 202

2. **`POST /service/{service_id}/suspend`**
   - Spec: status not documented
   - Code: returns **204** (No Content)
   - **Impact**: Go implementor may return 200 instead of 204

---

## Stubs & Unimplemented Routes

### ⚠️ Letter-Contact Endpoints — Documented but unimplemented (5 routes)
- Routes exist in spec as functioning endpoints
- Code (`app/service/rest.py`) has only `pass` bodies for all 5
- **RISK**: Spec is misleading — Go implementor will implement fake functionality

### ⚠️ `/service/{id}/send-pdf-letter` — Documented but unimplemented
- Spec describes it; code has only `pass`
- **RISK**: Same misleading stub issue

Note: `GET /letters/returned` is correctly marked as stub in spec ✅

---

## RISK Items for Go Implementors

### 🔴 CRITICAL

1. **Newsletter has 3 undocumented routes that must be implemented**
   - `POST /newsletter/update-language/<subscriber_id>`
   - `GET /newsletter/send-latest/<subscriber_id>`
   - `GET /newsletter/find-subscriber`

2. **Platform Stats missing `/send-method-stats-by-service`**
   - Publicly accessible, existing clients may call it

### 🟡 HIGH

3. **5 letter-contact routes + `/send-pdf-letter` are spec-documented but code-stub-only**
   - Spec claims they work; code has `pass` bodies
   - Decision needed: implement in Go or explicitly mark as stubs

4. **Missing status codes for `revoke` (202) and `suspend` (204)**
   - Wire format compatibility requires exact status codes

---

## Endpoint Count Summary

| Category | Spec | Code | Variance |
|---|---|---|---|
| Admin endpoints | ~195 | ~200 | +5 undocumented |
| V2 public endpoints | ~15 | ~15 | ✅ |
| **TOTAL** | **~210** | **~215** | **+5** |

---

## Recommendations

1. Add 3 newsletter routes to spec
2. Add 1 platform-stats route to spec
3. Document status codes for revoke (202) and suspend (204)
4. Mark letter-contact stubs and send-pdf-letter explicitly as "stub/not implemented" in spec
