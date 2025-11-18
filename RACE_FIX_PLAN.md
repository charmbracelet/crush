# Race Condition Fix Plan

## 🎯 HIGH-IMPACT TODOs (Pareto Analysis)

### 1% → 51% IMPACT (Critical Path - Fixes Breaking Races)

#### 1. Background Shell Buffer Protection (12min)
**Problem**: Concurrent access to `bytes.Buffer` in `GetOutput()` causing read/write races
**Files**: `internal/shell/background.go:30-31, 179-186`
**Fix**: Add `sync.RWMutex` to protect stdout/stderr buffers
**Impact**: Eliminates most frequent races in shell operations

#### 2. Theme Manager Thread Safety (10min) 
**Problem**: Global `defaultManager` and theme building without synchronization
**Files**: `internal/tui/styles/theme.go:494-517, 132-137`
**Fix**: Add `sync.Once` for manager initialization and `sync.RWMutex` for theme building
**Impact**: Fixes UI component races across all TUI tests

#### 3. Permission Service Active Request Protection (8min)
**Problem**: Concurrent access to `activeRequest` field without synchronization
**Files**: `internal/permission/permission.go:67-69, 99-103, 158`
**Fix**: Protect `activeRequest` with atomic operations or mutex
**Impact**: Fixes permission service state corruption

#### 4. CSync Map Iterator Safety (15min)
**Problem**: Iterator functions creating race conditions during map copying
**Files**: `internal/csync/maps.go:99-123`
**Fix**: Ensure proper synchronization during map copying for iterators
**Impact**: Fixes foundational data structure races

### 4% → 64% IMPACT (Professional Polish)

#### 5. LSP Client Process Safety (10min)
**Problem**: Process closing races between different goroutines
**Files**: `internal/lsp/client.go` (need to examine)
**Fix**: Add proper synchronization for process lifecycle
**Impact**: Fixes LSP functionality races

#### 6. Enhanced Test Synchronization (20min)
**Problem**: Test harness itself causing some races due to rapid concurrent access
**Files**: Multiple test files
**Fix**: Add proper synchronization patterns in tests
**Impact**: Improves test reliability and coverage

### 20% → 80% IMPACT (Complete Package)

#### 7. Global State Management Review (25min)
**Problem**: Review all global variables for thread safety
**Files**: Across codebase
**Fix**: Add proper synchronization for all global state
**Impact**: Comprehensive race elimination

#### 8. Memory Barrier Documentation (15min)
**Problem**: Missing documentation about memory ordering and synchronization
**Files**: All fixed files
**Fix**: Add comprehensive documentation
**Impact**: Future-proof against race regressions

## 🔧 IMPLEMENTATION STRATEGY

### Phase 1: Critical Race Fixes (45min)
1. Fix Background Shell Buffer Protection
2. Fix Theme Manager Thread Safety  
3. Fix Permission Service Active Request Protection
4. Fix CSync Map Iterator Safety

### Phase 2: Component Polish (35min)
5. Fix LSP Client Process Safety
6. Enhanced Test Synchronization

### Phase 3: Comprehensive Finish (40min)
7. Global State Management Review
8. Memory Barrier Documentation

## 🚨 TESTING STRATEGY

After each fix:
- Run `go test -race ./...` to verify fix
- Run specific package tests to ensure functionality
- Run integration tests to verify no regressions

## 📋 VERIFICATION CHECKLIST

- [ ] All `go test -race` passes with no race warnings
- [ ] Individual package tests pass
- [ ] Integration tests work correctly
- [ ] Performance is not degraded
- [ ] Code review shows proper synchronization patterns

## 🎯 SUCCESS METRICS

- **Zero race conditions** in `go test -race ./...`
- **All tests passing** with race detector enabled
- **No performance degradation** (>5% slower)
- **Clean code review** for synchronization patterns