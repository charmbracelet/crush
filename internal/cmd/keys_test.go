package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeysCmdJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CRUSH_GLOBAL_CONFIG", dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keybindings.json"),
		[]byte(`{"quit": ["ctrl+q"], "suspend": []}`), 0o644))

	var out, errOut bytes.Buffer
	require.NoError(t, runKeys(&out, &errOut, true))
	require.Empty(t, errOut.String())

	var entries []struct {
		Action  string   `json:"action"`
		Keys    []string `json:"keys"`
		Help    string   `json:"help"`
		Enabled bool     `json:"enabled"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &entries))
	byAction := map[string]int{}
	for i, e := range entries {
		byAction[e.Action] = i
	}
	require.Contains(t, byAction, "chat.new_session")
	require.NotContains(t, byAction, "chat.switcher", "the switcher is gone")
	require.Equal(t, []string{"alt+w"}, entries[byAction["workspaces"]].Keys)
	require.True(t, entries[byAction["workspaces"]].Enabled)
	// Retired from the defaults (roadmap P1.3a), still bindable.
	require.Contains(t, byAction, "chat.jump_session")
	require.Empty(t, entries[byAction["chat.jump_session"]].Keys)
	require.False(t, entries[byAction["chat.jump_session"]].Enabled)
	require.Equal(t, []string{"ctrl+q"}, entries[byAction["quit"]].Keys)
	require.False(t, entries[byAction["suspend"]].Enabled)
}

func TestKeysCmdTable(t *testing.T) {
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())

	var out, errOut bytes.Buffer
	require.NoError(t, runKeys(&out, &errOut, false))
	require.Regexp(t, `ACTION\s+KEYS\s+HELP`, out.String())
	require.Regexp(t, `chat\.new_session\s+alt\+n, ctrl\+n\s+new\n`, out.String())
	require.Regexp(t, `sessions\s+alt\+s, ctrl\+s\s+sessions\n`, out.String())
	require.Regexp(t, `models\s+alt\+m, ctrl\+m, ctrl\+l\s+model\n`, out.String())
}
