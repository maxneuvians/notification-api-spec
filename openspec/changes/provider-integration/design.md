## Context
Provider management covers 7 providers (ses, sns, pinpoint, mmg, firetext, loadtesting, dvla). Each carries priority, active flag, and audit history. Provider selection for every send is computed at call time via the `ProviderToUse` function. This change implements provider CRUD endpoints, the selection algorithm, priority-swap mechanics, and rate tracking. AWS client implementations are in `notification-delivery-pipeline`.

## Goals / Non-Goals
**Goals:** Provider CRUD (4 endpoints + version history), `ProviderToUse` selection algorithm with all routing rules, priority-swap on `dao_switch_sms_provider`, history write on every update, `create_provider_rates`, `dao_get_provider_stats`.

**Non-Goals:** AWS SES/SNS/Pinpoint client implementations (notification-delivery-pipeline), receipt processing (notification-receipt-callbacks).

## Decisions

### Provider list sorted by type then priority
`GET /provider-details` returns providers ordered by `notification_type` then `priority ASC`. Stable ordering verified against: ses, sns, mmg, firetext, loadtesting, pinpoint, dvla.

### History write on every update
`UpdateProviderDetails` (wraps `dao_update_provider_details`): bumps `version`, sets `updated_at = utcnow()`, writes a `ProviderDetailsHistory` snapshot. Field guard: `identifier`, `version`, `updated_at` are not writable (400 if attempted).

### current_month_billable_sms is a computed column
`GET /provider-details` and `GET /provider-details/{id}` compute `current_month_billable_sms` via a LEFT OUTER JOIN subquery on `ft_billing` (SMS rows, `bst_date >= first_day_of_current_month`, sum `billable_units × rate_multiplier`). Absent from version history records.

### Provider selection: 6-condition do_not_use_pinpoint gate
`ProviderToUse(notificationType, sender, to, templateID, international)`:
1. Classify recipient (5 flags: has_dedicated_number, sending_to_us_number, recipient_outside_canada, cannot_determine_recipient_country, using_sc_pool_template, zone_1_outside_canada)
2. `do_not_use_pinpoint = true` if any of 6 conditions hold (see brief)
3. Build candidate list: SNS path excludes "pinpoint"; Pinpoint path excludes "sns"
4. Return `clients.GetClientByNameAndType(candidates[0])`. Empty list → error.

### Priority-swap mechanics
Switching A→B:
- B.priority > A.priority: swap the two values
- B.priority == A.priority: A.priority += 10
Both providers get version bump + history row. `NOTIFY_USER_ID` recorded as `created_by`.

### Provider failover is currently a no-op
`ToggleSMSProvider(identifier)` calls `GetAlternativeSMSProvider` → returns same provider → switch is identity. Infrastructure is fully implemented; a second SMS provider enables it. Called on general send exception only (not PinpointConflict/Validation exceptions).

### Provider rates are append-only
`CreateProviderRates`: INSERT only; no update/delete. Rate for a notification = most recent rate with `valid_from <= notification.created_at`.

## Risks / Trade-offs
- Two-provider priority swap is the only path for provider switches; no automatic health-check polling.
- `GetAlternativeSMSProvider` is currently identity — failover is inert until a second SMS provider is added.
