# Go Leaderboard Service

A high-performance, real-time leaderboard service built with Go, featuring Kafka-based message queuing, PostgreSQL persistence, and in-memory caching for optimal read/write performance.

## Performance Metrics

Our service delivers exceptional performance under load:

This test running 2 read and 1 write api concurrently and was performed running the wrk locally and everything else in docker.

![Load Test Results](assets/load_test.png)

**Single Instance Performance:**
- **Separate Operations**: ~25k+ writes/sec, ~30k+ reads/sec
- **Concurrent Operations**: ~15k+ writes/sec, ~13k+ reads/sec



## Architecture Overview

The service implements a hybrid architecture optimized for high-throughput scenarios:

### Core Components

- **Kafka Message Queue**: Handles ~5,000 Scores/message queue length for high-throughput asynchronous writes
- **PostgreSQL**: Primary persistent storage with optimized indexes
- **In-Memory Cache**: Skip list implementation for ultra-fast lookups and ranking operations
- **Real-time Updates**: Automatic cache synchronization with database changes

### Data Flow

**Write Operations**: Client → Kafka → PostgreSQL → In-Memory Cache Update

**Read Operations**: Client → In-Memory Cache (Skip List) → Response

## Prerequisites

### Production Deployment
- Docker & Docker Compose

### Local Development
- Go 1.24+
- Docker & Docker Compose
- wrk or k6 (for load testing)

## Quick Start

### 1. Clone and Configure

```bash
git clone https://github.com/iwhitebird/go-leader-board
cd go-leader-board
cp .env.example .env
# Configure .env according to your requirements
```

### 2. Running the service

Direct using docker (Recommended)

```bash
make prod_infra_up
```

#### Or

Running infra on docker and server Locally.

Start database and kafka:
```bash
make local_infra_up
```

Install dependencies and run:
```bash
go mod tidy #or go mod download
```
Optionally generate documentation

```bash
make swagger-docs
```

build and run the service 

```bash
make run # Or with ""air" if you have go-air installed
```

## API Reference

### Endpoints

| Method | Endpoint | Description | Complexity |
|--------|----------|-------------|------------|
| `GET` | `/api/health` | Health check | O(1) |
| `POST` | `/api/leaderboard/score` | Submit player score | O(log n) |
| `GET` | `/api/leaderboard/top/{gameId}` | Get top players | O(k) |
| `GET` | `/api/leaderboard/rank/{gameId}/{userId}` | Get player rank | O(log n) |

### Query Parameters

The leaderboard endpoints support an optional `window` parameter:
- `24h` - Last 24 hours
- `3d` - Last 3 days  
- `7d` - Last 7 days
- Default: All time

### API Documentation

Interactive API documentation is available at `http://localhost:8080/swagger/index.html`

## Load Testing

### Using wrk

```bash
sudo apt-get install wrk parallel

# Combined read/write stress test
make wrk_stress

# Individual tests
make wrk_read_stress
make wrk_write_stress

#I have also provided the wrk docker for testing not recommended it uses docker internal gateway which is slow.
#make docker_wrk_stress:
#make docker_wrk_read_stress
#make docker_wrk_write_stress
```

## Technical Implementation

### Caching Strategy

The service uses a multi-level caching approach:

1. **Skip List with map for Cache**: O(log n) insertions and lookups
2. **Request Caching**: Reduces repeated query overhead
3. **Time-based Partitioning**: Separate skip lists for different time windows

### Data Consistency

- **Write Path**: Eventual consistency through Kafka
- **Durability**: PostgreSQL ensures data persistence
- **Availability**: On startup service fetches data from postgres and keeps re-create cache's parallely for faster performance.

### Optimizations

- **Batch Processing**: Kafka enables efficient batch writes
- **Connection Pooling**: We are using pool of connecitons for faster and parallel writes to database.
- **Channel Queuing**: Go channels buffer Kafka messages for improved throughput
- **Concurrent Processing**: Lock-free dirty-reads with minimal write contention
- **Index Optimization**: PostgreSQL indexes on frequently queried fields

## Future Enhancements & Notes

### Scalability Improvements
- **Load Balancer**: Consistent hashing for multi-node deployment
- **Stateless Design**: External cache service for stateless horizontal scaling
- **Database Sharding**: Sharding db with something like CockroachDB with game_id 
- **Kafka Partitioning**: Topic partitioning by game_id for reduced lock contention while writing to db and updating cache.

## Trade-offs and Design Decisions

1. **Eventual Consistency**: Prioritizes write performance over immediate consistency
2. **Memory Usage**: In-memory caching trades memory for read performance
3. **Dirty Reads**: Accepts potential read inconsistency for higher throughput
4. **Channel Buffering with batch writes to kafka**: Improves performance but introduces potential message loss risk
5. **Time Window Separation**: Multiple skip lists increase memory usage but improve query performance


## Architecture Limitations & Production Considerations

Here are some things that need attention to make this system truly production-ready.

### Current Scaling Challenges

Our architecture has a fundamental limitation - the in-memory cache makes horizontal scaling tricky. The obvious fix would be to separate the cache into its own service, making our app stateless and much easier to scale horizontally. This will add network overhead.

Alternatively, we could stick with the current approach but use a game_id-based load balancer so the same requests always hit the same servers. Not as scalable, but it could work for moderate loads. but this complicates our load balancer and doesn't scale well with consistant hashing.

### Kafka Optimization Opportunities

We should configure Kafka so that message batches (~5000 messages) contain scores for the same game_id. This would significantly speed up writes since we'd have better data locality.

### Database Considerations

Sharding the database by game_id using cockroachdb would increase our startup latences and eventual writes. or use write heavy databases with direct partitioning support like scylladb or cassandra.

### Time Window Data Management

There's an interesting challenge with our time-based windows. We store separate skip lists for different time ranges, but over time these will accumulate stale data that falls outside the window (except for the "all" case).

Our current structure looks like:
```
map[game_id] -> [all]  -> ['All Skiplist']
                [24h]  -> ['24h Skiplist'] 
                [72hr] -> ['72hr Skiplist']
```

The time-based lists are periodically cleaned up to remove expired entries.
