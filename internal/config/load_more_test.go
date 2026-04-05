package config_test

import (
	"testing"
	"time"

	"github.com/maxneuvians/notification-api-spec/internal/config"
)

func TestLoadSetsDerivedValues(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("CACHE_OPS_URL", "")
	t.Setenv("STATSD_HOST", "127.0.0.1")
	t.Setenv("CRONITOR_KEYS", `{"job":"secret"}`)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.CacheOpsURL != cfg.RedisURL {
		t.Fatalf("CacheOpsURL = %q, want RedisURL %q", cfg.CacheOpsURL, cfg.RedisURL)
	}
	if cfg.DBPoolRecycle != 5*time.Minute {
		t.Fatalf("DBPoolRecycle = %v, want 5m", cfg.DBPoolRecycle)
	}
	if cfg.SendingNotificationsTimeout != 72*time.Hour {
		t.Fatalf("SendingNotificationsTimeout = %v, want 72h", cfg.SendingNotificationsTimeout)
	}
	if !cfg.StatsDEnabled {
		t.Fatal("expected StatsDEnabled to be true")
	}
	if cfg.StatsDPort != 8125 {
		t.Fatalf("StatsDPort = %d, want 8125", cfg.StatsDPort)
	}
	if cfg.AdminClientUserName != "notify-admin" {
		t.Fatalf("AdminClientUserName = %q, want notify-admin", cfg.AdminClientUserName)
	}
	if len(cfg.CronitorKeys) != 1 || cfg.CronitorKeys["job"] != "secret" {
		t.Fatalf("CronitorKeys = %#v, want job secret", cfg.CronitorKeys)
	}
}
