# Brief: service-management

## Source Files

- `spec/behavioral-spec/services.md` — endpoint contracts and DAO behavior extracted from the Python test suite
- `spec/business-rules/services.md` — domain rules, invariants, DAO signatures, full query inventory
- `openspec/changes/service-management/proposal.md` — scope, C7 fix description, impact/dependencies

---

## Endpoints

### GET /service

- Returns `{"data": [...]}` with all services ordered by `created_at ASC`. Each item includes `id`, `name`, `permissions`, and other service fields.
- Query params:
  - `only_active=True` — exclude services where `active=False`.
  - `user_id=<uuid>` — filter to services the user belongs to; empty list if user has no services.
  - `detailed=True` — include per-service `statistics` (requested/delivered/failed for sms/email/letter). Default date range: today. Supports `start_date`/`end_date`.
  - `include_from_test_key=False` — exclude KEY_TYPE_TEST notifications from statistics (default: include).
- Auth: internal auth header required.
- Sub-route: `GET /service/find-by-name?service_name=ABC` — case-insensitive partial name search. Returns 400 if `service_name` param is absent.

---

### GET /service/`<service_id>`

- Returns `{"data": {...}}` with full service object.
- Fields: `id`, `name`, `email_from`, `permissions`, `research_mode`, `prefix_sms` (default `true`), `email_branding` (nullable), `letter_logo_filename` (nullable), `go_live_user`, `go_live_at`, `organisation_type`, `count_as_live`. Note: `branding` key NOT present; `email_branding` IS present.
- Optional query params: `user_id` (scope to user membership), `detailed=True` (add statistics), `today_only=True|False`.
- 404 if service not found (with or without `user_id`): `{"result":"error","message":"No result found"}`.
- Auth: internal auth header required.

---

### POST /service

- Creates service; returns 201 `{"data": {...}}`.
- Required fields: `name`, `user_id`, `message_limit`, `restricted`, `email_from`, `created_by`. Missing any → 400 with per-field error `"Missing data for required field."`.
- `user_id` not found → 404.
- Duplicate `name` → 400 `"Duplicate service name '<name>'"`.
- Duplicate `email_from` → 400 `"Duplicate service name '<email_from>'"`.
- `count_as_live = not user.platform_admin`.
- Organisation auto-assigned: user email domain matched against organisation domains (longest match wins); no match → `organisation: null`.
- New service does NOT inherit email/letter branding from its organisation by default.
- Default permissions created: `email`, `sms`, `international_sms`.
- One default `ServiceSmsSender` created using `FROM_NUMBER` config.
- If NHS email domain detected AND NHS branding exists in DB: NHS email + letter branding auto-assigned.
- Calls Salesforce `engagement_create` on creation.
- Response includes: `id`, `name`, `email_from`, `rate_limit` (1000), `letter_branding` (null), `count_as_live`, permissions, default SMS sender.
- Auth: internal auth header required.

---

### POST /service/`<service_id>`

- Updates service fields; returns 200 `{"data": {...}}`.
- Validation:
  - `research_mode`: boolean required; non-boolean → 400 `"Not a valid boolean."`.
  - `organisation_type`: valid enum; invalid → 500.
  - `permissions`: list of valid strings; invalid → 400 `"Invalid Service Permission: '<value>'"`. Duplicates → 400 `"Duplicate Service Permission: ['<value>']"`.
  - `volume_email`, `volume_sms`, `volume_letter`: integer or null.
  - `consent_to_research`: accepts `True`, `False`, `"Yes"`, `"No"`. Invalid → 400.
  - `prefix_sms`: boolean; null → 400 `{"prefix_sms": ["Field may not be null."]}`.
  - `email_branding: null` is valid and removes branding.
  - Duplicate `name`/`email_from` for a different service → 400.
  - Non-existent `service_id` → 404.
- Permissions update is full-replace: sending `["sms","email"]` removes all other permissions.
- Side effects:
  - `restricted: True → False` (go-live): sends notification with `SERVICE_BECAME_LIVE_TEMPLATE_ID`; calls Salesforce `engagement_update({StageName: LIVE})`.
  - `message_limit` change on live service: notify users with `DAILY_EMAIL_LIMIT_UPDATED_TEMPLATE_ID`; clear Redis daily-limit cache. No notification for trial/restricted services or when limit unchanged.
  - `sms_annual_limit` change: delete `near_sms_limit` and `near_email_limit` from Redis annual-limit hash.
  - `email_annual_limit` change: delete `over_email_limit` and `near_email_limit` from Redis annual-limit hash.
  - `name` change: Salesforce `engagement_update({"Name": "<new_name>"})`.
- Auth: internal auth header required.

---

### GET /service/`<service_id>`/users

- Returns `{"data": [...]}` with user details (`name`, `email`, `mobile`).
- Unknown service → 404.
- No users → 200 `{"data": []}`.
- Auth: internal auth header required.

---

### POST /service/`<service_id>`/users/`<user_id>`

- Adds user to service; returns 201 with updated service data.
- `permissions`: list of `{"permission": "<name>"}` objects. Missing → 400.
- `folder_permissions`: optional list of folder UUID strings.
- Service not found → 404 `"No result found"`. User not found → 404 `"No result found"`.
- User already in service → 409 `"User id: <id> already part of service id: <id>"`.
- Folder permissions for non-existent folders silently ignored. Folder permissions from different service → IntegrityError.
- Auth: internal auth header required.

---

### DELETE /service/`<service_id>`/users/`<user_id>`

- Removes user; returns 204.
- Last remaining user → 400 `"You cannot remove the only user for a service"`.
- Last user with `manage_settings` permission → 400 `"SERVICE_NEEDS_USER_W_MANAGE_SETTINGS_PERM"`.
- Would leave fewer than 2 members → 400 `"SERVICE_CANNOT_HAVE_LT_2_MEMBERS"`.
- User not in service → 404.
- Calls Salesforce `engagement_delete_contact_role(service, user)`.
- Removes `Permission` records and `user_folder_permissions` for that service only; other services unaffected.
- Auth: internal auth header required.

---

### POST /service/`<service_id>`/api-key

- Creates API key; returns 201 `{"data": {"key": "<prefixed_key>", "key_name": "<name>"}}`.
- Required: `name`, `created_by`, `key_type`. Missing `key_type` → 400 `{"key_type": ["Missing data for required field."]}`.
- Unknown service → 404.
- `key_name` in response prefixed with `API_KEY_PREFIX` config value.
- Multiple keys per service allowed; each key produces a distinct value.
- Auth: internal auth header required.

---

### GET /service/`<service_id>`/api-keys

- Returns `{"apiKeys": [...]}` including both active and expired keys.
- Optional `key_id` query param returns single matching key.
- Keys for other services excluded; expired keys included.
- Auth: internal auth header required.

---

### POST /service/`<service_id>`/api-key/`<api_key_id>`/revoke

- Sets `expiry_date` on key; returns 202.
- Auth: internal auth header required.

---

### POST /service/`<service_id>`/archive

- Archives service; returns 204. POST only (GET → 405).
- Unknown service → 404. Already inactive → 204 (idempotent no-op).
- On archive:
  - Renames `name` → `_archived_<YYYY-MM-DD>_<HH:MM:SS>_<original_name>`.
  - Renames `email_from` → `_archived_<YYYY-MM-DD>_<HH:MM:SS>_<original_email_from>`.
  - Sets `active = False`.
  - Sets `expiry_date` on all API keys that do not already have one. Pre-revoked keys unchanged.
  - Sets `archived = True` on all non-archived templates. Pre-archived templates unchanged.
  - Creates new service history record (version incremented).
  - Sends deletion email to service users with `SERVICE_DEACTIVATED_TEMPLATE_ID` and `{"service_name": "<name>"}`.
- Entire operation transactional; failure rolls back all changes.
- Auth: internal auth header required.

---

### POST /service/`<service_id>`/suspend

- Suspends service (sets `active = False`); returns 204. POST only (GET → 405).
- Unknown service → 404. Already inactive → 204 (no-op).
- Creates service history record (version incremented,`active = False`).
- Does NOT revoke API keys (`expiry_date` unchanged).
- Sets `suspended_at` timestamp (UTC).
- Optional body `user_id`; if absent, `suspended_by_id` remains `None`.
- Auth: internal auth header required.

---

### POST /service/`<service_id>`/resume

- Resumes service (sets `active = True`); returns 204. POST only (GET → 405).
- Unknown service → 404. Already active → 204 (no-op).
- Creates service history record.
- Clears `suspended_at` and `suspended_by_id`.
- Previously revoked API keys remain revoked.
- Auth: internal auth header required.

---

### GET /service/`<service_id>`/safelist

- Returns `{"email_addresses": [...], "phone_numbers": [...]}`.
- Unknown service → 404 `{"result":"error","message":"No result found"}`.
- Empty safelist → 200 `{"email_addresses": [], "phone_numbers": []}`.
- Auth: internal auth header required.

---

### PUT /service/`<service_id>`/safelist

- Replaces entire safelist; returns 204.
- Each entry must be valid email or phone number. Empty string → 400 `{"result":"error","message":"Invalid safelist: \"\" is not a valid email address or phone number"}`.
- On invalid entry → 400; existing safelist preserved (no partial writes).
- Auth: internal auth header required.

---

### GET /service/`<service_id>`/data-retention

- Returns list of `ServiceDataRetention` objects; 200. Empty list if none exist.
- Auth: internal auth header required.

---

### GET /service/`<service_id>`/data-retention/`<retention_id>`

- Returns serialized `ServiceDataRetention`; 200.
- Not found → 200 `{}` (NOT a 404).
- Auth: internal auth header required.

---

### GET /service/`<service_id>`/data-retention/notification-type/`<type>`

- Returns serialized `ServiceDataRetention` for the given notification type; 200.
- Auth: internal auth header required.

---

### POST /service/`<service_id>`/data-retention

- Creates data retention record; returns 201 `{"result": {...}}`.
- `notification_type`: `sms`, `email`, `letter`. Invalid → 400 `"notification_type <value> is not one of [sms, letter, email]"`.
- Duplicate `(service_id, notification_type)` → 400 `{"result":"error","message":"Service already has data retention for <type> notification type"}`.
- Auth: internal auth header required.

---

### POST /service/`<service_id>`/data-retention/`<retention_id>`

- Updates `days_of_retention`; returns 204.
- Body must contain `days_of_retention`. Missing → 400. Unknown `retention_id` → 404.
- Auth: internal auth header required.

---

### GET /service/`<service_id>`/email-reply-to

- Returns list; 200 (may be empty). Each entry: `id`, `service_id`, `email_address`, `is_default`, `created_at`, `updated_at` (null if never updated).
- Auth: internal auth header required.

---

### GET /service/`<service_id>`/email-reply-to/`<reply_to_id>`

- Returns serialized `ServiceEmailReplyTo`; 200.
- Auth: internal auth header required.

---

### POST /service.add_service_reply_to_email_address

- Creates reply-to; returns 201 `{"data": {...}}`.
- First reply-to added with `is_default=False` → 400 `"You must have at least one reply to email address as the default."`.
- Duplicate email for same service → 400.
- Unknown service → 404.
- Adding with `is_default=True` demotes existing default.
- Auth: internal auth header required.

---

### POST /service.update_service_reply_to_email_address

- Updates reply-to; returns 200 `{"data": {...}}`.
- Setting `is_default=False` when only one address exists → 400 `"You must have at least one reply to email address as the default."`.
- Unknown service → 404.
- Auth: internal auth header required.

---

### POST /service.delete_service_reply_to_email_address

- Archives reply-to (sets `archived=True`); returns 200.
- Cannot archive default while other non-archived addresses exist → 400 `"You cannot delete a default email reply to address if other reply to addresses exist"`.
- Archiving the only address (even if default) is permitted.
- Auth: internal auth header required.

---

### POST /service.verify_reply_to_email_address

- Sends verification notification; returns 201 `{"data": {"id": "<notification_id>"}}`.
- Duplicate email already used by service → 400.
- Delivers via `deliver_email` Celery task on `notify-internal-tasks` queue.
- `reply_to_text` on notification = service's current default reply-to.
- Auth: internal auth header required.

---

### GET /service/`<service_id>`/sms-sender

- Returns list; 200. Each entry: `id`, `sms_sender`, `is_default`, `inbound_number_id`.
- Unknown service → 200 `[]` (no 404).
- Auth: internal auth header required.

---

### GET /service/`<service_id>`/sms-sender/`<sms_sender_id>`

- Returns serialized `ServiceSmsSender`; 200.
- Unknown service or `sms_sender_id` → 404.
- Auth: internal auth header required.

---

### POST /service/`<service_id>`/sms-sender (add)

- Creates new sender; returns 201.
- `inbound_number_id` provided + service has one non-archived sender → replaces it. If multiple non-archived senders exist → inserts new one.
- `is_default=True` demotes existing default.
- Unknown service → 404.
- Auth: internal auth header required.

---

### POST /service/`<service_id>`/sms-sender/`<sms_sender_id>` (update)

- Updates sender; returns 200.
- Cannot update `sms_sender` string if sender has `inbound_number_id` → 400.
- `is_default=True` switches default away from previous sender.
- Unknown service or sender → 404.
- Auth: internal auth header required.

---

### POST /service.delete_service_sms_sender (archive)

- Sets `archived=True`; returns 200.
- Cannot archive inbound number → 400 `{"message":"You cannot delete an inbound number","result":"error"}`.
- Cannot archive default sender → 400.
- Auth: internal auth header required.

---

### POST /service_callback.create_service_inbound_api

- Creates inbound SMS API webhook; returns 201 `{"data": {id, service_id, url, updated_by_id, created_at, updated_at: null}}`.
- Unknown service → 404 `{"message":"No result found"}`.
- Required: `url`, `bearer_token`, `updated_by_id`.
- Auth: internal auth header required.

---

### POST /service_callback.update_service_inbound_api

- Updates URL or bearer token; returns 200 `{"data": {...}}`.
- Auth: internal auth header required.

---

### GET /service_callback.fetch_service_inbound_api

- Returns `{"data": {...}}`; 200.
- Auth: internal auth header required.

---

### DELETE /service_callback.remove_service_inbound_api

- Hard-deletes record; returns 204.
- Auth: internal auth header required.

---

### POST /service_callback.create_service_callback_api

- Creates delivery-status callback; returns 201 `{"data": {id, service_id, url, updated_by_id, created_at, updated_at: null}}`.
- Unknown service → 404 `{"message":"No result found"}`.
- Auth: internal auth header required.

---

### POST /service_callback.update_service_callback_api

- Updates URL or bearer token; returns 200 `{"data": {...}}`.
- URL must be valid HTTPS (not HTTP) → error `"url is not a valid https url"`.
- `bearer_token` minimum 10 characters → error `"bearer_token <value> is too short"`.
- Required: `updated_by_id` (UUID).
- Auth: internal auth header required.

---

### GET /service_callback.fetch_service_callback_api

- Returns `{"data": {...}}`; 200.
- Auth: internal auth header required.

---

### DELETE /service_callback.remove_service_callback_api

- Hard-deletes record; returns 204.
- Auth: internal auth header required.

---

### POST /service_callback.suspend_callback_api

- Toggles `is_suspended`; returns 200. `suspend_unsuspend=true` suspends, `false` unsuspends.
- Required: `suspend_unsuspend` (bool), `updated_by_id`.
- Auth: internal auth header required.

---

### GET /service/`<service_id>`/statistics

- Returns `{"data": {sms: {requested, delivered, failed}, email: {...}, letter: {...}}}`.
- `today_only=True` = today only; `today_only=False` = last 7 days (combining `ft_notification_status` + live table).
- Unknown service → 200 with zeroed stats (no 404).
- Auth: internal auth header required.

---

### GET /service/`<service_id>`/monthly-usage

- Returns `{"data": {"<YYYY-MM>": {sms: {...}, email: {...}, letter: {...}}}}` for a full fiscal year.
- `year` query param required and must be numeric → 400 `{"message":"Year must be a number","result":"error"}` if missing/non-numeric.
- Unknown service → 404.
- Returns all 12 months in the fiscal year even with no data.
- Auth: internal auth header required.

---

### GET /service/`<service_id>`/history

- Returns `{"data": {"service_history": [...], "api_key_history": [...]}}`.
- Auth: internal auth header required.

---

### GET /service/`<service_id>`/organisation

- Returns serialized organisation or empty `{}` if not associated.
- Auth: internal auth header required.

---

### GET /service/live-services

- Returns `{"data": [...]}` for all live (`active=True AND restricted=False AND count_as_live=True`) services.
- Fields: `service_id`, `service_name`, `organisation_name`, `organisation_type`, `live_date`, `contact_name`, `contact_email`, `contact_mobile`, `sms_totals`, `email_totals`, `letter_totals`, `sms_volume_intent`, `email_volume_intent`, `letter_volume_intent`, `consent_to_research`, `free_sms_fragment_limit`.
- Auth: internal auth header required.

---

### GET /service/sensitive-service-ids

- Returns `{"data": ["<uuid>", ...]}` for services with `sensitive_service=True`.
- Returns `{"data": []}` when none exist.
- Auth: internal auth header required.

---

### GET /service/is-name-unique

- Returns `{"result": true/false}`.
- Required: `name`, `service_id`. Missing → 400. Case-insensitive, ignoring punctuation. Excludes own current name.
- Auth: internal auth header required.

---

### GET /service/is-email-from-unique

- Returns `{"result": true/false}`.
- Required: `email_from`, `service_id`. Missing → 400.
- Auth: internal auth header required.

---

### GET /service/`<service_id>`/annual-limit-stats

- Returns `{email_delivered_today, email_failed_today, sms_delivered_today, sms_failed_today, total_email_fiscal_year_to_yesterday, total_sms_fiscal_year_to_yesterday}`.
- Auth: internal auth header required.

---

## DAO Functions

### services_dao.py

- **`dao_fetch_all_services(only_active=False)`** — SELECT all services ordered `created_at ASC`; optional `active=true` filter; eager-loads users. Returns `list[Service]`.
- **`get_services_by_partial_name(service_name)`** — ILIKE `%<escaped>%` search; escapes special LIKE characters. Returns `list[Service]`.
- **`dao_count_live_services()`** — COUNT where `active=True AND restricted=False AND count_as_live=True`. Returns `int`.
- **`dao_fetch_live_services_data(filter_heartbeats=None)`** — aggregate reporting; joins `AnnualBilling`, optionally `FactBilling`, `Organisation`, go-live `User`; post-processes per-type billing totals; ordered `go_live_at ASC`.
- **`dao_fetch_service_by_id(service_id, only_active=False, use_cache=False)`** — single service by PK; optional Redis cache (`service_cache_key`); raises `NoResultFound` on miss.
- **`dao_fetch_service_by_inbound_number(number)`** — resolve phone number to owning service via `InboundNumber.active=true`; returns `None` on miss.
- **`dao_fetch_service_by_id_with_api_keys(service_id, only_active=False)`** — service + api_keys; uses read replica.
- **`dao_fetch_all_services_by_user(user_id, only_active=False)`** — all services the user belongs to; ordered `created_at ASC`.
- **`dao_archive_service(service_id)`** — transactional wrapper (commits); returns original service name. Do NOT call from inside an outer transaction.
- **`dao_archive_service_no_transaction(service_id)`** — core archive: `active=False`, name prefix, `email_from` prefix, mark templates archived, expire API keys without existing expiry; decorated `@version_class(ApiKey, Service, Template/TemplateHistory)`.
- **`dao_fetch_service_by_id_and_user(service_id, user_id)`** — service only if user is a member; raises `NoResultFound`.
- **`dao_create_service(service, user, service_id=None, service_permissions=None, organisation_id=None)`** — full creation; raises `ValueError("Can't create a service without a user")` if user falsy; inserts default permissions and default SMS sender; decorated `@transactional @version_class(Service)`.
- **`dao_update_service(service)`** — persists changes; appends `ServiceHistory` row (version incremented); decorated `@transactional @version_class(Service)`.
- **`dao_add_user_to_service(service, user, permissions, folder_permissions)`** — appends user, sets permissions, validates folder UUIDs; rolls back on exception.
- **`dao_remove_user_from_service(service, user)`** — removes membership, deletes `Permission` rows; rolls back on exception.
- **`dao_suspend_service(service_id, user_id=None)`** — transactional wrapper; delegates to `dao_suspend_service_no_transaction`.
- **`dao_suspend_service_no_transaction(service_id, user_id=None)`** — sets `active=False`, `suspended_at=now(UTC)`, optionally `suspended_by_id`; decorated `@version_class(ApiKey, Service)`.
- **`dao_resume_service(service_id)`** — sets `active=True`, clears `suspended_at`/`suspended_by_id`; decorated `@transactional @version_class(Service)`.
- **`dao_fetch_active_users_for_service(service_id)`** — users with `state='active'` for the service.
- **`dao_fetch_service_creator(service_id)`** — user from version-1 history record via `MIN(version)` subquery.
- **`dao_fetch_service_ids_of_sensitive_services()`** — list of UUIDs where `sensitive_service=True`.
- **`dao_fetch_stats_for_service(service_id, limit_days)`** — grouped `(notification_type, status, count)` past N days; excludes `KEY_TYPE_TEST`.
- **`dao_fetch_todays_stats_for_service(service_id)`** — today only (EST midnight boundary); excludes `KEY_TYPE_TEST`.
- **`fetch_todays_total_message_count(service_id)`** — non-test notifications + scheduled jobs today. Returns `int`.
- **`fetch_todays_total_sms_count(service_id)`** — non-test SMS today. Returns `int`.
- **`fetch_todays_total_sms_billable_units(service_id)`** — SUM billable_units non-test SMS today; `NULL` treated as 0. Returns `int`.
- **`fetch_service_email_limit(service_id)`** — returns `service.message_limit`.
- **`fetch_todays_total_email_count(service_id)`** — non-test emails + scheduled email jobs today.
- **`dao_fetch_todays_stats_for_all_services(include_from_test_key=True, only_active=True)`** — admin view; uses local-timezone midnight; outer JOIN to services.

### service_permissions_dao.py

- **`dao_fetch_service_permissions(service_id)`** — all capability flags for a service.
- **`dao_add_service_permission(service_id, permission)`** — INSERT; `@transactional`; DB unique constraint prevents duplicates.
- **`dao_remove_service_permission(service_id, permission)`** — DELETE; returns count of deleted rows (0 if not found).

### service_safelist_dao.py

- **`dao_fetch_service_safelist(service_id)`** — all safelisted contacts (email/phone) for a service.
- **`dao_add_and_commit_safelisted_contacts(objs)`** — bulk INSERT and commit.
- **`dao_remove_service_safelist(service_id)`** — DELETE all for service; returns count; caller must commit.

### service_sms_sender_dao.py

- **`insert_service_sms_sender(service, sms_sender)`** — initial default sender at service creation; always `is_default=True`; relies on outer transaction.
- **`dao_get_service_sms_senders_by_id(service_id, service_sms_sender_id)`** — non-archived sender by ID (both IDs must match).
- **`dao_get_sms_senders_by_service_id(service_id)`** — all non-archived senders; ordered `is_default DESC`.
- **`dao_add_sms_sender_for_service(service_id, sms_sender, is_default, inbound_number_id=None)`** — INSERT; if `is_default=True` clears old default first; if `is_default=False` and no existing default → raises Exception (**C7 bug**: bare `Exception("...", 400)`); `@transactional`.
- **`dao_update_service_sms_sender(service_id, service_sms_sender_id, is_default, sms_sender=None)`** — UPDATE; if `is_default=True` clears old default; if `is_default=False` and record IS the current only default → raises Exception (**C7 bug**); inbound-number `sms_sender` string is immutable; `@transactional`.
- **`update_existing_sms_sender_with_inbound_number(service_sms_sender, sms_sender, inbound_number_id)`** — binds inbound number to existing sender; `@transactional`.
- **`archive_sms_sender(service_id, sms_sender_id)`** — soft-delete; raises `ArchiveValidationError` if `inbound_number_id` set or `is_default=True`; `@transactional`.

### service_email_reply_to_dao.py

- **`dao_get_reply_to_by_service_id(service_id)`** — non-archived reply-tos; ordered `is_default DESC, created_at DESC`.
- **`dao_get_reply_to_by_id(service_id, reply_to_id)`** — single non-archived; raises `NoResultFound`.
- **`add_reply_to_email_address_for_service(service_id, email_address, is_default)`** — INSERT; if `is_default=True` clears old; if `is_default=False` and no default exists → raises `InvalidRequest("You must have at least one reply to email address as the default.", 400)`; `@transactional`.
- **`update_reply_to_email_address(service_id, reply_to_id, email_address, is_default)`** — UPDATE; same default guards; `@transactional`.
- **`archive_reply_to_email_address(service_id, reply_to_id)`** — `archived=True`; if default AND only non-archived entry → clears `is_default` first, archives; if default AND others exist → raises `ArchiveValidationError("You cannot delete a default email reply to address if other reply to addresses exist")`; `@transactional`.

### service_inbound_api_dao.py

- **`save_service_inbound_api(service_inbound_api)`** — INSERT; sets `id`, `created_at`; `@transactional @version_class(ServiceInboundApi)`.
- **`reset_service_inbound_api(service_inbound_api, updated_by_id, url=None, bearer_token=None)`** — UPDATE; use `!= nil` check (not truthiness — Python has truthiness bug here); `@transactional @version_class(ServiceInboundApi)`.
- **`get_service_inbound_api(service_inbound_api_id, service_id)`** — by id + service_id; returns `None` on miss.
- **`get_service_inbound_api_for_service(service_id)`** — at most one per service; returns `None` on miss.
- **`delete_service_inbound_api(service_inbound_api)`** — hard DELETE; `@transactional`.

### service_callback_api_dao.py

- **`resign_service_callbacks(resign, unsafe=False)`** — re-sign stored bearer tokens after key rotation; dry-run if `resign=False`; `@transactional`.
- **`save_service_callback_api(service_callback_api)`** — INSERT; `@transactional @version_class(ServiceCallbackApi)`.
- **`reset_service_callback_api(service_callback_api, updated_by_id, url=None, bearer_token=None)`** — UPDATE; use `!= nil` check (same truthiness bug as inbound API); `@transactional @version_class(ServiceCallbackApi)`.
- **`get_service_callback_api_with_service_id(service_id)`** — all callbacks for a service.
- **`get_service_callback_api(service_callback_api_id, service_id)`** — by id + service_id.
- **`get_service_delivery_status_callback_api_for_service(service_id)`** — type = `DELIVERY_STATUS_CALLBACK_TYPE`.
- **`get_service_complaint_callback_api_for_service(service_id)`** — type = `COMPLAINT_CALLBACK_TYPE`.
- **`delete_service_callback_api(service_callback_api)`** — hard DELETE; `@transactional`.
- **`suspend_unsuspend_service_callback_api(service_callback_api, updated_by_id, suspend=False)`** — sets `is_suspended`, `suspended_at`, `updated_by_id`, `updated_at`; `@transactional @version_class(ServiceCallbackApi)`.

### service_data_retention_dao.py

- **`fetch_service_data_retention_by_id(service_id, data_retention_id)`** — by service+id; returns `None` on miss.
- **`fetch_service_data_retention(service_id)`** — all policies; ordered by `notification_type`.
- **`fetch_service_data_retention_by_notification_type(service_id, notification_type)`** — lookup by type; returns `None` on miss.
- **`insert_service_data_retention(service_id, notification_type, days_of_retention)`** — INSERT; `@transactional`; DB unique constraint on `(service_id, notification_type)`.
- **`update_service_data_retention(service_data_retention_id, service_id, days_of_retention)`** — UPDATE; sets `updated_at`; returns row count (0 if not found or wrong service ID); `@transactional`.

### service_user_dao.py

- **`dao_get_service_user(user_id, service_id)`** — single membership record; raises `NoResultFound`.
- **`dao_get_active_service_users(service_id)`** — JOIN `User` where `state='active'`.
- **`dao_get_service_users_by_user_id(user_id)`** — all memberships for a user.
- **`dao_update_service_user(service_user)`** — persist changes (e.g. folder permissions); `@transactional`.

### service_letter_contact_dao.py

- **`dao_get_letter_contacts_by_service_id(service_id)`** — non-archived; ordered `is_default DESC, created_at DESC`.
- **`dao_get_letter_contact_by_id(service_id, letter_contact_id)`** — non-archived; raises `NoResultFound`.
- **`add_letter_contact_for_service(service_id, contact_block, is_default)`** — INSERT; if `is_default=True` clears old default; no guard for `is_default=False` (no default required); `@transactional`.
- **`update_letter_contact(service_id, letter_contact_id, contact_block, is_default)`** — UPDATE; clears old default if new `is_default=True`; `@transactional`.
- **`archive_letter_contact(service_id, letter_contact_id)`** — soft-delete `archived=True`; cascades `Template.service_letter_contact_id` to `NULL`; no guard on default; `@transactional`.

---

## Business Rules & Invariants

### Service Lifecycle

| State | `active` | `suspended_at` | Name prefix |
|---|---|---|---|
| Active | `true` | `null` | none |
| Suspended | `false` | set (UTC) | none |
| Archived | `false` | unchanged | `_archived_<YYYY-MM-DD_HH:MM:SS>_` |

- Created with: `active=True`, `research_mode=False`. Default permissions `[sms, email, international_sms]`. One default SMS sender (FROM_NUMBER).
- `count_as_live = not user.platform_admin`. Platform admin services excluded from live metrics.
- "Live" service: `active=True AND restricted=False AND count_as_live=True`.
- Suspension sets `active=False` + records `suspended_at`. API keys NOT expired on suspend.
- Resume clears `active`, `suspended_at`, `suspended_by_id`.
- Archive: name/email_from mangling, all templates and API keys deactivated atomically. Pre-existing revoked/archived records are not re-processed.
- Province/territory services (`organisation_type == "province_or_territory"`): data retention added at creation via `add_pt_data_retention()`.

### API Key Rules

- Secret is hashed before storage; plaintext returned once in create response only.
- `key_name` in response prefixed with `API_KEY_PREFIX` config value.
- Multiple keys per service allowed.
- Revoking: sets `expiry_date = utcnow()`.
- Archive operation expires all keys without an existing expiry date. Pre-revoked keys unchanged.
- Expired keys included in `GET /api-keys` listings.

### Permission Types

- Default: `sms`, `email`, `international_sms`.
- Permissions update on service update is full-replace.
- DB-level unique constraint prevents duplicate `ServicePermission` rows.

### Safelist (Whitelist)

- Stores email addresses and phone numbers permitted for restricted-mode delivery.
- Canonical update: DELETE all then bulk INSERT (replace-all semantics).
- Validation before any DELETE: if any entry is invalid, return 400 and preserve existing safelist.
- Safelist not consulted when service is unrestricted, but records are retained.

### SMS Sender

- Exactly one sender with `is_default=True` at all times.
- Adding new default clears old default.
- Un-defaulting the sole default → HTTP 400 `InvalidRequestError` (C7 fix — Python raises garbled `Exception(msg, 400)`).
- Archiving blocked if `is_default=True` or `inbound_number_id` set.
- Inbound-number senders: `sms_sender` string immutable via normal update path.
- Archived senders excluded from all list queries (`archived=false` filter).

### Email Reply-To

- Zero or more reply-to addresses per service.
- If any exist, exactly one must be `is_default=True`.
- First reply-to must be set as default.
- Archiving default permitted only when it is the last non-archived entry (clears `is_default` first).
- Archiving default while others exist → `ArchiveValidationError`.

### Letter Contact

- At most one `is_default=True` (no default required when none exist).
- Archiving a letter contact cascades `Template.service_letter_contact_id` to NULL.
- No guard prevents archiving the default letter contact.

### Callback API

- Two types: `DELIVERY_STATUS_CALLBACK_TYPE`, `COMPLAINT_CALLBACK_TYPE`.
- At most one of each type per service (application-level constraint; no DB unique constraint).
- Bearer token stored signed (itsdangerous); `_bearer_token` column = signed value; `bearer_token` property = unsigned plaintext.
- Callbacks can be independently suspended (`is_suspended` flag) without affecting the service.
- Key rotation: `resign_service_callbacks(resign=True)`. Dry-run: `resign=False`.
- URL must be HTTPS. Bearer token minimum 10 characters.

### Data Retention

- At most one policy per `(service_id, notification_type)`.
- `notification_type` values: `email`, `sms`, `letter`.
- Only `days_of_retention` is mutable after creation.

### Inbound API

- One webhook config per service.
- URL and bearer token updatable via `reset_service_inbound_api`.
- Go must use `!= nil` checks (not Python's `if url:` truthiness) to permit clearing a field to empty string.

---

## Error Conditions

| Location | Condition | Error |
|---|---|---|
| `dao_create_service` | `user` argument is falsy | `ValueError("Can't create a service without a user")` |
| `POST /service` | `user_id` not in DB | HTTP 404 |
| `POST /service` | duplicate `name` | HTTP 400 `"Duplicate service name '<name>'"` |
| `POST /service` | duplicate `email_from` | HTTP 400 `"Duplicate service name '<email_from>'"` |
| `POST /service/<id>` | invalid `research_mode` | HTTP 400 `"Not a valid boolean."` |
| `POST /service/<id>` | unknown `permissions` entry | HTTP 400 `"Invalid Service Permission: '<value>'"` |
| `POST /service/<id>` | duplicate `permissions` entry | HTTP 400 `"Duplicate Service Permission: ['<value>']"` |
| `POST /service/<id>` | `prefix_sms: null` | HTTP 400 `{"prefix_sms": ["Field may not be null."]}` |
| `POST /service/<id>/users/<uid>` | user already in service | HTTP 409 `"User id: <id> already part of service id: <id>"` |
| `DELETE /service/<id>/users/<uid>` | last remaining user | HTTP 400 `"You cannot remove the only user for a service"` |
| `DELETE /service/<id>/users/<uid>` | last user with manage_settings | HTTP 400 `"SERVICE_NEEDS_USER_W_MANAGE_SETTINGS_PERM"` |
| `DELETE /service/<id>/users/<uid>` | would leave < 2 members | HTTP 400 `"SERVICE_CANNOT_HAVE_LT_2_MEMBERS"` |
| `dao_add_sms_sender_for_service` | `is_default=False` and no existing default | **C7 fix**: HTTP 400 `InvalidRequestError` (Python raises garbled `Exception(msg, 400)`) |
| `dao_update_service_sms_sender` | un-defaulting sole default | **C7 fix**: HTTP 400 `InvalidRequestError` |
| `_get_existing_default` (sms) | more than one default found | `Exception("There should only be one default sms sender for each service. Service {id} has {n}")` |
| `archive_sms_sender` | `inbound_number_id` set | `ArchiveValidationError("You cannot delete an inbound number")` |
| `archive_sms_sender` | `is_default=True` | `ArchiveValidationError("You cannot delete a default sms sender")` |
| `add_reply_to_email_address_for_service` | `is_default=False`, no existing default | `InvalidRequest("You must have at least one reply to email address as the default.", 400)` |
| `update_reply_to_email_address` | setting `is_default=False` on sole default | `InvalidRequest("You must have at least one reply to email address as the default.", 400)` |
| `archive_reply_to_email_address` | default exists while others present | `ArchiveValidationError("You cannot delete a default email reply to address if other reply to addresses exist")` |
| `_get_existing_default` (reply-to) | more than one default found | `Exception("There should only be one default reply to email for each service. Service {id} has {n}")` |
| `_get_existing_default` (letter) | more than one default found | `Exception("There should only be one default letter contact for each service. Service {id} has {n}")` |
| Callback URL validation | non-HTTPS URL | `"url is not a valid https url"` |
| Callback bearer_token | length < 10 | `"bearer_token <value> is too short"` |
| `resign_service_callbacks` | bad signature + `unsafe=False` | `BadSignature` re-raised |
| `PUT /service/<id>/safelist` | invalid email or phone entry | HTTP 400 `"Invalid safelist: \"<val>\" is not a valid email address or phone number"` |
| `POST /service/<id>/data-retention` | invalid `notification_type` | HTTP 400 `"notification_type <value> is not one of [sms, letter, email]"` |
| `POST /service/<id>/data-retention` | duplicate type for service | HTTP 400 `"Service already has data retention for <type> notification type"` |
| `GET /service/find-by-name` | `service_name` param absent | HTTP 400 |
| `GET /service/<id>/monthly-usage` | `year` missing or non-numeric | HTTP 400 `{"message":"Year must be a number","result":"error"}` |
