FROM golang:1.23-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

#Optimized static binary:
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -trimpath \
    -ldflags="-s -w -extldflags '-static'" \
    -o leaderboard-service ./cmd/leaderboard


FROM alpine:3.21

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/leaderboard-service .

RUN mkdir -p /app/data/wal

ENV TZ=UTC

EXPOSE 8080

CMD ["/app/leaderboard-service"]
