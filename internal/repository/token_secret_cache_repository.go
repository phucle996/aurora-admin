package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultTokenSecretRedisPrefix   = "aurora:token-secret"
	defaultTokenSecretRedisChannel  = "aurora:token-secret:invalidate"
	defaultTokenSecretRedisEntryTTL = 30 * 24 * time.Hour
)

type TokenSecretCacheRecord struct {
	Kind          string `json:"kind"`
	Version       int64  `json:"version"`
	Secret        string `json:"secret"`
	RotatedAtUnix int64  `json:"rotated_at_unix"`
}

type TokenSecretCacheRepository interface {
	Upsert(ctx context.Context, rec TokenSecretCacheRecord) error
	PublishInvalidate(ctx context.Context, kind string, version int64) error
	KeyPrefix() string
	InvalidateChannel() string
}

type RedisTokenSecretCacheRepository struct {
	redis             *redis.Client
	keyPrefix         string
	invalidateChannel string
}

func NewRedisTokenSecretCacheRepository(client *redis.Client) *RedisTokenSecretCacheRepository {
	return &RedisTokenSecretCacheRepository{
		redis:             client,
		keyPrefix:         defaultTokenSecretRedisPrefix,
		invalidateChannel: defaultTokenSecretRedisChannel,
	}
}

func (r *RedisTokenSecretCacheRepository) Upsert(ctx context.Context, rec TokenSecretCacheRecord) error {
	if r == nil || r.redis == nil {
		return fmt.Errorf("token secret cache redis repository is nil")
	}
	kind := strings.TrimSpace(rec.Kind)
	secret := strings.TrimSpace(rec.Secret)
	if kind == "" || secret == "" || rec.Version <= 0 {
		return fmt.Errorf("invalid token secret cache payload")
	}

	payload, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return r.redis.Set(ctx, r.secretKey(kind), payload, defaultTokenSecretRedisEntryTTL).Err()
}

func (r *RedisTokenSecretCacheRepository) PublishInvalidate(ctx context.Context, kind string, version int64) error {
	if r == nil || r.redis == nil {
		return fmt.Errorf("token secret cache redis repository is nil")
	}
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return fmt.Errorf("token secret kind is required")
	}

	msg := kind + ":" + strconv.FormatInt(version, 10)
	return r.redis.Publish(ctx, r.invalidateChannel, msg).Err()
}

func (r *RedisTokenSecretCacheRepository) KeyPrefix() string {
	if r == nil || strings.TrimSpace(r.keyPrefix) == "" {
		return defaultTokenSecretRedisPrefix
	}
	return strings.TrimSpace(r.keyPrefix)
}

func (r *RedisTokenSecretCacheRepository) InvalidateChannel() string {
	if r == nil || strings.TrimSpace(r.invalidateChannel) == "" {
		return defaultTokenSecretRedisChannel
	}
	return strings.TrimSpace(r.invalidateChannel)
}

func (r *RedisTokenSecretCacheRepository) secretKey(kind string) string {
	return strings.TrimRight(r.KeyPrefix(), ":") + ":" + strings.TrimSpace(kind)
}
