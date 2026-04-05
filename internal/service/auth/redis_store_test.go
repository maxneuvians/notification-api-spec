package auth

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
)

func TestNewRedisStore(t *testing.T) {
	t.Run("invalid url returns error", func(t *testing.T) {
		if _, err := NewRedisStore("://bad-url"); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("store supports get set and del", func(t *testing.T) {
		redisServer, err := miniredis.Run()
		if err != nil {
			t.Fatalf("miniredis.Run() error = %v", err)
		}
		defer redisServer.Close()

		store, err := NewRedisStore("redis://" + redisServer.Addr())
		if err != nil {
			t.Fatalf("NewRedisStore() error = %v", err)
		}

		ctx := context.Background()
		if err := store.Set(ctx, "service-auth:test", "payload", time.Minute); err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		got, err := store.Get(ctx, "service-auth:test")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got != "payload" {
			t.Fatalf("Get() = %q, want payload", got)
		}

		if err := store.Del(ctx, "service-auth:test"); err != nil {
			t.Fatalf("Del() error = %v", err)
		}

		got, err = store.Get(ctx, "service-auth:test")
		if err != nil {
			t.Fatalf("Get() after delete error = %v", err)
		}
		if got != "" {
			t.Fatalf("Get() after delete = %q, want empty string", got)
		}
	})
}
