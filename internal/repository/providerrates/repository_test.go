package providerrates

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"

	providersrepo "github.com/maxneuvians/notification-api-spec/internal/repository/providers"
)

type stubProviderLookup struct {
	provider providersrepo.ProviderDetail
	err      error
}

func (s *stubProviderLookup) GetProviderDetailsByIdentifier(_ context.Context, _ string) (providersrepo.ProviderDetail, error) {
	if s.err != nil {
		return providersrepo.ProviderDetail{}, s.err
	}
	return s.provider, nil
}

func TestCreateProviderRates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	providerID := uuid.New()
	repo := NewRepository(db, &stubProviderLookup{provider: providersrepo.ProviderDetail{ID: providerID}})
	validFrom := time.Now().UTC().Truncate(time.Second)

	mock.ExpectExec(regexp.QuoteMeta(`
		INSERT INTO provider_rates (id, valid_from, rate, provider_id)
		VALUES ($1, $2, $3, $4)
	`)).
		WithArgs(sqlmock.AnyArg(), validFrom, "0.045", providerID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	created, err := repo.CreateProviderRates(context.Background(), "sns", validFrom, "0.045")
	if err != nil {
		t.Fatalf("CreateProviderRates() error = %v", err)
	}
	if created.ProviderID != providerID {
		t.Fatalf("provider_id = %v, want %v", created.ProviderID, providerID)
	}
}

func TestGetRateForProvider(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	providerID := uuid.New()
	rateID := uuid.New()
	repo := NewRepository(db, &stubProviderLookup{})
	now := time.Now().UTC().Truncate(time.Second)

	mock.ExpectQuery("SELECT id, valid_from, rate, provider_id\\s+FROM provider_rates").
		WithArgs(providerID, now).
		WillReturnRows(sqlmock.NewRows([]string{"id", "valid_from", "rate", "provider_id"}).
			AddRow(rateID, now.Add(-time.Hour), "0.050", providerID))

	rate, err := repo.GetRateForProvider(context.Background(), providerID, now)
	if err != nil {
		t.Fatalf("GetRateForProvider() error = %v", err)
	}
	if rate.Rate != "0.050" {
		t.Fatalf("rate = %q, want 0.050", rate.Rate)
	}
}

func TestGetRateForProviderMissingRate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	providerID := uuid.New()
	repo := NewRepository(db, &stubProviderLookup{})
	now := time.Now().UTC().Truncate(time.Second)

	mock.ExpectQuery("SELECT id, valid_from, rate, provider_id\\s+FROM provider_rates").
		WithArgs(providerID, now).
		WillReturnError(sql.ErrNoRows)

	_, err = repo.GetRateForProvider(context.Background(), providerID, now)
	if err == nil {
		t.Fatal("GetRateForProvider() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "missing provider rate") {
		t.Fatalf("error = %q, want missing provider rate", err.Error())
	}
	if errors.Is(err, sql.ErrNoRows) {
		t.Fatal("expected wrapped error, got sql.ErrNoRows")
	}
}
