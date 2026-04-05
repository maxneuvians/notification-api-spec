//go:build smoke

package users

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/repository/testutil"
)

func TestCreateUserRoundTripSmoke(t *testing.T) {
	db := testutil.OpenSmokeDB(t)
	queries := New(db)
	ctx := context.Background()
	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	created, err := queries.CreateUser(ctx, CreateUserParams{
		ID:                    id,
		Name:                  "Smoke User",
		EmailAddress:          "smoke-user-" + id.String()[:8] + "@example.com",
		CreatedAt:             now,
		UpdatedAt:             sql.NullTime{},
		Password:              "ciphertext",
		MobileNumber:          sql.NullString{},
		PasswordChangedAt:     now,
		LoggedInAt:            sql.NullTime{},
		FailedLoginCount:      0,
		State:                 "active",
		PlatformAdmin:         false,
		CurrentSessionID:      uuid.NullUUID{},
		AuthType:              "email_auth",
		Blocked:               false,
		AdditionalInformation: json.RawMessage(`{"source":"smoke"}`),
		PasswordExpired:       false,
		VerifiedPhonenumber:   sql.NullBool{},
		DefaultEditorIsRte:    false,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if created.ID != id {
		t.Fatalf("CreateUser() id = %v, want %v", created.ID, id)
	}

	got, err := queries.GetUserByID(ctx, id)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if got.ID != id {
		t.Fatalf("GetUserByID() id = %v, want %v", got.ID, id)
	}
}
