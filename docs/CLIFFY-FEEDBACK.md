# Cliffy Developer Experience Feedback

Based on extensive testing from a Claude Code coordinator perspective.

---

## Current State Assessment

### What Works Well ✅

1. **Parallel execution is excellent**
   - Clean summary output with timing/tokens/cost
   - Worker assignment visualization `[1/3] ▶ Task (worker 3)`
   - Success/failure markers `✓` and progress tracking
   - Auto-retry is transparent and "just works"

2. **Cost transparency**
   - Every volley shows token usage and cost
   - Helps with budget planning
   - Great for free model validation

3. **Performance feedback**
   - Duration per task helps identify slow operations
   - "avg tokens/task" helps optimize prompts
   - Clear completion ratio (e.g., "20/20 tasks")

4. **Task context in output**
   - Full task prompt shown in separator: `Task 1/3: What is 2+2?`
   - Easy to correlate output with input

---

## Why I Use `--quiet` Flag

### 1. **Tool logs are minimal and not very informative**

Without `--quiet`:
```
10
[TOOL] view
```

With `--quiet`:
```
10
```

**The tool name alone doesn't tell me:**
- Which file was read?
- How many lines?
- Were there errors/retries?
- What was the glob pattern?

**I'd prefer:**
```
10
[TOOL] view: src/main.rs (1-800, 800 lines read)
```

Or even better, nothing unless there's an issue:
```
10
```

### 2. **Tool logs clutter output in batch operations**

With 20 tasks using tools, you get 20x `[TOOL] view` messages that don't add value. In a coordinator role, I care about:
- Did it succeed? ✓
- What was the result? (the actual output)
- How long did it take?
- Were there failures? ❌

### 3. **No intermediate progress visibility**

When cliffy is reading a large file or running a complex task, there's no feedback. It just... waits. Compare:

**Current (silent for 10+ seconds):**
```
[waiting...]
Result appears
```

**Better:**
```
[1/3] ▶ Analyze large file (reading 50k lines...)
[1/3] ▶ Analyze large file (parsing structure...)
[1/3] ✓ Analyze large file (15.2s)
```

---

## Developer Experience Improvements

### Priority 1: Enhanced Tool Logging (opt-in with `--verbose`)

#### Current behavior:
```
[TOOL] view
[TOOL] glob
[TOOL] write
```

#### Proposed with `--verbose`:
```
[TOOL] read: src/main.rs (800 lines, 45KB)
[TOOL] glob: **/*.rs → 79 matches
[TOOL] write: docs/output.md (created, 234 lines)
[TOOL] bash: cargo check (exit 0, 2.3s)
```

**Benefits:**
- Understand what cliffy is doing (debugging)
- Verify file access (security/correctness)
- Catch unintended operations (e.g., wrong file read)
- Learn which tools exist and how they're used

#### Default behavior (without `--verbose`):
- Show nothing unless error
- Keep output clean for production use

**Result:** `--quiet` becomes less necessary, `--verbose` becomes the debugging tool.

---

### Priority 2: Streaming Progress for Long Tasks

When a task takes >5 seconds, show incremental updates:

```
[1/8] ▶ Analyze crate architecture...
[1/8] ⋯ Reading 45 source files...
[1/8] ⋯ Parsing dependencies...
[1/8] ⋯ Generating summary...
[1/8] ✓ Analyze crate architecture (12.4s)
```

**Indicators:**
- `▶` = Started
- `⋯` = Working (updates every 2-3s)
- `✓` = Success
- `✗` = Failed
- `↻` = Retrying (show retry count)

**Benefits:**
- Reduces anxiety on long tasks ("is it stuck?")
- Helps identify bottlenecks
- Shows cliffy is alive and working

---

### Priority 3: Smart Output Formatting

#### Problem: Large outputs are hard to parse

When asking cliffy to "list all types in lib.rs", output is:
```
- GamePlugin
- Position
- Player
- Health
- Damage
...
```

#### Proposed: Structured output with summary

```
Found 15 exported types in crates/toft-game/src/lib.rs:

Components (7):
  - Position, Player, Health, Damage, Armor, Inventory, Enemy

Resources (3):
  - GameLog, Map, RngRes

Plugins (1):
  - GamePlugin

Others (4):
  - Item, Weapon, Magic, Experience
```

**Benefits:**
- Easier to scan
- Better for coordinator decision-making
- Can be parsed programmatically

---

### Priority 4: Diff Preview for File Modifications

#### Problem: Silent file modifications are dangerous

Current behavior:
```
Task complete
[TOOL] write
```

#### Proposed: Show diff for modifications

```
Task complete: Modified crates/toft-game/src/components.rs

[DIFF]
+ /// Mana component
+ #[derive(Component, Reflect)]
+ pub struct Mana {
+     pub current: i32,
+     pub max: i32,
+ }

Modified: 1 file, +8 lines, 0 deletions
Run `git diff` to review changes
```

**Benefits:**
- Immediate visibility of changes
- Catch unintended modifications
- Review without switching to git
- Builds trust in cliffy's operations

**Alternative:** Require explicit `--allow-write` flag for file modifications

---

### Priority 5: Better Error Messages

#### Current (inferred from testing):
When tasks fail, they're retried silently. The summary shows:
```
Retries: 1 total
```

But which task failed? What was the error?

#### Proposed: Error details in summary

```
Volley Summary
═══════════════════════════════════════════════════════════════

Completed:  19/20 tasks
Failed:     1/20 tasks
Duration:   42.1s

Failed tasks:
  [5/20] Read nonexistent-file.rs
    Error: File not found: nonexistent-file.rs
    Retried: 3 times
    Last error: FileNotFoundError after 3 attempts
```

**Benefits:**
- Debug failures without checking logs
- Understand which tasks need fixing
- Avoid re-running entire volley

---

### Priority 6: Interactive Mode for Ambiguous Tasks

#### Problem: Vague prompts cause unintended actions

We saw "Task 2" → created Mana struct unexpectedly.

#### Proposed: Clarification prompts

```
[2/20] Task 2
  ⚠️  Warning: Task description is vague. Cliffy interpreted as:
      "Add Mana struct to components.rs based on project context"

  Proceed? [y/N/edit]:
```

**Alternative:** Dry-run mode `--dry-run` that shows interpretations without executing

---

### Priority 7: Context Validation

#### Problem: Context files can be stale or wrong

When using `--context-file`, there's no validation.

#### Proposed: Show context being used

```
Volley: 3 tasks queued with shared context (242 tokens)

Context preview:
  Project: Bevy Rogue
  Language: Rust
  Style: snake_case
  [... 5 more lines]

Proceed? [Y/n]:
```

**Benefits:**
- Verify context is correct
- Avoid propagating wrong conventions
- Understand token overhead

---

### Priority 8: Result Aggregation

#### Problem: 20 results are hard to synthesize

After running 20 code analysis tasks, I get 20 separate outputs. As a coordinator, I want:

#### Proposed: Optional aggregation flag `--summarize`

```bash
cliffy volley --summarize \
  "Count functions in crate A" \
  "Count functions in crate B" \
  "Count functions in crate C"
```

**Output:**
```
[Standard individual results...]

═══════════════════════════════════════════════════════════════
Aggregated Summary
═══════════════════════════════════════════════════════════════

Total functions across 3 crates: 247
  - Crate A: 87 functions
  - Crate B: 93 functions
  - Crate C: 67 functions

Average: 82.3 functions per crate
```

**Benefits:**
- Saves manual aggregation
- Better for reporting
- Easier decision-making

---

### Priority 9: Task Templates / Aliases

#### Problem: Repeating similar volleys

I often run the same pattern:
```bash
cliffy volley --yes --quiet --max-concurrent 8 [8 similar tasks]
```

#### Proposed: Save volley templates

```bash
# Define template
cliffy volley save-template "code-survey" \
  --max-concurrent 8 \
  --quiet \
  --yes \
  --context-file .cliffy/context.md

# Use template
cliffy volley use-template "code-survey" \
  "Analyze crate A" \
  "Analyze crate B" \
  "Analyze crate C"
```

**Benefits:**
- Reduce command-line boilerplate
- Standardize volley patterns
- Easier to share with team

---

### Priority 10: Integration with Development Workflow

#### Problem: Results exist only in terminal

After generating docs or analysis, I need to:
1. Copy results to files manually
2. Git add/commit separately
3. No audit trail

#### Proposed: Workflow integration flags

```bash
cliffy volley --yes --output-dir docs/analysis \
  "Analyze crate A (save as a.md)" \
  "Analyze crate B (save as b.md)" \
  "Analyze crate C (save as c.md)"
```

After completion:
```
3 files created in docs/analysis/
  - a.md
  - b.md
  - c.md

Git status: 3 new files
Add to git? [y/N]:
```

**Benefits:**
- Streamlined workflow
- Less manual file handling
- Integrated with version control

---

## Tool Visibility Improvements

### Current: Minimal tool feedback
```
Result
[TOOL] view
```

### Proposed: Three verbosity levels

#### Default (current `--quiet`)
```
Result
```

#### Normal (new default, current without `--quiet`)
```
Result
[TOOLS] read(1), glob(2), write(1)
```

#### Verbose (`--verbose` or `-v`)
```
Result

Tool trace:
  [1.2s] read: src/main.rs (800 lines)
  [0.3s] glob: **/*.rs → 79 files
  [0.5s] read: Cargo.toml (45 lines)
  [0.1s] write: output.md (created)
```

**Benefits:**
- Clean by default
- Debugging when needed
- Learning tool capabilities

---

## JSON Output Improvements

### Current: Inconsistent JSON
```bash
cliffy --output-format json "task"
# Sometimes returns: 8
# Sometimes returns: {"result": "8"}
```

### Proposed: Structured JSON always

```json
{
  "task": "What is 5 + 3?",
  "result": "8",
  "metadata": {
    "duration_ms": 1234,
    "tokens": 13240,
    "cost": 0.0,
    "model": "grok-4-fast:free",
    "tools_used": ["none"]
  }
}
```

**Benefits:**
- Programmatic parsing
- Consistent structure
- Machine-readable metrics

---

## Volley-Specific Improvements

### 1. Task Dependencies

**Problem:** Some tasks depend on others

**Proposed:**
```bash
cliffy volley --yes \
  "1: Create base struct in components.rs" \
  "2: Add impl block [depends: 1]" \
  "3: Register in plugin [depends: 2]"
```

Cliffy would execute 1 first, then 2, then 3, even with high concurrency.

### 2. Task Grouping

**Problem:** 20 tasks is hard to understand

**Proposed:**
```bash
cliffy volley --yes \
  --group "Core Analysis" \
    "Analyze A" "Analyze B" \
  --group "Documentation" \
    "Document X" "Document Y"
```

**Output:**
```
Group: Core Analysis (2/2 complete)
  [1/4] ✓ Analyze A (2.1s)
  [2/4] ✓ Analyze B (2.3s)

Group: Documentation (2/2 complete)
  [3/4] ✓ Document X (1.8s)
  [4/4] ✓ Document Y (1.9s)
```

### 3. Conditional Execution

**Proposed:** `--continue-on-error` vs `--fail-fast` is binary

**Better:**
```bash
cliffy volley --on-error retry --max-retries 5 [tasks]
cliffy volley --on-error skip [tasks]
cliffy volley --on-error abort [tasks]
```

---

## Safety Improvements

### 1. Dry-Run Mode

```bash
cliffy --dry-run "Create struct Foo in lib.rs"
```

**Output:**
```
[DRY RUN] Would execute:
  - Read lib.rs
  - Parse structure
  - Generate: pub struct Foo { ... }
  - Write to lib.rs at line 45

No files were modified.
```

### 2. Write Protection

```bash
# Explicit flag required for modifications
cliffy --allow-write "Add struct to lib.rs"

# Otherwise, error
cliffy "Add struct to lib.rs"
# Error: Task requires file modifications. Use --allow-write to proceed.
```

### 3. Undo Command

```bash
cliffy undo  # Reverts last operation
cliffy undo --task 5  # Reverts specific task from last volley
```

---

## Summary of Recommendations

### Immediate Impact (High Priority)

1. **Enhanced tool logging with `--verbose`** - Better debugging
2. **Streaming progress for long tasks** - Reduce anxiety
3. **Diff preview for file modifications** - Build trust
4. **Better error messages in summary** - Easier debugging
5. **Three-level verbosity** - Clean by default, verbose when needed

### Medium Priority

6. **Interactive mode for ambiguous tasks** - Prevent accidents
7. **Result aggregation `--summarize`** - Better coordinator experience
8. **Smart output formatting** - Easier to parse
9. **Context validation** - Avoid mistakes

### Nice to Have

10. **Task templates** - Reduce boilerplate
11. **Workflow integration** - Git integration
12. **Task dependencies** - Complex workflows
13. **Dry-run mode** - Safety
14. **Undo command** - Recover from mistakes

---

## Why I Use `--quiet`: The Real Answer

**It's not that the info isn't helpful—it's that it's not helpful *enough*.**

- Tool names alone (`[TOOL] view`) don't tell me what I need to know
- Progress is invisible, making long tasks feel stuck
- File modifications are silent and scary
- Errors are buried in logs instead of shown inline

**If cliffy had `--verbose` mode with detailed tool traces, I'd use that instead of `--quiet`.**

The ideal flow:
- Default: Clean output, only show results
- `--verbose`: Show everything (files read, patterns used, diffs)
- `--quiet`: Suppress even summaries (for scripts)

---

## Final Thoughts

Cliffy is **incredibly powerful** for parallel execution. The missing piece is **observability**:

- What is cliffy doing right now?
- What files is it touching?
- Why did something fail?
- What's taking so long?

With better observability, cliffy would transition from "fast but opaque" to "fast and trustworthy," making it the perfect coordinator companion.

---

*Feedback based on extensive testing 2025-10-01*
*Claude Code coordinator perspective after running 100+ cliffy tasks*
