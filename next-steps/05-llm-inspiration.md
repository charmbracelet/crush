# LLM Project Inspiration

Analysis of Simon Willison's [LLM project](https://llm.datasette.io/) for ideas to improve Cliffy.

## Overview of LLM

**LLM** is a CLI tool and Python library for interacting with Large Language Models, created by Simon Willison (creator of Datasette).

### Core Philosophy
- **Unified interface** across multiple model providers
- **Developer-first** CLI and Python API
- **Extensible** through comprehensive plugin system
- **Logging by default** - SQLite database tracks all interactions
- **Template-driven** - Save and reuse prompts with variables

### Key Features
1. **Multi-provider support** - OpenAI, Anthropic, Google, local models
2. **Interactive chat mode** - Conversation continuation
3. **Plugin architecture** - Add models, tools, commands, templates
4. **Prompt templates** - Save/reuse with variables and defaults
5. **Model aliases** - Short names like "4o" for "gpt-4o"
6. **Logging & search** - Full conversation history in SQLite
7. **Embeddings** - Generate and search similar text
8. **Attachments** - Images and files as input
9. **Schema extraction** - Structured JSON output
10. **Code extraction** - Auto-extract code blocks

## Ideas Directly Applicable to Cliffy

### 1. Model Aliases System ⭐⭐⭐

**What LLM does:**
```bash
llm aliases set 4o gpt-4o
llm -m 4o "hello"  # Uses gpt-4o

llm aliases list
# 4o -> gpt-4o
# sonnet -> claude-sonnet-4
```

**Why it's good for Cliffy:**
- Typing `cliffy --model grok-4-fast` is tedious
- Users develop favorites and want shortcuts
- More memorable than full model IDs

**Implementation for Cliffy:**
```bash
# List available models
cliffy models

# Set alias
cliffy alias set fast grok-4-fast
cliffy alias set smart claude-sonnet-4

# Use alias
cliffy -m fast "quick task"
cliffy --fast  # Built-in alias for "small"
```

**Storage:** `~/.config/cliffy/aliases.json`
```json
{
  "fast": "x-ai/grok-4-fast:free",
  "smart": "anthropic/claude-sonnet-4",
  "cheap": "x-ai/grok-4-fast:free"
}
```

**Priority:** High - Simple to implement, huge usability win

### 2. Model Discovery Commands ⭐⭐⭐

**What LLM does:**
```bash
llm models                    # List all available
llm models -q gpt-4           # Search models
llm models default            # Show default model
llm models set-default 4o     # Change default
```

**Why it's good for Cliffy:**
- Users don't know what models are configured
- Hard to remember exact model IDs from config
- Makes exploring providers easier

**Implementation for Cliffy:**
```bash
cliffy models
# Available models:
# • large (x-ai/grok-4-fast:free) [default]
# • small (x-ai/grok-4-fast:free)

cliffy models --provider openrouter
# Shows all OpenRouter models from config
```

**Priority:** High - Discoverability is key for UX

### 3. Code Extraction Flag ⭐⭐

**What LLM does:**
```bash
llm -x "write python to parse json"
# Outputs ONLY the code block, no explanation
```

**Why it's good for Cliffy:**
- Automation needs code without prose
- Pipe directly to file or interpreter
- Common use case: "generate script"

**Implementation for Cliffy:**
```bash
cliffy --extract-code "write bash to count files" > script.sh
cliffy -x "python to parse csv" | python -
```

**Priority:** Medium-high - Specific but valuable for automation

### 4. Key Management Helpers ⭐⭐

**What LLM does:**
```bash
llm keys set openai            # Interactive prompt
llm keys list                  # Show configured keys
```

**Why it's good for Cliffy:**
- First-time setup is friction
- Easier than editing config JSON
- Can validate keys immediately

**Implementation for Cliffy:**
```bash
cliffy keys set openrouter
# Prompt: Enter OpenRouter API key:
# Validates and saves to config

cliffy keys list
# openrouter: sk-...xyz (set)
# anthropic: (not set)
```

**Priority:** Medium - Nice-to-have, not critical

## Ideas to Adapt for Cliffy

### 5. Opt-in Logging ⭐⭐

**What LLM does:**
- Logs ALL prompts/responses to SQLite by default
- `llm logs` to view history
- Search, filter, export conversations

**Cliffy's twist:**
- Default: **No logging** (zero persistence philosophy)
- Opt-in: `--log` flag saves to SQLite
- Useful for auditing automation, debugging prompts

**Why adapt, not adopt:**
- Cliffy = one-off tasks, LLM = research/exploration
- Most Cliffy users don't want history
- But CI/CD users might want audit trail

**Implementation:**
```bash
# Default: No logging
cliffy "task"

# Opt-in logging
cliffy --log "task"

# View logs (if any exist)
cliffy logs
cliffy logs --last 10
cliffy logs --search "error"
```

**Storage:** `~/.local/share/cliffy/logs.db` (only if used)

**Priority:** Medium - Useful for specific use cases, not core

### 6. Template System (Simplified) ⭐

**What LLM does:**
- Save prompts as YAML templates
- Variables like `$input`, custom params
- System prompts, model settings

**Cliffy's twist:**
- Much simpler - just prompt text + model
- No system prompts (we have agents/prompts)
- Focus on automation shortcuts

**Why adapt:**
- Full template system is overkill for one-off tasks
- But saved prompts are useful for repeated automation

**Implementation:**
```bash
# Save a prompt template
cliffy --save-as refactor "refactor this code for readability"

# Use template
cliffy refactor file.go

# List templates
cliffy templates
# • refactor (fast model)
# • summarize (smart model)
```

**Storage:** `~/.config/cliffy/templates/`
```json
{
  "name": "refactor",
  "prompt": "refactor this code for readability",
  "model": "fast"
}
```

**Priority:** Low-medium - Nice but not essential

### 7. Multi-line Input Helpers ⭐

**What LLM does:**
- Reads stdin if no prompt provided
- Interactive mode for complex prompts

**Cliffy's twist:**
- Already supports stdin via tools
- Add explicit flag for clarity

**Implementation:**
```bash
# Read from stdin
echo "some context" | cliffy "explain this"
cat file.txt | cliffy "summarize"

# Heredoc support
cliffy "$(cat <<EOF
Refactor this:
- Make it faster
- Add tests
EOF
)"
```

**Priority:** Low - Already possible, just document it

## Ideas to Skip

### ❌ Interactive Chat Mode

**What LLM does:** `llm chat` for back-and-forth conversations

**Why skip:**
- Cliffy is optimized for one-off tasks
- Conversation state = complexity we removed
- Use Crush if you want chat sessions

### ❌ Embeddings Support

**What LLM does:** `llm embed`, similarity search

**Why skip:**
- Out of scope for coding assistant
- Adds complexity without clear benefit
- Specialized use case

### ❌ Plugin System (Initially)

**What LLM does:** Extensible plugins for models, tools, commands

**Why skip (for now):**
- Cliffy already has tool system from Crush
- Plugin architecture requires stable API
- Focus on core experience first
- Could revisit after 1.0

### ❌ Python Library

**What LLM does:** Full Python API for programmatic use

**Why skip:**
- Cliffy is CLI-focused
- Go library already exists (internal packages)
- Different design goals

## Priority Implementation Roadmap

### Phase 1: Low-Hanging Fruit (Week 1)
**Focus:** Easy wins with big UX impact

1. **Model aliases** (`cliffy alias set/list`)
   - Simple JSON file storage
   - Resolve aliases in model flag
   - 2-3 hours work

2. **Model listing** (`cliffy models`)
   - Read from current config
   - Show available models
   - 1-2 hours work

3. **Code extraction** (`--extract-code`)
   - Parse output for code blocks
   - Strip markdown formatting
   - 2-3 hours work

**Total effort:** ~1 day
**Impact:** High - daily usability improvement

### Phase 2: Configuration Helpers (Week 2)
**Focus:** Reduce setup friction

1. **Key management** (`cliffy keys set/list`)
   - Interactive key input
   - Update config JSON
   - Validation helpers

2. **Model discovery** (`cliffy models --provider`)
   - Query provider for available models
   - Update config with new models
   - Smart defaults

**Total effort:** 2-3 days
**Impact:** Medium-high - better onboarding

### Phase 3: Power User Features (Week 3+)
**Focus:** Advanced automation

1. **Opt-in logging** (`--log`, `cliffy logs`)
   - SQLite storage (only if used)
   - Basic search and export
   - Keep it minimal

2. **Simple templates** (`--save-as`, `cliffy templates`)
   - JSON storage for prompts
   - Variable substitution
   - No complex features

**Total effort:** 4-5 days
**Impact:** Medium - useful for specific workflows

## Detailed Implementation: Model Aliases

### User Stories

**Story 1: Developer with preferred models**
```bash
# Setup once
cliffy alias set work claude-sonnet-4
cliffy alias set quick grok-4-fast

# Daily use
cliffy -m work "review this PR"
cliffy -m quick "what's 2+2"
```

**Story 2: Team standardization**
```bash
# Share aliases.json in repo
echo '{"standard": "gpt-4o"}' > .cliffy-aliases.json
cliffy -m standard "generate tests"
```

### Technical Design

**Alias resolution order:**
1. Check built-in aliases (`--fast` → `small`)
2. Check user aliases (`~/.config/cliffy/aliases.json`)
3. Check project aliases (`./.cliffy-aliases.json`)
4. Treat as literal model name

**Code structure:**
```go
// internal/config/aliases.go
type AliasStore struct {
    aliases map[string]string
}

func (a *AliasStore) Resolve(name string) string {
    if resolved, ok := a.aliases[name]; ok {
        return resolved
    }
    return name  // Not an alias, use as-is
}

func LoadAliases() (*AliasStore, error) {
    // Load from global config
    global := loadJSON("~/.config/cliffy/aliases.json")

    // Load from project (optional)
    project := loadJSON("./.cliffy-aliases.json")

    // Merge (project overrides global)
    return &AliasStore{aliases: merge(global, project)}, nil
}
```

**CLI commands:**
```go
// cmd/cliffy/alias.go
var aliasCmd = &cobra.Command{
    Use: "alias",
    Short: "Manage model aliases",
}

var aliasSetCmd = &cobra.Command{
    Use: "set <alias> <model>",
    Args: cobra.ExactArgs(2),
    Run: func(cmd *cobra.Command, args []string) {
        // Read existing aliases
        // Add new alias
        // Write back to JSON
    },
}

var aliasListCmd = &cobra.Command{
    Use: "list",
    Run: func(cmd *cobra.Command, args []string) {
        // Read aliases
        // Print formatted list
    },
}
```

### Testing Strategy

```bash
# Test alias resolution
cliffy alias set test grok-4-fast
cliffy -m test "hello"  # Should use grok-4-fast

# Test built-in aliases
cliffy --fast "hello"   # Should use small model

# Test non-existent alias
cliffy -m fake "hello"  # Should error clearly

# Test project overrides
echo '{"test": "different-model"}' > .cliffy-aliases.json
cliffy -m test "hello"  # Should use project alias
```

## Questions to Consider

### 1. Default Model Behavior

**LLM approach:** Explicit default via `llm models default`

**Cliffy options:**
- A) Keep current: Use "large" from config
- B) Add default alias: `cliffy alias set-default fast`
- C) Implicit: Last-used model becomes default

**Recommendation:** Keep current, add `alias set-default` later if needed

### 2. Alias Scope

**Options:**
- Global only (`~/.config/cliffy/aliases.json`)
- Project-specific (`.cliffy-aliases.json`)
- Both (project overrides global)

**Recommendation:** Both - team collaboration benefit

### 3. Template Complexity

**Question:** How complex should templates be?

**Options:**
- A) Just prompt text
- B) Prompt + model
- C) Prompt + model + flags

**Recommendation:** Start with A, add B if users ask for it

## Success Metrics

### Phase 1 Success
- Model aliases reduce typing by 50%
- Users discover models via `cliffy models`
- Code extraction enables direct piping

### Phase 2 Success
- First-time setup takes <2 minutes
- No manual config editing needed
- Clear error messages guide users

### Phase 3 Success
- Power users adopt logging for audit trails
- Templates reduce repeated automation tasks
- CI/CD integrations easier to set up

## Related Documentation

See also:
- [03-personality-branding.md](./03-personality-branding.md) - Cliffy's CLI design philosophy
- [04-performance-scaling.md](./04-performance-scaling.md) - Token usage transparency (similar UX goal)
- [02-fork-sustainability.md](./02-fork-sustainability.md) - Evaluating external ideas

## The Bottom Line

**LLM excels at:** Research, exploration, conversation, comprehensive logging

**Cliffy excels at:** One-off tasks, automation, speed, zero persistence

**Best ideas to borrow:**
1. Model aliases (essential UX)
2. Model discovery (reduce friction)
3. Code extraction (automation focused)

**Key insight:** LLM's features work because it's designed for repeated use and exploration. Adapt, don't copy - keep Cliffy fast and focused.

Take the UX patterns that reduce friction (aliases, discovery, extraction) while maintaining zero-persistence philosophy.

ᕕ( ᐛ )ᕗ  Learn from LLM, stay true to Cliffy
