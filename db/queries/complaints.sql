-- name: CreateOrUpdateComplaint :one
INSERT INTO complaints (
        id,
        notification_id,
        service_id,
        ses_feedback_id,
        complaint_type,
        complaint_date,
        created_at
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(notification_id),
        sqlc.arg(service_id),
        sqlc.narg(ses_feedback_id),
        sqlc.narg(complaint_type),
        sqlc.narg(complaint_date),
        sqlc.arg(created_at)
    ) ON CONFLICT (id) DO
UPDATE
SET ses_feedback_id = EXCLUDED.ses_feedback_id,
    complaint_type = EXCLUDED.complaint_type,
    complaint_date = EXCLUDED.complaint_date
RETURNING *;
-- name: GetComplaintsPage :many
SELECT *
FROM complaints
WHERE complaint_date >= sqlc.arg(start_at)
    AND complaint_date <= sqlc.arg(end_at)
ORDER BY complaint_date DESC NULLS LAST,
    created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);
-- name: CountComplaintsByDateRange :one
SELECT count(*)::bigint
FROM complaints
WHERE complaint_date >= sqlc.arg(start_at)
    AND complaint_date <= sqlc.arg(end_at);