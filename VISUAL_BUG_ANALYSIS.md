# Visual Bug Analysis: Text Input Field Rendering Issue

## Executive Summary

**Problem**: Text input field is invisible after session restore, only cursor visible.

**Most Likely Root Cause**: The bubbles viewport component returns an empty string when its effective content area (width or height after subtracting frame size) is 0. The cursor is still rendered because it's calculated independently.

**Key Code Paths**:
1. `viewport.View()` returns "" if `w == 0 || h == 0`
2. `viewport.visibleLines()` returns nil if `maxHeight == 0 || maxWidth == 0`  
3. Where `maxHeight = max(0, Height - Style.GetVerticalFrameSize())`

**Status**: Root cause mechanism identified. Need to determine what triggers the zero dimension condition.

---

## Problem Description

- Text input field (editor) rendering is completely gone after session restore
- Only a pink cursor block is visible at the bottom
- Cursor position appears 1-2 lines above where it should be in other instances
- Problem persists across session restore after restart

## Affected Session

Database: `~/mobile/gabriela-bogk/mob-code-search/.crush/crush.db`
Session ID: `e7f3b6fc-f3cf-41f0-95c9-beb31020213c`
Title: "Technical difficulties interrupted our progress on documentation"
Message count: 186

---

## Code Analysis

### Key Files

| File | Purpose |
|------|---------|
| `internal/tui/page/chat/chat.go` | Main chat page layout and sizing |
| `internal/tui/components/chat/editor/editor.go` | Text input component |
| `internal/tui/tui.go` | Main app model, window resize handling |
| `charm.land/bubbles/v2/textarea` | Textarea component from bubbles |
| `charm.land/bubbles/v2/viewport` | Viewport component (renders textarea content) |

### Constants

```go
EditorHeight = 5          // Height of editor including padding
HeaderHeight = 1          // Height of header (compact mode)
pillHeightWithBorder = 3  // Height of pills area
```

### Editor Height Flow

1. `WindowSizeMsg` arrives → `tui.handleWindowResize()`
2. Height adjusted: `height -= 2` (or 5 for full help)
3. `chat.SetSize(width, height)` called
4. Editor gets: `editor.SetSize(width, EditorHeight)` → always 5
5. Textarea gets: `textarea.SetHeight(height - 2)` → 3 lines
6. Position set: `editor.SetPosition(0, height - EditorHeight)`

### Cursor Position Calculation

```go
// editor.go:500-508
func (m *editorCmp) Cursor() *tea.Cursor {
    cursor := m.textarea.Cursor()
    if cursor != nil {
        c := *cursor
        c.X = c.X + m.x + 1
        c.Y = c.Y + m.y + 1  // m.y = position set by SetPosition
        return &c
    }
    return nil
}
```

---

## Theories

### Theory 1: Race Condition in Session Restore
**Status**: Less likely

When restoring a session, `setSession` calls `SetSize(p.width, p.height)`. The page dimensions are set from WindowSizeMsg which should arrive early.

Analysis shows that WindowSizeMsg sets p.width and p.height before any session operations.

### Theory 2: Pills Height Miscalculation
**Status**: Possible but unlikely

In compact mode (line 821):
```go
cmds = append(cmds, p.chat.SetSize(width, height-EditorHeight-HeaderHeight-pillsAreaHeight))
```

This affects the chat area, not the editor. Editor always gets `EditorHeight=5`.

### Theory 3: Initialization Sequence Issue
**Status**: Investigated - Not the primary cause

- Editor created with default textarea dimensions 40x6
- WindowSizeMsg should arrive before View is needed for user interaction
- Textarea has protective minimum width/height clamping

### Theory 4: Viewport Returns Empty String
**Status**: KEY FINDING

Found in `bubbles/v2/viewport/viewport.go:734-736`:
```go
if w == 0 || h == 0 {
    return ""
}
```

If the viewport has width=0 OR height=0, View() returns empty string.

**Analysis of how this could happen**:
- Viewport inside textarea is created with width=0, height=0
- textarea.New() calls SetHeight(6) and SetWidth(40) which should fix this
- editor.SetSize() calls textarea.SetWidth(width-2) and textarea.SetHeight(height-2)
- These have minimum value protection in textarea code

### Theory 5: Style Width/Height Override
**Status**: Under investigation

The viewport.View() checks if Style.GetWidth() or Style.GetHeight() return non-zero values, and uses min(dimensions, style) in that case. If a style sets width or height to 0, it wouldn't override the viewport dimensions (only non-zero values are used).

### Theory 6: Compositor/Layer Issue
**Status**: New theory

The chatPage uses `lipgloss.NewCompositor` with layers. The editor is part of the base `chatView` layer. If something causes the chatView string to be malformed or the compositor to render incorrectly, the editor could be cut off.

---

## Investigation Log

### 2026-01-16: Initial Analysis

**Examined files**:
- `internal/tui/page/chat/chat.go` - SetSize, View functions
- `internal/tui/components/chat/editor/editor.go` - SetSize, View, Cursor functions
- `internal/tui/tui.go` - handleWindowResize, WindowSizeMsg handling

**Key findings**:
1. EditorHeight is constant (5), should not change
2. Editor constructor does NOT initialize width/height (defaults to 0)
3. Recent commit `cd41a84e` fixed cursor drift by using struct copies in Cursor() methods
4. Cursor Y = textarea.cursor.Y + m.y + 1, where m.y comes from SetPosition

### 2026-01-16: Viewport Analysis

**Examined bubbles library**:
- `textarea/textarea.go` - New(), SetWidth(), SetHeight(), Cursor()
- `viewport/viewport.go` - View(), SetWidth(), SetHeight()

**Critical findings**:

1. **Viewport returns "" when dimensions are zero** (viewport.go:734-736)
   - If `w == 0 || h == 0`, `View()` returns empty string
   - This is the smoking gun for invisible editor content

2. **Textarea has minimum width protection** (textarea.go:1118-1119)
   - `minWidth := reservedInner + reservedOuter + 1`
   - `inputWidth := max(w, minWidth)`
   - Even negative widths are clamped to minWidth

3. **Textarea has minimum height protection** (textarea.go:1152-1158)
   - `m.height = max(h, minHeight)`  // minHeight = 1
   - Heights are clamped to at least 1

4. **Textarea has default dimensions** (textarea.go:369-370)
   - Default height: 6, default width: 40
   - Set in `New()` function

5. **Cursor is calculated independently of View**
   - `Cursor()` method uses `m.LineInfo()` and `m.viewport.YOffset()`
   - Does NOT depend on View() being non-empty
   - Cursor can still be positioned even when content is invisible

### 2026-01-16: Deep Dive into Dimension Propagation

**Tracing the flow**:

1. `viewport.New()` creates viewport with width=0, height=0
2. `setInitialValues()` does NOT set width/height
3. `textarea.New()` calls `SetHeight(6)` and `SetWidth(40)`
4. `textarea.SetHeight(6)` → `viewport.SetHeight(6)` 
5. `textarea.SetWidth(40)` → `viewport.SetWidth(inputWidth - reservedOuter)`

The viewport SHOULD have positive dimensions after textarea.New() completes.

**Question**: What could reset viewport dimensions to 0 after initialization?

Possibilities:
- Direct call to viewport.SetWidth(0) or SetHeight(0) - NOT found in codebase
- Style override - checked, crush styles don't set width/height on textarea
- Some bubbles internal operation - needs more investigation

---

## Current Status

The investigation has identified that the viewport returning "" is the likely cause, but we haven't found what sets the viewport dimensions to 0.

**Next steps**:
1. Add debug logging to editor.SetSize to capture width/height values
2. Add debug logging to textarea.View() to check viewport dimensions
3. Look for any resize events that could trigger with bad dimensions
4. Check if the issue is specific to session restore or window resize

---

## Test Results

(To be updated with test results)

---

## Conclusions

### Preliminary Conclusion

The most likely cause is that the viewport component inside the textarea has width=0 or height=0, which causes its View() method to return an empty string. The cursor still appears because cursor positioning is calculated independently.

### Root Cause - Potential Discovery

**New Finding**: The viewport's `visibleLines()` also checks dimensions:
```go
// viewport.go:329-335
func (m Model) visibleLines() (lines []string) {
    maxHeight := m.maxHeight()
    maxWidth := m.maxWidth()

    if maxHeight == 0 || maxWidth == 0 {
        return nil
    }
    ...
}
```

Where:
```go
func (m Model) maxWidth() int {
    return max(0, m.Width()-m.Style.GetHorizontalFrameSize()-gutterSize)
}
func (m Model) maxHeight() int {
    return max(0, m.Height()-m.Style.GetVerticalFrameSize())
}
```

**This means**: Even if viewport.Width() > 0 and viewport.Height() > 0, if the Style has frame sizes (borders, padding) that exceed the dimensions, the content area becomes 0.

For example:
- If viewport.Height() = 1 and Style has 1 line of border/padding, maxHeight = 0
- If viewport.Width() = 10 and Style has 12px of frame, maxWidth = 0

This could explain the issue if the textarea's Base style (from crush's theme) has padding/border that exceeds the available space.

### Potential Investigation Approaches

1. **Add instrumentation** - Log dimensions at key points
2. **Binary search git history** - Find when the bug was introduced
3. **Check bubbles version** - See if there were recent changes to textarea/viewport
4. **Reproduce in isolation** - Create a minimal test case

### Possible Fix (if we can't find root cause)

Add defensive guards in editor.View():
```go
func (m *editorCmp) View() string {
    t := styles.CurrentTheme()
    
    // Guard against zero dimensions
    if m.width <= 0 || m.height <= 0 {
        return "" // Or return a placeholder
    }
    
    // ... rest of view logic
}
```

Or in editor.SetSize():
```go
func (m *editorCmp) SetSize(width, height int) tea.Cmd {
    if width <= 2 || height <= 2 {
        return nil  // Skip invalid dimensions
    }
    m.width = width
    m.height = height
    m.textarea.SetWidth(width - 2)
    m.textarea.SetHeight(height - 2)
    return nil
}
```

---

## Debug Logging Added (2026-01-16)

Debug logging has been added to track dimensions when the bug is triggered.

### Files Modified

1. **`internal/tui/components/chat/editor/editor.go`**
   - `SetSize()`: Logs width, height, textarea dimensions
   - `View()`: Warns if textarea.View() returns empty string
   - `SetPosition()`: Logs x, y values

2. **`internal/tui/page/chat/chat.go`**
   - `SetSize()`: Logs incoming dimensions, session state, compact mode, and editor sizing

### Log Messages to Watch For

| Message | Level | Meaning |
|---------|-------|---------|
| `editor.SetSize` | DEBUG | Normal dimension setting |
| `editor.SetSize: invalid dimensions` | WARN | Textarea dimensions are ≤0 |
| `editor.View: textarea.View() returned empty string` | WARN | **BUG TRIGGERED** |
| `chatPage.SetSize` | DEBUG | Page-level dimension setting |
| `editor.SetPosition` | DEBUG | Editor position in the layout |

### How to Use

1. Run crush with debug logging enabled: `crush --debug` or with log level set to debug
2. Use the application normally
3. When the bug triggers, check logs for warning messages
4. Look for `editor.View: textarea.View() returned empty string` - this confirms the issue

### Expected Log Output (Normal)

```
DEBUG chatPage.SetSize width=120 height=40 session_id="" compact=false splash_fullscreen=false
DEBUG chatPage.SetSize: no session, editor sizing editor_width=120 editor_height=5 editor_y=35
DEBUG editor.SetSize width=120 height=5 textarea_width=118 textarea_height=3 actual_ta_width=118 actual_ta_height=3
DEBUG editor.SetPosition x=0 y=35
```

### Expected Log Output (Bug Triggered)

```
WARN editor.SetSize: invalid dimensions textarea_width=-1 textarea_height=0
WARN editor.View: textarea.View() returned empty string width=1 height=2 ...
```
