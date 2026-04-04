# Behavioral Spec: Services

## Processed Files

- [x] tests/app/service/test_api_key_endpoints.py
- [x] tests/app/service/test_archived_service.py
- [x] tests/app/service/test_callback_rest.py
- [x] tests/app/service/test_rest.py
- [x] tests/app/service/test_schema.py
- [x] tests/app/service/test_send_one_off_notification.py
- [x] tests/app/service/test_sender.py
- [x] tests/app/service/test_service_data_retention_rest.py
- [x] tests/app/service/test_service_whitelist.py
- [x] tests/app/service/test_statistics.py
- [x] tests/app/service/test_statistics_rest.py
- [x] tests/app/service/test_suspend_resume_service.py
- [x] tests/app/service/test_url_for.py
- [x] tests/app/service/test_utils.py
- [x] tests/app/dao/test_services_dao.py
- [x] tests/app/dao/test_service_callback_api_dao.py
- [x] tests/app/dao/test_service_data_retention_dao.py
- [x] tests/app/dao/test_service_email_reply_to_dao.py
- [x] tests/app/dao/test_service_inbound_api_dao.py
- [x] tests/app/dao/test_service_letter_contact_dao.py
- [x] tests/app/dao/test_service_permissions_dao.py
- [x] tests/app/dao/test_service_sms_sender_dao.py
- [x] tests/app/dao/test_service_whitelist_dao.py

---

## Endpoint Contracts

### GET /service

- **Happy path**: Returns `{"data": [...]}` with all services ordered by creation time. Each entry includes `id`, `name`, `permissions`, and other service fields.
- **Validation rules**:
  - `only_active=True` filters out services where `active=False`.
  - `user_id=<uuid>` filters to services the user belongs to; returns empty list if user has no services.
  - `detailed=True` includes per-service `statistics` (requested/delivered/failed counts for sms/email/letter). Default date range is today; supports `start_date` / `end_date` query params.
  - `include_from_test_key=False` excludes test-key notifications from statistics (default includes them).
- **Auth requirements**: Internal auth header required.
- **Notable edge cases**:
  - Default permissions for all newly created services: `["email", "sms", "international_sms"]`.
  - Services with `detailed=True` and `include_from_test_key=False` exclude KEY_TYPE_TEST notifications from counts.
  - `start_date` / `end_date` accepted when `detailed=True`; defaults to today if not provided.
  - `find-by-name` sub-route (`GET /service/find-by-name?service_name=ABC`) performs partial-name search (case-insensitive); returns 400 if `service_name` param is missing.

---

### GET /service/`<service_id>`

- **Happy path**: Returns `{"data": {...}}` with full service object including `id`, `name`, `email_from`, `permissions`, `research_mode`, `prefix_sms` (default `true`), `email_branding` (nullable), `letter_logo_filename` (nullable), `go_live_user`, `go_live_at`, `organisation_type`, `count_as_live`.
- **Validation rules**:
  - Optional `user_id` query param scopes to services accessible by that user.
  - Optional `detailed=True` adds `statistics`.
  - Optional `today_only=True` / `today_only=False` controls the statistics window.
- **Error cases**:
  - Unknown `service_id` → 404 `{"result":"error","message":"No result found"}`.
  - Unknown `service_id` with `user_id` → 404.
- **Auth requirements**: Internal auth header required.
- **Notable edge cases**:
  - `organisation_type` is always present (may be `null`).
  - `email_branding` field present; `branding` key NOT present.
  - `prefix_sms` defaults to `true` for new services.

---

### POST /service

- **Happy path**: Creates service, returns 201 with `{"data": {...}}`. Response includes `id`, `name`, `email_from`, `rate_limit` (1000), `letter_branding` (null), `count_as_live`, permissions, default SMS sender (set to `FROM_NUMBER` config).
- **Validation rules**:
  - Required fields: `name`, `user_id`, `message_limit`, `restricted`, `email_from`, `created_by`. Missing any → 400 with per-field error `"Missing data for required field."`.
  - `user_id` must exist in DB → 404 if not found.
  - Duplicate `name` → 400 `"Duplicate service name '<name>'"`.
  - Duplicate `email_from` → 400 `"Duplicate service name '<email_from>'"`.
- **Auth requirements**: Internal auth header required.
- **Notable edge cases**:
  - `count_as_live` is `False` if the creating user is a platform admin; `True` otherwise.
  - Organisation auto-assigned by matching the user's email domain against organisation domains. Longest matching domain wins; no match → `organisation: null`.
  - New service does NOT inherit email/letter branding from its organisation.
  - Calls Salesforce `engagement_create` on creation.
  - Default service permissions created: `email`, `sms`, `international_sms`.
  - One default `ServiceSmsSender` record created using `FROM_NUMBER` config value.
  - If user email matches NHS domains/org types and NHS branding exists in DB, email and letter branding are auto-set to NHS.

---

### POST /service/`<service_id>`

- **Happy path**: Updates service fields; returns 200 with `{"data": {...}}`.
- **Validation rules**:
  - `research_mode` must be boolean; non-boolean value (e.g., `"dedede"`) → 400 `"Not a valid boolean."`.
  - `organisation_type` must be a valid enum value; invalid value → 500.
  - `permissions` list: each entry must be a known permission string. Invalid → 400 `"Invalid Service Permission: '<value>'"`. Duplicates → 400 `"Duplicate Service Permission: ['<value>']"`.
  - `volume_email`, `volume_sms`, `volume_letter`: integer or null. Non-integer → 400.
  - `consent_to_research`: accepts `True`, `False`, `"Yes"`, `"No"`. Invalid strings → 400.
  - `prefix_sms`: boolean; null → 400 `{"prefix_sms": ["Field may not be null."]}`.
  - Duplicate `name` or `email_from` for a different service → 400.
  - Non-existent `service_id` → 404.
- **Error cases**:
  - `{"email_branding": null}` is valid and removes branding.
- **Auth requirements**: Internal auth header required.
- **Notable edge cases**:
  - When `restricted` changes from `True` → `False` (service goes live):
    - Sends notification to service users via `send_notification_to_service_users` with template `SERVICE_BECAME_LIVE_TEMPLATE_ID`.
    - Calls Salesforce `engagement_update` with `{"StageName": ENGAGEMENT_STAGE_LIVE}`. Uses `go_live_user` if set, otherwise falls back to service creator.
  - When `restricted` is set back to `True` on a live service: no notification is sent.
  - When `message_limit` changes for a live (non-restricted) service: sends notification to users with `DAILY_EMAIL_LIMIT_UPDATED_TEMPLATE_ID`; clears Redis daily-limit cache keys. No notification sent for trial/restricted services or when limit unchanged.
  - When `sms_annual_limit` changes: deletes `near_sms_limit` and `near_email_limit` from annual-limit Redis hash.
  - When `email_annual_limit` changes: deletes `over_email_limit` and `near_email_limit` from annual-limit Redis hash.
  - When `name` changes: calls Salesforce `engagement_update` with `{"Name": "<new_name>"}`.
  - Permissions update is a full replace: sending `{"permissions": ["sms","email"]}` removes all other permissions.

---

### GET /service/`<service_id>`/users

- **Happy path**: Returns `{"data": [...]}` with user details (name, email, mobile).
- **Error cases**:
  - Unknown service → 404.
  - Service with no users → 200 `{"data": []}`.
- **Auth requirements**: Internal auth header required.

---

### POST /service/`<service_id>`/users/`<user_id>`

- **Happy path**: Adds user to service with specified permissions; returns 201 with updated service data.
- **Validation rules**:
  - `permissions`: list of `{"permission": "<name>"}` objects. Missing → 400.
  - `folder_permissions`: optional list of folder UUID strings.
- **Error cases**:
  - Service not found → 404 `"No result found"`.
  - User not found → 404 `"No result found"`.
  - User already in service → 409 `"User id: <id> already part of service id: <id>"`.
- **Notable edge cases**:
  - Folder permissions for non-existent folders are silently ignored.
  - Folder permissions from a different service raise an IntegrityError.
  - Adding user to a second service preserves all existing permissions on other services.

---

### DELETE /service/`<service_id>`/users/`<user_id>`

- **Happy path**: Removes user, returns 204.
- **Error cases**:
  - Last remaining user → 400 `"You cannot remove the only user for a service"`.
  - Last user with `manage_settings` permission → 400 `"SERVICE_NEEDS_USER_W_MANAGE_SETTINGS_PERM"`.
  - Removal would leave fewer than 2 members → 400 `"SERVICE_CANNOT_HAVE_LT_2_MEMBERS"`.
  - User not in service → 404.
- **Auth requirements**: Internal auth header required.
- **Notable edge cases**:
  - Calls Salesforce `engagement_delete_contact_role(service, user)`.
  - Removing a user deletes their `Permission` records and `user_folder_permissions` for that service only; folder permissions on other services are preserved.

---

### POST /service/`<service_id>`/api-key

- **Happy path**: Creates API key; returns 201 with `{"data": {"key": "<prefixed_key>", "key_name": "<name>"}}`.
- **Validation rules**:
  - Required: `name`, `created_by`, `key_type`. Missing `key_type` → 400 `{"key_type": ["Missing data for required field."]}`.
- **Error cases**:
  - Unknown service → 404.
- **Auth requirements**: Internal auth header required.
- **Notable edge cases**:
  - Multiple API keys per service are allowed.
  - `key_name` in response contains `API_KEY_PREFIX` config value as prefix.
  - Two keys for same service always produce different `key` values.

---

### GET /service/`<service_id>`/api-keys

- **Happy path**: Returns `{"apiKeys": [...]}` including both active and expired keys.
- **Validation rules**:
  - Optional `key_id` query param returns single matching key.
- **Error cases**:
  - Unknown service → 404? (not explicitly tested, inferred).
- **Auth requirements**: Internal auth header required.
- **Notable edge cases**:
  - Keys for other services are excluded.
  - Expired keys are included in the listing.

---

### POST /service/`<service_id>`/api-key/`<api_key_id>`/revoke

- **Happy path**: Sets `expiry_date` on key; returns 202.
- **Auth requirements**: Internal auth header required.

---

### POST /service/`<service_id>`/archive

- **Happy path**: Archives (deactivates) the service; returns 204 with empty body. POST only (GET returns 405).
- **Validation rules**:
  - Unknown service → 404.
- **Notable edge cases**:
  - If service is already inactive, returns 204 but makes no changes (idempotent no-op).
  - On archive:
    - Renames service: `_archived_<YYYY-MM-DD>_<HH:MM:SS>_<original_name>`.
    - Renames `email_from`: `_archived_<YYYY-MM-DD>_<HH:MM:SS>_<original_email_from>`.
    - Sets `active = False`.
    - Revokes all API keys (sets `expiry_date`).
    - Marks all templates as `archived = True`.
    - Creates a new service history record (version incremented).
    - Sends deletion email to all service users via `send_notification_to_service_users` with `SERVICE_DEACTIVATED_TEMPLATE_ID` and `{"service_name": "<name>"}`.
    - Pre-existing revoked API keys (with earlier `expiry_date`) are not re-revoked; `expiry_date` and `version` unchanged.
    - Pre-existing archived templates are not re-archived; `updated_at` and `version` unchanged.
  - Entire operation is transactional: if any part fails, all changes are rolled back (dirty session objects revert).
- **Auth requirements**: Internal auth header required.

---

### POST /service/`<service_id>`/suspend

- **Happy path**: Suspends service (sets `active = False`); returns 204. POST only (GET → 405).
- **Validation rules**:
  - Unknown service → 404.
- **Notable edge cases**:
  - If service is already inactive, returns 204 without calling the DAO (idempotent no-op).
  - Creates service history record (version 2, `active = False`).
  - Does NOT revoke (expire) API keys; `expiry_date` remains null.
  - Sets `suspended_at` timestamp on the service.
  - If no `user_id` body param provided, `suspended_by_id` remains `None`.
- **Auth requirements**: Internal auth header required.

---

### POST /service/`<service_id>`/resume

- **Happy path**: Resumes service (sets `active = True`); returns 204. POST only (GET → 405).
- **Validation rules**:
  - Unknown service → 404.
- **Notable edge cases**:
  - If service is already active, returns 204 without calling the DAO (idempotent no-op).
  - Creates service history record.
  - API keys that were REVOKED before suspension remain revoked; resume does not re-activate them.
- **Auth requirements**: Internal auth header required.

---

### GET /service/`<service_id>`/safelist

- **Happy path**: Returns `{"email_addresses": [...], "phone_numbers": [...]}`.
- **Error cases**:
  - Unknown service → 404 `{"result":"error","message":"No result found"}`.
  - Service with empty safelist → 200 `{"email_addresses": [], "phone_numbers": []}`.
- **Auth requirements**: Internal auth header required.
- **Notable edge cases**:
  - Email entries appear in `email_addresses`, phone entries in `phone_numbers`.
  - Phone numbers returned as stored (not normalized).

---

### PUT /service/`<service_id>`/safelist

- **Happy path**: Replaces entire safelist with provided contacts; returns 204.
- **Validation rules**:
  - Each entry must be a valid email address or phone number. Empty string → 400 `{"result":"error","message":"Invalid safelist: \"\" is not a valid email address or phone number"}`.
- **Error cases**:
  - Validation failure → 400; existing safelist is preserved (no partial writes).
- **Auth requirements**: Internal auth header required.

---

### GET /service/`<service_id>`/data-retention

- **Happy path**: Returns list of `ServiceDataRetention` objects; 200. Empty list if none exist.
- **Auth requirements**: Internal auth header required.

---

### GET /service/`<service_id>`/data-retention/`<retention_id>`

- **Happy path**: Returns serialized `ServiceDataRetention` object; 200.
- **Error cases**:
  - Not found → 200 `{}` (not a 404).
- **Auth requirements**: Internal auth header required.

---

### GET /service/`<service_id>`/data-retention/notification-type/`<type>`

- **Happy path**: Returns serialized `ServiceDataRetention` for the given notification type; 200.
- **Auth requirements**: Internal auth header required.

---

### POST /service/`<service_id>`/data-retention

- **Happy path**: Creates data retention record; returns 201 with `{"result": {...}}`.
- **Validation rules**:
  - `notification_type` must be one of `sms`, `email`, `letter`. Invalid → 400 `"notification_type <value> is not one of [sms, letter, email]"`.
  - Duplicate (service + notification_type) → 400 `{"result":"error","message":"Service already has data retention for <type> notification type"}`.
- **Auth requirements**: Internal auth header required.

---

### POST /service/`<service_id>`/data-retention/`<retention_id>`

- **Happy path**: Updates `days_of_retention`; returns 204 with empty body.
- **Validation rules**:
  - Body must contain `days_of_retention`. Missing/invalid key → 400.
- **Error cases**:
  - Unknown `retention_id` → 404.
- **Auth requirements**: Internal auth header required.

---

### GET /service/`<service_id>`/notifications

- **Happy path**: Returns `{"notifications": [...]}` ordered newest first.
- **Validation rules**:
  - Optional filters: `format_for_csv=True`, `include_from_test_key=<bool>`, `include_jobs=<bool>`, `include_one_off=<bool>`, `count_pages=<bool>`, `page_size=<int>`, `limit_days=<int>`, `to=<recipient>`, `template_type=<sms|email|letter>`, `status=<status>` (repeatable).
  - `to` + `template_type` filter by recipient; letter not allowed for recipient search → 400 `"Only email and SMS can use search by recipient"`.
- **Error cases**:
  - Notification with non-UUID id → 404.
  - Notification belonging to a different service → 404.
- **Notable edge cases**:
  - `format_for_csv=True` changes shape: `recipient` instead of `to`, adds `row_number`, `template_name`, `template_type`, `status` (human-readable, e.g., "In transit").
  - `include_from_test_key=False` omits KEY_TYPE_TEST notifications (default includes them).
  - `include_jobs=False` + `include_one_off=False` returns only API-created notifications.
  - `count_pages=False` returns `"total": null`.
  - Notifications include `template.redact_personalisation` and `template.is_precompiled_letter` fields.
  - Only returns notifications created within the data-retention window (`limit_days` param or service default).
- **Auth requirements**: Internal auth header required.

---

### GET /service/`<service_id>`/notifications/`<notification_id>`

- **Happy path**: Returns notification object with `id`, `created_by` (expanded `{id, name, email_address}`), `template` (at version that was used, not current version), `reference`, `template.redact_personalisation`, `template.is_precompiled_letter`.
- **Error cases**:
  - Not found → 404 `{"message":"Notification not found in database","result":"error"}`.
  - Belongs to different service → 404.
  - Non-UUID id → 404.
- **Auth requirements**: Internal auth header required.

---

### POST /service/`<service_id>`/send-notification  *(one-off / "create_one_off_notification")*

- **Happy path**: Creates and queues notification; returns 201 `{"id": "<notification_id>"}`.
- **Auth requirements**: Internal auth header required.
- *(All send-notification logic detailed in the Business Rules section below.)*

---

### GET /service/`<service_id>`/email-reply-to

- **Happy path**: Returns list (may be empty `[]`); 200.
- Each entry: `id`, `service_id`, `email_address`, `is_default`, `created_at`, `updated_at` (null if never updated).
- **Auth requirements**: Internal auth header required.

---

### GET /service/`<service_id>`/email-reply-to/`<reply_to_id>`

- **Happy path**: Returns serialized `ServiceEmailReplyTo`; 200.
- **Auth requirements**: Internal auth header required.

---

### POST /service.add_service_reply_to_email_address

- **Happy path**: Creates reply-to address; returns 201 `{"data": {...}}`.
- **Validation rules**:
  - First reply-to must have `is_default=True` → 400 `"You must have at least one reply to email address as the default."`.
  - Duplicate email for same service → 400 with split message parts.
- **Error cases**:
  - Unknown service → 404.
- **Notable edge cases**:
  - Adding a second address with `is_default=True` demotes the existing default to `is_default=False`.
  - Multiple active reply-to addresses per service are allowed.
- **Auth requirements**: Internal auth header required.

---

### POST /service.update_service_reply_to_email_address

- **Happy path**: Updates reply-to; returns 200 `{"data": {...}}`.
- **Validation rules**:
  - Setting `is_default=False` when only one address exists → 400 `"You must have at least one reply to email address as the default."`.
- **Error cases**:
  - Unknown service → 404.
- **Auth requirements**: Internal auth header required.

---

### POST /service.delete_service_reply_to_email_address  *(archive)*

- **Happy path**: Marks as `archived=True`; returns 200.
- **Validation rules**:
  - Cannot archive the default reply-to if other (non-archived) addresses exist → 400 `"You cannot delete a default email reply to address if other reply to addresses exist"`.
  - Archiving the only existing address (even if default) is permitted.
- **Auth requirements**: Internal auth header required.

---

### POST /service.verify_reply_to_email_address

- **Happy path**: Sends verification notification; returns 201 `{"data": {"id": "<notification_id>"}}`.
- **Validation rules**:
  - Duplicate email already used by service → 400 with multi-part message.
- **Notable edge cases**:
  - Delivers email via `deliver_email` Celery task on `notify-internal-tasks` queue.
  - `reply_to_text` on the notification is the service's current default reply-to address.
- **Auth requirements**: Internal auth header required.

---

### GET /service/`<service_id>`/sms-sender

- **Happy path**: Returns list; 200. Each entry: `id`, `sms_sender`, `is_default`, `inbound_number_id`.
- **Error cases**:
  - Unknown service → 200 `[]` (empty list; no 404).
- **Auth requirements**: Internal auth header required.

---

### GET /service/`<service_id>`/sms-sender/`<sms_sender_id>`

- **Happy path**: Returns serialized `ServiceSmsSender`; 200.
- **Error cases**:
  - Unknown service → 404.
  - Unknown sms_sender_id → 404.
- **Auth requirements**: Internal auth header required.

---

### POST /service/`<service_id>`/sms-sender  *(add)*

- **Happy path**: Creates new sender; returns 201. Multiple senders per service allowed.
- **Validation rules**:
  - `inbound_number_id` provided: if service already has exactly one non-archived sender, replaces it; if multiple exist, inserts a new sender (so total count grows by one minus any replaced).
  - `is_default=True` demotes existing default sender to `is_default=False`.
- **Error cases**:
  - Unknown service → 404.
- **Notable edge cases**:
  - When `inbound_number_id` is provided, associates the InboundNumber record with the service (`InboundNumber.service_id` set).
  - Archived senders are excluded when counting "existing non-archived senders".
- **Auth requirements**: Internal auth header required.

---

### POST /service/`<service_id>`/sms-sender/`<sms_sender_id>`  *(update)*

- **Happy path**: Updates sender; returns 200.
- **Validation rules**:
  - Cannot update `sms_sender` if it is an inbound number → 400.
  - `is_default=True` switches default away from previous default sender.
- **Error cases**:
  - Unknown service or sender → 404.
- **Auth requirements**: Internal auth header required.

---

### POST /service.delete_service_sms_sender  *(archive)*

- **Happy path**: Sets `archived=True`; returns 200.
- **Validation rules**:
  - Cannot archive an inbound number → 400 `{"message":"You cannot delete an inbound number","result":"error"}`.
- **Auth requirements**: Internal auth header required.

---

### POST /service_callback.create_service_inbound_api

- **Happy path**: Creates record; returns 201 `{"data": {id, service_id, url, updated_by_id, created_at, updated_at: null}}`.
- **Error cases**:
  - Unknown service → 404 `{"message":"No result found"}`.
- **Validation rules**:
  - `url`, `bearer_token`, `updated_by_id` required.
- **Auth requirements**: Internal auth header required.

---

### POST /service_callback.update_service_inbound_api

- **Happy path**: Updates URL or bearer token; returns 200 `{"data": {...}}`.
- **Auth requirements**: Internal auth header required.

---

### GET /service_callback.fetch_service_inbound_api

- **Happy path**: Returns `{"data": {...}}` serialized inbound API; 200.
- **Auth requirements**: Internal auth header required.

---

### DELETE /service_callback.remove_service_inbound_api

- **Happy path**: Deletes record; returns `null` (204 no content). Count drops to 0.
- **Auth requirements**: Internal auth header required.

---

### POST /service_callback.create_service_callback_api

- **Happy path**: Creates delivery-status callback; returns 201 `{"data": {id, service_id, url, updated_by_id, created_at, updated_at: null}}`.
- **Error cases**:
  - Unknown service → 404 `{"message":"No result found"}`.
- **Auth requirements**: Internal auth header required.

---

### POST /service_callback.update_service_callback_api

- **Happy path**: Updates URL or bearer token; returns 200 `{"data": {...}}`.
- **Auth requirements**: Internal auth header required.

---

### GET /service_callback.fetch_service_callback_api

- **Happy path**: Returns `{"data": {...}}`; 200.
- **Auth requirements**: Internal auth header required.

---

### DELETE /service_callback.remove_service_callback_api

- **Happy path**: Deletes record; returns `null` (204). Count drops to 0.
- **Auth requirements**: Internal auth header required.

---

### POST /service_callback.suspend_callback_api

- **Happy path**: Toggles `is_suspended` on the callback; `suspend_unsuspend=true` suspends, `false` unsuspends. Returns 200.
- **Validation rules**:
  - `suspend_unsuspend` (bool) and `updated_by_id` required.
- **Auth requirements**: Internal auth header required.

---

### Callback API Schema Validation (`update_service_callback_api_schema`)

- `url` must be a valid HTTPS URL (not HTTP, not plain text) → error `"url is not a valid https url"`.
- `bearer_token` minimum length 10 characters → error `"bearer_token <value> is too short"`.
- `updated_by_id` required UUID.

---

### GET /service/`<service_id>`/statistics  *(get_service_notification_statistics)*

- **Happy path**: Returns `{"data": {sms: {requested, delivered, failed}, email: {...}, letter: {...}}}`.
- **Validation rules**:
  - `today_only=True` counts only today's notifications; `False` counts last 7 days (combining historic `ft_notification_status` with today's live notifications).
- **Error cases**:
  - Unknown service → 200 with zeroed stats (not 404).
- **Auth requirements**: Internal auth header required.

---

### GET /service/`<service_id>`/monthly-usage  *(get_monthly_notification_stats)*

- **Happy path**: Returns `{"data": {"<YYYY-MM>": {sms: {...}, email: {...}, letter: {...}}, ...}}` covering a full fiscal year (April of `year` to March of `year+1`).
- **Validation rules**:
  - `year` query param required and must be a number → 400 `{"message":"Year must be a number","result":"error"}` if missing or non-numeric.
- **Error cases**:
  - Unknown service → 404 `{"message":"No result found","result":"error"}`.
- **Notable edge cases**:
  - Returns all months in the fiscal year even if no data (empty `{}`).
  - Combines historic `ft_notification_status` rows with live notifications from today's date.
  - Excludes `KEY_TYPE_TEST` notifications.
  - `2016` year → keys `2016-04` through `2017-03` (12 entries).
  - Notifications older than ~1 day are expected to be in `ft_notification_status`; live table cut-off is today only.
  - Month boundaries are date-based using EST timezone.
- **Auth requirements**: Internal auth header required.

---

### GET /service/`<service_id>`/template-statistics/monthly  *(get_monthly_template_usage)*

- **Happy path**: Returns `{"stats": [{template_id, name, type, month, year, count, is_precompiled_letter}, ...]}`.
- **Notable edge cases**:
  - Combines `ft_notification_status` (historical) with live notifications for the current day.
  - `is_precompiled_letter: true` for templates with `hidden=True` and name `PRECOMPILED_TEMPLATE_NAME`.
  - Results can include multiple entries per template (one per month).
- **Auth requirements**: Internal auth header required.

---

### GET /service/live-services  *(get_live_services_data)*

- **Happy path**: Returns `{"data": [...]}` with fields: `service_id`, `service_name`, `organisation_name`, `organisation_type`, `live_date`, `contact_name`, `contact_email`, `contact_mobile`, `sms_totals`, `email_totals`, `letter_totals`, `sms_volume_intent`, `email_volume_intent`, `letter_volume_intent`, `consent_to_research`, `free_sms_fragment_limit`.
- **Notable edge cases**:
  - Excludes: `restricted=True` services, `active=False` services, `count_as_live=False` services.
  - Totals reflect current fiscal year (not previous years).
- **Auth requirements**: Internal auth header required.

---

### GET /service/delivered-notifications-by-month  *(get_delivered_notification_stats_by_month_data)*

- **Happy path**: Returns `{"data": [{month, notification_type, count}]}`.
- **Notable edge cases**:
  - Accepts optional `filter_heartbeats=True` to exclude the NOTIFY_SERVICE_ID service.
- **Auth requirements**: Internal auth header required.

---

### GET /service/sensitive-service-ids

- **Happy path**: Returns `{"data": ["<uuid>", ...]}` listing IDs of services with `sensitive_service=True`.
- Returns `{"data": []}` when no sensitive services exist.
- **Auth requirements**: Internal auth header required.

---

### GET /service/`<service_id>`/history

- **Happy path**: Returns `{"data": {"service_history": [...], "api_key_history": [...]}}`.
- **Auth requirements**: Internal auth header required.

---

### GET /service/`<service_id>`/organisation

- **Happy path**: Returns serialized organisation if associated; empty `{}` if not.
- **Auth requirements**: Internal auth header required.

---

### POST /service/`<service_id>`/notifications/`<notification_id>`/cancel

- **Error cases**:
  - Not found → 404 `{"message":"Notification not found","result":"error"}`.
  - Not a letter → 400 `{"message":"Notification cannot be cancelled - only letters can be cancelled","result":"error"}`.
- **Auth requirements**: Internal auth header required.

---

### GET /service/`<service_id>`/annual-limit-stats

- **Happy path**: Returns `{email_delivered_today, email_failed_today, sms_delivered_today, sms_failed_today, total_email_fiscal_year_to_yesterday, total_sms_fiscal_year_to_yesterday}`.
- **Auth requirements**: Internal auth header required.

---

### GET /service/is-name-unique  *(is_service_name_unique)*

- **Happy path**: Returns `{"result": true}` if name is unique (case-insensitive, ignoring punctuation), `{"result": false}` otherwise.
- **Validation rules**:
  - `name` and `service_id` required → 400 if missing.
  - Uniqueness check excludes the service's own current name (same `service_id`).
- **Auth requirements**: Internal auth header required.

---

### GET /service/is-email-from-unique  *(is_service_email_from_unique)*

- **Happy path**: Returns `{"result": true/false}`.
- **Validation rules**:
  - `email_from` and `service_id` required → 400 if missing.
- **Auth requirements**: Internal auth header required.

---

### GET /service/monthly-notification-data  *(get_monthly_notification_data_by_service)*

- **Happy path**: Returns list of per-service monthly notification statuses.
- **Validation rules**: `start_date` and `end_date` required.
- **Auth requirements**: Internal auth header required.

---

## DAO Behavior Contracts

### `dao_create_service`

- **Expected behavior**: Persists `Service` record and creates a linked `ServiceHistory` record (version=1). Associates the `user` with the service. Optionally accepts `service_permissions` list; defaults to `["sms","email","international_sms"]`.
- **Edge cases and constraints verified**:
  - `user` parameter is required; `None` → `ValueError("Can't create a service without a user")`.
  - `name` must be unique (DB constraint: `services_name_key`).
  - `email_from` must be unique (DB constraint: `services_email_from_key`).
  - Both service and history records are created in the same transaction; if name is null, both rollback (IntegrityError).
  - `prefix_sms` defaults to `True`.
  - `research_mode` defaults to `False`.
  - `crown` is `None` by default.
  - Auto-assigns organisation if user's email domain matches an organisation's registered domains (inherits `organisation_type` from organisation).
  - NHS branding (email + letter) is auto-assigned if user email domain matches NHS patterns AND branding named "NHS" exists in the DB.

---

### `dao_update_service`

- **Expected behavior**: Persists changes to service and appends a new `ServiceHistory` record (version incremented).
- **Edge cases and constraints verified**:
  - Each call to update creates a new history entry.
  - Changing `permissions` list creates history entry.
  - Version n history record reflects the state after the nth change.

---

### `dao_fetch_service_by_id`

- **Expected behavior**: Returns service by UUID. Optionally reads from Redis cache when `use_cache=True`.
- **Edge cases and constraints verified**:
  - Not found → raises `NoResultFound` (`"No row was found when one was required"`).
  - Redis cache hit: deserializes JSON from bytes, returns service-like object with `id` as string.

---

### `dao_fetch_all_services`

- **Expected behavior**: Returns all services ordered by `created_at` ascending.
- **Edge cases and constraints verified**: Returns empty list when no services exist.

---

### `dao_fetch_all_services_by_user`

- **Expected behavior**: Returns services accessible to the given user (via `service_users` join); ordered by creation.
- **Edge cases and constraints verified**:
  - Only returns services the user has been explicitly added to.
  - Adding a user to service does not return that service for other users.
  - Returns empty list when user has no services.

---

### `get_services_by_partial_name`

- **Expected behavior**: Case-insensitive partial match on service name.
- **Edge cases and constraints verified**: Returns multiple matches; case differences do not affect matching.

---

### `dao_add_user_to_service`

- **Expected behavior**: Adds user–service relationship. Optionally accepts `permissions` list and `folder_permissions` list of folder UUIDs.
- **Edge cases and constraints verified**:
  - `folder_permissions`: non-existent folder UUIDs are silently ignored.
  - Folder UUIDs from a different service raise `IntegrityError` (FK violation on `user_folder_permissions`).
  - Adding user to a second service does NOT alter the user's permissions on the first service.

---

### `dao_remove_user_from_service`

- **Expected behavior**: Removes user–service relationship, deletes user's `Permission` records for that service, and deletes `user_folder_permissions` for that service.
- **Edge cases and constraints verified**:
  - Folder permissions on OTHER services are preserved.
  - `Permission` query returns empty after removal.

---

### `dao_suspend_service`

- **Expected behavior**: Sets `service.active = False`. Does NOT expire/revoke API keys (`expiry_date` remains `None`).
- **Edge cases and constraints verified**:
  - API keys survive suspension intact.

---

### `dao_resume_service`

- **Expected behavior**: Sets `service.active = True`. Does not alter API key state.
- **Edge cases and constraints verified**:
  - API keys that were previously expired via `expire_api_key` remain expired after resume.

---

### `dao_fetch_active_users_for_service`

- **Expected behavior**: Returns only users with `state="active"` for the given service.
- **Edge cases and constraints verified**: Users with `state="pending"` are excluded.

---

### `dao_fetch_service_by_inbound_number`

- **Expected behavior**: Returns service linked to an active inbound number.
- **Edge cases and constraints verified**:
  - Returns `None` if number is not assigned to any service.
  - Returns `None` if inbound number exists but has no service set.
  - Returns `None` if number is unknown.
  - Returns `None` if the inbound number has been deactivated.

---

### `dao_fetch_service_creator`

- **Expected behavior**: Returns the user recorded as `created_by_id` in the version-1 history record.
- **Edge cases and constraints verified**:
  - If the version-1 history entry has been deleted, falls back to the earliest available history entry.

---

### `dao_fetch_service_ids_of_sensitive_services`

- **Expected behavior**: Returns list of `str(uuid)` for services with `sensitive_service=True`.
- **Edge cases and constraints verified**: Returns empty list when no sensitive services.

---

### `dao_fetch_stats_for_service`

- **Expected behavior**: Returns rows of `(notification_type, status, count)` for the given service within the last `limit_days` days (calendar-day boundary uses local/EST timezone).
- **Edge cases and constraints verified**:
  - Excludes `KEY_TYPE_TEAM` and `KEY_TYPE_TEST`; counts normal + team? Actually verified: counts `KEY_TYPE_NORMAL` + `KEY_TYPE_TEAM`, excludes `KEY_TYPE_TEST`. (Test shows 3 from normal+team+one without key, excluding test.)
  - Excludes `NotificationHistory`; only queries live `notifications` table.
  - Filters by `service_id`.
  - Time boundary: notifications older than `limit_days` calendar days (in EST) are excluded.
  - Decorated with `__wrapped__` (transactional decorator).

---

### `dao_fetch_todays_stats_for_service`

- **Expected behavior**: Returns notification counts grouped by status for today only (EST midnight boundary).
- **Edge cases and constraints verified**:
  - "Yesterday at 23:59 EST" is excluded; "today at 00:01 EST" is included.

---

### `dao_fetch_todays_stats_for_all_services`

- **Expected behavior**: Returns rows grouped by `(service_id, service_name, restricted, research_mode, active, created_at, notification_type, status, count)` for all services.
- **Edge cases and constraints verified**:
  - Default includes all key types; `include_from_test_key=False` excludes `KEY_TYPE_TEST`.
  - Only includes today's notifications (EST boundary).
  - Results ordered by service ID.

---

### `dao_fetch_live_services_data`

- **Expected behavior**: Returns billing + contact data for live services (not restricted, active, `count_as_live=True`).
- **Edge cases and constraints verified**:
  - `filter_heartbeats=True` excludes the NOTIFY_SERVICE_ID service.
  - Totals reflect the current fiscal year only.

---

### `fetch_todays_total_message_count`

- **Expected behavior**: Returns total notification count for the service today (all types).
- **Edge cases and constraints verified**:
  - Returns 0 for unknown service.
  - Returns 0 for yesterday's notifications.
  - Includes scheduled jobs whose `scheduled_for` date is today (not tomorrow, not yesterday).

---

### `fetch_todays_total_sms_count`

- **Expected behavior**: Returns count of SMS notifications created today.
- **Edge cases and constraints verified**:
  - Excludes email notifications.
  - Returns 0 for unknown service or when no SMS today.

---

### `fetch_todays_total_sms_billable_units`

- **Expected behavior**: Returns the sum of `billable_units` for today's SMS notifications (`NULL` treated as 0).
- **Edge cases and constraints verified**:
  - Excludes email notifications.
  - Returns 0 when no SMS sent today.
  - Excludes yesterday's notifications.
  - `NULL` `billable_units` treated as 0 in the sum (feature-flag-disabled scenario).

---

### `delete_service_and_all_associated_db_objects`

- **Expected behavior**: Cascades deletion of all related data.
- **Edge cases and constraints verified**: After deletion, the following counts are 0: `VerifyCode`, `ApiKey`, `ApiKey` history, `Template`, `TemplateHistory`, `Job`, `Notification`, `Permission`, `User`, `InvitedUser`, `Service`, `Service` history, `ServicePermission`.

---

### `save_service_callback_api`

- **Expected behavior**: Persists `ServiceCallbackApi` with bearer token stored signed (not plaintext). Creates version-1 history record. One callback per `(service_id, callback_type)` — attempting a second with same type raises `SQLAlchemyError`.
- **Edge cases and constraints verified**:
  - Two callbacks of different types (`delivery_status` and `complaint`) are allowed per service.
  - `_bearer_token` (stored) ≠ `bearer_token` (plaintext).
  - `updated_at` is `None` on creation.
  - Fails if service does not exist (`SQLAlchemyError`).

---

### `reset_service_callback_api`

- **Expected behavior**: Updates URL and/or bearer token; creates a new history record (version incremented); sets `updated_at`.
- **Edge cases and constraints verified**:
  - History properly tracks old and new URLs across versions.
  - Signed bearer token rewritten on update.

---

### `suspend_unsuspend_service_callback_api`

- **Expected behavior**: Sets `is_suspended` + `suspended_at` (when suspending); creates new history record.
- **Edge cases and constraints verified**:
  - Version 1: `is_suspended=None`. Version 2: `is_suspended=True` after suspend.

---

### `resign_service_callbacks`

- **Expected behavior**: Re-signs `_bearer_token` values using the new active signing key.
- **Edge cases and constraints verified**:
  - `resign=True`: re-signs (signature changes, plaintext same).
  - `resign=False`: preview only (signature unchanged).
  - Fails with `BadSignature` if old key is no longer in the signer's key list, unless `unsafe=True`.
  - `unsafe=True`: re-signs even when old signature cannot be verified.

---

### `insert_service_data_retention`

- **Expected behavior**: Inserts `ServiceDataRetention` record.
- **Edge cases and constraints verified**:
  - One record per `(service_id, notification_type)` — duplicate raises `IntegrityError`.
  - `created_at` set to today's date.

---

### `update_service_data_retention`

- **Expected behavior**: Updates `days_of_retention` for the matching record. Returns number of updated rows (1 on success, 0 if not found).
- **Edge cases and constraints verified**:
  - Returns 0 if `service_data_retention_id` not found.
  - Returns 0 if `service_id` does not match the record's service.
  - `updated_at` is set after update.

---

### `fetch_service_data_retention`

- **Expected behavior**: Returns list of all retention records for a service, ordered by notification type.
- **Edge cases and constraints verified**:
  - Only returns records for the specified service.
  - Returns empty list when none exist.

---

### `fetch_service_data_retention_by_id`

- **Expected behavior**: Returns record by ID scoped to a service.
- **Edge cases and constraints verified**:
  - Returns `None` if not found.
  - Returns `None` if record belongs to a different service.

---

### `fetch_service_data_retention_by_notification_type`

- **Expected behavior**: Returns single record for (service, notification_type).
- **Edge cases and constraints verified**: Returns `None` when no matching record.

---

### `add_reply_to_email_address_for_service`

- **Expected behavior**: Creates `ServiceEmailReplyTo`. If `is_default=True`, demotes existing default.
- **Edge cases and constraints verified**:
  - First address must be `is_default=True`; `False` raises `InvalidRequest`.
  - Multiple addresses are allowed.
  - Adding a new default demotes the current default to non-default.
  - Having two defaults already raises an exception.
  - `archived` defaults to `False`.

---

### `update_reply_to_email_address`

- **Expected behavior**: Updates email address and/or `is_default`. Sets `updated_at`.
- **Edge cases and constraints verified**:
  - Setting a non-default as default demotes the current default.
  - Setting the only address to `is_default=False` raises `InvalidRequest`.

---

### `archive_reply_to_email_address`

- **Expected behavior**: Sets `archived=True` and updates `updated_at`.
- **Edge cases and constraints verified**:
  - Can archive the default if it is the only address.
  - Cannot archive the default when other non-archived addresses exist → raises `ArchiveValidationError("You cannot delete a default email reply to address if other reply to addresses exist")`.
  - Cannot archive a reply-to from a different service → `SQLAlchemyError`.

---

### `dao_get_reply_to_by_service_id`

- **Expected behavior**: Returns all non-archived reply-to addresses for a service.
- **Edge cases and constraints verified**:
  - Archived addresses are excluded.

---

### `dao_get_reply_to_by_id`

- **Expected behavior**: Returns `ServiceEmailReplyTo` by id scoped to service.
- **Edge cases and constraints verified**:
  - Not found → `SQLAlchemyError`.
  - Archived address → `SQLAlchemyError`.
  - Wrong service → `SQLAlchemyError`.

---

### `save_service_inbound_api`

- **Expected behavior**: Persists `ServiceInboundApi` with bearer token signed. Creates version-1 history.
- **Edge cases and constraints verified**:
  - Fails if service does not exist → `SQLAlchemyError`.
  - `_bearer_token` ≠ `bearer_token` (token is signed).

---

### `reset_service_inbound_api`

- **Expected behavior**: Updates URL and/or bearer token; creates new history record (version 2); sets `updated_at`.
- **Edge cases and constraints verified**: Each update increments version.

---

### `get_service_inbound_api`

- **Expected behavior**: Returns `ServiceInboundApi` by (id, service_id).
- **Edge cases and constraints verified**: Signed `_bearer_token` verified via `signer_bearer_token.verify`.

---

### `get_service_inbound_api_for_service`

- **Expected behavior**: Returns the inbound API config for a service.

---

### `add_letter_contact_for_service`

- **Expected behavior**: Creates `ServiceLetterContact`. If `is_default=True`, demotes existing default.
- **Edge cases and constraints verified**:
  - `is_default=False` is valid (no first-address constraint unlike email reply-to).
  - Setting new default demotes existing default.
  - Multiple defaults in DB before adding raises exception.

---

### `update_letter_contact`

- **Expected behavior**: Updates `contact_block` and `is_default`. Sets `updated_at`.
- **Edge cases and constraints verified**:
  - Updating to default demotes existing default.
  - Setting the only contact to `is_default=False` is allowed (no constraint).

---

### `archive_letter_contact`

- **Expected behavior**: Sets `archived=True`, sets `updated_at`. Disassociates templates that reference this contact (`template.reply_to → None`) before archiving.
- **Edge cases and constraints verified**:
  - Can archive the default contact (no restriction).
  - Cannot archive a contact from a different service → `SQLAlchemyError`.
  - Templates previously using this letter contact have `reply_to` set to `None` after archiving.

---

### `dao_get_letter_contacts_by_service_id`

- **Expected behavior**: Returns non-archived letter contacts for a service, default contact first.
- **Edge cases and constraints verified**: Archived contacts excluded.

---

### `dao_get_letter_contact_by_id`

- **Expected behavior**: Returns contact by (service_id, letter_contact_id).
- **Edge cases and constraints verified**:
  - Not found → `SQLAlchemyError`.
  - Archived contact → `SQLAlchemyError`.
  - Wrong service → `SQLAlchemyError`.

---

### `dao_add_sms_sender_for_service`

- **Expected behavior**: Creates new `ServiceSmsSender`. If `is_default=True`, demotes existing default.
- **Edge cases and constraints verified**: Default switches correctly on new sender.

---

### `dao_update_service_sms_sender`

- **Expected behavior**: Updates sender; if `is_default=True`, demotes existing default.
- **Edge cases and constraints verified**:
  - Cannot leave the service with no default sender → raises `Exception("You must have at least one SMS sender as the default")`.

---

### `archive_sms_sender`

- **Expected behavior**: Sets `archived=True`, sets `updated_at`.
- **Edge cases and constraints verified**:
  - Cannot archive the default sender → `ArchiveValidationError("You cannot delete a default sms sender")`.
  - Cannot archive an inbound number sender (regardless of whether it is default) → `ArchiveValidationError("You cannot delete an inbound number")`.
  - Cannot archive a sender from a different service → `SQLAlchemyError`.

---

### `dao_get_service_sms_senders_by_id`

- **Expected behavior**: Returns `ServiceSmsSender` by (service_id, sms_sender_id).
- **Edge cases and constraints verified**:
  - Not found or archived → `SQLAlchemyError`.

---

### `dao_get_sms_senders_by_service_id`

- **Expected behavior**: Returns all non-archived SMS senders for a service.
- **Edge cases and constraints verified**: Archived senders excluded.

---

### `update_existing_sms_sender_with_inbound_number`

- **Expected behavior**: Updates an existing `ServiceSmsSender` to link it to an inbound number.
- **Edge cases and constraints verified**:
  - Non-existent `inbound_number_id` → `SQLAlchemyError`.

---

### `dao_add_service_permission` / `dao_remove_service_permission` / `dao_fetch_service_permissions`

- **Expected behavior**: Add/remove/list `ServicePermission` records for a service.
- **Edge cases and constraints verified**:
  - Remove one permission: remaining permissions are correct.
  - Can add a permission back after removing it.
  - Fetch returns only the specified service's permissions.

---

### `dao_add_and_commit_safelisted_contacts`

- **Expected behavior**: Bulk-inserts `ServiceSafelist` records and commits.
- **Edge cases and constraints verified**: Records appear in DB after call.

---

### `dao_remove_service_safelist`

- **Expected behavior**: Deletes safelist entries for the given service WITHOUT committing.
- **Edge cases and constraints verified**:
  - Only removes entries for the target service (other services unaffected).
  - No auto-commit: calling `session.rollback()` after this restores the records.

---

### `dao_fetch_service_safelist`

- **Expected behavior**: Returns safelist entries for the given service.
- **Edge cases and constraints verified**: Returns empty list for unknown service UUID.

---

## Business Rules Verified by Tests

### Service Lifecycle

**Create**
- On creation, Salesforce `engagement_create` is called.
  - **⚠️ No retry or idempotency**: all 4 Salesforce call points (`engagement_create`, `engagement_update`, `engagement_delete_contact_role`, and the archive/suspend close path) fire synchronously with no retry logic and are entirely mocked in tests. A Salesforce API failure will surface as an unhandled exception in the service layer. Go must decide on a retry / circuit-breaker policy for Salesforce calls before implementation.
- `count_as_live` defaults to `True` for non-platform-admin users; `False` for platform admins.
- Organisation auto-assigned from user's email domain; longest-matching domain wins.
- NHS branding auto-assigned if email domain matches NHS pattern and NHS branding exists.
- Service starts with default permissions `[email, sms, international_sms]` and one SMS sender using `FROM_NUMBER`.

**Update / Go Live**
- When `restricted` flips `True → False`: sends go-live notification; updates Salesforce stage to `ENGAGEMENT_STAGE_LIVE`. Engagement user is `go_live_user` if set, else service creator.
- When `message_limit` changes on a live (non-restricted) service: notifies users; clears daily-limit Redis keys.
- Annual limit changes clear the appropriate annual-limit Redis hash fields.
- Name change updates Salesforce engagement name.

**Suspend**
- `POST /service/<id>/suspend`: sets `active=False`, records `suspended_at`. API keys are NOT revoked. History version incremented.
- If already suspended, no-op (returns 204, DAO not called).
- `suspended_by_id` set only if `user_id` provided in request body.

**Resume**
- `POST /service/<id>/resume`: sets `active=True`. Pre-existing revoked API keys stay revoked.
- If already active, no-op.

**Archive**
- `POST /service/<id>/archive`: full deactivation. Renames both `name` and `email_from` with timestamp prefix. Revokes all API keys. Archives all templates. Sends deletion email to all service users. Entire operation is a single transaction (no partial writes).
- Already-inactive service: no-op (returns 204, no renames/revocations).
- Pre-existing revoked keys and archived templates are not modified.

---

### Permission Management

- Default service permissions: `email`, `sms`, `international_sms`.
- Permissions stored per service; update operation is a FULL replace (not additive).
- Valid permissions include: `email`, `sms`, `letter`, `international_sms`, `inbound_sms`.
- Invalid permission string → 400. Duplicate in list → 400.
- User permissions: independent per-service; adding user to a second service does not affect permissions on other services.

---

### Safelist / Whitelist Behavior

- `PUT /service/<id>/safelist` is a full replace: all existing entries are deleted, new ones inserted atomically.
- On validation failure (empty string, invalid email or phone), the update is rejected entirely and the existing safelist preserved.
- Response separates `email_addresses` from `phone_numbers`.
- `dao_remove_service_safelist` does not auto-commit; must be explicitly committed by caller.

---

### Default Sender / Reply-To / Contact Constraints

**Email reply-to**
- First reply-to address MUST be default.
- Multiple addresses allowed.
- Setting a new default demotes the current default.
- Cannot archive the default when other active addresses exist.
- Can archive the default if it is the only address.
- Archived addresses excluded from listings and GET-by-id.
- `dao_get_reply_to_by_id` raises `SQLAlchemyError` for archived addresses.

**SMS senders**
- At least one non-archived default sender must exist at all times.
- Attempting to update to `is_default=False` when it would leave no default → error.
- Cannot archive the default sender.
- Cannot archive an inbound number sender (regardless of default status).
- Archived senders excluded from listing.

**Letter contacts**
- First contact can be non-default (no constraint enforced by DAO).
- Archiving a letter contact nulls out `reply_to` on any templates that reference it.
- Can archive the default contact (no restriction).

**Callback / Inbound APIs**
- Bearer tokens stored encrypted (signed via `signer_bearer_token`); plaintext never persisted.
- One callback per `(service_id, callback_type)` (unique constraint).
- Two types are allowed per service simultaneously: `delivery_status` and `complaint`.
- Callback can be suspended/unsuspended via `is_suspended` flag.
- Key rotation: `resign_service_callbacks(resign=True)` re-signs tokens; fails with `BadSignature` if old key not available unless `unsafe=True`.

---

### Callback URL Validation

From `update_service_callback_api_schema`:
- `url` must be HTTPS (`https://...`). HTTP, bare text, or missing scheme → validation error.
- `bearer_token` minimum 10 characters.
- Applies to both delivery-status callbacks and inbound SMS API URLs.

---

### Statistics Calculation

**`format_statistics(stats)`** — maps raw (notification_type, status, count) rows to `{sms/email/letter: {requested, delivered, failed}}`:
- `"requested"` = sum of all statuses for the type (each raw row contributes to requested).
- `"delivered"` = statuses `"delivered"` or `"sent"` (sending also counts as delivered for SMS).
- `"failed"` = failed statuses: `"failed"`, `"technical-failure"`, `"temporary-failure"`, `"permanent-failure"`, `"validation-failed"`, `"virus-scan-failed"`, `"cancelled"`.
- `None` rows safely ignored.
- Types are independent; stats for one type never pollute another.

**`format_admin_stats(rows)`** — maps (notification_type, status, key_type, count) to `{type: {total, test-key, failures: {...}}}`:
- `"test-key"` accumulates only `KEY_TYPE_TEST` notifications; NOT added to `total`.
- `"total"` accumulates non-test-key notifications.
- `"failures"` sub-keys: `technical-failure`, `permanent-failure`, `temporary-failure`, `virus-scan-failed`.

**Monthly stats (`create_empty_monthly_notification_status_stats_dict`)**:
- Fiscal year starts April 1; year `2018` means April 2018 – March 2019 (12 months).
- `add_monthly_notification_status_stats` merges row data into the dict; existing values are summed, not replaced.

**Monthly API stats**:
- `GET .../monthly-usage`: excludes `KEY_TYPE_TEST` notifications.
- Combines `ft_notification_status` table (historical, older data) with live `notifications` table (today's data).
- Data for the day before today is expected to be in `ft_notification_status` (backfill assumed).

---

### One-Off Notification Sending Rules (`send_one_off_notification`)

**General flow**:
1. Validate template belongs to service.
2. Validate `created_by` user is a member of the service.
3. Validate recipient format (phone for SMS, email for email).
4. For restricted (trial) services: validate recipient is in safelist.
5. Check daily limits (SMS and email separately).
6. Check annual limits.
7. Check SMS message length (character count ≤ `SMS_CHAR_COUNT_LIMIT`).
8. Persist notification.
9. Enqueue via `send_notification_to_queue`.
10. Increment daily count (unless simulated/test recipient).
11. Return `{"id": "<notification_id>"}`.

**Queue routing**:
- `email` + `priority` process type → `SEND_EMAIL_HIGH`; `bulk` or `normal` → `SEND_EMAIL_MEDIUM`.
- `sms` + `priority` process type → `SEND_SMS_HIGH`; `bulk` or `normal` → `SEND_SMS_MEDIUM`.

**Research mode**:
- Services with `research_mode=True`: `send_notification_to_queue` called with `research_mode=True`.

**Sender selection**:
- `sender_id` in request body: uses the specified reply-to email or SMS sender. Not found → `BadRequestError("Reply to email address not found")` or `BadRequestError("SMS sender not found")`.
- No `sender_id`: uses service's default SMS sender for SMS; no reply-to for email.
- `reply_to_text` on notification is set to the sender's address.

**Billable units (feature-flagged via `FF_USE_BILLABLE_UNITS`)**:
- `FF_USE_BILLABLE_UNITS=True`: passes `billable_units` from the persisted notification to `increment_sms_daily_count_send_warnings_if_needed`.
- `FF_USE_BILLABLE_UNITS=False`: increments daily count by 1 regardless of message length.
- Test/simulated recipients do NOT increment the daily count.
- Email notifications always increment by 1 (no billable_units concept).

**Error cases**:
- Invalid phone → `InvalidPhoneError`.
- Recipient not in safelist for restricted service → `BadRequestError("service is in trial mode")`.
- Over daily SMS limit → `LiveServiceTooManySMSRequestsError`.
- Over daily email limit → `LiveServiceTooManyEmailRequestsError`.
- Message too long → `BadRequestError("Content for template has a character count greater than the limit of <N>")`.
- `created_by` not in service → `BadRequestError("Can't create notification - <name> is not part of the \"<service>\" service")`.

---

### `send_notification_to_service_users`

- Sends a notification to ALL active (`state="active"`) users of a service.
- Inactive/pending users are excluded.
- The notification is sent FROM the NOTIFY_SERVICE using the specified template.
- `reply_to_text` on the notification is the NOTIFY_SERVICE's default reply-to email address.
- Optional `include_user_fields` list: injects user attributes (`name`, `email_address`, `state`, etc.) into personalisation.
- Dispatches to Celery send queue; `send_notification_to_queue` called once per user.

---

### Financial Year Calculation

- Financial year starts April 1 (Canadian government fiscal year).
- UTC boundary: before April 1 04:00 UTC (00:00 EST) is still in the previous year.
- `get_current_financial_year_start_year`: `2017-03-31 22:59:59.999999 UTC` → year 2016; `2017-04-01 04:00:00 UTC` → year 2017.

---

### Organisation Lookup from CRM Notes (`get_organisation_id_from_crm_org_notes`)

- Matches `org_notes` string (in `"<name> > <suffix>"` format) against GC organisation data by English or French name.
- Returns `notify_organisation_id` when name matches AND the organisation has a non-null `notify_organisation_id`.
- Returns `None` if name matches but `notify_organisation_id` is `None`, or if name not found.
- Falls back to local bundled data when `GC_ORGANISATIONS_BUCKET_NAME` config is `None`.
- Fallback data includes Canadian organisations (e.g., "Canadian Space Agency", bilingual French names).
