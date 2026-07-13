package acp

import (
	"testing"

	sdk "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/agent"
)

func TestToolKindFromName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want sdk.ToolKind
	}{
		{name: "view", want: sdk.ToolKindRead},
		{name: "zed_view", want: sdk.ToolKindRead},
		{name: "edit", want: sdk.ToolKindEdit},
		{name: "zed_write", want: sdk.ToolKindEdit},
		{name: "bash", want: sdk.ToolKindExecute},
		{name: "zed_bash", want: sdk.ToolKindExecute},
		{name: "fetch", want: sdk.ToolKindFetch},
		{name: "todos", want: sdk.ToolKindThink},
		{name: "agent", want: sdk.ToolKindSwitchMode},
		{name: "unknown", want: sdk.ToolKindOther},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, toolKindFromName(tt.name))
		})
	}
}

func TestToolLocationsFromInput(t *testing.T) {
	t.Parallel()

	t.Run("file path with offset", func(t *testing.T) {
		t.Parallel()

		got := toolLocationsFromInput(`{"file_path":"/tmp/a.go","offset":41}`)
		require.Len(t, got, 1)
		require.Equal(t, "/tmp/a.go", got[0].Path)
		require.NotNil(t, got[0].Line)
		require.Equal(t, 41, *got[0].Line)
	})

	t.Run("path fallback", func(t *testing.T) {
		t.Parallel()

		got := toolLocationsFromInput(`{"path":"/tmp/b.go"}`)
		require.Len(t, got, 1)
		require.Equal(t, "/tmp/b.go", got[0].Path)
		require.Nil(t, got[0].Line)
	})

	t.Run("invalid input", func(t *testing.T) {
		t.Parallel()

		require.Nil(t, toolLocationsFromInput(`not json`))
		require.Nil(t, toolLocationsFromInput(`{}`))
		require.Nil(t, toolLocationsFromInput(""))
	})
}

func TestToolCommandFromInput(t *testing.T) {
	t.Parallel()

	require.Equal(t, "go test ./...", toolCommandFromInput(`{"command":"go test ./..."}`))
	require.Empty(t, toolCommandFromInput(`{"path":"/tmp/a.go"}`))
	require.Empty(t, toolCommandFromInput(`not json`))
	require.Empty(t, toolCommandFromInput(""))
}

func TestBuildVisualMeta(t *testing.T) {
	t.Parallel()

	line := 7
	a := NewAdapter(nil, nil, nil)

	got := a.buildVisualMeta(sdk.ToolKindEdit, []sdk.ToolCallLocation{
		{Path: "/tmp/main.go", Line: &line},
	}, "zed_write", `{"file_path":"/tmp/main.go","offset":7}`)

	cmd, ok := got["_zed_visual_command"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "open_file", cmd["command"])
	params, ok := cmd["params"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "/tmp/main.go", params["path"])
	require.Equal(t, line, params["line"])

	got = a.buildVisualMeta(sdk.ToolKindExecute, nil, "bash", `{"command":"go test ./..."}`)
	cmd, ok = got["_zed_visual_command"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "run_in_terminal", cmd["command"])
	params, ok = cmd["params"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "go test ./...", params["command"])

	require.Nil(t, a.buildVisualMeta(sdk.ToolKindThink, []sdk.ToolCallLocation{
		{Path: "/tmp/main.go"},
	}, "todos", `{}`))
	require.Nil(t, a.buildVisualMeta(sdk.ToolKindRead, nil, "view", `{}`))
}

func TestPlanMapping(t *testing.T) {
	t.Parallel()

	require.Equal(t, sdk.PlanEntryPriorityHigh, planPriority(agent.PlanPriorityHigh))
	require.Equal(t, sdk.PlanEntryPriorityMedium, planPriority(agent.PlanPriorityMedium))
	require.Equal(t, sdk.PlanEntryPriorityLow, planPriority(agent.PlanPriorityLow))
	require.Equal(t, sdk.PlanEntryPriorityMedium, planPriority(agent.PlanPriority(99)))

	require.Equal(t, sdk.PlanEntryStatusPending, planStatus(agent.PlanStatusPending))
	require.Equal(t, sdk.PlanEntryStatusInProgress, planStatus(agent.PlanStatusInProgress))
	require.Equal(t, sdk.PlanEntryStatusCompleted, planStatus(agent.PlanStatusCompleted))
	require.Equal(t, sdk.PlanEntryStatusPending, planStatus(agent.PlanStatus(99)))
}

func TestUsageCost(t *testing.T) {
	t.Parallel()

	require.Nil(t, usageCost(0))

	got := usageCost(0.42)
	require.NotNil(t, got)
	require.Equal(t, 0.42, got.Amount)
	require.Equal(t, "USD", got.Currency)
}
