package skills

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApproxTokenCount(t *testing.T) {
	t.Parallel()

	require.Equal(t, 0, ApproxTokenCount(""))
	require.Equal(t, 1, ApproxTokenCount("a"))
	require.Equal(t, 1, ApproxTokenCount("abcd"))
	require.Equal(t, 2, ApproxTokenCount("abcde"))
	// 12 chars → 3 tokens.
	require.Equal(t, 3, ApproxTokenCount("abcdefghijkl"))
}

func TestTracker_LoadedNamesAndCount(t *testing.T) {
	t.Parallel()

	active := []*Skill{{Name: "b"}, {Name: "a"}, {Name: "c"}}
	tr := NewTracker(active)
	require.Equal(t, 0, tr.LoadedCount())
	require.Empty(t, tr.LoadedNames())

	tr.MarkLoaded("b")
	tr.MarkLoaded("a")
	require.Equal(t, 2, tr.LoadedCount())
	require.Equal(t, []string{"a", "b"}, tr.LoadedNames())

	// Nil safety.
	var nilTr *Tracker
	require.Equal(t, 0, nilTr.LoadedCount())
	require.Nil(t, nilTr.LoadedNames())
}

func TestDiscoverBuiltinWithStates(t *testing.T) {
	t.Parallel()

	skills, states := DiscoverBuiltinWithStates()
	require.NotEmpty(t, skills)
	require.NotEmpty(t, states)

	// Every returned skill should have a corresponding StateNormal entry.
	ok := 0
	for _, s := range states {
		if s.State == StateNormal {
			ok++
		}
	}
	require.Equal(t, len(skills), ok)
}

func TestDiscoverWithStates_MissingPath(t *testing.T) {
	t.Parallel()

	// A clearly nonexistent path should not panic; it may log an error.
	skills, _ := DiscoverWithStates([]string{"/nonexistent/crush/skills/path"})
	require.Empty(t, skills)
}
