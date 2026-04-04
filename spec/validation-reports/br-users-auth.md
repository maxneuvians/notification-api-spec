# Validation Report: business-rules/users-auth.md
Date: 2026-04-04

## Summary
- **Spec DAO functions**: 32 across 7 files
- **Code functions**: All 32 present + 8 undocumented helpers
- **Confirmed**: 32/32
- **Discrepancies**: 2 (auth types scope, `get_user_by_id` nil-argument behavior)
- **Missing from spec**: 4 (FIDO2 helpers + 2 user lifecycle functions)
- **Risk items**: 3

## Verdict
**CONDITIONAL PASS** — All core auth/user/permissions/invitation DAOs present. Two gaps in auth type documentation must be resolved before Go.

---

## Confirmed

**users_dao.py (20/20)**: `create_secret_code`, `save_user_attribute`, `save_model_user`, `create_user_code`, `get_user_code`, `delete_codes_older_created_more_than_a_day_ago`, `use_user_code`, `delete_model_user`, `delete_user_verify_codes`, `count_user_verify_codes`, `verify_within_time`, `get_user_by_id`, `get_user_by_email`, `get_users_by_partial_email`, `increment_failed_login_count`, `reset_failed_login_count`, `update_user_password`, `get_user_and_accounts`, `dao_archive_user`, `dao_deactivate_user` ✅

**permissions_dao.py (6/6)**: `add_default_service_permissions_for_user`, `remove_user_service_permissions`, `remove_user_service_permissions_for_all_services`, `set_user_service_permission`, `get_permissions_by_user_id`, `get_permissions_by_user_id_and_service_id` ✅

**fido2_key_dao.py (7/7)**: `delete_fido2_key`, `get_fido2_key`, `list_fido2_keys`, `save_fido2_key`, `create_fido2_session`, `get_fido2_session`, `delete_fido2_session` ✅

**login_event_dao.py (2/2)**: `list_login_events` (LIMIT 3, DESC created_at), `save_login_event` ✅

**invited_user_dao.py (5/5)** and **invited_org_user_dao.py (5/5)**: All invitation CRUD + cleanup functions ✅

**api_key_dao.py (9/9)**: `resign_api_keys`, `save_model_api_key`, `expire_api_key`, `update_last_used_api_key`, `update_compromised_api_key_info`, `get_api_key_by_secret`, `get_model_api_keys`, `get_unsigned_secrets`, `get_unsigned_secret` ✅

---

## Discrepancies

### 1. Auth Type Scope Incomplete — MODERATE
**Spec**: Documents Bearer JWT and ApiKey-v1 authentication schemes.
**Code**: Supports 4 types: `JWT`, `ApiKey-v1`, `CacheClear-v1`, `Cypress-v1`.
**Impact**: `CacheClear-v1` and `Cypress-v1` are internal-use schemes not documented; Go must implement them to serve `/cache-clear` and `/cypress` endpoints correctly.

### 2. `get_user_by_id` Nil-Argument Behavior — MINOR
**Spec**: Returns single user by UUID.
**Code**: If `user_id=None` is passed, returns ALL users (no filter applied).
**Impact**: Accidental nil can cause data leakage; Go implementation should guard against nil user_id.

---

## Missing from Spec

| Function | Location | Purpose |
|---|---|---|
| `get_services_for_all_users` | `users_dao.py` | Bulk retention/reporting query |
| `deserialize_fido2_key` | `fido2_key_dao.py` | Backward compat for pre-upgrade fido2 0.9.x credentials |
| `decode_and_register` | `fido2_key_dao.py` | WebAuthn registration completion |
| FIDO2 session state helpers | `fido2_key_dao.py` | `create_fido2_session`, `get_fido2_session`, `delete_fido2_session` (create_fido2_session documented, others not) |

---

## RISK Items for Go Implementors

### 🔴 CRITICAL
**1. Undocumented auth types `CacheClear-v1` and `Cypress-v1`**
- Required for `/cache-clear` and `/cypress` endpoints
- Must not be exposed externally; require separate user credentials in config
- Add to spec before Go auth middleware implementation

### 🟡 MODERATE
**2. FIDO2 backward compatibility**
- `deserialize_fido2_key` handles legacy credentials from before library upgrade
- Go rewrite can skip this if migrating to fresh FIDO2 keys; document the decision

### 🟡 MODERATE
**3. `get_user_by_id` nil returns all users**
- Guard against nil input; Go should return error or empty result on nil user_id
