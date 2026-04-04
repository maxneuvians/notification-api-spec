# Validation Report: behavioral-spec/services.md
Date: 2026-04-04

## Summary
- **Contracts in spec**: ~35
- **Contracts with test coverage**: ~33
- **CONFIRMED**: 33
- **DISCREPANCIES**: 0
- **UNCOVERED**: 2 edge cases
- **EXTRA BEHAVIORS in tests**: 1 (framework convention, not a gap)
- **Risk items**: 3

## Verdict
**CONFIRMED** — No discrepancies. All major contracts covered. Three risks for Go implementors around suspension semantics, Salesforce integration, and NHS branding auto-assignment.

---

## Confirmed (concise)

- GET /service: ordering, `only_active`, `user_id` scoping, `detailed`, `find-by-name` case-insensitive ✅
- POST /service: 201, required fields, duplicate name/email_from → 400, organisation auto-assign (longest domain match), NHS branding auto-assign, ServiceSmsSender created ✅
- Service go-live transition: SERVICE_BECAME_LIVE notification + Salesforce `engagement_update` ✅
- Service message_limit/annual_limit changes: email notification + Redis cache clear ✅
- POST/DELETE /service/{id}/users: 201/204, permission list, folder permissions, blocked if last user, blocked if last with manage_settings ✅
- API key CRUD: 201/202, name format, revoke sets expiry_date ✅
- POST /service/{id}/archive: 204 idempotent, name mangling, key revocation, template archival, history v2, SERVICE_DEACTIVATED notification ✅
- POST /service/{id}/suspend: 204, sets active=False, does NOT expire keys ✅
- POST /service/{id}/resume: 204, sets active=True, does not re-activate revoked keys ✅
- Safelist CRUD: full-replace pattern, validation per entry ✅
- Data retention CRUD: duplicate type → 400, separate per type ✅
- Service notifications list: all filter params, CSV format ✅
- Email reply-to CRUD: first must be default, duplicate → 400, cannot de-default sole reply-to ✅
- SMS sender CRUD: default enforcement, inbound_number_id binding ✅
- Service callback/inbound API CRUD: bearer token signed, version tracking ✅
- Service statistics: today_only vs 7-day, monthly usage, template statistics monthly ✅
- `GET /service/is-name-unique`, `GET /service/is-email-from-unique`: case + punctuation insensitive ✅

---

## Discrepancies
None found.

---

## Uncovered Contracts

1. POST /service/{id}/sms-sender with multiple existing senders + `inbound_number_id`: "one existing → replace, multiple existing → append" branch logic has no explicit test
2. `delete_service_and_all_associated_db_objects` cascade exhaustiveness — cascade logic tested indirectly but no dedicated full-cascade test

---

## RISK Items for Go Implementors

### 🔴
**1. Service suspension does NOT expire API keys**
- `POST /service/{id}/suspend` sets `active=False` but keys remain expired=False
- Go must check BOTH `service.active=True` AND `api_key.expiry_date IS NULL` before allowing sends
- Resuming a suspended service silently re-enables all previously-valid keys

### 🟡
**2. Salesforce integration (4 call points)**
- `engagement_create` (service creation), `engagement_update` (go-live, name change), `engagement_delete_contact_role` (user removal)
- Tests mock Salesforce entirely — no retry/idempotency behavior tested
- Go must implement timeouts and graceful degradation; Salesforce unavailability should not block service creation

### 🟡
**3. NHS branding auto-assignment is data-dependent**
- Auto-assigns if user email domain matches NHS pattern AND NHS branding record exists in DB
- NHS domain patterns and case sensitivity not fully specified
- If branding record missing, assignment silently skipped
- Extract NHS domain list to a shared config; test missing-branding path
