## ADDED Requirements

### Requirement: Organisation list and detail
`GET /organisations` SHALL return all organisations (active and inactive) sorted active-first then alphabetical by name, with fields `name`, `id`, `active`, `count_of_live_services`, `domains`, `organisation_type`. `domains` SHALL always be present (empty list if none). `GET /organisations/{id}` SHALL return full detail including all nullable MOU fields. All organisation endpoints SHALL require admin JWT authentication.

#### Scenario: List returns active orgs before inactive
- **WHEN** GET /organisations is called with a mix of active and inactive orgs
- **THEN** active organisations appear first, then inactive, each group alphabetical by name

#### Scenario: domains always present
- **WHEN** GET /organisations is called on an org with no registered domains
- **THEN** `domains` is `[]`, not absent or null

---

### Requirement: Organisation create with validated fields
`POST /organisations` SHALL require `name` (string), `crown` (boolean — `null` explicitly rejected: "crown None is not of type boolean"), and `organisation_type` (one of: `central`, `province_or_territory`, `local`, `nhs_central`, `nhs_local`, `nhs_gp`, `emergency_service`, `school_or_college`, `other`). Returns 201. Duplicate name → 400 "Organisation name already exists".

#### Scenario: crown null rejected
- **WHEN** POST /organisations is called with `"crown": null`
- **THEN** HTTP 400 with message "crown None is not of type boolean"

#### Scenario: invalid organisation_type rejected
- **WHEN** POST /organisations is called with an organisation_type not in the allowed enum
- **THEN** HTTP 400 ValidationError

#### Scenario: duplicate name rejected
- **WHEN** POST /organisations is called with a name already owned by another organisation
- **THEN** HTTP 400 "Organisation name already exists"

#### Scenario: missing required field produces per-field error
- **WHEN** POST /organisations is called without `name`
- **THEN** HTTP 400 with one ValidationError entry for the `name` field

---

### Requirement: Organisation partial update with atomic domain replacement
`POST /organisations/{id}` SHALL support partial updates. `domains` update SHALL atomically replace all existing domains (lowercased); `domains=null` → no-op; `domains=[]` → clears all. Updating non-domain fields SHALL NOT affect existing domains. 204 on success. Duplicate name → 400. Domain already owned by another org → 400 "Domain already exists". Org not found → 404.

#### Scenario: domains atomically replaced and lowercased
- **WHEN** POST /organisations/{id} is called with `"domains": ["A.GOV.CA", "B.GOV.CA"]`
- **THEN** exactly `["a.gov.ca", "b.gov.ca"]` are stored and all previous domains are removed

#### Scenario: domains null is a no-op
- **WHEN** POST /organisations/{id} is called with `"domains": null`
- **THEN** existing domains are unchanged

#### Scenario: domains empty clears all
- **WHEN** POST /organisations/{id} is called with `"domains": []`
- **THEN** all existing domains are removed

#### Scenario: domain already owned by another org
- **WHEN** POST /organisations/{id} is called with a domain registered to a different org
- **THEN** HTTP 400 "Domain already exists"

#### Scenario: non-domain field update does not clear domains
- **WHEN** POST /organisations/{id} is called with only `"crown": false`
- **THEN** existing domains are unchanged

#### Scenario: org not found
- **WHEN** POST /organisations/{id} is called with a non-existent organisation_id
- **THEN** HTTP 404 "Organisation not found"

---

### Requirement: MOU agreement email dispatch
When `POST /organisations/{id}` includes `agreement_signed=true` AND `agreement_signed_by_id`, the platform SHALL dispatch: (1) alert to notify-support team inbox; (2) receipt to signer (template varies on `agreement_signed_on_behalf_of_name`); (3) if `agreement_signed_on_behalf_of_email_address` set, additional email to that address. If `agreement_signed_by_id` absent → no emails sent.

#### Scenario: agreement update with by_id sends emails
- **WHEN** POST /organisations/{id} sets agreement_signed=true and agreement_signed_by_id
- **THEN** notification emails are dispatched to support team and signer

#### Scenario: agreement update without by_id sends no emails
- **WHEN** POST /organisations/{id} sets agreement_signed=true without agreement_signed_by_id
- **THEN** no notification emails are sent

---

### Requirement: Organisation type cascade to child services
When `POST /organisations/{id}` changes `organisation_type`, the new type SHALL be propagated to ALL currently linked services, with a versioned history row per service.

#### Scenario: type change cascades to all linked services
- **WHEN** POST /organisations/{id} changes organisation_type from "central" to "local"
- **THEN** all linked services have organisation_type "local" and each has a new service history row

---

### Requirement: Service linking with field inheritance and province/territory retention
`POST /organisations/{id}/services` SHALL link a service (204). Service inherits `organisation_type` + `crown` from the org; any previous org link is removed; a service history row is recorded. If org type is `province_or_territory`, email and sms data retention SHALL be set to 3 days (`PT_DATA_RETENTION_DAYS`) for each type without an existing policy. Missing `service_id` → 400. Org or service not found → 404.

#### Scenario: service inherits org type on link
- **WHEN** POST /organisations/{id}/services links a service with type "central" to a "province_or_territory" org
- **THEN** service.organisation_type becomes "province_or_territory" and a history row is recorded

#### Scenario: province_or_territory org sets 3-day retention
- **WHEN** a service is linked to a province_or_territory organisation with no existing retention policies
- **THEN** email and sms data retention are each set to 3 days

#### Scenario: re-linking removes from previous org
- **WHEN** a service in org A is linked to org B
- **THEN** the service's organisation_id is org B

#### Scenario: missing service_id returns 400
- **WHEN** POST /organisations/{id}/services is called without service_id body
- **THEN** HTTP 400

---

### Requirement: Organisation services list sorted active-first
`GET /organisations/{id}/services` SHALL return services serialised with the org-dashboard serialiser, sorted active (alphabetical) then inactive (alphabetical).

#### Scenario: services sorted active-first then alphabetical
- **WHEN** GET /organisations/{id}/services returns mixed active/inactive services
- **THEN** active services appear first (A-Z), then inactive (A-Z)

---

### Requirement: Organisation user membership
`POST /organisations/{id}/users/{user_id}` SHALL add user to org; returns 200 with full user data including `organisations`. `GET /organisations/{id}/users` SHALL return 200 `{"data": [...]}` of active members only (inactive users excluded). Missing user → 404.

#### Scenario: add user returns user with org in organisations list
- **WHEN** POST /organisations/{id}/users/{user_id} is called
- **THEN** HTTP 200 with user data containing the org id in the organisations array

#### Scenario: get users excludes inactive members
- **WHEN** GET /organisations/{id}/users is called with an inactive member present
- **THEN** the inactive user is not in the response

#### Scenario: add non-existent user returns 404
- **WHEN** POST /organisations/{id}/users/{user_id} is called with a non-existent user_id
- **THEN** HTTP 404

---

### Requirement: by-domain lookup with gsi normalisation and longest-match
`GET /organisations/by-domain?domain={domain}` SHALL return the org owning the domain. Normalisation: `.gsi.gov.uk` → `.gov.uk`; input lowercased. Matching: all Domain rows sorted by length descending (longest-match-first). No caller-context scoping. Missing `domain` param or param containing `@` → 400. No match → 404.

#### Scenario: by-domain returns owning org
- **WHEN** GET /organisations/by-domain?domain=example.gov.ca is called
- **THEN** HTTP 200 with the org that registered that domain

#### Scenario: gsi.gov.uk normalisation resolves to gov.uk owner
- **WHEN** GET /organisations/by-domain?domain=example.gsi.gov.uk is called
- **THEN** matches an org that has registered domain example.gov.uk

#### Scenario: domain param with @ returns 400
- **WHEN** GET /organisations/by-domain?domain=user@example.gov.ca is called
- **THEN** HTTP 400

#### Scenario: unregistered domain returns 404
- **WHEN** GET /organisations/by-domain?domain=unknown.example.ca is called
- **THEN** HTTP 404

---

### Requirement: Name uniqueness check endpoint
`GET /organisations/unique?org_id={id}&name={name}` SHALL return `{"result": true}` when name is available, `{"result": false}` when name matches another org case-insensitively. Org's own current name always returns true. Both params required; missing → 400 "Can't be empty".

#### Scenario: name of different org is not unique
- **WHEN** GET /organisations/unique?org_id=A&name=existing-other-org is called
- **THEN** HTTP 200 `{"result": false}`

#### Scenario: org's own name is unique
- **WHEN** GET /organisations/unique?org_id=A&name=my-own-name is called (A owns this name)
- **THEN** HTTP 200 `{"result": true}`

#### Scenario: case-insensitive collision with another org
- **WHEN** GET /organisations/unique?org_id=A&name=UNIQUE collides with "unique" owned by a different org
- **THEN** HTTP 200 `{"result": false}`

#### Scenario: missing org_id returns 400
- **WHEN** GET /organisations/unique is called without org_id
- **THEN** HTTP 400 `{"org_id": ["Can't be empty"]}`

---

### Requirement: Organisation invite flow with cleanup
`POST /organisations/{id}/invite` SHALL create an InvitedOrgUser (status=pending), send invitation email with `reply_to_text` = inviting user's email, personalisation `{organisation_name, user_name, url}`. Returns 201. URL token = `generate_token(invited_org_user.id, SECRET_KEY)`. Override URL via `invite_link_host`. Invalid email → 400. `GET /organisations/{id}/invite` returns all invitations. `POST /organisations/{id}/invite/{invite_id}` updates status (pending/accepted/cancelled); invalid status → 400; wrong org → 404. Beat task purges records older than 2 days.

#### Scenario: invite creates pending record and queues email
- **WHEN** POST /organisations/{id}/invite is called with a valid email_address
- **THEN** InvitedOrgUser created with status=pending and invitation email queued on notify-internal-tasks

#### Scenario: invite reply_to is inviter's email address
- **WHEN** an org invitation email is sent
- **THEN** reply_to_text equals the inviting user's email address

#### Scenario: invite_link_host overrides default URL
- **WHEN** POST /organisations/{id}/invite includes invite_link_host
- **THEN** the invitation URL uses that host

#### Scenario: invalid invite status returns 400
- **WHEN** POST /organisations/{id}/invite/{invite_id} is called with status="unknown"
- **THEN** HTTP 400

#### Scenario: invite from different org returns 404
- **WHEN** POST /organisations/{id}/invite/{invite_id} is called where the invite belongs to a different org
- **THEN** HTTP 404 "No result found"

#### Scenario: stale invites purged by cleanup beat task
- **WHEN** the `delete_org_invitations_created_more_than_two_days_ago` beat task runs
- **THEN** InvitedOrgUser records with created_at older than 2 days are deleted

---

### Requirement: count_organisations_with_live_services
An internal query SHALL count distinct organisations with at least one live service. A service is live when `active=true` AND `restricted=false` AND `count_as_live=true`.

#### Scenario: restricted service not counted as live
- **WHEN** CountOrganisationsWithLiveServices is called and an org has only restricted=true services
- **THEN** that org is not included in the count
