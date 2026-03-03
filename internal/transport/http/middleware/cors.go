package middleware

import (
	"admin/internal/config"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func CORS(cfg *config.CorsCfg) gin.HandlerFunc {
	conf := normalizeCors(cfg)
	allowMethods := strings.Join(conf.AllowMethods, ", ")
	allowHeaders := strings.Join(conf.AllowHeaders, ", ")
	exposeHeaders := strings.Join(conf.ExposeHeaders, ", ")
	maxAge := strconv.Itoa(int(conf.MaxAge.Seconds()))

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && isOriginAllowed(origin, conf.AllowOrigins) {
			if conf.AllowCredentials && hasWildcard(conf.AllowOrigins) {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
			} else if hasWildcard(conf.AllowOrigins) {
				c.Header("Access-Control-Allow-Origin", "*")
			} else {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
			}

			if allowMethods != "" {
				c.Header("Access-Control-Allow-Methods", allowMethods)
			}
			if allowHeaders != "" {
				c.Header("Access-Control-Allow-Headers", allowHeaders)
			}
			if exposeHeaders != "" {
				c.Header("Access-Control-Expose-Headers", exposeHeaders)
			}
			if conf.AllowCredentials {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
			if conf.MaxAge > 0 {
				c.Header("Access-Control-Max-Age", maxAge)
			}
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func normalizeCors(cfg *config.CorsCfg) config.CorsCfg {
	if cfg == nil {
		return config.CorsCfg{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
			AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		}
	}

	out := *cfg
	if len(out.AllowOrigins) == 0 {
		out.AllowOrigins = []string{"*"}
	}
	if len(out.AllowMethods) == 0 {
		out.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	}
	if len(out.AllowHeaders) == 0 {
		out.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	}
	return out
}

func hasWildcard(origins []string) bool {
	for _, origin := range origins {
		if strings.TrimSpace(origin) == "*" {
			return true
		}
	}
	return false
}

func isOriginAllowed(origin string, allowed []string) bool {
	if origin == "" {
		return false
	}
	for _, entry := range allowed {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if entry == "*" {
			return true
		}
		if strings.HasPrefix(entry, "*.") {
			if strings.HasSuffix(origin, entry[1:]) {
				return true
			}
			continue
		}
		if strings.HasSuffix(entry, "*") {
			prefix := strings.TrimSuffix(entry, "*")
			if strings.HasPrefix(origin, prefix) {
				return true
			}
			continue
		}
		if strings.HasPrefix(entry, "*") {
			suffix := strings.TrimPrefix(entry, "*")
			if strings.HasSuffix(origin, suffix) {
				return true
			}
			continue
		}
		if origin == entry {
			return true
		}
	}
	return false
}
