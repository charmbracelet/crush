# Model Selection & Reasoning in Crush-Headless

## Overview

One-off tasks have **different model requirements** than interactive sessions:
- **Quick tasks** → Fast, cheap models (Haiku, GPT-4o-mini)
- **Complex tasks** → Powerful models (Sonnet 4, o1, GPT-4)
- **Reasoning tasks** → Models with extended thinking

Headless needs flexible, **per-invocation** model selection.

## Current Crush Model System

### Model Types in Config

```json
{
  "models": {
    "large": {
      "provider": "anthropic",
      "model": "claude-sonnet-4-20250514"
    },
    "small": {
      "provider": "anthropic",
      "model": "claude-3-5-haiku-20241022"
    }
  }
}
```

**Usage:**
- `large` → Main agent (coder)
- `small` → Title generation, quick tasks

### Reasoning Levels

In Crush TUI, you can set reasoning level per session:
- **Low:** Minimal extended thinking
- **Medium:** Balanced thinking
- **High:** Maximum thinking budget

This controls the model's `thinking_budget` or similar parameters.

## Headless Model Selection

### Design Principles

1. **Default from config** - Use config `large` model by default
2. **Override per task** - CLI flags for one-off selection
3. **Reasoning control** - Explicit thinking budget
4. **Provider flexibility** - Switch providers easily
5. **Cost awareness** - Show model/cost before execution

### CLI Flags

#### `--model` / `-m`

**Purpose:** Override model for this execution

**Examples:**

```bash
# Use specific model
crush-headless --model=claude-3-5-haiku-20241022 "quick task"

# Use model ID from config
crush-headless --model=small "simple question"

# Use different provider's model
crush-headless --model=gpt-4o "task"
```

**Behavior:**
- Accepts model ID (e.g., `claude-sonnet-4-20250514`)
- Accepts model type from config (e.g., `small`, `large`)
- Auto-detects provider based on config

#### `--provider` / `-p`

**Purpose:** Override provider (uses their default model)

**Examples:**

```bash
# Use OpenAI instead of Anthropic
crush-headless --provider=openai "task"

# Use specific provider
crush-headless --provider=gemini "task"
```

**Behavior:**
- Uses provider's default model from config
- Or combine with `--model`:
  ```bash
  crush-headless --provider=openai --model=gpt-4o-mini "task"
  ```

#### `--reasoning` / `-r`

**Purpose:** Control extended thinking behavior

**Options:** `none`, `low`, `medium`, `high`, `auto`

**Examples:**

```bash
# Disable extended thinking for speed
crush-headless --reasoning=none "simple task"

# Low thinking for quick tasks
crush-headless --reasoning=low "check syntax"

# Medium (balanced)
crush-headless --reasoning=medium "refactor code"

# High for complex problems
crush-headless --reasoning=high "debug race condition"

# Auto (model default)
crush-headless --reasoning=auto "task"
```

**Implementation:**

For Anthropic (extended thinking):
```go
switch reasoningLevel {
case "none":
    // No thinking blocks
    disableThinking = true
case "low":
    // thinking_budget: low or thinking: {type: "disabled"}
case "medium":
    // thinking_budget: medium (default)
case "high":
    // thinking_budget: high
case "auto":
    // Use model default
}
```

For OpenAI o1/o3 (reasoning models):
```go
switch reasoningLevel {
case "none":
    // Don't use o1, fall back to GPT-4
case "low":
    reasoningEffort = "low"
case "medium":
    reasoningEffort = "medium"
case "high":
    reasoningEffort = "high"
}
```

#### `--fast`

**Purpose:** Shortcut for fast, cheap model

**Example:**

```bash
# Equivalent to --model=small --reasoning=none
crush-headless --fast "simple question"
```

**Behavior:**
- Uses config `small` model
- Disables extended thinking
- Optimized for speed/cost

#### `--smart`

**Purpose:** Shortcut for most capable model

**Example:**

```bash
# Equivalent to --model=large --reasoning=high
crush-headless --smart "complex debugging task"
```

**Behavior:**
- Uses config `large` model
- Enables high reasoning
- Optimized for quality

### Model Selection Algorithm

```go
func selectModel(flags Flags, config Config) Model {
    // 1. Explicit model flag takes precedence
    if flags.Model != "" {
        if model := config.GetModelByID(flags.Model); model != nil {
            return model
        }
        if model := config.GetModelByType(flags.Model); model != nil {
            return model
        }
        return error("model not found")
    }

    // 2. Check shortcuts
    if flags.Fast {
        return config.GetModelByType("small")
    }
    if flags.Smart {
        return config.GetModelByType("large")
    }

    // 3. Provider flag (use provider's default)
    if flags.Provider != "" {
        provider := config.GetProvider(flags.Provider)
        return provider.DefaultModel()
    }

    // 4. Fall back to config default (large)
    return config.GetModelByType("large")
}
```

## Config Enhancements

### Model Aliases

```json
{
  "models": {
    "large": {
      "provider": "anthropic",
      "model": "claude-sonnet-4-20250514"
    },
    "small": {
      "provider": "anthropic",
      "model": "claude-3-5-haiku-20241022"
    },
    "fast": {
      "provider": "anthropic",
      "model": "claude-3-5-haiku-20241022"
    },
    "smart": {
      "provider": "anthropic",
      "model": "claude-sonnet-4-20250514"
    },
    "reasoning": {
      "provider": "openai",
      "model": "o1"
    }
  }
}
```

**Usage:**
```bash
crush-headless --model=reasoning "complex problem"
```

### Default Reasoning Levels

```json
{
  "models": {
    "large": {
      "provider": "anthropic",
      "model": "claude-sonnet-4-20250514",
      "default_reasoning": "medium"
    },
    "small": {
      "provider": "anthropic",
      "model": "claude-3-5-haiku-20241022",
      "default_reasoning": "none"
    }
  }
}
```

### Per-Provider Defaults

```json
{
  "providers": {
    "anthropic": {
      "default_model": "claude-sonnet-4-20250514",
      "default_reasoning": "medium"
    },
    "openai": {
      "default_model": "gpt-4o",
      "default_reasoning": "low"
    }
  }
}
```

## Reasoning Level Mapping

### Anthropic Claude (Extended Thinking)

| Level | Implementation | Use Case |
|-------|---------------|----------|
| `none` | `thinking: {type: "disabled"}` | Simple tasks, speed priority |
| `low` | `thinking_budget_tokens: 1000` | Quick analysis |
| `medium` | `thinking_budget_tokens: 5000` | Standard tasks (default) |
| `high` | `thinking_budget_tokens: 10000` | Complex debugging |
| `auto` | Model default | Let model decide |

### OpenAI o1/o3 (Reasoning Models)

| Level | Implementation | Use Case |
|-------|---------------|----------|
| `none` | Don't use o1, use GPT-4 | Not a reasoning task |
| `low` | `reasoning_effort: "low"` | Simple reasoning |
| `medium` | `reasoning_effort: "medium"` | Standard reasoning |
| `high` | `reasoning_effort: "high"` | Deep reasoning |
| `auto` | Model default | Let model decide |

### Gemini (Thinking Mode)

| Level | Implementation | Use Case |
|-------|---------------|----------|
| `none` | Standard mode | No thinking needed |
| `low` | `thinking_mode: "minimal"` | Light thinking |
| `medium` | `thinking_mode: "standard"` | Normal thinking |
| `high` | `thinking_mode: "extended"` | Deep thinking |
| `auto` | Model default | Let model decide |

## Cost Awareness

### Pre-Execution Cost Estimate

When `--show-cost` flag is set, show estimate before execution:

```bash
$ crush-headless --show-cost --model=claude-sonnet-4-20250514 "task"

Model: claude-sonnet-4-20250514
Reasoning: medium
Estimated cost: $0.01 - $0.05 (based on average task)

Continue? [Y/n]
```

### Cost-Based Auto-Selection

```bash
# Prefer cheap model, fall back to smart if needed
crush-headless --auto-select "task"
```

**Logic:**
1. Try with `small` model first
2. If model refuses (task too complex), retry with `large`
3. Track cost vs. quality trade-off

### Cost Limits

```bash
# Abort if cost exceeds limit
crush-headless --max-cost=0.10 "task"
```

**Behavior:**
- Estimate cost before starting
- Abort if estimate > limit
- Track actual cost, abort if exceeded

## Usage Patterns

### Quick Questions (Cheap & Fast)

```bash
crush-headless --fast "what's in main.go?"
# or
crush-headless --model=small --reasoning=none "what's in main.go?"
```

**Cost:** ~$0.0001-$0.001
**Time:** ~1-2 seconds

### Standard Tasks (Balanced)

```bash
crush-headless "refactor the auth module"
# Uses config default (large model, medium reasoning)
```

**Cost:** ~$0.01-$0.05
**Time:** ~5-10 seconds

### Complex Debugging (Quality > Speed)

```bash
crush-headless --smart "debug this race condition"
# or
crush-headless --reasoning=high "debug this race condition"
```

**Cost:** ~$0.05-$0.20
**Time:** ~10-30 seconds

### Reasoning-Heavy Tasks

```bash
crush-headless --model=o1 --reasoning=high "design the architecture"
# or with alias
crush-headless --model=reasoning "design the architecture"
```

**Cost:** ~$0.10-$0.50
**Time:** ~20-60 seconds

### Multi-Provider Fallback

```bash
# Try cheap provider first
crush-headless --provider=groq "task" || \
  crush-headless --provider=anthropic "task"
```

### Cost-Optimized Batch

```bash
# Process many files with cheap model
for file in src/**/*.go; do
  crush-headless --fast "lint $file" >> results.txt
done
```

## Environment Variables

### Default Model

```bash
export CRUSH_HEADLESS_MODEL=claude-3-5-haiku-20241022
crush-headless "task"  # Uses Haiku
```

### Default Reasoning

```bash
export CRUSH_HEADLESS_REASONING=low
crush-headless "task"  # Uses low reasoning
```

### Default Provider

```bash
export CRUSH_HEADLESS_PROVIDER=openai
crush-headless "task"  # Uses OpenAI
```

## Interactive Model Selection

### Prompt Before Execution

```bash
crush-headless --interactive "complex task"
```

**Output:**
```
Select model:
  1. claude-3-5-haiku (fast, $0.001)
  2. claude-sonnet-4 (balanced, $0.01) [default]
  3. gpt-4o (powerful, $0.02)
  4. o1 (reasoning, $0.05)

Choice [2]: 3

Select reasoning level:
  1. none (fastest)
  2. low
  3. medium [default]
  4. high

Choice [3]: 4

Running with gpt-4o (reasoning: high)...
```

### Config-Based Prompts

```json
{
  "headless": {
    "prompt_for_model": true,
    "show_cost_estimate": true,
    "confirm_expensive": true,
    "expensive_threshold": 0.10
  }
}
```

## Model Selection in JSON Output

When `--output-format=json`, include model info:

```json
{
  "content": "...",
  "model": {
    "id": "claude-sonnet-4-20250514",
    "provider": "anthropic",
    "reasoning_level": "medium"
  },
  "tokens_used": {
    "input_tokens": 1234,
    "output_tokens": 567,
    "thinking_tokens": 2000
  },
  "cost": 0.0234
}
```

## Implementation

### Model Selection in Runner

```go
type RunnerOptions struct {
    Model          string
    Provider       string
    Reasoning      string
    Fast           bool
    Smart          bool
    ShowCost       bool
    MaxCost        float64
    Interactive    bool
}

func (r *Runner) selectModel(opts RunnerOptions) (*config.Model, error) {
    // Apply selection algorithm
    model := selectModel(opts, r.config)

    // Apply reasoning level
    reasoning := selectReasoning(opts.Reasoning, model)

    // Cost check
    if opts.ShowCost || opts.MaxCost > 0 {
        estimate := estimateCost(model, reasoning)
        if opts.ShowCost {
            fmt.Fprintf(os.Stderr, "Estimated cost: $%.4f\n", estimate)
        }
        if opts.MaxCost > 0 && estimate > opts.MaxCost {
            return nil, fmt.Errorf("estimated cost $%.4f exceeds limit $%.4f",
                estimate, opts.MaxCost)
        }
    }

    return model, nil
}
```

### Provider Options

```go
func (r *Runner) createProvider(model *config.Model, reasoning string) (provider.Provider, error) {
    opts := []provider.ProviderClientOption{
        provider.WithModel(model.ID),
        provider.WithSystemMessage(getHeadlessPrompt()),
    }

    // Apply reasoning settings
    switch model.Provider {
    case "anthropic":
        opts = append(opts, applyAnthropicReasoning(reasoning))
    case "openai":
        opts = append(opts, applyOpenAIReasoning(reasoning))
    }

    return provider.NewProvider(model.ProviderConfig, opts...)
}

func applyAnthropicReasoning(level string) provider.ProviderClientOption {
    return func(o *provider.ProviderClientOptions) {
        switch level {
        case "none":
            o.ExtraBody["thinking"] = map[string]any{"type": "disabled"}
        case "low":
            o.ExtraBody["thinking_budget_tokens"] = 1000
        case "medium":
            o.ExtraBody["thinking_budget_tokens"] = 5000
        case "high":
            o.ExtraBody["thinking_budget_tokens"] = 10000
        }
    }
}
```

## Future Enhancements

### Model Auto-Selection

Learn from past tasks to auto-select:

```bash
# Analyzes task complexity and picks model
crush-headless --auto "complex task"
```

### Cost Budgets

Track spending across runs:

```bash
# Stop when daily budget exceeded
crush-headless --daily-budget=1.00 "task"
```

### Model Chaining

Start with cheap, escalate if needed:

```bash
# Try Haiku, fallback to Sonnet if needed
crush-headless --cascade=small,large "task"
```

### Performance History

Track which models work best for which tasks:

```bash
# Uses historical data to pick model
crush-headless --learned "similar task to yesterday"
```

## Summary

**Key Additions for Headless:**

1. **`--model`** - Override model per task
2. **`--provider`** - Override provider per task
3. **`--reasoning`** - Control thinking level (none/low/medium/high)
4. **`--fast`** - Shortcut for cheap + fast
5. **`--smart`** - Shortcut for quality + thinking
6. **`--show-cost`** - Display cost estimate
7. **`--max-cost`** - Abort if too expensive
8. **`--interactive`** - Prompt for model selection

**Default Behavior:**
- Use config `large` model
- Use model's default reasoning level
- No cost prompts (non-interactive)

**Override Hierarchy:**
1. Explicit `--model` flag
2. Shortcuts (`--fast`, `--smart`)
3. `--provider` flag
4. Config default (`large`)

This gives users **complete control** over model selection for one-off tasks while maintaining sensible defaults.
