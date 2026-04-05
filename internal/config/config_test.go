package config_test

import (
	"strings"
	"testing"

	"github.com/maxneuvians/notification-api-spec/internal/config"
)

func TestLoadReturnsMissingRequiredVariables(t *testing.T) {
	t.Setenv("DATABASE_URI", "")
	t.Setenv("ADMIN_CLIENT_SECRET", "admin-secret")
	t.Setenv("SECRET_KEY", "secret-one")
	t.Setenv("DANGEROUS_SALT", "danger-salt")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "DATABASE_URI") {
		t.Fatalf("error = %q, want missing variable name", err.Error())
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("POOL_SIZE", "")
	t.Setenv("NOTIFY_ENVIRONMENT", "")
	t.Setenv("ATTACHMENT_NUM_LIMIT", "")
	t.Setenv("ATTACHMENT_SIZE_LIMIT", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.DBPoolSize != 5 {
		t.Fatalf("DBPoolSize = %d, want 5", cfg.DBPoolSize)
	}

	if cfg.NotifyEnvironment != "development" {
		t.Fatalf("NotifyEnvironment = %q, want development", cfg.NotifyEnvironment)
	}

	if cfg.AttachmentNumLimit != 10 {
		t.Fatalf("AttachmentNumLimit = %d, want 10", cfg.AttachmentNumLimit)
	}

	if cfg.AttachmentSizeLimit != 10485760 {
		t.Fatalf("AttachmentSizeLimit = %d, want 10485760", cfg.AttachmentSizeLimit)
	}
}

func TestLoadSplitsSecretKeys(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SECRET_KEY", "key1,key2,key3")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if len(cfg.SecretKeys) != 3 {
		t.Fatalf("len(SecretKeys) = %d, want 3", len(cfg.SecretKeys))
	}

	if cfg.SecretKeys[0] != "key1" {
		t.Fatalf("SecretKeys[0] = %q, want key1", cfg.SecretKeys[0])
	}
}

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URI", "postgresql://localhost/test")
	t.Setenv("ADMIN_CLIENT_SECRET", "admin-secret")
	t.Setenv("SECRET_KEY", "secret-one")
	t.Setenv("DANGEROUS_SALT", "danger-salt")
}
