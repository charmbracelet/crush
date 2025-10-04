# STDIN and Task File Support Implementation

## Summary

Implemented improvement #5 from IMPROVEMENT_REPORT.md: Support for reading tasks from STDIN and task files, enabling better integration with shell pipelines and scripts.

## Files Modified/Created

### 1. `/Users/bwl/Developer/cliffy/cmd/cliffy/task_parser.go` (NEW)
- Added comprehensive task parsing functionality
- Supports three input modes:
  - CLI arguments (existing, backward compatible)
  - Task files via `--tasks-file <path>`
  - STDIN via `-` argument
- Supports two formats:
  - Line-separated text (default)
  - JSON array via `--json` flag
- Includes comment support (lines starting with `#` are ignored)
- Handles empty lines gracefully

## Key Functions

### `parseTasks(args []string, tasksFile string, tasksJSON bool) ([]string, error)`
Main entry point that routes to appropriate parser based on input source:
1. Priority 1: `--tasks-file` flag
2. Priority 2: `-` argument (STDIN)
3. Priority 3: CLI arguments (default)

### `parseTasksFromFile(filepath string, asJSON bool) ([]string, error)`
Reads tasks from a file, supporting both text and JSON formats.

### `parseTasksFromReader(r io.Reader, asJSON bool) ([]string, error)`
Core reader function used by both file and STDIN parsing.

### `parseJSONTasks(r io.Reader) ([]string, error)`
Parses JSON array format: `["task1", "task2", "task3"]`
- Filters empty strings
- Trims whitespace

### `parseLineTasks(r io.Reader) ([]string, error)`
Parses newline-separated format:
```
task1
task2
# comment (ignored)
task3
```
- Skips empty lines
- Ignores comment lines (starting with `#`)

## Required Changes to main.go

The following variables and flags need to be added to `cmd/cliffy/main.go`:

```go
var (
    // ... existing vars ...
    tasksFile string
    tasksJSON bool
)

func init() {
    // ... existing flags ...
    rootCmd.Flags().StringVar(&tasksFile, "tasks-file", "", "Load tasks from file (one per line, or JSON array if --json)")
    rootCmd.Flags().BoolVar(&tasksJSON, "json", false, "Parse tasks as JSON array (for STDIN or --tasks-file)")
}
```

Update the Args validator:
```go
Args: func(cmd *cobra.Command, args []string) error {
    if showVersion {
        return nil
    }
    // Allow no args if reading from file or STDIN
    if tasksFile != "" {
        return nil
    }
    if len(args) == 1 && args[0] == "-" {
        return nil
    }
    return cobra.MinimumNArgs(1)(cmd, args)
},
```

Update RunE to parse tasks:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    // ... existing code ...

    // Parse tasks from various input sources
    taskPrompts, err := parseTasks(args, tasksFile, tasksJSON)
    if err != nil {
        return fmt.Errorf("failed to parse tasks: %w", err)
    }

    if len(taskPrompts) == 0 {
        return fmt.Errorf("no tasks provided")
    }

    // Route based on task count
    if len(taskPrompts) == 1 {
        return executeSingleTask(cmd, taskPrompts[0], verbosity)
    }
    return executeVolley(cmd, taskPrompts, verbosity)
},
```

Update executeVolley to accept []string instead of args:
```go
func executeVolley(cmd *cobra.Command, taskPrompts []string, verbosity config.VerbosityLevel) error {
    // ... existing setup ...

    // Convert prompts to tasks
    tasks := make([]volley.Task, len(taskPrompts))
    for i, prompt := range taskPrompts {
        tasks[i] = volley.Task{
            Index:  i + 1,
            Prompt: prompt,
        }
    }

    // ... rest of function ...
}
```

## Usage Examples

### 1. Tasks from file (line-separated)
```bash
# Create a tasks file
cat > tasks.txt <<EOF
analyze auth.go
analyze db.go
analyze api.go
EOF

# Run tasks from file
cliffy --tasks-file tasks.txt
```

### 2. Tasks from file (JSON format)
```bash
# Create JSON tasks file
echo '["analyze auth.go", "analyze db.go", "analyze api.go"]' > tasks.json

# Run with JSON flag
cliffy --json --tasks-file tasks.json
```

### 3. Tasks from STDIN (line-separated)
```bash
# Pipe tasks to cliffy
echo -e "task1\ntask2\ntask3" | cliffy -

# From a file
cat tasks.txt | cliffy -

# Shell pipeline integration
find . -name "*.go" | head -3 | \
  xargs -I {} echo "analyze {}" | cliffy -
```

### 4. Tasks from STDIN (JSON format)
```bash
# JSON array from stdin
echo '["analyze auth.go", "analyze db.go"]' | cliffy --json -

# From API or generator
curl https://api.example.com/tasks | jq -r '.tasks[]' | \
  jq -R -s -c 'split("\n")[:-1]' | cliffy --json -
```

### 5. With shared context
```bash
# Combine with context flags
cliffy --context "You are a security expert" --tasks-file security-tasks.txt

# Or with context file
cliffy --context-file security-rules.md --tasks-file auth-files.txt
```

### 6. Comments in task files
```bash
cat > review-tasks.txt <<EOF
# Authentication layer
review auth/login.go
review auth/session.go

# Database layer
review db/users.go
review db/migrations.go
EOF

cliffy --tasks-file review-tasks.txt
```

## Backward Compatibility

✅ All existing CLI usage patterns continue to work:
- `cliffy "single task"`
- `cliffy "task1" "task2" "task3"`
- All flags work as before

## Benefits

1. **Shell Pipeline Integration**: Easily integrate cliffy into shell scripts and pipelines
2. **Batch Processing**: Store recurring task lists in files for reuse
3. **API Integration**: Consume tasks from APIs or other tools via JSON
4. **Automation**: Better scriptability for CI/CD and automation workflows
5. **Flexibility**: Mix and match input methods as needed

## Testing

To test the implementation:

1. Build the project:
```bash
go build -o bin/cliffy ./cmd/cliffy
```

2. Test STDIN:
```bash
echo -e "what is 2+2?\nwhat is 3+3?" | ./bin/cliffy -
```

3. Test task file:
```bash
echo -e "task1\ntask2" > /tmp/tasks.txt
./bin/cliffy --tasks-file /tmp/tasks.txt
```

4. Test JSON:
```bash
echo '["task1", "task2"]' | ./bin/cliffy --json -
```

## Notes

- The implementation gracefully handles empty lines and comments in text files
- JSON parsing validates and filters empty strings
- Error messages clearly indicate parsing failures
- Maintains the existing single-task vs multi-task routing logic
