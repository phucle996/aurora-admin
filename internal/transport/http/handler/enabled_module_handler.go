package handler

import (
	"admin/internal/service"
	resdto "admin/internal/transport/http/handler/dto/response"
	time_util "admin/pkg/logger/time"
	"time"

	"admin/internal/transport/http/response"

	"github.com/gin-gonic/gin"
)

type EnabledModuleHandler struct {
	Svc *service.EnabledModuleService
}

func NewEnabledModuleHandler(svc *service.EnabledModuleService) *EnabledModuleHandler {
	return &EnabledModuleHandler{Svc: svc}
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
