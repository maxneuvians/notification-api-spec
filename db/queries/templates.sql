-- name: CreateTemplate :one
INSERT INTO templates (
        id,
        name,
        template_type,
        created_at,
        updated_at,
        content,
        service_id,
        subject,
        created_by_id,
        version,
        archived,
        process_type,
        service_letter_contact_id,
        hidden,
        postage,
        template_category_id,
        text_direction_rtl
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(name),
        sqlc.arg(template_type),
        sqlc.arg(created_at),
        sqlc.narg(updated_at),
        sqlc.arg(content),
        sqlc.arg(service_id),
        sqlc.narg(subject),
        sqlc.arg(created_by_id),
        sqlc.arg(version),
        sqlc.arg(archived),
        sqlc.narg(process_type),
        sqlc.narg(service_letter_contact_id),
        sqlc.arg(hidden),
        sqlc.narg(postage),
        sqlc.narg(template_category_id),
        sqlc.arg(text_direction_rtl)
    )
RETURNING *;
-- name: GetTemplateByID :one
SELECT *
FROM templates
WHERE id = sqlc.arg(id)
    AND service_id = sqlc.arg(service_id);
-- name: GetTemplateByIDAndVersion :one
SELECT *
FROM templates_history
WHERE id = sqlc.arg(id)
    AND version = sqlc.arg(version);
-- name: GetTemplatesByServiceID :many
SELECT *
FROM templates
WHERE service_id = sqlc.arg(service_id)
    AND archived = false
ORDER BY created_at DESC;
-- name: UpdateTemplate :one
UPDATE templates
SET name = sqlc.arg(name),
    updated_at = sqlc.narg(updated_at),
    content = sqlc.arg(content),
    subject = sqlc.narg(subject),
    version = sqlc.arg(version),
    archived = sqlc.arg(archived),
    process_type = sqlc.narg(process_type),
    service_letter_contact_id = sqlc.narg(service_letter_contact_id),
    hidden = sqlc.arg(hidden),
    postage = sqlc.narg(postage),
    template_category_id = sqlc.narg(template_category_id),
    text_direction_rtl = sqlc.arg(text_direction_rtl)
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: ArchiveTemplate :one
UPDATE templates
SET archived = true,
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: GetTemplateVersions :many
SELECT *
FROM templates_history
WHERE id = sqlc.arg(id)
ORDER BY version DESC;
-- name: GetPrecompiledLetterTemplate :one
SELECT *
FROM templates
WHERE service_id = sqlc.arg(service_id)
    AND template_type = 'letter'
    AND archived = false
ORDER BY updated_at DESC NULLS LAST,
    created_at DESC
LIMIT 1;
-- name: GetTemplateFolders :many
SELECT *
FROM template_folder
WHERE service_id = sqlc.arg(service_id)
ORDER BY name ASC;
-- name: CreateTemplateFolder :one
INSERT INTO template_folder (
        id,
        service_id,
        name,
        parent_id
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(service_id),
        sqlc.arg(name),
        sqlc.narg(parent_id)
    )
RETURNING *;
-- name: UpdateTemplateFolder :one
UPDATE template_folder
SET name = sqlc.arg(name),
    parent_id = sqlc.narg(parent_id)
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: DeleteTemplateFolder :execrows
DELETE FROM template_folder
WHERE id = sqlc.arg(id);
-- name: MoveTemplateContents :execrows
UPDATE template_folder_map
SET template_folder_id = sqlc.narg(target_folder_id)
WHERE template_id = ANY(sqlc.arg(template_ids)::uuid [])
    OR template_folder_id = ANY(sqlc.arg(folder_ids)::uuid []);
-- name: InsertTemplateHistory :exec
INSERT INTO templates_history (
        id,
        name,
        template_type,
        created_at,
        updated_at,
        content,
        service_id,
        subject,
        created_by_id,
        version,
        archived,
        process_type,
        service_letter_contact_id,
        hidden,
        postage,
        template_category_id,
        text_direction_rtl
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(name),
        sqlc.arg(template_type),
        sqlc.arg(created_at),
        sqlc.narg(updated_at),
        sqlc.arg(content),
        sqlc.arg(service_id),
        sqlc.narg(subject),
        sqlc.arg(created_by_id),
        sqlc.arg(version),
        sqlc.arg(archived),
        sqlc.narg(process_type),
        sqlc.narg(service_letter_contact_id),
        sqlc.arg(hidden),
        sqlc.narg(postage),
        sqlc.narg(template_category_id),
        sqlc.arg(text_direction_rtl)
    );