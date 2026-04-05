//go:build smoke

package services

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/repository/testutil"
)

func TestCreateServiceRoundTripSmoke(t *testing.T) {
	db := testutil.OpenSmokeDB(t)
	queries := New(db)
	ctx := context.Background()
	userID := testutil.MustCreateUser(t, db)
	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)
	name := fmt.Sprintf("repo-service-%s", id.String()[:8])

	created, err := queries.CreateService(ctx, CreateServiceParams{
		ID:               id,
		Name:             name,
		CreatedAt:        now,
		UpdatedAt:        sql.NullTime{},
		Active:           true,
		MessageLimit:     1000,
		Restricted:       false,
		EmailFrom:        name + "@example.com",
		CreatedByID:      userID,
		Version:          1,
		ResearchMode:     false,
		PrefixSms:        false,
		RateLimit:        1000,
		CountAsLive:      true,
		SmsDailyLimit:    1000,
		EmailAnnualLimit: 20000000,
		SmsAnnualLimit:   100000,
	})
	if err != nil {
		t.Fatalf("CreateService() error = %v", err)
	}
	if created.ID != id {
		t.Fatalf("CreateService() id = %v, want %v", created.ID, id)
	}

	got, err := queries.GetServiceByID(ctx, GetServiceByIDParams{ID: id, OnlyActive: false})
	if err != nil {
		t.Fatalf("GetServiceByID() error = %v", err)
	}
	if got.ID != id {
		t.Fatalf("GetServiceByID() id = %v, want %v", got.ID, id)
	}
}
