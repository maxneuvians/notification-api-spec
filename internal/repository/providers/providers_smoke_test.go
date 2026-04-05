//go:build smoke

package providers

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/repository/testutil"
)

func TestUpdateProviderRoundTripSmoke(t *testing.T) {
	db := testutil.OpenSmokeDB(t)
	queries := New(db)
	ctx := context.Background()
	id := testutil.MustCreateProvider(t, db)
	now := time.Now().UTC().Truncate(time.Second)

	updated, err := queries.UpdateProvider(ctx, UpdateProviderParams{
		DisplayName:           "Updated Provider",
		Identifier:            "updated-provider-" + id.String()[:8],
		Priority:              20,
		NotificationType:      NotificationTypeSms,
		Active:                true,
		UpdatedAt:             sql.NullTime{Time: now, Valid: true},
		Version:               2,
		CreatedByID:           uuid.NullUUID{},
		SupportsInternational: true,
		ID:                    id,
	})
	if err != nil {
		t.Fatalf("UpdateProvider() error = %v", err)
	}
	if updated.ID != id {
		t.Fatalf("UpdateProvider() id = %v, want %v", updated.ID, id)
	}

	got, err := queries.GetProviderByID(ctx, id)
	if err != nil {
		t.Fatalf("GetProviderByID() error = %v", err)
	}
	if got.ID != id {
		t.Fatalf("GetProviderByID() id = %v, want %v", got.ID, id)
	}
}
