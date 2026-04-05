//go:build smoke

package billing

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/repository/testutil"
)

func TestUpsertFreeSMSFragmentLimitRoundTripSmoke(t *testing.T) {
	db := testutil.OpenSmokeDB(t)
	queries := New(db)
	ctx := context.Background()
	userID := testutil.MustCreateUser(t, db)
	serviceID := testutil.MustCreateService(t, db, userID)
	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	created, err := queries.UpsertFreeSMSFragmentLimit(ctx, UpsertFreeSMSFragmentLimitParams{
		ID:                   id,
		ServiceID:            serviceID,
		FinancialYearStart:   2026,
		FreeSmsFragmentLimit: 240,
		UpdatedAt:            sql.NullTime{Time: now, Valid: true},
		CreatedAt:            now,
	})
	if err != nil {
		t.Fatalf("UpsertFreeSMSFragmentLimit() error = %v", err)
	}
	if created.ID != id {
		t.Fatalf("UpsertFreeSMSFragmentLimit() id = %v, want %v", created.ID, id)
	}

	got, err := queries.GetFreeSMSFragmentLimit(ctx, GetFreeSMSFragmentLimitParams{ServiceID: serviceID, FinancialYearStart: 2026})
	if err != nil {
		t.Fatalf("GetFreeSMSFragmentLimit() error = %v", err)
	}
	if got.ID != id {
		t.Fatalf("GetFreeSMSFragmentLimit() id = %v, want %v", got.ID, id)
	}
}
