-- partial unique index `uix_service_to_key_name` only applies to active keys (`expiry_date IS NULL`), so revoked names can be reused.
-- name: CreateAPIKey :one
INSERT INTO api_keys (
        id,
        name,
        secret,
        service_id,
        expiry_date,
        created_at,
        created_by_id,
        updated_at,
        version,
        key_type,
        compromised_key_info,
        last_used_timestamp
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(name),
        sqlc.arg(secret),
        sqlc.arg(service_id),
        sqlc.narg(expiry_date),
        sqlc.arg(created_at),
        sqlc.arg(created_by_id),
        sqlc.narg(updated_at),
        sqlc.arg(version),
        sqlc.arg(key_type),
        sqlc.narg(compromised_key_info),
        sqlc.narg(last_used_timestamp)
    )
RETURNING *;
-- name: GetAPIKeysByServiceID :many
SELECT *
FROM api_keys
WHERE service_id = sqlc.arg(service_id)
    AND (expiry_date IS NULL OR expiry_date > now())
ORDER BY created_at DESC;
-- name: GetAPIKeyByID :one
SELECT *
FROM api_keys
WHERE id = sqlc.arg(id);
-- name: RevokeAPIKey :one
UPDATE api_keys
SET expiry_date = now(),
    updated_at = now(),
    version = version + 1
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: GetAPIKeyBySecret :one
SELECT *
FROM api_keys
WHERE secret = sqlc.arg(secret)
    AND (expiry_date IS NULL OR expiry_date > now())
LIMIT 1;
-- name: UpdateAPIKeyLastUsed :one
UPDATE api_keys
SET last_used_timestamp = sqlc.arg(last_used_timestamp),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: RecordAPIKeyCompromise :one
UPDATE api_keys
SET compromised_key_info = sqlc.arg(compromised_key_info),
    updated_at = now(),
    version = version + 1
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: GetAPIKeySummaryStats :one
SELECT k.id,
    k.service_id,
    k.name,
    k.expiry_date,
    k.last_used_timestamp,
    count(n.id) AS notifications_sent
FROM api_keys AS k
    LEFT JOIN notifications AS n ON n.api_key_id = k.id
WHERE k.id = sqlc.arg(id)
GROUP BY k.id,
    k.service_id,
    k.name,
    k.expiry_date,
    k.last_used_timestamp;
-- name: GetAPIKeysRankedByNotifications :many
SELECT k.id,
    k.service_id,
    k.name,
    count(n.id) AS notifications_sent
FROM api_keys AS k
    LEFT JOIN notifications AS n ON n.api_key_id = k.id
    AND n.created_at >= sqlc.arg(since)
GROUP BY k.id,
    k.service_id,
    k.name
ORDER BY notifications_sent DESC,
    k.name ASC;
-- name: InsertAPIKeyHistory :exec
INSERT INTO api_keys_history (
        id,
        name,
        secret,
        service_id,
        expiry_date,
        created_at,
        updated_at,
        created_by_id,
        version,
        key_type,
        compromised_key_info,
        last_used_timestamp
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(name),
        sqlc.arg(secret),
        sqlc.arg(service_id),
        sqlc.narg(expiry_date),
        sqlc.arg(created_at),
        sqlc.narg(updated_at),
        sqlc.arg(created_by_id),
        sqlc.arg(version),
        sqlc.arg(key_type),
        sqlc.narg(compromised_key_info),
        sqlc.narg(last_used_timestamp)
    );