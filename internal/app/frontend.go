package app

import (
	"admin/pkg/logger"
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

const frontendDistDir = "dist"
const frontendIndexPath = frontendDistDir + "/index.html"

//go:embed all:dist
var frontendEmbedFS embed.FS

func RegisterFrontendSPA(r *gin.Engine) {
	if _, err := fs.Stat(frontendEmbedFS, frontendIndexPath); err != nil {
		logger.SysWarn("frontend.serve", "skip serving embedded frontend: %s not found (%v)", frontendIndexPath, err)
		return
	}

	logger.SysInfo("frontend.serve", "serving embedded frontend from %s", frontendDistDir)

	r.NoRoute(func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		if isBackendRoute(requestPath) {
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

		relativePath := strings.TrimPrefix(path.Clean(requestPath), "/")
		if relativePath == "." {
			relativePath = ""
		}
		if relativePath != "" {
			if !strings.HasPrefix(relativePath, "..") {
				targetPath := path.Join(frontendDistDir, relativePath)
				if stat, err := fs.Stat(frontendEmbedFS, targetPath); err == nil && !stat.IsDir() {
					contentType := mime.TypeByExtension(path.Ext(targetPath))
					if contentType == "" {
						contentType = "application/octet-stream"
					}
					payload, err := fs.ReadFile(frontendEmbedFS, targetPath)
					if err == nil {
						c.Data(http.StatusOK, contentType, payload)
						return
					}
				}
			}
		}

		indexPayload, err := fs.ReadFile(frontendEmbedFS, frontendIndexPath)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":   "frontend_unavailable",
				"message": "frontend index not found",
			})
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexPayload)
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
