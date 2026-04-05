-- name: CreateReport :one
INSERT INTO reports (
        id,
        report_type,
        requested_at,
        completed_at,
        expires_at,
        requesting_user_id,
        service_id,
        job_id,
        url,
        status,
        language
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(report_type),
        sqlc.arg(requested_at),
        sqlc.narg(completed_at),
        sqlc.narg(expires_at),
        sqlc.narg(requesting_user_id),
        sqlc.arg(service_id),
        sqlc.narg(job_id),
        sqlc.narg(url),
        sqlc.arg(status),
        sqlc.narg(language)
    )
RETURNING *;
-- name: GetReportByID :one
SELECT *
FROM reports
WHERE id = sqlc.arg(id);
-- name: GetReportsByServiceID :many
SELECT *
FROM reports
WHERE service_id = sqlc.arg(service_id)
ORDER BY requested_at DESC;
-- name: UpdateReport :one
UPDATE reports
SET report_type = sqlc.arg(report_type),
    requested_at = sqlc.arg(requested_at),
    completed_at = sqlc.narg(completed_at),
    expires_at = sqlc.narg(expires_at),
    requesting_user_id = sqlc.narg(requesting_user_id),
    service_id = sqlc.arg(service_id),
    job_id = sqlc.narg(job_id),
    url = sqlc.narg(url),
    status = sqlc.arg(status),
    language = sqlc.narg(language)
WHERE id = sqlc.arg(id)
RETURNING *;