## Context
Organisations are the top-level administrative entities: one org → many services → many users. Organisation fields (`organisation_type`, `crown`) cascade to child services. Domain records enable automatic org resolution from a user's email address. This change implements all 14 REST endpoints plus the underlying repository layer.

## Goals / Non-Goals
**Goals:** Organisation CRUD (14 endpoints), domain atomic replacement, service linking with field inheritance, MOU agreement email dispatch, org-user membership, invitations, name uniqueness, `by-domain` lookup, `count_organisations_with_live_services`.

**Non-Goals:** Email/letter branding asset CRUD (platform-admin-features), user CRUD (user-management).

## Decisions

### Domain replacement is atomic
`POST /organisations/{id}` with `domains` field: DELETE all existing `Domain` rows for the org + bulk-INSERT new values (lowercased) in one transaction. `domains=null` → no-op (existing domains preserved). `domains=[]` → clears all domains.

### Organisation type cascades to services via history write
`UpdateOrganisation` with `organisation_type` propagates via an internal helper: iterates all child services, sets `service.organisation_type`, writes a versioned history row per service. `AddServiceToOrganisation` also copies both `organisation_type` and `crown` from org onto service at link time.

### Province/territory data retention trigger
When a service is linked to a `province_or_territory` org, if no retention policy exists for email or sms, create a 3-day (`PT_DATA_RETENTION_DAYS = 3`) retention record for each missing type.

### by-domain lookup: longest-match, no access-scoping
`GetOrganisationByEmailAddress`: fetch all Domain rows, sort by domain length descending; lowercase + normalise `.gsi.gov.uk` → `.gov.uk`; return first match unconditionally (no caller-context scoping). `GET /organisations/by-domain`: `domain` param with `@` returns 400; no match returns 404.

### MOU emails: conditional on agreement_signed_by_id presence
When `agreement_signed=true` AND `agreement_signed_by_id` is in the update payload, dispatch: (1) alert to support-team inbox, (2) receipt to signer (template varies on `agreement_signed_on_behalf_of_name`), (3) optional copy to `agreement_signed_on_behalf_of_email_address`. If `agreement_signed_by_id` absent — no emails sent.

### Name uniqueness: two-layer guard
DB: `ix_organisation_name` case-sensitive unique index. REST layer: `/unique` endpoint case-insensitive pre-check (ilike). `IntegrityError` on name index → 400 "Organisation name already exists". `IntegrityError` on `domain_pkey` → 400 "Domain already exists".

### Invite token
URL token: `generate_token(invited_org_user.id, SECRET_KEY)` (URL-safe signed). Stale invites (>2 days) purged by scheduled beat task. `reply_to_text` = inviting user's email address.

## Risks / Trade-offs
- Service type cascade is O(n services per org) per update — acceptable for current org sizes.
- Domain PK uniqueness means swapping a domain between orgs requires two atomic updates.
