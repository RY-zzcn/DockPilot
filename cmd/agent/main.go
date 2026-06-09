package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dockpilot/dockpilot/internal/agent"
)

func main() {
	cfg := agent.LoadConfig()
	client := agent.NewClient(cfg)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := client.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("agent stopped: %v", err)
	}
}
