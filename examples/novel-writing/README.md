# Novel Writing with Cliffy: The Shotgun Approach

**Use Case:** Generate massive amounts of prose quickly using parallel AI generation

---

## The Problem with Serial Writing

**Traditional approach (Crush, ChatGPT, etc.):**
```
1. Write scene 1 (5 minutes)
2. Write scene 2 (5 minutes)
3. Write scene 3 (5 minutes)
...
Total for 20 scenes: 100 minutes = 1h 40min
```

**Cliffy volley approach:**
```
1. Define all 20 scenes
2. Generate ALL in parallel (3 workers)
Total time: ~35 minutes (3x faster!)
```

---

## Quick Start: First Draft Generation

### 1. Generate Multiple Scenes in Parallel

```bash
# Create scene outline
cat > scenes.txt <<EOF
Chapter 1, Scene 1: Sarah discovers the letter in her grandmother's attic
Chapter 1, Scene 2: Sarah confronts her mother about family secrets
Chapter 2, Scene 1: Flashback to grandmother's arrival in New York, 1952
Chapter 2, Scene 2: Sarah meets the mysterious librarian
Chapter 3, Scene 1: Discovery of the hidden photographs
EOF

# Generate all scenes at once (using template)
cliffy --template "Write a 500-word scene: {scene}" \
       --file scenes.txt \
       --output-dir drafts/
```

**Result:** 5 scenes written in parallel, ~2-3 minutes total

### 2. Character Development in Bulk

```bash
# Generate character backstories simultaneously
cliffy \
  "Write a detailed backstory for Sarah, a 35-year-old historian" \
  "Write a detailed backstory for Margaret, Sarah's grandmother, WWII era" \
  "Write a detailed backstory for David, the mysterious librarian" \
  "Write a detailed backstory for Elena, Margaret's best friend" \
  --output-dir characters/
```

**Result:** 4 character profiles, ready in parallel

### 3. Dialogue Options (Shotgun Approach)

```bash
# Generate 10 different dialogue versions
cliffy \
  "Write dialogue: Sarah confronts her mother (aggressive tone)" \
  "Write dialogue: Sarah confronts her mother (gentle questioning)" \
  "Write dialogue: Sarah confronts her mother (emotional breakdown)" \
  "Write dialogue: Sarah confronts her mother (detective-like inquiry)" \
  "Write dialogue: Sarah confronts her mother (humor to deflect tension)" \
  --json > dialogue_options.json

# Pick the best one or combine elements
```

**Result:** Multiple variations to choose from, generated simultaneously

---

## Advanced Workflows

### World-Building at Scale

```json
// world-building-tasks.json
{
  "tasks": [
    {
      "id": "location-1",
      "prompt": "Describe Margaret's neighborhood in 1952 Brooklyn in vivid detail",
      "model": "large"
    },
    {
      "id": "location-2",
      "prompt": "Describe the modern-day university library where Sarah works",
      "model": "small"
    },
    {
      "id": "timeline",
      "prompt": "Create a detailed timeline of events from 1952 to present day",
      "model": "large"
    },
    {
      "id": "customs",
      "prompt": "Research and describe social customs of 1950s Italian-American families",
      "model": "small"
    }
  ]
}
```

```bash
cliffy --tasks world-building-tasks.json \
       --output-format markdown \
       > world_building.md
```

### Chapter-by-Chapter Generation

```bash
# Generate entire book outline
cliffy --batch <<EOF
Outline Chapter 1: The Discovery (5 scenes)
Outline Chapter 2: Following the Trail (7 scenes)
Outline Chapter 3: Revelations (6 scenes)
Outline Chapter 4: The Confrontation (4 scenes)
Outline Chapter 5: Resolution (3 scenes)
EOF

# Then generate first drafts for all chapters
cliffy --template "Write Chapter {num}: {title} based on this outline: {outline}" \
       --data chapters.csv
```

### Revision Variations (The Shotgun Method)

```bash
# Generate 5 different versions of the same scene
cliffy \
  --context "Original scene: $(cat scene_3.txt)" \
  "Rewrite this scene from David's perspective" \
  "Rewrite this scene with more tension and mystery" \
  "Rewrite this scene as a flashback" \
  "Rewrite this scene with darker tone" \
  "Rewrite this scene focusing on sensory details" \
  --output-dir revisions/scene_3/
```

**Pick the best elements from each variation!**

---

## Real-World Example: 50,000 Word Novel in a Day

### Morning: Structure (2 hours)

```bash
# 1. Generate comprehensive outline
cliffy "Create a detailed 25-chapter outline for a mystery novel about family secrets" \
       > outline.txt

# 2. Generate character profiles (parallel)
cliffy \
  "Create protagonist profile: Sarah Chen, historian" \
  "Create antagonist profile: Unknown family member" \
  "Create mentor character: Librarian David" \
  "Create love interest: Marcus, journalist" \
  "Create supporting cast: Sarah's mother, grandmother (deceased), colleagues" \
  --output-dir characters/
```

### Afternoon: Bulk Generation (4 hours)

```bash
# 3. Generate all 25 chapters in parallel (batched by volley)
# Each chapter ~2000 words = 50,000 total

# Create tasks file
cat > chapter_tasks.txt <<EOF
Write Chapter 1 (2000 words): The Letter - Sarah finds grandmother's hidden correspondence
Write Chapter 2 (2000 words): Questions - Sarah confronts her mother
Write Chapter 3 (2000 words): The Librarian - David offers cryptic help
...
Write Chapter 25 (2000 words): The Truth - Final revelations and resolution
EOF

# Generate ALL chapters in parallel (10 workers)
cliffy --tasks chapter_tasks.txt \
       --workers 10 \
       --rate-limit openrouter:50/min \
       --output-dir chapters/ \
       --verbose

# Cliffy runs 10 chapters at a time, cycling through all 25
# Total time: ~3-4 hours depending on model speed
```

### Evening: Variations & Polish (2 hours)

```bash
# 4. Generate alternative versions of key scenes
cliffy --template "Rewrite the climax scene from Chapter 23: {original}" \
       --variations 5 \
       --file chapters/chapter_23.txt

# 5. Generate transitional text
cliffy \
  "Write a transition between Chapter 5 and Chapter 6" \
  "Write a transition between Chapter 12 and Chapter 13" \
  "Write a transition between Chapter 19 and Chapter 20" \
  --output-dir transitions/
```

**Result: 50,000 word first draft + variations, ready for human editing**

---

## Advanced Techniques

### 1. Genre-Specific Batch Generation

**Romance Novel:**
```bash
cliffy \
  "Write a meet-cute scene between Alex and Jordan" \
  "Write their first date (awkward but charming)" \
  "Write the misunderstanding that drives them apart" \
  "Write the grand gesture reconciliation" \
  "Write the epilogue (1 year later)" \
  --context "Contemporary romance, slow burn, witty banter" \
  --output-dir romance_arc/
```

**Sci-Fi Worldbuilding:**
```bash
# Generate entire universe in parallel
cliffy \
  "Describe the political system of the Galactic Federation" \
  "Describe FTL technology and its limitations" \
  "Describe alien species: The Krel (insectoid traders)" \
  "Describe alien species: The Vrynn (telepathic pacifists)" \
  "Describe human colonies on Mars, Europa, Titan" \
  "Create timeline of human expansion (2100-2500)" \
  --workers 6 \
  --output-format markdown \
  > worldbuilding.md
```

### 2. Dialogue Generator (Rapid Prototyping)

```bash
# Generate 20 different conversation versions
cliffy --template "Write dialogue between Sarah and {character}: {situation}" \
       --data conversations.csv \
       --output-dir dialogues/

# conversations.csv:
# character,situation
# mother,confrontation about secrets
# David,flirting while researching
# Marcus,argument about publishing the story
# mother,reconciliation
# David,confession of love
```

### 3. Description Library

```bash
# Build a reusable description library
cliffy \
  "Describe a thunderstorm in 5 different moods" \
  "Describe a city street in 5 different times of day" \
  "Describe a character's apartment in 5 different emotional states" \
  "Describe a conversation using 5 different metaphors" \
  --output-dir description_library/
```

### 4. A/B Testing Scenes

```bash
# Generate two completely different versions of the same plot point
cliffy \
  --context "Scene: Sarah discovers the truth about her grandmother" \
  "Version A: Dramatic revelation in confrontation with her mother" \
  "Version B: Quiet discovery while reading letters alone" \
  "Version C: Piecing it together during conversation with David" \
  "Version D: Flashback sequence showing grandmother's perspective" \
  --output-dir scene_variations/big_reveal/

# Pick the version that works best, or combine elements
```

---

## Practical Tips

### 1. Use Specific, Detailed Prompts
❌ Bad: "Write a scene"
✅ Good: "Write a 500-word scene where Sarah confronts her mother about the letters, using tense dialogue and revealing that Sarah already knows the truth"

### 2. Batch Similar Tasks Together
```bash
# All character intros in one volley
cliffy \
  "Introduce Sarah in first chapter (show her as meticulous historian)" \
  "Introduce David in library (mysterious, helpful, attractive)" \
  "Introduce Marcus at newspaper office (ambitious, charming)" \
  --context "Contemporary mystery novel, third-person limited POV"
```

### 3. Use Templates for Consistency
```bash
# Create template for all chapters
cat > chapter_template.txt <<EOF
Write Chapter {num}: {title}

Word count: 2000 words
POV: {pov}
Setting: {setting}
Key events: {events}
Emotional arc: {arc}
End with: {cliffhanger}
EOF

# Apply to all chapters via data file
cliffy --template chapter_template.txt --data chapters.csv
```

### 4. Cost Management
```bash
# Use cheaper models for drafts, expensive for polish
cliffy --model small \
  "Generate rough draft of Chapter 1" \
  "Generate rough draft of Chapter 2" \
  ...

# Then use large model for final version
cliffy --model large \
  "Polish and refine: $(cat chapter_1_draft.txt)" \
  --output chapter_1_final.txt
```

### 5. Progressive Enhancement
```bash
# Pass 1: Basic plot
cliffy "Write bare-bones plot summary for each chapter" ...

# Pass 2: Add scenes
cliffy "Expand Chapter 1 plot into 5 detailed scenes" ...

# Pass 3: Write prose
cliffy "Write full prose for Chapter 1, Scene 1" ...

# Pass 4: Add description
cliffy "Enhance Chapter 1 with sensory details and metaphors" ...
```

---

## Output Organization

```
novel-project/
├── outline.txt                    # Master outline
├── characters/                    # Character profiles
│   ├── sarah.txt
│   ├── david.txt
│   └── marcus.txt
├── chapters/                      # Generated chapters
│   ├── 01-the-letter.txt
│   ├── 02-questions.txt
│   └── ...
├── variations/                    # Alternative versions
│   ├── chapter_01_var1.txt
│   ├── chapter_01_var2.txt
│   └── ...
├── worldbuilding/                 # Setting details
│   ├── locations.md
│   ├── timeline.md
│   └── customs.md
└── dialogues/                     # Conversation options
    ├── sarah_mother_v1.txt
    ├── sarah_mother_v2.txt
    └── ...
```

---

## Why This Works Better Than Crush

**Crush (Interactive):**
- Generate one scene at a time
- Wait for each to complete
- No batching capability
- Total time for 25 chapters: **~8-10 hours**

**Cliffy (Volley):**
- Generate 10 scenes in parallel
- Continuous processing
- Batch mode for efficiency
- Total time for 25 chapters: **~3-4 hours**

**For a novelist, Cliffy is 2-3x faster for bulk generation.**

---

## Real Cost Example

**50,000 word novel using DeepSeek (free tier):**
```bash
cliffy --model "deepseek/deepseek-r1:free" \
       --tasks chapters.txt \
       --workers 10 \
       --show-costs

# Estimated output:
# Total: 50,000 words generated
# Cost: $0.00 (using free tier)
# Time: 3.5 hours
# Success rate: 24/25 chapters (1 retry)
```

**Same novel using Claude Sonnet:**
```bash
cliffy --model "anthropic/claude-3.5-sonnet" \
       --tasks chapters.txt \
       --workers 5 \
       --budget-limit 25.00

# Estimated output:
# Total: 50,000 words generated
# Cost: ~$15-20 (depending on input context)
# Time: 2.5 hours
# Success rate: 25/25 chapters
```

---

## The Shotgun Approach Workflow

```bash
#!/bin/bash
# novel-generator.sh - Generate entire novel first draft

echo "🎯 Shotgun Novel Generator"

# 1. Outline
echo "📝 Generating outline..."
cliffy "Create detailed 25-chapter outline for mystery novel" > outline.txt

# 2. Characters (parallel)
echo "👥 Generating character profiles..."
cliffy --batch characters.txt --output-dir characters/

# 3. Chapters (bulk generation)
echo "📚 Generating all chapters..."
cliffy --tasks chapters.txt \
       --workers 10 \
       --rate-limit openrouter:50/min \
       --output-dir chapters/ \
       --json > generation_report.json

# 4. Variations for key scenes
echo "🎭 Generating scene variations..."
cliffy --template "Rewrite climax: {original}" \
       --variations 3 \
       --file chapters/chapter_23.txt \
       --output-dir variations/

# 5. Summary
echo "✅ Complete!"
echo "Generated: $(ls chapters/ | wc -l) chapters"
echo "Word count: $(cat chapters/*.txt | wc -w) words"
echo "Cost: $(jq '.summary.total_cost' generation_report.json)"
```

---

## Conclusion

Cliffy's volley mode is **perfect for novelists** who want to:
- Generate massive amounts of prose quickly
- Explore multiple creative directions simultaneously
- Build comprehensive story worlds in parallel
- Iterate rapidly with variations

**The shotgun approach:** Throw lots of ideas at the wall in parallel, then curate the best ones.

**This is impossible with serial tools like Crush.**

🚀 **Write faster. Write more. Write better with Cliffy.**
