# Detailed Code Changes for Blush Rebranding

## 1. Main Application Changes

### main.go
- Environment variable check: `CRUSH_PROFILE` → `BLUSH_PROFILE`

### internal/cmd/root.go
- Command usage: `"crush"` → `"blush"`
- Flag descriptions: "Custom crush data directory" → "Custom blush data directory"
- Long description references to "Crush" → "Blush"
- Examples in help text

### internal/cmd/run.go
- Error messages: "no providers configured - please run 'crush' to set up a provider" → "no providers configured - please run 'blush' to set up a provider"

### internal/cmd/update_providers.go
- Examples in help text

### internal/cmd/logs.go
- Short description: "View crush logs" → "View blush logs"
- Log file path: `crush.log` → `blush.log`
- Warning messages: "Looks like you are not in a crush project" → "Looks like you are not in a blush project"

## 2. Configuration Changes

### internal/config/config.go
- Application name variable: `appName = "crush"` → `appName = "blush"`
- Default data directory: `defaultDataDirectory = ".crush"` → `defaultDataDirectory = ".blush"`

### internal/config/load.go
- Environment variable: `CRUSH_DISABLE_PROVIDER_AUTO_UPDATE` → `BLUSH_DISABLE_PROVIDER_AUTO_UPDATE`

### Schema references throughout config package

## 3. File System Changes

### internal/fsext/ls.go
- File path: `".config", "crush"` → `".config", "blush"`

### internal/fsext/fileutil.go
- Ignore list entry: `".crush"` → `".blush"`

### internal/home/home.go
- Temporary directory prefix: `os.MkdirTemp("crush", "")` → `os.MkdirTemp("blush", "")`

## 4. LLM and Agent Changes

### internal/llm/agent/mcp-tools.go
- Agent name: `"Crush"` → `"Blush"`

### internal/llm/prompt/coder.go
- Environment variable: `CRUSH_CODER_V2` → `BLUSH_CODER_V2`

## 5. TUI Changes

### internal/tui/components/chat/header/header.go
- Display text: `"CRUSH"` → `"BLUSH"`

### internal/tui/components/core/core.go
- Style name: `"crush"` → `"blush"`

### internal/tui/components/dialogs/commands/loader.go
- File path: `"crush", "commands"` → `"blush", "commands"`

### internal/tui/components/logo/logo.go
- Display text: `"Crush"` → `"Blush"`

### internal/tui/highlight/highlight.go
- Style name: `"crush"` → `"blush"`

## 6. Database and Session Changes

### internal/db/db.go
- Database file name references

### internal/session/session.go
- Session file paths

## 7. Logging Changes

### internal/log/log.go
- Log file paths
- Application name in logs

## 8. Utility Functions

### internal/csync/csync.go
- File paths and names

### internal/format/format.go
- File paths and names

## 9. Testing Files

### All test files (*.go files in _test.go)
- Expected output strings
- Test descriptions
- Golden file references
- Mock data references

## 10. Configuration Schema

### schema.json
- Description updates
- Examples that reference "crush"
- Default values for data directory

## 11. Spell Check Dictionary

### cspell.json
- Word list entry: `"crush"` → `"blush"`