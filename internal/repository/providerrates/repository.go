package providerrates

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	providersrepo "github.com/maxneuvians/notification-api-spec/internal/repository/providers"
)

type ProviderRate struct {
	ID         uuid.UUID `json:"id"`
	ValidFrom  time.Time `json:"valid_from"`
	Rate       string    `json:"rate"`
	ProviderID uuid.UUID `json:"provider_id"`
}

type providerLookup interface {
	GetProviderDetailsByIdentifier(ctx context.Context, identifier string) (providersrepo.ProviderDetail, error)
}

type Repository struct {
	queries         *Queries
	providerQueries providerLookup
}

func NewRepository(db DBTX, providerQueries providerLookup) *Repository {
	return &Repository{queries: New(db), providerQueries: providerQueries}
}

func (r *Repository) CreateProviderRates(ctx context.Context, identifier string, validFrom time.Time, rate string) (ProviderRate, error) {
	provider, err := r.providerQueries.GetProviderDetailsByIdentifier(ctx, identifier)
	if err != nil {
		return ProviderRate{}, err
	}

	record := ProviderRate{
		ID:         uuid.New(),
		ValidFrom:  validFrom.UTC().Truncate(time.Second),
		Rate:       rate,
		ProviderID: provider.ID,
	}

	const query = `
		INSERT INTO provider_rates (id, valid_from, rate, provider_id)
		VALUES ($1, $2, $3, $4)
	`

	if _, err := r.queries.db.ExecContext(ctx, query, record.ID, record.ValidFrom, record.Rate, record.ProviderID); err != nil {
		return ProviderRate{}, err
	}

	return record, nil
}

func (r *Repository) GetRateForProvider(ctx context.Context, providerID uuid.UUID, notificationCreatedAt time.Time) (ProviderRate, error) {
	const query = `
		SELECT id, valid_from, rate, provider_id
		FROM provider_rates
		WHERE provider_id = $1
			AND valid_from <= $2
		ORDER BY valid_from DESC
		LIMIT 1
	`

	row := r.queries.db.QueryRowContext(ctx, query, providerID, notificationCreatedAt.UTC())
	var record ProviderRate
	if err := row.Scan(&record.ID, &record.ValidFrom, &record.Rate, &record.ProviderID); err != nil {
		log.Printf("[error-sms-rates] missing rate for provider %s at %s", providerID, notificationCreatedAt.UTC().Format(time.RFC3339))
		return ProviderRate{}, fmt.Errorf("missing provider rate for provider %s", providerID)
	}

	return record, nil
}
