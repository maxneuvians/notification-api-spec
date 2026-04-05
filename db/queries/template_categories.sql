-- name: GetTemplateCategories :many
SELECT *
FROM template_categories
WHERE (
        sqlc.narg(hidden)::boolean IS NULL
        OR hidden = sqlc.narg(hidden)::boolean
    )
ORDER BY name_en ASC;
-- name: GetTemplateCategoryByID :one
SELECT *
FROM template_categories
WHERE id = sqlc.arg(id);
-- name: CreateTemplateCategory :one
INSERT INTO template_categories (
        id,
        name_en,
        name_fr,
        description_en,
        description_fr,
        sms_process_type,
        email_process_type,
        hidden,
        created_at,
        updated_at,
        sms_sending_vehicle,
        created_by_id,
        updated_by_id
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(name_en),
        sqlc.arg(name_fr),
        sqlc.narg(description_en),
        sqlc.narg(description_fr),
        sqlc.arg(sms_process_type),
        sqlc.arg(email_process_type),
        sqlc.arg(hidden),
        sqlc.narg(created_at),
        sqlc.narg(updated_at),
        sqlc.arg(sms_sending_vehicle),
        sqlc.arg(created_by_id),
        sqlc.narg(updated_by_id)
    )
RETURNING *;
-- name: UpdateTemplateCategory :one
UPDATE template_categories
SET name_en = sqlc.arg(name_en),
    name_fr = sqlc.arg(name_fr),
    description_en = sqlc.narg(description_en),
    description_fr = sqlc.narg(description_fr),
    sms_process_type = sqlc.arg(sms_process_type),
    email_process_type = sqlc.arg(email_process_type),
    hidden = sqlc.arg(hidden),
    updated_at = now(),
    sms_sending_vehicle = sqlc.arg(sms_sending_vehicle),
    updated_by_id = sqlc.narg(updated_by_id)
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: DeleteTemplateCategory :execrows
DELETE FROM template_categories
WHERE id = sqlc.arg(id);