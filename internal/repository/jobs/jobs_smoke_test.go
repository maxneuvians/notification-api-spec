//go:build smoke

package jobs

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/repository/testutil"
)

func TestCreateJobRoundTripSmoke(t *testing.T) {
	db := testutil.OpenSmokeDB(t)
	queries := New(db)
	ctx := context.Background()
	userID := testutil.MustCreateUser(t, db)
	serviceID := testutil.MustCreateService(t, db, userID)
	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	created, err := queries.CreateJob(ctx, CreateJobParams{
		ID:                     id,
		OriginalFileName:       "batch.csv",
		ServiceID:              serviceID,
		TemplateID:             uuid.NullUUID{},
		CreatedAt:              now,
		UpdatedAt:              sql.NullTime{},
		NotificationCount:      1,
		NotificationsSent:      0,
		ProcessingStarted:      sql.NullTime{},
		ProcessingFinished:     sql.NullTime{},
		CreatedByID:            uuid.NullUUID{},
		TemplateVersion:        1,
		NotificationsDelivered: 0,
		NotificationsFailed:    0,
		JobStatus:              "pending",
		ScheduledFor:           sql.NullTime{},
		Archived:               false,
		ApiKeyID:               uuid.NullUUID{},
		SenderID:               uuid.NullUUID{},
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	if created.ID != id {
		t.Fatalf("CreateJob() id = %v, want %v", created.ID, id)
	}

	got, err := queries.GetJobByID(ctx, id)
	if err != nil {
		t.Fatalf("GetJobByID() error = %v", err)
	}
	if got.ID != id {
		t.Fatalf("GetJobByID() id = %v, want %v", got.ID, id)
	}
}
