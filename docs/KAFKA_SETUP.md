# Kafka Integration for Webhook Delivery

This document explains how the webhook delivery service uses Apache Kafka for event-driven architecture.

## Architecture Overview

```
┌─────────────────┐     ┌──────────┐     ┌───────────────────┐
│  Application    │────▶│  Kafka   │────▶│ Webhook Consumer  │
│  (Event Source) │     │  Broker  │     │  (Worker Pool)    │
└─────────────────┘     └──────────┘     └─────────┬─────────┘
                                                    │
                                                    ▼
                                         ┌──────────────────────┐
                                         │  Webhook Delivery    │
                                         │  Service (HTTP POST) │
                                         └──────────────────────┘
```

## Flow

1. **Event Production**: Application events (user.created, referral.verified, etc.) are published to Kafka
2. **Event Storage**: Kafka durably stores events in the `webhook-events` topic
3. **Event Consumption**: Consumer worker pool (default: 10 workers) consumes events
4. **Webhook Delivery**: Each event is dispatched to subscribed webhooks
5. **Retry Logic**: Failed deliveries are tracked and retried with exponential backoff

## Environment Variables

Add these to your `env.local` file:

```bash
# Kafka Configuration
KAFKA_BROKERS=localhost:9092                  # Comma-separated list of Kafka brokers
KAFKA_TOPIC=webhook-events                    # Topic name for webhook events (optional, default: webhook-events)
KAFKA_CONSUMER_GROUP=webhook-consumers        # Consumer group ID (optional, default: webhook-consumers)
```

### For AWS MSK (Managed Streaming for Apache Kafka)

```bash
# AWS MSK Example
KAFKA_BROKERS=b-1.mycluster.abc123.kafka.us-east-1.amazonaws.com:9092,b-2.mycluster.abc123.kafka.us-east-1.amazonaws.com:9092
KAFKA_TOPIC=webhook-events
KAFKA_CONSUMER_GROUP=webhook-consumers-prod
```

## Benefits

### Durability
- Events are persisted in Kafka before webhook delivery
- No event loss even if the application crashes
- Events can be replayed if needed

### Scalability
- Kafka partitions enable horizontal scaling
- Multiple consumer instances can process events in parallel
- Worker pool (10 workers per consumer) processes events concurrently

### Reliability
- At-least-once delivery guarantee
- Failed webhook deliveries are retried automatically
- Delivery history tracked in database

### Decoupling
- Event producers don't wait for webhook delivery
- Fast response times for API calls
- Asynchronous processing of webhooks

## Event Schema

Events published to Kafka follow this schema:

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "user.created",
  "account_id": "account-uuid",
  "campaign_id": "campaign-uuid",
  "data": {
    "user_id": "user-uuid",
    "email": "user@example.com",
    "position": 42
  },
  "timestamp": "2025-01-01T12:00:00Z"
}
```

## Partitioning Strategy

Events are partitioned by `account_id` to ensure:
- All events for an account go to the same partition
- Ordering is maintained per account
- Load balancing across partitions

## Consumer Configuration

- **Consumer Group**: `webhook-consumers` (configurable)
- **Worker Pool Size**: 10 workers per consumer instance
- **Offset Commit**: Manual commit after successful webhook delivery
- **Start Offset**: Earliest (processes all messages from beginning)

## Monitoring

### Kafka Metrics
- Topic lag: Monitor consumer group lag
- Partition count: Ensure even distribution
- Throughput: Messages per second

### Application Metrics
- Events published: Count of events sent to Kafka
- Events consumed: Count of events processed
- Webhook deliveries: Success/failure rates
- Processing time: Time to process each event

## Local Development

### Using Docker Compose

Create a `docker-compose.kafka.yml`:

```yaml
version: '3.8'
services:
  zookeeper:
    image: confluentinc/cp-zookeeper:latest
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
      ZOOKEEPER_TICK_TIME: 2000
    ports:
      - "2181:2181"

  kafka:
    image: confluentinc/cp-kafka:latest
    depends_on:
      - zookeeper
    ports:
      - "9092:9092"
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
```

Start Kafka:
```bash
docker-compose -f docker-compose.kafka.yml up -d
```

Set environment variables:
```bash
export KAFKA_BROKERS=localhost:9092
export KAFKA_TOPIC=webhook-events
export KAFKA_CONSUMER_GROUP=webhook-consumers
```

## Production Deployment

### AWS MSK Setup

1. **Create MSK Cluster**
   ```bash
   aws kafka create-cluster \
     --cluster-name webhook-events-cluster \
     --kafka-version 2.8.1 \
     --number-of-broker-nodes 3 \
     --broker-node-group-info file://broker-config.json
   ```

2. **Create Topic**
   ```bash
   kafka-topics --create \
     --bootstrap-server $KAFKA_BROKERS \
     --topic webhook-events \
     --partitions 10 \
     --replication-factor 2
   ```

3. **Configure Application**
   - Set `KAFKA_BROKERS` to MSK bootstrap servers
   - Use IAM authentication for production
   - Enable encryption in transit

### Scaling Considerations

- **Partitions**: 10-20 partitions per topic for high throughput
- **Replication**: 2-3 replicas for fault tolerance
- **Consumer Instances**: Scale based on partition count
- **Worker Pool**: 10-20 workers per consumer instance

## Troubleshooting

### Events Not Being Consumed

1. Check consumer lag:
   ```bash
   kafka-consumer-groups --bootstrap-server $KAFKA_BROKERS \
     --group webhook-consumers --describe
   ```

2. Verify topic exists:
   ```bash
   kafka-topics --list --bootstrap-server $KAFKA_BROKERS
   ```

3. Check application logs for errors

### Slow Processing

1. Increase worker pool size in `main.go`:
   ```go
   eventConsumer := webhookConsumer.New(kafkaConsumer, webhookSvc, logger, 20)
   ```

2. Scale horizontally (add more consumer instances)

3. Optimize webhook delivery timeout

### Event Loss

1. Verify Kafka replication factor ≥ 2
2. Check `min.insync.replicas` setting
3. Enable producer acknowledgments (`acks=all`)

## Migration from Direct Delivery

If you're migrating from direct webhook delivery:

1. **Both modes can run concurrently** during transition
2. **New events** automatically use Kafka
3. **Existing retry logic** continues to work
4. **No data loss** during migration

## References

- [Apache Kafka Documentation](https://kafka.apache.org/documentation/)
- [AWS MSK Documentation](https://docs.aws.amazon.com/msk/)
- [segmentio/kafka-go](https://github.com/segmentio/kafka-go)
