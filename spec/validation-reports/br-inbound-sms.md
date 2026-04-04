# Validation Report: business-rules/inbound-sms.md
Date: 2026-04-04

## Summary
- **DAO functions validated**: 12/12 present and functional
- **Confirmed**: 12/12
- **Discrepancies**: 0 significant
- **Missing from spec**: 2 (phone normalization failure behavior, ORM content signing lifecycle)
- **Risk items**: 4

## Verdict
**PASS** — All specified business rules correctly implemented.

---

## Confirmed

- `dao_get_paginated_inbound_sms_for_service_for_public_api`: Cursor-based pagination via `created_at` scalar subquery ✅
- `dao_allocate_number_for_service`: Single conditional UPDATE as atomicity lock ✅
- `delete_inbound_sms_older_than_retention`: Two-pass logic with per-service override + 7-day fallback ✅
- Inbox view self-join filters most-recent per sender ✅
- Phone number normalization in REST layer via `try_validate_and_format_phone_number` ✅
- Content signing via `signer_inbound_sms` ORM property ✅

---

## Discrepancies
None significant. Spec documents SQL anti-join as `t2.id IS NULL`; code uses SQLAlchemy `t2.id == None` — generates identical SQL.

---

## Missing from Spec

1. Phone number normalization failure behavior — spec notes "best-effort"; code continues with unnormalized value but no documented fallback
2. `signer_inbound_sms` ORM signing lifecycle — `_content` column encryption not mentioned in spec (also flagged in data-model report)

---

## RISK Items for Go Implementors

1. **Retention sweep failure = unbounded data growth**: No alerting/monitoring if `delete_inbound_sms_older_than_retention` Celery task fails. Recommend distributed lock + failure notification.

2. **Unsafe signing mode**: `resign_inbound_sms` has `unsafe=True` flag that bypasses signature verification during key rotation. Require audit logging when used operationally.

3. **Alphanumeric sender IDs**: Alphanumeric short codes fail normalization; code continues with unnormalized value. May cause routing anomalies. Document explicitly.

4. **Retention window hardcoded in dashboard summary**: Inbox summary endpoint always uses 7 days regardless of per-service retention policy. Could confuse admins with differing retention settings.
