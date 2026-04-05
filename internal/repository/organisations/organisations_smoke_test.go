//go:build smoke

package organisations

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/repository/testutil"
)

func TestCreateOrganisationRoundTripSmoke(t *testing.T) {
	db := testutil.OpenSmokeDB(t)
	queries := New(db)
	ctx := context.Background()
	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	created, err := queries.CreateOrganisation(ctx, CreateOrganisationParams{
		ID:                                    id,
		Name:                                  "Smoke Org " + id.String()[:8],
		Active:                                true,
		CreatedAt:                             now,
		UpdatedAt:                             sql.NullTime{},
		EmailBrandingID:                       uuid.NullUUID{},
		LetterBrandingID:                      uuid.NullUUID{},
		AgreementSigned:                       sql.NullBool{},
		AgreementSignedAt:                     sql.NullTime{},
		AgreementSignedByID:                   uuid.NullUUID{},
		AgreementSignedVersion:                sql.NullFloat64{},
		Crown:                                 sql.NullBool{},
		OrganisationType:                      sql.NullString{},
		RequestToGoLiveNotes:                  sql.NullString{},
		AgreementSignedOnBehalfOfEmailAddress: sql.NullString{},
		AgreementSignedOnBehalfOfName:         sql.NullString{},
		DefaultBrandingIsFrench:               sql.NullBool{},
	})
	if err != nil {
		t.Fatalf("CreateOrganisation() error = %v", err)
	}
	if created.ID != id {
		t.Fatalf("CreateOrganisation() id = %v, want %v", created.ID, id)
	}

	got, err := queries.GetOrganisationByID(ctx, id)
	if err != nil {
		t.Fatalf("GetOrganisationByID() error = %v", err)
	}
	if got.ID != id {
		t.Fatalf("GetOrganisationByID() id = %v, want %v", got.ID, id)
	}
}
