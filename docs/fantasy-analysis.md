# Fantasy Analysis: Relevance to Cliffy

**Date:** 2025-10-23
**Fantasy Version:** Preview (main branch @ 38c35392)
**Analysis By:** Claude Code

## What is Fantasy?

**Fantasy** is Charm Bracelet's new standalone Go library for building AI agents. It's described as:
> "Build AI agents with Go. Multi-provider, multi-model, one API."

### Key Characteristics

1. **Standalone Library** - Not an application, but a reusable package (`charm.land/fantasy`)
2. **Multi-Provider** - Single API supporting multiple LLM providers
3. **Tool-Based Agents** - Built around the concept of agent tools
4. **Native Compilation** - Compiles to native machine code
5. **Preview Status** - Currently experimental, expect API changes

### Supported Providers

- **Anthropic** (via `charmbracelet/anthropic-sdk-go`)
- **OpenAI** (via `openai/openai-go/v2`)
- **Google** (via `charmbracelet/go-genai`)
- **Azure**
- **Bedrock** (AWS)
- **OpenRouter**
- **OpenAI-Compatible** (generic fallback for other providers)

### Architecture

```go
Provider → LanguageModel → Agent + Tools → Generate
```

- **Provider**: Factory for creating models (e.g., `openrouter.New()`)
- **LanguageModel**: Specific model from provider (e.g., "moonshotai/kimi-k2")
- **Agent**: Orchestrator with tools
- **Tools**: Custom functions the agent can call

---

## Relationship to Crush

From `CRUSH.md`:
> "We built Fantasy to power [Crush](https://github.com/charmbracelet/crush), a hot coding agent for glamourously invincible development."

**Fantasy is an extraction/refactoring of Crush's core agent logic into a reusable library.**

This means:
- Crush will likely migrate to use Fantasy
- Fantasy represents the "clean" version of the agent architecture
- Fantasy is the modular, library-first approach

---

## Relevance to Cliffy: HIGH ✅

### Why Fantasy is Relevant

#### 1. **Architectural Alignment**
Cliffy's goals align perfectly with Fantasy's design:
- **Headless**: Fantasy is a library, not a TUI
- **Fast**: Native compilation, no unnecessary dependencies
- **Focused**: Single-purpose agent library
- **Modular**: Clean provider abstraction

#### 2. **Shared Codebase Benefits**
If both Cliffy and Crush use Fantasy:
- **Easier syncing**: Core agent logic is in a shared library
- **Better separation**: TUI code stays in Crush, headless stays in Cliffy
- **Upstream improvements**: Fantasy bug fixes benefit both projects
- **Reduced duplication**: One well-tested agent implementation

#### 3. **Current Overlap**
Cliffy currently has its own implementations of:
- `internal/llm/agent/` - Agent orchestration
- `internal/llm/provider/` - Provider abstractions
- `internal/llm/tools/` - Tool system

**These directly overlap with Fantasy's core functionality.**

#### 4. **Provider Parity**
Fantasy supports the same providers Cliffy needs:
- ✅ Anthropic
- ✅ OpenAI
- ✅ Bedrock
- ✅ Vertex AI (via Google provider)
- ✅ OpenRouter
- ✅ Azure

---

## Migration Path: Cliffy → Fantasy

### Option A: Full Migration (Recommended Long-term)

**Replace** `internal/llm/agent/` and `internal/llm/provider/` with Fantasy.

**Pros:**
- Shared codebase with Crush
- Upstream improvements automatically benefit Cliffy
- Less code to maintain
- Better tested (used by both Crush and Cliffy)
- Cleaner architecture

**Cons:**
- Requires significant refactoring
- Fantasy is still in preview (API changes expected)
- May need to contribute Cliffy-specific features back to Fantasy
- Need to adapt Cliffy's tool system to Fantasy's tool API

**Effort:** High (2-4 weeks of work)

### Option B: Hybrid Approach (Recommended Short-term)

**Use Fantasy for providers**, keep Cliffy's agent/tool system.

**Pros:**
- Smaller migration surface
- Provider code is the most complex to maintain
- Can migrate incrementally
- Keeps Cliffy's optimized agent logic

**Cons:**
- Still maintaining custom agent code
- Partial benefits only
- May need adapter layer

**Effort:** Medium (1-2 weeks)

### Option C: Wait and Watch

**Monitor Fantasy** development, migrate when stable.

**Pros:**
- Avoid API churn
- Fantasy may add features Cliffy needs
- Learn from Crush's migration experience

**Cons:**
- Continued code divergence
- Miss out on improvements
- Harder migration later (more divergence)

**Effort:** Low now, High later

---

## Specific Benefits for Cliffy

### 1. **Simplified Provider Management**
Fantasy handles:
- Provider initialization
- Model selection
- API key management
- Error handling
- Retry logic

Cliffy currently does this in `internal/llm/provider/*/`

### 2. **Better Tool Abstraction**
Fantasy's tool system:
```go
fantasy.NewAgentTool(
  "tool_name",
  "Description",
  toolFunc,
)
```

Cliffy's tool system in `internal/llm/tools/` could be adapted to this.

### 3. **Streaming Support**
Fantasy has built-in streaming:
```go
stream, err := agent.Stream(ctx, fantasy.AgentCall{Prompt: prompt})
for chunk := range stream {
    // Handle streaming response
}
```

This could improve Cliffy's real-time output.

### 4. **Provider Agnostic**
Switch providers without code changes:
```go
// From OpenRouter to Anthropic - just change provider init
provider, _ := anthropic.New(anthropic.WithAPIKey(key))
// Everything else stays the same
```

---

## Challenges & Considerations

### 1. **Preview Status**
Fantasy is marked as preview:
> "Fantasy is currently a preview. Expect API changes."

**Risk:** Breaking changes could require rework
**Mitigation:** Pin to specific version, contribute to stabilization

### 2. **Tool System Differences**
Cliffy's tools are more sophisticated:
- LSP integration
- MCP integration
- File operations with permissions
- Bash tool with persistent shell

**Question:** Can these be modeled in Fantasy's tool system?
**Likely:** Yes, but may need Fantasy enhancements

### 3. **Missing Features**
Fantasy README notes:
> Fantasy does not yet support things like:
> - Image models
> - Audio models
> - PDF uploads
> - Provider tools (e.g. web_search)

Cliffy doesn't need these currently, but may in future.

### 4. **Dependency Management**
Fantasy has its own dependencies:
- `charmbracelet/anthropic-sdk-go` (different from Cliffy's)
- `charmbracelet/go-genai` (for Google)
- `openai/openai-go/v2` (Cliffy uses v1)

**Impact:** May require dependency updates

---

## Recommended Action Plan

### Phase 1: Investigation (1-2 days)
1. ✅ Clone Fantasy and review code architecture
2. ✅ Understand Fantasy's agent and tool APIs
3. ⬜ Prototype a simple Cliffy command using Fantasy
4. ⬜ Identify integration points and challenges

### Phase 2: Proof of Concept (1 week)
1. ⬜ Create experimental branch
2. ⬜ Migrate one provider (OpenRouter) to Fantasy
3. ⬜ Adapt one tool to Fantasy's API
4. ⬜ Run benchmarks vs current implementation

### Phase 3: Decision Point
Based on PoC results:
- **If successful**: Plan full migration
- **If promising**: Continue hybrid approach
- **If problematic**: Stay with current architecture, revisit in 6 months

### Phase 4: Full Migration (if approved)
1. ⬜ Migrate all providers to Fantasy
2. ⬜ Adapt tool system
3. ⬜ Update agent orchestration
4. ⬜ Remove deprecated code
5. ⬜ Update tests
6. ⬜ Benchmark performance

---

## Questions to Answer

1. **Tool Compatibility**: Can Fantasy's tool system handle Cliffy's complex tools (LSP, MCP, Bash)?
2. **Performance**: Is Fantasy as fast as Cliffy's current implementation?
3. **Configuration**: Can Fantasy support Cliffy's config structure?
4. **Error Handling**: Does Fantasy's error system match Cliffy's needs?
5. **Streaming**: Does Fantasy's streaming work with Cliffy's output model?

---

## Conclusion

**Fantasy is HIGHLY RELEVANT to Cliffy.**

### Recommendation: **Hybrid Approach** (Short-term) → **Full Migration** (Long-term)

**Immediate Action (Next Sprint):**
1. Create PoC branch testing Fantasy integration
2. Prototype provider migration (OpenRouter first)
3. Evaluate tool compatibility
4. Share findings with team/users

**Long-term Vision:**
- Cliffy and Crush share Fantasy as core agent library
- Cliffy focuses on headless execution, optimization, volley mode
- Crush focuses on TUI, interactive features
- Both benefit from Fantasy improvements

**Risk Level:** Medium (Fantasy is preview, but Charm team is reliable)
**Benefit Level:** High (reduced maintenance, better architecture, shared improvements)
**Effort Level:** Medium-High (significant refactoring needed)

**Bottom Line:** Fantasy represents the future architecture for both Crush and Cliffy. Adopting it sooner rather than later will reduce long-term maintenance burden and keep Cliffy aligned with Charm's ecosystem.

---

## Next Steps

Would you like me to:
1. Create a PoC branch experimenting with Fantasy integration?
2. Analyze specific compatibility issues (tools, config, etc.)?
3. Create migration plan with detailed task breakdown?
4. Open discussion with Crush maintainers about Fantasy roadmap?
