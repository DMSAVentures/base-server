package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"base-server/internal/clients/kafka"
	"base-server/internal/email"
	"base-server/internal/events/consumers"
	"base-server/internal/jobs/workers"
	"base-server/internal/observability"
	"base-server/internal/store"

	"github.com/joho/godotenv"
	"golang.org/x/sync/errgroup"
)

func main() {
	// Load environment variables
	if os.Getenv("GO_ENV") != "production" {
		if err := godotenv.Load("env.local"); err != nil {
			log.Printf("Warning: .env.local file not found: %v", err)
		}
	}

	// Initialize logger
	logger := observability.NewLogger()
	ctx := context.Background()

	logger.Info(ctx, "Starting Kafka event consumer server...")

	// Get Kafka configuration from environment
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "localhost:9092"
	}
	brokers := strings.Split(kafkaBrokers, ",")

	kafkaTopic := os.Getenv("KAFKA_TOPIC")
	if kafkaTopic == "" {
		kafkaTopic = "domain-events"
	}

	// Worker pool size
	workerCountStr := os.Getenv("KAFKA_WORKER_POOL_SIZE")
	workerCount := 10 // default
	if workerCountStr != "" {
		if parsed, err := strconv.Atoi(workerCountStr); err == nil && parsed > 0 {
			workerCount = parsed
		}
	}

	// Database configuration
	dbHost := os.Getenv("DB_HOST")
	dbUsername := os.Getenv("DB_USERNAME")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	if dbHost == "" || dbUsername == "" || dbPassword == "" || dbName == "" {
		log.Fatal("Database configuration not set")
	}

	// Create database connection string
	connectionString := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbUsername, dbPassword, dbName)

	// Initialize store
	dataStore, err := store.New(connectionString, logger)
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}

	// Initialize email service
	emailService := email.NewService(logger)

	// Initialize workers (shared across consumers)
	emailWorker := workers.NewEmailWorker(&dataStore, emailService, logger)
	positionWorker := workers.NewPositionWorker(&dataStore, logger)
	rewardWorker := workers.NewRewardWorker(&dataStore, emailService, nil, logger)

	// Create separate Kafka consumers for each concern
	// Each consumer group processes the same events independently (Kafka fan-out)

	// Email consumer: subscribes to user.*, referral.verified, reward.earned, campaign.*
	emailKafkaConsumer := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: brokers,
		Topic:   kafkaTopic,
		GroupID: "email-workers",
	}, logger)
	emailConsumer := consumers.NewEmailConsumer(emailKafkaConsumer, emailWorker, logger, workerCount)

	// Position consumer: subscribes to referral.verified
	// CRITICAL: Use 1 worker to prevent race conditions during position recalculation
	// Multiple workers processing the same campaign would cause read-modify-write conflicts
	positionKafkaConsumer := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: brokers,
		Topic:   kafkaTopic,
		GroupID: "position-workers",
	}, logger)
	positionConsumer := consumers.NewPositionConsumer(positionKafkaConsumer, positionWorker, logger, 1)

	// Reward consumer: subscribes to reward.earned
	rewardKafkaConsumer := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: brokers,
		Topic:   kafkaTopic,
		GroupID: "reward-workers",
	}, logger)
	rewardConsumer := consumers.NewRewardConsumer(rewardKafkaConsumer, rewardWorker, logger, workerCount)

	logger.Info(ctx, fmt.Sprintf(`Kafka event consumer server configuration:
  - Domain events topic: %s
  - Kafka brokers: %v
  - Consumer groups:
    * email-workers (%d workers): user.*, referral.verified, reward.earned, campaign.*
    * position-workers (1 worker): referral.verified [single-threaded to prevent race conditions]
    * reward-workers (%d workers): reward.earned`,
		kafkaTopic, brokers, workerCount, workerCount))

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start all consumers concurrently using errgroup
	g, gCtx := errgroup.WithContext(ctx)

	// Start email consumer
	g.Go(func() error {
		logger.Info(gCtx, "Starting email consumer...")
		if err := emailConsumer.Start(gCtx); err != nil && err != context.Canceled {
			logger.Error(gCtx, "Email consumer error", err)
			return err
		}
		return nil
	})

	// Start position consumer
	g.Go(func() error {
		logger.Info(gCtx, "Starting position consumer...")
		if err := positionConsumer.Start(gCtx); err != nil && err != context.Canceled {
			logger.Error(gCtx, "Position consumer error", err)
			return err
		}
		return nil
	})

	// Start reward consumer
	g.Go(func() error {
		logger.Info(gCtx, "Starting reward consumer...")
		if err := rewardConsumer.Start(gCtx); err != nil && err != context.Canceled {
			logger.Error(gCtx, "Reward consumer error", err)
			return err
		}
		return nil
	})

	logger.Info(ctx, "Kafka event consumer server started successfully")

	// Wait for shutdown signal or error
	go func() {
		<-sigChan
		logger.Info(ctx, "Received shutdown signal, stopping consumers...")
		cancel()
	}()

	// Wait for all consumers to finish
	if err := g.Wait(); err != nil {
		logger.Error(ctx, "Consumer group error", err)
	}

	// Stop all consumers
	logger.Info(ctx, "Stopping all consumers...")
	if err := emailConsumer.Stop(); err != nil {
		logger.Error(ctx, "Error stopping email consumer", err)
	}
	if err := positionConsumer.Stop(); err != nil {
		logger.Error(ctx, "Error stopping position consumer", err)
	}
	if err := rewardConsumer.Stop(); err != nil {
		logger.Error(ctx, "Error stopping reward consumer", err)
	}

	logger.Info(ctx, "Kafka event consumer server stopped")
}
