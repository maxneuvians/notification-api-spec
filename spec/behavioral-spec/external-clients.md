# Behavioral Spec: External Clients

## Processed Files
- [x] tests/app/clients/test_airtable.py
- [x] tests/app/clients/test_aws_sns.py
- [x] tests/app/clients/test_document_download.py
- [x] tests/app/clients/test_freshdesk.py
- [x] tests/app/clients/test_performance_platform.py
- [x] tests/app/clients/test_salesforce_account.py
- [x] tests/app/clients/test_salesforce_auth.py
- [x] tests/app/clients/test_salesforce_client.py
- [x] tests/app/clients/test_salesforce_contact.py
- [x] tests/app/clients/test_salesforce_engagement.py
- [x] tests/app/clients/test_salesforce_utils.py
- [x] tests/app/celery/test_process_sns_receipts_tasks.py
- [x] tests/app/performance_platform/test_processing_time.py

---

## Client Contracts

### Airtable (`app/clients/airtable`)

**Purpose**: Stores newsletter subscriber data and newsletter template references in Airtable bases. Two concrete models: `NewsletterSubscriber` and `LatestNewsletterTemplate`, both built on `AirtableTableMixin`.

#### `AirtableTableMixin` (base class)

**Key methods**:

| Method | Description |
|--------|-------------|
| `table_exists()` | Checks if the table named in `Meta.table_name` exists in the base; returns `bool`. Requires a `Meta` attribute. |
| `get_table_schema()` | Abstract — raises `NotImplementedError` with message `"Subclasses must implement get_table_schema"`. |
| `create_table()` | Calls `meta.base.create_table(name, fields=[...])` using the schema returned by `get_table_schema()`. Requires a `meta` attribute (lowercase). |

**Error handling**:
- `table_exists()` raises `AttributeError("Model must have a Meta attribute")` when `Meta` is absent.
- `create_table()` raises `AttributeError("Model must have a meta attribute")` when `meta` is absent.

---

#### `NewsletterSubscriber`

**Auth/config**: Reads from Flask config inside `Meta`:
- `AIRTABLE_API_KEY`
- `AIRTABLE_NEWSLETTER_BASE_ID`
- `AIRTABLE_NEWSLETTER_TABLE_NAME`

**Defaults on construction**: `language = Languages.EN.value` ("en"), `status = Statuses.UNCONFIRMED.value` ("unconfirmed").

**Enums**:
- `Languages`: `EN = "en"`, `FR = "fr"`
- `Statuses`: `UNCONFIRMED = "unconfirmed"`, `SUBSCRIBED = "subscribed"`, `UNSUBSCRIBED = "unsubscribed"`

**Airtable schema** (7 fields): `Email`, `Language`, `Status`, `Created At`, `Confirmed At`, `Unsubscribed At`, `Has Resubscribed`; `Language` and `Status` are single-select with enum-derived choices.

**Key methods**:

| Method | Behavior |
|--------|----------|
| `from_email(email)` | Queries Airtable with formula `{Email} = 'email'`. Returns the first match. Raises `HTTPError(404)` if no records found. Propagates any other exception unchanged. |
| `save_unconfirmed_subscriber()` | Sets `status = UNCONFIRMED`, `created_at = datetime.now()`, then calls `save()`. Returns the result of `save()`. |
| `confirm_subscription(has_resubscribed=False)` | Sets `status = SUBSCRIBED`, `confirmed_at = datetime.now()`, `has_resubscribed = has_resubscribed`, then calls `save()`. |
| `unsubscribe_user()` | Sets `status = UNSUBSCRIBED`, `unsubscribed_at = datetime.now()`, clears `confirmed_at = None`, then calls `save()`. |
| `update_language(lang)` | Validates `lang` is in `Languages` enum, sets `language = lang`, calls `save()`. |

**Behavioral contracts from tests**:
- Table auto-created on first `save()` if `table_exists()` returns `False`.
- `from_email` uses exact formula-based lookup, not a scan.
- `confirm_subscription()` does not set `has_resubscribed` to `True` by default; caller must pass `has_resubscribed=True` explicitly.
- `unsubscribe_user()` clears `confirmed_at` to `None`.

---

#### `LatestNewsletterTemplate`

**Auth/config**: Same `AIRTABLE_API_KEY` and `AIRTABLE_NEWSLETTER_BASE_ID`; table name from `AIRTABLE_CURRENT_NEWSLETTER_TEMPLATES_TABLE_NAME` (defaults to `"Newsletter Templates"` if not set).

**Airtable schema** (2 fields, both `singleLineText`): `(EN) Template ID`, `(FR) Template ID`.

**Key methods**:

| Method | Behavior |
|--------|----------|
| `get_latest_newsletter_templates()` | Creates table if absent. Queries with `sort=["-Created at"], max_records=1`. Returns the single result. Raises `HTTPError(404)` if no records exist. |

**Behavioral contracts from tests**:
- Table is created on `save()` and on `get_latest_newsletter_templates()` when `table_exists()` returns `False`.

---

### AWS SNS (`app` — `aws_sns_client`)

**Purpose**: Sends transactional SMS messages via Amazon SNS.

**Auth/config**: Uses boto3 internally via `_client` (standard SNS endpoint) and `_long_codes_client` (long-code/toll-free origination). `AWS_US_TOLL_FREE_NUMBER` config key provides the US origination number.

**Key methods**:

`send_sms(to, content, reference, sender=None)`:
- Formats `to` to E.164 by prepending `+1` (e.g. `"6135555555"` → `"+16135555555"`).
- Raises `ValueError("No valid numbers found for SMS delivery")` when `to` is empty/invalid.
- Message type is always `"Transactional"`.

**Routing logic**:

| Condition | Client used | Extra attribute |
|-----------|-------------|-----------------|
| `sender` is None, Canadian number | `_client` | None |
| `sender` is a long-code (e.g. `+19025551234`), Canadian number | `_long_codes_client` | `AWS.MM.SMS.OriginationNumber = sender` |
| US number (regardless of `sender`) | `_long_codes_client` | `AWS.MM.SMS.OriginationNumber = AWS_US_TOLL_FREE_NUMBER` |

**SNS publish payload structure**:
```json
{
  "PhoneNumber": "+1XXXXXXXXXX",
  "Message": "<content>",
  "MessageAttributes": {
    "AWS.SNS.SMS.SMSType": {"DataType": "String", "StringValue": "Transactional"},
    "AWS.MM.SMS.OriginationNumber": {"DataType": "String", "StringValue": "<number>"}
  }
}
```
(The `OriginationNumber` attribute is omitted when routing through `_client`.)

**Behavioral contracts from tests**:
- US area codes (e.g. 718 NYC) always use the toll-free origination number, even when `sender=None`.
- Canadian long-code sends use the provided `sender` as origination number.

---

### Document Download (`app/clients/document_download.DocumentDownloadClient`)

**Purpose**: Uploads documents to the GC Notify document-download service and returns a retrievable URL.

**Auth/config**:
- `DOCUMENT_DOWNLOAD_API_HOST`: base URL (e.g. `"https://document-download"`)
- `DOCUMENT_DOWNLOAD_API_KEY`: Bearer token sent in `Authorization` header

**Key methods**:

| Method | Behavior |
|--------|----------|
| `get_upload_url(service_id)` | Returns `"{host}/services/{service_id}/documents"`. No HTTP call. |
| `upload_document(service_id, document_data)` | POSTs multipart form to the upload URL. Sends `Authorization: Bearer {key}`. Returns the full JSON response body on HTTP 201. |

**`upload_document` filename handling**:
- If `document_data` contains a `"filename"` key: includes a `filename` form field in the multipart body.
- If `"filename"` is absent: the `filename` field is omitted entirely from the request body.

**Error handling**:

| Condition | Exception raised |
|-----------|-----------------|
| HTTP error response (e.g. 403) | `DocumentDownloadError(message=response.json()["error"], status_code=<N>)` |
| Connection error (e.g. `ConnectTimeout`) | `DocumentDownloadError(message="error connecting to document download")` |

**Behavioral contracts from tests**:
- The presence/absence of `filename` in the multipart body is specifically verified (not just the response).
- `document_data` must always contain `"sending_method"`.

---

### Freshdesk (`app/clients/freshdesk.Freshdesk`)

**Purpose**: Creates support tickets in Freshdesk for various user contact request types (demo requests, go-live requests, branding requests, etc.).

**Auth/config**:
- `FRESH_DESK_ENABLED`: feature flag (bool); when `False`, method returns `201` immediately without HTTP call.
- `FRESH_DESK_API_KEY`: used as `Authorization: Basic {base64(api_key:x)}`.
- `FRESHDESK_URL`: base URL; tickets POSTed to `{FRESHDESK_URL}/api/v2/tickets`.
- `CONTACT_FORM_EMAIL_ADDRESS`: used for fallback email when Freshdesk is down.
- `SENSITIVE_SERVICE_EMAIL` + `CONTACT_FORM_SENSITIVE_SERVICE_EMAIL_TEMPLATE_ID`: for protected-service escalation.

**Ticket payload structure** (all ticket types):
```json
{
  "product_id": 42,
  "subject": "<derived>",
  "description": "<derived>",
  "email": "<contact_request.email_address>",
  "priority": 1,
  "status": 2,
  "tags": []
}
```

**`send_ticket()` — ticket content by `support_type`**:

| `support_type` | `subject` | `description` key fields |
|----------------|-----------|--------------------------|
| `"demo"` | `friendly_support_type` | user name/email, department/org, program/service, intended recipients, main use case, use case details |
| `"go_live_request"` | `"Support Request"` | service name + timestamp, org, recipients, purposes (with "other"), email/SMS volume (daily/yearly with exact counts), service URL |
| `"branding_request"` | `"Branding request"` | bilingual (EN + FR, separated by `<hr><br>`): service id/name, org id/name, logo filename, logo name, alt text EN/FR |
| `"new_template_category_request"` | `"New template category request"` | bilingual: service id, template category name, template id link |
| Any other / empty `support_type` | `friendly_support_type` or `"Support Request"` | Empty, or appends `<br><br>---<br><br> {user_profile}` if `user_profile` is set |

**Return value**: HTTP status code of the Freshdesk API response (e.g. `201`).

**Error handling**:

| Condition | Behavior |
|-----------|----------|
| `FRESH_DESK_ENABLED = False` | Returns `201` immediately; no HTTP call made; fallback email not sent |
| `RequestException` from `requests.post` | Calls `email_freshdesk_ticket()` as fallback (persists and queues a Notify notification); still returns `201` |

**Fallback methods**:
- `email_freshdesk_ticket_freshdesk_down()`: calls `persist_notification()` + `send_notification_to_queue()` using `CONTACT_FORM_EMAIL_ADDRESS`.
- `email_freshdesk_ticket_pt_service()`: sends to `SENSITIVE_SERVICE_EMAIL` using `CONTACT_FORM_SENSITIVE_SERVICE_EMAIL_TEMPLATE_ID`. If `SENSITIVE_SERVICE_EMAIL` is `None`, logs an error (`"SENSITIVE_SERVICE_EMAIL not set"`) and proceeds with `None` as the address.

---

### Performance Platform (`app/clients/performance_platform/performance_platform_client.PerformancePlatformClient`)

**Purpose**: Sends aggregated notification statistics to the GDS Performance Platform API.

**Auth/config**:
- `PERFORMANCE_PLATFORM_ENABLED`: activates the client (`_active` flag).
- `PERFORMANCE_PLATFORM_URL`: base URL.
- `PERFORMANCE_PLATFORM_ENDPOINTS`: dict mapping dataset name → Bearer token (e.g. `{"foo": "my_token", "bar": "other_token"}`).

**Key methods**:

`send_stats_to_performance_platform(stats)`:
- No-op if `_active` is `False`.
- POSTs to `{url}/{stats["dataType"]}`.
- Sets `Authorization: Bearer {PERFORMANCE_PLATFORM_ENDPOINTS[dataType]}`.
- Calls `raise_for_status()` — raises `requests.HTTPError` on non-2xx responses.

**Behavioral contracts from tests**:
- Each dataset uses its own token; tokens are not interchangeable.
- The method is entirely suppressed when `_active = False`.

---

### Salesforce Account (`app/clients/salesforce/salesforce_account`)

**Purpose**: Resolves Salesforce Account IDs from service `organisation_notes` strings.

**Key methods**:

`get_org_name_from_notes(notes, index=ORG_NOTES_ORG_NAME_INDEX)`:
- Returns `None` if `notes` is `None`.
- Splits on `>`, strips whitespace from each segment, returns the segment at `index`.
- `ORG_NOTES_ORG_NAME_INDEX = 0` (organisation name), `ORG_NOTES_OTHER_NAME_INDEX = 1` (service name).
- Returns `""` when `notes` is `">"` (segment is empty after stripping).
- Supports arbitrary depth: index=2 returns the third segment.

`get_account_id_from_name(session, name, generic_account_id)`:
- Returns `generic_account_id` immediately if `name` is `None`, empty, or whitespace-only.
- Queries: `SELECT Id FROM Account where Name = '{name}' OR CDS_AccountNameFrench__c = '{name}' LIMIT 1`
- Single quotes in `name` are escaped to `\'` before interpolation.
- Returns `record["Id"]` if a row is found; returns `generic_account_id` if no row found.

---

### Salesforce Auth (`app/clients/salesforce/salesforce_auth`)

**Purpose**: Manages Salesforce OAuth session lifecycle.

**Key methods**:

`get_session(client_id, username, password, security_token, domain)`:
- Creates a `requests.Session` and mounts a `TimeoutAdapter` for both `https://` and `http://`.
- Returns `Salesforce(client_id=..., username=..., password=..., security_token=..., domain=..., session=session)`.
- Returns `None` on `SalesforceAuthenticationFailed` (does not raise).

`end_session(session)`:
- If `session.session_id` is not `None`: POSTs token revocation via `session.oauth2("revoke", {"token": session_id}, method="POST")`.
- If `session.session_id` is `None`: no-op.

---

### Salesforce Client (`app/clients/salesforce/salesforce_client.SalesforceClient`)

**Purpose**: High-level facade over the Salesforce contact and engagement modules. All public methods open a session, perform work, and close the session in a try/finally pattern.

**Auth/config** (read via `init_app`):
- `SALESFORCE_CLIENT_ID`, `SALESFORCE_USERNAME`, `SALESFORCE_PASSWORD`, `SALESFORCE_SECURITY_TOKEN`, `SALESFORCE_DOMAIN`
- `SALESFORCE_GENERIC_ACCOUNT_ID`: fallback account ID when org name cannot be resolved

**Key methods**:

| Method | Delegates to | Notes |
|--------|-------------|-------|
| `get_session()` | `salesforce_auth.get_session(...)` | Passes all 5 config credentials |
| `end_session(session)` | `salesforce_auth.end_session(session)` | |
| `contact_create(user)` | `salesforce_contact.create(session, user, {})` | |
| `contact_update(user)` | `salesforce_contact.update(session, user, {FirstName, LastName, Email})` | Splits `user.name` into first/last |
| `contact_update_account_id(session, service, user)` | Resolves account from `service.organisation_notes`, calls `salesforce_contact.update(session, user, {"AccountId": account_id})` | Returns `(account_id, contact_id)` |
| `engagement_create(service, user)` | `contact_update_account_id` then `salesforce_engagement.create(session, service, {}, account_id, contact_id)` | |
| `engagement_update(service, user, updates)` | `contact_update_account_id` then `salesforce_engagement.update(session, service, updates, account_id, contact_id)` | |
| `engagement_close(service)` | Looks up engagement by `service.id`; if found: updates with `CDS_Close_Reason__c="Service deleted by user"` and `StageName="Closed"`; if not found: skips update | No contact resolution needed |
| `engagement_add_contact_role(service, user)` | `contact_update_account_id` then `salesforce_engagement.contact_role_add(session, service, account_id, contact_id)` | |
| `engagement_delete_contact_role(service, user)` | `contact_update_account_id` then `salesforce_engagement.contact_role_delete(session, service, account_id, contact_id)` | |

**Behavioral contracts from tests**:
- `engagement_close` with no existing engagement: does not call `update`, still calls `end_session`.
- All methods that split `user.name` pass `FirstName` and `LastName` as separate fields.

---

### Salesforce Contact (`app/clients/salesforce/salesforce_contact`)

**Purpose**: CRUD operations on Salesforce `Contact` objects.

**Key methods**:

`create(session, user, custom_fields)`:
- Base payload: `FirstName` (first word of `user.name`, `""` for single-word names), `LastName` (remainder), `Title = "created by Notify API"`, `CDS_Contact_ID__c = str(user.id)`, `Email = user.email_address`.
- `custom_fields` is merged in, overriding any base field.
- Calls `session.Contact.create(payload, headers={"Sforce-Duplicate-Rule-Header": "allowSave=true"})`.
- Returns `response["id"]` on success (`{"success": True}`).
- Returns `None` on `{"success": False}` or any exception.

`update(session, user, fields)`:
- Looks up existing contact via `get_contact_by_user_id(session, str(user.id))`.
- If found: calls `session.Contact.update(contact_id, fields, headers={"Sforce-Duplicate-Rule-Header": "allowSave=true"})`; returns `contact_id`.
- If not found: calls `create(session, user, fields)` and returns the new contact ID.

`get_contact_by_user_id(session, user_id)`:
- Returns `None` if `user_id` is `None`, empty, or whitespace-only.
- Queries: `SELECT Id, FirstName, LastName, AccountId FROM Contact WHERE CDS_Contact_ID__c = '{user_id}' LIMIT 1`
- Returns the record dict or `None`.

---

### Salesforce Engagement (`app/clients/salesforce/salesforce_engagement`)

**Purpose**: CRUD operations on Salesforce `Opportunity` objects and their `OpportunityContactRole` records.

**Constants** (used in Opportunity creation): `ENGAGEMENT_STAGE_TRIAL`, `ENGAGEMENT_TYPE`, `ENGAGEMENT_TEAM`, `ENGAGEMENT_PRODUCT`.

**Config keys**: `SALESFORCE_ENGAGEMENT_RECORD_TYPE`, `SALESFORCE_ENGAGEMENT_STANDARD_PRICEBOOK_ID`, `SALESFORCE_ENGAGEMENT_PRODUCT_ID`.

**Key methods**:

`create(session, service, custom_fields, account_id, contact_id)`:
- Returns `None` immediately if `account_id` or `contact_id` is `None`.
- Creates `Opportunity` with:
  ```
  Name                       = service.name
  AccountId                  = account_id
  ContactId                  = contact_id
  CDS_Opportunity_Number__c  = str(service.id)
  Notify_Organization_Other__c = None
  CloseDate                  = today (YYYY-MM-DD)
  RecordTypeId               = SALESFORCE_ENGAGEMENT_RECORD_TYPE
  StageName                  = ENGAGEMENT_STAGE_TRIAL
  Type                       = ENGAGEMENT_TYPE
  CDS_Lead_Team__c           = ENGAGEMENT_TEAM
  Product_to_Add__c          = ENGAGEMENT_PRODUCT
  ```
- `custom_fields` overrides any of the above.
- Returns `None` on `{"success": False}`; on success also creates `OpportunityLineItem` (`Quantity=1, UnitPrice=0`) and returns the Opportunity ID.

`update(session, service, fields, account_id, contact_id)`:
- Looks up existing engagement via `get_engagement_by_service_id`.
- If found: updates it; returns the engagement ID or `None` on failure.
- If not found: calls `create(session, service, fields, account_id, contact_id)`.

`contact_role_add(session, service, account_id, contact_id)`:
- Looks up engagement; if missing, first calls `create` to establish one.
- Creates `OpportunityContactRole(ContactId=contact_id, OpportunityId=opportunity_id)`.
- Returns `None` always.

`contact_role_delete(session, service, account_id, contact_id)`:
- Looks up engagement then contact role.
- Deletes the `OpportunityContactRole` if found; otherwise no-op.
- Returns `None` always.

`get_engagement_by_service_id(session, service_id)`:
- Returns `None` for blank/None/whitespace input.
- Queries: `SELECT Id, Name, ContactId, AccountId FROM Opportunity where CDS_Opportunity_Number__c = '{service_id}' LIMIT 1`

`get_engagement_contact_role(session, opportunity_id, contact_id)`:
- Returns `None` if either argument is blank/None/whitespace.
- Queries: `SELECT Id, OpportunityId, ContactId FROM OpportunityContactRole WHERE OpportunityId = '{opp_id}' AND ContactId = '{contact_id}' LIMIT 1`

`engagement_maxlengths(fields)`:
- Truncates `Name` field to a maximum of 120 characters if present and over limit.
- All other fields are returned unchanged.

---

### Salesforce Utils (`app/clients/salesforce/salesforce_utils`)

**Purpose**: Shared query helpers and data utilities for Salesforce client modules.

**Key functions**:

`get_name_parts(name)` → `{"first": str, "last": str}`:
- Empty string → `{"first": "", "last": ""}`.
- Single word → `{"first": "", "last": name}`.
- Multi-word → `{"first": first_word, "last": everything_after_first_space}` (e.g. `"Gandalf The Grey"` → first=`"Gandalf"`, last=`"The Grey"`).

`query_one(session, query)`:
- Calls `session.query(query)`.
- Returns `records[0]` when `totalSize == 1`.
- Returns `None` when `totalSize != 1` or when the response has no `records` key.

`query_param_sanitize(value)`:
- Escapes single quotes with a backslash for safe SOQL interpolation.

`parse_result(response, type)`:
- `type="int"`: returns `True` iff `200 <= response <= 299`.
- `type="dict"`: returns `True` iff `response.get("success") is True`; `False` for `{"success": False}` or `{}`.

---

## SNS Receipt Processing

From `tests/app/celery/test_process_sns_receipts_tasks.py`.

### `process_sns_results`

**Input**: An SNS delivery receipt callback dict (produced by `sns_success_callback()` or `sns_failed_callback()`).

#### Success receipt

| Input condition | Expected outcome |
|----------------|------------------|
| Notification found, status=`sent`, `sent_by="sns"` | Status → `delivered`; `provider_response = "Message has been accepted by phone carrier"` |
| Service has a callback API configured | `send_delivery_status_to_service.apply_async([notification_id, signed_data, service_id], queue="service-callbacks")` called |

Side effects on success:
- `statsd_client.incr("callback.sns.delivered")`
- `statsd_client.timing_with_dates("callback.sns.elapsed-time", now, notification.sent_at)`
- `annual_limit_client.increment_sms_delivered(service_id)` called once

#### Failure receipt — provider response → notification status mapping

| Provider response string | Notification status |
|--------------------------|---------------------|
| `"Blocked as spam by phone carrier"` | `permanent-failure` |
| `"Destination is on a blocked list"` | `permanent-failure` |
| `"Invalid phone number"` | `permanent-failure` |
| `"Message body is invalid"` | `permanent-failure` |
| `"Phone is currently unreachable/unavailable"` | `permanent-failure` |
| `"Unknown error attempting to reach phone"` | `permanent-failure` |
| `"Unhandled provider"` | `permanent-failure` |
| `"Phone carrier has blocked this message"` | `temporary-failure` |
| `"Phone carrier is currently unreachable/unavailable"` | `temporary-failure` |
| `"Phone has blocked SMS"` | `temporary-failure` |
| `"Phone is on a blocked list"` | `temporary-failure` |
| `"This delivery would exceed max price"` | `temporary-failure` |
| `"Phone number is opted out"` | `technical-failure` |
| Any unrecognized string | `technical-failure` + `logger.warning` logged |

All failure cases: `provider_response` field saved to notification; `annual_limit_client.increment_sms_failed(service_id)` called.

#### Missing notification (retry logic)

| Condition | Behavior |
|-----------|----------|
| Notification not found in DB | `process_sns_results.retry()` called |
| `MaxRetriesExceededError` | `logger.warning("notification not found for SNS reference: {ref} (update to delivered). Giving up.")` |
| DB error during `_update_notification_status` | `process_sns_results.retry()` called |

#### Wrong-provider guard

If the notification's `sent_by` is not `"sns"` (e.g. `"pinpoint"`): logs an exception, does not update notification status, returns `None`.

#### Annual limit integration

| Scenario | Behavior |
|----------|----------|
| Already seeded today | Calls `increment_sms_delivered` or `increment_sms_failed` as appropriate |
| Not yet seeded today (first callback) | Calls `seed_annual_limit_notifications(service_id, data)` with current counts; does NOT call `increment_*` |
| `FF_USE_BILLABLE_UNITS` enabled | Seed data includes additional billable unit fields (e.g. `sms_billable_units_delivered_today`, `total_sms_billable_units_fiscal_year_to_yesterday`) |

**Nightly reset**: `create_nightly_notification_status_for_day(date)` resets all annual limit counters to `0` for every service that had notifications that day.

---

## Performance Platform Processing

From `tests/app/performance_platform/test_processing_time.py`.

### `send_processing_time_to_performance_platform(date)`

**Purpose**: Aggregates notification processing times for a given date and posts them to the Performance Platform.

**Behavior**:
- The query window maps `date` at local-timezone midnight to a UTC start time (e.g. date `2016-10-17` in EST/UTC-4 → `datetime(2016, 10, 17, 4, 0)` UTC).
- Counts only notifications whose `created_at` falls within the date window and whose `sent_at` is set (i.e. actually sent).
- Notifications from other days are excluded.
- Makes exactly two calls to `send_processing_time_data`:
  1. `send_processing_time_data(start_time_utc, "messages-total", total_count)` — all notifications sent that day.
  2. `send_processing_time_data(start_time_utc, "messages-within-10-secs", fast_count)` — notifications where `(sent_at - created_at) <= 10 seconds`.

### `send_processing_time_data(start_time, status, count)`

**Purpose**: Formats and sends a single stat record to the Performance Platform.

**Payload sent to `performance_platform_client.send_stats_to_performance_platform()`**:

| Field | Value |
|-------|-------|
| `dataType` | `"processing-time"` |
| `service` | `"govuk-notify"` |
| `period` | `"day"` |
| `status` | passed `status` argument |
| `_timestamp` | `start_time` formatted as `"YYYY-MM-DDTHH:MM:SS"` (e.g. `"2016-10-16T00:00:00"`) |
| `count` | passed `count` argument |
| `_id` | base64 of `"{_timestamp}{service}{status}{dataType}{period}"` |

**Example `_id` derivation**:
- Input: `"2016-10-16T00:00:00"` + `"govuk-notify"` + `"foo"` + `"processing-time"` + `"day"`
- base64 → `"MjAxNi0xMC0xNlQwMDowMDowMGdvdnVrLW5vdGlmeWZvb3Byb2Nlc3NpbmctdGltZWRheQ=="`
