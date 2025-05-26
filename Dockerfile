FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o leaderboard-service ./cmd/leaderboard

FROM alpine:3.21

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/leaderboard-service .

RUN mkdir -p /app/data/wal

ENV TZ=UTC

EXPOSE 8080

CMD ["/app/leaderboard-service"] 