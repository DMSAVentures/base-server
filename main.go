package main

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

var ErrEmptyEnvironmentVariable = errors.New("empty environment variable")

func main() {
	logger := observability.NewLogger()
	ctx := context.Background()
	ctx = observability.WithFields(ctx, observability.Field{Key: "service-name", Value: "base-server"})

	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		logger.Error(ctx, "DB_HOST is not set", ErrEmptyEnvironmentVariable)
		os.Exit(1)
	}

	dbUsername := os.Getenv("DB_USERNAME")
	if dbUsername == "" {
		logger.Error(ctx, "DB_USERNAME is not set", ErrEmptyEnvironmentVariable)
		os.Exit(1)
	}

	if dbHost == "" {
		logger.Error(ctx, "DB_HOST is not set", ErrEmptyEnvironmentVariable)
		os.Exit(1)
	}

	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		logger.Error(ctx, "DB_PASSWORD is not set", ErrEmptyEnvironmentVariable)
		os.Exit(1)
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		logger.Error(ctx, "DB_NAME is not set", ErrEmptyEnvironmentVariable)
		os.Exit(1)

	}

	connectionString := "postgres://" + dbUsername + ":" + dbPassword + "@" + dbHost + ":5432/" + dbName
	_, err := store.New(connectionString)
	if err != nil {
		logger.Error(ctx, "failed to connect to database", err)
	}
	//authProcessor := processor.New(store)
	//authHandler := handler.New(authProcessor)

	//r := gin.Default()
	//api := api.New(r.Group("/"), authHandler)
	//api.Handler()
	r := gin.Default()
	apiGroup := r.Group("/api")
	apiGroup.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message":     "pongingtest",
			"db_endpoint": dbHost,
			"dbUsername":  dbUsername,
		})
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})
	srv := &http.Server{
		Addr:    ":80",
		Handler: r,
	}
	// Run the server in a goroutine so that it doesn't block
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(ctx, "server failed to start", err)
			os.Exit(1)
		}
	}()

	// Set up a channel to listen for OS signals for shutdown
	quit := make(chan os.Signal, 1)
	// kill (no param) default sends syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be caught, so don't need to add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Block until a signal is received
	<-quit
	logger.Info(ctx, "Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal(ctx, "Server forced to shutdown:", err)
	}

	logger.Info(ctx, "Server exiting")
}
