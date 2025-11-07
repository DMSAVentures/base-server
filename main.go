package main

import (
	"base-server/internal/bootstrap"
	"base-server/internal/config"
	"base-server/internal/observability"
	"base-server/internal/server"
	"context"
	"os"
)

func main() {
	// Initialize logger and context
	logger := observability.NewLogger()
	ctx := context.Background()
	ctx = observability.WithFields(ctx, observability.Field{Key: "service-name", Value: "base-server"})

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Error(ctx, "failed to load configuration", err)
		os.Exit(1)
	}

	// Initialize all dependencies
	deps, err := bootstrap.Initialize(ctx, cfg, logger)
	if err != nil {
		logger.Error(ctx, "failed to initialize dependencies", err)
		os.Exit(1)
	}

	// Create and setup server
	srv := server.New(cfg, deps, logger)
	srv.Setup()

	// Start server and background workers
	if err := srv.Start(ctx); err != nil {
		logger.Error(ctx, "failed to start server", err)
		os.Exit(1)
	}

	// Wait for shutdown signal and gracefully shutdown
	if err := srv.WaitForShutdown(ctx); err != nil {
		logger.Fatal(ctx, "Server shutdown error", err)
	}
}
