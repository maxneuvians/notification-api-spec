package services

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/lib/pq"

	usersRepo "github.com/maxneuvians/notification-api-spec/internal/repository/users"
	serviceerrs "github.com/maxneuvians/notification-api-spec/internal/service/services"
	"github.com/maxneuvians/notification-api-spec/pkg/signing"
)

var serviceColumns = []string{
	"id", "name", "created_at", "updated_at", "active", "message_limit", "restricted", "email_from", "created_by_id", "version",
	"research_mode", "organisation_type", "prefix_sms", "crown", "rate_limit", "contact_link", "consent_to_research", "volume_email",
	"volume_letter", "volume_sms", "count_as_live", "go_live_at", "go_live_user_id", "organisation_id", "sending_domain",
	"default_branding_is_french", "sms_daily_limit", "organisation_notes", "sensitive_service", "email_annual_limit", "sms_annual_limit",
	"suspended_by_id", "suspended_at",
}

var apiKeyColumns = []string{
	"id", "name", "secret", "service_id", "expiry_date", "created_at", "created_by_id", "updated_at", "version", "key_type", "compromised_key_info", "last_used_timestamp",
}

var userColumns = []string{
	"id", "name", "email_address", "created_at", "updated_at", "_password", "mobile_number", "password_changed_at", "logged_in_at", "failed_login_count", "state", "platform_admin", "current_session_id", "auth_type", "blocked", "additional_information", "password_expired", "verified_phonenumber", "default_editor_is_rte",
}

var templateColumns = []string{
	"id", "name", "template_type", "created_at", "updated_at", "content", "service_id", "subject", "created_by_id", "version", "archived", "process_type", "service_letter_contact_id", "hidden", "postage", "template_category_id", "text_direction_rtl",
}

func TestFetchAllServicesOrdersByCreatedAt(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	ctx := context.Background()
	older := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	newer := older.Add(30 * time.Minute)
	userID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(getAllServices)).
		WithArgs(false).
		WillReturnRows(sqlmock.NewRows(serviceColumns).
			AddRow(uuid.New(), "z-service", newer, nil, true, int64(1000), false, "z@example.com", userID, int32(1), false, nil, true, nil, int32(1000), nil, nil, nil, nil, nil, true, nil, nil, nil, nil, nil, int64(1000), nil, nil, int64(20000000), int64(100000), nil, nil).
			AddRow(uuid.New(), "a-service", older, nil, true, int64(1000), false, "a@example.com", userID, int32(1), false, nil, true, nil, int32(1000), nil, nil, nil, nil, nil, true, nil, nil, nil, nil, nil, int64(1000), nil, nil, int64(20000000), int64(100000), nil, nil))

	items, err := repo.FetchAllServices(ctx, false)
	if err != nil {
		t.Fatalf("FetchAllServices() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if !items[0].CreatedAt.Equal(older) || items[0].Name != "a-service" {
		t.Fatalf("first service = %#v, want older service first", items[0])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestGetServicesByPartialNameUsesCaseInsensitiveMatchAndOrdersByCreatedAt(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	ctx := context.Background()
	older := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	newer := older.Add(30 * time.Minute)
	userID := uuid.New()
	queryArg := sql.NullString{String: "ALPHA", Valid: true}

	mock.ExpectQuery(regexp.QuoteMeta(getServicesByPartialName)).
		WithArgs(queryArg).
		WillReturnRows(sqlmock.NewRows(serviceColumns).
			AddRow(uuid.New(), "alpha-two", newer, nil, true, int64(1000), false, "two@example.com", userID, int32(1), false, nil, true, nil, int32(1000), nil, nil, nil, nil, nil, true, nil, nil, nil, nil, nil, int64(1000), nil, nil, int64(20000000), int64(100000), nil, nil).
			AddRow(uuid.New(), "Alpha One", older, nil, true, int64(1000), false, "one@example.com", userID, int32(1), false, nil, true, nil, int32(1000), nil, nil, nil, nil, nil, true, nil, nil, nil, nil, nil, int64(1000), nil, nil, int64(20000000), int64(100000), nil, nil))

	items, err := repo.GetServicesByPartialName(ctx, "ALPHA")
	if err != nil {
		t.Fatalf("GetServicesByPartialName() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].Name != "Alpha One" {
		t.Fatalf("first result = %q, want Alpha One", items[0].Name)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestFetchServicesByUserIDHonorsOnlyActive(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	ctx := context.Background()
	userID := uuid.New()
	createdAt := time.Now().UTC().Truncate(time.Second)

	mock.ExpectQuery(regexp.QuoteMeta(getServicesByUserID)).
		WithArgs(uuid.NullUUID{UUID: userID, Valid: true}, true).
		WillReturnRows(sqlmock.NewRows(serviceColumns).
			AddRow(uuid.New(), "active-service", createdAt, nil, true, int64(1000), false, "active@example.com", userID, int32(1), false, nil, true, nil, int32(1000), nil, nil, nil, nil, nil, true, nil, nil, nil, nil, nil, int64(1000), nil, nil, int64(20000000), int64(100000), nil, nil))

	items, err := repo.FetchServicesByUserID(ctx, userID, true)
	if err != nil {
		t.Fatalf("FetchServicesByUserID() error = %v", err)
	}
	if len(items) != 1 || !items[0].Active {
		t.Fatalf("items = %#v, want one active service", items)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestFetchServiceByInboundNumberReturnsNilOnMiss(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	mock.ExpectQuery(regexp.QuoteMeta(getServiceByInboundNumber)).
		WithArgs("+16135550123").
		WillReturnError(sql.ErrNoRows)

	item, err := repo.FetchServiceByInboundNumber(context.Background(), "+16135550123")
	if err != nil {
		t.Fatalf("FetchServiceByInboundNumber() error = %v", err)
	}
	if item != nil {
		t.Fatalf("item = %#v, want nil", item)
	}
}

func TestFetchServiceByInboundNumberReturnsServiceForActiveInboundNumber(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	userID := uuid.New()
	createdAt := time.Now().UTC().Truncate(time.Second)
	mock.ExpectQuery(regexp.QuoteMeta(getServiceByInboundNumber)).
		WithArgs("+16135550123").
		WillReturnRows(sqlmock.NewRows(serviceColumns).
			AddRow(serviceID, "service", createdAt, nil, true, int64(1000), false, "service@example.com", userID, int32(1), false, nil, true, nil, int32(1000), nil, nil, nil, nil, nil, true, nil, nil, nil, nil, nil, int64(1000), nil, nil, int64(20000000), int64(100000), nil, nil))

	item, err := repo.FetchServiceByInboundNumber(context.Background(), "+16135550123")
	if err != nil {
		t.Fatalf("FetchServiceByInboundNumber() error = %v", err)
	}
	if item == nil || item.ID != serviceID {
		t.Fatalf("item = %#v, want service %v", item, serviceID)
	}
}

func TestFetchServiceWithAPIKeysUsesReaderReplica(t *testing.T) {
	readerDB, readerMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer readerDB.Close()
	writerDB, writerMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer writerDB.Close()

	repo := NewRepository(readerDB, writerDB)
	serviceID := uuid.New()
	userID := uuid.New()
	createdAt := time.Now().UTC().Truncate(time.Second)
	readerMock.ExpectQuery(regexp.QuoteMeta(getServiceByIDWithAPIKeys)).
		WithArgs(serviceID).
		WillReturnRows(sqlmock.NewRows(serviceColumns).
			AddRow(serviceID, "service", createdAt, nil, true, int64(1000), false, "service@example.com", userID, int32(1), false, nil, true, nil, int32(1000), nil, nil, nil, nil, nil, true, nil, nil, nil, nil, nil, int64(1000), nil, nil, int64(20000000), int64(100000), nil, nil))

	item, err := repo.FetchServiceWithAPIKeys(context.Background(), serviceID)
	if err != nil {
		t.Fatalf("FetchServiceWithAPIKeys() error = %v", err)
	}
	if item == nil || item.ID != serviceID {
		t.Fatalf("item = %#v, want service %v", item, serviceID)
	}
	if err := readerMock.ExpectationsWereMet(); err != nil {
		t.Fatalf("reader sql expectations: %v", err)
	}
	if err := writerMock.ExpectationsWereMet(); err != nil {
		t.Fatalf("writer sql expectations: %v", err)
	}
}

func TestFetchActiveUsersForServiceFiltersToActiveUsers(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	activeUser := baseUserRow(uuid.New(), time.Now().UTC().Truncate(time.Second))

	mock.ExpectQuery(regexp.QuoteMeta(listActiveUsersByServiceQuery)).
		WithArgs(serviceID).
		WillReturnRows(userRows(activeUser, nil))

	items, err := repo.FetchActiveUsersForService(context.Background(), serviceID)
	if err != nil {
		t.Fatalf("FetchActiveUsersForService() error = %v", err)
	}
	if len(items) != 1 || items[0].State != "active" {
		t.Fatalf("items = %#v", items)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestFetchServiceCreatorReturnsCreatorFromFirstHistoryVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	creator := baseUserRow(uuid.New(), time.Now().UTC().Truncate(time.Second))

	mock.ExpectQuery(regexp.QuoteMeta(getServiceCreatorQuery)).
		WithArgs(serviceID).
		WillReturnRows(userRows(creator, nil))

	item, err := repo.FetchServiceCreator(context.Background(), serviceID)
	if err != nil {
		t.Fatalf("FetchServiceCreator() error = %v", err)
	}
	if item == nil || item.ID != creator.ID {
		t.Fatalf("item = %#v", item)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestFetchLiveServicesDataUsesGeneratedQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	goLiveAt := sql.NullTime{Time: time.Now().UTC().Truncate(time.Second), Valid: true}

	mock.ExpectQuery(regexp.QuoteMeta(getLiveServicesData)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "active", "count_as_live", "go_live_at", "suspended_at"}).
			AddRow(serviceID, "live-service", true, true, goLiveAt, sql.NullTime{}))

	items, err := repo.FetchLiveServicesData(context.Background())
	if err != nil {
		t.Fatalf("FetchLiveServicesData() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != serviceID {
		t.Fatalf("items = %#v", items)
	}
}

func TestFetchSensitiveServiceIDsUsesGeneratedQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	firstID := uuid.New()
	secondID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(getSensitiveServiceIDs)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(firstID).AddRow(secondID))

	items, err := repo.FetchSensitiveServiceIDs(context.Background())
	if err != nil {
		t.Fatalf("FetchSensitiveServiceIDs() error = %v", err)
	}
	if len(items) != 2 || items[0] != firstID || items[1] != secondID {
		t.Fatalf("items = %#v", items)
	}
}

func TestFetchTodaysStatsForAllServicesHonorsFlagsAndScansRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)

	mock.ExpectQuery(regexp.QuoteMeta(listStatsForAllServicesByDateRangeQuery)).
		WithArgs(false, true, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "restricted", "research_mode", "active", "created_at", "notification_type", "notification_status", "count"}).
			AddRow(serviceID, "service", false, false, true, createdAt, string(NotificationTypeEmail), string(NotifyStatusTypeDelivered), int64(3)))

	items, err := repo.FetchTodaysStatsForAllServices(context.Background(), false, true)
	if err != nil {
		t.Fatalf("FetchTodaysStatsForAllServices() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].NotificationType.NotificationType != NotificationTypeEmail || items[0].NotificationStatus.NotifyStatusType != NotifyStatusTypeDelivered || items[0].Count != 3 {
		t.Fatalf("items[0] = %#v", items[0])
	}
}

func TestFetchStatsForAllServicesUsesSuppliedDateRange(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	createdAt := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	startDate := time.Date(2026, time.April, 1, 14, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, time.April, 3, 9, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta(listStatsForAllServicesByDateRangeQuery)).
		WithArgs(true, false, normalizeDateOnly(startDate), normalizeDateOnly(endDate)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "restricted", "research_mode", "active", "created_at", "notification_type", "notification_status", "count"}).
			AddRow(serviceID, "service", false, false, true, createdAt, string(NotificationTypeSms), string(NotifyStatusTypeSent), int64(5)))

	items, err := repo.FetchStatsForAllServices(context.Background(), true, false, startDate, endDate)
	if err != nil {
		t.Fatalf("FetchStatsForAllServices() error = %v", err)
	}
	if len(items) != 1 || items[0].ServiceID != serviceID || items[0].Count != 5 {
		t.Fatalf("items = %#v", items)
	}
}

func TestFetchStatsForServiceScansGroupedRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(listStatsForServiceQuery)).
		WithArgs(serviceID, 7).
		WillReturnRows(sqlmock.NewRows([]string{"notification_type", "notification_status", "notification_count"}).
			AddRow(string(NotificationTypeEmail), string(NotifyStatusTypeDelivered), int64(4)).
			AddRow(string(NotificationTypeSms), string(NotifyStatusTypeSent), int64(2)))

	items, err := repo.FetchStatsForService(context.Background(), serviceID, 7)
	if err != nil {
		t.Fatalf("FetchStatsForService() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].NotificationType != NotificationTypeEmail || items[0].NotificationStatus.NotifyStatusType != NotifyStatusTypeDelivered || items[0].Count != 4 {
		t.Fatalf("items[0] = %#v", items[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestFetchMonthlyUsageForServiceScansRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(listMonthlyUsageForServiceQuery)).
		WithArgs(serviceID, time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC), time.Date(2027, time.April, 1, 0, 0, 0, 0, time.UTC)).
		WillReturnRows(sqlmock.NewRows([]string{"month_label", "notification_type", "notification_status", "notification_count"}).
			AddRow("2026-04", string(NotificationTypeEmail), string(NotifyStatusTypeDelivered), int64(4)).
			AddRow("2026-05", string(NotificationTypeSms), string(NotifyStatusTypeFailed), int64(2)))

	items, err := repo.FetchMonthlyUsageForService(context.Background(), serviceID, 2026)
	if err != nil {
		t.Fatalf("FetchMonthlyUsageForService() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].Month != "2026-04" || items[0].NotificationType.NotificationType != NotificationTypeEmail || items[0].Count != 4 {
		t.Fatalf("items[0] = %#v", items[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestFetchTodaysStatsForServiceUsesTodayQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(listTodaysStatsForServiceQuery)).
		WithArgs(serviceID).
		WillReturnRows(sqlmock.NewRows([]string{"notification_type", "notification_status", "notification_count"}).
			AddRow(string(NotificationTypeLetter), string(NotifyStatusTypeCreated), int64(1)))

	items, err := repo.FetchTodaysStatsForService(context.Background(), serviceID)
	if err != nil {
		t.Fatalf("FetchTodaysStatsForService() error = %v", err)
	}
	if len(items) != 1 || items[0].NotificationType != NotificationTypeLetter || items[0].Count != 1 {
		t.Fatalf("items = %#v", items)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestFetchTodaysTotalsExcludeTestKeysAndCountScheduledJobs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(getTodaysTotalMessageCountQuery)).
		WithArgs(serviceID).
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(int64(9)))
	mock.ExpectQuery(regexp.QuoteMeta(getTodaysTotalSMSCountQuery)).
		WithArgs(serviceID).
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(int64(3)))
	mock.ExpectQuery(regexp.QuoteMeta(getTodaysTotalSMSBillableUnitsQuery)).
		WithArgs(serviceID).
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(int64(7)))
	mock.ExpectQuery(regexp.QuoteMeta(getTodaysTotalEmailCountQuery)).
		WithArgs(serviceID).
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow(int64(5)))

	messageCount, err := repo.FetchTodaysTotalMessageCount(context.Background(), serviceID)
	if err != nil {
		t.Fatalf("FetchTodaysTotalMessageCount() error = %v", err)
	}
	if messageCount != 9 {
		t.Fatalf("messageCount = %d, want 9", messageCount)
	}

	smsCount, err := repo.FetchTodaysTotalSmsCount(context.Background(), serviceID)
	if err != nil {
		t.Fatalf("FetchTodaysTotalSmsCount() error = %v", err)
	}
	if smsCount != 3 {
		t.Fatalf("smsCount = %d, want 3", smsCount)
	}

	billableUnits, err := repo.FetchTodaysTotalSmsBillableUnits(context.Background(), serviceID)
	if err != nil {
		t.Fatalf("FetchTodaysTotalSmsBillableUnits() error = %v", err)
	}
	if billableUnits != 7 {
		t.Fatalf("billableUnits = %d, want 7", billableUnits)
	}

	emailCount, err := repo.FetchTodaysTotalEmailCount(context.Background(), serviceID)
	if err != nil {
		t.Fatalf("FetchTodaysTotalEmailCount() error = %v", err)
	}
	if emailCount != 5 {
		t.Fatalf("emailCount = %d, want 5", emailCount)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestFetchServiceEmailLimitReturnsValueAndZeroOnMiss(t *testing.T) {
	t.Run("returns configured limit", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New() error = %v", err)
		}
		defer db.Close()

		repo := NewRepository(db, db)
		serviceID := uuid.New()

		mock.ExpectQuery(regexp.QuoteMeta(getServiceEmailLimitQuery)).
			WithArgs(serviceID).
			WillReturnRows(sqlmock.NewRows([]string{"message_limit"}).AddRow(int64(250000)))

		limit, err := repo.FetchServiceEmailLimit(context.Background(), serviceID)
		if err != nil {
			t.Fatalf("FetchServiceEmailLimit() error = %v", err)
		}
		if limit != 250000 {
			t.Fatalf("limit = %d, want 250000", limit)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("sql expectations: %v", err)
		}
	})

	t.Run("returns zero when service missing", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New() error = %v", err)
		}
		defer db.Close()

		repo := NewRepository(db, db)
		serviceID := uuid.New()

		mock.ExpectQuery(regexp.QuoteMeta(getServiceEmailLimitQuery)).
			WithArgs(serviceID).
			WillReturnError(sql.ErrNoRows)

		limit, err := repo.FetchServiceEmailLimit(context.Background(), serviceID)
		if err != nil {
			t.Fatalf("FetchServiceEmailLimit() error = %v", err)
		}
		if limit != 0 {
			t.Fatalf("limit = %d, want 0", limit)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("sql expectations: %v", err)
		}
	})
}

func TestAddUserToServiceSetsDefaultPermissionsAndIgnoresMissingFolders(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	userID := uuid.New()
	validFolderID := uuid.New()
	missingFolderID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(addUserToService)).
		WithArgs(uuid.NullUUID{UUID: userID, Valid: true}, uuid.NullUUID{UUID: serviceID, Valid: true}).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`WITH deleted AS \(\s*DELETE FROM permissions\s*WHERE service_id = \$1\s*AND user_id = \$2\s*\)\s*INSERT INTO permissions`).
		WithArgs(uuid.NullUUID{UUID: serviceID, Valid: true}, userID, permissionItemsArg{want: defaultUserPermissions}).
		WillReturnResult(sqlmock.NewResult(1, 8))
	mock.ExpectQuery(regexp.QuoteMeta(listTemplateFoldersByIDsQuery)).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "service_id"}).AddRow(validFolderID, serviceID))
	mock.ExpectExec(`WITH deleted AS \(\s*DELETE FROM user_folder_permissions\s*WHERE service_id = \$2\s*AND user_id = \$1\s*\)\s*INSERT INTO user_folder_permissions`).
		WithArgs(userID, serviceID, uuidArrayArg{want: []uuid.UUID{validFolderID}}).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = repo.AddUserToService(context.Background(), serviceID, userID, nil, []uuid.UUID{validFolderID, missingFolderID})
	if err != nil {
		t.Fatalf("AddUserToService() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestAddUserToServiceRollsBackForCrossServiceFolder(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	otherServiceID := uuid.New()
	userID := uuid.New()
	foreignFolderID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(addUserToService)).
		WithArgs(uuid.NullUUID{UUID: userID, Valid: true}, uuid.NullUUID{UUID: serviceID, Valid: true}).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`WITH deleted AS \(\s*DELETE FROM permissions\s*WHERE service_id = \$1\s*AND user_id = \$2\s*\)\s*INSERT INTO permissions`).
		WithArgs(uuid.NullUUID{UUID: serviceID, Valid: true}, userID, permissionItemsArg{want: []string{"send_emails"}}).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(listTemplateFoldersByIDsQuery)).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "service_id"}).AddRow(foreignFolderID, otherServiceID))
	mock.ExpectRollback()

	err = repo.AddUserToService(context.Background(), serviceID, userID, []string{"send_emails"}, []uuid.UUID{foreignFolderID})
	if err == nil {
		t.Fatal("AddUserToService() error = nil, want error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRemoveUserFromServiceClearsOnlyServiceScopedPermissions(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	userID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(`WITH deleted AS \(\s*DELETE FROM user_folder_permissions\s*WHERE service_id = \$2\s*AND user_id = \$1\s*\)\s*INSERT INTO user_folder_permissions`).
		WithArgs(userID, serviceID, uuidArrayArg{want: []uuid.UUID{}}).
		WillReturnResult(sqlmock.NewResult(1, 0))
	mock.ExpectExec(`WITH deleted AS \(\s*DELETE FROM permissions\s*WHERE service_id = \$1\s*AND user_id = \$2\s*\)\s*INSERT INTO permissions`).
		WithArgs(uuid.NullUUID{UUID: serviceID, Valid: true}, userID, permissionItemsArg{want: []string{}}).
		WillReturnResult(sqlmock.NewResult(1, 0))
	mock.ExpectExec(regexp.QuoteMeta(removeUserFromService)).
		WithArgs(uuid.NullUUID{UUID: userID, Valid: true}, uuid.NullUUID{UUID: serviceID, Valid: true}).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = repo.RemoveUserFromService(context.Background(), serviceID, userID)
	if err != nil {
		t.Fatalf("RemoveUserFromService() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestCreateServiceReturnsErrorWhenUserNil(t *testing.T) {
	repo := NewRepository(nil, nil, WithPlatformFromNumber("+16135550000"))
	service := baseServiceRow(uuid.New(), uuid.New(), time.Now().UTC().Truncate(time.Second))

	created, err := repo.CreateService(context.Background(), service, nil, nil)
	if err == nil || err.Error() != "can't create a service without a user" {
		t.Fatalf("CreateService() error = %v", err)
	}
	if created != nil {
		t.Fatalf("created = %#v, want nil", created)
	}
}

func TestCreateServiceWritesHistoryPermissionsAndDefaultSender(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	fromNumber := "+16135550000"
	repo := NewRepository(db, db, WithPlatformFromNumber(fromNumber))
	serviceID := uuid.New()
	userID := uuid.New()
	createdAt := time.Now().UTC().Truncate(time.Second)
	service := baseServiceRow(serviceID, userID, createdAt)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(createService)).
		WithArgs(
			serviceID,
			service.Name,
			createdAt,
			sql.NullTime{},
			service.Active,
			service.MessageLimit,
			service.Restricted,
			service.EmailFrom,
			userID,
			int32(1),
			service.ResearchMode,
			service.OrganisationType,
			service.PrefixSms,
			service.Crown,
			service.RateLimit,
			service.ContactLink,
			service.ConsentToResearch,
			service.VolumeEmail,
			service.VolumeLetter,
			service.VolumeSms,
			service.CountAsLive,
			service.GoLiveAt,
			service.GoLiveUserID,
			service.OrganisationID,
			service.SendingDomain,
			service.DefaultBrandingIsFrench,
			service.SmsDailyLimit,
			service.OrganisationNotes,
			service.SensitiveService,
			service.EmailAnnualLimit,
			service.SmsAnnualLimit,
			service.SuspendedByID,
			service.SuspendedAt,
		).
		WillReturnRows(serviceRows(service, nil))
	mock.ExpectExec(regexp.QuoteMeta(insertServicesHistoryRow)).
		WithArgs(
			serviceID, service.Name, createdAt, sql.NullTime{}, true, service.MessageLimit, false, service.EmailFrom,
			userID, int32(1), false, service.OrganisationType, sql.NullBool{Bool: true, Valid: true}, service.Crown, service.RateLimit,
			service.ContactLink, service.ConsentToResearch, service.VolumeEmail, service.VolumeLetter, service.VolumeSms, true, service.GoLiveAt,
			service.GoLiveUserID, service.OrganisationID, service.SendingDomain, service.DefaultBrandingIsFrench, service.SmsDailyLimit,
			service.OrganisationNotes, service.SensitiveService, service.EmailAnnualLimit, service.SmsAnnualLimit, service.SuspendedByID, service.SuspendedAt,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(setServicePermissions)).
		WithArgs(serviceID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 3))
	mock.ExpectQuery(regexp.QuoteMeta(createSMSSender)).
		WithArgs(sqlmock.AnyArg(), fromNumber, serviceID, true, uuid.NullUUID{}, sqlmock.AnyArg(), sql.NullTime{}, false).
		WillReturnRows(sqlmock.NewRows([]string{"id", "sms_sender", "service_id", "is_default", "inbound_number_id", "created_at", "updated_at", "archived"}).
			AddRow(uuid.New(), fromNumber, serviceID, true, nil, createdAt, nil, false))
	mock.ExpectCommit()

	created, err := repo.CreateService(context.Background(), service, &userID, nil)
	if err != nil {
		t.Fatalf("CreateService() error = %v", err)
	}
	if created == nil || created.ID != serviceID {
		t.Fatalf("created = %#v", created)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestCreateAPIKeyStoresSignedSecretAndReturnsPlaintext(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db, func(r *Repository) {
		r.apiKeyPrefix = "gcntfy-"
		r.apiKeySecrets = []string{"current-secret"}
	})
	serviceID := uuid.New()
	userID := uuid.New()
	createdAt := time.Now().UTC().Truncate(time.Second)

	mock.ExpectQuery("INSERT INTO api_keys").
		WithArgs(
			sqlmock.AnyArg(),
			"primary",
			sqlmock.AnyArg(),
			serviceID,
			sql.NullTime{},
			sqlmock.AnyArg(),
			userID,
			sql.NullTime{},
			int32(1),
			"normal",
			json.RawMessage(`null`),
			sql.NullTime{},
		).
		WillReturnRows(apiKeyRows(baseAPIKeyRow(uuid.New(), serviceID, userID, createdAt), nil))
	mock.ExpectExec("INSERT INTO api_keys_history").
		WillReturnResult(sqlmock.NewResult(1, 1))

	created, err := repo.CreateAPIKey(context.Background(), serviceID, "primary", userID, "normal")
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if created == nil {
		t.Fatal("created = nil")
	}
	if !strings.HasPrefix(created.Key, "gcntfy-"+serviceID.String()) {
		t.Fatalf("key = %q, want prefixed plaintext", created.Key)
	}
	if created.APIKey.Secret == created.Key {
		t.Fatalf("stored secret should not equal plaintext key")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestGetSmsSenderByIDAndListExcludeArchivedAndKeepDefaultFirst(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	defaultID := uuid.New()
	nonDefaultID := uuid.New()
	createdAt := time.Now().UTC().Truncate(time.Second)

	mock.ExpectQuery(regexp.QuoteMeta(getSMSSenders)).
		WithArgs(serviceID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "sms_sender", "service_id", "is_default", "inbound_number_id", "created_at", "updated_at", "archived"}).
			AddRow(defaultID, "+16135550000", serviceID, true, nil, createdAt, nil, false).
			AddRow(nonDefaultID, "+16135550001", serviceID, false, nil, createdAt.Add(time.Minute), nil, false))

	items, err := repo.GetSmsSendersByServiceID(context.Background(), serviceID)
	if err != nil {
		t.Fatalf("GetSmsSendersByServiceID() error = %v", err)
	}
	if len(items) != 2 || !items[0].IsDefault || items[0].ID != defaultID {
		t.Fatalf("items = %#v", items)
	}

	mock.ExpectQuery(regexp.QuoteMeta(getSMSSenders)).
		WithArgs(serviceID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "sms_sender", "service_id", "is_default", "inbound_number_id", "created_at", "updated_at", "archived"}).
			AddRow(defaultID, "+16135550000", serviceID, true, nil, createdAt, nil, false))

	item, err := repo.GetSmsSenderByID(context.Background(), serviceID, defaultID)
	if err != nil {
		t.Fatalf("GetSmsSenderByID() error = %v", err)
	}
	if item == nil || item.ID != defaultID {
		t.Fatalf("item = %#v", item)
	}

	mock.ExpectQuery(regexp.QuoteMeta(getSMSSenders)).
		WithArgs(serviceID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "sms_sender", "service_id", "is_default", "inbound_number_id", "created_at", "updated_at", "archived"}))

	missing, err := repo.GetSmsSenderByID(context.Background(), serviceID, uuid.New())
	if err != nil {
		t.Fatalf("GetSmsSenderByID(missing) error = %v", err)
	}
	if missing != nil {
		t.Fatalf("missing = %#v, want nil", missing)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestGetReplyToByIDAndListExcludeArchivedAndKeepDefaultFirst(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	defaultID := uuid.New()
	secondaryID := uuid.New()
	createdAt := time.Now().UTC().Truncate(time.Second)

	mock.ExpectQuery(regexp.QuoteMeta(getEmailReplyTo)).
		WithArgs(serviceID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "service_id", "email_address", "is_default", "created_at", "updated_at", "archived"}).
			AddRow(defaultID, serviceID, "default@example.com", true, createdAt, nil, false).
			AddRow(secondaryID, serviceID, "secondary@example.com", false, createdAt.Add(time.Minute), nil, false))

	items, err := repo.GetReplyTosByServiceID(context.Background(), serviceID)
	if err != nil {
		t.Fatalf("GetReplyTosByServiceID() error = %v", err)
	}
	if len(items) != 2 || !items[0].IsDefault || items[0].ID != defaultID {
		t.Fatalf("items = %#v", items)
	}

	mock.ExpectQuery(regexp.QuoteMeta(getEmailReplyTo)).
		WithArgs(serviceID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "service_id", "email_address", "is_default", "created_at", "updated_at", "archived"}).
			AddRow(defaultID, serviceID, "default@example.com", true, createdAt, nil, false))

	item, err := repo.GetReplyToByID(context.Background(), serviceID, defaultID)
	if err != nil {
		t.Fatalf("GetReplyToByID() error = %v", err)
	}
	if item == nil || item.ID != defaultID {
		t.Fatalf("item = %#v", item)
	}

	mock.ExpectQuery(regexp.QuoteMeta(getEmailReplyTo)).
		WithArgs(serviceID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "service_id", "email_address", "is_default", "created_at", "updated_at", "archived"}))

	missing, err := repo.GetReplyToByID(context.Background(), serviceID, uuid.New())
	if err != nil {
		t.Fatalf("GetReplyToByID(missing) error = %v", err)
	}
	if missing != nil {
		t.Fatalf("missing = %#v, want nil", missing)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestListAPIKeysIncludesExpiredAndCanFilterByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	userID := uuid.New()
	activeID := uuid.New()
	revokedID := uuid.New()
	createdAt := time.Now().UTC().Truncate(time.Second)

	active := baseAPIKeyRow(activeID, serviceID, userID, createdAt)
	revoked := baseAPIKeyRow(revokedID, serviceID, userID, createdAt.Add(-time.Hour))
	revoked.ExpiryDate = sql.NullTime{Time: createdAt, Valid: true}

	mock.ExpectQuery("SELECT id, name, secret, service_id, expiry_date, created_at, created_by_id, updated_at, version, key_type, compromised_key_info, last_used_timestamp FROM api_keys").
		WithArgs(serviceID).
		WillReturnRows(apiKeyRows(active, nil).AddRow(
			revoked.ID,
			revoked.Name,
			revoked.Secret,
			revoked.ServiceID,
			revoked.ExpiryDate,
			revoked.CreatedAt,
			revoked.CreatedByID,
			revoked.UpdatedAt,
			revoked.Version,
			revoked.KeyType,
			revoked.CompromisedKeyInfo,
			revoked.LastUsedTimestamp,
		))

	items, err := repo.ListAPIKeys(context.Background(), serviceID, nil)
	if err != nil {
		t.Fatalf("ListAPIKeys() error = %v", err)
	}
	if len(items) != 2 || !items[1].ExpiryDate.Valid {
		t.Fatalf("items = %#v", items)
	}

	mock.ExpectQuery("SELECT id, name, secret, service_id, expiry_date, created_at, created_by_id, updated_at, version, key_type, compromised_key_info, last_used_timestamp FROM api_keys").
		WithArgs(serviceID, activeID).
		WillReturnRows(apiKeyRows(active, nil))

	filtered, err := repo.ListAPIKeys(context.Background(), serviceID, &activeID)
	if err != nil {
		t.Fatalf("ListAPIKeys(filter) error = %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != activeID {
		t.Fatalf("filtered = %#v", filtered)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRevokeAPIKeySetsExpiryAndWritesHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	userID := uuid.New()
	keyID := uuid.New()
	createdAt := time.Now().UTC().Truncate(time.Second)
	apiKey := baseAPIKeyRow(keyID, serviceID, userID, createdAt)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, name, secret, service_id, expiry_date, created_at, created_by_id, updated_at, version, key_type, compromised_key_info, last_used_timestamp FROM api_keys WHERE id = ").
		WithArgs(keyID).
		WillReturnRows(apiKeyRows(apiKey, nil))
	mock.ExpectQuery("UPDATE api_keys").
		WithArgs(keyID).
		WillReturnRows(apiKeyRows(apiKey, func(item *ApiKey) {
			item.Version = 2
			item.UpdatedAt = sql.NullTime{Time: createdAt, Valid: true}
			item.ExpiryDate = sql.NullTime{Time: createdAt, Valid: true}
		}))
	mock.ExpectExec("INSERT INTO api_keys_history").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	revoked, err := repo.RevokeAPIKey(context.Background(), serviceID, keyID)
	if err != nil {
		t.Fatalf("RevokeAPIKey() error = %v", err)
	}
	if revoked == nil || !revoked.ExpiryDate.Valid || revoked.Version != 2 {
		t.Fatalf("revoked = %#v", revoked)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestUpdateServiceIncrementsVersionAndWritesHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	userID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	updatedAt := time.Now().UTC().Truncate(time.Second)
	service := baseServiceRow(serviceID, userID, createdAt)
	service.Name = "updated-service"
	service.Version = 1

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(updateService)).
		WithArgs(
			service.Name,
			sqlmock.AnyArg(),
			service.Active,
			service.MessageLimit,
			service.Restricted,
			service.EmailFrom,
			int32(2),
			service.ResearchMode,
			service.OrganisationType,
			service.PrefixSms,
			service.Crown,
			service.RateLimit,
			service.ContactLink,
			service.ConsentToResearch,
			service.VolumeEmail,
			service.VolumeLetter,
			service.VolumeSms,
			service.CountAsLive,
			service.GoLiveAt,
			service.GoLiveUserID,
			service.OrganisationID,
			service.SendingDomain,
			service.DefaultBrandingIsFrench,
			service.SmsDailyLimit,
			service.OrganisationNotes,
			service.SensitiveService,
			service.EmailAnnualLimit,
			service.SmsAnnualLimit,
			service.SuspendedByID,
			service.SuspendedAt,
			serviceID,
		).
		WillReturnRows(serviceRows(baseServiceRow(serviceID, userID, createdAt), func(s *Service) {
			s.Name = service.Name
			s.Version = 2
			s.UpdatedAt = sql.NullTime{Time: updatedAt, Valid: true}
		}))
	mock.ExpectExec(regexp.QuoteMeta(insertServicesHistoryRow)).
		WithArgs(
			serviceID, service.Name, createdAt, sql.NullTime{Time: updatedAt, Valid: true}, true, service.MessageLimit, false, service.EmailFrom,
			userID, int32(2), false, service.OrganisationType, sql.NullBool{Bool: true, Valid: true}, service.Crown, service.RateLimit,
			service.ContactLink, service.ConsentToResearch, service.VolumeEmail, service.VolumeLetter, service.VolumeSms, true, service.GoLiveAt,
			service.GoLiveUserID, service.OrganisationID, service.SendingDomain, service.DefaultBrandingIsFrench, service.SmsDailyLimit,
			service.OrganisationNotes, service.SensitiveService, service.EmailAnnualLimit, service.SmsAnnualLimit, service.SuspendedByID, service.SuspendedAt,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	updated, err := repo.UpdateService(context.Background(), service)
	if err != nil {
		t.Fatalf("UpdateService() error = %v", err)
	}
	if updated == nil || updated.Version != 2 || updated.Name != service.Name {
		t.Fatalf("updated = %#v", updated)
	}
}

func TestSuspendServiceWritesHistoryAndAllowsNilUserID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	userID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	suspendedAt := time.Now().UTC().Truncate(time.Second)
	current := baseServiceRow(serviceID, userID, createdAt)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(getServiceByID)).
		WithArgs(serviceID, false).
		WillReturnRows(serviceRows(current, nil))
	mock.ExpectQuery(regexp.QuoteMeta(updateService)).
		WithArgs(
			current.Name, sqlmock.AnyArg(), false, current.MessageLimit, current.Restricted, current.EmailFrom, int32(2), current.ResearchMode,
			current.OrganisationType, current.PrefixSms, current.Crown, current.RateLimit, current.ContactLink, current.ConsentToResearch,
			current.VolumeEmail, current.VolumeLetter, current.VolumeSms, current.CountAsLive, current.GoLiveAt, current.GoLiveUserID,
			current.OrganisationID, current.SendingDomain, current.DefaultBrandingIsFrench, current.SmsDailyLimit, current.OrganisationNotes,
			current.SensitiveService, current.EmailAnnualLimit, current.SmsAnnualLimit, uuid.NullUUID{}, sqlmock.AnyArg(), serviceID,
		).
		WillReturnRows(serviceRows(current, func(s *Service) {
			s.Active = false
			s.Version = 2
			s.UpdatedAt = sql.NullTime{Time: suspendedAt, Valid: true}
			s.SuspendedAt = sql.NullTime{Time: suspendedAt, Valid: true}
			s.SuspendedByID = uuid.NullUUID{}
		}))
	mock.ExpectExec(regexp.QuoteMeta(insertServicesHistoryRow)).
		WithArgs(
			serviceID, current.Name, createdAt, sql.NullTime{Time: suspendedAt, Valid: true}, false, current.MessageLimit, false, current.EmailFrom,
			userID, int32(2), false, current.OrganisationType, sql.NullBool{Bool: true, Valid: true}, current.Crown, current.RateLimit,
			current.ContactLink, current.ConsentToResearch, current.VolumeEmail, current.VolumeLetter, current.VolumeSms, true, current.GoLiveAt,
			current.GoLiveUserID, current.OrganisationID, current.SendingDomain, current.DefaultBrandingIsFrench, current.SmsDailyLimit,
			current.OrganisationNotes, current.SensitiveService, current.EmailAnnualLimit, current.SmsAnnualLimit, uuid.NullUUID{}, sql.NullTime{Time: suspendedAt, Valid: true},
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	updated, err := repo.SuspendService(context.Background(), serviceID, nil)
	if err != nil {
		t.Fatalf("SuspendService() error = %v", err)
	}
	if updated == nil || updated.Active || updated.SuspendedByID.Valid {
		t.Fatalf("updated = %#v", updated)
	}
}

func TestResumeServiceClearsSuspensionAndIsIdempotentWhenActive(t *testing.T) {
	t.Run("resume suspended service", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New() error = %v", err)
		}
		defer db.Close()

		repo := NewRepository(db, db)
		serviceID := uuid.New()
		userID := uuid.New()
		createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
		resumedAt := time.Now().UTC().Truncate(time.Second)
		current := baseServiceRow(serviceID, userID, createdAt)
		current.Active = false
		current.SuspendedByID = uuid.NullUUID{UUID: userID, Valid: true}
		current.SuspendedAt = sql.NullTime{Time: createdAt.Add(30 * time.Minute), Valid: true}

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(getServiceByID)).
			WithArgs(serviceID, false).
			WillReturnRows(serviceRows(current, nil))
		mock.ExpectQuery(regexp.QuoteMeta(updateService)).
			WithArgs(
				current.Name, sqlmock.AnyArg(), true, current.MessageLimit, current.Restricted, current.EmailFrom, int32(2), current.ResearchMode,
				current.OrganisationType, current.PrefixSms, current.Crown, current.RateLimit, current.ContactLink, current.ConsentToResearch,
				current.VolumeEmail, current.VolumeLetter, current.VolumeSms, current.CountAsLive, current.GoLiveAt, current.GoLiveUserID,
				current.OrganisationID, current.SendingDomain, current.DefaultBrandingIsFrench, current.SmsDailyLimit, current.OrganisationNotes,
				current.SensitiveService, current.EmailAnnualLimit, current.SmsAnnualLimit, uuid.NullUUID{}, sql.NullTime{}, serviceID,
			).
			WillReturnRows(serviceRows(current, func(s *Service) {
				s.Active = true
				s.Version = 2
				s.UpdatedAt = sql.NullTime{Time: resumedAt, Valid: true}
				s.SuspendedAt = sql.NullTime{}
				s.SuspendedByID = uuid.NullUUID{}
			}))
		mock.ExpectExec(regexp.QuoteMeta(insertServicesHistoryRow)).
			WithArgs(
				serviceID, current.Name, createdAt, sql.NullTime{Time: resumedAt, Valid: true}, true, current.MessageLimit, false, current.EmailFrom,
				userID, int32(2), false, current.OrganisationType, sql.NullBool{Bool: true, Valid: true}, current.Crown, current.RateLimit,
				current.ContactLink, current.ConsentToResearch, current.VolumeEmail, current.VolumeLetter, current.VolumeSms, true, current.GoLiveAt,
				current.GoLiveUserID, current.OrganisationID, current.SendingDomain, current.DefaultBrandingIsFrench, current.SmsDailyLimit,
				current.OrganisationNotes, current.SensitiveService, current.EmailAnnualLimit, current.SmsAnnualLimit, uuid.NullUUID{}, sql.NullTime{},
			).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		updated, err := repo.ResumeService(context.Background(), serviceID)
		if err != nil {
			t.Fatalf("ResumeService() error = %v", err)
		}
		if updated == nil || !updated.Active || updated.SuspendedAt.Valid || updated.SuspendedByID.Valid {
			t.Fatalf("updated = %#v", updated)
		}
	})

	t.Run("already active is no-op", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New() error = %v", err)
		}
		defer db.Close()

		repo := NewRepository(db, db)
		serviceID := uuid.New()
		userID := uuid.New()
		createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
		current := baseServiceRow(serviceID, userID, createdAt)

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(getServiceByID)).
			WithArgs(serviceID, false).
			WillReturnRows(serviceRows(current, nil))
		mock.ExpectCommit()

		updated, err := repo.ResumeService(context.Background(), serviceID)
		if err != nil {
			t.Fatalf("ResumeService() error = %v", err)
		}
		if updated == nil || !updated.Active || updated.Version != current.Version {
			t.Fatalf("updated = %#v", updated)
		}
	})
}

func TestArchiveServiceArchivesServiceKeysAndTemplates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	userID := uuid.New()
	keyID := uuid.New()
	templateID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	updatedAt := time.Now().UTC().Truncate(time.Second)
	current := baseServiceRow(serviceID, userID, createdAt)
	current.Version = 2
	apiKey := baseAPIKeyRow(keyID, serviceID, userID, createdAt)
	tmpl := baseTemplateRow(templateID, serviceID, userID, createdAt)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(getServiceByID)).
		WithArgs(serviceID, false).
		WillReturnRows(serviceRows(current, nil))
	mock.ExpectQuery(regexp.QuoteMeta(updateService)).
		WithArgs(
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			false,
			current.MessageLimit,
			current.Restricted,
			sqlmock.AnyArg(),
			int32(3),
			current.ResearchMode,
			current.OrganisationType,
			current.PrefixSms,
			current.Crown,
			current.RateLimit,
			current.ContactLink,
			current.ConsentToResearch,
			current.VolumeEmail,
			current.VolumeLetter,
			current.VolumeSms,
			current.CountAsLive,
			current.GoLiveAt,
			current.GoLiveUserID,
			current.OrganisationID,
			current.SendingDomain,
			current.DefaultBrandingIsFrench,
			current.SmsDailyLimit,
			current.OrganisationNotes,
			current.SensitiveService,
			current.EmailAnnualLimit,
			current.SmsAnnualLimit,
			current.SuspendedByID,
			current.SuspendedAt,
			serviceID,
		).
		WillReturnRows(serviceRows(current, func(s *Service) {
			s.Active = false
			s.Name = archivedValue(current.Name, updatedAt.Unix())
			s.EmailFrom = archivedValue(current.EmailFrom, updatedAt.Unix())
			s.Version = 3
			s.UpdatedAt = sql.NullTime{Time: updatedAt, Valid: true}
		}))
	mock.ExpectExec(regexp.QuoteMeta(insertServicesHistoryRow)).
		WithArgs(
			serviceID,
			sqlmock.AnyArg(),
			createdAt,
			sqlmock.AnyArg(),
			false,
			current.MessageLimit,
			false,
			sqlmock.AnyArg(),
			userID,
			int32(3),
			false,
			current.OrganisationType,
			sql.NullBool{Bool: true, Valid: true},
			current.Crown,
			current.RateLimit,
			current.ContactLink,
			current.ConsentToResearch,
			current.VolumeEmail,
			current.VolumeLetter,
			current.VolumeSms,
			true,
			current.GoLiveAt,
			current.GoLiveUserID,
			current.OrganisationID,
			current.SendingDomain,
			current.DefaultBrandingIsFrench,
			current.SmsDailyLimit,
			current.OrganisationNotes,
			current.SensitiveService,
			current.EmailAnnualLimit,
			current.SmsAnnualLimit,
			current.SuspendedByID,
			current.SuspendedAt,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id, name, secret, service_id, expiry_date, created_at, created_by_id, updated_at, version, key_type, compromised_key_info, last_used_timestamp FROM api_keys").
		WithArgs(serviceID).
		WillReturnRows(apiKeyRows(apiKey, nil))
	mock.ExpectQuery("UPDATE api_keys").
		WithArgs(keyID).
		WillReturnRows(apiKeyRows(apiKey, func(item *ApiKey) {
			item.Version = 2
			item.UpdatedAt = sql.NullTime{Time: updatedAt, Valid: true}
			item.ExpiryDate = sql.NullTime{Time: updatedAt, Valid: true}
		}))
	mock.ExpectExec("INSERT INTO api_keys_history").
		WithArgs(
			keyID,
			apiKey.Name,
			apiKey.Secret,
			serviceID,
			sqlmock.AnyArg(),
			createdAt,
			sqlmock.AnyArg(),
			userID,
			int32(2),
			apiKey.KeyType,
			apiKey.CompromisedKeyInfo,
			apiKey.LastUsedTimestamp,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id, name, template_type, created_at, updated_at, content, service_id, subject, created_by_id, version, archived, process_type, service_letter_contact_id, hidden, postage, template_category_id, text_direction_rtl FROM templates").
		WithArgs(serviceID).
		WillReturnRows(templateRows(tmpl, nil))
	mock.ExpectQuery("UPDATE templates").
		WithArgs(templateID).
		WillReturnRows(templateRows(tmpl, func(item *Template) {
			item.Archived = true
			item.UpdatedAt = sql.NullTime{Time: updatedAt, Valid: true}
		}))
	mock.ExpectExec("INSERT INTO templates_history").
		WithArgs(
			templateID,
			tmpl.Name,
			tmpl.TemplateType,
			createdAt,
			sqlmock.AnyArg(),
			tmpl.Content,
			serviceID,
			tmpl.Subject,
			userID,
			int32(1),
			true,
			tmpl.ProcessType,
			tmpl.ServiceLetterContactID,
			tmpl.Hidden,
			tmpl.Postage,
			tmpl.TemplateCategoryID,
			tmpl.TextDirectionRtl,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	archived, err := repo.ArchiveService(context.Background(), serviceID)
	if err != nil {
		t.Fatalf("ArchiveService() error = %v", err)
	}
	if archived == nil || archived.Active || archived.Version != 3 {
		t.Fatalf("archived = %#v", archived)
	}
	if !regexp.MustCompile(`^_archived_\d+_service$`).MatchString(archived.Name) {
		t.Fatalf("archived name = %q", archived.Name)
	}
	if !regexp.MustCompile(`^_archived_\d+_service@example.com$`).MatchString(archived.EmailFrom) {
		t.Fatalf("archived email = %q", archived.EmailFrom)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestArchiveServiceRollsBackOnTemplateArchiveError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	userID := uuid.New()
	keyID := uuid.New()
	templateID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	current := baseServiceRow(serviceID, userID, createdAt)
	apiKey := baseAPIKeyRow(keyID, serviceID, userID, createdAt)
	tmpl := baseTemplateRow(templateID, serviceID, userID, createdAt)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(getServiceByID)).
		WithArgs(serviceID, false).
		WillReturnRows(serviceRows(current, nil))
	mock.ExpectQuery(regexp.QuoteMeta(updateService)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), false, current.MessageLimit, current.Restricted, sqlmock.AnyArg(), int32(2), current.ResearchMode,
			current.OrganisationType, current.PrefixSms, current.Crown, current.RateLimit, current.ContactLink, current.ConsentToResearch,
			current.VolumeEmail, current.VolumeLetter, current.VolumeSms, current.CountAsLive, current.GoLiveAt, current.GoLiveUserID,
			current.OrganisationID, current.SendingDomain, current.DefaultBrandingIsFrench, current.SmsDailyLimit, current.OrganisationNotes,
			current.SensitiveService, current.EmailAnnualLimit, current.SmsAnnualLimit, current.SuspendedByID, current.SuspendedAt, serviceID,
		).
		WillReturnRows(serviceRows(current, func(s *Service) {
			s.Active = false
			s.Name = archivedValue(current.Name, createdAt.Unix())
			s.EmailFrom = archivedValue(current.EmailFrom, createdAt.Unix())
			s.Version = 2
			s.UpdatedAt = sql.NullTime{Time: createdAt, Valid: true}
		}))
	mock.ExpectExec(regexp.QuoteMeta(insertServicesHistoryRow)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id, name, secret, service_id, expiry_date, created_at, created_by_id, updated_at, version, key_type, compromised_key_info, last_used_timestamp FROM api_keys").
		WithArgs(serviceID).
		WillReturnRows(apiKeyRows(apiKey, nil))
	mock.ExpectQuery("UPDATE api_keys").
		WithArgs(keyID).
		WillReturnRows(apiKeyRows(apiKey, func(item *ApiKey) {
			item.Version = 2
			item.UpdatedAt = sql.NullTime{Time: createdAt, Valid: true}
			item.ExpiryDate = sql.NullTime{Time: createdAt, Valid: true}
		}))
	mock.ExpectExec("INSERT INTO api_keys_history").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id, name, template_type, created_at, updated_at, content, service_id, subject, created_by_id, version, archived, process_type, service_letter_contact_id, hidden, postage, template_category_id, text_direction_rtl FROM templates").
		WithArgs(serviceID).
		WillReturnRows(templateRows(tmpl, nil))
	mock.ExpectQuery("UPDATE templates").
		WithArgs(templateID).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	archived, err := repo.ArchiveService(context.Background(), serviceID)
	if err == nil {
		t.Fatal("ArchiveService() error = nil, want error")
	}
	if archived != nil {
		t.Fatalf("archived = %#v, want nil", archived)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func baseServiceRow(serviceID, userID uuid.UUID, createdAt time.Time) Service {
	return Service{
		ID:               serviceID,
		Name:             "service",
		CreatedAt:        createdAt,
		Active:           true,
		MessageLimit:     1000,
		Restricted:       false,
		EmailFrom:        "service@example.com",
		CreatedByID:      userID,
		Version:          1,
		ResearchMode:     false,
		PrefixSms:        true,
		RateLimit:        1000,
		CountAsLive:      true,
		SmsDailyLimit:    1000,
		EmailAnnualLimit: 20000000,
		SmsAnnualLimit:   100000,
	}
}

func baseAPIKeyRow(keyID, serviceID, userID uuid.UUID, createdAt time.Time) ApiKey {
	return ApiKey{
		ID:                 keyID,
		Name:               "test-key",
		Secret:             "hashed-secret",
		ServiceID:          serviceID,
		CreatedAt:          createdAt,
		CreatedByID:        userID,
		Version:            1,
		KeyType:            "normal",
		CompromisedKeyInfo: json.RawMessage(`null`),
	}
}

func baseTemplateRow(templateID, serviceID, userID uuid.UUID, createdAt time.Time) Template {
	return Template{
		ID:               templateID,
		Name:             "template",
		TemplateType:     TemplateTypeEmail,
		CreatedAt:        createdAt,
		Content:          "hello",
		ServiceID:        serviceID,
		CreatedByID:      userID,
		Version:          1,
		Archived:         false,
		Hidden:           false,
		TextDirectionRtl: false,
	}
}

func serviceRows(base Service, mutate func(*Service)) *sqlmock.Rows {
	item := base
	if mutate != nil {
		mutate(&item)
	}
	return sqlmock.NewRows(serviceColumns).AddRow(
		item.ID,
		item.Name,
		item.CreatedAt,
		item.UpdatedAt,
		item.Active,
		item.MessageLimit,
		item.Restricted,
		item.EmailFrom,
		item.CreatedByID,
		item.Version,
		item.ResearchMode,
		item.OrganisationType,
		item.PrefixSms,
		item.Crown,
		item.RateLimit,
		item.ContactLink,
		item.ConsentToResearch,
		item.VolumeEmail,
		item.VolumeLetter,
		item.VolumeSms,
		item.CountAsLive,
		item.GoLiveAt,
		item.GoLiveUserID,
		item.OrganisationID,
		item.SendingDomain,
		item.DefaultBrandingIsFrench,
		item.SmsDailyLimit,
		item.OrganisationNotes,
		item.SensitiveService,
		item.EmailAnnualLimit,
		item.SmsAnnualLimit,
		item.SuspendedByID,
		item.SuspendedAt,
	)
}

func apiKeyRows(base ApiKey, mutate func(*ApiKey)) *sqlmock.Rows {
	item := base
	if mutate != nil {
		mutate(&item)
	}
	return sqlmock.NewRows(apiKeyColumns).AddRow(
		item.ID,
		item.Name,
		item.Secret,
		item.ServiceID,
		item.ExpiryDate,
		item.CreatedAt,
		item.CreatedByID,
		item.UpdatedAt,
		item.Version,
		item.KeyType,
		item.CompromisedKeyInfo,
		item.LastUsedTimestamp,
	)
}

func templateRows(base Template, mutate func(*Template)) *sqlmock.Rows {
	item := base
	if mutate != nil {
		mutate(&item)
	}
	return sqlmock.NewRows(templateColumns).AddRow(
		item.ID,
		item.Name,
		item.TemplateType,
		item.CreatedAt,
		item.UpdatedAt,
		item.Content,
		item.ServiceID,
		item.Subject,
		item.CreatedByID,
		item.Version,
		item.Archived,
		item.ProcessType,
		item.ServiceLetterContactID,
		item.Hidden,
		item.Postage,
		item.TemplateCategoryID,
		item.TextDirectionRtl,
	)
}

var serviceSmsSenderColumns = []string{"id", "sms_sender", "service_id", "is_default", "inbound_number_id", "created_at", "updated_at", "archived"}

var serviceEmailReplyToColumns = []string{"id", "service_id", "email_address", "is_default", "created_at", "updated_at", "archived"}

var serviceLetterContactColumns = []string{"id", "service_id", "contact_block", "is_default", "created_at", "updated_at", "archived"}

var serviceCallbackAPIColumns = []string{"id", "service_id", "url", "bearer_token", "created_at", "updated_at", "updated_by_id", "version", "callback_type", "is_suspended", "suspended_at"}

var serviceInboundAPIColumns = []string{"id", "service_id", "url", "bearer_token", "created_at", "updated_at", "updated_by_id", "version"}

var serviceDataRetentionColumns = []string{"id", "service_id", "notification_type", "days_of_retention", "created_at", "updated_at"}

var serviceSafelistColumns = []string{"id", "service_id", "recipient_type", "recipient", "created_at"}

func TestFetchServiceDataRetentionQueriesByIDAndType(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	retentionID := uuid.New()
	createdAt := time.Now().UTC().Truncate(time.Second)
	retention := baseServiceDataRetentionRow(retentionID, serviceID, NotificationTypeEmail, createdAt)

	mock.ExpectQuery(regexp.QuoteMeta(getDataRetention)).
		WithArgs(serviceID).
		WillReturnRows(serviceDataRetentionRows(retention, nil))
	mock.ExpectQuery(regexp.QuoteMeta(getDataRetentionByIDQuery)).
		WithArgs(serviceID, retentionID).
		WillReturnRows(serviceDataRetentionRows(retention, nil))
	mock.ExpectQuery(regexp.QuoteMeta(getDataRetentionByNotificationTypeQuery)).
		WithArgs(serviceID, NotificationTypeEmail).
		WillReturnRows(serviceDataRetentionRows(retention, nil))

	items, err := repo.FetchServiceDataRetention(context.Background(), serviceID)
	if err != nil {
		t.Fatalf("FetchServiceDataRetention() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != retentionID {
		t.Fatalf("items = %#v", items)
	}
	byID, err := repo.FetchServiceDataRetentionByID(context.Background(), serviceID, retentionID)
	if err != nil {
		t.Fatalf("FetchServiceDataRetentionByID() error = %v", err)
	}
	if byID == nil || byID.ID != retentionID {
		t.Fatalf("byID = %#v", byID)
	}
	byType, err := repo.FetchDataRetentionByNotificationType(context.Background(), serviceID, NotificationTypeEmail)
	if err != nil {
		t.Fatalf("FetchDataRetentionByNotificationType() error = %v", err)
	}
	if byType == nil || byType.NotificationType != NotificationTypeEmail {
		t.Fatalf("byType = %#v", byType)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestFetchServiceDataRetentionByIDReturnsNilOnMiss(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	retentionID := uuid.New()

	mock.ExpectQuery(regexp.QuoteMeta(getDataRetentionByIDQuery)).
		WithArgs(serviceID, retentionID).
		WillReturnError(sql.ErrNoRows)

	item, err := repo.FetchServiceDataRetentionByID(context.Background(), serviceID, retentionID)
	if err != nil {
		t.Fatalf("FetchServiceDataRetentionByID() error = %v", err)
	}
	if item != nil {
		t.Fatalf("item = %#v, want nil", item)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestInsertServiceDataRetentionReturnsDuplicateAsInvalidRequest(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	retention := baseServiceDataRetentionRow(uuid.New(), serviceID, NotificationTypeSms, time.Now().UTC().Truncate(time.Second))

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(createDataRetentionQuery)).
		WithArgs(retention.ID, serviceID, NotificationTypeSms, retention.DaysOfRetention, retention.CreatedAt, sql.NullTime{}).
		WillReturnError(&pq.Error{Code: "23505"})
	mock.ExpectRollback()

	_, err = repo.InsertServiceDataRetention(context.Background(), retention)
	if err == nil {
		t.Fatal("InsertServiceDataRetention() error = nil, want error")
	}
	invalidErr, ok := err.(serviceerrs.InvalidRequestError)
	if !ok {
		t.Fatalf("err = %T, want InvalidRequestError", err)
	}
	if invalidErr.Message != "Service already has data retention for sms notification type" {
		t.Fatalf("message = %q", invalidErr.Message)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestUpdateServiceDataRetentionReturnsAffectedRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	retentionID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(updateDataRetentionByIDQuery)).
		WithArgs(int32(14), serviceID, retentionID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	count, err := repo.UpdateServiceDataRetention(context.Background(), serviceID, retentionID, 14)
	if err != nil {
		t.Fatalf("UpdateServiceDataRetention() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestUpdateServiceDataRetentionReturnsZeroForUnknownRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	retentionID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(updateDataRetentionByIDQuery)).
		WithArgs(int32(30), serviceID, retentionID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	count, err := repo.UpdateServiceDataRetention(context.Background(), serviceID, retentionID, 30)
	if err != nil {
		t.Fatalf("UpdateServiceDataRetention() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestFetchAndReplaceServiceSafelist(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	createdAt := time.Now().UTC().Truncate(time.Second)
	emailEntry := baseServiceSafelistRow(uuid.New(), serviceID, RecipientTypeEmail, "person@example.com", createdAt)
	phoneEntry := baseServiceSafelistRow(uuid.New(), serviceID, RecipientTypeMobile, "+16135550100", createdAt)

	mock.ExpectQuery(regexp.QuoteMeta(getSafelist)).
		WithArgs(serviceID).
		WillReturnRows(serviceSafelistRows(emailEntry, nil).AddRow(phoneEntry.ID, phoneEntry.ServiceID, phoneEntry.RecipientType, phoneEntry.Recipient, phoneEntry.CreatedAt))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(updateSafelist)).
		WithArgs(serviceID, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	items, err := repo.FetchServiceSafelist(context.Background(), serviceID)
	if err != nil {
		t.Fatalf("FetchServiceSafelist() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if err := repo.AddSafelistedContacts(context.Background(), serviceID, []string{"person@example.com"}, []string{"+16135550100"}); err != nil {
		t.Fatalf("AddSafelistedContacts() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestAddSafelistedContactsRollsBackOnFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(updateSafelist)).
		WithArgs(serviceID, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	err = repo.AddSafelistedContacts(context.Background(), serviceID, []string{"person@example.com"}, []string{"+16135550100"})
	if err == nil {
		t.Fatal("AddSafelistedContacts() error = nil, want error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRemoveServiceSafelistDeletesAllRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(deleteSafelistQuery)).
		WithArgs(serviceID).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	if err := repo.RemoveServiceSafelist(context.Background(), serviceID); err != nil {
		t.Fatalf("RemoveServiceSafelist() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestAddSmsSenderForServiceClearsExistingDefaultAndBindsInboundNumber(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	inboundID := uuid.New()
	existingID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	updatedAt := time.Now().UTC().Truncate(time.Second)
	existing := baseServiceSmsSenderRow(existingID, serviceID, createdAt)
	existing.IsDefault = true
	newSender := baseServiceSmsSenderRow(uuid.New(), serviceID, updatedAt)
	newSender.SmsSender = "Notify"
	newSender.IsDefault = true
	newSender.InboundNumberID = uuid.NullUUID{UUID: inboundID, Valid: true}
	newSender.CreatedAt = updatedAt

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(getSMSSenders)).
		WithArgs(serviceID).
		WillReturnRows(serviceSmsSenderRows(existing, nil))
	mock.ExpectQuery(regexp.QuoteMeta(updateSMSSender)).
		WithArgs(existing.SmsSender, false, existing.InboundNumberID, existing.Archived, existing.ID).
		WillReturnRows(serviceSmsSenderRows(existing, func(item *ServiceSmsSender) {
			item.IsDefault = false
			item.UpdatedAt = sql.NullTime{Time: updatedAt, Valid: true}
		}))
	mock.ExpectQuery(regexp.QuoteMeta(createSMSSender)).
		WithArgs(newSender.ID, newSender.SmsSender, serviceID, true, newSender.InboundNumberID, newSender.CreatedAt, sql.NullTime{}, false).
		WillReturnRows(serviceSmsSenderRows(newSender, nil))
	mock.ExpectQuery(regexp.QuoteMeta("UPDATE inbound_numbers\nSET service_id = $1,\n    active = true,\n    updated_at = now()\nWHERE id = $2\nRETURNING id, number, provider, service_id, active, created_at, updated_at\n")).
		WithArgs(uuid.NullUUID{UUID: serviceID, Valid: true}, inboundID).
		WillReturnRows(inboundNumberRows(inboundID, serviceID, updatedAt))
	mock.ExpectCommit()

	created, err := repo.AddSmsSenderForService(context.Background(), newSender)
	if err != nil {
		t.Fatalf("AddSmsSenderForService() error = %v", err)
	}
	if created == nil || !created.IsDefault || !created.InboundNumberID.Valid {
		t.Fatalf("created = %#v", created)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestUpdateServiceSmsSenderRejectsInboundNumberSenderValueChange(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	inboundID := uuid.New()
	senderID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	current := baseServiceSmsSenderRow(senderID, serviceID, createdAt)
	current.InboundNumberID = uuid.NullUUID{UUID: inboundID, Valid: true}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(getSMSSenders)).
		WithArgs(serviceID).
		WillReturnRows(serviceSmsSenderRows(current, nil))
	mock.ExpectRollback()

	_, err = repo.UpdateServiceSmsSender(context.Background(), ServiceSmsSender{
		ID:              senderID,
		ServiceID:       serviceID,
		SmsSender:       "Changed",
		IsDefault:       true,
		InboundNumberID: current.InboundNumberID,
	})
	if err == nil || err.Error() != errInboundSenderImmutable {
		t.Fatalf("err = %v, want %q", err, errInboundSenderImmutable)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestAddReplyToEmailAddressClearsExistingDefault(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	existingID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	updatedAt := time.Now().UTC().Truncate(time.Second)
	existing := baseServiceReplyToRow(existingID, serviceID, createdAt)
	existing.IsDefault = true
	newReplyTo := baseServiceReplyToRow(uuid.New(), serviceID, updatedAt)
	newReplyTo.EmailAddress = "new-reply@example.com"
	newReplyTo.IsDefault = true

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(getEmailReplyTo)).
		WithArgs(serviceID).
		WillReturnRows(serviceEmailReplyToRows(existing, nil))
	mock.ExpectQuery(regexp.QuoteMeta(updateEmailReplyTo)).
		WithArgs(existing.EmailAddress, false, existing.Archived, existing.ID).
		WillReturnRows(serviceEmailReplyToRows(existing, func(item *ServiceEmailReplyTo) {
			item.IsDefault = false
			item.UpdatedAt = sql.NullTime{Time: updatedAt, Valid: true}
		}))
	mock.ExpectQuery(regexp.QuoteMeta(createEmailReplyTo)).
		WithArgs(newReplyTo.ID, serviceID, newReplyTo.EmailAddress, true, newReplyTo.CreatedAt, sql.NullTime{}, false).
		WillReturnRows(serviceEmailReplyToRows(newReplyTo, nil))
	mock.ExpectCommit()

	created, err := repo.AddReplyToEmailAddress(context.Background(), newReplyTo)
	if err != nil {
		t.Fatalf("AddReplyToEmailAddress() error = %v", err)
	}
	if created == nil || !created.IsDefault {
		t.Fatalf("created = %#v", created)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestArchiveReplyToEmailAddressRejectsDefaultWhenOthersExist(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	defaultID := uuid.New()
	otherID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	defaultReplyTo := baseServiceReplyToRow(defaultID, serviceID, createdAt)
	defaultReplyTo.IsDefault = true
	otherReplyTo := baseServiceReplyToRow(otherID, serviceID, createdAt)
	otherReplyTo.EmailAddress = "other@example.com"

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(getEmailReplyTo)).
		WithArgs(serviceID).
		WillReturnRows(serviceEmailReplyToRows(defaultReplyTo, nil).AddRow(
			otherReplyTo.ID,
			otherReplyTo.ServiceID,
			otherReplyTo.EmailAddress,
			otherReplyTo.IsDefault,
			otherReplyTo.CreatedAt,
			otherReplyTo.UpdatedAt,
			otherReplyTo.Archived,
		))
	mock.ExpectRollback()

	_, err = repo.ArchiveReplyToEmailAddress(context.Background(), serviceID, defaultID)
	if err == nil || err.Error() != errDefaultReplyToArchive {
		t.Fatalf("err = %v, want %q", err, errDefaultReplyToArchive)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func baseServiceSmsSenderRow(senderID, serviceID uuid.UUID, createdAt time.Time) ServiceSmsSender {
	return ServiceSmsSender{
		ID:        senderID,
		SmsSender: "+16135550000",
		ServiceID: serviceID,
		CreatedAt: createdAt,
	}
}

func baseServiceReplyToRow(replyToID, serviceID uuid.UUID, createdAt time.Time) ServiceEmailReplyTo {
	return ServiceEmailReplyTo{
		ID:           replyToID,
		ServiceID:    serviceID,
		EmailAddress: "reply@example.com",
		CreatedAt:    createdAt,
	}
}

func baseUserRow(userID uuid.UUID, createdAt time.Time) usersRepo.User {
	return usersRepo.User{
		ID:                    userID,
		Name:                  "Test User",
		EmailAddress:          "user@example.com",
		CreatedAt:             createdAt,
		Password:              "hashed-password",
		PasswordChangedAt:     createdAt,
		FailedLoginCount:      0,
		State:                 "active",
		PlatformAdmin:         false,
		AuthType:              "sms_auth",
		Blocked:               false,
		AdditionalInformation: json.RawMessage(`{}`),
		PasswordExpired:       false,
		DefaultEditorIsRte:    false,
	}
}

func baseServiceDataRetentionRow(retentionID, serviceID uuid.UUID, notificationType NotificationType, createdAt time.Time) ServiceDataRetention {
	return ServiceDataRetention{
		ID:               retentionID,
		ServiceID:        serviceID,
		NotificationType: notificationType,
		DaysOfRetention:  7,
		CreatedAt:        createdAt,
	}
}

func baseServiceSafelistRow(id, serviceID uuid.UUID, recipientType RecipientType, recipient string, createdAt time.Time) ServiceSafelist {
	return ServiceSafelist{
		ID:            id,
		ServiceID:     serviceID,
		RecipientType: recipientType,
		Recipient:     recipient,
		CreatedAt:     sql.NullTime{Time: createdAt, Valid: true},
	}
}

func serviceSmsSenderRows(base ServiceSmsSender, mutate func(*ServiceSmsSender)) *sqlmock.Rows {
	item := base
	if mutate != nil {
		mutate(&item)
	}
	return sqlmock.NewRows(serviceSmsSenderColumns).AddRow(
		item.ID,
		item.SmsSender,
		item.ServiceID,
		item.IsDefault,
		item.InboundNumberID,
		item.CreatedAt,
		item.UpdatedAt,
		item.Archived,
	)
}

func serviceEmailReplyToRows(base ServiceEmailReplyTo, mutate func(*ServiceEmailReplyTo)) *sqlmock.Rows {
	item := base
	if mutate != nil {
		mutate(&item)
	}
	return sqlmock.NewRows(serviceEmailReplyToColumns).AddRow(
		item.ID,
		item.ServiceID,
		item.EmailAddress,
		item.IsDefault,
		item.CreatedAt,
		item.UpdatedAt,
		item.Archived,
	)
}

func userRows(base usersRepo.User, mutate func(*usersRepo.User)) *sqlmock.Rows {
	item := base
	if mutate != nil {
		mutate(&item)
	}
	return sqlmock.NewRows(userColumns).AddRow(
		item.ID,
		item.Name,
		item.EmailAddress,
		item.CreatedAt,
		item.UpdatedAt,
		item.Password,
		item.MobileNumber,
		item.PasswordChangedAt,
		item.LoggedInAt,
		item.FailedLoginCount,
		item.State,
		item.PlatformAdmin,
		item.CurrentSessionID,
		item.AuthType,
		item.Blocked,
		item.AdditionalInformation,
		item.PasswordExpired,
		item.VerifiedPhonenumber,
		item.DefaultEditorIsRte,
	)
}

type permissionItemsArg struct {
	want []string
}

func (a permissionItemsArg) Match(value driver.Value) bool {
	raw, ok := valueAsBytes(value)
	if !ok {
		return false
	}
	var items []struct {
		Permission string `json:"permission"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return false
	}
	got := make([]string, 0, len(items))
	for _, item := range items {
		got = append(got, item.Permission)
	}
	return strings.Join(got, ",") == strings.Join(a.want, ",")
}

type uuidArrayArg struct {
	want []uuid.UUID
}

func (a uuidArrayArg) Match(value driver.Value) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	got, err := parsePostgresUUIDArray(text)
	if err != nil {
		return false
	}
	if len(got) != len(a.want) {
		return false
	}
	for index := range got {
		if got[index] != a.want[index] {
			return false
		}
	}
	return true
}

func valueAsBytes(value driver.Value) ([]byte, bool) {
	switch typed := value.(type) {
	case []byte:
		return typed, true
	case string:
		return []byte(typed), true
	default:
		return nil, false
	}
}

func parsePostgresUUIDArray(value string) ([]uuid.UUID, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "{}" {
		return []uuid.UUID{}, nil
	}
	trimmed = strings.TrimPrefix(trimmed, "{")
	trimmed = strings.TrimSuffix(trimmed, "}")
	if trimmed == "" {
		return []uuid.UUID{}, nil
	}
	parts := strings.Split(trimmed, ",")
	items := make([]uuid.UUID, 0, len(parts))
	for _, part := range parts {
		folderID, err := uuid.Parse(strings.Trim(part, `"`))
		if err != nil {
			return nil, err
		}
		items = append(items, folderID)
	}
	return items, nil
}

func serviceDataRetentionRows(base ServiceDataRetention, mutate func(*ServiceDataRetention)) *sqlmock.Rows {
	item := base
	if mutate != nil {
		mutate(&item)
	}
	return sqlmock.NewRows(serviceDataRetentionColumns).AddRow(
		item.ID,
		item.ServiceID,
		item.NotificationType,
		item.DaysOfRetention,
		item.CreatedAt,
		item.UpdatedAt,
	)
}

func serviceSafelistRows(base ServiceSafelist, mutate func(*ServiceSafelist)) *sqlmock.Rows {
	item := base
	if mutate != nil {
		mutate(&item)
	}
	return sqlmock.NewRows(serviceSafelistColumns).AddRow(
		item.ID,
		item.ServiceID,
		item.RecipientType,
		item.Recipient,
		item.CreatedAt,
	)
}

func inboundNumberRows(inboundID, serviceID uuid.UUID, updatedAt time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "number", "provider", "service_id", "active", "created_at", "updated_at"}).AddRow(
		inboundID,
		"+447700900000",
		"firetext",
		uuid.NullUUID{UUID: serviceID, Valid: true},
		true,
		updatedAt.Add(-time.Hour),
		sql.NullTime{Time: updatedAt, Valid: true},
	)
}

func TestGetLetterContactByIDAndListExcludeArchivedAndKeepDefaultFirst(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	defaultID := uuid.New()
	otherID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	defaultContact := baseServiceLetterContactRow(defaultID, serviceID, createdAt)
	defaultContact.IsDefault = true
	otherContact := baseServiceLetterContactRow(otherID, serviceID, createdAt.Add(10*time.Minute))
	otherContact.ContactBlock = "Other"

	mock.ExpectQuery(regexp.QuoteMeta(getLetterContactByIDQuery)).
		WithArgs(serviceID, defaultID).
		WillReturnRows(serviceLetterContactRows(defaultContact, nil))

	got, err := repo.GetLetterContactByID(context.Background(), serviceID, defaultID)
	if err != nil {
		t.Fatalf("GetLetterContactByID() error = %v", err)
	}
	if got == nil || got.ID != defaultID {
		t.Fatalf("got = %#v", got)
	}

	mock.ExpectQuery(regexp.QuoteMeta(listLetterContactsByServiceQuery)).
		WithArgs(serviceID).
		WillReturnRows(serviceLetterContactRows(defaultContact, nil).AddRow(
			otherContact.ID,
			otherContact.ServiceID,
			otherContact.ContactBlock,
			otherContact.IsDefault,
			otherContact.CreatedAt,
			otherContact.UpdatedAt,
			otherContact.Archived,
		))

	items, err := repo.GetLetterContactsByServiceID(context.Background(), serviceID)
	if err != nil {
		t.Fatalf("GetLetterContactsByServiceID() error = %v", err)
	}
	if len(items) != 2 || !items[0].IsDefault {
		t.Fatalf("items = %#v", items)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestAddLetterContactForServiceClearsExistingDefault(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	existingID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	updatedAt := time.Now().UTC().Truncate(time.Second)
	existing := baseServiceLetterContactRow(existingID, serviceID, createdAt)
	existing.IsDefault = true
	contact := baseServiceLetterContactRow(uuid.New(), serviceID, updatedAt)
	contact.ContactBlock = "New Contact"
	contact.IsDefault = true

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(listLetterContactsByServiceQuery)).
		WithArgs(serviceID).
		WillReturnRows(serviceLetterContactRows(existing, nil))
	mock.ExpectQuery(regexp.QuoteMeta(updateLetterContactQuery)).
		WithArgs(existing.ContactBlock, false, existing.Archived, existing.ID, existing.ServiceID).
		WillReturnRows(serviceLetterContactRows(existing, func(item *ServiceLetterContact) {
			item.IsDefault = false
			item.UpdatedAt = sql.NullTime{Time: updatedAt, Valid: true}
		}))
	mock.ExpectQuery(regexp.QuoteMeta(createLetterContactQuery)).
		WithArgs(contact.ID, contact.ServiceID, contact.ContactBlock, true, contact.CreatedAt, sql.NullTime{}, false).
		WillReturnRows(serviceLetterContactRows(contact, nil))
	mock.ExpectCommit()

	created, err := repo.AddLetterContactForService(context.Background(), contact)
	if err != nil {
		t.Fatalf("AddLetterContactForService() error = %v", err)
	}
	if created == nil || !created.IsDefault {
		t.Fatalf("created = %#v", created)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestUpdateLetterContactPromotesNewDefault(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	defaultID := uuid.New()
	otherID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	updatedAt := time.Now().UTC().Truncate(time.Second)
	defaultContact := baseServiceLetterContactRow(defaultID, serviceID, createdAt)
	defaultContact.IsDefault = true
	otherContact := baseServiceLetterContactRow(otherID, serviceID, createdAt.Add(time.Minute))

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(listLetterContactsByServiceQuery)).
		WithArgs(serviceID).
		WillReturnRows(serviceLetterContactRows(defaultContact, nil).AddRow(
			otherContact.ID,
			otherContact.ServiceID,
			otherContact.ContactBlock,
			otherContact.IsDefault,
			otherContact.CreatedAt,
			otherContact.UpdatedAt,
			otherContact.Archived,
		))
	mock.ExpectQuery(regexp.QuoteMeta(updateLetterContactQuery)).
		WithArgs(defaultContact.ContactBlock, false, defaultContact.Archived, defaultContact.ID, defaultContact.ServiceID).
		WillReturnRows(serviceLetterContactRows(defaultContact, func(item *ServiceLetterContact) {
			item.IsDefault = false
			item.UpdatedAt = sql.NullTime{Time: updatedAt, Valid: true}
		}))
	mock.ExpectQuery(regexp.QuoteMeta(updateLetterContactQuery)).
		WithArgs("Updated Block", true, otherContact.Archived, otherContact.ID, otherContact.ServiceID).
		WillReturnRows(serviceLetterContactRows(otherContact, func(item *ServiceLetterContact) {
			item.ContactBlock = "Updated Block"
			item.IsDefault = true
			item.UpdatedAt = sql.NullTime{Time: updatedAt, Valid: true}
		}))
	mock.ExpectCommit()

	updated, err := repo.UpdateLetterContact(context.Background(), ServiceLetterContact{ID: otherID, ServiceID: serviceID, ContactBlock: "Updated Block", IsDefault: true})
	if err != nil {
		t.Fatalf("UpdateLetterContact() error = %v", err)
	}
	if updated == nil || !updated.IsDefault || updated.ContactBlock != "Updated Block" {
		t.Fatalf("updated = %#v", updated)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestArchiveLetterContactClearsTemplateReference(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db)
	serviceID := uuid.New()
	contactID := uuid.New()
	templateID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	updatedAt := time.Now().UTC().Truncate(time.Second)
	contact := baseServiceLetterContactRow(contactID, serviceID, createdAt)
	contact.IsDefault = true
	tmpl := baseTemplateRow(templateID, serviceID, uuid.New(), createdAt)
	tmpl.ServiceLetterContactID = uuid.NullUUID{UUID: contactID, Valid: true}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(listLetterContactsByServiceQuery)).
		WithArgs(serviceID).
		WillReturnRows(serviceLetterContactRows(contact, nil))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, template_type, created_at, updated_at, content, service_id, subject, created_by_id, version, archived, process_type, service_letter_contact_id, hidden, postage, template_category_id, text_direction_rtl\nFROM templates\nWHERE service_id = $1\n    AND archived = false\nORDER BY created_at DESC\n")).
		WithArgs(serviceID).
		WillReturnRows(templateRows(tmpl, nil))
	mock.ExpectQuery(regexp.QuoteMeta("UPDATE templates\nSET name = $1,\n    updated_at = $2,\n    content = $3,\n    subject = $4,\n    version = $5,\n    archived = $6,\n    process_type = $7,\n    service_letter_contact_id = $8,\n    hidden = $9,\n    postage = $10,\n    template_category_id = $11,\n    text_direction_rtl = $12\nWHERE id = $13\nRETURNING id, name, template_type, created_at, updated_at, content, service_id, subject, created_by_id, version, archived, process_type, service_letter_contact_id, hidden, postage, template_category_id, text_direction_rtl\n")).
		WithArgs(
			tmpl.Name,
			sqlmock.AnyArg(),
			tmpl.Content,
			tmpl.Subject,
			tmpl.Version,
			tmpl.Archived,
			tmpl.ProcessType,
			uuid.NullUUID{},
			tmpl.Hidden,
			tmpl.Postage,
			tmpl.TemplateCategoryID,
			tmpl.TextDirectionRtl,
			tmpl.ID,
		).
		WillReturnRows(templateRows(tmpl, func(item *Template) {
			item.ServiceLetterContactID = uuid.NullUUID{}
			item.UpdatedAt = sql.NullTime{Time: updatedAt, Valid: true}
		}))
	mock.ExpectQuery(regexp.QuoteMeta(updateLetterContactQuery)).
		WithArgs(contact.ContactBlock, contact.IsDefault, true, contact.ID, contact.ServiceID).
		WillReturnRows(serviceLetterContactRows(contact, func(item *ServiceLetterContact) {
			item.Archived = true
			item.UpdatedAt = sql.NullTime{Time: updatedAt, Valid: true}
		}))
	mock.ExpectCommit()

	archived, err := repo.ArchiveLetterContact(context.Background(), serviceID, contactID)
	if err != nil {
		t.Fatalf("ArchiveLetterContact() error = %v", err)
	}
	if archived == nil || !archived.Archived {
		t.Fatalf("archived = %#v", archived)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func baseServiceLetterContactRow(contactID, serviceID uuid.UUID, createdAt time.Time) ServiceLetterContact {
	return ServiceLetterContact{
		ID:           contactID,
		ServiceID:    serviceID,
		ContactBlock: "Example Contact",
		CreatedAt:    createdAt,
	}
}

func serviceLetterContactRows(base ServiceLetterContact, mutate func(*ServiceLetterContact)) *sqlmock.Rows {
	item := base
	if mutate != nil {
		mutate(&item)
	}
	return sqlmock.NewRows(serviceLetterContactColumns).AddRow(
		item.ID,
		item.ServiceID,
		item.ContactBlock,
		item.IsDefault,
		item.CreatedAt,
		item.UpdatedAt,
		item.Archived,
	)
}

func TestSaveServiceCallbackApiStoresSignedTokenAndWritesHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db, func(r *Repository) {
		r.apiKeySecrets = []string{"current-secret"}
		r.dangerousSalt = "dangerous-salt"
	})
	serviceID := uuid.New()
	updatedByID := uuid.New()
	createdAt := time.Now().UTC().Truncate(time.Second)
	signed, err := repo.signBearerToken("plaintext-token")
	if err != nil {
		t.Fatalf("signBearerToken() error = %v", err)
	}
	stored := baseServiceCallbackAPIRow(uuid.New(), serviceID, updatedByID, createdAt)
	stored.BearerToken = signed
	stored.CallbackType = sql.NullString{String: callbackTypeDeliveryStatus, Valid: true}
	stored.IsSuspended = sql.NullBool{Bool: false, Valid: true}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(upsertCallbackAPI)).
		WithArgs(sqlmock.AnyArg(), serviceID, "https://callback.example.com", sqlmock.AnyArg(), sqlmock.AnyArg(), sql.NullTime{}, updatedByID, int32(1), sql.NullString{String: callbackTypeDeliveryStatus, Valid: true}, sql.NullBool{Bool: false, Valid: true}, sql.NullTime{}).
		WillReturnRows(serviceCallbackAPIRows(stored, nil))
	mock.ExpectExec(regexp.QuoteMeta(insertCallbackAPIHistoryQuery)).
		WithArgs(stored.ID, stored.ServiceID, stored.Url, stored.BearerToken, stored.CreatedAt, stored.UpdatedAt, stored.UpdatedByID, stored.Version, stored.CallbackType, stored.IsSuspended, stored.SuspendedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	created, err := repo.SaveServiceCallbackApi(context.Background(), serviceID, callbackTypeDeliveryStatus, "https://callback.example.com", "plaintext-token", updatedByID)
	if err != nil {
		t.Fatalf("SaveServiceCallbackApi() error = %v", err)
	}
	if created == nil || created.BearerToken != "plaintext-token" {
		t.Fatalf("created = %#v", created)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestResetServiceCallbackApiAllowsExplicitEmptyURL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db, func(r *Repository) {
		r.apiKeySecrets = []string{"current-secret"}
		r.dangerousSalt = "dangerous-salt"
	})
	serviceID := uuid.New()
	updatedByID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	updatedAt := time.Now().UTC().Truncate(time.Second)
	signed, _ := repo.signBearerToken("plaintext-token")
	stored := baseServiceCallbackAPIRow(uuid.New(), serviceID, updatedByID, createdAt)
	stored.BearerToken = signed
	stored.CallbackType = sql.NullString{String: callbackTypeDeliveryStatus, Valid: true}
	stored.IsSuspended = sql.NullBool{Bool: false, Valid: true}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(getCallbackAPIs)).
		WithArgs(serviceID, sql.NullString{}).
		WillReturnRows(serviceCallbackAPIRows(stored, nil))
	mock.ExpectQuery(regexp.QuoteMeta(upsertCallbackAPI)).
		WithArgs(stored.ID, serviceID, "", signed, stored.CreatedAt, sqlmock.AnyArg(), updatedByID, int32(2), stored.CallbackType, stored.IsSuspended, stored.SuspendedAt).
		WillReturnRows(serviceCallbackAPIRows(stored, func(item *ServiceCallbackApi) {
			item.Url = ""
			item.Version = 2
			item.UpdatedAt = sql.NullTime{Time: updatedAt, Valid: true}
		}))
	mock.ExpectExec(regexp.QuoteMeta(insertCallbackAPIHistoryQuery)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	empty := ""
	updated, err := repo.ResetServiceCallbackApi(context.Background(), serviceID, stored.ID, updatedByID, &empty, nil)
	if err != nil {
		t.Fatalf("ResetServiceCallbackApi() error = %v", err)
	}
	if updated == nil || updated.Url != "" || updated.Version != 2 {
		t.Fatalf("updated = %#v", updated)
	}
}

func TestSuspendUnsuspendCallbackApiWritesHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db, func(r *Repository) {
		r.apiKeySecrets = []string{"current-secret"}
		r.dangerousSalt = "dangerous-salt"
	})
	serviceID := uuid.New()
	updatedByID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	updatedAt := time.Now().UTC().Truncate(time.Second)
	signed, _ := repo.signBearerToken("plaintext-token")
	stored := baseServiceCallbackAPIRow(uuid.New(), serviceID, updatedByID, createdAt)
	stored.BearerToken = signed
	stored.CallbackType = sql.NullString{String: callbackTypeDeliveryStatus, Valid: true}
	stored.IsSuspended = sql.NullBool{Bool: false, Valid: true}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(getCallbackAPIs)).
		WithArgs(serviceID, sql.NullString{String: callbackTypeDeliveryStatus, Valid: true}).
		WillReturnRows(serviceCallbackAPIRows(stored, nil))
	mock.ExpectQuery(regexp.QuoteMeta(upsertCallbackAPI)).
		WithArgs(stored.ID, serviceID, stored.Url, signed, stored.CreatedAt, sqlmock.AnyArg(), updatedByID, int32(2), stored.CallbackType, sql.NullBool{Bool: true, Valid: true}, sqlmock.AnyArg()).
		WillReturnRows(serviceCallbackAPIRows(stored, func(item *ServiceCallbackApi) {
			item.Version = 2
			item.IsSuspended = sql.NullBool{Bool: true, Valid: true}
			item.UpdatedAt = sql.NullTime{Time: updatedAt, Valid: true}
			item.SuspendedAt = sql.NullTime{Time: updatedAt, Valid: true}
		}))
	mock.ExpectExec(regexp.QuoteMeta(insertCallbackAPIHistoryQuery)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	updated, err := repo.SuspendUnsuspendCallbackApi(context.Background(), serviceID, updatedByID, true)
	if err != nil {
		t.Fatalf("SuspendUnsuspendCallbackApi() error = %v", err)
	}
	if updated == nil || !updated.IsSuspended.Valid || !updated.IsSuspended.Bool {
		t.Fatalf("updated = %#v", updated)
	}
}

func TestResignServiceCallbacksDryRunDoesNotWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db, func(r *Repository) {
		r.apiKeySecrets = []string{"current-secret"}
		r.dangerousSalt = "dangerous-salt"
	})
	serviceID := uuid.New()
	updatedByID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	stored := baseServiceCallbackAPIRow(uuid.New(), serviceID, updatedByID, createdAt)
	stored.BearerToken, err = repo.signBearerToken("plaintext-token")
	if err != nil {
		t.Fatalf("signBearerToken() error = %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(listSignedCallbackAPIsForResignQuery)).
		WillReturnRows(serviceCallbackAPIRows(stored, nil))
	mock.ExpectRollback()

	count, err := repo.ResignServiceCallbacks(context.Background(), false, false)
	if err != nil {
		t.Fatalf("ResignServiceCallbacks() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestResignServiceCallbacksUpdatesSignatures(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db, func(r *Repository) {
		r.apiKeySecrets = []string{"current-secret", "old-secret"}
		r.dangerousSalt = "dangerous-salt"
	})
	serviceID := uuid.New()
	updatedByID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	stored := baseServiceCallbackAPIRow(uuid.New(), serviceID, updatedByID, createdAt)
	stored.BearerToken, err = signing.Sign("plaintext-token", "old-secret", repo.dangerousSalt)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(listSignedCallbackAPIsForResignQuery)).
		WillReturnRows(serviceCallbackAPIRows(stored, nil))
	mock.ExpectExec(regexp.QuoteMeta(updateCallbackAPIBearerTokenQuery)).
		WithArgs(signedCallbackTokenArg{oldToken: stored.BearerToken, plaintext: "plaintext-token", secrets: []string{"current-secret"}, salt: repo.dangerousSalt}, stored.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	count, err := repo.ResignServiceCallbacks(context.Background(), true, false)
	if err != nil {
		t.Fatalf("ResignServiceCallbacks() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestResignServiceCallbacksUnsafeAllowsUnknownSignatures(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db, func(r *Repository) {
		r.apiKeySecrets = []string{"current-secret"}
		r.dangerousSalt = "dangerous-salt"
	})
	serviceID := uuid.New()
	updatedByID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	stored := baseServiceCallbackAPIRow(uuid.New(), serviceID, updatedByID, createdAt)
	stored.BearerToken, err = signing.Sign("plaintext-token", "old-secret", repo.dangerousSalt)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(listSignedCallbackAPIsForResignQuery)).
		WillReturnRows(serviceCallbackAPIRows(stored, nil))
	mock.ExpectExec(regexp.QuoteMeta(updateCallbackAPIBearerTokenQuery)).
		WithArgs(signedCallbackTokenArg{oldToken: stored.BearerToken, plaintext: "plaintext-token", secrets: []string{"current-secret"}, salt: repo.dangerousSalt}, stored.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	count, err := repo.ResignServiceCallbacks(context.Background(), true, true)
	if err != nil {
		t.Fatalf("ResignServiceCallbacks() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestResignServiceCallbacksReturnsErrorForUnknownSignatureWithoutUnsafe(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db, func(r *Repository) {
		r.apiKeySecrets = []string{"current-secret"}
		r.dangerousSalt = "dangerous-salt"
	})
	serviceID := uuid.New()
	updatedByID := uuid.New()
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	stored := baseServiceCallbackAPIRow(uuid.New(), serviceID, updatedByID, createdAt)
	stored.BearerToken, err = signing.Sign("plaintext-token", "old-secret", repo.dangerousSalt)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(listSignedCallbackAPIsForResignQuery)).
		WillReturnRows(serviceCallbackAPIRows(stored, nil))
	mock.ExpectRollback()

	count, err := repo.ResignServiceCallbacks(context.Background(), true, false)
	if err == nil {
		t.Fatal("ResignServiceCallbacks() error = nil, want error")
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestSaveAndResetInboundApiStoresSignedTokenAndAllowsEmptyURL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db, func(r *Repository) {
		r.apiKeySecrets = []string{"current-secret"}
		r.dangerousSalt = "dangerous-salt"
	})
	serviceID := uuid.New()
	updatedByID := uuid.New()
	createdAt := time.Now().UTC().Truncate(time.Second)
	updatedAt := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	signed, _ := repo.signBearerToken("plaintext-token")
	stored := baseServiceInboundAPIRow(uuid.New(), serviceID, updatedByID, createdAt)
	stored.BearerToken = signed

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(upsertInboundAPI)).
		WithArgs(sqlmock.AnyArg(), serviceID, "https://inbound.example.com", sqlmock.AnyArg(), sqlmock.AnyArg(), sql.NullTime{}, updatedByID, int32(1)).
		WillReturnRows(serviceInboundAPIRows(stored, nil))
	mock.ExpectExec(regexp.QuoteMeta(insertInboundAPIHistoryQuery)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	created, err := repo.SaveServiceInboundApi(context.Background(), serviceID, "https://inbound.example.com", "plaintext-token", updatedByID)
	if err != nil {
		t.Fatalf("SaveServiceInboundApi() error = %v", err)
	}
	if created == nil || created.BearerToken != "plaintext-token" {
		t.Fatalf("created = %#v", created)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(getInboundAPI)).
		WithArgs(serviceID).
		WillReturnRows(serviceInboundAPIRows(stored, nil))
	mock.ExpectQuery(regexp.QuoteMeta(upsertInboundAPI)).
		WithArgs(stored.ID, serviceID, "", signed, stored.CreatedAt, sqlmock.AnyArg(), updatedByID, int32(2)).
		WillReturnRows(serviceInboundAPIRows(stored, func(item *ServiceInboundApi) {
			item.Url = ""
			item.Version = 2
			item.UpdatedAt = sql.NullTime{Time: updatedAt, Valid: true}
		}))
	mock.ExpectExec(regexp.QuoteMeta(insertInboundAPIHistoryQuery)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	empty := ""
	updated, err := repo.ResetServiceInboundApi(context.Background(), serviceID, stored.ID, updatedByID, &empty, nil)
	if err != nil {
		t.Fatalf("ResetServiceInboundApi() error = %v", err)
	}
	if updated == nil || updated.Url != "" || updated.Version != 2 {
		t.Fatalf("updated = %#v", updated)
	}
}

func TestGetAndDeleteInboundApi(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewRepository(db, db, func(r *Repository) {
		r.apiKeySecrets = []string{"current-secret"}
		r.dangerousSalt = "dangerous-salt"
	})
	serviceID := uuid.New()
	updatedByID := uuid.New()
	createdAt := time.Now().UTC().Truncate(time.Second)
	signed, _ := repo.signBearerToken("plaintext-token")
	stored := baseServiceInboundAPIRow(uuid.New(), serviceID, updatedByID, createdAt)
	stored.BearerToken = signed

	mock.ExpectQuery(regexp.QuoteMeta(getInboundAPI)).
		WithArgs(serviceID).
		WillReturnRows(serviceInboundAPIRows(stored, nil))
	got, err := repo.GetServiceInboundApi(context.Background(), serviceID, stored.ID)
	if err != nil {
		t.Fatalf("GetServiceInboundApi() error = %v", err)
	}
	if got == nil || got.BearerToken != "plaintext-token" {
		t.Fatalf("got = %#v", got)
	}

	mock.ExpectQuery(regexp.QuoteMeta(getInboundAPI)).
		WithArgs(serviceID).
		WillReturnRows(serviceInboundAPIRows(stored, nil))
	mock.ExpectExec(regexp.QuoteMeta(deleteInboundAPI)).
		WithArgs(stored.ID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	deleted, err := repo.DeleteServiceInboundApi(context.Background(), serviceID, stored.ID)
	if err != nil {
		t.Fatalf("DeleteServiceInboundApi() error = %v", err)
	}
	if !deleted {
		t.Fatal("expected delete to succeed")
	}
}

func baseServiceCallbackAPIRow(id, serviceID, updatedByID uuid.UUID, createdAt time.Time) ServiceCallbackApi {
	return ServiceCallbackApi{
		ID:          id,
		ServiceID:   serviceID,
		Url:         "https://callback.example.com",
		CreatedAt:   createdAt,
		UpdatedByID: updatedByID,
		Version:     1,
	}
}

func baseServiceInboundAPIRow(id, serviceID, updatedByID uuid.UUID, createdAt time.Time) ServiceInboundApi {
	return ServiceInboundApi{
		ID:          id,
		ServiceID:   serviceID,
		Url:         "https://inbound.example.com",
		CreatedAt:   createdAt,
		UpdatedByID: updatedByID,
		Version:     1,
	}
}

func serviceCallbackAPIRows(base ServiceCallbackApi, mutate func(*ServiceCallbackApi)) *sqlmock.Rows {
	item := base
	if mutate != nil {
		mutate(&item)
	}
	return sqlmock.NewRows(serviceCallbackAPIColumns).AddRow(item.ID, item.ServiceID, item.Url, item.BearerToken, item.CreatedAt, item.UpdatedAt, item.UpdatedByID, item.Version, item.CallbackType, item.IsSuspended, item.SuspendedAt)
}

type signedCallbackTokenArg struct {
	oldToken  string
	plaintext string
	secrets   []string
	salt      string
}

func (a signedCallbackTokenArg) Match(value driver.Value) bool {
	token, ok := value.(string)
	if !ok || token == a.oldToken {
		return false
	}
	plaintext, err := signing.Unsign(token, a.secrets, a.salt)
	if err != nil {
		return false
	}
	return plaintext == a.plaintext
}

func serviceInboundAPIRows(base ServiceInboundApi, mutate func(*ServiceInboundApi)) *sqlmock.Rows {
	item := base
	if mutate != nil {
		mutate(&item)
	}
	return sqlmock.NewRows(serviceInboundAPIColumns).AddRow(item.ID, item.ServiceID, item.Url, item.BearerToken, item.CreatedAt, item.UpdatedAt, item.UpdatedByID, item.Version)
}
