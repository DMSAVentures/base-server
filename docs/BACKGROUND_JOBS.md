# Background Job Workers

This document explains the background job processing system using **two complementary patterns**:

1. **🔔 Event-Driven Jobs** - Triggered by user actions (via Kafka)
2. **⏰ Scheduled Jobs** - Run on time intervals (via built-in scheduler)

## Why Two Patterns?

**Event-Driven** is perfect for:
- Reacting to user actions (signup → send email)
- Processing triggered by events
- Workload scales with user activity

**Scheduled** is perfect for:
- Periodic batch processing
- Analytics aggregation
- System maintenance tasks
- Fraud detection scans

## Architecture Overview

### Event-Driven Jobs (Kafka)
```
User Signs Up ──▶ App Publishes Event ──▶ Kafka ──▶ Worker Pool ──▶ Send Email
User Refers   ──▶ App Publishes Event ──▶ Kafka ──▶ Worker Pool ──▶ Update Position
Reward Earned ──▶ App Publishes Event ──▶ Kafka ──▶ Worker Pool ──▶ Deliver Reward
```

### Scheduled Jobs (Cron-like)
```
Every Hour     ──▶ Scheduler ──▶ Analytics Aggregation
Every 15 Minutes ──▶ Scheduler ──▶ Fraud Detection Scan
```

## Job Types

### 🔔 Event-Driven Jobs

#### 1. Email Jobs (Event-Driven)
**Trigger**: User actions (signup, verification, referral, etc.)
**Pattern**: Kafka event → Worker pool processes → Email sent

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

#### 2. Position Recalculation (Event-Driven)
**Trigger**: Referral verification completed
**Pattern**: Kafka event → Recalculate all positions

**Algorithm**: `Position = Original Position - (Verified Referrals × Points Per Referral)`

**Usage**:
```go
jobProducer.EnqueuePositionRecalcJob(ctx, jobs.PositionRecalcJobPayload{
	CampaignID:        campaignID,
	PointsPerReferral: 1,
})
```

#### 3. Reward Delivery (Event-Driven)
**Trigger**: User earns a reward
**Pattern**: Kafka event → Deliver reward via email/webhook

**Usage**:
```go
jobProducer.EnqueueRewardDeliveryJob(ctx, jobs.RewardDeliveryJobPayload{
	UserRewardID: userRewardID,
})
```

### ⏰ Scheduled Jobs

#### 4. Analytics Aggregation (Scheduled)
**Schedule**: Every hour (configurable)
**Pattern**: Cron scheduler → Aggregate metrics for all campaigns

**Metrics Collected**:
- New signups count
- Email verification count
- Referral count
- Emails sent count
- Rewards delivered count

**No manual triggering needed** - runs automatically on schedule.

#### 5. Fraud Detection (Scheduled)
**Schedule**: Every 15 minutes (configurable)
**Pattern**: Cron scheduler → Scan recent users for fraud

**Check Types**:
- Self-referral detection
- Velocity checks (too many referrals too quickly)
- Fake email detection
- Bot detection
- Duplicate IP address detection
- Duplicate device fingerprint detection

**No manual triggering needed** - runs automatically on schedule.

## Environment Variables

```bash
# Kafka Configuration (for event-driven jobs)
KAFKA_BROKERS=localhost:9092                   # Comma-separated broker list
KAFKA_JOB_TOPIC=job-events                     # Topic name (default: job-events)
KAFKA_JOB_CONSUMER_GROUP=job-workers           # Consumer group (default: job-workers)
KAFKA_WORKER_POOL_SIZE=10                      # Concurrent workers (default: 10)

# Scheduler Configuration (for scheduled jobs)
ANALYTICS_INTERVAL=1h                          # Analytics aggregation interval (default: 1h)
FRAUD_DETECTION_INTERVAL=15m                   # Fraud detection interval (default: 15m)
```

### AWS MSK Example

```bash
KAFKA_BROKERS=b-1.cluster.kafka.us-east-1.amazonaws.com:9092,b-2.cluster.kafka.us-east-1.amazonaws.com:9092
KAFKA_JOB_TOPIC=job-events
KAFKA_JOB_CONSUMER_GROUP=job-workers-prod
KAFKA_WORKER_POOL_SIZE=20
ANALYTICS_INTERVAL=1h
FRAUD_DETECTION_INTERVAL=10m
```

## Running the Worker Server

```bash
# Start the worker server (handles BOTH event-driven and scheduled jobs)
go run cmd/kafka-worker/main.go
```

The worker server will:
1. ✅ Start Kafka consumer for event-driven jobs (emails, position, rewards)
2. ✅ Start scheduler for cron-based jobs (analytics, fraud detection)
3. ✅ Process jobs concurrently with worker pool
4. ✅ Handle graceful shutdown

**Startup Log Example**:
```
Kafka job worker server configuration:
  - Event-driven jobs: 10 concurrent workers
  - Kafka brokers: [localhost:9092]
  - Kafka topic: job-events
  - Consumer group: job-workers
  - Analytics aggregation: every 1h0m0s
  - Fraud detection: every 15m0s
```

## Usage Examples

### Event-Driven Job Enqueueing

```go
import (
	"base-server/internal/clients/kafka"
	"base-server/internal/jobs/producer"
)

// Initialize producer
kafkaProducer := kafka.NewProducer(kafka.ProducerConfig{
	Brokers: []string{"localhost:9092"},
	Topic:   "job-events",
}, logger)

jobProducer := producer.New(kafkaProducer, logger)

// Example 1: Send verification email (event-driven)
err := jobProducer.EnqueueEmailJob(ctx, jobs.EmailJobPayload{
	Type:       "verification",
	CampaignID: campaignID,
	UserID:     userID,
	Priority:   1,
})

// Example 2: Recalculate positions after referral (event-driven)
err := jobProducer.EnqueuePositionRecalcJob(ctx, jobs.PositionRecalcJobPayload{
	CampaignID:        campaignID,
	PointsPerReferral: 1,
})

// Example 3: Deliver reward (event-driven)
err := jobProducer.EnqueueRewardDeliveryJob(ctx, jobs.RewardDeliveryJobPayload{
	UserRewardID: userRewardID,
})
```

### Scheduled Jobs

**No code needed!** Scheduled jobs (analytics, fraud detection) run automatically based on the configured intervals.

To customize intervals, set environment variables:
```bash
ANALYTICS_INTERVAL=30m        # Run analytics every 30 minutes
FRAUD_DETECTION_INTERVAL=5m   # Run fraud detection every 5 minutes
```

## Benefits

### Event-Driven Jobs
- ✅ **Responsive**: Immediate processing of user actions
- ✅ **Scalable**: Workload scales with user activity
- ✅ **Durable**: Kafka persists events before processing
- ✅ **Replay**: Can reprocess events from any point in time
- ✅ **Decoupled**: Application doesn't wait for job completion

### Scheduled Jobs
- ✅ **Predictable**: Runs at consistent intervals
- ✅ **Efficient**: Batch processing reduces overhead
- ✅ **Simple**: No need to trigger manually
- ✅ **Resource-aware**: Runs during low-traffic periods
- ✅ **Comprehensive**: Scans all data, not just recent events

## Event Schema

Event-driven jobs published to Kafka follow this schema:

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

Event-driven jobs use a worker pool for concurrent processing:

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
4. Each worker processes job
5. Offset committed after success

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
   ANALYTICS_INTERVAL=1h \
   FRAUD_DETECTION_INTERVAL=10m \
   ./kafka-worker
   ```

### Scaling

**Event-Driven Jobs**:
- Add more worker instances (horizontal scaling)
- Increase worker pool size per instance
- Add more Kafka partitions

**Scheduled Jobs**:
- Run in a single instance (distributed locking not needed for these jobs)
- Adjust intervals based on load
- Schedule during off-peak hours

## Monitoring

### Kafka Metrics (Event-Driven)
```bash
# Check consumer lag
kafka-consumer-groups --bootstrap-server $KAFKA_BROKERS \
  --group job-workers --describe
```

### Application Logs
- Event-driven jobs: Check Kafka consumer logs
- Scheduled jobs: Check scheduler logs (runs every interval)
- Look for:
  - Jobs processed count
  - Processing time
  - Error rates

### Key Metrics to Track
- **Event-driven**: Messages per second, consumer lag, processing time
- **Scheduled**: Job completion time, records processed, error count

## Troubleshooting

### Event-Driven Jobs Not Processing

1. Check consumer lag (are messages piling up?)
2. Verify topic exists and has messages
3. Check worker logs for errors
4. Ensure Kafka brokers are reachable

### Scheduled Jobs Not Running

1. Check scheduler logs for job execution
2. Verify intervals are configured correctly
3. Check database connectivity
4. Look for errors in job execution logs

### Slow Processing

1. Increase worker pool size: `KAFKA_WORKER_POOL_SIZE=20`
2. Scale horizontally (add more instances)
3. Optimize job processing logic
4. Adjust scheduled job intervals

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
