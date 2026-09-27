package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/router"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestRouterInfoEmptyWhenNeverConsulted pins the sidebar convention shared
// with the other optional sections (files/lsp/mcp/skills): a router that
// has never been consulted for this session renders nothing, rather than
// an empty "Router" heading.
func TestRouterInfoEmptyWhenNeverConsulted(t *testing.T) {
	m := newPrismTestUI()
	require.Empty(t, m.routerInfo(40))
}

// TestRouterInfoShowsConsultingWithConfiguredModel proves the live
// "consulting" indicator names the model the router call was actually
// sent to, so the user can tell which router model is configured while
// waiting on it — not just that something is happening.
func TestRouterInfoShowsConsultingWithConfiguredModel(t *testing.T) {
	m := newPrismTestUI()
	m.routerQuerying = true
	m.routerQueryingModel = "typesafe/jev-latest"

	got := ansi.Strip(m.routerInfo(60))
	require.Contains(t, got, "consulting typesafe/jev-latest")
}

// TestRouterInfoShowsConsultingWithoutModelName covers the case where the
// router config has no model set at all (defaultRouterModel resolves it
// downstream in the coordinator, never here), so the indicator degrades to
// a plain "Consulting…" rather than showing an empty name.
func TestRouterInfoShowsConsultingWithoutModelName(t *testing.T) {
	m := newPrismTestUI()
	m.routerQuerying = true
	m.routerQueryingModel = ""

	got := ansi.Strip(m.routerInfo(40))
	require.Contains(t, got, "consulting…")
}

// TestRouterInfoShowsLastDecisionWhenNotQuerying proves that once a router
// call completes, the sidebar shows the applied model, effort, and
// confidence — more detail than the compact status-bar badge, since the
// sidebar has the room for it.
func TestRouterInfoShowsLastDecisionWhenNotQuerying(t *testing.T) {
	m := newPrismTestUI()
	m.status.SetRouterDecision(router.Decision{
		ModelID:         "anthropic/claude-haiku-4",
		ReasoningEffort: "high",
		Confidence:      0.92,
	}, true)

	got := ansi.Strip(m.routerInfo(40))
	require.Contains(t, got, "claude-haiku-4")
	require.Contains(t, got, "high")
	require.Contains(t, got, "92%")
}

// TestRouterInfoShowsClassifierModelAlongsideDecision proves the router
// line names which model actually produced the decision (the "consulting"
// indicator shows this while a call is in flight; this pins that it's
// still shown once the decision lands), since without model_pool
// configured the decision's own ModelID is empty and there is otherwise
// no way to tell which model classified the message.
func TestRouterInfoShowsClassifierModelAlongsideDecision(t *testing.T) {
	m := newPrismTestUI()
	m.status.SetRouterDecision(router.Decision{ReasoningEffort: "low", Confidence: 0.98}, true)
	m.routerModel = "~typesafe/jev-latest"

	got := ansi.Strip(m.routerInfo(60))
	require.Contains(t, got, "low")
	require.Contains(t, got, "98%")
	require.Contains(t, got, "jev-latest")
}

// TestRouterSavingsInfoShowsSessionTotalSeparateFromDecisionLine proves
// the cumulative session total renders as its own line, not mixed into
// the per-message decision line — and in both directions: a positive
// total reads as savings, a negative one (the router's choices cost
// more than the session's default model would have) reads as a
// negative dollar figure rather than a confusing bare negative.
func TestRouterSavingsInfoShowsSessionTotalSeparateFromDecisionLine(t *testing.T) {
	m := newPrismTestUI()
	m.status.SetRouterDecision(router.Decision{ReasoningEffort: "low", Confidence: 0.9}, true)
	m.routerSavings = 0.0034

	decisionLine := ansi.Strip(m.routerInfo(60))
	require.NotContains(t, decisionLine, "0.0034", "savings must not be mixed into the decision line")

	savingsLine := ansi.Strip(m.routerSavingsInfo(60))
	require.Contains(t, savingsLine, "Router savings (session): $0.0034")

	m.routerSavings = -0.0012
	savingsLine = ansi.Strip(m.routerSavingsInfo(60))
	require.Contains(t, savingsLine, "Router savings (session): -$0.0012")
}

// TestRouterSavingsInfoEmptyWhenNothingSavedYet proves the line is
// omitted entirely (not shown as "$0.0000") when the router has never
// affected a message's cost this session — matching the convention
// every other optional sidebar line already follows.
func TestRouterSavingsInfoEmptyWhenNothingSavedYet(t *testing.T) {
	m := newPrismTestUI()
	require.Empty(t, m.routerSavingsInfo(60))
}

// TestRouterInfoQueryingTakesPriorityOverStaleDecision proves the live
// "consulting" state always wins while a call is in flight, even when a
// decision from an earlier message is still memoized — the sidebar must
// never show a stale result while a newer one is being fetched.
func TestRouterInfoQueryingTakesPriorityOverStaleDecision(t *testing.T) {
	m := newPrismTestUI()
	m.status.SetRouterDecision(router.Decision{ReasoningEffort: "low", Confidence: 0.8}, true)
	m.routerQuerying = true
	m.routerQueryingModel = "typesafe/jev-latest"

	got := ansi.Strip(m.routerInfo(40))
	require.Contains(t, got, "consulting")
	require.NotContains(t, got, "low")
}

// TestRouterInfoShowsErrorWhenRouterIsBroken proves a router failure is
// visible in the sidebar instead of just rendering nothing — the prior
// behavior, before LastRouterError existed, made "the router silently
// isn't working" indistinguishable from "the router was never
// consulted this session".
func TestRouterInfoShowsErrorWhenRouterIsBroken(t *testing.T) {
	m := newPrismTestUI()
	m.routerError = "router call failed: router: request failed: dial tcp: connection refused"

	got := ansi.Strip(m.routerInfo(60))
	require.Contains(t, got, "error")
	require.Contains(t, got, "connection refused")
}

// TestRouterInfoDecisionTakesPriorityOverStaleError proves a completed
// decision (the router just started working again) is shown instead of
// a leftover error field — mirrors how run() clears the error on a
// successful call, so the UI-side rendering priority matches the
// backing state's own invariant instead of silently relying on it.
func TestRouterInfoDecisionTakesPriorityOverStaleError(t *testing.T) {
	m := newPrismTestUI()
	m.status.SetRouterDecision(router.Decision{ReasoningEffort: "low", Confidence: 0.9}, true)
	m.routerError = "stale error that should not be shown"

	got := ansi.Strip(m.routerInfo(60))
	require.Contains(t, got, "low")
	require.NotContains(t, got, "stale error")
}
