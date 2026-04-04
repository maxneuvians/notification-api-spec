## Requirements

### Requirement: List all users
The `GET /user` endpoint SHALL return all users with name, mobile_number, email_address, state, and a permissions map keyed by service UUID.

#### Scenario: Returns all users with permission map
- **WHEN** `GET /user` is called with a valid admin JWT
- **THEN** HTTP 200 and each user object includes `name`, `mobile_number`, `email_address`, `state`, and `permissions`

#### Scenario: Requires admin JWT
- **WHEN** `GET /user` is called without an Authorization header
- **THEN** HTTP 401

---

### Requirement: Fetch single user by ID
The `GET /user/{id}` endpoint SHALL return a single user with full detail including permissions, services, and organisations.

#### Scenario: Returns full user detail
- **WHEN** `GET /user/{id}` is called with a valid user UUID
- **THEN** HTTP 200, response includes `id`, `name`, `mobile_number`, `email_address`, `state`, `auth_type`, `default_editor_is_rte`, `permissions`, `services`, `organisations`

#### Scenario: default_editor_is_rte defaults to false
- **WHEN** the user was created without setting `default_editor_is_rte`
- **THEN** `GET /user/{id}` returns `default_editor_is_rte: false`

#### Scenario: Inactive services excluded from services list and permissions
- **WHEN** the user belongs to both an active and an inactive service
- **THEN** the inactive service is absent from `services` and `permissions`

#### Scenario: Inactive organisations excluded
- **WHEN** the user belongs to an inactive organisation
- **THEN** that organisation is absent from `organisations`

---

### Requirement: Fetch user by email address
The `GET /user/email` endpoint SHALL look up a user by exact email and return `password_expired`.

#### Scenario: Returns matching user
- **WHEN** `GET /user/email?email=user@example.com` matches an existing user
- **THEN** HTTP 200, response includes `password_expired`

#### Scenario: Missing email param returns 400
- **WHEN** `GET /user/email` is called without the `email` query param
- **THEN** HTTP 400, body contains `"Invalid request. Email query string param required"`

#### Scenario: No matching user returns 404
- **WHEN** `GET /user/email?email=unknown@example.com` matches no user
- **THEN** HTTP 404, body `{"result": "error", "message": "No result found"}`

---

### Requirement: Create user
`POST /user` SHALL create a user with a hashed password, defaulting to `email_auth`.

#### Scenario: Valid creation returns 201
- **WHEN** `POST /user` is called with valid `email_address`, `password`, and `name`
- **THEN** HTTP 201, response includes `id` and `email_address`; password stored hashed

#### Scenario: Default auth_type is email_auth
- **WHEN** `POST /user` body omits `auth_type`
- **THEN** created user has `auth_type = email_auth`

#### Scenario: sms_auth without mobile rejects
- **WHEN** `POST /user` sets `auth_type = sms_auth` with `mobile_number: null`
- **THEN** HTTP 400, body contains `"Mobile number must be set if auth_type is set to sms_auth"`

#### Scenario: Banned password rejected
- **WHEN** `POST /user` supplies a common/banned password
- **THEN** HTTP 400, body `{"password": ["Password is not allowed."]}`

#### Scenario: Missing email rejects
- **WHEN** `POST /user` body omits `email_address`
- **THEN** HTTP 400, body `{"email_address": ["Missing data for required field."]}`

#### Scenario: Missing password rejects
- **WHEN** `POST /user` body omits `password`
- **THEN** HTTP 400, body `{"password": ["Missing data for required field."]}`

---

### Requirement: Update user attributes
`POST /user/{id}` SHALL update user fields and trigger notifications for name/email/mobile changes.

#### Scenario: Valid update returns 200 and calls Salesforce
- **WHEN** `POST /user/{id}` updates the user's `name`
- **THEN** HTTP 200; `salesforce_client.contact_update` called

#### Scenario: Password cannot be updated via this endpoint
- **WHEN** `POST /user/{id}` body includes a `password` field
- **THEN** HTTP 400, body `{"_schema": ["Unknown field name password"]}`

#### Scenario: sms_auth with null mobile rejected
- **WHEN** `POST /user/{id}` sets `auth_type = sms_auth` and `mobile_number: null`
- **THEN** HTTP 400, body contains `"Mobile number must be set if auth_type is set to sms_auth"`

#### Scenario: Empty string mobile rejected
- **WHEN** `POST /user/{id}` sets `mobile_number: ""`
- **THEN** HTTP 400, body contains `"Invalid phone number: Not a valid international number"`

#### Scenario: Can clear mobile when switching to email_auth
- **WHEN** `POST /user/{id}` simultaneously sets `auth_type = email_auth` and `mobile_number: null`
- **THEN** HTTP 200; `mobile_number` is null on the updated user

#### Scenario: Email/mobile/name change triggers notification with updater name
- **WHEN** `POST /user/{id}` changes `email_address` and `updated_by` is provided
- **THEN** HTTP 200; notification sent to new email with personalization including updater's name; template `c73f1d71`

#### Scenario: blocked=true invalidates session, no notification
- **WHEN** `POST /user/{id}` sets `blocked: true`
- **THEN** HTTP 200; `current_session_id` set to nil UUID; no notification email dispatched

#### Scenario: default_editor_is_rte change does not notify
- **WHEN** `POST /user/{id}` only changes `default_editor_is_rte`
- **THEN** HTTP 200; no notification sent

---

### Requirement: Activate user
`POST /user/{id}/activate` SHALL transition a pending user to active and create a Salesforce contact.

#### Scenario: Activates pending user
- **WHEN** `POST /user/{id}/activate` on a pending user
- **THEN** HTTP 200; user state is `active`; `salesforce_client.contact_create` called

#### Scenario: Already-active user rejected
- **WHEN** `POST /user/{id}/activate` on an already active user
- **THEN** HTTP 400, body contains `"User already active"`

---

### Requirement: Deactivate user
`POST /user/{id}/deactivate` SHALL set the user inactive and may suspend live services when the last member is deactivated.

#### Scenario: Deactivates user and sends deactivation email
- **WHEN** `POST /user/{id}/deactivate` on an active user
- **THEN** HTTP 200; user `state = inactive`; deactivation notification email sent to user

#### Scenario: Live service with one remaining member is suspended
- **WHEN** deactivating a user who is the second-to-last active member of a live service
- **THEN** service gets `active=False`, `suspended_at=now`, and a suspension notification is sent to remaining member

#### Scenario: Live service becoming empty is deactivated without suspended_at
- **WHEN** deactivating the last active member of a live service
- **THEN** service gets `active=False`; `suspended_at` is NOT set; no suspension email

#### Scenario: Trial service with one remaining member is unchanged
- **WHEN** deactivating a user who is the second-to-last member of a trial service
- **THEN** trial service state is unchanged; no suspension email

#### Scenario: Live service with 2+ remaining members is unchanged
- **WHEN** deactivating a user from a live service that has 2+ other active members
- **THEN** service state unchanged

#### Scenario: Exception rolls back entire transaction
- **WHEN** an unexpected error occurs mid-deactivation
- **THEN** HTTP 500; all DB changes rolled back

---

### Requirement: Archive user
`POST /user/{id}/archive` SHALL permanently decommission a user row while retaining it for audit.

#### Scenario: Archives user and renames email
- **WHEN** `POST /user/{id}/archive` on an archivable user
- **THEN** HTTP 204; email renamed to `_archived_{YYYY-MM-DD}_{original_email}`; `mobile_number` null; session=nil UUID; state=inactive; all permissions removed

#### Scenario: Non-existent user returns 404
- **WHEN** `POST /user/{id}/archive` with an unknown user UUID
- **THEN** HTTP 404

#### Scenario: Sole manage_settings holder cannot be archived
- **WHEN** the user is the only active member with `manage_settings` on an active service
- **THEN** HTTP 400, body contains `"User cannot be removed from service. Check that all services have another team member who can manage settings"`

#### Scenario: User with no services can be archived
- **WHEN** the user belongs to no services
- **THEN** `POST /user/{id}/archive` returns HTTP 204 without error

---

### Requirement: Search users by partial email
`POST /user/find-users-by-email` SHALL support partial-match email lookup.

#### Scenario: Partial match returns results
- **WHEN** `POST /user/find-users-by-email` with `{"email": "jane"}` matching existing users
- **THEN** HTTP 200, `{"data": [<matching users>]}`

#### Scenario: No match returns empty data
- **WHEN** no users match the given partial email
- **THEN** HTTP 200, `{"data": []}`

#### Scenario: Non-string email rejected
- **WHEN** `POST /user/find-users-by-email` with `{"email": 123}`
- **THEN** HTTP 400, body `{"email": ["Not a valid string."]}`

---

### Requirement: Get user organisations and services
`GET /user/{id}/organisations-and-services` SHALL return the user's active services and active organisations scoped to their own memberships.

#### Scenario: Returns active orgs and services
- **WHEN** `GET /user/{id}/organisations-and-services` for a user with memberships
- **THEN** HTTP 200, response has `organisations` (active only) and `services` (active, in active orgs only)

#### Scenario: Inactive orgs and services excluded
- **WHEN** the user belongs to an inactive org and an inactive service
- **THEN** both are absent from the response

#### Scenario: Service organisation field is actual org UUID
- **WHEN** a service belongs to an org the user is not a member of
- **THEN** `organisation` field on the service still shows the org UUID

---

### Requirement: Update password
`POST /user/{id}/update-password` SHALL hash and store a new password and optionally record a login event.

#### Scenario: Valid password update returns 200
- **WHEN** `POST /user/{id}/update-password` with a non-banned password
- **THEN** HTTP 200; `password_changed_at` set in response

#### Scenario: Banned password rejected
- **WHEN** `POST /user/{id}/update-password` with a known-bad password
- **THEN** HTTP 400

#### Scenario: LoginData present creates LoginEvent
- **WHEN** request body includes `loginData`
- **THEN** a `LoginEvent` record is created for the user

#### Scenario: No loginData means no LoginEvent
- **WHEN** request body does not include `loginData`
- **THEN** no `LoginEvent` record is created

---

### Requirement: Verify password
`POST /user/{id}/verify-password` SHALL check the password hash and reset failed login count on success.

#### Scenario: Correct password returns 204 and resets count
- **WHEN** `POST /user/{id}/verify-password` with the correct password
- **THEN** HTTP 204; `failed_login_count` reset to 0; `logged_in_at` NOT updated

#### Scenario: Missing password field returns 400
- **WHEN** `POST /user/{id}/verify-password` body omits `password`
- **THEN** HTTP 400, body contains `"Required field missing data"`; `failed_login_count` unchanged

#### Scenario: Wrong password returns 400 and increments count
- **WHEN** `POST /user/{id}/verify-password` with an incorrect password
- **THEN** HTTP 400, body contains `"Incorrect password for user_id"`; `failed_login_count` incremented by 1

---

### Requirement: Reset failed login count
`POST /user/{id}/reset-failed-login-count` SHALL set `failed_login_count` to zero.

#### Scenario: Resets count and returns 200
- **WHEN** `POST /user/{id}/reset-failed-login-count` on an existing user
- **THEN** HTTP 200; `failed_login_count = 0`

#### Scenario: Non-existent user returns 404
- **WHEN** `POST /user/{id}/reset-failed-login-count` with an unknown UUID
- **THEN** HTTP 404

---

### Requirement: Reset password (request email)
`POST /user/{id}/reset-password` SHALL send a password-reset email.

#### Scenario: Valid request sends email and returns 204
- **WHEN** `POST /user/{id}/reset-password` with valid `email` for an unblocked user
- **THEN** HTTP 204; password-reset email dispatched via `deliver_email`

#### Scenario: Missing email field returns 400
- **WHEN** body omits `email`
- **THEN** HTTP 400, body `{"email": ["Missing data for required field."]}`

#### Scenario: Invalid email format returns 400
- **WHEN** body has `email` with an invalid format
- **THEN** HTTP 400, body `{"email": ["Not a valid email address"]}`

#### Scenario: Blocked user returns 400
- **WHEN** `POST /user/{id}/reset-password` on a blocked user
- **THEN** HTTP 400, body contains `"user blocked"`

#### Scenario: Unknown user returns 404
- **WHEN** user_id does not exist
- **THEN** HTTP 404

---

### Requirement: Forced password reset
`POST /user/{id}/forced-password-reset` SHALL send a forced-reset email using the forced template.

#### Scenario: Valid request uses forced template and returns 204
- **WHEN** `POST /user/{id}/forced-password-reset` on an existing user
- **THEN** HTTP 204; email dispatched using `forced_password_reset_email_template`

#### Scenario: Unknown user returns 404
- **WHEN** user_id does not exist
- **THEN** HTTP 404

---

### Requirement: Send already-registered email
`POST /user/{id}/email` SHALL send an "already registered" notification.

#### Scenario: Sends notification and returns 204
- **WHEN** `POST /user/{id}/email` with a valid `email` field
- **THEN** HTTP 204; already-registered email dispatched

#### Scenario: Missing email field returns 400
- **WHEN** body omits `email`
- **THEN** HTTP 400, body `{"email": ["Missing data for required field."]}`

---

### Requirement: Confirm new email address
`POST /user/{id}/confirm-new-email` SHALL send a confirmation email to the new address.

#### Scenario: Sends confirmation and returns 204
- **WHEN** `POST /user/{id}/confirm-new-email` with a valid `email`
- **THEN** HTTP 204; confirmation email sent to the new address

#### Scenario: Missing email field returns 400
- **WHEN** body omits `email`
- **THEN** HTTP 400, body `{"email": ["Missing data for required field."]}`

---

### Requirement: Contact request (Freshdesk ticket)
`POST /user/{id}/contact-request` SHALL send a Freshdesk support ticket with context-specific tags and Salesforce engagement update for go-live requests.

#### Scenario: Demo/ask_question context sends ticket with skip tags
- **WHEN** `support_type` is `ask_question` or `demo` (no live service)
- **THEN** HTTP 204; ticket created with `z_skip_opsgenie` and `z_skip_urgent_escalation` tags; no Salesforce call

#### Scenario: Live service sends ticket without Salesforce
- **WHEN** user has a live service and `support_type` is not `go_live_request`
- **THEN** HTTP 204; ticket created; `salesforce_client.engagement_update` NOT called

#### Scenario: go_live_request triggers Salesforce engagement update
- **WHEN** `support_type = go_live_request` with `service_id` provided
- **THEN** HTTP 204; `salesforce_client.engagement_update(ENGAGEMENT_STAGE_ACTIVATION)` called with `main_use_case` as description

#### Scenario: PT service with feature flag uses alternate Freshdesk call
- **WHEN** `organisation_type = province_or_territory` AND `FF_PT_SERVICE_SKIP_FRESHDESK=True`
- **THEN** HTTP 201; `Freshdesk.email_freshdesk_ticket_pt_service()` called; `send_ticket()` NOT called

#### Scenario: Central service with feature flag still calls send_ticket
- **WHEN** service is not province_or_territory AND `FF_PT_SERVICE_SKIP_FRESHDESK=True`
- **THEN** HTTP 204; `send_ticket()` called as normal

---

### Requirement: Branding request
`POST /user/{id}/branding-request` SHALL send a Freshdesk branding ticket.

#### Scenario: Sends branding ticket and returns 204
- **WHEN** `POST /user/{id}/branding-request` with required fields
- **THEN** HTTP 204; Freshdesk ticket created with branding payload fields

---

### Requirement: New template category request
`POST /user/{id}/new-template-category-request` SHALL send a Freshdesk ticket for a new template category.

#### Scenario: Sends ticket and returns 204
- **WHEN** `POST /user/{id}/new-template-category-request` with required fields
- **THEN** HTTP 204; Freshdesk ticket created with template category payload fields

---

### Requirement: Send 2FA SMS code
`POST /user/{id}/2fa-code` with `code_type=sms` SHALL generate a 5-digit code, store it hashed, and send it via SMS.

#### Scenario: Code generated, stored hashed, and SMS sent
- **WHEN** `POST /user/{id}/2fa-code` with `code_type=sms` for an existing user
- **THEN** HTTP 204; a `VerifyCode` row inserted with the code hashed; SMS dispatched via `deliver_sms`

#### Scenario: to override sends to alternate number
- **WHEN** `to` field is provided in request body
- **THEN** SMS sent to the override number, not the account mobile

#### Scenario: Retry within 30-second window returns 204 without new code or SMS
- **WHEN** a second request arrives within 30 seconds for the same destination
- **THEN** HTTP 204; no new `VerifyCode` row created; no new SMS dispatched

#### Scenario: 10+ unexpired codes guard returns 204 without new code
- **WHEN** user already has 10 or more unexpired, unused `VerifyCode` rows
- **THEN** HTTP 204; no new code created

#### Scenario: Non-existent user returns 404
- **WHEN** user_id does not exist
- **THEN** HTTP 404, body contains `"No result found"`

---

### Requirement: Send 2FA email code
`POST /user/{id}/2fa-code` with `code_type=email` SHALL send a 5-digit OTP by email using `EMAIL_2FA_TEMPLATE_ID`.

#### Scenario: Code sent to account email
- **WHEN** `code_type=email` and no `to` override
- **THEN** HTTP 204; email sent to user's `email_address`; personalisation includes `name` and `verify_code`

#### Scenario: to override sends to alternate address
- **WHEN** `to` field is provided
- **THEN** email sent to override address

#### Scenario: email_auth_link_host and next included in personalisation
- **WHEN** `email_auth_link_host` and `next` are provided
- **THEN** both appear in email personalisation

---

### Requirement: Send email verification (new user)
`POST /user/{id}/email-verification` SHALL dispatch a sign-up verification link without creating a VerifyCode record.

#### Scenario: Sends verification link email
- **WHEN** `POST /user/{id}/email-verification` on an existing user
- **THEN** HTTP 204; verification link email dispatched; no `VerifyCode` row inserted

#### Scenario: Non-existent user returns 404
- **WHEN** user_id does not exist
- **THEN** HTTP 404

---

### Requirement: Verify OTP code (primary login step)
`POST /user/{id}/verify-code` SHALL verify an OTP, enforce account lockout, and set session on success.

#### Scenario: Valid code marks used and resets session
- **WHEN** `POST /user/{id}/verify-code` with a valid, unused, unexpired code
- **THEN** HTTP 204; `code_used=true`; `logged_in_at=now`; new `current_session_id` generated; `failed_login_count=0`

#### Scenario: Missing code returns 400 without incrementing count
- **WHEN** request body omits `code`
- **THEN** HTTP 400; `failed_login_count` unchanged

#### Scenario: Not-found code returns 404 and increments count
- **WHEN** supplied code value has no matching `VerifyCode` row
- **THEN** HTTP 404; `failed_login_count` incremented by 1

#### Scenario: Expired code returns 400 and increments count
- **WHEN** the matching `VerifyCode` row has `expiry_datetime` in the past
- **THEN** HTTP 400; `failed_login_count` incremented by 1

#### Scenario: Already-used code returns 400 without incrementing count
- **WHEN** the matching `VerifyCode` has `code_used=true`
- **THEN** HTTP 400; `failed_login_count` unchanged

#### Scenario: Account locked at 10 failures returns 404 even for correct code
- **WHEN** `failed_login_count >= 10` at time of request
- **THEN** HTTP 404; code NOT marked used; `failed_login_count` unchanged

#### Scenario: E2E bypass accepts any code in dev environment
- **WHEN** `NOTIFY_ENVIRONMENT=development`, host is `localhost:3000`, and user email prefix matches `CYPRESS_EMAIL_PREFIX`
- **THEN** HTTP 204 for any submitted code value

---

### Requirement: Verify OTP code (2FA step — no lockout)
`POST /user/{id}/verify-2fa` SHALL verify an OTP without enforcing lockout or incrementing failed_login_count.

#### Scenario: Valid code succeeds with same side-effects as verify-code
- **WHEN** `POST /user/{id}/verify-2fa` with a valid, unused, unexpired code
- **THEN** HTTP 204; code marked used; `logged_in_at` set; new `current_session_id`; `failed_login_count` reset to 0

#### Scenario: Bad code returns 404 without incrementing count
- **WHEN** code value not found
- **THEN** HTTP 404; `failed_login_count` unchanged

#### Scenario: Expired code returns 400 without incrementing count
- **WHEN** code is expired
- **THEN** HTTP 400; `failed_login_count` unchanged

#### Scenario: Already-used code returns 400 without incrementing count
- **WHEN** code was already used
- **THEN** HTTP 400; `failed_login_count` unchanged

#### Scenario: Missing code returns 400 without incrementing count
- **WHEN** body omits `code`
- **THEN** HTTP 400; `failed_login_count` unchanged

#### Scenario: High failed_login_count does not block verify-2fa
- **WHEN** `failed_login_count = 15`
- **THEN** HTTP 204 for a valid code (no lockout enforced)

#### Scenario: E2E bypass works same as verify-code
- **WHEN** `NOTIFY_ENVIRONMENT=development`, host is `localhost:3000`, email matches `CYPRESS_EMAIL_PREFIX`
- **THEN** HTTP 204 for any code

---

### Requirement: FIDO2 key registration — begin ceremony
`POST /user/{id}/fido2-keys/register` SHALL initialise a WebAuthn registration and store a challenge session.

#### Scenario: Returns CBOR-encoded registration options
- **WHEN** `POST /user/{id}/fido2-keys/register` for a user
- **THEN** HTTP 200, `{"data": "<base64-CBOR>"}` where decoded CBOR contains `publicKey.rp.id = <hostname>` and `publicKey.user.id = <user UUID bytes>`

#### Scenario: Second call replaces previous session
- **WHEN** `POST /user/{id}/fido2-keys/register` called twice for the same user
- **THEN** only one `Fido2Session` row exists; second call succeeds with HTTP 200

---

### Requirement: FIDO2 key registration — complete ceremony
`POST /user/{id}/fido2-keys` SHALL complete registration, persist the credential, and send a notification.

#### Scenario: Valid attestation creates key and returns id
- **WHEN** `POST /user/{id}/fido2-keys` with valid `{"payload": "<base64-CBOR>"}` and an active session
- **THEN** HTTP 200, `{"id": "<key_id>"}`; a `Fido2Key` row created; account-change notification sent

#### Scenario: Session consumed on success (cannot replay)
- **WHEN** the registration completes successfully
- **THEN** `Fido2Session` row is deleted; `Fido2Session.query.count() == 0`

---

### Requirement: FIDO2 authentication — begin ceremony
`POST /user/{id}/fido2-keys/authenticate` SHALL load all registered keys and return an assertion challenge.

#### Scenario: Returns CBOR authentication options
- **WHEN** `POST /user/{id}/fido2-keys/authenticate` for a user with registered keys
- **THEN** HTTP 200, `{"data": "<base64-CBOR>"}` containing `rpId`

#### Scenario: Session is consume-once
- **WHEN** the authentication ceremony CBOR options are retrieved and the completion step runs
- **THEN** the `Fido2Session` is deleted; a re-use attempt fails

---

### Requirement: List FIDO2 keys
`GET /user/{id}/fido2-keys` SHALL return all registered keys in creation order.

#### Scenario: Returns keys sorted by created_at ascending
- **WHEN** user has 2 registered FIDO2 keys
- **THEN** HTTP 200; keys returned in `created_at ASC` order

#### Scenario: No keys returns empty array
- **WHEN** user has no registered FIDO2 keys
- **THEN** HTTP 200, `[]`

---

### Requirement: Delete FIDO2 key
`DELETE /user/{id}/fido2-keys/{key_id}` SHALL remove the key and send an account-change notification.

#### Scenario: Deletes key and returns 200 with key id
- **WHEN** `DELETE /user/{id}/fido2-keys/{key_id}` for an existing key
- **THEN** HTTP 200, response includes deleted key id; no `Fido2Key` rows remain for that id; account-change notification sent

#### Scenario: Non-existent key returns 404
- **WHEN** key_id does not exist for this user
- **THEN** HTTP 404

---

### Requirement: Set user permissions for a service
`POST /user/{id}/permissions/{service_id}` SHALL atomically replace all permissions.

#### Scenario: Replaces all permissions and returns 204
- **WHEN** `POST /user/{id}/permissions/{service_id}` with a new permission list
- **THEN** HTTP 204; old permissions for this user+service removed; new permissions inserted

#### Scenario: Replaces folder permissions without affecting other services
- **WHEN** `folder_permissions` list is provided
- **THEN** folder permissions for this service replaced; other services unaffected

#### Scenario: User not in service returns 404
- **WHEN** user does not belong to `service_id`
- **THEN** HTTP 404

---

### Requirement: List login events
`GET /user/{id}/login-events` SHALL return the most recent 3 login events in reverse-chronological order.

#### Scenario: Returns at most 3 events in reverse-chronological order
- **WHEN** user has more than 3 login events
- **THEN** HTTP 200; exactly 3 events returned, newest first

#### Scenario: No events returns empty list
- **WHEN** user has no login events
- **THEN** HTTP 200, `[]`

---

### Requirement: Create service invitation
`POST /service/{service_id}/invite` SHALL create an InvitedUser, send an invitation email, and return 201.

#### Scenario: Valid invite returns 201 with full invite data
- **WHEN** `POST /service/{service_id}/invite` with valid `email_address`
- **THEN** HTTP 201; response includes `service`, `email_address`, `from_user`, `permissions`, `auth_type`, `id`, `folder_permissions`; invitation email sent

#### Scenario: email_address missing or invalid returns 400
- **WHEN** `email_address` is missing or not a valid email
- **THEN** HTTP 400, body `{"email_address": ["Not a valid email address"]}`

#### Scenario: Default auth_type is email_auth
- **WHEN** `auth_type` is omitted from the invite body
- **THEN** created invite has `auth_type = email_auth`

#### Scenario: Invitation email uses invitor address as reply-to
- **WHEN** invite is created
- **THEN** email dispatched via `deliver_email`; reply-to is set to invitor's email address

#### Scenario: URL constructed from invite_link_host
- **WHEN** `invite_link_host` is provided
- **THEN** invite URL in email personalisation starts with that host; when omitted defaults to `http://localhost:6012/invitation/`

---

### Requirement: List service invitations
`GET /service/{service_id}/invite` SHALL return all invitations for the service.

#### Scenario: Returns all invitations
- **WHEN** service has pending invitations
- **THEN** HTTP 200, `{"data": [...]}`; each entry includes `service`, `from_user`, `auth_type`, `id`

#### Scenario: No invitations returns empty list
- **WHEN** service has no invitations
- **THEN** HTTP 200, `{"data": []}`

---

### Requirement: Update service invitation
`POST /service/{service_id}/invite/{invite_id}` SHALL update the invitation status.

#### Scenario: Valid status update returns 200
- **WHEN** `POST /service/{service_id}/invite/{invite_id}` with `status: "cancelled"`
- **THEN** HTTP 200; invite returned with updated `status`

#### Scenario: Invite from different service returns 404
- **WHEN** `invite_id` belongs to a different service
- **THEN** HTTP 404

#### Scenario: Invalid status value returns 400
- **WHEN** `status` is not a valid invite status
- **THEN** HTTP 400

---

### Requirement: Decode invite token
`GET /invite/{invitation_type}/{token}` SHALL decode and return the invite for both service and organisation types.

#### Scenario: Valid service token returns invite detail
- **WHEN** `GET /invite/service/{valid_token}`
- **THEN** HTTP 200; response includes `id`, `email_address`, `from_user`, `service`, `status`, `permissions`, `folder_permissions`

#### Scenario: Valid org token returns org invite
- **WHEN** `GET /invite/organisation/{valid_token}`
- **THEN** HTTP 200; response includes serialised org invite object

#### Scenario: Expired token returns 400
- **WHEN** token TTL has elapsed
- **THEN** HTTP 400, body `{"invitation": "invitation expired"}`

#### Scenario: Malformed token returns 400
- **WHEN** token is truncated or has invalid signature
- **THEN** HTTP 400, body `{"invitation": "bad invitation link"}`

#### Scenario: Valid token but invited user not found returns 404
- **WHEN** token is cryptographically valid but the referenced invited_user_id has no DB row
- **THEN** HTTP 404, body contains `"No result found"`

---

### Requirement: Verify code DAO — one-time use with bcrypt
Verify code rows are hashed before storage and must be marked used immediately on first successful verification.

#### Scenario: Used verify code is not accepted again
- **WHEN** the same 5-digit code is submitted for verification a second time after a successful first use
- **THEN** HTTP 400; `code_used` is `true`; `failed_login_count` not changed

#### Scenario: Successive create_secret_code calls produce different values
- **WHEN** `create_secret_code` is called twice in succession
- **THEN** the two codes differ (not identical)

#### Scenario: Codes at 24h+ are cleaned up; codes before 24h are kept
- **WHEN** `delete_codes_older_created_more_than_a_day_ago` runs
- **THEN** codes with `created_at < now - 24h` are deleted; codes at 23h 59m 59s are kept

#### Scenario: count_user_verify_codes excludes expired and used codes
- **WHEN** counting active codes
- **THEN** rows with `code_used=true` are excluded; rows with `expiry_datetime` in the past are excluded

---

### Requirement: FIDO2 session DAO — consume-once
`get_fido2_session` must delete the row after retrieval; a second call must fail or return empty.

#### Scenario: Session deleted after retrieval
- **WHEN** `GetAndDeleteFido2Session` is called for a user with an active session
- **THEN** the session state is returned; `Fido2Session.query.count() == 0`

#### Scenario: Second create replaces previous session
- **WHEN** `UpsertFido2Session` is called twice for the same user
- **THEN** only one session row exists; second call overwrites the first

---

### Requirement: user_can_be_archived logic
`dao_archive_user` must guard against leaving any active service without a manage_settings holder.

#### Scenario: User with no active services can be archived
- **WHEN** user has no services or all services are inactive
- **THEN** `user_can_be_archived` returns true; archive succeeds

#### Scenario: Other active member with manage_settings allows archive
- **WHEN** another active user holds `manage_settings` on every service the user belongs to
- **THEN** `user_can_be_archived` returns true; archive succeeds

#### Scenario: Service with no other manage_settings holder blocks archive
- **WHEN** the user is the only active `manage_settings` holder on at least one service
- **THEN** `user_can_be_archived` returns false; `POST /user/{id}/archive` returns HTTP 400

#### Scenario: Pending/inactive other members do not count as manage_settings holders
- **WHEN** other service members are all pending or inactive
- **THEN** `user_can_be_archived` returns false

---

### Requirement: GetUserByID nil-safety
The Go repository MUST reject a nil or zero-value UUID before querying to prevent returning all users.

#### Scenario: Zero UUID input returns not-found error
- **WHEN** the repository function receives a nil/zero-value UUID
- **THEN** returns not-found error; no DB SELECT ALL executed

#### Scenario: Valid UUID returns the correct user
- **WHEN** the repository receives a well-formed UUID
- **THEN** returns the single matching user row

---

### Requirement: blocked state forces session invalidation
`save_user_attribute` with `blocked=True` MUST set `current_session_id` to the nil UUID in the same UPDATE.

#### Scenario: Blocking a user invalidates session in same transaction
- **WHEN** `save_user_attribute` called with `{blocked: true}` for a user
- **THEN** `current_session_id` is `"00000000-0000-0000-0000-000000000000"` after the update

#### Scenario: Blocked user password check returns false
- **WHEN** `check_password` is called on a blocked user
- **THEN** returns false regardless of whether the supplied password is correct

---

### Requirement: permission_dao inactive service exclusion
Permissions for inactive services MUST NOT be returned in any permission query.

#### Scenario: Archived service permissions are invisible
- **WHEN** a service is archived (active=false) after permissions were granted
- **THEN** `GetPermissionsByUserID` does not return those permissions

#### Scenario: Default 8 permissions granted on service join
- **WHEN** `add_default_service_permissions_for_user` is called for a new member
- **THEN** exactly 8 permission rows created: manage_users, manage_templates, manage_settings, send_texts, send_emails, send_letters, manage_api_keys, view_activity

---

### Requirement: Invited user status lifecycle
Invitation status follows `pending → accepted | cancelled`; stale invitations cleaned after 48 hours.

#### Scenario: Invitation created with pending status
- **WHEN** `POST /service/{service_id}/invite` succeeds
- **THEN** the `InvitedUser` row has `status = pending`

#### Scenario: Invitation cancelled via update endpoint
- **WHEN** `POST /service/{service_id}/invite/{invite_id}` with `status: "cancelled"`
- **THEN** `status = cancelled` on the record

#### Scenario: Invitations older than 48 hours are deleted by cleanup
- **WHEN** `delete_invitations_created_more_than_two_days_ago` runs
- **THEN** invitations with `created_at <= now - 2 days` are deleted; invitations at 47h 59m 59s are kept

#### Scenario: folder_permissions defaults to empty array
- **WHEN** invite is created without `folder_permissions`
- **THEN** `folder_permissions = []` on the saved record
