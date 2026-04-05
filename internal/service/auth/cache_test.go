package auth

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	apiKeysRepo "github.com/maxneuvians/notification-api-spec/internal/repository/api_keys"
	servicesRepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
)

type mockRedisStore struct {
	values  map[string]string
	lastKey string
	lastTTL time.Duration
	getErr  error
	setErr  error
	delErr  error
	deleted []string
}

func (m *mockRedisStore) Get(_ context.Context, key string) (string, error) {
	m.lastKey = key
	if m.getErr != nil {
		return "", m.getErr
	}
	return m.values[key], nil
}

func (m *mockRedisStore) Set(_ context.Context, key string, value string, ttl time.Duration) error {
	m.lastKey = key
	m.lastTTL = ttl
	if m.setErr != nil {
		return m.setErr
	}
	if m.values == nil {
		m.values = make(map[string]string)
	}
	m.values[key] = value
	return nil
}

func (m *mockRedisStore) Del(_ context.Context, key string) error {
	m.deleted = append(m.deleted, key)
	if m.delErr != nil {
		return m.delErr
	}
	delete(m.values, key)
	return nil
}

func TestServiceAuthCache(t *testing.T) {
	ctx := context.Background()
	serviceID := uuid.New()
	fixture := &CachedServiceAuth{
		Service:     servicesRepo.Service{ID: serviceID, Name: "Example", Active: true},
		Permissions: []string{"send_emails", "view_activity"},
		APIKeys:     []apiKeysRepo.ApiKey{{ID: uuid.New(), ServiceID: serviceID, Secret: "secret", KeyType: "normal", ExpiryDate: sql.NullTime{}}},
	}

	t.Run("cache miss returns false", func(t *testing.T) {
		cache := NewServiceAuthCache(&mockRedisStore{values: map[string]string{}})
		if got, ok := cache.Get(ctx, serviceID); ok || got != nil {
			t.Fatalf("Get() = (%v, %v), want (nil, false)", got, ok)
		}
	})

	t.Run("cache hit returns data", func(t *testing.T) {
		store := &mockRedisStore{values: map[string]string{}}
		cache := NewServiceAuthCache(store)
		cache.Set(ctx, serviceID, fixture, 30*time.Second)

		got, ok := cache.Get(ctx, serviceID)
		if !ok {
			t.Fatal("expected cache hit")
		}
		if got.Service.ID != fixture.Service.ID {
			t.Fatalf("service id = %v, want %v", got.Service.ID, fixture.Service.ID)
		}
		if len(got.Permissions) != len(fixture.Permissions) {
			t.Fatalf("permissions length = %d, want %d", len(got.Permissions), len(fixture.Permissions))
		}
	})

	t.Run("Set populates key with ttl", func(t *testing.T) {
		store := &mockRedisStore{values: map[string]string{}}
		cache := NewServiceAuthCache(store)
		ttl := 45 * time.Second

		cache.Set(ctx, serviceID, fixture, ttl)

		if store.lastTTL != ttl {
			t.Fatalf("Set() ttl = %v, want %v", store.lastTTL, ttl)
		}
		if store.values[cache.key(serviceID)] == "" {
			t.Fatal("expected cached value to be written")
		}
	})

	t.Run("Invalidate deletes key", func(t *testing.T) {
		store := &mockRedisStore{values: map[string]string{}}
		cache := NewServiceAuthCache(store)
		cache.Set(ctx, serviceID, fixture, 30*time.Second)

		cache.Invalidate(ctx, serviceID)

		if len(store.deleted) != 1 || store.deleted[0] != cache.key(serviceID) {
			t.Fatalf("deleted keys = %v, want [%q]", store.deleted, cache.key(serviceID))
		}
	})

	t.Run("blank payload treated as miss", func(t *testing.T) {
		cache := NewServiceAuthCache(&mockRedisStore{values: map[string]string{cacheKey(serviceID): ""}})
		if got, ok := cache.Get(ctx, serviceID); ok || got != nil {
			t.Fatalf("Get() = (%v, %v), want (nil, false)", got, ok)
		}
	})

	t.Run("invalid json treated as miss", func(t *testing.T) {
		cache := NewServiceAuthCache(&mockRedisStore{values: map[string]string{cacheKey(serviceID): "{"}})
		if got, ok := cache.Get(ctx, serviceID); ok || got != nil {
			t.Fatalf("Get() = (%v, %v), want (nil, false)", got, ok)
		}
	})

	t.Run("nil cache behaves as no-op miss", func(t *testing.T) {
		var cache *ServiceAuthCache
		if got, ok := cache.Get(ctx, serviceID); ok || got != nil {
			t.Fatalf("Get() = (%v, %v), want (nil, false)", got, ok)
		}
		cache.Set(ctx, serviceID, fixture, time.Second)
		cache.Invalidate(ctx, serviceID)
	})

	t.Run("nil payload does not write", func(t *testing.T) {
		store := &mockRedisStore{values: map[string]string{}}
		cache := NewServiceAuthCache(store)
		cache.Set(ctx, serviceID, nil, time.Second)
		if len(store.values) != 0 {
			t.Fatalf("stored values = %v, want empty", store.values)
		}
	})

	t.Run("marshal failure does not write", func(t *testing.T) {
		store := &mockRedisStore{values: map[string]string{}}
		cache := NewServiceAuthCache(store)
		invalid := &CachedServiceAuth{
			Service:     fixture.Service,
			Permissions: fixture.Permissions,
			APIKeys:     []apiKeysRepo.ApiKey{{ID: uuid.New(), ServiceID: serviceID, CompromisedKeyInfo: []byte("{")}},
		}
		cache.Set(ctx, serviceID, invalid, time.Second)
		if len(store.values) != 0 {
			t.Fatalf("stored values = %v, want empty", store.values)
		}
	})
}

func cacheKey(serviceID uuid.UUID) string {
	return defaultCacheKeyPrefix + serviceID.String()
}
