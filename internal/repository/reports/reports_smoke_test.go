//go:build smoke

package reports

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/repository/testutil"
)

func TestCreateReportRoundTripSmoke(t *testing.T) {
	db := testutil.OpenSmokeDB(t)
	queries := New(db)
	ctx := context.Background()
	userID := testutil.MustCreateUser(t, db)
	serviceID := testutil.MustCreateService(t, db, userID)
	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	created, err := queries.CreateReport(ctx, CreateReportParams{
		ID:               id,
		ReportType:       "notifications",
		RequestedAt:      now,
		CompletedAt:      sql.NullTime{},
		ExpiresAt:        sql.NullTime{},
		RequestingUserID: uuid.NullUUID{UUID: userID, Valid: true},
		ServiceID:        serviceID,
		JobID:            uuid.NullUUID{},
		Url:              sql.NullString{},
		Status:           "created",
		Language:         sql.NullString{String: "en", Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateReport() error = %v", err)
	}
	if created.ID != id {
		t.Fatalf("CreateReport() id = %v, want %v", created.ID, id)
	}

	got, err := queries.GetReportByID(ctx, id)
	if err != nil {
		t.Fatalf("GetReportByID() error = %v", err)
	}
	if got.ID != id {
		t.Fatalf("GetReportByID() id = %v, want %v", got.ID, id)
	}
}
