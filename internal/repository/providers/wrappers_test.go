package providers

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestUpdateProviderDetailsBumpsVersionAndWritesHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	queries := New(db)
	ctx := context.Background()
	id := uuid.New()
	userID := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)
	priority := int32(20)
	active := false

	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, display_name, identifier, priority, notification_type, active, updated_at, version, created_by_id, supports_international
		FROM provider_details
		WHERE id = $1
	`)).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"id", "display_name", "identifier", "priority", "notification_type", "active", "updated_at", "version", "created_by_id", "supports_international"}).
			AddRow(id, "SNS", "sns", int32(10), "sms", true, nil, int32(1), nil, true))

	mock.ExpectQuery(regexp.QuoteMeta(updateProvider)).
		WithArgs("SNS", "sns", priority, NotificationTypeSms, active, sql.NullTime{Time: now, Valid: true}, int32(2), uuid.NullUUID{UUID: userID, Valid: true}, true, id).
		WillReturnRows(sqlmock.NewRows([]string{"id", "display_name", "identifier", "priority", "notification_type", "active", "updated_at", "version", "created_by_id", "supports_international"}).
			AddRow(id, "SNS", "sns", priority, "sms", active, now, int32(2), userID, true))

	mock.ExpectExec(regexp.QuoteMeta(insertProviderHistory)).
		WithArgs(id, "SNS", "sns", priority, NotificationTypeSms, active, int32(2), sql.NullTime{Time: now, Valid: true}, uuid.NullUUID{UUID: userID, Valid: true}, true).
		WillReturnResult(sqlmock.NewResult(1, 1))

	got, err := queries.UpdateProviderDetails(ctx, UpdateProviderDetailsParams{
		ID:          id,
		Priority:    &priority,
		Active:      &active,
		CreatedByID: &userID,
	})
	if err != nil {
		t.Fatalf("UpdateProviderDetails() error = %v", err)
	}
	if got.Version != 2 {
		t.Fatalf("version = %d, want 2", got.Version)
	}
	if !got.CreatedByID.Valid || got.CreatedByID.UUID != userID {
		t.Fatalf("created_by_id = %#v, want %v", got.CreatedByID, userID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestSwitchSMSProviderToIdentifierSwapsPriorities(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	queries := New(db)
	ctx := context.Background()
	currentID := uuid.New()
	targetID := uuid.New()
	notifyUserID, _ := uuid.Parse("6af522d0-2915-4e52-83a3-3690455a5fe6")
	now := time.Now().UTC().Truncate(time.Second)

	expectProviderByIdentifier(mock, targetID, "pinpoint", 50, true, 1)
	expectCurrentProvider(mock, currentID, "sns", 10, true, 1)
	expectProviderByID(mock, targetID, "pinpoint", 50, true, 1)
	expectUpdateWithHistory(mock, targetID, "pinpoint", 10, true, 2, notifyUserID, now)
	expectProviderByID(mock, currentID, "sns", 10, true, 1)
	expectUpdateWithHistory(mock, currentID, "sns", 50, true, 2, notifyUserID, now)

	if err := queries.SwitchSMSProviderToIdentifier(ctx, "pinpoint"); err != nil {
		t.Fatalf("SwitchSMSProviderToIdentifier() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestSwitchSMSProviderToIdentifierNoOps(t *testing.T) {
	t.Run("current provider", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New() error = %v", err)
		}
		defer db.Close()

		queries := New(db)
		ctx := context.Background()
		id := uuid.New()

		expectProviderByIdentifier(mock, id, "sns", 10, true, 1)
		expectCurrentProvider(mock, id, "sns", 10, true, 1)

		if err := queries.SwitchSMSProviderToIdentifier(ctx, "sns"); err != nil {
			t.Fatalf("SwitchSMSProviderToIdentifier() error = %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("sql expectations: %v", err)
		}
	})

	t.Run("inactive provider", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New() error = %v", err)
		}
		defer db.Close()

		queries := New(db)
		ctx := context.Background()
		id := uuid.New()

		expectProviderByIdentifier(mock, id, "mmg", 20, false, 1)

		if err := queries.SwitchSMSProviderToIdentifier(ctx, "mmg"); err != nil {
			t.Fatalf("SwitchSMSProviderToIdentifier() error = %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("sql expectations: %v", err)
		}
	})
}

func TestGetAlternativeSMSProviderReturnsSameProvider(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	queries := New(db)
	ctx := context.Background()
	id := uuid.New()
	expectProviderByIdentifier(mock, id, "sns", 10, true, 1)

	got, err := queries.GetAlternativeSMSProvider(ctx, "sns")
	if err != nil {
		t.Fatalf("GetAlternativeSMSProvider() error = %v", err)
	}
	if got.Identifier != "sns" {
		t.Fatalf("identifier = %q, want sns", got.Identifier)
	}
}

func TestToggleSMSProviderByIdentifierCallsSwitch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	queries := New(db)
	ctx := context.Background()
	id := uuid.New()

	expectProviderByIdentifier(mock, id, "sns", 10, true, 1)
	expectProviderByIdentifier(mock, id, "sns", 10, true, 1)
	expectCurrentProvider(mock, id, "sns", 10, true, 1)

	if err := queries.ToggleSMSProviderByIdentifier(ctx, "sns"); err != nil {
		t.Fatalf("ToggleSMSProviderByIdentifier() error = %v", err)
	}
}

func TestGetDaoProviderStatsIncludesBillingAndZeros(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	queries := New(db)
	ctx := context.Background()
	id1 := uuid.New()
	id2 := uuid.New()

	mock.ExpectQuery("SELECT\\s+p\\.id").WillReturnRows(
		sqlmock.NewRows([]string{"id", "display_name", "identifier", "priority", "notification_type", "active", "updated_at", "version", "created_by_id", "supports_international", "created_by_name", "current_month_billable_sms"}).
			AddRow(id1, "SES", "ses", int32(5), "email", true, nil, int32(1), nil, false, "", int64(0)).
			AddRow(id2, "SNS", "sns", int32(10), "sms", true, nil, int32(1), nil, true, "Notify User", int64(42)),
	)

	stats, err := queries.GetDaoProviderStats(ctx)
	if err != nil {
		t.Fatalf("GetDaoProviderStats() error = %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("len(stats) = %d, want 2", len(stats))
	}
	if stats[0].CurrentMonthBillableSMS != 0 {
		t.Fatalf("first current_month_billable_sms = %d, want 0", stats[0].CurrentMonthBillableSMS)
	}
	if stats[1].CurrentMonthBillableSMS != 42 {
		t.Fatalf("second current_month_billable_sms = %d, want 42", stats[1].CurrentMonthBillableSMS)
	}
}

func expectProviderByIdentifier(mock sqlmock.Sqlmock, id uuid.UUID, identifier string, priority int32, active bool, version int32) {
	mock.ExpectQuery("SELECT id, display_name, identifier, priority, notification_type, active, updated_at, version, created_by_id, supports_international\\s+FROM provider_details\\s+WHERE identifier = \\$1").
		WithArgs(identifier).
		WillReturnRows(sqlmock.NewRows([]string{"id", "display_name", "identifier", "priority", "notification_type", "active", "updated_at", "version", "created_by_id", "supports_international"}).
			AddRow(id, identifier, identifier, priority, "sms", active, nil, version, nil, true))
}

func expectCurrentProvider(mock sqlmock.Sqlmock, id uuid.UUID, identifier string, priority int32, active bool, version int32) {
	mock.ExpectQuery("SELECT id, display_name, identifier, priority, notification_type, active, updated_at, version, created_by_id, supports_international\\s+FROM provider_details\\s+WHERE notification_type = \\$1").
		WithArgs(NotificationTypeSms).
		WillReturnRows(sqlmock.NewRows([]string{"id", "display_name", "identifier", "priority", "notification_type", "active", "updated_at", "version", "created_by_id", "supports_international"}).
			AddRow(id, identifier, identifier, priority, "sms", active, nil, version, nil, true))
}

func expectProviderByID(mock sqlmock.Sqlmock, id uuid.UUID, identifier string, priority int32, active bool, version int32) {
	mock.ExpectQuery(regexp.QuoteMeta(getProviderByID)).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"id", "display_name", "identifier", "priority", "notification_type", "active", "updated_at", "version", "created_by_id", "supports_international"}).
			AddRow(id, identifier, identifier, priority, "sms", active, nil, version, nil, true))
}

func expectUpdateWithHistory(mock sqlmock.Sqlmock, id uuid.UUID, identifier string, priority int32, active bool, version int32, userID uuid.UUID, now time.Time) {
	mock.ExpectQuery(regexp.QuoteMeta(updateProvider)).
		WithArgs(identifier, identifier, priority, NotificationTypeSms, active, sql.NullTime{Time: now, Valid: true}, version, uuid.NullUUID{UUID: userID, Valid: true}, true, id).
		WillReturnRows(sqlmock.NewRows([]string{"id", "display_name", "identifier", "priority", "notification_type", "active", "updated_at", "version", "created_by_id", "supports_international"}).
			AddRow(id, identifier, identifier, priority, "sms", active, now, version, userID, true))
	mock.ExpectExec(regexp.QuoteMeta(insertProviderHistory)).
		WithArgs(id, identifier, identifier, priority, NotificationTypeSms, active, version, sql.NullTime{Time: now, Valid: true}, uuid.NullUUID{UUID: userID, Valid: true}, true).
		WillReturnResult(sqlmock.NewResult(1, 1))
}
