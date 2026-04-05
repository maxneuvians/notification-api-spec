## Source Files

- `spec/behavioral-spec/providers.md` — endpoint contracts, DAO behavior, delivery logic, client behavior, business rules
- `spec/business-rules/providers.md` — data access patterns, provider selection algorithm, priority mechanics

## Scope

Provider CRUD + selection algorithm. Note: AWS client implementations and receipt processing are in `notification-delivery-pipeline` and `notification-receipt-callbacks`.

---

## Endpoints

### GET /provider-details
- Returns all 7 providers sorted by notification_type then by priority (ASC)
- Fields: `id`, `created_by_name`, `display_name`, `identifier`, `priority`, `notification_type`, `active`, `updated_at`, `supports_international`, `current_month_billable_sms`
- `current_month_billable_sms` = sum of `billable_unit × rate_multiplier` from `ft_billing` for current calendar month only
- Auth: internal authorization header

### GET /provider-details/{id}
- Single provider in `provider_details` envelope
- Auth: internal authorization header

### POST /provider-details/{id}
- Writable fields: `priority`, `active`, `created_by`
- Returns 200 with updated provider_details object
- Attempting to update `identifier`, `version`, or `updated_at` → 400 `{"result": "error", "message": {"<field>": ["Not permitted to be updated"]}}`
- Auth: internal authorization header + Content-Type: application/json

### GET /provider-details/{id}/versions
- Returns version history in `data` array
- Fields: `id`, `created_by`, `display_name`, `identifier`, `priority`, `notification_type`, `active`, `version`, `updated_at`, `supports_international`
- Note: `current_month_billable_sms` absent from history records

---

## Provider Registry (7 providers)

| Identifier | Type | supports_international |
|------------|------|----------------------|
| ses | email | False |
| sns | sms | configurable |
| pinpoint | sms | configurable |
| mmg | sms | True (inactive) |
| firetext | sms | inactive |
| loadtesting | sms | inactive |
| dvla | (separate type) | |

- `SMS_PROVIDERS = ["sns", "pinpoint"]`; `EMAIL_PROVIDERS = ["ses"]`
- Fresh provider has exactly 1 history row

---

## dao_update_provider_details

- `@transactional`; bumps `version` by 1; writes `ProviderDetailsHistory` snapshot; sets `updated_at = utcnow()`
- Every modification must go through this function to maintain audit history
- After update: both old history row (version=1) and new row (version=2) exist

---

## Provider Selection Algorithm (provider_to_use)

**Step 1 — Classify recipient and sender:**

| Flag | Condition |
|------|-----------|
| has_dedicated_number | sender not None AND starts with "+1" |
| sending_to_us_number | parsed phone region = "US" |
| recipient_outside_canada | region != "CA" AND region != "US" |
| cannot_determine_recipient_country | phonenumbers.PhoneNumberMatcher returns no match |
| using_sc_pool_template | template_id in config.AWS_PINPOINT_SC_TEMPLATE_IDS |
| zone_1_outside_canada | recipient_outside_canada AND NOT international |

**Step 2 — do_not_use_pinpoint = True if ANY of:**
1. has_dedicated_number AND NOT config.FF_USE_PINPOINT_FOR_DEDICATED
2. sending_to_us_number
3. cannot_determine_recipient_country
4. zone_1_outside_canada
5. config.AWS_PINPOINT_SC_POOL_ID is empty/falsy
6. config.AWS_PINPOINT_DEFAULT_POOL_ID is empty/falsy AND NOT using_sc_pool_template

**Step 3 — Build candidate list:**
- do_not_use_pinpoint → active providers where identifier != "pinpoint" (→ SNS)
- else → active providers where identifier != "sns" (→ Pinpoint)
- sorted ascending by priority (DAO pre-sort)

**Step 4:** Return `clients.get_client_by_name_and_type(candidates[0].identifier, type)`. Empty list → raises `Exception("No active {type} providers")`

### Sending Vehicles (Pinpoint SMS only)
Template category → SmsSendingVehicles enum:
- SHORT_CODE → uses AWS_PINPOINT_SC_POOL_ID
- LONG_CODE → uses AWS_PINPOINT_DEFAULT_POOL_ID
- No category → None (→ DEFAULT_POOL_ID)

---

## Priority Mechanics

- Lower priority integer = higher priority
- `get_current_provider(type)`: lowest priority among active providers; returns None if all inactive
- When switching provider A → B:
  - B.priority > A.priority: swap the two values
  - B.priority == A.priority: increment A.priority by 10
- Every priority change: bump version, write history row, record NOTIFY_USER_ID as created_by

---

## Provider Failover

- `dao_toggle_sms_provider(identifier)`: gets alternate via `get_alternative_sms_provider`, then `dao_switch_sms_provider_to_provider_with_identifier`
- Currently a no-op (only one active SMS provider; `get_alternative_sms_provider` returns same provider)
- Called on any general SMS send exception (NOT on PinpointConflictException or PinpointValidationException)
- `dao_switch_sms_provider_to_provider_with_identifier`: no-op if target is already current; no-op if target is inactive

---

## Provider Rates

- `create_provider_rates(identifier, valid_from, rate)`: insert into provider_rates; rate is Decimal; valid_from is datetime
- Append-only; no update/delete path
- Rate for a notification: most recent rate with `valid_from <= notification.created_at`
- Missing rate: `ValueError` with `[error-sms-rates]` log tag

---

## Error Conditions

| Condition | Response |
|-----------|----------|
| Update disallowed field (identifier/version/updated_at) | 400 `{"result":"error","message":{"field":["Not permitted to be updated"]}}` |
| All providers of a type are inactive | Exception "No active {type} providers" |
| provider_rates lookup: no matching identifier | NoResultFound |
| get_provider_details_by_identifier: not found | NoResultFound |
