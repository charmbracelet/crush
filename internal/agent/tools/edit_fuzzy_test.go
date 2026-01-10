package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/filetracker"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestFindBestMatch_ExactMatch(t *testing.T) {
	t.Parallel()

	content := "func foo() {\n\treturn 1\n}\n"
	oldString := "func foo() {\n\treturn 1\n}"

	matched, found, isMultiple := findBestMatch(content, oldString)
	require.True(t, found)
	require.False(t, isMultiple)
	require.Equal(t, oldString, matched)
}

func TestFindBestMatch_TrailingWhitespacePerLine(t *testing.T) {
	t.Parallel()

	// Content has no trailing spaces, but oldString has trailing spaces.
	content := "func foo() {\n\treturn 1\n}\n"
	oldString := "func foo() {  \n\treturn 1  \n}"

	matched, found, isMultiple := findBestMatch(content, oldString)
	require.True(t, found)
	require.False(t, isMultiple)
	require.Equal(t, "func foo() {\n\treturn 1\n}", matched)
}

func TestFindBestMatch_TrailingNewline(t *testing.T) {
	t.Parallel()

	// Content has trailing newline, oldString doesn't.
	content := "line1\nline2\n"
	oldString := "line1\nline2"

	matched, found, isMultiple := findBestMatch(content, oldString)
	require.True(t, found)
	require.False(t, isMultiple)
	require.Equal(t, "line1\nline2", matched)
}

func TestFindBestMatch_MissingTrailingNewline(t *testing.T) {
	t.Parallel()

	// Content doesn't have trailing newline after match, but oldString does.
	content := "line1\nline2"
	oldString := "line1\nline2\n"

	matched, found, isMultiple := findBestMatch(content, oldString)
	require.True(t, found)
	require.False(t, isMultiple)
	require.Equal(t, "line1\nline2", matched)
}

func TestFindBestMatch_IndentationDifference(t *testing.T) {
	t.Parallel()

	// Content uses tabs, oldString uses spaces.
	content := "func foo() {\n\treturn 1\n}\n"
	oldString := "func foo() {\n    return 1\n}"

	matched, found, isMultiple := findBestMatch(content, oldString)
	require.True(t, found)
	require.False(t, isMultiple)
	require.Equal(t, "func foo() {\n\treturn 1\n}", matched)
}

func TestFindBestMatch_DifferentIndentLevel(t *testing.T) {
	t.Parallel()

	// Content has 4-space indent, oldString has 2-space indent.
	content := "func foo() {\n    return 1\n}\n"
	oldString := "func foo() {\n  return 1\n}"

	matched, found, isMultiple := findBestMatch(content, oldString)
	require.True(t, found)
	require.False(t, isMultiple)
	require.Equal(t, "func foo() {\n    return 1\n}", matched)
}

func TestFindBestMatch_CollapseBlankLines(t *testing.T) {
	t.Parallel()

	// Content has single blank line, oldString has multiple.
	content := "line1\n\nline2\n"
	oldString := "line1\n\n\n\nline2"

	matched, found, isMultiple := findBestMatch(content, oldString)
	require.True(t, found)
	require.False(t, isMultiple)
	require.Equal(t, "line1\n\nline2", matched)
}

func TestFindBestMatch_MultipleMatches(t *testing.T) {
	t.Parallel()

	content := "foo\nbar\nfoo\n"
	oldString := "foo"

	matched, found, isMultiple := findBestMatch(content, oldString)
	require.True(t, found)
	require.True(t, isMultiple)
	require.Equal(t, "foo", matched)
}

func TestFindBestMatch_NoMatch(t *testing.T) {
	t.Parallel()

	content := "func foo() {\n\treturn 1\n}\n"
	oldString := "func bar() {\n\treturn 2\n}"

	_, found, _ := findBestMatch(content, oldString)
	require.False(t, found)
}

func TestFindBestMatch_StripsViewLineNumbers(t *testing.T) {
	t.Parallel()

	content := "line1\nline2\nline3\n"
	oldString := "  1|line1\n  2|line2\n  3|line3"

	matched, found, isMultiple := findBestMatch(content, oldString)
	require.True(t, found)
	require.False(t, isMultiple)
	require.Equal(t, "line1\nline2\nline3", matched)
}

func TestFindBestMatch_StripsMarkdownCodeFences(t *testing.T) {
	t.Parallel()

	content := "line1\nline2\n"
	oldString := "```go\nline1\nline2\n```"

	matched, found, isMultiple := findBestMatch(content, oldString)
	require.True(t, found)
	require.False(t, isMultiple)
	require.Equal(t, "line1\nline2", matched)
}

func TestFindBestMatch_TrimsSurroundingBlankLines(t *testing.T) {
	t.Parallel()

	content := "line1\nline2\n"
	oldString := "\n\nline1\nline2\n\n"

	matched, found, isMultiple := findBestMatch(content, oldString)
	require.True(t, found)
	require.False(t, isMultiple)
	require.Equal(t, "line1\nline2", matched)
}

func TestFindBestMatch_StripsZeroWidthCharacters(t *testing.T) {
	t.Parallel()

	content := "line1\nline2\n"
	oldString := "line\u200b1\nline2"

	matched, found, isMultiple := findBestMatch(content, oldString)
	require.True(t, found)
	require.False(t, isMultiple)
	require.Equal(t, "line1\nline2", matched)
}

func TestFindBestMatch_ComplexIndentation(t *testing.T) {
	t.Parallel()

	content := `func example() {
	if true {
		doSomething()
	}
}
`
	// Model provided with wrong indentation (2 spaces instead of tabs).
	oldString := `func example() {
  if true {
    doSomething()
  }
}`

	matched, found, isMultiple := findBestMatch(content, oldString)
	require.True(t, found)
	require.False(t, isMultiple)
	require.Contains(t, matched, "\t")
}

func TestApplyEditToContent_FuzzyMatch(t *testing.T) {
	t.Parallel()

	// Content uses tabs, edit uses spaces - should still match.
	content := "func foo() {\n\treturn 1\n}\n"

	newContent, err := applyEditToContent(content, MultiEditOperation{
		OldString: "func foo() {\n    return 1\n}",
		NewString: "func foo() {\n\treturn 2\n}",
	})
	require.NoError(t, err)
	require.Contains(t, newContent, "return 2")
}

func TestApplyEditToContent_FuzzyMatchTrailingSpaces(t *testing.T) {
	t.Parallel()

	content := "line 1\nline 2\nline 3\n"

	// Edit has trailing spaces that don't exist in content.
	newContent, err := applyEditToContent(content, MultiEditOperation{
		OldString: "line 1  \nline 2  ",
		NewString: "LINE 1\nLINE 2",
	})
	require.NoError(t, err)
	require.Contains(t, newContent, "LINE 1")
	require.Contains(t, newContent, "LINE 2")
}

func TestApplyEditToContent_FuzzyMatchReplaceAll(t *testing.T) {
	t.Parallel()

	content := "foo bar\nfoo baz\n"

	// With replaceAll and fuzzy match (trailing space).
	newContent, err := applyEditToContent(content, MultiEditOperation{
		OldString:  "foo ",
		NewString:  "FOO ",
		ReplaceAll: true,
	})
	require.NoError(t, err)
	require.Contains(t, newContent, "FOO bar")
	require.Contains(t, newContent, "FOO baz")
}

func TestTrimTrailingWhitespacePerLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "trailing spaces",
			input:    "line1  \nline2   \nline3",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "trailing tabs",
			input:    "line1\t\nline2\t\t\nline3",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "mixed trailing whitespace",
			input:    "line1 \t \nline2\t \nline3  ",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "no trailing whitespace",
			input:    "line1\nline2\nline3",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "preserves leading whitespace",
			input:    "  line1  \n\tline2\t\n    line3  ",
			expected: "  line1\n\tline2\n    line3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := trimTrailingWhitespacePerLine(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestCollapseBlankLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "multiple blank lines",
			input:    "line1\n\n\n\nline2",
			expected: "line1\n\nline2",
		},
		{
			name:     "single blank line unchanged",
			input:    "line1\n\nline2",
			expected: "line1\n\nline2",
		},
		{
			name:     "no blank lines",
			input:    "line1\nline2",
			expected: "line1\nline2",
		},
		{
			name:     "many blank lines",
			input:    "line1\n\n\n\n\n\n\nline2",
			expected: "line1\n\nline2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := collapseBlankLines(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

// Integration tests that test the actual replaceContent and deleteContent
// functions with real files.

func newTestEditContext(t *testing.T) (editContext, string) {
	t.Helper()
	tmpDir := t.TempDir()
	permissions := &mockPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
	files := &mockHistoryService{Broker: pubsub.NewBroker[history.File]()}
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	return editContext{ctx, permissions, files, tmpDir}, tmpDir
}

func TestEditTool_ReplaceContent_FuzzyIndentation(t *testing.T) {
	t.Parallel()

	edit, tmpDir := newTestEditContext(t)
	testFile := filepath.Join(tmpDir, "test.go")

	// File uses tabs for indentation.
	content := "func foo() {\n\treturn 1\n}\n"
	err := os.WriteFile(testFile, []byte(content), 0o644)
	require.NoError(t, err)

	// Simulate reading the file first.
	filetracker.RecordRead(testFile)

	// Model provides spaces instead of tabs.
	oldString := "func foo() {\n    return 1\n}"
	newString := "func foo() {\n\treturn 2\n}"

	resp, err := replaceContent(edit, testFile, oldString, newString, false, fantasy.ToolCall{ID: "test"})
	require.NoError(t, err)
	require.False(t, resp.IsError, "expected no error, got: %s", resp.Content)

	// Verify the file was updated.
	result, err := os.ReadFile(testFile)
	require.NoError(t, err)
	require.Contains(t, string(result), "return 2")
}

func TestEditTool_ReplaceContent_FuzzyTrailingWhitespace(t *testing.T) {
	t.Parallel()

	edit, tmpDir := newTestEditContext(t)
	testFile := filepath.Join(tmpDir, "test.txt")

	// File has no trailing whitespace.
	content := "line 1\nline 2\nline 3\n"
	err := os.WriteFile(testFile, []byte(content), 0o644)
	require.NoError(t, err)

	filetracker.RecordRead(testFile)

	// Model provides trailing spaces.
	oldString := "line 1  \nline 2  "
	newString := "LINE 1\nLINE 2"

	resp, err := replaceContent(edit, testFile, oldString, newString, false, fantasy.ToolCall{ID: "test"})
	require.NoError(t, err)
	require.False(t, resp.IsError, "expected no error, got: %s", resp.Content)

	result, err := os.ReadFile(testFile)
	require.NoError(t, err)
	require.Contains(t, string(result), "LINE 1")
	require.Contains(t, string(result), "LINE 2")
}

func TestEditTool_ReplaceContent_FuzzyTrailingNewline(t *testing.T) {
	t.Parallel()

	edit, tmpDir := newTestEditContext(t)
	testFile := filepath.Join(tmpDir, "test.txt")

	// File content.
	content := "hello\nworld\n"
	err := os.WriteFile(testFile, []byte(content), 0o644)
	require.NoError(t, err)

	filetracker.RecordRead(testFile)

	// Model omits trailing newline.
	oldString := "hello\nworld"
	newString := "HELLO\nWORLD"

	resp, err := replaceContent(edit, testFile, oldString, newString, false, fantasy.ToolCall{ID: "test"})
	require.NoError(t, err)
	require.False(t, resp.IsError, "expected no error, got: %s", resp.Content)

	result, err := os.ReadFile(testFile)
	require.NoError(t, err)
	require.Contains(t, string(result), "HELLO")
	require.Contains(t, string(result), "WORLD")
}

func TestEditTool_ReplaceContent_ExactMatchStillWorks(t *testing.T) {
	t.Parallel()

	edit, tmpDir := newTestEditContext(t)
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "func foo() {\n\treturn 1\n}\n"
	err := os.WriteFile(testFile, []byte(content), 0o644)
	require.NoError(t, err)

	filetracker.RecordRead(testFile)

	// Exact match should still work.
	oldString := "func foo() {\n\treturn 1\n}"
	newString := "func foo() {\n\treturn 2\n}"

	resp, err := replaceContent(edit, testFile, oldString, newString, false, fantasy.ToolCall{ID: "test"})
	require.NoError(t, err)
	require.False(t, resp.IsError, "expected no error, got: %s", resp.Content)

	result, err := os.ReadFile(testFile)
	require.NoError(t, err)
	require.Contains(t, string(result), "return 2")
}

func TestEditTool_ReplaceContent_NoMatchStillFails(t *testing.T) {
	t.Parallel()

	edit, tmpDir := newTestEditContext(t)
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "func foo() {\n\treturn 1\n}\n"
	err := os.WriteFile(testFile, []byte(content), 0o644)
	require.NoError(t, err)

	filetracker.RecordRead(testFile)

	// Completely wrong content should still fail.
	oldString := "func bar() {\n\treturn 999\n}"
	newString := "func baz() {\n\treturn 0\n}"

	resp, err := replaceContent(edit, testFile, oldString, newString, false, fantasy.ToolCall{ID: "test"})
	require.NoError(t, err)
	require.True(t, resp.IsError, "expected error for no match")
	require.Contains(t, resp.Content, "not found")
}

func TestEditTool_DeleteContent_FuzzyIndentation(t *testing.T) {
	t.Parallel()

	edit, tmpDir := newTestEditContext(t)
	testFile := filepath.Join(tmpDir, "test.go")

	// File uses tabs.
	content := "func foo() {\n\treturn 1\n}\n\nfunc bar() {\n\treturn 2\n}\n"
	err := os.WriteFile(testFile, []byte(content), 0o644)
	require.NoError(t, err)

	filetracker.RecordRead(testFile)

	// Model provides spaces instead of tabs.
	oldString := "func foo() {\n    return 1\n}\n\n"

	resp, err := deleteContent(edit, testFile, oldString, false, fantasy.ToolCall{ID: "test"})
	require.NoError(t, err)
	require.False(t, resp.IsError, "expected no error, got: %s", resp.Content)

	result, err := os.ReadFile(testFile)
	require.NoError(t, err)
	require.NotContains(t, string(result), "return 1")
	require.Contains(t, string(result), "return 2")
}

func TestEditTool_ReplaceContent_ReplaceAllFuzzy(t *testing.T) {
	t.Parallel()

	edit, tmpDir := newTestEditContext(t)
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "foo bar\nfoo baz\nfoo qux\n"
	err := os.WriteFile(testFile, []byte(content), 0o644)
	require.NoError(t, err)

	filetracker.RecordRead(testFile)

	// ReplaceAll with exact match.
	oldString := "foo"
	newString := "FOO"

	resp, err := replaceContent(edit, testFile, oldString, newString, true, fantasy.ToolCall{ID: "test"})
	require.NoError(t, err)
	require.False(t, resp.IsError, "expected no error, got: %s", resp.Content)

	result, err := os.ReadFile(testFile)
	require.NoError(t, err)
	require.Contains(t, string(result), "FOO bar")
	require.Contains(t, string(result), "FOO baz")
	require.Contains(t, string(result), "FOO qux")
	require.NotContains(t, string(result), "foo")
}

func TestEditTool_ReplaceContent_MultipleMatchesFails(t *testing.T) {
	t.Parallel()

	edit, tmpDir := newTestEditContext(t)
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "foo\nbar\nfoo\n"
	err := os.WriteFile(testFile, []byte(content), 0o644)
	require.NoError(t, err)

	filetracker.RecordRead(testFile)

	// Should fail because "foo" appears multiple times.
	oldString := "foo"
	newString := "FOO"

	resp, err := replaceContent(edit, testFile, oldString, newString, false, fantasy.ToolCall{ID: "test"})
	require.NoError(t, err)
	require.True(t, resp.IsError, "expected error for multiple matches")
	require.Contains(t, resp.Content, "multiple times")
}
