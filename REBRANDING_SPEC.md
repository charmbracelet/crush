# Blush Rebranding Specification

This document outlines all the changes required to rebrand the "Crush" CLI application to "Blush".

## 1. Application Name Changes

### Core Application References
- Command name: `crush` → `blush`
- Application name: "Crush" → "Blush"
- Module name: `github.com/charmbracelet/crush` → `github.com/nom-nom-hub/blush`

### Executable Name
- Build output: `crush`/`crush.exe` → `blush`/`blush.exe`

## 2. Code Changes

### Command Line Interface
- Root command usage: `"crush"` → `"blush"`
- Flag descriptions referencing "crush data directory" → "blush data directory"
- Error messages referencing "crush" → "blush"
- Examples in help text

### Configuration
- Default data directory: `".crush"` → `".blush"`
- Config file names: `crush.json` → `blush.json`
- Environment variables:
  - `CRUSH_PROFILE` → `BLUSH_PROFILE`
  - `CRUSH_DISABLE_PROVIDER_AUTO_UPDATE` → `BLUSH_DISABLE_PROVIDER_AUTO_UPDATE`
  - `CRUSH_CODER_V2` → `BLUSH_CODER_V2`

### Internal References
- Application name in logs and UI
- Module imports (update import paths)
- String literals throughout the codebase
- TUI display text ("CRUSH" → "BLUSH")

### File Structure
- Log file paths: `.crush/logs/crush.log` → `.blush/logs/blush.log`
- Data directory: `.crush` → `.blush`
- Ignore files: `.crushignore` → `.blushignore`

## 3. Documentation Changes

### README.md
- Project title: "Crush" → "Blush"
- Description updates
- Installation instructions
- Usage examples
- Configuration examples
- All references to "crush" command

### Other Documentation
- CRUSH.md → BLUSH.md (development guide)
- Comments in code examples
- Schema descriptions

## 4. Build and Release Changes

### Taskfile.yaml
- Task names and descriptions
- Generated file names
- Environment variable references

### .goreleaser.yml
- `project_name`: `crush` → `blush`
- Homepage URL
- File naming templates
- Package installation paths
- Completion script names
- Man page names
- AUR package names
- Homebrew formula names
- Scoop package names
- npm package names
- All file paths and references

### go.mod
- Module declaration: `github.com/charmbracelet/crush` → `github.com/nom-nom-hub/blush`

## 5. Configuration Schema Changes

### schema.json
- Description updates
- Examples that reference "crush"
- Default values for data directory

## 6. GitHub and CI/CD

### GitHub Workflows
- Workflow names
- File paths
- Environment variables
- Release configurations

### Issue Templates
- References to "crush"
- Command examples

## 7. Package Management

### Package Installation Scripts
- Homebrew formula
- AUR packages
- Scoop manifests
- NPM package
- Winget package
- Debian/RPM packages

## 8. Website and Marketing

### Project Website
- URLs: `https://charm.sh/crush` → `https://charm.sh/blush`
- Content updates
- Logo and branding

### Social Media and Documentation
- References in documentation
- Blog posts
- Announcement materials

## 9. Testing

### Test Files
- Test names and descriptions
- Expected output in tests
- Golden files that contain "crush"

## 10. Migration Considerations

### Backward Compatibility
- Option to read existing `.crush` directories
- Migration path for existing users
- Configuration file compatibility

### Data Migration
- Existing log files
- Database files
- Configuration files