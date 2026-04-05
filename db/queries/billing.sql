-- name: GetMonthlyBillingUsage :many
SELECT date_trunc('month', bst_date)::date AS month,
    sum(coalesce(billing_total, 0)) AS billing_total,
    sum(coalesce(notifications_sent, 0))::bigint AS notifications_sent
FROM ft_billing
WHERE service_id = sqlc.arg(service_id)
    AND extract(
        year
        FROM bst_date
    ) = sqlc.arg(year)
GROUP BY 1
ORDER BY 1;
-- name: GetYearlyBillingUsage :many
SELECT *
FROM annual_billing
WHERE service_id = sqlc.arg(service_id)
ORDER BY financial_year_start DESC;
-- name: GetFreeSMSFragmentLimit :one
SELECT *
FROM annual_billing
WHERE service_id = sqlc.arg(service_id)
    AND financial_year_start = sqlc.arg(financial_year_start)
LIMIT 1;
-- name: UpsertFreeSMSFragmentLimit :one
WITH updated AS (
    UPDATE annual_billing
    SET free_sms_fragment_limit = sqlc.arg(free_sms_fragment_limit),
        updated_at = now()
    WHERE service_id = sqlc.arg(service_id)
        AND financial_year_start = sqlc.arg(financial_year_start)
    RETURNING *
)
INSERT INTO annual_billing (
        id,
        service_id,
        financial_year_start,
        free_sms_fragment_limit,
        updated_at,
        created_at
    )
SELECT sqlc.arg(id),
    sqlc.arg(service_id),
    sqlc.arg(financial_year_start),
    sqlc.arg(free_sms_fragment_limit),
    sqlc.narg(updated_at),
    sqlc.arg(created_at)
WHERE NOT EXISTS (
        SELECT 1
        FROM updated
    )
RETURNING *;
-- name: UpsertFactBillingForDay :one
INSERT INTO ft_billing (
        bst_date,
        template_id,
        service_id,
        notification_type,
        provider,
        rate_multiplier,
        international,
        rate,
        billable_units,
        notifications_sent,
        updated_at,
        created_at,
        postage,
        sms_sending_vehicle,
        billing_total
    )
VALUES (
        sqlc.arg(bst_date),
        sqlc.arg(template_id),
        sqlc.arg(service_id),
        sqlc.arg(notification_type),
        sqlc.arg(provider),
        sqlc.arg(rate_multiplier),
        sqlc.arg(international),
        sqlc.arg(rate),
        sqlc.narg(billable_units),
        sqlc.narg(notifications_sent),
        sqlc.narg(updated_at),
        sqlc.arg(created_at),
        sqlc.arg(postage),
        sqlc.arg(sms_sending_vehicle),
        sqlc.narg(billing_total)
    ) ON CONFLICT (
        bst_date,
        template_id,
        service_id,
        notification_type,
        provider,
        rate_multiplier,
        international,
        rate,
        postage,
        sms_sending_vehicle
    ) DO
UPDATE
SET billable_units = EXCLUDED.billable_units,
    notifications_sent = EXCLUDED.notifications_sent,
    billing_total = EXCLUDED.billing_total,
    updated_at = EXCLUDED.updated_at
RETURNING *;
-- name: GetAnnualLimitsData :many
SELECT *
FROM annual_limits_data
WHERE service_id = sqlc.arg(service_id)
ORDER BY time_period,
    notification_type;
-- name: GetPlatformStatsByDateRange :many
SELECT bst_date,
    sum(coalesce(billing_total, 0)) AS billing_total,
    sum(coalesce(notifications_sent, 0))::bigint AS notifications_sent
FROM ft_billing
WHERE bst_date >= sqlc.arg(start_date)
    AND bst_date <= sqlc.arg(end_date)
GROUP BY bst_date
ORDER BY bst_date;
-- name: GetDeliveredNotificationsByMonth :many
SELECT month,
    service_id,
    notification_type,
    notification_count
FROM monthly_notification_stats_summary
WHERE service_id = sqlc.arg(service_id)
ORDER BY month DESC,
    notification_type ASC;
-- name: GetUsageForTrialServices :many
SELECT s.id AS service_id,
    s.name,
    sum(coalesce(f.notifications_sent, 0))::bigint AS notifications_sent
FROM services AS s
    LEFT JOIN ft_billing AS f ON f.service_id = s.id
WHERE s.restricted = true
GROUP BY s.id,
    s.name
ORDER BY notifications_sent DESC,
    s.name ASC;
-- name: GetUsageForAllServices :many
SELECT s.id AS service_id,
    s.name,
    sum(coalesce(f.notifications_sent, 0))::bigint AS notifications_sent
FROM services AS s
    LEFT JOIN ft_billing AS f ON f.service_id = s.id
GROUP BY s.id,
    s.name
ORDER BY notifications_sent DESC,
    s.name ASC;
-- name: GetFactNotificationStatusForDay :many
SELECT *
FROM ft_notification_status
WHERE bst_date = sqlc.arg(bst_date)
    AND service_id = sqlc.arg(service_id)
ORDER BY template_id,
    job_id,
    notification_status;
-- name: UpsertFactNotificationStatus :one
INSERT INTO ft_notification_status (
        bst_date,
        template_id,
        service_id,
        job_id,
        notification_type,
        key_type,
        notification_status,
        notification_count,
        created_at,
        updated_at,
        billable_units
    )
VALUES (
        sqlc.arg(bst_date),
        sqlc.arg(template_id),
        sqlc.arg(service_id),
        sqlc.arg(job_id),
        sqlc.arg(notification_type),
        sqlc.arg(key_type),
        sqlc.arg(notification_status),
        sqlc.arg(notification_count),
        sqlc.arg(created_at),
        sqlc.narg(updated_at),
        sqlc.arg(billable_units)
    ) ON CONFLICT (
        bst_date,
        template_id,
        service_id,
        job_id,
        notification_type,
        key_type,
        notification_status
    ) DO
UPDATE
SET notification_count = EXCLUDED.notification_count,
    billable_units = EXCLUDED.billable_units,
    updated_at = EXCLUDED.updated_at
RETURNING *;
-- name: UpsertMonthlyNotificationStatsSummary :one
INSERT INTO monthly_notification_stats_summary (
        month,
        service_id,
        notification_type,
        notification_count,
        updated_at
    )
VALUES (
        sqlc.arg(month),
        sqlc.arg(service_id),
        sqlc.arg(notification_type),
        sqlc.arg(notification_count),
        coalesce(sqlc.narg(updated_at), now())
    ) ON CONFLICT (month, service_id, notification_type) DO
UPDATE
SET notification_count = EXCLUDED.notification_count,
    updated_at = EXCLUDED.updated_at
RETURNING *;