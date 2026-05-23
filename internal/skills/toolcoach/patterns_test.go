package toolcoach

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEditWithoutViewPattern(t *testing.T) {
	t.Parallel()

	state := newSessionState()
	cfg := ToolcoachConfig{Enabled: true, MaxPatternsPerTurn: 3}

	t.Run("detects edit on unseen file", func(t *testing.T) {
		t.Parallel()
		input := `{"file_path":"foo.go","old_string":"x","new_string":"y"}`
		result := state.runCoach(cfg, CoachingTutor, "edit", input)
		require.NotNil(t, result)
		require.Equal(t, "edit_without_view", result.PatternID)
	})

	t.Run("no fire after view", func(t *testing.T) {
		state2 := newSessionState()
		state2.recordToolCall("view", `{"file_path":"bar.go"}`)
		state2.trackFileAccess("view", `{"file_path":"bar.go"}`)
		input := `{"file_path":"bar.go","old_string":"x","new_string":"y"}`
		result := state2.runCoach(cfg, CoachingTutor, "edit", input)
		require.Nil(t, result)
	})
}

func TestRepeatedViewPattern(t *testing.T) {
	t.Parallel()

	// Override threshold for tests so we don't sleep 30s.
	oldThreshold := repeatedViewThreshold
	repeatedViewThreshold = 10 * time.Millisecond
	defer func() { repeatedViewThreshold = oldThreshold }()

	state := newSessionState()
	cfg := ToolcoachConfig{Enabled: true, MaxPatternsPerTurn: 3}

	t.Run("no fire on first view", func(t *testing.T) {
		t.Parallel()
		input := `{"file_path":"foo.go"}`
		result := state.runCoach(cfg, CoachingTutor, "view", input)
		require.Nil(t, result)
	})

	t.Run("fires on second view without edit after threshold", func(t *testing.T) {
		state2 := newSessionState()
		input := `{"file_path":"foo.go"}`
		state2.runCoach(cfg, CoachingTutor, "view", input)
		time.Sleep(20 * time.Millisecond)
		result := state2.runCoach(cfg, CoachingTutor, "view", input)
		require.NotNil(t, result)
		require.Equal(t, "repeated_view", result.PatternID)
	})

	t.Run("no fire on rapid re-view", func(t *testing.T) {
		state3 := newSessionState()
		input := `{"file_path":"foo.go"}`
		state3.runCoach(cfg, CoachingTutor, "view", input)
		result := state3.runCoach(cfg, CoachingTutor, "view", input)
		require.Nil(t, result)
	})
}

func TestBroadGrepPattern(t *testing.T) {
	t.Parallel()

	state := newSessionState()
	cfg := ToolcoachConfig{Enabled: true, MaxPatternsPerTurn: 3}

	t.Run("detects short pattern", func(t *testing.T) {
		t.Parallel()
		input := `{"pattern":"ab"}`
		result := state.runCoach(cfg, CoachingTutor, "grep", input)
		require.NotNil(t, result)
		require.Equal(t, "broad_grep", result.PatternID)
	})

	t.Run("detects wildcard only", func(t *testing.T) {
		t.Parallel()
		input := `{"pattern":".*"}`
		result := state.runCoach(cfg, CoachingTutor, "grep", input)
		require.NotNil(t, result)
	})

	t.Run("no fire on concrete pattern", func(t *testing.T) {
		t.Parallel()
		input := `{"pattern":"func Main"}`
		result := state.runCoach(cfg, CoachingTutor, "grep", input)
		require.Nil(t, result)
	})
}

func TestWriteOverExistingPattern(t *testing.T) {
	t.Parallel()

	state := newSessionState()
	cfg := ToolcoachConfig{Enabled: true, MaxPatternsPerTurn: 3}

	t.Run("no fire on new file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		input := `{"file_path":"` + filepath.Join(tmpDir, "new_file.go") + `"}`
		result := state.runCoach(cfg, CoachingTutor, "write", input)
		require.Nil(t, result)
	})

	t.Run("fires on existing file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		existing := filepath.Join(tmpDir, "existing.go")
		require.NoError(t, os.WriteFile(existing, []byte("package main"), 0o644))

		input := `{"file_path":"` + existing + `"}`
		result := state.runCoach(cfg, CoachingTutor, "write", input)
		require.NotNil(t, result)
		require.Equal(t, "write_over_existing", result.PatternID)
	})
}

func TestDestructiveBashPattern(t *testing.T) {
	t.Parallel()

	state := newSessionState()
	cfg := ToolcoachConfig{Enabled: true, MaxPatternsPerTurn: 3}

	t.Run("detects rm -rf /", func(t *testing.T) {
		t.Parallel()
		input := `{"command":"rm -rf /"}`
		result := state.runCoach(cfg, CoachingTutor, "bash", input)
		require.NotNil(t, result)
		require.Equal(t, "destructive_bash", result.PatternID)
	})

	t.Run("no fire on safe rm", func(t *testing.T) {
		t.Parallel()
		input := `{"command":"rm old_file.txt"}`
		result := state.runCoach(cfg, CoachingTutor, "bash", input)
		require.Nil(t, result)
	})
}

func TestMissingMultieditPattern(t *testing.T) {
	t.Parallel()

	cfg := ToolcoachConfig{Enabled: true, MaxPatternsPerTurn: 3}

	t.Run("no fire on first edit to viewed file", func(t *testing.T) {
		t.Parallel()
		state2 := newSessionState()
		state2.trackFileAccess("view", `{"file_path":"foo.go"}`)
		input := `{"file_path":"foo.go","old_string":"x","new_string":"y"}`
		result := state2.runCoach(cfg, CoachingTutor, "edit", input)
		require.Nil(t, result)
	})

	t.Run("fires on third consecutive edit to viewed file", func(t *testing.T) {
		state2 := newSessionState()
		state2.trackFileAccess("view", `{"file_path":"foo.go"}`)
		state2.recordToolCall("edit", `{"file_path":"foo.go","old_string":"a","new_string":"b"}`)
		state2.recordToolCall("edit", `{"file_path":"foo.go","old_string":"c","new_string":"d"}`)
		input := `{"file_path":"foo.go","old_string":"x","new_string":"y"}`
		result := state2.runCoach(cfg, CoachingTutor, "edit", input)
		require.NotNil(t, result)
		require.Equal(t, "missing_multiedit", result.PatternID)
	})
}

func TestMaxPatternsPerTurn(t *testing.T) {
	t.Parallel()

	state := newSessionState()
	cfg := ToolcoachConfig{Enabled: true, MaxPatternsPerTurn: 2}

	// First two edits to unseen files should fire.
	result1 := state.runCoach(cfg, CoachingTutor, "edit", `{"file_path":"a.go","old_string":"x","new_string":"y"}`)
	require.NotNil(t, result1)

	result2 := state.runCoach(cfg, CoachingTutor, "edit", `{"file_path":"b.go","old_string":"x","new_string":"y"}`)
	require.NotNil(t, result2)

	// Third should be suppressed.
	result3 := state.runCoach(cfg, CoachingTutor, "edit", `{"file_path":"c.go","old_string":"x","new_string":"y"}`)
	require.Nil(t, result3)
}
