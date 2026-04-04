# Validation Report: business-rules/providers.md
Date: 2026-04-04

## Summary
- **DAO functions validated**: 9/9 present and match spec exactly
- **Provider selection logic**: All 6 classification flags and DO-NOT-USE conditions implemented
- **Failover infrastructure**: Implemented but currently inert (no second SMS provider deployed)
- **Rate recording**: Append-only ProviderRates table confirmed
- **Discrepancies**: 0
- **Risk items**: 4

## Verdict
**CONDITIONAL PASS** — Business rules correct. Failover feature is inert by design; Go must track when second SMS provider is added.

---

## Confirmed

- Provider selection: all 6 classification flags (`has_dedicated_number`, `sending_to_us_number`, `recipient_outside_canada`, `cannot_determine_recipient_country`, `using_sc_pool_template`, `zone_1_outside_canada`) computed correctly ✅
- `do_not_use_pinpoint` conditions: 6 OR conditions match spec exactly ✅
- Candidate list filtered by active + identifier exclusion, exception if empty ✅
- `dao_toggle_sms_provider`: calls `get_alternative_sms_provider` which returns same provider (no-op — as documented) ✅
- `dao_switch_sms_provider_to_provider_with_identifier`: full priority swap logic ✅
- `dao_update_provider_details`: increments `version`, sets `updated_at`, appends history row ✅
- `create_provider_rates`: append-only INSERT ✅
- Provider failover: `PinpointConflictException`/`PinpointValidationException` re-raised as-is; other exceptions trigger `dao_toggle_sms_provider` ✅

---

## Discrepancies
None.

---

## Missing from Spec

1. `FF_USE_PINPOINT_FOR_DEDICATED` gates dedicated-number routing — mentioned in context but not explicitly listed in provider selection decision tree
2. No recovery procedure documented for unavailable provider (other than manual UI toggle + exception-triggered failover)

---

## RISK Items for Go Implementors

1. **Failover currently inert**: `get_alternative_sms_provider` returns same provider. When second SMS provider is added, this function MUST be updated. Add integration test to verify failover triggers actual swap.

2. **Priority integer bounds**: Priority is a signed integer with no documented max. Recommend documenting max-priority constraint to prevent sorting anomalies.

3. **Silent routing drift**: If `AWS_PINPOINT_SC_POOL_ID` / `AWS_PINPOINT_DEFAULT_POOL_ID` are cleared in config, provider selection silently prefers SNS. Log WARN at provider-selection time when critical config is empty.

4. **Timezone in billing stats**: `dao_get_provider_stats` uses `convert_utc_to_local_timezone` for monthly boundary. If deployment timezone differs from stats consumers, month boundary shifts by a day in admin UI.
