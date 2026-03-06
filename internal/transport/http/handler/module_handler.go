package handler

import (
	installsvc "admin/internal/moduleinstall"
	"admin/internal/service"
	reqdto "admin/internal/transport/http/handler/dto/request"
	resdto "admin/internal/transport/http/handler/dto/response"
	time_util "admin/pkg/logger/time"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"admin/internal/transport/http/response"
	"admin/pkg/errorvar"

	"github.com/gin-gonic/gin"
)

type EnabledModuleHandler struct {
	Svc       *service.EnabledModuleService
	InstallSv *installsvc.ModuleInstallService
}

func NewEnabledModuleHandler(svc *service.EnabledModuleService, installSvc *installsvc.ModuleInstallService) *EnabledModuleHandler {
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

	result, err := h.InstallSv.InstallWithLog(ctx, installsvc.ModuleInstallRequest{
		ModuleName:            moduleName,
		Scope:                 req.Scope,
		AppHost:               req.AppHost,
		AppPort:               req.AppPort,
		Endpoint:              req.Endpoint,
		InstallCommand:        req.InstallCommand,
		SSHHost:               req.SSHHost,
		SSHPort:               req.SSHPort,
		SSHUsername:           req.SSHUsername,
		SSHPassword:           normalizeOptionalSecret(req.SSHPassword),
		SSHPrivateKey:         normalizeOptionalSecret(req.SSHPrivateKey),
		SSHHostKeyFingerprint: normalizeOptionalSecret(req.SSHHostKeyFingerprint),
	}, nil)
	if err != nil {
		switch {
		case errors.Is(err, errorvar.ErrModuleInstallServiceNil):
			response.RespondServiceUnavailable(c, err.Error())
		case errors.Is(err, errorvar.ErrModuleNameInvalid),
			errors.Is(err, errorvar.ErrModuleInstallScope),
			errors.Is(err, errorvar.ErrModuleInstallCommand),
			errors.Is(err, errorvar.ErrModuleInstallerMissing),
			errors.Is(err, errorvar.ErrEndpointNameInvalid):
			response.RespondBadRequest(c, err.Error())
		default:
			response.RespondInternalError(c, err.Error())
		}
		return
	}

	response.RespondSuccess(c, buildModuleInstallResponse(*result), "module install completed")
}

func (h *EnabledModuleHandler) InstallStream(c *gin.Context) {
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

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.RespondInternalError(c, "streaming is not supported")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher.Flush()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
	defer cancel()

	var writeMu sync.Mutex
	writeEvent := func(eventType string, stage string, message string, data any) error {
		payload := map[string]any{
			"type": eventType,
		}
		if strings.TrimSpace(stage) != "" {
			payload["stage"] = stage
		}
		if strings.TrimSpace(message) != "" {
			payload["message"] = message
		}
		if data != nil {
			payload["data"] = data
		}

		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		writeMu.Lock()
		defer writeMu.Unlock()
		if _, err := c.Writer.Write([]byte("data: ")); err != nil {
			return err
		}
		if _, err := c.Writer.Write(raw); err != nil {
			return err
		}
		if _, err := c.Writer.Write([]byte("\n\n")); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	_ = writeEvent("log", "ui", "stream connected, start install", nil)

	result, err := h.InstallSv.InstallWithLog(ctx, installsvc.ModuleInstallRequest{
		ModuleName:            moduleName,
		Scope:                 req.Scope,
		AppHost:               req.AppHost,
		AppPort:               req.AppPort,
		Endpoint:              req.Endpoint,
		InstallCommand:        req.InstallCommand,
		SSHHost:               req.SSHHost,
		SSHPort:               req.SSHPort,
		SSHUsername:           req.SSHUsername,
		SSHPassword:           normalizeOptionalSecret(req.SSHPassword),
		SSHPrivateKey:         normalizeOptionalSecret(req.SSHPrivateKey),
		SSHHostKeyFingerprint: normalizeOptionalSecret(req.SSHHostKeyFingerprint),
	}, func(stage, message string) {
		_ = writeEvent("log", stage, message, nil)
	})
	if err != nil {
		_ = writeEvent("error", "service", err.Error(), nil)
		return
	}

	_ = writeEvent("result", "service", "module install completed", buildModuleInstallResponse(*result))
}

func (h *EnabledModuleHandler) ReinstallCert(c *gin.Context) {
	if h == nil || h.InstallSv == nil {
		response.RespondServiceUnavailable(c, "module install service unavailable")
		return
	}

	var req reqdto.ModuleReinstallCertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.RespondBadRequest(c, "invalid request")
		return
	}

	moduleName := strings.TrimSpace(req.ModuleName)
	if moduleName == "" {
		response.RespondBadRequest(c, "module_name is required")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Minute)
	defer cancel()

	result, err := h.InstallSv.ReinstallCertWithLog(ctx, installsvc.ModuleReinstallCertRequest{
		ModuleName: moduleName,
	}, nil)
	if err != nil {
		switch {
		case errors.Is(err, errorvar.ErrModuleInstallServiceNil):
			response.RespondServiceUnavailable(c, err.Error())
		case errors.Is(err, errorvar.ErrModuleNameInvalid), errors.Is(err, errorvar.ErrModuleEndpointInvalid):
			response.RespondBadRequest(c, err.Error())
		case errors.Is(err, errorvar.ErrModuleEndpointNotFound):
			response.RespondNotFound(c, err.Error())
		default:
			response.RespondInternalError(c, err.Error())
		}
		return
	}

	response.RespondSuccess(c, buildModuleReinstallCertResponse(*result), "module cert reinstalled")
}

func (h *EnabledModuleHandler) ReinstallCertStream(c *gin.Context) {
	if h == nil || h.InstallSv == nil {
		response.RespondServiceUnavailable(c, "module install service unavailable")
		return
	}

	var req reqdto.ModuleReinstallCertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.RespondBadRequest(c, "invalid request")
		return
	}

	moduleName := strings.TrimSpace(req.ModuleName)
	if moduleName == "" {
		response.RespondBadRequest(c, "module_name is required")
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.RespondInternalError(c, "streaming is not supported")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher.Flush()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	var writeMu sync.Mutex
	writeEvent := func(eventType string, stage string, message string, data any) error {
		payload := map[string]any{
			"type": eventType,
		}
		if strings.TrimSpace(stage) != "" {
			payload["stage"] = stage
		}
		if strings.TrimSpace(message) != "" {
			payload["message"] = message
		}
		if data != nil {
			payload["data"] = data
		}

		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		writeMu.Lock()
		defer writeMu.Unlock()
		if _, err := c.Writer.Write([]byte("data: ")); err != nil {
			return err
		}
		if _, err := c.Writer.Write(raw); err != nil {
			return err
		}
		if _, err := c.Writer.Write([]byte("\n\n")); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	_ = writeEvent("log", "ui", "stream connected, start reinstall cert", nil)

	result, err := h.InstallSv.ReinstallCertWithLog(ctx, installsvc.ModuleReinstallCertRequest{
		ModuleName: moduleName,
	}, func(stage, message string) {
		_ = writeEvent("log", stage, message, nil)
	})
	if err != nil {
		_ = writeEvent("error", "service", err.Error(), nil)
		return
	}

	_ = writeEvent("result", "service", "module cert reinstalled", buildModuleReinstallCertResponse(*result))
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

func buildModuleInstallResponse(item installsvc.ModuleInstallResult) resdto.ModuleInstallResult {
	return resdto.ModuleInstallResult{
		ModuleName:        item.ModuleName,
		Scope:             item.Scope,
		Endpoint:          item.Endpoint,
		EndpointValue:     item.EndpointValue,
		InstallExecuted:   item.InstallExecuted,
		InstallOutput:     item.InstallOutput,
		InstallExitCode:   item.InstallExitCode,
		HostsUpdated:      item.HostsUpdated,
		Warnings:          item.Warnings,
		SchemaKey:         item.SchemaKey,
		SchemaName:        item.SchemaName,
		MigrationFiles:    item.MigrationFiles,
		MigrationSource:   item.MigrationSource,
		HealthcheckPassed: item.HealthcheckPassed,
		HealthcheckOutput: item.HealthcheckOutput,
	}
}

func buildModuleReinstallCertResponse(item installsvc.ModuleReinstallCertResult) resdto.ModuleReinstallCertResult {
	return resdto.ModuleReinstallCertResult{
		ModuleName:        item.ModuleName,
		Scope:             item.Scope,
		Endpoint:          item.Endpoint,
		TargetHost:        item.TargetHost,
		CertPath:          item.CertPath,
		KeyPath:           item.KeyPath,
		CAPath:            item.CAPath,
		Warnings:          item.Warnings,
		HealthcheckPassed: item.HealthcheckPassed,
		HealthcheckOutput: item.HealthcheckOutput,
	}
}
