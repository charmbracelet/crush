Spawns a new Crush instance as a subprocess to perform tasks independently.

This tool allows the agent to launch parallel Crush instances for isolated task execution.
Each instance runs in a separate process with its own context and tool access.

Use this tool when:
- You need to perform complex, multi-step tasks that would benefit from parallel execution
- A task requires isolated context to avoid polluting the current session
- You want to explore multiple approaches simultaneously
- A task might require multiple independent tool chains
- The user requests multiple separate analyses or investigations

**Important Notes:**
- Each spawned instance uses the same model configuration unless explicitly specified
- Results are returned only after the subprocess completes (no streaming yet)
- Sub-processes inherit the current working directory
- All subprocesses are automatically cleaned up if the parent context is cancelled
- Token usage and costs are tracked separately for each instance

**Parameters:**
- `prompt` (required): The task description for the Crush instance
- `model` (optional): Specific model to use (e.g., 'gpt-4', 'openai/gpt-4', 'anthropic/claude-sonnet-4')

**Examples:**
```
{"prompt": "Analyze the performance of the database queries in src/db/query.go"}
{"prompt": "Review the authentication flow and identify security vulnerabilities", "model": "openai/gpt-4"}
```
