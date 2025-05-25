# Leaderboard Realtime Service

A high-performance, real-time leaderboard service built with Go, Kafka, PostgreSQL, and Redis-like in-memory caching.

## Features

- **Real-time leaderboard updates** with sub-second latency
- **Game-specific Kafka topics** for isolated message processing
- **High-performance in-memory caching** with multiple time windows
- **PostgreSQL persistence** for data durability
- **Horizontal scaling** with load balancing via Traefik
- **Batch processing** for optimal throughput
- **RESTful API** with Swagger documentation

## Service-Specific Processing

The service uses a simple but effective approach: **whoever produces a message should consume it**.

### How It Works

1. **Single Topic**: All scores go to one topic: `leaderboard-scores`
2. **Game-based Partitioning**: Messages are partitioned by game ID using the key `game-{gameID}`
3. **Unique Consumer Groups**: Each service instance gets its own consumer group: `{consumer-group}-{serviceID}`
4. **Load Balancing**: Kafka automatically distributes partitions across consumer instances

### Benefits

- **Simplicity**: One topic, clear message flow
- **Isolation**: Each service instance processes its own messages
- **Scalability**: Kafka handles partition distribution automatically
- **Fault Tolerance**: If one instance fails, others continue processing

### Configuration

Set these environment variables:

```bash
KAFKA_SCORES_TOPIC_PREFIX=leaderboard-scores  # Topic name for scores
KAFKA_CONSUMER_GROUP=score-processor          # Base consumer group name
```

The service automatically generates unique consumer groups per instance using hostname or timestamp.

## Quick Start

### Using Docker Compose

```bash
# Start all services
docker-compose up -d

# Scale leaderboard service to 4 instances
docker-compose up -d --scale leaderboard=4

# View logs
docker-compose logs -f leaderboard
```

### API Endpoints

- `POST /api/leaderboard/score/{gameId}` - Submit a score
- `GET /api/leaderboard/top/{gameId}` - Get top players
- `GET /api/leaderboard/rank/{gameId}/{userId}` - Get player rank
- `GET /api/health` - Health check

### API Documentation

Visit `http://localhost/swagger/index.html` for interactive API documentation.

## Architecture

```
┌─────────────┐    ┌──────────────┐    ┌─────────────┐
│   Client    │───▶│   Traefik    │───▶│ Leaderboard │
└─────────────┘    │ Load Balancer│    │  Service    │
                   └──────────────┘    │ (8 replicas)│
                                       └─────────────┘
                                              │
                   ┌─────────────────────────────────────┐
                   │                                     │
                   ▼                                     ▼
            ┌─────────────┐                    ┌─────────────┐
            │    Kafka    │                    │ PostgreSQL  │
            │Single Topic │                    │  Database   │
            │(Partitioned)│                    │             │
            └─────────────┘                    └─────────────┘
```

## Performance

- **Throughput**: 50,000+ requests/second
- **Latency**: Sub-millisecond for cached queries
- **Batch Processing**: 500 messages per batch, 10ms flush interval
- **Memory Usage**: Optimized skip-list data structures

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_HOST` | `127.0.0.1` | Server bind address |
| `SERVER_PORT` | `8080` | Server port |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `postgres` | PostgreSQL username |
| `DB_PASSWORD` | `postgres` | PostgreSQL password |
| `DB_NAME` | `leaderboard` | PostgreSQL database name |
| `KAFKA_BROKERS` | `localhost:9092` | Kafka broker addresses |
| `KAFKA_SCORES_TOPIC_PREFIX` | `leaderboard-scores` | Topic name for scores |
| `KAFKA_CONSUMER_GROUP` | `score-processor` | Base consumer group |
| `KAFKA_BATCH_SIZE` | `5000` | Consumer batch size |
| `KAFKA_BATCH_TIMEOUT` | `5` | Consumer batch timeout (seconds) |

## Development

### Prerequisites

- Go 1.24+
- Docker & Docker Compose
- PostgreSQL 17+
- Kafka 2.8+

### Local Development

```bash
# Clone repository
git clone <repository-url>
cd leaderboard-realtime

# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build -o leaderboard ./cmd/leaderboard

# Run locally (requires PostgreSQL and Kafka)
./leaderboard
```

### Load Testing

```bash
# Install wrk
sudo apt-get install wrk

# Test score submission
wrk -t12 -c400 -d30s -s scripts/wrk/score_post.lua http://localhost/api

# Test leaderboard queries
wrk -t12 -c400 -d30s -s scripts/wrk/get_top_leaders.lua http://localhost/api
```

## Monitoring

- **Health Check**: `GET /api/health`
- **Metrics**: Kafka producer/consumer metrics logged every 30 seconds
- **Logs**: Structured logging with request tracing

## License

MIT License

