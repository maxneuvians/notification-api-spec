# Tasks: data-model-migrations

## 1. notifications query file
- [x] 1.1 Write `db/queries/notifications.sql` with all named queries for `notifications`, `notification_history`, `scheduled_notifications` (CreateNotification, BulkInsertNotifications, UpdateNotificationStatusByID, UpdateNotificationStatusByReference, BulkUpdateNotificationStatuses, GetNotificationByID, GetNotificationsByServiceID, GetNotificationsForJob, GetNotificationsCreatedSince, TimeoutSendingNotifications, DeleteNotificationsOlderThanRetention, GetLastNotificationAddedForJobID, GetNotificationsByReference, GetHardBouncesForService, GetMonthlyNotificationStats, GetTemplateUsageMonthly, InsertNotificationHistory, GetNotificationFromHistory, GetBounceRateTimeSeries, GetLastTemplateUsage)
- [x] 1.2 Run `sqlc generate` for the notifications package; fix any type-mismatch errors until `internal/repository/notifications/` compiles

## 2. services query file
- [x] 2.1 Write `db/queries/services.sql` covering services, services_history, service_permissions, service_sms_senders, service_email_reply_to, service_callback_api, service_callback_api_history, service_inbound_api, service_inbound_api_history, service_data_retention, service_safelist, service_email_branding (GetAllServices, GetServicesByPartialName, CountLiveServices, GetLiveServicesData, GetServiceByID, GetServiceByInboundNumber, GetServiceByIDWithAPIKeys, GetServicesByUserID, CreateService, UpdateService, ArchiveService, SuspendService, ResumeService, GetServiceByIDAndUser, AddUserToService, RemoveUserFromService, GetServicePermissions, SetServicePermissions, GetSafelist, UpdateSafelist, GetDataRetention, UpsertDataRetention, GetSMSSenders, CreateSMSSender, UpdateSMSSender, GetEmailReplyTo, CreateEmailReplyTo, UpdateEmailReplyTo, GetCallbackAPIs, UpsertCallbackAPI, DeleteCallbackAPI, GetInboundAPI, UpsertInboundAPI, DeleteInboundAPI, InsertServicesHistory, GetSensitiveServiceIDs, GetMonthlyDataByService)
- [x] 2.2 Run `sqlc generate`; fix type errors; confirm `InsertServicesHistory` signature accepts `*ServicesHistory`

## 3. api_keys query file
- [x] 3.1 Write `db/queries/api_keys.sql` (CreateAPIKey, GetAPIKeysByServiceID, GetAPIKeyByID, RevokeAPIKey, GetAPIKeyBySecret, UpdateAPIKeyLastUsed, RecordAPIKeyCompromise, GetAPIKeySummaryStats, GetAPIKeysRankedByNotifications, InsertAPIKeyHistory); ensure `compromised_key_info` column maps to `json.RawMessage`; confirm partial unique index `uix_service_to_key_name` is documented in comments
- [x] 3.2 Run `sqlc generate`; fix type errors

## 4. templates query file
- [x] 4.1 Write `db/queries/templates.sql` covering templates, templates_history, template_folder, template_folder_map, template_redacted (CreateTemplate, GetTemplateByID, GetTemplateByIDAndVersion, GetTemplatesByServiceID, UpdateTemplate, ArchiveTemplate, GetTemplateVersions, GetPrecompiledLetterTemplate, GetTemplateFolders, CreateTemplateFolder, UpdateTemplateFolder, DeleteTemplateFolder, MoveTemplateContents, InsertTemplateHistory)
- [x] 4.2 Run `sqlc generate`; confirm `GetTemplatesByServiceID` default query has `WHERE archived = false`

## 5. template_categories query file
- [x] 5.1 Write `db/queries/template_categories.sql` (GetTemplateCategories, GetTemplateCategoryByID, CreateTemplateCategory, UpdateTemplateCategory, DeleteTemplateCategory); confirm `hidden` filter is optional
- [x] 5.2 Run `sqlc generate`; fix type errors

## 6. jobs query file
- [x] 6.1 Write `db/queries/jobs.sql` (CreateJob, GetJobByID, GetJobsByServiceID, UpdateJob, SetScheduledJobsToPending, GetInProgressJobs, GetStalledJobs, ArchiveOldJobs, HasJobs); confirm `WHERE archived = false` on default list queries; `SetScheduledJobsToPending` targets `scheduled_for <= now()`
- [x] 6.2 Run `sqlc generate`; fix type errors

## 7. billing query file
- [x] 7.1 Write `db/queries/billing.sql` covering annual_billing, annual_limits_data, ft_billing (10-column composite PK upsert), ft_notification_status (7-column composite PK upsert), monthly_notification_stats_summary upsert, rates, letter_rates, daily_sorted_letter (GetMonthlyBillingUsage, GetYearlyBillingUsage, GetFreeSMSFragmentLimit, UpsertFreeSMSFragmentLimit, UpsertFactBillingForDay, GetAnnualLimitsData, GetPlatformStatsByDateRange, GetDeliveredNotificationsByMonth, GetUsageForTrialServices, GetUsageForAllServices, GetFactNotificationStatusForDay, UpsertFactNotificationStatus, UpsertMonthlyNotificationStatsSummary)
- [x] 7.2 Write `db/queries/annual_limits.sql` (InsertQuarterData, GetAnnualLimitsData specialty queries)
- [x] 7.3 Run `sqlc generate`; confirm all upserts use `INSERT ... ON CONFLICT DO UPDATE` semantics

## 8. users query file
- [x] 8.1 Write `db/queries/users.sql` covering users, verify_codes, login_events, fido2_keys, fido2_sessions, user_to_service, user_folder_permissions, permissions, invited_users (CreateUser, GetUserByID, GetUserByEmail, FindUsersByEmail, GetAllUsers, UpdateUser, ArchiveUser, DeactivateUser, ActivateUser, GetUsersByServiceID, SetUserPermissions, GetUserPermissions, SetFolderPermissions, CreateVerifyCode, GetVerifyCode, MarkVerifyCodeUsed, DeleteExpiredVerifyCodes, CreateLoginEvent, GetLoginEventsByUserID, GetFido2KeysByUserID, CreateFido2Key, DeleteFido2Key, CreateFido2Session, GetFido2Session)
- [x] 8.2 Run `sqlc generate`; confirm `folder_permissions` (JSONB) maps to `json.RawMessage`; confirm `invited_users.status` uses `invited_users_status_types` PG enum mapping

## 9. organisations query file
- [x] 9.1 Write `db/queries/organisations.sql` covering organisation, organisation_types, domain, invited_organisation_users, user_to_organisation (GetAllOrganisations, GetOrganisationByID, GetOrganisationByDomain, CreateOrganisation, UpdateOrganisation, LinkServiceToOrganisation, GetServicesByOrganisationID, AddUserToOrganisation, GetUsersByOrganisationID, IsOrganisationNameUnique, GetInvitedOrgUsers, CreateInvitedOrgUser, UpdateInvitedOrgUser)
- [x] 9.2 Run `sqlc generate`; fix type errors

## 10. inbound SMS query file
- [x] 10.1 Write `db/queries/inbound_sms.sql` covering inbound_numbers and inbound_sms (GetInboundNumbers, GetAvailableInboundNumbers, GetInboundNumberByServiceID, AddInboundNumber, DisableInboundNumberForService, CreateInboundSMS, GetInboundSMSForService, GetMostRecentInboundSMS, GetInboundSMSSummary, GetInboundSMSByID, DeleteInboundSMSOlderThan); ensure `inbound_sms` queries reference physical column `content` (not `_content`) and include a comment marking it as encrypted
- [x] 10.2 Run `sqlc generate`; confirm generated `InboundSMS` struct has field `Content string` (no underscore prefix)

## 11. providers query file
- [x] 11.1 Write `db/queries/providers.sql` (GetAllProviders, GetProviderByID, GetProviderVersions, UpdateProvider, ToggleSMSProvider, InsertProviderHistory); `ToggleSMSProvider` must flip the active provider for each SMS notification type atomically
- [x] 11.2 Run `sqlc generate`; fix type errors

## 12. complaints and reports query files
- [x] 12.1 Write `db/queries/complaints.sql` (CreateOrUpdateComplaint, GetComplaintsPage, CountComplaintsByDateRange) and `db/queries/reports.sql` (CreateReport, GetReportByID, GetReportsByServiceID, UpdateReport)
- [x] 12.2 Run `sqlc generate`; fix type errors

## 13. Full generate pass and repository smoke tests
- [x] 13.1 Run `make sqlc` (or `sqlc generate`) with all 14 query files in place; resolve any remaining type errors; confirm `go build ./...` passes
- [x] 13.2 Write repository smoke tests for each of the 14 packages: at minimum one `Create` + one `Get` round-trip against a test Postgres instance using `testcontainers-go` or a pre-existing test DSN
- [x] 13.3 Verify no file outside `internal/repository/` references `NotifyStatusType`; verify no job-status field in any query uses the `job_status_types` PG enum

## 14. Pattern enforcement and code review checklist
- [x] 14.1 Add a code review checklist item in CONTRIBUTING.md (or equivalent): "For every service-layer mutating function on a versioned entity (services, api_keys, templates, provider_details, service_callback_api, service_inbound_api), confirm `InsertXxxHistory` is called within the same transaction"
- [x] 14.2 Add a checklist item: "For all 8 encrypted columns, verify the repository call site in the service layer has a `crypto.Encrypt` before write and `crypto.Decrypt` after read"; verify `inbound_sms.content` is in the list
- [x] 14.3 Document reader vs writer `*sql.DB` assignment in `cmd/api/main.go` and `cmd/worker/main.go`; confirm `GetServiceByIDWithAPIKeys` and `GetAPIKeyBySecret` receive the reader instance
