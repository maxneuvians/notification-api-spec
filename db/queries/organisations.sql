-- name: GetAllOrganisations :many
SELECT *
FROM organisation
ORDER BY name ASC;
-- name: GetOrganisationByID :one
SELECT *
FROM organisation
WHERE id = sqlc.arg(id);
-- name: GetOrganisationByDomain :one
SELECT o.*
FROM organisation AS o
    JOIN domain AS d ON d.organisation_id = o.id
WHERE d.domain = sqlc.arg(domain)
LIMIT 1;
-- name: CreateOrganisation :one
INSERT INTO organisation (
        id,
        name,
        active,
        created_at,
        updated_at,
        email_branding_id,
        letter_branding_id,
        agreement_signed,
        agreement_signed_at,
        agreement_signed_by_id,
        agreement_signed_version,
        crown,
        organisation_type,
        request_to_go_live_notes,
        agreement_signed_on_behalf_of_email_address,
        agreement_signed_on_behalf_of_name,
        default_branding_is_french
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(name),
        sqlc.arg(active),
        sqlc.arg(created_at),
        sqlc.narg(updated_at),
        sqlc.narg(email_branding_id),
        sqlc.narg(letter_branding_id),
        sqlc.narg(agreement_signed),
        sqlc.narg(agreement_signed_at),
        sqlc.narg(agreement_signed_by_id),
        sqlc.narg(agreement_signed_version),
        sqlc.narg(crown),
        sqlc.narg(organisation_type),
        sqlc.narg(request_to_go_live_notes),
        sqlc.narg(agreement_signed_on_behalf_of_email_address),
        sqlc.narg(agreement_signed_on_behalf_of_name),
        sqlc.narg(default_branding_is_french)
    )
RETURNING *;
-- name: UpdateOrganisation :one
UPDATE organisation
SET name = sqlc.arg(name),
    active = sqlc.arg(active),
    updated_at = sqlc.narg(updated_at),
    email_branding_id = sqlc.narg(email_branding_id),
    letter_branding_id = sqlc.narg(letter_branding_id),
    agreement_signed = sqlc.narg(agreement_signed),
    agreement_signed_at = sqlc.narg(agreement_signed_at),
    agreement_signed_by_id = sqlc.narg(agreement_signed_by_id),
    agreement_signed_version = sqlc.narg(agreement_signed_version),
    crown = sqlc.narg(crown),
    organisation_type = sqlc.narg(organisation_type),
    request_to_go_live_notes = sqlc.narg(request_to_go_live_notes),
    agreement_signed_on_behalf_of_email_address = sqlc.narg(agreement_signed_on_behalf_of_email_address),
    agreement_signed_on_behalf_of_name = sqlc.narg(agreement_signed_on_behalf_of_name),
    default_branding_is_french = sqlc.narg(default_branding_is_french)
WHERE id = sqlc.arg(id)
RETURNING *;
-- name: LinkServiceToOrganisation :one
UPDATE services
SET organisation_id = sqlc.arg(organisation_id),
    updated_at = now()
WHERE id = sqlc.arg(service_id)
RETURNING *;
-- name: GetServicesByOrganisationID :many
SELECT *
FROM services
WHERE organisation_id = sqlc.arg(organisation_id)
ORDER BY name ASC;
-- name: AddUserToOrganisation :exec
INSERT INTO user_to_organisation (user_id, organisation_id)
VALUES (sqlc.arg(user_id), sqlc.arg(organisation_id));
-- name: GetUsersByOrganisationID :many
SELECT u.*
FROM users AS u
    JOIN user_to_organisation AS uto ON uto.user_id = u.id
WHERE uto.organisation_id = sqlc.arg(organisation_id)
ORDER BY u.name ASC;
-- name: IsOrganisationNameUnique :one
SELECT NOT EXISTS(
        SELECT 1
        FROM organisation
        WHERE lower(name) = lower(sqlc.arg(name))
            AND id <> coalesce(sqlc.narg(exclude_id), id)
    ) AS is_unique;
-- name: GetInvitedOrgUsers :many
SELECT *
FROM invited_organisation_users
WHERE organisation_id = sqlc.arg(organisation_id)
ORDER BY created_at DESC;
-- name: CreateInvitedOrgUser :one
INSERT INTO invited_organisation_users (
        id,
        email_address,
        invited_by_id,
        organisation_id,
        created_at,
        status
    )
VALUES (
        sqlc.arg(id),
        sqlc.arg(email_address),
        sqlc.arg(invited_by_id),
        sqlc.arg(organisation_id),
        sqlc.arg(created_at),
        sqlc.arg(status)
    )
RETURNING *;
-- name: UpdateInvitedOrgUser :one
UPDATE invited_organisation_users
SET email_address = sqlc.arg(email_address),
    status = sqlc.arg(status)
WHERE id = sqlc.arg(id)
RETURNING *;