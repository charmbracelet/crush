package symbols

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractFile_Go(t *testing.T) {
	t.Parallel()
	ext := NewExtractor()
	result := ext.ExtractFile("testdata/go/sample.go")

	assert.False(t, result.Fallback, "should not fallback: %s", result.Reason)
	require.NotEmpty(t, result.Symbols)

	names := symbolNames(result.Symbols)
	assert.Contains(t, names, "Greet")
	assert.Contains(t, names, "Add")
	assert.Contains(t, names, "Multiply")
}

func TestExtractFile_Python(t *testing.T) {
	t.Parallel()
	ext := NewExtractor()
	result := ext.ExtractFile("testdata/python/sample.py")

	assert.False(t, result.Fallback, "should not fallback: %s", result.Reason)
	require.NotEmpty(t, result.Symbols)

	names := symbolNames(result.Symbols)
	assert.Contains(t, names, "greet")
	assert.Contains(t, names, "add")
}

func TestExtractFile_TypeScript(t *testing.T) {
	t.Parallel()
	ext := NewExtractor()
	result := ext.ExtractFile("testdata/typescript/sample.ts")

	assert.False(t, result.Fallback, "should not fallback: %s", result.Reason)
	require.NotEmpty(t, result.Symbols)

	names := symbolNames(result.Symbols)
	assert.Contains(t, names, "greet")
	assert.Contains(t, names, "add")
}

func TestExtractFile_BrokenSyntax_Fallback(t *testing.T) {
	t.Parallel()
	ext := NewExtractor()
	result := ext.ExtractFile("testdata/broken/sample.go")

	assert.True(t, result.Fallback, "broken syntax should trigger fallback")
	assert.Contains(t, result.Reason, "parse tree contains errors")
}

func TestExtractFile_UnsupportedLanguage(t *testing.T) {
	t.Parallel()
	ext := NewExtractor()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "data.xyz")
	require.NoError(t, os.WriteFile(path, []byte("some content"), 0o644))

	result := ext.ExtractFile(path)
	assert.True(t, result.Fallback)
	assert.Contains(t, result.Reason, "unsupported language")
}

func TestExtractFile_MissingFile(t *testing.T) {
	t.Parallel()
	ext := NewExtractor()
	result := ext.ExtractFile("/nonexistent/path/file.go")

	assert.True(t, result.Fallback)
	assert.Contains(t, result.Reason, "read error")
}

func TestExtractFile_SymbolLineRanges(t *testing.T) {
	t.Parallel()
	ext := NewExtractor()
	result := ext.ExtractFile("testdata/go/sample.go")

	require.False(t, result.Fallback, "should not fallback: %s", result.Reason)

	for _, sym := range result.Symbols {
		assert.GreaterOrEqual(t, sym.StartLine, 0, "start line must be non-negative")
		assert.GreaterOrEqual(t, sym.EndLine, sym.StartLine, "end line must be >= start line")
	}
}

// TestExtractFile_SymbolSpansWholeDefinition is the regression test for
// symbols collapsing to a single line. These ranges exist to be chunk
// boundaries, so a symbol must cover its body, not just the line its
// identifier sits on — the earlier NameRange-based version reported
// Greet as 5..5 and would have chunked away every function body.
func TestExtractFile_SymbolSpansWholeDefinition(t *testing.T) {
	t.Parallel()
	ext := NewExtractor()
	result := ext.ExtractFile("testdata/go/sample.go")
	require.False(t, result.Fallback, "should not fallback: %s", result.Reason)

	// Line numbers are 0-based; see testdata/go/sample.go.
	want := map[string][2]int{
		"Greet":    {5, 7},   // func Greet ... through its closing brace
		"Add":      {10, 12}, //
		"Multiply": {18, 21}, //
	}
	got := make(map[string][2]int, len(result.Symbols))
	for _, s := range result.Symbols {
		got[s.Name] = [2]int{s.StartLine, s.EndLine}
	}
	for name, lines := range want {
		assert.Equal(t, lines, got[name], "%s should span its whole definition", name)
	}
}

// TestExtractorReusesLanguageTools pins the per-language parser/tagger
// cache. Building a Tagger compiles its tags query, so rebuilding one per
// file made query compilation the dominant cost of a walk.
func TestExtractorReusesLanguageTools(t *testing.T) {
	t.Parallel()
	ext := NewExtractor()

	require.False(t, ext.ExtractFile("testdata/go/sample.go").Fallback)
	first := ext.tools["go"]
	require.NotNil(t, first, "expected a cached tagger for go")

	require.False(t, ext.ExtractFile("testdata/go/sample.go").Fallback)
	assert.Same(t, first, ext.tools["go"], "second file should reuse the cached tools")
}

// TestWalkAndExtract_HiddenRootIsWalked guards the case where the caller
// explicitly points at a directory the skip rules would otherwise reject.
// Skipping the root turns an intentional request into a silent no-op.
func TestWalkAndExtract_HiddenRootIsWalked(t *testing.T) {
	t.Parallel()
	ext := NewExtractor()

	root := filepath.Join(t.TempDir(), ".hidden-root")
	require.NoError(t, os.MkdirAll(root, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"),
		[]byte("package a\n\nfunc Foo() {\n\treturn\n}\n"), 0o644))

	var paths []string
	for r := range ext.WalkAndExtract(context.Background(), root) {
		paths = append(paths, r.Path)
	}

	require.Len(t, paths, 1, "the root the caller asked for must be walked")
	assert.Contains(t, paths[0], "a.go")
}

func TestWalkAndExtract(t *testing.T) {
	t.Parallel()
	ext := NewExtractor()
	ctx := context.Background()

	ch := ext.WalkAndExtract(ctx, "testdata")

	var results []ExtractResult
	for r := range ch {
		results = append(results, r)
	}

	require.NotEmpty(t, results, "should find at least one file")

	var successCount int
	for _, r := range results {
		if !r.Fallback && len(r.Symbols) > 0 {
			successCount++
		}
	}
	assert.GreaterOrEqual(t, successCount, 3, "should extract symbols from Go, Python, and TypeScript fixtures")
}

func TestWalkAndExtract_SkipsHiddenDirs(t *testing.T) {
	t.Parallel()
	ext := NewExtractor()

	tmpDir := t.TempDir()
	hiddenDir := filepath.Join(tmpDir, ".hidden")
	require.NoError(t, os.MkdirAll(hiddenDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hiddenDir, "test.go"), []byte("package hidden\nfunc Foo() {}\n"), 0o644))

	visibleFile := filepath.Join(tmpDir, "visible.go")
	require.NoError(t, os.WriteFile(visibleFile, []byte("package visible\nfunc Bar() {}\n"), 0o644))

	ctx := context.Background()
	ch := ext.WalkAndExtract(ctx, tmpDir)

	var paths []string
	for r := range ch {
		paths = append(paths, r.Path)
	}

	assert.Len(t, paths, 1, "should only find the visible file")
	assert.Contains(t, paths[0], "visible.go")
}

func TestWalkAndExtract_ContextCancellation(t *testing.T) {
	t.Parallel()
	ext := NewExtractor()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	ch := ext.WalkAndExtract(ctx, "testdata")

	var count int
	for range ch {
		count++
	}
	assert.Zero(t, count, "cancelled context should produce no results")
}

func symbolNames(symbols []Symbol) []string {
	names := make([]string, len(symbols))
	for i, s := range symbols {
		names[i] = s.Name
	}
	return names
}
