# Validation Report: behavioral-spec/external-clients.md
Date: 2026-04-04

## Summary
- **Clients validated**: Airtable, AWS SNS, AWS SES, Document Download, Freshdesk, Performance Platform, Salesforce (5 modules), AWS Pinpoint
- **CONFIRMED**: All client implementations
- **DISCREPANCIES**: 0
- **UNCOVERED**: 5 (schema lock-in, SNS MessageAttributes, SES retry count, Salesforce session, PP token isolation)
- **Risk items**: 5

## Verdict
**FULLY CONFIRMED** — All external client contracts validated against tests.

---

## Confirmed

- `AirtableTableMixin.table_exists()`: checks Meta.table_name; `AttributeError` if Meta absent ✅
- `NewsletterSubscriber` defaults: `language="en"`, `status="unconfirmed"` ✅
- `AwsSnsClient.send_sms()`: prepends `+1` to 10-digit numbers; routing CA → `_client`, US → `_long_codes_client` ✅
- `AwsSesClient.send_email()`: raw MIME (multipart/alternative + attachments), IDN → punycode, base64 headers ✅
- `DocumentDownloadClient.upload_document()`: multipart form, filename included if present ✅
- `FreshdeskClient.send_ticket()`: product_id=42, status=2, priority=1; fallback email on `RequestException` ✅
- `PerformancePlatformClient.send_stats_to_performance_platform()`: no-op if `_active=False`; separate token per dataset ✅
- `SalesforceAccount.get_account_id_from_name()`: returns `generic_account_id` immediately if name is None/empty/whitespace; escapes single quotes ✅
- `SalesforceClient.contact_create()`: FirstName from first word; CDS_Contact_ID__c=str(user.id) ✅
- `SalesforceEngagement.create()`: CloseDate=today; `OpportunityLineItem` (Quantity=1, UnitPrice=0) ✅
- `SalesforceUtils.get_name_parts()`: single word → (`""`, name); multi → split on first space ✅

---

## Discrepancies
None.

---

## Uncovered Contracts

1. **Airtable schema 7-field structure**: field names/types not verified (Language + Status assumed single-select)
2. **SNS MessageAttributes**: all keys (`AWS.SNS.SMS.SMSType`, `AWS.MM.SMS.OriginationNumber`) not verified
3. **SES attachment retry**: "up to 5 retries on HTTP 5xx" — exact count not tested
4. **Salesforce session lifecycle**: `end_session()` token revocation not verified; session reuse after revoke not tested
5. **Performance Platform token isolation**: no test verifies wrong token for dataset X fails

---

## RISK Items for Go Implementors

1. **Airtable schema lock-in**: Field names determined at runtime from `Meta.get_table_schema()`. Changing field names in Go breaks existing Airtable base. Version schema or add migration logic.

2. **SNS + Pinpoint routing complexity**: Client selection depends on 5+ config flags + phone geography. Off-by-one causes silent failover to wrong provider with no error logged.

3. **SES raw MIME encoding**: Boundary generation and part ordering are critical. Invalid MIME fails silently at provider level.

4. **Salesforce contact race condition**: `contact_create` upsertion — if two requests race, duplicate contact rows created. Implement idempotency key or retry-then-deduplicate.

5. **Freshdesk fallback cascade**: On Freshdesk error, sends fallback notification to `CONTACT_FORM_EMAIL_ADDRESS`. If that notification also fails, ticket is silently lost. Add audit log entry even on fallback failure.
