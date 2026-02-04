# Feature: Environment Variable Configuration in Crush

The following plan should be complete, but its important that you validate documentation and codebase patterns and task sanity before you start implementing.

Pay special attention to naming of existing utils types and models. Import from the right files etc.

## Feature Description

This feature enables users to configure environment variables directly within their `crush.json` configuration file. These environment variables will be made available to agentic skills and commands during Crush sessions, eliminating the need for users to set them in their OS environment before starting a session.

## User Story

As a developer working with Crush
I want to define environment variables in my crush.json config file
So that I don't have to manually export them in my shell before launching Crush

## Problem Statement

Currently, developers must manually set environment variables in their OS shell (e.g., `export MY_VAR=value`) before starting a Crush session. This creates friction when working with different projects or contexts that require specific environment variables.

## Solution Statement

Add support for an "env" field in the crush.json configuration file that allows users to specify key-value pairs of environment variables. These will be automatically set in the environment context used by agentic skills and commands during Crush sessions.

## Feature Metadata

**Feature Type**: Enhancement
**Estimated Complexity**: Medium
**Primary Systems Affected**: Configuration system, agent execution context, environment management
**Dependencies**: No external dependencies required

---

## CONTEXT REFERENCES

### Relevant Codebase Files IMPORTANT: YOU MUST READ THESE FILES BEFORE IMPLEMENTING!

- `internal/config/config.go` (lines 1-100) - Why: Main configuration structure and loading logic
- `internal/config/load.go` (lines 1-50) - Why: Configuration file loading patterns  
- `internal/agent/tools/mcp/init.go` (lines 1-30) - Why: Environment handling for MCP tools
- `internal/app/app.go` (lines 128-140) - Why: Application startup and environment setup

### New Files to Create

- No new files required - this feature extends existing configuration structures

### Relevant Documentation YOU SHOULD READ THESE BEFORE IMPLEMENTING!

- [Crush Configuration Docs](https://github.com/charmbracelet/crush/blob/main/README.md#configuration)
  - Specific section: Configuration
  - Why: Understanding how current config works and where to add new fields

### Patterns to Follow

**Naming Conventions:**
- Use snake_case for JSON field names (consistent with existing config structure)
- PascalCase for Go struct fields

**Error Handling:** 
- Return explicit errors when configuration parsing fails
- Log warnings for invalid environment variable configurations

**Logging Pattern:**
- Use `slog.Info` or `slog.Warn` for logging configuration changes
- Follow existing patterns in `internal/config/load.go`

**Other Relevant Patterns:**
- Configuration loading follows a pattern of unmarshaling JSON into structs with validation
- Environment variables are set via `os.Setenv()` calls during application startup

---

## IMPLEMENTATION PLAN

### Phase 1: Foundation

<Describe foundational work needed before main implementation>

**Tasks:**

- Extend configuration struct to include environment variable support
- Add parsing logic for the new "env" field in JSON config
- Implement environment variable setting at app startup

### Phase 2: Core Implementation

<Describe the main implementation work>

**Tasks:**

- Modify `Config` struct to add environment variables field
- Update configuration loading to parse env section
- Implement environment setup during application initialization
- Add validation for environment variable names and values

### Phase 3: Integration

<Describe how feature integrates with existing functionality>

**Tasks:**

- Ensure environment variables are available to agent tools
- Test integration with MCP tool execution context
- Verify compatibility with existing configuration loading patterns

### Phase 4: Testing & Validation

<Describe testing approach>

**Tasks:**

- Add unit tests for config parsing with env section
- Create integration test verifying environment is set correctly
- Validate that environment variables are accessible to agent tools

---

## STEP-BY-STEP TASKS

### CREATE internal/config/env.go

- **IMPLEMENT**: Create new file defining EnvironmentConfig struct and related functions
- **PATTERN**: Follow pattern from `internal/config/provider.go` for struct definition 
- **IMPORTS**: Import "os", "log/slog"
- **GOTCHA**: Ensure environment variable names follow POSIX standards (alphanumeric + underscore, not starting with digit)
- **VALIDATE**: `go build ./...`

### UPDATE internal/config/config.go

- **IMPLEMENT**: Add `Env map[string]string` field to Config struct
- **PATTERN**: Follow existing pattern for other config fields like Providers and Models  
- **IMPORTS**: No additional imports needed
- **GOTCHA**: Keep the field name consistent with JSON naming convention (snake_case)
- **VALIDATE**: `go build ./...`

### UPDATE internal/config/load.go

- **IMPLEMENT**: Add parsing logic to load env section from JSON config
- **PATTERN**: Follow existing pattern in same file for loading other sections like Providers and Models  
- **IMPORTS**: Import "os" 
- **GOTCHA**: Ensure environment variables are set after configuration is loaded but before app initialization
- **VALIDATE**: `go build ./...`

### UPDATE internal/app/app.go

- **IMPLEMENT**: Add logic to apply configured environment variables during application startup
- **PATTERN**: Follow pattern in same file for setting up LSP clients and other services  
- **IMPORTS**: Import "os", "log/slog"
- **GOTCHA**: Set environment variables before initializing agents or tools that might use them
- **VALIDATE**: `go build ./...`

### ADD internal/config/load_test.go

- **IMPLEMENT**: Add test cases for environment variable configuration parsing  
- **PATTERN**: Follow existing tests in same file for other config sections
- **IMPORTS**: Import "testing", "reflect"
- **GOTCHA**: Test both valid and invalid environment variables (empty keys, etc.)
- **VALIDATE**: `go test ./internal/config -run TestLoadConfig`

---

## TESTING STRATEGY

### Unit Tests

Test configuration parsing with various environment variable scenarios including:
- Valid key-value pairs
- Empty values  
- Special characters in values
- Invalid keys that should be rejected
- Missing env section (should default to empty)

### Integration Tests

Verify that configured environment variables are actually set and accessible during agent execution by:
- Creating a test that checks if an environment variable is available via `os.Getenv()`
- Testing with actual agent tools that access the environment

### Edge Cases

- Empty environment map in config
- Environment variable names starting with digits (should be rejected)
- Keys containing invalid characters
- Very long environment values
- Duplicate keys in configuration (last one wins)

---

## VALIDATION COMMANDS

### Level 1: Syntax & Style

```bash
go build ./...
gofumpt -d .
```

### Level 2: Unit Tests

```bash
go test ./internal/config -v
```

### Level 3: Integration Tests

```bash
go test ./internal/app -v
```

### Level 4: Manual Validation

Create a test configuration with environment variables and verify they're set:
1. Create `test-crush.json` with env section  
2. Run `crush --config test-crush.json`
3. Verify environment is accessible in agent context

### Level 5: Additional Validation (Optional)

```bash
go test ./internal/agent/tools -v
```

---

## ACCEPTANCE CRITERIA

- [ ] Feature implements all specified functionality for environment variable configuration  
- [ ] All validation commands pass with zero errors
- [ ] Unit test coverage meets requirements (80%+)
- [ ] Integration tests verify end-to-end workflows 
- [ ] Code follows project conventions and patterns
- [ ] No regressions in existing functionality
- [ ] Environment variables are properly set before agent execution
- [ ] Invalid environment variable names are rejected with appropriate warnings

---

## COMPLETION CHECKLIST

- [ ] All tasks completed in order
- [ ] Each task validation passed immediately  
- [ ] All validation commands executed successfully
- [ ] Full test suite passes (unit + integration)
- [ ] No linting or type checking errors
- [ ] Manual testing confirms feature works
- [ ] Acceptance criteria all met
- [ ] Code reviewed for quality and maintainability

---

## NOTES

This implementation leverages the existing configuration loading infrastructure without requiring major architectural changes. The environment variables will be set early in the application lifecycle, ensuring they're available to all components that might need them including agent tools and MCP servers.

The feature follows established patterns in the codebase where configuration fields are defined as Go structs with JSON tags for serialization/deserialization. Environment variable names follow POSIX standards to ensure compatibility across platforms.

Key design considerations:
1. Environment variables should be set before any agents or tools that might depend on them
2. Invalid environment variable names (e.g., starting with digits) should be rejected gracefully  
3. The implementation should not interfere with existing environment variables already in the system
4. Configuration loading must handle missing env sections gracefully by defaulting to empty map

**Confidence Score**: 8/10 that execution will succeed on first attempt