# API Surface

## Overview

- **Total endpoint count**: ~210 (including aliases and stubs)
- **Admin endpoints** (internal, consumed by GC Notify admin UI): ~195
- **Public v2 endpoints** (`/v2/` prefix, consumed by external service teams): ~15

### Authentication mechanisms

Four JWT authentication mechanisms and one API-key mechanism are in use:

| Header scheme | Used by | Auth function |
|---|---|---|
| `Authorization: Bearer <jwt>` where issuer = `ADMIN_CLIENT_USER_NAME` | Admin UI & internal tooling | `requires_admin_auth` |
| `Authorization: Bearer <jwt>` where issuer = `SRE_USER_NAME` | SRE tooling | `requires_sre_auth` |
| `Authorization: Bearer <jwt>` where issuer = `CACHE_CLEAR_USER_NAME` | Deployment cache-clear | `requires_cache_clear_auth` |
| `Authorization: Bearer <jwt>` where issuer = `CYPRESS_AUTH_USER_NAME` | Cypress CI tests | `requires_cypress_auth` |
| `Authorization: Bearer <jwt>` (service key as issuer) | Service API callers (v1 + v2) | `requires_auth` (JWT path) |
| `Authorization: ApiKey-v1 <plaintext_secret>` | Service API callers (v1 + v2) | `requires_auth` (API key path) |
| *(none)* | Public health/spec endpoints | `requires_no_auth` |

---

## Authentication & Authorization

### `requires_admin_auth`
- Applied to all admin blueprints (services, users, templates, billing, invites, etc.).
- Checks `Authorization: Bearer <jwt>`.
- Decodes JWT; requires the issuer claim to equal `ADMIN_CLIENT_USER_NAME` (env var).
- Validates the JWT against `ADMIN_CLIENT_SECRET`.
- On failure: **401** `{"token": ["...message..."]}`.

### `requires_sre_auth`
- Same JWT flow as admin but requires issuer == `SRE_USER_NAME`, secret == `SRE_CLIENT_SECRET`.
- Applied only to the `/sre-tools` blueprint.

### `requires_cache_clear_auth`
- Same JWT flow; issuer == `CACHE_CLEAR_USER_NAME`, secret == `CACHE_CLEAR_CLIENT_SECRET`.
- Applied only to the `/cache-clear` blueprint.

### `requires_cypress_auth`
- Same JWT flow; issuer == `CYPRESS_AUTH_USER_NAME`, secret == `CYPRESS_AUTH_CLIENT_SECRET`.
- Applied only to the `/cypress` blueprint.
- **Note**: cypress routes additionally check `NOTIFY_ENVIRONMENT != "production"` at request time and return **403** if running in production.

### `requires_auth` (service/public API auth)
Supports two sub-schemes, selected via the `Authorization` header prefix:

**JWT path** (`Authorization: Bearer <jwt>`):
1. Decodes the JWT to extract the issuer (expected to be a `service_id` UUID).
2. Fetches the service and its API keys from the database.
3. Tries each API key's secret to find one that verifies the JWT signature.
4. Populates `g.authenticated_service` and `g.api_user` (the matched `ApiKey` record).
5. Errors: **401** no token; **403** service not found / no API keys / service archived / expired / invalid token.

**API key path** (`Authorization: ApiKey-v1 <secret>`):
1. Looks up the API key record by comparing the hashed secret.
2. Populates `g.authenticated_service` and `g.api_user`.

### `requires_no_auth`
No-op. Used for the health-check and OpenAPI spec endpoints.

---

## Endpoints by Domain

---

### Status / Healthcheck (blueprint: `status`, prefix: *(none)*, auth: `requires_no_auth`)

Surface: **public/internal** — no authentication required.

#### GET /
#### GET /_status
#### POST /_status
- **Auth**: none
- **Query params**: `simple` (optional string) — if present returns minimal `{"status":"ok"}`
- **Response 200**: `{"status":"ok","current_time_utc":"...","commit_sha":"...","build_time":"...","db_version":"..."}`

#### GET /_status/live-service-and-organisation-counts
- **Auth**: none
- **Response 200**: `{"organisations": <int>, "services": <int>}`

---

### Accept Invite / Validate Invitation Token (blueprint: `accept_invite`, prefix: `/invite`, auth: `requires_admin_auth`)

Surface: **admin**

#### GET /invite/{invitation_type}/{token}
- **Auth**: admin JWT
- **Path params**: `invitation_type` (string: `service` | `organisation`); `token` (URL-safe signed token)
- **Response 200**: serialized `InvitedUser` (schema: `invited_user_schema`) for service invites, or `InvitedOrganisationUser.serialize()` for org invites
- **Response errors**:
  - **400** `{"invitation":"invitation expired"}` — token past `INVITATION_EXPIRATION_DAYS` days
  - **400** `{"invitation":"bad invitation link"}` — malformed token
  - **400** `"Unrecognised invitation type: ..."` — unrecognised type value

---

### API Key Stats (blueprint: `api_key`, prefix: `/api-key`, auth: `requires_admin_auth`)

Surface: **admin**

#### GET /api-key/{api_key_id}/summary-statistics
- **Auth**: admin JWT
- **Path params**: `api_key_id` (UUID)
- **Response 200**: `{"api_key_id","email_sends","sms_sends","total_sends","last_send"}` — last_send is ISO datetime string or null

#### GET /api-key/ranked-by-notifications-created/{n_days_back}
- **Auth**: admin JWT
- **Path params**: `n_days_back` (integer, 1–7 inclusive)
- **Response 200**: `{"data":[{"api_key_name","api_key_type","service_name","api_key_id","service_id","last_notification_created","email_notifications","sms_notifications","total_notifications"}]}`
- **Notes**: Returns empty list if `n_days_back` is not an integer or is outside 1–7.

---

### SRE Tools (blueprint: `sre_tools`, prefix: `/sre-tools`, auth: `requires_sre_auth`)

Surface: **admin/internal**

#### POST /sre-tools/api-key-revoke
- **Auth**: SRE JWT
- **Request body**: `{"token":"gcntfy-...", "type":"...", "url":"https://...", "source":"content"}`
  - `token` (string, required): the raw API key secret
  - `type` (string, required): token type label
  - `url` (string, required): public URL where the key was found
  - `source` (string, required): detection source
- **Response 200**: `{"result":"ok"}` — key not found (no-op, success)
- **Response 201**: `{"result":"ok"}` — key found, revoked, and compromise info recorded; revocation email sent to service owners
- **Response 400**: `{"result":"error","message":"Invalid payload"}` — malformed or missing required fields
- **Notes**: Side effect — sends a revocation notification email to service users via Celery task.

---

### Billing (blueprint: `billing`, prefix: `/service/{service_id}/billing`, auth: `requires_admin_auth`)

Surface: **admin**

#### GET /service/{service_id}/billing/ft-monthly-usage
#### GET /service/{service_id}/billing/monthly-usage *(alias)*
- **Auth**: admin JWT
- **Path params**: `service_id` (UUID)
- **Query params**: `year` (integer, required)
- **Response 200**: serialized monthly billing rows (emails excluded)
- **Response 400**: `{"result":"error","message":"No valid year provided"}`

#### GET /service/{service_id}/billing/ft-yearly-usage-summary
#### GET /service/{service_id}/billing/yearly-usage-summary *(alias)*
- **Auth**: admin JWT
- **Path params**: `service_id` (UUID)
- **Query params**: `year` (integer, required)
- **Response 200**: serialized yearly billing totals
- **Response 400**: `{"result":"error","message":"No valid year provided"}`

#### GET /service/{service_id}/billing/free-sms-fragment-limit
- **Auth**: admin JWT
- **Query params**: `financial_year_start` (integer, optional) — defaults to current financial year
- **Response 200**: `{"financial_year_start","free_sms_fragment_limit","service_id",...}`
- **Response 404**: no billing record found for service

#### POST /service/{service_id}/billing/free-sms-fragment-limit
- **Auth**: admin JWT
- **Request body** (schema: `create_or_update_free_sms_fragment_limit_schema`): `{"free_sms_fragment_limit": int, "financial_year_start": int (optional)}`
- **Response 201**: the submitted form data
- **Notes**: Also updates all future financial years when `financial_year_start` is current or future.

---

### Cache Clear (blueprint: `cache`, prefix: `/cache-clear`, auth: `requires_cache_clear_auth`)

Surface: **internal**

#### POST /cache-clear
- **Auth**: cache-clear JWT
- **Request body**: none
- **Response 201**: `{"result":"ok"}`
- **Response 500**: `{"error":"Unable to clear the cache"}`
- **Notes**: Deletes all Redis keys matching patterns defined in `CACHE_KEYS_ALL`.

---

### Complaints (blueprint: `complaint`, prefix: `/complaint`, auth: `requires_admin_auth`)

Surface: **admin**

#### GET /complaint
- **Auth**: admin JWT
- **Query params**: `page` (integer, default 1)
- **Response 200**: `{"complaints":[...serialized Complaint objects...],"links":{pagination links}}`

#### GET /complaint/count-by-date-range
- **Auth**: admin JWT
- **Query params**: `start_date` (date YYYY-MM-DD, optional, default today), `end_date` (date YYYY-MM-DD, optional, default today)
- **Request validation**: schema `complaint_count_request`
- **Response 200**: integer — count of complaints in range

---

### Cypress Test Utilities (blueprint: `cypress`, prefix: `/cypress`, auth: `requires_cypress_auth`)

Surface: **internal** — **non-production only**. All routes return **403** if `NOTIFY_ENVIRONMENT == "production"`.

#### POST /cypress/create_user/{email_name}
- **Auth**: cypress JWT
- **Path params**: `email_name` (string, lowercase alphanumeric only — validated by regex `^[a-z0-9]+$`)
- **Response 201**: `{"regular": {serialized user}, "admin": {serialized user}}`
- **Response 400**: invalid email_name
- **Response 403**: running in production
- **Notes**: Creates two users (`notify-ui-tests+ag_{email_name}@cds-snc.ca` and `…_admin@cds-snc.ca`), both added to the configured CYPRESS_SERVICE_ID with full permissions. Password is SHA256 of `CYPRESS_USER_PW_SECRET + DANGEROUS_SALT`.

#### GET /cypress/cleanup
- **Auth**: cypress JWT
- **Response 201**: `{"message":"Clean up complete"}`
- **Response 403**: running in production
- **Response 500**: cleanup error
- **Notes**: Deletes all users matching `%notify-ui-tests+ag_%@cds-snc.ca%` created more than 3 hours ago, including all their associated services, templates, permissions, etc.

---

### Email Branding (blueprint: `email_branding`, prefix: `/email-branding`, auth: `requires_admin_auth`)

Surface: **admin**

#### GET /email-branding
- **Auth**: admin JWT
- **Query params**: `organisation_id` (UUID, optional) — filter by organisation
- **Response 200**: `{"email_branding":[...serialized EmailBranding objects...]}`

#### GET /email-branding/{email_branding_id}
- **Auth**: admin JWT
- **Path params**: `email_branding_id` (UUID)
- **Response 200**: `{"email_branding": {...}}`

#### POST /email-branding
- **Auth**: admin JWT
- **Request body** (schema: `post_create_email_branding_schema`): `{"name": str, "colour": str|null, "logo": str|null, "text": str|null, "brand_type": str}`
- **Response 201**: `{"data": {...serialized EmailBranding...}}`
- **Response 400**: `{...validation errors...}` or duplicate name error

#### POST /email-branding/{email_branding_id}
- **Auth**: admin JWT
- **Request body** (schema: `post_update_email_branding_schema`)
- **Response 200**: `{"data": {...}}`
- **Response 400**: duplicate name error
- **Notes**: If `text` is absent but `name` is present, `text` is set to match `name`.

---

### Events (blueprint: `events`, prefix: `/events`, auth: `requires_admin_auth`)

Surface: **admin**

#### POST /events
- **Auth**: admin JWT
- **Request body** (schema: `event_schema`): `{"event_type": str, "data": obj}`
- **Response 201**: `{"data": {serialized Event}}`

---

### Inbound Numbers (blueprint: `inbound_number`, prefix: `/inbound-number`, auth: `requires_admin_auth`)

Surface: **admin**

#### GET /inbound-number
- **Auth**: admin JWT
- **Response 200**: `{"data": [...serialized InboundNumber...]}`

#### POST /inbound-number/add
- **Auth**: admin JWT
- **Request body**: `{"inbound_number": str}`
- **Response 201**: the inbound_number string that was added

#### GET /inbound-number/service/{service_id}
- **Auth**: admin JWT
- **Path params**: `service_id` (UUID)
- **Response 200**: `{"data": {...}} ` or `{"data":{}}` if none found

#### POST /inbound-number/service/{service_id}/off
- **Auth**: admin JWT
- **Path params**: `service_id` (UUID)
- **Response 204**: empty

#### GET /inbound-number/available
- **Auth**: admin JWT
- **Response 200**: `{"data": [...]}` — list of unallocated inbound numbers

---

### Inbound SMS — Admin (blueprint: `inbound_sms`, prefix: `/service/{service_id}/inbound-sms`, auth: `requires_admin_auth`)

Surface: **admin**

#### POST /service/{service_id}/inbound-sms
- **Auth**: admin JWT
- **Request body** (schema: `get_inbound_sms_for_service_schema`): `{"phone_number": str|null}`
- **Response 200**: `{"data": [...serialized InboundSms rows...]}`
- **Notes**: Normalises phone number to E.164 international format. Respects service data retention `limit_days` (default 7).

#### GET /service/{service_id}/inbound-sms/most-recent
- **Auth**: admin JWT
- **Query params**: `page` (integer, default 1)
- **Response 200**: `{"data": [...], "has_next": bool}` — most-recent message per sender, paginated

#### GET /service/{service_id}/inbound-sms/summary
- **Auth**: admin JWT
- **Response 200**: `{"count": int, "most_recent": ISO datetime|null}` — based on last 7 days

#### GET /service/{service_id}/inbound-sms/{inbound_sms_id}
- **Auth**: admin JWT
- **Path params**: `inbound_sms_id` (UUID)
- **Response 200**: serialized InboundSms object

---

### Service Invitations (blueprint: `invite`, prefix: `/service/{service_id}/invite`, auth: `requires_admin_auth`)

Surface: **admin**

#### POST /service/{service_id}/invite
- **Auth**: admin JWT
- **Request body** (schema: `invited_user_schema`): `{"email_address": str, "service_id": UUID, "from_user": UUID, "permissions": str, ...}`
- **Response 201**: `{"data": {serialized InvitedUser}}`
- **Notes**: Sends an invitation email via the internal `INVITATION_EMAIL_TEMPLATE_ID` template through the Celery NOTIFY queue.

#### GET /service/{service_id}/invite
- **Auth**: admin JWT
- **Response 200**: `{"data": [...serialized InvitedUser...]}`

#### POST /service/{service_id}/invite/{invited_user_id}
- **Auth**: admin JWT
- **Request body** (schema: `invited_user_schema` fields to update, e.g. `{"status": "accepted"}`): partial update merged onto existing record
- **Response 200**: `{"data": {serialized InvitedUser}}`

---

### Jobs (blueprint: `job`, prefix: `/service/{service_id}/job`, auth: `requires_admin_auth`)

Surface: **admin**

#### GET /service/{service_id}/job/{job_id}
- **Auth**: admin JWT
- **Response 200**: `{"data": {job + statistics array}}`
- **Response 404**: job not found

#### POST /service/{service_id}/job/{job_id}/cancel
- **Auth**: admin JWT
- **Response 200**: same shape as GET job — re-fetches and returns updated job
- **Notes**: Sets job to CANCELLED status; decrements today's email count by `job.notification_count`.

#### POST /service/{service_id}/job/{job_id}/cancel-letter-job
- **Auth**: admin JWT
- **Response 200**: cancellation data
- **Response 400**: `{"message": "..."}` — job cannot be cancelled
- **Response 404**: job not found

#### GET /service/{service_id}/job/{job_id}/notifications
- **Auth**: admin JWT
- **Query params** (schema: `notifications_filter_schema`): `page`, `page_size`, `status`, `template_type`, `include_jobs` (bool), `format_for_csv` (bool)
- **Response 200**: `{"notifications":[...],"page_size":int,"total":int,"links":{...}}`

#### GET /service/{service_id}/job
- **Auth**: admin JWT
- **Query params**: `limit_days` (int, optional), `statuses` (comma-separated string), `page` (int, default 1)
- **Response 200**: `{"data":[...],"page_size":int,"total":int,"links":{...}}` — jobs with per-job statistics batched from DB

#### POST /service/{service_id}/job
- **Auth**: admin JWT
- **Request body** (schema: `job_schema`): `{"id": UUID, "template_id": UUID, "notification_count": int, "scheduled_for": ISO datetime|null, "sender_id": UUID|null, ...}`
- **Response 201**: `{"data": {...job...,"statistics":[]}}`
- **Response 400**: validation errors (inactive service, invalid file, template archived, mixed test/real recipients, limit exceeded)
- **Notes**: Reads job CSV from S3; validates SMS/email daily and annual limits; enqueues `process_job` Celery task if `job_status == PENDING`. Feature flag `FF_USE_BILLABLE_UNITS` determines whether limits are checked in SMS fragments or message counts.

#### GET /service/{service_id}/job/has_jobs
- **Auth**: admin JWT
- **Response 200**: `{"data": {"has_jobs": bool}}`

---

### Letter Branding (blueprint: `letter_branding`, prefix: `/letter-branding`, auth: `requires_admin_auth`)

Surface: **admin**

#### GET /letter-branding
- **Auth**: admin JWT
- **Response 200**: `[...serialized LetterBranding...]`

#### GET /letter-branding/{letter_branding_id}
- **Auth**: admin JWT
- **Response 200**: serialized LetterBranding

#### POST /letter-branding
- **Auth**: admin JWT
- **Request body** (schema: `post_letter_branding_schema`): `{"name": str, "filename": str}`
- **Response 201**: serialized LetterBranding
- **Response 400**: duplicate name or filename

#### POST /letter-branding/{letter_branding_id}
- **Auth**: admin JWT
- **Request body** (schema: `post_letter_branding_schema`)
- **Response 201**: updated serialized LetterBranding
- **Response 400**: duplicate name or filename

---

### Letters — Returned (blueprint: `letter_job`, prefix: *(none)*, auth: `requires_admin_auth`)

Surface: **admin**

#### POST /letters/returned
- **Auth**: admin JWT
- **Notes**: **Unimplemented stub** — function body is `pass`. Returns an implicit `None`/empty response.

---

### Newsletter (blueprint: `newsletter`, prefix: `/newsletter`, auth: `requires_admin_auth`)

Surface: **admin** (Airtable-backed newsletter subscription management)

#### POST /newsletter/unconfirmed-subscriber
- **Auth**: admin JWT
- **Request body**: `{"email": str (required), "language": str (default "en")}`
- **Response 201**: `{"result":"success","subscriber":{...}}` — new subscriber created, confirmation email sent
- **Response 200**: `{"result":"success","message":"A subscriber with this email already exists","subscriber":{...}}` — re-sends confirmation email
- **Response 400**: email missing; Airtable error
- **Notes**: Sends confirmation via `send_confirmation_email()` through Celery NOTIFY queue.

#### GET /newsletter/confirm/{subscriber_id}
- **Auth**: admin JWT
- **Path params**: `subscriber_id` (Airtable record ID string)
- **Response 200**: `{"result":"success","message":"Subscription confirmed","subscriber":{...}}` or already-confirmed message
- **Response 404**: subscriber not found
- **Response 500**: Airtable save failure

#### GET /newsletter/unsubscribe/{subscriber_id}
- **Auth**: admin JWT
- **Path params**: `subscriber_id` (Airtable record ID string)
- **Response 200**: `{"result":"success","subscriber":{...}}`
- **Response 404**: subscriber not found

#### POST /newsletter/update-language/{subscriber_id}
- **Auth**: admin JWT
- **Path params**: `subscriber_id` (Airtable record ID string)
- **Request body**: `{"language": str (required)}`
- **Response 200**: `{"result":"success","message":"Language updated successfully","subscriber":{...}}`
- **Response 400**: language missing or Airtable save failure
- **Response 404**: subscriber not found

#### GET /newsletter/send-latest/{subscriber_id}
- **Auth**: admin JWT
- **Path params**: `subscriber_id` (Airtable record ID string)
- **Response 200**: `{"result":"success","subscriber":{...}}` — triggers sending latest newsletter template to subscriber
- **Response 400**: subscriber status is not `SUBSCRIBED`
- **Response 404**: subscriber not found or no current newsletter templates in Airtable

#### GET /newsletter/find-subscriber
- **Auth**: admin JWT
- **Query params**: `subscriber_id` (Airtable record ID, optional) OR `email` (string, optional) — at least one required
- **Response 200**: `{"result":"success","subscriber":{...}}`
- **Response 400**: neither `subscriber_id` nor `email` provided
- **Response 404**: subscriber not found

---

### Notifications — v1 (blueprint: `notifications`, prefix: *(none)*, auth: `requires_auth`)

Surface: **admin/legacy-public** — authenticated via service API key or JWT. Used by older clients.

#### GET /notifications/{notification_id}
- **Auth**: service JWT or ApiKey-v1
- **Path params**: `notification_id` (UUID)
- **Response 200**: `{"data":{"notification":{...notification_with_personalisation_schema...}}}`
- **Response 404**: `{"result":"error","message":"Notification not found in database"}`

#### GET /notifications
- **Auth**: service JWT or ApiKey-v1
- **Query params** (schema: `notifications_filter_schema`): `status` (list), `template_type` (list), `page` (int), `page_size` (int), `limit_days` (int), `include_jobs` (bool)
- **Response 200**: `{"notifications":[...],"page_size":int,"total":int,"links":{...}}`

#### POST /notifications/{notification_type}
- **Auth**: service JWT or ApiKey-v1
- **Path params**: `notification_type` (string: `sms` | `email`)
- **Request body**:
  - SMS (schema: `sms_template_notification_schema`): `{"to": str, "template": UUID, "personalisation": obj}`
  - Email (schema: `email_notification_schema`): `{"to": str, "template": UUID, "personalisation": obj}`
- **Response 201**: `{"data":{"body":str,"template_version":int,"notification":{"id":UUID},"subject":str (email only)}}`
- **Response 400**: invalid type, validation errors, content >char limit, international SMS blocked, service lacking permission
- **Response 429**: rate limit exceeded (daily/annual SMS or email limits)
- **Notes**: Rate-limited per `check_rate_limiting`. Validates template active and matching type. Simulated recipients (test numbers/addresses) are persisted but not enqueued.

---

### Organisation Invites (blueprint: `organisation_invite`, prefix: `/organisation/{organisation_id}/invite`, auth: `requires_admin_auth`)

Surface: **admin**

#### POST /organisation/{organisation_id}/invite
- **Auth**: admin JWT
- **Request body** (schema: `post_create_invited_org_user_status_schema`): `{"email_address": str, "invited_by": UUID}`
- **Response 201**: serialized InvitedOrganisationUser
- **Notes**: Sends invitation email via `ORGANISATION_INVITATION_EMAIL_TEMPLATE_ID`.

#### GET /organisation/{organisation_id}/invite
- **Auth**: admin JWT
- **Response 200**: `{"data":[...serialized InvitedOrganisationUser...]}`

#### POST /organisation/{organisation_id}/invite/{invited_org_user_id}
- **Auth**: admin JWT
- **Request body** (schema: `post_update_invited_org_user_status_schema`): `{"status": str}`
- **Response 200**: serialized InvitedOrganisationUser

---

### Organisations (blueprint: `organisation`, prefix: `/organisations`, auth: `requires_admin_auth`)

Surface: **admin**

#### GET /organisations
- **Auth**: admin JWT
- **Response 200**: `[...serialized_for_list Organisation...]`

#### GET /organisations/{organisation_id}
- **Auth**: admin JWT
- **Response 200**: full serialized Organisation

#### GET /organisations/by-domain
- **Auth**: admin JWT
- **Query params**: `domain` (string, required, must not contain `@`)
- **Response 200**: full serialized Organisation
- **Response 400**: missing or invalid domain
- **Response 404**: no matching organisation

#### POST /organisations
- **Auth**: admin JWT
- **Request body** (schema: `post_create_organisation_schema`): `{"name": str, "active": bool, "organisation_type": str, "crown": bool|null, ...}`
- **Response 201**: serialized Organisation
- **Response 400**: duplicate name (`ix_organisation_name` unique constraint)

#### POST /organisations/{organisation_id}
- **Auth**: admin JWT
- **Request body** (schema: `post_update_organisation_schema`)
- **Response 204**: empty
- **Response 404**: organisation not found
- **Notes**: If `agreement_signed: true` and `agreement_signed_by_id` present, sends MOU-signed notification emails to signatories.

#### POST /organisations/{organisation_id}/service
- **Auth**: admin JWT
- **Request body** (schema: `post_link_service_to_organisation_schema`): `{"service_id": UUID}`
- **Response 204**: empty
- **Notes**: If organisation type is `province_or_territory`, sets service data retention to `PT_DATA_RETENTION_DAYS` (3 days).

#### GET /organisations/{organisation_id}/services
- **Auth**: admin JWT
- **Response 200**: `[...serialized_for_org_dashboard Service...]` sorted by (-active, name)

#### POST /organisations/{organisation_id}/users/{user_id}
- **Auth**: admin JWT
- **Response 200**: `{"data": serialized User with org membership}`

#### GET /organisations/{organisation_id}/users
- **Auth**: admin JWT
- **Response 200**: `{"data": [...serialized User...]}`

#### GET /organisations/unique
- **Auth**: admin JWT
- **Query params**: `org_id` (UUID, required), `name` (string, required)
- **Response 200**: `{"result": bool}` — true if name is available (not taken by another org)

---

### Platform Stats (blueprint: `platform_stats`, prefix: `/platform-stats`, auth: `requires_admin_auth`)

Surface: **admin**

#### GET /platform-stats
- **Auth**: admin JWT
- **Query params** (schema: `platform_stats_request`): `start_date` (date, optional), `end_date` (date, optional) — default today
- **Response 200**: notification status totals formatted for admin stats view

#### GET /platform-stats/usage-for-trial-services
- **Auth**: admin JWT
- **Response 200**: `[...notification stats for services in trial/restricted mode...]`

#### GET /platform-stats/usage-for-all-services
- **Auth**: admin JWT
- **Query params**: `start_date` (date YYYY-MM-DD, required), `end_date` (date YYYY-MM-DD, required) — must be within same financial year
- **Response 200**: combined SMS/letter cost and usage data per service
- **Response 400**: invalid date format, end < start, or dates span two financial years

#### GET /platform-stats/send-method-stats-by-service
- **Auth**: admin JWT
- **Query params**: `start_date` (date YYYY-MM-DD, required), `end_date` (date YYYY-MM-DD, required)
- **Response 200**: `[{"service_id", "service_name", "api_count", "admin_count", "total_count"}]` — breakdown of notification-send method (API vs admin UI) per service for the date range
- **Notes**: Queries `send_method_stats_by_service` DAO function on `notifications`. No financial-year boundary constraint (unlike `usage-for-all-services`).

---

### Provider Details (blueprint: `provider_details`, prefix: `/provider-details`, auth: `requires_admin_auth`)

Surface: **admin**

#### GET /provider-details
- **Auth**: admin JWT
- **Response 200**: `{"provider_details":[{"id","display_name","identifier","priority","notification_type","active","updated_at","supports_international","created_by_name","current_month_billable_sms"}]}`

#### GET /provider-details/{provider_details_id}
- **Auth**: admin JWT
- **Response 200**: `{"provider_details": {schema: provider_details_schema}}`

#### GET /provider-details/{provider_details_id}/versions
- **Auth**: admin JWT
- **Response 200**: `{"data": [...provider_details_history_schema...]}`

#### POST /provider-details/{provider_details_id}
- **Auth**: admin JWT
- **Request body**: subset of `{"priority": int, "created_by": UUID, "active": bool}` — only these keys permitted
- **Response 200**: `{"provider_details": {...updated...}}`
- **Response 400**: keys outside `{"priority","created_by","active"}` are rejected

---

### Reports (blueprint: `report`, prefix: `/service/{service_id}/report`, auth: `requires_admin_auth`)

Surface: **admin**

#### POST /service/{service_id}/report
- **Auth**: admin JWT
- **Request body**: `{"report_type": str, "requesting_user_id": UUID|null, "language": str|null, "notification_statuses": list|null, "job_id": UUID|null}`
  - `report_type` must be one of `ReportType` enum values
- **Response 201**: `{"data": {report_schema dump}}`
- **Response 400**: invalid report_type or validation errors
- **Notes**: Enqueues `generate_report` Celery task on `GENERATE_REPORTS` queue.

#### GET /service/{service_id}/report
- **Auth**: admin JWT
- **Query params**: `limit_days` (int, optional, default 7)
- **Response 200**: `{"data": [...report_schema dump...]}`

---

### Service Callbacks / Webhooks (blueprint: `service_callback`, prefix: `/service/{service_id}`, auth: `requires_admin_auth`)

Surface: **admin**

#### POST /service/{service_id}/inbound-api
- **Auth**: admin JWT
- **Request body** (schema: `create_service_callback_api_schema`): `{"url": str (https), "bearer_token": str, "updated_by_id": UUID}`
- **Response 201**: serialized ServiceInboundApi

#### POST /service/{service_id}/inbound-api/{inbound_api_id}
- **Auth**: admin JWT
- **Request body** (schema: `update_service_callback_api_schema`): `{"updated_by_id": UUID, "url": str|null, "bearer_token": str|null}`
- **Response 200**: serialized ServiceInboundApi

#### GET /service/{service_id}/inbound-api/{inbound_api_id}
- **Auth**: admin JWT
- **Response 200**: serialized ServiceInboundApi

#### DELETE /service/{service_id}/inbound-api/{inbound_api_id}
- **Auth**: admin JWT
- **Response 204**: empty
- **Response 404**: not found

#### POST /service/{service_id}/delivery-receipt-api
- **Auth**: admin JWT
- **Request body** (schema: `create_service_callback_api_schema`): `{"url": str (https), "bearer_token": str, "updated_by_id": UUID}`
- **Response 201**: serialized ServiceCallbackApi

#### POST /service/{service_id}/delivery-receipt-api/{callback_api_id}
- **Auth**: admin JWT
- **Request body** (schema: `update_service_callback_api_schema`)
- **Response 200**: serialized ServiceCallbackApi

#### GET /service/{service_id}/delivery-receipt-api/{callback_api_id}
- **Auth**: admin JWT
- **Response 200**: serialized ServiceCallbackApi

#### DELETE /service/{service_id}/delivery-receipt-api/{callback_api_id}
- **Auth**: admin JWT
- **Response 204**: empty
- **Response 404**: not found

#### POST /service/{service_id}/delivery-receipt-api/suspend-callback
- **Auth**: admin JWT
- **Request body**: `{"updated_by_id": UUID, "suspend_unsuspend": bool}`
- **Response 200**: serialized ServiceCallbackApi
- **Response 404**: no callback API found for service

---

### Services (blueprint: `service`, prefix: `/service`, auth: `requires_admin_auth`)

Surface: **admin**

#### GET /service
- **Auth**: admin JWT
- **Query params**: `only_active` (bool), `detailed` (bool), `user_id` (UUID), `include_from_test_key` (bool, default True), `start_date` (date), `end_date` (date)
- **Response 200**: `{"data": [...service_schema...]}` or detailed stats shape when `detailed=True`

#### GET /service/find-services-by-name
- **Auth**: admin JWT
- **Query params**: `service_name` (string, required)
- **Response 200**: `{"data": [...serialized_for_org_dashboard...]}`
- **Response 400**: missing service_name

#### GET /service/live-services-data
- **Auth**: admin JWT
- **Query params**: `filter_heartbeats` (bool, optional)
- **Response 200**: `{"data": [live service rows]}`

#### GET /service/delivered-notifications-stats-by-month-data
- **Auth**: admin JWT
- **Query params**: `filter_heartbeats` (bool, optional)
- **Response 200**: `{"data": [monthly delivery stats]}`

#### GET /service/unique
- **Auth**: admin JWT
- **Query params**: `service_id` (UUID, required), `name` (string, required), `email_from `(string, required)
- **Response 200**: `{"result": bool}` — true if both name and email_from are available

#### GET /service/name/unique
- **Auth**: admin JWT
- **Query params**: `service_id` (UUID, required), `name` (string, required)
- **Response 200**: `{"result": bool}`

#### GET /service/email-from/unique
- **Auth**: admin JWT
- **Query params**: `service_id` (UUID, required), `email_from` (string, required)
- **Response 200**: `{"result": bool}`

#### GET /service/sensitive-service-ids
- **Auth**: admin JWT
- **Response 200**: `{"data": [list of service_id UUIDs]}`

#### GET /service/monthly-data-by-service
- **Auth**: admin JWT
- **Query params**: `start_date` (date string), `end_date` (date string)
- **Response 200**: monthly notification statuses per service

#### GET /service/{service_id}
- **Auth**: admin JWT
- **Query params**: `detailed` (bool), `today_only` (bool)
- **Response 200**: `{"data": {service_schema or detailed_service_schema}}`

#### GET /service/{service_id}/statistics
- **Auth**: admin JWT
- **Query params**: `today_only` (bool), `limit_days` (int, default 7)
- **Response 200**: `{"data": {formatted statistics}}`

#### POST /service
- **Auth**: admin JWT
- **Request body** (schema: `service_schema`): `{"name": str, "user_id": UUID, "message_limit": int, "sms_daily_limit": int, "restricted": bool, "email_from": str, "created_by": UUID, "organisation_notes": str|null, ...}`
- **Response 201**: `{"data": {service_schema}}`
- **Response 400**: duplicate name or email_from; missing `user_id`
- **Notes**: If `FF_SALESFORCE_CONTACT` is enabled and the service goes live, an engagement is created in Salesforce.

#### POST /service/{service_id}
- **Auth**: admin JWT
- **Request body** (schema: `service_schema` fields): partial update
- **Response 200**: `{"data": {service_schema}}`
- **Response 400**: duplicate name
- **Notes**:
  - Changing `message_limit` clears Redis daily limit cache keys and emails service users.
  - Changing `sms_daily_limit` clears Redis SMS limit cache keys and emails service users.
  - Changing `email_annual_limit` or `sms_annual_limit` emails service users.
  - Service going live (`restricted: false`) sends a "now live" email.
  - Salesforce engagement updated when service goes live or name changes.

#### POST /service/{service_id}/api-key
- **Auth**: admin JWT
- **Request body** (schema: `api_key_schema`): `{"name": str, "key_type": str ("normal"|"team"|"test"), "created_by": UUID}`
- **Response 201**: `{"data": {"key": str, "key_name": str}}` — unsigned raw secret prefixed by API_KEY_PREFIX

#### POST /service/{service_id}/api-key/revoke/{api_key_id}
- **Auth**: admin JWT
- **Response 202**: empty

#### GET /service/{service_id}/api-keys
#### GET /service/{service_id}/api-keys/{key_id}
- **Auth**: admin JWT
- **Response 200**: `{"apiKeys": [...api_key_schema...]}`
- **Response 404**: key not found

#### GET /service/{service_id}/users
- **Auth**: admin JWT
- **Response 200**: `{"data": [...serialized User...]}`

#### POST /service/{service_id}/users/{user_id}
- **Auth**: admin JWT
- **Request body** (schema: `post_set_permissions_schema`): `{"permissions": [{"permission": str}], "folder_permissions": [UUID]}`
- **Response 201**: `{"data": {service_schema}}`
- **Response 409**: user already in service

#### DELETE /service/{service_id}/users/{user_id}
- **Auth**: admin JWT
- **Response 204**: empty
- **Response 404**: user not in service
- **Response 400**: various removal rules (only user, fewer than 2 members, last user with manage_settings)

#### GET /service/{service_id}/history
- **Auth**: admin JWT
- **Response 200**: `{"data": {"service_history":[...],"api_key_history":[...],"template_history":[...],"events":[]}}`

#### GET /service/{service_id}/notifications
- **Auth**: admin JWT
- **Query params** (schema: `notifications_filter_schema`): `to`, `status`, `template_type`, `page`, `page_size`, `limit_days`, `include_jobs`, `include_from_test_key`, `include_one_off`, `format_for_csv`
- **Response 200**: `{"notifications":[...],"page_size":int,"total":int,"links":{...}}`
- **Notes**: If `to` is provided, performs a recipient search across notification type.

#### GET /service/{service_id}/notifications/{notification_id}
- **Auth**: admin JWT
- **Response 200**: `{notification_with_template_schema dump}`
- **Response 404**: `{"result":"error","message":"Notification not found in database"}`

#### POST /service/{service_id}/notifications/{notification_id}/cancel
- **Auth**: admin JWT
- **Response 200**: updated notification
- **Response 400**: not a letter; or notification not found

#### GET /service/{service_id}/notifications/monthly
- **Auth**: admin JWT
- **Query params**: `year` (int, required)
- **Response 200**: `{"data": {monthly status stats dict}}`
- **Response 400**: year not a number

#### GET /service/{service_id}/notifications/templates_usage/monthly
- **Auth**: admin JWT
- **Query params**: `year` (int, required)
- **Response 200**: `{"stats": [{"template_id","name","type","month","year","count","is_precompiled_letter"}]}`

#### POST /service/{service_id}/send-notification
- **Auth**: admin JWT
- **Request body**: one-off notification fields (see `send_one_off_notification`)
- **Response 201**: `{notification response}`

#### POST /service/{service_id}/send-pdf-letter
- **Auth**: admin JWT
- **Notes**: **Unimplemented stub** — `pass`. Returns implicit empty response.

#### GET /service/{service_id}/safelist
- **Auth**: admin JWT
- **Response 200**: `{"email_addresses":[...],"phone_numbers":[...]}`

#### PUT /service/{service_id}/safelist
- **Auth**: admin JWT
- **Request body**: `{"email_addresses":[...],"phone_numbers":[...]}`
- **Response 204**: empty
- **Response 400**: invalid email/phone number in list

#### POST /service/{service_id}/archive
- **Auth**: admin JWT
- **Response 204**: empty
- **Notes**: Deactivates the service, archives all templates, revokes all API keys, sends deactivation email to service users. Irreversible. Updates Salesforce to closed if `FF_SALESFORCE_CONTACT`.

#### POST /service/{service_id}/suspend
#### POST /service/{service_id}/suspend/{user_id}
- **Auth**: admin JWT
- **Response 204**: empty
- **Notes**: Marks service inactive and revokes API keys. Unlike archive, resumable.

#### POST /service/{service_id}/resume
- **Auth**: admin JWT
- **Response 204**: empty
- **Notes**: Re-activates a previously suspended service (API keys must be re-created).

#### GET /service/{service_id}/email-reply-to
- **Auth**: admin JWT
- **Response 200**: `[...serialized ServiceEmailReplyTo...]`

#### GET /service/{service_id}/email-reply-to/{reply_to_id}
- **Auth**: admin JWT
- **Response 200**: serialized ServiceEmailReplyTo

#### POST /service/{service_id}/email-reply-to/verify
- **Auth**: admin JWT
- **Request body** (schema: `email_data_request_schema`): `{"email": str}`
- **Response 201**: `{"data": {"id": UUID}}` — sends a verification email via NOTIFY queue
- **Response 400**: address already in use

#### POST /service/{service_id}/email-reply-to
- **Auth**: admin JWT
- **Request body** (schema: `add_service_email_reply_to_request`): `{"email_address": str, "is_default": bool}`
- **Response 201**: serialized ServiceEmailReplyTo
- **Response 400**: address already in use

#### POST /service/{service_id}/email-reply-to/{reply_to_email_id}
- **Auth**: admin JWT
- **Request body** (schema: `add_service_email_reply_to_request`): `{"email_address": str, "is_default": bool}`
- **Response 200**: serialized ServiceEmailReplyTo

#### POST /service/{service_id}/email-reply-to/{reply_to_email_id}/archive
- **Auth**: admin JWT
- **Response 200**: serialized archived ServiceEmailReplyTo

#### GET /service/{service_id}/letter-contact
#### GET /service/{service_id}/letter-contact/{letter_contact_id}
#### POST /service/{service_id}/letter-contact
#### POST /service/{service_id}/letter-contact/{letter_contact_id}
#### POST /service/{service_id}/letter-contact/{letter_contact_id}/archive
- **Auth**: admin JWT
- **Notes**: All **unimplemented stubs** — `pass`. Return implicit empty responses. Go must implement stub handlers returning **501 Not Implemented** for all five routes.

#### POST /service/{service_id}/sms-sender
- **Auth**: admin JWT
- **Request body** (schema: `add_service_sms_sender_request`): `{"sms_sender": str, "is_default": bool, "inbound_number_id": UUID|null}`
- **Response 201**: serialized ServiceSmsSender
- **Notes**: If `inbound_number_id` provided and only one sender exists, updates the existing sender instead of creating a new one.

#### POST /service/{service_id}/sms-sender/{sms_sender_id}
- **Auth**: admin JWT
- **Request body** (schema: `add_service_sms_sender_request`)
- **Response 200**: serialized ServiceSmsSender
- **Response 400**: cannot change the number on an inbound-number-linked sender

#### POST /service/{service_id}/sms-sender/{sms_sender_id}/archive
- **Auth**: admin JWT
- **Response 200**: `{"data": serialized ServiceSmsSender}`

#### GET /service/{service_id}/sms-sender/{sms_sender_id}
- **Auth**: admin JWT
- **Response 200**: serialized ServiceSmsSender

#### GET /service/{service_id}/sms-sender
- **Auth**: admin JWT
- **Response 200**: `[...serialized ServiceSmsSender...]`

#### GET /service/{service_id}/organisation
- **Auth**: admin JWT
- **Response 200**: serialized Organisation or `{}`

#### GET /service/{service_id}/data-retention
- **Auth**: admin JWT
- **Response 200**: `[...serialized ServiceDataRetention...]`

#### GET /service/{service_id}/data-retention/notification-type/{notification_type}
- **Auth**: admin JWT
- **Path params**: `notification_type` (string: `email` | `sms` | `letter`)
- **Response 200**: serialized ServiceDataRetention or `{}`

#### GET /service/{service_id}/data-retention/{data_retention_id}
- **Auth**: admin JWT
- **Response 200**: serialized ServiceDataRetention or `{}`

#### POST /service/{service_id}/data-retention
- **Auth**: admin JWT
- **Request body** (schema: `add_service_data_retention_request`): `{"notification_type": str, "days_of_retention": int}`
- **Response 201**: `{"result": {serialized ServiceDataRetention}}`
- **Response 400**: duplicate notification type for this service

#### POST /service/{service_id}/data-retention/{data_retention_id}
- **Auth**: admin JWT
- **Request body** (schema: `update_service_data_retention_request`): `{"days_of_retention": int}`
- **Response 204**: empty
- **Response 404**: record not found

#### GET /service/{service_id}/annual-limit-stats
- **Auth**: admin JWT
- **Response 200**: current annual limit notification counts from Redis (or empty `{}`)

---

### Support — Find by ID (blueprint: `support`, prefix: `/support`, auth: `requires_admin_auth`)

Surface: **admin**

#### GET /support/find-ids
- **Auth**: admin JWT
- **Query params**: `ids` (comma-space separated UUID list, required)
- **Response 200**: `[{"id": UUID, "type": "notification|template|service|job|user|not a uuid|no result found", ...fields...}]`
- **Response 400**: no ids provided
- **Notes**: For each UUID, tries to find it against users → services → templates → jobs → notifications (short-circuits on first match).

---

### Templates (blueprint: `template`, prefix: `/service/{service_id}/template`, auth: `requires_admin_auth`)

Surface: **admin**

#### POST /service/{service_id}/template
- **Auth**: admin JWT
- **Request body** (schema: `post_create_template_schema`): `{"name": str, "template_type": str, "content": str, "subject": str (email), "postage": str (letter), "parent_folder_id": UUID|null, "process_type": str, "template_category_id": UUID|null, "reply_to": UUID|null, "text_direction_rtl": bool}`
- **Response 201**: `{"data": {template_schema}}`
- **Response 400**: service lacks permission for template type; content over char limit; name over char limit; invalid reply-to; parent_folder_id not found
- **Notes**: If service's organisation is `province_or_territory`, personalisation is redacted (GDPR). Letter templates default to `SECOND_CLASS` postage.

#### POST /service/{service_id}/template/{template_id}/category/{template_category_id}
- **Auth**: admin JWT
- **Response 200**: `{"data": {template_schema}}`

#### POST /service/{service_id}/template/{template_id}/process-type
- **Auth**: admin JWT
- **Request body**: `{"process_type": str}` (required)
- **Response 200**: `{"data": {template_schema}}`
- **Response 400**: missing process_type

#### POST /service/{service_id}/template/{template_id}
- **Auth**: admin JWT
- **Request body** (schema: `template_schema` fields): partial update
- **Response 200**: `{"data": {template_schema}}`
- **Response 400**: content/name over char limit; service lacks permission
- **Notes**: If `redact_personalisation: true` in body, triggers redaction (requires `created_by` field). If `reply_to` present, updates reply-to only. If no change detected, returns current data with 200.

#### GET /service/{service_id}/template/precompiled
- **Auth**: admin JWT
- **Response 200**: `{template_schema for precompiled letter template}`

#### GET /service/{service_id}/template
- **Auth**: admin JWT
- **Response 200**: `{"data": [...reduced_template_schema...]}`

#### GET /service/{service_id}/template/{template_id}
- **Auth**: admin JWT
- **Response 200**: `{"data": {template_schema}}`

#### GET /service/{service_id}/template/{template_id}/preview
- **Auth**: admin JWT
- **Query params**: personalisation key=value pairs for rendering
- **Response 200**: template_schema + rendered `subject` and `content`
- **Response 400**: missing required personalisation placeholders

#### GET /service/{service_id}/template/{template_id}/version/{version}
- **Auth**: admin JWT
- **Path params**: `version` (int)
- **Response 200**: `{"data": {template_history_schema}}`

#### GET /service/{service_id}/template/{template_id}/versions
- **Auth**: admin JWT
- **Response 200**: `{"data": [...template_history_schema...]}`

#### GET /service/{service_id}/template/preview/{notification_id}/{file_type}
- **Auth**: admin JWT
- **Path params**: `notification_id` (UUID), `file_type` (string: `pdf` | `png`)
- **Query params**: `page` (int, optional), `overlay` (optional)
- **Response 200**: `{"content": base64-encoded string or raw bytes}`
- **Response 400**: invalid file_type; PDF extraction error
- **Notes**: For precompiled letters, fetches PDF from S3 and optionally calls the template-preview service for PNG rendering or overlay.

---

### Template Categories (blueprint: `template_category`, prefix: `/template-category`, auth: `requires_admin_auth`)

Surface: **admin**

#### POST /template-category
- **Auth**: admin JWT
- **Request body** (schema: `template_category_schema`): `{"name_en": str, "name_fr": str, "description_en": str|null, "description_fr": str|null, "hidden": bool, "sms_process_type": str, "email_process_type": str, ...}`
- **Response 201**: `{"template_category": {schema}}`
- **Response 400**: duplicate EN or FR name

#### GET /template-category/{template_category_id}
- **Auth**: admin JWT
- **Response 200**: `{"template_category": {schema}}`

#### GET /template-category/by-template-id/{template_id}
- **Auth**: admin JWT
- **Path params**: `template_id` (UUID)
- **Response 200**: `{"template_category": {schema}}`

#### GET /template-category
- **Auth**: admin JWT
- **Query params**: `template_type` (string: `sms`|`email`, optional), `hidden` (bool: `True`|`False`, optional)
- **Response 200**: `{"template_categories": [...schema...]}`
- **Response 400**: invalid template_type value

#### POST /template-category/{template_category_id}
- **Auth**: admin JWT
- **Request body** (schema: `template_category_schema` fields): partial update merged with existing
- **Response 200**: `{"template_category": {schema}}`
- **Response 400**: duplicate EN or FR name

#### DELETE /template-category/{template_category_id}
- **Auth**: admin JWT
- **Query params**: `cascade` (bool string `"True"`, optional, default False)
- **Response 204**: empty
- **Notes**: Without cascade, fails if category is used by any template. With cascade, dissociates the category from all templates before deleting.

---

### Template Folders (blueprint: `template_folder`, prefix: `/service/{service_id}/template-folder`, auth: `requires_admin_auth`)

Surface: **admin**

#### GET /service/{service_id}/template-folder
- **Auth**: admin JWT
- **Response 200**: `{"template_folders": [...serialized TemplateFolder...]}`

#### POST /service/{service_id}/template-folder
- **Auth**: admin JWT
- **Request body** (schema: `post_create_template_folder_schema`): `{"name": str, "parent_id": UUID|null}`
- **Response 201**: `{"data": {serialized TemplateFolder}}`
- **Response 400**: parent_id not found
- **Notes**: New folder inherits user access from parent (or all active service users if top-level).

#### POST /service/{service_id}/template-folder/{template_folder_id}
- **Auth**: admin JWT
- **Request body** (schema: `post_update_template_folder_schema`): `{"name": str, "users_with_permission": [UUID]}`
- **Response 200**: `{"data": {serialized TemplateFolder}}`

#### DELETE /service/{service_id}/template-folder/{template_folder_id}
- **Auth**: admin JWT
- **Response 204**: empty
- **Response 400**: `{"result":"error","message":"Folder is not empty"}` — folder has subfolders or templates

#### POST /service/{service_id}/template-folder/contents
#### POST /service/{service_id}/template-folder/{target_template_folder_id}/contents
- **Auth**: admin JWT
- **Request body** (schema: `post_move_template_folder_schema`): `{"folders": [UUID], "templates": [UUID]}`
- **Response 204**: empty
- **Response 400**: folder not found; moving folder into itself; moving folder into its own subfolder

---

### Template Statistics (blueprint: `template_statistics`, prefix: `/service/{service_id}/template-statistics`, auth: `requires_admin_auth`)

Surface: **admin**

#### GET /service/{service_id}/template-statistics
- **Auth**: admin JWT
- **Query params**: `whole_days` or `limit_days` (int, 0–7, required)
- **Response 200**: `{"data": [{count, template_id, template_name, template_type, is_precompiled_letter, status, billable_units (if FF_USE_BILLABLE_UNITS)}]}`
- **Response 400**: not integer; outside 0–7

#### GET /service/{service_id}/template-statistics/{template_id}
- **Auth**: admin JWT
- **Response 200**: `{"data": {notification_with_template_schema dump or null}}`

---

### Users (blueprint: `user`, prefix: `/user`, auth: `requires_admin_auth`)

Surface: **admin**

#### POST /user
- **Auth**: admin JWT
- **Request body** (schema: `create_user_schema`): `{"name": str, "email_address": str, "password": str, "mobile_number": str|null, "auth_type": str}`
- **Response 201**: `{"data": {serialized User}}`
- **Response 400**: password found in Have I Been Pwned database

#### POST /user/{user_id}
- **Auth**: admin JWT
- **Request body** (schema: `user_update_schema_load_json`): `{"name", "email_address", "mobile_number", "auth_type", "blocked", "current_session_id", "default_editor_is_rte", "updated_by": UUID|null, ...}`
- **Response 200**: `{"data": {serialized User}}`
- **Notes**: If `updated_by` is set (a team manager edit), sends an email or SMS change-alert notification to the affected user. Salesforce contact updated if `FF_SALESFORCE_CONTACT`.

#### POST /user/{user_id}/archive
- **Auth**: admin JWT
- **Response 204**: empty
- **Notes**: Soft-deletes the user (marks as archived, state = `inactive`).

#### POST /user/{user_id}/activate
- **Auth**: admin JWT
- **Response 200**: `{"data": {serialized User}}`
- **Response 400**: user already active
- **Notes**: Sets state to `active`. If `FF_SALESFORCE_CONTACT`, creates a Salesforce contact.

#### POST /user/{user_id}/reset-failed-login-count
- **Auth**: admin JWT
- **Response 200**: `{"data": {serialized User}}`

#### POST /user/{user_id}/verify/password
- **Auth**: admin JWT
- **Request body**: `{"password": str, "loginData": obj (optional)}`
- **Response 204**: password correct; login event optionally saved
- **Response 400**: incorrect password (increments failed_login_count)

#### POST /user/{user_id}/verify/code
- **Auth**: admin JWT
- **Request body** (schema: `post_verify_code_schema`): `{"code": str, "code_type": "sms"|"email"}`
- **Response 204**: code verified; updates `logged_in_at` and issues new `current_session_id`
- **Response 400**: code already sent; code expired; code already used
- **Response 404**: wrong code; too many failures
- **Notes**: Bypassed in development/staging for Cypress test users.

#### POST /user/{user_id}/verify-2fa
- **Auth**: admin JWT
- **Request body** (schema: `post_verify_code_schema`)
- **Response 204**: code verified
- **Response 400/404**: same validation as `verify/code` but omits `failed_login_count` checks
- **Notes**: Used when switching 2FA method from the user profile, not during login.

#### POST /user/{user_id}/{code_type}-code
- **Auth**: admin JWT
- **Path params**: `code_type` (string: `sms` | `email`)
- **Request body**:
  - SMS (schema: `post_send_user_sms_code_schema`): `{"to": str (optional override number)}`
  - Email (schema: `post_send_user_email_code_schema`): `{}`
- **Response 204**: empty (code sent or silently throttled)
- **Notes**: Throttled: no-op if >MAX_VERIFY_CODE_COUNT active codes or a code was sent in the last 10 seconds. Sends via internal NOTIFY queue.

#### POST /user/{user_id}/change-email-verification
- **Auth**: admin JWT
- **Request body** (schema: `email_data_request_schema`): `{"email": str}`
- **Response 204**: empty — change-email confirmation link sent to new address

#### POST /user/{user_id}/email-verification
- **Auth**: admin JWT
- **Response 204**: empty — new-user email verification link sent to user's registered address

#### POST /user/{user_id}/email-already-registered
- **Auth**: admin JWT
- **Request body** (schema: `email_data_request_schema`): `{"email": str}`
- **Response 204**: empty — "already registered" email sent to that address

#### POST /user/{user_id}/contact-request
- **Auth**: admin JWT
- **Request body**: `ContactRequest` fields (varies by `support_type`; includes `name`, `email_address`, `service_id`, `main_use_case`, etc.)
- **Response 204**: `{"status_code": <Freshdesk response code>}`
- **Response 400**: malformed request body
- **Notes**: Creates a Freshdesk support ticket. For P/T organisation services, sends a secure email instead if `FF_PT_SERVICE_SKIP_FRESHDESK`. Updates Salesforce engagement stage for go-live requests.

#### POST /user/{user_id}/branding-request
- **Auth**: admin JWT
- **Request body**: `{"serviceID": UUID, "service_name": str, "organisation_id": UUID, "organisation_name": str, "filename": str, "branding_logo_name": str, "alt_text_en": str, "alt_text_fr": str}`
- **Response 204**: `{"status_code": int}`
- **Response 400**: missing/invalid fields or user not found

#### POST /user/{user_id}/new-template-category-request
- **Auth**: admin JWT
- **Request body**: `{"service_id": UUID, "template_category_name_en": str, "template_category_name_fr": str, "template_id": UUID}`
- **Response 204**: `{"status_code": int}`
- **Response 400**: missing/invalid fields

#### GET /user
#### GET /user/{user_id}
- **Auth**: admin JWT
- **Response 200**: `{"data": {serialized User}}` or `{"data": [...]}` when no ID given (returns all users)

#### POST /user/{user_id}/service/{service_id}/permission
- **Auth**: admin JWT
- **Request body** (schema: `post_set_permissions_schema`): `{"permissions": [{"permission": str}], "folder_permissions": [UUID]}`
- **Response 204**: empty

#### GET /user/email
- **Auth**: admin JWT
- **Query params**: `email` (string, required)
- **Response 200**: `{"data": {serialized User}}`
- **Response 400**: missing email param

#### POST /user/find-users-by-email
- **Auth**: admin JWT
- **Request body** (schema: `partial_email_data_request_schema`): `{"email": str}` — partial match
- **Response 200**: `{"data": [...serialized_for_users_list User...]}`

#### POST /user/reset-password
- **Auth**: admin JWT
- **Request body** (schema: `email_data_request_schema`): `{"email": str}`
- **Response 200/204**: password reset email sent
- **Response 400**: user is blocked

#### POST /user/forced-password-reset
- **Auth**: admin JWT
- **Request body**: `{"email": str}`
- **Response**: forced password reset email sent

#### POST /user/{user_id}/update-password
- **Auth**: admin JWT
- **Request body** (schema: `user_update_password_schema_load_json`): `{"_password": str}`
- **Response 200**: `{"data": {serialized User}}`
- **Notes**: Validates new password against Have I Been Pwned. Clears `password_expired` flag.

#### GET /user/{user_id}/organisations-and-services
- **Auth**: admin JWT
- **Response 200**: list of organisations and services the user belongs to (via `get_user_and_accounts`)

#### GET /user/{user_id}/fido2_keys
- **Auth**: admin JWT
- **Response 200**: `[...serialized Fido2Key...]`

#### POST /user/{user_id}/fido2_keys
- **Auth**: admin JWT
- **Request body**: `{"name": str}` — deletes a specific key by name
- **Response 200**: success

#### POST /user/{user_id}/fido2_keys/register
- **Auth**: admin JWT
- **Response 200**: FIDO2 registration options (challenge) for the user

#### POST /user/{user_id}/fido2_keys/authenticate
- **Auth**: admin JWT
- **Response 200**: FIDO2 authentication assertion options

#### POST /user/{user_id}/fido2_keys/validate
- **Auth**: admin JWT
- **Request body**: FIDO2 attestation or assertion response
- **Response 200**: validated response

#### DELETE /user/{user_id}/fido2_keys/{key_id}
- **Auth**: admin JWT
- **Path params**: `key_id` (UUID)
- **Response 200**: empty / confirmation

#### GET /user/{user_id}/login_events
- **Auth**: admin JWT
- **Response 200**: `[...serialized LoginEvent...]`

#### POST /user/{user_id}/deactivate
- **Auth**: admin JWT
- **Response 204**: empty
- **Notes**: Deactivates the user; archives/suspends all services the user created (using `dao_archive_service_no_transaction` and `dao_suspend_service_no_transaction`).

---

### V2 — Notifications (blueprint: `v2_notifications`, prefix: `/v2/notifications`, auth: `requires_auth`)

Surface: **public**

#### GET /v2/notifications/{notification_id}
- **Auth**: service JWT or ApiKey-v1
- **Path params**: `notification_id` (UUID)
- **Response 200**: serialized notification (`notification.serialize()`)
- **Response 404**: `{"result":"error","message":"Notification not found in database"}`

#### GET /v2/notifications/{notification_id}/pdf
- **Auth**: service JWT or ApiKey-v1
- **Path params**: `notification_id` (UUID)
- **Response 200**: PDF file stream (`application/pdf`)
- **Response 400**: not a letter; virus scan failed; technical failure
- **Response 503** (PDFNotReadyError): PDF not yet generated (pending virus check or processing)

#### GET /v2/notifications
- **Auth**: service JWT or ApiKey-v1
- **Query params** (schema: `get_notifications_request`): `status` (list), `template_type` (list), `reference` (string), `older_than` (UUID), `include_jobs` (bool)
- **Response 200**: `{"notifications":[{notification.serialize()}...],"links":{"current":url,"next":url}}`

#### POST /v2/notifications/email
- **Auth**: service JWT or ApiKey-v1
- **Request body** (schema: `post_email_request`): `{"email_address": str, "template_id": UUID, "personalisation": obj|null, "reference": str|null, "email_reply_to_id": UUID|null, "scheduled_for": ISO datetime|null}`
- **Response 201**: `{"id": UUID, "reference": str|null, "content": {"body":str,"from_email":str,"subject":str}, "template": {...}, "uri": str, "scheduled_for": ISO datetime|null}`
- **Response 400**: validation errors, service lacks email permission, template not found/archived
- **Response 403**: service suspended
- **Response 429**: daily or annual email limit exceeded
- **Notes**: Checks rate limiting. Does not enqueue simulated recipients. Feature flag `FF_USE_BILLABLE_UNITS` controls SMS counting only; email always counts 1 per send.

#### POST /v2/notifications/sms
- **Auth**: service JWT or ApiKey-v1
- **Request body** (schema: `post_sms_request`): `{"phone_number": str, "template_id": UUID, "personalisation": obj|null, "reference": str|null, "sms_sender_id": UUID|null, "scheduled_for": ISO datetime|null}`
- **Response 201**: `{"id": UUID, "reference": str|null, "content": {"body":str,"from_number":str}, "template": {...}, "uri": str, "scheduled_for": ISO datetime|null}`
- **Response 400**: validation errors, international SMS blocked, service lacks SMS permission
- **Response 429**: daily or annual SMS limit exceeded

#### POST /v2/notifications/letter
- **Auth**: service JWT or ApiKey-v1
- **Request body**:
  - Precompiled (schema: `post_precompiled_letter_request`, when `content` key present): `{"reference": str, "content": base64 PDF string}`
  - Regular (schema: `post_letter_request`): `{"template_id": UUID, "personalisation": {"address_line_1":str, "address_line_2":str, "postcode":str, ...}, "reference": str|null}`
- **Response 201** (precompiled): `{"id": UUID, "reference": str, "postage": str}`
- **Response 201** (regular): `{"id": UUID, "reference": str|null, "content": {"body":str,"subject":str}, "template": {...}, "uri": str}`

#### POST /v2/notifications/bulk
- **Auth**: service JWT or ApiKey-v1
- **Request body** (schema: `post_bulk_request(max_rows)`): `{"name": str, "template_id": UUID, "rows": [[header, ...], [value, ...]] | null, "csv": str|null, "scheduled_for": ISO datetime|null, "reply_to_id": UUID|null}`
  - Exactly one of `rows` or `csv` must be provided
  - `rows` is a 2D array: first row is column headers, subsequent rows are recipient data
- **Response 201**: `{"data": {job_schema dump}}`
- **Response 400**: both or neither of rows/csv; CSV errors; mixed test/real recipients for SMS; limit exceeded
- **Response 403**: service suspended
- **Response 415**: non-JSON Content-Type
- **Notes**: Creates a Job record in the database and stores the CSV in S3. Validates recipient count against daily/annual limits. Enqueues `process_job` Celery task.

---

### V2 — Template (blueprint: `v2_template`, prefix: `/v2/template`, auth: `requires_auth`)

Surface: **public**

#### GET /v2/template/{template_id}
#### GET /v2/template/{template_id}/version/{version}
- **Auth**: service JWT or ApiKey-v1
- **Path params**: `template_id` (UUID); `version` (int, optional)
- **Response 200**: `{template.serialize()}`

#### POST /v2/template/{template_id}/preview
- **Auth**: service JWT or ApiKey-v1
- **Request body** (schema: `post_template_preview_request`): `{"personalisation": obj|null}`
- **Response 200**: rendered template preview (schema: `create_post_template_preview_response`)
- **Response 400**: missing required personalisation placeholders

---

### V2 — Templates List (blueprint: `v2_templates`, prefix: `/v2/templates`, auth: `requires_auth`)

Surface: **public**

#### GET /v2/templates
- **Auth**: service JWT or ApiKey-v1
- **Query params** (schema: `get_all_template_request`): `type` (string: `email`|`sms`|`letter`, optional)
- **Response 200**: `{"templates": [{template.serialize()}]}`

---

### V2 — Inbound SMS (blueprint: `v2_inbound_sms`, prefix: `/v2/received-text-messages`, auth: `requires_auth`)

Surface: **public**

#### GET /v2/received-text-messages
- **Auth**: service JWT or ApiKey-v1
- **Query params** (schema: `get_inbound_sms_request`): `older_than` (UUID, optional)
- **Response 200**: `{"received_text_messages":[{inbound_sms.serialize()}...],"links":{"current":url,"next":url}}`
- **Notes**: Paginated cursor-based — no total count; uses `older_than` for forward pagination.

---

### V2 — OpenAPI Spec (blueprint: `v2_api_spec`, prefix: `/v2`, auth: `requires_no_auth`)

Surface: **public**

#### GET /v2/openapi-en
- **Auth**: none
- **Response 200**: YAML content (`application/yaml`) — English OpenAPI 3.0 spec from `openapi/v2-notifications-api-en.yaml`

#### GET /v2/openapi-fr
- **Auth**: none
- **Response 200**: YAML content (`application/yaml`) — French OpenAPI 3.0 spec from `openapi/v2-notifications-api-fr.yaml`

---

## Schema Validation Reference

### Schema validation approach
The codebase uses two schema systems in parallel:
- **Marshmallow** (`app/schemas.py`, `flask_marshmallow`) for serialisation/deserialisation of model objects
- **jsonschema** (`app/schema_validation/`, `validate()` function) for request body validation using JSON Schema draft-07 style definitions

### Key Marshmallow schemas (`app/schemas.py`)

| Schema name | Used for |
|---|---|
| `service_schema` | Service CRUD request/response |
| `detailed_service_schema` | Service with embedded statistics |
| `api_key_schema` | API key creation and listing |
| `api_key_history_schema` | API key audit history |
| `job_schema` | Bulk send job |
| `notification_with_personalisation_schema` | Notification incl. decrypted personalisation |
| `notification_with_template_schema` | Notification with template details |
| `notifications_filter_schema` | Query filters for notification listing |
| `invited_user_schema` | Service invitation |
| `email_notification_schema` | v1 email send request |
| `sms_template_notification_schema` | v1 SMS send request |
| `template_schema` | Full template object |
| `reduced_template_schema` | Lightweight template list |
| `template_history_schema` | Template version history |
| `template_category_schema` | Template category |
| `provider_details_schema` | Provider detail object |
| `provider_details_history_schema` | Provider version history |
| `event_schema` | Audit event |
| `email_data_request_schema` | `{"email": str}` — email address input |
| `partial_email_data_request_schema` | `{"email": str}` — partial email search |
| `create_user_schema` | New user registration |
| `user_update_schema_load_json` | User attribute update |
| `user_update_password_schema_load_json` | Password update |
| `report_schema` | Service report object |
| `unarchived_template_schema` | Validates `archived: false` |

### JSON Schema definitions (`app/schema_validation/definitions.py`)

| Definition | Type | Notes |
|---|---|---|
| `uuid` | `string`, format `validate_uuid` | Standard UUID |
| `nullable_uuid` | `["string","null"]`, format `validate_uuid` | Optional UUID |
| `personalisation` | `object` | Key-value pairs; document attachment values must have `file` (base64), `sending_method` (`attach`\|`link`), and `filename` (required when `attach`) |
| `letter_personalisation` | `object` | Extension of personalisation; requires `address_line_1`, `address_line_2`, `postcode` |
| `https_url` | `string`, format `uri`, pattern `^https.*` | HTTPS URL for callback endpoints |

---

## Feature Flags

The following feature flags (config variables prefixed `FF_`) affect request handling:

| Flag | Effect on API behaviour |
|---|---|
| `FF_USE_BILLABLE_UNITS` | When true, SMS limit checks use SMS fragment counts instead of message counts; affects `/v2/notifications/sms`, `POST /service/{id}/job`, and `POST /notifications/sms` |
| `FF_SALESFORCE_CONTACT` | When true, service lifecycle events (create, go-live, update name, archive) and user events (activate, update) are mirrored to Salesforce |
| `FF_BOUNCE_RATE_SEED_EPOCH_MS` | When set to an epoch timestamp, seeds bounce-rate Redis data for a service within a 24-hour window after that timestamp |
| `FF_PT_SERVICE_SKIP_FRESHDESK` | When true, contact requests from P/T service members are sent via secure email instead of Freshdesk |

---

## CORS Policy

The `after_request` handler adds CORS headers only for requests from these origins:
- `https://documentation.notification.canada.ca`
- `https://documentation.staging.notification.cdssandbox.xyz`
- `https://documentation.dev.notification.cdssandbox.xyz`
- `https://cds-snc.github.io`

Allowed methods: `GET, PUT, POST, DELETE`. Allowed headers: `Content-Type, Authorization`.
