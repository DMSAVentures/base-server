# Base Server

A Go-based server application with webhook delivery, Kafka event streaming, and comprehensive API endpoints.

## Local Development

### Prerequisites
- Docker and Docker Compose
- Go 1.23+

### Running Services Locally

Start all services (PostgreSQL, Kafka, Zookeeper) with migrations:

```bash
docker-compose -f docker-compose.services.yml up -d
```

This will start:
- **PostgreSQL** on port 5432 (Database: `base_db`, User: `base_user`, Password: `base_password`)
- **Kafka** on port 9092
- **Zookeeper** on port 2181
- **Flyway** (runs migrations and exits)

### Environment Configuration

Create an `env.local` file with the following variables:

```bash
# Database
DB_HOST=localhost
DB_USERNAME=base_user
DB_PASSWORD=base_password
DB_NAME=base_db

# Kafka
KAFKA_BROKERS=localhost:9092
KAFKA_TOPIC=webhook-events
KAFKA_CONSUMER_GROUP=webhook-consumers

# Add other required environment variables...
```

### Running the Application

```bash
go run main.go
```

### Stopping Services

```bash
docker-compose -f docker-compose.services.yml down
```

To also remove volumes (database data):

```bash
docker-compose -f docker-compose.services.yml down -v
```

## Production Database Migrations

### Step 1: Build the docker image
```bash
docker build -t flyway-migrate -f dbmigrator.dockerfile .
```

### Step 2: Connect to DB using AWS SSM
```bash
aws ssm start-session \
  --target <instance-id> \
  --document-name AWS-StartPortForwardingSessionToRemoteHost \
  --parameters "host=DB_HOST,portNumber=5432,localPortNumber=5432"
```

### Step 3: Run the docker image
```bash
docker run --platform linux/amd64 --rm \
  -e DB_HOST=host.docker.internal \
  -e DB_USERNAME=username \
  -e DB_PASSWORD=password \
  flyway-migrate
```

## Documentation

- [Kafka Setup Guide](docs/KAFKA_SETUP.md) - Comprehensive guide for Kafka integration
- [Developer Guidelines](CLAUDE.md) - Code conventions and architecture patterns