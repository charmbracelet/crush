package skills

import (
	"os"
	"path/filepath"
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

func TestGetLatestStates(t *testing.T) {
	// Not parallel - manipulates package-level cache.
	prev := GetLatestStates()
	t.Cleanup(func() { SetLatestStates(prev) })

	SetLatestStates(nil)
	require.Nil(t, GetLatestStates())

	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillDir, SkillFileName),
		[]byte("---\nname: my-skill\ndescription: A test skill.\n---\nInstructions.\n"),
		0o644,
	))

	_, states := DiscoverWithStates([]string{dir})
	SetLatestStates(states)

	got := GetLatestStates()
	require.Len(t, got, 1)
	require.Equal(t, "my-skill", got[0].Name)
}

func TestGetLatestStates_Isolation(t *testing.T) {
	// Not parallel - manipulates package-level cache.
	prev := GetLatestStates()
	t.Cleanup(func() { SetLatestStates(prev) })

	initial := []*SkillState{{Name: "test"}}
	SetLatestStates(initial)

	got := GetLatestStates()
	got[0].Name = "corrupted"

	check := GetLatestStates()
	require.Equal(t, "test", check[0].Name, "Cache should be isolated from caller mutations")
}
