# Business Rules: Users & Authentication

## Overview

This domain covers user accounts, authentication, and access control for notification-api. It spans
four authentication schemes (Bearer JWT, ApiKey-v1, admin/SRE/Cypress JWTs), two MFA mechanisms
(email/SMS OTP and FIDO2/WebAuthn), service-scoped permissions, invite flows for both services and
organisations, API key lifecycle, and login-event auditing.

Source files analysed:
- `app/dao/users_dao.py`
- `app/dao/permissions_dao.py`
- `app/dao/fido2_key_dao.py`
- `app/dao/login_event_dao.py`
- `app/dao/invited_user_dao.py`
- `app/dao/invited_org_user_dao.py`
- `app/dao/api_key_dao.py`
- `app/authentication/auth.py`
- `app/authentication/bearer_auth.py`

---

## Data Access Patterns

### users_dao.py

#### `create_secret_code()`
- **Purpose:** Generate a random 5-digit numeric OTP.
- **Query type:** None (pure computation).
- **Key filters/conditions:** Uses `SystemRandom` (cryptographically secure).
- **Returns:** `str` — e.g. `"04821"`.
- **Notes:** Digits are sampled individually from 0–9; leading zeros are preserved.

#### `save_user_attribute(usr, update_dict={})`
- **Purpose:** Atomically update one or more User columns by primary key.
- **Query type:** UPDATE `users` WHERE `id = usr.id`.
- **Key filters/conditions:** If `"blocked": True` is in the dict, `current_session_id` is forced to `"00000000-0000-0000-0000-000000000000"` (instant session invalidation).
- **Returns:** None.
- **Notes:** Direct `.update()` — bypasses SQLAlchemy ORM events.

#### `save_model_user(usr, update_dict={}, pwd=None)`
- **Purpose:** INSERT a new user or UPDATE existing columns.
- **Query type:** INSERT or UPDATE `users`.
- **Key filters/conditions:** If `pwd` is set, hashes it and updates `password_changed_at`. Strips `id` and `password_changed_at` from `update_dict` before applying.
- **Returns:** None.
- **Notes:** Primary insert path for new users. Password hashing is done by the `User.password` setter (bcrypt).

#### `create_user_code(user, code, code_type)`
- **Purpose:** Persist a hashed OTP with a 30-minute TTL.
- **Query type:** INSERT `verify_codes`.
- **Key filters/conditions:** `expiry_datetime = now + 30 min`; `code_type` ∈ `{"email", "sms"}`.
- **Returns:** `VerifyCode` instance.
- **Notes:** The raw `code` is hashed by the `VerifyCode.code` setter before storage.

#### `get_user_code(user, code, code_type)`
- **Purpose:** Find a matching, non-used verify code for the user.
- **Query type:** SELECT `verify_codes` WHERE `user = user AND code_type = code_type` ORDER BY `created_at DESC`.
- **Key filters/conditions:** Iterates results newest-first; returns first row where `check_code(code)` (bcrypt) passes.
- **Returns:** `VerifyCode` or `None`.
- **Notes:** Does not filter out expired or used codes in the query — the caller must check. Newest-first ordering minimises hash comparisons in the common case.

#### `delete_codes_older_created_more_than_a_day_ago()`
- **Purpose:** Scheduled cleanup of stale OTPs.
- **Query type:** DELETE `verify_codes` WHERE `created_at < now - 24h`.
- **Returns:** `int` rows deleted.

#### `use_user_code(id)`
- **Purpose:** Mark an OTP as consumed after successful verification.
- **Query type:** SELECT `verify_codes` by PK, then UPDATE `code_used = True`.
- **Returns:** None.

#### `delete_model_user(user)`
- **Purpose:** Hard-delete a user record.
- **Query type:** DELETE `users`.
- **Returns:** None.
- **Notes:** Use only for test teardown; prefer `dao_archive_user` or `dao_deactivate_user` in production paths.

#### `delete_user_verify_codes(user)`
- **Purpose:** Remove all OTPs associated with a user.
- **Query type:** DELETE `verify_codes` WHERE `user = user`.
- **Returns:** None.

#### `count_user_verify_codes(user)`
- **Purpose:** Count pending (non-expired, unused) OTPs for rate-limiting.
- **Query type:** COUNT `verify_codes` WHERE `user = user AND expiry_datetime > now AND code_used IS False`.
- **Returns:** `int`.

#### `verify_within_time(user, age=timedelta(seconds=30))`
- **Purpose:** Determine whether a code was recently created (flood guard).
- **Query type:** COUNT `verify_codes` WHERE `user = user AND expiry_datetime > now AND code_used IS False AND created_at > now - age`.
- **Returns:** `int`.
- **Notes:** Default `age` is 30 seconds. Callers use the non-zero result to reject duplicate OTP requests.

#### `get_user_by_id(user_id=None)`
- **Purpose:** Fetch a single user by UUID or all users.
- **Query type:** SELECT `users` WHERE `id = user_id` (`.one()`), or SELECT all if no id.
- **Returns:** `User` or `list[User]`.

#### `get_user_by_email(email)`
- **Purpose:** Case-insensitive lookup by email address.
- **Query type:** SELECT `users` WHERE `lower(email_address) = lower(email)` (`.one()`).
- **Returns:** `User`.

#### `get_users_by_partial_email(email)`
- **Purpose:** Admin search by partial email.
- **Query type:** SELECT `users` WHERE `email_address ILIKE '%email%'`.
- **Key filters/conditions:** Special characters (`%`, `_`, etc.) escaped before building the pattern.
- **Returns:** `list[User]`.

#### `increment_failed_login_count(user)`
- **Purpose:** Record a failed login attempt.
- **Query type:** UPDATE `users` SET `failed_login_count = failed_login_count + 1`.
- **Returns:** None.

#### `reset_failed_login_count(user)`
- **Purpose:** Reset counter after successful authentication.
- **Query type:** UPDATE `users` SET `failed_login_count = 0` (only when currently > 0).
- **Returns:** None.

#### `update_user_password(user, password)`
- **Purpose:** Set a new password after a password-reset flow.
- **Query type:** UPDATE `users`.
- **Key filters/conditions:** Sets `password` (triggers bcrypt hash), `password_changed_at = now`, `password_expired = False`.
- **Returns:** None.
- **Notes:** Comment in code notes this also resets failed login count (implied safe re-entry after reset).

#### `get_user_and_accounts(user_id)`
- **Purpose:** Fetch a user with all associated services and organisations in a single query.
- **Query type:** SELECT `users` with `joinedload` on `services`, `organisations`, `organisations.services`, `services.organisation`.
- **Returns:** `User` (with relationships eagerly loaded).
- **Notes:** Used by the "user accounts" endpoint to avoid N+1 queries.

#### `dao_archive_user(user)`
- **Purpose:** Permanently decommission a user while retaining the row for audit.
- **Query type:** Multiple writes within `@transactional`.
- **Key filters/conditions:** Only allowed when `user_can_be_archived(user)` returns True (see below).
- **Returns:** None (raises on failure).
- **Notes:** Side effects — removes all service permissions, removes all `user_to_service` rows, clears org memberships, resets `auth_type = email_auth`, renames email to `_archived_{YYYY-MM-DD}_{email}`, nulls `mobile_number`, sets `password = random UUID`, invalidates session, sets `state = inactive`.

#### `user_can_be_archived(user)`
- **Purpose:** Guard for `dao_archive_user`.
- **Query type:** Read-only inspection of loaded relationships.
- **Key filters/conditions:** For each active service the user belongs to: (1) at least one other active user must exist, AND (2) at least one other active user must hold `manage_settings`.
- **Returns:** `bool`.

#### `get_archived_email_address(email_address)`
- **Purpose:** Derive the archival email string.
- **Returns:** `str` — `"_archived_{YYYY-MM-DD}_{original_email}"`.

#### `get_services_for_all_users()`
- **Purpose:** Bulk retention/reporting query — all (user, email, [service_ids]) tuples.
- **Query type:** SELECT `users` JOIN `user_to_service` JOIN `services` WHERE `users.state = active AND services.active = True AND services.restricted = False AND services.research_mode = False` GROUP BY `users.id, users.email_address`.
- **Returns:** `list[(user_id, email_address, [service_id, ...])]` (namedtuple-like rows).

#### `dao_deactivate_user(user_id)`
- **Purpose:** Deactivate a user without archiving (email address is NOT renamed).
- **Query type:** Multiple writes (no `@transactional` decorator — relies on implicit session).
- **Key filters/conditions:** Raises `InvalidRequest` if user is already inactive.
- **Returns:** `User`.
- **Notes:** Same side effects as `dao_archive_user` except email is kept intact.

---

### permissions_dao.py

#### `add_default_service_permissions_for_user(user, service)`
- **Purpose:** Grant all 8 default permissions when a user joins a service.
- **Query type:** INSERT `permissions` × 8 (no intermediate commit — `_commit=False`).
- **Returns:** None.
- **Notes:** Default set: `manage_users`, `manage_templates`, `manage_settings`, `send_texts`, `send_emails`, `send_letters`, `manage_api_keys`, `view_activity`.

#### `remove_user_service_permissions(user, service)`
- **Purpose:** Revoke all permissions for a user on a specific service.
- **Query type:** DELETE `permissions` WHERE `user = user AND service = service`.
- **Returns:** None.

#### `remove_user_service_permissions_for_all_services(user)`
- **Purpose:** Revoke all service permissions for a user (used during deactivation/archival).
- **Query type:** DELETE `permissions` WHERE `user = user`.
- **Returns:** None.

#### `set_user_service_permission(user, service, permissions, _commit=False, replace=False)`
- **Purpose:** Set or replace a user's permissions on a service.
- **Query type:** Optional DELETE then INSERT `permissions`.
- **Key filters/conditions:** If `replace=True`, existing permissions are deleted first. On exception, rolls back if `_commit=True`.
- **Returns:** None.

#### `get_permissions_by_user_id(user_id)`
- **Purpose:** Fetch all service permissions for a user (used by `User.get_permissions()`).
- **Query type:** SELECT `permissions` WHERE `user_id = user_id` JOIN `services` WHERE `services.active = True`.
- **Returns:** `list[Permission]`.
- **Notes:** Inactive services are excluded — permissions appear to evaporate when a service is archived.

#### `get_permissions_by_user_id_and_service_id(user_id, service_id)`
- **Purpose:** Fetch permissions for a user scoped to one active service.
- **Query type:** SELECT `permissions` WHERE `user_id = user_id` JOIN `services` WHERE `id = service_id AND active = True`.
- **Returns:** `list[Permission]`.

#### `get_team_members_with_permission(service_id, permission)`
- **Purpose:** Find active users on a service who hold a specific permission.
- **Query type:** SELECT `permissions` WHERE `service_id = service_id AND permission = permission` JOIN `users` WHERE `state = active`.
- **Returns:** `list[User]`.
- **Notes:** Used for e.g. finding all users who can manage team membership.

---

### fido2_key_dao.py

#### `delete_fido2_key(user_id, id)`
- **Purpose:** Remove a registered security key.
- **Query type:** DELETE `fido2_keys` WHERE `user_id = user_id AND id = id`.
- **Returns:** None.

#### `get_fido2_key(user_id, id)`
- **Purpose:** Fetch a specific security key by owner and key ID.
- **Query type:** SELECT `fido2_keys` WHERE `user_id = user_id AND id = id` (`.one()`).
- **Returns:** `Fido2Key`.

#### `list_fido2_keys(user_id)`
- **Purpose:** List all security keys owned by a user.
- **Query type:** SELECT `fido2_keys` WHERE `user_id = user_id` ORDER BY `created_at ASC`.
- **Returns:** `list[Fido2Key]`.

#### `save_fido2_key(fido2_key)`
- **Purpose:** Persist a newly registered FIDO2 credential.
- **Query type:** INSERT `fido2_keys` (`@transactional`).
- **Returns:** None.

#### `create_fido2_session(user_id, session)`
- **Purpose:** Store a WebAuthn challenge state for the current registration/authentication ceremony.
- **Query type:** DELETE existing `fido2_sessions` for user, then INSERT new row (`@transactional`).
- **Key filters/conditions:** `session` is JSON-serialised before storage. One session per user — any previous session is destroyed.
- **Returns:** None.

#### `delete_fido2_session(user_id)`
- **Purpose:** Invalidate an in-flight FIDO2 session.
- **Query type:** DELETE `fido2_sessions` WHERE `user_id = user_id`.
- **Returns:** None.

#### `get_fido2_session(user_id)`
- **Purpose:** Retrieve and consume the FIDO2 session state (one-time read).
- **Query type:** SELECT `fido2_sessions` (`.one()`), then DELETE.
- **Returns:** `dict` (decoded JSON).
- **Notes:** The session is deleted immediately after retrieval. Callers must not assume it persists.

#### `deserialize_fido2_key(serialized_key)`
- **Purpose:** Decode a stored credential from the DB for assertion verification.
- **Returns:** `AttestedCredentialData` (fido2 library type).
- **Notes:** Uses a custom `_Fido2CredentialUnpickler` that remaps old fido2 0.9.x module paths (`fido2.ctap2`) to current paths (`fido2.webauthn`) for backward compatibility with keys registered under the old library.

#### `decode_and_register(data, state)`
- **Purpose:** Complete a WebAuthn registration ceremony and return serialised credential data.
- **Key filters/conditions:** Calls `Config.FIDO2_SERVER.register_complete(state, client_data, att_obj)` using the attested credential data from the client.
- **Returns:** `str` — base64-encoded pickle of `auth_data.credential_data`.
- **Notes:** The returned value is stored in `Fido2Key.key`.

---

### login_event_dao.py

#### `list_login_events(user_id)`
- **Purpose:** Retrieve recent login events for a user.
- **Query type:** SELECT `login_events` WHERE `user_id = user_id` ORDER BY `created_at DESC` LIMIT 3.
- **Returns:** `list[LoginEvent]` (at most 3).

#### `save_login_event(login_event)`
- **Purpose:** Record a login event.
- **Query type:** INSERT `login_events` (`@transactional`).
- **Returns:** None.

---

### invited_user_dao.py

#### `save_invited_user(invited_user)`
- **Purpose:** Persist a new service invitation.
- **Query type:** INSERT `invited_users`.
- **Returns:** None.

#### `get_invited_user(service_id, invited_user_id)`
- **Purpose:** Fetch an invitation scoped to a service.
- **Query type:** SELECT `invited_users` WHERE `service_id = service_id AND id = invited_user_id` (`.one()`).
- **Returns:** `InvitedUser`.

#### `get_invited_user_by_id(invited_user_id)`
- **Purpose:** Fetch an invitation by ID only (used during accept flow where service is implicit).
- **Query type:** SELECT `invited_users` WHERE `id = invited_user_id` (`.one()`).
- **Returns:** `InvitedUser`.

#### `get_invited_users_for_service(service_id)`
- **Purpose:** List all pending/accepted/cancelled invitations for a service.
- **Query type:** SELECT `invited_users` WHERE `service_id = service_id`.
- **Returns:** `list[InvitedUser]`.

#### `delete_invitations_created_more_than_two_days_ago()`
- **Purpose:** Scheduled cleanup of expired invitations.
- **Query type:** DELETE `invited_users` WHERE `created_at <= now - 2 days`.
- **Returns:** `int` rows deleted.

---

### invited_org_user_dao.py

#### `save_invited_org_user(invited_org_user)`
- **Purpose:** Persist a new organisation invitation.
- **Query type:** INSERT `invited_organisation_users`.
- **Returns:** None.

#### `get_invited_org_user(organisation_id, invited_org_user_id)`
- **Purpose:** Fetch an organisation invitation scoped to an org.
- **Query type:** SELECT `invited_organisation_users` WHERE `organisation_id = organisation_id AND id = invited_org_user_id` (`.one()`).
- **Returns:** `InvitedOrganisationUser`.

#### `get_invited_org_user_by_id(invited_org_user_id)`
- **Purpose:** Fetch an organisation invitation by ID only.
- **Query type:** SELECT `invited_organisation_users` WHERE `id = invited_org_user_id` (`.one()`).
- **Returns:** `InvitedOrganisationUser`.

#### `get_invited_org_users_for_organisation(organisation_id)`
- **Purpose:** List all invitations for an organisation.
- **Query type:** SELECT `invited_organisation_users` WHERE `organisation_id = organisation_id`.
- **Returns:** `list[InvitedOrganisationUser]`.

#### `delete_org_invitations_created_more_than_two_days_ago()`
- **Purpose:** Scheduled cleanup of expired org invitations.
- **Query type:** DELETE `invited_organisation_users` WHERE `created_at <= now - 2 days`.
- **Returns:** `int` rows deleted.

---

### api_key_dao.py

#### `resign_api_keys(resign, unsafe=False)`
- **Purpose:** Rotate the signing key used for API key secrets (key rotation maintenance).
- **Query type:** SELECT all `api_keys`, then conditional bulk UPDATE (`@transactional`).
- **Key filters/conditions:** If `resign=True`, new signatures are persisted. If `resign=False`, counts how many keys need resigning without writing. `unsafe=True` allows proceeding past `BadSignature` via `verify_unsafe`.
- **Returns:** None.

#### `save_model_api_key(api_key)`
- **Purpose:** Create a new API key.
- **Query type:** INSERT `api_keys` (`@transactional`, `@version_class`).
- **Key filters/conditions:** Assigns `id = uuid4()` and `secret = uuid4()` (the UUID is then signed via `ApiKey.secret` setter).
- **Returns:** None.

#### `expire_api_key(service_id, api_key_id)`
- **Purpose:** Revoke an API key by setting its expiry timestamp.
- **Query type:** SELECT `api_keys` WHERE `id = api_key_id AND service_id = service_id` (`.one()`), then UPDATE `expiry_date = now` (`@transactional`, `@version_class`).
- **Returns:** None.
- **Notes:** A key with `expiry_date` set is considered revoked; it will be rejected by `_auth_with_api_key`.

#### `update_last_used_api_key(api_key_id, last_used=None)`
- **Purpose:** Track when an API key was last used for security visibility.
- **Query type:** UPDATE `api_keys` SET `last_used_timestamp = timestamp` WHERE `id = api_key_id` (`@transactional`, rate-limited).
- **Key filters/conditions:** `synchronize_session=False` for performance. Rate-limited to once per 10 seconds per key via Redis to reduce write pressure on high-traffic services.
- **Returns:** None.

#### `update_compromised_api_key_info(service_id, api_key_id, compromised_info)`
- **Purpose:** Record compromise metadata (e.g., exposure details) on a key.
- **Query type:** UPDATE `api_keys` SET `compromised_key_info = compromised_info` (`@transactional`, `@version_class`).
- **Returns:** None.

#### `get_api_key_by_secret(secret, service_id=None)`
- **Purpose:** Look up an API key from a raw `ApiKey-v1` token.
- **Query type:** SELECT `api_keys` WHERE `_secret IN (sign_with_all_keys(token))` with eager load of `service`.
- **Key filters/conditions:**
  1. Token must start with `API_KEY_PREFIX`.
  2. Last 36 characters are the UUID token.
  3. Characters `[-73:-37]` (36 chars) must match `api_key.service_id` when total length ≥ 79.
- **Returns:** `ApiKey`.
- **Notes:** Tries all valid signing variants to support key rotation. Raises `ValueError` on format errors, `NoResultFound` if no matching key exists.

#### `get_model_api_keys(service_id, id=None)`
- **Purpose:** Fetch API keys for a service for display/management.
- **Query type:** SELECT `api_keys` WHERE `service_id = service_id AND (expiry_date IS NULL OR date(expiry_date) > now - 7 days)`. If `id` is given, returns only active (non-expired) key with that ID.
- **Returns:** `ApiKey` or `list[ApiKey]`.
- **Notes:** Recently expired keys (within 7 days) are included in the list view to support audit visibility.

#### `get_unsigned_secrets(service_id)`
- **Purpose:** Return plaintext secrets for JWT validation during `requires_auth()`.
- **Query type:** SELECT `api_keys` WHERE `service_id = service_id AND expiry_date IS NULL`.
- **Returns:** `list[str]` (unsigned/decrypted secrets).
- **Notes:** Marked as internal-only. Must never be exposed via HTTP responses.

#### `get_unsigned_secret(key_id)`
- **Purpose:** Return the plaintext secret for a single API key.
- **Query type:** SELECT `api_keys` WHERE `id = key_id AND expiry_date IS NULL` (`.one()`).
- **Returns:** `str`.

---

### authentication/auth.py

#### `get_auth_token(req)`
- **Purpose:** Parse the Authorization header and identify the auth scheme.
- **Returns:** `(auth_type, token)` tuple where `auth_type` ∈ `{jwt, api_key_v1, cache_clear_v1, cypress_v1}`.

#### `requires_no_auth()`
- **Purpose:** Decorator for endpoints that require no authentication.
- No-op. Used as a consistent placeholder.

#### `requires_auth()`
- **Purpose:** Authenticate service API requests (both JWT and ApiKey-v1).
- **Flow:**
  1. Check proxy header.
  2. If `ApiKey-v1` scheme → `_auth_by_api_key`.
  3. If `Bearer` scheme → extract issuer claim (`iss`), look up service in DB.
  4. Try each of the service's API key secrets with `decode_jwt_token`.
  5. On success, sets `g.service_id`, `g.authenticated_service`, `g.api_user`.
- **Notes:** Expired JWT raises `AuthError` (with clock-skew message); unrecognised/invalid signature is silently skipped for each key.

#### `requires_admin_auth()`
- **Purpose:** Authenticate internal admin API calls.
- **Flow:** Requires `Bearer` JWT; issuer must equal `ADMIN_CLIENT_USER_NAME`; validates against `ADMIN_CLIENT_SECRET`.

#### `requires_sre_auth()`
- **Purpose:** Authenticate SRE-only endpoints.
- Issuer = `SRE_USER_NAME`, secret = `SRE_CLIENT_SECRET`.

#### `requires_cache_clear_auth()`
- **Purpose:** Authenticate post-deployment cache-clear calls.
- Issuer = `CACHE_CLEAR_USER_NAME`, secret = `CACHE_CLEAR_CLIENT_SECRET`.

#### `requires_cypress_auth()`
- **Purpose:** Authenticate Cypress E2E test user creation (staging only).
- Issuer = `CYPRESS_AUTH_USER_NAME`, secret = `CYPRESS_AUTH_CLIENT_SECRET`.

#### `_auth_by_api_key(auth_token)`
- **Purpose:** Route an `ApiKey-v1` token through key lookup then common auth handler.

#### `_auth_with_api_key(api_key, service)`
- **Purpose:** Final validation after a matching key is found — checks `expiry_date`, then sets Flask `g` context.

#### `handle_admin_key(auth_token, secret)`
- **Purpose:** Validate a JWT against a fixed symmetric secret (used by all admin-class auth functions).

---

## Domain Rules & Invariants

### Authentication Schemes

| Scheme header | Type constant | Who uses it | Validated against |
|---|---|---|---|
| `Bearer` | `jwt` | External service clients (for API) | Each non-expired API key secret for the service (`iss` = service UUID) |
| `Bearer` | `jwt` | Internal admin | `ADMIN_CLIENT_SECRET` (`iss` = `ADMIN_CLIENT_USER_NAME`) |
| `Bearer` | `jwt` | SRE | `SRE_CLIENT_SECRET` |
| `Bearer` | `jwt` | Cache-clear | `CACHE_CLEAR_CLIENT_SECRET` |
| `Bearer` | `jwt` | Cypress (staging) | `CYPRESS_AUTH_CLIENT_SECRET` |
| `ApiKey-v1` | `api_key_v1` | External service clients (alternative to JWT) | Raw API key validated by format + DB lookup |

JWT tokens use HS256; the clock tolerance for expiry is 30 seconds. Tokens with unsupported
algorithms are logged and silently skipped (not rejected with an error) so that other keys in the
set can still be tried.

### API Key Secret Format

```
{API_KEY_PREFIX}{service_id_uuid36}{token_uuid36}
```

Total length ≥ 79 characters. Fields:

| Slice | Content |
|---|---|
| `[:len(prefix)]` | Config `API_KEY_PREFIX` (e.g. `"gcntfy-"`) |
| `[-73:-37]` | Service UUID (36 chars) |
| `[-36:]` | Token UUID (36 chars) |

The `_secret` column stores the token UUID signed with `signer_api_key`. Multiple signing variants
are tried during lookup to support key rotation (`sign_with_all_keys`).

### API Key Types

| Type | Behaviour |
|---|---|
| `normal` | Full access; notifications are actually sent |
| `team` | Can only send to recipient addresses/numbers that are members of the service team |
| `test` | Creates notification rows but never dispatches them; used for integration testing |

### User Auth Types (MFA second-factor)

| Value | Meaning | Prerequisite |
|---|---|---|
| `email_auth` | OTP delivered via email | Default; no additional fields required |
| `sms_auth` | OTP delivered via SMS | `mobile_number` must be non-null (DB CHECK constraint) |
| `security_key_auth` | FIDO2/WebAuthn assertion | At least one `Fido2Key` registered |

The DB has: `CHECK (auth_type = 'email_auth' OR mobile_number IS NOT NULL)`.

### Password / Verify-Code Flow

1. **Request code:** `create_user_code(user, code="NNNNN", code_type)` — inserts a hashed, 30-minute OTP. Multiple codes may exist; a user can have multiple unused non-expired codes simultaneously.
2. **Flood guard:** Before issuing a new code, callers check `verify_within_time(user)` (default window 30 s). A non-zero count means a code was already created recently.
3. **Verify code:** `get_user_code(user, code, code_type)` — iterates newest-first until bcrypt match found. Caller must separately check that the returned code is not expired and not already `code_used`.
4. **Consume code:** `use_user_code(id)` — sets `code_used = True`; subsequent calls to `get_user_code` will skip it.
5. **Pending-code limit:** `count_user_verify_codes(user)` counts non-expired, unused codes; callers use this to enforce a maximum (the limit itself is enforced in the calling endpoint layer, not this DAO).
6. **Cleanup:** `delete_codes_older_created_more_than_a_day_ago()` — scheduled task; removes codes created > 24 hours ago regardless of use status.

### FIDO2 / WebAuthn Flow

**Registration:**
1. `create_fido2_session(user_id, state)` — stores FIDO2_SERVER challenge state. Any existing session for the user is deleted first (only one active ceremony at a time per user).
2. Client responds with attestation data.
3. `decode_and_register(data, state)` — calls `FIDO2_SERVER.register_complete`; returns base64-pickle of `credential_data`.
4. `save_fido2_key(Fido2Key(user_id=.., name=.., key=<b64_pickle>))` — persists.

**Authentication:**
1. `create_fido2_session(user_id, state)` — stores assertion challenge state.
2. Client responds with assertion data.
3. Server calls `deserialize_fido2_key(key.key)` to reconstruct credential, then calls `FIDO2_SERVER.authenticate_complete` (done in endpoint layer, not this DAO).
4. `get_fido2_session(user_id)` is destructive — consuming the session is part of replay-attack prevention.

**Backward compatibility:** `_Fido2CredentialUnpickler` remaps `fido2.ctap2.AttestedCredentialData`
→ `fido2.webauthn.AttestedCredentialData` to support credentials registered under fido2 0.9.x.

### User Permissions

Eight service-scoped permission types:

| Permission | Meaning |
|---|---|
| `manage_users` | Add/remove/edit team members |
| `manage_templates` | Create and edit message templates |
| `manage_settings` | Edit service configuration |
| `send_texts` | Send SMS notifications |
| `send_emails` | Send email notifications |
| `send_letters` | Send letter notifications |
| `manage_api_keys` | Create and revoke API keys |
| `view_activity` | View notification history and reporting |

Plus the global `platform_admin` flag on the `User` model (not stored in `permissions` table).

**Invariants:**
- Permissions only materialise for **active** services (the DAO joins on `services.active = True`).
- The unique constraint `(service_id, user_id, permission)` prevents duplicates.
- Default grant (on service join): all 8 non-platform-admin permissions above.

### User Invitation Flow

**Service invitations (`invited_users`):**
1. An existing team member creates an invite; status starts as `pending`.
2. `permissions` column is a comma-separated string of permission names (see `get_permissions()` which calls `.split(",")`).
3. `auth_type` defaults to `email_auth` and is carried into the created account.
4. `folder_permissions` is a JSONB array of template-folder UUIDs.
5. Invitations expire (are deleted) 2 days after `created_at`.
6. Status transitions: `pending` → `accepted` | `cancelled`.

**Organisation invitations (`invited_organisation_users`):**
1. Same lifecycle (status: `pending` → `accepted` | `cancelled`).
2. No `permissions` field — org membership carries no fine-grained permissions.
3. Deleted 2 days after creation.

### Login Events

- Most recent 3 events are returned per user query.
- `data` is free-form JSONB — the event schema is defined by the caller.
- Used for security audit and anomaly detection UI in the admin interface.

### User States

| State | Meaning |
|---|---|
| `pending` | Registered but email not yet verified |
| `active` | Verified; can log in |
| `inactive` | Deactivated; cannot log in |

### Blocking vs. Deactivating vs. Archiving

| Action | `state` | Email renamed | Permissions removed | Session invalidated | Note |
|---|---|---|---|---|---|
| `blocked = True` | unchanged | No | No | Yes (null UUID) | Blocks `check_password`; user row unchanged otherwise |
| `dao_deactivate_user` | `inactive` | No | Yes | Yes (null UUID) | Removes all service/org memberships; keeps original email |
| `dao_archive_user` | `inactive` | Yes (`_archived_{date}_{email}`) | Yes | Yes (null UUID) | Like deactivate + email rename; guarded by archivability check |

**Session invalidation** is accomplished by setting `current_session_id = "00000000-0000-0000-0000-000000000000"` — any in-flight JWT that references the old session ID is rejected at the application layer.

---

## Error Conditions

| Exception | Raised by | Condition |
|---|---|---|
| `InvalidRequest("User cannot be removed from service...", 400)` | `dao_archive_user` | No other active user with `manage_settings` on at least one active service |
| `InvalidRequest("User is already inactive", 400)` | `dao_deactivate_user` | `user.state == "inactive"` |
| `AuthError("Unauthorized, authentication token must be provided", 401)` | `get_auth_token` | No `Authorization` header |
| `AuthError("Unauthorized, Authorization header is invalid...", 401)` | `get_auth_token` | Auth scheme not in known list |
| `AuthError("Invalid scheme: can only use JWT for admin authentication", 401)` | `requires_admin_auth` | Non-`Bearer` scheme on admin route |
| `AuthError("Unauthorized, admin authentication token required", 401)` | `requires_admin_auth` | JWT issuer ≠ `ADMIN_CLIENT_USER_NAME` |
| `AuthError("Invalid scheme: can only use JWT for sre authentication", 401)` | `requires_sre_auth` | Non-`Bearer` scheme |
| `AuthError("Unauthorized, sre authentication token required", 401)` | `requires_sre_auth` | JWT issuer ≠ `SRE_USER_NAME` |
| `AuthError("Invalid scheme: can only use JWT for cypress authentication", 401)` | `requires_cypress_auth` | Non-`Bearer` scheme |
| `AuthError("Unauthorized, cypress authentication token required", 401)` | `requires_cypress_auth` | JWT issuer ≠ `CYPRESS_AUTH_USER_NAME` |
| `AuthError("Invalid token: service id is not the right data type", 403)` | `requires_auth` | `DataError` on service UUID parse |
| `AuthError("Invalid token: service not found", 403)` | `requires_auth` | `NoResultFound` on service lookup |
| `AuthError("Invalid token: service has no API keys", 403)` | `requires_auth` | Service exists but `api_keys` is empty |
| `AuthError("Invalid token: service is archived", 403)` | `requires_auth` | `service.active == False` |
| `AuthError("Invalid token: API key revoked", 403)` | `_auth_with_api_key` | `api_key.expiry_date` is set |
| `AuthError("Invalid token: API key not found", 403)` | `_auth_by_api_key` | `NoResultFound` from `get_api_key_by_secret` |
| `AuthError("Invalid token: Enter your full API key", 403)` | `_auth_by_api_key` | `ValueError` from `get_api_key_by_secret` (format error) |
| `AuthError("Invalid token: signature, api token not found", 403)` | `requires_auth` | No API key secret matched the JWT signature |
| `AuthError("Error: Your system clock must be accurate to within 30 seconds", 403)` | `requires_auth` | JWT `TokenExpiredError` on a service key |
| `AuthError("Invalid token: expired, check that your system clock is accurate", 403)` | `handle_admin_key` | `TokenExpiredError` on admin JWT |
| `AuthError("Invalid token: signature, api token is not valid", 403)` | `handle_admin_key` | `TokenAlgorithmError` or `TokenDecodeError` |
| `AuthError("Invalid token: iss field not provided", 403)` | `__get_token_issuer` | `TokenIssuerError` |
| `AuthError("Invalid token: signature, api token is not valid", 403)` | `__get_token_issuer` | `TokenDecodeError` or `PyJWTError` |
| `ValueError()` | `get_api_key_by_secret` | Prefix mismatch or service_id mismatch in token |
| `NoResultFound()` | `get_api_key_by_secret` | No DB row matches any signed variant |
| `BadSignature` | `resign_api_keys` | Re-signing fails and `unsafe=False` |

---

## Query Inventory (for sqlc)

| Query name | Type | Tables | Description |
|---|---|---|---|
| `CreateUser` | INSERT | `users` | Insert new user row |
| `UpdateUser` | UPDATE | `users` | Update arbitrary user columns by id |
| `GetUserByID` | SELECT ONE | `users` | Fetch user by UUID |
| `ListAllUsers` | SELECT MANY | `users` | Fetch all users |
| `GetUserByEmail` | SELECT ONE | `users` | Case-insensitive email lookup |
| `SearchUsersByEmail` | SELECT MANY | `users` | ILIKE partial-email search |
| `ArchiveUser` | UPDATE | `users` | Set state=inactive, rename email, null mobile, invalidate session |
| `DeactivateUser` | UPDATE | `users` | Set state=inactive, null mobile, invalidate session (email kept) |
| `DeleteUser` | DELETE | `users` | Hard-delete (test teardown only) |
| `IncrementFailedLoginCount` | UPDATE | `users` | Increment failed_login_count by 1 |
| `ResetFailedLoginCount` | UPDATE | `users` | Set failed_login_count = 0 |
| `UpdateUserPassword` | UPDATE | `users` | Set password hash, password_changed_at, password_expired=false |
| `GetUserWithAccounts` | SELECT ONE | `users`, `user_to_service`, `services`, `user_to_organisation`, `organisation` | Eager-loaded user + services + orgs |
| `GetActiveUsersWithServices` | SELECT MANY | `users`, `user_to_service`, `services` | Active users × live services (reporting) |
| `CreateVerifyCode` | INSERT | `verify_codes` | Insert hashed OTP with 30-min expiry |
| `ListVerifyCodesByUser` | SELECT MANY | `verify_codes` | All codes for user ordered desc by created_at |
| `MarkVerifyCodeUsed` | UPDATE | `verify_codes` | Set code_used=true by id |
| `DeleteVerifyCodesByUser` | DELETE | `verify_codes` | Remove all codes for a user |
| `CountActiveVerifyCodes` | COUNT | `verify_codes` | Non-expired, unused codes per user |
| `CountRecentVerifyCodes` | COUNT | `verify_codes` | Non-expired, unused codes created within N seconds |
| `DeleteOldVerifyCodes` | DELETE | `verify_codes` | Remove codes created > 24h ago |
| `AddPermission` | INSERT | `permissions` | Insert a single permission |
| `RemoveUserServicePermissions` | DELETE | `permissions` | Remove all permissions for user+service |
| `RemoveAllUserPermissions` | DELETE | `permissions` | Remove all permissions for a user |
| `GetPermissionsByUserID` | SELECT MANY | `permissions`, `services` | All permissions for user where service.active=true |
| `GetPermissionsByUserAndService` | SELECT MANY | `permissions`, `services` | Permissions for user+service where service.active=true |
| `GetTeamMembersWithPermission` | SELECT MANY | `permissions`, `users` | Active users on service who hold a permission |
| `CreateFido2Key` | INSERT | `fido2_keys` | Store new FIDO2 credential |
| `GetFido2Key` | SELECT ONE | `fido2_keys` | Fetch by user_id + key_id |
| `ListFido2Keys` | SELECT MANY | `fido2_keys` | All keys for user ordered by created_at asc |
| `DeleteFido2Key` | DELETE | `fido2_keys` | Remove key by user_id + key_id |
| `UpsertFido2Session` | DELETE + INSERT | `fido2_sessions` | Replace existing session for user |
| `GetAndDeleteFido2Session` | SELECT ONE + DELETE | `fido2_sessions` | Consume session (one-time read) |
| `DeleteFido2Session` | DELETE | `fido2_sessions` | Invalidate session by user_id |
| `SaveLoginEvent` | INSERT | `login_events` | Record a login event |
| `ListRecentLoginEvents` | SELECT MANY | `login_events` | Last 3 events for user ordered desc |
| `SaveInvitedUser` | INSERT | `invited_users` | Create a service invitation |
| `GetInvitedUserByServiceAndID` | SELECT ONE | `invited_users` | Fetch by service_id + invitation_id |
| `GetInvitedUserByID` | SELECT ONE | `invited_users` | Fetch by invitation_id |
| `ListInvitedUsersForService` | SELECT MANY | `invited_users` | All invitations for a service |
| `DeleteExpiredInvitations` | DELETE | `invited_users` | Remove invitations created > 2 days ago |
| `SaveInvitedOrgUser` | INSERT | `invited_organisation_users` | Create an org invitation |
| `GetInvitedOrgUserByOrgAndID` | SELECT ONE | `invited_organisation_users` | Fetch by org_id + invitation_id |
| `GetInvitedOrgUserByID` | SELECT ONE | `invited_organisation_users` | Fetch by invitation_id |
| `ListInvitedOrgUsersForOrg` | SELECT MANY | `invited_organisation_users` | All org invitations |
| `DeleteExpiredOrgInvitations` | DELETE | `invited_organisation_users` | Remove org invitations created > 2 days ago |
| `CreateApiKey` | INSERT | `api_keys` | Create a new API key (versioned) |
| `ExpireApiKey` | UPDATE | `api_keys` | Set expiry_date = now (versioned) |
| `UpdateApiKeyLastUsed` | UPDATE | `api_keys` | Set last_used_timestamp |
| `UpdateApiKeyCompromisedInfo` | UPDATE | `api_keys` | Set compromised_key_info JSONB (versioned) |
| `GetApiKeyBySecret` | SELECT ONE | `api_keys`, `services` | Lookup by signed secret value (ApiKey-v1 auth) |
| `ListApiKeysForService` | SELECT MANY | `api_keys` | Active + recently-expired keys for service |
| `GetActiveApiKeyByID` | SELECT ONE | `api_keys` | Non-expired key by id + service_id |
| `GetUnsignedSecretsByService` | SELECT MANY | `api_keys` | Unsigned secrets for JWT validation (internal only) |
| `GetUnsignedSecretByKeyID` | SELECT ONE | `api_keys` | Single unsigned secret (internal only) |
| `BulkResignApiKeys` | SELECT MANY + UPDATE | `api_keys` | Re-sign all secrets with current signing key |
