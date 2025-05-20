.PHONY: build run dev clean test swagger-docs stress

# Go build flags
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
BUILD_DIR ?= ./bin

# Application info
APP_NAME = leaderboard-service
VERSION ?= 1.0.0

# Docker compose files
DOCKER_COMPOSE = docker-compose.yml

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(APP_NAME) .

# Run the application
run: build
	@echo "Running $(APP_NAME)..."
	@$(BUILD_DIR)/$(APP_NAME)

# Run with hot reloading using Air
dev:
	@echo "Starting development server with hot reload..."
	@air

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -rf tmp

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Generate Swagger documentation
swagger-docs:
	@echo "Generating Swagger documentation..."
	@swag init


stress:
	@echo "Running stress test..."
	@docker compose -f docker-compose.stress.yml up
