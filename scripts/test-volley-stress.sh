#!/bin/bash
# Test script for volley progress display with complex multi-task scenario
# Simulates a large project with multiple tasks and many tool calls

set -e

# Determine cliffy binary location
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CLIFFY_BIN="${CLIFFY_BIN:-$PROJECT_ROOT/bin/cliffy}"

# Check if cliffy binary exists
if [ ! -f "$CLIFFY_BIN" ]; then
    echo "Error: cliffy binary not found at $CLIFFY_BIN"
    echo "Please build it first with: go build -o bin/cliffy ./cmd/cliffy"
    exit 1
fi

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  Cliffy Volley Stress Test: NY Jets Super Bowl Blueprint      ║${NC}"
echo -e "${BLUE}║  (A wonderfully implausible scenario)                          ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${YELLOW}This test simulates 5 complex tasks, each requiring multiple tool calls.${NC}"
echo -e "${YELLOW}Expected duration: ~90 seconds${NC}"
echo ""

# Create a temporary test directory
TEST_DIR=$(mktemp -d -t cliffy-test-XXXXXX)
echo -e "${GREEN}Created test directory: ${TEST_DIR}${NC}"

# Change to test directory
cd "$TEST_DIR"

# Initialize a fake Jets project structure
mkdir -p {roster,playbook,facilities,coaching,analytics}

echo -e "\n${BLUE}Starting volley execution...${NC}\n"

# Run the volley with 5 complex tasks
"$CLIFFY_BIN" --verbose \
  --context "You are helping build the NY Jets Super Bowl championship team. Be creative and detailed. For each task, create multiple files with realistic content. Use bash commands to set up directory structures and git repos where appropriate." \
  "Create a complete roster analysis in roster/. Include: 1) quarterback depth chart with stats, 2) offensive line evaluation, 3) defensive secondary improvements needed, 4) special teams assessment. Create separate markdown files for each position group with detailed player evaluations and salary cap implications." \
  "Design an innovative playbook in playbook/. Create: 1) offensive-scheme.md with revolutionary passing concepts, 2) defensive-formations.md with exotic blitz packages, 3) red-zone-plays.md with guaranteed scoring plays, 4) two-minute-drill.md with clutch time strategies. Include diagrams using ASCII art." \
  "Plan state-of-the-art facilities upgrade in facilities/. Document: 1) training-center.md with cutting-edge equipment, 2) recovery-spa.md with cryotherapy and hyperbaric chambers, 3) film-room.md with AI-powered analysis systems, 4) locker-room.md with championship-caliber amenities. Include budget estimates." \
  "Assemble dream coaching staff in coaching/. Create files for: 1) head-coach-profile.md (proven winner), 2) offensive-coordinator.md (genius play-caller), 3) defensive-coordinator.md (defensive mastermind), 4) special-teams-coach.md (hidden game expert), 5) sports-psychologist.md (mental edge specialist). Include their philosophies and track records." \
  "Build advanced analytics system in analytics/. Create: 1) player-performance-metrics.py with statistical models, 2) opponent-analysis.py for scouting reports, 3) game-simulation.py for scenario planning, 4) draft-evaluation.py for talent assessment, 5) injury-prediction.py for player health monitoring. Include sample data and visualizations."

EXIT_CODE=$?

echo -e "\n${BLUE}Volley execution complete!${NC}\n"

# Show what was created
echo -e "${GREEN}Files created in test directory:${NC}"
find . -type f | head -30

echo -e "\n${YELLOW}Test directory contents (first 20 lines of tree):${NC}"
if command -v tree &> /dev/null; then
    tree -L 3 | head -20
else
    find . -type d | head -20
fi

# Show some stats
FILE_COUNT=$(find . -type f | wc -l | tr -d ' ')
DIR_COUNT=$(find . -type d | wc -l | tr -d ' ')

echo -e "\n${GREEN}Summary Statistics:${NC}"
echo -e "  Files created: ${FILE_COUNT}"
echo -e "  Directories: ${DIR_COUNT}"
echo -e "  Test location: ${TEST_DIR}"

if [ $EXIT_CODE -eq 0 ]; then
    echo -e "\n${GREEN}✓ Test completed successfully!${NC}"
    echo -e "${YELLOW}Note: Test files are in ${TEST_DIR}${NC}"
    echo -e "${YELLOW}Clean up with: rm -rf ${TEST_DIR}${NC}"
else
    echo -e "\n${YELLOW}⚠ Test completed with errors (exit code: ${EXIT_CODE})${NC}"
fi

exit $EXIT_CODE
