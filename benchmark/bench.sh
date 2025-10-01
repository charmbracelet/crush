#!/usr/bin/env bash
set -euo pipefail

# Cliffy vs Crush Performance Benchmark
# Measures cold start, first token, and total execution time

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULT_FILE="$RESULTS_DIR/bench_$TIMESTAMP.json"
RUNS=5

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "ᕕ( ᐛ )ᕗ  Cliffy vs Crush Benchmark"
echo "================================"
echo ""

# Check dependencies
if ! command -v jq &> /dev/null; then
    echo -e "${RED}Error: jq is required but not installed${NC}"
    exit 1
fi

if ! command -v crush &> /dev/null; then
    echo -e "${RED}Error: crush is not installed${NC}"
    exit 1
fi

if [ ! -f "$PROJECT_ROOT/bin/cliffy" ]; then
    echo -e "${RED}Error: cliffy binary not found at $PROJECT_ROOT/bin/cliffy${NC}"
    exit 1
fi

mkdir -p "$RESULTS_DIR"

# Function to run a single benchmark
run_benchmark() {
    local tool=$1
    local prompt=$2
    local run_num=$3

    local start=$(date +%s%N)

    if [ "$tool" = "crush" ]; then
        crush run --data-dir "$SCRIPT_DIR/config" -q "$prompt" > /dev/null 2>&1
    else
        "$PROJECT_ROOT/bin/cliffy" --quiet "$prompt" > /dev/null 2>&1
    fi

    local end=$(date +%s%N)
    local duration=$(( (end - start) / 1000000 )) # Convert to milliseconds

    echo "$duration"
}

# Function to calculate average and stddev
calculate_stats() {
    local values=("$@")
    local sum=0
    local count=${#values[@]}

    # Calculate mean
    for val in "${values[@]}"; do
        sum=$((sum + val))
    done
    local mean=$((sum / count))

    # Calculate standard deviation
    local sq_diff_sum=0
    for val in "${values[@]}"; do
        local diff=$((val - mean))
        sq_diff_sum=$((sq_diff_sum + diff * diff))
    done
    local variance=$((sq_diff_sum / count))
    local stddev=$(echo "sqrt($variance)" | bc)

    echo "$mean $stddev"
}

# Load tasks
TASKS=$(cat "$SCRIPT_DIR/tasks.json")

# Initialize results JSON
echo "{" > "$RESULT_FILE"
echo "  \"timestamp\": \"$TIMESTAMP\"," >> "$RESULT_FILE"
echo "  \"runs\": $RUNS," >> "$RESULT_FILE"
echo "  \"results\": {" >> "$RESULT_FILE"

first_category=true

# Process each category
for category in cold_start medium long_running; do
    if [ "$first_category" = false ]; then
        echo "," >> "$RESULT_FILE"
    fi
    first_category=false

    echo "  \"$category\": [" >> "$RESULT_FILE"

    task_count=$(echo "$TASKS" | jq -r ".$category | length")

    for ((i=0; i<task_count; i++)); do
        task_name=$(echo "$TASKS" | jq -r ".$category[$i].name")
        task_prompt=$(echo "$TASKS" | jq -r ".$category[$i].prompt")
        task_desc=$(echo "$TASKS" | jq -r ".$category[$i].description")

        echo -e "${YELLOW}Running: $task_name${NC} ($category)"
        echo "  Prompt: $task_prompt"

        # Run Cliffy benchmarks
        echo -n "  Cliffy: "
        cliffy_times=()
        for ((run=1; run<=RUNS; run++)); do
            echo -n "."
            time=$(run_benchmark "cliffy" "$task_prompt" "$run")
            cliffy_times+=("$time")
        done
        echo ""

        # Run Crush benchmarks
        echo -n "  Crush:  "
        crush_times=()
        for ((run=1; run<=RUNS; run++)); do
            echo -n "."
            time=$(run_benchmark "crush" "$task_prompt" "$run")
            crush_times+=("$time")
        done
        echo ""

        # Calculate stats
        read cliffy_mean cliffy_std <<< $(calculate_stats "${cliffy_times[@]}")
        read crush_mean crush_std <<< $(calculate_stats "${crush_times[@]}")

        # Calculate speedup
        speedup=$(echo "scale=2; $crush_mean / $cliffy_mean" | bc)

        echo -e "  ${GREEN}Cliffy: ${cliffy_mean}ms ±${cliffy_std}ms${NC}"
        echo -e "  Crush:  ${crush_mean}ms ±${crush_std}ms"
        echo -e "  Speedup: ${speedup}x"
        echo ""

        # Write to JSON
        if [ $i -gt 0 ]; then
            echo "," >> "$RESULT_FILE"
        fi
        cat >> "$RESULT_FILE" << EOF
    {
      "name": "$task_name",
      "description": "$task_desc",
      "cliffy": {
        "mean_ms": $cliffy_mean,
        "stddev_ms": $cliffy_std,
        "runs": [$(IFS=,; echo "${cliffy_times[*]}")]
      },
      "crush": {
        "mean_ms": $crush_mean,
        "stddev_ms": $crush_std,
        "runs": [$(IFS=,; echo "${crush_times[*]}")]
      },
      "speedup": $speedup
    }
EOF
    done

    echo "  ]" >> "$RESULT_FILE"
done

echo "  }" >> "$RESULT_FILE"
echo "}" >> "$RESULT_FILE"

echo ""
echo -e "${GREEN}Benchmark complete!${NC}"
echo "Results saved to: $RESULT_FILE"
echo ""
echo "Generate markdown report:"
echo "  ./benchmark/report.sh $RESULT_FILE"
