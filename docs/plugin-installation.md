# Crush Plugin Installation Guide

## Quick Start

Install plugins with a single command:

```bash
# Install morph-compact plugin
crush plugin install morph-compact

# Install from local directory
crush plugin install ./my-plugin

# List installed plugins
crush plugin list

# Uninstall plugin
crush plugin uninstall morph-compact
```

## Dependency Management

Crush plugin system uses **pnpm** by default for dependency installation to save disk space and avoid duplicate `node_modules` directories.

### Why pnpm?

- **Disk Space Efficiency**: pnpm uses a content-addressable storage system that stores packages globally and creates hard links to them
- **No Duplication**: Multiple plugins using the same dependency share a single copy
- **Faster Installation**: Cached packages don't need to be re-downloaded

### Fallback to npm

If pnpm is not installed, Crush falls back to npm. To get the best experience, install pnpm:

```bash
npm install -g pnpm
```

### Storage Location

- **pnpm global store**: `~/.pnpm-store/` (Windows: `D:\.pnpm-store\`)
- **Plugin local cache**: `.crush/plugins/[plugin-name]/node_modules/`

## Plugin Structure

A plugin directory should contain:

```
.crush/plugins/
  my-plugin/
    plugin.json      # Plugin metadata
    index.mjs        # Plugin code
    package.json     # npm dependencies (optional)
```

### plugin.json Format

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "description": "My custom plugin",
  "command": "node",
  "args": ["index.mjs"],
  "hooks": ["chat_messages_transform", "session_compacting"],
  "timeout_ms": 60000,
  "env": {
    "MY_API_KEY": "$MY_API_KEY"
  }
}
```

## Built-in Plugins

### morph-compact

Compresses conversation history to save context window space.

```bash
# Install
crush plugin install morph-compact

# Set environment variable
export MORPH_API_KEY="your-api-key"

# The plugin will automatically compress old messages when context exceeds threshold
```

## Tips

1. **Install pnpm first**: Get the best disk space efficiency
2. **Check disk usage**: Use `du -sh .crush/plugins/*/node_modules` to see plugin sizes
3. **Clean unused plugins**: `crush plugin uninstall [plugin-name]`
4. **Global pnpm store**: Check `pnpm store path` to see where packages are cached

## Troubleshooting

### "No package manager found"

Install pnpm or npm:
```bash
npm install -g pnpm
```

### Plugin not loading

1. Check plugin directory: `crush plugin list`
2. Verify plugin.json format
3. Check logs: `crush logs`

### Dependencies not installing

1. Ensure internet connection
2. Try clearing pnpm cache: `pnpm store prune`
3. Check for deprecated packages in `package.json`
