//go:build smoke

package template_categories

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/repository/testutil"
)

func TestCreateTemplateCategoryRoundTripSmoke(t *testing.T) {
	db := testutil.OpenSmokeDB(t)
	queries := New(db)
	ctx := context.Background()
	userID := testutil.MustCreateUser(t, db)
	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	created, err := queries.CreateTemplateCategory(ctx, CreateTemplateCategoryParams{
		ID:                id,
		NameEn:            "Smoke Category",
		NameFr:            "Categorie Smoke",
		DescriptionEn:     sql.NullString{},
		DescriptionFr:     sql.NullString{},
		SmsProcessType:    "normal",
		EmailProcessType:  "normal",
		Hidden:            false,
		CreatedAt:         sql.NullTime{Time: now, Valid: true},
		UpdatedAt:         sql.NullTime{Time: now, Valid: true},
		SmsSendingVehicle: SmsSendingVehicleLongCode,
		CreatedByID:       userID,
		UpdatedByID:       uuid.NullUUID{},
	})
	if err != nil {
		t.Fatalf("CreateTemplateCategory() error = %v", err)
	}
	if created.ID != id {
		t.Fatalf("CreateTemplateCategory() id = %v, want %v", created.ID, id)
	}

	got, err := queries.GetTemplateCategoryByID(ctx, id)
	if err != nil {
		t.Fatalf("GetTemplateCategoryByID() error = %v", err)
	}
	if got.ID != id {
		t.Fatalf("GetTemplateCategoryByID() id = %v, want %v", got.ID, id)
	}
}
