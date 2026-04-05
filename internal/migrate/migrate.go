package migrate

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"database/sql"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func Run(db *sql.DB, connURL string) error {
	if db == nil {
		return errors.New("nil database handle")
	}

	m, err := migrate.New("file://db/migrations", NormalizeDatabaseURI(connURL))
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}

func NormalizeDatabaseURI(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.Replace(trimmed, "postgresql+psycopg2://", "postgres://", 1)
	trimmed = strings.Replace(trimmed, "postgresql://", "postgres://", 1)

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}

	if parsed.Scheme == "postgres" {
		return parsed.String()
	}

	return trimmed
}
