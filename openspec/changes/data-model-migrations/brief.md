# Brief: data-model-migrations

## Source Files
- `spec/data-model.md`
- `spec/out.sql`
- `spec/go-architecture.md` (§Data Layer, §Repository Package Structure, §Key Divergences)

---

## Total Table Count
**68 tables** in PostgreSQL (see `spec/out.sql`).

---

## Domain Groups and Tables

### Notifications Domain
| Table | Notes |
|---|---|
| `notifications` | Live in-flight and recent notifications; primary transactional table |
| `notification_history` | Long-term archive; rows moved here after retention period |
| `scheduled_notifications` | Future-delivery schedule entries |
| `notification_status_types` | Enum lookup: 16 values |
| `ft_notification_status` | Fact table — daily aggregated notification counts by status (composite PK, no FK constraints) |
| `monthly_notification_stats_summary` | Pre-aggregated monthly counts (composite PK, no FK constraints) |

### Services Domain
| Table | Notes |
|---|---|
| `services` | Core service records; versioned |
| `services_history` | Audit history (composite PK: id, version) |
| `service_permissions` | M2M capabilities enabled per service |
| `service_sms_senders` | SMS sender IDs; one default per service; soft-delete via `archived` |
| `service_email_reply_to` | Reply-to addresses; one default; soft-delete via `archived` |
| `service_letter_contacts` | Letter contact blocks; one default; soft-delete via `archived` |
| `service_callback_api` | Delivery/complaint webhook config; versioned; bearer_token encrypted |
| `service_callback_api_history` | Audit history (composite PK: id, version) |
| `service_inbound_api` | Inbound SMS webhook; UNIQUE(service_id); versioned; bearer_token encrypted |
| `service_inbound_api_history` | Audit history (composite PK: id, version) |
| `service_data_retention` | Per-service notification retention overrides |
| `service_safelist` | Trial-mode allowlist (phone/email) |
| `service_email_branding` | Association table: service ↔ email_branding (PK on service_id) |
| `service_letter_branding` | Association table: service ↔ letter_branding (PK on service_id) |
| `service_permission_types` | Enum lookup: 11 capability values |
| `service_callback_type` | Enum lookup: delivery_status, complaint |

### Users Domain
| Table | Notes |
|---|---|
| `users` | Platform users; `_password` encrypted |
| `user_to_service` | M2M join: users ↔ services |
| `user_to_organisation` | M2M join: users ↔ organisations |
| `user_folder_permissions` | Template folder access grants per user/service |
| `verify_codes` | MFA codes; `_code` encrypted |
| `login_events` | Audit log; `data` JSONB |
| `fido2_keys` | Registered WebAuthn security keys per user |
| `fido2_sessions` | Transient FIDO2 challenge sessions (PK: user_id — one active session per user) |
| `permissions` | User-level permissions per service (or platform-wide when service_id NULL) |
| `invited_users` | Pending/accepted service invitations; `folder_permissions` JSONB; `status` is PG enum `invited_users_status_types` |

### Templates Domain
| Table | Notes |
|---|---|
| `templates` | Service templates; `archived` soft-delete |
| `templates_history` | Template versions (composite PK: id, version) |
| `template_folder` | Folder hierarchy for template organisation |
| `template_folder_map` | M2M: templates ↔ folders |
| `template_redacted` | Stores obfuscated template content for PII-safe display |
| `template_categories` | Template categories with urgency/process classification |
| `template_process_type` | Enum lookup: bulk, normal, priority, low, medium, high |

### Billing Domain
| Table | Notes |
|---|---|
| `annual_billing` | Free SMS fragment allowance per service per financial year |
| `annual_limits_data` | Per-service notification count tracking against annual limits |
| `ft_billing` | Fact table — daily billing units (composite PK: 10 columns, no FK constraints) |
| `rates` | Platform-level notification cost rates per channel + SMS vehicle |
| `letter_rates` | Per-sheet letter postage rates (time-ranged) |
| `provider_rates` | Time-series cost rates per provider |
| `daily_sorted_letter` | Daily letter counts split by sorted/unsorted |

### Organisations Domain
| Table | Notes |
|---|---|
| `organisation` | Government organisations; `email_branding_id` has no DB FK constraint |
| `organisation_types` | Enum lookup: 9 values; includes `annual_free_sms_fragment_limit` |
| `domain` | Verified email domains for auto-org-joining |
| `invited_organisation_users` | Pending org invitations |
| `user_to_organisation` | M2M join |

### Providers Domain
| Table | Notes |
|---|---|
| `provider_details` | Registered notification providers (SES, SNS, Pinpoint); versioned |
| `provider_details_history` | Audit history (composite PK: id, version) |
| `provider_rates` | Time-series cost rates per provider |

### Inbound SMS Domain
| Table | Notes |
|---|---|
| `inbound_numbers` | Provisioned inbound SMS phone numbers |
| `inbound_sms` | Received inbound SMS messages; `_content` (physical: `content`) encrypted |
| `service_inbound_api` | (shared with Services domain) |
| `service_inbound_api_history` | (shared with Services domain) |

### Auth/Security Domain
| Table | Notes |
|---|---|
| `auth_type` | Enum lookup: sms_auth, email_auth, security_key_auth |
| `api_keys` | Service API keys; `secret` hashed (UNIQUE); soft-delete via `expiry_date`; versioned; `compromised_key_info` JSONB |
| `api_keys_history` | Audit history (composite PK: id, version) |
| `key_types` | Enum lookup: normal, team, test |
| `fido2_keys` | (shared with Users domain) |
| `fido2_sessions` | (shared with Users domain) |
| `verify_codes` | (shared with Users domain) |
| `login_events` | (shared with Users domain) |
| `invited_users` | (shared with Users domain) |
| `invite_status_type` | Enum lookup: pending, accepted, cancelled |

### Branding Domain
| Table | Notes |
|---|---|
| `email_branding` | Email branding assets (logo, colour, text, alt text FR/EN) |
| `letter_branding` | Letter branding identifiers |
| `branding_type` | Enum lookup: 8 values |

### Analytics/Reporting Domain
| Table | Notes |
|---|---|
| `events` | Generic audit event log; `data` plain `json` (not jsonb) |
| `reports` | Async report generation requests with lifecycle status |
| `dm_datetime` | Date-dimension table (BST/UTC boundaries, fiscal calendar; analytics only) |
| `complaints` | SES complaint feedback against notifications |

---

## Query Files (14 files under `db/queries/`)

| File | Domains / Tables covered |
|---|---|
| `notifications.sql` | notifications, notification_history, scheduled_notifications |
| `services.sql` | services, service_permissions, service_sms_senders, service_email_reply_to, service_letter_contacts, service_callback_api, service_inbound_api, service_data_retention, service_safelist, service_email_branding |
| `api_keys.sql` | api_keys, api_keys_history |
| `templates.sql` | templates, templates_history, template_folder, template_folder_map, template_redacted |
| `jobs.sql` | jobs |
| `billing.sql` | annual_billing, annual_limits_data, ft_billing, ft_notification_status, monthly_notification_stats_summary, rates, letter_rates, daily_sorted_letter |
| `users.sql` | users, verify_codes, login_events, fido2_keys, fido2_sessions, user_to_service, user_folder_permissions, permissions, invited_users |
| `organisations.sql` | organisation, organisation_types, domain, invited_organisation_users, user_to_organisation |
| `inbound_sms.sql` | inbound_numbers, inbound_sms |
| `providers.sql` | provider_details, provider_details_history, provider_rates |
| `complaints.sql` | complaints |
| `reports.sql` | reports |
| `annual_limits.sql` | annual_limits_data (specialised queries: InsertQuarterData, GetAnnualLimitsData) |
| `template_categories.sql` | template_categories, template_process_type |

---

## Repository Package Function Catalogue

### `internal/repository/notifications`
```
GetLastTemplateUsage(ctx, templateID, templateType, serviceID) → *Notification
CreateNotification(ctx, n *Notification) → error
BulkInsertNotifications(ctx, ns []*Notification) → error
UpdateNotificationStatusByID(ctx, id, status, sentBy, feedbackReason) → *Notification
UpdateNotificationStatusByReference(ctx, reference, status) → *Notification
BulkUpdateNotificationStatuses(ctx, updates []StatusUpdate) → error
GetNotificationByID(ctx, id) → *Notification
GetNotificationsByServiceID(ctx, serviceID, filters) → []*Notification, int
GetNotificationsForJob(ctx, jobID, filters) → []*Notification, int
GetNotificationsCreatedSince(ctx, since, status) → []*Notification
TimeoutSendingNotifications(ctx, timeout) → []uuid.UUID
DeleteNotificationsOlderThanRetention(ctx, notificationType) → int64
GetLastNotificationAddedForJobID(ctx, jobID) → *Notification
GetNotificationsByReference(ctx, reference) → []*Notification
GetHardBouncesForService(ctx, serviceID, since) → []BounceRow
GetMonthlyNotificationStats(ctx, serviceID, year) → []MonthlyStats
GetTemplateUsageMonthly(ctx, serviceID, year) → []TemplateUsageRow
InsertNotificationHistory(ctx, n *NotificationHistory) → error
GetNotificationFromHistory(ctx, id) → *NotificationHistory
GetBounceRateTimeSeries(ctx, serviceID, since) → []BounceTimeRow
```

### `internal/repository/services`
```
GetAllServices(ctx, onlyActive) → []*Service
GetServicesByPartialName(ctx, name) → []*Service
CountLiveServices(ctx) → int64
GetLiveServicesData(ctx, filterHeartbeats) → []LiveServiceRow
GetServiceByID(ctx, id, onlyActive) → *Service
GetServiceByInboundNumber(ctx, number) → *Service
GetServiceByIDWithAPIKeys(ctx, id) → *Service
GetServicesByUserID(ctx, userID, onlyActive) → []*Service
CreateService(ctx, s *Service) → error
UpdateService(ctx, id, fields) → error
ArchiveService(ctx, id) → (string, error)
SuspendService(ctx, id) → error
ResumeService(ctx, id) → error
GetServiceByIDAndUser(ctx, serviceID, userID) → *Service
AddUserToService(ctx, serviceID, userID, permissions, folderPermissions) → error
RemoveUserFromService(ctx, serviceID, userID) → error
GetServicePermissions(ctx, serviceID) → []string
SetServicePermissions(ctx, serviceID, permissions) → error
GetSafelist(ctx, serviceID) → *Safelist
UpdateSafelist(ctx, serviceID, emails, phones) → error
GetDataRetention(ctx, serviceID) → []*ServiceDataRetention
UpsertDataRetention(ctx, serviceID, notificationType, days) → error
GetSMSSenders(ctx, serviceID) → []*ServiceSmsSender
CreateSMSSender(ctx, sender) → error
UpdateSMSSender(ctx, senderID, fields) → error
GetEmailReplyTo(ctx, serviceID) → []*ServiceEmailReplyTo
CreateEmailReplyTo(ctx, replyTo) → error
UpdateEmailReplyTo(ctx, id, fields) → error
GetCallbackAPIs(ctx, serviceID, callbackType) → []*ServiceCallbackAPI
UpsertCallbackAPI(ctx, cb *ServiceCallbackAPI) → error
DeleteCallbackAPI(ctx, id) → error
GetInboundAPI(ctx, serviceID) → *ServiceInboundAPI
UpsertInboundAPI(ctx, api *ServiceInboundAPI) → error
DeleteInboundAPI(ctx, id) → error
InsertServicesHistory(ctx, h *ServicesHistory) → error
GetSensitiveServiceIDs(ctx) → []uuid.UUID
GetMonthlyDataByService(ctx, start, end) → []MonthlyServiceRow
```

### `internal/repository/api_keys`
```
CreateAPIKey(ctx, key *APIKey) → error
GetAPIKeysByServiceID(ctx, serviceID) → []*APIKey
GetAPIKeyByID(ctx, id) → *APIKey
RevokeAPIKey(ctx, id) → error
GetAPIKeyBySecret(ctx, hashedSecret) → *APIKey
UpdateAPIKeyLastUsed(ctx, id, ts) → error
RecordAPIKeyCompromise(ctx, id, info json.RawMessage) → error
GetAPIKeySummaryStats(ctx, id) → *APIKeySummaryStats
GetAPIKeysRankedByNotifications(ctx, nDaysBack) → []APIKeyRankedRow
InsertAPIKeyHistory(ctx, h *APIKeyHistory) → error
```

### `internal/repository/templates`
```
CreateTemplate(ctx, t *Template) → error
GetTemplateByID(ctx, id, serviceID) → *Template
GetTemplateByIDAndVersion(ctx, id, version) → *TemplateHistory
GetTemplatesByServiceID(ctx, serviceID) → []*Template
UpdateTemplate(ctx, id, fields) → error
ArchiveTemplate(ctx, id) → error
GetTemplateVersions(ctx, id) → []*TemplateHistory
GetPrecompiledLetterTemplate(ctx, serviceID) → *Template
GetTemplateFolders(ctx, serviceID) → []*TemplateFolder
CreateTemplateFolder(ctx, f *TemplateFolder) → error
UpdateTemplateFolder(ctx, id, fields) → error
DeleteTemplateFolder(ctx, id) → error
MoveTemplateContents(ctx, targetFolderID, folderIDs, templateIDs) → error
GetTemplateCategories(ctx, templateType, hidden) → []*TemplateCategory
GetTemplateCategoryByID(ctx, id) → *TemplateCategory
CreateTemplateCategory(ctx, c *TemplateCategory) → error
UpdateTemplateCategory(ctx, id, fields) → error
DeleteTemplateCategory(ctx, id, cascade) → error
InsertTemplateHistory(ctx, h *TemplateHistory) → error
```

### `internal/repository/jobs`
```
CreateJob(ctx, j *Job) → error
GetJobByID(ctx, id) → *Job
GetJobsByServiceID(ctx, serviceID, filters) → []*Job, int
UpdateJob(ctx, id, fields) → error
SetScheduledJobsToPending(ctx) → []*Job
GetInProgressJobs(ctx) → []*Job
GetStalledJobs(ctx, minAge, maxAge) → []*Job
ArchiveOldJobs(ctx, olderThan) → int64
HasJobs(ctx, serviceID) → bool
```

### `internal/repository/billing`
```
GetMonthlyBillingUsage(ctx, serviceID, year) → []BillingRow
GetYearlyBillingUsage(ctx, serviceID, year) → []YearlyBillingRow
GetFreeSMSFragmentLimit(ctx, serviceID, financialYear) → *AnnualBilling
UpsertFreeSMSFragmentLimit(ctx, ab *AnnualBilling) → error
UpsertFactBillingForDay(ctx, day, data []FactBillingRow) → error
GetAnnualLimitsData(ctx, serviceID) → []AnnualLimitsDataRow
InsertQuarterData(ctx, rows []AnnualLimitsDataRow) → error
GetPlatformStatsByDateRange(ctx, start, end) → []PlatformStatsRow
GetDeliveredNotificationsByMonth(ctx, filterHeartbeats) → []MonthlyDeliveryRow
GetUsageForTrialServices(ctx) → []TrialServiceUsageRow
GetUsageForAllServices(ctx, start, end) → []AllServiceUsageRow
GetFactNotificationStatusForDay(ctx, day, serviceIDs) → []FactNotifStatusRow
UpsertFactNotificationStatus(ctx, day, rows) → error
UpsertMonthlyNotificationStatsSummary(ctx, month) → error
```

### `internal/repository/users`
```
CreateUser(ctx, u *User) → error
GetUserByID(ctx, id) → *User
GetUserByEmail(ctx, email) → *User
FindUsersByEmail(ctx, partialEmail) → []*User
GetAllUsers(ctx) → []*User
UpdateUser(ctx, id, fields) → error
ArchiveUser(ctx, id) → error
DeactivateUser(ctx, id) → error
ActivateUser(ctx, id) → error
GetUsersByServiceID(ctx, serviceID) → []*User
SetUserPermissions(ctx, userID, serviceID, permissions) → error
GetUserPermissions(ctx, userID, serviceID) → []string
SetFolderPermissions(ctx, userID, serviceID, folderIDs) → error
CreateVerifyCode(ctx, code *VerifyCode) → error
GetVerifyCode(ctx, userID, codeType) → *VerifyCode
MarkVerifyCodeUsed(ctx, id) → error
DeleteExpiredVerifyCodes(ctx) → int64
CreateLoginEvent(ctx, e *LoginEvent) → error
GetLoginEventsByUserID(ctx, userID) → []*LoginEvent
GetFido2KeysByUserID(ctx, userID) → []*Fido2Key
CreateFido2Key(ctx, key *Fido2Key) → error
DeleteFido2Key(ctx, id) → error
CreateFido2Session(ctx, s *Fido2Session) → error
GetFido2Session(ctx, sessionID) → *Fido2Session
```

### `internal/repository/organisations`
```
GetAllOrganisations(ctx) → []*Organisation
GetOrganisationByID(ctx, id) → *Organisation
GetOrganisationByDomain(ctx, domain) → *Organisation
CreateOrganisation(ctx, o *Organisation) → error
UpdateOrganisation(ctx, id, fields) → error
LinkServiceToOrganisation(ctx, orgID, serviceID) → error
GetServicesByOrganisationID(ctx, orgID) → []*Service
AddUserToOrganisation(ctx, orgID, userID) → error
GetUsersByOrganisationID(ctx, orgID) → []*User
IsOrganisationNameUnique(ctx, orgID, name) → bool
GetInvitedOrgUsers(ctx, orgID) → []*InvitedOrganisationUser
CreateInvitedOrgUser(ctx, i *InvitedOrganisationUser) → error
UpdateInvitedOrgUser(ctx, id, status) → error
```

### `internal/repository/inbound`
```
GetInboundNumbers(ctx) → []*InboundNumber
GetAvailableInboundNumbers(ctx) → []*InboundNumber
GetInboundNumberByServiceID(ctx, serviceID) → *InboundNumber
AddInboundNumber(ctx, number) → error
DisableInboundNumberForService(ctx, serviceID) → error
CreateInboundSMS(ctx, sms *InboundSMS) → error
GetInboundSMSForService(ctx, serviceID, phoneNumber, limitDays) → []*InboundSMS
GetMostRecentInboundSMS(ctx, serviceID, page) → []*InboundSMS, bool
GetInboundSMSSummary(ctx, serviceID) → *InboundSMSSummary
GetInboundSMSByID(ctx, id) → *InboundSMS
DeleteInboundSMSOlderThan(ctx, olderThan) → int64
```

### `internal/repository/providers`
```
GetAllProviders(ctx) → []*ProviderDetails
GetProviderByID(ctx, id) → *ProviderDetails
GetProviderVersions(ctx, id) → []*ProviderDetailsHistory
UpdateProvider(ctx, id, fields) → error
ToggleSMSProvider(ctx) → error
InsertProviderHistory(ctx, h *ProviderDetailsHistory) → error
```

### `internal/repository/complaints`
```
CreateOrUpdateComplaint(ctx, c *Complaint) → error
GetComplaintsPage(ctx, page) → []*Complaint, *Pagination
CountComplaintsByDateRange(ctx, start, end) → int64
```

### `internal/repository/reports`
```
CreateReport(ctx, r *Report) → error
GetReportByID(ctx, id) → *Report
GetReportsByServiceID(ctx, serviceID, limitDays) → []*Report
UpdateReport(ctx, id, fields) → error
```

---

## History / Versioned Tables (6 entities)

| Parent table | History table | Composite PK | Trigger condition |
|---|---|---|---|
| `services` | `services_history` | (id, version) | Every UPDATE to services |
| `api_keys` | `api_keys_history` | (id, version) | Every UPDATE to api_keys |
| `templates` | `templates_history` | (id, version) | Every UPDATE or version bump to templates |
| `provider_details` | `provider_details_history` | (id, version) | Every UPDATE to provider_details |
| `service_callback_api` | `service_callback_api_history` | (id, version) | Every UPDATE to service_callback_api |
| `service_inbound_api` | `service_inbound_api_history` | (id, version) | Every UPDATE to service_inbound_api |

**Pattern**: History tables carry NO FK constraints. Service layer MUST call `InsertXxxHistory` within the same `*sql.Tx` as the parent UPDATE. If the history insert fails, the transaction rolls back and the parent row is NOT modified.

---

## Encrypted Columns (8 columns)

| Table | Column (Go name) | Physical DB column | Signer/context |
|---|---|---|---|
| `notifications` | `_personalisation` | `_personalisation` | `signer_personalisation` |
| `notifications` | `to` | `to` | `SensitiveString` (transparent encrypt/decrypt) |
| `notifications` | `normalised_to` | `normalised_to` | `SensitiveString` |
| `inbound_sms` | `_content` | `content` | `signer_inbound_sms` |
| `service_callback_api` | `bearer_token` | `bearer_token` | `signer_bearer_token` |
| `service_inbound_api` | `bearer_token` | `bearer_token` | `signer_bearer_token` |
| `verify_codes` | `_code` | `_code` | encryption |
| `users` | `_password` | `_password` | encryption |

**Go protocol**:
- Repository functions accept/return raw encrypted bytes/strings.
- Service layer calls `pkg/crypto.Encrypt(plaintext, config.SecretKey[0])` before INSERT/UPDATE.
- Service layer calls `pkg/crypto.Decrypt(ciphertext, config.SecretKeys)` after SELECT.
- Key rotation: SecretKey is a list; encrypt uses first key; decrypt tries all keys in order.

---

## Soft-Delete Conventions

| Table | Mechanism | Active record filter |
|---|---|---|
| `api_keys` | `expiry_date IS NULL` = active; set `expiry_date` to soft-delete | `WHERE expiry_date IS NULL` |
| `jobs` | `archived = false` = active; set `archived = true` | `WHERE archived = false` |
| `service_email_reply_to` | `archived = false` | `WHERE archived = false` |
| `service_letter_contacts` | `archived = false` | `WHERE archived = false` |
| `service_sms_senders` | `archived = false` | `WHERE archived = false` |
| `templates` | `archived = false` | `WHERE archived = false` |

Default list queries MUST filter out soft-deleted records. An explicit `IncludeArchived`/`IncludeExpired` option may be added to override.

**Partial unique index**: `api_keys` has `uix_service_to_key_name` (UNIQUE on `(service_id, name) WHERE expiry_date IS NULL`) — a service may reuse a key name after the prior key is expired.

---

## Enum / Lookup Tables

### Lookup tables (string PK, FK-referenced)
| Table | Values |
|---|---|
| `auth_type` | `sms_auth`, `email_auth`, `security_key_auth` |
| `branding_type` | `fip_english`, `org`, `org_banner`, `custom_logo`, `both_english`, `both_french`, `custom_logo_with_background_colour`, `no_branding` |
| `invite_status_type` | `pending`, `accepted`, `cancelled` |
| `job_status` | `pending`, `in progress`, `finished`, `sending limits exceeded`, `scheduled`, `cancelled`, `ready to send`, `sent to dvla`, `error` (9 values) |
| `key_types` | `normal`, `team`, `test` |
| `notification_status_types` | `cancelled`, `created`, `sending`, `sent`, `delivered`, `pending`, `failed`, `technical-failure`, `temporary-failure`, `permanent-failure`, `provider-failure`, `pending-virus-check`, `validation-failed`, `virus-scan-failed`, `returned-letter`, `pii-check-failed` (16 values) |
| `organisation_types` | `central`, `province_or_territory`, `local`, `nhs_central`, `nhs_local`, `nhs_gp`, `emergency_service`, `school_or_college`, `other` |
| `service_callback_type` | `delivery_status`, `complaint` |
| `service_permission_types` | `email`, `sms`, `letter`, `international_sms`, `inbound_sms`, `schedule_notifications`, `email_auth`, `letters_as_pdf`, `upload_document`, `edit_folder_permissions`, `upload_letters` |
| `template_process_type` | `bulk`, `normal`, `priority`, `low`, `medium`, `high` |

### Native PostgreSQL ENUMs (DDL `CREATE TYPE`)
| Type | Values | Go usage |
|---|---|---|
| `notification_type` | `email`, `sms`, `letter` | Use in queries |
| `invited_users_status_types` | `pending`, `accepted`, `cancelled` | Use in queries |
| `permission_types` | `manage_users`, `manage_templates`, `manage_settings`, `send_texts`, `send_emails`, `send_letters`, `manage_api_keys`, `platform_admin`, `view_activity` | Use in queries |
| `notification_feedback_types` | `hard-bounce`, `soft-bounce`, `unknown-bounce` | Use in queries |
| `notification_feedback_subtypes` | 9 values | Use in queries |
| `verify_code_types` | `email`, `sms` | Use in queries |
| `sms_sending_vehicle` | `short_code`, `long_code` | Use in queries |
| `template_type` | `email`, `sms`, `letter` | Use in queries |
| `recipient_type` | `mobile`, `email` | Use in queries |
| `job_status_types` | 4 values only | **DO NOT USE** — use `job_status` lookup table (9 values) |
| `notify_status_type` | mirror of notification_status_types | **DEAD CODE** — never reference in production |

---

## Denormalised Fact Tables

| Table | Composite PK columns | FK constraints | Write pattern |
|---|---|---|---|
| `ft_billing` | 10 columns (bst_date, template_id, service_id, notification_type, provider, rate_multiplier, international, rate, postage, sms_sending_vehicle) | None | `INSERT ... ON CONFLICT DO UPDATE` |
| `ft_notification_status` | 7 columns (bst_date, template_id, service_id, job_id, notification_type, key_type, notification_status) | None | `INSERT ... ON CONFLICT DO UPDATE` |
| `monthly_notification_stats_summary` | 3 columns (month, service_id, notification_type) | None | `INSERT ... ON CONFLICT DO UPDATE` |
| `annual_limits_data` | (service_id, time_period, notification_type) | No FK on service_id | `INSERT` (quarterly batch) |

---

## Key sqlc Configuration Requirements
- Engine: `postgresql`
- Queries directory: `db/queries/`
- Schema directory: `db/migrations/`
- Output: `internal/repository/` (one package per domain sub-directory)
- Type overrides:
  - `uuid` → `github.com/google/uuid.UUID`
  - nullable `uuid` → `github.com/google/uuid.NullUUID`
  - `jsonb` → `encoding/json.RawMessage`
  - `pg_catalog.timestamptz` → `time.Time`
  - nullable `pg_catalog.timestamptz` → `*time.Time`
- Emit flags: `emit_json_tags: true`, `emit_pointers_for_null_types: true`, `emit_enum_valid_method: true`, `emit_all_enum_values: true`

---

## Read Replica Routing

**Use reader (`*sql.DB` reader) for:**
- `GetServiceByIDWithAPIKeys` (request-time auth)
- `GetAPIKeyBySecret` (request-time auth)
- Read-only list/fetch endpoints with acceptable eventual consistency
- `ft_billing` / `ft_notification_status` queries
- `annual_billing`, `annual_limits_data` limit-check reads

**Always use writer for:**
- Any INSERT, UPDATE, DELETE
- Reads immediately after a write within the same request
- Auth-critical reads (permission checks)
- Rate-limit counter DB fallback

---

## Migration Setup

- Migration tool: `golang-migrate`
- Directory: `db/migrations/`
- Seed file: `db/migrations/0001_initial.sql` (derived from `spec/out.sql`)
- Subsequent schema changes: `0002_...sql`, `0003_...sql`, etc.
- CLI: `migrate -path db/migrations -database "$SQLALCHEMY_DATABASE_URI" up`
- In-process: `github.com/golang-migrate/migrate/v4` with postgres driver; called on startup

---

## Validation Gaps from Spec Review

- **C1 (inbound_sms._content)**: Physical column `content` was missing from earlier encrypted-column lists. MUST decrypt on read and encrypt on write via `pkg/crypto`.
- **job_status_types**: DDL enum has 4 values; lookup table has 9. Production code MUST use lookup table string constants, never the `job_status_types` PG enum or generated Go type.
- **notify_status_type**: Dead code. Never reference in production queries or Go code paths.
- **organisation.email_branding_id**: No FK constraint in DB — application must not assume referential integrity enforcement.
