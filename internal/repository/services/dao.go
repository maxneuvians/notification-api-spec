package services

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/maxneuvians/notification-api-spec/internal/config"
	apiKeysRepo "github.com/maxneuvians/notification-api-spec/internal/repository/api_keys"
	inboundRepo "github.com/maxneuvians/notification-api-spec/internal/repository/inbound"
	organisationsRepo "github.com/maxneuvians/notification-api-spec/internal/repository/organisations"
	templatesRepo "github.com/maxneuvians/notification-api-spec/internal/repository/templates"
	usersRepo "github.com/maxneuvians/notification-api-spec/internal/repository/users"
	serviceerrs "github.com/maxneuvians/notification-api-spec/internal/service/services"
	"github.com/maxneuvians/notification-api-spec/pkg/signing"
)

const listAPIKeysByServiceQuery = `
SELECT id, name, secret, service_id, expiry_date, created_at, created_by_id, updated_at, version, key_type, compromised_key_info, last_used_timestamp
FROM api_keys
WHERE service_id = $1
`

const insertCallbackAPIHistoryQuery = `
INSERT INTO service_callback_api_history (
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
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
`

const insertInboundAPIHistoryQuery = `
INSERT INTO service_inbound_api_history (
	id,
	service_id,
	url,
	bearer_token,
	created_at,
	updated_at,
	updated_by_id,
	version
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
`

const listSignedCallbackAPIsForResignQuery = `
SELECT id, service_id, url, bearer_token, created_at, updated_at, updated_by_id, version, callback_type, is_suspended, suspended_at
FROM service_callback_api
WHERE bearer_token IS NOT NULL
	AND bearer_token <> ''
ORDER BY created_at ASC
`

const updateCallbackAPIBearerTokenQuery = `
UPDATE service_callback_api
SET bearer_token = $1
WHERE id = $2
`

const listLetterContactsByServiceQuery = `
SELECT id, service_id, contact_block, is_default, created_at, updated_at, archived
FROM service_letter_contacts
WHERE service_id = $1
	AND archived = false
ORDER BY is_default DESC, created_at DESC
`

const getLetterContactByIDQuery = `
SELECT id, service_id, contact_block, is_default, created_at, updated_at, archived
FROM service_letter_contacts
WHERE service_id = $1
	AND id = $2
	AND archived = false
`

const createLetterContactQuery = `
INSERT INTO service_letter_contacts (
		id,
		service_id,
		contact_block,
		is_default,
		created_at,
		updated_at,
		archived
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, service_id, contact_block, is_default, created_at, updated_at, archived
`

const updateLetterContactQuery = `
UPDATE service_letter_contacts
SET contact_block = $1,
		is_default = $2,
		updated_at = now(),
		archived = $3
WHERE id = $4
	AND service_id = $5
RETURNING id, service_id, contact_block, is_default, created_at, updated_at, archived
`

const listActiveUsersByServiceQuery = `
SELECT u.id, u.name, u.email_address, u.created_at, u.updated_at, u._password, u.mobile_number, u.password_changed_at, u.logged_in_at, u.failed_login_count, u.state, u.platform_admin, u.current_session_id, u.auth_type, u.blocked, u.additional_information, u.password_expired, u.verified_phonenumber, u.default_editor_is_rte
FROM users AS u
	JOIN user_to_service AS uts ON uts.user_id = u.id
WHERE uts.service_id = $1
	AND u.state = 'active'
ORDER BY u.name ASC
`

const getServiceCreatorQuery = `
SELECT u.id, u.name, u.email_address, u.created_at, u.updated_at, u._password, u.mobile_number, u.password_changed_at, u.logged_in_at, u.failed_login_count, u.state, u.platform_admin, u.current_session_id, u.auth_type, u.blocked, u.additional_information, u.password_expired, u.verified_phonenumber, u.default_editor_is_rte
FROM users AS u
	JOIN services_history AS sh ON sh.created_by_id = u.id
WHERE sh.id = $1
	AND sh.version = (
		SELECT MIN(version)
		FROM services_history
		WHERE id = $1
	)
LIMIT 1
`

const listTodaysStatsForAllServicesQuery = `
SELECT s.id,
	s.name,
	s.restricted,
	s.research_mode,
	s.active,
	s.created_at,
	daily.notification_type,
	daily.notification_status,
	COALESCE(daily.notification_count, 0)::bigint
FROM services AS s
LEFT JOIN (
	SELECT n.service_id,
		n.notification_type,
		n.notification_status,
		COUNT(*)::bigint AS notification_count
	FROM notifications AS n
	WHERE (n.created_at AT TIME ZONE 'America/Toronto')::date = (now() AT TIME ZONE 'America/Toronto')::date
		AND ($1::boolean OR n.key_type <> 'test')
	GROUP BY n.service_id,
		n.notification_type,
		n.notification_status
) AS daily ON daily.service_id = s.id
WHERE ($2::boolean OR s.active = true)
ORDER BY s.created_at ASC,
	daily.notification_type,
	daily.notification_status
`

const listStatsForAllServicesByDateRangeQuery = `
SELECT s.id,
	s.name,
	s.restricted,
	s.research_mode,
	s.active,
	s.created_at,
	ranged.notification_type,
	ranged.notification_status,
	COALESCE(ranged.notification_count, 0)::bigint
FROM services AS s
LEFT JOIN (
	SELECT n.service_id,
		n.notification_type,
		n.notification_status,
		COUNT(*)::bigint AS notification_count
	FROM notifications AS n
	WHERE (n.created_at AT TIME ZONE 'America/Toronto')::date >= $3::date
		AND (n.created_at AT TIME ZONE 'America/Toronto')::date <= $4::date
		AND ($1::boolean OR n.key_type <> 'test')
	GROUP BY n.service_id,
		n.notification_type,
		n.notification_status
) AS ranged ON ranged.service_id = s.id
WHERE ($2::boolean OR s.active = true)
ORDER BY s.created_at ASC,
	ranged.notification_type,
	ranged.notification_status
`

const listStatsForServiceQuery = `
SELECT n.notification_type,
	n.notification_status,
	COUNT(*)::bigint AS notification_count
FROM notifications AS n
WHERE n.service_id = $1
	AND n.key_type <> 'test'
	AND (n.created_at AT TIME ZONE 'America/Toronto')::date >= ((now() AT TIME ZONE 'America/Toronto')::date - GREATEST($2::int - 1, 0))
GROUP BY n.notification_type,
	n.notification_status
ORDER BY n.notification_type,
	n.notification_status
`

const listTodaysStatsForServiceQuery = `
SELECT n.notification_type,
	n.notification_status,
	COUNT(*)::bigint AS notification_count
FROM notifications AS n
WHERE n.service_id = $1
	AND n.key_type <> 'test'
	AND (n.created_at AT TIME ZONE 'America/Toronto')::date = (now() AT TIME ZONE 'America/Toronto')::date
GROUP BY n.notification_type,
	n.notification_status
ORDER BY n.notification_type,
	n.notification_status
`

const getTodaysTotalMessageCountQuery = `
SELECT COALESCE((
		SELECT COUNT(*)::bigint
		FROM notifications AS n
		WHERE n.service_id = $1
			AND n.key_type <> 'test'
			AND n.created_at >= date_trunc('day', now() AT TIME ZONE 'UTC')
			AND n.created_at < date_trunc('day', now() AT TIME ZONE 'UTC') + interval '1 day'
	), 0) + COALESCE((
		SELECT SUM(j.notification_count)::bigint
		FROM jobs AS j
		WHERE j.service_id = $1
			AND j.job_status = 'scheduled'
			AND j.scheduled_for >= date_trunc('day', now() AT TIME ZONE 'UTC')
			AND j.scheduled_for < date_trunc('day', now() AT TIME ZONE 'UTC') + interval '1 day'
	), 0)
`

const getTodaysTotalSMSCountQuery = `
SELECT COALESCE(COUNT(*)::bigint, 0)
FROM notifications AS n
WHERE n.service_id = $1
	AND n.key_type <> 'test'
	AND n.notification_type = 'sms'
	AND n.created_at >= date_trunc('day', now() AT TIME ZONE 'UTC')
	AND n.created_at < date_trunc('day', now() AT TIME ZONE 'UTC') + interval '1 day'
`

const getTodaysTotalSMSBillableUnitsQuery = `
SELECT COALESCE(SUM(n.billable_units)::bigint, 0)
FROM notifications AS n
WHERE n.service_id = $1
	AND n.key_type <> 'test'
	AND n.notification_type = 'sms'
	AND n.created_at >= date_trunc('day', now() AT TIME ZONE 'UTC')
	AND n.created_at < date_trunc('day', now() AT TIME ZONE 'UTC') + interval '1 day'
`

const getServiceEmailLimitQuery = `
SELECT message_limit::bigint
FROM services
WHERE id = $1
`

const getTodaysTotalEmailCountQuery = `
SELECT COALESCE((
		SELECT COUNT(*)::bigint
		FROM notifications AS n
		WHERE n.service_id = $1
			AND n.key_type <> 'test'
			AND n.notification_type = 'email'
			AND n.created_at >= date_trunc('day', now() AT TIME ZONE 'UTC')
			AND n.created_at < date_trunc('day', now() AT TIME ZONE 'UTC') + interval '1 day'
	), 0) + COALESCE((
		SELECT SUM(j.notification_count)::bigint
		FROM jobs AS j
		JOIN templates AS t ON t.id = j.template_id
		WHERE j.service_id = $1
			AND j.job_status = 'scheduled'
			AND j.scheduled_for >= date_trunc('day', now() AT TIME ZONE 'UTC')
			AND j.scheduled_for < date_trunc('day', now() AT TIME ZONE 'UTC') + interval '1 day'
			AND t.template_type = 'email'
	), 0)
`

const listTemplateFoldersByIDsQuery = `
SELECT id, service_id
FROM template_folder
WHERE id = ANY($1::uuid[])
`

const getDataRetentionByIDQuery = `
SELECT id, service_id, notification_type, days_of_retention, created_at, updated_at
FROM service_data_retention
WHERE service_id = $1
	AND id = $2
LIMIT 1
`

const listServiceHistoryByServiceIDQuery = `
SELECT id, name, created_at, updated_at, active, message_limit, restricted, email_from, created_by_id, version, research_mode, organisation_type, prefix_sms, crown, rate_limit, contact_link, consent_to_research, volume_email, volume_letter, volume_sms, count_as_live, go_live_at, go_live_user_id, organisation_id, sending_domain, default_branding_is_french, sms_daily_limit, organisation_notes, sensitive_service, email_annual_limit, sms_annual_limit, suspended_by_id, suspended_at
FROM services_history
WHERE id = $1
ORDER BY version DESC
`

const listAPIKeyHistoryByServiceIDQuery = `
SELECT id, name, secret, service_id, expiry_date, created_at, updated_at, created_by_id, version, key_type, compromised_key_info, last_used_timestamp
FROM api_keys_history
WHERE service_id = $1
ORDER BY created_at DESC, version DESC
`

const isServiceNameUniqueQuery = `
SELECT NOT EXISTS (
	SELECT 1
	FROM services
	WHERE regexp_replace(lower(name), '[^a-z0-9]+', '', 'g') = regexp_replace(lower($1), '[^a-z0-9]+', '', 'g')
		AND id <> $2
)
`

const isServiceEmailFromUniqueQuery = `
SELECT NOT EXISTS (
	SELECT 1
	FROM services
	WHERE lower(email_from) = lower($1)
		AND id <> $2
)
`

const getServiceOrganisationQuery = `
SELECT o.id, o.name, o.active, o.created_at, o.updated_at, o.email_branding_id, o.letter_branding_id, o.agreement_signed, o.agreement_signed_at, o.agreement_signed_by_id, o.agreement_signed_version, o.crown, o.organisation_type, o.request_to_go_live_notes, o.agreement_signed_on_behalf_of_email_address, o.agreement_signed_on_behalf_of_name, o.default_branding_is_french
FROM organisation AS o
JOIN services AS s ON s.organisation_id = o.id
WHERE s.id = $1
LIMIT 1
`

const getOrganisationByEmailDomainQuery = `
SELECT o.id, o.name, o.active, o.created_at, o.updated_at, o.email_branding_id, o.letter_branding_id, o.agreement_signed, o.agreement_signed_at, o.agreement_signed_by_id, o.agreement_signed_version, o.crown, o.organisation_type, o.request_to_go_live_notes, o.agreement_signed_on_behalf_of_email_address, o.agreement_signed_on_behalf_of_name, o.default_branding_is_french
FROM organisation AS o
JOIN domain AS d ON d.organisation_id = o.id
WHERE lower($1) = lower(d.domain)
	OR lower($1) LIKE ('%.' || lower(d.domain))
ORDER BY length(d.domain) DESC
LIMIT 1
`

const getEmailBrandingIDByNameQuery = `
SELECT id
FROM email_branding
WHERE lower(name) = lower($1)
ORDER BY created_at ASC
LIMIT 1
`

const getLetterBrandingIDByNameQuery = `
SELECT id
FROM letter_branding
WHERE lower(name) = lower($1)
ORDER BY name ASC
LIMIT 1
`

const upsertServiceEmailBrandingQuery = `
INSERT INTO service_email_branding (service_id, email_branding_id)
VALUES ($1, $2)
ON CONFLICT (service_id) DO UPDATE SET email_branding_id = EXCLUDED.email_branding_id
`

const upsertServiceLetterBrandingQuery = `
INSERT INTO service_letter_branding (service_id, letter_branding_id)
VALUES ($1, $2)
ON CONFLICT (service_id) DO UPDATE SET letter_branding_id = EXCLUDED.letter_branding_id
`

const getAnnualLimitStatsQuery = `
SELECT
	COALESCE(SUM(CASE WHEN n.notification_type = 'email' AND n.notification_status = 'delivered' THEN 1 ELSE 0 END), 0)::bigint AS email_delivered_today,
	COALESCE(SUM(CASE WHEN n.notification_type = 'email' AND n.notification_status IN ('failed', 'technical-failure', 'temporary-failure', 'permanent-failure') THEN 1 ELSE 0 END), 0)::bigint AS email_failed_today,
	COALESCE(SUM(CASE WHEN n.notification_type = 'sms' AND n.notification_status = 'delivered' THEN 1 ELSE 0 END), 0)::bigint AS sms_delivered_today,
	COALESCE(SUM(CASE WHEN n.notification_type = 'sms' AND n.notification_status IN ('failed', 'technical-failure', 'temporary-failure', 'permanent-failure') THEN 1 ELSE 0 END), 0)::bigint AS sms_failed_today,
	COALESCE((
		SELECT COUNT(*)::bigint
		FROM notifications AS fiscal
		WHERE fiscal.service_id = $1
			AND fiscal.key_type <> 'test'
			AND fiscal.notification_type = 'email'
			AND fiscal.created_at >= $2
			AND fiscal.created_at < $3
	), 0) AS total_email_fiscal_year_to_yesterday,
	COALESCE((
		SELECT COUNT(*)::bigint
		FROM notifications AS fiscal
		WHERE fiscal.service_id = $1
			AND fiscal.key_type <> 'test'
			AND fiscal.notification_type = 'sms'
			AND fiscal.created_at >= $2
			AND fiscal.created_at < $3
	), 0) AS total_sms_fiscal_year_to_yesterday
FROM notifications AS n
WHERE n.service_id = $1
	AND n.key_type <> 'test'
	AND n.created_at >= $3
	AND n.created_at < $4
`

const listMonthlyUsageForServiceQuery = `
SELECT month_label,
	notification_type,
	notification_status,
	SUM(notification_count)::bigint AS notification_count
FROM (
	SELECT to_char(fns.bst_date, 'YYYY-MM') AS month_label,
		fns.notification_type,
		fns.notification_status,
		SUM(fns.notification_count)::bigint AS notification_count
	FROM ft_notification_status AS fns
	WHERE fns.service_id = $1
		AND fns.key_type <> 'test'
		AND fns.bst_date >= $2::date
		AND fns.bst_date < $3::date
	GROUP BY 1, 2, 3

	UNION ALL

	SELECT to_char((n.created_at AT TIME ZONE 'America/Toronto'), 'YYYY-MM') AS month_label,
		n.notification_type,
		n.notification_status,
		COUNT(*)::bigint AS notification_count
	FROM notifications AS n
	WHERE n.service_id = $1
		AND n.key_type <> 'test'
		AND (n.created_at AT TIME ZONE 'America/Toronto')::date >= $2::date
		AND (n.created_at AT TIME ZONE 'America/Toronto')::date < $3::date
		AND (n.created_at AT TIME ZONE 'America/Toronto')::date = (now() AT TIME ZONE 'America/Toronto')::date
	GROUP BY 1, 2, 3
) AS monthly_usage
GROUP BY month_label, notification_type, notification_status
ORDER BY month_label, notification_type, notification_status
`

const getDataRetentionByNotificationTypeQuery = `
SELECT id, service_id, notification_type, days_of_retention, created_at, updated_at
FROM service_data_retention
WHERE service_id = $1
	AND notification_type = $2
LIMIT 1
`

const createDataRetentionQuery = `
INSERT INTO service_data_retention (
		id,
		service_id,
		notification_type,
		days_of_retention,
		created_at,
		updated_at
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, service_id, notification_type, days_of_retention, created_at, updated_at
`

const updateDataRetentionByIDQuery = `
UPDATE service_data_retention
SET days_of_retention = $1,
	updated_at = now()
WHERE service_id = $2
	AND id = $3
`

const deleteSafelistQuery = `
DELETE FROM service_safelist
WHERE service_id = $1
`

const (
	errSMSDefaultRequired      = "You must have at least one SMS sender as the default."
	errReplyToDefaultRequired  = "You must have at least one reply to email address as the default."
	errInboundSenderImmutable  = "You cannot update an inbound number"
	errInboundSenderArchive    = "You cannot delete an inbound number"
	errDefaultReplyToArchive   = "You cannot delete a default email reply to address if other reply to addresses exist"
	errDefaultSMSSenderArchive = "You cannot delete a default sms sender"
	callbackTypeDeliveryStatus = "delivery_status"
	callbackTypeComplaint      = "complaint"
)

type Repository struct {
	readerDB           DBTX
	writerDB           DBTX
	reader             *Queries
	writer             *Queries
	platformFromNumber string
	apiKeyPrefix       string
	apiKeySecrets      []string
	dangerousSalt      string
}

type CreatedAPIKey struct {
	APIKey apiKeysRepo.ApiKey
	Key    string
}

type TodayStatsForAllServicesRow struct {
	ServiceID          uuid.UUID            `json:"service_id"`
	Name               string               `json:"name"`
	Restricted         bool                 `json:"restricted"`
	ResearchMode       bool                 `json:"research_mode"`
	Active             bool                 `json:"active"`
	CreatedAt          time.Time            `json:"created_at"`
	NotificationType   NullNotificationType `json:"notification_type"`
	NotificationStatus NullNotifyStatusType `json:"notification_status"`
	Count              int64                `json:"count"`
}

type ServiceStatsRow struct {
	NotificationType   NotificationType     `json:"notification_type"`
	NotificationStatus NullNotifyStatusType `json:"notification_status"`
	Count              int64                `json:"count"`
}

type AnnualLimitStats struct {
	EmailDeliveredToday        int64 `json:"email_delivered_today"`
	EmailFailedToday           int64 `json:"email_failed_today"`
	SmsDeliveredToday          int64 `json:"sms_delivered_today"`
	SmsFailedToday             int64 `json:"sms_failed_today"`
	TotalEmailFiscalYearToYday int64 `json:"total_email_fiscal_year_to_yesterday"`
	TotalSmsFiscalYearToYday   int64 `json:"total_sms_fiscal_year_to_yesterday"`
}

type MonthlyUsageRow struct {
	Month              string               `json:"month"`
	NotificationType   NullNotificationType `json:"notification_type"`
	NotificationStatus NullNotifyStatusType `json:"notification_status"`
	Count              int64                `json:"count"`
}

var defaultUserPermissions = []string{
	string(usersRepo.PermissionTypesManageUsers),
	string(usersRepo.PermissionTypesManageTemplates),
	string(usersRepo.PermissionTypesManageSettings),
	string(usersRepo.PermissionTypesSendTexts),
	string(usersRepo.PermissionTypesSendEmails),
	string(usersRepo.PermissionTypesSendLetters),
	string(usersRepo.PermissionTypesManageApiKeys),
	string(usersRepo.PermissionTypesViewActivity),
}

type txStarter interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

type Option func(*Repository)

func WithPlatformFromNumber(fromNumber string) Option {
	return func(r *Repository) {
		r.platformFromNumber = strings.TrimSpace(fromNumber)
	}
}

func WithConfig(cfg *config.Config) Option {
	if cfg == nil {
		return func(*Repository) {}
	}
	return func(r *Repository) {
		WithPlatformFromNumber(cfg.PlatformFromNumber)(r)
		r.apiKeyPrefix = cfg.APIKeyPrefix
		r.apiKeySecrets = append([]string(nil), cfg.SecretKeys...)
		r.dangerousSalt = cfg.DangerousSalt
	}
}

func NewRepository(readerDB, writerDB DBTX, options ...Option) *Repository {
	reader := New(readerDB)
	writer := reader
	if writerDB != nil {
		writer = New(writerDB)
	} else {
		writerDB = readerDB
	}
	repo := &Repository{readerDB: readerDB, writerDB: writerDB, reader: reader, writer: writer}
	for _, option := range options {
		option(repo)
	}
	return repo
}

func (r *Repository) FetchAllServices(ctx context.Context, onlyActive bool) ([]Service, error) {
	items, err := r.reader.GetAllServices(ctx, onlyActive)
	if err != nil {
		return nil, err
	}
	sortServicesByCreatedAt(items)
	return items, nil
}

func (r *Repository) GetServicesByPartialName(ctx context.Context, nameQuery string) ([]Service, error) {
	items, err := r.reader.GetServicesByPartialName(ctx, sql.NullString{String: nameQuery, Valid: nameQuery != ""})
	if err != nil {
		return nil, err
	}
	sortServicesByCreatedAt(items)
	return items, nil
}

func (r *Repository) CountLiveServices(ctx context.Context) (int64, error) {
	return r.reader.CountLiveServices(ctx)
}

func (r *Repository) FetchServiceByID(ctx context.Context, id uuid.UUID, onlyActive bool) (*Service, error) {
	item, err := r.reader.GetServiceByID(ctx, GetServiceByIDParams{ID: id, OnlyActive: onlyActive})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *Repository) FetchServiceByIDAndUser(ctx context.Context, serviceID uuid.UUID, userID uuid.UUID) (*Service, error) {
	item, err := r.reader.GetServiceByIDAndUser(ctx, GetServiceByIDAndUserParams{ServiceID: serviceID, UserID: uuid.NullUUID{UUID: userID, Valid: true}})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *Repository) FetchServicesByUserID(ctx context.Context, userID uuid.UUID, onlyActive bool) ([]Service, error) {
	items, err := r.reader.GetServicesByUserID(ctx, GetServicesByUserIDParams{UserID: uuid.NullUUID{UUID: userID, Valid: true}, OnlyActive: onlyActive})
	if err != nil {
		return nil, err
	}
	sortServicesByCreatedAt(items)
	return items, nil
}

func (r *Repository) FetchServiceByInboundNumber(ctx context.Context, number string) (*Service, error) {
	item, err := r.reader.GetServiceByInboundNumber(ctx, number)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *Repository) FetchServiceWithAPIKeys(ctx context.Context, id uuid.UUID) (*Service, error) {
	item, err := r.reader.GetServiceByIDWithAPIKeys(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateAPIKey(ctx context.Context, serviceID uuid.UUID, name string, createdByID uuid.UUID, keyType string) (*CreatedAPIKey, error) {
	secret, err := r.currentAPIKeySecret()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(r.apiKeyPrefix) == "" {
		return nil, fmt.Errorf("api key prefix is required")
	}
	if strings.TrimSpace(keyType) == "" {
		return nil, fmt.Errorf("key type is required")
	}

	plaintextToken := uuid.NewString()
	hashedSecret, err := signing.SignAPIKeyToken(plaintextToken, secret)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Truncate(time.Second)
	queries := apiKeysRepo.New(r.writerDB)
	created, err := queries.CreateAPIKey(ctx, apiKeysRepo.CreateAPIKeyParams{
		ID:                 uuid.New(),
		Name:               name,
		Secret:             hashedSecret,
		ServiceID:          serviceID,
		ExpiryDate:         sql.NullTime{},
		CreatedAt:          now,
		CreatedByID:        createdByID,
		UpdatedAt:          sql.NullTime{},
		Version:            1,
		KeyType:            keyType,
		CompromisedKeyInfo: json.RawMessage(`null`),
		LastUsedTimestamp:  sql.NullTime{},
	})
	if err != nil {
		return nil, err
	}
	if err := queries.InsertAPIKeyHistory(ctx, apiKeyHistoryParamsFromKey(created)); err != nil {
		return nil, err
	}

	return &CreatedAPIKey{
		APIKey: created,
		Key:    r.apiKeyPrefix + serviceID.String() + plaintextToken,
	}, nil
}

func (r *Repository) ListAPIKeys(ctx context.Context, serviceID uuid.UUID, keyID *uuid.UUID) ([]apiKeysRepo.ApiKey, error) {
	query := listAPIKeysByServiceQuery
	args := []any{serviceID}
	if keyID != nil {
		query += " AND id = $2"
		args = append(args, *keyID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.readerDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]apiKeysRepo.ApiKey, 0)
	for rows.Next() {
		var item apiKeysRepo.ApiKey
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Secret,
			&item.ServiceID,
			&item.ExpiryDate,
			&item.CreatedAt,
			&item.CreatedByID,
			&item.UpdatedAt,
			&item.Version,
			&item.KeyType,
			&item.CompromisedKeyInfo,
			&item.LastUsedTimestamp,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) FetchActiveUsersForService(ctx context.Context, serviceID uuid.UUID) ([]usersRepo.User, error) {
	rows, err := r.readerDB.QueryContext(ctx, listActiveUsersByServiceQuery, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []usersRepo.User
	for rows.Next() {
		var item usersRepo.User
		if err := scanUser(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) FetchServiceCreator(ctx context.Context, serviceID uuid.UUID) (*usersRepo.User, error) {
	row := r.readerDB.QueryRowContext(ctx, getServiceCreatorQuery, serviceID)
	var item usersRepo.User
	if err := scanUser(row, &item); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *Repository) FetchLiveServicesData(ctx context.Context) ([]GetLiveServicesDataRow, error) {
	return r.reader.GetLiveServicesData(ctx)
}

func (r *Repository) FetchSensitiveServiceIDs(ctx context.Context) ([]uuid.UUID, error) {
	return r.reader.GetSensitiveServiceIDs(ctx)
}

func (r *Repository) FetchTodaysStatsForAllServices(ctx context.Context, includeFromTestKey bool, onlyActive bool) ([]TodayStatsForAllServicesRow, error) {
	now := time.Now().In(loadStatsLocation())
	return r.FetchStatsForAllServices(ctx, includeFromTestKey, onlyActive, now, now)
}

func (r *Repository) FetchStatsForAllServices(ctx context.Context, includeFromTestKey bool, onlyActive bool, startDate time.Time, endDate time.Time) ([]TodayStatsForAllServicesRow, error) {
	start := normalizeDateOnly(startDate)
	end := normalizeDateOnly(endDate)
	rows, err := r.readerDB.QueryContext(ctx, listStatsForAllServicesByDateRangeQuery, includeFromTestKey, onlyActive, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []TodayStatsForAllServicesRow
	for rows.Next() {
		var item TodayStatsForAllServicesRow
		if err := rows.Scan(
			&item.ServiceID,
			&item.Name,
			&item.Restricted,
			&item.ResearchMode,
			&item.Active,
			&item.CreatedAt,
			&item.NotificationType,
			&item.NotificationStatus,
			&item.Count,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) FetchStatsForService(ctx context.Context, serviceID uuid.UUID, limitDays int) ([]ServiceStatsRow, error) {
	rows, err := r.readerDB.QueryContext(ctx, listStatsForServiceQuery, serviceID, limitDays)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ServiceStatsRow
	for rows.Next() {
		var item ServiceStatsRow
		if err := rows.Scan(&item.NotificationType, &item.NotificationStatus, &item.Count); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func loadStatsLocation() *time.Location {
	location, err := time.LoadLocation("America/Toronto")
	if err != nil {
		return time.UTC
	}
	return location
}

func normalizeDateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func emailDomain(value string) string {
	parts := strings.Split(strings.TrimSpace(value), "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

func scanOptionalUUID(scanner interface{ Scan(dest ...any) error }) (*uuid.UUID, error) {
	var value uuid.UUID
	if err := scanner.Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &value, nil
}

func (r *Repository) FetchTodaysStatsForService(ctx context.Context, serviceID uuid.UUID) ([]ServiceStatsRow, error) {
	rows, err := r.readerDB.QueryContext(ctx, listTodaysStatsForServiceQuery, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ServiceStatsRow
	for rows.Next() {
		var item ServiceStatsRow
		if err := rows.Scan(&item.NotificationType, &item.NotificationStatus, &item.Count); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) FetchTodaysTotalMessageCount(ctx context.Context, serviceID uuid.UUID) (int64, error) {
	return scanInt64(rowOrQuery(r.readerDB, ctx, getTodaysTotalMessageCountQuery, serviceID))
}

func (r *Repository) FetchTodaysTotalSmsCount(ctx context.Context, serviceID uuid.UUID) (int64, error) {
	return scanInt64(rowOrQuery(r.readerDB, ctx, getTodaysTotalSMSCountQuery, serviceID))
}

func (r *Repository) FetchTodaysTotalSmsBillableUnits(ctx context.Context, serviceID uuid.UUID) (int64, error) {
	return scanInt64(rowOrQuery(r.readerDB, ctx, getTodaysTotalSMSBillableUnitsQuery, serviceID))
}

func (r *Repository) FetchServiceEmailLimit(ctx context.Context, serviceID uuid.UUID) (int64, error) {
	row := r.readerDB.QueryRowContext(ctx, getServiceEmailLimitQuery, serviceID)
	var value int64
	if err := row.Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return value, nil
}

func (r *Repository) FetchTodaysTotalEmailCount(ctx context.Context, serviceID uuid.UUID) (int64, error) {
	return scanInt64(rowOrQuery(r.readerDB, ctx, getTodaysTotalEmailCountQuery, serviceID))
}

func (r *Repository) FetchServiceHistory(ctx context.Context, serviceID uuid.UUID) ([]ServicesHistory, error) {
	rows, err := r.readerDB.QueryContext(ctx, listServiceHistoryByServiceIDQuery, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ServicesHistory
	for rows.Next() {
		var item ServicesHistory
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.Active,
			&item.MessageLimit,
			&item.Restricted,
			&item.EmailFrom,
			&item.CreatedByID,
			&item.Version,
			&item.ResearchMode,
			&item.OrganisationType,
			&item.PrefixSms,
			&item.Crown,
			&item.RateLimit,
			&item.ContactLink,
			&item.ConsentToResearch,
			&item.VolumeEmail,
			&item.VolumeLetter,
			&item.VolumeSms,
			&item.CountAsLive,
			&item.GoLiveAt,
			&item.GoLiveUserID,
			&item.OrganisationID,
			&item.SendingDomain,
			&item.DefaultBrandingIsFrench,
			&item.SmsDailyLimit,
			&item.OrganisationNotes,
			&item.SensitiveService,
			&item.EmailAnnualLimit,
			&item.SmsAnnualLimit,
			&item.SuspendedByID,
			&item.SuspendedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) FetchAPIKeyHistory(ctx context.Context, serviceID uuid.UUID) ([]apiKeysRepo.APIKeyHistory, error) {
	rows, err := r.readerDB.QueryContext(ctx, listAPIKeyHistoryByServiceIDQuery, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []apiKeysRepo.APIKeyHistory
	for rows.Next() {
		var item apiKeysRepo.APIKeyHistory
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Secret,
			&item.ServiceID,
			&item.ExpiryDate,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.CreatedByID,
			&item.Version,
			&item.KeyType,
			&item.CompromisedKeyInfo,
			&item.LastUsedTimestamp,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) IsServiceNameUnique(ctx context.Context, name string, serviceID uuid.UUID) (bool, error) {
	var unique bool
	if err := r.readerDB.QueryRowContext(ctx, isServiceNameUniqueQuery, name, serviceID).Scan(&unique); err != nil {
		return false, err
	}
	return unique, nil
}

func (r *Repository) IsServiceEmailFromUnique(ctx context.Context, emailFrom string, serviceID uuid.UUID) (bool, error) {
	var unique bool
	if err := r.readerDB.QueryRowContext(ctx, isServiceEmailFromUniqueQuery, emailFrom, serviceID).Scan(&unique); err != nil {
		return false, err
	}
	return unique, nil
}

func (r *Repository) FetchServiceOrganisation(ctx context.Context, serviceID uuid.UUID) (*organisationsRepo.Organisation, error) {
	row := r.readerDB.QueryRowContext(ctx, getServiceOrganisationQuery, serviceID)
	var item organisationsRepo.Organisation
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.Active,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.EmailBrandingID,
		&item.LetterBrandingID,
		&item.AgreementSigned,
		&item.AgreementSignedAt,
		&item.AgreementSignedByID,
		&item.AgreementSignedVersion,
		&item.Crown,
		&item.OrganisationType,
		&item.RequestToGoLiveNotes,
		&item.AgreementSignedOnBehalfOfEmailAddress,
		&item.AgreementSignedOnBehalfOfName,
		&item.DefaultBrandingIsFrench,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *Repository) FetchUserByID(ctx context.Context, userID uuid.UUID) (*usersRepo.User, error) {
	item, err := usersRepo.New(r.readerDB).GetUserByID(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *Repository) FetchOrganisationByEmailAddress(ctx context.Context, emailAddress string) (*organisationsRepo.Organisation, error) {
	domain := strings.ToLower(strings.TrimSpace(emailDomain(emailAddress)))
	if domain == "" {
		return nil, nil
	}
	row := r.readerDB.QueryRowContext(ctx, getOrganisationByEmailDomainQuery, domain)
	var item organisationsRepo.Organisation
	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.Active,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.EmailBrandingID,
		&item.LetterBrandingID,
		&item.AgreementSigned,
		&item.AgreementSignedAt,
		&item.AgreementSignedByID,
		&item.AgreementSignedVersion,
		&item.Crown,
		&item.OrganisationType,
		&item.RequestToGoLiveNotes,
		&item.AgreementSignedOnBehalfOfEmailAddress,
		&item.AgreementSignedOnBehalfOfName,
		&item.DefaultBrandingIsFrench,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *Repository) FetchNHSEmailBrandingID(ctx context.Context) (*uuid.UUID, error) {
	return scanOptionalUUID(rowOrQuery(r.readerDB, ctx, getEmailBrandingIDByNameQuery, "NHS"))
}

func (r *Repository) FetchNHSLetterBrandingID(ctx context.Context) (*uuid.UUID, error) {
	return scanOptionalUUID(rowOrQuery(r.readerDB, ctx, getLetterBrandingIDByNameQuery, "NHS"))
}

func (r *Repository) AssignServiceBranding(ctx context.Context, serviceID uuid.UUID, emailBrandingID *uuid.UUID, letterBrandingID *uuid.UUID) error {
	if emailBrandingID != nil {
		if _, err := r.writerDB.ExecContext(ctx, upsertServiceEmailBrandingQuery, serviceID, *emailBrandingID); err != nil {
			return err
		}
	}
	if letterBrandingID != nil {
		if _, err := r.writerDB.ExecContext(ctx, upsertServiceLetterBrandingQuery, serviceID, *letterBrandingID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) FetchAnnualLimitStats(ctx context.Context, serviceID uuid.UUID) (*AnnualLimitStats, error) {
	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	fiscalYearStartYear := now.Year()
	if now.Month() < time.April {
		fiscalYearStartYear--
	}
	fiscalStart := time.Date(fiscalYearStartYear, time.April, 1, 0, 0, 0, 0, time.UTC)
	row := r.readerDB.QueryRowContext(ctx, getAnnualLimitStatsQuery, serviceID, fiscalStart, todayStart, todayStart.Add(24*time.Hour))
	var item AnnualLimitStats
	if err := row.Scan(
		&item.EmailDeliveredToday,
		&item.EmailFailedToday,
		&item.SmsDeliveredToday,
		&item.SmsFailedToday,
		&item.TotalEmailFiscalYearToYday,
		&item.TotalSmsFiscalYearToYday,
	); err != nil {
		if err == sql.ErrNoRows {
			return &AnnualLimitStats{}, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *Repository) FetchMonthlyUsageForService(ctx context.Context, serviceID uuid.UUID, fiscalYearStart int) ([]MonthlyUsageRow, error) {
	start := time.Date(fiscalYearStart, time.April, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(fiscalYearStart+1, time.April, 1, 0, 0, 0, 0, time.UTC)
	rows, err := r.readerDB.QueryContext(ctx, listMonthlyUsageForServiceQuery, serviceID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []MonthlyUsageRow
	for rows.Next() {
		var item MonthlyUsageRow
		if err := rows.Scan(&item.Month, &item.NotificationType, &item.NotificationStatus, &item.Count); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) AddUserToService(ctx context.Context, serviceID uuid.UUID, userID uuid.UUID, permissions []string, folderPermissions []uuid.UUID) error {
	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := txQueries.AddUserToService(ctx, AddUserToServiceParams{
		UserID:    uuid.NullUUID{UUID: userID, Valid: true},
		ServiceID: uuid.NullUUID{UUID: serviceID, Valid: true},
	}); err != nil {
		return err
	}

	userQueries := usersRepo.New(tx)
	now := time.Now().UTC().Truncate(time.Second)
	permissionItems, err := marshalPermissionItems(normalizeUserPermissions(permissions), now)
	if err != nil {
		return err
	}
	if err := userQueries.SetUserPermissions(ctx, usersRepo.SetUserPermissionsParams{
		ServiceID:       uuid.NullUUID{UUID: serviceID, Valid: true},
		UserID:          userID,
		PermissionItems: permissionItems,
	}); err != nil {
		return err
	}

	validFolderIDs, err := filterValidFolderPermissions(ctx, tx, serviceID, folderPermissions)
	if err != nil {
		return err
	}
	if len(validFolderIDs) > 0 {
		if err := userQueries.SetFolderPermissions(ctx, usersRepo.SetFolderPermissionsParams{
			UserID:    userID,
			ServiceID: serviceID,
			FolderIds: validFolderIDs,
		}); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *Repository) RemoveUserFromService(ctx context.Context, serviceID uuid.UUID, userID uuid.UUID) error {
	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	userQueries := usersRepo.New(tx)
	if err := userQueries.SetFolderPermissions(ctx, usersRepo.SetFolderPermissionsParams{
		UserID:    userID,
		ServiceID: serviceID,
		FolderIds: []uuid.UUID{},
	}); err != nil {
		return err
	}
	if err := userQueries.SetUserPermissions(ctx, usersRepo.SetUserPermissionsParams{
		ServiceID:       uuid.NullUUID{UUID: serviceID, Valid: true},
		UserID:          userID,
		PermissionItems: json.RawMessage("[]"),
	}); err != nil {
		return err
	}
	if _, err := txQueries.RemoveUserFromService(ctx, RemoveUserFromServiceParams{
		UserID:    uuid.NullUUID{UUID: userID, Valid: true},
		ServiceID: uuid.NullUUID{UUID: serviceID, Valid: true},
	}); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *Repository) RevokeAPIKey(ctx context.Context, serviceID uuid.UUID, keyID uuid.UUID) (*apiKeysRepo.ApiKey, error) {
	tx, err := beginExternalTx(ctx, r.writerDB)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	queries := apiKeysRepo.New(tx)
	current, err := queries.GetAPIKeyByID(ctx, keyID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if current.ServiceID != serviceID {
		return nil, nil
	}

	revoked, err := queries.RevokeAPIKey(ctx, keyID)
	if err != nil {
		return nil, err
	}
	if err := queries.InsertAPIKeyHistory(ctx, apiKeyHistoryParamsFromKey(revoked)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &revoked, nil
}

func (r *Repository) CreateService(ctx context.Context, service Service, userID *uuid.UUID, permissions []string) (*Service, error) {
	if userID == nil {
		return nil, fmt.Errorf("can't create a service without a user")
	}
	if strings.TrimSpace(r.platformFromNumber) == "" {
		return nil, fmt.Errorf("platform from number is required")
	}

	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	created := service
	now := time.Now().UTC().Truncate(time.Second)
	if created.ID == uuid.Nil {
		created.ID = uuid.New()
	}
	if created.CreatedAt.IsZero() {
		created.CreatedAt = now
	}
	created.CreatedByID = *userID
	if created.Version == 0 {
		created.Version = 1
	}

	stored, err := txQueries.CreateService(ctx, createParamsFromService(created))
	if err != nil {
		return nil, err
	}
	if err := txQueries.InsertServicesHistory(ctx, servicesHistoryFromService(stored)); err != nil {
		return nil, err
	}
	if len(permissions) == 0 {
		permissions = []string{"email", "sms", "international_sms"}
	}
	if err := txQueries.SetServicePermissions(ctx, SetServicePermissionsParams{ServiceID: stored.ID, Permissions: permissions}); err != nil {
		return nil, err
	}
	if _, err := insertServiceSmsSender(ctx, txQueries, stored.ID, r.platformFromNumber); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &stored, nil
}

func (r *Repository) GetSmsSenderByID(ctx context.Context, serviceID uuid.UUID, senderID uuid.UUID) (*ServiceSmsSender, error) {
	items, err := r.reader.GetSMSSenders(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == senderID {
			return &item, nil
		}
	}
	return nil, nil
}

func (r *Repository) GetSmsSendersByServiceID(ctx context.Context, serviceID uuid.UUID) ([]ServiceSmsSender, error) {
	return r.reader.GetSMSSenders(ctx, serviceID)
}

func (r *Repository) GetReplyTosByServiceID(ctx context.Context, serviceID uuid.UUID) ([]ServiceEmailReplyTo, error) {
	return r.reader.GetEmailReplyTo(ctx, serviceID)
}

func (r *Repository) GetReplyToByID(ctx context.Context, serviceID uuid.UUID, replyToID uuid.UUID) (*ServiceEmailReplyTo, error) {
	items, err := r.reader.GetEmailReplyTo(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == replyToID {
			return &item, nil
		}
	}
	return nil, nil
}

func (r *Repository) FetchServiceDataRetention(ctx context.Context, serviceID uuid.UUID) ([]ServiceDataRetention, error) {
	return r.reader.GetDataRetention(ctx, serviceID)
}

func (r *Repository) FetchServiceDataRetentionByID(ctx context.Context, serviceID, retentionID uuid.UUID) (*ServiceDataRetention, error) {
	row := r.readerDB.QueryRowContext(ctx, getDataRetentionByIDQuery, serviceID, retentionID)
	var item ServiceDataRetention
	if err := scanServiceDataRetention(row, &item); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *Repository) FetchDataRetentionByNotificationType(ctx context.Context, serviceID uuid.UUID, notificationType NotificationType) (*ServiceDataRetention, error) {
	row := r.readerDB.QueryRowContext(ctx, getDataRetentionByNotificationTypeQuery, serviceID, notificationType)
	var item ServiceDataRetention
	if err := scanServiceDataRetention(row, &item); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *Repository) InsertServiceDataRetention(ctx context.Context, retention ServiceDataRetention) (*ServiceDataRetention, error) {
	tx, err := beginExternalTx(ctx, r.writerDB)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if retention.ID == uuid.Nil {
		retention.ID = uuid.New()
	}
	if retention.CreatedAt.IsZero() {
		retention.CreatedAt = time.Now().UTC().Truncate(time.Second)
	}
	row := tx.QueryRowContext(ctx, createDataRetentionQuery, retention.ID, retention.ServiceID, retention.NotificationType, retention.DaysOfRetention, retention.CreatedAt, retention.UpdatedAt)
	var created ServiceDataRetention
	if err := scanServiceDataRetention(row, &created); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, serviceerrs.InvalidRequestError{Message: fmt.Sprintf("Service already has data retention for %s notification type", retention.NotificationType), StatusCode: http.StatusBadRequest}
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *Repository) UpdateServiceDataRetention(ctx context.Context, serviceID, retentionID uuid.UUID, daysOfRetention int32) (int64, error) {
	tx, err := beginExternalTx(ctx, r.writerDB)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, updateDataRetentionByIDQuery, daysOfRetention, serviceID, retentionID)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) FetchServiceSafelist(ctx context.Context, serviceID uuid.UUID) ([]ServiceSafelist, error) {
	return r.reader.GetSafelist(ctx, serviceID)
}

func (r *Repository) AddSafelistedContacts(ctx context.Context, serviceID uuid.UUID, emailAddresses, phoneNumbers []string) error {
	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	emailItems, err := marshalSafelistItems(emailAddresses)
	if err != nil {
		return err
	}
	phoneItems, err := marshalSafelistItems(phoneNumbers)
	if err != nil {
		return err
	}
	if err := txQueries.UpdateSafelist(ctx, UpdateSafelistParams{TargetServiceID: serviceID, EmailItems: emailItems, PhoneItems: phoneItems}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) RemoveServiceSafelist(ctx context.Context, serviceID uuid.UUID) error {
	tx, err := beginExternalTx(ctx, r.writerDB)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, deleteSafelistQuery, serviceID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) AddSmsSenderForService(ctx context.Context, sender ServiceSmsSender) (*ServiceSmsSender, error) {
	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	items, err := txQueries.GetSMSSenders(ctx, sender.ServiceID)
	if err != nil {
		return nil, err
	}
	if !sender.IsDefault && !hasDefaultSMSSender(items) {
		return nil, serviceerrs.InvalidRequestError{Message: errSMSDefaultRequired, StatusCode: http.StatusBadRequest}
	}
	if sender.IsDefault {
		if err := clearDefaultSMSSenders(ctx, txQueries, items, sender.ID); err != nil {
			return nil, err
		}
	}
	if sender.ID == uuid.Nil {
		sender.ID = uuid.New()
	}
	if sender.CreatedAt.IsZero() {
		sender.CreatedAt = time.Now().UTC().Truncate(time.Second)
	}
	created, err := txQueries.CreateSMSSender(ctx, CreateSMSSenderParams{
		ID:              sender.ID,
		SmsSender:       sender.SmsSender,
		ServiceID:       sender.ServiceID,
		IsDefault:       sender.IsDefault,
		InboundNumberID: sender.InboundNumberID,
		CreatedAt:       sender.CreatedAt,
		UpdatedAt:       sender.UpdatedAt,
		Archived:        false,
	})
	if err != nil {
		return nil, err
	}
	if created.InboundNumberID.Valid {
		inboundQueries := inboundRepo.New(tx)
		if _, err := inboundQueries.AddInboundNumber(ctx, inboundRepo.AddInboundNumberParams{ServiceID: uuid.NullUUID{UUID: sender.ServiceID, Valid: true}, ID: created.InboundNumberID.UUID}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *Repository) UpdateServiceSmsSender(ctx context.Context, sender ServiceSmsSender) (*ServiceSmsSender, error) {
	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	items, err := txQueries.GetSMSSenders(ctx, sender.ServiceID)
	if err != nil {
		return nil, err
	}
	current, ok := findSMSSender(items, sender.ID)
	if !ok {
		return nil, nil
	}
	if current.InboundNumberID.Valid && sender.SmsSender != current.SmsSender {
		return nil, serviceerrs.InvalidRequestError{Message: errInboundSenderImmutable, StatusCode: http.StatusBadRequest}
	}
	if !sender.IsDefault && current.IsDefault && isSoleDefaultSMSSender(items, current.ID) {
		return nil, serviceerrs.InvalidRequestError{Message: errSMSDefaultRequired, StatusCode: http.StatusBadRequest}
	}
	if sender.IsDefault {
		if err := clearDefaultSMSSenders(ctx, txQueries, items, sender.ID); err != nil {
			return nil, err
		}
	}
	updated, err := txQueries.UpdateSMSSender(ctx, UpdateSMSSenderParams{SmsSender: sender.SmsSender, IsDefault: sender.IsDefault, InboundNumberID: sender.InboundNumberID, Archived: sender.Archived, ID: sender.ID})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *Repository) UpdateSmsSenderWithInboundNumber(ctx context.Context, serviceID, senderID, inboundNumberID uuid.UUID) (*ServiceSmsSender, error) {
	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	items, err := txQueries.GetSMSSenders(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	current, ok := findSMSSender(items, senderID)
	if !ok {
		return nil, nil
	}
	updated, err := txQueries.UpdateSMSSender(ctx, UpdateSMSSenderParams{SmsSender: current.SmsSender, IsDefault: current.IsDefault, InboundNumberID: uuid.NullUUID{UUID: inboundNumberID, Valid: true}, Archived: current.Archived, ID: current.ID})
	if err != nil {
		return nil, err
	}
	inboundQueries := inboundRepo.New(tx)
	if _, err := inboundQueries.AddInboundNumber(ctx, inboundRepo.AddInboundNumberParams{ServiceID: uuid.NullUUID{UUID: serviceID, Valid: true}, ID: inboundNumberID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *Repository) ArchiveSmsSender(ctx context.Context, serviceID, senderID uuid.UUID) (*ServiceSmsSender, error) {
	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	items, err := txQueries.GetSMSSenders(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	current, ok := findSMSSender(items, senderID)
	if !ok {
		return nil, nil
	}
	if current.InboundNumberID.Valid {
		return nil, serviceerrs.InvalidRequestError{Message: errInboundSenderArchive, StatusCode: http.StatusBadRequest}
	}
	if current.IsDefault {
		return nil, serviceerrs.InvalidRequestError{Message: errDefaultSMSSenderArchive, StatusCode: http.StatusBadRequest}
	}
	updated, err := txQueries.UpdateSMSSender(ctx, UpdateSMSSenderParams{SmsSender: current.SmsSender, IsDefault: current.IsDefault, InboundNumberID: current.InboundNumberID, Archived: true, ID: current.ID})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *Repository) AddReplyToEmailAddress(ctx context.Context, replyTo ServiceEmailReplyTo) (*ServiceEmailReplyTo, error) {
	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	items, err := txQueries.GetEmailReplyTo(ctx, replyTo.ServiceID)
	if err != nil {
		return nil, err
	}
	if !replyTo.IsDefault && !hasDefaultReplyTo(items) {
		return nil, serviceerrs.InvalidRequestError{Message: errReplyToDefaultRequired, StatusCode: http.StatusBadRequest}
	}
	if replyTo.IsDefault {
		if err := clearDefaultReplyTos(ctx, txQueries, items, replyTo.ID); err != nil {
			return nil, err
		}
	}
	if replyTo.ID == uuid.Nil {
		replyTo.ID = uuid.New()
	}
	if replyTo.CreatedAt.IsZero() {
		replyTo.CreatedAt = time.Now().UTC().Truncate(time.Second)
	}
	created, err := txQueries.CreateEmailReplyTo(ctx, CreateEmailReplyToParams{ID: replyTo.ID, ServiceID: replyTo.ServiceID, EmailAddress: replyTo.EmailAddress, IsDefault: replyTo.IsDefault, CreatedAt: replyTo.CreatedAt, UpdatedAt: replyTo.UpdatedAt, Archived: false})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *Repository) UpdateReplyToEmailAddress(ctx context.Context, replyTo ServiceEmailReplyTo) (*ServiceEmailReplyTo, error) {
	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	items, err := txQueries.GetEmailReplyTo(ctx, replyTo.ServiceID)
	if err != nil {
		return nil, err
	}
	current, ok := findReplyTo(items, replyTo.ID)
	if !ok {
		return nil, nil
	}
	if !replyTo.IsDefault && current.IsDefault && isSoleDefaultReplyTo(items, current.ID) {
		return nil, serviceerrs.InvalidRequestError{Message: errReplyToDefaultRequired, StatusCode: http.StatusBadRequest}
	}
	if replyTo.IsDefault {
		if err := clearDefaultReplyTos(ctx, txQueries, items, replyTo.ID); err != nil {
			return nil, err
		}
	}
	updated, err := txQueries.UpdateEmailReplyTo(ctx, UpdateEmailReplyToParams{EmailAddress: replyTo.EmailAddress, IsDefault: replyTo.IsDefault, Archived: replyTo.Archived, ID: replyTo.ID})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *Repository) ArchiveReplyToEmailAddress(ctx context.Context, serviceID, replyToID uuid.UUID) (*ServiceEmailReplyTo, error) {
	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	items, err := txQueries.GetEmailReplyTo(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	current, ok := findReplyTo(items, replyToID)
	if !ok {
		return nil, nil
	}
	if current.IsDefault && len(items) > 1 {
		return nil, serviceerrs.InvalidRequestError{Message: errDefaultReplyToArchive, StatusCode: http.StatusBadRequest}
	}
	updated, err := txQueries.UpdateEmailReplyTo(ctx, UpdateEmailReplyToParams{EmailAddress: current.EmailAddress, IsDefault: false, Archived: true, ID: current.ID})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *Repository) GetLetterContactsByServiceID(ctx context.Context, serviceID uuid.UUID) ([]ServiceLetterContact, error) {
	rows, err := r.readerDB.QueryContext(ctx, listLetterContactsByServiceQuery, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ServiceLetterContact
	for rows.Next() {
		var item ServiceLetterContact
		if err := rows.Scan(&item.ID, &item.ServiceID, &item.ContactBlock, &item.IsDefault, &item.CreatedAt, &item.UpdatedAt, &item.Archived); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) GetLetterContactByID(ctx context.Context, serviceID, contactID uuid.UUID) (*ServiceLetterContact, error) {
	row := r.readerDB.QueryRowContext(ctx, getLetterContactByIDQuery, serviceID, contactID)
	var item ServiceLetterContact
	if err := row.Scan(&item.ID, &item.ServiceID, &item.ContactBlock, &item.IsDefault, &item.CreatedAt, &item.UpdatedAt, &item.Archived); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *Repository) AddLetterContactForService(ctx context.Context, contact ServiceLetterContact) (*ServiceLetterContact, error) {
	tx, err := beginExternalTx(ctx, r.writerDB)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	items, err := listLetterContacts(ctx, tx, contact.ServiceID)
	if err != nil {
		return nil, err
	}
	if contact.IsDefault {
		if err := clearDefaultLetterContacts(ctx, tx, items, contact.ID); err != nil {
			return nil, err
		}
	}
	if contact.ID == uuid.Nil {
		contact.ID = uuid.New()
	}
	if contact.CreatedAt.IsZero() {
		contact.CreatedAt = time.Now().UTC().Truncate(time.Second)
	}
	created, err := createLetterContact(ctx, tx, contact)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return created, nil
}

func (r *Repository) UpdateLetterContact(ctx context.Context, contact ServiceLetterContact) (*ServiceLetterContact, error) {
	tx, err := beginExternalTx(ctx, r.writerDB)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	items, err := listLetterContacts(ctx, tx, contact.ServiceID)
	if err != nil {
		return nil, err
	}
	current, ok := findLetterContact(items, contact.ID)
	if !ok {
		return nil, nil
	}
	if contact.IsDefault {
		if err := clearDefaultLetterContacts(ctx, tx, items, contact.ID); err != nil {
			return nil, err
		}
	}
	current.ContactBlock = contact.ContactBlock
	current.IsDefault = contact.IsDefault
	updated, err := updateLetterContact(ctx, tx, current)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *Repository) ArchiveLetterContact(ctx context.Context, serviceID, contactID uuid.UUID) (*ServiceLetterContact, error) {
	tx, err := beginExternalTx(ctx, r.writerDB)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	items, err := listLetterContacts(ctx, tx, serviceID)
	if err != nil {
		return nil, err
	}
	current, ok := findLetterContact(items, contactID)
	if !ok {
		return nil, nil
	}
	templateQueries := templatesRepo.New(tx)
	templates, err := templateQueries.GetTemplatesByServiceID(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	for _, tmpl := range templates {
		if tmpl.ServiceLetterContactID.Valid && tmpl.ServiceLetterContactID.UUID == contactID {
			if _, err := templateQueries.UpdateTemplate(ctx, templatesRepo.UpdateTemplateParams{
				Name:                   tmpl.Name,
				UpdatedAt:              sql.NullTime{Time: time.Now().UTC().Truncate(time.Second), Valid: true},
				Content:                tmpl.Content,
				Subject:                tmpl.Subject,
				Version:                tmpl.Version,
				Archived:               tmpl.Archived,
				ProcessType:            tmpl.ProcessType,
				ServiceLetterContactID: uuid.NullUUID{},
				Hidden:                 tmpl.Hidden,
				Postage:                tmpl.Postage,
				TemplateCategoryID:     tmpl.TemplateCategoryID,
				TextDirectionRtl:       tmpl.TextDirectionRtl,
				ID:                     tmpl.ID,
			}); err != nil {
				return nil, err
			}
		}
	}
	current.Archived = true
	updated, err := updateLetterContact(ctx, tx, current)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *Repository) SaveServiceCallbackApi(ctx context.Context, serviceID uuid.UUID, callbackType string, url string, bearerToken string, updatedByID uuid.UUID) (*ServiceCallbackApi, error) {
	signedToken, err := r.signBearerToken(bearerToken)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Truncate(time.Second)
	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	stored, err := txQueries.UpsertCallbackAPI(ctx, UpsertCallbackAPIParams{
		ID:           uuid.New(),
		ServiceID:    serviceID,
		Url:          url,
		BearerToken:  signedToken,
		CreatedAt:    now,
		UpdatedAt:    sql.NullTime{},
		UpdatedByID:  updatedByID,
		Version:      1,
		CallbackType: sql.NullString{String: callbackType, Valid: callbackType != ""},
		IsSuspended:  sql.NullBool{Bool: false, Valid: true},
		SuspendedAt:  sql.NullTime{},
	})
	if err != nil {
		return nil, err
	}
	if err := insertCallbackAPIHistory(ctx, tx, stored); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.unsignCallbackAPI(stored)
}

func (r *Repository) ResetServiceCallbackApi(ctx context.Context, serviceID, callbackID uuid.UUID, updatedByID uuid.UUID, url *string, bearerToken *string) (*ServiceCallbackApi, error) {
	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	current, err := r.getServiceCallbackApiWithQueries(ctx, txQueries, serviceID, callbackID)
	if err != nil || current == nil {
		return current, err
	}
	updated := *current
	if url != nil {
		updated.Url = *url
	}
	if bearerToken != nil {
		updated.BearerToken, err = r.signBearerToken(*bearerToken)
		if err != nil {
			return nil, err
		}
	} else {
		updated.BearerToken, err = r.signBearerToken(current.BearerToken)
		if err != nil {
			return nil, err
		}
	}
	updated.UpdatedByID = updatedByID
	updated.UpdatedAt = sql.NullTime{Time: time.Now().UTC().Truncate(time.Second), Valid: true}
	updated.Version++

	stored, err := txQueries.UpsertCallbackAPI(ctx, UpsertCallbackAPIParams{
		ID:           updated.ID,
		ServiceID:    updated.ServiceID,
		Url:          updated.Url,
		BearerToken:  updated.BearerToken,
		CreatedAt:    updated.CreatedAt,
		UpdatedAt:    updated.UpdatedAt,
		UpdatedByID:  updated.UpdatedByID,
		Version:      updated.Version,
		CallbackType: updated.CallbackType,
		IsSuspended:  updated.IsSuspended,
		SuspendedAt:  updated.SuspendedAt,
	})
	if err != nil {
		return nil, err
	}
	if err := insertCallbackAPIHistory(ctx, tx, stored); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.unsignCallbackAPI(stored)
}

func (r *Repository) GetCallbacksByServiceID(ctx context.Context, serviceID uuid.UUID) ([]ServiceCallbackApi, error) {
	items, err := r.reader.GetCallbackAPIs(ctx, GetCallbackAPIsParams{ServiceID: serviceID, CallbackType: sql.NullString{}})
	if err != nil {
		return nil, err
	}
	return r.unsignCallbackAPIs(items)
}

func (r *Repository) GetServiceCallbackApi(ctx context.Context, serviceID, callbackID uuid.UUID) (*ServiceCallbackApi, error) {
	return r.getServiceCallbackApiWithQueries(ctx, r.reader, serviceID, callbackID)
}

func (r *Repository) GetDeliveryStatusCallbackForService(ctx context.Context, serviceID uuid.UUID) (*ServiceCallbackApi, error) {
	return r.getCallbackByType(ctx, serviceID, callbackTypeDeliveryStatus)
}

func (r *Repository) GetComplaintCallbackForService(ctx context.Context, serviceID uuid.UUID) (*ServiceCallbackApi, error) {
	return r.getCallbackByType(ctx, serviceID, callbackTypeComplaint)
}

func (r *Repository) DeleteServiceCallbackApi(ctx context.Context, serviceID, callbackID uuid.UUID) (bool, error) {
	current, err := r.GetServiceCallbackApi(ctx, serviceID, callbackID)
	if err != nil {
		return false, err
	}
	if current == nil {
		return false, nil
	}
	rows, err := r.writer.DeleteCallbackAPI(ctx, callbackID)
	return rows > 0, err
}

func (r *Repository) ResignServiceCallbacks(ctx context.Context, resign bool, unsafe bool) (int, error) {
	tx, err := beginExternalTx(ctx, r.writerDB)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, listSignedCallbackAPIsForResignQuery)
	if err != nil {
		return 0, err
	}

	type callbackTokenUpdate struct {
		id          uuid.UUID
		bearerToken string
	}

	updates := make([]callbackTokenUpdate, 0)
	count := 0
	for rows.Next() {
		var item ServiceCallbackApi
		if err := scanServiceCallbackAPI(rows, &item); err != nil {
			_ = rows.Close()
			return count, err
		}

		plaintext, err := r.unsignBearerToken(item.BearerToken)
		if err != nil {
			if !unsafe {
				_ = rows.Close()
				return count, err
			}
			plaintext, err = unsignBearerTokenUnsafe(item.BearerToken)
			if err != nil {
				_ = rows.Close()
				return count, err
			}
		}

		resignedToken, err := r.signBearerToken(plaintext)
		if err != nil {
			_ = rows.Close()
			return count, err
		}

		count++
		if resign {
			updates = append(updates, callbackTokenUpdate{id: item.ID, bearerToken: resignedToken})
		}
	}
	if err := rows.Close(); err != nil {
		return count, err
	}
	if err := rows.Err(); err != nil {
		return count, err
	}

	if !resign {
		log.Printf("ResignServiceCallbacks dry-run: %d callback rows need re-signing", count)
		if err := tx.Rollback(); err != nil {
			return count, err
		}
		return count, nil
	}

	for _, update := range updates {
		if _, err := tx.ExecContext(ctx, updateCallbackAPIBearerTokenQuery, update.bearerToken, update.id); err != nil {
			return count, err
		}
	}

	if err := tx.Commit(); err != nil {
		return count, err
	}
	return count, nil
}

func (r *Repository) SuspendUnsuspendCallbackApi(ctx context.Context, serviceID uuid.UUID, updatedByID uuid.UUID, suspend bool) (*ServiceCallbackApi, error) {
	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	current, err := r.getCallbackByTypeWithQueries(ctx, txQueries, serviceID, callbackTypeDeliveryStatus)
	if err != nil || current == nil {
		return current, err
	}
	current.IsSuspended = sql.NullBool{Bool: suspend, Valid: true}
	current.SuspendedAt = sql.NullTime{Time: time.Now().UTC().Truncate(time.Second), Valid: true}
	current.UpdatedByID = updatedByID
	current.UpdatedAt = sql.NullTime{Time: time.Now().UTC().Truncate(time.Second), Valid: true}
	current.Version++
	current.BearerToken, err = r.signBearerToken(current.BearerToken)
	if err != nil {
		return nil, err
	}

	stored, err := txQueries.UpsertCallbackAPI(ctx, UpsertCallbackAPIParams{
		ID:           current.ID,
		ServiceID:    current.ServiceID,
		Url:          current.Url,
		BearerToken:  current.BearerToken,
		CreatedAt:    current.CreatedAt,
		UpdatedAt:    current.UpdatedAt,
		UpdatedByID:  current.UpdatedByID,
		Version:      current.Version,
		CallbackType: current.CallbackType,
		IsSuspended:  current.IsSuspended,
		SuspendedAt:  current.SuspendedAt,
	})
	if err != nil {
		return nil, err
	}
	if err := insertCallbackAPIHistory(ctx, tx, stored); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.unsignCallbackAPI(stored)
}

func (r *Repository) SaveServiceInboundApi(ctx context.Context, serviceID uuid.UUID, url string, bearerToken string, updatedByID uuid.UUID) (*ServiceInboundApi, error) {
	signedToken, err := r.signBearerToken(bearerToken)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Truncate(time.Second)
	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	stored, err := txQueries.UpsertInboundAPI(ctx, UpsertInboundAPIParams{ID: uuid.New(), ServiceID: serviceID, Url: url, BearerToken: signedToken, CreatedAt: now, UpdatedAt: sql.NullTime{}, UpdatedByID: updatedByID, Version: 1})
	if err != nil {
		return nil, err
	}
	if err := insertInboundAPIHistory(ctx, tx, stored); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.unsignInboundAPI(stored)
}

func (r *Repository) ResetServiceInboundApi(ctx context.Context, serviceID, inboundID uuid.UUID, updatedByID uuid.UUID, url *string, bearerToken *string) (*ServiceInboundApi, error) {
	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	current, err := r.getServiceInboundApiWithQueries(ctx, txQueries, serviceID, inboundID)
	if err != nil || current == nil {
		return current, err
	}
	updated := *current
	if url != nil {
		updated.Url = *url
	}
	if bearerToken != nil {
		updated.BearerToken, err = r.signBearerToken(*bearerToken)
		if err != nil {
			return nil, err
		}
	} else {
		updated.BearerToken, err = r.signBearerToken(current.BearerToken)
		if err != nil {
			return nil, err
		}
	}
	updated.UpdatedByID = updatedByID
	updated.UpdatedAt = sql.NullTime{Time: time.Now().UTC().Truncate(time.Second), Valid: true}
	updated.Version++

	stored, err := txQueries.UpsertInboundAPI(ctx, UpsertInboundAPIParams{ID: updated.ID, ServiceID: updated.ServiceID, Url: updated.Url, BearerToken: updated.BearerToken, CreatedAt: updated.CreatedAt, UpdatedAt: updated.UpdatedAt, UpdatedByID: updated.UpdatedByID, Version: updated.Version})
	if err != nil {
		return nil, err
	}
	if err := insertInboundAPIHistory(ctx, tx, stored); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.unsignInboundAPI(stored)
}

func (r *Repository) GetServiceInboundApi(ctx context.Context, serviceID, inboundID uuid.UUID) (*ServiceInboundApi, error) {
	return r.getServiceInboundApiWithQueries(ctx, r.reader, serviceID, inboundID)
}

func (r *Repository) GetServiceInboundApiForService(ctx context.Context, serviceID uuid.UUID) (*ServiceInboundApi, error) {
	item, err := r.reader.GetInboundAPI(ctx, serviceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return r.unsignInboundAPI(item)
}

func (r *Repository) DeleteServiceInboundApi(ctx context.Context, serviceID, inboundID uuid.UUID) (bool, error) {
	current, err := r.GetServiceInboundApi(ctx, serviceID, inboundID)
	if err != nil {
		return false, err
	}
	if current == nil {
		return false, nil
	}
	rows, err := r.writer.DeleteInboundAPI(ctx, inboundID)
	return rows > 0, err
}

func (r *Repository) UpdateService(ctx context.Context, service Service) (*Service, error) {
	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	updated := service
	updated.Version++
	updated.UpdatedAt = sql.NullTime{Time: time.Now().UTC().Truncate(time.Second), Valid: true}

	stored, err := txQueries.UpdateService(ctx, updateParamsFromService(updated))
	if err != nil {
		return nil, err
	}
	if err := txQueries.InsertServicesHistory(ctx, servicesHistoryFromService(stored)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &stored, nil
}

func (r *Repository) SuspendService(ctx context.Context, id uuid.UUID, userID *uuid.UUID) (*Service, error) {
	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	current, err := txQueries.GetServiceByID(ctx, GetServiceByIDParams{ID: id, OnlyActive: false})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	updated, err := suspendServiceNoTransaction(ctx, txQueries, current, userID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *Repository) ResumeService(ctx context.Context, id uuid.UUID) (*Service, error) {
	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	current, err := txQueries.GetServiceByID(ctx, GetServiceByIDParams{ID: id, OnlyActive: false})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	updated, err := resumeServiceNoTransaction(ctx, txQueries, current)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *Repository) ArchiveService(ctx context.Context, id uuid.UUID) (*Service, error) {
	tx, txQueries, err := r.beginWriteTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	current, err := txQueries.GetServiceByID(ctx, GetServiceByIDParams{ID: id, OnlyActive: false})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	updated, err := archiveServiceNoTransaction(ctx, tx, txQueries, current)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated, nil
}

func suspendServiceNoTransaction(ctx context.Context, queries *Queries, current Service, userID *uuid.UUID) (*Service, error) {
	if !current.Active {
		return &current, nil
	}
	updated := current
	updated.Active = false
	updated.Version++
	updated.UpdatedAt = sql.NullTime{Time: time.Now().UTC().Truncate(time.Second), Valid: true}
	updated.SuspendedAt = updated.UpdatedAt
	if userID != nil {
		updated.SuspendedByID = uuid.NullUUID{UUID: *userID, Valid: true}
	} else {
		updated.SuspendedByID = uuid.NullUUID{}
	}

	stored, err := queries.UpdateService(ctx, updateParamsFromService(updated))
	if err != nil {
		return nil, err
	}
	if err := queries.InsertServicesHistory(ctx, servicesHistoryFromService(stored)); err != nil {
		return nil, err
	}
	return &stored, nil
}

func resumeServiceNoTransaction(ctx context.Context, queries *Queries, current Service) (*Service, error) {
	if current.Active {
		return &current, nil
	}
	updated := current
	updated.Active = true
	updated.Version++
	updated.UpdatedAt = sql.NullTime{Time: time.Now().UTC().Truncate(time.Second), Valid: true}
	updated.SuspendedAt = sql.NullTime{}
	updated.SuspendedByID = uuid.NullUUID{}

	stored, err := queries.UpdateService(ctx, updateParamsFromService(updated))
	if err != nil {
		return nil, err
	}
	if err := queries.InsertServicesHistory(ctx, servicesHistoryFromService(stored)); err != nil {
		return nil, err
	}
	return &stored, nil
}

func archiveServiceNoTransaction(ctx context.Context, tx *sql.Tx, queries *Queries, current Service) (*Service, error) {
	timestamp := time.Now().UTC().Unix()
	updated := current
	updated.Active = false
	updated.Name = archivedValue(current.Name, timestamp)
	updated.EmailFrom = archivedValue(current.EmailFrom, timestamp)
	updated.Version++
	updated.UpdatedAt = sql.NullTime{Time: time.Now().UTC().Truncate(time.Second), Valid: true}

	stored, err := queries.UpdateService(ctx, updateParamsFromService(updated))
	if err != nil {
		return nil, err
	}
	if err := queries.InsertServicesHistory(ctx, servicesHistoryFromService(stored)); err != nil {
		return nil, err
	}

	apiKeyQueries := apiKeysRepo.New(tx)
	apiKeys, err := apiKeyQueries.GetAPIKeysByServiceID(ctx, stored.ID)
	if err != nil {
		return nil, err
	}
	for _, key := range apiKeys {
		revoked, err := apiKeyQueries.RevokeAPIKey(ctx, key.ID)
		if err != nil {
			return nil, err
		}
		if err := apiKeyQueries.InsertAPIKeyHistory(ctx, apiKeyHistoryParamsFromKey(revoked)); err != nil {
			return nil, err
		}
	}

	templateQueries := templatesRepo.New(tx)
	templates, err := templateQueries.GetTemplatesByServiceID(ctx, stored.ID)
	if err != nil {
		return nil, err
	}
	for _, tmpl := range templates {
		archived, err := templateQueries.ArchiveTemplate(ctx, tmpl.ID)
		if err != nil {
			return nil, err
		}
		if err := templateQueries.InsertTemplateHistory(ctx, templateHistoryParamsFromTemplate(archived)); err != nil {
			return nil, err
		}
	}

	return &stored, nil
}

func updateParamsFromService(service Service) UpdateServiceParams {
	return UpdateServiceParams{
		Name:                    service.Name,
		UpdatedAt:               service.UpdatedAt,
		Active:                  service.Active,
		MessageLimit:            service.MessageLimit,
		Restricted:              service.Restricted,
		EmailFrom:               service.EmailFrom,
		Version:                 service.Version,
		ResearchMode:            service.ResearchMode,
		OrganisationType:        service.OrganisationType,
		PrefixSms:               service.PrefixSms,
		Crown:                   service.Crown,
		RateLimit:               service.RateLimit,
		ContactLink:             service.ContactLink,
		ConsentToResearch:       service.ConsentToResearch,
		VolumeEmail:             service.VolumeEmail,
		VolumeLetter:            service.VolumeLetter,
		VolumeSms:               service.VolumeSms,
		CountAsLive:             service.CountAsLive,
		GoLiveAt:                service.GoLiveAt,
		GoLiveUserID:            service.GoLiveUserID,
		OrganisationID:          service.OrganisationID,
		SendingDomain:           service.SendingDomain,
		DefaultBrandingIsFrench: service.DefaultBrandingIsFrench,
		SmsDailyLimit:           service.SmsDailyLimit,
		OrganisationNotes:       service.OrganisationNotes,
		SensitiveService:        service.SensitiveService,
		EmailAnnualLimit:        service.EmailAnnualLimit,
		SmsAnnualLimit:          service.SmsAnnualLimit,
		SuspendedByID:           service.SuspendedByID,
		SuspendedAt:             service.SuspendedAt,
		ID:                      service.ID,
	}
}

func createParamsFromService(service Service) CreateServiceParams {
	return CreateServiceParams{
		ID:                      service.ID,
		Name:                    service.Name,
		CreatedAt:               service.CreatedAt,
		UpdatedAt:               service.UpdatedAt,
		Active:                  service.Active,
		MessageLimit:            service.MessageLimit,
		Restricted:              service.Restricted,
		EmailFrom:               service.EmailFrom,
		CreatedByID:             service.CreatedByID,
		Version:                 service.Version,
		ResearchMode:            service.ResearchMode,
		OrganisationType:        service.OrganisationType,
		PrefixSms:               service.PrefixSms,
		Crown:                   service.Crown,
		RateLimit:               service.RateLimit,
		ContactLink:             service.ContactLink,
		ConsentToResearch:       service.ConsentToResearch,
		VolumeEmail:             service.VolumeEmail,
		VolumeLetter:            service.VolumeLetter,
		VolumeSms:               service.VolumeSms,
		CountAsLive:             service.CountAsLive,
		GoLiveAt:                service.GoLiveAt,
		GoLiveUserID:            service.GoLiveUserID,
		OrganisationID:          service.OrganisationID,
		SendingDomain:           service.SendingDomain,
		DefaultBrandingIsFrench: service.DefaultBrandingIsFrench,
		SmsDailyLimit:           service.SmsDailyLimit,
		OrganisationNotes:       service.OrganisationNotes,
		SensitiveService:        service.SensitiveService,
		EmailAnnualLimit:        service.EmailAnnualLimit,
		SmsAnnualLimit:          service.SmsAnnualLimit,
		SuspendedByID:           service.SuspendedByID,
		SuspendedAt:             service.SuspendedAt,
	}
}

func servicesHistoryFromService(service Service) *ServicesHistory {
	return &ServicesHistory{
		ID:                      service.ID,
		Name:                    service.Name,
		CreatedAt:               service.CreatedAt,
		UpdatedAt:               service.UpdatedAt,
		Active:                  service.Active,
		MessageLimit:            service.MessageLimit,
		Restricted:              service.Restricted,
		EmailFrom:               service.EmailFrom,
		CreatedByID:             service.CreatedByID,
		Version:                 service.Version,
		ResearchMode:            service.ResearchMode,
		OrganisationType:        service.OrganisationType,
		PrefixSms:               sql.NullBool{Bool: service.PrefixSms, Valid: true},
		Crown:                   service.Crown,
		RateLimit:               service.RateLimit,
		ContactLink:             service.ContactLink,
		ConsentToResearch:       service.ConsentToResearch,
		VolumeEmail:             service.VolumeEmail,
		VolumeLetter:            service.VolumeLetter,
		VolumeSms:               service.VolumeSms,
		CountAsLive:             service.CountAsLive,
		GoLiveAt:                service.GoLiveAt,
		GoLiveUserID:            service.GoLiveUserID,
		OrganisationID:          service.OrganisationID,
		SendingDomain:           service.SendingDomain,
		DefaultBrandingIsFrench: service.DefaultBrandingIsFrench,
		SmsDailyLimit:           service.SmsDailyLimit,
		OrganisationNotes:       service.OrganisationNotes,
		SensitiveService:        service.SensitiveService,
		EmailAnnualLimit:        service.EmailAnnualLimit,
		SmsAnnualLimit:          service.SmsAnnualLimit,
		SuspendedByID:           service.SuspendedByID,
		SuspendedAt:             service.SuspendedAt,
	}
}

func apiKeyHistoryParamsFromKey(key apiKeysRepo.ApiKey) apiKeysRepo.InsertAPIKeyHistoryParams {
	return apiKeysRepo.InsertAPIKeyHistoryParams{
		ID:                 key.ID,
		Name:               key.Name,
		Secret:             key.Secret,
		ServiceID:          key.ServiceID,
		ExpiryDate:         key.ExpiryDate,
		CreatedAt:          key.CreatedAt,
		UpdatedAt:          key.UpdatedAt,
		CreatedByID:        key.CreatedByID,
		Version:            key.Version,
		KeyType:            key.KeyType,
		CompromisedKeyInfo: key.CompromisedKeyInfo,
		LastUsedTimestamp:  key.LastUsedTimestamp,
	}
}

func templateHistoryParamsFromTemplate(tmpl templatesRepo.Template) templatesRepo.InsertTemplateHistoryParams {
	return templatesRepo.InsertTemplateHistoryParams{
		ID:                     tmpl.ID,
		Name:                   tmpl.Name,
		TemplateType:           tmpl.TemplateType,
		CreatedAt:              tmpl.CreatedAt,
		UpdatedAt:              tmpl.UpdatedAt,
		Content:                tmpl.Content,
		ServiceID:              tmpl.ServiceID,
		Subject:                tmpl.Subject,
		CreatedByID:            tmpl.CreatedByID,
		Version:                tmpl.Version,
		Archived:               tmpl.Archived,
		ProcessType:            tmpl.ProcessType,
		ServiceLetterContactID: tmpl.ServiceLetterContactID,
		Hidden:                 tmpl.Hidden,
		Postage:                tmpl.Postage,
		TemplateCategoryID:     tmpl.TemplateCategoryID,
		TextDirectionRtl:       tmpl.TextDirectionRtl,
	}
}

func archivedValue(value string, timestamp int64) string {
	return fmt.Sprintf("_archived_%d_%s", timestamp, value)
}

func (r *Repository) currentAPIKeySecret() (string, error) {
	for _, secret := range r.apiKeySecrets {
		trimmed := strings.TrimSpace(secret)
		if trimmed != "" {
			return trimmed, nil
		}
	}
	return "", fmt.Errorf("at least one api key secret is required")
}

func (r *Repository) signBearerToken(token string) (string, error) {
	secret, err := r.currentAPIKeySecret()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(r.dangerousSalt) == "" {
		return "", fmt.Errorf("dangerous salt is required")
	}
	return signing.Sign(token, secret, r.dangerousSalt)
}

func (r *Repository) unsignBearerToken(token string) (string, error) {
	if strings.TrimSpace(token) == "" {
		return "", nil
	}
	if strings.TrimSpace(r.dangerousSalt) == "" {
		return "", fmt.Errorf("dangerous salt is required")
	}
	return signing.Unsign(token, r.apiKeySecrets, r.dangerousSalt)
}

func unsignBearerTokenUnsafe(token string) (string, error) {
	if strings.TrimSpace(token) == "" {
		return "", nil
	}
	lastSep := strings.LastIndex(token, ".")
	if lastSep == -1 {
		return "", fmt.Errorf("invalid signed token")
	}
	payloadWithTimestamp := token[:lastSep]
	timestampSep := strings.LastIndex(payloadWithTimestamp, ".")
	if timestampSep == -1 {
		return "", fmt.Errorf("timestamp missing")
	}
	if _, err := base64.RawURLEncoding.DecodeString(payloadWithTimestamp[timestampSep+1:]); err != nil {
		return "", fmt.Errorf("malformed timestamp: %w", err)
	}
	return payloadWithTimestamp[:timestampSep], nil
}

func scanServiceCallbackAPI(scanner interface{ Scan(dest ...any) error }, item *ServiceCallbackApi) error {
	return scanner.Scan(
		&item.ID,
		&item.ServiceID,
		&item.Url,
		&item.BearerToken,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.UpdatedByID,
		&item.Version,
		&item.CallbackType,
		&item.IsSuspended,
		&item.SuspendedAt,
	)
}

func scanServiceDataRetention(scanner interface{ Scan(dest ...any) error }, item *ServiceDataRetention) error {
	return scanner.Scan(
		&item.ID,
		&item.ServiceID,
		&item.NotificationType,
		&item.DaysOfRetention,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
}

func scanUser(scanner interface{ Scan(dest ...any) error }, item *usersRepo.User) error {
	return scanner.Scan(
		&item.ID,
		&item.Name,
		&item.EmailAddress,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.Password,
		&item.MobileNumber,
		&item.PasswordChangedAt,
		&item.LoggedInAt,
		&item.FailedLoginCount,
		&item.State,
		&item.PlatformAdmin,
		&item.CurrentSessionID,
		&item.AuthType,
		&item.Blocked,
		&item.AdditionalInformation,
		&item.PasswordExpired,
		&item.VerifiedPhonenumber,
		&item.DefaultEditorIsRte,
	)
}

func marshalSafelistItems(values []string) (json.RawMessage, error) {
	items := make([]map[string]any, 0, len(values))
	for _, value := range values {
		items = append(items, map[string]any{"id": uuid.New(), "recipient": value})
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func marshalPermissionItems(values []string, createdAt time.Time) (json.RawMessage, error) {
	items := make([]map[string]any, 0, len(values))
	for _, value := range values {
		items = append(items, map[string]any{"id": uuid.New(), "permission": value, "created_at": createdAt})
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func normalizeUserPermissions(permissions []string) []string {
	seen := make(map[string]struct{}, len(permissions))
	values := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		trimmed := strings.TrimSpace(permission)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		values = append(values, trimmed)
	}
	if len(values) == 0 {
		return append([]string(nil), defaultUserPermissions...)
	}
	return values
}

func filterValidFolderPermissions(ctx context.Context, db DBTX, serviceID uuid.UUID, folderPermissions []uuid.UUID) ([]uuid.UUID, error) {
	requested := uniqueUUIDs(folderPermissions)
	if len(requested) == 0 {
		return nil, nil
	}

	rows, err := db.QueryContext(ctx, listTemplateFoldersByIDsQuery, pq.Array(requested))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	serviceByFolder := make(map[uuid.UUID]uuid.UUID, len(requested))
	for rows.Next() {
		var folderID uuid.UUID
		var folderServiceID uuid.UUID
		if err := rows.Scan(&folderID, &folderServiceID); err != nil {
			return nil, err
		}
		serviceByFolder[folderID] = folderServiceID
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	valid := make([]uuid.UUID, 0, len(requested))
	for _, folderID := range requested {
		folderServiceID, ok := serviceByFolder[folderID]
		if !ok {
			continue
		}
		if folderServiceID != serviceID {
			return nil, fmt.Errorf("template folder %s does not belong to service %s", folderID, serviceID)
		}
		valid = append(valid, folderID)
	}
	return valid, nil
}

func uniqueUUIDs(values []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(values))
	unique := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		if value == uuid.Nil {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

func rowOrQuery(db DBTX, ctx context.Context, query string, args ...any) interface{ Scan(dest ...any) error } {
	return db.QueryRowContext(ctx, query, args...)
}

func scanInt64(scanner interface{ Scan(dest ...any) error }) (int64, error) {
	var value int64
	if err := scanner.Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return value, nil
}

func beginExternalTx(ctx context.Context, db DBTX) (*sql.Tx, error) {
	starter, ok := db.(txStarter)
	if !ok {
		return nil, fmt.Errorf("writer database does not support transactions")
	}
	return starter.BeginTx(ctx, nil)
}

func insertServiceSmsSender(ctx context.Context, queries *Queries, serviceID uuid.UUID, fromNumber string) (*ServiceSmsSender, error) {
	sender, err := queries.CreateSMSSender(ctx, CreateSMSSenderParams{
		ID:              uuid.New(),
		SmsSender:       fromNumber,
		ServiceID:       serviceID,
		IsDefault:       true,
		InboundNumberID: uuid.NullUUID{},
		CreatedAt:       time.Now().UTC().Truncate(time.Second),
		UpdatedAt:       sql.NullTime{},
		Archived:        false,
	})
	if err != nil {
		return nil, err
	}
	return &sender, nil
}

func (r *Repository) beginWriteTx(ctx context.Context) (*sql.Tx, *Queries, error) {
	starter, ok := r.writerDB.(txStarter)
	if !ok {
		return nil, nil, fmt.Errorf("writer database does not support transactions")
	}
	tx, err := starter.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	return tx, r.writer.WithTx(tx), nil
}

func hasDefaultSMSSender(items []ServiceSmsSender) bool {
	for _, item := range items {
		if !item.Archived && item.IsDefault {
			return true
		}
	}
	return false
}

func isSoleDefaultSMSSender(items []ServiceSmsSender, id uuid.UUID) bool {
	count := 0
	for _, item := range items {
		if !item.Archived && item.IsDefault {
			count++
			if item.ID != id {
				return false
			}
		}
	}
	return count == 1
}

func clearDefaultSMSSenders(ctx context.Context, queries *Queries, items []ServiceSmsSender, keepID uuid.UUID) error {
	for _, item := range items {
		if item.Archived || !item.IsDefault || item.ID == keepID {
			continue
		}
		if _, err := queries.UpdateSMSSender(ctx, UpdateSMSSenderParams{SmsSender: item.SmsSender, IsDefault: false, InboundNumberID: item.InboundNumberID, Archived: item.Archived, ID: item.ID}); err != nil {
			return err
		}
	}
	return nil
}

func findSMSSender(items []ServiceSmsSender, id uuid.UUID) (ServiceSmsSender, bool) {
	for _, item := range items {
		if item.ID == id && !item.Archived {
			return item, true
		}
	}
	return ServiceSmsSender{}, false
}

func hasDefaultReplyTo(items []ServiceEmailReplyTo) bool {
	for _, item := range items {
		if !item.Archived && item.IsDefault {
			return true
		}
	}
	return false
}

func isSoleDefaultReplyTo(items []ServiceEmailReplyTo, id uuid.UUID) bool {
	count := 0
	for _, item := range items {
		if !item.Archived && item.IsDefault {
			count++
			if item.ID != id {
				return false
			}
		}
	}
	return count == 1
}

func clearDefaultReplyTos(ctx context.Context, queries *Queries, items []ServiceEmailReplyTo, keepID uuid.UUID) error {
	for _, item := range items {
		if item.Archived || !item.IsDefault || item.ID == keepID {
			continue
		}
		if _, err := queries.UpdateEmailReplyTo(ctx, UpdateEmailReplyToParams{EmailAddress: item.EmailAddress, IsDefault: false, Archived: item.Archived, ID: item.ID}); err != nil {
			return err
		}
	}
	return nil
}

func findReplyTo(items []ServiceEmailReplyTo, id uuid.UUID) (ServiceEmailReplyTo, bool) {
	for _, item := range items {
		if item.ID == id && !item.Archived {
			return item, true
		}
	}
	return ServiceEmailReplyTo{}, false
}

func listLetterContacts(ctx context.Context, db DBTX, serviceID uuid.UUID) ([]ServiceLetterContact, error) {
	rows, err := db.QueryContext(ctx, listLetterContactsByServiceQuery, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ServiceLetterContact
	for rows.Next() {
		var item ServiceLetterContact
		if err := rows.Scan(&item.ID, &item.ServiceID, &item.ContactBlock, &item.IsDefault, &item.CreatedAt, &item.UpdatedAt, &item.Archived); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func createLetterContact(ctx context.Context, db DBTX, contact ServiceLetterContact) (*ServiceLetterContact, error) {
	row := db.QueryRowContext(ctx, createLetterContactQuery, contact.ID, contact.ServiceID, contact.ContactBlock, contact.IsDefault, contact.CreatedAt, contact.UpdatedAt, contact.Archived)
	var created ServiceLetterContact
	if err := row.Scan(&created.ID, &created.ServiceID, &created.ContactBlock, &created.IsDefault, &created.CreatedAt, &created.UpdatedAt, &created.Archived); err != nil {
		return nil, err
	}
	return &created, nil
}

func updateLetterContact(ctx context.Context, db DBTX, contact ServiceLetterContact) (*ServiceLetterContact, error) {
	row := db.QueryRowContext(ctx, updateLetterContactQuery, contact.ContactBlock, contact.IsDefault, contact.Archived, contact.ID, contact.ServiceID)
	var updated ServiceLetterContact
	if err := row.Scan(&updated.ID, &updated.ServiceID, &updated.ContactBlock, &updated.IsDefault, &updated.CreatedAt, &updated.UpdatedAt, &updated.Archived); err != nil {
		return nil, err
	}
	return &updated, nil
}

func clearDefaultLetterContacts(ctx context.Context, db DBTX, items []ServiceLetterContact, keepID uuid.UUID) error {
	for _, item := range items {
		if item.Archived || !item.IsDefault || item.ID == keepID {
			continue
		}
		item.IsDefault = false
		if _, err := updateLetterContact(ctx, db, item); err != nil {
			return err
		}
	}
	return nil
}

func findLetterContact(items []ServiceLetterContact, id uuid.UUID) (ServiceLetterContact, bool) {
	for _, item := range items {
		if item.ID == id && !item.Archived {
			return item, true
		}
	}
	return ServiceLetterContact{}, false
}

func insertCallbackAPIHistory(ctx context.Context, db DBTX, item ServiceCallbackApi) error {
	_, err := db.ExecContext(ctx, insertCallbackAPIHistoryQuery, item.ID, item.ServiceID, item.Url, item.BearerToken, item.CreatedAt, item.UpdatedAt, item.UpdatedByID, item.Version, item.CallbackType, item.IsSuspended, item.SuspendedAt)
	return err
}

func insertInboundAPIHistory(ctx context.Context, db DBTX, item ServiceInboundApi) error {
	_, err := db.ExecContext(ctx, insertInboundAPIHistoryQuery, item.ID, item.ServiceID, item.Url, item.BearerToken, item.CreatedAt, item.UpdatedAt, item.UpdatedByID, item.Version)
	return err
}

func (r *Repository) unsignCallbackAPIs(items []ServiceCallbackApi) ([]ServiceCallbackApi, error) {
	out := make([]ServiceCallbackApi, 0, len(items))
	for _, item := range items {
		decoded, err := r.unsignCallbackAPI(item)
		if err != nil {
			return nil, err
		}
		out = append(out, *decoded)
	}
	return out, nil
}

func (r *Repository) unsignCallbackAPI(item ServiceCallbackApi) (*ServiceCallbackApi, error) {
	decoded, err := r.unsignBearerToken(item.BearerToken)
	if err != nil {
		return nil, err
	}
	item.BearerToken = decoded
	return &item, nil
}

func (r *Repository) unsignInboundAPI(item ServiceInboundApi) (*ServiceInboundApi, error) {
	decoded, err := r.unsignBearerToken(item.BearerToken)
	if err != nil {
		return nil, err
	}
	item.BearerToken = decoded
	return &item, nil
}

func (r *Repository) getCallbackByType(ctx context.Context, serviceID uuid.UUID, callbackType string) (*ServiceCallbackApi, error) {
	return r.getCallbackByTypeWithQueries(ctx, r.reader, serviceID, callbackType)
}

func (r *Repository) getCallbackByTypeWithQueries(ctx context.Context, queries *Queries, serviceID uuid.UUID, callbackType string) (*ServiceCallbackApi, error) {
	items, err := queries.GetCallbackAPIs(ctx, GetCallbackAPIsParams{ServiceID: serviceID, CallbackType: sql.NullString{String: callbackType, Valid: callbackType != ""}})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return r.unsignCallbackAPI(items[0])
}

func (r *Repository) getServiceCallbackApiWithQueries(ctx context.Context, queries *Queries, serviceID, callbackID uuid.UUID) (*ServiceCallbackApi, error) {
	items, err := queries.GetCallbackAPIs(ctx, GetCallbackAPIsParams{ServiceID: serviceID, CallbackType: sql.NullString{}})
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == callbackID {
			return r.unsignCallbackAPI(item)
		}
	}
	return nil, nil
}

func (r *Repository) getServiceInboundApiWithQueries(ctx context.Context, queries *Queries, serviceID, inboundID uuid.UUID) (*ServiceInboundApi, error) {
	item, err := queries.GetInboundAPI(ctx, serviceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if item.ID != inboundID {
		return nil, nil
	}
	return r.unsignInboundAPI(item)
}

func sortServicesByCreatedAt(items []Service) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].Name < items[j].Name
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
}
