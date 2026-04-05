package providers

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/config"
)

type ProviderDetailStat struct {
	ProviderDetail
	CreatedByName           sql.NullString `json:"created_by_name"`
	CurrentMonthBillableSMS int64          `json:"current_month_billable_sms"`
}

type UpdateProviderDetailsParams struct {
	ID          uuid.UUID
	Priority    *int32
	Active      *bool
	CreatedByID *uuid.UUID
}

func (q *Queries) GetProviderDetailsByID(ctx context.Context, id uuid.UUID) (ProviderDetail, error) {
	return q.GetProviderByID(ctx, id)
}

func (q *Queries) GetProviderDetailsByIdentifier(ctx context.Context, identifier string) (ProviderDetail, error) {
	const query = `
		SELECT id, display_name, identifier, priority, notification_type, active, updated_at, version, created_by_id, supports_international
		FROM provider_details
		WHERE identifier = $1
	`

	row := q.db.QueryRowContext(ctx, query, identifier)
	return scanProviderDetail(row)
}

func (q *Queries) GetProviderDetailsByNotificationType(ctx context.Context, notificationType NotificationType, international bool) ([]ProviderDetail, error) {
	const query = `
		SELECT id, display_name, identifier, priority, notification_type, active, updated_at, version, created_by_id, supports_international
		FROM provider_details
		WHERE notification_type = $1
			AND ($2::boolean = false OR supports_international = true)
		ORDER BY priority ASC, display_name ASC
	`

	rows, err := q.db.QueryContext(ctx, query, notificationType, international)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ProviderDetail
	for rows.Next() {
		item, err := scanProviderDetail(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (q *Queries) GetCurrentProvider(ctx context.Context, notificationType NotificationType) (ProviderDetail, error) {
	const query = `
		SELECT id, display_name, identifier, priority, notification_type, active, updated_at, version, created_by_id, supports_international
		FROM provider_details
		WHERE notification_type = $1
			AND active = true
		ORDER BY priority ASC, display_name ASC
		LIMIT 1
	`

	row := q.db.QueryRowContext(ctx, query, notificationType)
	return scanProviderDetail(row)
}

func (q *Queries) UpdateProviderDetails(ctx context.Context, arg UpdateProviderDetailsParams) (ProviderDetail, error) {
	current, err := q.GetProviderDetailsByID(ctx, arg.ID)
	if err != nil {
		return ProviderDetail{}, err
	}

	updated := current
	if arg.Priority != nil {
		updated.Priority = *arg.Priority
	}
	if arg.Active != nil {
		updated.Active = *arg.Active
	}
	if arg.CreatedByID != nil {
		updated.CreatedByID = uuid.NullUUID{UUID: *arg.CreatedByID, Valid: true}
	}

	updated.Version = current.Version + 1
	updated.UpdatedAt = sql.NullTime{Time: time.Now().UTC().Truncate(time.Second), Valid: true}

	result, err := q.UpdateProvider(ctx, UpdateProviderParams{
		DisplayName:           updated.DisplayName,
		Identifier:            updated.Identifier,
		Priority:              updated.Priority,
		NotificationType:      updated.NotificationType,
		Active:                updated.Active,
		UpdatedAt:             updated.UpdatedAt,
		Version:               updated.Version,
		CreatedByID:           updated.CreatedByID,
		SupportsInternational: updated.SupportsInternational,
		ID:                    updated.ID,
	})
	if err != nil {
		return ProviderDetail{}, err
	}

	if err := q.insertProviderHistoryRow(ctx, result); err != nil {
		return ProviderDetail{}, err
	}

	return result, nil
}

func (q *Queries) GetAlternativeSMSProvider(ctx context.Context, identifier string) (ProviderDetail, error) {
	return q.GetProviderDetailsByIdentifier(ctx, identifier)
}

func (q *Queries) SwitchSMSProviderToIdentifier(ctx context.Context, identifier string) error {
	target, err := q.GetProviderDetailsByIdentifier(ctx, identifier)
	if err != nil {
		return err
	}
	if !target.Active {
		return nil
	}

	current, err := q.GetCurrentProvider(ctx, NotificationTypeSms)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	if current.ID == target.ID {
		return nil
	}

	notifyUserID, err := uuid.Parse(config.NotifyUserID)
	if err != nil {
		return fmt.Errorf("parse notify user id: %w", err)
	}

	targetPriority := target.Priority
	currentPriority := current.Priority
	if target.Priority > current.Priority {
		targetPriority = current.Priority
		currentPriority = target.Priority
	} else if target.Priority == current.Priority {
		currentPriority = current.Priority + 10
	}

	if _, err := q.UpdateProviderDetails(ctx, UpdateProviderDetailsParams{
		ID:          target.ID,
		Priority:    &targetPriority,
		CreatedByID: &notifyUserID,
	}); err != nil {
		return err
	}

	if _, err := q.UpdateProviderDetails(ctx, UpdateProviderDetailsParams{
		ID:          current.ID,
		Priority:    &currentPriority,
		CreatedByID: &notifyUserID,
	}); err != nil {
		return err
	}

	return nil
}

func (q *Queries) ToggleSMSProviderByIdentifier(ctx context.Context, identifier string) error {
	alternative, err := q.GetAlternativeSMSProvider(ctx, identifier)
	if err != nil {
		return err
	}

	return q.SwitchSMSProviderToIdentifier(ctx, alternative.Identifier)
}

func (q *Queries) GetDaoProviderStats(ctx context.Context) ([]ProviderDetailStat, error) {
	const query = `
		SELECT
			p.id,
			p.display_name,
			p.identifier,
			p.priority,
			p.notification_type,
			p.active,
			p.updated_at,
			p.version,
			p.created_by_id,
			p.supports_international,
			COALESCE(u.name, '') AS created_by_name,
			COALESCE(b.current_month_billable_sms, 0)::bigint AS current_month_billable_sms
		FROM provider_details p
		LEFT JOIN users u ON u.id = p.created_by_id
		LEFT JOIN (
			SELECT provider, SUM(COALESCE(billable_units, 0) * rate_multiplier) AS current_month_billable_sms
			FROM ft_billing
			WHERE notification_type = 'sms'
				AND bst_date >= date_trunc('month', CURRENT_DATE)::date
			GROUP BY provider
		) b ON b.provider = p.identifier
		ORDER BY p.notification_type ASC, p.priority ASC, p.display_name ASC
	`

	rows, err := q.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ProviderDetailStat
	for rows.Next() {
		var item ProviderDetailStat
		if err := rows.Scan(
			&item.ID,
			&item.DisplayName,
			&item.Identifier,
			&item.Priority,
			&item.NotificationType,
			&item.Active,
			&item.UpdatedAt,
			&item.Version,
			&item.CreatedByID,
			&item.SupportsInternational,
			&item.CreatedByName,
			&item.CurrentMonthBillableSMS,
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

func (q *Queries) GetProviderStatByID(ctx context.Context, id uuid.UUID) (ProviderDetailStat, error) {
	stats, err := q.GetDaoProviderStats(ctx)
	if err != nil {
		return ProviderDetailStat{}, err
	}

	for _, item := range stats {
		if item.ID == id {
			return item, nil
		}
	}

	return ProviderDetailStat{}, sql.ErrNoRows
}

func (q *Queries) insertProviderHistoryRow(ctx context.Context, provider ProviderDetail) error {
	return q.InsertProviderHistory(ctx, InsertProviderHistoryParams{
		ID:                    provider.ID,
		DisplayName:           provider.DisplayName,
		Identifier:            provider.Identifier,
		Priority:              provider.Priority,
		NotificationType:      provider.NotificationType,
		Active:                provider.Active,
		Version:               provider.Version,
		UpdatedAt:             provider.UpdatedAt,
		CreatedByID:           provider.CreatedByID,
		SupportsInternational: provider.SupportsInternational,
	})
}

type providerDetailScanner interface {
	Scan(dest ...any) error
}

func scanProviderDetail(scanner providerDetailScanner) (ProviderDetail, error) {
	var item ProviderDetail
	err := scanner.Scan(
		&item.ID,
		&item.DisplayName,
		&item.Identifier,
		&item.Priority,
		&item.NotificationType,
		&item.Active,
		&item.UpdatedAt,
		&item.Version,
		&item.CreatedByID,
		&item.SupportsInternational,
	)
	return item, err
}
