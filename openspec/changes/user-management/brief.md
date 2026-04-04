# Requirements Brief: user-management

## Source Files

- `spec/behavioral-spec/users-auth.md` — endpoint contracts, DAO behavior contracts, authentication behavior, business rules (derived from Python test suite)
- `spec/business-rules/users-auth.md` — DAO function documentation, domain rules, query inventory, error conditions
- `openspec/changes/user-management/proposal.md` — scope: `internal/handler/users/`, `internal/handler/invite/`, `internal/service/users/`

---

## Endpoints

### GET /user

- Returns list of all users.
- Each user includes: `name`, `mobile_number`, `email_address`, `state`, `permissions` (map keyed by service UUID).
- Auth: Admin JWT.

---

### GET /user/{id}

- Returns single user: `id`, `name`, `mobile_number`, `email_address`, `state`, `auth_type`, `default_editor_is_rte` (defaults `false`), `permissions` map, `services` list, `organisations` list.
- Inactive services are excluded from `services` list and from `permissions` map.
- Inactive organisations are excluded from `organisations` list.
- Auth: Admin JWT.

---

### GET /user/email

- Query param `email` required; returns matching user including `password_expired` field.
- Missing `email` param → 400 `"Invalid request. Email query string param required"`.
- No matching user → 404 `{"result": "error", "message": "No result found"}`.
- Auth: Admin JWT.

---

### POST /user (create user)

- 201; `id` and `email_address` in response. Password hashed before storage.
- Default `auth_type = email_auth` when omitted.
- Required: `email_address` (valid email), `password` (not banned/common).
- `name` must not be empty string.
- `mobile_number` must be valid international format if provided (non-null, non-empty).
- `auth_type = sms_auth` requires non-null `mobile_number` → 400 `"Mobile number must be set if auth_type is set to sms_auth"`.
- `auth_type = email_auth` permits `mobile_number: null`.
- Missing `email_address` → 400 `{"email_address": ["Missing data for required field."]}`.
- Missing `password` → 400 `{"password": ["Missing data for required field."]}`.
- Banned/common password → 400 `{"password": ["Password is not allowed."]}`.
- Auth: Admin JWT.

---

### POST /user/{id} (update user attributes)

- Updates one or more of: `name`, `email_address`, `mobile_number`, `auth_type`, `blocked`, `default_editor_is_rte`.
- Returns 200 with updated user data.
- Calls `salesforce_client.contact_update`.
- `password` cannot be updated via this endpoint → 400 `{"_schema": ["Unknown field name password"]}`.
- `auth_type = sms_auth` with `mobile_number: null` → 400 `"Mobile number must be set if auth_type is set to sms_auth"`.
- `mobile_number: ""` → 400 `"Invalid phone number: Not a valid international number"`.
- Can remove `mobile_number` (set null) only if `auth_type = email_auth`.
- Can simultaneously set `auth_type = email_auth` and `mobile_number: null` → 200.
- Changing `name`, `email_address`, or `mobile_number` triggers notification to user using updater's name (from `updated_by`); email template `c73f1d71`, SMS template `8a31520f`.
- Changing `default_editor_is_rte` does NOT trigger a notification.
- Setting `blocked: true` forces `current_session_id` to nil UUID `00000000-0000-0000-0000-000000000000`; does NOT trigger a notification from this endpoint.
- Auth: Admin JWT.

---

### POST /user/{id}/activate

- Transitions user `pending → active`; returns 200 with updated user.
- Calls `salesforce_client.contact_create`.
- Already active → 400 `"User already active"`.
- Auth: Admin JWT.

---

### POST /user/{id}/deactivate

- Sets `state = inactive`; sends deactivation notification email to the user. Returns 200.
- Service membership side-effects:

  | Service type | Other active members | Service outcome | Suspension email |
  |---|---|---|---|
  | Live | 0 | `active=False` (no `suspended_at`) | No |
  | Trial | 0 | `active=False` (no `suspended_at`) | No |
  | Live | 1 | `active=False` + `suspended_at=now` + `suspended_by_id=user` | Yes |
  | Trial | 1 | No change | No |
  | Live | 2+ | No change | No |
  | Trial | 2+ | No change | No |

- Transactional: all DB writes rolled back on exception; 500 returned.
- Auth: Admin JWT.

---

### POST /user/{id}/archive

- Archives user; returns 204. Delegates to `dao_archive_user`.
- Non-existent `user_id` → 404.
- Sole `manage_settings` holder on any active service → 400 `"User cannot be removed from service. Check that all services have another team member who can manage settings"`.
- Auth: Admin JWT.

---

### POST /user/{id}/update-password

- Updates password hash; returns 200 with `password_changed_at` set.
- Banned/common password → 400.
- Creates a `LoginEvent` record if `loginData` is present in request body.
- Does NOT create a `LoginEvent` if `loginData` is absent.
- Auth: Admin JWT.

---

### POST /user/{id}/verify-password

- Verifies supplied password against stored hash; returns 204.
- Resets `failed_login_count` to 0 on success.
- Does NOT update `logged_in_at`.
- Missing `password` field → 400 `"Required field missing data"`.
- Wrong password → 400 `"Incorrect password for user_id …"`; increments `failed_login_count`.
- Auth: Admin JWT.

---

### POST /user/{id}/reset-failed-login-count

- Sets `failed_login_count = 0`; returns 200.
- Non-existent user → 404.
- Auth: Admin JWT.

---

### POST /user/{id}/reset-password

- Sends password-reset notification email via `deliver_email` on `notify-internal-tasks`; returns 204.
- Reply-to set to notify service default.
- `email` field required and must be valid email → 400 `{"email": ["Missing data for required field."]}` or `{"email": ["Not a valid email address"]}`.
- User not found → 404.
- User blocked → 400, message contains `"user blocked"`.
- Auth: Admin JWT.

---

### POST /user/{id}/forced-password-reset

- Same as `reset-password` but uses `forced_password_reset_email_template`; returns 204.
- Reply-to set to notify service default.
- Auth: Admin JWT.

---

### POST /user/{id}/email (send already-registered email)

- Sends "already registered" notification email via `deliver_email` on `notify-internal-tasks`; returns 204.
- `email` field required → missing → 400 `{"email": ["Missing data for required field."]}`.
- Auth: Admin JWT.

---

### POST /user/{id}/confirm-new-email

- Sends change-email confirmation notification to the new address; returns 204.
- Reply-to set to notify service default.
- `email` field (new address) required → missing → 400 `{"email": ["Missing data for required field."]}`.
- Auth: Admin JWT.

---

### POST /user/{id}/contact-request

- Sends Freshdesk support ticket; base case returns 204.
- No live service / `ask_question` / `demo`: ticket with tags `z_skip_opsgenie`, `z_skip_urgent_escalation`; no Salesforce update.
- Live service: ticket sent; no Salesforce update.
- `go_live_request` with `service_id`: fetches service; ticket + `salesforce_client.engagement_update(ENGAGEMENT_STAGE_ACTIVATION, main_use_case)`; `department_org_name` from `organisation_notes` (fallback `"Unknown"`).
- PT service (`organisation_type = province_or_territory`) + `FF_PT_SERVICE_SKIP_FRESHDESK=True`: returns 201; `Freshdesk.email_freshdesk_ticket_pt_service()` called; `send_ticket()` NOT called. *(No existing Python test coverage for this path; Go must add tests.)*
- Central (non-PT) service + `FF_PT_SERVICE_SKIP_FRESHDESK=True`: `send_ticket()` called; returns 204.
- Auth: Admin JWT.

---

### POST /user/{id}/branding-request

- Sends Freshdesk branding ticket; returns 204. No Salesforce update.
- Payload fields: `service_name`, `email_address`, `serviceID`, `organisation_id`, `organisation_name`, `filename`, `alt_text_en`, `alt_text_fr`.
- Auth: Admin JWT.

---

### POST /user/{id}/new-template-category-request

- Sends Freshdesk new-template-category ticket; returns 204.
- Payload fields: `service_name`, `email_address`, `service_id`, `template_category_name_en`, `template_category_name_fr`, `template_id`.
- Auth: Admin JWT.

---

### GET /user/{id}/organisations-and-services

- Returns `{"organisations": [...], "services": [...]}`.
- Organisations: `name`, `id`, `count_of_live_services` — active orgs only.
- Services: `name`, `id`, `restricted`, `organisation` (org UUID or null) — active services in active orgs only; inactive orgs and their services excluded.
- Scoped to the requesting user's memberships only.
- `organisation` field on a service is its actual org UUID regardless of whether the user belongs to that org.
- Auth: Admin JWT.

---

### POST /user/find-users-by-email

- Partial and full email searches return matching users; 200 `{"data": [...]}`.
- No matches → 200 `{"data": []}`.
- `email` must be a string → non-string (e.g., integer) → 400 `{"email": ["Not a valid string."]}`.
- Auth: Admin JWT.

---

### POST /user/{id}/permissions/{service_id}

- Replaces the user's entire permission set for the given service; returns 204. Old permissions removed atomically.
- Also replaces `folder_permissions` for that service; does not affect other services.
- User does not belong to the service → 404.
- Auth: Admin JWT.

---

### GET /user/{id}/fido2-keys

- Returns array of FIDO2 key objects in `created_at ASC` order; 200.
- Auth: Admin JWT.

---

### POST /user/{id}/fido2-keys/register (begin registration ceremony)

- Writes `Fido2Session` for the user (any previous session deleted first).
- Returns 200 `{"data": "<base64-CBOR>"}`.
- Decoded CBOR: `publicKey.rp.id = <application hostname>`, `publicKey.user.id = <user UUID bytes>`.
- Auth: Admin JWT.

---

### POST /user/{id}/fido2-keys (complete registration)

- Accepts `{"payload": "<base64-CBOR>"}`.
- Requires a valid `Fido2Session` for the user; calls `decode_and_register(data, state)`.
- Returns 200 `{"id": "<key_id>"}`.
- Persists `Fido2Key`; sends account-change notification.
- Auth: Admin JWT.

---

### POST /user/{id}/fido2-keys/authenticate (begin authentication ceremony)

- Uses user's registered keys (each deserialized via `deserialize_fido2_key`); calls `FIDO2_SERVER.authenticate_begin`.
- Returns 200 `{"data": "<base64-CBOR>"}` containing `rpId`.
- Auth: Admin JWT.

---

### DELETE /user/{id}/fido2-keys/{key_id}

- Deletes the specified FIDO2 key; returns 200 with deleted key's id.
- Key count goes to zero (no orphan rows).
- Sends account-change notification.
- Auth: Admin JWT.

---

### GET /user/{id}/login-events

- Returns login event objects in reverse-chronological order; at most 3 entries; 200.
- Auth: Admin JWT.

---

### POST /service/{service_id}/invite (create service invitation)

- Creates an `InvitedUser` record; returns 201 with: `service`, `email_address`, `from_user`, `permissions`, `auth_type`, `id`, `folder_permissions`.
- Sends invitation email via `deliver_email` on `notify-internal-tasks`.
- Reply-to = invitor's email address. Personalisation: `service_name`, `user_name`, `url`.
- `url` constructed from optional `invite_link_host` (default `http://localhost:6012/invitation/`) + invite token.
- `email_address` must be valid email → 400 `{"email_address": ["Not a valid email address"]}`.
- Default `auth_type = email_auth` when omitted.
- Auth: Admin JWT.

---

### GET /service/{service_id}/invite

- Returns all invitations for the service; 200 `{"data": [...]}`.
- Empty list if none. Each invite includes: `service`, `from_user`, `auth_type`, `id`.
- Auth: Admin JWT.

---

### POST /service/{service_id}/invite/{invite_id} (update invitation)

- Updates invitation (e.g., `status: "cancelled"`); returns 200 with updated data.
- `invite_id` belongs to a different service → 404.
- Invalid status value → 400.
- Auth: Admin JWT.

---

### GET /invite/{invitation_type}/{token} (decode invite token)

- `invitation_type` ∈ `{service, organisation}`.
- Service invite: returns `id`, `email_address`, `from_user`, `service`, `status`, `permissions`, `folder_permissions`.
- Org invite: returns serialised org invite object.
- Expired token → 400 `{"invitation": "invitation expired"}`.
- Malformed/truncated token → 400 `{"invitation": "bad invitation link"}`.
- Valid token but invited user not found → 404 `"No result found"`.
- Auth: Admin JWT.

---

## OTP / MFA / Verify Code Flows

### POST /user/{id}/2fa-code — `code_type=sms`

- Generates 5-digit numeric code via `create_secret_code` (cryptographically random, preserves leading zeros).
- Stores as `VerifyCode` (hashed via `VerifyCode.code` setter; 30-min TTL).
- Sends SMS via `deliver_sms` on `notify-internal-tasks`; personalisation `{"verify_code": "<5-digit>"}`.
- Optional `to` override: sends to override number instead of account `mobile_number`.
- **Retry deduplication**: if a code was already sent within the 30-second delta window for the same destination, does NOT create a new code and does NOT dispatch a new notification; returns 204.
- **Max codes guard**: if 10+ unexpired, unused codes already exist for the user, no new code is generated; returns 204.
- Non-existent user → 404 `"No result found"`.

### POST /user/{id}/2fa-code — `code_type=email`

- Generates 5-digit code; sends email to user's `email_address` (or optional `to` override, null `to` → account email).
- Template `EMAIL_2FA_TEMPLATE_ID`; personalisation includes `name` and `verify_code`.
- Optional: `email_auth_link_host`, `next` (URL-encoded next param in personalisation).

### POST /user/{id}/email-verification

- Sends verification link email (not a numeric code); no `VerifyCode` record created.
- Via `deliver_email` on `notify-internal-tasks`; returns 204.
- Non-existent `user_id` → 404.

### POST /user/{id}/verify-code

- Validates OTP of type `sms` or `email`; checks newest-first by bcrypt.
- **On success**: marks code used (`MarkVerifyCodeUsed`), sets `logged_in_at = now`, generates new `current_session_id`, resets `failed_login_count = 0`; returns 204.
- Missing `code` → 400; `failed_login_count` NOT incremented.
- Code not found (bad value) → 404; `failed_login_count` incremented.
- Code expired → 400; `failed_login_count` incremented.
- Code already used → 400; `failed_login_count` NOT changed.
- `failed_login_count >= 10` → 404; code NOT marked used; count NOT changed (lockout takes priority even if code is correct).
- **E2E bypass**: when `NOTIFY_ENVIRONMENT=development`, host is `localhost:3000` or `dev.local`, and user email matches `CYPRESS_EMAIL_PREFIX` pattern — any code accepted → 204. Not active in production.

### POST /user/{id}/verify-2fa

- Same verification logic as `verify-code` with these key differences:
  - Does NOT increment `failed_login_count` on any failure.
  - Does NOT enforce account lockout (`failed_login_count >= 10` not checked).
- Errors: bad code → 404, expired → 400, already used → 400, missing code → 400. None increment `failed_login_count`.
- Same E2E Cypress bypass as `verify-code`.

---

## FIDO2 Flows

### Registration ceremony

1. `POST /fido2-keys/register` → `create_fido2_session(user_id, state)`: deletes any existing session for user, inserts new; returns CBOR options with `rp.id = hostname` and `user.id = UUID bytes`.
2. Client responds with attestation data.
3. `POST /fido2-keys` → `get_fido2_session(user_id)` (consume-once: retrieves + deletes row) → `decode_and_register(data, state)` → `save_fido2_key`.
4. Returns 200 `{"id": "<key_id>"}`. Sends account-change notification.

### Authentication ceremony

1. `POST /fido2-keys/authenticate` → `create_fido2_session(user_id, state)` (previous session deleted); loads all user keys via `deserialize_fido2_key`; calls `FIDO2_SERVER.authenticate_begin`; returns CBOR options.
2. Client responds with assertion; server calls `FIDO2_SERVER.authenticate_complete`.
3. `get_fido2_session` consumed during response — cannot be replayed.

### Session invariants

- Only one active ceremony per user at any time (`create_fido2_session` deletes previous).
- `get_fido2_session` deletes the row on read — replay protection.

### Backward compatibility

- `deserialize_fido2_key` uses `_Fido2CredentialUnpickler` to remap `fido2.ctap2.AttestedCredentialData` → `fido2.webauthn.AttestedCredentialData` for keys registered under fido2 0.9.x.

---

## DAO Functions

### Users

| Function | Query type | Key behavior |
|---|---|---|
| `CreateUser` | INSERT | Password hashed via `User.password` setter (bcrypt) |
| `save_user_attribute` | UPDATE | `blocked=True` → force `current_session_id = nil UUID` |
| `save_model_user` | INSERT/UPDATE | `pwd` arg → hash + set `password_changed_at`; strips `id` and `password_changed_at` from `update_dict` |
| `GetUserByID` | SELECT ONE | `user_id=nil` returns ALL users — Go must reject nil/zero UUID before calling |
| `GetUserByEmail` | SELECT ONE | `lower(email_address) = lower(email)` |
| `SearchUsersByEmail` | SELECT MANY | `ILIKE '%email%'`; special chars escaped before pattern |
| `dao_archive_user` | Multi-write (`@transactional`) | Email → `_archived_{YYYY-MM-DD}_{email}`; mobile=null; permissions removed; auth_type=email_auth; state=inactive; session=nil UUID; password=random UUID; guarded by `user_can_be_archived` |
| `dao_deactivate_user` | Multi-write | state=inactive; permissions removed; session=nil UUID; email address KEPT intact |
| `user_can_be_archived` | Read-only | False if any active service has no other active user with `manage_settings` |
| `increment_failed_login_count` | UPDATE | Increment by 1 |
| `reset_failed_login_count` | UPDATE | Set to 0 (only when `> 0`) |
| `update_user_password` | UPDATE | Hash, `password_changed_at=now`, `password_expired=False` |
| `get_user_and_accounts` | SELECT with joinedload | services, orgs, orgs.services, services.organisation; avoids N+1 |
| `get_services_for_all_users` | SELECT MANY | Active users × live+non-restricted+non-research services |

### VerifyCodes

| Function | Key behavior |
|---|---|
| `create_secret_code` | Cryptographically random 5-digit string; preserves leading zeros |
| `create_user_code` | INSERT; code hashed; `expiry_datetime = now + 30min` |
| `get_user_code` | SELECT newest-first; bcrypt check; caller must check `expiry_datetime` and `code_used` |
| `use_user_code` (MarkVerifyCodeUsed) | UPDATE `code_used = True` by id |
| `count_user_verify_codes` | COUNT non-expired AND `code_used IS False` |
| `verify_within_time` | COUNT non-expired, unused, `created_at > now - age` (default 30s) |
| `delete_codes_older_created_more_than_a_day_ago` | DELETE `created_at < now - 24h` |
| `delete_user_verify_codes` | DELETE all for user |

### Permissions

| Function | Key behavior |
|---|---|
| `add_default_service_permissions_for_user` | INSERT 8 permissions; `_commit=False` |
| `remove_user_service_permissions` | DELETE for user+service pair |
| `remove_user_service_permissions_for_all_services` | DELETE all for user |
| `set_user_service_permission` | `replace=True` → DELETE then INSERT |
| `get_permissions_by_user_id` | JOIN `services WHERE active=True` — inactive services excluded |
| `get_permissions_by_user_id_and_service_id` | JOIN active service |
| `get_team_members_with_permission` | Active users on service who hold a specific permission |

### FIDO2

| Function | Key behavior |
|---|---|
| `save_fido2_key` | INSERT (`@transactional`) |
| `get_fido2_key` | SELECT by `user_id + id` |
| `list_fido2_keys` | SELECT ORDER BY `created_at ASC` |
| `delete_fido2_key` | DELETE by `user_id + id` |
| `create_fido2_session` | DELETE existing for user + INSERT; one session per user |
| `get_fido2_session` | SELECT + DELETE (consume-once read) |
| `delete_fido2_session` | DELETE by `user_id` |
| `decode_and_register` | `FIDO2_SERVER.register_complete` → base64-pickle of `credential_data` |
| `deserialize_fido2_key` | Decode stored credential; backward-compat unpickler for 0.9.x |

### LoginEvents

| Function | Key behavior |
|---|---|
| `save_login_event` | INSERT (`@transactional`) |
| `list_login_events` | SELECT last 3 by `user_id` ORDER BY `created_at DESC` LIMIT 3 |

### Invite (Service)

| Function | Key behavior |
|---|---|
| `save_invited_user` | INSERT |
| `get_invited_user` | SELECT by `service_id + id` (scoped); `NoResultFound` if cross-service |
| `get_invited_user_by_id` | SELECT by `id` only (global lookup for accept flow) |
| `get_invited_users_for_service` | SELECT all for service |
| `delete_invitations_created_more_than_two_days_ago` | DELETE `created_at <= now - 2 days` |

### Invite (Organisation)

| Function | Key behavior |
|---|---|
| `save_invited_org_user` | INSERT |
| `get_invited_org_user` | SELECT by `organisation_id + id` |
| `get_invited_org_user_by_id` | SELECT by `id` only |
| `get_invited_org_users_for_organisation` | SELECT all for org |
| `delete_org_invitations_created_more_than_two_days_ago` | DELETE `created_at <= now - 2 days` |

---

## Business Rules

### User auth types

| Value | MFA delivery | Prerequisite |
|---|---|---|
| `email_auth` | OTP via email (5-digit code) | Default; no additional fields required |
| `sms_auth` | OTP via SMS (5-digit code) | `mobile_number` must be non-null |
| `security_key_auth` | FIDO2/WebAuthn assertion | At least one `Fido2Key` registered |

DB CHECK constraint: `auth_type = 'email_auth' OR mobile_number IS NOT NULL`.

### User state transitions

- **`pending → active`**: `POST /user/{id}/activate`; calls `salesforce_client.contact_create`.
- **`active → inactive` (archive)**: `POST /user/{id}/archive`; email renamed `_archived_{YYYY-MM-DD}_{email}`; permissions removed; mobile=null; session=nil UUID; password invalidated (random UUID); auth_type reset to email_auth.
- **`active → inactive` (deactivate)**: `POST /user/{id}/deactivate`; same minus email rename; email address kept intact. Sends deactivation email. May trigger service suspension.
- **Blocked state**: `blocked=True` → `current_session_id` forced to nil UUID; `check_password` returns `False` regardless of password.

### Password and crypto

- All password operations MUST use `pkg/crypto` (bcrypt).
- `User.password` setter applies bcrypt hash on assignment.
- `VerifyCode.code` setter applies bcrypt on assignment (codes never stored plaintext).
- `update_user_password`: sets `password_changed_at = now`, `password_expired = False`.
- `save_model_user` with `pwd` arg: hashes password; strips `id` and `password_changed_at` from update dict.

### Verify code limits

- Flood guard: `verify_within_time(user, age=30s)` — reject if non-zero (a code was recently created).
- Max codes: endpoint layer enforces a cap of 10 unexpired/unused codes per user; exceeding → 204 without creating a new code.
- TTL: 30 minutes from creation.
- Cleanup: `delete_codes_older_created_more_than_a_day_ago` removes any code `created_at < now - 24h` regardless of use status.

### Failed login count rules

- Incremented by: wrong password (`verify-password`), not-found code (`verify-code`), expired code (`verify-code`).
- NOT incremented by: missing code (`verify-code`), already-used code (`verify-code`), any failure in `verify-2fa`.
- Reset to 0 by: successful `verify-password`, successful `verify-code`.
- Lockout: `failed_login_count >= 10` → `verify-code` returns 404 even for correct code; code NOT marked used; count NOT changed. `verify-2fa` does NOT enforce lockout.

### Permissions

- 8 service-scoped permissions: `manage_users`, `manage_templates`, `manage_settings`, `send_texts`, `send_emails`, `send_letters`, `manage_api_keys`, `view_activity`.
- Unique constraint: `(service_id, user_id, permission)`.
- Permissions only visible for active services (JOIN `services WHERE active=True`).
- Default grant on service join: all 8 permissions.
- `set_permissions` is replace-not-append: atomically drops all old permissions for the user+service and writes the new set.
- Archiving a user clears all their permissions across all services.
- Global `platform_admin` flag is on `User` model (not in `permissions` table).

### Invitation token

- URL-safe signed token generated with application `SECRET_KEY`.
- Token includes embedded TTL; expired → 400 `{"invitation": "invitation expired"}`.
- Malformed/truncated token → 400 `{"invitation": "bad invitation link"}`.
- Status lifecycle: `pending` → `accepted` | `cancelled`.
- Stale invitations (>48h from `created_at`) cleaned up by scheduled task.

### Session invalidation

- `current_session_id = "00000000-0000-0000-0000-000000000000"` invalidates all in-flight sessions.
- Triggered by: `blocked=True` (via `save_user_attribute`), `dao_archive_user`, `dao_deactivate_user`.
- Successful `verify_user_code` also sets a NEW non-nil `current_session_id`.

### Blocking vs deactivating vs archiving

| Action | `state` | Email renamed | Permissions removed | Session invalidated |
|---|---|---|---|---|
| `blocked=True` | unchanged | No | No | Yes (nil UUID) |
| `dao_deactivate_user` | `inactive` | No | Yes | Yes (nil UUID) |
| `dao_archive_user` | `inactive` | Yes (`_archived_{date}_{email}`) | Yes | Yes (nil UUID) |

---

## Error Conditions

| Condition | Endpoint / function | HTTP | Message |
|---|---|---|---|
| Missing `email` query param | `GET /user/email` | 400 | `"Invalid request. Email query string param required"` |
| No user for email | `GET /user/email` | 404 | `{"result": "error", "message": "No result found"}` |
| `sms_auth` + null mobile | `POST /user`, `POST /user/{id}` | 400 | `"Mobile number must be set if auth_type is set to sms_auth"` |
| `mobile_number: ""` | `POST /user/{id}` | 400 | `"Invalid phone number: Not a valid international number"` |
| Banned password | `POST /user`, `update-password` | 400 | `{"password": ["Password is not allowed."]}` |
| Password in update body | `POST /user/{id}` | 400 | `{"_schema": ["Unknown field name password"]}` |
| User already active | `POST activate` | 400 | `"User already active"` |
| Sole `manage_settings` holder | `dao_archive_user` | 400 | `"User cannot be removed from service..."` |
| User already inactive | `dao_deactivate_user` | — | `InvalidRequest("User is already inactive", 400)` |
| Missing `password` | `verify-password` | 400 | `"Required field missing data"` |
| Wrong password | `verify-password` | 400 | `"Incorrect password for user_id …"` |
| Code not found | `verify-code` | 404 | (increments count) |
| Code expired | `verify-code` | 400 | (increments count) |
| Code already used | `verify-code`, `verify-2fa` | 400 | — |
| Account locked (`failed_login_count >= 10`) | `verify-code` | 404 | (code not marked used) |
| User blocked | `reset-password` | 400 | message contains `"user blocked"` |
| Invalid email (invite) | `POST /service/{id}/invite` | 400 | `{"email_address": ["Not a valid email address"]}` |
| Cross-service invite ID | `POST /service/{id}/invite/{inv_id}` | 404 | — |
| Invalid invite status | `POST /service/{id}/invite/{inv_id}` | 400 | — |
| Expired invite token | `GET /invite/{type}/{token}` | 400 | `{"invitation": "invitation expired"}` |
| Malformed invite token | `GET /invite/{type}/{token}` | 400 | `{"invitation": "bad invitation link"}` |
| Invited user not found | `GET /invite/{type}/{token}` | 404 | `"No result found"` |
| Non-existent `user_id` | Any `/user/{id}` endpoint | 404 | — |
| Non-string `email` | `POST /user/find-users-by-email` | 400 | `{"email": ["Not a valid string."]}` |
| User not in service | `POST /user/{id}/permissions/{service_id}` | 404 | — |
