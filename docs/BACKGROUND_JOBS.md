# Domain Event Processing with Kafka

This document explains the event-driven architecture using Apache Kafka for domain events.

## Architecture Overview

```
User Action ──▶ App Emits Domain Event ──▶ Kafka ──▶ Multiple Consumers
  Signup           user.signed_up            Topic      Email Consumer
  Referral         referral.verified                    Position Consumer
  Reward Earned    reward.earned                        Reward Consumer
```

**Pattern**: Domain events describe **what happened**, not **what to do**
- Application emits domain events (user.signed_up, referral.verified, reward.earned)
- Multiple consumer groups subscribe to the same events (Kafka fan-out)
- Each consumer processes events independently for their specific purpose

## Domain Events

### User Events

**user.signed_up** - Emitted when a user signs up for a campaign
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "user.signed_up",
  "account_id": "campaign-uuid",
  "campaign_id": "campaign-uuid",
  "data": {
    "user_id": "user-uuid",
    "campaign_id": "campaign-uuid",
    "email": "user@example.com"
  },
  "timestamp": "2025-01-01T12:00:00Z"
}
```

**user.verified** - Emitted when a user verifies their email
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "user.verified",
  "account_id": "campaign-uuid",
  "campaign_id": "campaign-uuid",
  "data": {
    "user_id": "user-uuid",
    "campaign_id": "campaign-uuid",
    "email": "user@example.com"
  },
  "timestamp": "2025-01-01T12:00:00Z"
}
```

### Referral Events

**referral.created** - Emitted when a referral is created
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "referral.created",
  "account_id": "campaign-uuid",
  "campaign_id": "campaign-uuid",
  "data": {
    "referral_id": "referral-uuid",
    "referrer_id": "user-uuid",
    "referred_id": "user-uuid",
    "campaign_id": "campaign-uuid"
  },
  "timestamp": "2025-01-01T12:00:00Z"
}
```

**referral.verified** - Emitted when a referral is verified
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "referral.verified",
  "account_id": "campaign-uuid",
  "campaign_id": "campaign-uuid",
  "data": {
    "referral_id": "referral-uuid",
    "referrer_id": "user-uuid",
    "referred_id": "user-uuid",
    "campaign_id": "campaign-uuid",
    "points_per_referral": 1
  },
  "timestamp": "2025-01-01T12:00:00Z"
}
```

### Reward Events

**reward.earned** - Emitted when a user earns a reward
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "reward.earned",
  "account_id": "campaign-uuid",
  "campaign_id": "campaign-uuid",
  "data": {
    "user_reward_id": "user-reward-uuid",
    "user_id": "user-uuid",
    "campaign_id": "campaign-uuid",
    "reward_id": "reward-uuid"
  },
  "timestamp": "2025-01-01T12:00:00Z"
}
```

### Campaign Events

**campaign.milestone_reached** - Emitted when campaign reaches a milestone
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "campaign.milestone_reached",
  "account_id": "campaign-uuid",
  "campaign_id": "campaign-uuid",
  "data": {
    "campaign_id": "campaign-uuid",
    "milestone": "1000_participants",
    "participant_count": 1000
  },
  "timestamp": "2025-01-01T12:00:00Z"
}
```

## Consumers

### Email Consumer (email-workers)
**Subscribes to**: user.signed_up, user.verified, referral.verified, reward.earned, campaign.milestone_reached

**Actions**:
- `user.signed_up` → Send welcome email
- `user.verified` → Send verification confirmation
- `referral.verified` → Send position update email to referrer
- `reward.earned` → Send reward notification email
- `campaign.milestone_reached` → Send milestone announcement email

### Position Consumer (position-workers)
**Subscribes to**: referral.verified

**Actions**:
- `referral.verified` → Recalculate leaderboard positions
  - Algorithm: `Position = Original Position - (Verified Referrals × Points Per Referral)`

**Concurrency**: Uses **1 worker** (not 10) to prevent race conditions
- Position recalculation requires reading entire campaign state, calculating new positions, and writing back
- Multiple concurrent workers processing the same campaign would cause read-modify-write conflicts
- Additional protection: PostgreSQL advisory locks ensure campaign-level serialization
- Future scaling: Can scale horizontally by adding more consumer instances (each processes different partitions)

### Reward Consumer (reward-workers)
**Subscribes to**: reward.earned

**Actions**:
- `reward.earned` → Deliver reward via email or webhook

## Kafka Fan-Out Pattern

```
                    ┌──▶ email-workers (10 workers, sends email)
user.signed_up  ───┤
                    └──▶ (future: analytics-workers)

                    ┌──▶ email-workers (10 workers, sends notification)
referral.verified ─┼──▶ position-workers (1 worker, recalculates positions)
                    └──▶ (future: analytics-workers)

                    ┌──▶ email-workers (10 workers, sends notification)
reward.earned   ───┼──▶ reward-workers (10 workers, delivers reward)
                    └──▶ (future: analytics-workers)
```

**Key Benefit**: Single event triggers multiple independent actions without coupling

**Partitioning**: Events are partitioned by `campaign_id`
- All events for the same campaign go to the same Kafka partition
- Ensures ordering guarantees within a campaign
- Enables horizontal scaling (add more consumer instances to process different campaigns)

## Environment Variables

```bash
# Kafka Configuration
KAFKA_BROKERS=localhost:9092                   # Comma-separated broker list
KAFKA_TOPIC=domain-events                      # Topic name (default: domain-events)
KAFKA_WORKER_POOL_SIZE=10                      # Concurrent workers per consumer (default: 10)
```

### AWS MSK Example

```bash
KAFKA_BROKERS=b-1.cluster.kafka.us-east-1.amazonaws.com:9092,b-2.cluster.kafka.us-east-1.amazonaws.com:9092
KAFKA_TOPIC=domain-events
KAFKA_WORKER_POOL_SIZE=20
```

## Running the Consumer Server

```bash
# Start the consumer server
go run cmd/kafka-worker/main.go
```

The consumer server will:
1. ✅ Connect to Kafka brokers
2. ✅ Start 3 independent consumer groups (email-workers, position-workers, reward-workers)
3. ✅ Each consumer group has 10 concurrent workers (default)
4. ✅ Consume domain events from `domain-events` topic
5. ✅ Process events concurrently within each consumer group
6. ✅ Handle graceful shutdown

**Startup Log Example**:
```
Kafka event consumer server configuration:
  - Domain events topic: domain-events
  - Kafka brokers: [localhost:9092]
  - Worker pool size: 10 per consumer
  - Consumer groups:
    * email-workers (processes: user.*, referral.verified, reward.earned, campaign.*)
    * position-workers (processes: referral.verified)
    * reward-workers (processes: reward.earned)
```

## Publishing Domain Events

### Example: Publishing from Application Code

```go
import (
	"base-server/internal/clients/kafka"
	"base-server/internal/events"
	"base-server/internal/observability"
)

// Initialize event publisher
kafkaProducer := kafka.NewProducer(kafka.ProducerConfig{
	Brokers: []string{"localhost:9092"},
	Topic:   "domain-events",
}, logger)

eventPublisher := events.NewPublisher(kafkaProducer, logger)

// Example 1: User signs up
err := eventPublisher.PublishUserSignedUp(ctx, userID, campaignID, email)

// Example 2: Referral is verified
err := eventPublisher.PublishReferralVerified(ctx, referralID, referrerID, referredID, campaignID, 1)

// Example 3: User earns reward
err := eventPublisher.PublishRewardEarned(ctx, userRewardID, userID, campaignID, rewardID)
```

### Integration Points

In your application code, emit domain events at the appropriate places:

**Signup Handler** (internal/auth/processor/signup.go):
```go
// After creating user
eventPublisher.PublishUserSignedUp(ctx, user.ID, campaignID, user.Email)
```

**Email Verification** (internal/auth/processor/verify.go):
```go
// After verifying email
eventPublisher.PublishUserVerified(ctx, user.ID, campaignID, user.Email)
```

**Referral Verification** (internal/referrals/processor/verify.go):
```go
// After verifying referral
eventPublisher.PublishReferralVerified(ctx, referral.ID, referral.ReferrerID, referral.ReferredID, campaignID, pointsPerReferral)
```

**Reward Earned** (internal/rewards/processor/earn.go):
```go
// After user earns reward
eventPublisher.PublishRewardEarned(ctx, userReward.ID, user.ID, campaignID, reward.ID)
```

## Benefits

✅ **Decoupled**: Consumers are independent, can be deployed separately
✅ **Scalable**: Add more consumer instances to scale horizontally
✅ **Extensible**: Add new consumers without changing event publishers
✅ **Durable**: Kafka persists events before processing
✅ **Replayable**: Can reprocess events from any point in time
✅ **Fan-out**: Single event triggers multiple actions automatically
✅ **Simple**: Events describe what happened, not what to do

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
  --topic domain-events \
  --partitions 10 \
  --replication-factor 1
```

## Production Deployment

### AWS MSK Setup

1. **Create MSK Cluster**
   ```bash
   aws kafka create-cluster \
     --cluster-name domain-events-cluster \
     --kafka-version 2.8.1 \
     --number-of-broker-nodes 3
   ```

2. **Create Topic**
   ```bash
   kafka-topics --create \
     --bootstrap-server $KAFKA_BROKERS \
     --topic domain-events \
     --partitions 10 \
     --replication-factor 2
   ```

3. **Deploy Consumer Server**
   ```bash
   go build -o kafka-worker cmd/kafka-worker/main.go

   KAFKA_BROKERS=$MSK_BROKERS \
   KAFKA_WORKER_POOL_SIZE=20 \
   ./kafka-worker
   ```

### Scaling

- **Horizontal**: Add more consumer instances (they share consumer group)
- **Vertical**: Increase worker pool size per instance
- **Partitions**: Add more Kafka partitions for parallelism
- **Consumer Groups**: Each consumer group processes all events independently

Example: 3 consumer servers, 20 workers each = 60 concurrent workers per consumer group

## Monitoring

### Kafka Metrics
```bash
# Check consumer lag for each group
kafka-consumer-groups --bootstrap-server $KAFKA_BROKERS \
  --group email-workers --describe

kafka-consumer-groups --bootstrap-server $KAFKA_BROKERS \
  --group position-workers --describe

kafka-consumer-groups --bootstrap-server $KAFKA_BROKERS \
  --group reward-workers --describe
```

### Application Logs
- Check consumer logs for each consumer group
- Look for:
  - Events processed count
  - Processing time per event type
  - Error rates by consumer group

### Key Metrics to Track
- Messages per second (by event type)
- Consumer lag (by consumer group)
- Processing time (by event type and consumer)
- Error rate (by event type and consumer)

## Concurrency & Race Conditions

### Worker Pool Sizing

Different consumers have different concurrency requirements:

**Email Consumer**: 10 workers (default)
- Each email is independent
- No shared state between emails
- Safe to process concurrently

**Reward Consumer**: 10 workers (default)
- Each reward delivery is independent
- Uses idempotency keys
- Safe to process concurrently

**Position Consumer**: 1 worker (fixed)
- **CRITICAL**: Position recalculation has read-modify-write pattern
- Multiple workers processing the same campaign would cause race conditions:
  1. Worker 1 reads campaign state
  2. Worker 2 reads campaign state (same snapshot)
  3. Both calculate new positions
  4. Worker 1 writes positions
  5. Worker 2 writes positions (overwrites Worker 1's updates with stale data)
- **Protection Layers**:
  1. Single worker per consumer instance
  2. PostgreSQL advisory locks (campaign-level serialization)
  3. Kafka partitioning by campaign_id (ensures ordering)

### Scaling Position Consumer

To scale position recalculation:

1. **Horizontal Scaling**: Add more consumer instances (each gets different Kafka partitions)
   - Instance 1 processes campaigns on partitions 0-4
   - Instance 2 processes campaigns on partitions 5-9
   - Each instance still uses 1 worker

2. **Partition Key**: Events are partitioned by campaign_id
   - All events for Campaign A go to Partition 3
   - All events for Campaign B go to Partition 7
   - Different campaigns can be processed in parallel safely

3. **Advisory Locks**: Extra safety if somehow multiple workers process same campaign
   - `pg_advisory_xact_lock(hashtext(campaign_id))`
   - Ensures only one transaction can recalculate positions for a campaign at a time

### Idempotency

All event handlers should be idempotent (safe to process the same event multiple times):

**Email Consumer**: Uses email service's idempotency (deduplicate within time window)
**Reward Consumer**: Checks delivery status before processing
**Position Consumer**: Recalculates from source of truth (verified_referral_count), not incremental

## Troubleshooting

### Events Not Processing

1. Check consumer lag (are messages piling up?)
2. Verify topic exists and has messages
3. Check consumer logs for errors
4. Ensure Kafka brokers are reachable
5. Verify consumer group IDs are correct

### Slow Processing

1. **For email/reward consumers**: Increase worker pool size: `KAFKA_WORKER_POOL_SIZE=20`
2. **For position consumer**: Scale horizontally (add more consumer instances)
3. Optimize event processing logic
4. Check database performance (often bottleneck)
5. Add more Kafka partitions for better parallelism

### Duplicate Processing

- Kafka guarantees at-least-once delivery
- Ensure your event handlers are idempotent
- Use idempotency keys for critical operations (e.g., email sending, payment processing)

## Adding New Consumers

To add a new consumer (e.g., analytics):

1. Create consumer in `internal/events/consumers/analytics_consumer.go`
2. Subscribe to relevant event types
3. Add consumer group in `cmd/kafka-worker/main.go`
4. Deploy and monitor

**No changes needed to event publishers!** This is the power of event-driven architecture.

## References

- [Apache Kafka Documentation](https://kafka.apache.org/documentation/)
- [AWS MSK Documentation](https://docs.aws.amazon.com/msk/)
- [segmentio/kafka-go](https://github.com/segmentio/kafka-go)
- [Webhook Delivery with Kafka](./KAFKA_SETUP.md)
- [Event-Driven Architecture Patterns](https://martinfowler.com/articles/201701-event-driven.html)
