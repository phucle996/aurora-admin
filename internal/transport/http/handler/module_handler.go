package handler

import (
	installsvc "admin/internal/moduleinstall"
	runtimesvc "admin/internal/runtime/service"
	"admin/internal/service"
	reqdto "admin/internal/transport/http/handler/dto/request"
	resdto "admin/internal/transport/http/handler/dto/response"
	time_util "admin/pkg/logger/time"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
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
	RuntimeSv *runtimesvc.RuntimeBootstrapService
	AppPort   int
}

const (
	moduleInstallStreamTimeout     = 45 * time.Minute
	moduleReinstallStreamTimeout   = 15 * time.Minute
	moduleInstallSSEHeartbeatEvery = 10 * time.Second
)

func NewEnabledModuleHandler(
	svc *service.EnabledModuleService,
	installSvc *installsvc.ModuleInstallService,
	runtimeSvc *runtimesvc.RuntimeBootstrapService,
	appPort int,
) *EnabledModuleHandler {
	return &EnabledModuleHandler{
		Svc:       svc,
		InstallSv: installSvc,
		RuntimeSv: runtimeSvc,
		AppPort:   appPort,
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
		ModuleName: moduleName,
		AgentID:    req.AgentID,
		AppHost:    req.AppHost,
	}, nil)
	if err != nil {
		switch {
		case errors.Is(err, errorvar.ErrModuleInstallServiceNil):
			response.RespondServiceUnavailable(c, err.Error())
		case errors.Is(err, errorvar.ErrModuleNameInvalid),
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
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Content-Encoding", "identity")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher.Flush()

	ctx, cancel := context.WithTimeout(c.Request.Context(), moduleInstallStreamTimeout)
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
	emitEvent := func(eventType string, stage string, message string, data any) bool {
		if err := writeEvent(eventType, stage, message, data); err != nil {
			cancel()
			return false
		}
		return true
	}
	stopHeartbeat := startSSEHeartbeat(ctx, c.Writer, flusher, &writeMu, cancel, moduleInstallSSEHeartbeatEvery)
	defer stopHeartbeat()

	result, err := h.InstallSv.InstallWithLog(ctx, installsvc.ModuleInstallRequest{
		ModuleName: moduleName,
		AgentID:    req.AgentID,
		AppHost:    req.AppHost,
	}, func(stage, message string) {
		_ = emitEvent("log", stage, message, nil)
	})
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		_ = emitEvent("error", "service", err.Error(), nil)
		return
	}

	if ctx.Err() != nil {
		return
	}
	_ = emitEvent("result", "service", "module install completed", buildModuleInstallResponse(*result))
}

func (h *EnabledModuleHandler) InstallAgents(c *gin.Context) {
	if h == nil || h.InstallSv == nil {
		response.RespondServiceUnavailable(c, "module install service unavailable")
		return
	}

	items, err := h.InstallSv.ListInstallAgents(c.Request.Context())
	if err != nil {
		response.RespondInternalError(c, err.Error())
		return
	}

	output := make([]resdto.ModuleInstallAgent, 0, len(items))
	for _, item := range items {
		output = append(output, resdto.ModuleInstallAgent{
			AgentID:           item.AgentID,
			Status:            item.Status,
			Hostname:          item.Hostname,
			AgentGRPCEndpoint: item.AgentGRPCEndpoint,
		})
	}

	response.RespondSuccess(c, gin.H{
		"items": output,
		"count": len(output),
	}, "module install agent list")
}

func (h *EnabledModuleHandler) InstallOperation(c *gin.Context) {
	if h == nil || h.InstallSv == nil {
		response.RespondServiceUnavailable(c, "module install service unavailable")
		return
	}
	operationID := strings.TrimSpace(c.Param("operation_id"))
	if operationID == "" {
		response.RespondBadRequest(c, "operation_id is required")
		return
	}

	summary, events, err := h.InstallSv.GetInstallOperation(c.Request.Context(), operationID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			response.RespondNotFound(c, err.Error())
			return
		}
		response.RespondInternalError(c, err.Error())
		return
	}
	response.RespondSuccess(c, gin.H{
		"summary": summary,
		"events":  events,
	}, "module install operation")
}

func (h *EnabledModuleHandler) RotateAgentBootstrapToken(c *gin.Context) {
	if h == nil || h.RuntimeSv == nil {
		response.RespondServiceUnavailable(c, "runtime bootstrap service unavailable")
		return
	}

	result, err := h.RuntimeSv.RotateAgentBootstrapToken(c.Request.Context())
	if err != nil {
		response.RespondInternalError(c, err.Error())
		return
	}

	response.RespondSuccess(c, gin.H{
		"token":          result.Token,
		"token_hash":     result.TokenHash,
		"cluster_policy": result.ClusterPolicy,
	}, "agent bootstrap token rotated")
}

func (h *EnabledModuleHandler) AgentInstallBootstrapMetadata(c *gin.Context) {
	if h == nil {
		response.RespondServiceUnavailable(c, "module handler unavailable")
		return
	}
	if h.AppPort <= 0 || h.AppPort > 65535 {
		response.RespondInternalError(c, "invalid admin grpc port")
		return
	}

	host := requestHostOnly(c.Request)
	if host == "" {
		response.RespondInternalError(c, "cannot resolve request host")
		return
	}

	response.RespondSuccess(c, gin.H{
		"admin_grpc_endpoint": net.JoinHostPort(host, fmt.Sprintf("%d", h.AppPort)),
		"admin_server_name":   host,
		"admin_grpc_port":     h.AppPort,
		"cluster_id":          "default",
	}, "agent install bootstrap metadata")
}

func requestHostOnly(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return ""
	}
	if strings.HasPrefix(host, "[") {
		if parsedHost, _, err := net.SplitHostPort(host); err == nil {
			return strings.Trim(strings.TrimSpace(parsedHost), "[]")
		}
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		return strings.TrimSpace(parsedHost)
	}
	if strings.Count(host, ":") == 1 {
		if idx := strings.LastIndex(host, ":"); idx > 0 {
			return strings.TrimSpace(host[:idx])
		}
	}
	return strings.Trim(strings.TrimSpace(host), "[]")
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
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Content-Encoding", "identity")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	flusher.Flush()

	ctx, cancel := context.WithTimeout(c.Request.Context(), moduleReinstallStreamTimeout)
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
	emitEvent := func(eventType string, stage string, message string, data any) bool {
		if err := writeEvent(eventType, stage, message, data); err != nil {
			cancel()
			return false
		}
		return true
	}
	stopHeartbeat := startSSEHeartbeat(ctx, c.Writer, flusher, &writeMu, cancel, moduleInstallSSEHeartbeatEvery)
	defer stopHeartbeat()

	result, err := h.InstallSv.ReinstallCertWithLog(ctx, installsvc.ModuleReinstallCertRequest{
		ModuleName: moduleName,
	}, func(stage, message string) {
		_ = emitEvent("log", stage, message, nil)
	})
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		_ = emitEvent("error", "service", err.Error(), nil)
		return
	}

	if ctx.Err() != nil {
		return
	}
	_ = emitEvent("result", "service", "module cert reinstalled", buildModuleReinstallCertResponse(*result))
}

func startSSEHeartbeat(
	ctx context.Context,
	w gin.ResponseWriter,
	flusher http.Flusher,
	mu *sync.Mutex,
	cancel context.CancelFunc,
	interval time.Duration,
) func() {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				var writeErr error
				mu.Lock()
				_, writeErr = w.Write([]byte(": keepalive\n\n"))
				if writeErr == nil {
					flusher.Flush()
				}
				mu.Unlock()
				if writeErr != nil {
					cancel()
					return
				}
			}
		}
	}()

	return func() {
		close(done)
	}
}

func buildModuleInstallResponse(item installsvc.ModuleInstallResult) resdto.ModuleInstallResult {
	return resdto.ModuleInstallResult{
		OperationID:      item.OperationID,
		ModuleName:       item.ModuleName,
		AgentID:          item.AgentID,
		Version:          item.Version,
		ArtifactChecksum: item.ArtifactChecksum,
		ServiceName:      item.ServiceName,
		Endpoint:         item.Endpoint,
		Health:           item.Health,
		HostsUpdated:     item.HostsUpdated,
		Warnings:         item.Warnings,
	}
}

func buildModuleReinstallCertResponse(item installsvc.ModuleReinstallCertResult) resdto.ModuleReinstallCertResult {
	return resdto.ModuleReinstallCertResult{
		ModuleName:        item.ModuleName,
		Endpoint:          item.Endpoint,
		Warnings:          item.Warnings,
		HealthcheckPassed: item.HealthcheckPassed,
	}
}
