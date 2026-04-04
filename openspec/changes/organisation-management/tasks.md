## 1. Repository Layer

- [ ] 1.1 Implement `internal/repository/organisations/` — `CreateOrganisation`, `GetOrganisationByID`, `GetOrganisations` (active-first, alpha), `UpdateOrganisation` (partial; atomic domain replacement: delete+bulk-insert; cascade org_type to services with history rows), `GetOrganisationByEmailAddress` (gsi normalisation + longest-match), `GetOrganisationByServiceID`, `GetOrganisationByDomain`; write tests for each
- [ ] 1.2 Implement `CountOrganisationsWithLiveServices` — joins to services with active+!restricted+count_as_live filters; write test
- [ ] 1.3 Implement `AddServiceToOrganisation` — copy org_type + crown to service, record service history row; write tests for field inheritance and re-linking
- [ ] 1.4 Implement `GetOrganisationServices` (returns all, caller sorts), `GetOrganisationByDomain` with domain param validation (reject `@`, missing)
- [ ] 1.5 Implement invited org user repository — `SaveInvitedOrgUser`, `GetInvitedOrgUser(org_id, invite_id)`, `GetInvitedOrgUserByID`, `GetInvitedOrgUsersForOrganisation`, `DeleteOrgInvitationsOlderThan(2 days)`; write tests

## 2. Organisation CRUD Handlers

- [ ] 2.1 Implement `GET /organisations` — sort active-first then alphabetical, fields include `domains` (always present); write tests including empty domains
- [ ] 2.2 Implement `GET /organisations/{id}` — full detail including all nullable MOU fields
- [ ] 2.3 Implement `POST /organisations` — validate `name`, `crown` (reject null with "crown None is not of type boolean"), `organisation_type` (enum); 201; duplicate name → 400; per-field validation errors; write tests for each validation case
- [ ] 2.4 Implement `POST /organisations/{id}` (update) — partial update; domain atomic replacement (`null` no-op, `[]` clears, list replaces lowercased); `email_branding_id` also refreshes ORM relationship; org not found → 404; duplicate name → 400; domain conflict → 400; write tests for domain scenarios

## 3. Organisation Type Cascade and Service Linking

- [ ] 3.1 Implement `_updateOrgTypeForServices` helper — iterates linked services, sets organisation_type, writes history row per service; write tests including cascade with 3+ services
- [ ] 3.2 Implement `POST /organisations/{id}/services` — link service, inherit type+crown, remove from previous org, record history; province_or_territory trigger: set 3-day retention for email+sms if absent; missing service_id → 400; 404 for unknown ids; write tests for each case
- [ ] 3.3 Implement `GET /organisations/{id}/services` — sorted active-first then alpha; write tests for ordering
- [ ] 3.4 Write integration test: org_type cascade when organisation is updated

## 4. MOU Agreement Emails

- [ ] 4.1 Implement `sendNotificationsOnMOUSigned` — dispatch support-team alert + signer receipt (template selects on `agreement_signed_on_behalf_of_name` presence) + on-behalf-of email if address set; write tests for both template branches and on-behalf-of path
- [ ] 4.2 Write test: updating agreement_signed without agreement_signed_by_id does NOT send emails

## 5. by-domain and Uniqueness Endpoints

- [ ] 5.1 Implement `GET /organisations/by-domain?domain=` — reject `@` in param, reject missing, normalise gsi, longest-match; 404 if not found; write tests including gsi normalisation
- [ ] 5.2 Implement `GET /organisations/unique?org_id=&name=` — case-insensitive ilike pre-check; own org name returns true; missing params → 400; write tests for case-insensitive collision and self-check

## 6. Organisation User Membership

- [ ] 6.1 Implement `POST /organisations/{id}/users/{user_id}` — add user to org, 200 with user data including organisations; 404 if user not found; write tests
- [ ] 6.2 Implement `GET /organisations/{id}/users` — active users only (state=active filter); empty list when none; write tests including inactive-exclusion

## 7. Invite Flow

- [ ] 7.1 Implement `POST /organisations/{id}/invite` — create InvitedOrgUser status=pending, send invitation email via internal-tasks queue (reply_to=inviter email, personalisation: org_name/user_name/url, token=signed invite id, invite_link_host override); invalid email → 400; 201; write tests
- [ ] 7.2 Implement `GET /organisations/{id}/invite` — return all invitations; empty list
- [ ] 7.3 Implement `POST /organisations/{id}/invite/{invite_id}` — update status (pending/accepted/cancelled); invalid status → 400; wrong org → 404; write tests for each error case
- [ ] 7.4 Implement beat task `delete_org_invitations_created_more_than_two_days_ago` — bulk DELETE where created_at <= now-2days; write test
