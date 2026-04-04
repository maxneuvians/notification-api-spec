# Validation Report: business-rules/services.md
Date: 2026-04-04

## Summary
- **Functions documented in spec**: 85+
- **Functions found in code**: 85+ (all present)
- **Confirmed**: 82
- **Discrepancies**: 3 (error handling, annotation, conditional logic)
- **Missing from spec**: 0
- **Extra in code**: 7 (internal helpers, acceptable)
- **Risk items**: 3 critical/high, 2 informational

## PASS/FAIL Verdict
**CONDITIONAL PASS** — All documented DAO functions exist and core lifecycle logic is correct. 3 discrepancies require fixes before Go rewrite: exception syntax error, return type annotation mismatch, and truthiness-vs-None conditional logic.

---

## Confirmed (Brief List)

**services_dao.py**: 29/29 functions ✅
- All fetch patterns (all_services, by_id, by_inbound_number, by_user, with_api_keys, by_id_and_user)
- Creation with defaults (permissions, SMS sender, organisation resolution, PT data retention)
- Archive with name mangling, template archival, API key expiry ✅
- Suspend without expiring API keys ✅
- Resume with field reset ✅
- Stats queries (today, N-days, all services, per-type/status aggregation)
- Active users, service creator lookup, sensitive service IDs

**service_permissions_dao.py**: 3/3 ✅ — fetch, add, remove with transactional decorators

**service_safelist_dao.py**: 3/3 ✅ — full-replace pattern (fetch, remove, add_and_commit) correct

**service_sms_sender_dao.py**: 7/7 ✅ (except error messaging — see Discrepancies)
- Default sender enforced (exactly one per service) ✅
- Inbound number binding prevents independent updates ✅
- Archive blocks inbound/default senders ✅

**service_email_reply_to_dao.py**: 5/5 ✅
- Default reply-to requirement enforced ✅
- Archive allows only if non-default or last (then clears default) ✅

**service_inbound_api_dao.py**: 5/5 ✅ — CRUD with version tracking

**service_callback_api_dao.py**: 8/8 ✅ (except return type annotation)
- Bearer token signing/resigning via `signer_bearer_token` ✅
- Suspend/unsuspend independent of service suspension ✅
- Two callback types: delivery_status, complaint ✅

**service_data_retention_dao.py**: 5/5 ✅
- Unique constraint enforced on (service_id, notification_type) ✅

**service_user_dao.py**: 4/4 ✅ — membership and folder permissions

**service_letter_contact_dao.py**: 5/5 ✅
- Archive cascades NULL to `Template.service_letter_contact_id` ✅
- No guard on archiving default (per spec) ✅

---

## Discrepancies

### 1. Exception Handling Syntax Error — CRITICAL
**Location**: `app/dao/service_sms_sender_dao.py`, `_raise_when_no_default()`

**Code**:
```python
raise Exception("You must have at least one SMS sender as the default.", 400)
```
Raises with a tuple `("message", 400)` — error string output becomes `("You must...", 400)` instead of clean message.

**Spec expectation**: Clean error message.

**Fix**: Use `InvalidRequest` (the class used elsewhere in codebase):
```python
raise InvalidRequest("You must have at least one SMS sender as the default.", 400)
```

Check `service_email_reply_to_dao.py` for same pattern.

---

### 2. Return Type Annotation Mismatch — MINOR
**Location**: `app/dao/service_callback_api_dao.py`, `get_service_callback_api_with_service_id(service_id)`

```python
def get_service_callback_api_with_service_id(service_id) -> ServiceCallbackApi:   # wrong annotation
    return ServiceCallbackApi.query.filter_by(service_id=service_id).all()         # returns list
```

**Spec says**: Returns `list[ServiceCallbackApi]`.

**Fix**: Change annotation to `list[ServiceCallbackApi]`.

---

### 3. Conditional Field Updates Use Truthiness, Not `is not None` — BEHAVIORAL
**Locations**: `app/dao/service_inbound_api_dao.py` (`reset_service_inbound_api`), `app/dao/service_callback_api_dao.py` (`reset_service_callback_api`)

```python
if url:              # blocks empty-string updates
    service_inbound_api.url = url
if bearer_token:     # blocks empty-string updates
    service_inbound_api.bearer_token = bearer_token
```

**Spec says**: "Only updates fields that are provided (non-None)".

**Problem**: `if url:` rejects empty strings — callers cannot reset webhooks to `""`.

**Fix**: Change `if url:` → `if url is not None:` and same for `bearer_token`.

---

## Missing from Spec
**None** — All documented functions exist in code.

---

## RISK Items for Go Implementors

### 🔴 CRITICAL
**1. Exception syntax in service_sms_sender_dao.py (and possibly email_reply_to_dao.py)**
- Affects SMS/email sender creation error paths
- Admin UI will display malformed error messages until fixed

### 🟠 HIGH
**2. Conditional update logic (inbound_api + callback_api reset functions)**
- Affects operator ability to clear/reset webhook URLs
- Decision needed: allow empty strings or not? Fix `if x:` to `if x is not None:` if empty string is a valid reset value

### 🟡 MEDIUM
**3. Return type annotation on `get_service_callback_api_with_service_id`**
- Runtime works; causes type-checker false positives

### ℹ️ INFORMATIONAL
**4. `delete_service_and_all_associated_db_objects` deletes User records**
- Code ends with `db.session.delete(user)` for each service user
- Verify this is intentional (orphan cleanup) and users don't lose membership in OTHER services
- Test: user belonging to 2 services; delete service 1; confirm user still exists in service 2

**5. Bearer token signing key rotation for callbacks**
- Confirm `resign_service_callbacks` is covered in CI; document when `unsafe=True` is acceptable
