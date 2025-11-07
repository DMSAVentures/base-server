package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"base-server/internal/email"
	"base-server/internal/jobs"
	"base-server/internal/jobs/workers"
	"base-server/internal/observability"
	"base-server/internal/store"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
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

	logger.Info(ctx, "Starting background worker server...")

	// Get configuration from environment
	redisAddr := os.Getenv("REDIS_HOST")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

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
	jobClient := jobs.NewClient(redisAddr, logger)
	defer jobClient.Close()

	// Initialize workers
	emailWorker := workers.NewEmailWorker(&dataStore, emailService, logger)
	positionWorker := workers.NewPositionWorker(&dataStore, logger)
	rewardWorker := workers.NewRewardWorker(&dataStore, emailService, jobClient, logger)
	analyticsWorker := workers.NewAnalyticsWorker(&dataStore, logger)
	fraudWorker := workers.NewFraudWorker(&dataStore, logger)

	// Create Asynq server with queue configuration
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: 20, // Total number of concurrent workers
			Queues: map[string]int{
				jobs.QueueHigh:   10, // High priority queue gets 10 workers
				jobs.QueueMedium: 5,  // Medium priority queue gets 5 workers
				jobs.QueueLow:    2,  // Low priority queue gets 2 workers
			},
			// Error handler
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				logger.Error(ctx, fmt.Sprintf("task %s failed: %v", task.Type(), err), err)
			}),
			// Retry configuration
			RetryDelayFunc: asynq.DefaultRetryDelayFunc,
		},
	)

	// Create task handler (mux)
	mux := asynq.NewServeMux()

	// Register email handlers
	mux.HandleFunc(jobs.TypeEmailVerification, emailWorker.ProcessEmailTask)
	mux.HandleFunc(jobs.TypeEmailWelcome, emailWorker.ProcessEmailTask)
	mux.HandleFunc(jobs.TypeEmailPositionUpdate, emailWorker.ProcessEmailTask)
	mux.HandleFunc(jobs.TypeEmailRewardEarned, emailWorker.ProcessEmailTask)
	mux.HandleFunc(jobs.TypeEmailMilestone, emailWorker.ProcessEmailTask)

	// Register position recalculation handler
	mux.HandleFunc(jobs.TypePositionRecalculation, positionWorker.ProcessPositionRecalcTask)

	// Register reward delivery handler
	mux.HandleFunc(jobs.TypeRewardDelivery, rewardWorker.ProcessRewardDeliveryTask)

	// Register analytics aggregation handler
	mux.HandleFunc(jobs.TypeAnalyticsAggregation, analyticsWorker.ProcessAnalyticsAggregationTask)

	// Register fraud detection handler
	mux.HandleFunc(jobs.TypeFraudDetection, fraudWorker.ProcessFraudDetectionTask)

	// Setup periodic tasks for hourly analytics aggregation
	scheduler := asynq.NewScheduler(
		asynq.RedisClientOpt{Addr: redisAddr},
		&asynq.SchedulerOpts{
			Logger: &asynqLogger{logger: logger},
		},
	)

	// Schedule hourly analytics aggregation
	_, err = scheduler.Register("@hourly", asynq.NewTask(jobs.TypeAnalyticsAggregation, nil))
	if err != nil {
		logger.Error(ctx, "failed to register hourly analytics task", err)
	}

	// Start the scheduler
	if err := scheduler.Start(); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Shutdown()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start the server in a goroutine
	go func() {
		logger.Info(ctx, fmt.Sprintf("Worker server started on Redis: %s", redisAddr))
		if err := srv.Run(mux); err != nil {
			log.Fatalf("Failed to run server: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	logger.Info(ctx, "Shutting down worker server...")

	// Graceful shutdown
	srv.Shutdown()
	logger.Info(ctx, "Worker server stopped")
}

// asynqLogger adapts observability.Logger to asynq.Logger interface
type asynqLogger struct {
	logger *observability.Logger
}

func (l *asynqLogger) Debug(args ...interface{}) {
	l.logger.Info(context.Background(), fmt.Sprint(args...))
}

func (l *asynqLogger) Info(args ...interface{}) {
	l.logger.Info(context.Background(), fmt.Sprint(args...))
}

func (l *asynqLogger) Warn(args ...interface{}) {
	l.logger.Error(context.Background(), fmt.Sprint(args...), nil)
}

func (l *asynqLogger) Error(args ...interface{}) {
	l.logger.Error(context.Background(), fmt.Sprint(args...), nil)
}

func (l *asynqLogger) Fatal(args ...interface{}) {
	l.logger.Error(context.Background(), fmt.Sprint(args...), nil)
	os.Exit(1)
}
