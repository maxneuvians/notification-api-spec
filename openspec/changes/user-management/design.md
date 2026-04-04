## Context

User management covers CRUD, password/MFA, verify-code OTP flows, service invite flows, FIDO2 key
management, permission assignment, and login-event auditing. Auth middleware itself lives in
`authentication-middleware`; this change implements only the user-facing surface.

## Goals / Non-Goals

**Goals:** User CRUD, archive/deactivate/activate lifecycle, password management, OTP verify-code
flows, FIDO2 registration and authentication, service invite flows (create/update/decode), permission
replacement, login events.

**Non-Goals:** JWT issuance (admin UI responsibility), session token management, organisation invite
creation (covered in `organisation-management`), API key lifecycle.

---

## Decisions

### Password hashing — use `pkg/crypto` for all operations

All password operations MUST go through `pkg/crypto` (bcrypt wrapper). The `User.password` setter
applies bcrypt on assignment; raw passwords are never stored. `VerifyCode.code` setter applies bcrypt
on assignment as well — verify codes are never stored in plaintext.

`update_user_password` must additionally set `password_changed_at = now()` and clear
`password_expired = false`.

### Verify codes — hashed, one-time-use, 30-second flood guard, 10-code cap

1. **Hashed before storage**: `VerifyCode.code` setter applies bcrypt. The raw 5-digit value is never
   persisted.
2. **One-time-use**: `MarkVerifyCodeUsed` is called immediately on the first successful verification.
   A row with `code_used=true` MUST NOT be accepted again (→ 400).
3. **Flood guard**: Before issuing a new code, check `CountRecentVerifyCodes` with a 30-second window.
   If the count is non-zero, return 204 without creating a new code and without dispatching a new
   notification.
4. **10-code cap**: Before issuing, check `CountActiveVerifyCodes`. If ≥ 10 unexpired/unused codes
   already exist, return 204 without creating a new code.
5. **TTL**: 30 minutes from `created_at`. Expired codes return 400 (not 404) and increment
   `failed_login_count`.
6. **E2E bypass**: When `NOTIFY_ENVIRONMENT=development`, the request host is `localhost:3000` or
   `dev.local`, and the user's email starts with `CYPRESS_EMAIL_PREFIX`, any submitted code is
   accepted (→ 204). This path MUST be absent in production builds.

### Verify-code vs verify-2fa — different lockout semantics

`POST /verify-code`:
- Enforces account lockout: `failed_login_count >= 10` → 404 even for a correct code; does not mark
  code used; does not change count.
- Increments `failed_login_count` on: not-found code (404) and expired code (400).
- Does NOT increment on: missing code (400) or already-used code (400).
- On success: reset count to 0, set `logged_in_at`, generate new `current_session_id`.

`POST /verify-2fa`:
- Does NOT enforce lockout — no `failed_login_count >= 10` check.
- Does NOT increment `failed_login_count` under any failure mode.
- Same success side-effects as `verify-code`.

### FIDO2 session — consume-once, one-per-user

`create_fido2_session(user_id, state)` deletes any existing `Fido2Session` for the user before
inserting a new one. Only one active ceremony per user at any time.

`get_fido2_session(user_id)` retrieves the session state AND immediately deletes the row (consume-
once semantics). The handler must not assume the session persists after a successful read.

`decode_and_register` returns a base64-pickle of `credential_data` (from fido2 library's
`AttestedCredentialData`). The stored value is passed to `deserialize_fido2_key` at authentication
time. `deserialize_fido2_key` uses `_Fido2CredentialUnpickler` to remap the old `fido2.ctap2` module
path to `fido2.webauthn` for backward compatibility with keys registered under fido2 0.9.x.

### User archive — transactional, guarded, irreversible email rename

`dao_archive_user` is wrapped in `@transactional` and:
1. Checks `user_can_be_archived(user)` — raises `InvalidRequest(400)` if any active service the user
   belongs to has no other active user who holds `manage_settings`.
2. Removes all `user_to_service` rows and all `permissions` for the user.
3. Clears org memberships.
4. Renames email to `_archived_{YYYY-MM-DD}_{original_email}` (use `get_archived_email_address`).
5. Sets `mobile_number = null`, `auth_type = email_auth`.
6. Sets `password` to a random UUID (password invalidated).
7. Sets `current_session_id = "00000000-0000-0000-0000-000000000000"` (session invalidated).
8. Sets `state = inactive`.

If any step fails, the entire transaction is rolled back. Returns 404 for non-existent user, 400 for
the archivability check failure.

### User deactivate vs archive

`dao_deactivate_user` (→ `POST /user/{id}/deactivate`):
- Does NOT rename the email address (kept intact).
- Sets `state = inactive`, removes all service/org memberships and permissions, invalidates session.
- Raises `InvalidRequest` if user is already `inactive`.
- Sends deactivation notification email to the user.
- Service-level side-effects:
  - A **live** service that becomes empty (0 remaining active members) → `active=False`, NO
    `suspended_at`.
  - A **live** service with exactly 1 remaining active member → `active=False` +
    `suspended_at=now` + team suspension email.
  - A **trial** service that becomes empty → `active=False`, no suspension.
  - A **trial** or **live** service with 2+ remaining active members → no change.
- Entire flow wrapped in a transaction; 500 on unexpected error.

### blocked=true forces session invalidation

When `save_user_attribute` receives `blocked: true` in its update dict, `current_session_id` is
unconditionally overwritten with the nil UUID `"00000000-0000-0000-0000-000000000000"` in the same
UPDATE statement. No notification is sent from the update endpoint when only `blocked` is changed.
`check_password` returns `false` for blocked users regardless of password correctness.

### Failed login count — precise increment rules

| Event | `failed_login_count` change |
|---|---|
| `verify-password` wrong password | +1 |
| `verify-code` code not found (404) | +1 |
| `verify-code` code expired (400) | +1 |
| `verify-code` missing code (400) | no change |
| `verify-code` already used (400) | no change |
| `verify-code` lockout (400) | no change |
| `verify-2fa` any failure | no change |
| `verify-password` success | reset to 0 |
| `verify-code` success | reset to 0 |

### Permission assignment — replace, not append

`POST /user/{id}/permissions/{service_id}` MUST atomically delete all existing permissions for the
user+service pair and insert the new set (replace semantics). Folder permissions on the
`service_user` join record are also replaced. The operation MUST NOT affect permissions or folder
permissions for any other service.

### GetUserByID nil-safety

The Python DAO `get_user_by_id(user_id=None)` returns ALL users when called with `None`. The Go
repository function MUST reject a nil or zero-value UUID before calling the DB and return a `not
found` error instead of silently returning all users.

### Invite token

Invite tokens are URL-safe signed tokens created with `generate_token` (using `SECRET_KEY`). The
handler for `GET /invite/{type}/{token}` must:
1. Verify the token signature and TTL.
2. Extract the invited-user ID.
3. Fetch the invited user record.
4. Return 400 `{"invitation": "invitation expired"}` on TTL failure.
5. Return 400 `{"invitation": "bad invitation link"}` on signature/parse failure.
6. Return 404 if the extracted ID has no corresponding record.

### Contact-request PT branch — untested Python path

The PT-service + `FF_PT_SERVICE_SKIP_FRESHDESK=True` branch (returns 201, calls
`email_freshdesk_ticket_pt_service`) has no Python test coverage. The Go implementation MUST add
test coverage for this path when writing handler tests.
