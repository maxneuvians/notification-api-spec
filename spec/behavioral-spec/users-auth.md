# Behavioral Spec: Users & Authentication

## Processed Files

- [x] `tests/app/user/test_rest.py`
- [x] `tests/app/user/test_rest_verify.py`
- [x] `tests/app/user/test_contact_request.py`
- [x] `tests/app/dao/test_users_dao.py`
- [x] `tests/app/dao/test_permissions_dao.py`
- [x] `tests/app/dao/test_fido2_keys_dao.py`
- [x] `tests/app/dao/test_login_event_dao.py`
- [x] `tests/app/dao/test_invited_user_dao.py`
- [x] `tests/app/invite/test_invite_rest.py`
- [x] `tests/app/accept_invite/test_accept_invite_rest.py`
- [x] `tests/app/authentication/test_authentication.py`
- [x] `tests/app/test_route_authentication.py`
- [x] `tests/app/test_encryption.py`

---

## Endpoint Contracts

### GET /user

- **Happy path**: Returns list of all users; each user includes name, mobile_number, email_address, state, and a `permissions` map keyed by service UUID.
- **Validation rules**: No query params required.
- **Error cases**: None tested beyond auth.
- **Auth requirements**: Admin JWT.

---

### GET /user/`<user_id>`

- **Happy path**: Returns single user record with id, name, mobile_number, email_address, state, auth_type, default_editor_is_rte (defaults `false`), permissions map, services list, and organisations list.
- **Validation rules**: Inactive services are excluded from `services` list and `permissions` map. Inactive organisations are excluded from `organisations` list.
- **Error cases**: (404 implied by UUID routing; not explicitly tested in this file.)
- **Auth requirements**: Admin JWT.

---

### GET /user/email

- **Happy path**: Returns user whose email_address matches the `email` query param; includes password_expired field.
- **Validation rules**: `email` query param is required.
- **Error cases**:
  - Missing `email` param → 400, `"Invalid request. Email query string param required"`.
  - No matching user → 404, `{"result": "error", "message": "No result found"}`.
- **Auth requirements**: Admin JWT.

---

### POST /user (create user)

- **Happy path**: Creates a user; 201 response includes user id and email_address. Password is stored hashed. Default `auth_type` is `email_auth` when omitted.
- **Validation rules**:
  - `email_address` required; must be a valid email address.
  - `password` required; must not be a known-bad/common password (→ 400, `"Password is not allowed."`).
  - `name` must not be empty string.
  - `mobile_number` must be a valid international phone number if provided (non-null, non-empty).
  - `auth_type = sms_auth` requires a non-null mobile_number (→ 400, `"Mobile number must be set if auth_type is set to sms_auth"`).
  - `auth_type = email_auth` permits `mobile_number: null`.
- **Error cases**:
  - Missing email → 400, `{"email_address": ["Missing data for required field."]}`.
  - Missing password → 400, `{"password": ["Missing data for required field."]}`.
  - Known-bad password → 400, `{"password": ["Password is not allowed."]}`.
  - Empty strings for name/email/mobile → 400, per-field validation messages.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>` (update user attributes)

- **Happy path**: Updates one or more of: name, email_address, mobile_number, auth_type, blocked, default_editor_is_rte. Returns 200 with updated user data. Calls `salesforce_client.contact_update`.
- **Validation rules**:
  - Password cannot be updated via this endpoint (→ 400, `{"_schema": ["Unknown field name password"]}`).
  - `auth_type = sms_auth` with `mobile_number: null` → 400, `"Mobile number must be set if auth_type is set to sms_auth"`.
  - `mobile_number: ""` → 400, `"Invalid phone number: Not a valid international number"`.
  - Can remove mobile_number (set to null) only if auth_type is `email_auth`.
  - Can simultaneously set `auth_type = email_auth` and `mobile_number: null`.
- **Side effects**:
  - Changing name, email_address, or mobile_number triggers a notification email/SMS to the user.
  - When `updated_by` is provided: sends email to the new email address (template `c73f1d71`) or SMS to the new mobile (template `8a31520f`) with personalisation including the updater's name.
  - Changing `default_editor_is_rte` does NOT trigger a notification.
  - Setting `blocked: true` does NOT trigger a notification directly from this endpoint.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/archive

- **Happy path**: Archives the user; returns 204. Delegates to `dao_archive_user`.
- **Error cases**:
  - Non-existent user_id → 404.
  - User cannot be archived (sole manage_settings holder) → 400, `"User cannot be removed from service. Check that all services have another team member who can manage settings"`.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/activate

- **Happy path**: Transitions user from `pending` to `active`; returns 200 with updated user. Calls `salesforce_client.contact_create`.
- **Error cases**: User already active → 400, `"User already active"`.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/deactivate

- **Happy path**: Sets user state to `inactive`. Sends deactivation notification email to the user. Returns 200. Service membership affects service state per the following matrix:

  | Service type | Other active members | Service outcome | Suspension email to team |
  |---|---|---|---|
  | Live | 0 | `active=False` (deactivated, no suspended_at) | No |
  | Trial | 0 | `active=False` (deactivated, no suspended_at) | No |
  | Live | 1 | `active=False` + `suspended_at=now` + `suspended_by_id=user` | Yes |
  | Trial | 1 | No change | No |
  | Live | 2+ | No change | No |
  | Trial | 2+ | No change | No |

- **Transactional guarantees**: All DB changes are rolled back if an exception occurs; returns 500 on unexpected errors. Changes are committed on success (persisted after response).
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/update-password

- **Happy path**: Updates password hash; returns 200 with `password_changed_at` set.
- **Validation rules**: Password must not be a banned/common password (→ 400).
- **Side effects**: Creates a `LoginEvent` record if `loginData` is present in the request body. Does NOT create a `LoginEvent` if `loginData` is absent.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/verify-password

- **Happy path**: Verifies the supplied password against the stored hash; returns 204. Resets `failed_login_count` to 0 on success. Does NOT update `logged_in_at`.
- **Validation rules**: `password` field required; missing → 400, `"Required field missing data"`.
- **Error cases**:
  - Wrong password → 400, `"Incorrect password for user_id …"`, increments `failed_login_count`.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/verify-code

- **Happy path**: Verifies an OTP code (type `sms` or `email`). On success: marks code as used (`code_used=true`), sets `logged_in_at` to now, generates a new `current_session_id`, resets `failed_login_count` to 0. Returns 204.
- **Validation rules**: `code` field required; missing → 400, `failed_login_count` not incremented.
- **Error cases**:
  - Bad (not found) code → 404, increments `failed_login_count`, code remains unused.
  - Expired code → 400, increments `failed_login_count`, code remains unused.
  - Code already used → 400.
  - Account locked (`failed_login_count >= 10`) → 404, even if code is correct; does not mark code used; does not change count.
- **E2E bypass**: When `NOTIFY_ENVIRONMENT=development`, host is `localhost:3000` or `dev.local`, and the user's email matches the `CYPRESS_EMAIL_PREFIX` pattern, any code is accepted → 204. Not active in production or on non-local hosts.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/verify-2fa

- **Happy path**: Same verification logic as `/verify-code`; returns 204 on success, marks code used.
- **Key difference from `/verify-code`**: Does NOT increment `failed_login_count` on any failure; does NOT enforce account lockout (failed_login_count check absent).
- **Error cases**: Bad code → 404; expired → 400; already used → 400; missing code → 400. None of these increment `failed_login_count`.
- **E2E bypass**: Same Cypress/dev bypass logic as `/verify-code`.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/2fa-code (send 2FA code — `code_type` path param)

**For `code_type=sms`:**
- **Happy path**: Generates a 5-digit secret code, stores as `VerifyCode`, sends SMS notification to user's mobile number (or optional `to` override) via `deliver_sms` on `notify-internal-tasks` queue. Returns 204. Notification personalisation: `{"verify_code": "<5-digit>"}`.
- **Retry window deduplication**: If a code was already sent within the delta window for the same `to` number, does NOT generate a new code and does NOT send a new notification (still returns 204).
- **Max codes guard**: If 10+ unexpired codes already exist for the user, still returns 204 without creating a new code.

**For `code_type=email`:**
- **Happy path**: Generates a 5-digit code, sends email to user's email_address (or optional `to` override; null `to` sends to account email). Returns 204. Personalisation includes `name` and `verify_code`. Template: `EMAIL_2FA_TEMPLATE_ID`. Optional params: `email_auth_link_host`, `next` (URL-encoded next param in personalisation).

**Shared:**
- **Error cases**: Non-existent user_id → 404, `"No result found"`.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/email-verification (send new user email verification)

- **Happy path**: Sends verification link email (not a numeric code; no `VerifyCode` record created). Sends via `deliver_email` to `notify-internal-tasks`. Returns 204.
- **Error cases**: Non-existent user_id → 404.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/reset-failed-login-count

- **Happy path**: Resets `failed_login_count` to 0; returns 200.
- **Error cases**: Non-existent user_id → 404.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/reset-password

- **Happy path**: Sends password-reset notification email via `deliver_email` on `notify-internal-tasks`. Reply-to set to notify service default. Returns 204.
- **Validation rules**: `email` field required and must be a valid email address.
- **Error cases**:
  - Missing `email` → 400, `{"email": ["Missing data for required field."]}`.
  - Invalid email format → 400, `{"email": ["Not a valid email address"]}`.
  - User not found → 404.
  - User is blocked → 400, message contains `"user blocked"`.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/forced-password-reset

- **Happy path**: Same as reset-password but uses the `forced_password_reset_email_template`. Returns 204. Reply-to set to notify service default.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/email (send already-registered email)

- **Happy path**: Sends "already registered" notification email via `deliver_email` on `notify-internal-tasks`. Returns 204.
- **Validation rules**: `email` field required.
- **Error cases**: Missing `email` → 400, `{"email": ["Missing data for required field."]}`.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/confirm-new-email

- **Happy path**: Sends change-email confirmation notification to the new address. Returns 204. Reply-to set to notify service default.
- **Validation rules**: `email` field (new address) required.
- **Error cases**: Missing `email` → 400, `{"email": ["Missing data for required field."]}`.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/contact-request

- **Happy path**: Sends a Freshdesk support ticket via `Freshdesk.send_ticket()`. Returns 204.
- **Business rules by context**:
  - No live service / `ask_question` / `demo`: sends ticket; tags include `z_skip_opsgenie`, `z_skip_urgent_escalation`; no Salesforce update.
  - Live service: sends ticket; no Salesforce update.
  - `go_live_request` with `service_id`: fetches service, sends ticket, calls `salesforce_client.engagement_update` with `ENGAGEMENT_STAGE_ACTIVATION` and the `main_use_case` as description. `department_org_name` populated from `organisation_notes` (falls back to `"Unknown"`).
  - Province/territory service (`organisation_type = "province_or_territory"`) when `FF_PT_SERVICE_SKIP_FRESHDESK=True`: returns 201 (not 204); calls `Freshdesk.email_freshdesk_ticket_pt_service()` instead of `send_ticket()`; `send_ticket()` not called.
  - Central (non-PT) service when `FF_PT_SERVICE_SKIP_FRESHDESK=True`: still calls `send_ticket()`; returns 204.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/branding-request

- **Happy path**: Sends a Freshdesk ticket for branding request. Returns 204. No Salesforce engagement update.
- **Payload fields**: service_name, email_address, serviceID, organisation_id, organisation_name, filename, alt_text_en, alt_text_fr.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/new-template-category-request

- **Happy path**: Sends a Freshdesk ticket for new template category. Returns 204.
- **Payload fields**: service_name, email_address, service_id, template_category_name_en, template_category_name_fr, template_id.
- **Auth requirements**: Admin JWT.

---

### GET /user/`<user_id>`/organisations-and-services

- **Happy path**: Returns `{"organisations": [...], "services": [...]}`. Organisations include name, id, count_of_live_services. Services include name, id, restricted, organisation (org UUID or null).
- **Filtering**: Only active organisations and active services are returned. Inactive organisations and their inactive services are excluded. Active services in inactive orgs are also excluded.
- **Scoping**: Only returns organisations and services the requesting user belongs to. The `organisation` field on a service is its actual org UUID regardless of whether the user belongs to that org.
- **Auth requirements**: Admin JWT.

---

### POST /user/find-users-by-email

- **Happy path**: Partial and full email searches both return matching users. Returns 200 with `{"data": [...]}`.
- **No results**: Returns 200 with `{"data": []}`.
- **Validation**: `email` must be a string; non-string (e.g., integer) → 400, `{"email": ["Not a valid string."]}`.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/permissions/`<service_id>` (set_permissions)

- **Happy path**: Replaces the user's entire permission set for the given service; returns 204. Old permissions are removed.
- **Folder permissions**: Sets the list of template folder IDs the user can access for that service. Replaces any existing folder list. Does not affect folder permissions for the user's other services.
- **Error cases**: User does not belong to the service → 404.
- **Auth requirements**: Admin JWT.

---

### GET /user/`<user_id>`/fido2-keys

- **Happy path**: Returns array of FIDO2 key objects (id, ...) in creation order. Returns 200.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/fido2-keys/register

- **Happy path**: Initialises a FIDO2 registration ceremony. Returns 200 with `{"data": "<base64-CBOR>"}`. Decoded CBOR contains `publicKey.rp.id` (application hostname) and `publicKey.user.id` (user UUID as bytes).
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/fido2-keys

- **Happy path**: Completes FIDO2 key registration. Accepts `{"payload": "<base64-CBOR>"}`. Requires a valid FIDO2 session. Returns 200 with `{"id": "<key_id>"}`. Sends account-change notification.
- **Auth requirements**: Admin JWT.

---

### POST /user/`<user_id>`/fido2-keys/authenticate

- **Happy path**: Initialises a FIDO2 authentication ceremony using the user's registered keys. Returns 200 with `{"data": "<base64-CBOR>"}` containing `rpId`.
- **Auth requirements**: Admin JWT.

---

### DELETE /user/`<user_id>`/fido2-keys/`<key_id>`

- **Happy path**: Deletes the specified FIDO2 key; returns 200 with the deleted key's id. Key count goes to zero. Sends account-change notification.
- **Auth requirements**: Admin JWT.

---

### GET /user/`<user_id>`/login-events

- **Happy path**: Returns login event objects in reverse-chronological order. Returns 200.
- **Auth requirements**: Admin JWT.

---

### POST /service/`<service_id>`/invite (create invited user)

- **Happy path**: Creates an `InvitedUser` record; returns 201 with data including service, email_address, from_user, permissions, auth_type, id, folder_permissions. Sends invitation email via `deliver_email` on `notify-internal-tasks` queue.
- **Notification**: Reply-to is the invitor's email address. Personalisation: `service_name`, `user_name`, `url` (starts with `invite_link_host` or default `http://localhost:6012/invitation/`).
- **Validation rules**: `email_address` must be a valid email; missing/invalid → 400, `{"email_address": ["Not a valid email address"]}`. Default `auth_type` is `email_auth` when omitted.
- **Auth requirements**: Admin JWT.

---

### GET /service/`<service_id>`/invite

- **Happy path**: Returns all pending invitations for the service in `{"data": [...]}`. Returns empty list if none. Each invite includes service, from_user, auth_type, id.
- **Auth requirements**: Admin JWT.

---

### POST /service/`<service_id>`/invite/`<invite_id>` (update invitation)

- **Happy path**: Updates the invitation (e.g., `status: "cancelled"`); returns 200 with updated data.
- **Error cases**:
  - invite_id belongs to a different service → 404.
  - Invalid status value → 400.
- **Auth requirements**: Admin JWT.

---

### GET /invite/`<invitation_type>`/`<token>`  (`invitation_type` = `service` | `organisation`)

- **Happy path**: Decodes token, fetches and returns the invited user record. Returns 200. For service invites: includes id, email_address, from_user, service, status, permissions, folder_permissions. For org invites: returns serialised org invite object.
- **Error cases**:
  - Expired token → 400, `{"invitation": "invitation expired"}`.
  - Malformed/truncated token → 400, `{"invitation": "bad invitation link"}`.
  - Valid token but invited user does not exist → 404, `"No result found"`.
- **Auth requirements**: Admin JWT.

---

## DAO Behavior Contracts

### `save_model_user` / `get_user_by_id`

- **Expected behavior**: `save_model_user` persists a new User. `platform_admin` defaults to `False`. `get_user_by_id()` with no args returns all users. `get_user_by_id(user_id=<UUID>)` returns the matching user.
- **Edge cases verified**: Non-existent UUID raises `NoResultFound`. Invalid UUID format (non-UUID string) raises `DataError`.

---

### `delete_model_user`

- **Expected behavior**: Removes user record from the database.
- **Edge cases verified**: None beyond basic deletion.

---

### `increment_failed_login_count` / `reset_failed_login_count`

- **Expected behavior**: `increment` adds 1 to `failed_login_count`. `reset` sets it to 0.
- **Edge cases verified**: None beyond basic increment/reset.

---

### `get_user_by_email`

- **Expected behavior**: Returns user matching the given email address.
- **Edge cases verified**: Lookup is case-insensitive (upper-cased email finds the record).

---

### `save_user_attribute`

- **Expected behavior**: Updates the specified fields on the user object.
- **Edge cases verified**: When `blocked=True` is included in the update dict, `current_session_id` is force-set to the nil UUID (`00000000-0000-0000-0000-000000000000`), regardless of other fields updated in the same call.

---

### `update_user_password`

- **Expected behavior**: Replaces the password hash; new password verifies successfully. Sets `password_expired = False`.
- **Edge cases verified**: Previously expired password is cleared on update.

---

### `delete_codes_older_created_more_than_a_day_ago`

- **Expected behavior**: Deletes `VerifyCode` records with `created_at` older than 24 hours.
- **Edge cases verified**: Records created exactly at the 24-hour boundary are deleted. Records at 23h 59m 59s are kept.

---

### `verify_within_time`

- **Expected behavior**: Returns the count of `VerifyCode` records for a user created within the configured time window (≈30 seconds).
- **Edge cases verified**: Codes at 0s and 10s old count; codes at 32s and 1h old do not.

---

### `count_user_verify_codes`

- **Expected behavior**: Counts unexpired, unused codes for a user.
- **Edge cases verified**: Codes with `code_used=True` are excluded. Codes whose `expiry_datetime` is in the past are excluded. Only counts valid, unconsumed codes.

---

### `create_secret_code`

- **Expected behavior**: Returns a 5-digit numeric code as a string.
- **Edge cases verified**: Successive calls produce different codes.

---

### `dao_archive_user`

- **Expected behavior**: Atomically removes the user from all services and organisations, clears all permissions, sets auth_type to `email_auth`, prefixes email address with `_archived_<YYYY-MM-DD>_`, sets `mobile_number` to null, sets `current_session_id` to nil UUID, sets `state` to `inactive`, and invalidates the password.
- **Edge cases verified**: `get_permissions()` returns `{}` after archive. `services` and `organisations` are empty lists. Folder permissions on service-user join records are also cleared. Raises `InvalidRequest` if `user_can_be_archived` returns `False`.

---

### `user_can_be_archived`

- **Expected behavior**: Returns `True` if the user can safely be removed from all services without leaving any service without a manage-settings owner.
- **Edge cases verified**:
  - User with no services → `True`.
  - User whose only services are all inactive → `True`.
  - All remaining service members have `manage_settings` → `True`.
  - Service has only pending/inactive other members (no active manage_settings holder) → `False`.
  - Other members lack `manage_settings` permission → `False`.

---

### `get_services_for_all_users`

- **Expected behavior**: Returns a list of `(user_id, email_address, [service_ids])` tuples covering every user and their associated services.
- **Edge cases verified**: A user belonging to multiple services appears once with a list of all service IDs.

---

### `permission_dao.get_permissions_by_user_id`

- **Expected behavior**: Returns all permission records for a user. Default service setup yields 8 permissions: `manage_users`, `manage_templates`, `manage_settings`, `send_texts`, `send_emails`, `send_letters`, `manage_api_keys`, `view_activity`.
- **Edge cases verified**: Only permissions from active services are returned; inactive services are excluded even if the user has permissions on them.

---

### `permission_dao.get_team_members_with_permission`

- **Expected behavior**: Returns all users who have the given permission on the specified service.
- **Edge cases verified**:
  - Returns the user when assigned.
  - Returns empty list after `remove_user_service_permissions` is called.
  - Returns empty list when `permission=None`.

---

### `save_fido2_key` / `list_fido2_keys` / `get_fido2_key` / `delete_fido2_key`

- **Expected behavior**: `save_fido2_key` persists a `Fido2Key`. `list_fido2_keys(user_id)` returns all keys for that user. `get_fido2_key(user_id, key_id)` returns the specific key. `delete_fido2_key(user_id, key_id)` removes it entirely.
- **Edge cases verified**: `list_fido2_keys` reflects records not yet flushed to session (queries across session boundary). After `delete_fido2_key`, `Fido2Key.query.count() == 0`.

---

### `create_fido2_session` / `get_fido2_session`

- **Expected behavior**: `create_fido2_session(user_id, state)` stores a `Fido2Session`. `get_fido2_session(user_id)` retrieves the state string AND immediately deletes the session record (consume-once semantics).
- **Edge cases verified**: Calling `create_fido2_session` a second time for the same user deletes the first session — only one session per user exists at any time (`Fido2Session.query.count() == 1`). After `get_fido2_session`, `Fido2Session.query.count() == 0`.

---

### `save_login_event` / `list_login_events`

- **Expected behavior**: `save_login_event` persists a `LoginEvent`. `list_login_events(user_id)` returns all events for that user.
- **Edge cases verified**: Two events for the same user both appear in the list (count = 2).

---

### `save_invited_user` (InvitedUser)

- **Expected behavior**: Persists an `InvitedUser` with service, email_address, from_user, permissions (comma-separated string), folder_permissions (list). `get_permissions()` returns the split list.
- **Edge cases verified**: `folder_permissions` defaults to `[]` when not supplied. Status can be mutated (e.g., `pending → cancelled`) and re-saved. Invitation cleanup (`delete_invitations_created_more_than_two_days_ago`) removes records older than 48 hours; records at 47h 59m 59s are kept.

---

### `get_invited_user` / `get_invited_user_by_id`

- **Expected behavior**: `get_invited_user(service_id, id)` is scoped to the service; raises `NoResultFound` if the id is unknown or belongs to a different service. `get_invited_user_by_id(id)` performs a global lookup.
- **Edge cases verified**: Unknown UUID raises `NoResultFound`.

---

### `get_invited_users_for_service`

- **Expected behavior**: Returns all `InvitedUser` records for the given service. Returns an empty list when there are none.
- **Edge cases verified**: All 5 created invites are retrievable in the list.

---

## Authentication Behavior Verified

### JWT token validation

- `Authorization` header is required on all protected endpoints. Missing header → `"Unauthorized, authentication token must be provided"`.
- Only `Bearer` and `ApiKey-v1` schemes are recognised; any other scheme (e.g., `Basic`) → error listing supported schemes.
- Malformed Bearer value (not a valid JWT) → `"Invalid token: signature, api token is not valid"`.
- `iss` claim required; missing → `"Invalid token: iss field not provided"`.
- `iat` claim required; missing or non-integer → `"Invalid token: …"`.
- Token age: `iat` must be within 30 seconds of the server clock; older tokens → `"Error: Your system clock must be accurate to within 30 seconds"` (with structured warning log including service_id, client, iat value, and server clock).
- JWT algorithm: only `HS256` accepted; other algorithms (e.g., `HS512`) → 403 (with warning log).
- Invalid signature → 403 (with warning log including service_id and client).
- `iss` must be a valid UUID-format service_id or the `ADMIN_CLIENT_USER_NAME`; wrong data type → `"Invalid token: service id is not the right data type"`.
- Service must exist → otherwise `"Invalid token: service not found"`.
- Service must be active → inactive service → `"Invalid token: service is archived"`.
- Service must have at least one API key → `"Invalid token: service has no API keys"`.
- Token signed with any of the service's non-expired keys is accepted (multi-key support).
- Token signed with an expired key while other valid keys exist → `AuthError` with `service_id` and `api_key_id` populated.
- On success, the matched API key is attached to `g.api_user`.

### API key validation (`ApiKey-v1` scheme)

- Scheme name is case-insensitive: `ApiKey-v1`, `apikey-v1`, `APIKEY-V1` are all accepted.
- Extra whitespace between scheme and token is tolerated.
- Token format: `gcntfy-keyname-<service_id>-<unsigned_secret>`.
- Incorrect format (e.g., wrong prefix or wrong structure) → 403, `"Invalid token: Enter your full API key"`.
- Entirely invalid value → 403, `"Invalid token: Enter your full API key"`.
- Expired API key → 403, `"Invalid token: API key revoked"`.
- Valid key → 200.
- `ApiKey-v1` is rejected for admin-only routes → `"Invalid scheme: can only use JWT for admin authentication"`.

### Auth failure scenarios (all tested error cases)

| Scenario | HTTP status | Message |
|---|---|---|
| No `Authorization` header | 401 | `"Unauthorized, authentication token must be provided"` |
| `Basic` scheme | error | lists supported schemes |
| Malformed Bearer token | error | `"Invalid token: signature, api token is not valid"` |
| JWT missing `iss` | error | `"Invalid token: iss field not provided"` |
| JWT missing `iat` (service) | error | `"Invalid token: signature, api token not found"` |
| JWT missing `iat` (admin) | error | `"Invalid token: signature, api token is not valid"` |
| JWT non-integer `iat` | error | `"Invalid token: …"` |
| Token expired (>30s) | 403 | `"Error: Your system clock must be accurate to within 30 seconds"` |
| Wrong JWT algorithm | 403 | `"Invalid token: signature, api token not found"` |
| Invalid JWT signature | 403 | `"Invalid token: signature, api token not found"` |
| Non-UUID service_id in `iss` | 403 | `"Invalid token: service id is not the right data type"` |
| Service not found | 403 | `"Invalid token: service not found"` |
| Service inactive | 403 | `"Invalid token: service is archived"` |
| Service has no API keys | error w/ service_id | `"Invalid token: service has no API keys"` |
| Empty admin secret | 403 | `"Invalid token: signature, api token is not valid"` |
| Wrong admin secret | 403 | `"Invalid token: signature, api token is not valid"` |
| Expired API key (ApiKey-v1) | 403 | `"Invalid token: API key revoked"` |
| Invalid API key format | 403 | `"Invalid token: Enter your full API key"` |
| `ApiKey-v1` on admin route | error | `"Invalid scheme: can only use JWT for admin authentication"` |

### Route-level auth requirements

- **Enforced globally**: Every blueprint registered on the application must have a `before_request` function. A test explicitly asserts that the set of blueprint names equals the set of blueprints that have `before_request` handlers — no blueprint can be registered without auth middleware.
- **Admin routes** (e.g., `/service`, `/user`): `requires_admin_auth` — only admin JWT accepted.
- **Public service routes** (e.g., `/notifications`, `/v2/notifications`): `requires_auth` — JWT or ApiKey-v1 accepted.
- **Cache-clear routes**: `requires_cache_clear_auth`.
- **Proxy header (`X-Custom-Forwarder`)**: When `CHECK_PROXY_HEADER=True`, admin-authed endpoints additionally require the correct proxy key; wrong key → 403. When `CHECK_PROXY_HEADER=False`, header is ignored. Non-auth endpoints (e.g., `/_status`) never enforce the proxy header.
- **CORS OPTIONS**: `OPTIONS` requests bypass all authentication — `requires_auth` returns without raising. Preflight responses include `Access-Control-Allow-Headers: Content-Type, Authorization` and `Access-Control-Allow-Methods`. The same endpoint rejects unauthenticated `GET` requests with 401.

---

## Business Rules Verified

### OTP / verify code flow

- Code types: `sms` and `email`.
- SMS codes: 5-digit numeric. Email codes: UUID-formatted (for the main 2FA flow); no `VerifyCode` record created for the new-user email-verification path.
- Successive `create_secret_code()` calls produce different values.
- Retry deduplication: a second request to send an SMS code within the retry delta for the same destination number does not create a new code and does not dispatch a new notification.
- Max-codes guard: if 10 or more unexpired codes exist for the user, no new code is generated; endpoint still returns 204.
- Code cleanup: `delete_codes_older_created_more_than_a_day_ago` removes any code with `created_at` > 24 hours ago.
- Account lockout on `verify_user_code`: once `failed_login_count >= 10`, even a correct code is rejected (404) — the lockout takes precedence over code validity.
- `verify_2fa_code` does NOT enforce lockout and does NOT increment `failed_login_count` on any failure mode.
- Successful `verify_user_code` resets `failed_login_count` to 0, sets `logged_in_at = now()`, and generates a new `current_session_id`.

### FIDO2

- Registration ceremony: `POST /fido2-keys/register` writes a `Fido2Session` (replacing any previous session for the user) and returns CBOR-encoded registration options with `rp.id = <hostname>` and `user.id = <user UUID bytes>`.
- Session consume-once: `get_fido2_session` both retrieves the state and deletes the row; the session cannot be replayed.
- Key creation: `POST /fido2-keys` with a base64-encoded CBOR payload; server calls `decode_and_register` with the session state. On success, saves `Fido2Key` and sends an account-change notification.
- Key listing returns keys in creation order.
- Key deletion removes the record and triggers an account-change notification.
- Authentication ceremony: `POST /fido2-keys/authenticate` uses the user's stored keys (each deserialized via `deserialize_fido2_key`) to call `FIDO2_SERVER.authenticate_begin`; returns CBOR-encoded authentication options.

### User permissions

- Default set of 8 permissions: `manage_users`, `manage_templates`, `manage_settings`, `send_texts`, `send_emails`, `send_letters`, `manage_api_keys`, `view_activity`.
- Permission assignment is **replace-not-append**: `set_permissions` removes all existing permissions for the user/service pair and writes the new set.
- Multiple permissions can be assigned in a single call.
- Folder permissions are stored on the `service_user` join record and scoped per service; updating folders for one service does not affect other services.
- Archiving a user clears all their permissions.
- `get_permissions_by_user_id` excludes permissions from inactive services.

### Invitation flow

- Creating an invite triggers an email notification to the invited address with personalisation: `service_name`, `user_name`, `url`. Reply-to is the invitor's email address.
- The `url` is constructed from an optional `invite_link_host` (defaults to `http://localhost:6012/invitation/`) followed by the invite token.
- The invite token is a URL-safe signed token (`generate_token`) using the application `SECRET_KEY`.
- Token validation reads a TTL from the token; expired tokens return 400 `{"invitation": "invitation expired"}`.
- Truncated/malformed tokens return 400 `{"invitation": "bad invitation link"}`.
- Valid token for an unknown user ID returns 404.
- Both service invitations and organisation invitations use the same token/validation mechanism via the `/invite/<type>/<token>` endpoint.
- Invite status lifecycle: created as `pending`; can be set to `cancelled` via the update endpoint. Invalid status values return 400.
- Stale invites (>48 hours old) are removed by `delete_invitations_created_more_than_two_days_ago`.

### User state transitions

- **`pending` → `active`**: via `POST /user/<id>/activate`; creates a Salesforce contact.
- **`active` → `inactive` (archive)**: via `POST /user/<id>/archive`; anonymises email address (`_archived_<date>_<original>`), removes from all services/orgs, clears permissions, nulls mobile, resets session, invalidates password.
- **`active` → `inactive` (deactivate)**: via `POST /user/<id>/deactivate`; sends deactivation email to the user; affects service state based on membership count (see deactivate endpoint contract above).
- **Blocked state**: setting `blocked=True` sets `current_session_id` to nil UUID (invalidating all sessions); `check_password` returns `False` for blocked users regardless of password correctness.
- **Password expiry**: `update_user_password` clears the `password_expired` flag.
- **Session management**: successful `verify_user_code` issues a new `current_session_id`; archiving or blocking sets it to the nil UUID `00000000-0000-0000-0000-000000000000`.

### Encryption (`CryptoSigner`)

- Sign/verify round-trip is lossless for strings and dicts.
- Different secrets → `BadSignature` on `verify`.
- Different salts → `BadSignature` on `verify`.
- Secret rotation: `CryptoSigner` accepts a list of secrets. If two instances share at least one secret, the second instance can verify a token signed by the first (tried sequentially).
- `verify_unsafe`: decodes the payload without validating the signature; always returns the original value regardless of secret mismatch (intended for migration or inspection only).
- `sign_with_all_keys(value)`: returns a list of signatures signed with each key (in reverse order of the key list), allowing broadcast/rotation scenarios.

### `ContactRequest` model

- Required field: `email_address` (must be a non-empty, valid email address). Missing, empty, or malformed email raises `TypeError`, `AssertionError`, or `InvalidEmailError`.
- Unknown keyword arguments raise `TypeError`.
- Default values: `language="en"`, `friendly_support_type="Support Request"`, all other string fields default to `""`.
- HTML input sanitisation: angle brackets and special characters in user-supplied fields are HTML-escaped before storage (XSS protection). Example: `<script>` → `&lt;script&gt;`.
