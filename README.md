# Go Leaderboard Service

A high-performance, real-time leaderboard service built with Go, featuring Kafka-based message queuing, PostgreSQL persistence, and in-memory caching for optimal read/write performance.

## Performance Metrics

Our service delivers exceptional performance under load:

![Load Test Results](assets/load_test.png)

**Single Instance Performance:**
- **Separate Operations**: ~25k writes/sec, ~30k reads/sec
- **Concurrent Operations**: ~13-14k writes/sec, ~10k reads/sec

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

Direct using docker

```bash
docker compose up -d
```

#### Or

Running server Locally  

Start dependencies:
```bash
docker compose up postgres kafka -d
```

Install dependencies and run:
```bash
go mod download
```
Optionally generate documentation

```bash
make swagger-docs
```

build and run the service 

```bash
make run
```

For development with hot reload:
```bash
# Ensure go-air is installed
air
```

## API Reference

### Endpoints

| Method | Endpoint | Description | Complexity |
|--------|----------|-------------|------------|
| `GET` | `/api/health` | Health check | O(1) |
| `POST` | `/api/leaderboard/score/{gameId}` | Submit player score | O(log n) |
| `GET` | `/api/leaderboard/top/{gameId}` | Get top players | O(k) |
| `GET` | `/api/leaderboard/rank/{gameId}/{userId}` | Get player rank | O(log n) |

### Query Parameters

The leaderboard endpoints support an optional `window` parameter:
- `24h` - Last 24 hours
- `3d` - Last 3 days  
- `7d` - Last 7 days
- Default: All time

### API Documentation

Interactive API documentation is available at `http://localhost/swagger/index.html`

## Load Testing

### Using wrk

```bash
sudo apt-get install wrk parallel

# Combined read/write stress test
make wrk_stress

# Individual tests
make wrk_read_stress
make wrk_write_stress
```

### Using k6 (Not recommended)

```bash
make k6_stress
```

## Technical Implementation

### Caching Strategy

The service uses a multi-level caching approach:

1. **Skip List Cache**: O(log n) insertions and lookups
2. **User Rank Mapping**: O(1) rank and percentile queries
3. **Request Caching**: Reduces repeated query overhead
4. **Time-based Partitioning**: Separate skip lists for different time windows

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
- **Stateless Design**: External cache service for horizontal scaling
- **Database Sharding**: CockroachDB with game_id partitioning
- **Kafka Partitioning**: Topic partitioning by game_id for reduced lock contention

### Performance Optimizations
- **Compression**: Message compression for reduced network overhead
- **Monitoring**: Comprehensive metrics and alerting

## Trade-offs and Design Decisions

1. **Eventual Consistency**: Prioritizes write performance over immediate consistency
2. **Memory Usage**: In-memory caching trades memory for read performance
3. **Dirty Reads**: Accepts potential read inconsistency for higher throughput
4. **Channel Buffering with batch writes to kafka**: Improves performance but introduces potential message loss risk
5. **Time Window Separation**: Multiple skip lists increase memory usage but improve query performance


## Architecutre and Current faults

This are of the thigns that are wrong with out system and also need for making this app production level.


Our current architecture doesnt allow for horizontal scaling ealityt due to in-memorty cache . we shuld seprate our cache from memeorty making our app stateless then it can easlity horizontally scaled.

or even if we dont seperate our cache we can use game_id based load balancer so same request will hit same servers (altohugh this might not be as saclaable).

we should confighure kafka in such way that to our message batch (~5000) should have all same game_id this will help with faster writes.

We should shard databse on game_id basis.

Currently our architecutrre is stroing seprate skiplist for time our window . so as time goesa this window will get dirty data that falls out of time range atleast (not for the `all` case) . we will need to handle this data time to time manually cleaning it up.
- map [game_id] -> [all] -> ['All Skiplist']
                    [24h] -> ['24h Skiplist']
                    ['72hr'] -> ['72hr Skiplist']
