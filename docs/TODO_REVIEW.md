# TODO/FIX Review Report

Generated: 2026-04-04  
Total items: 13

---

## HIGH Priority (Blocking/Functional Gaps)

### 1. Missing HybridBrain Execution
**File:** `internal/kernel/server/http_server.go:184`  
**Tag:** TODO  
**Code:**
```go
// TODO: 實際調用 HybridBrain.Execute()
ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
defer cancel()
result := h.executeTask(ctx, req)
```

**Issue:** The HTTP handler has a TODO indicating it should call `HybridBrain.Execute()` but currently uses `executeTask()` instead.  
**Suggestion:** Replace `h.executeTask()` with `HybridBrain.Execute()` or clarify the relationship between these methods.

---

### 2. Log Leakage During Config Load
**File:** `internal/cmd/root.go:162`  
**Tag:** FIXME  
**Code:**
```go
// FIXME: config.Load uses slog internally during provider resolution,
// but the file-based logger isn't set up until after config is loaded...
slog.SetDefault(slog.New(slog.DiscardHandler))
```

**Issue:** `config.Load` calls `slog` before the file-based logger is configured, causing early logs to leak to stderr.  
**Suggestion:** Remove `slog` calls from `config.Load` and return warnings/diagnostics instead of logging as a side effect.

---

## MEDIUM Priority (Should Fix)

### 3. Potential Config Mutation During Read
**File:** `internal/ui/dialog/models.go:487`  
**Tag:** FIXME  
**Code:**
```go
if len(validRecentItems) != len(recentItems) {
    // FIXME: Does this need to be here? Is it mutating the config during a read?
    if err := m.com.Workspace.SetConfigField(...); err != nil {
```

**Issue:** Config appears to be mutated during a read operation to filter invalid recent models.  
**Suggestion:** Move this logic to write time or investigate if the filtering can happen differently.

---

### 4. Unhandled Error in TUI Footer
**File:** `internal/ui/model/onboarding.go:19`  
**Tag:** TODO  
**Code:**
```go
// TODO: handle error so we show it in the tui footer
err := m.com.Workspace.MarkProjectInitialized()
```

**Issue:** Errors from `MarkProjectInitialized()` are only logged, not displayed to user.  
**Suggestion:** Return the error as a message to display in the TUI footer.

---

### 5. Environment Variable Support for Headers
**File:** `internal/config/config.go:179`  
**Tag:** TODO  
**Code:**
```go
// TODO: maybe make it possible to get the value from the env
Headers map[string]string `json:"headers,omitempty"`
```

**Issue:** MCP server headers don't support environment variable expansion.  
**Suggestion:** Implement `${ENV_VAR}` expansion in the Headers field similar to other config fields.

---

### 6. Static Agent Configuration
**File:** `internal/agent/coordinator.go:120`  
**Tag:** TODO  
**Code:**
```go
// TODO: make this dynamic when we support multiple agents
prompt, err := coderPrompt(prompt.WithWorkingDir(c.cfg.WorkingDir()))
```

**Issue:** Currently hardcoded to use `coderPrompt` for single agent support.  
**Suggestion:** Add agent type parameter to allow different prompt strategies.

---

### 7. Remove Agent Config Concept
**File:** `internal/app/app.go:122`  
**Tag:** TODO  
**Code:**
```go
// TODO: remove the concept of agent config, most likely.
if !cfg.IsConfigured() {
```

**Issue:** Agent configuration is treated as optional but may not be needed.  
**Suggestion:** Clarify if agent config is required or optional, then remove the conditional check.

---

### 8. Use tea.EnvMsg Instead of os.Getenv
**File:** `internal/ui/dialog/commands.go:474`  
**Tag:** TODO  
**Code:**
```go
// TODO: Use [tea.EnvMsg] to get environment variable instead of os.Getenv;
// because os.Getenv does IO is breaks the TEA paradigm
if os.Getenv("EDITOR") != "" {
```

**Issue:** `os.Getenv` performs I/O and breaks the TEA paradigm.  
**Suggestion:** Subscribe to `tea.EnvMsg` to receive environment variables through the message loop.

---

## LOW Priority (Minor/Technical Debt)

### 9. Manual Style Application
**File:** `internal/ui/chat/assistant.go:110`  
**Tag:** XXX  
**Code:**
```go
// XXX: Here, we're manually applying the focused/blurred styles because
// using lipgloss.Render can degrade performance for long messages
```

**Issue:** Known performance workaround - manual style application to avoid lipgloss wrapping overhead.  
**Suggestion:** Document this as acceptable technical debt; revisit if lipgloss improves.

---

### 10. Ghostty Progress Bar Hack
**File:** `internal/ui/model/ui.go:2137`  
**Tag:** HACK  
**Code:**
```go
// HACK: use a random percentage to prevent ghostty from hiding it
// after a timeout.
v.ProgressBar = tea.NewProgressBar(tea.ProgressBarIndeterminate, rand.Intn(100))
```

**Issue:** Terminal-specific workaround for Ghostty hiding indeterminate progress bars.  
**Suggestion:** Consider adding terminal detection or filing issue upstream.

---

### 11. LSP Server Name Resolution
**File:** `internal/lsp/manager.go:47`  
**Tag:** HACK  
**Code:**
```go
// HACK: the user might have the command name in their config instead
// of the actual name. Find and use the correct name.
actualName := resolveServerName(manager, name)
```

**Issue:** Users may specify command name instead of server name in config.  
**Suggestion:** Improve config validation to accept only valid server names.

---

### 12. Windows Test Skip
**File:** `internal/shell/shell_test.go:28`  
**Tag:** XXX  
**Code:**
```go
// XXX(@andreynering): This fails on Windows. Address once possible.
if runtime.GOOS == "windows" {
    t.Skip("Skipping test on Windows")
}
```

**Issue:** Timeout test behavior differs on Windows.  
**Suggestion:** Investigate Windows signal handling differences to enable this test.

---

### 13. Synctest Race Condition
**File:** `internal/shell/background_test.go:289`  
**Tag:** XXX  
**Code:**
```go
// XXX: can't use synctest here - causes --race to trip.
```

**Issue:** Cannot use `synctest` due to race detector issues.  
**Suggestion:** Investigate race condition or document why synctest is incompatible.

---

## Summary by Tag

| Tag | Count | Items |
|-----|-------|-------|
| TODO | 8 | #1, #4, #5, #6, #7, #8 + 2 others |
| FIXME | 2 | #2, #3 |
| XXX | 3 | #9, #12, #13 |
| HACK | 2 | #10, #11 |
