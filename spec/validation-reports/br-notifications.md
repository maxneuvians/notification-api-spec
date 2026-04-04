# Validation Report: business-rules/notifications.md
Date: 2026-04-04

## Summary
- **DAO functions in spec**: 40
- **DAO functions in code**: 40
- **CONFIRMED**: 38
- **DISCREPANCIES**: 2 (minor, non-blocking — decorator naming pattern, actually correct)
- **MISSING FROM SPEC**: 0
- **EXTRA IN SPEC**: 0
- **RISK items**: 3

**VALIDATION VERDICT: PASS ✅**

---

## DAO Function Audit

All 40 DAO functions from spec confirmed present in `app/dao/notifications_dao.py`. Full inventory:

| Function | Type | Status |
|---|---|---|
| `dao_get_last_template_usage` | SELECT | ✅ |
| `dao_create_notification` | INSERT | ✅ |
| `bulk_insert_notifications` | INSERT bulk | ✅ |
| `update_notification_status_by_id` | SELECT FOR UPDATE + UPDATE | ✅ |
| `update_notification_status_by_reference` | SELECT + UPDATE | ✅ |
| `dao_update_notification` | UPDATE | ✅ |
| `dao_update_notifications_by_reference` | UPDATE | ✅ |
| `_update_notification_statuses` / `update_notification_statuses` | UPDATE batch | ✅ (decorator pattern, both functions present) |
| `get_notification_for_job` | SELECT | ✅ |
| `get_notifications_for_job` | SELECT paginated | ✅ |
| `get_notification_count_for_job` | COUNT | ✅ |
| `get_notification_with_personalisation` | SELECT + joinedload | ✅ |
| `get_notification_by_id` | SELECT (read replica) | ✅ |
| `get_notifications` | SELECT (query object) | ✅ |
| `get_notifications_for_service` | SELECT filtered | ✅ |
| `delete_notifications_older_than_retention_by_type` | DELETE batched | ✅ |
| `insert_update_notification_history` | INSERT ON CONFLICT DO UPDATE | ✅ |
| `dao_delete_notifications_by_id` | DELETE | ✅ |
| `dao_timeout_notifications` | SELECT + UPDATE (2 passes) | ✅ |
| `is_delivery_slow_for_provider` | SELECT aggregate | ✅ |
| `dao_get_notifications_by_to_field` | SELECT (LIKE) | ✅ |
| `dao_get_notification_by_reference` | SELECT (read replica) | ✅ |
| `dao_get_notification_history_by_reference` | SELECT with fallback | ✅ |
| `dao_get_notifications_by_references` | SELECT IN | ✅ |
| `dao_created_scheduled_notification` | INSERT | ✅ |
| `dao_get_scheduled_notifications` | SELECT + JOIN | ✅ |
| `set_scheduled_notification_to_processed` | UPDATE | ✅ |
| `dao_get_total_notifications_sent_per_day_for_performance_platform` | SELECT aggregate | ✅ |
| `get_latest_sent_notification_for_job` | SELECT ORDER BY updated_at DESC LIMIT 1 | ✅ |
| `dao_get_last_notification_added_for_job_id` | SELECT ORDER BY job_row_number DESC LIMIT 1 | ✅ |
| `notifications_not_yet_sent` | SELECT | ✅ |
| `dao_old_letters_with_created_status` | SELECT | ✅ |
| `dao_precompiled_letters_still_pending_virus_check` | SELECT | ✅ |
| `send_method_stats_by_service` | SELECT aggregate | ✅ |
| `overall_bounce_rate_for_day` | SELECT aggregate | ✅ |
| `service_bounce_rate_for_day` | SELECT aggregate | ✅ |
| `total_notifications_grouped_by_hour` | SELECT aggregate | ✅ |
| `total_hard_bounces_grouped_by_hour` | SELECT aggregate | ✅ |
| `resign_notifications` | SELECT + UPDATE | ✅ |

---

## Limit Enforcement Logic

### Rate Limiting — ✅ CONFIRMED
- Window: 60 seconds ✅
- Cache key format: `rate_limit:{service_id}:{key_type}` ✅
- Guard: `API_RATE_LIMIT_ENABLED AND REDIS_ENABLED` ✅
- Error: `RateLimitError(rate_limit, interval=60, key_type)` ✅

### Daily Email Limits — ✅ CONFIRMED
- Redis key `email_daily_count:{service_id}`, TTL 2 hours ✅
- Threshold: `(emails_sent_today + requested) > service.message_limit` ✅
- Trial/Live distinction ✅
- Warnings at 80% (near limit) and 100% (over limit) ✅

### Daily SMS Limits — ✅ CONFIRMED
- Standard mode: `sms_daily_count:{service_id}` ✅
- Billable-unit mode (`FF_USE_BILLABLE_UNITS`): `billable_units_sms_daily_count:{service_id}` ✅
- Separate warning cache keys for billable-units mode ✅

### Annual Limits — ✅ CONFIRMED
- Redis hash key: `annual_limit_notifications_v2:{service_id}` ✅
- Fields: `TOTAL_EMAIL_FISCAL_YEAR_TO_YESTERDAY`, `TOTAL_SMS_FISCAL_YEAR_TO_YESTERDAY`, `TOTAL_SMS_BILLABLE_UNITS_FISCAL_YEAR_TO_YESTERDAY` ✅
- Zero-count guard prevents re-seeding loop ✅
- 80% warning and 100% threshold with deduplication ✅

---

## Status Machine — ✅ CONFIRMED

All transitions verified in `update_notification_status_by_id` and `update_notification_status_by_reference`:

| Guard condition | Allowed statuses | Disallowed result |
|---|---|---|
| By ID | created, sending, pending, sent, pending-virus-check | silent ignore + log |
| By reference | sending, pending | silent ignore + log |
| Timeout pass 1 | created → technical-failure | ✅ |
| Timeout pass 2 | sending, pending → temporary-failure | ✅ |
| Firetext correction | pending + permanent-failure → **temporary-failure** | ✅ |
| International SMS guard | international=True + no DLR → **skip update entirely** | ✅ |

---

## Retention/Archival — ✅ CONFIRMED

- Per-service `ServiceDataRetention` override respected ✅
- 7-day default ✅
- Archive via `INSERT … ON CONFLICT DO UPDATE` on `notification_history` ✅
- Test-key notifications: deleted only, never archived ✅
- Conflict resolution updates: `notification_status`, `reference`, `billable_units`, `updated_at`, `sent_at`, `sent_by` ✅

---

## Bounce Rate — ✅ CONFIRMED

- 24-hour window ✅
- Minimum 1000 emails (HAVING clause) ✅
- Formula: `(100 * hard_bounces / total_emails)` ✅
- Filter: `feedback_type = 'hard-bounce'` ✅

---

## Simulated Recipients — ✅ CONFIRMED

Hardcoded lists in `config.py`:
- `SIMULATED_EMAIL_ADDRESSES`: 3 addresses (`simulate-delivered@notification.canada.ca` variants) ✅
- `SIMULATED_SMS_NUMBERS`: 3 numbers (`+16132532222/3/4`) ✅
- SMS numbers are validated+formatted before comparison ✅
- Simulated notifications created but not persisted or enqueued ✅

---

## RISK Items for Go Implementors

### 🟡 RISK 1 — Duplicate Callbacks Silently Ignored
`update_notification_status_by_id` and `update_notification_status_by_reference` silently discard updates when the current status is not in the allowed set. Only logged via `_duplicate_update_warning`. Go implementation must replicate this behavior or delivery receipt replays will cause spurious errors.

### 🟡 RISK 2 — International SMS Status Updates Skipped
If `notification.international=True` and `country_records_delivery(phone_prefix)` is False, the status update returns None without recording anything — including hard bounces. Bounce rate will be artificially low for services with many international recipients. Document this business decision explicitly.

### 🟡 RISK 3 — Firetext Provider Correction is Hard-Coded
`_decide_permanent_temporary_failure` downgrades `pending + permanent-failure → temporary-failure`. This is provider-specific behavior for Firetext quirks. If new SMS providers are added in Go, this rule may not apply. Consider making it configurable per-provider.
