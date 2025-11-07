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
	"time"

	"base-server/internal/clients/kafka"
	"base-server/internal/email"
	"base-server/internal/jobs/consumer"
	"base-server/internal/jobs/producer"
	"base-server/internal/jobs/scheduler"
	scheduledJobs "base-server/internal/jobs/scheduler/jobs"
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

	logger.Info(ctx, "Starting Kafka background job worker server...")

	// Get configuration from environment
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "localhost:9092"
	}
	brokers := strings.Split(kafkaBrokers, ",")

	kafkaTopic := os.Getenv("KAFKA_JOB_TOPIC")
	if kafkaTopic == "" {
		kafkaTopic = "job-events"
	}

	kafkaConsumerGroup := os.Getenv("KAFKA_JOB_CONSUMER_GROUP")
	if kafkaConsumerGroup == "" {
		kafkaConsumerGroup = "job-workers"
	}

	// Worker pool size
	workerCountStr := os.Getenv("KAFKA_WORKER_POOL_SIZE")
	workerCount := 10 // default
	if workerCountStr != "" {
		if parsed, err := strconv.Atoi(workerCountStr); err == nil && parsed > 0 {
			workerCount = parsed
		}
	}

	// Scheduler intervals
	analyticsInterval := parseInterval(os.Getenv("ANALYTICS_INTERVAL"), 1*time.Hour)
	fraudInterval := parseInterval(os.Getenv("FRAUD_DETECTION_INTERVAL"), 15*time.Minute)

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

	// Initialize Kafka producer
	kafkaProducer := kafka.NewProducer(kafka.ProducerConfig{
		Brokers: brokers,
		Topic:   kafkaTopic,
	}, logger)
	defer kafkaProducer.Close()

	// Initialize job producer
	jobProducer := producer.New(kafkaProducer, logger)
	_ = jobProducer // Will be used by workers in the future

	// Initialize Kafka consumer
	kafkaConsumer := kafka.NewConsumer(kafka.ConsumerConfig{
		Brokers: brokers,
		Topic:   kafkaTopic,
		GroupID: kafkaConsumerGroup,
	}, logger)
	defer kafkaConsumer.Close()

	// Initialize workers
	emailWorker := workers.NewEmailWorker(&dataStore, emailService, logger)
	positionWorker := workers.NewPositionWorker(&dataStore, logger)
	rewardWorker := workers.NewRewardWorker(&dataStore, emailService, nil, logger)
	analyticsWorker := workers.NewAnalyticsWorker(&dataStore, logger)
	fraudWorker := workers.NewFraudWorker(&dataStore, logger)

	// Initialize event-driven job consumer (emails, position recalc, rewards)
	jobConsumer := consumer.New(
		kafkaConsumer,
		emailWorker,
		positionWorker,
		rewardWorker,
		logger,
		workerCount,
	)

	// Initialize scheduler for cron-based jobs
	jobScheduler := scheduler.New(logger)

	// Register analytics aggregation job (runs hourly by default)
	jobScheduler.Register(scheduledJobs.NewAnalyticsAggregationJob(
		&dataStore,
		logger,
		analyticsInterval,
	))

	// Register fraud detection job (runs every 15 minutes by default)
	jobScheduler.Register(scheduledJobs.NewFraudDetectionJob(
		&dataStore,
		fraudWorker,
		logger,
		fraudInterval,
	))

	logger.Info(ctx, fmt.Sprintf(`Kafka job worker server configuration:
  - Event-driven jobs: %d concurrent workers
  - Kafka brokers: %v
  - Kafka topic: %s
  - Consumer group: %s
  - Analytics aggregation: every %s
  - Fraud detection: every %s`,
		workerCount, brokers, kafkaTopic, kafkaConsumerGroup,
		analyticsInterval, fraudInterval))

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start both event-driven consumer and scheduler using errgroup
	g, gCtx := errgroup.WithContext(ctx)

	// Start event-driven job consumer
	g.Go(func() error {
		logger.Info(gCtx, "Starting event-driven job consumer...")
		return jobConsumer.Start(gCtx)
	})

	// Start scheduler for cron-based jobs
	g.Go(func() error {
		logger.Info(gCtx, "Starting scheduled job scheduler...")
		return jobScheduler.Start(gCtx)
	})

	logger.Info(ctx, "Kafka job worker server started successfully")

	// Wait for shutdown signal
	select {
	case <-sigChan:
		logger.Info(ctx, "Received shutdown signal, stopping workers...")
	case <-gCtx.Done():
		logger.Info(ctx, "Context cancelled")
	}

	cancel()

	// Wait for all goroutines to finish
	if err := g.Wait(); err != nil && err != context.Canceled {
		logger.Error(ctx, "Error during shutdown", err)
	}

	// Stop consumer
	if err := jobConsumer.Stop(); err != nil {
		logger.Error(ctx, "Error stopping job consumer", err)
	}

	logger.Info(ctx, "Kafka job worker server stopped")
}

// parseInterval parses a duration string with fallback to default
func parseInterval(intervalStr string, defaultInterval time.Duration) time.Duration {
	if intervalStr == "" {
		return defaultInterval
	}

	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		log.Printf("Warning: invalid interval '%s', using default %s", intervalStr, defaultInterval)
		return defaultInterval
	}

	return interval
}
