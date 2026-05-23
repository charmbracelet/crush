package toolcoach

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSemanticEditValidation(t *testing.T) {
	t.Parallel()

	state := newSessionState()
	cfg := ToolcoachConfig{Enabled: true, MaxPatternsPerTurn: 3}

	// Simulate viewing a file and caching its content.
	state.cacheFileContent("foo.go", "package main\n\nfunc Hello() string {\n\treturn \"world\"\n}\n")

	// Editing with old_string that IS in the cache should NOT trigger edit_without_view.
	input := `{"file_path":"foo.go","old_string":"func Hello() string {","new_string":"func Hello() int {"}`
	result := state.runCoach(cfg, CoachingTutor, "edit", input)
	require.Nil(t, result)

	// Editing with old_string NOT in cache on unseen file SHOULD trigger.
	state2 := newSessionState()
	input2 := `{"file_path":"bar.go","old_string":"func Foo() {}","new_string":"func Bar() {}"}`
	result2 := state2.runCoach(cfg, CoachingTutor, "edit", input2)
	require.NotNil(t, result2)
	require.Equal(t, "edit_without_view", result2.PatternID)
}

func TestPatternOrderingByFrequency(t *testing.T) {
	t.Parallel()

	state := newSessionState()
	cfg := ToolcoachConfig{Enabled: true, MaxPatternsPerTurn: 10}

	// Fire broad_grep many times to make it the most frequent.
	// We reset the turn counter every 9 calls so the per-turn limit
	// doesn't short-circuit before we hit the reorder threshold.
	for i := 0; i < 25; i++ {
		if i%9 == 0 && i > 0 {
			state.resetTurnCounters()
		}
		state.runCoach(cfg, CoachingTutor, "grep", `{"pattern":".*"}`)
	}

	// After 20 total checks, reordering should have happened at least once.
	// broad_grep should now be first in the order.
	state.mu.RLock()
	firstID := state.patternOrder[0].ID
	state.mu.RUnlock()
	require.Equal(t, "broad_grep", firstID)
}

func TestFastPathShortCircuit(t *testing.T) {
	t.Parallel()

	state := newSessionState()
	cfg := ToolcoachConfig{Enabled: true, MaxPatternsPerTurn: 3}

	// Tools with no patterns should return nil instantly.
	tools := []string{"ls", "glob", "todos", "crush_info", "diagnostics"}
	for _, tool := range tools {
		result := state.runCoach(cfg, CoachingTutor, tool, `{}`)
		require.Nil(t, result, "tool %s should short-circuit", tool)
	}
}
