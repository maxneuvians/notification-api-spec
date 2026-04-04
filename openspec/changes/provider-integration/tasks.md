## 1. Provider Repository

- [ ] 1.1 Implement `internal/repository/providers/` — `GetProviderDetailsByID`, `GetProviderDetailsByIdentifier`, `GetProviderDetailsByNotificationType(type, international)`, `GetCurrentProvider(type)`, `GetAllProviders`, `UpdateProviderDetails` (bump version, set updated_at, write ProviderDetailsHistory snapshot), `GetProviderVersions(provider_id)`; write tests including history-row verification after update
- [ ] 1.2 Implement `GetAlternativeSMSProvider(identifier)` — currently returns same provider (no-op); write test documenting intent
- [ ] 1.3 Implement `SwitchSMSProviderToIdentifier(identifier)` — priority-swap logic: swap if B>A, A+=10 if equal; no-op if target is current or inactive; write tests for all three cases
- [ ] 1.4 Implement `ToggleSMSProvider(identifier)` — calls GetAlternativeSMSProvider then SwitchSMSProviderToIdentifier; write test
- [ ] 1.5 Implement `GetDaoProviderStats` — LEFT OUTER JOIN subquery on ft_billing (SMS, current month) computing current_month_billable_sms; ordered by notification_type then priority; write test with billing rows and zero-row case

## 2. Provider Rate Repository

- [ ] 2.1 Implement `internal/repository/providerrates/` — `CreateProviderRates(identifier, validFrom, rate)` INSERT (resolves identifier to provider_id); `GetRateForProvider(providerID, notificationCreatedAt)` most-recent-first lookup; missing rate logs [error-sms-rates] and returns error; write tests for both functions and missing-rate error

## 3. Provider CRUD Handlers

- [ ] 3.1 Implement `GET /provider-details` — sorted by notification_type then priority; include current_month_billable_sms via DAO; write tests for sort order and billing count
- [ ] 3.2 Implement `GET /provider-details/{id}` — single provider in provider_details envelope; write test
- [ ] 3.3 Implement `POST /provider-details/{id}` — accept priority/active/created_by; guard against identifier/version/updated_at writes (400 per-field); call UpdateProviderDetails; return 200 with updated object; write tests for each disallowed field
- [ ] 3.4 Implement `GET /provider-details/{id}/versions` — data array from GetProviderVersions; verify current_month_billable_sms absent; write test

## 4. Provider Selection Algorithm

- [ ] 4.1 Implement `ProviderToUse(notificationType, sender, to, templateID, international bool)` in `internal/service/providers/`:
  - Classify: has_dedicated_number, sending_to_us_number, recipient_outside_canada, cannot_determine_recipient_country, using_sc_pool_template, zone_1_outside_canada
  - do_not_use_pinpoint from 6-condition gate (see brief)
  - Build candidate list; return first active provider's client
  - Empty list → error "No active {type} providers"
  - Write tests covering: US number, Canadian+both pools, dedicated+no-pinpoint flag, SC template without default pool, zone-1 outside Canada, unparseable number, all inactive
- [ ] 4.2 Implement `SmsSendingVehicles` enum (SHORT_CODE, LONG_CODE); resolve template category to sending vehicle; write tests for pool selection

## 5. Provider Failover Wiring

- [ ] 5.1 Wire `ToggleSMSProvider(identifier)` into SMS delivery error handler (called on generic exceptions, NOT PinpointConflictException, NOT PinpointValidationException); write tests confirming toggle is/is not called for each exception type
