# 🚀 Goleak Integration Guide for Permission System

## How Goleak Works

**Goleak** is a Go library that detects goroutine leaks by comparing goroutine counts before and after test execution.

### Core Mechanism:
1. **Baseline Capture**: `goleak.VerifyNone()` captures current goroutine count
2. **Test Execution**: Your test runs (potentially leaking goroutines)
3. **Verification**: Goleak checks if any goroutines were left running
4. **Failure Reporting**: Identifies leaked goroutines with stack traces

### Why It's Critical for Permission System:
- **Concurrent Operations**: Permission requests spawn goroutines for pubsub, channels
- **Resource Leaks**: Unclosed goroutines consume memory and file descriptors
- **Silent Failures**: Leaked goroutines may not cause immediate failures but degrade performance

---

## 📋 Implementing Goleak in Permission System

### 1. Add Goleak Dependency
```bash
go get go.uber.org/goleak
```

### 2. Test Structure Pattern
```go
func TestPermissionService_GoroutineLeak(t *testing.T) {
    defer goleak.VerifyNone(t)  // Verify no goroutine leaks at test end
    
    // Test code that might leak goroutines
    service := NewPermissionService("/tmp", false, []string{})
    // ... test operations ...
}
```

### 3. Integration with Existing Tests
Add to critical test functions:
- `TestPermissionService_SequentialProperties`
- `TestPermissionService_Stress`
- `TestRaceFreePermissionService`

---

## 🔧 Implementation Plan

### Phase 1: Basic Integration
1. Add goleak dependency
2. Add to existing permission tests
3. Fix any discovered leaks

### Phase 2: CI Integration
1. Add to all test suites
2. Configure CI to fail on goroutine leaks
3. Add goleak options for test environment

### Phase 3: Advanced Monitoring
1. Custom leak detection for specific patterns
2. Performance impact measurement
3. Integration with existing monitoring

---

## 🎯 Common Goroutine Leaks in Permission System

### 1. Pubsub Subscribers
```go
// BAD: Subscriber not closed
events := service.Subscribe(context.Background())
// Never close context or events channel

// GOOD: Proper cleanup
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
events := service.Subscribe(ctx)
```

### 2. Response Channels
```go
// BAD: Channel never read
respCh := make(chan enum.ToolCallState)
s.pendingRequests.Set(id, respCh)
// Nobody ever reads from respCh

// GOOD: Ensure reader exists
go func() {
    <-respCh  // Ensure channel is drained
}()
```

### 3. Background Workers
```go
// BAD: Goroutine never exits
go func() {
    for {
        // Work loop without exit condition
        time.Sleep(time.Second)
    }
}()

// GOOD: Context-aware shutdown
go func() {
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            // Do work
        case <-ctx.Done():
            return  // Clean exit
        }
    }
}()
```

---

## 🔍 Leak Detection Strategy

### 1. Start Simple
```go
func TestPermissionService_BasicGoleak(t *testing.T) {
    defer goleak.VerifyNone(t)
    
    service := NewPermissionService("/tmp", false, []string{})
    _ = service.Request(CreatePermissionRequest{
        SessionID: "test",
        ToolName:  "test-tool",
        Action:    "test-action",
        Path:      "/tmp",
    })
}
```

### 2. Gradual Expansion
Add to existing tests one by one:
1. PermissionService basic tests
2. Stress tests
3. Race condition tests
4. Integration tests

### 3. Custom Options for CI
```go
// CI-friendly configuration
defer goleak.VerifyNone(t, 
    goleak.IgnoreCurrent(),        // Ignore pre-existing leaks
    goleak.IgnoreTopFunction("runtime.main"), // Ignore main goroutine
)
```

---

## 📊 Integration Benefits

### 1. Automated Detection
- CI fails on goroutine leaks
- No manual inspection required
- Consistent across environments

### 2. Performance Preservation
- Prevents gradual memory consumption
- Maintains responsiveness under load
- Reduces production incident risk

### 3. Code Quality
- Forces proper resource cleanup
- Encourages context-aware design
- Improves concurrency patterns

---

## 🚨 Common Issues & Solutions

### 1. False Positives
- **Problem**: Test environment has pre-existing goroutines
- **Solution**: Use `goleak.IgnoreCurrent()` in CI

### 2. Timeout Issues
- **Problem**: Tests hang waiting for cleanup
- **Solution**: Implement proper context cancellation

### 3. Flaky Tests
- **Problem**: Intermittent leak detection
- **Solution**: Increase cleanup grace periods, fix race conditions

---

## 🎯 Implementation Priority

1. **HIGH**: Add to all permission service tests
2. **MEDIUM**: Add to pubsub/broker tests  
3. **LOW**: Add to integration test suites

This creates a comprehensive safety net for goroutine management in the permission system.