# Cliffy Visual System - Inventory & Assets

> Complete inventory of visual system components, symbols, and documentation

**Created**: 2025-10-20
**Status**: Design Complete, Implementation Pending
**Version**: 1.0-draft

---

## Executive Summary

This document inventories all assets for the Cliffy visual task processing methodology. The system provides a distinctive, Unicode-based visual language for displaying parallel task execution with the tennis/volley metaphor.

**Total Symbol Count**: 60+ Unicode symbols
**Documentation Pages**: 4 documents (~150KB)
**Implementation Phases**: 6 phases (3 core + 3 optional)
**Estimated Implementation Time**: 2-4 weeks

---

## Documentation Assets

### 1. Design Specification
**File**: `docs/cliffy-visual-system.md`
**Size**: ~800 lines
**Purpose**: Complete design specification and symbol definitions

**Contents**:
- Design philosophy and metaphors
- Complete symbol library (60+ symbols)
- Visual layout patterns
- Data flow diagrams
- Usage guidelines
- Color recommendations (future)
- Accessibility notes

**Audience**: Designers, developers, architects

### 2. Visual Mockups
**File**: `docs/cliffy-visual-mockups.md`
**Size**: ~600 lines
**Purpose**: Concrete examples of visual output across scenarios

**Contents**:
- Basic task execution examples
- Parallel volley scenarios
- Error and retry scenarios
- Advanced visualizations
- Debug and diagnostic views
- Performance metrics views
- Timeline views
- Real-world workflow example

**Audience**: Designers, users, QA testers

### 3. Implementation Plan
**File**: `docs/cliffy-visual-implementation-plan.md`
**Size**: ~700 lines
**Purpose**: Step-by-step technical implementation guide

**Contents**:
- Current state inventory
- 6 implementation phases
- File structure and modifications
- Testing strategy
- Rollout plan
- Risk mitigation
- Success metrics
- Unicode reference appendix

**Audience**: Developers, project managers

### 4. Quick Reference
**File**: `docs/cliffy-visual-quick-reference.md`
**Size**: ~250 lines
**Purpose**: One-page user guide to understanding output

**Contents**:
- Symbol legend
- Common patterns
- CLI flags
- Reading the output
- Troubleshooting
- At-a-glance examples

**Audience**: End users, support staff

### 5. Inventory (This Document)
**File**: `docs/cliffy-visual-inventory.md`
**Purpose**: Complete asset inventory and cross-reference

**Audience**: Project maintainers, documentation writers

---

## Symbol Inventory

### Task States (8 symbols)

| Symbol | Unicode | Name | Status | Phase |
|--------|---------|------|--------|-------|
| ○ | U+25CB | Queued | Existing | - |
| ◔ | U+25D4 | Initializing (25%) | New | 2 |
| ◑ | U+25D1 | Processing (50%) | New | 2 |
| ◕ | U+25D5 | Finalizing (75%) | New | 2 |
| ● | U+25CF | Complete (100%) | Existing | - |
| ◌ | U+25CC | Failed | New | 2 |
| ⦿ | U+29BF | Canceled | New | 2 |
| ◉ | U+25C9 | Cached | New | 6 |

**Current**: 2/8 (25%)
**New**: 6 symbols

### Tool States (7 symbols)

| Symbol | Unicode | Name | Status | Phase |
|--------|---------|------|--------|-------|
| ▤ | U+25A4 | Starting | New | 3 |
| ▥ | U+25A5 | Running | New | 3 |
| ▣ | U+25A3 | Success | Existing | - |
| ▦ | U+25A6 | Complex/Nested | New | 3 |
| ▩ | U+25A9 | Failed/Blocked | New | 3 |
| ☒ | U+2612 | Error | Existing | - |
| ▫ | U+25AB | Pending/Queued | New | 3 |

**Current**: 2/7 (29%)
**New**: 5 symbols

### Worker States (4 symbols)

| Symbol | Unicode | Name | Status | Phase |
|--------|---------|------|--------|-------|
| ⬡ | U+2B21 | Idle | New | 4 |
| ⬢ | U+2B22 | Active | New | 4 |
| ⬣ | U+2B23 | Overloaded | New | 4 |
| ⎔ | U+2394 | Suspended | New | 4 |

**Current**: 0/4 (0%)
**New**: 4 symbols

### Flow Indicators (10 symbols)

| Symbol | Unicode | Name | Status | Phase |
|--------|---------|------|--------|-------|
| → | U+2192 | Simple flow | New | 5 |
| ⇒ | U+21D2 | Strong flow | New | 5 |
| ⇨ | U+21E8 | Fast flow | New | 5 |
| ⟶ | U+27F6 | Long arrow | New | 6 |
| ↦ | U+21A6 | Maps to | New | 6 |
| ⇄ | U+21C4 | Bidirectional | New | 6 |
| ⟲ | U+27F2 | Retry loop | New | 5 |
| ⟳ | U+27F3 | Refresh | New | 6 |
| ↻ | U+21BB | Feedback | New | 6 |
| ⤴ | U+2934 | Return | New | 5 |

**Current**: 0/10 (0%)
**New**: 10 symbols

### Status Indicators (6 symbols)

| Symbol | Unicode | Name | Status | Phase |
|--------|---------|------|--------|-------|
| ⚠ | U+26A0 | Warning | New | 5 |
| ✗ | U+2717 | Error | Existing | - |
| ⊗ | U+2297 | Blocked | New | 5 |
| ⊘ | U+2298 | Prohibited | New | 6 |
| ⚡ | U+26A1 | Rate limited | New | 5 |
| ⏸ | U+23F8 | Paused | New | 5 |

**Current**: 1/6 (17%)
**New**: 5 symbols

### Tree Structure (4 symbols)

| Symbol | Unicode | Name | Status | Phase |
|--------|---------|------|--------|-------|
| ╮ | U+256E | Branch down-left | Existing | - |
| ├ | U+251C | Mid connector | Existing | - |
| ╰ | U+2570 | Last connector | Existing | - |
| ─── | U+2500×3 | Horizontal line | Existing | - |

**Current**: 4/4 (100%)
**New**: 0 symbols

### Container/Structure (11 symbols)

| Symbol | Unicode | Name | Status | Phase |
|--------|---------|------|--------|-------|
| □ | U+25A1 | Blueprint/Container | New | 6 |
| ▢ | U+25A2 | Rounded container | New | 6 |
| █ | U+2588 | Solid unit | New | 6 |
| ▮ | U+25AE | Output marker | New | 6 |
| ▯ | U+25AF | Input marker | New | 6 |
| ▬ | U+25AC | Buffer | New | 6 |
| ▭ | U+25AD | Empty buffer | New | 6 |
| ║ | U+2551 | Double vertical | New | 4 |
| ═ | U+2550 | Double horizontal | New | 4 |
| ╬ | U+256C | Junction | New | 4 |
| ▪ | U+25AA | Small block | New | 6 |

**Current**: 0/11 (0%)
**New**: 11 symbols

### Machine/Processing Nodes (6 symbols)

| Symbol | Unicode | Name | Status | Phase |
|--------|---------|------|--------|-------|
| ╱ | U+2571 | Left diagonal | New | 6 |
| ╲ | U+2572 | Right diagonal | New | 6 |
| ╳ | U+2573 | Crossed | New | 6 |
| ┼ | U+253C | Cross junction | New | 6 |
| ◇ | U+25C7 | Diamond | New | 6 |
| ◆ | U+25C6 | Filled diamond | New | 6 |

**Current**: 0/6 (0%)
**New**: 6 symbols

### Branding (2 symbols)

| Symbol | Unicode | Name | Status | Phase |
|--------|---------|------|--------|-------|
| ◍ | U+25CD | Tennis racket head | Existing | - |
| ᕕ( ᐛ )ᕗ | Various | Cliffy character | Existing | - |

**Current**: 2/2 (100%)
**New**: 0 symbols

### Metrics/Delimiters (6 symbols)

| Symbol | Unicode | Name | Status | Phase |
|--------|---------|------|--------|-------|
| ⟨ | U+27E8 | Left angle bracket | New | 6 |
| ⟩ | U+27E9 | Right angle bracket | New | 6 |
| ⌈ | U+2308 | Left ceiling | New | 6 |
| ⌉ | U+2309 | Right ceiling | New | 6 |
| ⌊ | U+230A | Left floor | New | 6 |
| ⌋ | U+230B | Right floor | New | 6 |

**Current**: 0/6 (0%)
**New**: 6 symbols

---

## Symbol Summary by Phase

### Phase 1: Symbol Library (Foundational)
- **Count**: 60 symbols defined
- **Implementation**: Add constants to `ascii.go`
- **Time**: 2-4 hours
- **Status**: Ready to implement

### Phase 2: Core Processing States (Critical)
- **Count**: 6 task state symbols (◔◑◕◌⦿◉)
- **Implementation**: Update progress tracker, agent events
- **Time**: 4-6 hours
- **Status**: Ready to implement

### Phase 3: Tool States (Important)
- **Count**: 5 tool state symbols (▤▥▦▩▫)
- **Implementation**: Update tool execution tracking
- **Time**: 3-5 hours
- **Status**: Ready to implement

### Phase 4: Worker Visualization (Optional)
- **Count**: 4 worker symbols + structure (⬡⬢⬣⎔ + ║═╬)
- **Implementation**: New worker visualizer component
- **Time**: 6-8 hours
- **Status**: Designed, not started

### Phase 5: Error & Retry (Quality of Life)
- **Count**: 9 symbols (flow + status: →⇒⇨⟲⤴ + ⚠⊗⚡⏸)
- **Implementation**: Enhanced error display
- **Time**: 4-6 hours
- **Status**: Designed, not started

### Phase 6: Advanced Features (Future)
- **Count**: 30+ symbols (containers, machines, metrics)
- **Implementation**: Metrics, timeline, diagrams
- **Time**: 8-12 hours
- **Status**: Speculative, low priority

---

## Code Impact Analysis

### Files to Create (New)

| File | Lines | Phase | Purpose |
|------|-------|-------|---------|
| `internal/volley/worker_visualizer.go` | ~150 | 4 | Worker pool display |
| `internal/volley/metrics.go` | ~200 | 6 | Metrics calculation |
| `internal/volley/timeline.go` | ~180 | 6 | Timeline visualization |
| `internal/volley/flow_diagram.go` | ~220 | 6 | Data flow diagrams |
| `internal/llm/tools/execution.go` | ~100 | 3 | Tool state tracking |

**Total New Files**: 5
**Total New Lines**: ~850

### Files to Modify (Existing)

| File | Current | Added | Phase | Changes |
|------|---------|-------|-------|---------|
| `internal/llm/tools/ascii.go` | 69 | +40 | 1 | Symbol constants |
| `internal/volley/task.go` | 122 | +20 | 2 | Task states |
| `internal/volley/progress.go` | 512 | +100 | 2-5 | Rendering logic |
| `internal/llm/agent/agent.go` | 800+ | +20 | 2 | Progress events |
| `internal/volley/scheduler.go` | 400+ | +30 | 2 | Event handling |
| `cmd/cliffy/main.go` | 300+ | +10 | 4 | CLI flags |

**Total Modified Files**: 6
**Total Modified Lines**: ~220

### Overall Code Impact

- **New code**: ~850 lines
- **Modified code**: ~220 lines
- **Documentation**: ~2300 lines
- **Total project**: ~3370 lines added

---

## Testing Assets

### Unit Tests Required

```
internal/llm/tools/ascii_test.go          - Symbol constant tests
internal/volley/progress_test.go          - Rendering tests
internal/volley/task_test.go              - State transition tests
internal/volley/worker_visualizer_test.go - Worker view tests
```

**Estimated test code**: ~400 lines

### Integration Test Scenarios

1. Single task with progress states
2. Parallel execution (3 workers, 5 tasks)
3. Retry scenario (rate limit)
4. Tool failure scenario
5. Worker pool visualization
6. Error chain display

### Visual Regression Tests

Test terminals:
- iTerm2 (macOS)
- Terminal.app (macOS)
- GNOME Terminal (Linux)
- Windows Terminal (Windows)
- tmux/screen
- VS Code integrated terminal

---

## Dependencies

### Direct Dependencies (None)
The visual system uses only Unicode characters from the standard terminal character set. No external libraries required.

### Indirect Dependencies
- Go 1.25.0+ (for string/rune handling)
- Terminal with UTF-8 support (user requirement)
- Monospace font (user requirement)

### Optional Future Dependencies
- `github.com/charmbracelet/lipgloss` - For color support (Phase 7+)
- `golang.org/x/term` - Already used for TTY detection

---

## Feature Matrix

### Current Implementation (v0.1)

| Feature | Status | Symbols |
|---------|--------|---------|
| Task queued/complete | ✅ | ○ ● |
| Spinner animation | ✅ | ◴◵◶◷ |
| Tool success/fail | ✅ | ▣ ☒ |
| Tree structure | ✅ | ╮├╰─── |
| Tennis branding | ✅ | ◍ |

### Planned Implementation

| Feature | Priority | Phase | Symbols |
|---------|----------|-------|---------|
| Processing states | High | 2 | ◔◑◕ |
| Failed/canceled | High | 2 | ◌⦿ |
| Tool states | Medium | 3 | ▤▥▦▩ |
| Worker view | Low | 4 | ⬡⬢⬣ |
| Retry indicators | Medium | 5 | ⟲⤴⏸ |
| Error symbols | Medium | 5 | ⚠⊗⚡ |
| Flow arrows | Low | 5-6 | →⇒⇨ |
| Advanced viz | Low | 6 | All others |

---

## Rollout Timeline

### Week 1: Foundation
- [x] Design complete
- [x] Documentation written
- [ ] Phase 1: Add symbols (Day 1-2)
- [ ] Phase 2: Basic states (Day 3-5)

### Week 2: Enhancement
- [ ] Phase 3: Tool viz (Day 1-2)
- [ ] Phase 5: Retry/error (Day 3-4)
- [ ] Testing/bugs (Day 5)

### Week 3: Polish
- [ ] Phase 4: Worker view (optional)
- [ ] Documentation review
- [ ] User testing

### Week 4+: Advanced (as needed)
- [ ] Phase 6: Metrics/timeline
- [ ] Performance tuning
- [ ] Color support prep

---

## Success Criteria

### Must Have (v1.0)
- [x] Design specification complete
- [x] Implementation plan documented
- [x] Quick reference guide created
- [ ] Phase 1 implemented (symbols)
- [ ] Phase 2 implemented (states)
- [ ] All existing tests pass
- [ ] Works in major terminals

### Should Have (v1.1)
- [ ] Phase 3 implemented (tools)
- [ ] Phase 5 implemented (retry/error)
- [ ] Test coverage >80%
- [ ] User feedback incorporated

### Nice to Have (v1.2+)
- [ ] Phase 4 implemented (worker view)
- [ ] Phase 6 implemented (advanced)
- [ ] Color support
- [ ] Interactive mode

---

## Open Questions

### Design Decisions
- [x] Progressive states (◔◑◕) vs spinner (◴◵◶◷)?
  **Decision**: Use progressive states, keep spinner as fallback

- [x] Worker view default or opt-in?
  **Decision**: Opt-in via `--workers-view` flag

- [ ] Should we auto-collapse completed tasks?
  **Current**: Yes, but needs user testing

- [ ] Retry symbol placement - inline or separate line?
  **Current**: Separate line for clarity

### Technical Decisions
- [ ] Render throttling - how often to update?
  **Proposed**: Max 10 updates/sec/task

- [ ] Terminal width handling - how to adapt?
  **Proposed**: Auto-detect, truncate descriptions

- [ ] Screen reader accessibility - test coverage?
  **Pending**: Need accessibility audit

### User Experience
- [ ] Is the tennis metaphor clear to non-tennis players?
  **Status**: Needs user research

- [ ] Too many symbols or just right?
  **Status**: Will monitor user feedback

- [ ] Default verbose mode or compact?
  **Current**: Compact default

---

## Related Documentation

### Internal Docs
- `CLAUDE.md` - Project overview and conventions
- `README.md` - User-facing documentation
- `docs/architecture.md` - System architecture (if exists)

### External References
- [Unicode Character Table](https://unicode-table.com/)
- [Terminal Emulator Compatibility](https://en.wikipedia.org/wiki/Unicode_and_HTML)
- [Accessibility Guidelines](https://www.w3.org/WAI/WCAG21/quickref/)

---

## Maintenance Plan

### Symbol Additions
- New symbols require: Unicode verification, terminal testing, documentation update
- Process: Design → Test → Document → Implement → Review

### Deprecation Policy
- No symbols will be removed in v1.x
- Breaking changes only in v2.0+
- Deprecation warnings for 1 version before removal

### Documentation Updates
- Update all 4 docs when adding symbols
- Keep quick reference in sync
- Version documentation appropriately

---

## Appendix: Symbol Categories

### By Usage Frequency (Expected)

**High Frequency** (used in every run):
- ○●◴◵◶◷ (task states)
- ▣ (tool success)
- ╮├╰─── (tree)
- ◍ (branding)

**Medium Frequency** (used in typical runs):
- ◔◑◕ (processing states)
- ▥ (tool running)
- → (flow)

**Low Frequency** (special cases):
- ◌⦿ (failed/canceled)
- ⟲⤴⏸ (retry)
- ⚠⊗⚡ (errors)

**Rare Frequency** (advanced features):
- ⬡⬢⬣ (workers)
- ╱╲╳ (machines)
- ⟨⟩⌈⌉⌊⌋ (metrics)

### By Implementation Priority

**P0 - Critical** (Phases 1-2):
- ◔◑◕●○◌⦿ (task lifecycle)

**P1 - Important** (Phase 3):
- ▤▥▦▩ (tool states)

**P2 - Nice to Have** (Phases 4-5):
- ⬡⬢⬣ (workers)
- →⇒⇨⟲⤴ (flow)
- ⚠⊗⚡⏸ (status)

**P3 - Future** (Phase 6):
- All others

---

## Version History

**v1.0-draft** (2025-10-20)
- Initial design and documentation
- 60+ symbols defined
- 4 documentation files created
- 6 implementation phases planned
- Ready for implementation start

---

**Total Assets Delivered**:
- ✅ 4 comprehensive documentation files
- ✅ 60+ Unicode symbols defined
- ✅ 6 implementation phases planned
- ✅ Complete testing strategy
- ✅ Quick reference guide
- ✅ This inventory document

**Status**: 🎾 Ready to volley! Design phase complete.

**Next Action**: Begin Phase 1 implementation (add symbols to `ascii.go`)
