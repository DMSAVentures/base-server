# Kafka-Based Background Workers

This document describes the Kafka-based background job system for the waitlist platform, which provides durability, replay capability, and event sourcing.

## Architecture

The system uses Apache Kafka for reliable, durable message processing with the following benefits:

### ✅ Advantages over Redis/Asynq

1. **Durability**: Messages are persisted to disk with replication across brokers
2. **Replay**: Can reprocess events from any point in time for recovery or debugging
3. **Event Sourcing**: Full audit trail of all business events
4. **Ordering Guarantees**: Per-partition message ordering
5. **Scalability**: Handles millions of messages per second
6. **Consumer Groups**: Automatic load balancing and failover
7. **Retention**: Configurable message retention (hours to years)

## Topic Architecture

### Job Topics (Transient Processing)

| Topic | Partitions | Replication | Retention | Description |
|-------|------------|-------------|-----------|-------------|
| `waitlist.email.verification` | 10 | 3 | 7 days | Email verification jobs |
| `waitlist.email.welcome` | 10 | 3 | 7 days | Welcome email jobs |
| `waitlist.email.position-update` | 10 | 3 | 7 days | Position update notifications |
| `waitlist.email.reward-earned` | 10 | 3 | 7 days | Reward earned emails |
| `waitlist.email.milestone` | 5 | 3 | 7 days | Milestone emails |
| `waitlist.reward.delivery` | 10 | 3 | 30 days | Reward delivery jobs (critical) |
| `waitlist.position.recalculation` | 5 | 3 | 7 days | Position recalc jobs |
| `waitlist.fraud.detection` | 5 | 3 | 30 days | Fraud detection analysis |
| `waitlist.webhook.delivery` | 5 | 3 | 7 days | Webhook delivery jobs |
| `waitlist.analytics.aggregation` | 3 | 3 | 7 days | Analytics aggregation |

### Event Sourcing Topics (Permanent Audit Log)

| Topic | Partitions | Replication | Retention | Description |
|-------|------------|-------------|-----------|-------------|
| `waitlist.events.user.signup` | 10 | 3 | 1 year | User signup events |
| `waitlist.events.user.verified` | 10 | 3 | 1 year | Email verification events |
| `waitlist.events.referral.created` | 10 | 3 | 1 year | Referral creation events |
| `waitlist.events.reward.earned` | 10 | 3 | 1 year | Reward earned events |

### Dead Letter Queue

| Topic | Partitions | Replication | Retention | Description |
|-------|------------|-------------|-----------|-------------|
| `waitlist.dlq` | 3 | 3 | 1 year | Failed messages from all topics |

## Consumer Groups

Each worker type runs in its own consumer group for independent scaling:

- `email-workers`: Processes all email topics
- `position-workers`: Processes position recalculation
- `reward-workers`: Processes reward delivery
- `analytics-workers`: Processes analytics aggregation
- `fraud-workers`: Processes fraud detection
- `webhook-workers`: Processes webhook delivery
- `event-processor`: Processes event sourcing topics

## Message Format

All Kafka messages follow this structure:

```json
{
  "key": "campaign_id_user_id",
  "value": {
    "type": "verification",
    "campaign_id": "uuid",
    "user_id": "uuid",
    "template_data": {}
  },
  "headers": {
    "message_id": "uuid",
    "produced_at": "2025-01-01T00:00:00Z",
    "producer": "base-server",
    "job_type": "email",
    "email_type": "verification"
  },
  "timestamp": "2025-01-01T00:00:00Z"
}
```

## Running Workers

### Prerequisites

1. **Kafka Cluster**: Running Kafka cluster (see docker-compose.kafka.yml)
2. **Database**: PostgreSQL with TimescaleDB
3. **Environment Variables**:
   ```bash
   KAFKA_BROKERS=localhost:9092  # Comma-separated list
   DB_HOST=localhost
   DB_USERNAME=postgres
   DB_PASSWORD=yourpassword
   DB_NAME=waitlist_platform
   ```

### Start Kafka Locally

```bash
# Start Kafka with Zookeeper and Kafka UI
docker-compose -f docker-compose.kafka.yml up -d

# View Kafka UI at http://localhost:8080
```

### Start Worker Server

```bash
# Run Kafka worker server
go run cmd/kafka-worker/main.go
```

The worker server will:
1. Connect to Kafka brokers
2. Create consumers for all topics
3. Start processing messages in parallel
4. Auto-commit offsets on successful processing
5. Send failed messages to dead letter queue

## Job Client Usage

### Enqueueing Jobs

```go
package main

import (
    "context"
    "base-server/internal/jobs"
    "base-server/internal/observability"
    "github.com/google/uuid"
)

func main() {
    logger := observability.NewLogger()
    brokers := []string{"localhost:9092"}

    jobClient := jobs.NewKafkaClient(brokers, logger)
    defer jobClient.Close()

    ctx := context.Background()

    // Email verification job
    err := jobClient.EnqueueEmailJob(ctx, jobs.EmailJobPayload{
        Type:       "verification",
        CampaignID: campaignID,
        UserID:     userID,
        TemplateData: map[string]interface{}{
            "verification_link": "https://example.com/verify/...",
        },
    })

    // Fraud detection job
    err = jobClient.EnqueueFraudDetectionJob(ctx, jobs.FraudDetectionJobPayload{
        CampaignID: campaignID,
        UserID:     userID,
        CheckTypes: []string{"self_referral", "fake_email", "bot"},
    })
}
```

### Publishing Events (Event Sourcing)

```go
// Publish user signup event for audit trail
err := jobClient.PublishEvent(ctx,
    kafka.TopicUserSignup,
    userID.String(),
    map[string]interface{}{
        "user_id":     userID,
        "campaign_id": campaignID,
        "email":       email,
        "source":      "referral",
        "timestamp":   time.Now(),
    },
)
```

## Message Ordering

Messages with the same key are guaranteed to be processed in order within a partition:

- **Email jobs**: Keyed by `campaign_id + user_id`
- **Position recalc**: Keyed by `campaign_id`
- **Rewards**: Keyed by `user_reward_id`
- **Fraud detection**: Keyed by `campaign_id + user_id`

## Error Handling & DLQ

Failed messages are automatically sent to the dead letter queue (`waitlist.dlq`) with:

- Original message payload
- Error details
- Timestamp of failure
- Original topic/partition/offset

### Monitoring DLQ

```bash
# View DLQ messages in Kafka UI
# http://localhost:8080/ui/clusters/local/all-topics/waitlist.dlq

# Or use kafka-console-consumer
kafka-console-consumer --bootstrap-server localhost:9092 \
  --topic waitlist.dlq \
  --from-beginning
```

### Replaying Failed Messages

1. Identify failed message in DLQ
2. Fix the underlying issue (e.g., invalid data, external service down)
3. Replay the message:

```go
// Read from DLQ, fix data, republish to original topic
```

## Replay & Recovery

### Replay from Specific Offset

```go
// Create consumer starting from specific offset
consumer := kafka.NewConsumer(
    kafka.ConsumerConfig{
        Brokers:     brokers,
        Topic:       "waitlist.email.verification",
        GroupID:     "replay-group",
        StartOffset: kafka.FirstOffset, // Start from beginning
    },
    handler,
    dlqProducer,
    logger,
)
```

### Rebuild Analytics from Events

```go
// Replay all user signup events to rebuild analytics
consumer := kafka.NewConsumer(
    kafka.ConsumerConfig{
        Topic:       kafka.TopicUserSignup,
        GroupID:     "analytics-rebuild",
        StartOffset: kafka.FirstOffset,
    },
    analyticsRebuildHandler,
    nil,
    logger,
)
```

## Monitoring & Metrics

### Consumer Lag

Monitor consumer lag to ensure workers are keeping up:

```bash
# Check consumer group lag
kafka-consumer-groups --bootstrap-server localhost:9092 \
  --describe \
  --group email-workers
```

### Producer Metrics

```go
// Get producer statistics
stats := producer.Stats()
fmt.Printf("Messages sent: %d\n", stats.Messages)
fmt.Printf("Bytes sent: %d\n", stats.Bytes)
fmt.Printf("Errors: %d\n", stats.Errors)
```

### Consumer Metrics

```go
// Get consumer statistics
stats := consumer.Stats()
fmt.Printf("Messages consumed: %d\n", stats.Messages)
fmt.Printf("Bytes consumed: %d\n", stats.Bytes)
fmt.Printf("Lag: %d\n", stats.Lag)
```

## Scaling

### Horizontal Scaling

Add more worker instances for a consumer group:

```bash
# Run multiple instances with same consumer group
# Kafka will auto-rebalance partitions across instances

# Instance 1
CONSUMER_INSTANCE=1 go run cmd/kafka-worker/main.go

# Instance 2
CONSUMER_INSTANCE=2 go run cmd/kafka-worker/main.go

# Instance 3
CONSUMER_INSTANCE=3 go run cmd/kafka-worker/main.go
```

Each instance will process a subset of partitions.

### Vertical Scaling

Increase partitions for higher parallelism:

```bash
# Increase partitions for a topic
kafka-topics --bootstrap-server localhost:9092 \
  --alter \
  --topic waitlist.email.verification \
  --partitions 20
```

## Production Deployment

### Recommended Configuration

```yaml
# Kafka Cluster
brokers: 3+
replication_factor: 3
min_insync_replicas: 2

# Topics
high_priority_partitions: 10-20
medium_priority_partitions: 5-10
low_priority_partitions: 2-5

# Workers
email_workers: 10 instances
position_workers: 3 instances
reward_workers: 5 instances
fraud_workers: 3 instances
analytics_workers: 2 instances
```

### Monitoring

- **Kafka Manager/UI**: Monitor topics, consumers, lag
- **DataDog/Prometheus**: Metrics for message rates, latency
- **Sentry**: Error tracking for failed jobs
- **Custom Dashboards**: Business metrics (emails sent, rewards delivered)

## Comparison: Kafka vs Asynq/Redis

| Feature | Kafka | Asynq/Redis |
|---------|-------|-------------|
| Durability | ✅ Disk + Replication | ⚠️ Memory (AOF/RDB optional) |
| Message Loss | ❌ No (if configured correctly) | ⚠️ Possible on crash |
| Replay | ✅ Yes, from any offset | ❌ No |
| Event Sourcing | ✅ Yes | ❌ No |
| Ordering | ✅ Per-partition | ⚠️ No guarantees |
| Latency | ~10-100ms | ~1-5ms |
| Throughput | Millions/sec | Hundreds of thousands/sec |
| Retention | Days to years | Minutes to hours |
| Complexity | High | Low |
| Cost | Higher (storage) | Lower (memory) |

## Troubleshooting

### Consumer Not Receiving Messages

1. Check consumer group is correct
2. Verify topic exists: `kafka-topics --list --bootstrap-server localhost:9092`
3. Check consumer lag: Consumer might be stuck on one message
4. Verify broker connectivity

### Messages Stuck in DLQ

1. View DLQ messages in Kafka UI
2. Identify error pattern
3. Fix root cause
4. Replay or manually reprocess

### High Consumer Lag

1. Add more worker instances
2. Increase topic partitions
3. Optimize message processing
4. Check for slow external dependencies

## Migration from Asynq

The codebase includes both Asynq (`cmd/worker/main.go`) and Kafka (`cmd/kafka-worker/main.go`) workers. To migrate:

1. ✅ Set up Kafka cluster
2. ✅ Run Kafka workers alongside Asynq workers
3. ✅ Gradually migrate job enqueueing to Kafka client
4. ✅ Monitor both systems
5. ✅ Decommission Asynq workers when Kafka is stable

## Resources

- [Kafka Documentation](https://kafka.apache.org/documentation/)
- [segmentio/kafka-go](https://github.com/segmentio/kafka-go)
- [Kafka UI](https://github.com/provectus/kafka-ui)
- [Event Sourcing Pattern](https://martinfowler.com/eaaDev/EventSourcing.html)
