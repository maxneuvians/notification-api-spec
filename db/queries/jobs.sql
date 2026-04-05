-- name: CreateJob :one
INSERT INTO jobs (
        id,
        original_file_name,
        service_id,
        template_id,
        created_at,
        updated_at,
        notification_count,
        notifications_sent,
        processing_started,
        processing_finished,
        created_by_id,
        template_version,
        notifications_delivered,
        notifications_failed,
        job_status,
        scheduled_for,
        archived,
        api_key_id,
        sender_id
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(original_file_name),
        sqlc.arg(service_id),
        sqlc.narg(template_id),
        sqlc.arg(created_at),
        sqlc.narg(updated_at),
        sqlc.arg(notification_count),
        sqlc.arg(notifications_sent),
        sqlc.narg(processing_started),
        sqlc.narg(processing_finished),
        sqlc.narg(created_by_id),
        sqlc.arg(template_version),
        sqlc.arg(notifications_delivered),
        sqlc.arg(notifications_failed),
        sqlc.arg(job_status),
        sqlc.narg(scheduled_for),
        sqlc.arg(archived),
        sqlc.narg(api_key_id),
        sqlc.narg(sender_id)
    )
RETURNING *;
-- name: GetJobByID :one
SELECT *
FROM jobs
WHERE id = sqlc.arg(id);
-- name: GetJobsByServiceID :many
SELECT *
FROM jobs
WHERE service_id = sqlc.arg(service_id)
    AND archived = false
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);
-- name: UpdateJob :one
UPDATE jobs
SET original_file_name = sqlc.arg(original_file_name),
    template_id = sqlc.narg(template_id),
    updated_at = sqlc.narg(updated_at),
    notification_count = sqlc.arg(notification_count),
    notifications_sent = sqlc.arg(notifications_sent),
    processing_started = sqlc.narg(processing_started),
    processing_finished = sqlc.narg(processing_finished),
    created_by_id = sqlc.narg(created_by_id),
    template_version = sqlc.arg(template_version),
    notifications_delivered = sqlc.arg(notifications_delivered),
    notifications_failed = sqlc.arg(notifications_failed),
    job_status = sqlc.arg(job_status),
    scheduled_for = sqlc.narg(scheduled_for),
    archived = sqlc.arg(archived),
    api_key_id = sqlc.narg(api_key_id),
    sender_id = sqlc.narg(sender_id)
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: SetScheduledJobsToPending :many
UPDATE jobs
SET job_status = 'pending',
    updated_at = now()
WHERE job_status = 'scheduled'
    AND scheduled_for <= now()
    AND archived = false
RETURNING *;
-- name: GetInProgressJobs :many
SELECT *
FROM jobs
WHERE job_status = 'in progress'
    AND archived = false
ORDER BY processing_started ASC NULLS LAST;
-- name: GetStalledJobs :many
SELECT *
FROM jobs
WHERE job_status = 'in progress'
    AND archived = false
    AND processing_started >= sqlc.arg(older_than)
    AND processing_started <= sqlc.arg(newer_than)
ORDER BY processing_started ASC;
-- name: ArchiveOldJobs :execrows
UPDATE jobs
SET archived = true,
    updated_at = now()
WHERE archived = false
    AND processing_finished < sqlc.arg(cutoff);
-- name: HasJobs :one
SELECT EXISTS(
        SELECT 1
        FROM jobs
        WHERE service_id = sqlc.arg(service_id)
            AND archived = false
    ) AS has_jobs;