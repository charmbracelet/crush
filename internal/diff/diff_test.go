package diff

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateDiff(t *testing.T) {
	t.Parallel()

	before := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n\tfmt.Println(\"Line 2\")\n}\n"
	after := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, Go!\")\n\tfmt.Println(\"Line 2\")\n}\n"

	t.Run("LF_before_LF_after", func(t *testing.T) {
		t.Parallel()

		diff, additions, removals := GenerateDiff(before, after, "main.go")

		require.Equal(t, 1, additions)
		require.Equal(t, 1, removals)
		require.Contains(t, diff, "-\tfmt.Println(\"Hello, World!\")")
		require.Contains(t, diff, "+\tfmt.Println(\"Hello, Go!\")")
	})

	t.Run("CRLF_before_LF_after", func(t *testing.T) {
		t.Parallel()

		crlfBefore := "package main\r\n\r\nimport \"fmt\"\r\n\r\nfunc main() {\r\n\tfmt.Println(\"Hello, World!\")\r\n\tfmt.Println(\"Line 2\")\r\n}\r\n"
		diff, additions, removals := GenerateDiff(crlfBefore, after, "main.go")

		require.Equal(t, 1, additions, "CRLF before should not inflate counts")
		require.Equal(t, 1, removals, "CRLF before should not inflate counts")
		require.Contains(t, diff, "-\tfmt.Println(\"Hello, World!\")")
		require.Contains(t, diff, "+\tfmt.Println(\"Hello, Go!\")")
	})

	t.Run("LF_before_CRLF_after", func(t *testing.T) {
		t.Parallel()

		crlfAfter := "package main\r\n\r\nimport \"fmt\"\r\n\r\nfunc main() {\r\n\tfmt.Println(\"Hello, Go!\")\r\n\tfmt.Println(\"Line 2\")\r\n}\r\n"
		diff, additions, removals := GenerateDiff(before, crlfAfter, "main.go")

		require.Equal(t, 1, additions, "CRLF after should not inflate counts")
		require.Equal(t, 1, removals, "CRLF after should not inflate counts")
		require.Contains(t, diff, "-\tfmt.Println(\"Hello, World!\")")
		require.Contains(t, diff, "+\tfmt.Println(\"Hello, Go!\")")
	})

	t.Run("CRLF_before_CRLF_after", func(t *testing.T) {
		t.Parallel()

		crlfBefore := "package main\r\n\r\nimport \"fmt\"\r\n\r\nfunc main() {\r\n\tfmt.Println(\"Hello, World!\")\r\n\tfmt.Println(\"Line 2\")\r\n}\r\n"
		crlfAfter := "package main\r\n\r\nimport \"fmt\"\r\n\r\nfunc main() {\r\n\tfmt.Println(\"Hello, Go!\")\r\n\tfmt.Println(\"Line 2\")\r\n}\r\n"
		diff, additions, removals := GenerateDiff(crlfBefore, crlfAfter, "main.go")

		require.Equal(t, 1, additions)
		require.Equal(t, 1, removals)
		require.Contains(t, diff, "-\tfmt.Println(\"Hello, World!\")")
		require.Contains(t, diff, "+\tfmt.Println(\"Hello, Go!\")")
	})

	t.Run("mixed_line_endings", func(t *testing.T) {
		t.Parallel()

		mixedBefore := "line1\r\nline2\nline3\r\nline4\n"
		mixedAfter := "line1\nline2\nchanged\nline4\n"
		diff, additions, removals := GenerateDiff(mixedBefore, mixedAfter, "test.txt")

		require.Equal(t, 1, additions)
		require.Equal(t, 1, removals)
		require.Contains(t, diff, "-line3")
		require.Contains(t, diff, "+changed")
	})

	t.Run("identical_content_different_endings", func(t *testing.T) {
		t.Parallel()

		lfContent := "line1\nline2\nline3\n"
		crlfContent := "line1\r\nline2\r\nline3\r\n"
		diff, additions, removals := GenerateDiff(lfContent, crlfContent, "test.txt")

		require.Equal(t, 0, additions, "identical content with different line endings should produce no diff")
		require.Equal(t, 0, removals, "identical content with different line endings should produce no diff")
		require.Empty(t, diff)
	})

	t.Run("tabs_are_not_normalized", func(t *testing.T) {
		t.Parallel()

		tabContent := "\tfoo\n"
		spaceContent := "    foo\n"
		diff, additions, removals := GenerateDiff(tabContent, spaceContent, "test.txt")

		require.Equal(t, 1, additions, "tab vs space should be a real diff")
		require.Equal(t, 1, removals, "tab vs space should be a real diff")
		require.NotEmpty(t, diff)
	})

	t.Run("empty_before", func(t *testing.T) {
		t.Parallel()

		diff, additions, removals := GenerateDiff("", "line1\nline2\n", "new.txt")

		require.Equal(t, 2, additions)
		require.Equal(t, 0, removals)
		require.Contains(t, diff, "+line1")
		require.Contains(t, diff, "+line2")
	})

	t.Run("empty_after", func(t *testing.T) {
		t.Parallel()

		diff, additions, removals := GenerateDiff("line1\nline2\n", "", "deleted.txt")

		require.Equal(t, 0, additions)
		require.Equal(t, 2, removals)
		require.Contains(t, diff, "-line1")
		require.Contains(t, diff, "-line2")
	})

	t.Run("leading_slash_trimmed", func(t *testing.T) {
		t.Parallel()

		diff, _, _ := GenerateDiff("a\n", "b\n", "/src/main.go")

		require.Contains(t, diff, "a/src/main.go")
		require.Contains(t, diff, "b/src/main.go")
		require.NotContains(t, diff, "a//src/main.go")
	})
}
