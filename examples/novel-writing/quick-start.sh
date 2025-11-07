#!/bin/bash
# Quick start script for novel generation with Cliffy
# Usage: ./quick-start.sh

set -e

echo "📚 Novel Writing with Cliffy - Quick Start"
echo "=========================================="
echo ""

# Check if cliffy is available
if ! command -v cliffy &> /dev/null; then
    echo "❌ Error: cliffy not found in PATH"
    echo "Please build cliffy first: go build -o bin/cliffy ./cmd/cliffy"
    exit 1
fi

# Create output directories
echo "📁 Creating output directories..."
mkdir -p output/{characters,chapters,worldbuilding,variations}

# Step 1: Generate character profiles
echo ""
echo "👥 Generating character profiles (parallel)..."
cliffy --batch tasks/characters.txt \
       --output-dir output/characters/ \
       --quiet

echo "   ✓ Generated $(ls output/characters/ | wc -l) character profiles"

# Step 2: Generate worldbuilding
echo ""
echo "🌍 Generating worldbuilding details (parallel)..."
cliffy --batch tasks/worldbuilding.txt \
       --output-dir output/worldbuilding/ \
       --quiet

echo "   ✓ Generated $(ls output/worldbuilding/ | wc -l) worldbuilding documents"

# Step 3: Generate first 5 chapters as demo
echo ""
echo "📖 Generating first 5 chapters (demo - parallel)..."
head -5 tasks/chapters.txt | cliffy --batch \
       --output-dir output/chapters/ \
       --workers 3 \
       --verbose

echo "   ✓ Generated $(ls output/chapters/ | wc -l) chapters"

# Summary
echo ""
echo "✅ Quick start complete!"
echo ""
echo "Generated:"
echo "  - $(ls output/characters/ | wc -l) character profiles"
echo "  - $(ls output/worldbuilding/ | wc -l) worldbuilding documents"
echo "  - $(ls output/chapters/ | wc -l) chapter drafts"
echo ""
echo "Next steps:"
echo "  1. Review generated content in output/"
echo "  2. Generate remaining chapters: cliffy --tasks tasks/chapters.txt --workers 10"
echo "  3. Generate variations: cliffy --template 'Rewrite {file} with darker tone'"
echo ""
echo "💡 Tip: Use --json flag to track costs and statistics"
