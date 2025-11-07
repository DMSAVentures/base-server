package scheduler

import (
	"base-server/internal/observability"
	"context"
	"fmt"
	"time"
)

// Job represents a scheduled job
type Job interface {
	// Name returns the job name for logging
	Name() string
	// Run executes the job
	Run(ctx context.Context) error
	// Schedule returns the interval between runs
	Schedule() time.Duration
}

// Scheduler manages scheduled jobs
type Scheduler struct {
	jobs   []Job
	logger *observability.Logger
}

// New creates a new scheduler
func New(logger *observability.Logger) *Scheduler {
	return &Scheduler{
		jobs:   make([]Job, 0),
		logger: logger,
	}
}

// Register adds a job to the scheduler
func (s *Scheduler) Register(job Job) {
	s.jobs = append(s.jobs, job)
	s.logger.Info(context.Background(), fmt.Sprintf("Registered scheduled job: %s (interval: %s)",
		job.Name(), job.Schedule()))
}

// Start begins running all scheduled jobs
func (s *Scheduler) Start(ctx context.Context) error {
	s.logger.Info(ctx, fmt.Sprintf("Starting scheduler with %d jobs", len(s.jobs)))

	// Start each job in its own goroutine
	for _, job := range s.jobs {
		go s.runJob(ctx, job)
	}

	// Wait for context cancellation
	<-ctx.Done()
	s.logger.Info(ctx, "Scheduler stopped")
	return ctx.Err()
}

// runJob runs a single job on its schedule
func (s *Scheduler) runJob(ctx context.Context, job Job) {
	jobCtx := observability.WithFields(ctx, observability.Field{Key: "scheduled_job", Value: job.Name()})

	s.logger.Info(jobCtx, fmt.Sprintf("Starting scheduled job: %s", job.Name()))

	// Run immediately on startup
	if err := s.executeJob(jobCtx, job); err != nil {
		s.logger.Error(jobCtx, fmt.Sprintf("Failed to execute job %s on startup", job.Name()), err)
	}

	// Create ticker for scheduled runs
	ticker := time.NewTicker(job.Schedule())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info(jobCtx, fmt.Sprintf("Stopping scheduled job: %s", job.Name()))
			return
		case <-ticker.C:
			if err := s.executeJob(jobCtx, job); err != nil {
				s.logger.Error(jobCtx, fmt.Sprintf("Failed to execute job %s", job.Name()), err)
			}
		}
	}
}

// executeJob executes a job and logs timing
func (s *Scheduler) executeJob(ctx context.Context, job Job) error {
	start := time.Now()
	s.logger.Info(ctx, fmt.Sprintf("Executing scheduled job: %s", job.Name()))

	err := job.Run(ctx)
	duration := time.Since(start)

	if err != nil {
		s.logger.Error(ctx, fmt.Sprintf("Job %s failed after %v", job.Name(), duration), err)
		return err
	}

	s.logger.Info(ctx, fmt.Sprintf("Job %s completed successfully in %v", job.Name(), duration))
	return nil
}
