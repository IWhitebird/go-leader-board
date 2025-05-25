.PHONY: build run dev clean test swagger-docs k6_stress wrk_stress

# Go build flags
CUR_DIR = $(shell pwd)
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
BUILD_DIR ?= ./bin

# Application info
APP_NAME = leaderboard-service
VERSION ?= 1.0.0

# Docker compose files
DOCKER_COMPOSE = docker-compose.yml

dev:
	@echo "Starting development server with hot reload..."
	@air

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/leaderboard

# Run the application
run: build
	@echo "Running $(APP_NAME)..."
	@$(BUILD_DIR)/$(APP_NAME)


# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -rf tmp
	@rm -rf docs

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Generate Swagger documentation
swagger-docs:
	@echo "Generating Swagger documentation..."
	@swag init -g ./cmd/leaderboard/main.go -o ./docs


k6_stress:
	@echo "Running stress test..."
	@k6 run --out dashboard=export=./scripts/k6/test-report.html ./scripts/k6/k6-loadtest.js

wrk_stress:
	@echo "Running stress test in parallel with clean output..."
	@mkdir -p logs
	@parallel ::: \
		"wrk -t6 -c10000 -d5s -s ./scripts/wrk/score_post.lua http://localhost:80 > logs/score_post.txt" \
		"wrk -t6 -c5000 -d5s -s ./scripts/wrk/get_top_leaders.lua http://localhost:80 > logs/get_top_leaders.txt" \
		"wrk -t6 -c5000 -d5s -s ./scripts/wrk/get_user_rank.lua http://localhost:80 > logs/get_user_rank.txt"
	@echo "\n\033[1;34m=== score_post.lua ===\033[0m"; cat logs/score_post.txt
	@echo "\n\033[1;32m=== get_top_leaders.lua ===\033[0m"; cat logs/get_top_leaders.txt
	@echo "\n\033[1;35m=== get_user_rank.lua ===\033[0m"; cat logs/get_user_rank.txt
