# Configuration Reference

Crush can be configured via JSON files at different levels of priority.

## Configuration Files
1. `.crush.json` (Local to project)
2. `crush.json` (Local to project)
3. `$HOME/.config/crush/crush.json` (Global user config)

## Configuration Schema

```json
{
  "$schema": "https://charm.land/crush.json",
  "models": {
    "large": {
      "model": "claude-3-5-sonnet-20241022",
      "provider": "anthropic"
    },
    "small": {
      "model": "claude-3-haiku-20240307",
      "provider": "anthropic"
    }
  },
  "providers": {
    "openai": {
      "api_key": "$OPENAI_API_KEY",
      "base_url": "https://api.openai.com/v1"
    }
  },
  "options": {
    "context_paths": [".cursorrules", "CRUSH.md"],
    "debug": false,
    "disable_auto_summarize": false
  }
}
```

## Key Configuration Options

### `models`
Defines the primary models used by Crush.
- `large`: Used for complex reasoning and code generation.
- `small`: Used for quick tasks like generating session titles.

### `providers`
Configures AI providers. Supports `openai`, `anthropic`, `gemini`, `azure`, `bedrock`, `google-vertex`, and `openai-compat`.
- `api_key`: Can be a literal string or an environment variable (e.g., `$MY_KEY`).
- `base_url`: Override for custom endpoints (proxies or local models like Ollama).

### `lsp`
Configures Language Server Protocol servers.
```json
"lsp": {
  "go": {
    "command": "gopls",
    "enabled": true
  }
}
```

### `mcp`
Configures Model Context Protocol servers.
```json
"mcp": {
  "my-tools": {
    "type": "stdio",
    "command": "node",
    "args": ["server.js"]
  }
}
```

### `permissions`
Controls tool execution safety.
- `allowed_tools`: List of tools that don't require confirmation.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `CRUSH_GLOBAL_CONFIG` | Path to the global config file. |
| `CRUSH_GLOBAL_DATA` | Path to the global data directory. |
| `CRUSH_SKILLS_DIR` | Path to the Agent Skills directory. |
| `ANTHROPIC_API_KEY` | Default key for Anthropic. |
| `OPENAI_API_KEY` | Default key for OpenAI. |
| `CRUSH_DISABLE_METRICS` | Set to `1` to disable usage metrics. |
