package main

import (
	"base-server/internal/api"
	"base-server/internal/auth/handler"
	"base-server/internal/auth/processor"
	"base-server/internal/store"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	store, err := store.New()
	if err != nil {
		log.Fatalf("failed to initialize the database: %s", err)
	}
	authProcessor := processor.New(store)
	authHandler := handler.New(authProcessor)

	r := gin.Default()
	api := api.New(r.Group("/"), authHandler)
	api.Handler()

	srv := &http.Server{
		Addr:    "localhost:3000",
		Handler: r,
	}
	// Run the server in a goroutine so that it doesn't block
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
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
	log.Println("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting")
}
