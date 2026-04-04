## 1. Airtable Client

- [ ] 1.1 Define `AirtableClient` interface (`GetSubscriber`, `CreateUnconfirmedSubscriber`, `ConfirmSubscription`, `UnsubscribeUser`, `UpdateSubscriberLanguage`, `GetLatestNewsletterTemplates`); implement `AirtableTableMixin` struct with `table_exists()`, `create_table()`, and create-if-absent helper
- [ ] 1.2 Implement `NewsletterSubscriber`: `from_email()` formula-based lookup; `save_unconfirmed_subscriber()`; `confirm_subscription(has_resubscribed)` with enum-controlled status; `unsubscribe_user()` clearing `confirmed_at` to nil; `update_language()` with enum validation; write unit tests for all state transitions and error cases
- [ ] 1.3 Implement `LatestNewsletterTemplate.get_latest_newsletter_templates()` (sort `-Created at`, max_records=1, table auto-create); write unit tests for 404 and table-absent cases

## 2. Freshdesk Client

- [ ] 2.1 Define `FreshdeskClient` interface; implement HTTP POST to `/api/v2/tickets` with Basic auth (`base64(api_key:x)`); build ticket payloads for all 4 `support_type` values (`demo`, `go_live_request`, `branding_request`, `new_template_category_request`) with correct subject and bilingual description formatting
- [ ] 2.2 Implement `FRESH_DESK_ENABLED=false` short-circuit (return 201 immediately); implement `RequestException` fallback (`persist_notification` + `send_notification_to_queue`); write unit tests for disabled flag and fallback path

## 3. Salesforce Client

- [ ] 3.1 Implement `salesforce_auth`: `get_session()` with `TimeoutAdapter` mounted on `http://` and `https://`, return `nil` on `SalesforceAuthenticationFailed`; `end_session()` with nil-check token revocation; write unit tests
- [ ] 3.2 Implement `salesforce_account`: `get_org_name_from_notes()` split-on-`>` logic; `get_account_id_from_name()` with SOQL query, single-quote escaping, and fallback to generic account ID; write unit tests including nil/empty/whitespace cases
- [ ] 3.3 Implement `salesforce_contact`: `create()` with base payload and `Sforce-Duplicate-Rule-Header: allowSave=true`; `update()` lookup-then-upsert via `CDS_Contact_ID__c`; `get_contact_by_user_id()` SOQL lookup; `get_name_parts()` first-word/remainder split; write unit tests
- [ ] 3.4 Implement `salesforce_engagement`: `create()` Opportunity + OpportunityLineItem; `update()` lookup-then-upsert; `engagement_close()` with Close Reason; `contact_role_add()` and `contact_role_delete()`; `engagement_maxlengths()` 120-char Name truncation; `get_engagement_by_service_id()` and `get_engagement_contact_role()` SOQL queries; write unit tests
- [ ] 3.5 Implement `SalesforceClient` facade with `defer end_session` pattern for all 8 public methods (`contact_create`, `contact_update`, `contact_update_account_id`, `engagement_create`, `engagement_update`, `engagement_close`, `engagement_add_contact_role`, `engagement_delete_contact_role`); write unit tests verifying session is always closed including on error

## 4. Cronitor Client

- [ ] 4.1 Define `CronitorClient` interface with `Ping(taskName string)`; implement HTTP GET to Cronitor URL with `CRONITOR_ENABLED=false` no-op; swallow and log errors (never propagate); wire to nightly beat task success path; write unit test confirming ping on success and silence on failure

## 5. Newsletter Handler Wiring

- [ ] 5.1 Implement `POST /newsletter/unconfirmed-subscriber`, `GET /newsletter/confirm/{subscriber_id}`, `GET /newsletter/unsubscribe/{subscriber_id}` handlers using the `AirtableClient` interface; register all three under the admin JWT middleware
- [ ] 5.2 Write integration tests for the 3 newsletter handlers with a mock `AirtableClient`; assert 201/200/404 responses and that a missing admin JWT returns 401 for all three routes
