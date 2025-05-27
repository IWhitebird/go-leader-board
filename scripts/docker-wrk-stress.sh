#!/bin/bash

# Docker WRK Stress Testing Script
# This script runs inside the wrk-tester container and targets the leaderboard service

LEADERBOARD_URL=${LEADERBOARD_URL:-"http://host.docker.internal:8080"}

echo "Starting Docker WRK stress tests against: $LEADERBOARD_URL"
echo "Creating logs directory..."
mkdir -p logs

# Function to run stress tests
run_stress_test() {
    echo "Running stress test in parallel with clean output..."
    
    parallel ::: \
        "wrk -t6 -c10000 -d5s -s ./scripts/wrk/score_post.lua $LEADERBOARD_URL > logs/score_post.txt" \
        "wrk -t6 -c2500 -d5s -s ./scripts/wrk/get_top_leaders.lua $LEADERBOARD_URL > logs/get_top_leaders.txt" \
        "wrk -t6 -c2500 -d5s -s ./scripts/wrk/get_user_rank.lua $LEADERBOARD_URL > logs/get_user_rank.txt"
    
    echo -e "\n\033[1;34m=== score_post.lua ===\033[0m"
    cat logs/score_post.txt
    echo -e "\n\033[1;32m=== get_top_leaders.lua ===\033[0m"
    cat logs/get_top_leaders.txt
    echo -e "\n\033[1;35m=== get_user_rank.lua ===\033[0m"
    cat logs/get_user_rank.txt
}

# Function to run read stress tests
run_read_stress_test() {
    echo "Running read stress test in parallel with clean output..."
    
    parallel ::: \
        "wrk -t6 -c2500 -d5s -s ./scripts/wrk/get_top_leaders.lua $LEADERBOARD_URL > logs/get_top_leaders.txt" \
        "wrk -t6 -c2500 -d5s -s ./scripts/wrk/get_user_rank.lua $LEADERBOARD_URL > logs/get_user_rank.txt"
    
    echo -e "\n\033[1;32m=== get_top_leaders.lua ===\033[0m"
    cat logs/get_top_leaders.txt
    echo -e "\n\033[1;35m=== get_user_rank.lua ===\033[0m"
    cat logs/get_user_rank.txt
}

# Function to run write stress tests
run_write_stress_test() {
    echo "Running write stress test in parallel with clean output..."
    
    parallel ::: \
        "wrk -t6 -c10000 -d30s -s ./scripts/wrk/score_post.lua $LEADERBOARD_URL > logs/score_post.txt"
    
    echo -e "\n\033[1;34m=== score_post.lua ===\033[0m"
    cat logs/score_post.txt
}

# Check command line arguments
case "$1" in
    "stress")
        run_stress_test
        ;;
    "read")
        run_read_stress_test
        ;;
    "write")
        run_write_stress_test
        ;;
    *)
        echo "Usage: $0 {stress|read|write}"
        echo "  stress - Run full stress test (read + write)"
        echo "  read   - Run read-only stress test"
        echo "  write  - Run write-only stress test"
        echo ""
        echo "Or run individual wrk commands manually:"
        echo "  wrk -t6 -c2500 -d5s -s ./scripts/wrk/get_top_leaders.lua $LEADERBOARD_URL"
        echo "  wrk -t6 -c2500 -d5s -s ./scripts/wrk/get_user_rank.lua $LEADERBOARD_URL"
        echo "  wrk -t6 -c10000 -d5s -s ./scripts/wrk/score_post.lua $LEADERBOARD_URL"
        exit 1
        ;;
esac 