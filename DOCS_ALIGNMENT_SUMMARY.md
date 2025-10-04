# Documentation Alignment Summary

**Date:** 2025-10-04
**Task:** Align docs with implementation (Improvement #12 from IMPROVEMENT_REPORT.md)
**Status:** ✅ Complete

## Overview

The documentation in `docs/` was originally written for a planned "crush-headless" project during the design phase. Cliffy has since been implemented with some differences from the original design. This summary details all changes made to align documentation with the actual implementation.

## Files Updated

### 1. docs/architecture.md ✅ MAJOR UPDATE
**Status:** Completely updated to reflect current implementation

**Changes Made:**
- ✅ Replaced "Crush-Headless" branding with "Cliffy"
- ✅ Updated high-level flow diagram to show Volley + Runner dual-path
- ✅ Replaced non-existent components:
  - ❌ HeadlessRunner → ✅ CLI Entry Point + Runner + Volley
  - ❌ StreamingProcessor → ✅ Event processing in Scheduler/Agent
  - ❌ DirectToolExecutor → ✅ Agent system with tool execution
  - ❌ ThinkingFormatter → ✅ Progress tracker with inline thinking
  - ❌ OutputFormatter (as designed) → ✅ Actual output/formatter.go
- ✅ Added actual components:
  - Volley Scheduler (parallel task execution)
  - Agent System (LLM interaction)
  - Output Formatter (tool traces, JSON, NDJSON)
- ✅ Updated execution flow to show actual single-task vs volley paths
- ✅ Updated memory model with realistic numbers
- ✅ Added header noting document status and last update date

**Notes Added:**
- Diff mode declared but not fully implemented (returns placeholder)
- Runner is minimal (full streaming planned but not yet done)
- See ROADMAP.md for unimplemented features

### 2. docs/implementation-guide.md ✅ MARKED HISTORICAL
**Status:** Marked as historical design document

**Changes Made:**
- ✅ Added prominent warning header marking document as historical
- ✅ Explained this was the original "crush-headless" design
- ✅ Noted actual implementation evolved differently (Volley pattern)
- ✅ Added references to architecture.md and ROADMAP.md for current info
- ✅ Kept all original content for historical reference

**Purpose:** Preserved to show original design thinking and evolution of the project.

### 3. docs/README.md ✅ MAJOR UPDATE
**Status:** Completely rewritten as documentation index

**Changes Made:**
- ✅ Updated title from "Crush-Headless Documentation" to "Cliffy Documentation"
- ✅ Added status section explaining doc history (design → implementation)
- ✅ Listed what's currently implemented vs planned
- ✅ Updated all document descriptions with status indicators:
  - ⚠️ UPDATED (architecture.md)
  - ⭐ NEW (ROADMAP.md)
  - ⚠️ HISTORICAL (implementation-guide.md)
  - ⚠️ PARTIALLY OUTDATED (api-specification.md)
- ✅ Replaced "proposed" examples with actual working commands
- ✅ Updated performance comparison table with real vs planned numbers
- ✅ Replaced "Implementation Phases" with actual file structure
- ✅ Added "Key Improvements Over Crush" section
- ✅ Updated quick reference with working examples

### 4. docs/ROADMAP.md ⭐ NEW FILE
**Status:** Created to track unimplemented features

**Contents:**
- ✅ "Currently Implemented" checklist (17 items)
- ✅ High Priority features with status indicators:
  - ⚠️ Partially Implemented (3 items)
  - 📋 Not Implemented (4 items)
- ✅ Medium Priority DX improvements (4 items)
- ✅ Low Priority polish/optimization (3 items)
- ✅ Features from design docs that won't be implemented
- ✅ Recently completed features
- ✅ References to IMPROVEMENT_REPORT.md for each item
- ✅ Code locations for each feature
- ✅ Contribution guidelines

**Purpose:** Central place to track all missing features mentioned in docs or referenced in code.

## Files NOT Updated (Preserved As-Is)

### Intentionally Preserved

1. **docs/api-specification.md**
   - Marked as "⚠️ PARTIALLY OUTDATED" in README
   - Users directed to `cliffy --help` for accurate flags
   - JSON schemas still valid
   - Will be updated in future when features are implemented

2. **docs/performance-analysis.md**
   - Still relevant for benchmarking
   - Contains Crush vs Cliffy comparisons
   - No changes needed

3. **docs/model-selection.md**
   - Still accurate (describes `--fast`, `--smart`, `--preset`)
   - No changes needed

4. **docs/fork-strategy.md**
   - Still relevant for maintainers
   - No changes needed

5. **docs/crush-headless-overview.md**
   - Historical design document
   - Preserved for context
   - Not linked in README (to avoid confusion)

6. **docs/AGENT_BRIEFING.md**
   - Internal planning doc
   - Preserved for context

## Key Discrepancies Documented

### Features Mentioned in Docs but Not Implemented

**From architecture.md (original):**
1. ❌ HeadlessRunner component → Uses Volley + Runner instead
2. ❌ StreamingProcessor component → Event processing in Scheduler
3. ❌ DirectToolExecutor component → Tool execution in Agent
4. ❌ ThinkingFormatter as separate component → Inline in progress tracker
5. ❌ Full diff output mode → Placeholder only (ROADMAP item #2)

**From implementation-guide.md:**
1. ❌ Parallel read-only tool execution → Not implemented
2. ❌ Lazy LSP initialization → LSP clients created but not lazy
3. ❌ Headless-specific prompt → Uses Crush prompts with agent configs

**From api-specification.md:**
1. ❌ Full thinking JSON/text formatting → Basic implementation only
2. ❌ Diff output format → Returns placeholder
3. ❌ Some thinking-related features → Partially working

### Features Working Better Than Designed

1. ✅ **Smart Retry Logic** - Implemented with per-error backoff strategies (better than design)
2. ✅ **Jitter in Retry** - Added ±25% jitter to prevent thundering herd
3. ✅ **Adaptive Metrics** - Infrastructure for adaptive concurrency (not yet utilized)
4. ✅ **Tool Traces** - NDJSON emission for automation (not in original design)
5. ✅ **Preset System** - Full implementation (mentioned but not detailed in design)
6. ✅ **Max Concurrent Override** - `--max-concurrent` flag added

## Recommended Next Steps

### For Contributors

1. **Check ROADMAP.md first** - See what's planned vs implemented
2. **Read architecture.md** - Understand current system design
3. **Reference IMPROVEMENT_REPORT.md** - Detailed analysis of each gap
4. **Test before assuming** - Some features may work differently than docs describe

### For Maintainers

1. **Prioritize ROADMAP items** - Focus on high-priority gaps:
   - Wire CLI flags into single-task execution (#1)
   - Complete diff output mode (#2)
   - Enhanced stats display (#3)

2. **Update api-specification.md** - When features are implemented:
   - Remove crush-headless references
   - Mark implemented vs planned features
   - Update examples to match `cliffy --help`

3. **Consider removing** - Old design docs if they cause confusion:
   - crush-headless-overview.md (not linked, preserved for history)
   - AGENT_BRIEFING.md (internal planning doc)

4. **Keep up-to-date:**
   - Update ROADMAP.md as features are completed
   - Mark items ✅ in ROADMAP when implemented
   - Add new planned features to appropriate priority level

### Issues to Create (Recommended)

Based on ROADMAP.md high-priority items:

1. **Issue: Wire CLI flags into single-task execution**
   - Labels: enhancement, priority:high
   - Description: Make `--show-thinking`, `--thinking-format`, `--stats` work in single-task mode
   - Reference: IMPROVEMENT_REPORT.md #1

2. **Issue: Implement diff output mode**
   - Labels: feature, priority:high
   - Description: Complete `FormatDiffOutput()` to extract and format diffs from tool metadata
   - Reference: IMPROVEMENT_REPORT.md #2

3. **Issue: Enhanced stats display**
   - Labels: enhancement, priority:high
   - Description: Make `--stats` work independently of verbosity, don't hide failures in quiet mode
   - Reference: IMPROVEMENT_REPORT.md #3

4. **Issue: STDIN and task file support**
   - Labels: feature, priority:medium, DX
   - Description: Accept tasks from STDIN (`-`) and files (`--tasks-file`)
   - Reference: IMPROVEMENT_REPORT.md #5

5. **Issue: Consolidate single-task and volley execution**
   - Labels: refactor, priority:medium
   - Description: Unify streaming logic between Runner and Volley paths
   - Reference: IMPROVEMENT_REPORT.md #7

## Summary Statistics

**Documents Updated:** 4 files
- architecture.md (major update)
- implementation-guide.md (marked historical)
- docs/README.md (major rewrite)
- ROADMAP.md (new file)

**Documents Preserved:** 6 files
- api-specification.md (marked as partially outdated)
- performance-analysis.md
- model-selection.md
- fork-strategy.md
- crush-headless-overview.md (historical)
- AGENT_BRIEFING.md (internal)

**Features Documented:**
- ✅ Implemented: 17 features
- ⚠️ Partially Done: 3 features
- 📋 Planned: 11 features
- ❌ Won't Implement: 5 design components (replaced by different approach)

**References Added:**
- All ROADMAP items link to IMPROVEMENT_REPORT.md sections
- All ROADMAP items include code locations
- Cross-references between docs for easy navigation

## Conclusion

Documentation now accurately reflects:
1. ✅ What Cliffy is (not "crush-headless")
2. ✅ What's implemented vs planned
3. ✅ How the actual architecture differs from original design
4. ✅ Where to find accurate information (ROADMAP, architecture.md)
5. ✅ Historical context preserved for reference

Contributors will no longer chase "ghost features" - they can clearly see what exists, what's planned, and what was designed but evolved differently.

---

**Completed By:** Claude (claude-sonnet-4-5-20250929)
**Task From:** IMPROVEMENT_REPORT.md #12 - Align Docs With Implementation
