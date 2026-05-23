package toolcoach

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJSONPeek(t *testing.T) {
	t.Parallel()

	t.Run("extracts file_path", func(t *testing.T) {
		t.Parallel()
		input := `{"file_path":"foo.go","old_string":"x"}`
		val, ok := jsonpeek(input, "file_path")
		require.True(t, ok)
		require.Equal(t, "foo.go", val)
	})

	t.Run("extracts pattern", func(t *testing.T) {
		t.Parallel()
		input := `{"pattern":"func Main","path":"/src"}`
		val, ok := jsonpeek(input, "pattern")
		require.True(t, ok)
		require.Equal(t, "func Main", val)
	})

	t.Run("extracts command", func(t *testing.T) {
		t.Parallel()
		input := `{"command":"ls -la"}`
		val, ok := jsonpeek(input, "command")
		require.True(t, ok)
		require.Equal(t, "ls -la", val)
	})

	t.Run("handles spaces around colon", func(t *testing.T) {
		t.Parallel()
		input := `{"file_path" : "bar.go"}`
		val, ok := jsonpeek(input, "file_path")
		require.True(t, ok)
		require.Equal(t, "bar.go", val)
	})

	t.Run("handles escaped quotes", func(t *testing.T) {
		t.Parallel()
		input := `{"old_string":"\"hello\"","new_string":"\"world\""}`
		val, ok := jsonpeek(input, "old_string")
		require.True(t, ok)
		require.Equal(t, `\"hello\"`, val)
	})

	t.Run("missing key", func(t *testing.T) {
		t.Parallel()
		input := `{"file_path":"foo.go"}`
		_, ok := jsonpeek(input, "pattern")
		require.False(t, ok)
	})

	t.Run("non-string value", func(t *testing.T) {
		t.Parallel()
		input := `{"count":42}`
		_, ok := jsonpeek(input, "count")
		require.False(t, ok)
	})

	t.Run("empty input", func(t *testing.T) {
		t.Parallel()
		_, ok := jsonpeek("", "file_path")
		require.False(t, ok)
	})
}

func BenchmarkJSONPeek(b *testing.B) {
	input := `{"file_path":"foo.go","old_string":"x","new_string":"y"}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		jsonpeek(input, "file_path")
	}
}
