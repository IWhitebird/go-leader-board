FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o leaderboard-service ./cmd/leaderboard

# Use a smaller image for the final container
FROM alpine:3.21

WORKDIR /app

# Install dependencies required for runtime
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary from the builder stage
COPY --from=builder /app/leaderboard-service .

# Create necessary directories
RUN mkdir -p /app/data/wal

# Set timezone
ENV TZ=UTC

# Expose port
EXPOSE 8080

# Command to run
CMD ["/app/leaderboard-service"] 