# Validation Report: behavioral-spec/providers.md
Date: 2026-04-04

## Summary
- **Contracts tested**: ~12 endpoint + ~10 DAO + 3 task contracts
- **CONFIRMED**: All core contracts
- **DISCREPANCIES**: 0
- **UNCOVERED**: 3 (multi-SMS failover skipped, rate limiting, research mode + provider)
- **Risk items**: 5

## Verdict
**MOSTLY CONFIRMED** — Core functionality verified. Provider failover (multi-SMS switch) tests are currently skipped.

---

## Confirmed

- GET /provider-details: 7 providers sorted by type then priority; `current_month_billable_sms` included ✅
- POST /provider-details/{id}: update priority/active; rejects identifier/version/updated_at (400) ✅
- GET /provider-details/{id}/versions: history records, no billable_sms ✅
- `dao_get_provider_details_by_notification_type`: 5 SMS (domestic), 3 (international), 1 email ✅
- `dao_update_provider_details`: bumps version, writes history row ✅
- deliver_sms task: retry 25s then 300s; `PinpointValidationException` = no retry ✅
- deliver_email task: `InvalidEmailError` = no retry, `AwsSesClientException` = retry ✅
- process_pinpoint_results: delivered, spam → permanent, carrier issue → temporary, opted-out → terminal ✅
- process_ses_results: hard bounce → permanent, soft bounce → temporary, no status downgrade ✅

---

## Discrepancies
None.

---

## Uncovered Contracts

1. **Multi-SMS provider failover**: `dao_toggle_sms_provider` and `dao_switch_sms_provider_to_provider_with_identifier` tests are **SKIPPED** — spec says switching is no-op if already current, but not validated
2. **Provider rate limiting**: `create_provider_rates()` stores rate + valid_from but no enforcement logic tested
3. **Research mode + provider selection**: research-mode SMS uses mock, but spec doesn't clarify if provider selection logic still runs

---

## RISK Items for Go Implementors

1. **SMS provider selection (8 conditions)**: Complex branching with Pinpoint SC pool, default pool, dedicated sender, geographic flags. Add unit tests for each condition in isolation; current tests only validate final decision.

2. **Pinpoint vs SNS determination**: Depends on `AWS_PINPOINT_SC_POOL_ID` and `AWS_PINPOINT_DEFAULT_POOL_ID`. Go must handle all 4 combinations (both set, one set, neither set).

3. **Callback dispatch silent failure**: Callback task queued asynchronously — if task fails silently, webhook never fires. No test verifies callback exception handling.

4. **Annual limit seeding race**: Seeding uses freeze_time (single-threaded). Concurrent requests may double-count on first delivery of day.

5. **Hard bounce status downgrade prevention**: SES receipt won't downgrade permanent-failure. Verify SNS/Pinpoint receipts respect the same guard.
