## Requirements

### Requirement: AirtableTableMixin base class
`AirtableTableMixin` SHALL provide `table_exists()`, abstract `get_table_schema()`, and `create_table()` as the shared foundation for all Airtable model types.

#### Scenario: table_exists returns true when table is present
- **WHEN** `table_exists()` is called and the Airtable base contains a table matching `Meta.table_name`
- **THEN** returns `true`

#### Scenario: table_exists raises when Meta is absent
- **WHEN** `table_exists()` is called on a model with no `Meta` attribute
- **THEN** raises `AttributeError("Model must have a Meta attribute")`

#### Scenario: get_table_schema raises NotImplementedError
- **WHEN** the base-class `get_table_schema()` is called directly without a subclass override
- **THEN** raises `NotImplementedError("Subclasses must implement get_table_schema")`

#### Scenario: create_table delegates to meta.base
- **WHEN** `create_table()` is called on a valid model with a `meta` attribute
- **THEN** calls `meta.base.create_table(name=Meta.table_name, fields=[...get_table_schema()...])`

#### Scenario: create_table raises when lowercase meta is absent
- **WHEN** `create_table()` is called on a model with no lowercase `meta` attribute
- **THEN** raises `AttributeError("Model must have a meta attribute")`

---

### Requirement: NewsletterSubscriber construction defaults
`NewsletterSubscriber` SHALL default `language` to `"en"` and `status` to `"unconfirmed"` when no explicit values are supplied.

#### Scenario: Default language and status on construction
- **WHEN** a `NewsletterSubscriber` is constructed with only an email address
- **THEN** `language == "en"` and `status == "unconfirmed"`

---

### Requirement: NewsletterSubscriber.from_email formula lookup
`from_email(email)` SHALL query Airtable with formula `{Email} = '{email}'` and return the first matching record.

#### Scenario: Subscriber found by email
- **WHEN** `from_email("user@example.com")` is called and a matching record exists
- **THEN** returns the first Airtable record with that email address

#### Scenario: Subscriber not found raises HTTPError 404
- **WHEN** `from_email("user@example.com")` is called and no records are returned
- **THEN** raises `HTTPError` with status code `404`

#### Scenario: Non-404 Airtable error propagates unchanged
- **WHEN** the Airtable API returns a non-404 error (e.g. 503 Service Unavailable)
- **THEN** the exception is propagated to the caller without wrapping

---

### Requirement: NewsletterSubscriber state transitions
The `save_unconfirmed_subscriber`, `confirm_subscription`, `unsubscribe_user`, and `update_language` methods SHALL update the subscriber fields and call `save()`.

#### Scenario: save_unconfirmed_subscriber sets UNCONFIRMED status
- **WHEN** `save_unconfirmed_subscriber()` is called
- **THEN** sets `status = "unconfirmed"`, `created_at = now()`, and calls `save()`

#### Scenario: confirm_subscription without resubscribe flag
- **WHEN** `confirm_subscription()` is called without arguments
- **THEN** sets `status = "subscribed"`, `confirmed_at = now()`, `has_resubscribed = false`, and calls `save()`

#### Scenario: confirm_subscription with has_resubscribed=true
- **WHEN** `confirm_subscription(has_resubscribed=true)` is called
- **THEN** sets `has_resubscribed = true` in the saved Airtable record

#### Scenario: unsubscribe_user clears confirmed_at to None
- **WHEN** `unsubscribe_user()` is called
- **THEN** sets `status = "unsubscribed"`, `unsubscribed_at = now()`, and sets `confirmed_at = None` (explicitly nil, not just unset)

#### Scenario: update_language validates enum and saves
- **WHEN** `update_language("fr")` is called
- **THEN** sets `language = "fr"` and calls `save()`

#### Scenario: update_language rejects invalid value
- **WHEN** `update_language("de")` is called with a value not in the Languages enum
- **THEN** raises a validation error before calling `save()`

#### Scenario: Table auto-created on first save when absent
- **WHEN** `save()` is called and `table_exists()` returns false
- **THEN** `create_table()` is called before the Airtable record is written

---

### Requirement: LatestNewsletterTemplate retrieval
`get_latest_newsletter_templates()` SHALL return the most recently created newsletter template record, auto-creating the table if it does not exist.

#### Scenario: Latest template returned by creation date
- **WHEN** `get_latest_newsletter_templates()` is called and at least one record exists
- **THEN** returns the record with the most recent `Created at` value (sort descending, max_records=1)

#### Scenario: No templates raises HTTPError 404
- **WHEN** `get_latest_newsletter_templates()` is called and no records exist
- **THEN** raises `HTTPError` with status code `404`

#### Scenario: Table created if absent before query
- **WHEN** `get_latest_newsletter_templates()` is called and `table_exists()` returns false
- **THEN** calls `create_table()` before executing the Airtable query

---

### Requirement: POST /newsletter/unconfirmed-subscriber handler
`POST /newsletter/unconfirmed-subscriber` SHALL create a new unconfirmed subscriber in Airtable and return HTTP 201.

#### Scenario: New subscriber created successfully
- **WHEN** `POST /newsletter/unconfirmed-subscriber` is called with a valid email address and admin JWT
- **THEN** calls `AirtableClient.SaveUnconfirmedSubscriber(email)` and returns HTTP 201

#### Scenario: Unauthenticated request rejected
- **WHEN** `POST /newsletter/unconfirmed-subscriber` is called without an admin JWT
- **THEN** returns HTTP 401

---

### Requirement: GET /newsletter/confirm/{subscriber_id} handler
`GET /newsletter/confirm/{subscriber_id}` SHALL confirm the subscriber's subscription in Airtable and return HTTP 200.

#### Scenario: Subscription confirmed successfully
- **WHEN** `GET /newsletter/confirm/{subscriber_id}` is called with admin JWT
- **THEN** calls `AirtableClient.ConfirmSubscription(subscriber_id)` and returns HTTP 200

#### Scenario: Subscriber not found returns 404
- **WHEN** `GET /newsletter/confirm/{subscriber_id}` is called and Airtable returns a 404 HTTPError
- **THEN** returns HTTP 404

---

### Requirement: GET /newsletter/unsubscribe/{subscriber_id} handler
`GET /newsletter/unsubscribe/{subscriber_id}` SHALL mark the subscriber as unsubscribed in Airtable and return HTTP 200.

#### Scenario: Subscriber unsubscribed successfully
- **WHEN** `GET /newsletter/unsubscribe/{subscriber_id}` is called with admin JWT
- **THEN** calls `AirtableClient.UnsubscribeUser(subscriber_id)` and returns HTTP 200

#### Scenario: Subscriber not found returns 404
- **WHEN** `GET /newsletter/unsubscribe/{subscriber_id}` is called and Airtable returns 404
- **THEN** returns HTTP 404

---

### Requirement: Freshdesk ticket creation by support type
`CreateTicket()` SHALL POST to `{FRESHDESK_URL}/api/v2/tickets` with Basic auth (`base64(api_key:x)`) and type-specific subject and description.

#### Scenario: Demo request ticket created
- **WHEN** `CreateTicket()` is called with `support_type = "demo"`
- **THEN** POSTs with `subject = friendly_support_type` and description containing user name/email, department/org, program/service, intended recipients, and use case details

#### Scenario: Go-live request ticket created
- **WHEN** `CreateTicket()` is called with `support_type = "go_live_request"`
- **THEN** POSTs with `subject = "Support Request"` and description containing service name+timestamp, org, recipients, purposes, email/SMS volume (daily/yearly counts), and service URL

#### Scenario: Branding request contains bilingual content
- **WHEN** `CreateTicket()` is called with `support_type = "branding_request"`
- **THEN** POSTs with `subject = "Branding request"` and description with EN and FR sections separated by `<hr><br>`, each containing service id/name, org id/name, logo filename, logo name, and alt text

#### Scenario: New template category request ticket
- **WHEN** `CreateTicket()` is called with `support_type = "new_template_category_request"`
- **THEN** POSTs with `subject = "New template category request"` and bilingual description containing service id, template category name, and template id link

#### Scenario: Ticket payload always contains base fields
- **WHEN** any `CreateTicket()` call is made
- **THEN** the payload includes `product_id = 42`, `priority = 1`, `status = 2`, `tags = []`, and `email = contact_request.email_address`

#### Scenario: Returns Freshdesk HTTP status code
- **WHEN** Freshdesk returns HTTP 201
- **THEN** `CreateTicket()` returns `201` to the caller

---

### Requirement: Freshdesk feature flag and fallback
When `FRESH_DESK_ENABLED = false`, no HTTP call is made. On HTTP errors, a fallback email is queued silently.

#### Scenario: Feature disabled returns 201 with no HTTP call
- **WHEN** `FRESH_DESK_ENABLED = false` and `CreateTicket()` is called
- **THEN** returns `201` immediately without making any HTTP request to Freshdesk

#### Scenario: HTTP error triggers silent fallback email
- **WHEN** the Freshdesk POST raises a `RequestException`
- **THEN** queues a fallback notification via `persist_notification()` + `send_notification_to_queue()` to `CONTACT_FORM_EMAIL_ADDRESS`, and still returns `201` to the caller

---

### Requirement: Salesforce session management
`get_session()` SHALL create an authenticated Salesforce session with a timeout adapter; `end_session()` SHALL revoke the token if a session_id is present.

#### Scenario: Session created with TimeoutAdapter
- **WHEN** `get_session()` is called with valid Salesforce credentials
- **THEN** returns a `Salesforce` object with a `TimeoutAdapter` mounted on both `http://` and `https://`

#### Scenario: Authentication failure returns nil
- **WHEN** `get_session()` is called and `SalesforceAuthenticationFailed` is raised
- **THEN** returns `nil` without propagating the error

#### Scenario: Token revoked on end_session
- **WHEN** `end_session(session)` is called and `session.session_id` is not nil
- **THEN** POSTs token revocation via `session.oauth2("revoke", {"token": session_id})`

#### Scenario: end_session is a no-op when session_id is nil
- **WHEN** `end_session(session)` is called and `session.session_id` is nil
- **THEN** no HTTP call is made

---

### Requirement: Salesforce account resolution
`get_account_id_from_name()` SHALL query Salesforce Account objects by English and French name and fall back to the generic account ID when no match exists or the name is empty.

#### Scenario: Account found by English or French name
- **WHEN** `get_account_id_from_name(session, "Health Canada", generic_id)` is called and a matching Account exists
- **THEN** returns `record["Id"]` from Salesforce

#### Scenario: Account not found returns generic ID
- **WHEN** `get_account_id_from_name(session, "Unknown Org", generic_id)` is called and no Account matches
- **THEN** returns `generic_account_id`

#### Scenario: Empty or nil name returns generic ID immediately
- **WHEN** `get_account_id_from_name(session, "", generic_id)` is called
- **THEN** returns `generic_account_id` without executing any SOQL query

#### Scenario: Single quotes in name are escaped in SOQL
- **WHEN** `get_account_id_from_name(session, "O'Brien's Agency", generic_id)` is called
- **THEN** the SOQL query contains `O\'Brien\'s Agency` (escaped single quotes)

#### Scenario: get_org_name_from_notes splits on > character
- **WHEN** `get_org_name_from_notes("Org Name > Service Name", 0)` is called
- **THEN** returns `"Org Name"` (trimmed segment at index 0)

---

### Requirement: Salesforce contact CRUD
`salesforce_contact.create()` and `update()` SHALL manage `Contact` objects using `CDS_Contact_ID__c` as the external identifier.

#### Scenario: Contact created with base payload and duplicate-rule header
- **WHEN** `create(session, user, {})` is called
- **THEN** creates a Contact with `FirstName`, `LastName`, `Title = "created by Notify API"`, `CDS_Contact_ID__c = user.id`, `Email = user.email_address`, and the header `Sforce-Duplicate-Rule-Header: allowSave=true`

#### Scenario: Single-word name gets empty FirstName
- **WHEN** `user.name = "Madonna"` is supplied to `create()`
- **THEN** `FirstName = ""` and `LastName = "Madonna"`

#### Scenario: Multi-word name splits at first space
- **WHEN** `user.name = "Gandalf The Grey"` is supplied
- **THEN** `FirstName = "Gandalf"` and `LastName = "The Grey"`

#### Scenario: Contact update finds existing by CDS_Contact_ID__c
- **WHEN** `update(session, user, fields)` is called and a Contact with `CDS_Contact_ID__c = user.id` exists
- **THEN** updates the existing Contact with `fields` and returns the contact ID

#### Scenario: Contact update creates new when not found
- **WHEN** `update(session, user, fields)` is called and no Contact matches `CDS_Contact_ID__c`
- **THEN** calls `create(session, user, fields)` and returns the new contact ID

---

### Requirement: Salesforce engagement lifecycle
`salesforce_engagement.create()` SHALL create an Opportunity and an OpportunityLineItem; update, close, and role methods SHALL manage the full engagement lifecycle.

#### Scenario: Engagement created with all required fields
- **WHEN** `create(session, service, {}, account_id, contact_id)` is called with non-nil IDs
- **THEN** creates an Opportunity with `Name = service.name`, `CDS_Opportunity_Number__c = service.id`, `StageName = ENGAGEMENT_STAGE_TRIAL`, `CloseDate = today`, and also creates `OpportunityLineItem(Quantity=1, UnitPrice=0)`

#### Scenario: Engagement creation skipped when account_id or contact_id is nil
- **WHEN** `create(session, service, {}, nil, contact_id)` is called
- **THEN** returns `nil` immediately without any Salesforce API call

#### Scenario: Engagement Name truncated to 120 characters
- **WHEN** `service.name` is 150 characters long
- **THEN** the `Name` field in the Opportunity is truncated to exactly 120 characters by `engagement_maxlengths()`

#### Scenario: Engagement closed with reason
- **WHEN** `engagement_close(service)` is called and an Opportunity for `service.id` exists
- **THEN** updates the Opportunity with `CDS_Close_Reason__c = "Service deleted by user"` and `StageName = "Closed"`

#### Scenario: Engagement close is no-op when not found
- **WHEN** `engagement_close(service)` is called and no Opportunity for `service.id` exists
- **THEN** no Opportunity update is attempted; `end_session` is still called

---

### Requirement: SalesforceClient facade session lifecycle
Every public method on `SalesforceClient` SHALL open a session, perform work, and close the session in a defer/finally pattern — even when an error occurs.

#### Scenario: Session always closed after engagement_create
- **WHEN** `engagement_create(service, user)` is called successfully
- **THEN** `end_session` is called after the operation completes

#### Scenario: Session closed even when operation raises
- **WHEN** `engagement_create(service, user)` raises an error during Salesforce contact creation
- **THEN** `end_session` is still called before the error propagates

#### Scenario: contact_update_account_id returns both resolved IDs
- **WHEN** `contact_update_account_id(session, service, user)` is called
- **THEN** returns `(account_id, contact_id)` with the resolved Salesforce account ID and the updated or created contact ID

---

### Requirement: Cronitor heartbeat pings
`CronitorClient.Ping()` SHALL send a GET request to the Cronitor monitor URL after successful nightly task runs.

#### Scenario: Heartbeat pinged on task success
- **WHEN** a nightly beat task completes successfully
- **THEN** `CronitorClient.Ping(task_name)` is called and an HTTP GET is sent to the Cronitor monitor URL

#### Scenario: Heartbeat not pinged on task failure
- **WHEN** a nightly beat task returns an error
- **THEN** `CronitorClient.Ping()` is NOT called

#### Scenario: Cronitor disabled — no HTTP call
- **WHEN** `CRONITOR_ENABLED = false`
- **THEN** `Ping()` returns immediately without making any HTTP request

#### Scenario: Cronitor error does not affect calling task
- **WHEN** the Cronitor GET request fails (network error or non-2xx response)
- **THEN** the error is logged and discarded; the calling task's success/failure outcome is unaffected

---

### Requirement: All clients are interface-based for testability
Each external client package SHALL define a Go interface. Production code depends on the interface; tests inject a mock implementation.

#### Scenario: Airtable client injectable via interface
- **WHEN** a newsletter handler is constructed
- **THEN** it accepts an `AirtableClient` interface parameter, not a concrete struct

#### Scenario: Freshdesk client injectable via interface
- **WHEN** the contact form handler is constructed
- **THEN** it accepts a `FreshdeskClient` interface parameter

#### Scenario: Salesforce client injectable via interface
- **WHEN** a service lifecycle handler is constructed
- **THEN** it accepts a `SalesforceClient` interface parameter
