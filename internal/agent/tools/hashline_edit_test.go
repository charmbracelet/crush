package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/filetracker"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

type hashlineEditFileTracker struct {
	lastRead map[string]time.Time
}

func newHashlineEditFileTracker() *hashlineEditFileTracker {
	return &hashlineEditFileTracker{lastRead: make(map[string]time.Time)}
}

func (m *hashlineEditFileTracker) RecordRead(_ context.Context, _ string, path string) {
	m.lastRead[path] = time.Now().Add(time.Second)
}

func (m *hashlineEditFileTracker) LastReadTime(_ context.Context, _ string, path string) time.Time {
	if value, ok := m.lastRead[path]; ok {
		return value
	}
	return time.Time{}
}

func (m *hashlineEditFileTracker) ListReadFiles(context.Context, string) ([]string, error) {
	return nil, nil
}

var _ filetracker.Service = (*hashlineEditFileTracker)(nil)

func runHashlineEditTool(t *testing.T, tool fantasy.AgentTool, ctx context.Context, params HashlineEditParams) (fantasy.ToolResponse, error) {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	return tool.Run(ctx, fantasy.ToolCall{
		ID:    "test-call",
		Name:  HashlineEditToolName,
		Input: string(input),
	})
}

func newHashlineEditToolForTest(t *testing.T, workingDir string, tracker filetracker.Service) fantasy.AgentTool {
	t.Helper()

	permissions := &mockPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
	historySvc := &mockHistoryService{Broker: pubsub.NewBroker[history.File]()}
	return NewHashlineEditTool(nil, permissions, historySvc, tracker, workingDir)
}

func TestHashlineEditRejectsHashMismatch(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("alpha\nbeta\ngamma\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	tracker.lastRead[filePath] = time.Now().Add(time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	lineRef := formatHashlineReference(2, "beta")
	lineRef = lineRef[:len(lineRef)-1] + mutateHashlineNibble(lineRef[len(lineRef)-1])

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpReplaceLine,
				Line:      lineRef,
				Content:   "BETA",
			},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "hash mismatch")

	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	require.Equal(t, "alpha\nbeta\ngamma\n", string(data))
}

func TestHashlineEditAppliesOperationsSuccessfully(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("alpha\nbeta\ngamma\ndelta\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	tracker.lastRead[filePath] = time.Now().Add(time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	ref1 := formatHashlineReference(1, "alpha")
	ref2 := formatHashlineReference(2, "beta")
	ref3 := formatHashlineReference(3, "gamma")
	ref4 := formatHashlineReference(4, "delta")

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpPrepend,
				Line:      ref3,
				Content:   "before-gamma",
			},
			{
				Operation: hashlineEditOpAppend,
				Line:      ref3,
				Content:   "after-gamma",
			},
			{
				Operation: hashlineEditOpReplaceRange,
				Start:     ref1,
				End:       ref2,
				Content:   "HEADER",
			},
			{
				Operation: hashlineEditOpReplaceLine,
				Line:      ref4,
				Content:   "DELTA!",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "Applied 4 hashline operation")

	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	require.Equal(t, "HEADER\nbefore-gamma\ngamma\nafter-gamma\nDELTA!\n", string(data))
}

func TestHashlineEditDetectsRemovedAnchoredLineInLaterOperation(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("alpha\nbeta\ngamma\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	tracker.lastRead[filePath] = time.Now().Add(time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	ref2 := formatHashlineReference(2, "beta")

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpReplaceRange,
				Start:     ref2,
				End:       ref2,
				Content:   "",
			},
			{
				Operation: hashlineEditOpAppend,
				Line:      ref2,
				Content:   "extra",
			},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, strings.ToLower(resp.Content), "line 2 no longer exists")

	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	require.Equal(t, "alpha\nbeta\ngamma\n", string(data))
}

func mutateHashlineNibble(current byte) string {
	for i := 0; i < len(hashlineNibbleAlphabet); i++ {
		if hashlineNibbleAlphabet[i] != current {
			return string(hashlineNibbleAlphabet[i])
		}
	}
	return string(current)
}

// TestHashlineEditMultiplePrependsSameLine tests multiple prepends to the same original line.
func TestHashlineEditMultiplePrependsSameLine(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("line1\nline2\nline3\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	tracker.lastRead[filePath] = time.Now().Add(time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	// All operations reference the original line 2 ("line2")
	ref2 := formatHashlineReference(2, "line2")

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpPrepend,
				Line:      ref2,
				Content:   "before2-first",
			},
			{
				Operation: hashlineEditOpPrepend,
				Line:      ref2,
				Content:   "before2-second",
			},
			{
				Operation: hashlineEditOpPrepend,
				Line:      ref2,
				Content:   "before2-third",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, "Response error: %s", resp.Content)

	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	// All prepends should be inserted before the original line 2, in order
	expected := "line1\nbefore2-first\nbefore2-second\nbefore2-third\nline2\nline3\n"
	require.Equal(t, expected, string(data))
}

// TestHashlineEditMultipleAppendsSameLine tests multiple appends to the same original line.
func TestHashlineEditMultipleAppendsSameLine(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("line1\nline2\nline3\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	tracker.lastRead[filePath] = time.Now().Add(time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	// All operations reference the original line 2 ("line2")
	ref2 := formatHashlineReference(2, "line2")

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpAppend,
				Line:      ref2,
				Content:   "after2-first",
			},
			{
				Operation: hashlineEditOpAppend,
				Line:      ref2,
				Content:   "after2-second",
			},
			{
				Operation: hashlineEditOpAppend,
				Line:      ref2,
				Content:   "after2-third",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, "Response error: %s", resp.Content)

	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	// All appends should be inserted after the original line 2, in order
	expected := "line1\nline2\nafter2-first\nafter2-second\nafter2-third\nline3\n"
	require.Equal(t, expected, string(data))
}

// TestHashlineEditMixedPrependAppendSameLine tests mixed prepend and append to the same line.
func TestHashlineEditMixedPrependAppendSameLine(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("line1\nline2\nline3\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	tracker.lastRead[filePath] = time.Now().Add(time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	ref2 := formatHashlineReference(2, "line2")

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpAppend,
				Line:      ref2,
				Content:   "after2-first",
			},
			{
				Operation: hashlineEditOpPrepend,
				Line:      ref2,
				Content:   "before2-first",
			},
			{
				Operation: hashlineEditOpAppend,
				Line:      ref2,
				Content:   "after2-second",
			},
			{
				Operation: hashlineEditOpPrepend,
				Line:      ref2,
				Content:   "before2-second",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, "Response error: %s", resp.Content)

	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	// Operations should be applied sequentially:
	// 1. append after line 2: line1, line2, after2-first, line3
	// 2. prepend before line 2: line1, before2-first, line2, after2-first, line3
	// 3. append after line 2: line1, before2-first, line2, after2-first, after2-second, line3
	// 4. prepend before line 2: line1, before2-first, before2-second, line2, after2-first, after2-second, line3
	expected := "line1\nbefore2-first\nbefore2-second\nline2\nafter2-first\nafter2-second\nline3\n"
	require.Equal(t, expected, string(data))
}

// TestHashlineEditReplaceRangeThenReferenceAfter tests referencing a line after a replaced range.
func TestHashlineEditReplaceRangeThenReferenceAfter(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("line1\nline2\nline3\nline4\nline5\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	tracker.lastRead[filePath] = time.Now().Add(time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	ref1 := formatHashlineReference(1, "line1")
	ref3 := formatHashlineReference(3, "line3")
	ref5 := formatHashlineReference(5, "line5")

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpReplaceRange,
				Start:     ref1,
				End:       ref3,
				Content:   "REPLACED",
			},
			{
				Operation: hashlineEditOpReplaceLine,
				Line:      ref5,
				Content:   "NEW_LINE5",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, "Response error: %s", resp.Content)

	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	// Original line 5 should now be at line 3 (replaced 3 lines with 1)
	expected := "REPLACED\nline4\nNEW_LINE5\n"
	require.Equal(t, expected, string(data))
}

// TestHashlineEditReplaceRangeWithMultipleLinesThenReferenceAfter tests referencing after
// replacing a range with multiple lines.
func TestHashlineEditReplaceRangeWithMultipleLinesThenReferenceAfter(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("line1\nline2\nline3\nline4\nline5\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	tracker.lastRead[filePath] = time.Now().Add(time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	ref1 := formatHashlineReference(1, "line1")
	ref2 := formatHashlineReference(2, "line2")
	ref5 := formatHashlineReference(5, "line5")

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpReplaceRange,
				Start:     ref1,
				End:       ref2,
				Content:   "newA\nnewB\nnewC",
			},
			{
				Operation: hashlineEditOpReplaceLine,
				Line:      ref5,
				Content:   "NEW_LINE5",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, "Response error: %s", resp.Content)

	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	// Original line 5 should now be at line 6 (replaced 2 lines with 3: +1 line)
	expected := "newA\nnewB\nnewC\nline3\nline4\nNEW_LINE5\n"
	require.Equal(t, expected, string(data))
}

// TestHashlineEditAppendToLastLine tests appending to the last line of the file.
func TestHashlineEditAppendToLastLine(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("line1\nline2\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	tracker.lastRead[filePath] = time.Now().Add(time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	ref2 := formatHashlineReference(2, "line2")

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpAppend,
				Line:      ref2,
				Content:   "line3",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, "Response error: %s", resp.Content)

	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	expected := "line1\nline2\nline3\n"
	require.Equal(t, expected, string(data))
}

// TestHashlineEditPrependToFirstLine tests prepending to the first line of the file.
func TestHashlineEditPrependToFirstLine(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("line1\nline2\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	tracker.lastRead[filePath] = time.Now().Add(time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	ref1 := formatHashlineReference(1, "line1")

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpPrepend,
				Line:      ref1,
				Content:   "header",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, "Response error: %s", resp.Content)

	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	expected := "header\nline1\nline2\n"
	require.Equal(t, expected, string(data))
}

// TestHashlineEditDeleteRangeThenAppendAtEnd tests deleting a range then appending at the deletion point.
func TestHashlineEditDeleteRangeThenAppendAtEnd(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("line1\nline2\nline3\nline4\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	tracker.lastRead[filePath] = time.Now().Add(time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	ref2 := formatHashlineReference(2, "line2")
	ref3 := formatHashlineReference(3, "line3")

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpReplaceRange,
				Start:     ref2,
				End:       ref3,
				Content:   "", // Delete the range
			},
			{
				Operation: hashlineEditOpAppend,
				Line:      ref2, // Original line 2 is deleted, should fail
				Content:   "after-deletion",
			},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.IsError, "Expected error but got success: %s", resp.Content)
	require.Contains(t, strings.ToLower(resp.Content), "no longer exists")
}

// TestHashlineEditComplexScenario tests a complex real-world scenario.
func TestHashlineEditComplexScenario(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.go")
	originalContent := `package main

import "fmt"

func main() {
	fmt.Println("hello")
	fmt.Println("world")
}
`
	require.NoError(t, os.WriteFile(filePath, []byte(originalContent), 0o644))

	tracker := newHashlineEditFileTracker()
	tracker.lastRead[filePath] = time.Now().Add(time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	ref1 := formatHashlineReference(1, "package main")
	ref6 := formatHashlineReference(6, "\tfmt.Println(\"hello\")")
	ref7 := formatHashlineReference(7, "\tfmt.Println(\"world\")")

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpPrepend,
				Line:      ref1,
				Content:   "// Copyright 2024",
			},
			{
				Operation: hashlineEditOpReplaceRange,
				Start:     ref6,
				End:       ref7,
				Content:   "\tfmt.Println(\"hello world\")",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, "Response error: %s", resp.Content)

	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	expected := `// Copyright 2024
package main

import "fmt"

func main() {
	fmt.Println("hello world")
}
`
	require.Equal(t, expected, string(data))
}

// TestHashlineEditReplaceLineThenPrependAppend tests replacing a line then prepend/append to it.
func TestHashlineEditReplaceLineThenPrependAppend(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("line1\nline2\nline3\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	tracker.lastRead[filePath] = time.Now().Add(time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	ref2 := formatHashlineReference(2, "line2")

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpReplaceLine,
				Line:      ref2,
				Content:   "NEW_LINE2",
			},
			{
				Operation: hashlineEditOpPrepend,
				Line:      ref2, // Original line 2 is replaced, but the position still exists
				Content:   "before2",
			},
			{
				Operation: hashlineEditOpAppend,
				Line:      ref2,
				Content:   "after2",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, "Response error: %s", resp.Content)

	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	// Original line 2 is replaced with NEW_LINE2, then prepend before it, then append after it
	expected := "line1\nbefore2\nNEW_LINE2\nafter2\nline3\n"
	require.Equal(t, expected, string(data))
}

// TestHashlineEditSequentialRangeReplacements tests multiple sequential range replacements.
func TestHashlineEditSequentialRangeReplacements(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("a\nb\nc\nd\ne\nf\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	tracker.lastRead[filePath] = time.Now().Add(time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	ref1 := formatHashlineReference(1, "a")
	ref2 := formatHashlineReference(2, "b")
	ref5 := formatHashlineReference(5, "e")
	ref6 := formatHashlineReference(6, "f")

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpReplaceRange,
				Start:     ref1,
				End:       ref2,
				Content:   "X",
			},
			{
				Operation: hashlineEditOpReplaceRange,
				Start:     ref5,
				End:       ref6,
				Content:   "Y",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, resp.IsError, "Response error: %s", resp.Content)

	data, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	// First replace: X, c, d, e, f (line 5->4, line 6->5)
	// Second replace: X, c, d, Y
	expected := "X\nc\nd\nY\n"
	require.Equal(t, expected, string(data))
}

// TestHashlineEditAfterExternalModification tests that hash_edit rejects operations
// when the file was modified externally (simulating edit tool usage).
func TestHashlineEditAfterExternalModification(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("line1\nline2\nline3\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	tracker.lastRead[filePath] = time.Now().Add(time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	// Simulate external modification (like edit tool modifying the file)
	time.Sleep(10 * time.Millisecond) // Ensure mod time is different
	require.NoError(t, os.WriteFile(filePath, []byte("line1\nMODIFIED\nline3\n"), 0o644))

	ref2 := formatHashlineReference(2, "line2") // This is now stale, line 2 has different content

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpReplaceLine,
				Line:      ref2,
				Content:   "NEW_LINE2",
			},
		},
	})
	require.NoError(t, err)
	// Should fail either because:
	// 1. File was modified since last read (mod time check), or
	// 2. Hash mismatch
	require.True(t, resp.IsError, "Expected error due to external modification")
}

// TestHashlineEditAfterExternalModificationWithStaleHash tests that hash_edit
// correctly rejects operations when the file content changed but we have stale hashes.
func TestHashlineEditAfterExternalModificationWithStaleHash(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	// Write original content
	require.NoError(t, os.WriteFile(filePath, []byte("line1\nline2\nline3\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	// Record read time BEFORE modification
	tracker.lastRead[filePath] = time.Now().Add(-time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	// Now modify the file externally (simulating edit tool)
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, os.WriteFile(filePath, []byte("line1\nMODIFIED\nline3\n"), 0o644))

	// The hash is for the original "line2" content, but file now has "MODIFIED"
	// AND we update the tracker to simulate that user viewed the file again after modification
	// but used stale hash from previous view
	tracker.lastRead[filePath] = time.Now().Add(time.Second)

	ref2 := formatHashlineReference(2, "line2") // Stale hash for original "line2"

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpReplaceLine,
				Line:      ref2,
				Content:   "NEW_LINE2",
			},
		},
	})
	require.NoError(t, err)
	// Should fail because hash mismatch - current line 2 content is "MODIFIED" not "line2"
	require.True(t, resp.IsError, "Expected error due to hash mismatch")
	require.Contains(t, resp.Content, "hash mismatch")
}

// TestHashlineEditRequiresReadFirst tests that hash_edit requires reading the file first.
func TestHashlineEditRequiresReadFirst(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("line1\nline2\nline3\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	// Don't record any read - simulate that file was never read with view tool
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	ref2 := formatHashlineReference(2, "line2")

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpReplaceLine,
				Line:      ref2,
				Content:   "NEW_LINE2",
			},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "must read the file before editing")
}

// TestHashlineEditSequentialOperationsWithFreshHashes tests that sequential
// hash_edit operations work correctly when using fresh hashes from previous edits.
func TestHashlineEditSequentialOperationsWithFreshHashes(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "main.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("line1\nline2\nline3\n"), 0o644))

	tracker := newHashlineEditFileTracker()
	tracker.lastRead[filePath] = time.Now().Add(time.Second)
	tool := newHashlineEditToolForTest(t, workingDir, tracker)

	ref2 := formatHashlineReference(2, "line2")

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

	// First hash_edit operation
	resp1, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpReplaceLine,
				Line:      ref2,
				Content:   "MODIFIED_LINE2",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, resp1.IsError, "First operation failed: %s", resp1.Content)

	// After successful edit, tracker.RecordRead was called, so lastRead is updated
	// Now we need fresh hashes for the modified file
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, "line1\nMODIFIED_LINE2\nline3\n", string(data))

	// Using stale hash for subsequent operation should fail
	resp2, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpReplaceLine,
				Line:      ref2, // Stale hash for original "line2"
				Content:   "NEW_CONTENT",
			},
		},
	})
	require.NoError(t, err)
	require.True(t, resp2.IsError, "Expected error due to stale hash")
	require.Contains(t, resp2.Content, "hash mismatch")

	// Now use fresh hash for the modified line
	freshRef2 := formatHashlineReference(2, "MODIFIED_LINE2")
	resp3, err := runHashlineEditTool(t, tool, ctx, HashlineEditParams{
		FilePath: filePath,
		Operations: []HashlineEditOperation{
			{
				Operation: hashlineEditOpReplaceLine,
				Line:      freshRef2,
				Content:   "NEW_LINE2",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, resp3.IsError, "Third operation failed: %s", resp3.Content)

	data, err = os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, "line1\nNEW_LINE2\nline3\n", string(data))
}
