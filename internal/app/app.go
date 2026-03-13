package app

import (
	"admin/internal/config"
	grpcTransport "admin/internal/transport/grpc"
	"admin/internal/transport/http/handler"
	"admin/internal/transport/http/middleware"
	"admin/pkg/logger"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/http2"
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
	enabledModuleHandler := handler.NewEnabledModuleHandler(
		modules.EnabledModuleSvc,
		modules.ModuleInstallSvc,
		modules.RuntimeSvc,
		cfg.App.Port,
	)
	// --------------------
	// gin http framework
	// --------------------
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(
		middleware.RequestContext(),
		middleware.AccessLog(),
		gin.Recovery(),
	)

	RegisterRoutes(router, modules, health, apiKeyHandler, enabledModuleHandler)
	RegisterFrontendSPA(router)

	grpcServer := grpc.NewServer()
	grpcTransport.RegisterCertTransportServer(
		grpcServer,
		grpcTransport.NewCertTransportService(modules.CertStoreSvc),
	)
	grpcTransport.RegisterRuntimeTransportServer(
		grpcServer,
		grpcTransport.NewRuntimeTransportService(modules.RuntimeSvc),
	)

	go modules.TokenSecretSvc.StartAutoRotate(ctx, time.Minute, func(err error) {
		logger.SysWarn("token.secret.rotate", "auto rotate failed: %v", err)
	})

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

	a.Server = &http.Server{}
	handler := a.muxHTTPAndGRPC()

	tlsCfg, tlsErr := buildServerTLSConfig(cfg.App, cfg.AgentMTLS.CACert)
	if tlsErr != nil {
		_ = ln.Close()
		return tlsErr
	}
	a.Server.Handler = handler
	a.Server.TLSConfig = tlsCfg
	if http2Err := http2.ConfigureServer(a.Server, &http2.Server{}); http2Err != nil {
		_ = ln.Close()
		return fmt.Errorf("configure http2 tls server failed: %w", http2Err)
	}
	ln = tls.NewListener(ln, tlsCfg)
	logger.SysInfo("http", "starting https+grpc (h2) server at %s", httpAddr)

	a.hc.MarkReady()

	if err := a.Server.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func buildServerTLSConfig(cfg config.AppCfg, agentCACertPath string) (*tls.Config, error) {
	certFile := strings.TrimSpace(cfg.TLSCert)
	keyFile := strings.TrimSpace(cfg.TLSKey)
	if certFile == "" || keyFile == "" {
		return nil, fmt.Errorf("tls requires APP_TLS_CERT_FILE and APP_TLS_KEY_FILE")
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load tls cert/key failed: %w", err)
	}

	tlsCfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h2", "http/1.1"},
	}

	caFile := strings.TrimSpace(cfg.TLSCA)
	if caFile != "" {
		caPEM, readErr := os.ReadFile(caFile)
		if readErr != nil {
			return nil, fmt.Errorf("read tls ca file failed: %w", readErr)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("invalid tls ca pem")
		}
		agentCA := strings.TrimSpace(agentCACertPath)
		if agentCA != "" && agentCA != caFile {
			agentCAPEM, agentReadErr := os.ReadFile(agentCA)
			if agentReadErr != nil {
				return nil, fmt.Errorf("read agent tls ca file failed: %w", agentReadErr)
			}
			if !pool.AppendCertsFromPEM(agentCAPEM) {
				return nil, fmt.Errorf("invalid agent tls ca pem")
			}
		}

		leaf, parseLeafErr := x509.ParseCertificate(cert.Certificate[0])
		if parseLeafErr != nil {
			return nil, fmt.Errorf("parse server tls leaf certificate failed: %w", parseLeafErr)
		}
		intermediates := x509.NewCertPool()
		for i := 1; i < len(cert.Certificate); i++ {
			if parsed, parseErr := x509.ParseCertificate(cert.Certificate[i]); parseErr == nil {
				intermediates.AddCert(parsed)
			}
		}
		verifyOpts := x509.VerifyOptions{
			Roots:         pool,
			Intermediates: intermediates,
			CurrentTime:   time.Now(),
		}
		if _, verifyErr := leaf.Verify(verifyOpts); verifyErr != nil {
			return nil, fmt.Errorf("tls server cert is not signed by APP_TLS_CA_FILE: %w", verifyErr)
		}

		tlsCfg.ClientCAs = pool
		// Allow first-time agent bootstrap over one-way TLS; sensitive RPCs still enforce mTLS in handler.
		tlsCfg.ClientAuth = tls.VerifyClientCertIfGiven
	} else {
		return nil, fmt.Errorf("tls requires app tls ca file for mTLS")
	}

	return tlsCfg, nil
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
		if a.Modules.Redis != nil {
			if err := a.Modules.Redis.Close(); err != nil {
				logger.SysError("shutdown.redis", err, "redis shutdown failed")
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
