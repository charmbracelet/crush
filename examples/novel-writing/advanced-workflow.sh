#!/bin/bash
# Advanced novel generation workflow
# Demonstrates the full power of Cliffy's volley mode

set -e

echo "🎯 Advanced Novel Generation Workflow"
echo "======================================"
echo ""

# Configuration
WORKERS=10
MODEL="deepseek/deepseek-r1:free"  # Use free tier for demo
OUTPUT_BASE="output"
BUDGET_LIMIT=25.00

# Create comprehensive directory structure
echo "📁 Setting up project structure..."
mkdir -p $OUTPUT_BASE/{
    outline,
    characters/{profiles,backstories,relationships},
    worldbuilding/{locations,timeline,customs},
    chapters/{drafts,revisions,final},
    scenes/{variations,deleted},
    dialogues/{options,selected},
    descriptions/{library,selected},
    analysis/{structure,pacing,themes}
}

# Phase 1: Planning and Structure
echo ""
echo "📋 Phase 1: Planning and Structure"
echo "-----------------------------------"

echo "  → Generating master outline..."
cliffy "Create a detailed 25-chapter outline for a mystery novel about family secrets, with chapter titles, key events, and emotional arcs" \
    --model "$MODEL" \
    --output $OUTPUT_BASE/outline/master.txt \
    --quiet

echo "  → Generating story structure analysis..."
cliffy \
    "Analyze the story structure: three-act breakdown with plot points" \
    "Identify character arcs for protagonist Sarah" \
    "Map out mystery revelation pacing across 25 chapters" \
    "Suggest subplot threads to weave through main narrative" \
    --context "Novel outline: $(cat $OUTPUT_BASE/outline/master.txt)" \
    --model "$MODEL" \
    --output-dir $OUTPUT_BASE/analysis/ \
    --quiet

echo "  ✓ Planning complete"

# Phase 2: Character Development
echo ""
echo "👥 Phase 2: Character Development"
echo "---------------------------------"

echo "  → Generating character profiles (parallel)..."
cliffy --batch tasks/characters.txt \
    --model "$MODEL" \
    --workers 7 \
    --output-dir $OUTPUT_BASE/characters/profiles/ \
    --quiet

echo "  → Generating character relationships..."
cliffy \
    "Describe the relationship between Sarah and her mother Linda (tension, history, evolution)" \
    "Describe the relationship between Sarah and David (attraction, trust, collaboration)" \
    "Describe the relationship between Sarah and Marcus (past romance, current tension)" \
    "Describe how Margaret's memory affects all character relationships" \
    --model "$MODEL" \
    --output-dir $OUTPUT_BASE/characters/relationships/ \
    --quiet

echo "  ✓ Characters developed"

# Phase 3: World Building
echo ""
echo "🌍 Phase 3: World Building"
echo "-------------------------"

echo "  → Generating worldbuilding (parallel)..."
cliffy --batch tasks/worldbuilding.txt \
    --model "$MODEL" \
    --workers 8 \
    --output-dir $OUTPUT_BASE/worldbuilding/ \
    --quiet

echo "  ✓ World building complete"

# Phase 4: First Draft Generation
echo ""
echo "📚 Phase 4: First Draft Generation"
echo "----------------------------------"

echo "  → Generating all 25 chapters (10 workers, may take 2-3 hours)..."
cliffy --tasks tasks/chapters.txt \
    --model "$MODEL" \
    --workers $WORKERS \
    --rate-limit openrouter:50/min \
    --output-dir $OUTPUT_BASE/chapters/drafts/ \
    --json > $OUTPUT_BASE/generation_report.json \
    --verbose

echo "  ✓ First draft complete"

# Phase 5: Variation Generation
echo ""
echo "🎭 Phase 5: Generating Variations"
echo "---------------------------------"

echo "  → Generating variations for key scenes..."

# Climax variations
cliffy \
    --context "Original climax from Chapter 23: $(cat $OUTPUT_BASE/chapters/drafts/chapter_23.txt)" \
    "Rewrite the climax with more dramatic confrontation" \
    "Rewrite the climax with quiet, emotional revelation" \
    "Rewrite the climax from David's perspective" \
    "Rewrite the climax from Mother's perspective" \
    "Rewrite the climax with unexpected twist" \
    --model "$MODEL" \
    --output-dir $OUTPUT_BASE/scenes/variations/chapter_23/ \
    --quiet

# Opening variations
cliffy \
    --context "Original opening from Chapter 1: $(cat $OUTPUT_BASE/chapters/drafts/chapter_01.txt)" \
    "Rewrite opening starting with dialogue" \
    "Rewrite opening starting with action" \
    "Rewrite opening starting with description" \
    "Rewrite opening starting in medias res" \
    --model "$MODEL" \
    --output-dir $OUTPUT_BASE/scenes/variations/chapter_01/ \
    --quiet

echo "  ✓ Variations generated"

# Phase 6: Dialogue Options
echo ""
echo "💬 Phase 6: Dialogue Generation"
echo "-------------------------------"

echo "  → Generating dialogue options for key conversations..."
cliffy \
    "Write Sarah confronting her mother (aggressive/accusatory tone)" \
    "Write Sarah confronting her mother (gentle questioning)" \
    "Write Sarah confronting her mother (emotional breakdown)" \
    "Write Sarah and David's first flirtation (subtle/professional)" \
    "Write Sarah and David's first flirtation (obvious attraction)" \
    "Write Marcus trying to convince Sarah to go public (manipulative)" \
    "Write Marcus trying to convince Sarah to go public (genuinely concerned)" \
    --model "$MODEL" \
    --output-dir $OUTPUT_BASE/dialogues/options/ \
    --quiet

echo "  ✓ Dialogue options ready"

# Phase 7: Description Library
echo ""
echo "🎨 Phase 7: Building Description Library"
echo "----------------------------------------"

echo "  → Generating reusable descriptions..."
cliffy \
    "Describe grandmother's attic in 5 different emotional tones (nostalgic, eerie, comforting, sad, mysterious)" \
    "Describe the library reading room at different times of day (morning light, afternoon calm, evening shadows)" \
    "Describe Sarah's emotional states through physical sensations (anxiety, excitement, dread, joy)" \
    "Describe weather as metaphor for story events (building storm, clearing skies, fog, bright sunshine)" \
    --model "$MODEL" \
    --output-dir $OUTPUT_BASE/descriptions/library/ \
    --quiet

echo "  ✓ Description library built"

# Phase 8: Analysis and Polish Preparation
echo ""
echo "📊 Phase 8: Analysis"
echo "-------------------"

echo "  → Analyzing generated content..."
cliffy \
    "Analyze pacing across all 25 chapters, identify any slow sections" \
    "Check for plot holes or inconsistencies across the narrative" \
    "Evaluate character voice consistency, especially for Sarah" \
    "Assess mystery reveal pacing - is information revealed too fast or too slow?" \
    --context "Full novel draft: $(cat $OUTPUT_BASE/chapters/drafts/*.txt)" \
    --model "$MODEL" \
    --output-dir $OUTPUT_BASE/analysis/ \
    --quiet

echo "  ✓ Analysis complete"

# Generate Summary Report
echo ""
echo "📈 Generating Summary Report"
echo "---------------------------"

# Extract stats from generation report
TOTAL_CHAPTERS=$(ls $OUTPUT_BASE/chapters/drafts/ | wc -l)
TOTAL_WORDS=$(cat $OUTPUT_BASE/chapters/drafts/*.txt 2>/dev/null | wc -w || echo "0")
TOTAL_COST=$(jq -r '.summary.total_cost // 0' $OUTPUT_BASE/generation_report.json 2>/dev/null || echo "0")
SUCCESS_RATE=$(jq -r '.summary.succeeded_tasks // 0' $OUTPUT_BASE/generation_report.json 2>/dev/null || echo "0")
TOTAL_TIME=$(jq -r '.summary.total_duration // "unknown"' $OUTPUT_BASE/generation_report.json 2>/dev/null || echo "unknown")

cat > $OUTPUT_BASE/SUMMARY.md <<EOF
# Novel Generation Summary

**Generated:** $(date)
**Project:** Mystery Novel - Family Secrets

## Statistics

- **Total Chapters:** $TOTAL_CHAPTERS
- **Total Words:** $(printf "%'d" $TOTAL_WORDS)
- **Generation Time:** $TOTAL_TIME
- **Success Rate:** ${SUCCESS_RATE}/25 chapters
- **Total Cost:** \$${TOTAL_COST}

## Generated Content

### Structure
- ✓ Master outline (25 chapters)
- ✓ Story structure analysis
- ✓ Character arcs
- ✓ Mystery pacing map

### Characters
- ✓ 7 detailed character profiles
- ✓ 4 relationship dynamics
- ✓ Character voice guides

### World Building
- ✓ 8 location descriptions
- ✓ Historical timeline (1952-2025)
- ✓ Cultural context
- ✓ Location map

### Draft Content
- ✓ 25 chapter first drafts
- ✓ 9 scene variations (climax + opening)
- ✓ 7 dialogue option sets
- ✓ 4 description library entries

### Analysis
- ✓ Pacing analysis
- ✓ Plot consistency check
- ✓ Character voice evaluation
- ✓ Mystery reveal assessment

## Next Steps

1. **Human Review:** Read through chapters, select best variations
2. **Editing:** Combine best elements, polish prose, fix inconsistencies
3. **Second Pass:** Generate any missing scenes or transitions
4. **Final Polish:** Use high-quality model for final refinement

## File Locations

\`\`\`
$OUTPUT_BASE/
├── outline/              # Story structure
├── characters/           # Character development
├── worldbuilding/        # Setting and context
├── chapters/drafts/      # First draft chapters
├── scenes/variations/    # Alternative scene versions
├── dialogues/options/    # Dialogue variations
├── descriptions/library/ # Reusable descriptions
└── analysis/            # Content analysis
\`\`\`

---

**Generated with Cliffy volley mode** 🚀
**Model:** $MODEL
**Workers:** $WORKERS
EOF

echo ""
echo "✅ COMPLETE!"
echo ""
cat $OUTPUT_BASE/SUMMARY.md
echo ""
echo "Full report saved to: $OUTPUT_BASE/SUMMARY.md"
echo "Generation details: $OUTPUT_BASE/generation_report.json"
