## ADDED Requirements

### Requirement: Service list

`GET /service` SHALL return all services as a JSON array in `{"data": [...]}` ordered by `created_at` ascending.

#### Scenario: List all services
- **WHEN** GET /service is called without filters
- **THEN** HTTP 200 is returned
- **AND** the response is `{"data": [...]}` with every service ordered by `created_at ASC`

#### Scenario: Filter to active services only
- **WHEN** GET /service is called with `?only_active=true`
- **THEN** HTTP 200 is returned
- **AND** services with `active=false` are excluded from the response

#### Scenario: Filter to services for a specific user
- **WHEN** GET /service is called with `?user_id=<uuid>`
- **THEN** HTTP 200 is returned
- **AND** only services that user belongs to are included
- **AND** an empty `{"data": []}` is returned if the user has no services

#### Scenario: Detailed statistics included
- **WHEN** GET /service is called with `?detailed=true`
- **THEN** each service object includes a `statistics` object with `requested`, `delivered`, and `failed` counts for `sms`, `email`, and `letter`
- **AND** KEY_TYPE_TEST notifications are excluded when `include_from_test_key=false`

---

### Requirement: Service fetch by ID

`GET /service/{service_id}` SHALL return the full service object for the given ID.

#### Scenario: Happy path fetch
- **WHEN** GET /service/{service_id} is called with a valid service ID
- **THEN** HTTP 200 is returned
- **AND** the response contains `id`, `name`, `email_from`, `permissions`, `research_mode`, `prefix_sms`, `email_branding`, `go_live_user`, `go_live_at`, `organisation_type`, `count_as_live`
- **AND** the `branding` key is absent (use `email_branding` instead)
- **AND** `prefix_sms` defaults to `true` for newly created services

#### Scenario: Unknown service ID returns 404
- **WHEN** GET /service/{service_id} is called with an ID that does not exist
- **THEN** HTTP 404 is returned with `{"result":"error","message":"No result found"}`

#### Scenario: Scoped fetch with user_id
- **WHEN** GET /service/{service_id}?user_id={uid} is called
- **THEN** HTTP 404 is returned if the user is not a member of the service

---

### Requirement: Service create

`POST /service` SHALL create a new service and return HTTP 201 with the service object.

#### Scenario: Happy path creation
- **WHEN** POST /service is called with valid `name`, `user_id`, `message_limit`, `restricted`, `email_from`, `created_by`
- **THEN** HTTP 201 is returned
- **AND** the service has default permissions `["email", "sms", "international_sms"]`
- **AND** one default `ServiceSmsSender` is created using the platform `FROM_NUMBER`
- **AND** `rate_limit` is 1000 and `letter_branding` is null in the response

#### Scenario: Missing required field returns 400
- **WHEN** POST /service is called without `email_from`
- **THEN** HTTP 400 is returned
- **AND** the error body contains `"Missing data for required field."` for the missing field

#### Scenario: Duplicate service name returns 400
- **WHEN** POST /service is called with a `name` already used by another service
- **THEN** HTTP 400 is returned with message `"Duplicate service name '<name>'"`

#### Scenario: Duplicate email_from returns 400
- **WHEN** POST /service is called with an `email_from` already used by another service
- **THEN** HTTP 400 is returned with message `"Duplicate service name '<email_from>'"`

#### Scenario: Platform-admin service is not counted as live
- **WHEN** POST /service is called and the creating user is a platform admin
- **THEN** the created service has `count_as_live=false`

#### Scenario: NHS branding auto-assigned for matching domain
- **WHEN** POST /service is called by a user whose email domain matches NHS patterns
- **AND** NHS branding exists in the database
- **THEN** the service's `email_branding` and `letter_branding` are set to NHS branding

---

### Requirement: Service update

`POST /service/{service_id}` SHALL update the specified service fields and return HTTP 200.

#### Scenario: Happy path update
- **WHEN** POST /service/{service_id} is called with valid fields
- **THEN** HTTP 200 is returned with the updated service object

#### Scenario: Invalid research_mode returns 400
- **WHEN** POST /service/{service_id} is called with `research_mode` set to a non-boolean value
- **THEN** HTTP 400 is returned with `"Not a valid boolean."`

#### Scenario: Invalid permission returns 400
- **WHEN** POST /service/{service_id} is called with `permissions` containing an unknown permission string
- **THEN** HTTP 400 is returned with `"Invalid Service Permission: '<value>'"`

#### Scenario: Duplicate permission in list returns 400
- **WHEN** POST /service/{service_id} is called with `permissions` containing a duplicate entry
- **THEN** HTTP 400 is returned with `"Duplicate Service Permission: ['<value>']"`

#### Scenario: null prefix_sms returns 400
- **WHEN** POST /service/{service_id} is called with `prefix_sms: null`
- **THEN** HTTP 400 is returned with `{"prefix_sms": ["Field may not be null."]}`

#### Scenario: Permissions update is full-replace
- **WHEN** POST /service/{service_id} is called with `{"permissions": ["sms","email"]}`
- **THEN** all other permissions previously held by the service are removed
- **AND** only `sms` and `email` permissions remain

#### Scenario: Go-live notification sent
- **WHEN** POST /service/{service_id} is called changing `restricted` from `true` to `false`
- **THEN** a notification is sent to service users using `SERVICE_BECAME_LIVE_TEMPLATE_ID`
- **AND** Salesforce `engagement_update` is called with `{StageName: LIVE}`

#### Scenario: Daily limit change notifies users for live services
- **WHEN** POST /service/{service_id} changes `message_limit` on a live (non-restricted) service
- **THEN** a notification is sent using `DAILY_EMAIL_LIMIT_UPDATED_TEMPLATE_ID`
- **AND** Redis daily-limit cache keys for the service are cleared

---

### Requirement: Service archive

`POST /service/{service_id}/archive` SHALL archive the service atomically and return HTTP 204.

#### Scenario: Happy path archive
- **WHEN** POST /service/{service_id}/archive is called on an active service
- **THEN** HTTP 204 is returned
- **AND** `active` is set to `false`
- **AND** `name` is prefixed with `_archived_<YYYY-MM-DD>_<HH:MM:SS>_`
- **AND** `email_from` is prefixed with `_archived_<YYYY-MM-DD>_<HH:MM:SS>_`
- **AND** all API keys without an existing `expiry_date` are expired
- **AND** all non-archived templates are marked `archived=true`
- **AND** a `services_history` row exists with the incremented version

#### Scenario: Archive sends deactivation notification
- **WHEN** a service is archived
- **THEN** a notification is sent to all service users with `SERVICE_DEACTIVATED_TEMPLATE_ID` and `{"service_name": "<original_name>"}`

#### Scenario: Already-inactive service is a no-op
- **WHEN** POST /service/{service_id}/archive is called on a service that is already inactive
- **THEN** HTTP 204 is returned
- **AND** no changes are made (idempotent)

#### Scenario: Pre-revoked API keys are not re-revoked
- **WHEN** a service is archived and it has an API key with an existing `expiry_date`
- **THEN** that key's `expiry_date` and `version` are unchanged

#### Scenario: Archive is atomic — failure rolls back all changes
- **WHEN** the archive operation fails midway (e.g. DB error after name rename but before template archival)
- **THEN** all changes are rolled back and the service remains in its original state

---

### Requirement: Service suspend

`POST /service/{service_id}/suspend` SHALL suspend the service and return HTTP 204.

#### Scenario: Happy path suspend
- **WHEN** POST /service/{service_id}/suspend is called on an active service
- **THEN** HTTP 204 is returned
- **AND** `active` is set to `false`
- **AND** `suspended_at` is set to the current UTC timestamp
- **AND** a `services_history` row exists with `active=false` and the incremented version

#### Scenario: Suspend does NOT expire API keys
- **WHEN** a service is suspended
- **THEN** all API keys retain their existing `expiry_date` (null for active keys)

#### Scenario: Suspended_by_id set when user_id provided
- **WHEN** POST /service/{service_id}/suspend is called with a `user_id` body param
- **THEN** `suspended_by_id` is set to that user's ID
- **AND** when called without `user_id`, `suspended_by_id` remains null

#### Scenario: Already-inactive service suspend is a no-op
- **WHEN** POST /service/{service_id}/suspend is called on a service with `active=false`
- **THEN** HTTP 204 is returned without calling the DAO

---

### Requirement: Service resume

`POST /service/{service_id}/resume` SHALL re-activate a suspended service and return HTTP 204.

#### Scenario: Happy path resume
- **WHEN** POST /service/{service_id}/resume is called on a suspended service
- **THEN** HTTP 204 is returned
- **AND** `active` is set to `true`
- **AND** `suspended_at` and `suspended_by_id` are cleared to null
- **AND** a `services_history` row exists with the incremented version

#### Scenario: Resume does not restore revoked API keys
- **WHEN** a service is resumed that had API keys revoked prior to suspension
- **THEN** those API keys remain expired (`expiry_date` is unchanged)

#### Scenario: Already-active service resume is a no-op
- **WHEN** POST /service/{service_id}/resume is called on a service with `active=true`
- **THEN** HTTP 204 is returned without calling the DAO

---

### Requirement: Service user list

`GET /service/{service_id}/users` SHALL return the list of users in the service.

#### Scenario: Happy path
- **WHEN** GET /service/{service_id}/users is called
- **THEN** HTTP 200 is returned with `{"data": [...]}` where each entry has `name`, `email`, and `mobile`

#### Scenario: Service with no users returns empty list
- **WHEN** GET /service/{service_id}/users is called for a service that has no members
- **THEN** HTTP 200 is returned with `{"data": []}`

---

### Requirement: Add user to service

`POST /service/{service_id}/users/{user_id}` SHALL add a user to a service with the specified permissions.

#### Scenario: Happy path
- **WHEN** POST /service/{service_id}/users/{user_id} is called with a valid `permissions` list
- **THEN** HTTP 201 is returned with the updated service data

#### Scenario: User already in service returns 409
- **WHEN** POST /service/{service_id}/users/{user_id} is called and the user is already a member
- **THEN** HTTP 409 is returned with `"User id: <id> already part of service id: <id>"`

#### Scenario: Folder permissions for non-existent folders are silently ignored
- **WHEN** POST /service/{service_id}/users/{user_id} is called with `folder_permissions` containing a UUID that does not exist
- **THEN** HTTP 201 is returned and the non-existent folder UUID is silently ignored

---

### Requirement: Remove user from service

`DELETE /service/{service_id}/users/{user_id}` SHALL remove a user and their permissions from a service.

#### Scenario: Happy path removal
- **WHEN** DELETE /service/{service_id}/users/{user_id} is called for a valid user in the service
- **THEN** HTTP 204 is returned
- **AND** the user's `Permission` records and `user_folder_permissions` for this service are deleted

#### Scenario: Cannot remove last remaining user
- **WHEN** DELETE /service/{service_id}/users/{user_id} is called and that user is the only member
- **THEN** HTTP 400 is returned with `"You cannot remove the only user for a service"`

#### Scenario: Cannot remove last user with manage_settings permission
- **WHEN** DELETE /service/{service_id}/users/{user_id} is called and the user is the last member holding `manage_settings`
- **THEN** HTTP 400 is returned with `"SERVICE_NEEDS_USER_W_MANAGE_SETTINGS_PERM"`

#### Scenario: Removal below 2 members is rejected
- **WHEN** DELETE /service/{service_id}/users/{user_id} would leave fewer than 2 members
- **THEN** HTTP 400 is returned with `"SERVICE_CANNOT_HAVE_LT_2_MEMBERS"`

---

### Requirement: API key create

`POST /service/{service_id}/api-key` SHALL create an API key, hash the secret, and return the plaintext once.

#### Scenario: Happy path creation returns plaintext key
- **WHEN** POST /service/{service_id}/api-key is called with valid `name`, `created_by`, `key_type`
- **THEN** HTTP 201 is returned
- **AND** the response contains `{"data": {"key": "<prefixed_plaintext_secret>", "key_name": "<prefixed_name>"}}`
- **AND** the `key` value is prefixed with the `API_KEY_PREFIX` config value

#### Scenario: Stored secret is never the plaintext
- **WHEN** an API key is created
- **THEN** the value in `api_keys.secret` is NOT equal to the plaintext in the create response
- **AND** a subsequent GET /service/{service_id}/api-keys does not expose the plaintext secret

#### Scenario: Missing key_type returns 400
- **WHEN** POST /service/{service_id}/api-key is called without `key_type`
- **THEN** HTTP 400 is returned with `{"key_type": ["Missing data for required field."]}`

#### Scenario: Multiple keys per service are allowed
- **WHEN** POST /service/{service_id}/api-key is called twice for the same service
- **THEN** two separate API keys are created with distinct `key` values

---

### Requirement: API key list

`GET /service/{service_id}/api-keys` SHALL return all API keys for the service, including expired ones.

#### Scenario: Returns active and expired keys
- **WHEN** GET /service/{service_id}/api-keys is called
- **THEN** HTTP 200 is returned with `{"apiKeys": [...]}`
- **AND** both active keys and keys with an `expiry_date` are included
- **AND** keys belonging to other services are excluded

#### Scenario: Single key fetch by key_id
- **WHEN** GET /service/{service_id}/api-keys?key_id={key_uuid} is called
- **THEN** HTTP 200 is returned with the matching key only

---

### Requirement: API key revoke

`POST /service/{service_id}/api-key/{api_key_id}/revoke` SHALL set the `expiry_date` on the API key.

#### Scenario: Happy path revoke
- **WHEN** POST /service/{service_id}/api-key/{key_id}/revoke is called
- **THEN** HTTP 202 is returned
- **AND** the API key's `expiry_date` is set to the current UTC timestamp

#### Scenario: Revoked key still appears in list
- **WHEN** GET /service/{service_id}/api-keys is called after revoking a key
- **THEN** the revoked key is still present in `{"apiKeys": [...]}` with its `expiry_date` set

---

### Requirement: SMS sender list

`GET /service/{service_id}/sms-sender` SHALL return all non-archived SMS senders for the service.

#### Scenario: Happy path list
- **WHEN** GET /service/{service_id}/sms-sender is called
- **THEN** HTTP 200 is returned with a list where each entry has `id`, `sms_sender`, `is_default`, `inbound_number_id`
- **AND** the default sender appears first (ordered by `is_default DESC`)

#### Scenario: Unknown service returns empty list (not 404)
- **WHEN** GET /service/{service_id}/sms-sender is called for an unknown service
- **THEN** HTTP 200 is returned with an empty list `[]`

---

### Requirement: SMS sender fetch by ID

`GET /service/{service_id}/sms-sender/{sms_sender_id}` SHALL return the specified non-archived SMS sender.

#### Scenario: Happy path fetch
- **WHEN** GET /service/{service_id}/sms-sender/{sms_sender_id} is called with valid IDs
- **THEN** HTTP 200 is returned with the serialized `ServiceSmsSender`

#### Scenario: Unknown sender or service returns 404
- **WHEN** GET /service/{service_id}/sms-sender/{sms_sender_id} is called with an unknown sender or service
- **THEN** HTTP 404 is returned

---

### Requirement: SMS sender create

`POST /service/{service_id}/sms-sender` SHALL add a new SMS sender to the service.

#### Scenario: Happy path create
- **WHEN** POST /service/{service_id}/sms-sender is called with a valid sender string
- **THEN** HTTP 201 is returned with the new sender

#### Scenario: New default sender demotes existing default
- **WHEN** POST /service/{service_id}/sms-sender is called with `is_default=true`
- **THEN** the previously-default sender has `is_default` set to `false`
- **AND** the new sender has `is_default=true`

#### Scenario: Inbound number ID associates the inbound number with service
- **WHEN** POST /service/{service_id}/sms-sender is called with an `inbound_number_id`
- **AND** the service has exactly one non-archived sender
- **THEN** the existing sender is replaced (updated with the inbound number binding)
- **AND** `InboundNumber.service_id` is set to the service's ID

---

### Requirement: SMS sender update

`POST /service/{service_id}/sms-sender/{sms_sender_id}` SHALL update an SMS sender's details.

#### Scenario: Happy path update
- **WHEN** POST /service/{service_id}/sms-sender/{sms_sender_id} is called with a new `sms_sender` string
- **THEN** HTTP 200 is returned with the updated sender

#### Scenario: Cannot update sms_sender string for inbound number senders
- **WHEN** POST /service/{service_id}/sms-sender/{sms_sender_id} is called to change `sms_sender` on a sender that has `inbound_number_id` set
- **THEN** HTTP 400 is returned

#### Scenario: Switching default
- **WHEN** POST /service/{service_id}/sms-sender/{sms_sender_id} is called with `is_default=true`
- **THEN** the previous default sender has `is_default` cleared to `false`

---

### Requirement: SMS sender archive

`POST /service.delete_service_sms_sender` SHALL soft-delete an SMS sender.

#### Scenario: Happy path archive
- **WHEN** the archive endpoint is called for a non-default, non-inbound sender
- **THEN** HTTP 200 is returned
- **AND** the sender has `archived=true`
- **AND** the sender no longer appears in `GET /service/{service_id}/sms-sender`

#### Scenario: Cannot archive inbound number sender
- **WHEN** the archive endpoint is called for a sender with `inbound_number_id` set
- **THEN** HTTP 400 is returned with `{"message":"You cannot delete an inbound number","result":"error"}`

#### Scenario: Cannot archive default sender
- **WHEN** the archive endpoint is called for a sender with `is_default=true`
- **THEN** HTTP 400 is returned

---

### Requirement: SMS sender default guard (C7 fix)

The server SHALL return HTTP 400 with a structured JSON error body when an operation would leave the service with no default SMS sender. The Python code raises `Exception("...", 400)` — a tuple exception — which produces garbled 500 output; Go must use `InvalidRequestError` instead.

#### Scenario: Add sender with is_default=false when no default exists returns 400
- **WHEN** `dao_add_sms_sender_for_service` is called with `is_default=false` and the service has no existing default sender
- **THEN** HTTP 400 is returned with `{"result":"error","message":"You must have at least one SMS sender as the default."}`
- **AND** the response is NOT a 500 with a stringified Python exception tuple

#### Scenario: Update sender to is_default=false when it is the sole default returns 400
- **WHEN** `dao_update_service_sms_sender` is called with `is_default=false` on the service's only default sender
- **THEN** HTTP 400 is returned with `{"result":"error","message":"You must have at least one SMS sender as the default."}`

#### Scenario: Correctly-structured error body
- **WHEN** the SMS sender default guard triggers
- **THEN** the response body is `{"result":"error","message":"..."}` with HTTP status 400
- **AND** the response Content-Type is `application/json`

---

### Requirement: Email reply-to list

`GET /service/{service_id}/email-reply-to` SHALL return all non-archived reply-to addresses for the service.

#### Scenario: Happy path list
- **WHEN** GET /service/{service_id}/email-reply-to is called
- **THEN** HTTP 200 is returned with a list where each entry has `id`, `service_id`, `email_address`, `is_default`, `created_at`, `updated_at`
- **AND** `updated_at` is null for addresses that have never been updated

#### Scenario: Empty list when no reply-tos exist
- **WHEN** GET /service/{service_id}/email-reply-to is called for a service with no reply-to addresses
- **THEN** HTTP 200 is returned with an empty list `[]`

---

### Requirement: Email reply-to create

`POST /service.add_service_reply_to_email_address` SHALL add a new reply-to address, enforcing the default invariant.

#### Scenario: Happy path create (first address must be default)
- **WHEN** the first reply-to is added with `is_default=true`
- **THEN** HTTP 201 is returned with the new reply-to address

#### Scenario: First reply-to with is_default=false returns 400
- **WHEN** the first reply-to is added with `is_default=false`
- **THEN** HTTP 400 is returned with `"You must have at least one reply to email address as the default."`

#### Scenario: Second address with is_default=true demotes existing default
- **WHEN** a second reply-to is added with `is_default=true`
- **THEN** the existing default reply-to has `is_default` set to `false`

---

### Requirement: Email reply-to update

`POST /service.update_service_reply_to_email_address` SHALL update a reply-to address.

#### Scenario: Happy path update
- **WHEN** the update endpoint is called with a new `email_address`
- **THEN** HTTP 200 is returned with the updated reply-to address

#### Scenario: Cannot un-default the only reply-to
- **WHEN** the update is called setting `is_default=false` on the service's only reply-to address
- **THEN** HTTP 400 is returned with `"You must have at least one reply to email address as the default."`

#### Scenario: Setting is_default=true switches the existing default
- **WHEN** the update sets `is_default=true` on a non-default reply-to
- **THEN** the previously-default reply-to has `is_default` cleared

---

### Requirement: Email reply-to archive

`POST /service.delete_service_reply_to_email_address` SHALL archive a reply-to address, enforcing the last-default rule.

#### Scenario: Archive non-default reply-to
- **WHEN** a non-default reply-to is archived
- **THEN** HTTP 200 is returned and `archived=true` is set

#### Scenario: Cannot archive default while others exist
- **WHEN** the archive is called on the default reply-to and other non-archived addresses exist
- **THEN** HTTP 400 is returned with `"You cannot delete a default email reply to address if other reply to addresses exist"`

#### Scenario: Can archive the sole remaining reply-to even if it is the default
- **WHEN** the archive is called on the last non-archived reply-to address (which is also the default)
- **THEN** HTTP 200 is returned
- **AND** `is_default` is cleared before archiving

---

### Requirement: Callback API create (delivery-status and complaint types)

`POST /service_callback.create_service_callback_api` SHALL create a callback webhook for a service.

#### Scenario: Happy path create
- **WHEN** POST /service_callback.create_service_callback_api is called with valid `url`, `bearer_token`, `updated_by_id`
- **THEN** HTTP 201 is returned with `{"data": {id, service_id, url, updated_by_id, created_at, updated_at: null}}`
- **AND** `_bearer_token` in the database stores the signed (not plaintext) bearer token

#### Scenario: HTTPS URL is required
- **WHEN** the create is called with an HTTP (non-HTTPS) URL
- **THEN** HTTP 400 is returned with `"url is not a valid https url"`

#### Scenario: Bearer token minimum length enforced
- **WHEN** the create is called with a `bearer_token` shorter than 10 characters
- **THEN** HTTP 400 is returned with `"bearer_token <value> is too short"`

#### Scenario: Unknown service returns 404
- **WHEN** the create is called for a service that does not exist
- **THEN** HTTP 404 is returned with `{"message":"No result found"}`

---

### Requirement: Callback API update

`POST /service_callback.update_service_callback_api` SHALL update the URL or bearer token of an existing callback.

#### Scenario: Happy path update
- **WHEN** the update endpoint is called with a new valid HTTPS URL
- **THEN** HTTP 200 is returned with the updated callback data
- **AND** a new `service_callbacks_history` row is written with the incremented version

#### Scenario: Update stores signed bearer token
- **WHEN** the update is called with a new bearer token
- **THEN** the `_bearer_token` column is updated with the newly signed value
- **AND** HTTP 200 is returned

---

### Requirement: Callback API fetch and delete

`GET /service_callback.fetch_service_callback_api` SHALL return the callback configuration. `DELETE /service_callback.remove_service_callback_api` SHALL hard-delete it.

#### Scenario: Fetch callback
- **WHEN** the fetch endpoint is called for an existing callback
- **THEN** HTTP 200 is returned with `{"data": {...}}`

#### Scenario: Delete callback
- **WHEN** the delete endpoint is called for an existing callback
- **THEN** HTTP 204 is returned
- **AND** the callback record no longer exists in the database

---

### Requirement: Callback API suspend/unsuspend

`POST /service_callback.suspend_callback_api` SHALL toggle the `is_suspended` flag on a callback.

#### Scenario: Suspend a callback
- **WHEN** the suspend endpoint is called with `suspend_unsuspend=true`
- **THEN** HTTP 200 is returned
- **AND** `is_suspended=true` and `suspended_at` is set on the callback
- **AND** a new `service_callbacks_history` row is written

#### Scenario: Unsuspend a callback
- **WHEN** the suspend endpoint is called with `suspend_unsuspend=false`
- **THEN** HTTP 200 is returned
- **AND** `is_suspended=false` on the callback

---

### Requirement: Inbound SMS API configuration

`POST /service_callback.create_service_inbound_api` SHALL create an inbound SMS webhook for a service. `POST /service_callback.update_service_inbound_api` SHALL update it. `DELETE /service_callback.remove_service_inbound_api` SHALL hard-delete it.

#### Scenario: Create inbound API
- **WHEN** create is called with valid `url`, `bearer_token`, `updated_by_id`
- **THEN** HTTP 201 is returned with `{"data": {id, service_id, url, updated_by_id, created_at, updated_at: null}}`
- **AND** a `service_inbound_api_history` row is written at version 1

#### Scenario: Update uses strict nil check (truthiness bug fix)
- **WHEN** the update endpoint is called with `url` explicitly set to an empty string
- **THEN** the `url` field is updated to empty (not silently skipped)
- **AND** HTTP 200 is returned

#### Scenario: Delete inbound API
- **WHEN** the delete endpoint is called for an existing inbound API record
- **THEN** HTTP 204 is returned and the record is permanently deleted

---

### Requirement: Data retention create

`POST /service/{service_id}/data-retention` SHALL create a notification type retention policy for the service.

#### Scenario: Happy path create
- **WHEN** POST /service/{service_id}/data-retention is called with a valid `notification_type` and `days_of_retention`
- **THEN** HTTP 201 is returned with `{"result": {...}}`

#### Scenario: Invalid notification_type returns 400
- **WHEN** POST /service/{service_id}/data-retention is called with `notification_type` not in `[sms, email, letter]`
- **THEN** HTTP 400 is returned with `"notification_type <value> is not one of [sms, letter, email]"`

#### Scenario: Duplicate type for same service returns 400
- **WHEN** POST /service/{service_id}/data-retention is called for a `(service_id, notification_type)` pair that already has a policy
- **THEN** HTTP 400 is returned with `"Service already has data retention for <type> notification type"`

---

### Requirement: Data retention list and update

`GET /service/{service_id}/data-retention` SHALL list policies. `POST /service/{service_id}/data-retention/{retention_id}` SHALL update `days_of_retention`.

#### Scenario: List returns all policies
- **WHEN** GET /service/{service_id}/data-retention is called
- **THEN** HTTP 200 is returned with a list of all data retention policies for the service ordered by `notification_type`
- **AND** an empty list is returned if no policies exist

#### Scenario: Update days_of_retention
- **WHEN** POST /service/{service_id}/data-retention/{retention_id} is called with a valid `days_of_retention`
- **THEN** HTTP 204 is returned
- **AND** `updated_at` is set on the record

#### Scenario: Get by ID — not-found returns 200 empty object
- **WHEN** GET /service/{service_id}/data-retention/{retention_id} is called for a non-existent ID
- **THEN** HTTP 200 is returned with `{}` (not a 404)

---

### Requirement: Safelist fetch

`GET /service/{service_id}/safelist` SHALL return the current safelist of email addresses and phone numbers.

#### Scenario: Returns populated safelist
- **WHEN** GET /service/{service_id}/safelist is called for a service with safelisted contacts
- **THEN** HTTP 200 is returned with `{"email_addresses": [...], "phone_numbers": [...]}`

#### Scenario: Empty safelist returns 200 with empty arrays
- **WHEN** GET /service/{service_id}/safelist is called for a service with no safelisted contacts
- **THEN** HTTP 200 is returned with `{"email_addresses": [], "phone_numbers": []}`

---

### Requirement: Safelist update

`PUT /service/{service_id}/safelist` SHALL atomically replace the entire safelist.

#### Scenario: Happy path replace
- **WHEN** PUT /service/{service_id}/safelist is called with a valid list of emails and phone numbers
- **THEN** HTTP 204 is returned
- **AND** the previous safelist entries are deleted and the new entries are saved

#### Scenario: Invalid entry returns 400 and preserves existing safelist
- **WHEN** PUT /service/{service_id}/safelist is called with an entry that is neither a valid email nor phone number
- **THEN** HTTP 400 is returned with `"Invalid safelist: \"<value>\" is not a valid email address or phone number"`
- **AND** the existing safelist is unchanged (no partial writes)

#### Scenario: Empty string entry returns 400
- **WHEN** PUT /service/{service_id}/safelist is called with an empty string `""` as an entry
- **THEN** HTTP 400 is returned with the appropriate invalid safelist message

---

### Requirement: Service statistics

`GET /service/{service_id}/statistics` SHALL return notification counts grouped by type and status.

#### Scenario: Today-only statistics
- **WHEN** GET /service/{service_id}/statistics?today_only=true is called
- **THEN** HTTP 200 is returned with `{"data": {sms: {requested, delivered, failed}, email: {...}, letter: {...}}}`
- **AND** only today's notifications are counted

#### Scenario: Unknown service returns zeroed stats
- **WHEN** GET /service/{service_id}/statistics is called for an unknown service
- **THEN** HTTP 200 is returned with all counts at zero (not a 404)

---

### Requirement: Service history

`GET /service/{service_id}/history` SHALL return version history for the service and its API keys.

#### Scenario: Returns history after mutations
- **WHEN** GET /service/{service_id}/history is called after the service has been updated multiple times
- **THEN** HTTP 200 is returned with `{"data": {"service_history": [...], "api_key_history": [...]}}`
- **AND** `service_history` contains one entry per mutation with incremented versions

#### Scenario: Each mutation creates exactly one history row
- **WHEN** `dao_update_service` is called
- **THEN** one new `services_history` row is created with version incremented by 1

---

### Requirement: Service go-live side-effects

When a service transitions from restricted to live, the system SHALL notify users and update Salesforce.

#### Scenario: Go-live triggers user notification
- **WHEN** POST /service/{service_id} sets `restricted` from `true` to `false`
- **THEN** a notification is sent to all service users using `SERVICE_BECAME_LIVE_TEMPLATE_ID`

#### Scenario: Setting restricted back to true does NOT send a notification
- **WHEN** POST /service/{service_id} sets `restricted` from `false` back to `true`
- **THEN** no `SERVICE_BECAME_LIVE_TEMPLATE_ID` notification is sent

---

### Requirement: Service name and email uniqueness checks

`GET /service/is-name-unique` and `GET /service/is-email-from-unique` SHALL return a boolean uniqueness result.

#### Scenario: Name uniqueness check
- **WHEN** GET /service/is-name-unique?name=Foo&service_id={id} is called
- **THEN** HTTP 200 is returned with `{"result": true}` if no other service uses that name
- **AND** `{"result": false}` if another service already uses that name

#### Scenario: Missing required params returns 400
- **WHEN** GET /service/is-name-unique is called without `name` or `service_id`
- **THEN** HTTP 400 is returned

---

### Requirement: Letter contact management

Letter contacts store postal address blocks for outbound letters. `archived=true` contacts must cascade `Template.service_letter_contact_id` to NULL.

#### Scenario: Archive letter contact cascades to templates
- **WHEN** a letter contact is archived
- **THEN** all `Template` records referencing that contact have `service_letter_contact_id` set to `NULL`

#### Scenario: Default letter contact can be archived (no guard unlike reply-to and SMS)
- **WHEN** a letter contact that is the default is archived
- **THEN** HTTP 200 is returned without error (no guard prevents this)

---

### Requirement: Live services and sensitive service IDs

`GET /service/live-services` and `GET /service/sensitive-service-ids` support admin reporting.

#### Scenario: Live services returns correct fields
- **WHEN** GET /service/live-services is called
- **THEN** HTTP 200 is returned with `{"data": [...]}` where each entry has `service_id`, `service_name`, `organisation_name`, `live_date`, `contact_name`, `contact_email`, `sms_totals`, `email_totals`, `letter_totals`, `free_sms_fragment_limit`
- **AND** services with `restricted=true`, `active=false`, or `count_as_live=false` are excluded

#### Scenario: Sensitive service IDs
- **WHEN** GET /service/sensitive-service-ids is called
- **THEN** HTTP 200 is returned with `{"data": ["<uuid>", ...]}` for services where `sensitive_service=true`
- **AND** `{"data": []}` is returned when no sensitive services exist
