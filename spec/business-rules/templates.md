# Business Rules: Templates

## Overview

The templates domain manages notification templates (SMS, email, letter) and their associated metadata.
Every template belongs to a service and has a type, content body, and an optional subject (email/letter).
Templates are versioned: every mutating operation writes a snapshot row to `templates_history`.
A `TemplateRedacted` record is created alongside every template and controls whether personalisation
placeholders are redacted from notification logs.  Template categories group templates for routing
and process-type resolution; template folders provide an optional hierarchical organisation layer
within a service.

---

## Data Access Patterns

### `templates_dao.py`

#### `dao_create_template(template, redact_personalisation=False)`
- **Purpose**: Inserts a new template and its associated `TemplateRedacted` record, then writes the first `TemplateHistory` snapshot.
- **Query type**: INSERT (`templates`, `template_redacted`, `templates_history`)
- **Key filters/conditions**: None (caller supplies a fully-populated `Template` object).
- **Returns**: `None` (the `template` object is mutated in-place: `template.id` is set before the insert so the history row can share it).
- **Notes**:
  - A UUID is assigned to `template.id` explicitly so the history row can use the same id before the parent row is flushed.
  - `TemplateRedacted` is created with `redact_personalisation=False` by default; the `updated_by` field is populated from `template.created_by` (or `created_by_id`).
  - The `@version_class(VersionOptions(Template, history_class=TemplateHistory))` decorator calls `create_history()` after the session add, which sets `version = 1` and `created_at = utcnow()` on the template object and writes a `TemplateHistory` row with that version.
  - Wrapped in `@transactional` (commits on success, rolls back on exception).

#### `dao_update_template(template)`
- **Purpose**: Persists mutations to an existing `Template` object and writes a new `TemplateHistory` snapshot.
- **Query type**: UPDATE (`templates`), INSERT (`templates_history`)
- **Key filters/conditions**: None (caller supplies a mutated `Template` object already loaded from the session).
- **Returns**: `None`.
- **Notes**:
  - The `@version_class` decorator's `create_history()` increments `template.version` and sets `template.updated_at = utcnow()` before writing the history row.
  - Wrapped in `@transactional`.

#### `dao_update_template_reply_to(template_id, reply_to)`
- **Purpose**: Updates the letter-contact (`service_letter_contact_id`) on a template and manually appends a history row.
- **Query type**: UPDATE (`templates`), INSERT (`templates_history`)
- **Key filters/conditions**: `templates.id = template_id`
- **Returns**: The updated `Template` object.
- **Notes**:
  - Increments `version` via `Template.version + 1` in the update expression and sets `updated_at = utcnow()`.
  - Does **not** use the `@version_class` decorator; the `TemplateHistory` row is constructed manually from the current template fields.
  - Wrapped in `@transactional`.

#### `dao_update_template_process_type(template_id, process_type)`
- **Purpose**: Overrides the explicit `process_type` column on a template and writes a history row.
- **Query type**: UPDATE (`templates`), INSERT (`templates_history`)
- **Key filters/conditions**: `templates.id = template_id`
- **Returns**: The updated `Template` object.
- **Notes**:
  - **Does not** increment `version` or set `updated_at` in the UPDATE expression (unlike `dao_update_template_reply_to` and `dao_update_template_category`). The history row therefore shares the same version number as the previous snapshot — this is an anomaly in the current implementation.
  - The `TemplateHistory` row is constructed manually (no `@version_class` decorator).
  - Wrapped in `@transactional`.

#### `dao_update_template_category(template_id, category_id)`
- **Purpose**: Changes the `template_category_id` on a template and appends a history row.
- **Query type**: UPDATE (`templates`), INSERT (`templates_history`)
- **Key filters/conditions**: `templates.id = template_id`
- **Returns**: The updated `Template` object.
- **Notes**:
  - Increments `version` via `Template.version + 1` and sets `updated_at = utcnow()` in the UPDATE expression.
  - The `TemplateHistory` row is constructed manually (no `@version_class` decorator).
  - Wrapped in `@transactional`.

#### `dao_redact_template(template, user_id)`
- **Purpose**: Enables personalisation redaction on a template by updating the `TemplateRedacted` record.
- **Query type**: UPDATE (`template_redacted`)
- **Key filters/conditions**: Operates on `template.template_redacted` (the relationship must already be loaded).
- **Returns**: `None`.
- **Notes**:
  - Sets `redact_personalisation = True`, `updated_at = utcnow()`, `updated_by_id = user_id`.
  - **Does not** bump the template's version; this is a metadata-only change on the `template_redacted` table.
  - Wrapped in `@transactional`.

#### `dao_get_template_by_id_and_service_id(template_id, service_id, version=None)`
- **Purpose**: Fetches a single template scoped to a service, optionally at a specific historical version.
- **Query type**: SELECT
- **Key filters/conditions**:
  - `id = template_id`, `service_id = service_id`, `hidden = False`
  - If `version` is provided: queries `TemplateHistory` filtered by `version`
  - If `version` is `None`: queries `Template` (current) via `db.on_reader()` (read replica)
- **Returns**: `TemplateHistory` (versioned) or `Template` (current).
- **Notes**: Uses `.one()` — raises if no result or multiple results.

#### `dao_get_template_by_id(template_id, version=None, use_cache=False)`
- **Purpose**: Fetches a template by id, with optional Redis cache and optional historical version.
- **Query type**: SELECT (or Redis cache read)
- **Key filters/conditions**:
  - If `use_cache=True`: checks Redis key `template_version_cache_key(template_id, version)` first; returns deserialized object if found (**not added to the SQLAlchemy session**).
  - If `version` is provided: queries `TemplateHistory` by `id` and `version`.
  - If `version` is `None`: queries `Template` by `id`.
- **Returns**: `Template` or `TemplateHistory`.
- **Notes**:
  - Cached objects are returned in the SQLAlchemy *transient* state (not session-attached). Fields populated only by lazy-loading (e.g., `reply_to_text`) will be absent.
  - No `hidden` or `service_id` filter — the caller is responsible for access checks.

#### `dao_get_all_templates_for_service(service_id, template_type=None)`
- **Purpose**: Lists all active, visible templates for a service.
- **Query type**: SELECT
- **Key filters/conditions**:
  - `service_id = service_id`, `hidden = False`, `archived = False`
  - Optionally filtered by `template_type`
  - Ordered by `name ASC`, `template_type ASC`
- **Returns**: `list[Template]`
- **Notes**:
  - Eager-loads `template_redacted`, `template_category`, `created_by`, and `service_letter_contact` to avoid N+1 queries during serialization.

#### `dao_get_template_versions(service_id, template_id)`
- **Purpose**: Returns the complete version history of a template.
- **Query type**: SELECT (`templates_history`)
- **Key filters/conditions**: `service_id = service_id`, `id = template_id`, `hidden = False`
- **Returns**: `list[TemplateHistory]` ordered by `version DESC` (newest first).

#### `get_precompiled_letter_template(service_id)`
- **Purpose**: Returns the singleton pre-compiled letter template for a service, creating it if it does not exist.
- **Query type**: SELECT then INSERT if not found
- **Key filters/conditions**: `service_id = service_id`, `template_type = 'letter'`, `hidden = True`
- **Returns**: `Template`
- **Notes**:
  - The pre-compiled template has `name = "Pre-compiled PDF"`, `hidden = True`, `content = ""`, `postage = "second"`, and is created under the system `NOTIFY_USER_ID`.
  - Uses `Template.query.filter_by(...).first()` (no exception on miss — falls through to create).

---

### `template_categories_dao.py`

#### `dao_create_template_category(template_category)`
- **Purpose**: Inserts a new `TemplateCategory`, assigning a UUID if none is set.
- **Query type**: INSERT (`template_categories`)
- **Key filters/conditions**: None.
- **Returns**: `None`.
- **Notes**: Wrapped in `@transactional`.

#### `dao_get_template_category_by_id(template_category_id)`
- **Purpose**: Fetches a single category by primary key.
- **Query type**: SELECT (`template_categories`)
- **Key filters/conditions**: `id = template_category_id`
- **Returns**: `TemplateCategory`
- **Notes**: Uses `.one()` — raises on miss.

#### `dao_get_template_category_by_template_id(template_id)`
- **Purpose**: Fetches the category associated with a given template.
- **Query type**: SELECT (`templates` → relationship `template_category`)
- **Key filters/conditions**: `templates.id = template_id`
- **Returns**: `TemplateCategory` (or `None` if the template has no category).
- **Notes**: Navigates the ORM relationship; triggers a secondary SELECT for the category.

#### `dao_get_all_template_categories(template_type=None, hidden=None)`
- **Purpose**: Lists template categories with optional filtering.
- **Query type**: SELECT (`template_categories`, optionally JOINed to `templates`)
- **Key filters/conditions**:
  - If `template_type` is provided: JOIN to `templates` and filter `template_type`.
  - If `hidden` is provided: filter `template_categories.hidden`.
- **Returns**: `list[TemplateCategory]`
- **Notes**: The `template_type` filter returns categories *used by at least one template of that type*. A TODO in the code notes this behaviour but it is the current implementation.

#### `dao_update_template_category(template_category)`
- **Purpose**: Persists mutations to an existing `TemplateCategory` object.
- **Query type**: UPDATE (`template_categories`)
- **Key filters/conditions**: None (ORM tracks changes via identity map).
- **Returns**: `None`.
- **Notes**: Calls `db.session.commit()` explicitly inside the function in addition to the `@transactional` wrapper (redundant double-commit — the second is a no-op).

#### `dao_delete_template_category_by_id(template_category_id, cascade=False)`
- **Purpose**: Deletes a `TemplateCategory`, with optional reassignment of associated templates.
- **Query type**: SELECT + DELETE (`template_categories`), conditionally UPDATE (`templates`)
- **Key filters/conditions**: `template_categories.id = template_category_id`
- **Returns**: `None`.
- **Notes**:
  - If `cascade=False` and there are templates using the category, raises `InvalidRequest(400)`.
  - If `cascade=True`: each associated template is reassigned to a default category whose process-type matches the deleted category's process-type for the template's type (SMS → `sms_process_type`, email → `email_process_type`). Each template update is committed individually inside the loop.
  - Default category mapping (from `_get_default_category_id`):
    - `"bulk"` → `DEFAULT_TEMPLATE_CATEGORY_LOW`
    - `"normal"` → `DEFAULT_TEMPLATE_CATEGORY_MEDIUM`
    - `"priority"` → `DEFAULT_TEMPLATE_CATEGORY_HIGH`
    - any other value → `DEFAULT_TEMPLATE_CATEGORY_LOW`
  - Wrapped in `@transactional`.

#### `_get_default_category_id(process_type)` *(private helper)*
- **Purpose**: Resolves a process-type string to the corresponding default-category config value.
- **Returns**: A category id from application config.

---

### `template_folder_dao.py`

#### `dao_get_template_folder_by_id_and_service_id(template_folder_id, service_id)`
- **Purpose**: Fetches a folder, scoped to a specific service.
- **Query type**: SELECT (`template_folder`)
- **Key filters/conditions**: `id = template_folder_id`, `service_id = service_id`
- **Returns**: `TemplateFolder`
- **Notes**: Uses `.one()` — raises on miss or cross-service access.

#### `dao_get_valid_template_folders_by_id(folder_ids)`
- **Purpose**: Fetches a set of folders by a list of ids (no service scope).
- **Query type**: SELECT (`template_folder`)
- **Key filters/conditions**: `id IN (folder_ids)`
- **Returns**: `list[TemplateFolder]` (only ids that exist are returned; missing ids are silently omitted).

#### `dao_create_template_folder(template_folder)`
- **Purpose**: Inserts a new folder.
- **Query type**: INSERT (`template_folder`)
- **Returns**: `None`.
- **Notes**: Wrapped in `@transactional`.

#### `dao_update_template_folder(template_folder)`
- **Purpose**: Persists mutations to an existing folder.
- **Query type**: UPDATE (`template_folder`)
- **Returns**: `None`.
- **Notes**: Wrapped in `@transactional`.

#### `dao_delete_template_folder(template_folder)`
- **Purpose**: Hard-deletes a folder.
- **Query type**: DELETE (`template_folder`)
- **Returns**: `None`.
- **Notes**: Wrapped in `@transactional`. The caller is expected to verify the folder is empty before calling this.

---

## Domain Rules & Invariants

### Template Versioning

- The `templates` table holds the **current** state of a template; `templates_history` holds every previous snapshot.
- `TemplateHistory` uses a composite primary key of `(id, version)`.
- When `dao_create_template` or `dao_update_template` is used, versioning is managed by the `@version_class` decorator which calls `create_history()`:
  - First save (version is falsy): sets `version = 1`, sets `created_at`.
  - Subsequent saves: increments `version`, sets `updated_at`.
- When specialized update functions (`dao_update_template_reply_to`, `dao_update_template_category`) are used, they manually increment `version` and set `updated_at` in the UPDATE statement and build the history row from template fields.
- `dao_update_template_process_type` is an anomaly: it writes a history row without incrementing `version`, so two snapshots may share the same version number.
- Hidden templates (`hidden = True`) are excluded from version-history queries in `dao_get_template_versions`.

### Template Categories

- A `TemplateCategory` carries **separate process types** for SMS and email (`sms_process_type`, `email_process_type`), allowing the same category to route SMS and email at different priorities.
- Categories have bilingual names and descriptions (`name_en`, `name_fr`, `description_en`, `description_fr`).
- Categories can be hidden (`hidden = True`); hidden categories are not returned by `dao_get_all_template_categories` when `hidden=False` is passed.
- A `TemplateCategory` also carries an `sms_sending_vehicle` field (e.g., `long_code`) which governs the SMS sending path.
- `template_category_id` is **nullable** on templates; a template may have no explicit category.

### Process Types (`bulk` / `normal` / `priority`)

- Valid values are defined by the `template_process_type` lookup table: `bulk`, `normal`, `priority`.
- A template's effective `process_type` is a **derived hybrid property**:
  1. If `template.process_type_column` (the stored column) is non-null, that value is used.
  2. Otherwise, the category's `sms_process_type` (for SMS templates) or `email_process_type` (for email templates) is used.
- Letter templates have no process-type resolution via category (the `else_` branch of the hybrid expression returns `process_type_column` directly).
- Process type drives which Celery queue a notification is routed to (bulk = low priority, priority = high priority).

### Template Redaction

- Every template has exactly one `TemplateRedacted` record (one-to-one, created by `dao_create_template`).
- `redact_personalisation = False` by default; set to `True` by `dao_redact_template`.
- Once set to `True`, personalisation values in notification bodies are replaced with placeholder names in logs — this is irreversible at the DAO layer (there is no `dao_unredact_template`).
- Redaction is a metadata change; it does **not** bump the template `version`.

### Template Types (`sms` / `email` / `letter`)

- Defined in `TEMPLATE_TYPES = ["sms", "email", "letter"]`.
- **Letter templates**: must have a non-null `postage` value of `"first"` or `"second"` (enforced by a `CheckConstraint`); non-letter templates must have `postage = NULL`.
- **Letter templates**: `reply_to` maps to `service_letter_contact_id`; setting a non-null `reply_to` on an SMS or email template raises `ValueError`.
- **Pre-compiled letter template**: a special singleton per service with `hidden = True`, `name = "Pre-compiled PDF"`, `content = ""`, `postage = "second"`. Created on first access via `get_precompiled_letter_template`.
- **Email/SMS templates**: `subject` is used for email; ignored for SMS.
- All types support `text_direction_rtl` for right-to-left language content.

### Folder Structure

- Folders belong to a service (`service_id` FK).
- Folders can be nested: `parent_id` is a self-referential FK (nullable; `NULL` means root-level folder).
- A template can belong to **at most one folder** (`template_id` is a primary key in the `template_folder_map` join table).
- Folders have per-user access permissions via the `user_folder_permissions` junction table (managed separately from the folder DAO).
- `TemplateFolder.is_parent_of(other)` walks the `parent` chain to determine ancestry.
- `dao_get_valid_template_folders_by_id` does not scope by service — the caller must enforce service scoping when needed.

### Archive / Hidden Flags

- `archived = True`: soft-delete; the template is excluded from `dao_get_all_templates_for_service` and most active-template queries.
- `hidden = True`: used exclusively for the pre-compiled letter template; excluded from `dao_get_template_by_id_and_service_id` and `dao_get_template_versions`.

---

## Error Conditions

| Condition | Raised by | Exception |
|---|---|---|
| Deleting a category that has associated templates when `cascade=False` | `dao_delete_template_category_by_id` | `InvalidRequest("Cannot delete categories associated with templates…", 400)` |
| Setting `reply_to` to a non-null value on a non-letter template | `TemplateBase.reply_to.setter` | `ValueError("Unable to set sender for {type} template")` |
| `@version_class` finds no new/dirty `Template` objects in the session (early flush) | `version_class` decorator in `dao_utils` | `RuntimeError("Can't record history for Template …")` |
| Any `.one()` query returns no row (template/folder/category not found) | All DAO `get` functions using `.one()` | `sqlalchemy.orm.exc.NoResultFound` |
| Any `.one()` query returns more than one row | All DAO `get` functions using `.one()` | `sqlalchemy.orm.exc.MultipleResultsFound` |

---

## Query Inventory (for sqlc)

| Query name | Type | Tables | Description |
|---|---|---|---|
| `InsertTemplate` | INSERT | `templates`, `template_redacted`, `templates_history` | Create a new template with its redacted record and initial history snapshot |
| `UpdateTemplate` | UPDATE + INSERT | `templates`, `templates_history` | General-purpose template update with auto-versioning |
| `UpdateTemplateReplyTo` | UPDATE + INSERT | `templates`, `templates_history` | Update `service_letter_contact_id`, increment version, write history |
| `UpdateTemplateProcessType` | UPDATE + INSERT | `templates`, `templates_history` | Update explicit `process_type` column, write history (no version increment) |
| `UpdateTemplateCategory` | UPDATE + INSERT | `templates`, `templates_history` | Update `template_category_id`, increment version, write history |
| `UpdateTemplateRedacted` | UPDATE | `template_redacted` | Set `redact_personalisation = true` |
| `GetTemplateBySvcAndId` | SELECT | `templates` | Fetch current template by id + service_id, exclude hidden |
| `GetTemplateHistoryBySvcAndVersion` | SELECT | `templates_history` | Fetch specific version by id + service_id, exclude hidden |
| `GetTemplateById` | SELECT | `templates` | Fetch current template by id only |
| `GetTemplateHistoryByIdAndVersion` | SELECT | `templates_history` | Fetch specific version by id + version |
| `GetTemplateFromCache` | CACHE + SELECT | `templates` / `templates_history` | Fetch template via Redis cache, falling back to DB |
| `ListTemplatesForService` | SELECT | `templates`, `template_redacted`, `template_categories`, `users`, `service_letter_contacts` | List active non-hidden non-archived templates for a service, optional type filter |
| `ListTemplateVersions` | SELECT | `templates_history` | All history rows for a template, ordered version DESC |
| `GetPrecompiledLetterTemplate` | SELECT | `templates` | Fetch the hidden pre-compiled letter template for a service |
| `InsertPrecompiledLetterTemplate` | INSERT | `templates`, `template_redacted`, `templates_history` | Create the pre-compiled letter template if absent |
| `InsertTemplateCategory` | INSERT | `template_categories` | Create a new category |
| `GetTemplateCategoryById` | SELECT | `template_categories` | Fetch category by PK |
| `GetTemplateCategoryByTemplateId` | SELECT | `templates`, `template_categories` | Fetch the category for a given template |
| `ListTemplateCategories` | SELECT | `template_categories` | List categories with optional `template_type` and `hidden` filters |
| `UpdateTemplateCategory` | UPDATE | `template_categories` | Persist changes to a category |
| `DeleteTemplateCategoryById` | DELETE | `template_categories` | Delete a category (must have no templates, or cascade reassign) |
| `ReassignTemplateCategory` | UPDATE | `templates` | Reassign a template to a default category (used during cascade delete) |
| `GetTemplateFolderBySvcAndId` | SELECT | `template_folder` | Fetch folder scoped to service |
| `GetTemplateFoldersByIds` | SELECT | `template_folder` | Fetch multiple folders by id list |
| `InsertTemplateFolder` | INSERT | `template_folder` | Create a new folder |
| `UpdateTemplateFolder` | UPDATE | `template_folder` | Persist changes to a folder |
| `DeleteTemplateFolder` | DELETE | `template_folder` | Hard-delete a folder |
