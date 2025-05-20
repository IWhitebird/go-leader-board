# Leaderboard Testing Guide

This document provides information on how to run tests for the Go Leaderboard service.

## Prerequisites

Before running tests, make sure you have the following installed:

- Go 1.16 or higher
- Docker and Docker Compose (for integration tests with PostgreSQL)
- K6 (for load testing)

## System Architecture

The leaderboard service now uses the following components:

1. **SkipList Data Structure**: A highly efficient O(log N) data structure for ranking operations
2. **Write-Ahead Log (WAL)**: For data durability and recovery
3. **Flexible Time Windows**: Support for multiple time windows (24h, 3d, 7d, etc.)

## Unit Tests

Unit tests cover the core functionality of the application without external dependencies.

### Running Unit Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## Integration Tests

Integration tests verify that the components work together correctly, including the API endpoints.

```bash
# Run integration tests
go test -tags=integration ./...
```

## Performance Testing

We provide two options for performance testing:

### 1. Simple Benchmark with Hey

[Hey](https://github.com/rakyll/hey) is a simple HTTP load generator that can be used for quick benchmarks.

```bash
# Install Hey
go install github.com/rakyll/hey@latest

# Make the benchmark script executable
chmod +x benchmark.sh

# Run the benchmark script
./benchmark.sh
```

The script will:
- Check if the server is running and start it if needed
- Submit initial test data
- Run benchmarks on key endpoints
- Display results

### 2. Advanced Load Testing with K6

[K6](https://k6.io/) is a modern load testing tool that provides more detailed metrics and scenarios.

```bash
# Install K6 (see https://k6.io/docs/getting-started/installation/)

# Run the K6 load test
k6 run k6-loadtest.js
```

The K6 script simulates:
- Concurrent users submitting scores (write operations)
- Concurrent users fetching top leaders (read operations)
- Concurrent users checking player ranks (read operations)

It collects metrics on:
- Request latency
- Error rates
- Request counts

## Storage Implementations

The leaderboard service has two storage implementations:

### 1. In-Memory Storage with WAL

The in-memory storage uses:
- **SkipList**: A probabilistic data structure that allows O(log N) operations for insertions and lookups
- **Write-Ahead Log (WAL)**: For durability and crash recovery
- **Periodic Snapshots**: For faster recovery

This approach prioritizes speed while maintaining durability through the WAL.

### 2. PostgreSQL Persistence

The service can also persist data to PostgreSQL for long-term storage and analysis.

## Time Window Support

The leaderboard supports flexible time windows:

- `all time` (default): All scores ever recorded
- `24h`: Scores from the last 24 hours
- `3d`: Scores from the last 3 days
- `7d`: Scores from the last 7 days

Custom time windows can also be specified using the format `{number}h` for hours or `{number}d` for days.

## Test Coverage

The tests cover:

1. **Unit Tests**:
   - SkipList implementation
   - Score sorting and ranking
   - Time-based windowing
   - Write-Ahead Log operations

2. **API Tests**:
   - Submit score endpoint
   - Get top leaders endpoint
   - Get player rank endpoint
   - Error handling
   - Time window query parameters

3. **Load Tests**:
   - System behavior under concurrent load
   - Performance metrics (P95 latency, throughput)
   - Stability over time

## Interpreting Results

When running the load tests, focus on these key metrics:

- **P95 Latency**: 95% of requests should complete faster than this time
  - Submit score: < 200ms
  - Get top leaders: < 50ms
  - Get player rank: < 50ms

- **Error Rate**: Should be < 1% for all operations

- **Throughput**: The system should handle:
  - At least 5,000 score submissions per second on 4 cores
  - At least 10,000 read operations per second on 4 cores

## Troubleshooting

If you encounter issues:

1. **Database connectivity**: Ensure PostgreSQL is running with `docker-compose up -d`
2. **Port conflicts**: Make sure port 8080 is available or configure a different port
3. **Performance issues**: Check CPU/memory usage during tests with `top` or `htop` 