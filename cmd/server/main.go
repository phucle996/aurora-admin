package main

import (
	"admin/internal/app"
	"admin/internal/config"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := config.LoadConfig()

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
