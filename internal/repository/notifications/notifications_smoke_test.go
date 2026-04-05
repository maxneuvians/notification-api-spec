//go:build smoke

package notifications

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/repository/testutil"
)

func TestCreateNotificationRoundTripSmoke(t *testing.T) {
	db := testutil.OpenSmokeDB(t)
	queries := New(db)
	ctx := context.Background()
	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	created, err := queries.CreateNotification(ctx, CreateNotificationParams{
		ID:                 id,
		Recipient:          "smoke@example.com",
		CreatedAt:          now,
		TemplateVersion:    1,
		KeyType:            "normal",
		NotificationType:   NotificationTypeEmail,
		BillableUnits:      1,
		NotificationStatus: sql.NullString{String: "created", Valid: true},
		NormalisedTo:       sql.NullString{String: "smoke@example.com", Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateNotification() error = %v", err)
	}
	if created.ID != id {
		t.Fatalf("CreateNotification() id = %v, want %v", created.ID, id)
	}

	got, err := queries.GetNotificationByID(ctx, id)
	if err != nil {
		t.Fatalf("GetNotificationByID() error = %v", err)
	}
	if got.ID != id {
		t.Fatalf("GetNotificationByID() id = %v, want %v", got.ID, id)
	}
}
