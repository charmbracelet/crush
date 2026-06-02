package replacer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDecision(t *testing.T) {
	t.Parallel()

	t.Run("stop", func(t *testing.T) {
		t.Parallel()
		d, err := ParseDecision(`{"action":"stop","prompt":""}`)
		require.NoError(t, err)
		require.Equal(t, "stop", d.Action)
		require.Equal(t, "", d.Prompt)
	})

	t.Run("continue", func(t *testing.T) {
		t.Parallel()
		d, err := ParseDecision(`{"action":"continue","prompt":"Tell me more"}`)
		require.NoError(t, err)
		require.Equal(t, "continue", d.Action)
		require.Equal(t, "Tell me more", d.Prompt)
	})

	t.Run("markdown fences", func(t *testing.T) {
		t.Parallel()
		d, err := ParseDecision("```json\n{\"action\":\"stop\",\"prompt\":\"\"}\n```")
		require.NoError(t, err)
		require.Equal(t, "stop", d.Action)
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		_, err := ParseDecision("")
		require.Error(t, err)
	})

	t.Run("invalid action", func(t *testing.T) {
		t.Parallel()
		_, err := ParseDecision(`{"action":"maybe","prompt":""}`)
		require.Error(t, err)
	})

	t.Run("invalid json", func(t *testing.T) {
		t.Parallel()
		_, err := ParseDecision("not json")
		require.Error(t, err)
	})
}
