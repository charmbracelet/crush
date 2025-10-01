# Cliffy Next Steps

ᕕ( ᐛ )ᕗ  Strategic planning for Cliffy's future

## What's Here

This folder contains strategic planning documents covering key areas of Cliffy's development and evolution. Read them in any order based on what you're working on.

### [01 - Crush Branding Audit](./01-crush-branding-audit.md)

**Focus:** Completing Cliffy's identity while honoring Crush

**Key topics:**
- Systematic audit of remaining "Crush" references
- User-facing vs internal branding considerations
- Git history and attribution philosophy
- Config compatibility decisions

**Read this when:** Working on user-facing features, documentation, or external integrations

**TL;DR:** We love Crush and openly credit it, but Cliffy needs its own clear identity for users.

### [02 - Fork Sustainability](./02-fork-sustainability.md)

**Focus:** Staying current with innovations from Crush and other projects

**Key topics:**
- Dependency strategies (full Crush, catwalk-only, or structured sync)
- Tracking improvements from OpenCode, Codex, Gemini
- Component classification (sync vs diverge vs Cliffy-only)
- Concrete sync process and scripts

**Read this when:** Considering architecture changes, dependency updates, or integration strategies

**TL;DR:** Recommended approach is structured sync - keep forked code but maintain monthly sync process for key components.

### [03 - Personality & Branding](./03-personality-branding.md)

**Focus:** Cliffy's "ballboy" character and how to express it

**Key topics:**
- ASCII art library (ᕕ( ᐛ )ᕗ and tennis ball)
- Voice and tone guidelines
- Feature ideas that enhance personality without adding latency
- Where personality lives (help text, banners, completions)
- Testing and measuring personality effectiveness

**Read this when:** Working on CLI interface, output formatting, or user experience

**TL;DR:** Quick, focused, enthusiastic. Like a ballboy at the US Open - exactly where you need them, fast and efficient, not chatty.

### [04 - Performance & Scaling](./04-performance-scaling.md)

**Focus:** Making Cliffy faster and handling scale gracefully

**Key topics:**
- Quick wins (lazy loading, parallel execution, prompt optimization)
- Big ideas (batch mode, rate limiting, smart caching)
- Trade-offs between speed, reliability, and maintainability
- Concrete benchmarks and success criteria
- Implementation phases and priorities

**Read this when:** Working on performance improvements, scaling features, or optimization work

**TL;DR:** Already faster than Crush due to architecture. Can be much faster with parallel tools, batch execution, and lazy loading.

### [05 - LLM Inspiration](./05-llm-inspiration.md)

**Focus:** Learning from Simon Willison's LLM project to improve Cliffy

**Key topics:**
- Model aliases for easy shortcuts (4o → gpt-4o)
- Model discovery and listing commands
- Code extraction for automation
- Opt-in logging vs zero persistence
- What to adopt, adapt, and skip

**Read this when:** Planning UX improvements, model management features, or automation enhancements

**TL;DR:** LLM excels at exploration and logging. Borrow UX patterns (aliases, discovery) while keeping Cliffy's zero-persistence, one-off task focus.

## How to Use These Docs

### When Starting New Work

1. **Check relevant doc** for existing thinking on the topic
2. **Update doc** if you discover new information or make decisions
3. **Reference doc** in PRs/commits when implementing planned features

### When Making Strategic Decisions

1. **Read all docs** to understand full context
2. **Consider impacts** across branding, sustainability, personality, performance, and external ideas
3. **Document decision** in relevant doc for future reference

### When Onboarding

1. **Start with 03-personality-branding** to understand Cliffy's character
2. **Read 01-crush-branding-audit** to understand our relationship with Crush
3. **Read 05-llm-inspiration** to see how we evaluate external ideas
4. **Skim 02 and 04** to understand technical strategy

## Document Status

| Doc | Status | Last Updated | Next Review |
|-----|--------|--------------|-------------|
| 01 - Branding Audit | Draft | 2025-10-01 | Before 1.0 |
| 02 - Fork Sustainability | Draft | 2025-10-01 | After sync process tested |
| 03 - Personality | Draft | 2025-10-01 | After first-time UX improved |
| 04 - Performance | Draft | 2025-10-01 | After Phase 1 quick wins |
| 05 - LLM Inspiration | Draft | 2025-10-01 | After Phase 1 features tested |

## Contributing

These docs are living strategy documents. Update them as:
- Decisions are made
- Features are implemented
- Problems are discovered
- Ideas emerge

Keep them:
- **Concise** - Link to external docs for details
- **Actionable** - Include concrete next steps
- **Current** - Update when reality diverges from plan
- **Honest** - Document trade-offs and unknowns

## Quick Reference

### Core Principles (from 03-personality-branding)

1. Speed over words - Show, don't tell
2. ASCII over emoji - Terminal compatibility
3. Concise over cute - Ballboy efficiency
4. Helpful over chatty - Info when needed, quiet otherwise
5. Professional with personality - Serious about tasks, lighthearted in spirit

### Relationship with Crush (from 01-branding-audit)

**Cliffy adores Crush.** We're not competing - we're a specialized variant. Like a sprint runner vs marathon runner. Built on Crush's excellent foundation, optimized for fast one-off tasks.

### Dependency Strategy (from 02-fork-sustainability)

**Structured sync is recommended.** Keep forked code, maintain monthly sync process for core components (agent, tools, LSP), diverge where Cliffy's needs differ (config, runner, output).

### Performance Target (from 04-performance-scaling)

**Cold start <100ms, first token <500ms.** Already 1.1-1.5x faster than Crush. Can be 2-3x faster with lazy loading, parallel tools, and optimized prompts.

## Questions or Ideas?

Update the relevant doc with your thoughts! These are working documents designed to capture our collective thinking about Cliffy's future.

ᕕ( ᐛ )ᕗ  Planning the path forward
