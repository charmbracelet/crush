package skills

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestManager_NoGlobalMirrorByDefault(t *testing.T) {
	// Not parallel - touches package-level cache.
	prev := GetLatestStates()
	t.Cleanup(func() { SetLatestStates(prev) })

	SetLatestStates(nil)

	mgrA := NewManager(nil, nil, []*SkillState{{Name: "a", State: StateNormal}})
	mgrB := NewManager(nil, nil, []*SkillState{{Name: "b", State: StateNormal}})

	mgrA.PublishStates(mgrA.States())
	mgrB.PublishStates(mgrB.States())

	// Without WithGlobalMirror, the package-level cache must not be
	// touched by manager construction or PublishStates calls.
	require.Nil(t, GetLatestStates(), "package global must remain untouched")
	require.Equal(t, "a", mgrA.States()[0].Name)
	require.Equal(t, "b", mgrB.States()[0].Name)
}

func TestManager_GlobalMirror(t *testing.T) {
	// Not parallel - touches package-level cache.
	prev := GetLatestStates()
	t.Cleanup(func() { SetLatestStates(prev) })

	SetLatestStates(nil)

	mgr := NewManager(nil, nil, []*SkillState{{Name: "x", State: StateNormal}}, WithGlobalMirror())

	got := GetLatestStates()
	require.Len(t, got, 1)
	require.Equal(t, "x", got[0].Name)

	// PublishStates with mirror enabled forwards to the global cache.
	mgr.SetLatestStates([]*SkillState{{Name: "y", State: StateNormal}})
	got = GetLatestStates()
	require.Len(t, got, 1)
	require.Equal(t, "y", got[0].Name)
}

func TestManager_PublishStatesUpdatesCache(t *testing.T) {
	// Not parallel - exercises WithGlobalMirror, which touches the
	// package-level cache.
	prev := GetLatestStates()
	t.Cleanup(func() { SetLatestStates(prev) })

	SetLatestStates(nil)

	mgr := NewManager(nil, nil, []*SkillState{{Name: "old"}}, WithGlobalMirror())
	t.Cleanup(mgr.Shutdown)

	// PublishStates must update every observable snapshot, not just the
	// SSE subscribers: Manager.States() (used by workspaceToProto on
	// the backend) and skills.GetLatestStates() (read by the TUI on the
	// client process and in local mode) must reflect the new value.
	mgr.PublishStates([]*SkillState{{Name: "new"}})

	got := mgr.States()
	require.Len(t, got, 1)
	require.Equal(t, "new", got[0].Name)

	cached := GetLatestStates()
	require.Len(t, cached, 1)
	require.Equal(t, "new", cached[0].Name)
}

func TestManager_SubscribeReceivesPublishedStates(t *testing.T) {
	t.Parallel()

	mgr := NewManager(nil, nil, nil)
	t.Cleanup(mgr.Shutdown)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ch := mgr.SubscribeEvents(ctx)

	want := []*SkillState{{Name: "k", State: StateNormal}}
	go mgr.PublishStates(want)

	select {
	case ev := <-ch:
		require.Equal(t, "k", ev.Payload.States[0].Name)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for manager event")
	}
}

func TestManager_ConcurrentWorkspacesAreIsolated(t *testing.T) {
	t.Parallel()

	// Two managers without WithGlobalMirror should not see each other's
	// events; this models the multi-workspace backend.
	mgrA := NewManager(nil, nil, nil)
	mgrB := NewManager(nil, nil, nil)
	t.Cleanup(mgrA.Shutdown)
	t.Cleanup(mgrB.Shutdown)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	chA := mgrA.SubscribeEvents(ctx)
	chB := mgrB.SubscribeEvents(ctx)

	go mgrA.PublishStates([]*SkillState{{Name: "from-a"}})

	select {
	case ev := <-chA:
		require.Equal(t, "from-a", ev.Payload.States[0].Name)
	case <-time.After(2 * time.Second):
		t.Fatal("workspace A never received its own event")
	}

	select {
	case ev := <-chB:
		t.Fatalf("workspace B received workspace A's event: %v", ev)
	case <-time.After(100 * time.Millisecond):
		// Expected — B's stream is isolated.
	}
}

func TestDiscoverFromConfig(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "custom-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillDir, SkillFileName),
		[]byte("---\nname: custom-skill\ndescription: A custom skill for tests.\n---\nDo a thing.\n"),
		0o644,
	))

	allSkills, activeSkills, states := DiscoverFromConfig(DiscoveryConfig{
		SkillsPaths:    []string{tmp},
		DisabledSkills: nil,
	})

	// Builtins plus our one custom skill.
	require.NotEmpty(t, allSkills)
	require.NotEmpty(t, activeSkills)
	require.GreaterOrEqual(t, len(allSkills), 2)
	require.GreaterOrEqual(t, len(activeSkills), 2)

	// The custom skill is present with full Instructions populated, so
	// the coordinator can render system prompts without re-walking the
	// filesystem.
	var custom *Skill
	for _, s := range allSkills {
		if s.Name == "custom-skill" {
			custom = s
			break
		}
	}
	require.NotNil(t, custom)
	require.NotEmpty(t, custom.Instructions, "DiscoverFromConfig must return Skill.Instructions")

	// State snapshot includes the custom skill too.
	foundCustom := false
	for _, s := range states {
		if s.Name == "custom-skill" {
			foundCustom = true
			require.Equal(t, StateNormal, s.State)
		}
	}
	require.True(t, foundCustom, "states slice should include the custom skill")
}

func TestDiscoverFromConfig_DisabledFiltered(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "off-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillDir, SkillFileName),
		[]byte("---\nname: off-skill\ndescription: Should be filtered.\n---\nx\n"),
		0o644,
	))

	allSkills, activeSkills, states := DiscoverFromConfig(DiscoveryConfig{
		SkillsPaths:    []string{tmp},
		DisabledSkills: []string{"off-skill"},
	})

	// All discovered: yes; active: no.
	hasInAll := false
	for _, s := range allSkills {
		if s.Name == "off-skill" {
			hasInAll = true
		}
	}
	require.True(t, hasInAll, "DisabledSkills must not be removed from allSkills")

	for _, s := range activeSkills {
		require.NotEqual(t, "off-skill", s.Name, "DisabledSkills must be removed from activeSkills")
	}

	// State snapshot still carries discovered entries (UI re-applies filter).
	hasInStates := false
	for _, s := range states {
		if s.Name == "off-skill" {
			hasInStates = true
		}
	}
	require.True(t, hasInStates)
}

func TestDiscoverFromConfig_Resolver(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "envvar-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillDir, SkillFileName),
		[]byte("---\nname: envvar-skill\ndescription: Env-resolved.\n---\nx\n"),
		0o644,
	))

	allSkills, _, _ := DiscoverFromConfig(DiscoveryConfig{
		SkillsPaths: []string{"$CUSTOM_SKILLS_DIR"},
		Resolver: func(s string) (string, error) {
			if s == "$CUSTOM_SKILLS_DIR" {
				return tmp, nil
			}
			return s, errors.New("unknown")
		},
	})

	found := false
	for _, s := range allSkills {
		if s.Name == "envvar-skill" {
			found = true
		}
	}
	require.True(t, found, "DiscoverFromConfig must expand $VAR via Resolver")
}

func TestManager_Reload(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()

	// Start with no skills.
	cfg := DiscoveryConfig{
		SkillsPaths: []string{tmp},
	}
	mgr := NewManager(nil, nil, nil, WithDiscoveryConfig(cfg))
	t.Cleanup(mgr.Shutdown)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Initial state: no user skills (only builtins).
	activeBefore := mgr.ActiveSkills()
	statesBefore := mgr.States()

	// Add a skill file after construction.
	skillDir := filepath.Join(tmp, "late-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillDir, SkillFileName),
		[]byte("---\nname: late-skill\ndescription: Added after startup.\n---\nDo a thing.\n"),
		0o644,
	))

	// Reload should pick up the new skill.
	_, activeAfter, err := mgr.Reload(ctx)
	require.NoError(t, err)

	var found bool
	for _, s := range activeAfter {
		if s.Name == "late-skill" {
			found = true
		}
	}
	require.True(t, found, "Reload must discover skills added after construction")

	// Manager slices must reflect the new state.
	require.Greater(t, len(mgr.ActiveSkills()), len(activeBefore),
		"ActiveSkills must reflect reload")
	require.Greater(t, len(mgr.States()), len(statesBefore),
		"States must reflect reload")
	require.Equal(t, mgr.ActiveSkills(), activeAfter,
		"ActiveSkills must match the returned slice")
}

func TestManager_ReloadCancelled(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := DiscoveryConfig{SkillsPaths: []string{tmp}}
	mgr := NewManager(nil, nil, nil, WithDiscoveryConfig(cfg))
	t.Cleanup(mgr.Shutdown)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling

	_, _, err := mgr.Reload(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cancelled")
}
