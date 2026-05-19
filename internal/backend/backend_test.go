package backend_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/stretchr/testify/require"
)

// TestBackend_WorkspaceSkillsIsolation verifies that skill discovery
// state and SSE events are per-workspace, not process-global. This
// guards the structural change in §5 of the plan: two workspaces in the
// same backend process must not see each other's discoveries (either in
// their initial snapshot or in subsequent PublishStates events).
//
// Before that change landed, the package-level latestStates cache and
// the package-level broker meant that:
//   - workspace A's PublishStates would arrive on workspace B's SSE
//     stream, and
//   - the most recent SetLatestStates would silently overwrite the
//     other workspace's cache.
//
// This test fails on either scenario.
func TestBackend_WorkspaceSkillsIsolation(t *testing.T) {
	// Isolate all of config.Init's filesystem reads from the host. The
	// project-local .agents/skills/<name>/SKILL.md per working dir is
	// what we actually want each workspace to see; everything else
	// (global skills, XDG dirs, etc.) must be empty/deterministic.
	hostHome := t.TempDir()
	t.Setenv("HOME", hostHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(hostHome, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(hostHome, ".local", "share"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(hostHome, ".cache"))
	t.Setenv("CRUSH_SKILLS_DIR", t.TempDir())

	// Each workspace gets its own working directory containing a
	// distinct project-local skill so the discovery output differs.
	wdA := t.TempDir()
	wdB := t.TempDir()
	writeSkill(t, wdA, "wsa-only-skill", "Workspace A only skill.")
	writeSkill(t, wdB, "wsb-only-skill", "Workspace B only skill.")

	srvCfg, err := config.Init(wdA, "", false)
	require.NoError(t, err)
	b := backend.New(t.Context(), srvCfg, nil)

	wsA, _, err := b.CreateWorkspace(proto.Workspace{
		Path:    wdA,
		DataDir: filepath.Join(wdA, ".crush"),
	})
	require.NoError(t, err)
	t.Cleanup(func() { b.DeleteWorkspace(wsA.ID) })

	wsB, _, err := b.CreateWorkspace(proto.Workspace{
		Path:    wdB,
		DataDir: filepath.Join(wdB, ".crush"),
	})
	require.NoError(t, err)
	t.Cleanup(func() { b.DeleteWorkspace(wsB.ID) })

	require.NotNil(t, wsA.Skills, "workspace A must have its own skills.Manager")
	require.NotNil(t, wsB.Skills, "workspace B must have its own skills.Manager")
	require.NotSame(t, wsA.Skills, wsB.Skills, "managers must be distinct instances per workspace")

	// Initial snapshots see each workspace's own filesystem skill, and
	// neither sees the other's.
	statesA := wsA.Skills.States()
	statesB := wsB.Skills.States()
	require.True(t, containsSkillName(statesA, "wsa-only-skill"),
		"workspace A snapshot missing its own skill")
	require.False(t, containsSkillName(statesA, "wsb-only-skill"),
		"workspace A snapshot leaked workspace B's skill")
	require.True(t, containsSkillName(statesB, "wsb-only-skill"),
		"workspace B snapshot missing its own skill")
	require.False(t, containsSkillName(statesB, "wsa-only-skill"),
		"workspace B snapshot leaked workspace A's skill")

	// Subscribe to each workspace's SSE event stream.
	ctxA, cancelA := context.WithCancel(t.Context())
	t.Cleanup(cancelA)
	chA, err := b.SubscribeEvents(ctxA, wsA.ID)
	require.NoError(t, err)

	ctxB, cancelB := context.WithCancel(t.Context())
	t.Cleanup(cancelB)
	chB, err := b.SubscribeEvents(ctxB, wsB.ID)
	require.NoError(t, err)

	// Trigger a republish on workspace A only. The marker name lets us
	// distinguish this event from any incidental backend activity.
	const marker = "wsa-republish-marker"
	wsA.Skills.PublishStates([]*skills.SkillState{
		{Name: marker, State: skills.StateNormal},
	})

	// Workspace A must receive its own event.
	require.True(t,
		waitForSkillsEvent(t, chA, marker, 2*time.Second),
		"workspace A never received its own skills event")

	// Workspace B must NOT receive workspace A's event.
	require.False(t,
		waitForSkillsEvent(t, chB, marker, 250*time.Millisecond),
		"workspace B leaked workspace A's skills event")

	// And A's published states are now visible on its manager's
	// snapshot (verifies PublishStates updates the cache, not just the
	// broker).
	updatedA := wsA.Skills.States()
	require.True(t, containsSkillName(updatedA, marker),
		"PublishStates must update Manager.States()")

	// B's manager snapshot is untouched.
	require.False(t, containsSkillName(wsB.Skills.States(), marker),
		"workspace B's Manager.States() leaked workspace A's republish")
}

func writeSkill(t *testing.T, workingDir, name, desc string) {
	t.Helper()
	skillDir := filepath.Join(workingDir, ".agents", "skills", name)
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	content := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n%s\n", name, desc, desc)
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644))
}

func containsSkillName(states []*skills.SkillState, name string) bool {
	for _, s := range states {
		if s.Name == name {
			return true
		}
	}
	return false
}

// waitForSkillsEvent drains the given event channel until either a
// pubsub.Event[skills.Event] containing a state named marker arrives or
// the timeout elapses. Non-skills events on the channel are silently
// skipped — the backend fans in many event types and we only care
// about skills here.
func waitForSkillsEvent(t *testing.T, ch <-chan pubsub.Event[tea.Msg], marker string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return false
			}
			se, ok := ev.Payload.(pubsub.Event[skills.Event])
			if !ok {
				continue
			}
			if containsSkillName(se.Payload.States, marker) {
				return true
			}
		case <-deadline:
			return false
		}
	}
}
