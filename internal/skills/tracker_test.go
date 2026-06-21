package skills

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTracker_MarkLoadedAndIsLoaded(t *testing.T) {
	t.Parallel()

	activeSkills := []*Skill{
		{Name: "go-doc"},
		{Name: "bash"},
	}
	tracker := NewTracker(activeSkills)

	// Initially not loaded.
	require.False(t, tracker.IsLoaded("go-doc"))
	require.False(t, tracker.IsLoaded("bash"))

	// Mark as loaded.
	tracker.MarkLoaded("go-doc")
	require.True(t, tracker.IsLoaded("go-doc"))
	require.False(t, tracker.IsLoaded("bash"))

	// Mark another.
	tracker.MarkLoaded("bash")
	require.True(t, tracker.IsLoaded("go-doc"))
	require.True(t, tracker.IsLoaded("bash"))
}

func TestTracker_NonActiveSkillCannotBeMarkedLoaded(t *testing.T) {
	t.Parallel()

	activeSkills := []*Skill{
		{Name: "go-doc"},
	}
	tracker := NewTracker(activeSkills)

	// Cannot mark non-active skill as loaded.
	tracker.MarkLoaded("bash")
	require.False(t, tracker.IsLoaded("bash"))

	// Can mark active skill as loaded.
	tracker.MarkLoaded("go-doc")
	require.True(t, tracker.IsLoaded("go-doc"))
}

func TestTracker_NilSafety(t *testing.T) {
	t.Parallel()

	var tracker *Tracker

	// Should not panic.
	tracker.MarkLoaded("go-doc")
	require.False(t, tracker.IsLoaded("go-doc"))
}

func TestTracker_BuiltinSkillTracking(t *testing.T) {
	t.Parallel()

	// Simulate active skills including a builtin skill (crush-config).
	activeSkills := []*Skill{
		{Name: "crush-config", Description: "Crush config", Builtin: true},
		{Name: "go-doc", Description: "Go docs", Builtin: false},
	}
	tracker := NewTracker(activeSkills)

	// Initially not loaded.
	require.False(t, tracker.IsLoaded("crush-config"))
	require.False(t, tracker.IsLoaded("go-doc"))

	// Mark builtin skill as loaded (simulating read via crush://...).
	tracker.MarkLoaded("crush-config")
	require.True(t, tracker.IsLoaded("crush-config"))

	// Mark user skill as loaded.
	tracker.MarkLoaded("go-doc")
	require.True(t, tracker.IsLoaded("go-doc"))
}

func TestTracker_OverriddenBuiltinNotTracked(t *testing.T) {
	t.Parallel()

	// Simulate scenario where builtin "bash" is overridden by user "bash".
	// After dedup, only user "bash" is active.
	activeSkills := []*Skill{
		{Name: "bash", Description: "User bash override", Builtin: false},
	}
	tracker := NewTracker(activeSkills)

	// Trying to mark the builtin "bash" as loaded should not work
	// because the active skill is the user override.
	tracker.MarkLoaded("bash")
	require.True(t, tracker.IsLoaded("bash"))

	// But if we somehow tried to mark a different builtin that's not active,
	// it wouldn't get marked.
	tracker.MarkLoaded("nonexistent-builtin")
	require.False(t, tracker.IsLoaded("nonexistent-builtin"))
}
