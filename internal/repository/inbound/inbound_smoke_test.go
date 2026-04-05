//go:build smoke

package inbound

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/repository/testutil"
)

func TestCreateInboundSMSRoundTripSmoke(t *testing.T) {
	db := testutil.OpenSmokeDB(t)
	queries := New(db)
	ctx := context.Background()
	userID := testutil.MustCreateUser(t, db)
	serviceID := testutil.MustCreateService(t, db, userID)
	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	created, err := queries.CreateInboundSMS(ctx, CreateInboundSMSParams{
		ID:                id,
		ServiceID:         serviceID,
		Content:           "ciphertext",
		NotifyNumber:      "+12025550100",
		UserNumber:        "+12025550101",
		CreatedAt:         now,
		ProviderDate:      sql.NullTime{},
		ProviderReference: sql.NullString{String: "ref-" + id.String()[:8], Valid: true},
		Provider:          "sns",
	})
	if err != nil {
		t.Fatalf("CreateInboundSMS() error = %v", err)
	}
	if created.ID != id {
		t.Fatalf("CreateInboundSMS() id = %v, want %v", created.ID, id)
	}

	got, err := queries.GetInboundSMSByID(ctx, id)
	if err != nil {
		t.Fatalf("GetInboundSMSByID() error = %v", err)
	}
	if got.ID != id {
		t.Fatalf("GetInboundSMSByID() id = %v, want %v", got.ID, id)
	}
}
