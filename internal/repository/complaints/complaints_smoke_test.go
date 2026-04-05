//go:build smoke

package complaints

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/repository/testutil"
)

func TestCreateOrUpdateComplaintRoundTripSmoke(t *testing.T) {
	db := testutil.OpenSmokeDB(t)
	queries := New(db)
	ctx := context.Background()
	userID := testutil.MustCreateUser(t, db)
	serviceID := testutil.MustCreateService(t, db, userID)
	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	created, err := queries.CreateOrUpdateComplaint(ctx, CreateOrUpdateComplaintParams{
		ID:             id,
		NotificationID: uuid.New(),
		ServiceID:      serviceID,
		SesFeedbackID:  sql.NullString{String: "ses-" + id.String()[:8], Valid: true},
		ComplaintType:  sql.NullString{String: "abuse", Valid: true},
		ComplaintDate:  sql.NullTime{Time: now, Valid: true},
		CreatedAt:      now,
	})
	if err != nil {
		t.Fatalf("CreateOrUpdateComplaint() error = %v", err)
	}
	if created.ID != id {
		t.Fatalf("CreateOrUpdateComplaint() id = %v, want %v", created.ID, id)
	}

	got, err := queries.GetComplaintsPage(ctx, GetComplaintsPageParams{
		StartAt:     sql.NullTime{Time: now.Add(-time.Hour), Valid: true},
		EndAt:       sql.NullTime{Time: now.Add(time.Hour), Valid: true},
		OffsetCount: 0,
		LimitCount:  10,
	})
	if err != nil {
		t.Fatalf("GetComplaintsPage() error = %v", err)
	}
	if len(got) == 0 {
		t.Fatal("GetComplaintsPage() returned no rows")
	}
}
