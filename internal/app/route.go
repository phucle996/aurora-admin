package app

import (
	"admin/internal/transport/http/handler"
	"admin/internal/transport/http/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes wires HTTP routes to handlers.
func RegisterRoutes(r *gin.Engine, m *Modules,
	health *handler.HealthHandler,
	apiKey *handler.APIKeyHandler,
	enabledModule *handler.EnabledModuleHandler,
) {

	r.GET("/health/liveness", health.Liveness)
	r.GET("/health/readiness", health.Readiness)
	r.GET("/health/startup", health.Startup)

	adminAPIKeyAuth := middleware.AuthAdminAPIKey(m.APIKeySvc)

	api := r.Group("/api/v1")
	{
		api.POST("/apikey/login", apiKey.Login)
		api.POST("/apikey/rotate", adminAPIKeyAuth, apiKey.Rotate)
		api.GET("/modules/enabled", adminAPIKeyAuth, enabledModule.List)
		api.GET("/modules/status", adminAPIKeyAuth, enabledModule.Status)
		api.GET("/modules/install/agents", adminAPIKeyAuth, enabledModule.InstallAgents)
		api.POST("/modules/install/agent-bootstrap-token", adminAPIKeyAuth, enabledModule.RotateAgentBootstrapToken)
		api.POST("/modules/install", adminAPIKeyAuth, enabledModule.Install)
		api.POST("/modules/install/stream", adminAPIKeyAuth, enabledModule.InstallStream)
		api.POST("/modules/reinstall-cert", adminAPIKeyAuth, enabledModule.ReinstallCert)
		api.POST("/modules/reinstall-cert/stream", adminAPIKeyAuth, enabledModule.ReinstallCertStream)
	}
}
