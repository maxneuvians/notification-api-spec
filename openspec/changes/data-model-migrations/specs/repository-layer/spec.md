# Capability: repository-layer

The sqlc-generated repository function catalogue for all 14 domain modules, plus the history-write, encrypted-column, and soft-delete patterns that all domain service layers must follow.

---

## Requirement: Query files and generated packages for all 14 domain modules

**R1** — 14 SQL query files SHALL exist under `db/queries/`: `notifications.sql`, `services.sql`, `api_keys.sql`, `templates.sql`, `jobs.sql`, `billing.sql`, `users.sql`, `organisations.sql`, `inbound_sms.sql`, `providers.sql`, `complaints.sql`, `reports.sql`, `annual_limits.sql`, `template_categories.sql`.

**R2** — Running `sqlc generate` SHALL produce a valid, compilable Go package at `internal/repository/<domain>/` for each of the 14 domains.

#### Scenario: sqlc generate succeeds for all 14 domains
- **WHEN** `sqlc generate` is run with all 14 query files present and the seed migration schema in `db/migrations/`
- **THEN** all 14 `internal/repository/<domain>/` packages compile without errors under `go build ./...`

#### Scenario: UUID columns use google/uuid
- **WHEN** a generated repository function has a non-nullable UUID parameter or return value
- **THEN** the Go type is `github.com/google/uuid.UUID`

#### Scenario: Nullable UUID columns use NullUUID
- **WHEN** a generated repository function has a nullable UUID column (e.g. a nullable FK)
- **THEN** the Go type is `github.com/google/uuid.NullUUID`

#### Scenario: JSONB columns use json.RawMessage
- **WHEN** a generated function has a JSONB column (e.g. `compromised_key_info`, `folder_permissions`, `data`)
- **THEN** the Go type is `encoding/json.RawMessage`

#### Scenario: Timestamptz columns use time.Time
- **WHEN** a generated function has a non-nullable `timestamptz` column
- **THEN** the Go type is `time.Time`

#### Scenario: Nullable timestamptz columns use *time.Time
- **WHEN** a generated function has a nullable `timestamptz` column
- **THEN** the Go type is `*time.Time`

---

## Requirement: Repository function signatures — notifications domain

**R3** — `internal/repository/notifications` SHALL expose the following functions (all return `(T, error)` or `error`):

`GetLastTemplateUsage`, `CreateNotification`, `BulkInsertNotifications`, `UpdateNotificationStatusByID`, `UpdateNotificationStatusByReference`, `BulkUpdateNotificationStatuses`, `GetNotificationByID`, `GetNotificationsByServiceID`, `GetNotificationsForJob`, `GetNotificationsCreatedSince`, `TimeoutSendingNotifications`, `DeleteNotificationsOlderThanRetention`, `GetLastNotificationAddedForJobID`, `GetNotificationsByReference`, `GetHardBouncesForService`, `GetMonthlyNotificationStats`, `GetTemplateUsageMonthly`, `InsertNotificationHistory`, `GetNotificationFromHistory`, `GetBounceRateTimeSeries`.

#### Scenario: Create and retrieve notification round-trip
- **WHEN** `CreateNotification` is called with a valid `*Notification`
- **THEN** `GetNotificationByID` with the same ID returns the row without error

#### Scenario: Bulk insert multiple notifications
- **WHEN** `BulkInsertNotifications` is called with a slice of N notifications
- **THEN** all N rows are inserted in a single query; the function returns without error

#### Scenario: Status update by reference
- **WHEN** `UpdateNotificationStatusByReference` is called with a provider reference string and new status
- **THEN** all notifications matching that reference have their `notification_status` updated

#### Scenario: Timeout sending notifications
- **WHEN** `TimeoutSendingNotifications` is called with a timeout duration
- **THEN** all notifications in `sending` status older than the timeout are updated to `temporary-failure` and their IDs are returned

#### Scenario: Notification history archived
- **WHEN** `InsertNotificationHistory` is called with a `*NotificationHistory`
- **THEN** the row is inserted into `notification_history`; `GetNotificationFromHistory` returns it

---

## Requirement: Repository function signatures — services domain

**R4** — `internal/repository/services` SHALL expose (at minimum): `GetAllServices`, `GetServicesByPartialName`, `CountLiveServices`, `GetLiveServicesData`, `GetServiceByID`, `GetServiceByInboundNumber`, `GetServiceByIDWithAPIKeys`, `GetServicesByUserID`, `CreateService`, `UpdateService`, `ArchiveService`, `SuspendService`, `ResumeService`, `GetServiceByIDAndUser`, `AddUserToService`, `RemoveUserFromService`, `GetServicePermissions`, `SetServicePermissions`, `GetSafelist`, `UpdateSafelist`, `GetDataRetention`, `UpsertDataRetention`, `GetSMSSenders`, `CreateSMSSender`, `UpdateSMSSender`, `GetEmailReplyTo`, `CreateEmailReplyTo`, `UpdateEmailReplyTo`, `GetCallbackAPIs`, `UpsertCallbackAPI`, `DeleteCallbackAPI`, `GetInboundAPI`, `UpsertInboundAPI`, `DeleteInboundAPI`, `InsertServicesHistory`, `GetSensitiveServiceIDs`, `GetMonthlyDataByService`.

#### Scenario: GetServiceByIDWithAPIKeys uses read replica
- **WHEN** `GetServiceByIDWithAPIKeys` is called during request authentication
- **THEN** the query is executed against the reader `*sql.DB`

#### Scenario: ArchiveService marks service inactive
- **WHEN** `ArchiveService` is called with a service ID
- **THEN** the service's name is suffixed with `_archived_<timestamp>` and its active flag is false

#### Scenario: UpsertCallbackAPI creates or replaces callback config
- **WHEN** `UpsertCallbackAPI` is called with a callback type that already has a row
- **THEN** the existing row is updated (not a duplicate key error)

---

## Requirement: Repository function signatures — api_keys domain

**R5** — `internal/repository/api_keys` SHALL expose: `CreateAPIKey`, `GetAPIKeysByServiceID`, `GetAPIKeyByID`, `RevokeAPIKey`, `GetAPIKeyBySecret`, `UpdateAPIKeyLastUsed`, `RecordAPIKeyCompromise`, `GetAPIKeySummaryStats`, `GetAPIKeysRankedByNotifications`, `InsertAPIKeyHistory`.

#### Scenario: GetAPIKeyBySecret uses read replica
- **WHEN** `GetAPIKeyBySecret` is called during service auth
- **THEN** the query is executed against the reader `*sql.DB`

#### Scenario: RevokeAPIKey sets expiry_date
- **WHEN** `RevokeAPIKey` is called with a key ID
- **THEN** `expiry_date` is set to the current timestamp (not null deletion)

#### Scenario: RecordAPIKeyCompromise stores JSONB
- **WHEN** `RecordAPIKeyCompromise` is called with a `json.RawMessage` payload
- **THEN** `compromised_key_info` in the `api_keys` row contains that payload

---

## Requirement: Repository function signatures — templates domain

**R6** — `internal/repository/templates` SHALL expose: `CreateTemplate`, `GetTemplateByID`, `GetTemplateByIDAndVersion`, `GetTemplatesByServiceID`, `UpdateTemplate`, `ArchiveTemplate`, `GetTemplateVersions`, `GetPrecompiledLetterTemplate`, `GetTemplateFolders`, `CreateTemplateFolder`, `UpdateTemplateFolder`, `DeleteTemplateFolder`, `MoveTemplateContents`, `GetTemplateCategories`, `GetTemplateCategoryByID`, `CreateTemplateCategory`, `UpdateTemplateCategory`, `DeleteTemplateCategory`, `InsertTemplateHistory`.

#### Scenario: GetTemplateByIDAndVersion returns historical version
- **WHEN** `GetTemplateByIDAndVersion` is called with a template ID and an older version number
- **THEN** the function returns the `*TemplateHistory` row for that version, not the current template row

#### Scenario: MoveTemplateContents reassigns folder membership
- **WHEN** `MoveTemplateContents` is called with a target folder ID and a list of template IDs
- **THEN** all specified templates have their folder association updated to the target folder

---

## Requirement: Repository function signatures — jobs domain

**R7** — `internal/repository/jobs` SHALL expose: `CreateJob`, `GetJobByID`, `GetJobsByServiceID`, `UpdateJob`, `SetScheduledJobsToPending`, `GetInProgressJobs`, `GetStalledJobs`, `ArchiveOldJobs`, `HasJobs`.

#### Scenario: SetScheduledJobsToPending promotes due jobs
- **WHEN** `SetScheduledJobsToPending` is called
- **THEN** all jobs with `job_status = 'scheduled'` and `scheduled_for <= now()` are updated to `pending` and returned

#### Scenario: GetStalledJobs finds stuck in-progress jobs
- **WHEN** `GetStalledJobs` is called with minAge and maxAge
- **THEN** all jobs with `job_status = 'in progress'` and `processing_started` between now-maxAge and now-minAge are returned

---

## Requirement: Repository function signatures — billing domain

**R8** — `internal/repository/billing` SHALL expose: `GetMonthlyBillingUsage`, `GetYearlyBillingUsage`, `GetFreeSMSFragmentLimit`, `UpsertFreeSMSFragmentLimit`, `UpsertFactBillingForDay`, `GetAnnualLimitsData`, `InsertQuarterData`, `GetPlatformStatsByDateRange`, `GetDeliveredNotificationsByMonth`, `GetUsageForTrialServices`, `GetUsageForAllServices`, `GetFactNotificationStatusForDay`, `UpsertFactNotificationStatus`, `UpsertMonthlyNotificationStatsSummary`.

---

## Requirement: Repository function signatures — users domain

**R9** — `internal/repository/users` SHALL expose: `CreateUser`, `GetUserByID`, `GetUserByEmail`, `FindUsersByEmail`, `GetAllUsers`, `UpdateUser`, `ArchiveUser`, `DeactivateUser`, `ActivateUser`, `GetUsersByServiceID`, `SetUserPermissions`, `GetUserPermissions`, `SetFolderPermissions`, `CreateVerifyCode`, `GetVerifyCode`, `MarkVerifyCodeUsed`, `DeleteExpiredVerifyCodes`, `CreateLoginEvent`, `GetLoginEventsByUserID`, `GetFido2KeysByUserID`, `CreateFido2Key`, `DeleteFido2Key`, `CreateFido2Session`, `GetFido2Session`.

---

## Requirement: Repository function signatures — organisations domain

**R10** — `internal/repository/organisations` SHALL expose: `GetAllOrganisations`, `GetOrganisationByID`, `GetOrganisationByDomain`, `CreateOrganisation`, `UpdateOrganisation`, `LinkServiceToOrganisation`, `GetServicesByOrganisationID`, `AddUserToOrganisation`, `GetUsersByOrganisationID`, `IsOrganisationNameUnique`, `GetInvitedOrgUsers`, `CreateInvitedOrgUser`, `UpdateInvitedOrgUser`.

---

## Requirement: Repository function signatures — inbound SMS domain

**R11** — `internal/repository/inbound` SHALL expose: `GetInboundNumbers`, `GetAvailableInboundNumbers`, `GetInboundNumberByServiceID`, `AddInboundNumber`, `DisableInboundNumberForService`, `CreateInboundSMS`, `GetInboundSMSForService`, `GetMostRecentInboundSMS`, `GetInboundSMSSummary`, `GetInboundSMSByID`, `DeleteInboundSMSOlderThan`.

---

## Requirement: Repository function signatures — providers domain

**R12** — `internal/repository/providers` SHALL expose: `GetAllProviders`, `GetProviderByID`, `GetProviderVersions`, `UpdateProvider`, `ToggleSMSProvider`, `InsertProviderHistory`.

---

## Requirement: Repository function signatures — complaints, reports domains

**R13** — `internal/repository/complaints` SHALL expose: `CreateOrUpdateComplaint`, `GetComplaintsPage`, `CountComplaintsByDateRange`.

**R14** — `internal/repository/reports` SHALL expose: `CreateReport`, `GetReportByID`, `GetReportsByServiceID`, `UpdateReport`.

---

## Requirement: History-write pattern for all 6 versioned entities

**R15** — For each of the 6 versioned entities, the repository package SHALL expose an `InsertXxxHistory(ctx, h *XxxHistory) error` function. Every service-layer mutating call on a versioned entity SHALL invoke the corresponding history function within the same `*sql.Tx`, immediately after the parent row mutation.

The 6 pairs are:
- `services` → `InsertServicesHistory` (`services_history`)
- `api_keys` → `InsertAPIKeyHistory` (`api_keys_history`)
- `templates` → `InsertTemplateHistory` (`templates_history`)
- `provider_details` → `InsertProviderHistory` (`provider_details_history`)
- `service_callback_api` → `InsertServiceCallbackAPIHistory` (`service_callback_api_history`)
- `service_inbound_api` → `InsertServiceInboundAPIHistory` (`service_inbound_api_history`)

#### Scenario: Update and history insert are atomic
- **WHEN** a service layer function updates a `services` row and calls `InsertServicesHistory`
- **THEN** both writes execute within the same `*sql.Tx`; if either fails the entire transaction is rolled back

#### Scenario: History insert failure rolls back parent update
- **WHEN** `InsertServicesHistory` returns an error (e.g. DB error)
- **THEN** the parent `services` row is NOT modified (full rollback)

#### Scenario: History row has no FK constraints
- **WHEN** a history row is inserted referencing a service_id that has been archived
- **THEN** the insert succeeds (no FK constraint enforcement on history tables)

#### Scenario: History table composite PK prevents duplicate
- **WHEN** `InsertServicesHistory` is called with an (id, version) pair that already exists
- **THEN** the insert returns a unique-constraint violation error

---

## Requirement: Encrypted column read/write protocol

**R16** — The 8 encrypted columns SHALL be stored as ciphertext in PostgreSQL. Repository functions SHALL accept and return raw encrypted bytes/strings without any decryption. Service-layer code on read SHALL call `pkg/crypto.Decrypt(ciphertext, config.SecretKeys)`; on write SHALL call `pkg/crypto.Encrypt(plaintext, config.SecretKey[0])`.

The 8 encrypted columns are:
1. `notifications._personalisation` (physical: `_personalisation`)
2. `notifications.to` (physical: `to`) — SensitiveString pattern
3. `notifications.normalised_to` (physical: `normalised_to`) — SensitiveString pattern
4. `inbound_sms._content` (physical: `content`) — **C1 fix: physical column is `content`, not `_content`**
5. `service_callback_api.bearer_token` (physical: `bearer_token`)
6. `service_inbound_api.bearer_token` (physical: `bearer_token`)
7. `verify_codes._code` (physical: `_code`)
8. `users._password` (physical: `_password`)

#### Scenario: Plaintext never stored directly
- **WHEN** service code writes a notification with personalisation data
- **THEN** the value passed to the repository `CreateNotification` call is the output of `pkg/crypto.Encrypt(plaintext, secret)`, not the raw plaintext string

#### Scenario: Decryption happens in service layer only
- **WHEN** `GetNotificationByID` returns a `*Notification` struct
- **THEN** the `_personalisation` field contains encrypted bytes; the service layer must call `pkg/crypto.Decrypt` before reading the plaintext

#### Scenario: inbound_sms content is stored under physical column name `content`
- **WHEN** the `inbound_sms` query file references the message body column
- **THEN** the SQL uses column name `content` (not `_content`); the generated Go field is `Content`

#### Scenario: Key rotation decrypts with any active key
- **WHEN** `pkg/crypto.Decrypt` is called with ciphertext encrypted by an older key
- **THEN** it tries each key in `config.SecretKeys` in order and returns the plaintext on first match

#### Scenario: Encryption always uses the first key
- **WHEN** any repository write for an encrypted column is performed
- **THEN** the service layer uses `config.SecretKey[0]` (the current key) for encryption

---

## Requirement: Soft-delete conventions

**R17** — Six entity types use soft-delete rather than physical DELETE. Default list/fetch queries SHALL exclude soft-deleted records. An explicit flag (e.g. `IncludeArchived bool` or `IncludeExpired bool`) may be added to override.

| Table | Mechanism | Active filter |
|---|---|---|
| `api_keys` | `expiry_date IS NULL` | `WHERE expiry_date IS NULL` |
| `jobs` | `archived = false` | `WHERE archived = false` |
| `service_email_reply_to` | `archived = false` | `WHERE archived = false` |
| `service_letter_contacts` | `archived = false` | `WHERE archived = false` |
| `service_sms_senders` | `archived = false` | `WHERE archived = false` |
| `templates` | `archived = false` | `WHERE archived = false` |

#### Scenario: Archived template excluded from default list query
- **WHEN** `GetTemplatesByServiceID` is called without an explicit `IncludeArchived` option
- **THEN** templates with `archived = true` are not included in the result

#### Scenario: Expired API key excluded from default active-key query
- **WHEN** `GetAPIKeysByServiceID` is called without `IncludeExpired`
- **THEN** keys with a non-NULL `expiry_date` in the past are not in the result

#### Scenario: Active-only SMS senders returned by default
- **WHEN** `GetSMSSenders` is called for a service
- **THEN** only senders with `archived = false` are returned

#### Scenario: api_keys name uniqueness only among active keys
- **WHEN** a new API key is created with a name matching a revoked (expired) key for the same service
- **THEN** the unique constraint `uix_service_to_key_name` does not block the insert (partial index WHERE expiry_date IS NULL)

---

## Requirement: Enum lookup table values for status fields

**R18** — All job statuses, notification statuses, key types, and auth types SHALL use string values from the lookup tables (`job_status`, `notification_status_types`, `key_types`, `auth_type`), NOT the native PostgreSQL ENUM types. The `notify_status_type` native ENUM SHALL NOT be referenced in any query or generated type used by production code.

#### Scenario: Notification status stored as lookup table string
- **WHEN** a notification is created with status `created`
- **THEN** the value stored in `notifications.notification_status` is the string `"created"`, not the value from the `notify_status_type` enum

#### Scenario: notify_status_type generated type is unused in production code
- **WHEN** all production Go code is reviewed
- **THEN** no file outside `internal/repository/` imports or uses the `NotifyStatusType` generated Go type

#### Scenario: job_status uses 9-value lookup table, not 4-value PG enum
- **WHEN** a job is created with status `scheduled`
- **THEN** the value `"scheduled"` is accepted by the DB (it is in the `job_status` lookup table but NOT in the `job_status_types` PG enum)

---

## Requirement: Denormalised fact table write functions are idempotent

**R19** — The `ft_billing`, `ft_notification_status`, `annual_limits_data`, and `monthly_notification_stats_summary` tables SHALL have upsert repository functions using `INSERT ... ON CONFLICT DO UPDATE` semantics.

#### Scenario: UpsertFactBillingForDay is idempotent
- **WHEN** `UpsertFactBillingForDay` is called twice with the same `bst_date`, `service_id`, and all PK-constituent columns
- **THEN** the second call updates `billable_units`, `notifications_sent`, `billing_total` and `updated_at` without raising a duplicate-key error

#### Scenario: UpsertFactNotificationStatus is idempotent
- **WHEN** `UpsertFactNotificationStatus` is called twice for the same day and service
- **THEN** the second call updates `notification_count` in place

#### Scenario: UpsertMonthlyNotificationStatsSummary is idempotent
- **WHEN** `UpsertMonthlyNotificationStatsSummary` is called for the same month and service
- **THEN** no duplicate row is created

---

## Requirement: notifications table postage constraint

**R20** — For `notifications` and `notification_history`, postage MUST be `first` or `second` when `notification_type = 'letter'`; postage MUST be NULL for all other notification types.

#### Scenario: Letter notification must have postage
- **WHEN** a notification with `notification_type = 'letter'` is inserted without a postage value
- **THEN** the DB raises the `chk_notifications_postage_null` constraint violation

#### Scenario: Non-letter notification must have NULL postage
- **WHEN** a notification with `notification_type = 'email'` is inserted with `postage = 'first'`
- **THEN** the DB raises the `chk_notifications_postage_null` constraint violation

---

## Requirement: fido2_sessions one-session-per-user constraint

**R21** — The `fido2_sessions` table has PK on `user_id`. At most one active FIDO2 session may exist per user at a time.

#### Scenario: Duplicate session for same user
- **WHEN** `CreateFido2Session` is called for a user who already has an active session
- **THEN** the function either replaces the existing session (upsert) or returns an error — the old session is not left orphaned

---

## Requirement: inbound_numbers one-number-per-service constraint

**R22** — `inbound_numbers.service_id` has a UNIQUE index. At most one inbound number may be assigned to a service.

#### Scenario: Assigning a second number to the same service fails
- **WHEN** a second `inbound_numbers` row is inserted with the same `service_id`
- **THEN** a unique-constraint violation is returned

---

## Requirement: service_callback_api one-callback-per-type constraint

**R23** — `service_callback_api` has UNIQUE(`service_id`, `callback_type`).

#### Scenario: Second callback of same type for same service
- **WHEN** `UpsertCallbackAPI` is called with a `(service_id, callback_type)` pair that already exists
- **THEN** the existing row is updated; no duplicate is created

---

## Requirement: notifications removed to notification_history after retention period

**R24** — `DeleteNotificationsOlderThanRetention` SHALL physically delete rows from `notifications` that are older than the service's configured retention days. The `InsertNotificationHistory` function is called by the worker before deletion to archive the row.

#### Scenario: Notification moved to history before deletion
- **WHEN** the nightly retention worker runs
- **THEN** each notification older than retention is first inserted via `InsertNotificationHistory` and then deleted from `notifications`

#### Scenario: Deletion count returned
- **WHEN** `DeleteNotificationsOlderThanRetention` completes
- **THEN** it returns the count of deleted rows as `int64`
