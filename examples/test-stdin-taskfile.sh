#!/usr/bin/env bash
# Test script for STDIN and task file support
# This demonstrates all the new input methods for cliffy

set -e

CLIFFY="./bin/cliffy"

echo "=== Testing STDIN and Task File Support ==="
echo

# Check if cliffy is built
if [ ! -f "$CLIFFY" ]; then
    echo "Error: cliffy not found at $CLIFFY"
    echo "Please run: go build -o bin/cliffy ./cmd/cliffy"
    exit 1
fi

echo "1. Testing line-separated tasks from file..."
echo "   Command: cliffy --tasks-file examples/tasks-simple.txt"
$CLIFFY --tasks-file examples/tasks-simple.txt
echo

echo "2. Testing JSON tasks from file..."
echo "   Command: cliffy --json --tasks-file examples/tasks.json"
$CLIFFY --json --tasks-file examples/tasks.json
echo

echo "3. Testing line-separated tasks from STDIN..."
echo "   Command: echo -e 'task1\ntask2' | cliffy -"
echo -e "what is 10 + 5?\nwhat is 20 * 2?" | $CLIFFY -
echo

echo "4. Testing JSON tasks from STDIN..."
echo "   Command: echo '[\"task1\", \"task2\"]' | cliffy --json -"
echo '["what is the speed of light?", "what is the boiling point of water?"]' | $CLIFFY --json -
echo

echo "5. Testing with shared context..."
echo "   Command: cliffy --context 'Be concise' --tasks-file examples/tasks-simple.txt"
$CLIFFY --context "Be concise, answer in one sentence" --tasks-file examples/tasks-simple.txt
echo

echo "6. Testing shell pipeline integration..."
echo "   Command: find . -name '*.go' | head -3 | xargs -I {} echo 'count lines in {}' | cliffy -"
find . -name "*.go" -type f | head -3 | xargs -I {} echo "count the total lines in {}" | $CLIFFY -
echo

echo "=== All tests completed successfully! ==="
