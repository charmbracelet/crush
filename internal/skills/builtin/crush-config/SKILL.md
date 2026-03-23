---
name: crush-config
description: Configure Crush settings including providers, LSPs, MCPs, skills, permissions, and behavior options. Use when the user needs help with crush.json configuration, setting up providers, configuring LSPs, adding MCP servers, or changing Crush behavior.
---

# Crush Configuration

Crush uses JSON configuration files with the following priority (highest to lowest):

1. `.crush.json` (project-local, hidden)
2. `crush.json` (project-local)
3. `$HOME/.config/crush/crush.json` (global)

## Basic Structure

```json
{
  "$schema": "https://charm.land/crush.json",
  "options": {}
}
```

The `$schema` property enables IDE autocomplete but is optional.

## Common Configurations

### Project-Local Skills

Add a relative path to keep project-specific skills alongside your code:

```json
{
  "options": {
    "skills_paths": ["./skills"]
  }
}
```

### LSP Configuration

```json
{
  "lsp": {
    "go": {
      "command": "gopls",
      "env": {
        "GOTOOLCHAIN": "go1.24.5"
      }
    },
    "typescript": {
      "command": "typescript-language-server",
      "args": ["--stdio"]
    }
  }
}
```

### MCP Servers

```json
{
  "mcp": {
    "filesystem": {
      "type": "stdio",
      "command": "node",
      "args": ["/path/to/mcp-server.js"]
    },
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "Bearer $GH_PAT"
      }
    }
  }
}
```

### Custom Provider

```json
{
  "providers": {
    "deepseek": {
      "type": "openai-compat",
      "base_url": "https://api.deepseek.com/v1",
      "api_key": "$DEEPSEEK_API_KEY",
      "models": [
        {
          "id": "deepseek-chat",
          "name": "Deepseek V3",
          "context_window": 64000
        }
      ]
    }
  }
}
```

### Tool Permissions

```json
{
  "permissions": {
    "allowed_tools": ["view", "ls", "grep", "edit"]
  }
}
```

### Disable Built-in Tools

```json
{
  "options": {
    "disabled_tools": ["bash", "sourcegraph"]
  }
}
```

### Debug Options

```json
{
  "options": {
    "debug": true,
    "debug_lsp": true
  }
}
```

### Attribution Settings

```json
{
  "options": {
    "attribution": {
      "trailer_style": "assisted-by",
      "generated_with": true
    }
  }
}
```

## Environment Variables

- `CRUSH_GLOBAL_CONFIG` - Override global config location
- `CRUSH_GLOBAL_DATA` - Override data directory location
- `CRUSH_SKILLS_DIR` - Override default skills directory

## Provider Types

- `openai` - For OpenAI or OpenAI-compatible APIs that route through OpenAI
- `openai-compat` - For non-OpenAI providers with OpenAI-compatible APIs
- `anthropic` - For Anthropic-compatible APIs
