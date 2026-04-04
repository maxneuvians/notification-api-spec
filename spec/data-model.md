# Data Model

## Overview

**Total table count: 68**

All tables in alphabetical order (two-column layout):

| | |
|---|---|
| annual_billing | notification_history |
| annual_limits_data | notification_status_types |
| api_keys | notifications |
| api_keys_history | organisation |
| auth_type | organisation_types |
| branding_type | permissions |
| complaints | provider_details |
| daily_sorted_letter | provider_details_history |
| dm_datetime | provider_rates |
| domain | rates |
| email_branding | reports |
| events | scheduled_notifications |
| fido2_keys | service_callback_api |
| fido2_sessions | service_callback_api_history |
| ft_billing | service_callback_type |
| ft_notification_status | service_data_retention |
| inbound_numbers | service_email_branding |
| inbound_sms | service_email_reply_to |
| invite_status_type | service_inbound_api |
| invited_organisation_users | service_inbound_api_history |
| invited_users | service_letter_branding |
| job_status | service_letter_contacts |
| jobs | service_permission_types |
| key_types | service_permissions |
| letter_branding | service_safelist |
| letter_rates | service_sms_senders |
| login_events | services |
| monthly_notification_stats_summary | services_history |
| notification_history | template_categories |
| notification_status_types | template_folder |
| notifications | template_folder_map |
| organisation | template_process_type |
| organisation_types | template_redacted |
| permissions | templates |

(continued)

| | |
|---|---|
| templates_history | user_folder_permissions |
| user_to_organisation | user_to_service |
| users | verify_codes |

**Main domains:**
- **Notifications**: `notifications`, `notification_history`, `scheduled_notifications`, `notification_status_types`, `ft_notification_status`, `monthly_notification_stats_summary`
- **Services**: `services`, `services_history`, `service_permissions`, `service_sms_senders`, `service_email_reply_to`, `service_letter_contacts`, `service_callback_api`, `service_inbound_api`, `service_data_retention`, `service_safelist`, `service_email_branding`, `service_letter_branding`, `service_permission_types`, `service_callback_type`
- **Users**: `users`, `user_to_service`, `user_to_organisation`, `user_folder_permissions`, `verify_codes`, `login_events`, `fido2_keys`, `fido2_sessions`, `permissions`
- **Templates**: `templates`, `templates_history`, `template_folder`, `template_folder_map`, `template_redacted`, `template_categories`, `template_process_type`
- **Billing**: `annual_billing`, `annual_limits_data`, `ft_billing`, `rates`, `letter_rates`, `provider_rates`, `daily_sorted_letter`
- **Organisations**: `organisation`, `organisation_types`, `domain`, `invited_organisation_users`, `user_to_organisation`
- **Providers**: `provider_details`, `provider_details_history`, `provider_rates`
- **Inbound SMS**: `inbound_numbers`, `inbound_sms`, `service_inbound_api`, `service_inbound_api_history`
- **Branding**: `email_branding`, `letter_branding`, `branding_type`, `service_email_branding`, `service_letter_branding`
- **Auth / Security**: `auth_type`, `api_keys`, `api_keys_history`, `key_types`, `fido2_keys`, `fido2_sessions`, `verify_codes`, `login_events`, `invited_users`, `invite_status_type`

**Key patterns present in the schema:**
- **History / audit tables**: Versioned entities (api_keys, provider_details, service_callback_api, service_inbound_api, services, templates) are mirrored to a `*_history` sibling with a composite `(id, version)` PK. No FK constraints enforced on history rows.
- **Enum lookup tables**: Single-column `name` PK tables (`auth_type`, `branding_type`, `invite_status_type`, `job_status`, `key_types`, `notification_status_types`, `organisation_types`, `service_callback_type`, `service_permission_types`, `template_process_type`) with referencing columns declared as FKs.
- **Soft-delete via `archived` / `expiry_date`**: `api_keys` (expiry_date), `jobs` (archived), `service_email_reply_to` (archived), `service_letter_contacts` (archived), `service_sms_senders` (archived), `templates` (archived).
- **Versioned entities via composite `(id, version)` PKs**: `api_keys_history`, `provider_details_history`, `service_callback_api_history`, `service_inbound_api_history`, `services_history`, `templates_history`.
- **Encrypted columns**: `_personalisation`, `to`, `normalised_to` on `notifications`; `_content` on `inbound_sms` (signer: `signer_inbound_sms`); `bearer_token` on `service_callback_api` and `service_inbound_api`; `_code` on `verify_codes`; `_password` on `users`. Column names prefixed with `_` are SQLAlchemy hybrid properties whose getter decrypts and setter encrypts transparently. `to` and `normalised_to` use the `SensitiveString` custom column type (see §Notifications).
- **JSONB columns**: `compromised_key_info` (api_keys), `folder_permissions` (invited_users), `data` (login_events), `additional_information` (users).
- **Native PostgreSQL ENUMs**: `notification_type` (email/sms/letter), `invited_users_status_types`, `permission_types`, `notification_feedback_types`, `notification_feedback_subtypes`, `verify_code_types`, `sms_sending_vehicle`, `template_type`, `recipient_type`.
- **Denormalised fact tables**: `ft_billing` and `ft_notification_status` use wide composite PKs and carry no FK constraints; `monthly_notification_stats_summary` and `annual_limits_data` similarly.
- **Association / secondary tables**: `user_to_service`, `user_to_organisation`, `service_email_branding`, `service_letter_branding`, `template_folder_map`, `user_folder_permissions`.

---

## Enum / Lookup Tables

### Lookup tables (single-column `name` PK, referenced via FK)

| Table | Stored values |
|---|---|
| `auth_type` | `sms_auth`, `email_auth`, `security_key_auth` |
| `branding_type` | `fip_english` (deprecated), `org` (migrations only), `org_banner` (migrations only), `custom_logo`, `both_english`, `both_french`, `custom_logo_with_background_colour`, `no_branding` |
| `invite_status_type` | `pending`, `accepted`, `cancelled` |
| `job_status` | `pending`, `in progress`, `finished`, `sending limits exceeded`, `scheduled`, `cancelled`, `ready to send`, `sent to dvla`, `error` |
| `key_types` | `normal`, `team`, `test` |
| `notification_status_types` | `cancelled`, `created`, `sending`, `sent`, `delivered`, `pending`, `failed`, `technical-failure`, `temporary-failure`, `permanent-failure`, `provider-failure`, `pending-virus-check`, `validation-failed`, `virus-scan-failed`, `returned-letter`, `pii-check-failed` |
| `organisation_types` | `central`, `province_or_territory`, `local`, `nhs_central`, `nhs_local`, `nhs_gp`, `emergency_service`, `school_or_college`, `other` |
| `service_callback_type` | `delivery_status`, `complaint` |
| `service_permission_types` | `email`, `sms`, `letter`, `international_sms`, `inbound_sms`, `schedule_notifications`, `email_auth`, `letters_as_pdf`, `upload_document`, `edit_folder_permissions`, `upload_letters` |
| `template_process_type` | `bulk`, `normal`, `priority`, `low`, `medium`, `high` |

### Native PostgreSQL ENUM types (DDL `CREATE TYPE`)

| Type name | Values |
|---|---|
| `notification_type` | `email`, `sms`, `letter` |
| `invited_users_status_types` | `pending`, `accepted`, `cancelled` |
| `permission_types` | `manage_users`, `manage_templates`, `manage_settings`, `send_texts`, `send_emails`, `send_letters`, `manage_api_keys`, `platform_admin`, `view_activity` |
| `notification_feedback_types` | `hard-bounce`, `soft-bounce`, `unknown-bounce` |
| `notification_feedback_subtypes` | 9 values (bounce sub-classification codes) |
| `verify_code_types` | `email`, `sms` |
| `sms_sending_vehicle` | `short_code`, `long_code` |
| `template_type` | `email`, `sms`, `letter` |
| `recipient_type` | `mobile`, `email` |
| `job_status_types` | `pending`, `in progress`, `finished`, `sending limits exceeded` — **4 values only**; the authoritative lookup table `job_status` has 9 values (adds `cancelled`, `ready to send`, `sent to dvla`, `error`, `scheduled`). The application code uses the FK lookup table, not this native enum. **Go must use the lookup table values, not this enum.** |
| `notify_status_type` | Mirror of `notification_status_types` values — **dead code**. Defined in DDL (`CREATE TYPE`) but never referenced by application code or any FK constraint. Go implementation must ignore this type. |

---

## Tables

### annual_billing

**Purpose**: Stores the free SMS fragment allowance granted to a service for a given financial year.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | | PK |
| service_id | uuid | NO | | FK → services.id |
| financial_year_start | integer | NO | | e.g. 2024 for FY 2024/25 |
| free_sms_fragment_limit | integer | NO | | Annual free SMS fragments |
| created_at | timestamp | NO | | |
| updated_at | timestamp | YES | | |

**Indexes**: `ix_annual_billing_service_id` on (service_id)

**Foreign Keys**:
- `service_id` → `services.id`

---

### annual_limits_data

**Purpose**: Tracks per-service notification counts against annual email/SMS limits, broken down by time period and notification type.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| service_id | uuid | NO | | Part of logical PK; FK → services (no FK constraint) |
| time_period | varchar | NO | | e.g. `"2024-01"` (month) or year string |
| annual_email_limit | bigint | NO | | |
| annual_sms_limit | bigint | NO | | |
| notification_type | varchar | NO | | `email` or `sms` |
| notification_count | bigint | NO | | Cumulative count for the period |

**Indexes**:
- `ix_service_id_notification_type` on (service_id, notification_type)
- `ix_service_id_notification_type_time` on (time_period, service_id, notification_type)

---

### api_keys

**Purpose**: API keys issued to services for authenticating API requests; supports soft-expiry and compromise tracking.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | | PK |
| name | varchar(255) | NO | | Display name |
| secret | varchar(255) | NO | | Hashed secret; UNIQUE |
| service_id | uuid | NO | | FK → services.id |
| key_type | varchar(255) | NO | | FK → key_types.name |
| version | integer | NO | | Optimistic-lock version |
| created_at | timestamp | NO | | |
| created_by_id | uuid | NO | | FK → users.id |
| updated_at | timestamp | YES | | |
| expiry_date | timestamp | YES | | NULL = active; set to soft-delete |
| compromised_key_info | jsonb | YES | | Details if key was compromised |
| last_used_timestamp | timestamp | YES | | |

**Indexes**:
- `ix_api_keys_service_id` on (service_id)
- `ix_api_keys_key_type` on (key_type)
- `ix_api_keys_created_by_id` on (created_by_id)
- `uix_service_to_key_name` UNIQUE on (service_id, name) WHERE expiry_date IS NULL

**Foreign Keys**:
- `service_id` → `services.id`
- `key_type` → `key_types.name`
- `created_by_id` → `users.id` (constraint: `fk_api_keys_created_by_id`)

**Constraints**: UNIQUE (`secret`)

**Notes**: Soft-deleted by setting `expiry_date`; `compromised_key_info` is JSONB. Versioned — changes are mirrored to `api_keys_history`. The partial index `uix_service_to_key_name` enforces name-uniqueness only for **active** keys (`WHERE expiry_date IS NULL`), so a service may reuse a key name after the prior key is expired/revoked.

---

### api_keys_history

**Purpose**: Audit history table for `api_keys`; records every version of each key.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | | Part of composite PK |
| version | integer | NO | | Part of composite PK |
| name | varchar(255) | NO | | |
| secret | varchar(255) | NO | | |
| service_id | uuid | NO | | |
| key_type | varchar(255) | NO | | |
| created_at | timestamp | NO | | |
| created_by_id | uuid | NO | | |
| updated_at | timestamp | YES | | |
| expiry_date | timestamp | YES | | |
| compromised_key_info | jsonb | YES | | |
| last_used_timestamp | timestamp | YES | | |

**Indexes**:
- `ix_api_keys_history_service_id` on (service_id)
- `ix_api_keys_history_key_type` on (key_type)
- `ix_api_keys_history_created_by_id` on (created_by_id)

**Notes**: History table pattern — composite PK (id, version). No FK constraints enforced on history rows.

---

### auth_type

**Purpose**: Lookup table enumerating valid user authentication methods.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| name | varchar | NO | | PK |

**Notes**: Enum lookup. Values: `sms_auth`, `email_auth`, `security_key_auth`.

---

### branding_type

**Purpose**: Lookup table enumerating valid email branding style identifiers.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| name | varchar(255) | NO | | PK |

**Notes**: Enum lookup. Values: `fip_english` (deprecated), `org` (migrations only), `org_banner` (migrations only), `custom_logo`, `both_english`, `both_french`, `custom_logo_with_background_colour`, `no_branding`.

---

### complaints

**Purpose**: Records email complaint feedback received from SES (e.g. spam reports) against notifications.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | | PK |
| notification_id | uuid | NO | | FK → notifications (index only, no FK constraint) |
| service_id | uuid | NO | | FK → services.id |
| ses_feedback_id | text | YES | | SES feedback message ID |
| complaint_type | text | YES | | e.g. `abuse` |
| complaint_date | timestamp | YES | | When SES received the complaint |
| created_at | timestamp | NO | | Record creation time |

**Indexes**:
- `ix_complaints_notification_id` on (notification_id)
- `ix_complaints_service_id` on (service_id)

**Foreign Keys**:
- `service_id` → `services.id`

---

### daily_sorted_letter

**Purpose**: Tracks daily physical letter counts split into sorted and unsorted volumes, keyed by billing day.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | | PK |
| billing_day | date | NO | | |
| unsorted_count | integer | NO | | |
| sorted_count | integer | NO | | |
| file_name | varchar | YES | | Source file name |
| updated_at | timestamp | YES | | |

**Indexes**:
- `ix_daily_sorted_letter_billing_day` on (billing_day)
- `ix_daily_sorted_letter_file_name` on (file_name)

---

### dm_datetime

**Purpose**: Date-dimension table for analytics queries; provides BST/UTC boundary times and fiscal calendar attributes for each calendar date.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| bst_date | date | NO | | PK |
| year | integer | NO | | |
| month | integer | NO | | |
| month_name | varchar | NO | | |
| day | integer | NO | | |
| bst_day | integer | NO | | Day-of-week in BST |
| day_of_year | integer | NO | | |
| week_day_name | varchar | NO | | |
| calendar_week | integer | NO | | ISO week |
| quartal | varchar | NO | | Quarter label |
| year_quartal | varchar | NO | | e.g. `2024Q1` |
| year_month | varchar | NO | | e.g. `2024-03` |
| year_calendar_week | varchar | NO | | e.g. `2024-W12` |
| financial_year | integer | NO | | |
| utc_daytime_start | timestamp | NO | | Start of BST day in UTC |
| utc_daytime_end | timestamp | NO | | End of BST day in UTC |

**Indexes**:
- `ix_dm_datetime_bst_date` on (bst_date)
- `ix_dm_datetime_yearmonth` on (year, month)

**Notes**: Analytics/reporting dimension table; no FK constraints.

---

### domain

**Purpose**: Associates verified email domains with organisations for auto-joining.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| domain | varchar(255) | NO | | PK |
| organisation_id | uuid | NO | | FK → organisation.id |

**Foreign Keys**:
- `organisation_id` → `organisation.id`

---

### email_branding

**Purpose**: Stores email branding assets (logo, colours, alt text) that can be assigned to services or organisations.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | | PK |
| name | varchar(255) | NO | | UNIQUE |
| brand_type | varchar(255) | NO | | FK → branding_type.name |
| colour | varchar(7) | YES | | Hex colour e.g. `#FF0000` |
| logo | varchar(255) | YES | | S3 path to logo file |
| text | varchar(255) | YES | | Banner text |
| organisation_id | uuid | YES | | FK → organisation.id (SET NULL on delete) |
| alt_text_en | varchar | YES | | English logo alt text |
| alt_text_fr | varchar | YES | | French logo alt text |
| created_by_id | uuid | NO | | FK → users.id |
| created_at | timestamp | NO | `now()` | |
| updated_by_id | uuid | YES | | FK → users.id |
| updated_at | timestamp | YES | `now()` | |

**Indexes**:
- `ix_email_branding_brand_type` on (brand_type)
- `ix_email_branding_organisation_id` on (organisation_id)
- `ix_email_branding_created_by_id` on (created_by_id)
- `ix_email_branding_updated_by_id` on (updated_by_id)

**Foreign Keys**:
- `brand_type` → `branding_type.name`
- `organisation_id` → `organisation.id` (ON DELETE SET NULL)
- `created_by_id` → `users.id`
- `updated_by_id` → `users.id`

**Constraints**: UNIQUE (`name`) — constraint `uq_email_branding_name`

---

### events

**Purpose**: Generic audit/event log storing arbitrary JSON payloads for system events.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | | PK |
| event_type | varchar(255) | NO | | Event category string |
| created_at | timestamp | NO | | |
| data | json | NO | | Event payload (JSON, not JSONB) |

**Notes**: No FK constraints; `data` uses plain `json` type (not `jsonb`).

---

### fido2_keys

**Purpose**: Stores registered FIDO2/WebAuthn security keys for users enabling hardware-token MFA.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | | PK |
| user_id | uuid | NO | | FK → users.id |
| name | varchar | NO | | User-assigned key label |
| key | text | NO | | Serialised FIDO2 credential |
| created_at | timestamp | NO | | |
| updated_at | timestamp | YES | | |

**Indexes**: `ix_fido2_keys_user_id` on (user_id)

**Foreign Keys**:
- `user_id` → `users.id`

---

### fido2_sessions

**Purpose**: Holds transient FIDO2 challenge sessions during the WebAuthn authentication ceremony.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| user_id | uuid | NO | | PK; FK → users.id |
| session | text | NO | | Serialised challenge/session data |
| created_at | timestamp | NO | | |

**Foreign Keys**:
- `user_id` → `users.id`

**Notes**: PK is `user_id` — one active session per user at a time.

---

### ft_billing

**Purpose**: Fact table aggregating daily billing units and notification counts per template/service/provider combination for reporting.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| bst_date | date | NO | | Part of composite PK |
| template_id | uuid | NO | | Part of composite PK |
| service_id | uuid | NO | | Part of composite PK |
| notification_type | text | NO | | Part of composite PK (`email`, `sms`, `letter`) |
| provider | text | NO | | Part of composite PK |
| rate_multiplier | integer | NO | | Part of composite PK |
| international | boolean | NO | | Part of composite PK |
| rate | numeric | NO | | Part of composite PK |
| postage | varchar | NO | | Part of composite PK |
| sms_sending_vehicle | sms_sending_vehicle | NO | `long_code` | Part of composite PK; enum: `short_code`, `long_code` |
| billable_units | integer | YES | | |
| notifications_sent | integer | YES | | |
| billing_total | numeric(16,8) | YES | | Pre-computed cost |
| created_at | timestamp | NO | | |
| updated_at | timestamp | YES | | |

**Indexes**:
- `ix_ft_billing_bst_date` on (bst_date)
- `ix_ft_billing_service_id` on (service_id)
- `ix_ft_billing_template_id` on (template_id)

**Notes**: Composite PK across 10 columns. No FK constraints (denormalised fact table). `sms_sending_vehicle` is a PostgreSQL ENUM type.

---

### ft_notification_status

**Purpose**: Fact table aggregating daily notification counts by status, key type, and template/service/job for reporting.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| bst_date | date | NO | | Part of composite PK |
| template_id | uuid | NO | | Part of composite PK |
| service_id | uuid | NO | | Part of composite PK |
| job_id | uuid | NO | | Part of composite PK |
| notification_type | text | NO | | Part of composite PK |
| key_type | text | NO | | Part of composite PK |
| notification_status | text | NO | | Part of composite PK |
| notification_count | integer | NO | | |
| billable_units | integer | NO | | |
| created_at | timestamp | NO | | |
| updated_at | timestamp | YES | | |

**Indexes**:
- `ix_ft_notification_service_bst` on (service_id, bst_date)
- `ix_ft_notification_status_bst_date` on (bst_date)
- `ix_ft_notification_status_job_id` on (job_id)
- `ix_ft_notification_status_service_id` on (service_id)
- `ix_ft_notification_status_template_id` on (template_id)
- `ix_ft_notification_status_stats_lookup` on (bst_date, notification_status, key_type) INCLUDE (notification_type, notification_count)

**Notes**: Composite PK across 7 columns. No FK constraints (denormalised fact table).

---

### inbound_numbers

**Purpose**: Manages inbound SMS phone numbers provisioned for services, tracking which number belongs to which service.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | | PK |
| number | varchar(11) | NO | | UNIQUE |
| provider | varchar | NO | | SMS provider name |
| service_id | uuid | YES | | FK → services.id; NULL = unassigned |
| active | boolean | NO | | |
| created_at | timestamp | NO | | |
| updated_at | timestamp | YES | | |

**Indexes**: `ix_inbound_numbers_service_id` UNIQUE on (service_id)

**Foreign Keys**:
- `service_id` → `services.id`

**Constraints**: UNIQUE (`number`); unique index on `service_id` (one number per service)

---

### inbound_sms

**Purpose**: Stores inbound SMS messages received on service numbers so they can be retrieved by the service.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | | PK |
| service_id | uuid | NO | | FK → services.id |
| content (`_content`) | varchar | NO | | Message body — **encrypted at application layer** via `signer_inbound_sms`; accessed in Python as `_content` (hybrid property) |
| notify_number | varchar | NO | | The Notify number the message was sent to |
| user_number | varchar | NO | | Sender's phone number |
| provider | varchar | NO | | |
| created_at | timestamp | NO | | |
| provider_date | timestamp | YES | | Timestamp from provider |
| provider_reference | varchar | YES | | Provider message ID |

**Indexes**:
- `ix_inbound_sms_service_id` on (service_id)
- `ix_inbound_sms_user_number` on (user_number)

**Foreign Keys**:
- `service_id` → `services.id`

---

### invite_status_type

**Purpose**: Lookup table enumerating valid invitation statuses.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| name | varchar | NO | | PK |

**Notes**: Enum lookup. Values: `pending`, `accepted`, `cancelled`.

---

### invited_organisation_users

**Purpose**: Tracks pending and processed invitations for users to join an organisation.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | | PK |
| email_address | varchar(255) | NO | | Invitee email |
| invited_by_id | uuid | NO | | FK → users.id |
| organisation_id | uuid | NO | | FK → organisation.id |
| status | varchar | NO | | FK → invite_status_type.name |
| created_at | timestamp | NO | | |

**Foreign Keys**:
- `invited_by_id` → `users.id`
- `organisation_id` → `organisation.id`
- `status` → `invite_status_type.name`

---

### invited_users

**Purpose**: Tracks pending and processed invitations for users to join a service, including permissions granted.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | | PK |
| email_address | varchar(255) | NO | | |
| user_id | uuid | NO | | FK → users.id (the inviter) |
| service_id | uuid | YES | | FK → services.id |
| status | invited_users_status_types | NO | | PG enum: `pending`, `accepted`, `cancelled` |
| auth_type | varchar | NO | `sms_auth` | FK → auth_type.name |
| permissions | varchar | NO | | Comma-separated permission list |
| folder_permissions | jsonb | NO | | Template folder access (JSONB array) |
| created_at | timestamp | NO | | |

**Indexes**:
- `ix_invited_users_user_id` on (user_id)
- `ix_invited_users_service_id` on (service_id)
- `ix_invited_users_auth_type` on (auth_type)

**Foreign Keys**:
- `user_id` → `users.id`
- `service_id` → `services.id`
- `auth_type` → `auth_type.name`

**Notes**: `status` column uses native PostgreSQL ENUM type `invited_users_status_types`. `folder_permissions` is JSONB.

---

### job_status

**Purpose**: Lookup table enumerating valid bulk-job lifecycle states.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| name | varchar(255) | NO | | PK |

**Notes**: Enum lookup. Values: `pending`, `in progress`, `finished`, `sending limits exceeded`, `scheduled`, `cancelled`, `ready to send`, `sent to dvla`, `error`.

---

### jobs

**Purpose**: Represents a bulk notification job created from an uploaded CSV file; tracks processing progress and final counts.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | | PK |
| original_file_name | varchar | NO | | Uploaded CSV filename |
| service_id | uuid | NO | | FK → services.id |
| template_id | uuid | YES | | FK → templates.id |
| template_version | integer | NO | | Template version at job creation |
| job_status | varchar(255) | NO | | FK → job_status.name |
| notification_count | integer | NO | | Total rows in CSV |
| notifications_sent | integer | NO | | Notifications dispatched |
| notifications_delivered | integer | NO | | Successfully delivered |
| notifications_failed | integer | NO | | Failed to deliver |
| created_at | timestamp | NO | | |
| created_by_id | uuid | YES | | FK → users.id |
| updated_at | timestamp | YES | | |
| processing_started | timestamp | YES | | |
| processing_finished | timestamp | YES | | |
| scheduled_for | timestamp | YES | | Delayed execution time |
| archived | boolean | NO | `false` | Soft-delete flag |
| api_key_id | uuid | YES | | FK → api_keys.id |
| sender_id | uuid | YES | | Sender identity UUID |

**Indexes**:
- `ix_jobs_service_id` on (service_id)
- `ix_jobs_template_id` on (template_id)
- `ix_jobs_job_status` on (job_status)
- `ix_jobs_created_at` on (created_at)
- `ix_jobs_created_by_id` on (created_by_id)
- `ix_jobs_processing_started` on (processing_started)
- `ix_jobs_scheduled_for` on (scheduled_for)
- `ix_jobs_api_key_id` on (api_key_id)

**Foreign Keys**:
- `service_id` → `services.id`
- `template_id` → `templates.id`
- `job_status` → `job_status.name`
- `created_by_id` → `users.id`
- `api_key_id` → `api_keys.id`

**Notes**: `archived = true` is the soft-delete pattern. `sender_id` has no FK constraint.

---

### key_types

**Purpose**: Lookup table enumerating the categories of API keys.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| name | varchar(255) | NO | | PK |

**Notes**: Enum lookup. Values: `normal`, `team`, `test`.

---

### letter_branding

**Purpose**: Stores letter branding identifiers (name + filename) that can be assigned to organisations or services.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | | PK |
| name | varchar(255) | NO | | UNIQUE |
| filename | varchar(255) | NO | | UNIQUE; S3 asset path |

**Constraints**: UNIQUE (`name`), UNIQUE (`filename`)

---

### letter_rates

**Purpose**: Stores per-sheet postage rates for letters, supporting date-ranged pricing tiers.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | | PK |
| start_date | timestamp | NO | | Rate effective from |
| end_date | timestamp | YES | | NULL = currently active rate |
| sheet_count | integer | NO | | Number of sheets |
| rate | numeric | NO | | Cost per letter |
| crown | boolean | NO | | Crown vs non-crown rate |
| post_class | varchar | NO | | e.g. `first`, `second` |

---

### login_events

**Purpose**: Audit log of user login events with arbitrary metadata.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | | PK |
| user_id | uuid | NO | | FK → users.id |
| data | jsonb | NO | | Login event metadata (JSONB) |
| created_at | timestamp | NO | | |
| updated_at | timestamp | YES | | |

**Indexes**: `ix_login_events_user_id` on (user_id)

**Foreign Keys**:
- `user_id` → `users.id`

**Notes**: `data` is JSONB.

---

### monthly_notification_stats_summary

**Purpose**: Pre-aggregated monthly notification counts per service and notification type for dashboard/stats queries.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| month | text | NO | | Part of composite PK; e.g. `"2024-03"` |
| service_id | uuid | NO | | Part of composite PK |
| notification_type | text | NO | | Part of composite PK |
| notification_count | integer | NO | | |
| updated_at | timestamp | NO | `now()` | |

**Indexes**:
- `ix_monthly_notification_stats_notification_type` on (notification_type)
- `ix_monthly_notification_stats_updated_at` on (updated_at)

**Notes**: Composite PK (month, service_id, notification_type). No FK constraints (denormalised summary table).

---

### notification_history

**Purpose**: Long-term archive of sent notifications; rows are moved here from `notifications` after a retention period. Carries all delivery and billing metadata but not personalisation.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | | PK |
| job_id | uuid | YES | | FK → jobs.id |
| job_row_number | integer | YES | | Row position within parent job |
| service_id | uuid | YES | | FK → services.id |
| template_id | uuid | YES | | FK → templates_history (composite) |
| template_version | integer | NO | | Part of FK to templates_history |
| api_key_id | uuid | YES | | FK → api_keys.id |
| key_type | varchar | NO | | FK → key_types.name |
| notification_type | notification_type | NO | | PG enum: `email`, `sms`, `letter` |
| notification_status | text | YES | | FK → notification_status_types.name |
| created_at | timestamp | NO | | |
| sent_at | timestamp | YES | | |
| sent_by | varchar | YES | | Provider that sent it |
| updated_at | timestamp | YES | | |
| reference | varchar | YES | | AWS message ID |
| billable_units | integer | NO | | SMS fragments or letter sheets |
| client_reference | varchar | YES | | Caller-supplied reference |
| international | boolean | YES | | |
| phone_prefix | varchar | YES | | Country dial code |
| rate_multiplier | numeric | YES | | Billing rate multiplier |
| created_by_id | uuid | YES | | |
| postage | varchar | YES | | `first` or `second` (letter only) |
| queue_name | text | YES | | Celery queue that processed it |
| feedback_type | notification_feedback_types | YES | | PG enum: `hard-bounce`, `soft-bounce`, `unknown-bounce` |
| feedback_subtype | notification_feedback_subtypes | YES | | PG enum (9 values) |
| ses_feedback_id | varchar | YES | | SES feedback identifier |
| ses_feedback_date | timestamp | YES | | |
| sms_total_message_price | double precision | YES | | Total SMS cost |
| sms_total_carrier_fee | double precision | YES | | Carrier portion of cost |
| sms_iso_country_code | varchar | YES | | ISO country code of destination |
| sms_carrier_name | varchar | YES | | Carrier name |
| sms_message_encoding | varchar | YES | | GSM7 / UCS2 |
| sms_origination_phone_number | varchar | YES | | Sending phone number |
| feedback_reason | varchar(255) | YES | | Pinpoint failure reason |

**Indexes**:
- `ix_notification_history_api_key_id` on (api_key_id)
- `ix_notification_history_api_key_id_created` on (api_key_id, created_at)
- `ix_notification_history_created_api_key_id` on (created_at, api_key_id)
- `ix_notification_history_created_at` on (created_at)
- `ix_notification_history_created_by_id` on (created_by_id)
- `ix_notification_history_feedback_reason` on (feedback_reason)
- `ix_notification_history_feedback_type` on (feedback_type)
- `ix_notification_history_job_id` on (job_id)
- `ix_notification_history_key_type` on (key_type)
- `ix_notification_history_notification_status` on (notification_status)
- `ix_notification_history_notification_type` on (notification_type)
- `ix_notification_history_reference` on (reference)
- `ix_notification_history_service_id` on (service_id)
- `ix_notification_history_service_id_created_at` on (service_id, date(created_at))
- `ix_notification_history_template_id` on (template_id)
- `ix_notification_history_week_created` on (date_trunc('week', created_at))

**Foreign Keys**:
- `job_id` → `jobs.id`
- `service_id` → `services.id`
- `api_key_id` → `api_keys.id`
- `key_type` → `key_types.name`
- `notification_status` → `notification_status_types.name`
- `(template_id, template_version)` → `templates_history(id, version)`

**Constraints**: CHECK `chk_notification_history_postage_null` — for `letter` notifications postage must be `first` or `second`; for all others postage must be NULL.

**Notes**: History/archive table — no `_personalisation` column (personalisation is not retained). `notification_type` and `feedback_type`/`feedback_subtype` are native PostgreSQL ENUM types.

---

### notification_status_types

**Purpose**: Lookup table enumerating all valid notification delivery statuses.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| name | varchar | NO | | PK |

**Notes**: Enum lookup. Values: `cancelled`, `created`, `sending`, `sent`, `delivered`, `pending`, `failed`, `technical-failure`, `temporary-failure`, `permanent-failure`, `provider-failure`, `pending-virus-check`, `validation-failed`, `virus-scan-failed`, `returned-letter`, `pii-check-failed`.

---

### notifications

**Purpose**: Live store for in-flight and recently sent notifications; the primary transactional notifications table that includes personalisation data.

**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | | PK |
| to | varchar | NO | | Recipient address (email or phone) |
| job_id | uuid | YES | | FK → jobs.id |
| service_id | uuid | YES | | FK → services.id |
| template_id | uuid | YES | | FK → templates_history (composite) |
| template_version | integer | NO | | Part of FK to templates_history |
| created_at | timestamp | NO | | |
| sent_at | timestamp | YES | | |
| sent_by | varchar | YES | | Provider that sent it |
| updated_at | timestamp | YES | | |
| reference | varchar | YES | | AWS message ID |
| job_row_number | integer | YES | | Row position within parent job |
| _personalisation | varchar | YES | | Encrypted personalisation payload |
| api_key_id | uuid | YES | | FK → api_keys.id |
| key_type | varchar(255) | NO | | FK → key_types.name |
| notification_type | notification_type | NO | | PG enum: `email`, `sms`, `letter` |
| billable_units | integer | NO | | SMS fragments or letter sheets |
| client_reference | varchar | YES | | Caller-supplied reference |
| international | boolean | YES | | |
| phone_prefix | varchar | YES | | Country dial code |
| rate_multiplier | numeric | YES | | Billing rate multiplier |
| notification_status | text | YES | | FK → notification_status_types.name |
| normalised_to | varchar | YES | | Normalised recipient address for dedup |
| created_by_id | uuid | YES | | FK → users.id |
| reply_to_text | varchar | YES | | Reply-to address or number |
| postage | varchar | YES | | `first` or `second` (letter only) |
| provider_response | text | YES | | Raw provider response |
| queue_name | text | YES | | Celery queue that processed it |
| feedback_type | notification_feedback_types | YES | | PG enum: `hard-bounce`, `soft-bounce`, `unknown-bounce` |
| feedback_subtype | notification_feedback_subtypes | YES | | PG enum (9 values) |
| ses_feedback_id | varchar | YES | | SES feedback identifier |
| ses_feedback_date | timestamp | YES | | |
| sms_total_message_price | double precision | YES | | Total SMS cost |
| sms_total_carrier_fee | double precision | YES | | Carrier portion of cost |
| sms_iso_country_code | varchar | YES | | ISO country code of destination |
| sms_carrier_name | varchar | YES | | Carrier name |
| sms_message_encoding | varchar | YES | | GSM7 / UCS2 |
| sms_origination_phone_number | varchar | YES | | Sending phone number |
| feedback_reason | varchar(255) | YES | | Pinpoint failure reason |

**Indexes**:
- `ix_notifications_api_key_id` on (api_key_id)
- `ix_notifications_client_reference` on (client_reference)
- `ix_notifications_created_at` on (created_at)
- `ix_notifications_feedback_reason` on (feedback_reason)
- `ix_notifications_feedback_type` on (feedback_type)
- `ix_notifications_job_id` on (job_id)
- `ix_notifications_key_type` on (key_type)
- `ix_notifications_notification_status` on (notification_status)
- `ix_notifications_notification_type` on (notification_type)
- `ix_notifications_reference` on (reference)
- `ix_notifications_service_created_at` on (service_id, created_at)
- `ix_notifications_service_id` on (service_id)
- `ix_notifications_service_id_created_at` on (service_id, date(created_at))
- `ix_notifications_template_id` on (template_id)

**Foreign Keys**:
- `job_id` → `jobs.id`
- `service_id` → `services.id`
- `api_key_id` → `api_keys.id`
- `key_type` → `key_types.name`
- `notification_status` → `notification_status_types.name`
- `created_by_id` → `users.id`
- `(template_id, template_version)` → `templates_history(id, version)`

**Constraints**: CHECK `chk_notifications_postage_null` — for `letter` notifications postage must be `first` or `second`; for all others postage must be NULL.

**Notes**: Multiple columns are encrypted or PII-protected using the `SensitiveString` custom SQLAlchemy column type (`to`, `normalised_to`) or the hybrid-property encryption pattern (`_personalisation` via `signer_personalisation`). `SensitiveString` encrypts on write and decrypts on read transparently. Differs from `notification_history` by also holding `to`, `normalised_to`, `reply_to_text`, `provider_response`, and `_personalisation`. Rows are archived to `notification_history` and deleted on a rolling schedule.

---

### organisation
**Purpose**: Represents a government organisation that owns one or more services and may sign a service agreement.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| name | varchar(255) | NO | — | UNIQUE |
| active | boolean | NO | — | |
| created_at | timestamp | NO | — | |
| updated_at | timestamp | YES | — | |
| email_branding_id | uuid | YES | — | soft FK to email_branding.id (no DB constraint) |
| letter_branding_id | uuid | YES | — | FK → letter_branding.id |
| agreement_signed | boolean | YES | — | |
| agreement_signed_at | timestamp | YES | — | |
| agreement_signed_by_id | uuid | YES | — | FK → users.id |
| agreement_signed_version | float8 | YES | — | |
| crown | boolean | YES | — | true = Crown Corporation |
| organisation_type | varchar(255) | YES | — | FK → organisation_types.name |
| request_to_go_live_notes | text | YES | — | |
| agreement_signed_on_behalf_of_email_address | varchar(255) | YES | — | |
| agreement_signed_on_behalf_of_name | varchar(255) | YES | — | |
| default_branding_is_french | boolean | YES | false | nullable; DEFAULT false but no NOT NULL constraint |

**Indexes**: `ix_organisation_name` UNIQUE btree(name)  
**Foreign Keys**:
- `agreement_signed_by_id` → `users.id`
- `letter_branding_id` → `letter_branding.id`
- `organisation_type` → `organisation_types.name`

**Notes**: `email_branding_id` is stored but enforced only via `service_email_branding` association table at the service level; the Organisation model uses a SQLAlchemy relationship without a DB-level FK constraint on `email_branding_id`. Has M2M with `users` via `user_to_organisation`.

---

### organisation_types
**Purpose**: Lookup table of valid organisation type codes with their free SMS fragment allowance.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| name | varchar(255) | NO | — | PK |
| is_crown | boolean | YES | — | nullable; true/false/null |
| annual_free_sms_fragment_limit | bigint | NO | — | |

**Indexes**: none beyond PK  
**Foreign Keys**: none  
**Notes**: Enum lookup. Known values: `central`, `province_or_territory`, `local`, `nhs_central`, `nhs_local`, `nhs_gp`, `emergency_service`, `school_or_college`, `other`.

---

### permissions
**Purpose**: User-level permissions granting access to a specific operation within a service (or platform-wide when `service_id` is NULL).  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| service_id | uuid | YES | — | FK → services.id; NULL = platform-wide |
| user_id | uuid | NO | — | FK → users.id |
| permission | permission_types (enum) | NO | — | |
| created_at | timestamp | NO | — | |

**Indexes**:
- `ix_permissions_service_id` btree(service_id)
- `ix_permissions_user_id` btree(user_id)

**Foreign Keys**:
- `service_id` → `services.id`
- `user_id` → `users.id`

**Constraints**: UNIQUE(`service_id`, `user_id`, `permission`) — `uix_service_user_permission`  
**Notes**: `permission` type is a PostgreSQL enum `permission_types`. Known values: `manage_users`, `manage_templates`, `manage_settings`, `send_texts`, `send_emails`, `send_letters`, `manage_api_keys`, `platform_admin`, `view_activity`.

---

### provider_details
**Purpose**: Registered notification providers (e.g. SNS, SES, Pinpoint) with routing priority per channel.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| display_name | varchar | NO | — | |
| identifier | varchar | NO | — | internal code name |
| priority | integer | NO | — | lower = higher priority |
| notification_type | notification_type (enum) | NO | — | `email`, `sms`, or `letter` |
| active | boolean | NO | — | |
| updated_at | timestamp | YES | — | |
| version | integer | NO | — | ORM versioning |
| created_by_id | uuid | YES | — | FK → users.id |
| supports_international | boolean | NO | false | |

**Indexes**: `ix_provider_details_created_by_id` btree(created_by_id)  
**Foreign Keys**:
- `created_by_id` → `users.id`

**Notes**: Versioned — changes are mirrored to `provider_details_history`.

---

### provider_details_history
**Purpose**: Immutable audit log of all versions of each provider configuration.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK (composite with version) |
| display_name | varchar | NO | — | |
| identifier | varchar | NO | — | |
| priority | integer | NO | — | |
| notification_type | notification_type (enum) | NO | — | |
| active | boolean | NO | — | |
| version | integer | NO | — | PK (composite with id) |
| updated_at | timestamp | YES | — | |
| created_by_id | uuid | YES | — | FK → users.id |
| supports_international | boolean | NO | false | |

**Indexes**: `ix_provider_details_history_created_by_id` btree(created_by_id)  
**Foreign Keys**:
- `created_by_id` → `users.id`

**Notes**: History table pattern. PK is `(id, version)`.

---

### provider_rates
**Purpose**: Time-series of per-provider cost rates, effective from a given timestamp.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| valid_from | timestamp | NO | — | |
| rate | numeric | NO | — | cost per unit |
| provider_id | uuid | NO | — | FK → provider_details.id |

**Indexes**: `ix_provider_rates_provider_id` btree(provider_id)  
**Foreign Keys**:
- `provider_id` → `provider_details.id`

---

### rates
**Purpose**: Platform-level notification cost rates per channel type and SMS sending vehicle, effective from a given timestamp.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| valid_from | timestamp | NO | — | |
| rate | numeric | NO | — | |
| notification_type | notification_type (enum) | NO | — | |
| sms_sending_vehicle | sms_sending_vehicle (enum) | NO | `long_code` | `long_code` or `short_code` |

**Indexes**: `ix_rates_notification_type` btree(notification_type)  
**Notes**: Rate lookup for billing; `sms_sending_vehicle` enum distinguishes long-code vs short-code pricing.

---

### reports
**Purpose**: Tracks async report generation requests (e.g. CSV exports for notifications or jobs) with lifecycle status and download URL.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| report_type | varchar(255) | NO | — | `sms`, `email`, `job` |
| requested_at | timestamp | NO | — | |
| completed_at | timestamp | YES | — | |
| expires_at | timestamp | YES | — | S3 link TTL |
| requesting_user_id | uuid | YES | — | FK → users.id; NULL if system-generated |
| service_id | uuid | NO | — | FK → services.id |
| job_id | uuid | YES | — | FK → jobs.id; only for job reports |
| url | varchar(2000) | YES | — | S3 presigned URL |
| status | varchar(255) | NO | — | `requested`, `generating`, `ready`, `error` |
| language | varchar(2) | YES | — | `en` or `fr` |

**Indexes**: `ix_reports_service_id` btree(service_id)  
**Foreign Keys**:
- `requesting_user_id` → `users.id`
- `service_id` → `services.id`
- `job_id` → `jobs.id`

---

### scheduled_notifications
**Purpose**: Holds a future-delivery schedule entry for a notification that was created with a `scheduled_for` time.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| notification_id | uuid | NO | — | FK → notifications.id |
| scheduled_for | timestamp | NO | — | |
| pending | boolean | NO | — | true until Celery picks it up |

**Indexes**: `ix_scheduled_notifications_notification_id` btree(notification_id)  
**Foreign Keys**:
- `notification_id` → `notifications.id`

**Notes**: One-to-one with `notifications`; bidirectional via SQLAlchemy `back_populates`.

---

### service_callback_api
**Purpose**: Webhook configuration for a service to receive real-time delivery-status or complaint callbacks.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| service_id | uuid | NO | — | FK → services.id |
| url | varchar | NO | — | target URL |
| bearer_token | varchar | NO | — | encrypted at application layer |
| created_at | timestamp | NO | — | |
| updated_at | timestamp | YES | — | |
| updated_by_id | uuid | NO | — | FK → users.id |
| version | integer | NO | — | ORM versioning |
| callback_type | varchar | YES | — | FK → service_callback_type.name |
| is_suspended | boolean | YES | — | |
| suspended_at | timestamp | YES | — | set when suspended, retained on re-activation |

**Indexes**:
- `ix_service_callback_api_service_id` btree(service_id)
- `ix_service_callback_api_updated_by_id` btree(updated_by_id)

**Foreign Keys**:
- `service_id` → `services.id`
- `updated_by_id` → `users.id`
- `callback_type` → `service_callback_type.name`

**Constraints**: UNIQUE(`service_id`, `callback_type`) — `uix_service_callback_type`  
**Notes**: Versioned (mirrored to `service_callback_api_history`). `bearer_token` stored encrypted via `signer_bearer_token`.

---

### service_callback_api_history
**Purpose**: Immutable audit log of all versions of each service callback configuration.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK (composite with version) |
| service_id | uuid | NO | — | |
| url | varchar | NO | — | |
| bearer_token | varchar | NO | — | encrypted |
| created_at | timestamp | NO | — | |
| updated_at | timestamp | YES | — | |
| updated_by_id | uuid | NO | — | |
| version | integer | NO | — | PK (composite with id) |
| callback_type | varchar | YES | — | |
| is_suspended | boolean | YES | — | |
| suspended_at | timestamp | YES | — | |

**Indexes**:
- `ix_service_callback_api_history_service_id` btree(service_id)
- `ix_service_callback_api_history_updated_by_id` btree(updated_by_id)

**Notes**: History table pattern. PK is `(id, version)`.

---

### service_callback_type
**Purpose**: Lookup table of valid callback event types a service can subscribe to.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| name | varchar | NO | — | PK |

**Notes**: Enum lookup. Known values: `delivery_status`, `complaint`.

---

### service_data_retention
**Purpose**: Per-service override of how many days notifications of a given type are retained before purge.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| service_id | uuid | NO | — | FK → services.id |
| notification_type | notification_type (enum) | NO | — | |
| days_of_retention | integer | NO | — | |
| created_at | timestamp | NO | — | |
| updated_at | timestamp | YES | — | |

**Indexes**: `ix_service_data_retention_service_id` btree(service_id)  
**Foreign Keys**:
- `service_id` → `services.id`

**Constraints**: UNIQUE(`service_id`, `notification_type`) — `uix_service_data_retention`

---

### service_email_branding
**Purpose**: Association table linking a service to its email branding (one-to-one at the service level).  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| service_id | uuid | NO | — | PK (also enforces one-per-service) |
| email_branding_id | uuid | NO | — | FK → email_branding.id |

**Foreign Keys**:
- `service_id` → `services.id`
- `email_branding_id` → `email_branding.id`

**Notes**: PK on `service_id` enforces max one email branding per service. Used as SQLAlchemy `secondary` table.

---

### service_email_reply_to
**Purpose**: Stores reply-to email addresses configured for a service; one address is designated as default.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| service_id | uuid | NO | — | FK → services.id |
| email_address | text | NO | — | |
| is_default | boolean | NO | — | |
| created_at | timestamp | NO | — | |
| updated_at | timestamp | YES | — | |
| archived | boolean | NO | false | soft-delete |

**Indexes**: `ix_service_email_reply_to_service_id` btree(service_id)  
**Foreign Keys**:
- `service_id` → `services.id`

**Notes**: Soft-delete via `archived`. Accessible via `Service.reply_to_email_addresses` backref.

---

### service_inbound_api
**Purpose**: Webhook endpoint configuration for a service to receive inbound SMS payloads.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| service_id | uuid | NO | — | FK → services.id; UNIQUE (one per service) |
| url | varchar | NO | — | |
| bearer_token | varchar | NO | — | encrypted |
| created_at | timestamp | NO | — | |
| updated_at | timestamp | YES | — | |
| updated_by_id | uuid | NO | — | FK → users.id |
| version | integer | NO | — | ORM versioning |

**Indexes**:
- `ix_service_inbound_api_service_id` UNIQUE btree(service_id)
- `ix_service_inbound_api_updated_by_id` btree(updated_by_id)

**Foreign Keys**:
- `service_id` → `services.id`
- `updated_by_id` → `users.id`

**Notes**: Versioned (mirrored to `service_inbound_api_history`). UNIQUE on `service_id` enforces one inbound API per service.

---

### service_inbound_api_history
**Purpose**: Immutable audit log of all versions of each service inbound API configuration.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK (composite with version) |
| service_id | uuid | NO | — | |
| url | varchar | NO | — | |
| bearer_token | varchar | NO | — | encrypted |
| created_at | timestamp | NO | — | |
| updated_at | timestamp | YES | — | |
| updated_by_id | uuid | NO | — | |
| version | integer | NO | — | PK (composite with id) |

**Indexes**:
- `ix_service_inbound_api_history_service_id` btree(service_id)
- `ix_service_inbound_api_history_updated_by_id` btree(updated_by_id)

**Notes**: History table pattern. PK is `(id, version)`.

---

### service_letter_branding
**Purpose**: Association table linking a service to its letter branding (one-to-one at the service level).  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| service_id | uuid | NO | — | PK |
| letter_branding_id | uuid | NO | — | FK → letter_branding.id |

**Foreign Keys**:
- `service_id` → `services.id`
- `letter_branding_id` → `letter_branding.id`

**Notes**: PK on `service_id` enforces max one letter branding per service. Used as SQLAlchemy `secondary` table.

---

### service_letter_contacts
**Purpose**: Stores letter sender (contact block) addresses for a service; one is designated as default.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| service_id | uuid | NO | — | FK → services.id |
| contact_block | text | NO | — | multi-line address block for letter header |
| is_default | boolean | NO | — | |
| created_at | timestamp | NO | — | |
| updated_at | timestamp | YES | — | |
| archived | boolean | NO | false | soft-delete |

**Indexes**: `ix_service_letter_contacts_service_id` btree(service_id)  
**Foreign Keys**:
- `service_id` → `services.id`

**Notes**: Also FK source from `templates.service_letter_contact_id`.

---

### service_permission_types
**Purpose**: Lookup table of capabilities that can be enabled on a service.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| name | varchar(255) | NO | — | PK |

**Notes**: Enum lookup. Known values: `email`, `sms`, `letter`, `international_sms`, `inbound_sms`, `schedule_notifications`, `email_auth`, `letters_as_pdf`, `upload_document`, `edit_folder_permissions`, `upload_letters`.

---

### service_permissions
**Purpose**: Many-to-many join recording which capabilities are enabled for each service.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| service_id | uuid | NO | — | PK (composite) |
| permission | varchar(255) | NO | — | PK (composite); FK → service_permission_types.name |
| created_at | timestamp | NO | — | |

**Indexes**:
- `ix_service_permissions_service_id` btree(service_id)
- `ix_service_permissions_permission` btree(permission)

**Foreign Keys**:
- `service_id` → `services.id`
- `permission` → `service_permission_types.name`

---

### service_safelist
**Purpose**: Allowlist of phone numbers or email addresses that can receive notifications from a service operating in trial (restricted) mode.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| service_id | uuid | NO | — | FK → services.id |
| recipient_type | recipient_type (enum) | NO | — | `mobile` or `email` |
| recipient | varchar(255) | NO | — | phone number or email address |
| created_at | timestamp | YES | — | |

**Indexes**: `ix_service_whitelist_service_id` btree(service_id)  
**Foreign Keys**:
- `service_id` → `services.id`

---

### service_sms_senders
**Purpose**: SMS sender IDs (alphanumeric or numeric) configured for a service; one is designated as default.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| sms_sender | varchar(11) | NO | — | |
| service_id | uuid | NO | — | FK → services.id |
| is_default | boolean | NO | — | |
| inbound_number_id | uuid | YES | — | FK → inbound_numbers.id; UNIQUE |
| created_at | timestamp | NO | — | |
| updated_at | timestamp | YES | — | |
| archived | boolean | NO | false | soft-delete |

**Indexes**:
- `ix_service_sms_senders_service_id` btree(service_id)
- `ix_service_sms_senders_inbound_number_id` UNIQUE btree(inbound_number_id)

**Foreign Keys**:
- `service_id` → `services.id`
- `inbound_number_id` → `inbound_numbers.id`

**Notes**: UNIQUE on `inbound_number_id` ensures a given inbound number is linked to at most one SMS sender.

---

### services
**Purpose**: Core service entity — a government team's sending account with limits, branding, and configuration.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| name | varchar(255) | NO | — | UNIQUE |
| created_at | timestamp | NO | — | |
| updated_at | timestamp | YES | — | |
| active | boolean | NO | — | |
| message_limit | bigint | NO | — | daily notification limit |
| restricted | boolean | NO | — | true = trial mode |
| email_from | text | NO | — | UNIQUE; the `From` address |
| created_by_id | uuid | NO | — | FK → users.id |
| version | integer | NO | — | ORM versioning |
| research_mode | boolean | NO | — | notifications not actually sent |
| organisation_type | varchar(255) | YES | — | FK → organisation_types.name |
| prefix_sms | boolean | NO | — | prepend service name to SMS |
| crown | boolean | YES | — | |
| rate_limit | integer | NO | 1000 | notifications per minute |
| contact_link | varchar(255) | YES | — | |
| consent_to_research | boolean | YES | — | |
| volume_email | integer | YES | — | anticipated email volume |
| volume_letter | integer | YES | — | anticipated letter volume |
| volume_sms | integer | YES | — | anticipated SMS volume |
| count_as_live | boolean | NO | true | counted in live service stats |
| go_live_at | timestamp | YES | — | |
| go_live_user_id | uuid | YES | — | FK → users.id |
| organisation_id | uuid | YES | — | FK → organisation.id |
| sending_domain | text | YES | — | email sending domain |
| default_branding_is_french | boolean | YES | false | |
| sms_daily_limit | bigint | NO | — | daily SMS limit |
| organisation_notes | varchar | YES | — | |
| sensitive_service | boolean | YES | — | |
| email_annual_limit | bigint | NO | 20000000 | |
| sms_annual_limit | bigint | NO | 100000 | |
| suspended_by_id | uuid | YES | — | FK → users.id |
| suspended_at | timestamp | YES | — | |

**Indexes**:
- `ix_services_created_by_id` btree(created_by_id)
- `ix_services_organisation_id` btree(organisation_id)
- `ix_service_sensitive_service` btree(sensitive_service)

**Foreign Keys**:
- `created_by_id` → `users.id`
- `go_live_user_id` → `users.id`
- `organisation_id` → `organisation.id`
- `organisation_type` → `organisation_types.name`
- `suspended_by_id` → `users.id`

**Constraints**:
- UNIQUE(`name`) — `services_name_key`
- UNIQUE(`email_from`) — `services_email_from_key`

**Notes**: Versioned (mirrored to `services_history`). Central entity referenced by almost every other table.

---

### services_history
**Purpose**: Immutable audit log of all versions of each service configuration.  
**Columns**: Same as `services` minus any difference in defaults; schema is identical column-for-column.  

| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK (composite with version) |
| name | varchar(255) | NO | — | |
| created_at | timestamp | NO | — | |
| updated_at | timestamp | YES | — | |
| active | boolean | NO | — | |
| message_limit | bigint | NO | — | |
| restricted | boolean | NO | — | |
| email_from | text | NO | — | |
| created_by_id | uuid | NO | — | |
| version | integer | NO | — | PK (composite with id) |
| research_mode | boolean | NO | — | |
| organisation_type | varchar(255) | YES | — | |
| prefix_sms | boolean | YES | — | nullable in history |
| crown | boolean | YES | — | |
| rate_limit | integer | NO | 1000 | |
| contact_link | varchar(255) | YES | — | |
| consent_to_research | boolean | YES | — | |
| volume_email | integer | YES | — | |
| volume_letter | integer | YES | — | |
| volume_sms | integer | YES | — | |
| count_as_live | boolean | NO | true | |
| go_live_at | timestamp | YES | — | |
| go_live_user_id | uuid | YES | — | |
| organisation_id | uuid | YES | — | |
| sending_domain | text | YES | — | |
| default_branding_is_french | boolean | YES | false | |
| sms_daily_limit | bigint | NO | — | |
| organisation_notes | varchar | YES | — | |
| sensitive_service | boolean | YES | — | |
| email_annual_limit | bigint | NO | 20000000 | |
| sms_annual_limit | bigint | NO | 100000 | |
| suspended_by_id | uuid | YES | — | FK → users.id |
| suspended_at | timestamp | YES | — | |

**Indexes**:
- `ix_services_history_created_by_id` btree(created_by_id)
- `ix_services_history_organisation_id` btree(organisation_id)
- `ix_service_history_sensitive_service` btree(sensitive_service)

**Foreign Keys**:
- `suspended_by_id` → `users.id`

**Notes**: History table pattern. PK is `(id, version)`. No FK constraints on other columns. **Schema discrepancy**: `services_history.prefix_sms` is nullable (`YES`), whereas `services.prefix_sms` carries a `NOT NULL` constraint. History rows pre-dating the constraint addition may contain null values; Go must treat null as `false` when reading history rows.

---

### template_categories
**Purpose**: Categorises templates with bilingual names and per-category processing priority / SMS sending vehicle defaults.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| name_en | varchar(255) | NO | — | UNIQUE |
| name_fr | varchar(255) | NO | — | UNIQUE |
| description_en | varchar(255) | YES | — | |
| description_fr | varchar(255) | YES | — | |
| sms_process_type | varchar(255) | NO | — | default process type for SMS templates in this category |
| email_process_type | varchar(255) | NO | — | default process type for email templates |
| hidden | boolean | NO | — | |
| created_at | timestamp | NO | `now()` | |
| updated_at | timestamp | YES | `now()` | |
| sms_sending_vehicle | sms_sending_vehicle (enum) | NO | `long_code` | |
| created_by_id | uuid | NO | — | FK → users.id |
| updated_by_id | uuid | YES | — | FK → users.id |

**Indexes**:
- `ix_template_categories_name_en` btree(name_en)
- `ix_template_categories_name_fr` btree(name_fr)
- `ix_template_categories_created_by_id` btree(created_by_id)
- `ix_template_categories_updated_by_id` btree(updated_by_id)

**Foreign Keys**:
- `created_by_id` → `users.id`
- `updated_by_id` → `users.id`

**Constraints**: UNIQUE(`name_en`), UNIQUE(`name_fr`)  
**Notes**: Templates reference this table for their default `process_type`; the `process_type` column on a template overrides the category default.

---

### template_folder
**Purpose**: Hierarchical folder structure for organising templates within a service.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| service_id | uuid | NO | — | FK → services.id |
| name | varchar | NO | — | |
| parent_id | uuid | YES | — | FK → template_folder.id (self-referential) |

**Indexes**: none beyond PK  
**Foreign Keys**:
- `service_id` → `services.id`
- `parent_id` → `template_folder.id`

**Constraints**: UNIQUE(`id`, `service_id`) — `ix_id_service_id` (used as composite FK target from `user_folder_permissions`)  
**Notes**: Self-referential hierarchy. Users' access to folders recorded in `user_folder_permissions`.

---

### template_folder_map
**Purpose**: Associates a template with the single folder it resides in (one-to-one at the template level).  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| template_id | uuid | NO | — | PK |
| template_folder_id | uuid | NO | — | FK → template_folder.id |

**Foreign Keys**:
- `template_id` → `templates.id`
- `template_folder_id` → `template_folder.id`

**Notes**: PK on `template_id` enforces one-folder-per-template constraint. Used as SQLAlchemy `secondary` table.

---

### template_process_type
**Purpose**: Lookup table of Celery queue processing priority levels for templates.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| name | varchar(255) | NO | — | PK |

**Notes**: Enum lookup. Known values (in priority order): `bulk`, `normal`, `priority`, `low`, `medium`, `high`.

---

### template_redacted
**Purpose**: One-to-one flag controlling whether personalisation data is stripped from notification history for a template.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| template_id | uuid | NO | — | PK; FK → templates.id |
| redact_personalisation | boolean | NO | — | |
| updated_at | timestamp | NO | — | |
| updated_by_id | uuid | NO | — | FK → users.id |

**Indexes**: `ix_template_redacted_updated_by_id` btree(updated_by_id)  
**Foreign Keys**:
- `template_id` → `templates.id`
- `updated_by_id` → `users.id`

---

### templates
**Purpose**: Current version of a notification template (email, SMS, or letter) belonging to a service.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| name | varchar(255) | NO | — | |
| template_type | template_type (enum) | NO | — | `email`, `sms`, `letter` |
| created_at | timestamp | NO | — | |
| updated_at | timestamp | YES | — | |
| content | text | NO | — | Notify template markup |
| service_id | uuid | NO | — | FK → services.id |
| subject | text | YES | — | email/letter subject |
| created_by_id | uuid | NO | — | FK → users.id |
| version | integer | NO | — | ORM version counter |
| archived | boolean | NO | — | soft-delete |
| process_type | varchar(255) | YES | — | FK → template_process_type.name; overrides category default |
| service_letter_contact_id | uuid | YES | — | FK → service_letter_contacts.id |
| hidden | boolean | NO | — | hide from listing |
| postage | varchar | YES | — | `first` or `second`; letters only |
| template_category_id | uuid | YES | — | FK → template_categories.id |
| text_direction_rtl | boolean | NO | false | render RTL |

**Indexes**:
- `ix_templates_service_id` btree(service_id)
- `ix_templates_created_by_id` btree(created_by_id)
- `ix_templates_process_type` btree(process_type)
- `ix_template_category_id` btree(template_category_id)

**Foreign Keys**:
- `service_id` → `services.id`
- `created_by_id` → `users.id`
- `process_type` → `template_process_type.name`
- `service_letter_contact_id` → `service_letter_contacts.id`
- `template_category_id` → `template_categories.id`

**Constraints**: `chk_templates_postage` — when `template_type = 'letter'`, `postage` must be `'first'` or `'second'`; otherwise `postage` must be NULL.  
**Notes**: Versioned (mirrored to `templates_history`). Folder membership in `template_folder_map`. Personalisation redaction flag in `template_redacted`.

---

### templates_history
**Purpose**: Immutable audit log of all versions of each template; also referenced directly by `notifications` and `notification_history` via composite FK.  
**Columns**: Identical to `templates`.

| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK (composite with version) |
| name | varchar(255) | NO | — | |
| template_type | template_type (enum) | NO | — | |
| created_at | timestamp | NO | — | |
| updated_at | timestamp | YES | — | |
| content | text | NO | — | |
| service_id | uuid | NO | — | FK → services.id |
| subject | text | YES | — | |
| created_by_id | uuid | NO | — | FK → users.id |
| version | integer | NO | — | PK (composite with id) |
| archived | boolean | NO | — | |
| process_type | varchar(255) | YES | — | FK → template_process_type.name |
| service_letter_contact_id | uuid | YES | — | FK → service_letter_contacts.id |
| hidden | boolean | NO | — | |
| postage | varchar | YES | — | |
| template_category_id | uuid | YES | — | |
| text_direction_rtl | boolean | NO | false | |

**Indexes**:
- `ix_templates_history_service_id` btree(service_id)
- `ix_templates_history_created_by_id` btree(created_by_id)
- `ix_templates_history_process_type` btree(process_type)

**Foreign Keys**:
- `service_id` → `services.id`
- `created_by_id` → `users.id`
- `process_type` → `template_process_type.name`
- `service_letter_contact_id` → `service_letter_contacts.id`

**Constraints**: `chk_templates_history_postage` — same postage rule as `templates`.  
**Notes**: PK is `(id, version)`. Target of composite FK from `notifications(template_id, template_version)` and `notification_history(template_id, template_version)`.

---

### user_folder_permissions
**Purpose**: Records which users have access to which template folders within a service.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| user_id | uuid | NO | — | PK (composite) |
| template_folder_id | uuid | NO | — | PK (composite); FK → template_folder.id |
| service_id | uuid | NO | — | PK (composite) |

**Foreign Keys**:
- `(user_id, service_id)` → `user_to_service(user_id, service_id)`
- `(template_folder_id, service_id)` → `template_folder(id, service_id)`

**Notes**: Three-column composite PK. Used as SQLAlchemy `secondary` table on `TemplateFolder.users`.

---

### user_to_organisation
**Purpose**: Many-to-many join linking users to organisations they are members of.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| user_id | uuid | YES | — | FK → users.id |
| organisation_id | uuid | YES | — | FK → organisation.id |

**Foreign Keys**:
- `user_id` → `users.id`
- `organisation_id` → `organisation.id`

**Constraints**: UNIQUE(`user_id`, `organisation_id`) — `uix_user_to_organisation`  
**Notes**: Used as SQLAlchemy `secondary` table on `User.organisations` / `Organisation.users`.

---

### user_to_service
**Purpose**: Many-to-many join linking users to services they are members of.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| user_id | uuid | YES | — | FK → users.id |
| service_id | uuid | YES | — | FK → services.id |

**Foreign Keys**:
- `user_id` → `users.id`
- `service_id` → `services.id`

**Constraints**: UNIQUE(`user_id`, `service_id`) — `uix_user_to_service`  
**Notes**: Used as SQLAlchemy `secondary` for `User.services` / `Service.users`. Also composite FK target from `user_folder_permissions`.

---

### users
**Purpose**: Platform user accounts including credentials, 2FA settings, and account state.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| name | varchar | NO | — | |
| email_address | varchar(255) | NO | — | UNIQUE |
| created_at | timestamp | NO | — | |
| updated_at | timestamp | YES | — | |
| _password | varchar | NO | — | bcrypt hash (column alias `_password`) |
| mobile_number | varchar | YES | — | required unless auth_type is email/security_key |
| password_changed_at | timestamp | NO | — | |
| logged_in_at | timestamp | YES | — | |
| failed_login_count | integer | NO | — | consecutive failures; reset on success |
| state | varchar | NO | — | `pending`, `active`, `inactive` |
| platform_admin | boolean | NO | — | global admin flag |
| current_session_id | uuid | YES | — | active session token |
| auth_type | varchar | NO | `sms_auth` | FK → auth_type.name; `sms_auth`, `email_auth`, `security_key_auth` |
| blocked | boolean | NO | false | hard block |
| additional_information | jsonb | YES | — | free-form metadata |
| password_expired | boolean | NO | false | forces password reset on next login |
| verified_phonenumber | boolean | YES | false | |
| default_editor_is_rte | boolean | NO | false | UI preference |

**Indexes**:
- `ix_users_email_address` UNIQUE btree(email_address)
- `ix_users_name` btree(name)
- `ix_users_auth_type` btree(auth_type)

**Foreign Keys**:
- `auth_type` → `auth_type.name`

**Constraints**: `ck_users_mobile_or_email_auth` — `mobile_number IS NOT NULL OR auth_type IN ('email_auth', 'security_key_auth')`  
**Notes**: `additional_information` is JSONB. Password stored as bcrypt hash; never readable via property. Has M2M to `services` and `organisations`.

---

### verify_codes
**Purpose**: Short-lived one-time codes (OTP) sent to a user for 2FA verification.  
**Columns**:
| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| id | uuid | NO | — | PK |
| user_id | uuid | NO | — | FK → users.id |
| _code | varchar | NO | — | bcrypt hash of the code |
| code_type | verify_code_types (enum) | NO | — | `email` or `sms` |
| expiry_datetime | timestamp | NO | — | |
| code_used | boolean | YES | false | true once consumed |
| created_at | timestamp | NO | — | |

**Indexes**: `ix_verify_codes_user_id` btree(user_id)  
**Foreign Keys**:
- `user_id` → `users.id`

**Notes**: Code is never stored plaintext; verified via `check_hash()`. Accessible via `User.verify_codes` backref (lazy dynamic).

---

## Relationships

### Services as the central tenant entity

`services` is the root tenant table. Nearly every domain table carries a `service_id` FK pointing back to it:

- **API keys**: `services` → `api_keys` (one service has many API keys; soft-deleted via `expiry_date`). `api_keys` → `api_keys_history` on every version bump.
- **Service capabilities**: `services` → `service_permissions` (M2M with `service_permission_types`), controlling which channels and features are enabled.
- **SMS senders**: `services` → `service_sms_senders`. Each SMS sender optionally links to an `inbound_number` via `service_sms_senders.inbound_number_id`.
- **Reply-to / sender addresses**: `services` → `service_email_reply_to`, `services` → `service_letter_contacts`.
- **Templates**: `services` → `templates` → `templates_history` (every `templates` write clones a row into `templates_history`).
- **Jobs**: `services` → `jobs` (bulk notification batches).
- **Notifications**: `services` → `notifications` and `services` → `notification_history`.
- **Branding**: `services` → `service_email_branding` (secondary/association) → `email_branding`; `services` → `service_letter_branding` → `letter_branding`.
- **Callbacks**: `services` → `service_callback_api` (delivery-status/complaint webhooks); `services` → `service_inbound_api` (inbound SMS webhook).
- **Safelist**: `services` → `service_safelist` (trial-mode allowlist).
- **Data retention**: `services` → `service_data_retention` (per-type retention overrides).

### Notifications chain

The primary flow is: API request → `notifications` (live) → archive → `notification_history` (long-term).

- `jobs` → `notifications`: bulk jobs spawn individual notification rows; `notifications.job_id` / `notifications.job_row_number` track provenance.
- `notifications.(template_id, template_version)` → `templates_history(id, version)`: notifications always reference the historic snapshot of the template at send time, not the live `templates` row.
- `notifications` → `scheduled_notifications`: a one-to-one row is inserted into `scheduled_notifications` when `scheduled_for` is set; Celery polls and clears `pending = true`.
- `notifications` → `notification_history`: rows are moved on a rolling retention schedule; `notification_history` drops `_personalisation`, `to`, `normalised_to`, `reply_to_text`, and `provider_response` for privacy.

### User membership

Users connect to services and organisations through explicit join tables:

- `users` → `user_to_service` → `services`: M2M membership. The join table is also a composite FK target for `user_folder_permissions`.
- `users` → `user_to_organisation` → `organisation`: M2M membership.
- `users` → `permissions`: fine-grained per-service (or platform-wide when `service_id IS NULL`) operation grants using the `permission_types` PG enum.

### Template hierarchy

Templates live inside a service and can be organised into folders:

- `services` → `templates`: a service owns many templates; `templates.archived = true` is the soft-delete.
- `templates` → `template_folder_map` → `template_folder`: each template may reside in one folder. `template_folder` is self-referential via `parent_id` for nesting.
- `template_folder` + `user_to_service` → `user_folder_permissions`: records per-user access to each folder within a service.
- `templates` → `template_redacted`: one-to-one flag; when `redact_personalisation = true` personalisation is not retained in `notification_history`.
- `templates` → `template_categories`: a template belongs to a category that supplies default `process_type` and `sms_sending_vehicle`; the template's own `process_type` column overrides the category default.

### Billing

Billing data is accumulated from notification activity and summarised into fact tables:

- `services` → `annual_billing`: one row per service per financial year recording the free SMS fragment allowance.
- `ft_billing`: denormalised fact table; populated by a nightly Celery task that aggregates `notification_history` rows into daily billing units per `(bst_date, template_id, service_id, provider, …)` composite key. No FK constraints.
- `ft_notification_status`: similar nightly aggregation of notification counts by status. No FK constraints.
- `rates` and `provider_rates`: time-series rate tables consumed by the billing aggregation tasks.
- `letter_rates`: per-sheet letter postage rates keyed by `(sheet_count, crown, post_class)` with date ranges.

### Callback chain

- `service_callback_api`: a service registers up to one webhook per `callback_type` (`delivery_status`, `complaint`). Versioned — each change appends a row to `service_callback_api_history`.
- `service_inbound_api`: a service registers at most one inbound-SMS webhook (UNIQUE on `service_id`). Versioned — `service_inbound_api_history`.
- Callbacks carry an encrypted `bearer_token`; the application layer signs/verifies via `signer_bearer_token`.

### Inbound SMS

- `inbound_numbers`: a pool of phone numbers; each is either unassigned (`service_id IS NULL`) or assigned to exactly one service (UNIQUE index on `service_id`).
- `inbound_numbers` → `service_sms_senders`: when a number is provisioned for a service, `service_sms_senders.inbound_number_id` links it; UNIQUE ensures one sender per number.
- `inbound_numbers` → `services`: direct FK for the assignment.
- `inbound_sms`: inbound messages received on a service's number are stored here with `service_id`, `notify_number`, and `user_number`.
- `service_inbound_api`: the webhook that Notify calls with inbound SMS payloads.

### Auth / Security

- `users` → `verify_codes`: OTP codes for 2FA; `_code` is stored as a bcrypt hash, consumed once (`code_used = true`), and expire at `expiry_datetime`.
- `users` → `fido2_keys`: registered WebAuthn credentials for hardware-key MFA; multiple keys per user.
- `users` → `fido2_sessions`: transient per-user challenge state during the WebAuthn ceremony (PK = `user_id`, one active session at a time).
- `users` → `login_events`: append-only audit log of login attempts with JSONB metadata.
- `users` → `api_keys` (via `created_by_id`): tracks who created each API key.
- `users` → `permissions`: per-service or platform-wide capability grants.

---

## ORM-Only Relationships

All `relationship()` and `backref` definitions found in `app/models.py` that describe inter-table associations:

| Source | Attribute | Target | Via |
|---|---|---|---|
| `User.services` | M2M | `Service` | secondary: `user_to_service` |
| `User.organisations` | M2M | `Organisation` | secondary: `user_to_organisation` |
| `Organisation.agreement_signed_by` | M2O | `User` | FK: `organisation.agreement_signed_by_id` |
| `Organisation.email_branding` | M2O (uselist=False) | `EmailBranding` | relationship (no DB FK on org side) |
| `Organisation.letter_branding` | M2O | `LetterBranding` | FK: `organisation.letter_branding_id` |
| `Organisation.domains` | O2M | `Domain` | FK: `domain.organisation_id` |
| `Organisation.services` (backref) | O2M | `Service` | FK: `services.organisation_id` |
| `Organisation.users` (backref) | M2M | `User` | secondary: `user_to_organisation` |
| `EmailBranding.organisation` | M2O | `Organisation` | FK: `email_branding.organisation_id` |
| `EmailBranding.created_by` | M2O | `User` | FK: `email_branding.created_by_id` |
| `EmailBranding.updated_by` | M2O | `User` | FK: `email_branding.updated_by_id` |
| `Service.created_by` | M2O | `User` | FK: `services.created_by_id` |
| `Service.go_live_user` | M2O | `User` | FK: `services.go_live_user_id` |
| `Service.organisation` | M2O | `Organisation` | FK: `services.organisation_id` |
| `Service.email_branding` | M2M→O2O (uselist=False) | `EmailBranding` | secondary: `service_email_branding` |
| `Service.letter_branding` | M2M→O2O (uselist=False) | `LetterBranding` | secondary: `service_letter_branding` |
| `Service.users` (backref) | M2M | `User` | secondary: `user_to_service` |
| `Service.annual_billing` (backref) | O2M | `AnnualBilling` | FK: `annual_billing.service_id` |
| `Service.api_keys` (backref) | O2M | `ApiKey` | FK: `api_keys.service_id` |
| `Service.inbound_number` (backref, uselist=False) | O2O | `InboundNumber` | FK: `inbound_numbers.service_id` |
| `Service.service_sms_senders` (backref) | O2M | `ServiceSmsSender` | FK: `service_sms_senders.service_id` |
| `Service.permissions` (backref, cascade) | O2M | `ServicePermission` | FK: `service_permissions.service_id` |
| `Service.safelist` (backref) | O2M | `ServiceSafelist` | FK: `service_safelist.service_id` |
| `Service.inbound_api` (backref) | O2O | `ServiceInboundApi` | FK: `service_inbound_api.service_id` |
| `Service.service_callback_api` (backref) | O2M | `ServiceCallbackApi` | FK: `service_callback_api.service_id` |
| `Service.reply_to_email_addresses` (backref) | O2M | `ServiceEmailReplyTo` | FK: `service_email_reply_to.service_id` |
| `Service.letter_contacts` (backref) | O2M | `ServiceLetterContact` | FK: `service_letter_contacts.service_id` |
| `Service.all_template_folders` (backref) | O2M | `TemplateFolder` | FK: `template_folder.service_id` |
| `Service.templates` (backref) | O2M | `Template` | FK: `templates.service_id` |
| `Service.jobs` (backref, lazy=dynamic) | O2M | `Job` | FK: `jobs.service_id` |
| `Service.inbound_sms` (backref) | O2M | `InboundSms` | FK: `inbound_sms.service_id` |
| `Service.service_data_retention` (backref) | O2M | `ServiceDataRetention` | FK: `service_data_retention.service_id` |
| `AnnualBilling.service` | M2O | `Service` | FK: `annual_billing.service_id` |
| `InboundNumber.service` | M2O | `Service` | FK: `inbound_numbers.service_id` |
| `ServiceSmsSender.service` | M2O | `Service` | FK: `service_sms_senders.service_id` |
| `ServiceSmsSender.inbound_number` | M2O | `InboundNumber` | FK: `service_sms_senders.inbound_number_id` |
| `ServiceInboundApi.service` | M2O | `Service` | FK: `service_inbound_api.service_id` |
| `ServiceInboundApi.updated_by` | M2O | `User` | FK: `service_inbound_api.updated_by_id` |
| `ServiceCallbackApi.service` | M2O | `Service` | FK: `service_callback_api.service_id` |
| `ServiceCallbackApi.updated_by` | M2O | `User` | FK: `service_callback_api.updated_by_id` |
| `ServiceSafelist.service` | M2O | `Service` | FK: `service_safelist.service_id` |
| `ServiceEmailReplyTo.service` | M2O | `Service` | FK: `service_email_reply_to.service_id` |
| `ServiceLetterContact.service` | M2O | `Service` | FK: `service_letter_contacts.service_id` |
| `ServiceDataRetention.service` | M2O | `Service` | FK: `service_data_retention.service_id` |
| `TemplateCategory.created_by` | M2O | `User` | FK: `template_categories.created_by_id` |
| `TemplateCategory.updated_by` | M2O | `User` | FK: `template_categories.updated_by_id` |
| `TemplateFolder.service` | M2O | `Service` | FK: `template_folder.service_id` |
| `TemplateFolder.parent` | self-ref M2O | `TemplateFolder` | FK: `template_folder.parent_id` |
| `TemplateFolder.subfolders` (backref) | O2M | `TemplateFolder` | FK: `template_folder.parent_id` |
| `TemplateFolder.users` | M2M | `ServiceUser` | secondary: `user_folder_permissions` |
| `TemplateFolder.templates` (backref) | M2M | `Template` | secondary: `template_folder_map` |
| `Template.service` (backref) | M2O | `Service` | FK: `templates.service_id` |
| `Template.folder` | M2M→O2O (uselist=False, lazy=joined) | `TemplateFolder` | secondary: `template_folder_map` |
| `Template.created_by` | M2O | `User` | FK: `templates.created_by_id` |
| `Template.template_category` | M2O | `TemplateCategory` | FK: `templates.template_category_id` |
| `Template.service_letter_contact` | M2O (viewonly) | `ServiceLetterContact` | FK: `templates.service_letter_contact_id` |
| `Template.jobs` (backref, lazy=dynamic) | O2M | `Job` | FK: `jobs.template_id` |
| `TemplateRedacted.template` | O2O (uselist=False) | `Template` | FK: `template_redacted.template_id` |
| `Template.template_redacted` (backref, uselist=False) | O2O | `TemplateRedacted` | FK: `template_redacted.template_id` |
| `TemplateRedacted.updated_by` | M2O | `User` | FK: `template_redacted.updated_by_id` |
| `TemplateHistory.service` | M2O | `Service` | FK: `templates_history.service_id` |
| `TemplateHistory.template_category` | M2O | `TemplateCategory` | FK: `templates_history.template_category_id` |
| `ProviderDetails.created_by` | M2O | `User` | FK: `provider_details.created_by_id` |
| `ProviderDetails.provider_rates` (backref, lazy=dynamic) | O2M | `ProviderRates` | FK: `provider_rates.provider_id` |
| `ProviderRates.provider` | M2O | `ProviderDetails` | FK: `provider_rates.provider_id` |
| `ProviderDetailsHistory.created_by` | M2O | `User` | FK: `provider_details_history.created_by_id` |
| `Job.service` | M2O (backref lazy=dynamic) | `Service` | FK: `jobs.service_id` |
| `Job.template` | M2O (backref lazy=dynamic) | `Template` | FK: `jobs.template_id` |
| `Job.created_by` | M2O | `User` | FK: `jobs.created_by_id` |
| `Job.api_key` | M2O | `ApiKey` | FK: `jobs.api_key_id` |
| `Notification.job` | M2O (backref lazy=dynamic) | `Job` | FK: `notifications.job_id` |
| `Notification.service` | M2O | `Service` | FK: `notifications.service_id` |
| `Notification.template` | M2O | `TemplateHistory` | composite FK: `(template_id, template_version)` |
| `Notification.api_key` | M2O | `ApiKey` | FK: `notifications.api_key_id` |
| `Notification.created_by` | M2O | `User` | FK: `notifications.created_by_id` |
| `Notification.scheduled_notification` | O2O (uselist=False) | `ScheduledNotification` | back_populates |
| `ScheduledNotification.notification` | O2O (uselist=False) | `Notification` | back_populates |
| `NotificationHistory.job` | M2O | `Job` | FK: `notification_history.job_id` |
| `NotificationHistory.service` | M2O | `Service` | FK: `notification_history.service_id` |
| `NotificationHistory.api_key` | M2O | `ApiKey` | FK: `notification_history.api_key_id` |
| `Permission.service` | M2O | `Service` | FK: `permissions.service_id` |
| `Permission.user` | M2O | `User` | FK: `permissions.user_id` |
| `ApiKey.service` (backref) | M2O | `Service` | FK: `api_keys.service_id` |
| `ApiKey.created_by` | M2O | `User` | FK: `api_keys.created_by_id` |
| `VerifyCode.user` | M2O | `User` | FK: `verify_codes.user_id` |
| `User.verify_codes` (backref, lazy=dynamic) | O2M | `VerifyCode` | FK: `verify_codes.user_id` |
| `Report.requesting_user` | M2O | `User` | FK: `reports.requesting_user_id` |
| `InboundSms.service` (backref) | M2O | `Service` | FK: `inbound_sms.service_id` |
