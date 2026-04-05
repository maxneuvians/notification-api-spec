//go:build smoke

package annual_limits

import (
	"context"
	"testing"

	"github.com/maxneuvians/notification-api-spec/internal/repository/testutil"
)

func TestInsertQuarterDataRoundTripSmoke(t *testing.T) {
	db := testutil.OpenSmokeDB(t)
	queries := New(db)
	ctx := context.Background()
	userID := testutil.MustCreateUser(t, db)
	serviceID := testutil.MustCreateService(t, db, userID)

	if err := queries.InsertQuarterData(ctx, InsertQuarterDataParams{
		ServiceID:         serviceID,
		TimePeriod:        "2026-Q1",
		AnnualEmailLimit:  100,
		AnnualSmsLimit:    200,
		NotificationType:  "email",
		NotificationCount: 12,
	}); err != nil {
		t.Fatalf("InsertQuarterData() error = %v", err)
	}

	got, err := queries.GetAnnualLimitsDataByServiceAndPeriod(ctx, GetAnnualLimitsDataByServiceAndPeriodParams{ServiceID: serviceID, TimePeriod: "2026-Q1"})
	if err != nil {
		t.Fatalf("GetAnnualLimitsDataByServiceAndPeriod() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("GetAnnualLimitsDataByServiceAndPeriod() len = %d, want 1", len(got))
	}
}
