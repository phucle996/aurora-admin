package handler

import (
	"admin/internal/service"
	reqdto "admin/internal/transport/http/handler/dto/request"
	resdto "admin/internal/transport/http/handler/dto/response"
	"admin/internal/transport/http/response"
	"admin/pkg/errorvar"
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type APIKeyHandler struct {
	Svc *service.APIKeyService
}

func NewAPIKeyHandler(svc *service.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{Svc: svc}
}

func (h *APIKeyHandler) Login(c *gin.Context) {

	var req reqdto.LoginAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.RespondBadRequest(c, "invalid request")
		return
	}
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		response.RespondBadRequest(c, "api_key is required")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	validated, err := h.Svc.ValidateAPIKey(ctx, apiKey)
	if err != nil {
		switch {
		case errors.Is(err, errorvar.ErrAPIKeyInvalid):
			response.RespondBadRequest(c, "API Key is invalid")
		case errors.Is(err, errorvar.ErrAPIKeyMismatch):
			response.RespondUnauthorized(c, "API Key is invalid")
		case errors.Is(err, errorvar.ErrAPIKeyServiceNil):
			response.RespondServiceUnavailable(c, "API Key service unavailable")
		default:
			response.RespondInternalError(c, err.Error())
		}
		return
	}

	secure := c.Request.TLS != nil
	if strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		secure = true
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("admin_api_key", apiKey, 24*60*60, "/", "", secure, true)

	response.RespondSuccess(c, gin.H{
		"authenticated": true,
		"version":       validated.Version.Version,
		"is_current":    validated.IsCurrent,
	}, "api key login successful")
}

func (h *APIKeyHandler) Rotate(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.RespondServiceUnavailable(c, "api key service unavailable")
		return
	}

	var req reqdto.RotateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.RespondBadRequest(c, "invalid request")
		return
	}
	oldKey := strings.TrimSpace(req.OldKey)
	if oldKey == "" {
		response.RespondBadRequest(c, "old_key is required")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	result, err := h.Svc.RotateAPIKey(ctx, oldKey)
	if err != nil {
		switch {
		case errors.Is(err, errorvar.ErrAPIKeyInvalid):
			response.RespondBadRequest(c, "old_key is invalid")
		case errors.Is(err, errorvar.ErrAPIKeyMismatch):
			response.RespondUnauthorized(c, "old_key is invalid")
		case errors.Is(err, errorvar.ErrAPIKeyRotateTooSoon):
			response.RespondConflict(c, err.Error())
		case errors.Is(err, errorvar.ErrAPIKeyConflict):
			response.RespondConflict(c, "api key changed, please retry")
		case errors.Is(err, errorvar.ErrAPIKeyServiceNil):
			response.RespondServiceUnavailable(c, "api key service unavailable")
		default:
			response.RespondInternalError(c, err.Error())
		}
		return
	}

	response.RespondSuccess(c, gin.H{
		"old": resdto.NewAPIKeyVersion(result.Old),
		"new": resdto.NewAPIKeyVersion(result.New),
	}, "api key rotated")
}
