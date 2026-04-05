-- name: CreateUser :one
INSERT INTO users (
        id,
        name,
        email_address,
        created_at,
        updated_at,
        _password,
        mobile_number,
        password_changed_at,
        logged_in_at,
        failed_login_count,
        state,
        platform_admin,
        current_session_id,
        auth_type,
        blocked,
        additional_information,
        password_expired,
        verified_phonenumber,
        default_editor_is_rte
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(name),
        sqlc.arg(email_address),
        sqlc.arg(created_at),
        sqlc.narg(updated_at),
        sqlc.arg(password),
        sqlc.narg(mobile_number),
        sqlc.arg(password_changed_at),
        sqlc.narg(logged_in_at),
        sqlc.arg(failed_login_count),
        sqlc.arg(state),
        sqlc.arg(platform_admin),
        sqlc.narg(current_session_id),
        sqlc.arg(auth_type),
        sqlc.arg(blocked),
        sqlc.narg(additional_information),
        sqlc.arg(password_expired),
        sqlc.narg(verified_phonenumber),
        sqlc.arg(default_editor_is_rte)
    )
RETURNING *;
-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = sqlc.arg(id);
-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email_address = sqlc.arg(email_address)
LIMIT 1;
-- name: FindUsersByEmail :many
SELECT *
FROM users
WHERE email_address ILIKE '%' || sqlc.arg(email_query) || '%'
ORDER BY email_address ASC;
-- name: GetAllUsers :many
SELECT *
FROM users
ORDER BY created_at DESC;
-- name: UpdateUser :one
UPDATE users
SET name = sqlc.arg(name),
    email_address = sqlc.arg(email_address),
    updated_at = sqlc.narg(updated_at),
    _password = sqlc.arg(password),
    mobile_number = sqlc.narg(mobile_number),
    password_changed_at = sqlc.arg(password_changed_at),
    logged_in_at = sqlc.narg(logged_in_at),
    failed_login_count = sqlc.arg(failed_login_count),
    state = sqlc.arg(state),
    platform_admin = sqlc.arg(platform_admin),
    current_session_id = sqlc.narg(current_session_id),
    auth_type = sqlc.arg(auth_type),
    blocked = sqlc.arg(blocked),
    additional_information = sqlc.narg(additional_information),
    password_expired = sqlc.arg(password_expired),
    verified_phonenumber = sqlc.narg(verified_phonenumber),
    default_editor_is_rte = sqlc.arg(default_editor_is_rte)
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: ArchiveUser :one
UPDATE users
SET state = 'archived',
    blocked = true,
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: DeactivateUser :one
UPDATE users
SET blocked = true,
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: ActivateUser :one
UPDATE users
SET blocked = false,
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: GetUsersByServiceID :many
SELECT u.*
FROM users AS u
    JOIN user_to_service AS uts ON uts.user_id = u.id
WHERE uts.service_id = sqlc.arg(service_id)
ORDER BY u.name ASC;
-- name: SetUserPermissions :exec
WITH deleted AS (
    DELETE FROM permissions
    WHERE service_id = sqlc.arg(service_id)
        AND user_id = sqlc.arg(user_id)
)
INSERT INTO permissions (id, service_id, user_id, permission, created_at)
SELECT id,
    sqlc.arg(service_id),
    sqlc.arg(user_id),
    permission::public.permission_types,
    created_at
FROM jsonb_to_recordset(sqlc.arg(permission_items)::jsonb) AS t(
        id uuid,
        permission text,
        created_at timestamp
    );
-- name: GetUserPermissions :many
SELECT permission
FROM permissions
WHERE service_id = sqlc.arg(service_id)
    AND user_id = sqlc.arg(user_id)
ORDER BY permission ASC;
-- name: SetFolderPermissions :exec
WITH deleted AS (
    DELETE FROM user_folder_permissions
    WHERE service_id = sqlc.arg(service_id)
        AND user_id = sqlc.arg(user_id)
)
INSERT INTO user_folder_permissions (user_id, template_folder_id, service_id)
SELECT sqlc.arg(user_id),
    folder_id,
    sqlc.arg(service_id)
FROM unnest(sqlc.arg(folder_ids)::uuid []) AS folder_id;
-- name: CreateVerifyCode :one
INSERT INTO verify_codes (
        id,
        user_id,
        _code,
        code_type,
        expiry_datetime,
        code_used,
        created_at
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(user_id),
        sqlc.arg(code),
        sqlc.arg(code_type),
        sqlc.arg(expiry_datetime),
        sqlc.narg(code_used),
        sqlc.arg(created_at)
    )
RETURNING *;
-- name: GetVerifyCode :one
SELECT *
FROM verify_codes
WHERE user_id = sqlc.arg(user_id)
    AND code_type = sqlc.arg(code_type)
    AND coalesce(code_used, false) = false
ORDER BY created_at DESC
LIMIT 1;
-- name: MarkVerifyCodeUsed :one
UPDATE verify_codes
SET code_used = true
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: DeleteExpiredVerifyCodes :execrows
DELETE FROM verify_codes
WHERE expiry_datetime < sqlc.arg(cutoff);
-- name: CreateLoginEvent :one
INSERT INTO login_events (
        id,
        user_id,
        data,
        created_at,
        updated_at
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(user_id),
        sqlc.arg(data),
        sqlc.arg(created_at),
        sqlc.narg(updated_at)
    )
RETURNING *;
-- name: GetLoginEventsByUserID :many
SELECT *
FROM login_events
WHERE user_id = sqlc.arg(user_id)
ORDER BY created_at DESC;
-- name: GetFido2KeysByUserID :many
SELECT *
FROM fido2_keys
WHERE user_id = sqlc.arg(user_id)
ORDER BY created_at DESC;
-- name: CreateFido2Key :one
INSERT INTO fido2_keys (
        id,
        user_id,
        name,
        key,
        created_at,
        updated_at
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(user_id),
        sqlc.arg(name),
        sqlc.arg(key),
        sqlc.arg(created_at),
        sqlc.narg(updated_at)
    )
RETURNING *;
-- name: DeleteFido2Key :execrows
DELETE FROM fido2_keys
WHERE id = sqlc.arg(id);
-- name: CreateFido2Session :one
INSERT INTO fido2_sessions (user_id, session, created_at)
VALUES (
        sqlc.arg(user_id),
        sqlc.arg(session),
        sqlc.arg(created_at)
    ) ON CONFLICT (user_id) DO
UPDATE
SET session = EXCLUDED.session,
    created_at = EXCLUDED.created_at
RETURNING *;
-- name: GetFido2Session :one
SELECT *
FROM fido2_sessions
WHERE user_id = sqlc.arg(user_id);
-- name: GetInvitedUsersByServiceID :many
SELECT *
FROM invited_users
WHERE service_id = sqlc.arg(service_id)
ORDER BY created_at DESC;