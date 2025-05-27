.PHONY: \
	build \
	run \
	dev \
	clean \
	test \
	swagger-docs \
	k6_stress \
	wrk_stress \
	wrk_read_stress \
	wrk_write_stress \
	build_and_push \
	docker_wrk_stress \
	docker_wrk_read_stress \
	docker_wrk_write_stress \
	docker_wrk_up \
	docker_wrk_down \
	local_infra_up \
	local_infra_down \
	prod_infra_up \
	prod_infra_down

# Go build flags
CUR_DIR = $(shell pwd)
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
BUILD_DIR ?= ./bin

# Application info
APP_NAME = leaderboard
DOCKER_REGISTRY = iwhitebird

# Local

dev:
	@echo "Starting development server with hot reload..."
	@air

build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/leaderboard

run: build
	@echo "Running $(APP_NAME)..."
	@$(BUILD_DIR)/$(APP_NAME)


clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -rf tmp
	@rm -rf docs

test:
	@echo "Running tests..."
	@go test -v ./...

swagger-docs:
	@echo "Generating Swagger documentation..."
	@swag init -g ./cmd/leaderboard/main.go -o ./docs


# Local Stress Tests

k6_stress:
	@echo "Running stress test..."
	@k6 run --out dashboard=export=./scripts/k6/test-report.html ./scripts/k6/k6-loadtest.js


wrk_stress:
	@echo "Running stress test in parallel with clean output..."
	@mkdir -p logs
	@parallel ::: \
		"wrk -t6 -c10000 -d5s -s ./scripts/wrk/score_post.lua http://localhost:8080 > logs/score_post.txt" \
		"wrk -t6 -c2500 -d5s -s ./scripts/wrk/get_top_leaders.lua http://localhost:8080 > logs/get_top_leaders.txt" \
		"wrk -t6 -c2500 -d5s -s ./scripts/wrk/get_user_rank.lua http://localhost:8080 > logs/get_user_rank.txt"
	@echo "\n\033[1;34m=== score_post.lua ===\033[0m"; cat logs/score_post.txt
	@echo "\n\033[1;32m=== get_top_leaders.lua ===\033[0m"; cat logs/get_top_leaders.txt
	@echo "\n\033[1;35m=== get_user_rank.lua ===\033[0m"; cat logs/get_user_rank.txt

wrk_read_stress:
	@echo "Running read stress test in parallel with clean output..."
	@mkdir -p logs
	@parallel ::: \
		"wrk -t6 -c2500 -d5s -s ./scripts/wrk/get_top_leaders.lua http://localhost:8080 > logs/get_top_leaders.txt" \
		"wrk -t6 -c2500 -d5s -s ./scripts/wrk/get_user_rank.lua http://localhost:8080 > logs/get_user_rank.txt"
	@echo "\n\033[1;32m=== get_top_leaders.lua ===\033[0m"; cat logs/get_top_leaders.txt
	@echo "\n\033[1;35m=== get_user_rank.lua ===\033[0m"; cat logs/get_user_rank.txt

wrk_write_stress:
	@echo "Running write stress test in parallel with clean output..."
	@mkdir -p logs
	@parallel ::: \
		"wrk -t6 -c10000 -d30s -s ./scripts/wrk/score_post.lua http://localhost:8080 > logs/score_post.txt"
	@echo "\n\033[1;34m=== score_post.lua ===\033[0m"; cat logs/score_post.txt


# Docker

build_and_push:
	@echo "Building and pushing $(APP_NAME)..."
	@cd docker/prod
	@docker build -t $(DOCKER_REGISTRY)/$(APP_NAME):latest .
	@docker push $(DOCKER_REGISTRY)/$(APP_NAME):latest

local_infra_up:
	@echo "Starting local infrastructure..."
	@docker compose -f docker/local/docker-compose.yml up -d

local_infra_down:
	@echo "Stopping local infrastructure..."
	@docker compose -f docker/local/docker-compose.yml down

prod_infra_up:
	@echo "Starting production infrastructure..."
	@docker compose -f docker/prod/docker-compose.yml up -d

prod_infra_down:
	@echo "Stopping production infrastructure..."
	@docker compose -f docker/prod/docker-compose.yml down

# Docker Stress Tests

docker_wrk_stress:
	@echo "Running Docker WRK stress test..."
	@docker compose -f docker-compose.stress.yml up -d
	@docker compose -f docker-compose.stress.yml exec wrk-tester ./docker-wrk-stress.sh stress
	@docker compose -f docker-compose.stress.yml down

docker_wrk_read_stress:
	@echo "Running Docker WRK read stress test..."
	@docker compose -f docker-compose.stress.yml up -d
	@docker compose -f docker-compose.stress.yml exec wrk-tester ./docker-wrk-stress.sh read
	@docker compose -f docker-compose.stress.yml down

docker_wrk_write_stress:
	@echo "Running Docker WRK write stress test..."
	@docker compose -f docker-compose.stress.yml up -d
	@docker compose -f docker-compose.stress.yml exec wrk-tester ./docker-wrk-stress.sh write
	@docker compose -f docker-compose.stress.yml down
