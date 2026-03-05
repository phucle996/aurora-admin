package main

import (
	etcdinfra "admin/infra/etcd"
	"admin/internal/app"
	"admin/internal/config"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := config.LoadConfig()

	bootstrapEtcd, err := etcdinfra.NewClient(&cfg.Etcd)
	if err != nil {
		log.Fatalf("failed to connect etcd for runtime config: %v", err)
	}
	bootstrapCtx, bootstrapCancel := context.WithTimeout(context.Background(), cfg.Etcd.DialTimeout)
	if cfg.Etcd.DialTimeout <= 0 {
		bootstrapCtx, bootstrapCancel = context.WithTimeout(context.Background(), 5*time.Second)
	}
	if err := config.SeedRuntimeToEtcdIfAbsent(bootstrapCtx, bootstrapEtcd, cfg); err != nil {
		bootstrapCancel()
		_ = bootstrapEtcd.Close()
		log.Fatalf("failed to seed runtime config to etcd: %v", err)
	}
	if err := config.LoadRuntimeFromEtcd(bootstrapCtx, bootstrapEtcd, cfg); err != nil {
		bootstrapCancel()
		_ = bootstrapEtcd.Close()
		log.Fatalf("failed to load runtime config from etcd: %v", err)
	}
	bootstrapCancel()
	if err := bootstrapEtcd.Close(); err != nil {
		log.Printf("warning: close bootstrap etcd client failed: %v", err)
	}

	loc, err := time.LoadLocation(cfg.App.TimeZone)
	if err != nil {
		log.Fatalf("invalid timezone: %s", cfg.App.TimeZone)
	}

	time.Local = loc

	application, err := app.NewApplication(cfg)
	if err != nil {
		log.Fatalf("failed to init application: %v", err)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := application.Start(cfg); err != nil {
			log.Printf("http server stopped: %v", err)
			stop <- syscall.SIGTERM
		}
	}()

	<-stop
	log.Println("shutting down application")
	application.Stop()
}
