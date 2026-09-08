package keybinds

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestActions(t *testing.T) {
	t.Parallel()
	ids := Actions()
	require.Len(t, ids, 57)
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		require.NotContains(t, seen, id)
		seen[id] = struct{}{}
		require.True(t, Valid(id), "duplicate or unknown registry ID %q", id)
	}
}

func TestExcludedHaveNoID(t *testing.T) {
	t.Parallel()
	for _, id := range []string{
		"chat.tab",
		"chat.up_down",
		"chat.up_down_one_item",
		"editor.mention_file",
		"chat.add_attachment",
		"editor.select_all",
	} {
		require.False(t, Valid(id), "excluded binding %q leaked into registry", id)
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()
	require.NoError(t, Validate("global.quit", []string{"ctrl+q"}))
	require.NoError(t, Validate("chat.page_down", []string{"pgdown", "space"}))
	require.Error(t, Validate("quit", []string{"ctrl+q"}))
	require.Error(t, Validate("global.quit", nil))
	require.Error(t, Validate("global.quit", []string{"  "}))
	require.Error(t, Validate(".quit", []string{"ctrl+q"}))
}

func TestNormalizeToken(t *testing.T) {
	t.Parallel()
	require.Equal(t, "space", NormalizeToken(" "))
	require.Equal(t, "space", NormalizeToken("space"))
	require.Equal(t, "ctrl+q", NormalizeToken("ctrl+q"))
}

func TestValidShape(t *testing.T) {
	t.Parallel()
	require.True(t, ValidShape("global.quit"))
	require.True(t, ValidShape("bogus.action"))
	require.False(t, ValidShape("quit"))
	require.False(t, ValidShape(".quit"))
	require.False(t, ValidShape("global."))
	require.False(t, ValidShape(""))
}
