# Behavioral Spec: Organisations

## Processed Files
- [x] tests/app/organisation/test_rest.py
- [x] tests/app/organisation/test_invite_rest.py
- [x] tests/app/dao/test_organisation_dao.py

---

## Endpoint Contracts

### GET /organisations

- **Happy path**: Returns all organisations (active and inactive). Each entry contains exactly the fields: `name`, `id`, `active`, `count_of_live_services`, `domains`, `organisation_type`. Results are returned in the order provided by the DAO (active-first, then alphabetical — see DAO section).
- **Validation rules**: No query parameters required.
- **Error cases**: None tested.
- **Auth requirements**: Admin API key (all organisation endpoints use `admin_request`).
- **Notable edge cases**:
  - `domains` is always present; returns an empty list `[]` when none are set.
  - `organisation_type` may be `null`.
  - `count_of_live_services` defaults to `0` for newly created organisations.

---

### GET /organisations/`{organisation_id}`

- **Happy path**: Returns full organisation detail with all fields: `id`, `name`, `active`, `crown`, `default_branding_is_french`, `organisation_type`, `agreement_signed`, `agreement_signed_at`, `agreement_signed_by_id`, `agreement_signed_version`, `agreement_signed_on_behalf_of_name`, `agreement_signed_on_behalf_of_email_address`, `letter_branding_id`, `email_branding_id`, `domains`, `request_to_go_live_notes`, `count_of_live_services`.
- **Validation rules**: `organisation_id` must resolve to an existing organisation.
- **Error cases**: No 404 test in this file for this endpoint specifically, but behaviour is consistent with other lookup endpoints.
- **Auth requirements**: Admin API key.
- **Notable edge cases**:
  - All nullable fields (`crown`, `organisation_type`, `agreement_signed`, `agreement_signed_by_id`, etc.) default to `null` on a freshly created organisation.
  - `domains` returns a list of domain strings; order is not guaranteed (tested with set comparison).

---

### GET /organisations/by-domain?domain=`{domain}`

- **Happy path**: `200` with organisation object (including `id`) when the given domain is registered to an organisation.
- **Validation rules**:
  - `domain` query parameter is required; missing value returns `400`.
  - Value must not contain `@` (i.e., must not look like an email address); such values return `400` with `result: error`.
- **Error cases**:
  - `404` with `result: error` if domain is not registered to any organisation.
  - `400` if `domain` is absent or contains `@`.
- **Auth requirements**: Admin API key.
- **Notable edge cases**:
  - A domain registered to a *different* organisation returns `200` but with that other organisation's `id`. The test for this case is marked `xfail` (expected assertion failure), meaning the endpoint does not differentiate — it returns whatever org owns the domain.
    - **⚠️ Ambiguous semantics**: it is intentionally unspecified whether context (i.e. which org is making the request) matters for this lookup. The xfail test implies the current Python behaviour is to return the owning org regardless of the requesting org. Go must replicate this: `GET /organisations/by-domain?domain=X` always returns the single org that has registered domain X, with no access-scoping by the caller.

---

### POST /organisations

- **Happy path**: Creates a new organisation. Returns `201` with the created organisation fields. `crown` may be `true`, `false`, or `null` (but not JSON `null` — see validation).
- **Validation rules** (all enforced, returning `400` with `ValidationError`):
  - `name` — required string.
  - `crown` — required boolean; `null` value is rejected: `"crown None is not of type boolean"`.
  - `organisation_type` — required; must be one of: `central`, `province_or_territory`, `local`, `nhs_central`, `nhs_local`, `nhs_gp`, `emergency_service`, `school_or_college`, `other`.
- **Error cases**:
  - `400` with `message: "Organisation name already exists"` if a different organisation already has the same name.
  - `400` with `errors[].error = "ValidationError"` for any missing/invalid required field.
- **Auth requirements**: Admin API key.
- **Notable edge cases**:
  - `active` is accepted but not required in the creation payload.
  - Each missing required field produces exactly one validation error entry.

---

### POST /organisations/`{organisation_id}`

- **Happy path**: Updates organisation fields. Returns `204` on success.
- **Validation rules**:
  - Partial updates are supported — only provided fields are modified.
  - `domains` update replaces all existing domains (full replacement semantics).
  - Updating non-domain fields (e.g., `crown`) does **not** clear existing domains.
  - `default_branding_is_french` defaults to `false` on a new organisation and can be toggled to `true`.
- **Error cases**:
  - `400` with `message: "Organisation name already exists"` when renaming to a name owned by a different organisation.
  - `400` with `message: "Domain already exists"` when the `domains` list includes a domain already registered to a different organisation.
  - `404` when `organisation_id` does not exist.
- **Auth requirements**: Admin API key.
- **Notable edge cases**:
  - Setting `agreement_signed: true` **without** `agreement_signed_by_id` does **not** trigger any email notification.
  - Setting `agreement_signed: true` **with** `agreement_signed_by_id` triggers MOU notification emails. Two notification templates are used depending on whether `agreement_signed_on_behalf_of_name` is set (signing on behalf of someone vs. direct signer). This test is currently `@pytest.mark.skip`.
    - **⚠️ Skipped test**: the MOU email-trigger behavior is documented above but the test is skipped and unverified in Python. Go must implement the email dispatch and add test coverage.
  - `email_branding_id` and `letter_branding_id` can be set independently and do not interfere with each other.

---

### POST /organisations/`{organisation_id}`/services

- **Happy path**: Links a service to an organisation. Returns `204`.
- **Validation rules**:
  - Request body must contain `service_id`; missing payload returns `400`.
- **Error cases**:
  - `404` if `organisation_id` does not exist.
  - `404` if `service_id` does not resolve to an existing service.
- **Auth requirements**: Admin API key.
- **Notable edge cases**:
  - A service can only belong to one organisation at a time. Linking a service to a **new** organisation automatically removes it from the previous organisation.
  - When the organisation is of type `province_or_territory`, data retention for both `email` and `sms` notification types is automatically set to **3 days** for the service.

---

### GET /organisations/`{organisation_id}`/services

- **Happy path**: Returns a list of services serialised with `serialize_for_org_dashboard()`. Returns `200`.
- **Validation rules**: None beyond valid `organisation_id`.
- **Error cases**: None tested explicitly.
- **Auth requirements**: Admin API key.
- **Notable edge cases**:
  - Results are sorted: **active services first** (alphabetically by name), then **inactive services** (alphabetically by name).

---

### POST /organisations/`{organisation_id}`/users/`{user_id}`

- **Happy path**: Adds the user to the organisation. Returns `200` with the user's full data, including an `organisations` array that contains the organisation id.
- **Validation rules**: Both `organisation_id` and `user_id` must be valid UUIDs resolving to existing records.
- **Error cases**:
  - `404` if `user_id` does not resolve to an existing user.
- **Auth requirements**: Admin API key.
- **Notable edge cases**:
  - The returned `organisations` list reflects the association immediately after creation.

---

### GET /organisations/`{organisation_id}`/users

- **Happy path**: Returns `200` with `{"data": [...]}` listing all users associated with the organisation.
- **Validation rules**: None beyond valid `organisation_id`.
- **Error cases**: None tested explicitly.
- **Auth requirements**: Admin API key.
- **Notable edge cases**: Results include multiple users; ordering matches insertion order via the DAO.

---

### GET /organisations/unique?org_id=`{id}`&name=`{name}`

- **Happy path**: Returns `200` with `{"result": true}` when the name is available.
- **Validation rules**:
  - Both `org_id` and `name` are required; missing either returns `400` with field-level messages (`"Can't be empty"`).
- **Error cases**:
  - `{"result": false}` when the name (case-insensitively) matches an organisation other than the one identified by `org_id`.
- **Auth requirements**: Admin API key.
- **Notable edge cases**:
  - Name uniqueness comparison is **case-insensitive**: `"UNIQUE"` and `"Unique"` collide with an existing `"unique"` for a *different* org.
  - A name with added capitalisation or punctuation (e.g., `"Unique."`, `"**uniQUE**"`) is considered **unique / available** when checked for the *same* org that already owns the plain version — even though normalised they would collide. (The check returns `true` for same-org regardless.)
  - An org's own current name always returns `{"result": true}` (no self-collision).

---

### POST /organisations/`{organisation_id}`/invite

- **Happy path**: Creates an `InvitedOrgUser` with `status = pending`. Returns `201` with `data` object containing `organisation`, `email_address`, `invited_by`, `status`, `id`.
- **Validation rules**:
  - `email_address` must be a valid email address; invalid values return `400` with `"email_address Not a valid email address"`.
- **Error cases**:
  - `400` on invalid email.
- **Auth requirements**: Admin API key.
- **Notable edge cases**:
  - An invite email is dispatched via `deliver_email.apply_async` to the `notify-internal-tasks` queue as a single-element list of the notification id.
  - The notification's `reply_to_text` is set to the **inviting user's email address**.
  - Personalisation keys: `organisation_name`, `user_name`, `url`.
  - The invite URL defaults to `http://localhost:6012/organisation-invitation/{token}` but can be overridden by passing `invite_link_host` in the request body.

---

### GET /organisations/`{organisation_id}`/invite

- **Happy path**: Returns `200` with `{"data": [...]}` of all invitations for the organisation.
- **Validation rules**: None beyond valid `organisation_id`.
- **Error cases**: None tested explicitly.
- **Auth requirements**: Admin API key.
- **Notable edge cases**: Returns an empty `data` list when no invitations exist.

---

### POST /organisations/`{organisation_id}`/invite/`{invited_org_user_id}`

- **Happy path**: Updates the status of an invitation. Returns `200` with `data.status` reflecting the new value.
- **Validation rules**:
  - `status` must be one of: `pending`, `accepted`, `cancelled`; invalid values return `400`.
- **Error cases**:
  - `404` with `"No result found"` if the `invited_org_user_id` does not belong to the specified `organisation_id`.
  - `400` if `status` is an invalid value.
- **Auth requirements**: Admin API key.
- **Notable edge cases**: Typical use is setting status to `"cancelled"`.

---

## DAO Behavior Contracts

### `dao_get_organisations()`

- **Expected behavior**: Returns all organisations sorted active-first, then inactive. Within each group, sorted alphabetically by name (case-sensitive/collation-dependent).
- **Edge cases verified**: Five organisations of mixed active/inactive state → strict ordering of active-alpha then inactive-alpha is enforced.

---

### `dao_get_organisation_by_id(organisation_id)`

- **Expected behavior**: Returns the `Organisation` ORM object matching the given id.
- **Edge cases verified**: Basic correctness only; no missing-id case tested at the DAO layer (contrast with REST layer which returns 404).

---

### `dao_update_organisation(organisation_id, **kwargs)`

- **Expected behavior**: Updates any subset of organisation fields passed as keyword arguments. Sets `updated_at` on every successful update.
- **Edge cases verified**:
  - All updatable fields: `name`, `crown`, `organisation_type`, `agreement_signed`, `agreement_signed_at`, `agreement_signed_by_id`, `agreement_signed_version`, `letter_branding_id`, `email_branding_id`.
  - `domains=["ABC", "DEF"]` → stored as `{"abc", "def"}` (lowercased).
  - `domains=[]` → clears all existing domains.
  - `domains=None` → leaves existing domains unchanged (no-op).
  - Duplicate domain values (same value with different cases in the same list) raise `IntegrityError` (marked xfail).
  - When `organisation_type` is included in the update, the type is **propagated to all linked services**, and a service history version is recorded.
  - When `organisation_type` is **not** included, linked services retain their own `organisation_type` unchanged.

---

### `dao_add_service_to_organisation(service, organisation_id)`

- **Expected behavior**: Associates the service with the organisation. Copies the organisation's `organisation_type` and `crown` values onto the service. Sets `service.organisation_id`. Records a new service history entry (version 2).
- **Edge cases verified**: Service previously had a different `organisation_type` (`"central"`) than the org (`"local"`) — org's values overwrite the service's values.

---

### `dao_get_organisation_services(organisation_id)`

- **Expected behavior**: Returns all services linked to the specified organisation, sorted alphabetically by name.
- **Edge cases verified**: Services of multiple organisations are isolated (services from a different org are not returned). Returns empty list for orgs with no services.

---

### `dao_get_organisation_by_service_id(service_id)`

- **Expected behavior**: Returns the organisation that owns the given service.
- **Edge cases verified**: Two services in two different organisations — each lookup returns the correct owner.

---

### `dao_get_invited_organisation_user(invite_id)`

- **Expected behavior**: Returns the `InvitedOrganisationUser` matching the given id.
- **Edge cases verified**: Raises `SQLAlchemyError` (not returning `None`) when the id does not exist.

---

### `dao_get_users_for_organisation(organisation_id)`

- **Expected behavior**: Returns a list of users associated with the organisation in insertion order.
- **Edge cases verified**:
  - Returns empty list when no users are associated.
  - **Filters out inactive users**: only users with `state != "inactive"` are returned.

---

### `dao_add_user_to_organisation(organisation_id, user_id)`

- **Expected behavior**: Creates the association between user and organisation. Returns the updated user object with `organisations` list populated.
- **Edge cases verified**:
  - Raises `SQLAlchemyError` if `user_id` does not exist.
  - Raises `SQLAlchemyError` if `organisation_id` does not exist.

---

### `dao_get_organisation_by_email_address(email_address)`

- **Expected behavior**: Extracts the domain from the email address and looks up which organisation has registered that domain. Returns the matching `Organisation` or `None` if no match.
- **Edge cases verified**:
  - Returns `None` for an unregistered domain.
  - `.gsi.gov.uk` addresses are normalised: `user@example.gsi.gov.uk` is treated as domain `example.gov.uk` and correctly resolves to the org that owns `example.gov.uk`.

---

## Business Rules Verified

### Organisation CRUD behavior
- Organisation names are globally unique (case-insensitive at REST layer; checked via `is_organisation_name_unique` endpoint and enforced on create/update).
- `crown` must be a boolean; `null` is explicitly rejected at creation time.
- Valid `organisation_type` values are an enumerated set: `central`, `province_or_territory`, `local`, `nhs_central`, `nhs_local`, `nhs_gp`, `emergency_service`, `school_or_college`, `other`.
- `updated_at` is set on every DAO-level update.

### Service association rules
- A service belongs to at most one organisation at a time. Re-linking a service to a new organisation implicitly removes it from the previous one.
- When a service is added to an organisation, the organisation's `organisation_type` and `crown` are copied to the service, overwriting whatever the service previously had.
- When an organisation's `organisation_type` is updated, the change is propagated to all currently associated services (with service history recorded).
- Linking a service to a `province_or_territory` organisation triggers a data retention policy: both `email` and `sms` retention are set to **3 days**.

### User membership rules
- Users can be members of organisations; the relationship is many-to-many.
- `dao_get_users_for_organisation` returns only **active** users; inactive users are silently excluded.
- Adding a non-existent user or adding to a non-existent organisation raises a `SQLAlchemyError`.

### Domain matching behavior
- Domains are stored in lowercase; uppercase input is normalised on write.
- A domain can belong to only one organisation; attempting to assign a domain already owned by another org returns `400 "Domain already exists"` at the REST layer, and an `IntegrityError` at the DB layer.
- Updating non-domain fields on an organisation does **not** affect that organisation's registered domains.
- Providing `domains: []` clears all domains; providing `domains: null` / omitting the field leaves domains untouched.
- Email address lookup (`dao_get_organisation_by_email_address`) strips the local part, uses the domain for matching, and normalises `.gsi.gov.uk` → `.gov.uk`.

### Invitation flow for org users
- Invitations are created with `status = pending`.
- An invite email is sent synchronously via Celery (`deliver_email.apply_async`) to the `notify-internal-tasks` queue.
- The invite URL is `{invite_link_host}/organisation-invitation/{token}`; `invite_link_host` defaults to the system's base URL but can be overridden per request.
- The notification's reply-to is set to the inviting user's email address.
- Invite status lifecycle: `pending` → `accepted` | `cancelled`. Setting an invalid status returns `400`.
- Attempting to update an invite via an organisation that does not own it returns `404`.

### Branding association
- An organisation can have an `email_branding_id` and a `letter_branding_id`; both default to `null`.
- Both can be set independently in a single update call.
- `default_branding_is_french` defaults to `false` and can be toggled to `true` independently of other branding fields.
