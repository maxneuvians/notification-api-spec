## 1. Service Repository Layer (DAO)

- [ ] 1.1 Implement `internal/repository/services/dao.go` — `FetchAllServices`, `GetServicesByPartialName`, `CountLiveServices`, `FetchServiceByID`, `FetchServiceByIDAndUser`, `FetchServicesByUserID`; write unit tests: ordered by `created_at`, partial-name case-insensitive match, filtered active-only
- [ ] 1.2 Implement `FetchServiceByInboundNumber` (two-step: inbound_number → service); write tests: not-found returns nil, inactive inbound number returns nil
- [ ] 1.3 Implement `FetchServiceWithApiKeys` using read-replica; write unit test confirming replica is targeted
- [ ] 1.4 Implement `CreateService`: INSERT service + `services_history` (version=1) + default `ServicePermission` rows + default `ServiceSmsSender` (FROM_NUMBER, is_default=true) within one transaction; write tests: returns error when user nil, confirms history row created, confirms default sender created
- [ ] 1.5 Implement `UpdateService`: UPDATE service + write `services_history` row (version+1) in same transaction; write tests: each call increments version, verify history row reflects post-update state
- [ ] 1.6 Implement `SuspendService` / `SuspendServiceNoTransaction`: set `active=false`, `suspended_at=now()`, `suspended_by_id` (only when user_id non-nil); write `services_history` row; write tests: API keys NOT expired, `suspended_by_id` null when no user_id provided
- [ ] 1.7 Implement `ResumeService`: set `active=true`, `suspended_at=nil`, `suspended_by_id=nil`; write history row; write tests: pre-revoked API keys stay revoked, idempotent on already-active service
- [ ] 1.8 Implement `ArchiveService` / `ArchiveServiceNoTransaction`: `active=false`, prefix `name` and `email_from` with `_archived_<timestamp>_`, expire non-revoked API keys, set `archived=true` on non-archived templates, write history rows for Service, ApiKey, Template; write tests: pre-existing revoked keys unchanged, pre-archived templates unchanged, entire operation rolls back on error
- [ ] 1.9 Implement `FetchActiveUsersForService`, `FetchServiceCreator`, `FetchLiveServicesData`, `FetchSensitiveServiceIDs`, `FetchTodaysStatsForAllServices`; write unit tests for each
- [ ] 1.10 Implement `AddUserToService`, `RemoveUserFromService`: manage `user_to_service`, `permissions`, and `user_folder_permissions`; write tests: non-existent folder UUIDs silently ignored, cross-service folder UUID raises error, other-service permissions preserved on removal
- [ ] 1.11 Implement stats queries: `FetchStatsForService`, `FetchTodaysStatsForService`, `FetchTodaysTotalMessageCount`, `FetchTodaysTotalSmsCount`, `FetchTodaysTotalSmsBillableUnits`, `FetchServiceEmailLimit`, `FetchTodaysTotalEmailCount`; write tests: KEY_TYPE_TEST excluded, scheduled jobs counted in totals

## 2. API Key Management

- [ ] 2.1 Implement `CreateAPIKey`: generate random secret, compute HMAC-SHA256 hash using service API key secret config, store hashed value in `api_keys.secret`, return plaintext in response prefixed with `API_KEY_PREFIX`; write tests: stored secret ≠ plaintext, two keys for same service produce distinct values
- [ ] 2.2 Implement `ListAPIKeys`: return all keys for a service (active and expired); optional `key_id` filter; write tests: expired keys included, other-service keys excluded
- [ ] 2.3 Implement `RevokeAPIKey`: set `expiry_date=now(UTC)` on the key; returns 202; write tests: key still appears in list after revoke, `expiry_date` set to current UTC
- [ ] 2.4 Implement `internal/handler/api_key/` — POST/GET/revoke endpoint handlers; write auth tests: each returns 401 without authorization header
- [ ] 2.5 Write test: subsequent GET /service/{id}/api-keys does NOT expose plaintext secret in any field

## 3. SMS Sender Management

- [ ] 3.1 Implement `InsertServiceSmsSender`: initial default sender creation within `CreateService` transaction; always `is_default=true`; no separate transaction decorator
- [ ] 3.2 Implement `GetSmsSenderByID`, `GetSmsSendersByServiceID`: non-archived filter, `is_default DESC` ordering; write tests: archived senders excluded
- [ ] 3.3 Implement `AddSmsSenderForService`: if `is_default=true` clear old default first; if `is_default=false` and no existing default → return `InvalidRequestError` HTTP 400 (**C7 fix**); `@transactional`; write C7 test: response is `{"result":"error","message":"You must have at least one SMS sender as the default."}` with status 400 (not 500)
- [ ] 3.4 Implement `UpdateServiceSmsSender`: if `is_default=true` clear old default; if `is_default=false` and record is sole default → return `InvalidRequestError` HTTP 400 (**C7 fix**); inbound-number senders: `sms_sender` string cannot be updated; `@transactional`; write tests: switching default clears old, inbound-number update rejected
- [ ] 3.5 Implement `UpdateSmsSenderWithInboundNumber`: bind inbound number to sender; set `InboundNumber.service_id`; `@transactional`
- [ ] 3.6 Implement `ArchiveSmsSender`: set `archived=true`; raise `ArchiveValidationError` if `inbound_number_id` set or `is_default=true`; `@transactional`; write tests: archived sender no longer in list, default sender archive rejected with HTTP 400

## 4. Email Reply-To Management

- [ ] 4.1 Implement `GetReplyTosByServiceID`, `GetReplyToByID`: non-archived filter; ordering `is_default DESC, created_at DESC`; write tests: archived entries excluded
- [ ] 4.2 Implement `AddReplyToEmailAddress`: if `is_default=true` clear old default; if `is_default=false` and no default exists → `InvalidRequest(400) "You must have at least one reply to email address as the default."`; `@transactional`; write test: first reply-to with `is_default=false` returns 400
- [ ] 4.3 Implement `UpdateReplyToEmailAddress`: same default guards as add; `@transactional`; write test: un-defaulting sole reply-to returns 400
- [ ] 4.4 Implement `ArchiveReplyToEmailAddress`: if default AND only non-archived entry → clear `is_default` then archive; if default AND others exist → `ArchiveValidationError`; `@transactional`; write tests: last-one-standing permitted, default-with-others returns 400

## 5. Letter Contact Management

- [ ] 5.1 Implement `GetLetterContactsByServiceID`, `GetLetterContactByID`: non-archived filter; `is_default DESC, created_at DESC` ordering
- [ ] 5.2 Implement `AddLetterContactForService`: if `is_default=true` clear old default; no guard for `is_default=false`; `@transactional`
- [ ] 5.3 Implement `UpdateLetterContact`: if new `is_default=true` clear old default; `@transactional`
- [ ] 5.4 Implement `ArchiveLetterContact`: set `archived=true`; cascade `Template.service_letter_contact_id = NULL` for all templates referencing this contact; no default guard; `@transactional`; write test: template reference cleared to NULL after archive

## 6. Callback and Inbound API Management

- [ ] 6.1 Implement `SaveServiceCallbackApi`: INSERT with `id=uuid()`, `created_at=now()`; store bearer token as itsdangerous-signed value in `_bearer_token`; write `service_callbacks_history` row (version 1); `@transactional @version_class(ServiceCallbackApi)`; write tests: `_bearer_token ≠ plaintext`, `updated_at=null` on create
- [ ] 6.2 Implement `ResetServiceCallbackApi`: UPDATE URL and/or bearer token using `!= nil` checks (NOT truthiness — Go must use pointer params); re-sign bearer token; write history row; `@transactional @version_class(ServiceCallbackApi)`; write test: explicit empty-string URL is stored (not skipped)
- [ ] 6.3 Implement `GetCallbacksByServiceID`, `GetServiceCallbackApi`, `GetDeliveryStatusCallbackForService`, `GetComplaintCallbackForService`, `DeleteServiceCallbackApi`, `SuspendUnsuspendCallbackApi`; write tests for each including history version increment on suspend
- [ ] 6.4 Implement `ResignServiceCallbacks`: dry-run mode (`resign=false`) logs count without writing; `resign=true` re-signs all rows with new key; raises on `BadSignature` unless `unsafe=true`; write tests: resign=false no mutation, resign=true signature updated, unsafe=true processes bad signatures
- [ ] 6.5 Implement `SaveServiceInboundApi`, `ResetServiceInboundApi` (with `!= nil` checks), `GetServiceInboundApi`, `GetServiceInboundApiForService`, `DeleteServiceInboundApi`; same signed-bearer-token handling as callback; write history row on create and update; write tests: delete removes record, update writes new history row

## 7. Data Retention and Safelist

- [ ] 7.1 Implement `FetchServiceDataRetention`, `FetchServiceDataRetentionByID`, `FetchDataRetentionByNotificationType`; write tests: not-found returns nil for by-ID, empty list for list query
- [ ] 7.2 Implement `InsertServiceDataRetention`: `@transactional`; DB unique constraint on `(service_id, notification_type)`; write test: duplicate returns HTTP 400 with `"Service already has data retention for <type> notification type"`
- [ ] 7.3 Implement `UpdateServiceDataRetention`: UPDATE `days_of_retention` and `updated_at`; returns row count; `@transactional`; write tests: unknown retention_id returns 404, mis-matched service_id returns 404
- [ ] 7.4 Implement `FetchServiceSafelist`, `AddSafelistedContacts`, `RemoveServiceSafelist`; wrap DELETE + INSERT in a single transaction (close Python's atomicity gap); write tests: replace-all semantics, transaction rollback on INSERT failure preserves original safelist

## 8. REST Endpoint Handlers — Core Service

- [ ] 8.1 Implement `GET /service` handler: `only_active`, `user_id`, `detailed` (with stats), `start_date`/`end_date`, `include_from_test_key` query params; write tests for each filter and combination
- [ ] 8.2 Implement `GET /service/find-by-name` handler: case-insensitive partial match; returns 400 if `service_name` param absent; write tests
- [ ] 8.3 Implement `POST /service` handler: validate required fields, resolve organisation by email domain (longest match), auto-assign NHS branding when applicable, call Salesforce `engagement_create`; write tests: missing field → 400, duplicate name → 400, duplicate email_from → 400, platform admin → `count_as_live=false`
- [ ] 8.4 Implement `GET /service/{service_id}` and `POST /service/{service_id}` handlers: field validation (research_mode, permissions, prefix_sms, consent_to_research); side-effect handling for go-live, message_limit change, annual-limit Redis cache invalidation, Salesforce updates; write tests for each validation error and each side-effect
- [ ] 8.5 Implement `POST /service/{service_id}/archive`, `POST /service/{service_id}/suspend`, `POST /service/{service_id}/resume` handlers: idempotency on inactive/active service; write tests for each including history row verification and API key expiry behaviour
- [ ] 8.6 Implement `GET /service/live-services`, `GET /service/sensitive-service-ids`, `GET /service/is-name-unique`, `GET /service/is-email-from-unique`, `GET /service/{service_id}/organisation`, `GET /service/{service_id}/history`, `GET /service/{service_id}/statistics`, `GET /service/{service_id}/annual-limit-stats`; write auth tests (401 on missing header) for each
- [ ] 8.7 Implement `GET /service/{service_id}/monthly-usage`: `year` required numeric param; fiscal year range; combine `ft_notification_status` with live data; returns all 12 months even when empty; write tests: missing year → 400, non-numeric year → 400, unknown service → 404

## 9. REST Endpoint Handlers — Service Sub-entities

- [ ] 9.1 Implement `GET /service/{id}/users`, `POST /service/{id}/users/{uid}`, `DELETE /service/{id}/users/{uid}` handlers; write tests: 409 on duplicate add, 400 on last user, 400 on manage_settings guard, 400 on <2-members guard, 404 on user-not-in-service
- [ ] 9.2 Implement `POST /service/{id}/api-key`, `GET /service/{id}/api-keys`, `POST /service/{id}/api-key/{key_id}/revoke` handlers; wire to API key service layer; write auth tests
- [ ] 9.3 Implement SMS sender handlers: `GET /service/{id}/sms-sender`, `GET /service/{id}/sms-sender/{sid}`, `POST /service/{id}/sms-sender` (add), `POST /service/{id}/sms-sender/{sid}` (update), `POST /service.delete_service_sms_sender`; write tests for C7 error format, inbound-number update rejection, archive guards
- [ ] 9.4 Implement email reply-to handlers: `GET /service/{id}/email-reply-to`, `GET /service/{id}/email-reply-to/{rid}`, `POST /service.add_service_reply_to_email_address`, `POST /service.update_service_reply_to_email_address`, `POST /service.delete_service_reply_to_email_address`, `POST /service.verify_reply_to_email_address`; write tests for default guard failures and archival edge cases
- [ ] 9.5 Implement callback API handlers: `POST/GET/DELETE/PUT /service_callback.*`; URL HTTPS validation; bearer token length validation; suspend/unsuspend endpoint; write schema validation tests
- [ ] 9.6 Implement inbound API handlers: `POST/GET/DELETE /service_callback.*_inbound_api`; write tests: nil URL update stores empty string, unknown service → 404
- [ ] 9.7 Implement data retention handlers: `GET /service/{id}/data-retention`, `GET /service/{id}/data-retention/{rid}`, `GET /service/{id}/data-retention/notification-type/{type}`, `POST /service/{id}/data-retention`, `POST /service/{id}/data-retention/{rid}`; note: GET by ID returns `{}` (not 404) on miss; write tests for each
- [ ] 9.8 Implement safelist handlers: `GET /service/{id}/safelist`, `PUT /service/{id}/safelist`; write tests: empty safelist → `{email_addresses:[], phone_numbers:[]}`, invalid entry → 400 without modifying existing safelist

## 10. C7 Fix — Validation and Error Handling

- [ ] 10.1 Define `InvalidRequestError{Message string, StatusCode int}` type (or reuse existing) in the service layer; confirm it serialises as `{"result":"error","message":"<msg>"}` with the given status code; write unit test for serialisation
- [ ] 10.2 Write integration test covering the C7 scenario end-to-end: service with one SMS sender → attempt to set `is_default=false` via `POST /service/{id}/sms-sender/{sid}` → assert HTTP 400, `Content-Type: application/json`, body `{"result":"error","message":"You must have at least one SMS sender as the default."}`
- [ ] 10.3 Write integration test for `AddSmsSenderForService` C7 path: attempt to add a sender with `is_default=false` when service has no existing default → HTTP 400 with same JSON body
- [ ] 10.4 Write test asserting that callback API validation errors (non-HTTPS URL, short bearer token) return HTTP 400 with `{"result":"error","message":"..."}` structured JSON body (not plain-text or 500)
