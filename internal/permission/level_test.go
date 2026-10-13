package permission

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// togglingHooks records the runtime auto-mode state a level change
// forwards to installed hooks.
type togglingHooks struct {
	enabled []bool
}

func (h *togglingHooks) PrePermission(context.Context, PermissionRequest) PreHookResult {
	return PreHookResult{}
}

func (h *togglingHooks) PermissionDenied(context.Context, PermissionRequest) {}

func (h *togglingHooks) SetAutoModeEnabled(enabled bool) {
	h.enabled = append(h.enabled, enabled)
}

func TestLevelTransitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		level      Level
		wantSkip   bool
		wantAuto   bool
		wantString string
	}{
		{name: "prompt asks", level: LevelPrompt, wantString: "prompt"},
		{name: "auto arms the classifier", level: LevelAuto, wantAuto: true, wantString: "auto"},
		{name: "bypass skips", level: LevelBypass, wantSkip: true, wantString: "bypass"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := NewPermissionService(t.TempDir(), false, nil)
			svc.SetLevel(tc.level)

			assert.Equal(t, tc.level, svc.Level())
			assert.Equal(t, tc.wantSkip, svc.SkipRequests())
			assert.Equal(t, tc.wantAuto, svc.AutoMode())
			assert.Equal(t, tc.wantString, tc.level.String())
		})
	}
}

// TestLevelAfterBypassForgetsAuto pins the normalization: bypass does not
// pause the classifier, it replaces it, so returning to prompting cannot
// silently resume a level the user already left.
func TestLevelAfterBypassForgetsAuto(t *testing.T) {
	t.Parallel()

	svc := NewPermissionService(t.TempDir(), false, nil)
	svc.SetLevel(LevelAuto)
	svc.SetLevel(LevelBypass)
	assert.False(t, svc.AutoMode(), "bypass must not leave the classifier armed")

	svc.SetLevel(LevelPrompt)
	assert.Equal(t, LevelPrompt, svc.Level())
	assert.False(t, svc.AutoMode())
}

// TestLevelMatchesRequestOrder keeps the derived level honest: Level() must
// report what Request actually does, and Request short-circuits on skip
// before it ever consults the classifier hooks.
func TestLevelMatchesRequestOrder(t *testing.T) {
	t.Parallel()

	svc := NewPermissionService(t.TempDir(), false, nil)
	svc.SetAutoMode(true)
	svc.SetSkipRequests(true)

	granted, err := svc.Request(context.Background(), CreatePermissionRequest{
		SessionID:  "session",
		ToolCallID: "call",
		ToolName:   "bash",
	})
	require.NoError(t, err)
	assert.True(t, granted, "skip approves without asking")
	assert.Equal(t, LevelBypass, svc.Level())
}

func TestSetLevelForwardsToHooks(t *testing.T) {
	t.Parallel()

	hooks := &togglingHooks{}
	svc := NewPermissionService(t.TempDir(), false, nil)
	svc.SetPermissionHooks(hooks)
	svc.SetLevel(LevelAuto)
	svc.SetLevel(LevelPrompt)

	assert.Equal(t, []bool{true, false}, hooks.enabled)
}

func TestParseLevel(t *testing.T) {
	t.Parallel()

	for _, level := range []Level{LevelPrompt, LevelAuto, LevelBypass} {
		parsed, ok := ParseLevel(level.String())
		assert.True(t, ok, level.String())
		assert.Equal(t, level, parsed)
	}

	for _, name := range []string{"", "yolo", "AUTO", "prompt "} {
		_, ok := ParseLevel(name)
		assert.False(t, ok, "must reject %q rather than accept the zero value", name)
	}
}
