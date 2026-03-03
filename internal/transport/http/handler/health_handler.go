package handler

import (
	"admin/internal/transport/http/response"
	"context"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// HealthHandler exposes startup/readiness/liveness endpoints.
type HealthHandler struct {
	etcd *clientv3.Client

	// internal state
	started atomic.Bool
	ready   atomic.Bool
}

// NewHealthHandler constructs health handler with optional deps.
func NewHealthHandler(etcd *clientv3.Client) *HealthHandler {
	return &HealthHandler{
		etcd: etcd,
	}
}

// MarkNotReady allows temporarily draining traffic.
func (h *HealthHandler) MarkNotReady() {
	h.ready.Store(false)
}

// MarkReady re-enables readiness.
func (h *HealthHandler) MarkReady() {
	h.ready.Store(true)
}

// Liveness: process health ONLY.
func (h *HealthHandler) Liveness(c *gin.Context) {
	response.RespondSuccess(c, gin.H{
		"status": "ok",
	}, "alive")
}

// Startup: app bootstrapped or not.
func (h *HealthHandler) Startup(c *gin.Context) {
	if !h.ready.Load() {
		response.RespondServiceUnavailable(c, "app still starting")
		return
	}
	response.RespondSuccess(c, gin.H{"status": "ok"}, "started")
}

// Readiness: can we accept new requests?
func (h *HealthHandler) Readiness(c *gin.Context) {
	if !h.ready.Load() {
		response.RespondServiceUnavailable(c, "app not ready")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	status := gin.H{
		"etcd": "skipped",
	}

	var errs []string

	// Core dependency: etcd
	if h.etcd != nil {
		endpoints := h.etcd.Endpoints()
		if len(endpoints) == 0 {
			status["etcd"] = "unhealthy"
			errs = append(errs, "etcd: no endpoints configured")
		} else {
			if _, err := h.etcd.Status(ctx, endpoints[0]); err != nil {
				status["etcd"] = "unhealthy"
				errs = append(errs, "etcd: "+err.Error())
			} else {
				status["etcd"] = "ok"
			}
		}
	}

	// Only FAIL readiness for sync core deps.
	if len(errs) > 0 {
		response.RespondServiceUnavailable(
			c,
			"readiness failed: "+joinErrors(errs),
		)
		return
	}

	response.RespondSuccess(c, status, "ready")
}

/*
==========
UTILS
==========
*/

func joinErrors(errs []string) string {
	switch len(errs) {
	case 0:
		return ""
	case 1:
		return errs[0]
	default:
		out := errs[0]
		for _, e := range errs[1:] {
			out += "; " + e
		}
		return out
	}
}

/*
==========
GUARDS
==========
*/
