package filehistory

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestCombinedHistoryMultiTurnRedoAndRecovery(t *testing.T) {
	s := fixture(t)
	file := filepath.Join(s.Cwd, "asset.bin")
	require.NoError(t, os.WriteFile(file, []byte{0, 255, 7}, 0600))
	current := json.RawMessage(`"before first"`)
	capture := func(label string) {
		ctx, release, err := s.Begin(t.Context(), "session")
		require.NoError(t, err)
		require.NoError(t, RecordConversation(ctx, label, current))
		release()
	}
	capture("first")
	require.NoError(t, os.WriteFile(file, []byte("first result"), 0600))
	current = json.RawMessage(`"before second"`)
	ctx, release, err := s.Begin(t.Context(), "session")
	require.NoError(t, err)
	require.NoError(t, RecordConversation(ctx, "second", current))
	created := filepath.Join(s.Cwd, ".created")
	require.NoError(t, Declare(ctx, created))
	require.NoError(t, os.WriteFile(created, []byte("second result"), 0600))
	release()
	current = json.RawMessage(`"after second"`)
	snapshot := func() (json.RawMessage, error) { return current, nil }
	apply := func(data json.RawMessage) error { current = data; return nil }
	listing, err := s.Navigate(t.Context(), "session", Request{Action: "list"}, snapshot, apply)
	require.NoError(t, err)
	for _, choice := range listing.Points {
		_, err = s.Navigate(t.Context(), "session", Request{Action: "rewind", Target: choice.Turn}, snapshot, apply)
		require.NoError(t, err)
	}
	require.JSONEq(t, `"before first"`, string(current))
	bytes, err := os.ReadFile(file)
	require.NoError(t, err)
	require.Equal(t, []byte{0, 255, 7}, bytes)
	_, err = os.Stat(created)
	require.True(t, os.IsNotExist(err))
	for range 2 {
		_, err = s.Navigate(t.Context(), "session", Request{Action: "redo"}, snapshot, apply)
		require.NoError(t, err)
	}
	require.JSONEq(t, `"after second"`, string(current))
	bytes, err = os.ReadFile(created)
	require.NoError(t, err)
	require.Equal(t, "second result", string(bytes))
	_, err = s.Navigate(t.Context(), "session", Request{Action: "redo"}, snapshot, apply)
	require.ErrorContains(t, err, "nothing to redo")
	failed := false
	_, err = s.Navigate(t.Context(), "session", Request{Action: "rewind", Target: listing.Points[1].Turn}, snapshot, func(data json.RawMessage) error {
		if !failed {
			failed = true
			return fmt.Errorf("database failure")
		}
		return apply(data)
	})
	require.ErrorContains(t, err, "original files and conversation recovered")
	require.JSONEq(t, `"after second"`, string(current))
	bytes, err = os.ReadFile(created)
	require.NoError(t, err)
	require.Equal(t, "second result", string(bytes))
}
