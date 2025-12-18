# Product Requirements Document: Karigor White-Label Rebrand

**Version:** 1.0
**Status:** Draft
**Owner:** Product Team
**Last Updated:** 2025-12-18

---

## Executive Summary

This PRD outlines the requirements for creating **Karigor**, a white-labeled, simplified fork of the Crush AI coding assistant. Karigor will provide a streamlined, single-provider experience with localized branding and ASCII-only interface elements, making it more accessible and focused for specific markets and use cases.

The primary goal is to transform the multi-provider Crush application into a branded, opinionated product that removes configuration complexity while maintaining full feature parity with the underlying technology.

---

## Problem Statement

### Current State

Crush is a powerful, flexible AI coding assistant that supports multiple LLM providers (Anthropic, OpenAI, Gemini, OpenRouter, etc.). While this flexibility is valuable for power users, it creates several challenges:

1. **Overwhelming Choice:** Users face decision fatigue when presented with 10+ provider options
2. **Complex Setup:** Multiple configuration files, provider keys, and model selection increases cognitive load
3. **Generic Branding:** "Crush" branding doesn't resonate with all markets or use cases
4. **Special Characters:** Emoji and unicode characters may not render correctly on all terminals

### User Pain Points

- **"I just want it to work"** - Users don't want to research which provider to choose
- **"Too many API keys"** - Managing multiple provider credentials is cumbersome
- **"Doesn't feel like mine"** - Generic branding doesn't align with local or organizational identity
- **"Characters look broken"** - Emoji rendering issues on certain terminal emulators

---

## Goals and Non-Goals

### Goals

1. **Complete Rebranding:** Replace all "Crush" references with "Karigor" in user-facing text
2. **Simplified Provider Experience:** Offer single provider ("Karigor Chintok") with one API key
3. **ASCII-Only Interface:** Remove all emoji and unicode special characters for universal compatibility
4. **Streamlined Configuration:** Use `.karigor.json` naming convention throughout
5. **Maintain Schema Compatibility:** Display Karigor branding while remaining compatible with upstream schema

### Non-Goals

1. Modifying core agent, tool, or LSP functionality
2. Changing internal code structure or variable names
3. Creating a completely independent codebase (maintain ability to merge upstream changes)
4. Supporting multiple providers or model switching
5. Translating UI text to other languages (ASCII only, English remains)

---

## User Stories

### Primary User: Developer Seeking Simplicity

**Story 1: Quick Setup**
> As a developer new to AI coding assistants,
> I want to set up Karigor with a single API key,
> So that I can start coding within minutes without researching providers.

**Acceptance Criteria:**
- First run prompts only for "Karigor Chintok API key"
- No provider selection screen or model picker
- Configuration automatically saved to `~/.config/karigor/karigor.json`
- User can immediately start chatting after entering key

**Story 2: Branded Experience**
> As a user in a specific market or organization,
> I want to see consistent "Karigor" branding throughout the application,
> So that it feels like a purpose-built tool for my context.

**Acceptance Criteria:**
- All UI text shows "Karigor" instead of "Crush"
- Git commit attribution references "Karigor"
- Help text and error messages use Karigor branding
- Configuration files use `.karigor.json` extension
- Data directories use `.karigor/` naming

**Story 3: Universal Terminal Compatibility**
> As a developer using various terminal emulators,
> I want all UI elements to render correctly,
> So that I don't see broken emoji or unicode characters.

**Acceptance Criteria:**
- No emoji characters in any UI text
- No unicode box-drawing or special symbols
- ASCII-only status indicators and labels
- Git commit messages use plain text (no emoji)
- All visual elements render correctly on basic terminals

### Secondary User: Team Administrator

**Story 4: Controlled Backend**
> As a team administrator,
> I want users to access a specific LLM backend (ZAI GLM-4.6) without knowing the implementation details,
> So that I can standardize on one model and manage costs.

**Acceptance Criteria:**
- "Karigor Chintok" provider connects directly to ZAI GLM-4.6 API
- Users only see "Karigor Chintok" as the provider name
- Backend model selection hidden from users
- All requests route through ZAI API endpoint transparently
- Single API key configuration (ZAI token)

---

## Feature Specifications

### 1. Complete Name Replacement

**Requirement:** Replace all instances of "Crush" with "Karigor" in user-facing contexts.

**Scope:**
- CLI binary name: `crush` → `karigor`
- Command output and help text
- TUI window title and branding
- Error messages and logs (user-facing only)
- Git commit attribution messages
- Configuration file references
- Documentation and examples

**Exclusions:**
- Internal code variables and function names
- Go package imports (remains `github.com/charmbracelet/crush`)
- Comments in source code (unless user-visible)

### 2. Configuration File Naming

**Requirement:** Use `.karigor.json` naming convention for all configuration files.

**File Path Changes:**
- Project-local: `.crush.json` → `.karigor.json`
- Project-local: `crush.json` → `karigor.json`
- Global config: `~/.config/crush/crush.json` → `~/.config/karigor/karigor.json`
- Global state: `~/.local/share/crush/` → `~/.local/share/karigor/`
- Data directory: `.crush/` → `.karigor/`

**Configuration Search Order:**
1. `.karigor.json` (hidden, project-local)
2. `karigor.json` (visible, project-local)
3. `~/.config/karigor/karigor.json` (global user config)

### 3. Schema URL Display

**Requirement:** Display Karigor schema URL while maintaining compatibility with upstream.

**Behavior:**
- **Display:** `"$schema": "https://charm.land/karigor.json"`
- **Actual fetch:** `https://charm.land/crush.json` (unchanged)
- **Rationale:** Maintain compatibility with upstream schema updates while providing branded experience

**Implementation Note:** Custom schema resolution logic required to map displayed URL to actual fetch URL.

### 4. ASCII-Only Interface

**Requirement:** Remove all emoji and special unicode characters from user interface.

**Changes Required:**

| Current | New |
|---------|-----|
| `💘 Generated with Crush` | `Generated with Karigor` |
| `❤️`, `💖`, other emoji | Removed or replaced with ASCII |
| Unicode box drawing | Standard ASCII characters |
| Fancy bullets/symbols | Standard `-`, `*`, `>` |

**Scope:**
- Git commit messages
- Pull request descriptions
- Status bar indicators
- Help text and documentation
- Error messages
- TUI decorative elements

### 5. Single Provider Configuration

**Requirement:** Provide single hardcoded provider "Karigor Chintok" connecting to ZAI GLM-4.6.

**Provider Configuration:**
```json
{
  "providers": {
    "zai": {
      "id": "zai",
      "name": "Karigor Chintok",
      "type": "openai-compat",
      "base_url": "https://open.bigmodel.cn/api/paas/v4",
      "api_key": "$KARIGOR_API_KEY",
      "models": [
        {
          "id": "glm-4.6",
          "name": "Karigor Chintok",
          "context_window": 204800,
          "default_max_tokens": 131072
        }
      ]
    }
  }
}
```

**Technical Details:**
- Uses existing ZAI provider type from Catwalk (`catwalk.InferenceProviderZAI`)
- ZAI uses OpenAI-compatible API with special handling (`tool_stream: true`)
- Model ID: `glm-4.6` (displayed to users as "Karigor Chintok")
- API endpoint: `https://open.bigmodel.cn/api/paas/v4` (official ZAI API)
- No OpenRouter middleman needed - direct connection to ZAI

**User Experience:**
- First run: Prompt "Enter your Karigor Chintok API key:"
- Store key in `~/.config/karigor/karigor.json`
- No provider selection UI
- No model switching capability
- Single model shown in status bar: "Karigor Chintok"

**Provider Removal:**
- Remove all providers except ZAI from default list
- Disable Catwalk auto-update functionality
- Set `KARIGOR_DISABLE_PROVIDER_AUTO_UPDATE=1` by default
- Remove provider selection dialog
- Remove model picker dialog
- Hide provider configuration UI
- Rebrand ZAI provider as "Karigor Chintok" in all UI elements

### 6. Environment Variables

**Requirement:** Update environment variable names to match Karigor branding.

**Variable Mapping:**

| Original | New |
|----------|-----|
| `CRUSH_DISABLE_METRICS` | `KARIGOR_DISABLE_METRICS` |
| `CRUSH_DISABLE_PROVIDER_AUTO_UPDATE` | `KARIGOR_DISABLE_PROVIDER_AUTO_UPDATE` |
| `CRUSH_PROFILE` | `KARIGOR_PROFILE` |
| (new) | `KARIGOR_API_KEY` |

**Backward Compatibility:** Not required (clean break from Crush).

### 7. Attribution and Branding

**Requirement:** Update git commit and PR attribution to reference Karigor.

**Git Commit Format:**
```
feat: add user authentication

Generated with Karigor
Assisted-by: Karigor Chintok <karigor@example.com>
```

**Pull Request Format:**
```markdown
## Summary
- Implemented user authentication
- Added login/logout endpoints

Generated with Karigor
```

---

## User Experience

### First-Time User Flow

1. **Installation**
   - User installs `karigor` binary (via package manager or direct download)
   - No dependencies or prerequisites beyond terminal access

2. **First Launch**
   ```bash
   $ karigor
   Welcome to Karigor!

   To get started, enter your Karigor Chintok API key.
   You can obtain an API key from ZAI (https://open.bigmodel.cn)

   API Key: [user input]
   ```

3. **Configuration Save**
   - Key saved to `~/.config/karigor/karigor.json`
   - User sees success message: "Configuration saved. You're ready to code!"

4. **First Chat**
   - TUI opens with clean ASCII interface
   - Status bar shows: "Karigor Chintok | Ready"
   - User can immediately start typing prompts

### Ongoing Usage

- **No configuration needed:** Single provider, single model, just works
- **Status bar:** Clean ASCII showing "Karigor Chintok" as active model
- **Commands:** All slash commands work identically to Crush
- **Sessions:** Managed identically to Crush (create, switch, delete)
- **Tools:** All tools (bash, edit, fetch, etc.) work unchanged

### Help and Documentation

- `karigor --help` shows Karigor-branded help text
- `karigor logs` accesses `.karigor/logs/karigor.log`
- Error messages reference Karigor (e.g., "Karigor encountered an error")
- Documentation examples use `.karigor.json` files

---

## Success Metrics

### Functional Completeness

| Metric | Target | Measurement |
|--------|--------|-------------|
| User-facing "Crush" references removed | 100% | Manual audit of UI, CLI output, docs |
| Config files use `.karigor.json` | 100% | Automated tests for file path resolution |
| ASCII-only interface | 100% | Visual inspection + automated regex check |
| Single provider operational | 100% | E2E test with OpenRouter API |
| Schema compatibility maintained | 100% | Config validation against upstream schema |

### User Experience

| Metric | Target | Measurement |
|--------|--------|-------------|
| Setup completion time | < 2 minutes | User testing with 10+ participants |
| User confusion about providers | 0 mentions | User feedback during onboarding |
| Terminal compatibility issues | 0 reports | Testing across 5+ terminal emulators |
| Successful first chat | 95%+ | Telemetry (if enabled) |

### Quality Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Regression test pass rate | 100% | CI/CD pipeline |
| Feature parity with Crush | 100% | Comparison testing |
| Documentation accuracy | 100% | Peer review |

---

## Technical Considerations

### High-Level Architecture

1. **Branding Layer**
   - Constants file defining all branding text
   - Centralized configuration for name replacement
   - Build-time or runtime flag for fork identification

2. **Provider System**
   - Hardcoded provider configuration
   - Disable Catwalk auto-update by default
   - Remove provider selection UI components
   - Single model, no switching logic needed

3. **Schema Resolution**
   - Custom middleware to map display URL → fetch URL
   - Maintain JSON schema compatibility with upstream
   - Validation against both karigor and crush schemas

4. **Migration Support**
   - Optional tool to migrate `.crush/` → `.karigor/`
   - One-time config file conversion
   - Session history preservation

### Integration Points

1. **ZAI API**
   - Use ZAI as direct provider backend (no middleman)
   - Endpoint: `https://open.bigmodel.cn/api/paas/v4`
   - Model: GLM-4.6 (displayed as "Karigor Chintok")
   - Standard OpenAI-compatible API with ZAI-specific extensions (`tool_stream: true`)

2. **Fantasy Library**
   - No changes required (provider abstraction layer)
   - Existing ZAI provider support via `catwalk.InferenceProviderZAI`
   - OpenAI-compatible adapter handles API calls

3. **Configuration System**
   - Modify file path resolution in `internal/config/load.go`
   - Update constants in `internal/config/config.go`
   - Hardcode single ZAI provider in provider list

### Performance Considerations

- No performance impact expected (same underlying code)
- Single provider may reduce startup time slightly
- Fewer UI elements (no provider picker) = simpler rendering

### Security Considerations

1. **API Key Storage**
   - Store in `~/.config/karigor/karigor.json` with 0o600 permissions
   - Support environment variable: `KARIGOR_API_KEY`
   - Never log or display API key in plaintext

2. **ZAI API Communication**
   - All requests route directly to ZAI API endpoint
   - User's API key authenticates requests with ZAI
   - No data stored on Karigor servers (client-side only)
   - Supports ZAI-specific features (tool streaming)

---

## Open Questions

1. **Licensing**
   - Q: Does FSL-1.1-MIT license allow white-label fork?
   - A: Need legal review

2. **Upstream Sync**
   - Q: Will Karigor maintain sync with upstream Crush changes?
   - A: Need decision on merge strategy (fork vs. branch)

3. **Branding Assets**
   - Q: Do we need custom ASCII art for Karigor?
   - A: Define brand identity (logo, tagline, etc.)

4. **API Key Source**
   - Q: Should users get ZAI API keys directly from open.bigmodel.cn or through a Karigor-specific portal?
   - A: Clarify distribution model and key provisioning

5. **Telemetry**
   - Q: Should Karigor have separate telemetry or disable entirely?
   - A: Privacy policy decision needed

6. **Support**
   - Q: Where do users report issues? (Original GitHub repo or fork?)
   - A: Define support channels

7. **Updates**
   - Q: How will Karigor handle updates? (Separate release cycle or track Crush?)
   - A: Define update mechanism

---

## Acceptance Criteria

### Must Have

- [ ] All CLI output uses "Karigor" instead of "Crush"
- [ ] Configuration files use `.karigor.json` naming
- [ ] Data directories use `.karigor/` naming
- [ ] Single provider "Karigor Chintok" connects to Zai GLM 4.6
- [ ] No emoji or unicode special characters in UI
- [ ] Schema URL displays "karigor" but fetches from "crush.json"
- [ ] First-run prompts for single API key
- [ ] Git commit attribution references "Karigor"
- [ ] All environment variables use `KARIGOR_` prefix
- [ ] Provider selection UI removed
- [ ] Model switching UI removed

### Should Have

- [ ] Migration tool for existing Crush users
- [ ] Updated documentation with Karigor examples
- [ ] Custom ASCII art branding
- [ ] Automated tests for branding completeness
- [ ] User onboarding guide

### Nice to Have

- [ ] Karigor-specific splash screen on startup
- [ ] Custom color scheme (while maintaining ASCII-only)
- [ ] Branded error messages with helpful guidance
- [ ] Command aliases (e.g., `kr` for `karigor`)

---

## Appendix

### A. Example Configuration File

**.karigor.json** (project-local):
```json
{
  "$schema": "https://charm.land/karigor.json",
  "options": {
    "context_paths": ["KARIGOR.md"],
    "debug": false
  }
}
```

**~/.config/karigor/karigor.json** (global):
```json
{
  "$schema": "https://charm.land/karigor.json",
  "providers": {
    "zai": {
      "id": "zai",
      "name": "Karigor Chintok",
      "type": "openai-compat",
      "base_url": "https://open.bigmodel.cn/api/paas/v4",
      "api_key": "your-zai-api-key-here.xxxxxxxxxxxxx"
    }
  },
  "selected_models": {
    "large": {
      "provider": "zai",
      "model": "glm-4.6"
    }
  }
}
```

### B. Before/After Comparison

| Element | Before (Crush) | After (Karigor) |
|---------|----------------|-----------------|
| Binary name | `crush` | `karigor` |
| Config file | `.crush.json` | `.karigor.json` |
| Data dir | `.crush/` | `.karigor/` |
| Env var | `CRUSH_DISABLE_METRICS` | `KARIGOR_DISABLE_METRICS` |
| Git attribution | `Generated with Crush 💘` | `Generated with Karigor` |
| Provider count | 10+ providers | 1 provider |
| Setup complexity | Choose provider + model | Enter API key |
| UI style | Emoji + unicode | ASCII only |

### C. Related Documents

- Original Crush README: `/README.md`
- Crush Configuration Schema: `https://charm.land/crush.json`
- Architecture Documentation: `/CLAUDE.md`
- Fantasy Library Docs: `https://github.com/charmbracelet/fantasy`
- ZAI API Documentation: `https://docs.z.ai/guides/llm/glm-4.6`
- ZAI Developer Platform: `https://open.bigmodel.cn`

---

**Document Status:** Ready for Review
**Next Steps:** Technical feasibility assessment, legal review, design mockups for ASCII branding
