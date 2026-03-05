package handler

import (
	"admin/internal/service"
	reqdto "admin/internal/transport/http/handler/dto/request"
	resdto "admin/internal/transport/http/handler/dto/response"
	time_util "admin/pkg/logger/time"
	"context"
	"errors"
	"strings"
	"time"

	"admin/internal/transport/http/response"
	"admin/pkg/errorvar"

	"github.com/gin-gonic/gin"
)

type EnabledModuleHandler struct {
	Svc       *service.EnabledModuleService
	InstallSv *service.ModuleInstallService
}

func NewEnabledModuleHandler(svc *service.EnabledModuleService, installSvc *service.ModuleInstallService) *EnabledModuleHandler {
	return &EnabledModuleHandler{
		Svc:       svc,
		InstallSv: installSvc,
	}
}

func (h *EnabledModuleHandler) List(c *gin.Context) {
	h.Status(c)
}

func (h *EnabledModuleHandler) Status(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.RespondServiceUnavailable(c, "enabled module service unavailable")
		return
	}

	items, err := h.Svc.List(c.Request.Context())
	if err != nil {
		response.RespondInternalError(c, err.Error())
		return
	}

	lastScanAt := time.Now().In(time.Local)
	installedCount := 0
	outputItems := make([]resdto.EnabledModule, 0, len(items))
	for _, item := range items {
		if item.Installed {
			installedCount++
		}
		outputItems = append(outputItems, resdto.NewEnabledModule(item))
	}

	lastScanText := ""
	lastScanUnix := int64(0)
	if !lastScanAt.IsZero() {
		lastScanText = time_util.FormatTimeLocal(lastScanAt)
		lastScanUnix = lastScanAt.Unix()
	}

	response.RespondSuccess(c, gin.H{
		"items":              outputItems,
		"count":              len(items),
		"installed_count":    installedCount,
		"last_scan_at":       lastScanText,
		"last_scan_at_unix":  lastScanUnix,
		"scan_interval_secs": 0,
	}, "module status")
}

func (h *EnabledModuleHandler) Install(c *gin.Context) {
	if h == nil || h.InstallSv == nil {
		response.RespondServiceUnavailable(c, "module install service unavailable")
		return
	}

	var req reqdto.ModuleInstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.RespondBadRequest(c, "invalid request")
		return
	}

	moduleName := strings.TrimSpace(req.ModuleName)
	if moduleName == "" {
		response.RespondBadRequest(c, "module_name is required")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	result, err := h.InstallSv.Install(ctx, service.ModuleInstallRequest{
		ModuleName:     moduleName,
		Scope:          req.Scope,
		AppHost:        req.AppHost,
		Endpoint:       req.Endpoint,
		InstallCommand: req.InstallCommand,
		SSHHost:        req.SSHHost,
		SSHPort:        req.SSHPort,
		SSHUsername:    req.SSHUsername,
		SSHPassword:    normalizeOptionalSecret(req.SSHPassword),
		SSHPrivateKey:  normalizeOptionalSecret(req.SSHPrivateKey),
	})
	if err != nil {
		switch {
		case errors.Is(err, errorvar.ErrModuleInstallServiceNil):
			response.RespondServiceUnavailable(c, err.Error())
		case errors.Is(err, errorvar.ErrModuleNameInvalid),
			errors.Is(err, errorvar.ErrModuleInstallScope),
			errors.Is(err, errorvar.ErrEndpointNameInvalid):
			response.RespondBadRequest(c, err.Error())
		default:
			response.RespondInternalError(c, err.Error())
		}
		return
	}

	response.RespondSuccess(c, result, "module install completed")
}

func normalizeOptionalSecret(raw *string) *string {
	if raw == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
