package toolcoach

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// === Progressive Coaching Tests ===

func TestSeverityAllowedByIntensity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		severity  string
		intensity CoachingIntensity
		want      bool
	}{
		{"tutor_allows_hint", SeverityHint, CoachingTutor, true},
		{"tutor_allows_warning", SeverityWarning, CoachingTutor, true},
		{"tutor_allows_critical", SeverityCritical, CoachingTutor, true},
		{"balanced_blocks_hint", SeverityHint, CoachingBalanced, false},
		{"balanced_allows_warning", SeverityWarning, CoachingBalanced, true},
		{"balanced_allows_critical", SeverityCritical, CoachingBalanced, true},
		{"minimal_blocks_hint", SeverityHint, CoachingMinimal, false},
		{"minimal_blocks_warning", SeverityWarning, CoachingMinimal, false},
		{"minimal_allows_critical", SeverityCritical, CoachingMinimal, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := severityAllowedByIntensity(tt.severity, tt.intensity)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestRunCoach_RespectsIntensity(t *testing.T) {
	t.Parallel()

	// Hint-level pattern (edit_without_view) should be suppressed under balanced.
	state := newSessionState()
	cfg := ToolcoachConfig{Enabled: true, MaxPatternsPerTurn: 3}

	result := state.runCoach(cfg, CoachingBalanced, "edit", `{"file_path":"/tmp/new.go","old_string":"foo"}`)
	require.Nil(t, result, "hint pattern should be suppressed under balanced intensity")

	// Critical-level pattern should still fire under balanced.
	result = state.runCoach(cfg, CoachingBalanced, "bash", `{"command":"rm -rf /"}`)
	require.NotNil(t, result, "critical pattern should fire under balanced intensity")
	require.Equal(t, SeverityCritical, result.Severity)
}

func TestMiddleware_EffectiveIntensity_AutoSwitch(t *testing.T) {
	t.Parallel()

	mw := NewMiddleware(&mockSessionAgent{}, ToolcoachConfig{
		Enabled:           true,
		Intensity:         CoachingTutor,
		AutoRetrySessions: 3,
	})
	require.NotNil(t, mw)

	// Before any sessions, intensity should be tutor.
	require.Equal(t, CoachingTutor, mw.effectiveIntensity())

	// After 2 unique sessions, still tutor.
	mw.sessionSeen["sid1"] = struct{}{}
	mw.sessionSeen["sid2"] = struct{}{}
	require.Equal(t, CoachingTutor, mw.effectiveIntensity())

	// After 3 unique sessions, auto-switch to balanced.
	mw.sessionSeen["sid3"] = struct{}{}
	require.Equal(t, CoachingBalanced, mw.effectiveIntensity())

	// If explicitly set to minimal, never auto-switch.
	mw2 := NewMiddleware(&mockSessionAgent{}, ToolcoachConfig{
		Enabled:           true,
		Intensity:         CoachingMinimal,
		AutoRetrySessions: 3,
	})
	mw2.sessionSeen["sid1"] = struct{}{}
	mw2.sessionSeen["sid2"] = struct{}{}
	mw2.sessionSeen["sid3"] = struct{}{}
	require.Equal(t, CoachingMinimal, mw2.effectiveIntensity())
}

// === Enhanced Success Tracking Tests ===

func TestBroadGrepValidator(t *testing.T) {
	t.Parallel()

	pat := patternByID("broad_grep")
	require.NotNil(t, pat)
	require.NotNil(t, pat.Validate)

	// Success: next grep has a longer, concrete pattern.
	state := newSessionState()
	ok := pat.Validate(state, "grep", `{"pattern":"foobar"}`, pendingTip{})
	require.True(t, ok)

	// Failure: next grep is still too short.
	ok = pat.Validate(state, "grep", `{"pattern":"ab"}`, pendingTip{})
	require.False(t, ok)

	// Failure: not a grep tool.
	ok = pat.Validate(state, "view", `{"file_path":"x.go"}`, pendingTip{})
	require.False(t, ok)
}

func TestCheckPendingTips_WithValidator(t *testing.T) {
	t.Parallel()

	m := newCoachMetrics()

	// Fire broad_grep tip.
	m.recordPatternFire("broad_grep", "grep", `{"pattern":"a"}`)
	require.Len(t, m.pendingTips, 1)

	// Default validator (nil) would count "grep" with any pattern as acted.
	// With the broad_grep validator, only a concrete pattern counts.
	validator := func(toolName, input string, tip pendingTip) bool {
		pat := patternByID(tip.patternID)
		if pat != nil && pat.Validate != nil {
			return pat.Validate(nil, toolName, input, tip)
		}
		return toolName == tip.expectedTool
	}

	// Agent uses grep but still with a short pattern — not acted.
	m.checkPendingTips("grep", `{"pattern":"b"}`, validator)
	require.Len(t, m.pendingTips, 1)
	require.Equal(t, uint64(0), m.patternActedCount["broad_grep"])

	// Agent uses grep with a concrete pattern — acted.
	m.checkPendingTips("grep", `{"pattern":"foobar"}`, validator)
	require.Equal(t, uint64(1), m.patternActedCount["broad_grep"])
	require.Len(t, m.pendingTips, 0)
}

// === Guided Retry Tests ===

func TestReplaceJSONField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		key      string
		newVal   string
		expected string
	}{
		{"simple", `{"pattern":"a"}`, "pattern", `\b\w+\b`, `{"pattern":"\b\w+\b"}`},
		{"with_spaces", `{"pattern" : "a"}`, "pattern", `\b\w+\b`, `{"pattern" : "\b\w+\b"}`},
		{"multiple_fields", `{"path":"src","pattern":"a"}`, "pattern", `\b\w+\b`, `{"path":"src","pattern":"\b\w+\b"}`},
		{"missing_key", `{"path":"src"}`, "pattern", `x`, ""},
		{"escaped_quote_in_value", `{"pattern":"a\"b"}`, "pattern", `fixed`, `{"pattern":"fixed"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := replaceJSONField(tt.input, tt.key, tt.newVal)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestBroadGrepFixInput(t *testing.T) {
	t.Parallel()

	pat := patternByID("broad_grep")
	require.NotNil(t, pat)
	require.NotNil(t, pat.FixInput)

	state := newSessionState()

	// .* should become word-boundary pattern.
	fixed := pat.FixInput(state, "grep", `{"pattern":".*"}`)
	require.Equal(t, `{"pattern":"\b\w+\b"}`, fixed)

	// Single char should get word boundary.
	fixed = pat.FixInput(state, "grep", `{"pattern":"a"}`)
	require.Equal(t, `{"pattern":"\ba\b"}`, fixed)

	// Already good pattern should not be fixed.
	fixed = pat.FixInput(state, "grep", `{"pattern":"foobar"}`)
	require.Empty(t, fixed)
}

func TestCoachedTool_GuidedRetry_Capped(t *testing.T) {
	t.Parallel()

	inner := &mockAgentTool{}
	state := newSessionState()
	cfg := ToolcoachConfig{Enabled: true, AutoRetry: true, MaxPatternsPerTurn: 3}

	ct := newCoachedTool(inner, state, "sid", nil, cfg, CoachingTutor).(*coachedTool)

	// First call with a broad pattern triggers retry.
	inner.respond = func(call fantasy.ToolCall) fantasy.ToolResponse {
		return fantasy.ToolResponse{Content: "retry-result"}
	}

	resp, err := ct.Run(context.Background(), fantasy.ToolCall{
		Name:  "grep",
		Input: `{"pattern":".*"}`,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Contains(t, resp.Content, "retry-result")

	// Retry flag should now be set.
	require.True(t, state.hasRetriedThisTurn())
	require.Equal(t, 1, inner.callCount)

	// Second call in same turn should NOT retry again (cap = 1).
	inner.callCount = 0
	inner.respond = func(call fantasy.ToolCall) fantasy.ToolResponse {
		return fantasy.ToolResponse{Content: "original-result"}
	}

	resp2, err := ct.Run(context.Background(), fantasy.ToolCall{
		Name:  "grep",
		Input: `{"pattern":".*"}`,
	})
	require.NoError(t, err)
	require.NotNil(t, resp2)
	require.Equal(t, 1, inner.callCount, "should not retry more than once per turn")
	require.Contains(t, resp2.Content, "original-result")
}

func TestCoachedTool_GuidedRetry_DisabledByDefault(t *testing.T) {
	t.Parallel()

	inner := &mockAgentTool{}
	state := newSessionState()
	cfg := ToolcoachConfig{Enabled: true, AutoRetry: false} // default

	ct := newCoachedTool(inner, state, "sid", nil, cfg, CoachingTutor).(*coachedTool)

	inner.respond = func(call fantasy.ToolCall) fantasy.ToolResponse {
		return fantasy.ToolResponse{Content: "result"}
	}

	resp, err := ct.Run(context.Background(), fantasy.ToolCall{
		Name:  "grep",
		Input: `{"pattern":".*"}`,
	})
	require.NoError(t, err)
	require.Equal(t, 1, inner.callCount, "should not retry when AutoRetry is false")
	require.NotNil(t, resp)
}

// mockMessageService is a minimal message.Service for testing indicator
// deduplication.
type mockMessageService struct {
	mu           sync.Mutex
	messages     []mockMsg
	createCount  int
	deleteCount  int
	deleteTarget string
}

type mockMsg struct {
	id    string
	label string
}

func (m *mockMessageService) Create(ctx context.Context, sessionID string, params message.CreateMessageParams) (message.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createCount++
	msg := message.Message{ID: fmt.Sprintf("msg-%d", m.createCount), SpinnerLabel: params.SpinnerLabel}
	m.messages = append(m.messages, mockMsg{id: msg.ID, label: params.SpinnerLabel})
	return msg, nil
}
func (m *mockMessageService) Update(ctx context.Context, msg message.Message) error                { return nil }
func (m *mockMessageService) Get(ctx context.Context, id string) (message.Message, error)          { return message.Message{}, nil }
func (m *mockMessageService) List(ctx context.Context, sessionID string) ([]message.Message, error) { return nil, nil }
func (m *mockMessageService) ListUserMessages(ctx context.Context, sessionID string) ([]message.Message, error) {
	return nil, nil
}
func (m *mockMessageService) ListAllUserMessages(ctx context.Context) ([]message.Message, error) { return nil, nil }
func (m *mockMessageService) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteCount++
	m.deleteTarget = id
	return nil
}
func (m *mockMessageService) DeleteSessionMessages(ctx context.Context, sessionID string) error { return nil }
func (m *mockMessageService) Flush(ctx context.Context, id string) error                         { return nil }
func (m *mockMessageService) FlushAll(ctx context.Context) error                                  { return nil }
func (m *mockMessageService) Subscribe(ctx context.Context) <-chan pubsub.Event[message.Message] { return nil }

func (m *mockMessageService) counts() (createCount, deleteCount int, deleteTarget string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.createCount, m.deleteCount, m.deleteTarget
}

func TestShowCoachIndicator_Deduplication(t *testing.T) {
	t.Parallel()

	state := newSessionState()
	msgs := &mockMessageService{}
	cfg := ToolcoachConfig{Enabled: true}
	ct := newCoachedTool(&mockAgentTool{}, state, "sid", msgs, cfg, CoachingTutor).(*coachedTool)

	// Fire two indicators in rapid succession.
	ct.showCoachIndicator("sid", &coachResult{PatternID: "p1", DelayMicros: 1})
	ct.showCoachIndicator("sid", &coachResult{PatternID: "p2", DelayMicros: 2})

	// Give goroutines time to run.
	time.Sleep(100 * time.Millisecond)

	// Both should have been created.
	cc, dc, dt := msgs.counts()
	require.Equal(t, 2, cc)

	// The first indicator should have been deleted immediately (deduplication).
	require.Equal(t, 1, dc)
	require.Equal(t, "msg-1", dt)

	// The second indicator should be the active one.
	require.Equal(t, "msg-2", state.getActiveIndicatorID())
}

func TestShowCoachIndicator_CleanupOnContextDone(t *testing.T) {
	t.Parallel()

	state := newSessionState()
	msgs := &mockMessageService{}
	cfg := ToolcoachConfig{Enabled: true}
	ct := newCoachedTool(&mockAgentTool{}, state, "sid", msgs, cfg, CoachingTutor).(*coachedTool)

	// Show an indicator.
	ct.showCoachIndicator("sid", &coachResult{PatternID: "p1", DelayMicros: 1})
	time.Sleep(50 * time.Millisecond)

	// Active indicator should be tracked.
	require.Equal(t, "msg-1", state.getActiveIndicatorID())

	// Simulate deletion (as would happen after the timer).
	// In the real flow, the goroutine deletes it after 800ms.
	// Here we just verify the tracking works.
	state.clearActiveIndicator("msg-1")
	require.Empty(t, state.activeIndicatorID)
}

// mockAgentTool is a test double for fantasy.AgentTool.
type mockAgentTool struct {
	callCount int
	respond   func(call fantasy.ToolCall) fantasy.ToolResponse
}

func (m *mockAgentTool) Info() fantasy.ToolInfo                          { return fantasy.ToolInfo{} }
func (m *mockAgentTool) ProviderOptions() fantasy.ProviderOptions        { return fantasy.ProviderOptions{} }
func (m *mockAgentTool) SetProviderOptions(opts fantasy.ProviderOptions) {}
func (m *mockAgentTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	m.callCount++
	if m.respond != nil {
		return m.respond(call), nil
	}
	return fantasy.ToolResponse{}, nil
}
