-- name: GetAllServices :many
SELECT *
FROM services
WHERE (
        NOT sqlc.arg(only_active)::boolean
        OR active = true
    )
ORDER BY name ASC;
-- name: GetServicesByPartialName :many
SELECT *
FROM services
WHERE name ILIKE '%' || sqlc.arg(name_query) || '%'
ORDER BY name ASC;
-- name: CountLiveServices :one
SELECT count(*)::bigint
FROM services
WHERE active = true
    AND count_as_live = true;
-- name: GetLiveServicesData :many
SELECT id,
    name,
    active,
    count_as_live,
    go_live_at,
    suspended_at
FROM services
WHERE active = true
    AND count_as_live = true
ORDER BY name ASC;
-- name: GetServiceByID :one
SELECT *
FROM services
WHERE id = sqlc.arg(id)
    AND (
        NOT sqlc.arg(only_active)::boolean
        OR active = true
    );
-- name: GetServiceByInboundNumber :one
SELECT s.*
FROM services AS s
    JOIN inbound_numbers AS i ON i.service_id = s.id
WHERE i.number = sqlc.arg(number)
    AND i.active = true;
-- name: GetServiceByIDWithAPIKeys :one
SELECT s.*
FROM services AS s
WHERE s.id = sqlc.arg(id);
-- name: GetServicesByUserID :many
SELECT s.*
FROM services AS s
    JOIN user_to_service AS uts ON uts.service_id = s.id
WHERE uts.user_id = sqlc.arg(user_id)
    AND (
        NOT sqlc.arg(only_active)::boolean
        OR s.active = true
    )
ORDER BY s.name ASC;
-- name: CreateService :one
INSERT INTO services (
        id,
        name,
        created_at,
        updated_at,
        active,
        message_limit,
        restricted,
        email_from,
        created_by_id,
        version,
        research_mode,
        organisation_type,
        prefix_sms,
        crown,
        rate_limit,
        contact_link,
        consent_to_research,
        volume_email,
        volume_letter,
        volume_sms,
        count_as_live,
        go_live_at,
        go_live_user_id,
        organisation_id,
        sending_domain,
        default_branding_is_french,
        sms_daily_limit,
        organisation_notes,
        sensitive_service,
        email_annual_limit,
        sms_annual_limit,
        suspended_by_id,
        suspended_at
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(name),
        sqlc.arg(created_at),
        sqlc.narg(updated_at),
        sqlc.arg(active),
        sqlc.arg(message_limit),
        sqlc.arg(restricted),
        sqlc.arg(email_from),
        sqlc.arg(created_by_id),
        sqlc.arg(version),
        sqlc.arg(research_mode),
        sqlc.narg(organisation_type),
        sqlc.arg(prefix_sms),
        sqlc.narg(crown),
        sqlc.arg(rate_limit),
        sqlc.narg(contact_link),
        sqlc.narg(consent_to_research),
        sqlc.narg(volume_email),
        sqlc.narg(volume_letter),
        sqlc.narg(volume_sms),
        sqlc.arg(count_as_live),
        sqlc.narg(go_live_at),
        sqlc.narg(go_live_user_id),
        sqlc.narg(organisation_id),
        sqlc.narg(sending_domain),
        sqlc.narg(default_branding_is_french),
        sqlc.arg(sms_daily_limit),
        sqlc.narg(organisation_notes),
        sqlc.narg(sensitive_service),
        sqlc.arg(email_annual_limit),
        sqlc.arg(sms_annual_limit),
        sqlc.narg(suspended_by_id),
        sqlc.narg(suspended_at)
    )
RETURNING *;
-- name: UpdateService :one
UPDATE services
SET name = sqlc.arg(name),
    updated_at = sqlc.narg(updated_at),
    active = sqlc.arg(active),
    message_limit = sqlc.arg(message_limit),
    restricted = sqlc.arg(restricted),
    email_from = sqlc.arg(email_from),
    version = sqlc.arg(version),
    research_mode = sqlc.arg(research_mode),
    organisation_type = sqlc.narg(organisation_type),
    prefix_sms = sqlc.arg(prefix_sms),
    crown = sqlc.narg(crown),
    rate_limit = sqlc.arg(rate_limit),
    contact_link = sqlc.narg(contact_link),
    consent_to_research = sqlc.narg(consent_to_research),
    volume_email = sqlc.narg(volume_email),
    volume_letter = sqlc.narg(volume_letter),
    volume_sms = sqlc.narg(volume_sms),
    count_as_live = sqlc.arg(count_as_live),
    go_live_at = sqlc.narg(go_live_at),
    go_live_user_id = sqlc.narg(go_live_user_id),
    organisation_id = sqlc.narg(organisation_id),
    sending_domain = sqlc.narg(sending_domain),
    default_branding_is_french = sqlc.narg(default_branding_is_french),
    sms_daily_limit = sqlc.arg(sms_daily_limit),
    organisation_notes = sqlc.narg(organisation_notes),
    sensitive_service = sqlc.narg(sensitive_service),
    email_annual_limit = sqlc.arg(email_annual_limit),
    sms_annual_limit = sqlc.arg(sms_annual_limit),
    suspended_by_id = sqlc.narg(suspended_by_id),
    suspended_at = sqlc.narg(suspended_at)
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: ArchiveService :one
UPDATE services
SET name = name || '_archived_' || extract(
        epoch
        FROM now()
    )::bigint::text,
    active = false,
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING name;
-- name: SuspendService :one
UPDATE services
SET suspended_at = now(),
    suspended_by_id = sqlc.narg(suspended_by_id),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: ResumeService :one
UPDATE services
SET suspended_at = NULL,
    suspended_by_id = NULL,
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: GetServiceByIDAndUser :one
SELECT s.*
FROM services AS s
    JOIN user_to_service AS uts ON uts.service_id = s.id
WHERE s.id = sqlc.arg(service_id)
    AND uts.user_id = sqlc.arg(user_id);
-- name: AddUserToService :exec
INSERT INTO user_to_service (user_id, service_id)
VALUES (sqlc.arg(user_id), sqlc.arg(service_id));
-- name: RemoveUserFromService :execrows
DELETE FROM user_to_service
WHERE user_id = sqlc.arg(user_id)
    AND service_id = sqlc.arg(service_id);
-- name: GetServicePermissions :many
SELECT permission
FROM service_permissions
WHERE service_id = sqlc.arg(service_id)
ORDER BY permission ASC;
-- name: SetServicePermissions :exec
WITH deleted AS (
    DELETE FROM service_permissions
    WHERE service_id = sqlc.arg(service_id)
)
INSERT INTO service_permissions (service_id, permission, created_at)
SELECT sqlc.arg(service_id),
    permission,
    now()
FROM unnest(sqlc.arg(permissions)::text []) AS permission;
-- name: GetSafelist :many
SELECT *
FROM service_safelist
WHERE service_id = sqlc.arg(service_id)
ORDER BY recipient_type,
    recipient;
-- name: UpdateSafelist :exec
WITH deleted AS (
    DELETE FROM service_safelist AS ss
    WHERE ss.service_id = sqlc.arg(target_service_id)
),
email_rows AS (
    SELECT *
    FROM jsonb_to_recordset(sqlc.arg(email_items)::jsonb) AS x(id uuid, recipient text)
),
inserted_emails AS (
    INSERT INTO service_safelist (
            id,
            service_id,
            recipient_type,
            recipient,
            created_at
        )
    SELECT id,
        sqlc.arg(target_service_id),
        'email'::public.recipient_type,
        recipient,
        now()
    FROM email_rows
),
phone_rows AS (
    SELECT *
    FROM jsonb_to_recordset(sqlc.arg(phone_items)::jsonb) AS x(id uuid, recipient text)
),
inserted_phones AS (
    INSERT INTO service_safelist (
            id,
            service_id,
            recipient_type,
            recipient,
            created_at
        )
    SELECT id,
        sqlc.arg(target_service_id),
        'mobile'::public.recipient_type,
        recipient,
        now()
    FROM phone_rows
)
SELECT 1;
-- name: GetDataRetention :many
SELECT *
FROM service_data_retention
WHERE service_id = sqlc.arg(service_id)
ORDER BY notification_type;
-- name: UpsertDataRetention :one
WITH updated AS (
    UPDATE service_data_retention
    SET days_of_retention = sqlc.arg(days_of_retention),
        updated_at = now()
    WHERE service_id = sqlc.arg(service_id)
        AND notification_type = sqlc.arg(notification_type)
    RETURNING *
)
INSERT INTO service_data_retention (
        id,
        service_id,
        notification_type,
        days_of_retention,
        created_at,
        updated_at
    )
SELECT sqlc.arg(id),
    sqlc.arg(service_id),
    sqlc.arg(notification_type),
    sqlc.arg(days_of_retention),
    sqlc.arg(created_at),
    sqlc.narg(updated_at)
WHERE NOT EXISTS (
        SELECT 1
        FROM updated
    )
RETURNING *;
-- name: GetSMSSenders :many
SELECT *
FROM service_sms_senders
WHERE service_id = sqlc.arg(service_id)
    AND archived = false
ORDER BY is_default DESC,
    created_at ASC;
-- name: CreateSMSSender :one
INSERT INTO service_sms_senders (
        id,
        sms_sender,
        service_id,
        is_default,
        inbound_number_id,
        created_at,
        updated_at,
        archived
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(sms_sender),
        sqlc.arg(service_id),
        sqlc.arg(is_default),
        sqlc.narg(inbound_number_id),
        sqlc.arg(created_at),
        sqlc.narg(updated_at),
        sqlc.arg(archived)
    )
RETURNING *;
-- name: UpdateSMSSender :one
UPDATE service_sms_senders
SET sms_sender = sqlc.arg(sms_sender),
    is_default = sqlc.arg(is_default),
    inbound_number_id = sqlc.narg(inbound_number_id),
    updated_at = now(),
    archived = sqlc.arg(archived)
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: GetEmailReplyTo :many
SELECT *
FROM service_email_reply_to
WHERE service_id = sqlc.arg(service_id)
    AND archived = false
ORDER BY is_default DESC,
    created_at ASC;
-- name: CreateEmailReplyTo :one
INSERT INTO service_email_reply_to (
        id,
        service_id,
        email_address,
        is_default,
        created_at,
        updated_at,
        archived
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(service_id),
        sqlc.arg(email_address),
        sqlc.arg(is_default),
        sqlc.arg(created_at),
        sqlc.narg(updated_at),
        sqlc.arg(archived)
    )
RETURNING *;
-- name: UpdateEmailReplyTo :one
UPDATE service_email_reply_to
SET email_address = sqlc.arg(email_address),
    is_default = sqlc.arg(is_default),
    updated_at = now(),
    archived = sqlc.arg(archived)
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: GetCallbackAPIs :many
SELECT *
FROM service_callback_api
WHERE service_id = sqlc.arg(service_id)
    AND (
        sqlc.narg(callback_type)::text IS NULL
        OR callback_type = sqlc.narg(callback_type)::text
    )
ORDER BY created_at ASC;
-- name: UpsertCallbackAPI :one
WITH updated AS (
    UPDATE service_callback_api
    SET url = sqlc.arg(url),
        bearer_token = sqlc.arg(bearer_token),
        updated_at = sqlc.narg(updated_at),
        updated_by_id = sqlc.arg(updated_by_id),
        version = sqlc.arg(version),
        is_suspended = sqlc.narg(is_suspended),
        suspended_at = sqlc.narg(suspended_at)
    WHERE service_id = sqlc.arg(service_id)
        AND callback_type IS NOT DISTINCT
    FROM sqlc.narg(callback_type)
    RETURNING *
)
INSERT INTO service_callback_api (
        id,
        service_id,
        url,
        bearer_token,
        created_at,
        updated_at,
        updated_by_id,
        version,
        callback_type,
        is_suspended,
        suspended_at
    )
SELECT sqlc.arg(id),
    sqlc.arg(service_id),
    sqlc.arg(url),
    sqlc.arg(bearer_token),
    sqlc.arg(created_at),
    sqlc.narg(updated_at),
    sqlc.arg(updated_by_id),
    sqlc.arg(version),
    sqlc.narg(callback_type),
    sqlc.narg(is_suspended),
    sqlc.narg(suspended_at)
WHERE NOT EXISTS (
        SELECT 1
        FROM updated
    )
RETURNING *;
-- name: DeleteCallbackAPI :execrows
DELETE FROM service_callback_api
WHERE id = sqlc.arg(id);
-- name: GetInboundAPI :one
SELECT *
FROM service_inbound_api
WHERE service_id = sqlc.arg(service_id)
LIMIT 1;
-- name: UpsertInboundAPI :one
WITH updated AS (
    UPDATE service_inbound_api
    SET url = sqlc.arg(url),
        bearer_token = sqlc.arg(bearer_token),
        updated_at = sqlc.narg(updated_at),
        updated_by_id = sqlc.arg(updated_by_id),
        version = sqlc.arg(version)
    WHERE service_id = sqlc.arg(service_id)
    RETURNING *
)
INSERT INTO service_inbound_api (
        id,
        service_id,
        url,
        bearer_token,
        created_at,
        updated_at,
        updated_by_id,
        version
    )
SELECT sqlc.arg(id),
    sqlc.arg(service_id),
    sqlc.arg(url),
    sqlc.arg(bearer_token),
    sqlc.arg(created_at),
    sqlc.narg(updated_at),
    sqlc.arg(updated_by_id),
    sqlc.arg(version)
WHERE NOT EXISTS (
        SELECT 1
        FROM updated
    )
RETURNING *;
-- name: DeleteInboundAPI :execrows
DELETE FROM service_inbound_api
WHERE id = sqlc.arg(id);
-- name: InsertServicesHistoryRow :exec
INSERT INTO services_history (
        id,
        name,
        created_at,
        updated_at,
        active,
        message_limit,
        restricted,
        email_from,
        created_by_id,
        version,
        research_mode,
        organisation_type,
        prefix_sms,
        crown,
        rate_limit,
        contact_link,
        consent_to_research,
        volume_email,
        volume_letter,
        volume_sms,
        count_as_live,
        go_live_at,
        go_live_user_id,
        organisation_id,
        sending_domain,
        default_branding_is_french,
        sms_daily_limit,
        organisation_notes,
        sensitive_service,
        email_annual_limit,
        sms_annual_limit,
        suspended_by_id,
        suspended_at
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(name),
        sqlc.arg(created_at),
        sqlc.narg(updated_at),
        sqlc.arg(active),
        sqlc.arg(message_limit),
        sqlc.arg(restricted),
        sqlc.arg(email_from),
        sqlc.arg(created_by_id),
        sqlc.arg(version),
        sqlc.arg(research_mode),
        sqlc.narg(organisation_type),
        sqlc.narg(prefix_sms),
        sqlc.narg(crown),
        sqlc.arg(rate_limit),
        sqlc.narg(contact_link),
        sqlc.narg(consent_to_research),
        sqlc.narg(volume_email),
        sqlc.narg(volume_letter),
        sqlc.narg(volume_sms),
        sqlc.arg(count_as_live),
        sqlc.narg(go_live_at),
        sqlc.narg(go_live_user_id),
        sqlc.narg(organisation_id),
        sqlc.narg(sending_domain),
        sqlc.narg(default_branding_is_french),
        sqlc.arg(sms_daily_limit),
        sqlc.narg(organisation_notes),
        sqlc.narg(sensitive_service),
        sqlc.arg(email_annual_limit),
        sqlc.arg(sms_annual_limit),
        sqlc.narg(suspended_by_id),
        sqlc.narg(suspended_at)
    );
-- name: GetSensitiveServiceIDs :many
SELECT id
FROM services
WHERE sensitive_service = true
ORDER BY id;
-- name: GetMonthlyDataByService :many
SELECT date_trunc('month', n.created_at)::date AS month,
    s.id AS service_id,
    s.name,
    count(n.id) AS notifications_sent
FROM services AS s
    LEFT JOIN notifications AS n ON n.service_id = s.id
WHERE n.created_at >= sqlc.arg(start_at)
    AND n.created_at < sqlc.arg(end_at)
GROUP BY 1,
    2,
    3
ORDER BY 1,
    3;