Dispatch a task to a registered worker for asynchronous processing.

This tool creates a dispatch document in the file cabinet and spawns the worker. The worker will:
1. Read its task from the API endpoint
2. Execute the task using its own tools
3. Submit the result back to the dispatch system
4. Exit

The dispatch is asynchronous - this tool returns immediately with a dispatch ID. Use the dispatch ID to check status or retrieve results later.

## Parameters

- **worker** (required): Name of a registered worker (must exist in the registry)
- **task** (required): The task description for the worker to execute
- **variables** (optional): Structured data passed to the worker (file paths, error messages, etc.)

## How It Works

1. Validates the worker exists and is enabled
2. Creates a dispatch document via the API
3. Spawns the worker with a preconfigured alert containing:
   - API endpoint
   - Dispatch ID
   - Spawn token (for authentication)
4. Returns the dispatch ID

## Example

```
dispatch_task(
  worker: "goose",
  task: "Fix the auth bug in login.go",
  variables: {
    file: "internal/auth/login.go",
    error: "invalid token on line 42"
  }
)
```

Returns:
```
Task dispatched to goose.
Dispatch ID: abc123
The worker has been spawned and will process this task.
```

## Architecture

The worker is spawned with a minimal alert message. The task details live in the dispatch document, not in the spawn command. This ensures:

- All task content is audited in the file cabinet
- Workers can't bypass the document system
- Task history is preserved

Workers read their instructions from the API, do their work, and submit results back to the API. They are ephemeral - spawned, work, exit.
