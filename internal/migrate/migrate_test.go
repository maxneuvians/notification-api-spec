package migrate

import (
	"database/sql"
	"os"
	"strings"
	"testing"
)

func TestNormalizeDatabaseURI(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "trim and convert psycopg2", raw: " postgresql+psycopg2://user:pass@host/db ", want: "postgres://user:pass@host/db"},
		{name: "convert postgres scheme", raw: "postgresql://user:pass@host/db?sslmode=disable", want: "postgres://user:pass@host/db?sslmode=disable"},
		{name: "keep postgres", raw: "postgres://user:pass@host/db", want: "postgres://user:pass@host/db"},
		{name: "return invalid parse input", raw: "postgres://[::1", want: "postgres://[::1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeDatabaseURI(tc.raw); got != tc.want {
				t.Fatalf("NormalizeDatabaseURI(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestRun(t *testing.T) {
	t.Run("rejects nil database", func(t *testing.T) {
		if err := Run(nil, "postgres://localhost/test"); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns create migrator error", func(t *testing.T) {
		err := Run(&sql.DB{}, "mysql://localhost/test")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "create migrator") {
			t.Fatalf("error = %q, want create migrator prefix", err.Error())
		}
	})

	t.Run("returns connection error while creating migrator", func(t *testing.T) {
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd() error = %v, want nil", err)
		}
		if err := os.Chdir("../.."); err != nil {
			t.Fatalf("Chdir() error = %v, want nil", err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		err = Run(&sql.DB{}, "postgres://127.0.0.1:1/notify?connect_timeout=1&sslmode=disable")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "create migrator") {
			t.Fatalf("error = %q, want create migrator prefix", err.Error())
		}
	})
}
