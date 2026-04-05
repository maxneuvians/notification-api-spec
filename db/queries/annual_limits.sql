-- name: InsertQuarterData :exec
INSERT INTO annual_limits_data (
        service_id,
        time_period,
        annual_email_limit,
        annual_sms_limit,
        notification_type,
        notification_count
    )
VALUES (
        sqlc.arg(service_id),
        sqlc.arg(time_period),
        sqlc.arg(annual_email_limit),
        sqlc.arg(annual_sms_limit),
        sqlc.arg(notification_type),
        sqlc.arg(notification_count)
    ) ON CONFLICT (service_id, notification_type, time_period) DO
UPDATE
SET annual_email_limit = EXCLUDED.annual_email_limit,
    annual_sms_limit = EXCLUDED.annual_sms_limit,
    notification_count = EXCLUDED.notification_count;
-- name: GetAnnualLimitsDataByServiceAndPeriod :many
SELECT *
FROM annual_limits_data
WHERE service_id = sqlc.arg(service_id)
    AND time_period = sqlc.arg(time_period)
ORDER BY notification_type;
-- name: GetAnnualLimitsDataByPeriod :many
SELECT *
FROM annual_limits_data
WHERE time_period = sqlc.arg(time_period)
ORDER BY service_id,
    notification_type;