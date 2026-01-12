# Vertex AI Thinking Toggle Bug

## Summary

Thinking mode toggle doesn't work for Claude models on Vertex AI. The UI command exists but has no effect.

## Symptoms

1. "Enable Thinking Mode" command appears in command palette (Ctrl+P)
2. Running the command shows "Thinking mode enabled" toast
3. Sidebar does NOT update to show "Thinking on"
4. Thinking is NOT actually sent to the API

## Root Cause

Two places check provider type but don't include Vertex AI:

### 1. Sidebar Display (`internal/tui/components/chat/sidebar/sidebar.go:567`)

```go
switch modelProvider.Type {
case catwalk.TypeAnthropic:
    // Shows "Thinking on/off"
default:
    // Shows "Reasoning {effort}" - WRONG for Vertex AI
}
```

**Fix:** Add `catwalk.TypeVertexAI` case that displays thinking status like Anthropic.

### 2. API Options (`internal/agent/coordinator.go:276-343`)

```go
switch providerCfg.Type {
case anthropic.Name:
    // Applies thinking options with budget_tokens
case google.Name:
    // Applies thinking_config for Google AI
// MISSING: "google-vertex" case
}
```

**Fix:** Add `"google-vertex"` case. Vertex AI uses the same API format as `google.Name` (uses `google.ParseOptions`), so it should apply `thinking_config` like the `google.Name` case.

Note: Provider building at line 822 already handles `"google-vertex"` correctly - it's only the options merging that's missing.

## Files to Modify

| File | Line | Change |
|------|------|--------|
| `internal/tui/components/chat/sidebar/sidebar.go` | 567 | Add `case catwalk.TypeVertexAI:` with same logic as `TypeAnthropic` |
| `internal/agent/coordinator.go` | 322 | Change `case google.Name:` to `case google.Name, "google-vertex":` |

## Testing

1. Configure Vertex AI provider with Claude Opus 4.5
2. Open command palette, run "Enable Thinking Mode"
3. Verify sidebar shows "Thinking On"
4. Send a message and verify thinking blocks appear in response
