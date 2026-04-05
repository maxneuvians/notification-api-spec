-- name: CreateNotification :one
INSERT INTO notifications (
        id,
        "to",
        job_id,
        service_id,
        template_id,
        created_at,
        sent_at,
        sent_by,
        updated_at,
        reference,
        template_version,
        job_row_number,
        _personalisation,
        api_key_id,
        key_type,
        notification_type,
        billable_units,
        client_reference,
        international,
        phone_prefix,
        rate_multiplier,
        notification_status,
        normalised_to,
        created_by_id,
        reply_to_text,
        postage,
        provider_response,
        queue_name,
        feedback_type,
        feedback_subtype,
        ses_feedback_id,
        ses_feedback_date,
        sms_total_message_price,
        sms_total_carrier_fee,
        sms_iso_country_code,
        sms_carrier_name,
        sms_message_encoding,
        sms_origination_phone_number,
        feedback_reason
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(recipient),
        sqlc.narg(job_id),
        sqlc.narg(service_id),
        sqlc.narg(template_id),
        sqlc.arg(created_at),
        sqlc.narg(sent_at),
        sqlc.narg(sent_by),
        sqlc.narg(updated_at),
        sqlc.narg(reference),
        sqlc.arg(template_version),
        sqlc.narg(job_row_number),
        sqlc.narg(personalisation),
        sqlc.narg(api_key_id),
        sqlc.arg(key_type),
        sqlc.arg(notification_type),
        sqlc.arg(billable_units),
        sqlc.narg(client_reference),
        sqlc.narg(international),
        sqlc.narg(phone_prefix),
        sqlc.narg(rate_multiplier),
        sqlc.narg(notification_status),
        sqlc.narg(normalised_to),
        sqlc.narg(created_by_id),
        sqlc.narg(reply_to_text),
        sqlc.narg(postage),
        sqlc.narg(provider_response),
        sqlc.narg(queue_name),
        sqlc.narg(feedback_type),
        sqlc.narg(feedback_subtype),
        sqlc.narg(ses_feedback_id),
        sqlc.narg(ses_feedback_date),
        sqlc.narg(sms_total_message_price),
        sqlc.narg(sms_total_carrier_fee),
        sqlc.narg(sms_iso_country_code),
        sqlc.narg(sms_carrier_name),
        sqlc.narg(sms_message_encoding),
        sqlc.narg(sms_origination_phone_number),
        sqlc.narg(feedback_reason)
    )
RETURNING *;
-- name: BulkInsertNotifications :exec
WITH rows AS (
    SELECT *
    FROM jsonb_to_recordset(sqlc.arg(items)::jsonb) AS x(
            id uuid,
            recipient text,
            job_id uuid,
            service_id uuid,
            template_id uuid,
            created_at timestamp,
            sent_at timestamp,
            sent_by text,
            updated_at timestamp,
            reference_value text,
            template_version integer,
            job_row_number integer,
            personalisation text,
            api_key_id uuid,
            key_type text,
            notification_type public.notification_type,
            billable_units integer,
            client_reference text,
            international boolean,
            phone_prefix text,
            rate_multiplier numeric,
            notification_status text,
            normalised_to text,
            created_by_id uuid,
            reply_to_text text,
            postage text,
            provider_response text,
            queue_name text,
            feedback_type public.notification_feedback_types,
            feedback_subtype public.notification_feedback_subtypes,
            ses_feedback_id text,
            ses_feedback_date timestamp,
            sms_total_message_price double precision,
            sms_total_carrier_fee double precision,
            sms_iso_country_code text,
            sms_carrier_name text,
            sms_message_encoding text,
            sms_origination_phone_number text,
            feedback_reason text
        )
)
INSERT INTO notifications (
        id,
        "to",
        job_id,
        service_id,
        template_id,
        created_at,
        sent_at,
        sent_by,
        updated_at,
        reference,
        template_version,
        job_row_number,
        _personalisation,
        api_key_id,
        key_type,
        notification_type,
        billable_units,
        client_reference,
        international,
        phone_prefix,
        rate_multiplier,
        notification_status,
        normalised_to,
        created_by_id,
        reply_to_text,
        postage,
        provider_response,
        queue_name,
        feedback_type,
        feedback_subtype,
        ses_feedback_id,
        ses_feedback_date,
        sms_total_message_price,
        sms_total_carrier_fee,
        sms_iso_country_code,
        sms_carrier_name,
        sms_message_encoding,
        sms_origination_phone_number,
        feedback_reason
    )
SELECT id,
    recipient,
    job_id,
    service_id,
    template_id,
    created_at,
    sent_at,
    sent_by,
    updated_at,
    reference_value,
    template_version,
    job_row_number,
    personalisation,
    api_key_id,
    key_type,
    notification_type,
    billable_units,
    client_reference,
    international,
    phone_prefix,
    rate_multiplier,
    notification_status,
    normalised_to,
    created_by_id,
    reply_to_text,
    postage,
    provider_response,
    queue_name,
    feedback_type,
    feedback_subtype,
    ses_feedback_id,
    ses_feedback_date,
    sms_total_message_price,
    sms_total_carrier_fee,
    sms_iso_country_code,
    sms_carrier_name,
    sms_message_encoding,
    sms_origination_phone_number,
    feedback_reason
FROM rows;
-- name: UpdateNotificationStatusByID :one
UPDATE notifications
SET notification_status = sqlc.arg(notification_status),
    sent_by = COALESCE(sqlc.narg(sent_by), sent_by),
    feedback_reason = COALESCE(sqlc.narg(feedback_reason), feedback_reason),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: UpdateNotificationStatusByReference :many
UPDATE notifications
SET notification_status = sqlc.arg(notification_status),
    updated_at = now()
WHERE reference = sqlc.arg(reference)
RETURNING *;
-- name: BulkUpdateNotificationStatuses :execrows
WITH updates AS (
    SELECT *
    FROM jsonb_to_recordset(sqlc.arg(items)::jsonb) AS u(
            id uuid,
            notification_status text,
            sent_by text,
            feedback_reason text
        )
)
UPDATE notifications AS n
SET notification_status = u.notification_status,
    sent_by = COALESCE(u.sent_by, n.sent_by),
    feedback_reason = COALESCE(u.feedback_reason, n.feedback_reason),
    updated_at = now()
FROM updates AS u
WHERE n.id = u.id;
-- name: GetNotificationByID :one
SELECT *
FROM notifications
WHERE id = sqlc.arg(id);
-- name: GetNotificationsByServiceID :many
SELECT *
FROM notifications
WHERE service_id = sqlc.arg(service_id)
    AND (
        sqlc.narg(notification_type)::public.notification_type IS NULL
        OR notification_type = sqlc.narg(notification_type)::public.notification_type
    )
    AND (
        sqlc.narg(notification_status)::text IS NULL
        OR notification_status = sqlc.narg(notification_status)::text
    )
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);
-- name: GetNotificationsForJob :many
SELECT *
FROM notifications
WHERE job_id = sqlc.arg(job_id)
    AND (
        sqlc.narg(notification_status)::text IS NULL
        OR notification_status = sqlc.narg(notification_status)::text
    )
ORDER BY created_at DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);
-- name: GetNotificationsCreatedSince :many
SELECT *
FROM notifications
WHERE created_at >= sqlc.arg(since)
    AND (
        sqlc.narg(notification_status)::text IS NULL
        OR notification_status = sqlc.narg(notification_status)::text
    )
ORDER BY created_at ASC;
-- name: TimeoutSendingNotifications :many
UPDATE notifications
SET notification_status = 'temporary-failure',
    updated_at = now()
WHERE notification_status = 'sending'
    AND COALESCE(updated_at, created_at) < sqlc.arg(cutoff)
RETURNING id;
-- name: DeleteNotificationsOlderThanRetention :execrows
DELETE FROM notifications
WHERE notification_type = sqlc.arg(notification_type)
    AND created_at < sqlc.arg(cutoff);
-- name: GetLastNotificationAddedForJobID :one
SELECT *
FROM notifications
WHERE job_id = sqlc.arg(job_id)
ORDER BY created_at DESC,
    id DESC
LIMIT 1;
-- name: GetNotificationsByReference :many
SELECT *
FROM notifications
WHERE reference = sqlc.arg(reference)
ORDER BY created_at DESC;
-- name: GetHardBouncesForService :many
SELECT *
FROM notifications
WHERE service_id = sqlc.arg(service_id)
    AND feedback_type = 'hard-bounce'
    AND created_at >= sqlc.arg(since)
ORDER BY created_at DESC;
-- name: GetMonthlyNotificationStats :many
SELECT date_trunc('month', created_at)::date AS month,
    notification_type,
    notification_status,
    count(*) AS notification_count,
    coalesce(sum(billable_units), 0)::bigint AS billable_units
FROM notifications
WHERE service_id = sqlc.arg(service_id)
    AND extract(
        year
        FROM created_at
    ) = sqlc.arg(year)
GROUP BY 1,
    2,
    3
ORDER BY 1,
    2,
    3;
-- name: GetTemplateUsageMonthly :many
SELECT date_trunc('month', created_at)::date AS month,
    template_id,
    template_version,
    count(*) AS notification_count
FROM notifications
WHERE service_id = sqlc.arg(service_id)
    AND extract(
        year
        FROM created_at
    ) = sqlc.arg(year)
GROUP BY 1,
    2,
    3
ORDER BY 1,
    2,
    3;
-- name: InsertNotificationHistory :exec
INSERT INTO notification_history (
        id,
        job_id,
        job_row_number,
        service_id,
        template_id,
        template_version,
        api_key_id,
        key_type,
        notification_type,
        created_at,
        sent_at,
        sent_by,
        updated_at,
        reference,
        billable_units,
        client_reference,
        international,
        phone_prefix,
        rate_multiplier,
        notification_status,
        created_by_id,
        postage,
        queue_name,
        feedback_type,
        feedback_subtype,
        ses_feedback_id,
        ses_feedback_date,
        sms_total_message_price,
        sms_total_carrier_fee,
        sms_iso_country_code,
        sms_carrier_name,
        sms_message_encoding,
        sms_origination_phone_number,
        feedback_reason
    )
VALUES (
        sqlc.arg(id),
        sqlc.narg(job_id),
        sqlc.narg(job_row_number),
        sqlc.narg(service_id),
        sqlc.narg(template_id),
        sqlc.arg(template_version),
        sqlc.narg(api_key_id),
        sqlc.arg(key_type),
        sqlc.arg(notification_type),
        sqlc.arg(created_at),
        sqlc.narg(sent_at),
        sqlc.narg(sent_by),
        sqlc.narg(updated_at),
        sqlc.narg(reference),
        sqlc.arg(billable_units),
        sqlc.narg(client_reference),
        sqlc.narg(international),
        sqlc.narg(phone_prefix),
        sqlc.narg(rate_multiplier),
        sqlc.narg(notification_status),
        sqlc.narg(created_by_id),
        sqlc.narg(postage),
        sqlc.narg(queue_name),
        sqlc.narg(feedback_type),
        sqlc.narg(feedback_subtype),
        sqlc.narg(ses_feedback_id),
        sqlc.narg(ses_feedback_date),
        sqlc.narg(sms_total_message_price),
        sqlc.narg(sms_total_carrier_fee),
        sqlc.narg(sms_iso_country_code),
        sqlc.narg(sms_carrier_name),
        sqlc.narg(sms_message_encoding),
        sqlc.narg(sms_origination_phone_number),
        sqlc.narg(feedback_reason)
    );
-- name: GetNotificationFromHistory :one
SELECT *
FROM notification_history
WHERE id = sqlc.arg(id)
ORDER BY created_at DESC
LIMIT 1;
-- name: GetBounceRateTimeSeries :many
SELECT date_trunc('day', created_at)::date AS day,
    count(*) FILTER (
        WHERE feedback_type = 'hard-bounce'
    ) AS hard_bounces,
    count(*) FILTER (
        WHERE feedback_type = 'soft-bounce'
    ) AS soft_bounces,
    count(*) AS total_notifications
FROM notifications
WHERE service_id = sqlc.arg(service_id)
    AND created_at >= sqlc.arg(since)
GROUP BY 1
ORDER BY 1;
-- name: GetLastTemplateUsage :one
SELECT *
FROM notifications
WHERE template_id = sqlc.arg(template_id)
    AND service_id = sqlc.arg(service_id)
    AND notification_type = sqlc.arg(notification_type)
ORDER BY created_at DESC
LIMIT 1;