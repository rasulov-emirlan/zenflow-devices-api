package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rasulov-emirlan/zenflow-devices-api/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a := app.New()
	if err := a.Init(ctx); err != nil {
		log.Printf("init error: %v", err)
		os.Exit(1)
	}

	runErr := a.Run(ctx)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	if runErr != nil {
		log.Printf("run error: %v", runErr)
		os.Exit(1)
	}
}
