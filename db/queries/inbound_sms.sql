-- inbound_sms.content is encrypted at rest and intentionally uses the physical column name `content`.
-- name: GetInboundNumbers :many
SELECT *
FROM inbound_numbers
WHERE service_id = sqlc.arg(service_id)
ORDER BY number ASC;
-- name: GetAvailableInboundNumbers :many
SELECT *
FROM inbound_numbers
WHERE service_id IS NULL
    AND active = true
ORDER BY number ASC;
-- name: GetInboundNumberByServiceID :one
SELECT *
FROM inbound_numbers
WHERE service_id = sqlc.arg(service_id)
    AND active = true
LIMIT 1;
-- name: AddInboundNumber :one
UPDATE inbound_numbers
SET service_id = sqlc.arg(service_id),
    active = true,
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: DisableInboundNumberForService :one
UPDATE inbound_numbers
SET service_id = NULL,
    active = false,
    updated_at = now()
WHERE service_id = sqlc.arg(service_id)
    AND id = sqlc.arg(id)
RETURNING *;
-- name: CreateInboundSMS :one
INSERT INTO inbound_sms (
        id,
        service_id,
        content,
        notify_number,
        user_number,
        created_at,
        provider_date,
        provider_reference,
        provider
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(service_id),
        sqlc.arg(content),
        sqlc.arg(notify_number),
        sqlc.arg(user_number),
        sqlc.arg(created_at),
        sqlc.narg(provider_date),
        sqlc.narg(provider_reference),
        sqlc.arg(provider)
    )
RETURNING *;
-- name: GetInboundSMSForService :many
SELECT *
FROM inbound_sms
WHERE service_id = sqlc.arg(service_id)
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);
-- name: GetMostRecentInboundSMS :one
SELECT *
FROM inbound_sms
WHERE service_id = sqlc.arg(service_id)
ORDER BY created_at DESC
LIMIT 1;
-- name: GetInboundSMSSummary :many
SELECT date_trunc('day', created_at)::date AS day,
    count(*) AS message_count
FROM inbound_sms
WHERE service_id = sqlc.arg(service_id)
GROUP BY 1
ORDER BY 1 DESC;
-- name: GetInboundSMSByID :one
SELECT *
FROM inbound_sms
WHERE id = sqlc.arg(id);
-- name: DeleteInboundSMSOlderThan :execrows
DELETE FROM inbound_sms
WHERE created_at < sqlc.arg(cutoff);