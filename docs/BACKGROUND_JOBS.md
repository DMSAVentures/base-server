# Background Job Workers

This document explains the event-driven background job processing system using Apache Kafka, following the same pattern as webhook delivery.

## Architecture Overview

```
User Action ──▶ App Enqueues Event ──▶ Kafka ──▶ Worker Pool ──▶ Process Job
  Signup           job.email.verification    (10 workers)      Send Email
  Referral         job.position.recalculate  (10 workers)      Update Positions
  Reward Earned    job.reward.deliver        (10 workers)      Deliver Reward
```

**Pattern**: Just like webhooks, jobs are:
1. Published to Kafka topic
2. Consumed by worker pool
3. Processed concurrently
4. Durable and replayable

## Job Types

### 1. Email Jobs
**Trigger**: User actions (signup, verification, referral, etc.)
**Pattern**: Event → Kafka → Worker → Send Email

Types:
- `job.email.verification` - Email address verification
- `job.email.welcome` - Welcome message after signup
- `job.email.position_update` - Position change notifications
- `job.email.reward_earned` - Reward earned notifications
- `job.email.milestone` - Campaign milestone emails

**Usage**:
```go
jobProducer.EnqueueEmailJob(ctx, jobs.EmailJobPayload{
	Type:       "verification",
	CampaignID: campaignID,
	UserID:     userID,
})
```

### 2. Position Recalculation
**Trigger**: Referral verification completed
**Pattern**: Event → Kafka → Worker → Recalculate

**Algorithm**: `Position = Original Position - (Verified Referrals × Points Per Referral)`

**Usage**:
```go
jobProducer.EnqueuePositionRecalcJob(ctx, jobs.PositionRecalcJobPayload{
	CampaignID:        campaignID,
	PointsPerReferral: 1,
})
```

### 3. Reward Delivery
**Trigger**: User earns a reward
**Pattern**: Event → Kafka → Worker → Deliver

**Usage**:
```go
jobProducer.EnqueueRewardDeliveryJob(ctx, jobs.RewardDeliveryJobPayload{
	UserRewardID: userRewardID,
})
```

## Environment Variables

```bash
# Kafka Configuration
KAFKA_BROKERS=localhost:9092                   # Comma-separated broker list
KAFKA_JOB_TOPIC=job-events                     # Topic name (default: job-events)
KAFKA_JOB_CONSUMER_GROUP=job-workers           # Consumer group (default: job-workers)
KAFKA_WORKER_POOL_SIZE=10                      # Concurrent workers (default: 10)
```

### AWS MSK Example

```bash
KAFKA_BROKERS=b-1.cluster.kafka.us-east-1.amazonaws.com:9092,b-2.cluster.kafka.us-east-1.amazonaws.com:9092
KAFKA_JOB_TOPIC=job-events
KAFKA_JOB_CONSUMER_GROUP=job-workers-prod
KAFKA_WORKER_POOL_SIZE=20
```

## Running the Worker Server

```bash
# Start the worker server
go run cmd/kafka-worker/main.go
```

The worker server will:
1. ✅ Connect to Kafka brokers
2. ✅ Start worker pool (default: 10 concurrent workers)
3. ✅ Consume job events from `job-events` topic
4. ✅ Process jobs concurrently
5. ✅ Handle graceful shutdown

**Startup Log Example**:
```
Kafka job worker server configuration:
  - Event-driven jobs: 10 concurrent workers
  - Kafka brokers: [localhost:9092]
  - Kafka topic: job-events
  - Consumer group: job-workers
```

## Usage Examples

### Enqueuing Jobs

```go
import (
	"base-server/internal/clients/kafka"
	"base-server/internal/jobs"
	"base-server/internal/jobs/producer"
)

// Initialize producer
kafkaProducer := kafka.NewProducer(kafka.ProducerConfig{
	Brokers: []string{"localhost:9092"},
	Topic:   "job-events",
}, logger)

jobProducer := producer.New(kafkaProducer, logger)

// Example 1: Send verification email
err := jobProducer.EnqueueEmailJob(ctx, jobs.EmailJobPayload{
	Type:       "verification",
	CampaignID: campaignID,
	UserID:     userID,
	Priority:   1,
})

// Example 2: Recalculate positions after referral
err := jobProducer.EnqueuePositionRecalcJob(ctx, jobs.PositionRecalcJobPayload{
	CampaignID:        campaignID,
	PointsPerReferral: 1,
})

// Example 3: Deliver reward
err := jobProducer.EnqueueRewardDeliveryJob(ctx, jobs.RewardDeliveryJobPayload{
	UserRewardID: userRewardID,
})
```

## Benefits

✅ **Responsive**: Immediate processing of user actions
✅ **Scalable**: Workload scales with user activity
✅ **Durable**: Kafka persists events before processing
✅ **Replay**: Can reprocess events from any point in time
✅ **Decoupled**: Application doesn't wait for job completion
✅ **Consistent**: Same pattern as webhook delivery

## Event Schema

Events published to Kafka follow this schema:

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
    "priority": 1
  },
  "timestamp": "2025-01-01T12:00:00Z"
}
```

## Worker Pool Architecture

Jobs are processed by a worker pool for concurrent execution:

```go
// Worker pool processes events concurrently
jobConsumer := consumer.New(
	kafkaConsumer,
	emailWorker,
	positionWorker,
	rewardWorker,
	logger,
	10, // Number of concurrent workers
)
```

**Flow**:
1. Kafka consumer fetches messages
2. Messages sent to buffered channel
3. Worker pool (10 goroutines) consumes from channel
4. Each worker processes job and routes to appropriate handler
5. Offset committed after successful processing

## Local Development

### Using Docker Compose

```bash
# Start Kafka and all services
docker-compose -f docker-compose.services.yml up -d

# View Kafka UI (if included)
open http://localhost:8080
```

### Create Kafka Topic

```bash
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
     --number-of-broker-nodes 3
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
   go build -o kafka-worker cmd/kafka-worker/main.go

   KAFKA_BROKERS=$MSK_BROKERS \
   KAFKA_WORKER_POOL_SIZE=20 \
   ./kafka-worker
   ```

### Scaling

- **Horizontal**: Add more worker instances (they share consumer group)
- **Vertical**: Increase worker pool size per instance
- **Partitions**: Add more Kafka partitions for parallelism

## Monitoring

### Kafka Metrics
```bash
# Check consumer lag
kafka-consumer-groups --bootstrap-server $KAFKA_BROKERS \
  --group job-workers --describe
```

### Application Logs
- Check Kafka consumer logs
- Look for:
  - Jobs processed count
  - Processing time
  - Error rates

### Key Metrics to Track
- Messages per second
- Consumer lag
- Processing time per job type
- Error rate by job type

## Troubleshooting

### Jobs Not Processing

1. Check consumer lag (are messages piling up?)
2. Verify topic exists and has messages
3. Check worker logs for errors
4. Ensure Kafka brokers are reachable

### Slow Processing

1. Increase worker pool size: `KAFKA_WORKER_POOL_SIZE=20`
2. Scale horizontally (add more instances)
3. Optimize job processing logic

## Migration from Asynq/Redis

Both Asynq (Redis) and Kafka workers are supported:

1. **Asynq workers**: `cmd/worker/main.go` (optional, for legacy)
2. **Kafka workers**: `cmd/kafka-worker/main.go` (recommended)
3. Gradual migration - switch enqueueing to Kafka at your own pace

## References

- [Apache Kafka Documentation](https://kafka.apache.org/documentation/)
- [AWS MSK Documentation](https://docs.aws.amazon.com/msk/)
- [segmentio/kafka-go](https://github.com/segmentio/kafka-go)
- [Webhook Delivery with Kafka](./KAFKA_SETUP.md)
