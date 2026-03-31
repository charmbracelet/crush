#!/bin/bash
# Profile helper script for Crush
# Usage: ./scripts/profile.sh [mode] [command...]
#
# Modes:
#   cpu       - CPU profiling only
#   mem       - Memory profiling only
#   trace     - Execution trace only
#   full      - All profiles (CPU, memory, trace)
#   http      - HTTP pprof server (for live profiling)
#   benchmark - Run benchmarks with profiling
#
# Examples:
#   ./scripts/profile.sh cpu run "your task"
#   ./scripts/profile.sh full run "your task"
#   ./scripts/profile.sh http
#   ./scripts/profile.sh benchmark

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

MODE=${1:-}
shift || true

# Create profiles directory
PROFILE_DIR="profiles/$(date +%Y%m%d_%H%M%S)"
mkdir -p "$PROFILE_DIR"

# Binary path
BINARY="./crush"

echo -e "${BLUE}Crush Profiling Tool${NC}"
echo -e "${BLUE}===================${NC}\n"

# Check if binary exists
if [ ! -f "$BINARY" ]; then
    echo -e "${YELLOW}Binary not found. Building...${NC}"
    go build -o "$BINARY"
fi

case "$MODE" in
    cpu)
        echo -e "${GREEN}Running with CPU profiling${NC}"
        CPU_PROF="$PROFILE_DIR/cpu.prof"
        "$BINARY" --cpuprofile="$CPU_PROF" "$@"
        echo -e "\n${GREEN}✓ CPU profile saved to: $CPU_PROF${NC}"
        echo -e "${BLUE}Analyze with: go tool pprof -http=:8080 $CPU_PROF${NC}"
        ;;

    mem)
        echo -e "${GREEN}Running with memory profiling${NC}"
        MEM_PROF="$PROFILE_DIR/mem.prof"
        "$BINARY" --memprofile="$MEM_PROF" "$@"
        echo -e "\n${GREEN}✓ Memory profile saved to: $MEM_PROF${NC}"
        echo -e "${BLUE}Analyze with: go tool pprof -http=:8080 $MEM_PROF${NC}"
        ;;

    trace)
        echo -e "${GREEN}Running with execution trace${NC}"
        TRACE_FILE="$PROFILE_DIR/trace.out"
        "$BINARY" --trace="$TRACE_FILE" "$@"
        echo -e "\n${GREEN}✓ Trace saved to: $TRACE_FILE${NC}"
        echo -e "${BLUE}Analyze with: go tool trace $TRACE_FILE${NC}"
        ;;

    full)
        echo -e "${GREEN}Running with full profiling (CPU + Memory + Trace)${NC}"
        CPU_PROF="$PROFILE_DIR/cpu.prof"
        MEM_PROF="$PROFILE_DIR/mem.prof"
        TRACE_FILE="$PROFILE_DIR/trace.out"
        "$BINARY" \
            --cpuprofile="$CPU_PROF" \
            --memprofile="$MEM_PROF" \
            --trace="$TRACE_FILE" \
            --blockprofile-rate=1 \
            --mutexprofile-frac=1 \
            "$@"
        echo -e "\n${GREEN}✓ Profiles saved to: $PROFILE_DIR${NC}"
        echo -e "  - CPU: $CPU_PROF"
        echo -e "  - Memory: $MEM_PROF"
        echo -e "  - Trace: $TRACE_FILE"
        echo -e "\n${BLUE}Analyze with:${NC}"
        echo -e "  CPU:    go tool pprof -http=:8080 $CPU_PROF"
        echo -e "  Memory: go tool pprof -http=:8080 $MEM_PROF"
        echo -e "  Trace:  go tool trace $TRACE_FILE"
        ;;

    http)
        echo -e "${GREEN}Starting HTTP pprof server on localhost:6060${NC}"
        echo -e "${BLUE}Available endpoints:${NC}"
        echo -e "  http://localhost:6060/debug/pprof/"
        echo -e "\n${BLUE}Quick commands:${NC}"
        echo -e "  CPU (30s):    go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30"
        echo -e "  Heap:         go tool pprof http://localhost:6060/debug/pprof/heap"
        echo -e "  Goroutines:   go tool pprof http://localhost:6060/debug/pprof/goroutine"
        echo -e "  Block:        go tool pprof http://localhost:6060/debug/pprof/block"
        echo -e "  Mutex:        go tool pprof http://localhost:6060/debug/pprof/mutex"
        echo -e "\n${YELLOW}Press Ctrl+C to stop${NC}\n"
        CRUSH_PROFILE=1 "$BINARY" "$@"
        ;;

    benchmark)
        echo -e "${GREEN}Running benchmarks with profiling${NC}"
        CPU_PROF="$PROFILE_DIR/cpu.prof"
        MEM_PROF="$PROFILE_DIR/mem.prof"
        BENCH_RESULT="$PROFILE_DIR/bench.txt"

        go test \
            -bench=. \
            -benchmem \
            -cpuprofile="$CPU_PROF" \
            -memprofile="$MEM_PROF" \
            ./internal/... \
            | tee "$BENCH_RESULT"

        echo -e "\n${GREEN}✓ Benchmark results and profiles saved to: $PROFILE_DIR${NC}"
        echo -e "  - Results: $BENCH_RESULT"
        echo -e "  - CPU: $CPU_PROF"
        echo -e "  - Memory: $MEM_PROF"
        echo -e "\n${BLUE}Analyze with:${NC}"
        echo -e "  go tool pprof -http=:8080 $CPU_PROF"
        echo -e "  go tool pprof -http=:8080 $MEM_PROF"
        ;;

    compare)
        echo -e "${GREEN}Memory leak detection (comparison mode)${NC}"
        if [ "$#" -lt 1 ]; then
            echo -e "${RED}Error: Please provide a command to run${NC}"
            echo -e "Usage: $0 compare run \"your task\""
            exit 1
        fi

        echo -e "${YELLOW}Starting HTTP pprof server...${NC}"
        CRUSH_PROFILE=1 "$BINARY" "$@" &
        PID=$!

        # Wait for server to start
        sleep 2

        echo -e "${BLUE}Collecting initial memory snapshot...${NC}"
        MEM1="$PROFILE_DIR/mem_initial.prof"
        curl -s http://localhost:6060/debug/pprof/heap > "$MEM1"
        echo -e "${GREEN}✓ Initial snapshot saved${NC}"

        echo -e "${YELLOW}Waiting 30 seconds for memory changes...${NC}"
        sleep 30

        echo -e "${BLUE}Collecting second memory snapshot...${NC}"
        MEM2="$PROFILE_DIR/mem_after.prof"
        curl -s http://localhost:6060/debug/pprof/heap > "$MEM2"
        echo -e "${GREEN}✓ Second snapshot saved${NC}"

        # Stop the application
        kill $PID 2>/dev/null || true

        echo -e "\n${GREEN}Memory snapshots saved to: $PROFILE_DIR${NC}"
        echo -e "  - Initial: $MEM1"
        echo -e "  - After:   $MEM2"
        echo -e "\n${BLUE}Compare with:${NC}"
        echo -e "  go tool pprof -base $MEM1 $MEM2"
        ;;

    *)
        echo -e "${RED}Unknown mode: $MODE${NC}\n"
        echo "Usage: $0 [mode] [command...]"
        echo ""
        echo "Modes:"
        echo "  cpu       - CPU profiling only"
        echo "  mem       - Memory profiling only"
        echo "  trace     - Execution trace only"
        echo "  full      - All profiles (CPU, memory, trace, block, mutex)"
        echo "  http      - HTTP pprof server (for live profiling)"
        echo "  benchmark - Run benchmarks with profiling"
        echo "  compare   - Memory leak detection (takes 2 snapshots)"
        echo ""
        echo "Examples:"
        echo "  $0 cpu run \"your task\""
        echo "  $0 full run \"your task\""
        echo "  $0 http"
        echo "  $0 benchmark"
        echo "  $0 compare run \"your task\""
        exit 1
        ;;
esac

echo -e "\n${GREEN}Done!${NC}"
