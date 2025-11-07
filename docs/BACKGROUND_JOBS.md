# Background Job Workers

This document explains the background job processing system for the waitlist platform using Apache Kafka for durability and scalability.

## Architecture Overview

```
┌─────────────────┐     ┌──────────┐     ┌───────────────────┐
│  Application    │────▶│  Kafka   │────▶│ Job Consumer      │
│  (Enqueue Jobs) │     │  Broker  │     │  (Worker Pool)    │
└─────────────────┘     └──────────┘     └─────────┬─────────┘
                                                    │
                                                    ▼
                                         ┌──────────────────────┐
                                         │  Job Workers:        │
                                         │  - Email             │
                                         │  - Position Recalc   │
                                         │  - Reward Delivery   │
                                         │  - Analytics         │
                                         │  - Fraud Detection   │
                                         └──────────────────────┘
```

## Job Types

### 1. Email Jobs
Handles sending various types of emails to waitlist users:
- **Verification emails**: Email address verification
- **Welcome emails**: Welcome message after signup
- **Position update emails**: Notify users of position changes
- **Reward earned emails**: Notify users when they earn rewards
- **Milestone emails**: Campaign milestone notifications

**Event Type**: `job.email.<type>` (e.g., `job.email.verification`)

### 2. Position Recalculation Jobs
Recalculates waitlist positions based on referral activity.

**Event Type**: `job.position.recalculate`

**Algorithm**: `Position = Original Position - (Verified Referrals × Points Per Referral)`

### 3. Reward Delivery Jobs
Delivers rewards to users via email or webhook.

**Event Type**: `job.reward.deliver`

**Delivery Methods**:
- Email delivery with reward codes
- Webhook notifications to external systems

### 4. Fraud Detection Jobs
Runs fraud detection checks on user signups and referrals.

**Event Type**: `job.fraud.detect`

**Check Types**:
- Self-referral detection
- Velocity checks (too many referrals too quickly)
- Fake email detection
- Bot detection
- Duplicate IP address detection
- Duplicate device fingerprint detection

### 5. Analytics Aggregation Jobs
Aggregates campaign metrics for time-series analysis.

**Event Type**: `job.analytics.aggregate`

**Metrics Collected**:
- New signups count
- Email verification count
- Referral count
- Emails sent count
- Rewards delivered count

## Environment Variables

Add these to your `env.local` file:

```bash
# Kafka Configuration
KAFKA_BROKERS=localhost:9092                  # Comma-separated list of Kafka brokers
KAFKA_JOB_TOPIC=job-events                    # Topic name for job events (optional, default: job-events)
KAFKA_JOB_CONSUMER_GROUP=job-workers          # Consumer group ID (optional, default: job-workers)
```

### For AWS MSK (Managed Streaming for Apache Kafka)

```bash
# AWS MSK Example
KAFKA_BROKERS=b-1.mycluster.abc123.kafka.us-east-1.amazonaws.com:9092,b-2.mycluster.abc123.kafka.us-east-1.amazonaws.com:9092
KAFKA_JOB_TOPIC=job-events
KAFKA_JOB_CONSUMER_GROUP=job-workers-prod
```

## Usage

### Enqueuing Jobs from Application Code

```go
import (
	"base-server/internal/clients/kafka"
	"base-server/internal/jobs"
	"base-server/internal/jobs/producer"
)

// Initialize Kafka producer
kafkaProducer := kafka.NewProducer(kafka.ProducerConfig{
	Brokers: []string{"localhost:9092"},
	Topic:   "job-events",
}, logger)

// Initialize job producer
jobProducer := producer.New(kafkaProducer, logger)

// Enqueue an email job
err := jobProducer.EnqueueEmailJob(ctx, jobs.EmailJobPayload{
	Type:       "verification",
	CampaignID: campaignID,
	UserID:     userID,
	Priority:   1,
})

// Enqueue a position recalculation job
err := jobProducer.EnqueuePositionRecalcJob(ctx, jobs.PositionRecalcJobPayload{
	CampaignID:        campaignID,
	PointsPerReferral: 1,
})

// Enqueue a fraud detection job
err := jobProducer.EnqueueFraudDetectionJob(ctx, jobs.FraudDetectionJobPayload{
	CampaignID: campaignID,
	UserID:     userID,
})

// Enqueue an analytics aggregation job
err := jobProducer.EnqueueAnalyticsAggregationJob(ctx, jobs.AnalyticsAggregationJobPayload{
	CampaignID:  campaignID,
	Granularity: "hourly",
	StartTime:   startTime,
	EndTime:     endTime,
})
```

### Running the Worker Server

```bash
# Start the Kafka worker server
go run cmd/kafka-worker/main.go
```

The worker server will:
1. Connect to Kafka brokers
2. Start a worker pool (default: 10 concurrent workers)
3. Consume job events from the `job-events` topic
4. Route events to the appropriate worker based on event type
5. Process jobs concurrently

## Benefits

### Durability
- **Persistent Storage**: Jobs are persisted to Kafka before processing
- **No Job Loss**: Jobs survive application crashes
- **Replay Capability**: Can reprocess jobs from any point in time

### Scalability
- **Horizontal Scaling**: Add more worker instances as needed
- **Worker Pool**: Each instance processes jobs concurrently (default: 10 workers)
- **Kafka Partitions**: Enable parallel processing across workers

### Reliability
- **At-Least-Once Delivery**: Kafka guarantees message delivery
- **Manual Offset Commit**: Only commit after successful processing
- **Error Handling**: Failed jobs can be retried or sent to DLQ

### Decoupling
- **Async Processing**: Application doesn't wait for job completion
- **Fast Response Times**: Jobs are queued and processed in background
- **Independent Scaling**: Scale workers independently from application

## Event Schema

Jobs published to Kafka follow this schema:

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "job.email.verification",
  "account_id": "campaign-uuid",
  "campaign_id": "campaign-uuid",
  "data": {
    "type": "verification",
    "campaign_id": "campaign-uuid",
    "user_id": "user-uuid",
    "template_id": "template-uuid",
    "template_data": {},
    "priority": 1
  },
  "timestamp": "2025-01-01T12:00:00Z"
}
```

## Worker Pool Architecture

The job consumer uses a worker pool pattern:

```go
// Create consumer with worker pool
jobConsumer := consumer.New(
	kafkaConsumer,
	emailWorker,
	positionWorker,
	rewardWorker,
	analyticsWorker,
	fraudWorker,
	logger,
	10, // Number of concurrent workers
)

// Start processing
jobConsumer.Start(ctx)
```

**Flow**:
1. Kafka consumer fetches messages from topic
2. Messages are sent to a buffered channel
3. Worker pool (10 goroutines) consumes from channel
4. Each worker processes job and routes to appropriate handler
5. Offset is committed only after successful processing

## Local Development

### Using Docker Compose

The Kafka service is available in `docker-compose.services.yml`:

```bash
# Start all services including Kafka
docker-compose -f docker-compose.services.yml up -d

# View Kafka UI (if included)
open http://localhost:8080
```

### Create Kafka Topic

```bash
# Connect to Kafka container
docker exec -it <kafka-container-id> bash

# Create job-events topic
kafka-topics --create \
  --bootstrap-server localhost:9092 \
  --topic job-events \
  --partitions 10 \
  --replication-factor 1
```

## Production Deployment

### AWS MSK Setup

1. **Create MSK Cluster**
   ```bash
   aws kafka create-cluster \
     --cluster-name job-events-cluster \
     --kafka-version 2.8.1 \
     --number-of-broker-nodes 3 \
     --broker-node-group-info file://broker-config.json
   ```

2. **Create Topic**
   ```bash
   kafka-topics --create \
     --bootstrap-server $KAFKA_BROKERS \
     --topic job-events \
     --partitions 10 \
     --replication-factor 2
   ```

3. **Deploy Worker Server**
   ```bash
   # Build worker binary
   go build -o kafka-worker cmd/kafka-worker/main.go

   # Run with environment variables
   KAFKA_BROKERS=$MSK_BROKERS \
   KAFKA_JOB_TOPIC=job-events \
   KAFKA_JOB_CONSUMER_GROUP=job-workers-prod \
   ./kafka-worker
   ```

### Scaling Considerations

- **Partitions**: 10-20 partitions per topic for high throughput
- **Replication**: 2-3 replicas for fault tolerance
- **Worker Instances**: Scale based on load (1-10+ instances)
- **Worker Pool Size**: 10-20 workers per instance
- **Consumer Group**: All instances share same consumer group

## Monitoring

### Kafka Metrics
- **Consumer Lag**: Monitor how far behind consumers are
  ```bash
  kafka-consumer-groups --bootstrap-server $KAFKA_BROKERS \
    --group job-workers --describe
  ```
- **Partition Count**: Ensure even distribution
- **Throughput**: Messages per second

### Application Metrics
- **Jobs Enqueued**: Count of jobs published to Kafka
- **Jobs Processed**: Count of jobs successfully completed
- **Job Failures**: Count and types of failures
- **Processing Time**: Average time per job type

### Logging
- All job processing is logged with structured fields
- Context includes: job_type, campaign_id, user_id, worker_id
- Use observability middleware for request tracing

## Troubleshooting

### Jobs Not Being Processed

1. Check consumer lag:
   ```bash
   kafka-consumer-groups --bootstrap-server $KAFKA_BROKERS \
     --group job-workers --describe
   ```

2. Verify topic exists:
   ```bash
   kafka-topics --list --bootstrap-server $KAFKA_BROKERS
   ```

3. Check worker logs for errors

### Slow Processing

1. Increase worker pool size:
   ```go
   workerCount := 20 // Increase from default 10
   ```

2. Scale horizontally (add more worker instances)

3. Optimize job processing logic

### Job Failures

1. Check application logs for error details
2. Verify database connectivity
3. Check email service configuration
4. Review Kafka offset commits

## Migration from Asynq/Redis

Both Asynq and Kafka workers are supported concurrently:

1. **Existing Asynq workers** continue to work via `cmd/worker/main.go`
2. **New Kafka workers** can be started via `cmd/kafka-worker/main.go`
3. **Gradual migration**: Switch to Kafka enqueueing at your own pace
4. **No data loss** during transition

## References

- [Apache Kafka Documentation](https://kafka.apache.org/documentation/)
- [AWS MSK Documentation](https://docs.aws.amazon.com/msk/)
- [segmentio/kafka-go](https://github.com/segmentio/kafka-go)
- [Webhook Delivery with Kafka](./KAFKA_SETUP.md)
