#!/bin/bash

# Start the AI SDK Bridge for Claude Code OAuth support

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# Check if npm/node is available
if ! command -v npm &> /dev/null; then
    echo "npm is required but not installed. Please install Node.js."
    exit 1
fi

# Install dependencies if needed
if [ ! -d "node_modules" ]; then
    echo "Installing dependencies..."
    npm install
fi

# Start the bridge
echo "Starting AI SDK Bridge on port 8765..."
node server.js