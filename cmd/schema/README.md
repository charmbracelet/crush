# Crush Configuration Schema Generator

This tool automatically generates a JSON Schema for the Crush configuration file by using Go reflection to analyze the configuration structs. The schema provides validation, autocompletion, and documentation for configuration files.

## Features

- **Automated Generation**: Uses reflection to automatically generate schemas from Go structs
- **Always Up-to-Date**: Schema stays in sync with code changes automatically
- **Comprehensive**: Includes all configuration options, types, and validation rules
- **Enhanced**: Adds provider enums, model lists, and custom descriptions
- **Extensible**: Easy to add new fields and modify existing ones

## Usage

```bash
# Generate the schema
go run cmd/schema/main.go > crush-schema.json

# Or use the task runner
task schema
```

## How It Works

The generator:

1. **Reflects on Config Structs**: Analyzes the `config.Config` struct and all related types
2. **Generates Base Schema**: Creates JSON Schema definitions for all struct fields
3. **Enhances with Runtime Data**: Adds provider lists, model enums, and tool lists from the actual codebase
4. **Adds Custom Descriptions**: Provides meaningful descriptions for configuration options
5. **Sets Default Values**: Includes appropriate defaults for optional fields

## Schema Features

The generated schema includes:

- **Type Safety**: Proper type definitions for all configuration fields
- **Validation**: Required fields, enum constraints, and format validation
- **Documentation**: Descriptions for all configuration options
- **Defaults**: Default values for optional settings
- **Provider Enums**: Current list of supported providers
- **Model Enums**: Available models from all configured providers
- **Tool Lists**: Valid tool names for agent configurations
- **Cross-References**: Proper relationships between different config sections

## Adding New Configuration Fields

To add new configuration options:

1. **Add to Config Structs**: Add the field to the appropriate struct in `internal/config/`
2. **Add JSON Tags**: Include proper JSON tags with field names
3. **Regenerate Schema**: Run the schema generator to update the JSON schema
4. **Update Validation**: Add any custom validation logic if needed

Example:
```go
type Options struct {
    // ... existing fields ...
    
    // New field with JSON tag and description
    NewFeature bool `json:"new_feature,omitempty"`
}
```

The schema generator will automatically:
- Detect the new field
- Generate appropriate JSON schema
- Add type information
- Include in validation

## Using the Schema

### Editor Integration

Most modern editors support JSON Schema:

**VS Code**: Add to your workspace settings:
```json
{
  "json.schemas": [
    {
      "fileMatch": ["crush.json", ".crush.json"],
      "url": "./crush-schema.json"
    }
  ]
}
```

**JetBrains IDEs**: Configure in Settings → Languages & Frameworks → Schemas and DTDs → JSON Schema Mappings

### Validation Tools

```bash
# Using jsonschema (Python)
pip install jsonschema
jsonschema -i crush.json crush-schema.json

# Using ajv-cli (Node.js)
npm install -g ajv-cli
ajv validate -s crush-schema.json -d crush.json
```

### Configuration Example

```json
{
  "models": {
    "large": {
      "model_id": "claude-3-5-sonnet-20241022",
      "provider": "anthropic",
      "reasoning_effort": "medium",
      "max_tokens": 8192
    },
    "small": {
      "model_id": "claude-3-5-haiku-20241022", 
      "provider": "anthropic"
    }
  },
  "providers": {
    "anthropic": {
      "id": "anthropic",
      "provider_type": "anthropic",
      "api_key": "your-api-key",
      "disabled": false
    }
  },
  "agents": {
    "coder": {
      "id": "coder",
      "name": "Coder",
      "model": "large",
      "disabled": false
    },
    "custom-agent": {
      "id": "custom-agent",
      "name": "Custom Agent",
      "description": "A custom agent for specific tasks",
      "model": "small",
      "allowed_tools": ["glob", "grep", "view"],
      "allowed_mcp": {
        "filesystem": ["read", "write"]
      }
    }
  },
  "mcp": {
    "filesystem": {
      "command": "mcp-filesystem",
      "args": ["--root", "/workspace"],
      "type": "stdio"
    }
  },
  "lsp": {
    "typescript": {
      "command": "typescript-language-server",
      "args": ["--stdio"],
      "enabled": true
    }
  },
  "options": {
    "context_paths": [
      "README.md",
      "docs/",
      ".cursorrules"
    ],
    "data_directory": ".crush",
    "debug": false,
    "tui": {
      "compact_mode": false
    }
  }
}
```

## Maintenance

The schema generator is designed to be maintenance-free. As long as:

- Configuration structs have proper JSON tags
- New enums are added to the enhancement functions
- The generator is run after significant config changes

The schema will stay current with the codebase automatically.