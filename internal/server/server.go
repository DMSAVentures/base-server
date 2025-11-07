package server

import (
	apisetup "base-server/internal/api"
	"base-server/internal/bootstrap"
	"base-server/internal/config"
	"base-server/internal/observability"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Server encapsulates the HTTP server and its dependencies
type Server struct {
	httpServer *http.Server
	router     *gin.Engine
	deps       *bootstrap.Dependencies
	config     *config.Config
	logger     *observability.Logger
}

// New creates a new Server instance
func New(cfg *config.Config, deps *bootstrap.Dependencies, logger *observability.Logger) *Server {
	return &Server{
		config: cfg,
		deps:   deps,
		logger: logger,
	}
}

// Setup configures the HTTP router with middleware and routes
func (s *Server) Setup() {
	s.router = gin.New()

	// Configure CORS
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowCredentials = true
	corsConfig.AllowMethods = []string{"GET", "POST", "OPTIONS", "DELETE"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Last-Event-ID", "Cache-Control", "Connection", "Accept", "Transfer-Encoding"}
	corsConfig.AllowOrigins = []string{s.config.Services.WebAppURI}

	// Allow localhost in non-production
	if os.Getenv("GO_ENV") != "production" {
		corsConfig.AllowOrigins = []string{"http://localhost:3000", "https://accounts.google.com"}
	}

	// Apply middleware
	s.router.Use(cors.New(corsConfig))
	s.router.Use(observability.Middleware(s.logger))

	// Register routes
	rootRouter := s.router.Group("/")
	api := apisetup.New(
		rootRouter,
		s.deps.AuthHandler,
		s.deps.CampaignHandler,
		s.deps.WaitlistHandler,
		s.deps.RewardHandler,
		s.deps.BillingHandler,
		s.deps.AIHandler,
		s.deps.VoiceCallHandler,
		s.deps.WebhookHandler,
	)
	api.RegisterRoutes()
}

// Start begins listening for HTTP requests and starts background workers
func (s *Server) Start(ctx context.Context) error {
	// Start webhook event consumer (processes events from Kafka)
	go func() {
		if err := s.deps.WebhookConsumer.Start(ctx); err != nil {
			s.logger.Error(ctx, "webhook event consumer stopped with error", err)
		}
	}()

	// Start webhook retry worker (runs every 30 seconds for failed deliveries)
	go s.deps.WebhookWorker.Start(ctx)

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.Server.Port),
		Handler: s.router,
	}

	// Run the server in a goroutine so that it doesn't block
	go func() {
		s.logger.Info(ctx, fmt.Sprintf("Server starting on port %d", s.config.Server.Port))
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error(ctx, "server failed to start", err)
			os.Exit(1)
		}
	}()

	return nil
}

// WaitForShutdown blocks until a shutdown signal is received, then gracefully shuts down
func (s *Server) WaitForShutdown(ctx context.Context) error {
	// Set up a channel to listen for OS signals for shutdown
	quit := make(chan os.Signal, 1)
	// kill (no param) default sends syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be caught, so don't need to add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Block until a signal is received
	<-quit
	s.logger.Info(ctx, "Shutting down server...")

	// Stop webhook worker and consumer
	s.deps.WebhookWorker.Stop()
	s.deps.WebhookConsumer.Stop()

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	// Cleanup dependencies
	s.deps.Cleanup()

	s.logger.Info(ctx, "Server exited gracefully")
	return nil
}
