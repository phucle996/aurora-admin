package app

import (
	"admin/internal/config"
	grpcTransport "admin/internal/transport/grpc"
	"admin/internal/transport/http/handler"
	"admin/internal/transport/http/middleware"
	"admin/pkg/logger"
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
)

type App struct {
	Ctx    context.Context
	Cancel context.CancelFunc

	Modules *Modules
	Router  *gin.Engine

	Server     *http.Server
	GRPCServer *grpc.Server
	hc         *handler.HealthHandler
}

// NewApplication initializes all dependencies but DOES NOT start the server.
func NewApplication(cfg *config.Config) (*App, error) {
	logger.InitLogger(&cfg.App)

	ctx, cancel := context.WithCancel(context.Background())

	// --------------------
	// Modules
	// --------------------
	modules, err := NewModules(ctx, cfg)
	if err != nil {
		cancel()
		return nil, err
	}
	health := handler.NewHealthHandler(modules.Etcd)
	apiKeyHandler := handler.NewAPIKeyHandler(modules.APIKeySvc)
	enabledModuleHandler := handler.NewEnabledModuleHandler(modules.EnabledModuleSvc)
	// --------------------
	// gin http framework
	// --------------------
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(
		middleware.RequestContext(),
		middleware.AccessLog(),
		middleware.CORS(&cfg.Cors),
		gin.Recovery(),
	)

	RegisterRoutes(router, modules, health, apiKeyHandler, enabledModuleHandler)
	RegisterFrontendSPA(router)

	grpcServer := grpc.NewServer(grpc.ForceServerCodec(grpcTransport.JSONCodec()))
	grpcTransport.RegisterCertTransportServer(
		grpcServer,
		grpcTransport.NewCertTransportService(modules.CertStoreSvc),
	)

	health.MarkReady()
	return &App{
		Ctx:        ctx,
		Cancel:     cancel,
		Modules:    modules,
		hc:         health,
		Router:     router,
		GRPCServer: grpcServer,
	}, nil
}

func (a *App) Start(cfg *config.Config) error {

	httpAddr := fmt.Sprintf(":%d", cfg.App.Port)

	ln, err := net.Listen("tcp", httpAddr)
	if err != nil {
		return err
	}

	a.Server = &http.Server{
		Handler: h2c.NewHandler(a.muxHTTPAndGRPC(), &http2.Server{}),
	}

	a.hc.MarkReady()

	logger.SysInfo("http", "starting http+grpc (h2c) server at %s", httpAddr)

	if err := a.Server.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (a *App) Stop() {
	logger.SysInfo("shutdown", "shutting down application")

	a.hc.MarkNotReady()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if a.Server != nil {
		if err := a.Server.Shutdown(ctx); err != nil {
			logger.SysError("shutdown.http", err, "http shutdown failed")
		}
	}
	if a.GRPCServer != nil {
		// gRPC is served via net/http (h2c + ServeHTTP).
		// Stop() avoids panic from GracefulStop() on serverHandlerTransport.
		a.GRPCServer.Stop()
		logger.SysInfo("shutdown.grpc", "grpc server stopped")
	}

	if a.Cancel != nil {
		a.Cancel()
	}

	if a.Modules != nil {
		// Close etcd
		if a.Modules.Etcd != nil {
			if err := a.Modules.Etcd.Close(); err != nil {
				logger.SysError("shutdown.etcd", err, "etcd shutdown failed")
			}
		}
	}

	logger.SysInfo("shutdown", "application stopped cleanly")
}

func (a *App) muxHTTPAndGRPC() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
		if a.GRPCServer != nil && r.ProtoMajor == 2 && strings.HasPrefix(contentType, "application/grpc") {
			a.GRPCServer.ServeHTTP(w, r)
			return
		}
		a.Router.ServeHTTP(w, r)
	})
}
