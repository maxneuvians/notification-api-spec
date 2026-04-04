## Source Files

- `spec/behavioral-spec/organisations.md` — endpoint contracts, DAO behavior, business rules verified
- `spec/business-rules/organisations.md` — data-access patterns, DAO internals, domain rules, invariants, error table

---

## Endpoints

### GET /organisations
- Returns all organisations (active + inactive); fields: `name`, `id`, `active`, `count_of_live_services`, `domains`, `organisation_type`
- Results sorted: active-first, then alphabetical by name
- `domains` always present (empty list if none); `organisation_type` may be null
- Auth: admin JWT

### GET /organisations/{organisation_id}
- Full fields: all scalars + `domains`, `request_to_go_live_notes`, `count_of_live_services`
- All nullable fields default to null on newly created org
- `domains` returns list of strings, order not guaranteed

### GET /organisations/by-domain?domain={domain}
- 200 + org object when domain is registered
- domain param required; missing → 400; contains `@` → 400
- Domain not matched → 404
- Always returns the owning org regardless of caller context (no access-scoping)
- MUST implement xfail semantics: returns whatever org owns the domain

### POST /organisations
- Creates new org; 201 + created object
- Required: `name` (string), `crown` (boolean — `null` rejected: "crown None is not of type boolean"), `organisation_type` (enum)
- Valid `organisation_type` values: `central`, `province_or_territory`, `local`, `nhs_central`, `nhs_local`, `nhs_gp`, `emergency_service`, `school_or_college`, `other`
- `active` accepted but not required
- Duplicate name → 400 "Organisation name already exists"
- Missing required field → 400 ValidationError (one error per field)

### POST /organisations/{organisation_id}  (update)
- 204 on success; partial updates supported
- `domains` update: full replacement semantics (replaces all existing domains)
- Updating non-domain fields does NOT clear domains
- `default_branding_is_french` can be toggled
- Duplicate name → 400 "Organisation name already exists"
- Domain already registered to another org → 400 "Domain already exists"
- 404 if org not found (dao returns 0 rows)
- `agreement_signed=true` + `agreement_signed_by_id` present → MOU emails sent (see below)
- `agreement_signed=true` without `agreement_signed_by_id` → no emails (silent)
- `email_branding_id` and `letter_branding_id` settable independently

### POST /organisations/{organisation_id}/services
- Links service to org; 204
- Body must contain `service_id`; missing → 400
- 404 if org_id or service_id not found
- Service from previous org is automatically unlinked (re-linking)
- Service inherits org's `organisation_type` + `crown`; service history recorded
- If org is `province_or_territory` → set email + sms data retention to 3 days (PT_DATA_RETENTION_DAYS) if no policy exists

### GET /organisations/{organisation_id}/services
- Returns services serialised with `serialize_for_org_dashboard()`; 200
- Sorted: active services first (alphabetical), then inactive services (alphabetical)
- Returns empty list for orgs with no services

### POST /organisations/{organisation_id}/users/{user_id}
- Adds user to org; 200 with full user data including `organisations` array
- 404 if user_id not found

### GET /organisations/{organisation_id}/users
- Returns 200 `{"data": [...]}` of all active org members (inactive users excluded)
- Empty list when no members

### GET /organisations/unique?org_id={id}&name={name}
- 200 `{"result": true}` when name is available
- Both params required; missing → 400 "Can't be empty"
- Return `false` when name matches another org (case-insensitive)
- Org's own current name always returns true (no self-collision)
- Case-insensitive check: "UNIQUE" collides with existing "unique" for a different org
- Same org: decorating the name does NOT cause collision (only exact case-insensitive match with another org triggers false)

### POST /organisations/{organisation_id}/invite
- Creates InvitedOrgUser; 201 with `data`: `organisation`, `email_address`, `invited_by`, `status`, `id`
- `email_address` required; invalid → 400
- Sends invitation email via `deliver_email` on `notify-internal-tasks` queue
- `reply_to_text` = inviting user's email address
- Personalisation: `organisation_name`, `user_name`, `url`
- URL defaults to `http://localhost:6012/organisation-invitation/{token}`; overridden by `invite_link_host`

### GET /organisations/{organisation_id}/invite
- Returns 200 `{"data": [...]}` of all invitations for org
- Empty list when none exist

### POST /organisations/{organisation_id}/invite/{invited_org_user_id}
- Updates invite status; 200 with `data.status`
- Valid statuses: `pending`, `accepted`, `cancelled`; invalid → 400
- invite_id not in org → 404 "No result found"

---

## Domain Rules & Invariants

### Organisation types (enum)
`central`, `province_or_territory`, `local`, `nhs_central`, `nhs_local`, `nhs_gp`, `emergency_service`, `school_or_college`, `other`

Crown orgs: `nhs_central`; Non-crown: `local`, `nhs_local`, `nhs_gp`, `emergency_service`, `school_or_college`; NHS: `nhs_central`, `nhs_local`, `nhs_gp`

### Domain handling
- Stored lowercase; uppercase input normalised on write
- Global uniqueness: `domain.domain` is PK → domain can only belong to one org
- Atomic replacement: delete all + bulk-insert in same transaction
- Lookup normalises `.gsi.gov.uk` → `.gov.uk`
- Longest-domain-first matching; both exact and subdomain matches supported

### Service inheritance cascade
- Service belongs to at most one org at a time (re-linking removes from previous)
- On `dao_add_service_to_organisation`: copy `organisation_type` + `crown` from org onto service; record service history version
- On `dao_update_organisation` with `organisation_type` change: cascade to ALL child services, each with a history row (`@version_class(Service)`)
- Province/territory org: set email+sms data retention to 3 days if not already set

### MOU agreement emails
- Triggered when `agreement_signed=true` AND `agreement_signed_by_id` is in the update payload
- Sends 2 emails: one to notify-support team inbox, one to signer
- Template varies: plain signer vs. signing on behalf of someone
- If `agreement_signed_on_behalf_of_email_address` set: additional email to on-behalf-of party
- No notification if `agreement_signed_by_id` absent

### Invitation lifecycle
- Status: `pending` → `accepted` | `cancelled`
- Records older than 2 days deleted by cleanup task `delete_org_invitations_created_more_than_two_days_ago`
- Token is signed (`generate_token(invited_org_user.id, SECRET_KEY)`) and URL-safe

### Organisation name uniqueness
- DB: `ix_organisation_name` case-sensitive unique index
- REST layer: case-insensitive pre-check via `/unique` endpoint (ilike query)

### User membership
- Many-to-many via `users_organisations` join table; binary membership (no per-user org role)
- `dao_get_users_for_organisation`: filters `state = 'active'` only
- `dao_add_user_to_organisation`: `@transactional`; raises SQLAlchemyError if user/org not found

### `dao_count_organisations_with_live_services()`
- Counts distinct orgs with ≥1 live service
- Live service conditions: `active IS TRUE` AND `restricted IS FALSE` AND `count_as_live IS TRUE`
- Note: Python has typo "organsations" — Go should use correct spelling

---

## Error Conditions

| Condition | Response |
|-----------|----------|
| IntegrityError on `ix_organisation_name` | 400 "Organisation name already exists" |
| IntegrityError on `domain_pkey` | 400 "Domain already exists" |
| Other IntegrityError | 500 logged |
| `dao_update_organisation` returns 0 rows | 404 "Organisation not found" |
| `get_org_by_domain`: `domain` param missing or contains `@` | 400 |
| `get_org_by_domain`: no match | 404 |
| `/unique`: `org_id` missing | 400 `{"org_id": ["Can't be empty"]}` |
| `/unique`: `name` missing | 400 `{"name": ["Can't be empty"]}` |
| Invite: invalid email | 400 "email_address Not a valid email address" |
| Invite update: invalid status | 400 |
| Invite not in org | 404 "No result found" |
| `dao_add_user_to_organisation`: user/org not found | SQLAlchemyError → 404 |
