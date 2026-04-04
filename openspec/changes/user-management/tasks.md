## Tasks

### Group 1: User repository

- [ ] 1.1 Implement `CreateUser` query — INSERT into `users`; wire `User.Password` setter through `pkg/crypto` bcrypt
- [ ] 1.2 Implement `GetUserByID` query — SELECT by UUID; guard against nil/zero UUID and return not-found instead of SELECT ALL
- [ ] 1.3 Implement `GetUserByEmail` — case-insensitive lowercase lookup
- [ ] 1.4 Implement `SearchUsersByEmail` — ILIKE with special-char escaping
- [ ] 1.5 Implement `UpdateUser` / `SaveUserAttribute` — include forced nil-UUID for `current_session_id` when `blocked=true`
- [ ] 1.6 Implement `GetUserWithAccounts` — eager-load services, orgs, orgs.services, services.organisation (no N+1)
- [ ] 1.7 Implement `GetActiveUsersWithServices` — active users × live+non-restricted+non-research services
- [ ] 1.8 Implement `IncrementFailedLoginCount` and `ResetFailedLoginCount` (reset only when > 0)
- [ ] 1.9 Implement `UpdateUserPassword` — set hash, `password_changed_at=now`, `password_expired=false`
- [ ] 1.10 Implement `dao_archive_user` — transactional; run `user_can_be_archived` guard first; rename email to `_archived_{YYYY-MM-DD}_{email}`; null mobile; clear permissions; reset auth_type; invalidate session; invalidate password
- [ ] 1.11 Implement `dao_deactivate_user` — set inactive; keep email; clear permissions; invalidate session; raise if already inactive
- [ ] 1.12 Implement `user_can_be_archived` logic — return false if any active service has no other active manage_settings holder

### Group 2: Password management handlers

- [ ] 2.1 Handler: `POST /user/{id}/update-password` — validate not-banned via `pkg/crypto`; update hash; create `LoginEvent` when `loginData` present; return 200 with `password_changed_at`
- [ ] 2.2 Handler: `POST /user/{id}/verify-password` — bcrypt check; 204 + reset count on success; 400 + increment count on mismatch; 400 on missing field
- [ ] 2.3 Handler: `POST /user/{id}/reset-failed-login-count` — set to 0; return 200; 404 on unknown user
- [ ] 2.4 Handler: `POST /user/{id}/reset-password` — validate email field; 400 on blocked user; dispatch `deliver_email` with reset template; return 204
- [ ] 2.5 Handler: `POST /user/{id}/forced-password-reset` — same as reset-password using `forced_password_reset_email_template`; return 204

### Group 3: Verify code repository

- [ ] 3.1 Implement `create_secret_code` — cryptographically random 5-digit string; preserve leading zeros
- [ ] 3.2 Implement `CreateVerifyCode` — INSERT with hashed code (bcrypt via `VerifyCode.Code` setter); `expiry_datetime = now + 30min`
- [ ] 3.3 Implement `ListVerifyCodesByUser` — ORDER BY `created_at DESC`; return all codes for user
- [ ] 3.4 Implement `MarkVerifyCodeUsed` — UPDATE `code_used=true` by id
- [ ] 3.5 Implement `CountActiveVerifyCodes` — COUNT non-expired AND `code_used IS false`
- [ ] 3.6 Implement `CountRecentVerifyCodes` — COUNT non-expired, unused, `created_at > now - age` (default 30s)
- [ ] 3.7 Implement `DeleteOldVerifyCodes` — DELETE `created_at < now - 24h`
- [ ] 3.8 Implement `DeleteVerifyCodesByUser` — DELETE all codes for user

### Group 4: Verify code handlers

- [ ] 4.1 Handler: `POST /user/{id}/2fa-code` (sms) — check flood guard (30s), check 10-code cap, generate code, INSERT VerifyCode, dispatch `deliver_sms`; support `to` override; return 204
- [ ] 4.2 Handler: `POST /user/{id}/2fa-code` (email) — generate code, INSERT VerifyCode, dispatch `deliver_email` with `EMAIL_2FA_TEMPLATE_ID`; support `to` override, `email_auth_link_host`, `next`; return 204
- [ ] 4.3 Handler: `POST /user/{id}/email-verification` — dispatch verification link email via `deliver_email`; no VerifyCode row; return 204; 404 on unknown user
- [ ] 4.4 Handler: `POST /user/{id}/verify-code` — bcrypt match newest-first; enforce lockout at count >= 10; increment count on not-found and expired; set `logged_in_at`, new session_id, reset count on success; include E2E Cypress bypass for dev environment; return 204/400/404
- [ ] 4.5 Handler: `POST /user/{id}/verify-2fa` — same verification logic as verify-code but no lockout check and no count increment on any failure; include E2E bypass; return 204/400/404

### Group 5: FIDO2 handlers and repository

- [ ] 5.1 Implement `UpsertFido2Session` — DELETE existing session for user then INSERT; one session per user; transactional
- [ ] 5.2 Implement `GetAndDeleteFido2Session` — SELECT then DELETE in same transaction (consume-once)
- [ ] 5.3 Implement `DeleteFido2Session` — DELETE by user_id
- [ ] 5.4 Implement `CreateFido2Key` / `GetFido2Key` / `ListFido2Keys` / `DeleteFido2Key` queries
- [ ] 5.5 Implement `decode_and_register` wrapper — call `FIDO2_SERVER.register_complete`; return base64-pickle of `credential_data`
- [ ] 5.6 Implement `deserialize_fido2_key` — decode stored credential with `_Fido2CredentialUnpickler` for 0.9.x backward compatibility
- [ ] 5.7 Handler: `POST /user/{id}/fido2-keys/register` — upsert Fido2Session; return 200 `{"data": base64-CBOR}` with rp.id and user.id bytes
- [ ] 5.8 Handler: `POST /user/{id}/fido2-keys` — consume session; call `decode_and_register`; save Fido2Key; send account-change notification; return 200 `{"id": key_id}`
- [ ] 5.9 Handler: `POST /user/{id}/fido2-keys/authenticate` — load and deserialize all user keys; upsert session; call `FIDO2_SERVER.authenticate_begin`; return 200 `{"data": base64-CBOR}`
- [ ] 5.10 Handler: `GET /user/{id}/fido2-keys` — call `ListFido2Keys`; return 200 ordered by `created_at ASC`
- [ ] 5.11 Handler: `DELETE /user/{id}/fido2-keys/{key_id}` — call `DeleteFido2Key`; send account-change notification; return 200 with deleted key id; 404 on unknown key

### Group 6: User CRUD handlers

- [ ] 6.1 Handler: `GET /user` — call `ListAllUsers`; return 200 with permissions map per user
- [ ] 6.2 Handler: `GET /user/{id}` — call `GetUserByID`; exclude inactive services from services list and permissions; exclude inactive orgs; return 200
- [ ] 6.3 Handler: `GET /user/email` — validate `email` query param present; call `GetUserByEmail`; return 200 with `password_expired`; 400 on missing param; 404 on no match
- [ ] 6.4 Handler: `POST /user` — validate email, password (not banned), name, mobile, auth_type constraints; call `CreateUser`; return 201
- [ ] 6.5 Handler: `POST /user/{id}` — validate no password field; check sms_auth+mobile constraints; call `UpdateUser`; trigger notifications for name/email/mobile changes (templates c73f1d71, 8a31520f); call salesforce contact_update; return 200
- [ ] 6.6 Handler: `POST /user/{id}/activate` — validate user is pending; call activate + salesforce contact_create; 400 if already active; return 200
- [ ] 6.7 Handler: `POST /user/{id}/deactivate` — run deactivation transaction; apply service suspension matrix; send deactivation email; 500 on error; return 200
- [ ] 6.8 Handler: `POST /user/{id}/archive` — call `dao_archive_user`; map InvalidRequest → 400; return 204
- [ ] 6.9 Handler: `POST /user/find-users-by-email` — validate email is string; call `SearchUsersByEmail`; return 200 `{"data": [...]}`
- [ ] 6.10 Handler: `GET /user/{id}/organisations-and-services` — call `GetUserWithAccounts`; filter to active orgs and active services; return 200

### Group 7: Account communications handlers

- [ ] 7.1 Handler: `POST /user/{id}/email` — validate `email` field present; dispatch already-registered email; return 204
- [ ] 7.2 Handler: `POST /user/{id}/confirm-new-email` — validate `email` field present; dispatch change-email confirmation; return 204
- [ ] 7.3 Handler: `POST /user/{id}/contact-request` — route by support_type and org_type; apply FF_PT_SERVICE_SKIP_FRESHDESK logic (PT → 201 + email_freshdesk_ticket_pt_service; central → 204 + send_ticket); call salesforce engagement_update on go_live_request; return 204 or 201
- [ ] 7.4 Handler: `POST /user/{id}/branding-request` — dispatch Freshdesk branding ticket; return 204
- [ ] 7.5 Handler: `POST /user/{id}/new-template-category-request` — dispatch Freshdesk template-category ticket; return 204

### Group 8: Permission management

- [ ] 8.1 Implement `AddPermission` / `RemoveUserServicePermissions` / `RemoveAllUserPermissions` queries
- [ ] 8.2 Implement `GetPermissionsByUserID` — JOIN `services WHERE active=true`; exclude inactive services
- [ ] 8.3 Implement `GetPermissionsByUserAndService` and `GetTeamMembersWithPermission`
- [ ] 8.4 Implement `add_default_service_permissions_for_user` — INSERT 8 permissions (no intermediate commit)
- [ ] 8.5 Handler: `POST /user/{id}/permissions/{service_id}` — validate user in service; atomically replace all permissions; replace folder_permissions; return 204; 404 if user not in service

### Group 9: Login events

- [ ] 9.1 Implement `SaveLoginEvent` and `ListRecentLoginEvents` (LIMIT 3, ORDER BY created_at DESC)
- [ ] 9.2 Handler: `GET /user/{id}/login-events` — call `ListRecentLoginEvents`; return 200

### Group 10: Service invite handlers and repository

- [ ] 10.1 Implement `SaveInvitedUser` / `GetInvitedUserByServiceAndID` / `GetInvitedUserByID` / `ListInvitedUsersForService` / `DeleteExpiredInvitations`
- [ ] 10.2 Implement `SaveInvitedOrgUser` / `GetInvitedOrgUserByOrgAndID` / `GetInvitedOrgUserByID` / `ListInvitedOrgUsersForOrg` / `DeleteExpiredOrgInvitations`
- [ ] 10.3 Handler: `POST /service/{service_id}/invite` — validate email_address; create InvitedUser with default email_auth; dispatch invite email (reply-to = invitor); return 201 with full invite data
- [ ] 10.4 Handler: `GET /service/{service_id}/invite` — call `ListInvitedUsersForService`; return 200 `{"data": [...]}`
- [ ] 10.5 Handler: `POST /service/{service_id}/invite/{invite_id}` — validate invite belongs to service (404 if not); validate status value (400 if invalid); update and return 200
- [ ] 10.6 Handler: `GET /invite/service/{token}` — verify token signature and TTL; extract invited_user_id; call `GetInvitedUserByID`; return 200 or 400/404 per error case
- [ ] 10.7 Handler: `GET /invite/organisation/{token}` — same token decode logic; call `GetInvitedOrgUserByID`; return 200 or 400/404
