# Refactor Plan: Unify cliffy and cliffy volley execution paths

**Date:** 2025-10-01
**Status:** Planning
**Goal:** Make volley the unified execution engine for both single and multi-task execution

## Problem Analysis

### Current Architecture (Duplicated Paths)

1. **`cliffy "prompt"`** → Uses `runner.Runner` (single task execution)
2. **`cliffy volley "t1" "t2"`** → Uses `volley.Scheduler` (parallel execution)

### Issues

- **Error handling not unified** - Rate limits, daily limits handled differently in two places
- **Can't gracefully upgrade** - Single tasks don't benefit from volley's retry logic
- **Different output formats** - Inconsistent between single and multi-task
- **Code duplication** - Same concerns implemented twice

### User's Vision

- `cliffy "task"` → Single task via volley scheduler (clean output, no verbosity)
- `cliffy "t1" "t2" "t3"` → Multi-task via volley scheduler (clean output)
- `cliffy volley "t1" "t2"` → Same as above but with verbose progress/stats

## Proposed Architecture

### Unified Execution Flow

```
cliffy [tasks...]
    ↓
  Parse args as tasks (1 or more)
    ↓
  volley.Scheduler (ALWAYS)
    ↓
  Output mode:
    - Silent (default): Just results
    - Verbose (--verbose/-v): Progress + stats
```

### Command Behavior

| Command | Tasks | Output Mode | Progress | Summary |
|---------|-------|-------------|----------|---------|
| `cliffy "task"` | 1 | Silent | ❌ | ❌ |
| `cliffy "t1" "t2"` | 2+ | Silent | ❌ | ❌ |
| `cliffy -v "task"` | 1+ | Verbose | ✅ | ✅ |
| `cliffy volley "t1" "t2"` | 2+ | Verbose | ✅ | ✅ |

### Benefits

1. **Unified error handling** - Rate limits, daily limits, retries handled once
2. **Single code path** - Easier to maintain, one place for improvements
3. **Clean output by default** - Just results, no timing spam
4. **Opt-in verbosity** - Use `-v/--verbose` for progress/stats
5. **Retry logic everywhere** - Even single tasks get smart retries

## Implementation Steps

### Step 1: Update main.go (Root Command)

**Changes:**
- Accept multiple args instead of requiring exactly 1
- Parse each arg as a volley task
- Route to volley scheduler instead of runner
- Add `--verbose/-v` flag for progress output

**Code changes:**
```go
// cmd/cliffy/main.go

var verbose bool  // Add flag

rootCmd = &cobra.Command{
    Use:   "cliffy [flags] <task1> [task2] [task3] ...",
    Args:  cobra.MinimumNArgs(1),  // Allow 1+ args
    RunE: func(cmd *cobra.Command, args []string) error {
        // Create tasks from all args
        tasks := make([]volley.Task, len(args))
        for i, arg := range args {
            tasks[i] = volley.Task{
                Index:  i + 1,
                Prompt: arg,
            }
        }

        // Create volley options (silent by default)
        opts := volley.VolleyOptions{
            MaxConcurrent: 3,
            MaxRetries:    3,
            ShowProgress:  verbose,  // Only if -v
            OutputFormat:  outputFormat,
        }

        // Use volley scheduler (always)
        // ... rest of volley execution
    },
}

rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show progress and stats")
```

### Step 2: Add Silent Mode to Volley

**Changes to `internal/volley/scheduler.go`:**

Add silent mode that:
- ✅ Executes tasks normally
- ✅ Handles retries and errors
- ❌ No progress output
- ❌ No summary output
- ✅ Still outputs task results to stdout
- ✅ Still shows errors to stderr

**Code changes:**
```go
// internal/volley/task.go

type VolleyOptions struct {
    // ... existing fields
    ShowProgress  bool   // Already exists
    ShowSummary   bool   // Add this - control summary separately
}

func DefaultVolleyOptions() VolleyOptions {
    return VolleyOptions{
        MaxConcurrent: 3,
        MaxRetries:    3,
        ShowProgress:  false,  // Silent by default
        ShowSummary:   false,  // Silent by default
        OutputFormat:  "text",
    }
}
```

**Changes to `internal/volley/progress.go`:**

```go
func (p *ProgressTracker) Finish(summary VolleySummary) {
    if !p.enabled {
        return
    }
    // Only show summary if enabled
    // ...
}
```

### Step 3: Update Output Functions

**Changes to `cmd/cliffy/volley.go` (or move to main.go):**

```go
func outputResults(results []volley.TaskResult, summary volley.VolleySummary, opts volley.VolleyOptions) error {
    // Output task results (always)
    for _, result := range results {
        if result.Status == volley.TaskStatusSuccess {
            fmt.Println(result.Output)
        } else if result.Status == volley.TaskStatusFailed {
            fmt.Fprintf(os.Stderr, "Task %d failed: %v\n", result.Task.Index, result.Error)
        }
    }

    // Only show summary if verbose
    if opts.ShowSummary {
        // ... existing summary code
    }

    return nil
}
```

### Step 4: Handle volley Subcommand

**Option A: Keep as verbose alias**
```go
var volleyCmd = &cobra.Command{
    Use:   "volley [flags] <task1> <task2> ...",
    Short: "Execute tasks with verbose progress (alias for cliffy -v)",
    RunE: func(cmd *cobra.Command, args []string) error {
        verbose = true  // Force verbose mode
        return runVolley(cmd, args)
    },
}
```

**Option B: Remove entirely**
- Deprecate `cliffy volley`
- Show migration message: "Use 'cliffy -v' for verbose output"

### Step 5: Simplify CLAUDE.md Prompt

**New simplified version:**

```markdown
<IMPORTANT_TOOL_USAGE>
## Cliffy - AI Task Executor

Cliffy executes AI tasks efficiently. It accepts single or multiple tasks.

### Usage

```bash
# Single task
cliffy "analyze auth.go"

# Multiple tasks (runs in parallel)
cliffy "analyze auth.go" "analyze db.go" "analyze api.go"

# Show progress (verbose mode)
cliffy -v "task1" "task2" "task3"

# With shared context
cliffy --context "You are a security expert" "review auth.go" "review db.go"
```

### When to Use Multiple Tasks

Use multiple task arguments when:
- Tasks are independent and can run concurrently
- User asks to process multiple items (files, questions, etc.)
- Tasks share the same context/instructions

### Error Handling

Cliffy automatically:
- Retries on rate limits (429 errors)
- Shows helpful messages for daily limits
- Handles network errors with exponential backoff

Do NOT add error handling yourself - cliffy does this internally.
</IMPORTANT_TOOL_USAGE>
```

## File Changes Summary

### Files to Modify

1. **`cmd/cliffy/main.go`**
   - Accept 1+ args (not exactly 1)
   - Parse args as volley tasks
   - Route to volley scheduler
   - Add `--verbose/-v` flag
   - Remove runner.Runner execution path

2. **`internal/volley/task.go`**
   - Add `ShowSummary` to VolleyOptions
   - Update DefaultVolleyOptions to be silent

3. **`internal/volley/scheduler.go`**
   - Support silent mode execution
   - Only create progress tracker if enabled

4. **`internal/volley/progress.go`**
   - Respect ShowProgress and ShowSummary flags
   - Suppress all output in silent mode

5. **`cmd/cliffy/volley.go`** (or merge into main.go)
   - Update outputResults to respect silent mode
   - Only show summary if ShowSummary=true
   - Always show task results (stdout)
   - Always show errors (stderr)

6. **`docs/CLAUDE_MD_BLOCK.md`**
   - Drastically simplify
   - Remove internal timing details
   - Focus on "cliffy takes multiple tasks"
   - Remove rate limit implementation details

### Files to Consider Removing

- **`internal/runner/runner.go`** - May no longer be needed if volley handles everything
- Keep for now, remove in future if unused

## Output Examples

### Silent Mode (Default)

```bash
$ cliffy "what is 2+2?"
4

$ cliffy "what is 2+2?" "what is 3+3?"
4

6

$ cliffy "summarize auth.go" "summarize db.go"
# Summary of auth.go
Authentication module using JWT tokens...

# Summary of db.go
Database layer with connection pooling...
```

### Verbose Mode

```bash
$ cliffy -v "what is 2+2?" "what is 3+3?"
[1/2] ▶ what is 2+2? (worker 1)
[2/2] ▶ what is 3+3? (worker 2)
[1/2] ✓ what is 2+2? (2.3s, 12.5k tokens, $0.0036, grok-4-fast)
[2/2] ✓ what is 3+3? (2.1s, 12.4k tokens, $0.0035, grok-4-fast)

4

6

═══════════════════════════════════════════════════════════════
Volley Summary
═══════════════════════════════════════════════════════════════

Completed:  2/2 tasks
Duration:   4.4s
Tokens:     24,900 total (avg 12,450/task)
Cost:       $0.0071 total
Model:      grok-4-fast:free
Workers:    3 concurrent (max)
```

### Error Handling (Unified)

```bash
$ cliffy "task1" "task2"
task1 result

Error: Rate limit exceeded. Please wait 2 hours for daily limit to reset.
Try: Use a different model or wait until quota resets
```

## Testing Plan

1. **Test single task execution**
   ```bash
   cliffy "what is 2+2?"  # Should show just: 4
   ```

2. **Test multi-task execution**
   ```bash
   cliffy "what is 2+2?" "what is 3+3?"  # Should show: 4\n\n6
   ```

3. **Test verbose mode**
   ```bash
   cliffy -v "task1" "task2"  # Should show progress + summary
   ```

4. **Test error handling**
   - Trigger rate limit → Should show unified error message
   - Trigger network error → Should retry automatically
   - Daily limit → Should show "wait X hours" message

5. **Test backward compatibility**
   ```bash
   cliffy volley "t1" "t2"  # Should still work (verbose mode)
   ```

## Migration Path

### Phase 1: Add Silent Mode (This Session)
- ✅ Add silent mode to volley
- ✅ Update main.go to use volley for single tasks
- ✅ Add --verbose flag
- ✅ Keep `cliffy volley` working as verbose alias

### Phase 2: Documentation Update
- ✅ Update CLAUDE.md with simplified instructions
- ✅ Update README if exists
- ✅ Add examples

### Phase 3: Deprecation (Future)
- Add warning to `cliffy volley`: "Use 'cliffy -v' instead"
- After transition period, remove volley subcommand
- Clean up any unused runner code

## Success Criteria

✅ `cliffy "task"` shows clean output (just result)
✅ `cliffy "t1" "t2"` runs in parallel, shows results
✅ `cliffy -v` shows progress and summary
✅ Error messages are unified and helpful
✅ All existing tests pass
✅ Rate limit errors show helpful messages
✅ Daily limit errors show wait time

## Next Steps

1. Implement silent mode in volley
2. Update main.go to route to volley
3. Add --verbose flag
4. Test all scenarios
5. Update documentation
6. Simplify CLAUDE.md prompt

---

**Status:** Ready to implement
**Estimated Time:** 2-3 hours
**Breaking Changes:** None (backward compatible)
