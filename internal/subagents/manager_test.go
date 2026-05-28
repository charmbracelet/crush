package subagents

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestManager_AllSubagents(t *testing.T) {
	t.Parallel()

	mgr := NewManager([]*Subagent{{Name: "a"}}, nil, nil)
	t.Cleanup(mgr.Shutdown)

	got := mgr.AllSubagents()
	require.Len(t, got, 1)
	require.Equal(t, "a", got[0].Name)
}

func TestManager_ActiveSubagents(t *testing.T) {
	t.Parallel()

	mgr := NewManager(nil, []*Subagent{{Name: "b"}}, nil)
	t.Cleanup(mgr.Shutdown)

	got := mgr.ActiveSubagents()
	require.Len(t, got, 1)
	require.Equal(t, "b", got[0].Name)
}

func TestManager_States(t *testing.T) {
	t.Parallel()

	mgr := NewManager(nil, nil, []*SubagentState{{Name: "x"}})
	t.Cleanup(mgr.Shutdown)

	got := mgr.States()
	require.Len(t, got, 1)
	require.Equal(t, "x", got[0].Name)
}

func TestManager_PublishStatesUpdatesCache(t *testing.T) {
	t.Parallel()

	mgr := NewManager(nil, nil, []*SubagentState{{Name: "old"}})
	t.Cleanup(mgr.Shutdown)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ch := mgr.SubscribeEvents(ctx)

	mgr.PublishStates([]*SubagentState{{Name: "new"}})

	got := mgr.States()
	require.Len(t, got, 1)
	require.Equal(t, "new", got[0].Name)

	select {
	case ev := <-ch:
		require.Len(t, ev.Payload.States, 1)
		require.Equal(t, "new", ev.Payload.States[0].Name)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for published states event")
	}
}

func TestManager_ConcurrentWorkspacesAreIsolated(t *testing.T) {
	t.Parallel()

	mgrA := NewManager(nil, nil, nil)
	mgrB := NewManager(nil, nil, nil)
	t.Cleanup(mgrA.Shutdown)
	t.Cleanup(mgrB.Shutdown)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	chA := mgrA.SubscribeEvents(ctx)
	chB := mgrB.SubscribeEvents(ctx)

	go mgrA.PublishStates([]*SubagentState{{Name: "from-a"}})

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

// Compile-time assertion: SubscribeEvents must return the correct channel type.
var _ <-chan pubsub.Event[Event] = ((*Manager)(nil)).SubscribeEvents(context.Background())

func TestDiscoverFromConfig(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(tmp, "my-agent.md"),
		[]byte("---\nname: my-agent\ndescription: Does the thing.\n---\n\nYou are a specialist agent.\n"),
		0o644,
	))

	all, active, states := DiscoverFromConfig(DiscoveryConfig{
		SubagentsPaths: []string{tmp},
	})

	require.NotEmpty(t, all)
	require.NotEmpty(t, active)

	var found *Subagent
	for _, a := range all {
		if a.Name == "my-agent" {
			found = a
			break
		}
	}
	require.NotNil(t, found, "my-agent must appear in all")
	require.NotEmpty(t, found.Body, "DiscoverFromConfig must return Subagent.Body")

	inActive := false
	for _, a := range active {
		if a.Name == "my-agent" {
			inActive = true
			break
		}
	}
	require.True(t, inActive, "my-agent must appear in active")

	foundState := false
	for _, s := range states {
		if s.Name == "my-agent" {
			foundState = true
			require.Equal(t, StateNormal, s.State)
		}
	}
	require.True(t, foundState, "states must include my-agent with StateNormal")
}

func TestDiscoverFromConfig_DisabledFiltered(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(tmp, "that-agent.md"),
		[]byte("---\nname: that-agent\ndescription: Should be disabled.\n---\n\nBody.\n"),
		0o644,
	))

	all, active, states := DiscoverFromConfig(DiscoveryConfig{
		SubagentsPaths:    []string{tmp},
		DisabledSubagents: []string{"that-agent"},
	})

	hasInAll := false
	for _, a := range all {
		if a.Name == "that-agent" {
			hasInAll = true
		}
	}
	require.True(t, hasInAll, "DisabledSubagents must not be removed from all")

	for _, a := range active {
		require.NotEqual(t, "that-agent", a.Name, "DisabledSubagents must be removed from active")
	}

	hasInStates := false
	for _, s := range states {
		if s.Name == "that-agent" {
			hasInStates = true
		}
	}
	require.True(t, hasInStates, "states must still include disabled agent")
}

func TestDiscoverFromConfig_Resolver(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(tmp, "env-agent.md"),
		[]byte("---\nname: env-agent\ndescription: Env-resolved agent.\n---\n\nBody.\n"),
		0o644,
	))

	all, _, _ := DiscoverFromConfig(DiscoveryConfig{
		SubagentsPaths: []string{"$CUSTOM_AGENTS_DIR"},
		Resolver: func(s string) (string, error) {
			if s == "$CUSTOM_AGENTS_DIR" {
				return tmp, nil
			}
			return s, errors.New("unknown variable")
		},
	})

	found := false
	for _, a := range all {
		if a.Name == "env-agent" {
			found = true
		}
	}
	require.True(t, found, "DiscoverFromConfig must expand $VAR via Resolver")
}

func TestDiscoverFromConfig_EmptyPaths(t *testing.T) {
	t.Parallel()

	all, active, _ := DiscoverFromConfig(DiscoveryConfig{})

	require.Empty(t, all)
	require.Empty(t, active)
}
