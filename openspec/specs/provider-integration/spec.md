# Capability: provider-integration

Provider administration, SMS provider selection, failover, and provider rate lookup behavior.

---

## Requirement: Provider list endpoint with billable SMS count

`GET /provider-details` SHALL return all providers sorted by `notification_type` then `priority ASC`. Fields SHALL include `id`, `created_by_name`, `display_name`, `identifier`, `priority`, `notification_type`, `active`, `updated_at`, `supports_international`, and `current_month_billable_sms`. `current_month_billable_sms` SHALL be computed as sum of `billable_unit × rate_multiplier` from `ft_billing` for the current calendar month (SMS rows only). All provider endpoints require internal authorization.

#### Scenario: Providers sorted by type then priority
- **WHEN** GET /provider-details is called
- **THEN** providers appear in order: ses, sns, mmg, firetext, loadtesting, pinpoint (sorted by notification_type then priority ASC)

#### Scenario: current_month_billable_sms computed correctly
- **WHEN** GET /provider-details is called and ft_billing has SMS rows for the current month
- **THEN** current_month_billable_sms reflects the sum of billable_unit × rate_multiplier for those rows

#### Scenario: current_month_billable_sms is zero for providers with no billing rows
- **WHEN** a provider has no ft_billing rows for the current month
- **THEN** current_month_billable_sms is 0

---

## Requirement: Provider update with field guard

`POST /provider-details/{id}` SHALL accept updates to `priority`, `active`, and `created_by`. Fields `identifier`, `version`, and `updated_at` SHALL NOT be updatable; attempting to set them SHALL return HTTP 400 with `{"result": "error", "message": {"<field>": ["Not permitted to be updated"]}}`. Returns 200 with updated object. Every update SHALL bump `version` by 1, set `updated_at = utcnow()`, and write a `ProviderDetailsHistory` snapshot.

#### Scenario: priority update accepted
- **WHEN** POST /provider-details/{id} is called with `{"priority": 5}`
- **THEN** HTTP 200 and the provider's priority is updated in DB

#### Scenario: active update accepted
- **WHEN** POST /provider-details/{id} is called with `{"active": false}`
- **THEN** HTTP 200 and provider.active is false in DB

#### Scenario: identifier update rejected
- **WHEN** POST /provider-details/{id} is called with `{"identifier": "new"}`
- **THEN** HTTP 400 `{"result": "error", "message": {"identifier": ["Not permitted to be updated"]}}`

#### Scenario: history row written on every update
- **WHEN** any provider field is updated
- **THEN** a new ProviderDetailsHistory row exists and version is incremented

---

## Requirement: Provider version history endpoint

`GET /provider-details/{id}/versions` SHALL return all history rows in a `data` array. Each row contains `id`, `created_by`, `display_name`, `identifier`, `priority`, `notification_type`, `active`, `version`, `updated_at`, `supports_international`. The `current_month_billable_sms` field SHALL NOT appear in history records.

#### Scenario: fresh provider has one history row
- **WHEN** GET /provider-details/{id}/versions is called on an unmodified provider
- **THEN** `data` contains exactly one entry

#### Scenario: current_month_billable_sms absent from version history
- **WHEN** GET /provider-details/{id}/versions is called
- **THEN** none of the history objects contain current_month_billable_sms

---

## Requirement: Provider selection algorithm (ProviderToUse)

The provider selection function SHALL classify the recipient and sender to determine the SMS provider. `do_not_use_pinpoint` SHALL be set to true if any of: (1) has_dedicated_number AND NOT FF_USE_PINPOINT_FOR_DEDICATED; (2) sending_to_us_number; (3) cannot_determine_recipient_country; (4) zone_1_outside_canada (recipient outside Canada and not international); (5) AWS_PINPOINT_SC_POOL_ID is empty; (6) AWS_PINPOINT_DEFAULT_POOL_ID is empty AND using_sc_pool_template is false. When Pinpoint is excluded, SNS is used; otherwise Pinpoint is used. Empty candidate list SHALL raise an error. For email, SES is always used.

#### Scenario: US number routes to SNS
- **WHEN** ProviderToUse is called with a US phone number
- **THEN** SNS is selected (do_not_use_pinpoint = true due to sending_to_us_number)

#### Scenario: Canadian number with both pool IDs configured routes to Pinpoint
- **WHEN** ProviderToUse is called with a Canadian number and both AWS_PINPOINT_SC_POOL_ID and AWS_PINPOINT_DEFAULT_POOL_ID configured
- **THEN** Pinpoint is selected

#### Scenario: dedicated sender with FF_USE_PINPOINT_FOR_DEDICATED=false routes to SNS
- **WHEN** ProviderToUse is called with a dedicated +1 sender and FF_USE_PINPOINT_FOR_DEDICATED=false
- **THEN** SNS is selected

#### Scenario: SC pool template routes to Pinpoint even without default pool
- **WHEN** template_id is in AWS_PINPOINT_SC_TEMPLATE_IDS and SC pool ID is configured
- **THEN** Pinpoint is selected regardless of default pool ID configuration

#### Scenario: zone-1 non-Canada number routes to SNS
- **WHEN** ProviderToUse is called with a +1671 (Guam) non-international number
- **THEN** SNS is selected (zone_1_outside_canada = true)

#### Scenario: unparseable phone number routes to SNS
- **WHEN** ProviderToUse is called with a number that phonenumbers cannot match
- **THEN** SNS is selected (cannot_determine_recipient_country = true)

#### Scenario: all SMS providers inactive raises error
- **WHEN** all SMS providers are inactive
- **THEN** ProviderToUse raises Exception("No active sms providers")

---

## Requirement: Sending vehicle selection (Pinpoint SMS)

When Pinpoint is selected, the template's category SHALL determine the sending vehicle. Templates mapped to `SHORT_CODE` SHALL use `AWS_PINPOINT_SC_POOL_ID`; `LONG_CODE` or no category SHALL use `AWS_PINPOINT_DEFAULT_POOL_ID`.

#### Scenario: short_code pool used for SC template
- **WHEN** template is in SmsSendingVehicles.SHORT_CODE category
- **THEN** OriginationIdentity is set to AWS_PINPOINT_SC_POOL_ID

---

## Requirement: Provider priority-swap mechanics

`SwitchSMSProviderToIdentifier` SHALL perform a priority swap: if B.priority > A.priority then swap values; if B.priority == A.priority then A.priority += 10. Both providers get a version bump and history row with NOTIFY_USER_ID as created_by. No-op if target is already current or is inactive.

#### Scenario: priority swap when target has lower priority
- **WHEN** SwitchSMSProviderToIdentifier is called and target has higher priority integer
- **THEN** the two providers' priority values are swapped

#### Scenario: equal priority increments current by 10
- **WHEN** SwitchSMSProviderToIdentifier is called and both providers share the same priority
- **THEN** the current provider's priority is incremented by 10, making the target primary

#### Scenario: switching to already-current provider is no-op
- **WHEN** SwitchSMSProviderToIdentifier is called with the identifier of the current primary
- **THEN** no update is performed

---

## Requirement: Provider failover on send failure

`ToggleSMSProvider(identifier)` SHALL be called on any general exception from the SMS provider send call. It SHALL NOT be called on `PinpointConflictException` or `PinpointValidationException`. Currently a no-op (`GetAlternativeSMSProvider` returns the same provider), but the infrastructure is implemented for when a second SMS provider is added.

#### Scenario: toggle not called on PinpointValidationException
- **WHEN** the provider raises PinpointValidationException
- **THEN** ToggleSMSProvider is NOT called

#### Scenario: toggle called on generic provider exception
- **WHEN** the provider raises a generic exception during send
- **THEN** ToggleSMSProvider is called before propagating the error

---

## Requirement: Provider rate records (append-only)

`CreateProviderRates(identifier, validFrom, rate)` SHALL insert a provider_rates row. Rate is a Decimal value. valid_from is a datetime. Rates are append-only (no update/delete). Rate selection for a notification: the most recent rate with `valid_from <= notification.created_at`. Missing rate SHALL log `[error-sms-rates]` and raise ValueError.

#### Scenario: rate record created with correct values
- **WHEN** CreateProviderRates is called with identifier, valid_from, and rate
- **THEN** exactly one provider_rates row is created linked to the correct provider_id

#### Scenario: missing rate raises ValueError
- **WHEN** rate lookup finds no rate with valid_from <= notification.created_at
- **THEN** ValueError is raised and [error-sms-rates] is logged