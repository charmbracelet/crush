package tools

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiagnoseMismatch(t *testing.T) {
	t.Parallel()

	t.Run("tabs vs spaces", func(t *testing.T) {
		t.Parallel()
		content := "func main() {\n\tfmt.Println(\"hello\")\n}\n"
		old := "func main() {\n    fmt.Println(\"hello\")\n}"
		hint := diagnoseMismatch(content, old)
		require.NotEmpty(t, hint)
		require.Contains(t, hint, "whitespace-normalized match")
		require.Contains(t, hint, "→")
		require.Contains(t, hint, "lines 1-3")
	})

	t.Run("wrong indent depth", func(t *testing.T) {
		t.Parallel()
		content := "func main() {\n\tif x {\n\t\tfmt.Println(\"deep\")\n\t}\n}\n"
		old := "if x {\n\tfmt.Println(\"deep\")\n}"
		hint := diagnoseMismatch(content, old)
		require.NotEmpty(t, hint)
		require.Contains(t, hint, "→")
	})

	t.Run("completely different text", func(t *testing.T) {
		t.Parallel()
		content := "package main\n\nfunc main() {}\n"
		old := "this text does not exist anywhere in the file at all"
		hint := diagnoseMismatch(content, old)
		require.Empty(t, hint)
	})

	t.Run("partial line match", func(t *testing.T) {
		t.Parallel()
		content := "func foo() {\n\tbar()\n\tbaz()\n}\n"
		old := "func foo() {\n\tbar()\n\tqux()\n}"
		hint := diagnoseMismatch(content, old)
		require.NotEmpty(t, hint)
		require.Contains(t, hint, "Closest match")
	})

	t.Run("visualize whitespace", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "····code", visualizeWS("    code"))
		require.Equal(t, "→code", visualizeWS("\tcode"))
		require.Equal(t, "→→code", visualizeWS("\t\tcode"))
		require.Equal(t, "code  more", visualizeWS("code  more"))
	})

	t.Run("spaces vs tabs multiline", func(t *testing.T) {
		t.Parallel()
		content := "class Foo:\n\tdef bar(self):\n\t\treturn 42\n"
		old := "class Foo:\n    def bar(self):\n        return 42"
		hint := diagnoseMismatch(content, old)
		require.NotEmpty(t, hint)
		require.Contains(t, hint, "whitespace-normalized match")
		require.Contains(t, hint, "→")
	})

	t.Run("extra trailing space", func(t *testing.T) {
		t.Parallel()
		content := "func main() {\n\tfmt.Println(\"hi\")\n}\n"
		old := "func main() {\n\tfmt.Println(\"hi\") \n}"
		hint := diagnoseMismatch(content, old)
		require.NotEmpty(t, hint)
		require.Contains(t, hint, "whitespace-normalized match")
	})

	t.Run("empty old string", func(t *testing.T) {
		t.Parallel()
		hint := diagnoseMismatch("some content", "")
		require.Empty(t, hint)
	})

	t.Run("normalizeWS", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "a b c", normalizeWS("a  b\t\tc"))
		require.Equal(t, "hello", normalizeWS("  hello  "))
		require.Equal(t, "", normalizeWS("   "))
	})
}

func TestFindAndReplaceWithDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("not found includes hint", func(t *testing.T) {
		t.Parallel()
		content := "package main\n\nfunc main() {}\n"
		old := "this does not exist"
		_, _, err := findAndReplace(content, old, "new", false)
		require.Error(t, err)
		require.Contains(t, err.Error(), "old_string not found")
	})

	t.Run("not found replaceAll includes hint", func(t *testing.T) {
		t.Parallel()
		content := "package main\n\nfunc main() {}\n"
		old := "this does not exist"
		_, _, err := findAndReplace(content, old, "new", true)
		require.Error(t, err)
		require.Contains(t, err.Error(), "old_string not found")
	})

	t.Run("exact match still works", func(t *testing.T) {
		t.Parallel()
		content := "func main() {\n\tfmt.Println(\"hello\")\n}\n"
		old := "func main() {\n\tfmt.Println(\"hello\")\n}"
		result, corrected, err := findAndReplace(content, old, "replaced", false)
		require.NoError(t, err)
		require.False(t, corrected)
		require.Equal(t, "replaced\n", result)
	})

	t.Run("fuzzy whitespace match succeeds", func(t *testing.T) {
		t.Parallel()
		content := "func main() {\n\tfmt.Println(\"hello\")\n}\n"
		old := "func main() {\n    fmt.Println(\"hello\")\n}"
		result, corrected, err := findAndReplace(content, old, "func main() {\n    fmt.Println(\"goodbye\")\n}", false)
		require.NoError(t, err)
		require.True(t, corrected)
		require.Equal(t, "func main() {\n\tfmt.Println(\"goodbye\")\n}\n", result)
	})

	t.Run("no hint for totally wrong text", func(t *testing.T) {
		t.Parallel()
		content := "package main\n"
		old := "zzzzz nothing like this"
		_, _, err := findAndReplace(content, old, "x", false)
		require.Error(t, err)
		require.Contains(t, err.Error(), "old_string not found")
		require.NotContains(t, err.Error(), "whitespace-normalized")
		require.NotContains(t, err.Error(), "Closest match")
	})
}

func TestCurrentRegionContext(t *testing.T) {
	t.Parallel()

	t.Run("not found error includes current region", func(t *testing.T) {
		t.Parallel()
		var sb strings.Builder
		for i := 1; i <= 100; i++ {
			fmt.Fprintf(&sb, "line %d\n", i)
		}
		sb.WriteString("func target() {\n\tactual body\n}\n")
		content := sb.String()

		// old resembles the target block but has drifted — the best
		// match is deep in the file, far from line 1.
		old := "func target() {\n\tstale body\n}"
		_, _, err := findAndReplace(content, old, "func target() {\n\tnew body\n}", false)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Current file content (lines")
		require.Contains(t, err.Error(), "actual body")
	})

	t.Run("region bounded around the match", func(t *testing.T) {
		t.Parallel()
		var sb strings.Builder
		for i := 1; i <= 200; i++ {
			fmt.Fprintf(&sb, "padding %d\n", i)
		}
		sb.WriteString("func target() {\n\treal line\n}\n")
		for i := 1; i <= 200; i++ {
			fmt.Fprintf(&sb, "trailer %d\n", i)
		}
		content := sb.String()

		old := "func target() {\n\twrong line\n}"
		_, _, err := findAndReplace(content, old, "x", false)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Current file content (lines")
		require.Contains(t, err.Error(), "real line")
		// ±40 lines around the match: line 1 and the tail are out of
		// range.
		require.NotContains(t, err.Error(), "padding 1\n")
		require.NotContains(t, err.Error(), "trailer 200")
	})

	t.Run("no region when nothing resembles old", func(t *testing.T) {
		t.Parallel()
		content := "package main\n\nfunc main() {}\n"
		old := "completely unrelated content that exists nowhere"
		_, _, err := findAndReplace(content, old, "x", false)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "Current file content")
	})
}

func TestWithWhitespaceNote(t *testing.T) {
	t.Parallel()
	require.Equal(t, "done", withWhitespaceNote("done", false))
	require.Contains(t, withWhitespaceNote("done", true), whitespaceCorrectedNote)
}

func TestApplyEditToContentReportsWhitespaceCorrection(t *testing.T) {
	t.Parallel()

	content := "func main() {\n\tfoo()\n}\n"
	result, corrected, err := applyEditToContent(content, MultiEditOperation{
		OldString: "    foo()",
		NewString: "    bar()",
	})
	require.NoError(t, err)
	require.True(t, corrected)
	require.Equal(t, "func main() {\n\tbar()\n}\n", result)

	result, corrected, err = applyEditToContent(content, MultiEditOperation{
		OldString: "\tfoo()",
		NewString: "\tbar()",
	})
	require.NoError(t, err)
	require.False(t, corrected)
	require.Equal(t, "func main() {\n\tbar()\n}\n", result)
}

func TestLineAtOffset(t *testing.T) {
	t.Parallel()
	lines := []string{"aaa", "bb", "ccccc"}
	require.Equal(t, 0, lineAtOffset(lines, 0))
	require.Equal(t, 0, lineAtOffset(lines, 2))
	require.Equal(t, 1, lineAtOffset(lines, 4))
	require.Equal(t, 2, lineAtOffset(lines, 7))
	require.Equal(t, 2, lineAtOffset(lines, 100))
}

func TestVisualizeWSPreservesInterior(t *testing.T) {
	t.Parallel()
	s := visualizeWS("\t\tif x > 0 {")
	require.True(t, strings.HasPrefix(s, "→→"))
	require.Contains(t, s, "if x > 0 {")
}

func TestNormalizedReplace(t *testing.T) {
	t.Parallel()

	t.Run("tabs to spaces conversion", func(t *testing.T) {
		t.Parallel()
		content := "func main() {\n\tfmt.Println(\"hello\")\n}\n"
		old := "func main() {\n    fmt.Println(\"hello\")\n}"
		new := "func main() {\n    fmt.Println(\"goodbye\")\n}"
		result, ok := normalizedReplace(content, old, new, false)
		require.True(t, ok)
		require.Equal(t, "func main() {\n\tfmt.Println(\"goodbye\")\n}\n", result)
	})

	t.Run("spaces to tabs conversion", func(t *testing.T) {
		t.Parallel()
		content := "func main() {\n    fmt.Println(\"hello\")\n}\n"
		old := "func main() {\n\tfmt.Println(\"hello\")\n}"
		new := "func main() {\n\tfmt.Println(\"goodbye\")\n}"
		result, ok := normalizedReplace(content, old, new, false)
		require.True(t, ok)
		require.Equal(t, "func main() {\n    fmt.Println(\"goodbye\")\n}\n", result)
	})

	t.Run("ambiguous match fails", func(t *testing.T) {
		t.Parallel()
		content := "func a() {\n\tfoo()\n}\nfunc b() {\n\tfoo()\n}\n"
		old := "func x() {\n    foo()\n}"
		new := "func x() {\n    bar()\n}"
		_, ok := normalizedReplace(content, old, new, false)
		require.False(t, ok)
	})

	t.Run("replaceAll with multiple matches", func(t *testing.T) {
		t.Parallel()
		content := "func a() {\n\tfoo()\n}\nfunc b() {\n\tfoo()\n}\n"
		old := "    foo()"
		new := "    bar()"
		result, ok := normalizedReplace(content, old, new, true)
		require.True(t, ok)
		require.Equal(t, "func a() {\n\tbar()\n}\nfunc b() {\n\tbar()\n}\n", result)
	})

	t.Run("partial line match rejected", func(t *testing.T) {
		t.Parallel()
		// The pattern starts mid-line, so replacing at line granularity would
		// discard "func a() " and "func b() ".
		content := "func a() {\n\tfoo()\n}\nfunc b() {\n\tfoo()\n}\n"
		old := "{\n    foo()\n}"
		new := "{\n    bar()\n}"
		_, ok := normalizedReplace(content, old, new, true)
		require.False(t, ok)
	})

	t.Run("surrounding text on the line is preserved", func(t *testing.T) {
		t.Parallel()
		content := "before foo = 1 after\n"
		_, ok := normalizedReplace(content, "foo  =  1", "foo  =  2", false)
		require.False(t, ok)
	})

	t.Run("repeated matches on one line rejected", func(t *testing.T) {
		t.Parallel()
		content := "a  b   c   a  b\n"
		_, ok := normalizedReplace(content, "a b", "Z", true)
		require.False(t, ok)
	})

	t.Run("no match returns false", func(t *testing.T) {
		t.Parallel()
		content := "package main\n"
		old := "does not exist"
		new := "replacement"
		_, ok := normalizedReplace(content, old, new, false)
		require.False(t, ok)
	})

	t.Run("same indent unit but wrong depth", func(t *testing.T) {
		t.Parallel()
		content := "func main() {\n\tif ok {\n\t\told()\n\t}\n}\n"
		old := "if ok {\n\told()\n}"
		new := "if ok {\n\tnew()\n}"
		result, ok := normalizedReplace(content, old, new, false)
		require.True(t, ok)
		require.Equal(t, "func main() {\n\tif ok {\n\t\tnew()\n\t}\n}\n", result)
	})

	t.Run("unindented old string in indented context", func(t *testing.T) {
		t.Parallel()
		content := "func f() {\n\tfoo := 1\n\tbar := 2\n}\n"
		result, ok := normalizedReplace(content, "foo :=  1", "foo := 99", false)
		require.True(t, ok)
		require.Equal(t, "func f() {\n\tfoo := 99\n\tbar := 2\n}\n", result)
	})

	t.Run("deep indentation preserved", func(t *testing.T) {
		t.Parallel()
		content := "func main() {\n\tif x {\n\t\tif y {\n\t\t\tfoo()\n\t\t}\n\t}\n}\n"
		old := "if x {\n    if y {\n        foo()\n    }\n}"
		new := "if x {\n    if y {\n        bar()\n    }\n}"
		result, ok := normalizedReplace(content, old, new, false)
		require.True(t, ok)
		require.Contains(t, result, "\t\t\tbar()")
	})
}

func TestAdaptIndentation(t *testing.T) {
	t.Parallel()

	t.Run("spaces to tabs", func(t *testing.T) {
		t.Parallel()
		actual := "func main() {\n\tfmt.Println(\"hello\")\n}"
		old := "func main() {\n    fmt.Println(\"hello\")\n}"
		new := "func main() {\n    fmt.Println(\"goodbye\")\n}"
		result := adaptIndentation(actual, old, new, "\t")
		require.Equal(t, "func main() {\n\tfmt.Println(\"goodbye\")\n}", result)
	})

	t.Run("tabs to spaces", func(t *testing.T) {
		t.Parallel()
		actual := "func main() {\n    fmt.Println(\"hello\")\n}"
		old := "func main() {\n\tfmt.Println(\"hello\")\n}"
		new := "func main() {\n\tfmt.Println(\"goodbye\")\n}"
		result := adaptIndentation(actual, old, new, "    ")
		require.Equal(t, "func main() {\n    fmt.Println(\"goodbye\")\n}", result)
	})

	t.Run("same style unchanged", func(t *testing.T) {
		t.Parallel()
		actual := "func main() {\n\tfmt.Println(\"hello\")\n}"
		old := "func main() {\n\tfmt.Println(\"hello\")\n}"
		new := "func main() {\n\tfmt.Println(\"goodbye\")\n}"
		result := adaptIndentation(actual, old, new, "\t")
		require.Equal(t, new, result)
	})

	t.Run("same style shifted deeper", func(t *testing.T) {
		t.Parallel()
		actual := "\tif ok {\n\t\told()\n\t}"
		old := "if ok {\n\told()\n}"
		new := "if ok {\n\tnew()\n}"
		result := adaptIndentation(actual, old, new, "\t")
		require.Equal(t, "\tif ok {\n\t\tnew()\n\t}", result)
	})

	t.Run("same style shifted shallower", func(t *testing.T) {
		t.Parallel()
		actual := "\tif ok {\n\t\told()\n\t}"
		old := "\t\tif ok {\n\t\t\told()\n\t\t}"
		new := "\t\tif ok {\n\t\t\tnew()\n\t\t}"
		result := adaptIndentation(actual, old, new, "\t")
		require.Equal(t, "\tif ok {\n\t\tnew()\n\t}", result)
	})

	t.Run("unindented old string uses new string style", func(t *testing.T) {
		t.Parallel()
		actual := "\tif ok {\n\t\told()\n\t}"
		old := "if ok { old() }"
		new := "if ok {\n    new()\n}"
		result := adaptIndentation(actual, old, new, "\t")
		require.Equal(t, "\tif ok {\n\t\tnew()\n\t}", result)
	})

	t.Run("2-space to 4-space", func(t *testing.T) {
		t.Parallel()
		actual := "func main() {\n    fmt.Println(\"hello\")\n}"
		old := "func main() {\n  fmt.Println(\"hello\")\n}"
		new := "func main() {\n  fmt.Println(\"goodbye\")\n}"
		result := adaptIndentation(actual, old, new, "    ")
		require.Equal(t, "func main() {\n    fmt.Println(\"goodbye\")\n}", result)
	})
}

func TestDetectIndentUnit(t *testing.T) {
	t.Parallel()

	t.Run("tabs", func(t *testing.T) {
		t.Parallel()
		lines := []string{"func main() {", "\tfmt.Println()", "}"}
		require.Equal(t, "\t", detectIndentUnit(lines))
	})

	t.Run("4 spaces", func(t *testing.T) {
		t.Parallel()
		lines := []string{"func main() {", "    fmt.Println()", "}"}
		require.Equal(t, "    ", detectIndentUnit(lines))
	})

	t.Run("2 spaces", func(t *testing.T) {
		t.Parallel()
		lines := []string{"func main() {", "  fmt.Println()", "}"}
		require.Equal(t, "  ", detectIndentUnit(lines))
	})

	t.Run("mixed uses minimum", func(t *testing.T) {
		t.Parallel()
		lines := []string{"func main() {", "  x()", "    y()", "}"}
		require.Equal(t, "  ", detectIndentUnit(lines))
	})

	t.Run("no indentation", func(t *testing.T) {
		t.Parallel()
		lines := []string{"package main", "func main() {}"}
		require.Equal(t, "", detectIndentUnit(lines))
	})
}

func TestMeasureDepth(t *testing.T) {
	t.Parallel()

	require.Equal(t, 2, measureDepth("\t\t", "\t"))
	require.Equal(t, 3, measureDepth("\t\t\t", "\t"))
	require.Equal(t, 2, measureDepth("    ", "  "))
	require.Equal(t, 1, measureDepth("    ", "    "))
	require.Equal(t, 0, measureDepth("", "  "))
	require.Equal(t, 0, measureDepth("  ", ""))
}
