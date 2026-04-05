//go:build smoke

package templates

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/repository/testutil"
)

func TestCreateTemplateRoundTripSmoke(t *testing.T) {
	db := testutil.OpenSmokeDB(t)
	queries := New(db)
	ctx := context.Background()
	userID := testutil.MustCreateUser(t, db)
	serviceID := testutil.MustCreateService(t, db, userID)
	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	created, err := queries.CreateTemplate(ctx, CreateTemplateParams{
		ID:                 id,
		Name:               "smoke-template-" + id.String()[:8],
		TemplateType:       TemplateTypeEmail,
		CreatedAt:          now,
		UpdatedAt:          sql.NullTime{},
		Content:            "hello from smoke",
		ServiceID:          serviceID,
		Subject:            sql.NullString{String: "subject", Valid: true},
		CreatedByID:        userID,
		Version:            1,
		Archived:           false,
		ProcessType:        sql.NullString{},
		Hidden:             false,
		Postage:            sql.NullString{},
		TemplateCategoryID: uuid.NullUUID{},
		TextDirectionRtl:   false,
	})
	if err != nil {
		t.Fatalf("CreateTemplate() error = %v", err)
	}
	if created.ID != id {
		t.Fatalf("CreateTemplate() id = %v, want %v", created.ID, id)
	}

	got, err := queries.GetTemplateByID(ctx, GetTemplateByIDParams{ID: id, ServiceID: serviceID})
	if err != nil {
		t.Fatalf("GetTemplateByID() error = %v", err)
	}
	if got.ID != id {
		t.Fatalf("GetTemplateByID() id = %v, want %v", got.ID, id)
	}
}
