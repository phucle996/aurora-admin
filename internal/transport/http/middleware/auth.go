package middleware

import (
	"admin/internal/service"
	"admin/internal/transport/http/response"
	"admin/pkg/logger"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type adminAPIKeyMemoryCache struct {
	mu    sync.RWMutex
	ttl   time.Duration
	items map[string]time.Time
}

func newAdminAPIKeyMemoryCache(ttl time.Duration) *adminAPIKeyMemoryCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &adminAPIKeyMemoryCache{
		ttl:   ttl,
		items: make(map[string]time.Time),
	}
}

func (m *adminAPIKeyMemoryCache) Has(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}

	m.mu.RLock()
	exp, ok := m.items[key]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		m.mu.Lock()
		delete(m.items, key)
		m.mu.Unlock()
		return false
	}
	return true
}

func (m *adminAPIKeyMemoryCache) Put(key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	m.mu.Lock()
	m.items[key] = time.Now().Add(m.ttl)
	m.mu.Unlock()
}

// AuthAdminAPIKey validates admin API key via in-memory cache first.
// On cache miss, it falls back to service (etcd) and backfills cache.
func AuthAdminAPIKey(apiKeySvc *service.APIKeyService) gin.HandlerFunc {
	cache := newAdminAPIKeyMemoryCache(5 * time.Minute)

	return func(c *gin.Context) {
		ctx := c.Request.Context()
		apiKey := extractAdminAPIKey(c)
		if apiKey == "" {
			response.RespondUnauthorized(c, "unauthorized")
			logger.HandlerInfo(ctx, "auth.apikey", "missing admin api key")
			c.Abort()
			return
		}

		if cache.Has(apiKey) {
			c.Set("admin_api_key", apiKey)
			c.Next()
			return
		}

		if apiKeySvc == nil {
			response.RespondServiceUnavailable(c, "api key validation unavailable")
			logger.HandlerWarn(ctx, "auth.apikey", "api key service is nil")
			c.Abort()
			return
		}

		validated, err := apiKeySvc.ValidateAPIKey(ctx, apiKey)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrAPIKeyInvalid), errors.Is(err, service.ErrAPIKeyMismatch):
				response.RespondUnauthorized(c, "unauthorized")
				logger.HandlerInfo(ctx, "auth.apikey", "admin api key mismatch")
			case errors.Is(err, service.ErrAPIKeyServiceNil):
				response.RespondServiceUnavailable(c, "api key validation unavailable")
				logger.HandlerWarn(ctx, "auth.apikey", "api key service unavailable: %s", err.Error())
			default:
				response.RespondServiceUnavailable(c, "api key validation unavailable")
				logger.HandlerWarn(ctx, "auth.apikey", "validate api key failed: %s", err.Error())
			}
			c.Abort()
			return
		}

		cache.Put(apiKey)
		c.Set("admin_api_key", apiKey)
		c.Set("admin_api_key_version", validated.Version.Version)
		c.Set("admin_api_key_is_current", validated.IsCurrent)
		c.Next()
	}
}

func extractAdminAPIKey(c *gin.Context) string {
	if key := strings.TrimSpace(c.GetHeader("X-Admin-API-Key")); key != "" {
		return key
	}
	if key := strings.TrimSpace(c.GetHeader("X-API-Key")); key != "" {
		return key
	}
	if key, err := c.Cookie("admin_api_key"); err == nil && strings.TrimSpace(key) != "" {
		return strings.TrimSpace(key)
	}
	return ""
}
