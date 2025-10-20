# Cliffy Visual System - Quick Reference

> One-page guide to understanding Cliffy's visual output

## Task States

```
○  Queued       - Task waiting in queue
◔  Initializing - 25% - Task starting up, loading context
◑  Processing   - 50% - Task actively running, executing tools
◕  Finalizing   - 75% - Task wrapping up, formatting output
●  Complete     - 100% - Task finished successfully
◌  Failed       - Task encountered error
⦿  Canceled     - Task canceled by user/system
◉  Cached       - Result retrieved from cache
```

**Animation**: `○ → ◔ → ◑ → ◕ → ●`

## Tool States

```
▤  Starting     - Tool initializing
▥  Running      - Tool currently executing
▣  Success      - Tool completed successfully
▩  Failed       - Tool returned error
☒  Error        - Tool execution blocked
```

## Worker States

```
⬡  Idle         - Worker available, waiting for task
⬢  Active       - Worker processing a task
⬣  Overloaded   - Worker at maximum capacity
```

## Status Indicators

```
⚠  Warning      - Retry needed, attention required
✗  Error        - Fatal error occurred
⊗  Blocked      - Operation cannot proceed
⏸  Paused       - Waiting/delayed
⚡  Rate Limited - Too many requests
```

## Flow & Actions

```
→  Flow         - Basic data flow
⇒  Strong Flow  - Emphasized direction
⇨  Fast Flow    - Rapid processing
⟲  Retry        - Task/request retrying
⤴  Return       - Error return path
↻  Feedback     - Tool feedback loop
```

## Tree Structure

```
╮  Branch       - Task with tools branches down
├  Mid          - Middle item in tree
╰  Last         - Last item in tree
───  Line       - Horizontal connector
```

## Common Patterns

### Simple Task
```
1   ● analyze auth.go (2.3s, 3.2k tokens)
```

### Task with Tools (Collapsed)
```
1 ╮ ● refactor auth [read grep edit]  4.5k tokens $0.0056  3.8s
```

### Task with Tools (Expanded)
```
1 ╮ ◑ refactor auth (worker 1)
  ├───▣ read     auth.go  0.2s
  ├───▣ grep     password  0.1s
  ╰───▥ edit     auth.go  editing...
```

### Retry Scenario
```
2   ◌ api call (attempt 2) ⟲
    ⤴ Error: rate limited (429)
    ⏸ Retrying in 2.0s...
```

### Tool Error
```
3 ╮ ◌ deploy ✗
  ├───▣ bash     git push  0.5s
  ╰───▩ bash     deploy.sh  ⊗ exit 1
    ⤴ Error: deployment failed
```

### Worker View
```
║ Worker 1 ⬢ ║ → 1  analyze auth.go
║ Worker 2 ⬢ ║ → 2  refactor db.go
║ Worker 3 ⬡ ║ (idle)
║═══════════║
║   Queue   ║ → 3  run tests
```

## Header & Footer

### Volley Start
```
◍═══╕  3 tasks volleyed
    ╰──╮ Using claude-3-5-sonnet
```

### Volley Complete (Success)
```
◍ 3/3 tasks succeeded in 8.3s
  15.2k tokens  $0.0187
```

### Volley Complete (Mixed)
```
◍ 2/3 succeeded, 1 failed in 12.1s
  ✓ Tasks: 1, 2
  ✗ Task: 3 (deployment)
```

## CLI Flags

```bash
--verbose        # Show detailed tool traces
--workers-view   # Show worker pool visualization (future)
--metrics        # Show detailed metrics (future)
--timeline       # Show timeline diagram (future)
```

## Reading the Output

### Task Line Format (Simple)
```
[number] [icon] [description] ([duration], [tokens] tokens)
```

### Task Line Format (With Tools)
```
[number] [branch] [icon] [description] [worker info]
  [tree] [tool icon] [tool name] [args] [duration]
```

### Task Line Format (Collapsed)
```
[number] [branch] [icon] [desc] [tools] [tokens] [cost] [duration]
```

## Color Coding (Future)

When color support is added:
- **Green** (●▣) - Success, completed
- **Yellow** (◔◑◕⬢) - In progress, active
- **Red** (◌▩✗) - Error, failed
- **Blue** (○⬡) - Queued, idle
- **Cyan** (→⇒⇨) - Flow, data movement
- **Magenta** (⟲⚠) - Retry, warning

## Tips

1. **Watch the circles**: Empty ○ → Filled ● shows progress
2. **Tree structure**: `╮` means task has tool details below
3. **Worker numbers**: Shows which parallel worker is running task
4. **Retry loop**: ◌ + ⟲ means task will retry automatically
5. **Quick scan**: Look for ● (done) vs ◌ (failed) at a glance

## Examples by Scenario

### Single Quick Task
```
◍═══╕  1 task volleyed
    ╰──╮ Using x-ai/grok-4-fast

1   ● list all Go files (0.8s, 1.2k tokens)

◍ 1/1 tasks succeeded in 0.8s
```

### Parallel Execution
```
◍═══╕  3 tasks volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ● analyze [read grep]  2.1k $0.0026  1.8s
2 ╮ ● refactor [edit bash]  3.2k $0.0039  2.3s
3 ╮ ● test [write bash]  2.8k $0.0034  2.1s

◍ 3/3 tasks succeeded in 2.8s
  8.1k tokens  $0.0099
```

### Live Progress
```
◍═══╕  3 tasks volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ● completed task [read]  2.1k $0.0026  1.8s
2 ╮ ◕ finalizing task (worker 1)
  ├───▣ edit     file.go  0.4s
  ╰───▥ bash     go test  running...
3   ◔ starting task (worker 2)
```

### Error Recovery
```
◍═══╕  2 tasks volleyed
    ╰──╮ Using claude-3-5-sonnet

1 ╮ ● first task [read]  2.1k $0.0026  1.8s
2   ◌ second task (attempt 3) ⟲
    ⤴ Error: rate limited (429)
    ⏸ Retrying in 4.0s...
```

## Troubleshooting

### Output looks garbled
- Your terminal may not support Unicode
- Try a different terminal emulator
- Check terminal encoding is set to UTF-8

### Symbols don't align
- Ensure monospace font is used
- Some fonts render symbols at different widths
- Try a programming font (Fira Code, JetBrains Mono, etc.)

### Too much detail
- Remove `--verbose` flag for compact output
- Completed tasks auto-collapse tool details

### Not enough detail
- Add `--verbose` flag to see all tool traces
- Use `--workers-view` to see worker pool (future)
- Use `--metrics` for full statistics (future)

## Legend at a Glance

```
States:  ○ ◔ ◑ ◕ ● ◌ ⦿ ◉
Tools:   ▤ ▥ ▣ ▩ ☒
Workers: ⬡ ⬢ ⬣
Status:  ⚠ ✗ ⊗ ⏸ ⚡
Flow:    → ⇒ ⇨ ⟲ ⤴ ↻
Tree:    ╮ ├ ╰ ───
Brand:   ◍
```

---

**The Tennis Factory Metaphor**

Think of Cliffy as a tennis ball factory:
- **Balls** (○) enter the queue
- **Workers** (⬢) process them through machines
- **Stages** (◔◑◕) fill the ball as it's processed
- **Quality checks** (▣) verify each step
- **Finished balls** (●) exit the system
- **Broken balls** (◌) get retried (⟲) or discarded (✗)

Keep volleying! 🎾
