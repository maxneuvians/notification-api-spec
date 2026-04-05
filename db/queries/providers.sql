-- name: GetAllProviders :many
SELECT *
FROM provider_details
ORDER BY notification_type ASC,
    priority ASC,
    display_name ASC;
-- name: GetProviderByID :one
SELECT *
FROM provider_details
WHERE id = sqlc.arg(id);
-- name: GetProviderVersions :many
SELECT *
FROM provider_details_history
WHERE id = sqlc.arg(id)
ORDER BY version DESC;
-- name: UpdateProvider :one
UPDATE provider_details
SET display_name = sqlc.arg(display_name),
    identifier = sqlc.arg(identifier),
    priority = sqlc.arg(priority),
    notification_type = sqlc.arg(notification_type),
    active = sqlc.arg(active),
    updated_at = sqlc.narg(updated_at),
    version = sqlc.arg(version),
    created_by_id = sqlc.narg(created_by_id),
    supports_international = sqlc.arg(supports_international)
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: ToggleSMSProvider :many
UPDATE provider_details
SET active = CASE
        WHEN id = sqlc.arg(id) THEN true
        ELSE false
    END,
    updated_at = now()
WHERE notification_type = 'sms'
RETURNING *;
-- name: InsertProviderHistory :exec
INSERT INTO provider_details_history (
        id,
        display_name,
        identifier,
        priority,
        notification_type,
        active,
        version,
        updated_at,
        created_by_id,
        supports_international
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(display_name),
        sqlc.arg(identifier),
        sqlc.arg(priority),
        sqlc.arg(notification_type),
        sqlc.arg(active),
        sqlc.arg(version),
        sqlc.narg(updated_at),
        sqlc.narg(created_by_id),
        sqlc.arg(supports_international)
    );