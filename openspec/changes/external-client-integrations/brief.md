## Source Files
- spec/behavioral-spec/external-clients.md

---

## Airtable Client

### `AirtableTableMixin` (base class)

#### Methods
| Method | Behavior |
|--------|----------|
| `table_exists()` | Checks if table named in `Meta.table_name` exists in the Airtable base; returns `bool`; raises `AttributeError("Model must have a Meta attribute")` when `Meta` is absent |
| `get_table_schema()` | Abstract; raises `NotImplementedError("Subclasses must implement get_table_schema")` |
| `create_table()` | Calls `meta.base.create_table(name, fields=[...get_table_schema()...])` using lowercase `meta` attribute; raises `AttributeError("Model must have a meta attribute")` when `meta` is absent |

---

### `NewsletterSubscriber`

#### Config keys
- `AIRTABLE_API_KEY`
- `AIRTABLE_NEWSLETTER_BASE_ID`
- `AIRTABLE_NEWSLETTER_TABLE_NAME`

#### Defaults on construction
- `language = Languages.EN.value` → `"en"`
- `status = Statuses.UNCONFIRMED.value` → `"unconfirmed"`

#### Enums
- `Languages`: `EN = "en"`, `FR = "fr"`
- `Statuses`: `UNCONFIRMED = "unconfirmed"`, `SUBSCRIBED = "subscribed"`, `UNSUBSCRIBED = "unsubscribed"`

#### Airtable table schema (7 fields)
`Email`, `Language`, `Status`, `Created At`, `Confirmed At`, `Unsubscribed At`, `Has Resubscribed`
- `Language` and `Status` are single-select fields with enum-derived choices

#### Methods
| Method | Behavior |
|--------|----------|
| `from_email(email)` | Formula `{Email} = 'email'`; returns first match; raises `HTTPError(404)` if no records; propagates other exceptions unchanged |
| `save_unconfirmed_subscriber()` | Sets `status = UNCONFIRMED`, `created_at = datetime.now()`, calls `save()`; returns result of `save()` |
| `confirm_subscription(has_resubscribed=False)` | Sets `status = SUBSCRIBED`, `confirmed_at = datetime.now()`, `has_resubscribed = has_resubscribed`, calls `save()` |
| `unsubscribe_user()` | Sets `status = UNSUBSCRIBED`, `unsubscribed_at = datetime.now()`, `confirmed_at = None`, calls `save()` |
| `update_language(lang)` | Validates `lang` is in `Languages` enum, sets `language = lang`, calls `save()` |

#### Invariants
- Table auto-created on first `save()` if `table_exists()` returns `False`
- `from_email` uses exact formula lookup (not a scan)
- `confirm_subscription()` does **not** set `has_resubscribed=True` by default; caller must pass explicitly
- `unsubscribe_user()` clears `confirmed_at` to `None` — not merely un-sets but explicitly sets nil

---

### `LatestNewsletterTemplate`

#### Config keys
- `AIRTABLE_API_KEY`
- `AIRTABLE_NEWSLETTER_BASE_ID`
- `AIRTABLE_CURRENT_NEWSLETTER_TEMPLATES_TABLE_NAME` (default `"Newsletter Templates"`)

#### Table schema (2 fields, both `singleLineText`)
`(EN) Template ID`, `(FR) Template ID`

#### Methods
| Method | Behavior |
|--------|----------|
| `get_latest_newsletter_templates()` | Creates table if absent; queries with `sort=["-Created at"], max_records=1`; returns single result; raises `HTTPError(404)` if no records exist |

#### Invariants
- Table created on `save()` and on `get_latest_newsletter_templates()` when table absent

---

## Newsletter Handlers (wired in this change)

Three routes call the Airtable client and are implemented here (the 4 additional undocumented routes are in `newsletter-endpoints`):

| Route | Behavior |
|-------|----------|
| `POST /newsletter/unconfirmed-subscriber` | Calls `save_unconfirmed_subscriber()`; returns 201; admin JWT |
| `GET /newsletter/confirm/{subscriber_id}` | Calls `confirm_subscription()`; returns 200; admin JWT |
| `GET /newsletter/unsubscribe/{subscriber_id}` | Calls `unsubscribe_user()`; returns 200; admin JWT |

---

## Freshdesk Client

### Config keys
- `FRESH_DESK_ENABLED` (bool feature flag)
- `FRESH_DESK_API_KEY`
- `FRESHDESK_URL`
- `CONTACT_FORM_EMAIL_ADDRESS` (fallback when Freshdesk down)
- `SENSITIVE_SERVICE_EMAIL`
- `CONTACT_FORM_SENSITIVE_SERVICE_EMAIL_TEMPLATE_ID`

### Auth
`Authorization: Basic {base64(api_key + ":" + "x")}`

### Ticket endpoint
`POST {FRESHDESK_URL}/api/v2/tickets`

### Ticket base payload
```json
{
  "product_id": 42,
  "email": "<contact_request.email_address>",
  "priority": 1,
  "status": 2,
  "tags": []
}
```

### Ticket content by `support_type`
| `support_type` | `subject` | `description` key fields |
|----------------|-----------|--------------------------|
| `"demo"` | `friendly_support_type` | user name/email, department/org, program/service, intended recipients, main use case, use case details |
| `"go_live_request"` | `"Support Request"` | service name + timestamp, org, recipients, purposes (with "other"), email/SMS volume (daily/yearly counts), service URL |
| `"branding_request"` | `"Branding request"` | bilingual EN + FR separated by `<hr><br>`: service id/name, org id/name, logo filename, logo name, alt text EN/FR |
| `"new_template_category_request"` | `"New template category request"` | bilingual: service id, template category name, template id link |
| Any other / empty | `friendly_support_type` or `"Support Request"` | Empty, or appends `user_profile` if present |

### Return value
HTTP status code of Freshdesk API response (e.g. `201`)

### Error conditions
| Condition | Behavior |
|-----------|----------|
| `FRESH_DESK_ENABLED = False` | Returns `201` immediately; no HTTP call; no fallback email |
| `RequestException` from POST | Calls `email_freshdesk_ticket()` as fallback; still returns `201` |

### Fallback methods
- `email_freshdesk_ticket_freshdesk_down()`: calls `persist_notification()` + `send_notification_to_queue()` to `CONTACT_FORM_EMAIL_ADDRESS`
- `email_freshdesk_ticket_pt_service()`: sends to `SENSITIVE_SERVICE_EMAIL` using `CONTACT_FORM_SENSITIVE_SERVICE_EMAIL_TEMPLATE_ID`; if `SENSITIVE_SERVICE_EMAIL` is `None`, logs error `"SENSITIVE_SERVICE_EMAIL not set"` and continues with `None`

---

## Salesforce Client

### Config keys
- `SALESFORCE_CLIENT_ID`, `SALESFORCE_USERNAME`, `SALESFORCE_PASSWORD`, `SALESFORCE_SECURITY_TOKEN`, `SALESFORCE_DOMAIN`
- `SALESFORCE_GENERIC_ACCOUNT_ID` (fallback account ID)
- `SALESFORCE_ENGAGEMENT_RECORD_TYPE`
- `SALESFORCE_ENGAGEMENT_STANDARD_PRICEBOOK_ID`
- `SALESFORCE_ENGAGEMENT_PRODUCT_ID`

### Session management
- `get_session()`: creates `requests.Session` with `TimeoutAdapter` mounted for `http://` and `https://`; returns `Salesforce(...)` object; returns `None` on `SalesforceAuthenticationFailed` (no raise)
- `end_session(session)`: if `session.session_id` not `None`, POSTs token revocation via `session.oauth2("revoke", {"token": session_id})`; else no-op

### Account resolution
`get_org_name_from_notes(notes, index=0)`:
- Returns `None` if `notes` is `None`
- Splits on `>`, strips each segment, returns segment at `index`
- `ORG_NOTES_ORG_NAME_INDEX = 0`, `ORG_NOTES_OTHER_NAME_INDEX = 1`
- Returns `""` when notes is `">"` (empty after strip)

`get_account_id_from_name(session, name, generic_account_id)`:
- Returns `generic_account_id` if `name` is `None`, empty, or whitespace-only
- SOQL: `SELECT Id FROM Account where Name = '{name}' OR CDS_AccountNameFrench__c = '{name}' LIMIT 1`
- Single quotes in `name` escaped to `\'` before interpolation
- Returns `record["Id"]` if found; `generic_account_id` if not

### Contact
`create(session, user, custom_fields)`:
- Base payload: `FirstName` (first word of `user.name`; `""` for single-word names), `LastName` (remainder), `Title = "created by Notify API"`, `CDS_Contact_ID__c = str(user.id)`, `Email = user.email_address`
- `custom_fields` overrides base fields
- POSTs with `{"Sforce-Duplicate-Rule-Header": "allowSave=true"}`
- Returns `response["id"]` on `{"success": True}`; `None` on `{"success": False}` or exception

`update(session, user, fields)`:
- Looks up by `CDS_Contact_ID__c`; if found: updates and returns `contact_id`; if not found: calls `create()` and returns new ID

`get_contact_by_user_id(session, user_id)`:
- Returns `None` for empty/None/whitespace input
- SOQL: `SELECT Id, FirstName, LastName, AccountId FROM Contact WHERE CDS_Contact_ID__c = '{user_id}' LIMIT 1`

### Engagement (Opportunity)
`create(session, service, custom_fields, account_id, contact_id)`:
- Returns `None` immediately if `account_id` or `contact_id` is `None`
- Creates `Opportunity`: `Name = service.name` (max 120 chars via `engagement_maxlengths`), `AccountId`, `ContactId`, `CDS_Opportunity_Number__c = str(service.id)`, `CloseDate = today`, `RecordTypeId = SALESFORCE_ENGAGEMENT_RECORD_TYPE`, `StageName = ENGAGEMENT_STAGE_TRIAL`, `Type = ENGAGEMENT_TYPE`, `CDS_Lead_Team__c = ENGAGEMENT_TEAM`, `Product_to_Add__c = ENGAGEMENT_PRODUCT`
- On success: also creates `OpportunityLineItem(Quantity=1, UnitPrice=0)` and returns Opportunity ID
- Returns `None` on `{"success": False}`

`update(session, service, fields, account_id, contact_id)`:
- Looks up by `service_id`; updates if found; calls `create()` if not
- Returns engagement ID or `None`

`engagement_close(service)`:
- Finds Opportunity by `service.id`; if found: updates `CDS_Close_Reason__c = "Service deleted by user"`, `StageName = "Closed"`
- If not found: skips update; `end_session` still called

`contact_role_add(session, service, account_id, contact_id)`:
- Creates engagement if none exists; creates `OpportunityContactRole(ContactId, OpportunityId)`; always returns `None`

`contact_role_delete(session, service, account_id, contact_id)`:
- Looks up role; deletes if found; no-op if not; always returns `None`

`engagement_maxlengths(fields)`:
- Truncates `Name` to 120 characters if present and over limit; other fields unchanged

### Salesforce utils
`get_name_parts(name)` → `{"first": str, "last": str}`:
- Empty → `{"first": "", "last": ""}`, single word → `{"first": "", "last": name}`, multi-word → first word / everything after first space

`query_one(session, query)`:
- Returns `records[0]` when `totalSize == 1`; `None` when `totalSize != 1` or no `records` key

### SalesforceClient facade (all public methods: open session → work → defer close)
| Method | Delegates to | Notes |
|--------|-------------|-------|
| `contact_create(user)` | `salesforce_contact.create(session, user, {})` | |
| `contact_update(user)` | `salesforce_contact.update(session, user, {FirstName, LastName, Email})` | Splits `user.name` |
| `contact_update_account_id(session, service, user)` | Resolves account from `organisation_notes`, calls `salesforce_contact.update` | Returns `(account_id, contact_id)` |
| `engagement_create(service, user)` | `contact_update_account_id` + `salesforce_engagement.create` | |
| `engagement_update(service, user, updates)` | `contact_update_account_id` + `salesforce_engagement.update` | |
| `engagement_close(service)` | Find Opportunity by `service.id`; update with Close Reason + Closed | No contact resolution |
| `engagement_add_contact_role(service, user)` | `contact_update_account_id` + `salesforce_engagement.contact_role_add` | |
| `engagement_delete_contact_role(service, user)` | `contact_update_account_id` + `salesforce_engagement.contact_role_delete` | |

Invariants:
- `engagement_close` with no existing engagement: does NOT call update; still calls `end_session`
- All methods splitting `user.name` pass `FirstName` and `LastName` as separate fields

---

## Cronitor Client

### Purpose
Sends GET heartbeat pings to Cronitor monitoring service after successful nightly task runs.

### Config keys
- `CRONITOR_ENABLED` (bool)
- `CRONITOR_KEYS` (map: task name → monitor key)
- Base URL: `https://cronitor.link/ping/{api_key}/{monitor_key}?state=complete`

### Behavior
- Called at the end of successful nightly beat tasks only (not on failure)
- When `CRONITOR_ENABLED = False`: no-op; no HTTP call
- Errors are logged and swallowed; never affect task success/failure status

---

## Business Rules
- All four external clients must be interface-based (injectable mocks for unit tests)
- Credentials must come from the application `Config` struct (not read from env in client code)
- Salesforce sessions must always be closed in `defer`/`finally` even on error
- Airtable tables are auto-provisioned on first use (create-if-absent)
- Freshdesk failures must never surface to the caller — silent fallback to email
- Cronitor ping failures must not affect task success/failure status
