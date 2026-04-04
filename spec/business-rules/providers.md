# Business Rules: Providers

## Overview

The providers domain manages the external delivery vendors used to send notifications. There are three provider identifiers:

| Identifier | Type  | Service          |
|------------|-------|------------------|
| `ses`      | email | AWS SES          |
| `sns`      | sms   | AWS SNS          |
| `pinpoint` | sms   | AWS Pinpoint     |

Each provider record carries a numeric `priority` (lower = higher priority), an `active` flag, and a `supports_international` flag. A full change-history is maintained in `provider_details_history`. Provider selection for each send is done at call time via `provider_to_use()` in `app/delivery/send_to_providers.py`.

---

## Data Access Patterns

### `provider_details_dao.py`

#### `get_provider_details_by_id(provider_details_id)`
- **Purpose**: Fetch a single provider record by UUID primary key.
- **Query type**: `SELECT` by primary key.
- **Key filters/conditions**: `id = provider_details_id`.
- **Returns**: `ProviderDetails` instance or `None`.
- **Notes**: Used by REST layer for updates and point lookups.

#### `get_provider_details_by_identifier(identifier)`
- **Purpose**: Fetch a single provider record by string identifier (e.g. `"sns"`, `"ses"`, `"pinpoint"`).
- **Query type**: `SELECT` with exact match; raises `NoResultFound` if absent.
- **Key filters/conditions**: `identifier = identifier`.
- **Returns**: `ProviderDetails` instance.
- **Notes**: Used throughout DAO and delivery layers to resolve provider names.

#### `get_alternative_sms_provider(identifier)`
- **Purpose**: Intended to return a failover SMS provider. Currently **not implemented** — returns the same provider.
- **Query type**: Delegates to `get_provider_details_by_identifier`.
- **Key filters/conditions**: Same as above.
- **Returns**: `ProviderDetails` instance (same provider as input).
- **Notes**: Code comment explicitly states: "We currently run with a single SMS provider (SNS) so this method is not implemented and does not switch providers."

#### `get_current_provider(notification_type)`
- **Purpose**: Return the highest-priority active provider for a given notification type.
- **Query type**: `SELECT` with filter and `ORDER BY priority ASC LIMIT 1`.
- **Key filters/conditions**: `notification_type = notification_type`, `active = True`.
- **Returns**: `ProviderDetails` instance with the lowest `priority` value, or `None`.
- **Notes**: Used by `dao_switch_sms_provider_to_provider_with_identifier` to find who is currently primary.

#### `dao_get_provider_versions(provider_id)`
- **Purpose**: Return the complete audit history for a provider.
- **Query type**: `SELECT` ordered descending.
- **Key filters/conditions**: `provider_details_history.id = provider_id`.
- **Returns**: List of `ProviderDetailsHistory` instances, newest first.
- **Notes**: Exposed directly by REST endpoint `GET /provider-details/<id>/versions`.

#### `dao_toggle_sms_provider(identifier)`
- **Purpose**: Toggle the current SMS provider away from `identifier` to the alternate.
- **Query type**: Transactional write (delegates).
- **Key filters/conditions**: Gets alternative provider via `get_alternative_sms_provider`, then calls `dao_switch_sms_provider_to_provider_with_identifier`.
- **Returns**: `None`.
- **Notes**: Because `get_alternative_sms_provider` currently returns the same provider, this is effectively a **no-op** in the current deployment. Called on any general SMS send failure (non-Pinpoint).

#### `dao_switch_sms_provider_to_provider_with_identifier(identifier)`
- **Purpose**: Make the provider identified by `identifier` the primary SMS provider by adjusting priorities.
- **Query type**: Transactional update.
- **Key filters/conditions**:
  1. Aborts if target provider is inactive.
  2. If another active SMS provider exists with the same priority as the target, calls `switch_providers(conflicting, target)` directly.
  3. Otherwise calls `switch_providers(current_primary, target)` if target is not already primary, then persists both via `dao_update_provider_details`.
- **Returns**: `None`.
- **Notes**: Priority swap logic lives in `switch_providers` in `app/provider_details/switch_providers.py`.

#### `get_provider_details_by_notification_type(notification_type, supports_international=False)`
- **Purpose**: Return all providers for a notification type, optionally filtered to international-capable ones.
- **Query type**: `SELECT` with optional filter, ordered ascending.
- **Key filters/conditions**: `notification_type = notification_type`; if `supports_international=True` adds `supports_international = True`.
- **Returns**: Ordered list of `ProviderDetails` instances (ascending `priority`).
- **Notes**: Called by `provider_to_use()` to get the candidate list; the delivery layer then additionally filters by `active` and excludes either `pinpoint` or `sns` depending on routing logic.

#### `dao_update_provider_details(provider_details)`
- **Purpose**: Persist a mutated `ProviderDetails` instance and append a history row.
- **Query type**: Transactional `UPDATE` + `INSERT` into `provider_details_history`.
- **Key filters/conditions**: Increments `version`, sets `updated_at = utcnow()`, creates a `ProviderDetailsHistory` snapshot.
- **Returns**: `None` (side-effect only).
- **Notes**: Every modification to provider metadata must go through this function to maintain audit history.

#### `dao_get_sms_provider_with_equal_priority(identifier, priority)`
- **Purpose**: Find any *other* active SMS provider that has the exact same priority as the target.
- **Query type**: `SELECT` with three filter conditions, `ORDER BY priority ASC LIMIT 1`.
- **Key filters/conditions**: `identifier != identifier`, `notification_type = "sms"`, `priority = priority`, `active = True`.
- **Returns**: `ProviderDetails` instance or `None`.
- **Notes**: Used inside `dao_switch_sms_provider_to_provider_with_identifier` to detect priority conflicts before a provider swap.

#### `dao_get_provider_stats()`
- **Purpose**: Return all providers annotated with their current-month billable SMS unit count for the admin dashboard.
- **Query type**: `SELECT` with `LEFT OUTER JOIN` subquery and `LEFT OUTER JOIN` to `users`.
- **Key filters/conditions**:
  - Subquery aggregates `fact_billing` rows where `notification_type = 'sms'` and `bst_date >= first_day_of_current_month`, summing `billable_units * rate_multiplier` grouped by `provider`.
  - Main query joins `provider_details` to subquery on `identifier = provider`, and to `users` on `created_by_id`.
  - Ordered by `notification_type`, then `priority`.
- **Returns**: List of row-tuples with columns: `id`, `display_name`, `identifier`, `priority`, `notification_type`, `active`, `updated_at`, `supports_international`, `created_by_name`, `current_month_billable_sms` (defaults to 0 via `COALESCE`).
- **Notes**: The current-day partial data is excluded because the `fact_billing` population job runs overnight.

---

### `provider_rates_dao.py`

#### `create_provider_rates(provider_identifier, valid_from, rate)`
- **Purpose**: Record a new billing rate for a provider, effective from a given datetime.
- **Query type**: Transactional `INSERT` into `provider_rates`.
- **Key filters/conditions**: Looks up provider by `identifier` first (raises `NoResultFound` if missing); creates `ProviderRates(provider_id, valid_from, rate)`.
- **Returns**: `None`.
- **Notes**: Rate history is append-only; there is no update or delete path. The valid rate for a given notification is determined by querying the most recent rate with `valid_from <= notification.created_at`.

---

## Domain Rules & Invariants

### Provider Types

| Identifier | `notification_type` | `supports_international` default |
|------------|---------------------|----------------------------------|
| `ses`      | `email`             | `False`                          |
| `sns`      | `sms`               | Configurable                     |
| `pinpoint` | `sms`               | Configurable                     |

- `SMS_PROVIDERS = ["sns", "pinpoint"]`
- `EMAIL_PROVIDERS = ["ses"]`
- `PROVIDERS = SMS_PROVIDERS + EMAIL_PROVIDERS`

### Provider Selection (`provider_to_use`)

Called at send time for both SMS and email. For email, provider selection is trivial (only SES exists). For SMS the logic is:

**Step 1 – Classify the recipient and sender:**

| Flag | Condition |
|------|-----------|
| `has_dedicated_number` | `sender` is not `None` and starts with `"+1"` |
| `sending_to_us_number` | Parsed phone region code is `"US"` |
| `recipient_outside_canada` | Parsed phone region is not `"CA"` and not `"US"` |
| `cannot_determine_recipient_country` | `phonenumbers.PhoneNumberMatcher` returns no match |
| `using_sc_pool_template` | `template_id` is in `config.AWS_PINPOINT_SC_TEMPLATE_IDS` |
| `zone_1_outside_canada` | `recipient_outside_canada AND NOT international` |

**Step 2 – Decide whether to exclude Pinpoint:**

`do_not_use_pinpoint` is `True` if **any** of the following hold:
1. `has_dedicated_number AND NOT config.FF_USE_PINPOINT_FOR_DEDICATED`
2. `sending_to_us_number`
3. `cannot_determine_recipient_country`
4. `zone_1_outside_canada`
5. `config.AWS_PINPOINT_SC_POOL_ID` is empty/falsy
6. `config.AWS_PINPOINT_DEFAULT_POOL_ID` is empty/falsy AND NOT `using_sc_pool_template`

**Step 3 – Build the candidate list:**

- If `do_not_use_pinpoint`: active providers where `identifier != "pinpoint"` → effectively SNS.
- Otherwise: active providers where `identifier != "sns"` → effectively Pinpoint.
- In both cases the list is pre-sorted ascending by `priority` by the DAO query.

**Step 4 – Return the client:**

`clients.get_client_by_name_and_type(active_providers_in_order[0].identifier, notification_type)`

If the candidate list is empty, raises `Exception("No active {type} providers")`.

### Sending Vehicles (Pinpoint only)

When Pinpoint is selected for SMS, the template's category is consulted. `SmsSendingVehicles` is an enum with two values:
- `SHORT_CODE` — maps to `"short_code"` (uses `AWS_PINPOINT_SC_POOL_ID`)
- `LONG_CODE` — maps to `"long_code"` (uses `AWS_PINPOINT_DEFAULT_POOL_ID`)

If a template has no category, `sending_vehicle` is passed as `None`.

### Provider Failover

SMS failover is implemented but currently inert:
- On any general exception during `provider.send_sms()` (other than `PinpointConflictException` / `PinpointValidationException`), `dao_toggle_sms_provider(provider.name)` is called before re-raising.
- `dao_toggle_sms_provider` calls `get_alternative_sms_provider`, which simply returns the same provider — so **no actual provider switch occurs** in the current deployment.
- The infrastructure for priority-swapping (`switch_providers`, `dao_switch_sms_provider_to_provider_with_identifier`) is fully implemented and ready for a second SMS provider to be added.

### Provider Priority Mechanics

- `priority` is an integer; lower value = higher priority.
- `get_current_provider` returns the provider with the smallest `priority` among active providers.
- When switching provider `A → B`:
  - If `B.priority > A.priority`: swap the two priority values so B becomes primary.
  - If `B.priority == A.priority`: increment `A.priority` by 10, leaving B with the lower value.
- Every priority change increments `version` and inserts a `provider_details_history` row.
- The notify system user (config `NOTIFY_USER_ID`) is recorded as `created_by_id` on both providers during an automatic switch.

### Provider Health Tracking

There is no automated health-check polling. Provider `active` state is managed purely by:
1. Admin API updates via `POST /provider-details/<id>` (only `priority`, `active`, `created_by` fields are writable).
2. Programmatic updates triggered by `dao_toggle_sms_provider` on send failure.

### Research Mode / Test Keys

Both send functions short-circuit actual AWS calls under three conditions:

| Condition | SMS | Email |
|-----------|-----|-------|
| `service.research_mode == True` | Skip AWS; call `send_sms_response()` simulation | Skip AWS; call `send_email_response()` |
| `notification.key_type == KEY_TYPE_TEST` | Skip AWS; call `send_sms_response()` simulation | Skip AWS; call `send_email_response()` |
| `to == INTERNAL_TEST_NUMBER` (`+16135550123`) | Skip AWS; call `send_sms_response()` simulation | — |
| `to == INTERNAL_TEST_EMAIL_ADDRESS` (`internal.test@cds-snc.ca`) | — | Skip AWS; call `send_email_response()` |
| `to == EXTERNAL_TEST_NUMBER` (`+16135550124`) | **Actual AWS call** + `send_sms_response()` dry-run | — |

In research/test mode the provider is still resolved via `provider_to_use` and recorded in `notification.sent_by`; only the external call is suppressed. A UUID is generated as the fake reference.

### Bounce Rate Monitoring

After every real (non-research) email send:
1. `check_service_over_bounce_rate(service_id)` computes the bounce rate and compares to thresholds:
   - `WARNING`: ≥ 5% — logs a warning.
   - `CRITICAL`: ≥ 10% — logs a warning. **No send-blocking action yet** (TODO: Bounce Rate V2 will raise `BadRequestError`).
2. `bounce_rate_client.set_sliding_notifications(service_id, notification_id)` adds the notification to the sliding-window counter.

Bounce rate data is stored in Redis (via `bounce_rate_client`), not the database.

### PII Scanning (Email)

When `config.SCAN_FOR_PII` is enabled, the plaintext email body is scanned for Canadian Social Insurance Numbers before sending. The pattern is `\s\d{3}-\d{3}-\d{3}\s`. Any match is validated with the Luhn algorithm; if valid, the notification is failed with status `contains-pii`.

### File Attachment / Document Download (Email)

Personalisation values containing `{"document": {...}}` are processed before send:
1. `check_file_url` asserts the URL begins with `http` (blocks `file://`, `ftp://`, etc.).
2. `document_download_client.check_scan_verdict()` is called to verify malware scan status.
3. If `sending_method == "attach"`: the file is fetched via HTTP (up to 5 retries) and attached. If the fetch fails the attachment is silently dropped.
4. Otherwise: the `document` object is replaced with its `url` string for inline linking.

---

## Error Conditions

| Exception | Raised when | Notification status set to |
|-----------|-------------|---------------------------|
| `NotificationTechnicalFailureException` | Service is inactive at send time | `technical-failure` |
| `NotificationTechnicalFailureException` | Notification body contains a valid SIN (Luhn check passes) | `contains-pii` (via `fail_pii`) |
| `MalwareDetectedException` | Document scan returns HTTP 423 (malicious content) | `virus-scan-failed` |
| `MalwareScanInProgressException` | Document scan returns HTTP 428 (in progress) — triggers Celery retry | *(unchanged; Celery retries)* |
| `DocumentDownloadException` | Document-download-api returns an unexpected response code | `technical-failure` |
| `InvalidUrlException` | File URL does not begin with `"http"` | *(none — raised before status update)* |
| `PinpointConflictException` | AWS Pinpoint returns a conflict error | *(re-raised as-is; no status update in this layer)* |
| `PinpointValidationException` | AWS Pinpoint returns a validation error | *(re-raised as-is; no status update in this layer)* |
| `Exception("No active {type} providers")` | `provider_to_use` finds no active, eligible providers | *(raised before send attempt)* |
| *(general SMS send exception)* | Any other exception from `provider.send_sms()` | `billable_units` recorded; `dao_toggle_sms_provider` called; exception re-raised |

Notes on malware scan response codes:
- `422` (scan failure): logged but send proceeds.
- `408` (scan timeout): logged but send proceeds.
- `200`: treated as clean.
- Any other code: `DocumentDownloadException`.

---

## REST Endpoints (provider_details blueprint)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| `GET` | `/provider-details` | `get_providers` | All providers with current-month billable SMS |
| `GET` | `/provider-details/<uuid>` | `get_provider_by_id` | Single provider by ID |
| `GET` | `/provider-details/<uuid>/versions` | `get_provider_versions` | Full version history for a provider |
| `POST` | `/provider-details/<uuid>` | `update_provider_details` | Update `priority`, `active`, and/or `created_by`; any other key returns HTTP 400 |

`POST /provider-details/<id>` allowed fields:
- `priority` (integer)
- `active` (boolean)
- `created_by` (user UUID — resolved to `created_by_id`)

Any key outside this set returns `400 {"errors": {key: ["Not permitted to be updated"]}}`.

---

## Query Inventory (for sqlc)

| Query name | Type | Tables | Description |
|------------|------|--------|-------------|
| `GetProviderDetailsByID` | SELECT ONE | `provider_details` | Fetch provider by UUID PK |
| `GetProviderDetailsByIdentifier` | SELECT ONE | `provider_details` | Fetch provider by string identifier |
| `GetCurrentProvider` | SELECT ONE | `provider_details` | Lowest-priority active provider for a notification type |
| `GetProvidersByNotificationType` | SELECT MANY | `provider_details` | Active providers for a type, ordered by priority; optional `supports_international` filter |
| `GetSMSProviderWithEqualPriority` | SELECT ONE | `provider_details` | Find conflicting active SMS provider with same priority (excluding self) |
| `GetProviderVersions` | SELECT MANY | `provider_details_history` | All history rows for a provider ID, newest first |
| `UpdateProviderDetails` | UPDATE | `provider_details` | Increment version, set updated_at; called alongside insert of history row |
| `InsertProviderDetailsHistory` | INSERT | `provider_details_history` | Append audit snapshot after any provider update |
| `GetProviderStats` | SELECT MANY | `provider_details`, `provider_details_history` (via subq on `fact_billing`), `users` | Providers joined with current-month billable SMS count and creator name |
| `CreateProviderRate` | INSERT | `provider_rates` | Insert a new rate record for a provider with valid_from timestamp |
