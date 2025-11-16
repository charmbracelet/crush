# Crush Priority Execution Plan - 2025-11-16_15_30-ARCHITECTURE_OVERHAUL

## 📊 PARETO PRIORITIZATION SUMMARY

| Impact Level | Tasks | Total Impact | Per Task Time |
|-------------|--------|-------------|----------------|
| **1% - 51% Impact** | 4 tasks | 51% | 15min each |
| **4% - 64% Impact** | 4 tasks | 13% | 30min each |
| **20% - 80% Impact** | 8 tasks | 16% | 60min each |
| **Remaining** | 11 tasks | 20% | Variable |

---

## 🎯 EXECUTION SEQUENCE (All 27 TODOs)

### **PHASE 1: CRITICAL FOUNDATION (1% Tasks - 51% Impact)**
| ID | Task | Time | Impact | Priority | File |
|----|------|------|--------|----------|-------|
| T01 | Centralize ToolCallState updates in OnStepFinish | 15min | HIGH | internal/agent/agent.go |
| T02 | Fix unused parameter 'nested' in renderer.go:878 | 15min | HIGH | internal/tui/components/chat/messages/renderer.go |
| T03 | Fix unused function 'isCancelledErr' in errors.go | 15min | HIGH | internal/agent/errors.go |
| T04 | Complete missing state transition logic | 15min | HIGH | internal/enum/tool_call_state.go |

### **PHASE 2: MAJOR ARCHITECTURE (4% Tasks - 64% Impact)**
| ID | Task | Time | Impact | Priority | File |
|----|------|------|--------|----------|-------|
| T05 | Implement ToolCallState.GetConfiguration() method | 30min | HIGH | internal/enum/tool_call_state.go |
| T06 | Update configureVisualAnimation to use GetConfiguration() | 30min | HIGH | internal/tui/components/chat/messages/tool.go |
| T07 | Add state transition validation to OnStepFinish | 30min | HIGH | internal/agent/agent.go |
| T08 | Update critical consumers to use GetConfiguration() | 30min | HIGH | Multiple files |

### **PHASE 3: COMPLETE REFACTOR (20% Tasks - 80% Impact)**
| ID | Task | Time | Impact | Priority | File |
|----|------|------|--------|----------|-------|
| T09 | Migrate all remaining consumers to GetConfiguration() | 60min | MEDIUM | Multiple files |
| T10 | Deprecate legacy ToolCallState methods (backward compat) | 30min | MEDIUM | internal/enum/tool_call_state.go |
| T11 | Add comprehensive test coverage for GetConfiguration() | 60min | MEDIUM | internal/enum/tool_call_state_test.go |
| T12 | Implement AnimationState configuration pattern | 60min | MEDIUM | internal/enum/animation_state.go |

### **PHASE 4: UX IMPROVEMENTS (Remaining 20%)**
| ID | Task | Time | Impact | Priority | File |
|----|------|------|--------|----------|-------|
| T13 | Fix "need to press ^c twice to cancel" issue | 30min | MEDIUM | internal/cmd/run.go |
| T14 | Implement proper stdout redirection handling | 30min | MEDIUM | internal/cmd/run.go |
| T15 | Handle images in view tool | 45min | MEDIUM | internal/agent/tools/view.go |
| T16 | Fix agentToolSessionID vs message.ToolCallID confusion | 45min | MEDIUM | internal/agent/tools/ + internal/agent/agentic_fetch_tool.go |
| T17 | Implement proper context handling for editors | 30min | MEDIUM | internal/tui/components/chat/editor/editor.go |
| T18 | Remove app instance from editor | 60min | LOW | internal/tui/components/chat/editor/editor.go |
| T19 | Update keymap concepts in editor keys | 45min | LOW | internal/tui/components/chat/editor/keys.go |
| T20 | Clean up magic numbers in TUI | 30min | LOW | internal/tui/tui.go |
| T21 | Fix "don't move to page if agent busy" logic | 30min | LOW | internal/tui/tui.go |
| T22 | Remove global config instance | 45min | LOW | internal/config/init.go |
| T23 | Make config values available from environment | 30min | LOW | internal/config/config.go |
| T24 | Remove agent config concept from app | 60min | LOW | internal/app/app.go |
| T25 | Make coordinator dynamic for multiple agents | 90min | LOW | internal/agent/coordinator.go |
| T26 | Update TODO with normal rendering process | 30min | LOW | internal/tui/components/chat/messages/renderer.go |
| T27 | Move layout.go to core | 15min | LOW | internal/tui/components/core/layout/layout.go |

---

## 🏗️ ARCHITECTURE IMPACT MATRIX

| Category | Current Tasks | 1% Tasks | 4% Tasks | 20% Tasks | Total |
|----------|---------------|------------|-----------|------------|-------|
| **ToolCallState Architecture** | T01, T04, T05, T09, T10, T11 | 4 | 2 | 2 | 8 |
| **Error Handling & Warnings** | T02, T03 | 2 | 0 | 0 | 2 |
| **Agent & Session Management** | T07, T16, T22, T24, T25 | 1 | 1 | 3 | 5 |
| **UI/UX Improvements** | T13, T14, T17, T18, T19, T20, T21, T26, T27 | 0 | 1 | 5 | 6 |
| **Animation System** | T06, T12 | 0 | 1 | 1 | 2 |
| **Configuration & Environment** | T23 | 0 | 0 | 1 | 1 |

---

## 📈 EXECUTION TIMELINE

| Phase | Duration | Start | End | Cumulative Impact |
|-------|----------|--------|------|------------------|
| **Phase 1** | 1 hour | T+0h | T+1h | **51%** |
| **Phase 2** | 2 hours | T+1h | T+3h | **64%** |
| **Phase 3** | 4 hours | T+3h | T+7h | **80%** |
| **Phase 4** | 6.5 hours | T+7h | T+13.5h | **100%** |

---

## 🎯 IMMEDIATE NEXT ACTIONS

**After this plan:**
1. Execute T01-T04 (Phase 1) - 1 hour
2. Run comprehensive tests
3. Commit Phase 1 completion
4. Report back with Phase 1 results

**Critical Success Metrics:**
- ✅ All unused parameter/function warnings eliminated
- ✅ ToolCallState updates centralized in OnStepFinish  
- ✅ State transition logic complete and validated
- ✅ Zero compile warnings

This plan ensures maximum impact with minimum initial investment, following the 80/20 principle rigorously.