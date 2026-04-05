//go:build smoke

package testutil

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

const defaultSmokeDSN = "postgres://postgres:postgres@db:5432/notification_api?sslmode=disable"

var setupOnce sync.Once

func OpenSmokeDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("REPOSITORY_TEST_DATABASE_URI")
	if strings.TrimSpace(dsn) == "" {
		dsn = defaultSmokeDSN
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open smoke database: %v", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Skipf("repository smoke database unavailable: %v", err)
	}

	setupOnce.Do(func() {
		ensureSchema(t, db)
		seedLookupTables(t, db)
	})

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

func MustCreateUser(t *testing.T, db *sql.DB) uuid.UUID {
	t.Helper()

	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)
	name := fmt.Sprintf("smoke-user-%s", id.String()[:8])
	email := fmt.Sprintf("%s@example.com", name)

	mustExec(t, db, `
		INSERT INTO users (
			id, name, email_address, created_at, _password, password_changed_at,
			failed_login_count, state, platform_admin, auth_type, blocked,
			password_expired, default_editor_is_rte, mobile_number
		) VALUES ($1, $2, $3, $4, $5, $6, 0, 'active', false, 'email_auth', false, false, false, NULL)
	`, id, name, email, now, "ciphertext", now)

	return id
}

func MustCreateService(t *testing.T, db *sql.DB, userID uuid.UUID) uuid.UUID {
	t.Helper()

	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)
	name := fmt.Sprintf("smoke-service-%s", id.String()[:8])
	emailFrom := fmt.Sprintf("%s@example.com", name)

	mustExec(t, db, `
		INSERT INTO services (
			id, name, created_at, active, message_limit, restricted, email_from,
			created_by_id, version, research_mode, prefix_sms, rate_limit,
			count_as_live, sms_daily_limit, email_annual_limit, sms_annual_limit
		) VALUES ($1, $2, $3, true, 1000, false, $4, $5, 1, false, false, 1000, true, 1000, 20000000, 100000)
	`, id, name, now, emailFrom, userID)

	return id
}

func MustCreateTemplateCategory(t *testing.T, db *sql.DB, userID uuid.UUID) uuid.UUID {
	t.Helper()

	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)
	suffix := id.String()[:8]

	mustExec(t, db, `
		INSERT INTO template_categories (
			id, name_en, name_fr, sms_process_type, email_process_type,
			hidden, created_at, updated_at, sms_sending_vehicle, created_by_id
		) VALUES ($1, $2, $3, 'normal', 'normal', false, $4, $4, 'long_code', $5)
	`, id, "Category "+suffix, "Categorie "+suffix, now, userID)

	return id
}

func MustCreateTemplate(t *testing.T, db *sql.DB, serviceID, userID uuid.UUID) uuid.UUID {
	t.Helper()

	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	mustExec(t, db, `
		INSERT INTO templates (
			id, name, template_type, created_at, content, service_id,
			created_by_id, version, archived, hidden, text_direction_rtl
		) VALUES ($1, $2, 'email', $3, 'hello', $4, $5, 1, false, false, false)
	`, id, "Template "+id.String()[:8], now, serviceID, userID)

	return id
}

func MustCreateJob(t *testing.T, db *sql.DB, serviceID uuid.UUID) uuid.UUID {
	t.Helper()

	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	mustExec(t, db, `
		INSERT INTO jobs (
			id, original_file_name, service_id, created_at, notification_count,
			notifications_sent, template_version, notifications_delivered,
			notifications_failed, job_status, archived
		) VALUES ($1, 'batch.csv', $2, $3, 1, 0, 1, 0, 0, 'pending', false)
	`, id, serviceID, now)

	return id
}

func MustCreateProvider(t *testing.T, db *sql.DB) uuid.UUID {
	t.Helper()

	id := uuid.New()
	mustExec(t, db, `
		INSERT INTO provider_details (
			id, display_name, identifier, priority, notification_type,
			active, version, supports_international
		) VALUES ($1, $2, $3, 10, 'sms', false, 1, false)
	`, id, "Provider "+id.String()[:8], "provider-"+id.String()[:8])

	return id
}

func ensureSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	mustExec(t, db, `SELECT pg_advisory_lock(94561123)`)
	defer mustExec(t, db, `SELECT pg_advisory_unlock(94561123)`)

	var exists sql.NullString
	if err := db.QueryRow(`SELECT to_regclass('public.users')`).Scan(&exists); err != nil {
		t.Fatalf("check schema: %v", err)
	}
	if exists.Valid {
		return
	}

	contents, err := os.ReadFile("/workspaces/db/migrations/0001_initial.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := db.Exec(string(contents)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
}

func seedLookupTables(t *testing.T, db *sql.DB) {
	t.Helper()

	statements := []string{
		`INSERT INTO auth_type (name) VALUES ('sms_auth'), ('email_auth'), ('security_key_auth') ON CONFLICT DO NOTHING`,
		`INSERT INTO key_types (name) VALUES ('normal') ON CONFLICT DO NOTHING`,
		`INSERT INTO job_status (name) VALUES ('pending'), ('scheduled'), ('in progress'), ('finished') ON CONFLICT DO NOTHING`,
		`INSERT INTO notification_status_types (name) VALUES ('created'), ('sending'), ('delivered'), ('pending'), ('failed'), ('technical-failure'), ('temporary-failure'), ('permanent-failure'), ('sent') ON CONFLICT DO NOTHING`,
		`INSERT INTO template_process_type (name) VALUES ('normal') ON CONFLICT DO NOTHING`,
		`INSERT INTO invite_status_type (name) VALUES ('pending'), ('accepted'), ('cancelled') ON CONFLICT DO NOTHING`,
	}

	for _, stmt := range statements {
		mustExec(t, db, stmt)
	}
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec query failed: %v\nquery: %s", err, strings.TrimSpace(query))
	}
}
