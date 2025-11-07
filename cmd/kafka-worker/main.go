package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"base-server/internal/email"
	"base-server/internal/jobs"
	"base-server/internal/jobs/workers"
	"base-server/internal/kafka"
	"base-server/internal/observability"
	"base-server/internal/store"

	"github.com/joho/godotenv"
	kafkago "github.com/segmentio/kafka-go"
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

	logger.Info(ctx, "Starting Kafka background worker server...")

	// Get configuration from environment
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "localhost:9092"
	}
	brokers := strings.Split(kafkaBrokers, ",")

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

	// Initialize job client for workers that need to enqueue other jobs
	jobClient := jobs.NewKafkaClient(brokers, logger)
	defer jobClient.Close()

	// Initialize dead letter queue producer
	dlqProducer := kafka.NewProducer(kafka.ProducerConfig{
		Brokers:      brokers,
		Topic:        kafka.TopicDeadLetter,
		Compression:  "snappy",
		BatchSize:    100,
		BatchTimeout: 10,
		RequiredAcks: -1,
	}, logger)
	defer dlqProducer.Close()

	// Initialize workers
	emailWorker := workers.NewEmailWorker(&dataStore, emailService, logger)
	positionWorker := workers.NewPositionWorker(&dataStore, logger)
	rewardWorker := workers.NewRewardWorker(&dataStore, emailService, jobClient, logger)
	analyticsWorker := workers.NewAnalyticsWorker(&dataStore, logger)
	fraudWorker := workers.NewFraudWorker(&dataStore, logger)

	// Create consumers for each topic
	consumers := []*kafka.Consumer{
		// Email consumers
		createEmailConsumer(brokers, kafka.TopicEmailVerification, emailWorker, dlqProducer, logger),
		createEmailConsumer(brokers, kafka.TopicEmailWelcome, emailWorker, dlqProducer, logger),
		createEmailConsumer(brokers, kafka.TopicEmailPositionUpdate, emailWorker, dlqProducer, logger),
		createEmailConsumer(brokers, kafka.TopicEmailRewardEarned, emailWorker, dlqProducer, logger),
		createEmailConsumer(brokers, kafka.TopicEmailMilestone, emailWorker, dlqProducer, logger),

		// Position recalculation consumer
		kafka.NewConsumer(
			kafka.ConsumerConfig{
				Brokers:       brokers,
				Topic:         kafka.TopicPositionRecalc,
				GroupID:       kafka.ConsumerGroupPositionWorkers,
				MaxWait:       5 * time.Second,
				StartOffset:   kafkago.LastOffset,
				RetentionTime: 24 * time.Hour,
			},
			createPositionHandler(positionWorker),
			dlqProducer,
			logger,
		),

		// Reward delivery consumer
		kafka.NewConsumer(
			kafka.ConsumerConfig{
				Brokers:       brokers,
				Topic:         kafka.TopicRewardDelivery,
				GroupID:       kafka.ConsumerGroupRewardWorkers,
				MaxWait:       5 * time.Second,
				StartOffset:   kafkago.LastOffset,
				RetentionTime: 24 * time.Hour,
			},
			createRewardHandler(rewardWorker),
			dlqProducer,
			logger,
		),

		// Analytics aggregation consumer
		kafka.NewConsumer(
			kafka.ConsumerConfig{
				Brokers:       brokers,
				Topic:         kafka.TopicAnalyticsAggregation,
				GroupID:       kafka.ConsumerGroupAnalyticsWorkers,
				MaxWait:       10 * time.Second,
				StartOffset:   kafkago.LastOffset,
				RetentionTime: 24 * time.Hour,
			},
			createAnalyticsHandler(analyticsWorker),
			dlqProducer,
			logger,
		),

		// Fraud detection consumer
		kafka.NewConsumer(
			kafka.ConsumerConfig{
				Brokers:       brokers,
				Topic:         kafka.TopicFraudDetection,
				GroupID:       kafka.ConsumerGroupFraudWorkers,
				MaxWait:       5 * time.Second,
				StartOffset:   kafkago.LastOffset,
				RetentionTime: 24 * time.Hour,
			},
			createFraudHandler(fraudWorker),
			dlqProducer,
			logger,
		),
	}

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start all consumers in an error group
	g, gCtx := errgroup.WithContext(ctx)

	for _, consumer := range consumers {
		c := consumer // Capture for goroutine
		g.Go(func() error {
			return c.Start(gCtx)
		})
	}

	logger.Info(ctx, fmt.Sprintf("Kafka worker server started with %d consumers on brokers: %v", len(consumers), brokers))

	// Setup periodic analytics aggregation (every hour)
	g.Go(func() error {
		return runPeriodicAnalytics(gCtx, jobClient, logger)
	})

	// Wait for shutdown signal
	select {
	case <-sigChan:
		logger.Info(ctx, "Received shutdown signal")
		cancel()
	case <-gCtx.Done():
		logger.Info(ctx, "Context cancelled")
	}

	// Wait for all consumers to finish
	if err := g.Wait(); err != nil && err != context.Canceled {
		logger.Error(ctx, "Error during shutdown", err)
	}

	// Close all consumers
	for _, consumer := range consumers {
		if err := consumer.Close(); err != nil {
			logger.Error(ctx, "failed to close consumer", err)
		}
	}

	logger.Info(ctx, "Kafka worker server stopped")
}

// createEmailConsumer creates a consumer for email topics
func createEmailConsumer(brokers []string, topic string, worker *workers.EmailWorker, dlqProducer *kafka.Producer, logger *observability.Logger) *kafka.Consumer {
	return kafka.NewConsumer(
		kafka.ConsumerConfig{
			Brokers:       brokers,
			Topic:         topic,
			GroupID:       kafka.ConsumerGroupEmailWorkers,
			MaxWait:       5 * time.Second,
			StartOffset:   kafkago.LastOffset,
			RetentionTime: 24 * time.Hour,
		},
		createEmailHandler(worker),
		dlqProducer,
		logger,
	)
}

// createEmailHandler creates a message handler for email jobs
func createEmailHandler(worker *workers.EmailWorker) kafka.MessageHandler {
	return func(ctx context.Context, message kafkago.Message) error {
		var payload jobs.EmailJobPayload
		if err := kafka.UnmarshalMessage(message, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal email job payload: %w", err)
		}

		// The existing EmailWorker expects an asynq.Task, but we can adapt it
		// For now, we'll process the payload directly
		return worker.ProcessEmailTask(ctx, message)
	}
}

// createPositionHandler creates a message handler for position recalculation jobs
func createPositionHandler(worker *workers.PositionWorker) kafka.MessageHandler {
	return func(ctx context.Context, message kafkago.Message) error {
		return worker.ProcessPositionRecalcTask(ctx, message)
	}
}

// createRewardHandler creates a message handler for reward delivery jobs
func createRewardHandler(worker *workers.RewardWorker) kafka.MessageHandler {
	return func(ctx context.Context, message kafkago.Message) error {
		return worker.ProcessRewardDeliveryTask(ctx, message)
	}
}

// createAnalyticsHandler creates a message handler for analytics aggregation jobs
func createAnalyticsHandler(worker *workers.AnalyticsWorker) kafka.MessageHandler {
	return func(ctx context.Context, message kafkago.Message) error {
		return worker.ProcessAnalyticsAggregationTask(ctx, message)
	}
}

// createFraudHandler creates a message handler for fraud detection jobs
func createFraudHandler(worker *workers.FraudWorker) kafka.MessageHandler {
	return func(ctx context.Context, message kafkago.Message) error {
		return worker.ProcessFraudDetectionTask(ctx, message)
	}
}

// runPeriodicAnalytics runs analytics aggregation every hour
func runPeriodicAnalytics(ctx context.Context, jobClient *jobs.KafkaClient, logger *observability.Logger) error {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// This would normally fetch all active campaigns and enqueue analytics jobs
			// For now, just log
			logger.Info(ctx, "Triggering hourly analytics aggregation")
			// TODO: Implement campaign fetching and job enqueueing
		}
	}
}
