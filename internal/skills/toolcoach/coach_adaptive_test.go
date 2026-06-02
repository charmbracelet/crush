package toolcoach

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdaptSeverity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		base         string
		fired        int64
		acted        int64
		ignored      int64
		wantSeverity string
	}{
		{"insufficient_samples_hint", "hint", 4, 0, 4, "hint"},
		{"insufficient_samples_warning", "warning", 3, 3, 0, "warning"},
		{"low_effectiveness_hint_to_silent", "hint", 10, 1, 9, "silent"},
		{"low_effectiveness_warning_to_hint", "warning", 10, 1, 9, "hint"},
		{"low_effectiveness_error_to_warning", "error", 10, 1, 9, "warning"},
		{"high_effectiveness_hint_to_warning", "hint", 10, 8, 2, "warning"},
		{"high_effectiveness_warning_to_error", "warning", 10, 8, 2, "error"},
		{"neutral_hint", "hint", 10, 5, 5, "hint"},
		{"neutral_warning", "warning", 10, 5, 5, "warning"},
		{"neutral_error", "error", 10, 5, 5, "error"},
		{"high_effectiveness_error_unchanged", "error", 10, 9, 1, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := EffectivenessRecord{
				PatternID:    "test_pattern",
				TotalFired:   tt.fired,
				TotalActed:   tt.acted,
				TotalIgnored: tt.ignored,
			}
			got := adaptSeverity(tt.base, rec)
			require.Equal(t, tt.wantSeverity, got)
		})
	}
}

func TestSessionState_BuildCoachSummary(t *testing.T) {
	t.Parallel()

	state := newSessionState()
	// No metrics fired.
	require.Empty(t, state.buildCoachSummary())

	// Fire a pattern and record acted/ignored.
	state.metrics.recordPatternFire("edit_without_view", "edit", `{"file_path":"/tmp/foo.go"}`)
	state.metrics.patternActedCount["edit_without_view"] = 2
	state.metrics.patternIgnoredCount["edit_without_view"] = 1

	summary := state.buildCoachSummary()
	require.Contains(t, summary, "Edit Without View")
	require.Contains(t, summary, "fired 1 time(s)")
	require.Contains(t, summary, "acted: 2, ignored: 1")
}

func TestSessionState_EffectiveSeverity(t *testing.T) {
	t.Parallel()

	state := newSessionState()
	pat := &pattern{ID: "test_pat", Severity: "hint"}

	// No adaptive override.
	require.Equal(t, "hint", state.effectiveSeverity(pat))

	// Set adaptive override.
	state.adaptiveSeverity = map[string]string{"test_pat": "warning"}
	require.Equal(t, "warning", state.effectiveSeverity(pat))

	// Silent pattern.
	state.adaptiveSeverity["test_pat"] = "silent"
	require.Equal(t, "silent", state.effectiveSeverity(pat))
}

func TestSessionState_AdaptiveSeveritySkipsSilentPatterns(t *testing.T) {
	t.Parallel()

	state := newSessionState()
	state.adaptiveSeverity = map[string]string{"edit_without_view": "silent"}

	cfg := ToolcoachConfig{Enabled: true, MaxPatternsPerTurn: 3}
	result := state.runCoach(cfg, CoachingTutor, "edit", `{"file_path":"/tmp/new.go","old_string":"foo"}`)
	require.Nil(t, result, "silent pattern should be skipped")
}

func TestMiddleware_GetCoachSummary(t *testing.T) {
	t.Parallel()

	primary := &mockSessionAgent{}
	mw := NewMiddleware(primary, ToolcoachConfig{Enabled: true})
	require.NotNil(t, mw)
	require.Empty(t, mw.GetCoachSummary("nonexistent"))

	state := newSessionState()
	state.metrics.recordPatternFire("broad_grep", "grep", `{"pattern":"a"}`)
	mw.states.Set("sid1", state)

	summary := mw.GetCoachSummary("sid1")
	require.Contains(t, summary, "Broad Grep Pattern")
	require.Contains(t, summary, "fired 1 time(s)")
}
