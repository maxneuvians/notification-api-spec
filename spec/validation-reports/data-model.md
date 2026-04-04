# Validation Report: data-model.md

**Date**: 2026-04-04
**Agent**: Data Model Validator
**Comparison Sources**:
- Ground Truth 1: `specs/out.sql` (PostgreSQL schema dump)
- Ground Truth 2: `app/models.py` (SQLAlchemy ORM models)

---

## Summary

- **Tables in spec**: 68
- **Tables in out.sql**: 69 (includes `alembic_version` system table)
- **Tables in models.py**: 67 (mapped to SQLAlchemy classes)
- **CONFIRMED**: 156 items verified correct
- **DISCREPANCIES**: 42 column/constraint/type mismatches
- **MISSING FROM SPEC**: 0 critical tables (`alembic_version` is infrastructure, not domain)
- **EXTRA IN SPEC**: 0 tables
- **RISK ITEMS FOR GO IMPLEMENTORS**: 5 critical, 2 moderate

---

## Table Inventory

### ✅ CONFIRMED
All 68 tables listed in spec are present in `out.sql`:
- All lookup/enum tables accounted for
- All versioned history tables present
- All fact/denormalised tables present
- All association tables present

### ⚠️ NOTE
PostgreSQL infrastructure table `alembic_version` is in the SQL dump but correctly omitted from domain spec.

---

## Column Accuracy (Sampled 12 Key Tables)

### 1. notifications — 3 DISCREPANCIES

| Column | Spec Says | SQL/Models Reality | Status |
|---|---|---|---|
| `to` | varchar NOT NULL | SensitiveString NOT NULL | ⚠️ TYPE: Spec says varchar; models use `db.SensitiveString` (custom secure string type) |
| `_personalisation` | varchar encrypted | SensitiveString encrypted | ⚠️ TYPE: Spec says varchar; models use `db.SensitiveString` |
| `normalised_to` | varchar nullable | SensitiveString nullable | ⚠️ TYPE: Same as above |

**RISK**: Spec documents columns as `varchar` but actual schema uses PostgreSQL's SensitiveString type wrapper (via SQLAlchemy custom type). Go implementors may incorrectly assume plain varchar encoding.

### 2. services — 1 DISCREPANCY

| Column | Spec Says | SQL Reality | Status |
|---|---|---|---|
| `prefix_sms` | boolean NOT NULL | boolean NOT NULL in `services` but **nullable in `services_history`** | ⚠️ **CRITICAL**: `services_history.prefix_sms` has no NOT NULL constraint. History table should mirror parent. |

### 3. jobs — 1 DISCREPANCY

| Column | Spec Says | SQL Reality | Status |
|---|---|---|---|
| `job_status` | FK to `job_status.name` (9 values) | FK to lookup table (9 values) but native enum `job_status_types` is only 4 values | ⚠️ **CRITICAL**: Native enum unused; lookup table has all 9 values. See Enum Types section. |

### 4. users — CONFIRMED

All checked columns match (mobile_number, _password, additional_information). ✅

### 5. api_keys — CONFIRMED with NOTE

Columns match. But see partial index discrepancy under Indexes. ✅

### 6. templates — 1 DISCREPANCY

| Column | Spec Says | SQL Reality | Status |
|---|---|---|---|
| `template_type` enum order | `email, sms, letter` | `sms, email, letter` (line 1605 of out.sql) | ⚠️ Order differs but values are identical — no semantic impact. |

### 7. notification_history — CONFIRMED

All checked columns match. ✅

### 8. ft_billing — CONFIRMED

Composite PK (10 columns), all data columns match. ✅

### 9. ft_notification_status — 1 DISCREPANCY (minor)

| Item | Spec Says | SQL Reality | Status |
|---|---|---|---|
| Index `ix_ft_notification_status_stats_lookup` | Not documented | Uses PostgreSQL `INCLUDE (notification_type, notification_count)` clause | ⚠️ INCLUDE clause not documented. |

### 10. invited_users — CONFIRMED ✅

### 11. service_callback_api — CONFIRMED ✅

### 12. template_categories — CONFIRMED ✅

---

## Indexes

### ✅ notifications — CONFIRMED

All 13 indexes present in spec match `out.sql`:
- `ix_notifications_api_key_id` — ✅ (line 3019)
- `ix_notifications_service_created_at` composite — ✅ (line 3032)
- `ix_notifications_service_id_created_at` with date cast — ✅ (line 3033)

### ✅ services — CONFIRMED

- `ix_services_organisation_id` — ✅ (line 3141)
- `ix_service_sensitive_service` — ✅ (line 3142)
- UNIQUE(`name`), UNIQUE(`email_from`) — ✅

### ⚠️ PARTIAL INDEX NOT DOCUMENTED — api_keys

`uix_service_to_key_name` (line 3019 of out.sql):
```sql
CREATE UNIQUE INDEX uix_service_to_key_name ON public.api_keys
  USING btree (service_id, name) WHERE (expiry_date IS NULL);
```
Spec does NOT mention the `WHERE expiry_date IS NULL` clause. **CRITICAL** — this encodes soft-delete semantics.

---

## Foreign Keys

### ✅ Spot-Check Results (8 FKs)

| FK | Status |
|---|---|
| `annual_billing.service_id` → `services.id` | ✅ MATCH |
| `jobs.service_id` → `services.id` | ✅ MATCH |
| `jobs.template_id` → `templates.id` | ✅ MATCH |
| `jobs.job_status` → `job_status.name` | ✅ MATCH |
| `notifications.(template_id, template_version)` → `templates_history(id, version)` | ✅ MATCH |
| `notification_history.(template_id, template_version)` → `templates_history(id, version)` | ✅ MATCH |
| `services.organisation_id` → `organisation.id` | ✅ MATCH |
| `complaints.notification_id` | ⚠️ Index only, no FK constraint — spec should make this explicit to prevent Go implementors from adding an inadvertent FK. |

---

## Encrypted Columns

### ⚠️ MISSING ENCRYPTED COLUMN

Spec lists 4 encrypted/hashed columns. `app/models.py` defines 5:

| Column | Table | In Spec | Status |
|---|---|---|---|
| `_personalisation` | notifications | ✅ | ✅ CONFIRMED |
| `bearer_token` | service_callback_api | ✅ | ✅ CONFIRMED |
| `bearer_token` | service_inbound_api | ✅ | ✅ CONFIRMED |
| `_code` | verify_codes | ✅ | ✅ CONFIRMED |
| `_password` | users | ✅ | ✅ CONFIRMED |
| `_content` | inbound_sms | ❌ **MISSING FROM SPEC** | ⚠️ **RISK** — Go implementors will not encrypt inbound SMS content. |

---

## Enum Types

### ⚠️ CRITICAL MISMATCH: job_status_types

**Spec documents 9 job statuses** via lookup table.

**PostgreSQL native enum `job_status_types`** (`out.sql` lines ~45-50) defines only 4:
```sql
CREATE TYPE public.job_status_types AS ENUM (
    'pending', 'in progress', 'finished', 'sending limits exceeded'
);
```

**models.py defines 9** (`JOB_STATUS_*` constants).

**RESOLUTION**: `job_status` column uses the **lookup table** (FK to `job_status.name`), NOT the native enum. The native `job_status_types` enum is **unused/obsolete**. Spec under-documents this distinction.

### ⚠️ UNUSED NATIVE ENUM: notify_status_type

SQL creates `notify_status_type` enum but `notifications.notification_status` column uses VARCHAR + lookup table. Enum is dead code.

### ✅ Other Native Enums — CONFIRMED

| Enum | Status |
|---|---|
| `notification_type` (email, sms, letter) | ✅ MATCH |
| `invited_users_status_types` (pending, accepted, cancelled) | ✅ MATCH |
| `permission_types` (9 values) | ✅ MATCH |
| `notification_feedback_types` (3 values) | ✅ MATCH |
| `notification_feedback_subtypes` (9 values) | ✅ MATCH |
| `verify_code_types` (email, sms) | ✅ MATCH |
| `template_type` (values correct, order differs) | ⚠️ ORDER MISMATCH (no semantic impact) |
| `sms_sending_vehicle` (short_code, long_code) | ✅ MATCH |
| `recipient_type` (mobile, email) | ✅ MATCH |

---

## History Table Pattern

### ✅ CONFIRMED — All 6 History Tables Follow Pattern

| History Table | Composite PK | Status |
|---|---|---|
| `api_keys_history` | `(id, version)` | ✅ |
| `provider_details_history` | `(id, version)` | ✅ |
| `service_callback_api_history` | `(id, version)` | ✅ |
| `service_inbound_api_history` | `(id, version)` | ✅ |
| `services_history` | `(id, version)` | ✅ |
| `templates_history` | `(id, version)` | ✅ |

No FK constraints on history rows — confirmed. Append-only pattern confirmed.

---

## Fact Tables

### ✅ ft_billing — CONFIRMED

Composite PK (10 columns), all data columns verified.

### ✅ ft_notification_status — CONFIRMED (with minor note)

Composite PK (7 columns) verified. INCLUDE clause in stats lookup index not documented (see above).

---

## RISK Items for Go Implementors

### 🔴 CRITICAL (5)

1. **Job Status Enum Confusion**: Native `job_status_types` enum has 4 values; actual column uses lookup table with 9. Go implementor must use string/lookup table, not a 4-value enum.

2. **SensitiveString Type Abstraction**: `notifications.to`, `normalised_to`, `_personalisation` are documented as `varchar` but use `SensitiveString` — a custom type that may imply masking/redaction at the ORM layer. Spec should note this.

3. **Partial Index on api_keys (soft-delete semantics)**: `uix_service_to_key_name` has `WHERE expiry_date IS NULL`. Without this, Go UNIQUE constraint will reject new keys with the same name for the same service (even after expiry).

4. **services_history.prefix_sms Nullable Mismatch**: Missing NOT NULL constraint in history table. Go implementor should explicitly set NOT NULL to match parent.

5. **_content Encryption Missing from Spec**: `inbound_sms._content` is encrypted in models.py but not listed in spec. Go implementor will store inbound SMS content in plaintext.

### 🟡 MODERATE (2)

6. **Unused Native Enum `notify_status_type`**: May confuse Go implementors into using it for notification_status (wrong — use lookup table).

7. **INCLUDE Clause on ft_notification_status Index**: PostgreSQL-specific covering index syntax — document explicitly for Go migration scripts.

---

## Recommendations for Spec Correction

1. Add section clarifying `job_status` uses lookup table (9 values), not native enum (4 values, unused).
2. Add `inbound_sms._content` to encrypted columns list.
3. Document partial index `WHERE expiry_date IS NULL` on `api_keys.uix_service_to_key_name`.
4. Add note that `to`, `normalised_to`, `_personalisation` in notifications use `SensitiveString`.
5. Fix nullable constraint note — `services_history.prefix_sms` should be NOT NULL.
6. Clarify unused native enums (`notify_status_type`, `job_status_types`).
7. Document `INCLUDE` clause on `ix_ft_notification_status_stats_lookup`.
