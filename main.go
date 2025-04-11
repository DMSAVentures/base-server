package main

import (
	aiHandler "base-server/internal/ai-capabilities/handler"
	AICapabilities "base-server/internal/ai-capabilities/processor"
	"base-server/internal/api"
	"base-server/internal/auth/handler"
	"base-server/internal/auth/processor"
	"base-server/internal/clients/googleoauth"
	"base-server/internal/clients/mail"
	"base-server/internal/email"
	billingHandler "base-server/internal/money/billing/handler"
	billingProcessor "base-server/internal/money/billing/processor"
	"base-server/internal/money/products"
	"base-server/internal/money/subscriptions"
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

var ErrEmptyEnvironmentVariable = errors.New("empty environment variable")

func main() {
	logger := observability.NewLogger()
	ctx := context.Background()
	ctx = observability.WithFields(ctx, observability.Field{Key: "service-name", Value: "base-server"})

	if os.Getenv("GO_ENV") != "production" {
		// Load the .env file
		err := godotenv.Load("env.local")
		if err != nil {
			logger.Fatal(ctx, "Error loading .env file", err)
		}
	}

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

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		logger.Error(ctx, "JWT_SECRET is not set", ErrEmptyEnvironmentVariable)
		os.Exit(1)

	}

	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	if googleClientID == "" {
		logger.Error(ctx, "GOOGLE_CLIENT_ID is not set", ErrEmptyEnvironmentVariable)
		os.Exit(1)

	}

	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if googleClientSecret == "" {
		logger.Error(ctx, "GOOGLE_CLIENT_SECRET is not set", ErrEmptyEnvironmentVariable)
		os.Exit(1)
	}

	googleRedirectURL := os.Getenv("GOOGLE_REDIRECT_URI")
	if googleRedirectURL == "" {
		logger.Error(ctx, "GOOGLE_REDIRECT_URI is not set", ErrEmptyEnvironmentVariable)
		os.Exit(1)
	}

	webAppURL := os.Getenv("WEBAPP_URI")
	if webAppURL == "" {
		logger.Error(ctx, "WEBAPP_URI is not set", ErrEmptyEnvironmentVariable)
		os.Exit(1)
	}

	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		logger.Error(ctx, "SERVER_PORT is not set", ErrEmptyEnvironmentVariable)
		os.Exit(1)
	}

	parsedServerPort, err := strconv.Atoi(serverPort)
	if err != nil {
		logger.Error(ctx, "failed to parse SERVER_PORT", err)
		os.Exit(1)
	}

	stripeSecretKey := os.Getenv("STRIPE_SECRET_KEY")
	if stripeSecretKey == "" {
		logger.Error(ctx, "STRIPE_SECRET_KEY is not set", ErrEmptyEnvironmentVariable)
		os.Exit(1)
	}

	// Get your Stripe webhook signing secret from environment or config
	webhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if webhookSecret == "" {
		logger.Error(ctx, "STRIPE_WEBHOOK_SECRET is not set", ErrEmptyEnvironmentVariable)
		os.Exit(1)
	}

	// Get Resend API key for sending emails
	resendAPIKey := os.Getenv("RESEND_API_KEY")
	if resendAPIKey == "" {
		logger.Error(ctx, "RESEND_API_KEY is not set", ErrEmptyEnvironmentVariable)
		os.Exit(1)
	}

	// Get default email sender
	defaultEmailSender := os.Getenv("DEFAULT_EMAIL_SENDER_ADDRESS")
	if defaultEmailSender == "" {
		logger.Error(ctx, "DEFAULT_EMAIL_SENDER_ADDRESS is not set", ErrEmptyEnvironmentVariable)
		os.Exit(1)
	}

	// Get Gemini API Key from environment
	geminiAPIKey := os.Getenv("GOOGLE_AI_API_KEY")
	if geminiAPIKey == "" {
		logger.Error(ctx, "GOOGLE_AI_API_KEY is not set", ErrEmptyEnvironmentVariable)
		os.Exit(1)
	}

	// Init open ai
	openAiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openAiAPIKey == "" {
		logger.Error(ctx, "OPENAI_API_KEY is not set", ErrEmptyEnvironmentVariable)
		os.Exit(1)
	}

	googleOauthClient := googleoauth.NewClient(googleClientID, googleClientSecret, googleRedirectURL, logger)

	connectionString := "postgres://" + dbUsername + ":" + dbPassword + "@" + dbHost + "/" + dbName
	store, err := store.New(connectionString, logger)
	if err != nil {
		logger.Error(ctx, "failed to connect to database", err)
	}

	// Initialize the Resend email client
	mailClient, err := mail.NewResendClient(resendAPIKey, logger)
	if err != nil {
		logger.Error(ctx, "failed to create resend client", err)
		os.Exit(1)
	}

	// Initialize the email service
	emailService := email.New(mailClient, defaultEmailSender, logger)

	r := gin.New()
	config := cors.DefaultConfig()
	config.AllowCredentials = true
	config.AllowMethods = []string{"GET", "POST", "OPTIONS", "DELETE"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Last-Event-ID", "Cache-Control", "Connection", "Accept", "Transfer-Encoding"}
	// For production, specify allowed origins instead of AllowAllOrigins
	config.AllowOrigins = []string{webAppURL}

	if os.Getenv("GO_ENV") != "production" {
		// Allow both localhost for the frontend and Google's OAuth callback
		config.AllowOrigins = []string{"http://localhost:3000", "https://accounts.google.com"}
	}

	r.Use(cors.New(config))
	r.Use(observability.Middleware(logger))
	rootRouter := r.Group("/")

	productService := products.New(stripeSecretKey, store, logger)
	subscriptionService := subscriptions.New(logger, stripeSecretKey, store)

	billingProcessor := billingProcessor.New(stripeSecretKey, webhookSecret, webAppURL, store, productService,
		subscriptionService, emailService, logger)
	billingHandler := billingHandler.New(billingProcessor, logger)

	authConfig := processor.AuthConfig{
		Email: processor.EmailConfig{
			JWTSecret: jwtSecret,
		},
		Google: processor.GoogleOauthConfig{
			ClientID:          googleClientID,
			ClientSecret:      googleClientSecret,
			ClientRedirectURL: googleRedirectURL,
			WebAppHost:        webAppURL,
		},
	}
	authProcessor := processor.New(store, authConfig, googleOauthClient, billingProcessor, *emailService, logger)
	authHandler := handler.New(authProcessor, logger)

	aiCapability := AICapabilities.New(logger, geminiAPIKey, openAiAPIKey, store)

	aiHandler := aiHandler.New(aiCapability, logger)

	api := api.New(rootRouter, authHandler, billingHandler, aiHandler)
	api.RegisterRoutes()

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", parsedServerPort),
		Handler: r,
	}
	// Run the server in a goroutine so that it doesn't block
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal(ctx, "Server forced to shutdown:", err)
	}

	logger.Info(ctx, "Server exiting")
}
