# Business Rules: Services

## Overview

The `Service` is the central tenant entity in notification-api. Every notification, template, API key, job, and billing record belongs to a service. A service represents a government programme or team that sends notifications via the platform. Services have two operational modes—**restricted** (trial/sandbox) and **live**—and three lifecycle states: active, suspended, and archived.

Sub-entities that belong to a service include: permission flags, a safelist (whitelist) for restricted-mode delivery, one or more SMS senders, email reply-to addresses, letter contacts, callback webhook endpoints, an inbound SMS API endpoint, data retention policies, and a user membership table with folder-level access control.

---

## Data Access Patterns

### `services_dao.py`

#### `dao_fetch_all_services(only_active=False)`
- **Purpose**: Return every service in the system.
- **Query type**: SELECT
- **Key filters/conditions**: Optionally filtered to `Service.active = true`. Ordered by `created_at ASC`. Eager-loads `users`.
- **Returns**: `list[Service]`
- **Notes**: Loads all users via a JOIN to avoid N+1. No pagination.

#### `get_services_by_partial_name(service_name)`
- **Purpose**: Case-insensitive partial name search.
- **Query type**: SELECT
- **Key filters/conditions**: `Service.name ILIKE '%<escaped_name>%'`. Special characters in the input are escaped before the LIKE query to prevent injection via pattern characters.
- **Returns**: `list[Service]`
- **Notes**: Uses `escape_special_characters()` to sanitise input.

#### `dao_count_live_services()`
- **Purpose**: Count services that are publicly visible and active.
- **Query type**: SELECT (COUNT)
- **Key filters/conditions**: `active=True AND restricted=False AND count_as_live=True`.
- **Returns**: `int`

#### `dao_fetch_live_services_data(filter_heartbeats=None)`
- **Purpose**: Aggregate reporting query for all live services including billing and contact information.
- **Query type**: SELECT with aggregation
- **Key filters/conditions**: `count_as_live=True AND active=True AND restricted=False`. Optionally excludes `NOTIFY_SERVICE_ID` (the internal heartbeat service). Joins `AnnualBilling` (most-recent-year subquery), optionally joins `FactBilling` (current financial year), `Organisation`, and the go-live `User`.
- **Returns**: `list[Row]` — each row has `service_id`, `service_name`, `organisation_name`, `organisation_type`, `consent_to_research`, `contact_name`, `contact_email`, `contact_mobile`, `live_date`, `sms_volume_intent`, `email_volume_intent`, `letter_volume_intent`, `email_totals`, `sms_totals`, `letter_totals`, `free_sms_fragment_limit`.
- **Notes**: Post-processes rows in Python to aggregate per-type billing totals into a single dict per service (multiple `FactBilling` rows for different notification types are merged). Ordered by `go_live_at ASC`.

#### `dao_fetch_service_by_id(service_id, only_active=False, use_cache=False)`
- **Purpose**: Fetch a single service by primary key with optional Redis cache.
- **Query type**: SELECT (or cache hit)
- **Key filters/conditions**: `id = service_id`. Optionally `active = true`. Eager-loads `users`.
- **Returns**: `Service` (raises `NoResultFound` if not found)
- **Notes**: When `use_cache=True`, checks Redis key `service_cache_key(service_id)` first. Cache hit deserialises via `Service.from_json()`. Cache miss falls through to DB.

#### `dao_fetch_service_by_inbound_number(number)`
- **Purpose**: Resolve a phone number to its owning service.
- **Query type**: SELECT (two queries)
- **Key filters/conditions**: `InboundNumber.number = number AND InboundNumber.active = true`, then `Service.id = inbound_number.service_id`.
- **Returns**: `Service | None`

#### `dao_fetch_service_by_id_with_api_keys(service_id, only_active=False)`
- **Purpose**: Fetch a service with its API keys, reading from the read replica.
- **Query type**: SELECT (read replica)
- **Key filters/conditions**: `id = service_id`. Optionally `active = true`. Eager-loads `api_keys`.
- **Returns**: `Service` (raises `NoResultFound` if not found)
- **Notes**: Uses `db.on_reader()` — targets the read replica specifically.

#### `dao_fetch_all_services_by_user(user_id, only_active=False)`
- **Purpose**: Return all services a user belongs to.
- **Query type**: SELECT
- **Key filters/conditions**: `Service.users` association contains user with `id = user_id`. Optionally `active = true`. Ordered by `created_at ASC`. Eager-loads `users`.
- **Returns**: `list[Service]`

#### `dao_archive_service(service_id)`
- **Purpose**: Transactional wrapper — archives a service and commits.
- **Query type**: UPDATE (multiple tables via version history)
- **Key filters/conditions**: Delegates to `dao_archive_service_no_transaction`.
- **Returns**: original service name (`str`)
- **Notes**: Do **not** call from inside an outer `db.session.begin()` block; the inner `@transactional` commit would end the outer transaction.

#### `dao_archive_service_no_transaction(service_id)`
- **Purpose**: Core archive logic without committing; for use inside caller-managed transactions.
- **Query type**: UPDATE
- **Key filters/conditions**: Eager-loads `templates`, `templates.template_redacted`, `api_keys`.
- **Returns**: original service name (`str`)
- **Notes**:
  - Sets `service.active = False`.
  - Renames `service.name` → `_archived_<YYYY-MM-DD_HH:MM:SS>_<original_name>`.
  - Renames `service.email_from` → `_archived_<YYYY-MM-DD_HH:MM:SS>_<original_email_from>`.
  - Sets `template.archived = True` for all non-archived templates.
  - Sets `api_key.expiry_date = utcnow()` for all api keys without an existing expiry.
  - Decorated with `@version_class(ApiKey, Service, Template/TemplateHistory)` — history records are written.

#### `dao_fetch_service_by_id_and_user(service_id, user_id)`
- **Purpose**: Fetch a service only if the given user is a member.
- **Query type**: SELECT
- **Key filters/conditions**: `Service.users` contains `user_id` AND `Service.id = service_id`. Eager-loads `users`.
- **Returns**: `Service` (raises `NoResultFound` if user is not a member)

#### `dao_create_service(service, user, service_id=None, service_permissions=None, organisation_id=None)`
- **Purpose**: Create a new service with all required defaults.
- **Query type**: INSERT
- **Key filters/conditions**: N/A (creation)
- **Returns**: `None` (service is added to session)
- **Notes** (all invariants enforced here):
  - Raises `ValueError("Can't create a service without a user")` if `user` is falsy.
  - Default permissions: `[SMS_TYPE, EMAIL_TYPE, INTERNATIONAL_SMS_TYPE]` if not supplied.
  - Organisation resolution: prefers explicit `organisation_id`, falls back to `dao_get_organisation_by_email_address(user.email_address)`.
  - `service.active = True`, `service.research_mode = False`.
  - `service.id` set to `service_id` or a new `uuid4()` — must be assigned before history model creation.
  - A `ServicePermission` row is inserted for each permission.
  - A default `ServiceSmsSender` is inserted using `FROM_NUMBER` config value with `is_default=True`.
  - If organisation found: inherits `organisation_type`, `crown`, and `letter_branding` (if not already set). If `organisation_type == "province_or_territory"`, calls `add_pt_data_retention()`.
  - If no organisation but user email is NHS or `organisation_type` in NHS types: assigns NHS email and letter branding.
  - `crown` derived from `organisation.crown` > `CROWN_ORGANISATION_TYPES` > `NON_CROWN_ORGANISATION_TYPES`.
  - `count_as_live = not user.platform_admin` — platform admins do not count toward live service metrics.
  - Decorated with `@transactional @version_class(Service)`.

#### `dao_update_service(service)`
- **Purpose**: Persist changes to an existing service.
- **Query type**: UPDATE
- **Key filters/conditions**: N/A
- **Returns**: `None`
- **Notes**: Decorated with `@transactional @version_class(Service)` — a history record is written on every call.

#### `dao_add_user_to_service(service, user, permissions=None, folder_permissions=None)`
- **Purpose**: Add a user to a service with optional permissions and template folder access.
- **Query type**: INSERT / UPDATE
- **Key filters/conditions**: N/A
- **Returns**: `None`
- **Notes**: Appends user to `service.users`, calls `permission_dao.set_user_service_permission`, validates requested `folder_permissions` via `dao_get_valid_template_folders_by_id`, assigns them to the `ServiceUser`. Rolls back and re-raises on any exception; commits on success.

#### `dao_remove_user_from_service(service, user)`
- **Purpose**: Remove a user from a service and delete all their permissions.
- **Query type**: DELETE
- **Key filters/conditions**: N/A
- **Returns**: `None`
- **Notes**: Calls `permission_dao.remove_user_service_permissions`, deletes `ServiceUser` row. Rolls back and re-raises on exception; commits on success.

#### `delete_service_and_all_associated_db_objects(service)`
- **Purpose**: Completely purge a service and all related data. For test/administrative use only.
- **Query type**: DELETE (many tables, multiple commits)
- **Key filters/conditions**: All records related to the service by `service_id` or association.
- **Returns**: `None`
- **Notes**: Deletion order matters due to foreign keys. Order: `TemplateRedacted` → `ServiceSmsSender` → `InvitedUser` → `Permission` → `NotificationHistory` → `Notification` → `Job` → `Template` → `TemplateHistory` → `ServicePermission` → `ApiKey` → `ApiKey history` → `AnnualBilling` → `VerifyCode` (via users) → `TemplateCategory` (by user) → service `users` membership → `Service history` → `Service` → `User` records. Each step is a separate `db.session.commit()`.

#### `dao_fetch_stats_for_service(service_id, limit_days)`
- **Purpose**: Notification counts grouped by type and status for the past N days.
- **Query type**: SELECT (aggregation)
- **Key filters/conditions**: `service_id`, `key_type != KEY_TYPE_TEST`, `created_at >= midnight_n_days_ago(limit_days)`. Groups by `notification_type` and `status`.
- **Returns**: `list[Row(notification_type, status, count)]`
- **Notes**: Decorated with `@statsd`. Excludes test-key notifications.

#### `dao_fetch_todays_stats_for_service(service_id)`
- **Purpose**: Notification counts grouped by type and status for today only.
- **Query type**: SELECT (aggregation)
- **Key filters/conditions**: `service_id`, `key_type != KEY_TYPE_TEST`, `date(created_at) = today()`.
- **Returns**: `list[Row(notification_type, status, count)]`
- **Notes**: Decorated with `@statsd`. Uses `date.today()` (UTC wall-clock date).

#### `fetch_todays_total_message_count(service_id)`
- **Purpose**: Combined total of today's sent notifications plus scheduled jobs for today (all types).
- **Query type**: SELECT (two subqueries summed in Python)
- **Key filters/conditions**: Notifications: `service_id`, `key_type != KEY_TYPE_TEST`, `created_at >= midnight UTC`. Scheduled jobs: `service_id`, `job_status = SCHEDULED`, `scheduled_for` within today's midnight window (midnight to midnight+24h).
- **Returns**: `int`

#### `fetch_todays_total_sms_count(service_id)`
- **Purpose**: Count of non-test SMS notifications sent since midnight UTC.
- **Query type**: SELECT (COUNT)
- **Key filters/conditions**: `service_id`, `key_type != KEY_TYPE_TEST`, `created_at > midnight UTC`, `notification_type = 'sms'`.
- **Returns**: `int` (0 if None)

#### `fetch_todays_total_sms_billable_units(service_id)`
- **Purpose**: Sum of billable units for non-test SMS notifications since midnight UTC.
- **Query type**: SELECT (SUM)
- **Key filters/conditions**: Same as `fetch_todays_total_sms_count` but SUM on `billable_units`.
- **Returns**: `int` (0 if None)

#### `fetch_service_email_limit(service_id)`
- **Purpose**: Retrieve the per-day email message limit for a service.
- **Query type**: SELECT
- **Key filters/conditions**: `id = service_id`
- **Returns**: `int` (`service.message_limit`)

#### `fetch_todays_total_email_count(service_id)`
- **Purpose**: Combined total of today's email notifications plus scheduled email jobs for today.
- **Query type**: SELECT (two subqueries summed in Python)
- **Key filters/conditions**: Notifications: `service_id`, `key_type != KEY_TYPE_TEST`, `created_at > midnight UTC`, `notification_type = 'email'`. Scheduled jobs: `service_id`, `job_status = SCHEDULED`, `scheduled_for` between midnight and midnight+23h59m59s.
- **Returns**: `int`

#### `dao_fetch_todays_stats_for_all_services(include_from_test_key=True, only_active=True)`
- **Purpose**: Admin view — notification stats for every service, current day.
- **Query type**: SELECT with subquery and OUTER JOIN
- **Key filters/conditions**: Time window: local-timezone midnight to next midnight (converted to UTC). Optional `key_type != KEY_TYPE_TEST`. Optional `Service.active = true`. Groups notifications by `type`, `status`, `service_id` in subquery, then outer-joins to `Service`.
- **Returns**: `list[Row]` with `service_id`, `name`, `restricted`, `research_mode`, `active`, `created_at`, `notification_type`, `status`, `count`.
- **Notes**: Decorated with `@statsd`. Uses `convert_utc_to_local_timezone` for timezone-aware day boundary.

#### `dao_suspend_service(service_id, user_id=None)`
- **Purpose**: Transactional wrapper — suspend a service and commit.
- **Query type**: UPDATE
- **Key filters/conditions**: Delegates to `dao_suspend_service_no_transaction`.
- **Returns**: `None`
- **Notes**: Same warning as `dao_archive_service` — do not call from inside an outer transaction.

#### `dao_suspend_service_no_transaction(service_id, user_id=None)`
- **Purpose**: Core suspend logic without committing; for use in caller-managed transactions.
- **Query type**: UPDATE
- **Key filters/conditions**: Eager-loads `api_keys`.
- **Returns**: `None`
- **Notes**: Sets `service.active = False`, `service.suspended_at = now(UTC)`. Sets `service.suspended_by_id = user_id` only when `user_id is not None`. Decorated with `@version_class(ApiKey, Service)`.

#### `dao_resume_service(service_id)`
- **Purpose**: Re-activate a suspended service.
- **Query type**: UPDATE
- **Key filters/conditions**: `id = service_id`
- **Returns**: `None`
- **Notes**: Sets `active=True`, `suspended_at=None`, `suspended_by_id=None`. Decorated with `@transactional @version_class(Service)`.

#### `dao_fetch_active_users_for_service(service_id)`
- **Purpose**: Return all active (non-pending/non-locked) users who belong to a service.
- **Query type**: SELECT
- **Key filters/conditions**: `Service.users` contains user with `id = service_id` AND `User.state = 'active'`.
- **Returns**: `list[User]`

#### `dao_fetch_service_creator(service_id)`
- **Purpose**: Identify the user who originally created the service, using audit history.
- **Query type**: SELECT (with MIN subquery on history table)
- **Key filters/conditions**: Finds `min(version)` in `services_history` where `id = service_id`, then joins to `User` on `created_by_id` at that version.
- **Returns**: `User` (raises `NoResultFound` if history is absent)

#### `dao_fetch_service_ids_of_sensitive_services()`
- **Purpose**: Return IDs of all services flagged as sensitive.
- **Query type**: SELECT
- **Key filters/conditions**: `Service.sensitive_service = true`
- **Returns**: `list[str]` (UUIDs as strings)

---

### `service_permissions_dao.py`

#### `dao_fetch_service_permissions(service_id)`
- **Purpose**: Retrieve all capability flags granted to a service.
- **Query type**: SELECT
- **Key filters/conditions**: `service_id = service_id`
- **Returns**: `list[ServicePermission]`

#### `dao_add_service_permission(service_id, permission)`
- **Purpose**: Grant a capability to a service.
- **Query type**: INSERT
- **Key filters/conditions**: N/A
- **Returns**: `None`
- **Notes**: Decorated with `@transactional`. No uniqueness check in the DAO; the DB unique constraint prevents duplicates.

#### `dao_remove_service_permission(service_id, permission)`
- **Purpose**: Revoke a capability from a service.
- **Query type**: DELETE
- **Key filters/conditions**: `service_id = service_id AND permission = permission`
- **Returns**: `int` (count of deleted rows)
- **Notes**: Commits immediately (not wrapped in `@transactional`). Returns 0 if the permission did not exist.

---

### `service_safelist_dao.py`

#### `dao_fetch_service_safelist(service_id)`
- **Purpose**: Retrieve all safelisted contacts (email/phone) for a service.
- **Query type**: SELECT
- **Key filters/conditions**: `service_id = service_id`
- **Returns**: `list[ServiceSafelist]`

#### `dao_add_and_commit_safelisted_contacts(objs)`
- **Purpose**: Bulk-insert safelisted contacts.
- **Query type**: INSERT
- **Key filters/conditions**: N/A
- **Returns**: `None`
- **Notes**: Commits immediately. The caller is responsible for assembling `ServiceSafelist` objects. The typical usage pattern is to call `dao_remove_service_safelist` first (replace-all semantics).

#### `dao_remove_service_safelist(service_id)`
- **Purpose**: Remove all safelisted contacts for a service.
- **Query type**: DELETE
- **Key filters/conditions**: `service_id = service_id`
- **Returns**: `int` (count of deleted rows)
- **Notes**: Not wrapped in `@transactional`; caller must commit. Intended to be called before `dao_add_and_commit_safelisted_contacts` for a full replace.

---

### `service_sms_sender_dao.py`

#### `insert_service_sms_sender(service, sms_sender)`
- **Purpose**: Insert the initial default SMS sender when creating a service. Called from within `dao_create_service`'s transaction.
- **Query type**: INSERT
- **Key filters/conditions**: N/A
- **Returns**: `None`
- **Notes**: Always sets `is_default=True`. Not decorated with `@transactional` — relies on outer transaction.

#### `dao_get_service_sms_senders_by_id(service_id, service_sms_sender_id)`
- **Purpose**: Fetch a specific non-archived SMS sender by its ID.
- **Query type**: SELECT
- **Key filters/conditions**: `id = service_sms_sender_id AND service_id = service_id AND archived = false`
- **Returns**: `ServiceSmsSender` (raises `NoResultFound` or `MultipleResultsFound`)

#### `dao_get_sms_senders_by_service_id(service_id)`
- **Purpose**: List all active SMS senders for a service.
- **Query type**: SELECT
- **Key filters/conditions**: `service_id = service_id AND archived = false`. Ordered by `is_default DESC` (default sender first).
- **Returns**: `list[ServiceSmsSender]`

#### `dao_add_sms_sender_for_service(service_id, sms_sender, is_default, inbound_number_id=None)`
- **Purpose**: Add a new SMS sender to a service.
- **Query type**: INSERT
- **Key filters/conditions**: N/A
- **Returns**: `ServiceSmsSender`
- **Notes**: If `is_default=True`, the existing default sender's `is_default` is cleared first. If `is_default=False`, raises `Exception` if no existing default exists (every service must have exactly one default). Decorated with `@transactional`.

#### `dao_update_service_sms_sender(service_id, service_sms_sender_id, is_default, sms_sender=None)`
- **Purpose**: Update an existing SMS sender's default flag or sender string.
- **Query type**: UPDATE
- **Key filters/conditions**: `id = service_sms_sender_id`
- **Returns**: `ServiceSmsSender`
- **Notes**: If `is_default=True`, clears old default first. If `is_default=False` but the current record is the existing default, raises `Exception("You must have at least one SMS sender as the default")`. Sender string cannot be updated if the record has an `inbound_number_id` (inbound number senders are immutable). Decorated with `@transactional`.
  - **⚠️ Exception syntax bug**: `raise Exception("...")` instead of `raise InvalidRequest("...", 400)` — produces a garbled error response (the exception message is a bare string, not a JSON body with a status code). Go must use the correct error pattern (`status 400` + JSON body).

#### `update_existing_sms_sender_with_inbound_number(service_sms_sender, sms_sender, inbound_number_id)`
- **Purpose**: Bind an inbound number to an existing SMS sender record when inbound SMS is configured.
- **Query type**: UPDATE
- **Key filters/conditions**: N/A (operates on passed object)
- **Returns**: `ServiceSmsSender`
- **Notes**: Used during inbound number assignment flow. Decorated with `@transactional`.

#### `archive_sms_sender(service_id, sms_sender_id)`
- **Purpose**: Soft-delete an SMS sender.
- **Query type**: UPDATE (`archived = true`)
- **Key filters/conditions**: `id = sms_sender_id AND service_id = service_id`
- **Returns**: `ServiceSmsSender`
- **Notes**: Raises `ArchiveValidationError("You cannot delete an inbound number")` if `inbound_number_id` is set. Raises `ArchiveValidationError("You cannot delete a default sms sender")` if `is_default=True`. Decorated with `@transactional`.

---

### `service_email_reply_to_dao.py`

#### `dao_get_reply_to_by_service_id(service_id)`
- **Purpose**: List all active email reply-to addresses for a service.
- **Query type**: SELECT
- **Key filters/conditions**: `service_id = service_id AND archived = false`. Ordered by `is_default DESC`, then `created_at DESC`.
- **Returns**: `list[ServiceEmailReplyTo]`

#### `dao_get_reply_to_by_id(service_id, reply_to_id)`
- **Purpose**: Fetch a specific non-archived reply-to address.
- **Query type**: SELECT
- **Key filters/conditions**: `service_id = service_id AND id = reply_to_id AND archived = false`
- **Returns**: `ServiceEmailReplyTo` (raises `NoResultFound`)

#### `add_reply_to_email_address_for_service(service_id, email_address, is_default)`
- **Purpose**: Add a new reply-to email address.
- **Query type**: INSERT
- **Key filters/conditions**: N/A
- **Returns**: `ServiceEmailReplyTo`
- **Notes**: If `is_default=True`, clears old default first. If `is_default=False` and no existing default, raises `InvalidRequest("You must have at least one reply to email address as the default.", 400)`. Decorated with `@transactional`.

#### `update_reply_to_email_address(service_id, reply_to_id, email_address, is_default)`
- **Purpose**: Update a reply-to address's email or default flag.
- **Query type**: UPDATE
- **Key filters/conditions**: `id = reply_to_id`
- **Returns**: `ServiceEmailReplyTo`
- **Notes**: If `is_default=True`, clears old default. If `is_default=False` and this is the current default, raises `InvalidRequest("You must have at least one reply to email address as the default.", 400)`. Decorated with `@transactional`.

#### `archive_reply_to_email_address(service_id, reply_to_id)`
- **Purpose**: Soft-delete a reply-to address.
- **Query type**: UPDATE (`archived = true`)
- **Key filters/conditions**: `id = reply_to_id AND service_id = service_id`
- **Returns**: `ServiceEmailReplyTo`
- **Notes**: Special case: if this is the default **and** it is the only non-archived reply-to, `is_default` is cleared and the record is still archived (last-one-standing can be deleted). If this is the default and **other** non-archived entries exist, raises `ArchiveValidationError("You cannot delete a default email reply to address if other reply to addresses exist")`. Decorated with `@transactional`.

---

### `service_inbound_api_dao.py`

#### `save_service_inbound_api(service_inbound_api)`
- **Purpose**: Persist a new inbound API configuration (webhook for receiving inbound SMS).
- **Query type**: INSERT
- **Key filters/conditions**: N/A
- **Returns**: `None`
- **Notes**: Sets `id = create_uuid()`, `created_at = utcnow()`. Decorated with `@transactional @version_class(ServiceInboundApi)`.

#### `reset_service_inbound_api(service_inbound_api, updated_by_id, url=None, bearer_token=None)`
- **Purpose**: Update the inbound API URL and/or bearer token.
- **Query type**: UPDATE
- **Key filters/conditions**: N/A (operates on passed object)
- **Returns**: `None`
- **Notes**: Only updates fields that are provided (non-None). Sets `updated_by_id` and `updated_at = utcnow()`. Decorated with `@transactional @version_class(ServiceInboundApi)`.
  - **⚠️ Truthiness bug**: uses `if url:` instead of `if url is not None:` — prevents clearing the URL/token to an empty string. Go must use `!= nil` / `is not None` semantics.

#### `get_service_inbound_api(service_inbound_api_id, service_id)`
- **Purpose**: Fetch a specific inbound API record by both its ID and service ID.
- **Query type**: SELECT
- **Key filters/conditions**: `id = service_inbound_api_id AND service_id = service_id`
- **Returns**: `ServiceInboundApi | None`

#### `get_service_inbound_api_for_service(service_id)`
- **Purpose**: Get the inbound API configuration for a service (at most one per service).
- **Query type**: SELECT
- **Key filters/conditions**: `service_id = service_id`
- **Returns**: `ServiceInboundApi | None`

#### `delete_service_inbound_api(service_inbound_api)`
- **Purpose**: Hard-delete the inbound API configuration.
- **Query type**: DELETE
- **Key filters/conditions**: N/A (operates on passed object)
- **Returns**: `None`
- **Notes**: Decorated with `@transactional`.

---

### `service_callback_api_dao.py`

#### `resign_service_callbacks(resign, unsafe=False)`
- **Purpose**: Re-sign the `_bearer_token` column for all callback rows after a key rotation.
- **Query type**: SELECT all, then conditional UPDATE (bulk save)
- **Key filters/conditions**: All `ServiceCallbackApi` rows with a non-null `_bearer_token`.
- **Returns**: `None`
- **Notes**: If `resign=False`, performs a dry-run and logs the count of rows needing re-signing without writing. If `resign=True`, calls `db.session.bulk_save_objects`. If a `BadSignature` is encountered: raises unless `unsafe=True`, in which case `signer_bearer_token.verify_unsafe()` is used. Decorated with `@transactional`.

#### `save_service_callback_api(service_callback_api)`
- **Purpose**: Create a new callback webhook configuration.
- **Query type**: INSERT
- **Key filters/conditions**: N/A
- **Returns**: `None`
- **Notes**: Sets `id = create_uuid()`, `created_at = utcnow()`. Decorated with `@transactional @version_class(ServiceCallbackApi)`.

#### `reset_service_callback_api(service_callback_api, updated_by_id, url=None, bearer_token=None)`
- **Purpose**: Update callback webhook URL and/or bearer token.
- **Query type**: UPDATE
- **Key filters/conditions**: N/A (operates on passed object)
- **Returns**: `None`
- **Notes**: Only updates provided (non-None) fields. Sets `updated_by_id`, `updated_at = utcnow()`. Decorated with `@transactional @version_class(ServiceCallbackApi)`.
  - **⚠️ Truthiness bug**: uses `if url:` instead of `if url is not None:` — prevents clearing the URL/token to an empty string (same issue as `reset_service_inbound_api`). Go must use `!= nil` / `is not None` semantics.

#### `get_service_callback_api_with_service_id(service_id)`
- **Purpose**: List all callback configurations for a service.
- **Query type**: SELECT
- **Key filters/conditions**: `service_id = service_id`
- **Returns**: `list[ServiceCallbackApi]`
- **Notes**: Comment in code notes only one callback is expected per service per type.

#### `get_service_callback_api(service_callback_api_id, service_id)`
- **Purpose**: Fetch a specific callback by ID and service.
- **Query type**: SELECT
- **Key filters/conditions**: `id = service_callback_api_id AND service_id = service_id`
- **Returns**: `ServiceCallbackApi | None`

#### `get_service_delivery_status_callback_api_for_service(service_id)`
- **Purpose**: Get the delivery-status callback for a service.
- **Query type**: SELECT
- **Key filters/conditions**: `service_id = service_id AND callback_type = DELIVERY_STATUS_CALLBACK_TYPE`
- **Returns**: `ServiceCallbackApi | None`

#### `get_service_complaint_callback_api_for_service(service_id)`
- **Purpose**: Get the complaint callback for a service.
- **Query type**: SELECT
- **Key filters/conditions**: `service_id = service_id AND callback_type = COMPLAINT_CALLBACK_TYPE`
- **Returns**: `ServiceCallbackApi | None`

#### `delete_service_callback_api(service_callback_api)`
- **Purpose**: Hard-delete a callback configuration.
- **Query type**: DELETE
- **Key filters/conditions**: N/A (operates on passed object)
- **Returns**: `None`
- **Notes**: Decorated with `@transactional`.

#### `suspend_unsuspend_service_callback_api(service_callback_api, updated_by_id, suspend=False)`
- **Purpose**: Temporarily suspend or re-enable a callback webhook.
- **Query type**: UPDATE
- **Key filters/conditions**: N/A (operates on passed object)
- **Returns**: `None`
- **Notes**: Sets `is_suspended = suspend`, `suspended_at = now(UTC)`, `updated_by_id`, `updated_at = now(UTC)`. Decorated with `@transactional @version_class(ServiceCallbackApi)`.

---

### `service_data_retention_dao.py`

#### `fetch_service_data_retention_by_id(service_id, data_retention_id)`
- **Purpose**: Fetch a specific data retention record.
- **Query type**: SELECT
- **Key filters/conditions**: `service_id = service_id AND id = data_retention_id`
- **Returns**: `ServiceDataRetention | None`

#### `fetch_service_data_retention(service_id)`
- **Purpose**: List all data retention policies for a service.
- **Query type**: SELECT
- **Key filters/conditions**: `service_id = service_id`. Ordered by `notification_type` (alphabetical: email, sms, letter).
- **Returns**: `list[ServiceDataRetention]`

#### `fetch_service_data_retention_by_notification_type(service_id, notification_type)`
- **Purpose**: Look up the retention policy for a specific notification type.
- **Query type**: SELECT
- **Key filters/conditions**: `service_id = service_id AND notification_type = notification_type`
- **Returns**: `ServiceDataRetention | None`

#### `insert_service_data_retention(service_id, notification_type, days_of_retention)`
- **Purpose**: Create a new data retention policy for a service/type pair.
- **Query type**: INSERT
- **Key filters/conditions**: N/A
- **Returns**: `ServiceDataRetention`
- **Notes**: Decorated with `@transactional`. No uniqueness check in the DAO; the DB unique constraint on `(service_id, notification_type)` prevents duplicates.

#### `update_service_data_retention(service_data_retention_id, service_id, days_of_retention)`
- **Purpose**: Change the retention duration for an existing policy.
- **Query type**: UPDATE
- **Key filters/conditions**: `id = service_data_retention_id AND service_id = service_id`
- **Returns**: `int` (number of updated rows, 0 or 1)
- **Notes**: Also sets `updated_at = utcnow()`. Decorated with `@transactional`.

---

### `service_user_dao.py`

#### `dao_get_service_user(user_id, service_id)`
- **Purpose**: Fetch the join-table record for a user–service membership.
- **Query type**: SELECT
- **Key filters/conditions**: `user_id = user_id AND service_id = service_id`
- **Returns**: `ServiceUser` (raises `NoResultFound` if the user is not a member)

#### `dao_get_active_service_users(service_id)`
- **Purpose**: List all active members of a service.
- **Query type**: SELECT with JOIN
- **Key filters/conditions**: `ServiceUser.service_id = service_id` JOIN `User` ON `User.id = ServiceUser.user_id` WHERE `User.state = 'active'`.
- **Returns**: `list[ServiceUser]`

#### `dao_get_service_users_by_user_id(user_id)`
- **Purpose**: List all services a user belongs to (as membership records).
- **Query type**: SELECT
- **Key filters/conditions**: `user_id = user_id`
- **Returns**: `list[ServiceUser]`

#### `dao_update_service_user(service_user)`
- **Purpose**: Persist changes to a `ServiceUser` record (e.g., folder permissions).
- **Query type**: UPDATE
- **Key filters/conditions**: N/A
- **Returns**: `None`
- **Notes**: Decorated with `@transactional`.

---

### `service_letter_contact_dao.py`

#### `dao_get_letter_contacts_by_service_id(service_id)`
- **Purpose**: List all active letter contact blocks for a service.
- **Query type**: SELECT
- **Key filters/conditions**: `service_id = service_id AND archived = false`. Ordered by `is_default DESC`, then `created_at DESC`.
- **Returns**: `list[ServiceLetterContact]`

#### `dao_get_letter_contact_by_id(service_id, letter_contact_id)`
- **Purpose**: Fetch a specific non-archived letter contact.
- **Query type**: SELECT
- **Key filters/conditions**: `service_id = service_id AND id = letter_contact_id AND archived = false`
- **Returns**: `ServiceLetterContact` (raises `NoResultFound`)

#### `add_letter_contact_for_service(service_id, contact_block, is_default)`
- **Purpose**: Create a new letter contact (postal address block for outbound letters).
- **Query type**: INSERT
- **Key filters/conditions**: N/A
- **Returns**: `ServiceLetterContact`
- **Notes**: If `is_default=True`, clears old default. If `is_default=False`, no default guard is applied (letter contacts do not require a default to exist). Decorated with `@transactional`.

#### `update_letter_contact(service_id, letter_contact_id, contact_block, is_default)`
- **Purpose**: Update a letter contact's content or default flag.
- **Query type**: UPDATE
- **Key filters/conditions**: `id = letter_contact_id`
- **Returns**: `ServiceLetterContact`
- **Notes**: If `is_default=True`, clears old default first. Decorated with `@transactional`.

#### `archive_letter_contact(service_id, letter_contact_id)`
- **Purpose**: Soft-delete a letter contact.
- **Query type**: UPDATE (two tables)
- **Key filters/conditions**: `id = letter_contact_id AND service_id = service_id`
- **Returns**: `ServiceLetterContact`
- **Notes**: **Cascades** `Template.service_letter_contact_id` to `NULL` for all templates referencing this contact. Sets `archived = True`. No guard prevents archiving the default (unlike reply-to and SMS sender). Decorated with `@transactional`.

---

## Domain Rules & Invariants

### Service Lifecycle

| State | `active` | `suspended_at` | Name prefix | Notes |
|-------|----------|----------------|-------------|-------|
| Active | `true` | `null` | none | Normal operating state |
| Suspended | `false` | set (UTC) | none | API keys kept; `suspended_by_id` set when triggered by a user |
| Archived | `false` | unchanged | `_archived_<timestamp>_` | Name and email_from prefixed; all templates archived; all API keys expired |

- **Creation**: always starts `active=True`, `research_mode=False`. Default permissions `[sms, email, international_sms]` applied. One default SMS sender inserted.
- **Suspension**: sets `active=False` and records timestamp. Callers may supply `user_id` to record who suspended the service. API keys are **not** expired on suspension (only on archive).
- **Resume**: clears `active`, `suspended_at`, `suspended_by_id` back to live state.
- **Archive**: soft-delete with name mangling. All templates and API keys are deactivated atomically. Name and email_from are prefixed with `_archived_<YYYY-MM-DD_HH:MM:SS>_` to free the original values for reuse.
- **`count_as_live`**: set to `not user.platform_admin` at creation; platform admin test services are excluded from live metrics.
- **`restricted`** flag gates trial mode; `count_as_live=True AND active=True AND restricted=False` defines a "live" service.

### Permission Types

Default on creation: `sms`, `email`, `international_sms`.

Additional permissions can be granted/revoked individually at any time. Permissions gate which notification channels a service is allowed to use. The `ServicePermission` table is a simple set; duplicate inserts are prevented at the DB level.

### Safelist (Whitelist)

The safelist stores email addresses and phone numbers that are permitted to receive notifications from a service **while it is in restricted / trial mode**. The canonical update pattern is full-replace:

1. `dao_remove_service_safelist(service_id)` — deletes all current entries (caller must commit after `dao_add_and_commit_safelisted_contacts`)
2. `dao_add_and_commit_safelisted_contacts(objs)` — bulk-inserts new entries and commits

When a service moves to live (unrestricted), the safelist is no longer consulted for delivery decisions, but existing records are kept.

### SMS Sender

- Every service must have **exactly one** sender with `is_default=True` at all times.
- On creation, one default sender is created using the platform `FROM_NUMBER`.
- Adding a new default resets the old default to `false`.
- Setting a sender to non-default fails if it is currently the only default.
- Archiving (soft-delete) is blocked if the sender `is_default=True` or has an `inbound_number_id`.
- Senders linked to an inbound number (`inbound_number_id` set) cannot have their `sms_sender` string updated via the normal update path; they must be updated via `update_existing_sms_sender_with_inbound_number`.
- Archived senders are excluded from all list queries (`archived=false` filter).

### Email Reply-To

- A service can have zero or more reply-to addresses.
- If any reply-to addresses exist, exactly one must be `is_default=True`.
- When the first reply-to is added, it must be marked default.
- Archiving the default is only permitted when it is the **last** non-archived entry; in that case `is_default` is cleared first, then it is archived.
- Archiving a default while others exist raises `ArchiveValidationError`.

### Letter Contact

- Stores postal address blocks used in outbound letter headers.
- At most one can be `is_default=True` (no default is required if no contacts exist).
- Archiving a letter contact automatically NULLs out `Template.service_letter_contact_id` for all templates referencing it.
- No guard prevents archiving the default letter contact (unlike reply-to and SMS sender).

### Callback API

- Two callback types: `DELIVERY_STATUS_CALLBACK_TYPE` and `COMPLAINT_CALLBACK_TYPE`.
- At most one of each type is expected per service (enforced by application logic, not a DB unique constraint).
- Bearer tokens are **signed** using `signer_bearer_token` before storage. The `_bearer_token` column stores the signed value; reading via the `bearer_token` property unsigns it.
- Key rotation is handled via `resign_service_callbacks(resign=True)`. A dry-run mode (`resign=False`) reports how many rows need re-signing without writing.
- Callbacks can be **suspended** independently of the service (operator mechanism, not user-facing archival).

### Data Retention

- Configures how long notification records are kept for each `notification_type` (`email`, `sms`, `letter`).
- A service can have at most one retention policy per notification type (unique constraint on `(service_id, notification_type)`).
- `days_of_retention` is the only mutable field after creation.
- Province/territory services (`organisation_type == "province_or_territory"`) have data retention added automatically at service creation via `add_pt_data_retention`.

### Inbound API

- One inbound API configuration per service (a webhook URL + bearer token for receiving inbound SMS).
- Gated by the presence of an `InboundNumber` linked to the service.
- Subject to versioning (`@version_class(ServiceInboundApi)`) — history is maintained.
- Deletion is a hard delete.

---

## Error Conditions

| Location | Condition | Exception |
|---|---|---|
| `dao_create_service` | `user` argument is falsy | `ValueError("Can't create a service without a user")` |
| `dao_add_sms_sender_for_service` | `is_default=False` and no existing default | `Exception("You must have at least one SMS sender as the default.", 400)` |
| `dao_update_service_sms_sender` | removing default from the only default sender | `Exception("You must have at least one SMS sender as the default")` |
| `_get_existing_default` (sms_sender) | more than one default sms sender found | `Exception("There should only be one default sms sender for each service. Service {id} has {n}")` |
| `archive_sms_sender` | sender has `inbound_number_id` set | `ArchiveValidationError("You cannot delete an inbound number")` |
| `archive_sms_sender` | sender `is_default=True` | `ArchiveValidationError("You cannot delete a default sms sender")` |
| `add_reply_to_email_address_for_service` | `is_default=False` and no existing default | `InvalidRequest("You must have at least one reply to email address as the default.", 400)` |
| `update_reply_to_email_address` | setting `is_default=False` on current default | `InvalidRequest("You must have at least one reply to email address as the default.", 400)` |
| `archive_reply_to_email_address` | archiving default when other non-archived entries exist | `ArchiveValidationError("You cannot delete a default email reply to address if other reply to addresses exist")` |
| `_get_existing_default` (email reply-to) | more than one default reply-to found | `Exception("There should only be one default reply to email for each service. Service {id} has {n}")` |
| `_get_existing_default` (letter contact) | more than one default letter contact found | `Exception("There should only be one default letter contact for each service. Service {id} has {n}")` |
| `resign_service_callbacks` | bad signature and `unsafe=False` | `BadSignature` (re-raised from itsdangerous) |
| `dao_add_user_to_service` | any DB exception | exception re-raised after `db.session.rollback()` |
| `dao_remove_user_from_service` | any DB exception | exception re-raised after `db.session.rollback()` |

---

## Query Inventory (for sqlc)

| Query name | Type | Tables | Description |
|---|---|---|---|
| `FetchAllServices` | SELECT | `services` | All services, ordered by `created_at`; optional `active` filter |
| `GetServicesByPartialName` | SELECT | `services` | ILIKE name search |
| `CountLiveServices` | SELECT | `services` | COUNT where `active=true, restricted=false, count_as_live=true` |
| `FetchLiveServicesData` | SELECT | `services, annual_billing, fact_billing, organisations, users` | Aggregate live service reporting |
| `FetchServiceByID` | SELECT | `services` | Single service by PK |
| `FetchServiceByInboundNumber` | SELECT | `inbound_numbers, services` | Resolve number → service |
| `FetchServiceWithApiKeys` | SELECT | `services, api_keys` | Service + api_keys (read replica) |
| `FetchServicesByUserID` | SELECT | `services, user_to_service` | All services for a user |
| `ArchiveService` | UPDATE | `services, services_history, templates, templates_history, api_keys, api_keys_history` | Set active=false, name/email prefix, archive templates, expire api keys |
| `FetchServiceByIDAndUser` | SELECT | `services, user_to_service` | Service only if user is a member |
| `CreateService` | INSERT | `services, services_history, service_permissions, service_sms_senders` | New service with defaults |
| `UpdateService` | UPDATE | `services, services_history` | Generic service update |
| `AddUserToService` | INSERT | `user_to_service, permissions` | Add user membership and permissions |
| `RemoveUserFromService` | DELETE | `user_to_service, permissions` | Remove user membership and permissions |
| `DeleteServiceAndAllObjects` | DELETE | multiple | Full cascade purge (test/admin only) |
| `FetchStatsForService` | SELECT | `notifications` | Type/status counts for past N days |
| `FetchTodaysStatsForService` | SELECT | `notifications` | Type/status counts today |
| `FetchTodaysTotalMessageCount` | SELECT | `notifications, jobs` | Total non-test notifications + scheduled jobs today |
| `FetchTodaysTotalSmsCount` | SELECT | `notifications` | Count of non-test SMS today |
| `FetchTodaysTotalSmsBillableUnits` | SELECT | `notifications` | Sum billable_units for SMS today |
| `FetchServiceEmailLimit` | SELECT | `services` | message_limit for a service |
| `FetchTodaysTotalEmailCount` | SELECT | `notifications, jobs` | Count of non-test emails + scheduled email jobs today |
| `FetchTodaysStatsForAllServices` | SELECT | `services, notifications` | Per-service type/status counts, current day |
| `SuspendService` | UPDATE | `services, services_history, api_keys, api_keys_history` | Set active=false, suspended_at, suspended_by_id |
| `ResumeService` | UPDATE | `services, services_history` | Set active=true, clear suspension fields |
| `FetchActiveUsersForService` | SELECT | `users, user_to_service` | Active users of a service |
| `FetchServiceCreator` | SELECT | `services_history, users` | User who created version 1 of the service |
| `FetchSensitiveServiceIDs` | SELECT | `services` | IDs where sensitive_service=true |
| `FetchServicePermissions` | SELECT | `service_permissions` | All permissions for a service |
| `AddServicePermission` | INSERT | `service_permissions` | Grant a capability |
| `RemoveServicePermission` | DELETE | `service_permissions` | Revoke a capability |
| `FetchServiceSafelist` | SELECT | `service_safelist` | All safelisted contacts |
| `AddSafelistedContacts` | INSERT | `service_safelist` | Bulk insert safelisted contacts |
| `RemoveServiceSafelist` | DELETE | `service_safelist` | Delete all safelisted contacts for a service |
| `InsertServiceSmsSender` | INSERT | `service_sms_senders` | Create initial default sender |
| `GetSmsSenderByID` | SELECT | `service_sms_senders` | Single non-archived sender |
| `GetSmsSendersByServiceID` | SELECT | `service_sms_senders` | All non-archived senders, default first |
| `AddSmsSenderForService` | INSERT | `service_sms_senders` | New sender with default management |
| `UpdateServiceSmsSender` | UPDATE | `service_sms_senders` | Update sender default/string |
| `UpdateSmsSenderWithInboundNumber` | UPDATE | `service_sms_senders` | Bind inbound number to sender |
| `ArchiveSmsSender` | UPDATE | `service_sms_senders` | Soft-delete |
| `GetReplyTosByServiceID` | SELECT | `service_email_reply_to` | Non-archived reply-tos, default first |
| `GetReplyToByID` | SELECT | `service_email_reply_to` | Single non-archived reply-to |
| `AddReplyToEmailAddress` | INSERT | `service_email_reply_to` | New reply-to with default management |
| `UpdateReplyToEmailAddress` | UPDATE | `service_email_reply_to` | Update address or default flag |
| `ArchiveReplyToEmailAddress` | UPDATE | `service_email_reply_to` | Soft-delete |
| `SaveServiceInboundApi` | INSERT | `service_inbound_api, service_inbound_api_history` | New inbound webhook config |
| `ResetServiceInboundApi` | UPDATE | `service_inbound_api, service_inbound_api_history` | Update URL/token |
| `GetServiceInboundApi` | SELECT | `service_inbound_api` | By id + service_id |
| `GetServiceInboundApiForService` | SELECT | `service_inbound_api` | By service_id |
| `DeleteServiceInboundApi` | DELETE | `service_inbound_api` | Hard delete |
| `SaveServiceCallbackApi` | INSERT | `service_callbacks, service_callbacks_history` | New callback config |
| `ResetServiceCallbackApi` | UPDATE | `service_callbacks, service_callbacks_history` | Update URL/token |
| `GetCallbacksByServiceID` | SELECT | `service_callbacks` | All callbacks for a service |
| `GetServiceCallbackApi` | SELECT | `service_callbacks` | By id + service_id |
| `GetDeliveryStatusCallbackForService` | SELECT | `service_callbacks` | delivery_status type |
| `GetComplaintCallbackForService` | SELECT | `service_callbacks` | complaint type |
| `DeleteServiceCallbackApi` | DELETE | `service_callbacks` | Hard delete |
| `SuspendUnsuspendCallbackApi` | UPDATE | `service_callbacks, service_callbacks_history` | Toggle is_suspended |
| `ResignServiceCallbacks` | UPDATE | `service_callbacks` | Re-sign bearer tokens |
| `FetchServiceDataRetentionByID` | SELECT | `service_data_retention` | By service_id + id |
| `FetchServiceDataRetention` | SELECT | `service_data_retention` | All for a service |
| `FetchDataRetentionByNotificationType` | SELECT | `service_data_retention` | By service_id + type |
| `InsertServiceDataRetention` | INSERT | `service_data_retention` | New retention policy |
| `UpdateServiceDataRetention` | UPDATE | `service_data_retention` | Change days_of_retention |
| `GetServiceUser` | SELECT | `user_to_service` | Single membership record |
| `GetActiveServiceUsers` | SELECT | `user_to_service, users` | Active members |
| `GetServiceUsersByUserID` | SELECT | `user_to_service` | All services for a user |
| `UpdateServiceUser` | UPDATE | `user_to_service` | Save service user (folder permissions etc.) |
| `GetLetterContactsByServiceID` | SELECT | `service_letter_contacts` | Non-archived, default first |
| `GetLetterContactByID` | SELECT | `service_letter_contacts` | Single non-archived contact |
| `AddLetterContactForService` | INSERT | `service_letter_contacts` | New contact with default management |
| `UpdateLetterContact` | UPDATE | `service_letter_contacts` | Update block or default flag |
| `ArchiveLetterContact` | UPDATE | `service_letter_contacts, templates` | Soft-delete, NULL template references |
