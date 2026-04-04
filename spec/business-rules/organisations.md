# Business Rules: Organisations

## Overview

Organisations are the top-level administrative entities in the platform. They sit above services in the hierarchy: one organisation may own many services, and each service belongs to at most one organisation. Organisations also hold users (org-level members), email/letter branding assets, domain records (for auto-association by email address), and MOU (memorandum of understanding) agreement metadata. Platform-level settings such as `organisation_type` and `crown` status cascade down to child services.

---

## Data Access Patterns

### `organisation_dao.py`

#### `dao_get_organisations()`
- **Purpose**: Return all organisations for list views.
- **Query type**: SELECT all
- **Key filters/conditions**: None; returns all rows.
- **Returns**: List of `Organisation` objects ordered by `active DESC`, `name ASC` (active orgs first, then alphabetical).
- **Notes**: Used when rendering admin dashboards or partner-facing org pickers.

---

#### `dao_count_organsations_with_live_services()`
- **Purpose**: Return a scalar count of how many distinct organisations have at least one live service.
- **Query type**: SELECT COUNT (distinct)
- **Key filters/conditions**: Joins `Organisation` → `Service`; filters `Service.active IS TRUE`, `Service.restricted IS FALSE`, `Service.count_as_live IS TRUE`.
- **Returns**: Integer count.
- **Notes**: A service qualifies as "live" only when all three conditions hold simultaneously. Note the intentional typo in the function name (`organsations`).

---

#### `dao_get_organisation_services(organisation_id)`
- **Purpose**: Retrieve all services that belong to a given organisation.
- **Query type**: SELECT via relationship traversal
- **Key filters/conditions**: Loads the `Organisation` by `id` (raises if not found), then returns its `services` relationship.
- **Returns**: List of `Service` objects (all statuses, no filtering).
- **Notes**: The REST layer sorts this list by `(-active, name)` before serialisation.

---

#### `dao_get_organisation_by_id(organisation_id)`
- **Purpose**: Fetch a single organisation by primary key.
- **Query type**: SELECT → `.one()`
- **Key filters/conditions**: `id = organisation_id`.
- **Returns**: `Organisation` object; raises `NoResultFound` if absent.

---

#### `dao_get_organisation_by_email_address(email_address)`
- **Purpose**: Resolve a user's email address to an organisation by matching registered domains.
- **Query type**: SELECT all `Domain` rows, then SELECT `Organisation` by matched `organisation_id`.
- **Key filters/conditions**:
  1. Input is lower-cased and `.gsi.gov.uk` suffix is rewritten to `.gov.uk`.
  2. All `Domain` rows are fetched, sorted by `char_length(domain) DESC` (longest match wins).
  3. The email is tested against each domain using two patterns: exact address-domain match (`@<domain>`) or subdomain match (`.<domain>`).
- **Returns**: `Organisation` object for the first matching domain, or `None` if no domain matches.
- **Notes**: The gsi→gov normalisation ensures legacy `.gsi.gov.uk` addresses resolve correctly. Longest-match ordering prevents shorter domains from incorrectly masking more specific ones.

---

#### `dao_get_organisation_by_service_id(service_id)`
- **Purpose**: Look up the parent organisation of a service.
- **Query type**: SELECT with join
- **Key filters/conditions**: Joins `Organisation` → `services`, filters `Service.id = service_id`.
- **Returns**: `Organisation` object or `None` if the service has no parent organisation.

---

#### `dao_create_organisation(organisation)`
- **Purpose**: Persist a new `Organisation` record.
- **Query type**: INSERT
- **Key filters/conditions**: None.
- **Returns**: Nothing (side-effect only).
- **Notes**: Wrapped with `@transactional`; the caller constructs the `Organisation` object before passing it in. Uniqueness of `name` is enforced by the `ix_organisation_name` database index.

---

#### `dao_update_organisation(organisation_id, **kwargs)`
- **Purpose**: Partial update of any scalar field on an organisation, plus optional atomic replacement of its domain list.
- **Query type**: UPDATE (scalar fields); DELETE + bulk INSERT (domains); SELECT + UPDATE (email branding relationship); cascade UPDATE (services if org type changes).
- **Key filters/conditions**:
  - All scalar `kwargs` are applied via `Organisation.query.filter_by(id=organisation_id).update(kwargs)`.
  - `domains` (if present and a list): all existing `Domain` rows for the org are deleted, then new ones are bulk-inserted (values lower-cased).
  - `email_branding_id` (if present): fetches the `EmailBranding` record and sets the ORM relationship directly in addition to the column.
  - `organisation_type` (if present): triggers `_update_org_type_for_organisation_services` to cascade the new type to all child services.
- **Returns**: Number of rows updated (0 if no org found, which the REST layer maps to 404).
- **Notes**: `@transactional`; domain replacement is all-or-nothing within the same transaction.

---

#### `_update_org_type_for_organisation_services(organisation)`  *(private)*
- **Purpose**: Cascade an organisation's `organisation_type` to every service it owns.
- **Query type**: UPDATE (one row per service)
- **Key filters/conditions**: Iterates `organisation.services`; sets `service.organisation_type = organisation.organisation_type` on each.
- **Returns**: Nothing.
- **Notes**: Decorated with `@version_class(Service)` so each service update produces a versioned history row.

---

#### `dao_add_service_to_organisation(service, organisation_id)`
- **Purpose**: Associate an existing service with an organisation, inheriting the org's type and crown status.
- **Query type**: UPDATE (service row)
- **Key filters/conditions**: Loads the `Organisation` by `id`; sets `service.organisation_id`, `service.organisation_type`, and `service.crown` from the org.
- **Returns**: Nothing.
- **Notes**: `@transactional` and `@version_class(Service)`; the REST layer clears `service.organisation` before calling this to handle re-linking.

---

#### `dao_get_invited_organisation_user(user_id)`
- **Purpose**: Fetch an `InvitedOrganisationUser` record by its own primary key, without scoping to a specific org.
- **Query type**: SELECT → `.one()`
- **Key filters/conditions**: `id = user_id`.
- **Returns**: `InvitedOrganisationUser` object.

---

#### `dao_get_users_for_organisation(organisation_id)`
- **Purpose**: List confirmed (active) members of an organisation.
- **Query type**: SELECT with many-to-many filter
- **Key filters/conditions**: `User.organisations.any(id=organisation_id)` AND `User.state = 'active'`.
- **Returns**: List of `User` objects ordered by `created_at ASC`.
- **Notes**: Deliberately excludes pending/inactive users; invitation-pending users are tracked in `InvitedOrganisationUser` until accepted.

---

#### `dao_add_user_to_organisation(organisation_id, user_id)`
- **Purpose**: Grant an existing user membership in an organisation.
- **Query type**: INSERT into the `users_organisations` join table (via ORM relationship append).
- **Key filters/conditions**: Loads both the org and the user, then appends the org to `user.organisations`.
- **Returns**: The `User` object.
- **Notes**: `@transactional`.

---

### `invited_org_user_dao.py`

#### `save_invited_org_user(invited_org_user)`
- **Purpose**: Persist a new or updated `InvitedOrganisationUser` record.
- **Query type**: INSERT or UPDATE
- **Key filters/conditions**: None.
- **Returns**: Nothing.
- **Notes**: Calls `db.session.commit()` directly (not using `@transactional` decorator); used for both creation and status updates.

---

#### `get_invited_org_user(organisation_id, invited_org_user_id)`
- **Purpose**: Fetch an invitation record scoped to a specific organisation (used when handling invite-status updates from a URL that includes the org id).
- **Query type**: SELECT → `.one()`
- **Key filters/conditions**: `organisation_id = organisation_id` AND `id = invited_org_user_id`.
- **Returns**: `InvitedOrganisationUser` object; raises if not found.
- **Notes**: The dual-filter prevents users from manipulating invites across org boundaries.

---

#### `get_invited_org_user_by_id(invited_org_user_id)`
- **Purpose**: Fetch an invitation record without org scoping (used for internal lookups).
- **Query type**: SELECT → `.one()`
- **Key filters/conditions**: `id = invited_org_user_id`.
- **Returns**: `InvitedOrganisationUser` object.

---

#### `get_invited_org_users_for_organisation(organisation_id)`
- **Purpose**: List all pending/past invitation records for an organisation.
- **Query type**: SELECT all matching
- **Key filters/conditions**: `organisation_id = organisation_id`.
- **Returns**: List of `InvitedOrganisationUser` objects (all statuses).

---

#### `delete_org_invitations_created_more_than_two_days_ago()`
- **Purpose**: Purge stale invitation records as a scheduled cleanup task.
- **Query type**: DELETE (bulk)
- **Key filters/conditions**: `InvitedOrganisationUser.created_at <= utcnow() - 2 days`.
- **Returns**: Count of deleted rows.
- **Notes**: Commits immediately; intended to be called from a periodic Celery beat task.

---

## Domain Rules & Invariants

### Organisation Types
The `organisation_type` field is constrained to the following values (stored in the `organisation_types` lookup table):

| Value | Description |
|---|---|
| `central` | Central / federal government body |
| `province_or_territory` | Provincial or territorial government body |
| `local` | Local / municipal government body |
| `nhs_central` | NHS Central (Crown) |
| `nhs_local` | NHS Local (Non-Crown) |
| `nhs_gp` | NHS GP practice (Non-Crown) |
| `emergency_service` | Emergency service (Non-Crown) |
| `school_or_college` | School or college (Non-Crown) |
| `other` | All other organisations |

Predefined groupings:
- **Crown organisations**: `nhs_central`
- **Non-Crown organisations**: `local`, `nhs_local`, `nhs_gp`, `emergency_service`, `school_or_college`
- **NHS organisations**: `nhs_central`, `nhs_local`, `nhs_gp`

The `crown` boolean on `Organisation` is separate from the type groupings and may be set independently.

---

### Organisation–Service Relationship
- A service has at most one parent organisation (`services.organisation_id`).
- When a service is added to an organisation (`dao_add_service_to_organisation`), the service inherits both `organisation_type` and `crown` from the organisation.
- If an organisation's `organisation_type` is subsequently changed, the new type cascades to **all** of its services automatically.
- When a service is linked to a `province_or_territory` organisation, data retention for both `email` and `sms` notification types is set to **3 days** (`PT_DATA_RETENTION_DAYS`) if no retention policy already exists for that type.
- A service's `count_as_live`, `active`, and `restricted` flags together determine whether it counts toward the organisation's live-service count.

---

### Organisation–User Relationship
- Users are associated with organisations through a many-to-many relationship (join table `users_organisations`).
- Only users in `state = 'active'` are returned when listing org members; pending/invitees appear in `InvitedOrganisationUser` until they accept.
- There is no per-user role within an organisation at the data layer; org membership is binary (member or not).

---

### Domain Verification
- Each organisation may register one or more email domains in the `domain` table.
- Domain values are always stored lower-case.
- The `domain.domain` column is the primary key, so domains are globally unique across all organisations — a domain cannot belong to two organisations simultaneously.
- When domains are updated, the replacement is **atomic**: all existing domain rows for the organisation are deleted and the new set is bulk-inserted in the same transaction.
- Lookups normalise the input: `.gsi.gov.uk` addresses are rewritten to `.gov.uk` before matching.
- Matching uses **longest-domain-first** ordering to prevent short domains masking more specific sub-domains.
- Both exact address-domain matches (`user@example.gov.uk`) and subdomain matches (`user@sub.example.gov.uk`) are recognised.

---

### Invitation Flow for Org Users
1. A caller POSTs to `/organisation/<org_id>/invite` with `email_address` and `invited_by` (user id).
2. An `InvitedOrganisationUser` row is created with `status = 'pending'`.
3. An email notification is sent immediately via the platform's own delivery pipeline, using the `ORGANISATION_INVITATION_EMAIL_TEMPLATE_ID` template. The email contains a signed URL-safe token (`generate_token(invited_org_user.id, SECRET_KEY)`) pointing to `{admin_base_url}/organisation-invitation/{token}`.
4. The invitee clicks the link; the admin app decodes the token, retrieves the invitation by id, and POSTs the new status (`accepted` or `cancelled`) to `/<org_id>/invite/<invited_org_user_id>`.
5. On acceptance the admin app separately calls `POST /organisation/<org_id>/users/<user_id>` to create the membership record.
6. Invitation records older than **2 days** are deleted by the cleanup function `delete_org_invitations_created_more_than_two_days_ago`.

Invitation statuses: `pending`, `accepted`, `cancelled`.

---

### Branding Inheritance
- An organisation may have an `email_branding_id` and a `letter_branding_id` pointing to shared branding assets.
- When `email_branding_id` is updated via `dao_update_organisation`, the ORM relationship (`org.email_branding`) is also refreshed within the same transaction to keep the in-memory object consistent.
- Individual services may override branding independently; the organisation-level branding serves as the default for new services.
- `default_branding_is_french` is a boolean on both `Organisation` and `Service`; the service copy is set independently and is not automatically cascaded from the org.

---

### MOU Agreement Tracking
- `agreement_signed` (boolean), `agreement_signed_at`, `agreement_signed_by_id`, `agreement_signed_version`, `agreement_signed_on_behalf_of_name`, and `agreement_signed_on_behalf_of_email_address` are all stored on the `Organisation` row.
- When `agreement_signed = true` is set **and** `agreement_signed_by_id` is present in the same `update_organisation` request, the platform sends transactional email notifications via `send_notifications_on_mou_signed`:
  - One alert to the notify-support team inbox.
  - One receipt to the signer (`agreement_signed_by.email_address`) — template varies depending on whether they signed on behalf of someone.
  - If `agreement_signed_on_behalf_of_email_address` is set, an additional notification is sent to the on-behalf-of party.
- If `agreement_signed_by_id` is absent from the update payload, no notifications are sent (allows platform admins to adjust the flag silently).

---

### Organisation Name Uniqueness
- The `ix_organisation_name` index enforces case-sensitive uniqueness at the database level.
- The `/unique` endpoint provides an application-level case-insensitive pre-check (`ilike` query) before write attempts.

---

## Error Conditions

| Location | Condition | Response |
|---|---|---|
| `rest.py` – `handle_integrity_error` | `IntegrityError` containing `ix_organisation_name` | 400 `"Organisation name already exists"` |
| `rest.py` – `handle_integrity_error` | `IntegrityError` containing `domain_pkey` | 400 `"Domain already exists"` |
| `rest.py` – `handle_integrity_error` | Any other `IntegrityError` | 500 `"Internal server error"` (logged) |
| `rest.py` – `update_organisation` | `dao_update_organisation` returns 0 rows | 404 `"Organisation not found"` (via `InvalidRequest`) |
| `rest.py` – `get_organisation_by_domain` | `domain` query param absent or contains `@` | 400 (Flask `abort`) |
| `rest.py` – `get_organisation_by_domain` | No matching organisation found | 404 (Flask `abort`) |
| `rest.py` – `check_request_args` | `org_id` query param absent | 400 `{"org_id": ["Can't be empty"]}` |
| `rest.py` – `check_request_args` | `name` query param absent | 400 `{"name": ["Can't be empty"]}` |
| `dao_get_organisation_by_id` | No matching row | `sqlalchemy.orm.exc.NoResultFound` (unmapped; caller or framework handles) |
| `dao_get_users_for_organisation` | (implicit) | Returns empty list; no explicit exception |
| `get_invited_org_user` | No row matching both `organisation_id` and `id` | `sqlalchemy.orm.exc.NoResultFound` |

---

## Query Inventory (for sqlc)

| Query name | Type | Tables | Description |
|---|---|---|---|
| `GetOrganisations` | SELECT | `organisation` | All organisations ordered by active desc, name asc |
| `CountOrganisationsWithLiveServices` | SELECT COUNT | `organisation`, `services` | Distinct orgs with at least one live service |
| `GetOrganisationServices` | SELECT | `organisation`, `services` | All services for a given organisation id |
| `GetOrganisationByID` | SELECT ONE | `organisation` | Organisation by primary key |
| `GetOrganisationByEmailAddress` | SELECT ALL + SELECT ONE | `domain`, `organisation` | Domain-to-org resolution; longest domain first |
| `GetOrganisationByServiceID` | SELECT ONE | `organisation`, `services` | Parent organisation of a service |
| `CreateOrganisation` | INSERT | `organisation` | Insert new organisation row |
| `UpdateOrganisation` | UPDATE | `organisation` | Partial scalar update of organisation fields |
| `DeleteDomainsForOrganisation` | DELETE | `domain` | Remove all domain rows for an organisation (part of atomic domain replacement) |
| `BulkInsertDomains` | INSERT | `domain` | Insert new domain set for an organisation |
| `UpdateEmailBrandingForOrganisation` | UPDATE | `organisation` | Set email_branding_id on organisation |
| `CascadeOrgTypeToServices` | UPDATE | `services` | Set organisation_type on all services for an org |
| `AddServiceToOrganisation` | UPDATE | `services` | Set organisation_id, organisation_type, crown on a service |
| `GetInvitedOrganisationUserByID` | SELECT ONE | `invited_organisation_users` | Invite record by id (no org scope) |
| `GetUsersForOrganisation` | SELECT | `users`, `user_to_organisation` | Active users for an organisation, ordered by created_at |
| `AddUserToOrganisation` | INSERT | `user_to_organisation` | Insert user–organisation membership row |
| `SaveInvitedOrgUser` | INSERT / UPDATE | `invited_organisation_users` | Upsert invitation record |
| `GetInvitedOrgUser` | SELECT ONE | `invited_organisation_users` | Invite by organisation_id + id |
| `GetInvitedOrgUserByIDGlobal` | SELECT ONE | `invited_organisation_users` | Invite by id only |
| `GetInvitedOrgUsersForOrganisation` | SELECT | `invited_organisation_users` | All invites for an organisation |
| `DeleteExpiredOrgInvitations` | DELETE | `invited_organisation_users` | Remove invites created more than 2 days ago |
