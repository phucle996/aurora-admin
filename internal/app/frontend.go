package app

import (
	"admin/pkg/logger"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

const FrontendDistDir = "./dist"

func RegisterFrontendSPA(r *gin.Engine) {

	indexPath := filepath.Join(FrontendDistDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		logger.SysWarn("frontend.serve", "skip serving frontend: %s not found (%v)", indexPath, err)
		return
	}

	logger.SysInfo("frontend.serve", "serving frontend from %s", FrontendDistDir)

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if isBackendRoute(path) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error":   "not_found",
				"message": "route not found",
			})
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error":   "not_found",
				"message": "route not found",
			})
			return
		}

		relativePath := strings.TrimPrefix(path, "/")
		if relativePath != "" {
			clean := filepath.Clean(relativePath)
			if !strings.HasPrefix(clean, "..") {
				targetPath := filepath.Join(FrontendDistDir, clean)
				if info, err := os.Stat(targetPath); err == nil && !info.IsDir() {
					c.File(targetPath)
					return
				}
			}
		}

		c.File(indexPath)
	})
}

func isBackendRoute(path string) bool {
	switch {
	case path == "/api", strings.HasPrefix(path, "/api/"):
		return true
	case path == "/health", strings.HasPrefix(path, "/health/"):
		return true
	default:
		return false
	}
}
