package main

import (
	"context"
	"log"
	"myreddit/config"
	"myreddit/internal/app"
	"os/signal"
	"syscall"
)

func main() {
	cfg := config.LoadConfig()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	a, err := app.NewApp(ctx, cfg)
	if err != nil {
		log.Fatalf("new app: %v", err)
	}

	if err := a.Run(ctx); err != nil {
		log.Fatalf("fatal error: %v", err)
	}
}
