package auth

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	apiKeysRepo "github.com/maxneuvians/notification-api-spec/internal/repository/api_keys"
	servicesRepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
)

const defaultCacheKeyPrefix = "service-auth:"

type RedisStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Del(ctx context.Context, key string) error
}

type CachedServiceAuth struct {
	Service     servicesRepo.Service `json:"service"`
	Permissions []string             `json:"permissions"`
	APIKeys     []apiKeysRepo.ApiKey `json:"api_keys"`
}

type ServiceAuthCache struct {
	store  RedisStore
	prefix string
}

func NewServiceAuthCache(store RedisStore) *ServiceAuthCache {
	return &ServiceAuthCache{store: store, prefix: defaultCacheKeyPrefix}
}

func NewRedisStore(redisURL string) (RedisStore, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	return &goRedisStore{client: redis.NewClient(options)}, nil
}

func (c *ServiceAuthCache) Get(ctx context.Context, serviceID uuid.UUID) (*CachedServiceAuth, bool) {
	if c == nil || c.store == nil {
		return nil, false
	}

	payload, err := c.store.Get(ctx, c.key(serviceID))
	if err != nil || payload == "" {
		return nil, false
	}

	var cached CachedServiceAuth
	if err := json.Unmarshal([]byte(payload), &cached); err != nil {
		return nil, false
	}

	return &cached, true
}

func (c *ServiceAuthCache) Set(ctx context.Context, serviceID uuid.UUID, data *CachedServiceAuth, ttl time.Duration) {
	if c == nil || c.store == nil || data == nil {
		return
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return
	}

	_ = c.store.Set(ctx, c.key(serviceID), string(payload), ttl)
}

func (c *ServiceAuthCache) Invalidate(ctx context.Context, serviceID uuid.UUID) {
	if c == nil || c.store == nil {
		return
	}

	_ = c.store.Del(ctx, c.key(serviceID))
}

func (c *ServiceAuthCache) key(serviceID uuid.UUID) string {
	prefix := defaultCacheKeyPrefix
	if c != nil && c.prefix != "" {
		prefix = c.prefix
	}
	return prefix + serviceID.String()
}

type goRedisStore struct {
	client *redis.Client
}

func (s *goRedisStore) Get(ctx context.Context, key string) (string, error) {
	value, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return value, err
}

func (s *goRedisStore) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return s.client.Set(ctx, key, value, ttl).Err()
}

func (s *goRedisStore) Del(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}
