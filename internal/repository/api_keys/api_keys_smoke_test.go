//go:build smoke

package api_keys

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/repository/testutil"
)

func TestCreateAPIKeyRoundTripSmoke(t *testing.T) {
	db := testutil.OpenSmokeDB(t)
	queries := New(db)
	ctx := context.Background()
	userID := testutil.MustCreateUser(t, db)
	serviceID := testutil.MustCreateService(t, db, userID)
	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)
	payload := json.RawMessage(`{"source":"smoke"}`)

	created, err := queries.CreateAPIKey(ctx, CreateAPIKeyParams{
		ID:                 id,
		Name:               "smoke-key-" + id.String()[:8],
		Secret:             "secret-" + id.String(),
		ServiceID:          serviceID,
		ExpiryDate:         sql.NullTime{},
		CreatedAt:          now,
		CreatedByID:        userID,
		UpdatedAt:          sql.NullTime{},
		Version:            1,
		KeyType:            "normal",
		CompromisedKeyInfo: payload,
		LastUsedTimestamp:  sql.NullTime{},
	})
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if created.ID != id {
		t.Fatalf("CreateAPIKey() id = %v, want %v", created.ID, id)
	}

	got, err := queries.GetAPIKeyByID(ctx, id)
	if err != nil {
		t.Fatalf("GetAPIKeyByID() error = %v", err)
	}
	if got.ID != id {
		t.Fatalf("GetAPIKeyByID() id = %v, want %v", got.ID, id)
	}
}
