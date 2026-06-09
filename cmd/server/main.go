package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dockpilot/dockpilot/internal/server"
)

func main() {
	cfg := server.LoadConfig()
	app, err := server.NewApp(cfg)
	if err != nil {
		log.Fatalf("start server: %v", err)
	}
	defer app.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
