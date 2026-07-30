package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

const (
	modelA    = "grok-4.5"               // parent's paid model
	modelB    = "claude-opus-5"          // grandchild's paid model
	modelFree = "deepseek-v4-flash-free" // child's free model
	provA     = "xai"
	provB     = "anthropic"
	provFree  = "opencode-zen"
)

// costStore is a real session + message store backed by a temporary SQLite
// database. The by-model breakdown is exercised through the same services the
// command uses, so the Finish usage it reads has actually round-tripped
// through SQL rather than being handed over in memory.
type costStore struct {
	sessions session.Service
	messages message.Service
}

func newCostStore(t *testing.T) costStore {
	t.Helper()
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	q := db.New(conn)
	return costStore{
		sessions: session.NewService(q, conn),
		messages: message.NewService(q),
	}
}

// spend appends an assistant message to sessionID carrying the given step
// usage, exactly as agent.OnStepFinish does, and adds the cost to the session
// row the way updateSessionUsage does.
func (c costStore) spend(t *testing.T, sessionID, provider, model string, prompt, completion int64, cost float64) {
	t.Helper()
	msg, err := c.messages.Create(t.Context(), sessionID, message.CreateMessageParams{
		Role:     message.Assistant,
		Model:    model,
		Provider: provider,
	})
	require.NoError(t, err)
	msg.AddFinishWithUsage(message.FinishReasonEndTurn, "", "", prompt, completion, cost)
	require.NoError(t, c.messages.Update(t.Context(), msg))
	require.NoError(t, c.messages.Flush(t.Context(), msg.ID))

	sess, err := c.sessions.Get(t.Context(), sessionID)
	require.NoError(t, err)
	sess.Cost += cost
	sess.PromptTokens = prompt
	sess.CompletionTokens = completion
	_, err = c.sessions.Save(t.Context(), sess)
	require.NoError(t, err)
}

// propagate mirrors coordinator.updateParentSessionCost: the child's WHOLE
// recorded cost (its own plus anything already propagated into it) is added to
// the parent. This is the write-side behaviour that makes the parent's `cost`
// column say nothing about which model was billed.
func (c costStore) propagate(t *testing.T, childID, parentID string) {
	t.Helper()
	child, err := c.sessions.Get(t.Context(), childID)
	require.NoError(t, err)
	parent, err := c.sessions.Get(t.Context(), parentID)
	require.NoError(t, err)
	parent.Cost += child.Cost
	_, err = c.sessions.Save(t.Context(), parent)
	require.NoError(t, err)
}

func (c costStore) report(t *testing.T, sess session.Session) sessionCostReport {
	t.Helper()
	report, err := sessionByModelReport(t.Context(), c.sessions, c.messages, sess)
	require.NoError(t, err)
	return report
}

func rowFor(t *testing.T, report sessionCostReport, provider, model string) sessionModelCost {
	t.Helper()
	for _, row := range report.ByModel {
		if row.Provider == provider && row.Model == model {
			return row
		}
	}
	t.Fatalf("no by_model row for %s/%s; got %+v", provider, model, report.ByModel)
	return sessionModelCost{}
}

func nodeFor(t *testing.T, report sessionCostReport, uuid string) sessionCostTreeNode {
	t.Helper()
	for _, node := range report.BySession {
		if node.UUID == uuid {
			return node
		}
	}
	t.Fatalf("no by_session row for %s; got %+v", uuid, report.BySession)
	return sessionCostTreeNode{}
}

// buildMisattributionTree reproduces the exact shape that produced the wrong
// figures in plans/per-session-model-cost-breakdown.md: an orchestrator on a
// paid model spawns a worker on a FREE model, and that worker spawns an
// agent-tool grandchild on a different PAID model. Every cent of the worker's
// recorded cost was actually spent by the grandchild.
func buildMisattributionTree(t *testing.T, c costStore) (root, child, grandchild session.Session) {
	t.Helper()
	root, err := c.sessions.Create(t.Context(), "orchestrator")
	require.NoError(t, err)
	child, err = c.sessions.CreateTaskSession(t.Context(), "child-tool-call", root.ID, "worker")
	require.NoError(t, err)
	grandchild, err = c.sessions.CreateTaskSession(t.Context(), "grandchild-tool-call", child.ID, "agent tool")
	require.NoError(t, err)

	// The grandchild burns real money on model B.
	c.spend(t, grandchild.ID, provB, modelB, 90_000, 500, 0.1778)
	// The worker itself only ever called the free model: zero cost.
	c.spend(t, child.ID, provFree, modelFree, 12_000, 300, 0)
	// The orchestrator's own steps on model A.
	c.spend(t, root.ID, provA, modelA, 146_385, 772, 0.05)

	// Cost propagates up the chain, deepest first, as the coordinator does.
	c.propagate(t, grandchild.ID, child.ID)
	c.propagate(t, child.ID, root.ID)

	root, err = c.sessions.Get(t.Context(), root.ID)
	require.NoError(t, err)
	child, err = c.sessions.Get(t.Context(), child.ID)
	require.NoError(t, err)
	grandchild, err = c.sessions.Get(t.Context(), grandchild.ID)
	require.NoError(t, err)
	return root, child, grandchild
}

// TestSessionByModelAttributesCostToTheIncurringModel is the acceptance test
// named by plans/per-session-model-cost-breakdown.md: a parent on model A
// spawning a child on model B must charge B's spend to B, report A's own spend
// excluding B, and own costs must sum to the tree total. It additionally
// asserts the spec's headline requirement that a session which only ever
// called a free model reads ZERO.
func TestSessionByModelAttributesCostToTheIncurringModel(t *testing.T) {
	t.Parallel()
	c := newCostStore(t)
	root, child, grandchild := buildMisattributionTree(t, c)

	// The write side is genuinely misattributing: both the worker and the
	// orchestrator carry money on their session row that they never spent.
	require.InDelta(t, 0.1778, child.Cost, 1e-9, "worker row carries the grandchild's spend")
	require.InDelta(t, 0.2278, root.Cost, 1e-9, "orchestrator row carries the whole subtree")

	report := c.report(t, root)

	// REQUIREMENT 1: cost lands on the model that incurred it.
	free := rowFor(t, report, provFree, modelFree)
	require.Zero(t, free.IncurredCostHere)
	require.Zero(t, free.IncurredCostSubSessions)
	require.Zero(t, free.IncurredCostTotal,
		"a free model must read zero even though the session that ran it shows $%.4f", child.Cost)
	require.Equal(t, int64(12_000), free.PromptTokens, "the free model's tokens are still reported")

	// Model B's spend is charged to model B, in the sub-session column.
	b := rowFor(t, report, provB, modelB)
	require.Zero(t, b.IncurredCostHere)
	require.InDelta(t, 0.1778, b.IncurredCostSubSessions, 1e-9)
	require.InDelta(t, 0.1778, b.IncurredCostTotal, 1e-9)

	// Model A reports its OWN spend, excluding B.
	a := rowFor(t, report, provA, modelA)
	require.InDelta(t, 0.05, a.IncurredCostHere, 1e-9)
	require.Zero(t, a.IncurredCostSubSessions)
	require.InDelta(t, 0.05, a.IncurredCostTotal, 1e-9)

	// REQUIREMENT 2: own cost is separated from propagated descendant cost,
	// and the own costs reconcile with the tree total.
	require.InDelta(t, 0.05, report.IncurredCostHere, 1e-9)
	require.InDelta(t, 0.1778, report.IncurredCostSubSessions, 1e-9)
	require.Equal(t, 2, report.SubSessionCount)
	require.InDelta(t, root.Cost, report.IncurredCostTotal, 1e-9,
		"sum of own costs across the tree must equal the tree total")
	require.InDelta(t, 0, report.UnattributedCost, 1e-9)

	var summed float64
	for _, row := range report.ByModel {
		summed += row.IncurredCostTotal
		require.InDelta(t, row.IncurredCostHere+row.IncurredCostSubSessions, row.IncurredCostTotal, 1e-12)
	}
	require.InDelta(t, report.IncurredCostTotal, summed, 1e-9,
		"per-model own costs must sum to the attributed total")

	var summedSessions float64
	for _, node := range report.BySession {
		summedSessions += node.IncurredCost
	}
	require.InDelta(t, report.IncurredCostTotal, summedSessions, 1e-9,
		"per-session own costs must sum to the attributed total")

	// The subtree listing is what explains the gap on the worker row.
	childNode := nodeFor(t, report, child.ID)
	require.InDelta(t, 0.1778, childNode.RecordedCost, 1e-9)
	require.Zero(t, childNode.IncurredCost, "the worker session incurred nothing itself")
	require.Equal(t, 1, childNode.Depth)
	require.Equal(t, 2, nodeFor(t, report, grandchild.ID).Depth)
}

// TestSessionByModelOnFreeWorkerSessionReadsZero covers the probe from the
// previous review: viewed directly, a free-model worker session used to print
// `Total cost: $0.177800` above a per-model table reading `$0.000000`, with
// nothing explaining the gap. The breakdown must now reconcile from the
// worker's own point of view too.
func TestSessionByModelOnFreeWorkerSessionReadsZero(t *testing.T) {
	t.Parallel()
	c := newCostStore(t)
	_, child, _ := buildMisattributionTree(t, c)

	report := c.report(t, child)
	require.InDelta(t, 0.1778, report.RecordedCost, 1e-9)
	require.Zero(t, report.IncurredCostHere)
	require.InDelta(t, 0.1778, report.IncurredCostSubSessions, 1e-9)
	require.InDelta(t, report.RecordedCost, report.IncurredCostTotal, 1e-9)
	require.InDelta(t, 0, report.UnattributedCost, 1e-9)
	require.Equal(t, 1, report.SubSessionCount)

	require.Zero(t, rowFor(t, report, provFree, modelFree).IncurredCostTotal)
	require.InDelta(t, 0.1778, rowFor(t, report, provB, modelB).IncurredCostSubSessions, 1e-9)

	// The orchestrator is not a descendant, so its spend must not appear here.
	for _, row := range report.ByModel {
		require.NotEqual(t, modelA, row.Model, "an ancestor's model must not leak into a child's breakdown")
	}
}

func TestSessionByModelHumanOutputReconciles(t *testing.T) {
	t.Parallel()
	c := newCostStore(t)
	root, _, _ := buildMisattributionTree(t, c)

	var buf bytes.Buffer
	require.NoError(t, outputSessionByModelHuman(&buf, root, c.report(t, root)))
	out := buf.String()

	require.Contains(t, out, "recorded on this session row   $0.227800")
	require.Contains(t, out, "incurred by this session       $0.050000")
	require.Contains(t, out, "incurred by 2 sub-sessions     $0.177800")
	require.Contains(t, out, "attributed total               $0.227800")
	require.Contains(t, out, "unattributed                   $0.000000")
	require.Contains(t, out, "$ HERE")
	require.Contains(t, out, "$ SUB")
	// The free model's row must show zeros in every cost column.
	require.Regexp(t, `deepseek-v4-flash-free\s+opencode-zen\s+1\s+12000\s+300\s+0\.000000\s+0\.000000\s+0\.000000`, out)
	// No bare "COST" column heading that silently mixes own and propagated spend.
	require.NotRegexp(t, `(?m)^MODEL.*\bCOST\b`, out)
	require.Contains(t, out, "Sessions in this subtree")
}

func TestSessionByModelJSONShape(t *testing.T) {
	t.Parallel()
	c := newCostStore(t)
	root, _, _ := buildMisattributionTree(t, c)

	var buf bytes.Buffer
	require.NoError(t, outputSessionByModelJSON(&buf, root, c.report(t, root)))

	var out sessionByModelOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	require.Equal(t, root.ID, out.Meta.UUID)
	require.Equal(t, session.HashID(root.ID), out.Meta.ID)
	require.Equal(t, int64(146_385), out.Meta.ContextPromptTokens)

	att := out.Meta.CostAttribution
	require.InDelta(t, 0.2278, att.RecordedCost, 1e-9)
	require.InDelta(t, 0.05, att.IncurredCostHere, 1e-9)
	require.InDelta(t, 0.1778, att.IncurredCostSubSessions, 1e-9)
	require.InDelta(t, 0.2278, att.IncurredCostTotal, 1e-9)
	require.InDelta(t, 0, att.UnattributedCost, 1e-9)
	require.Len(t, att.ByModel, 3)
	require.Len(t, att.BySession, 3)

	// The ambiguous keys the plain `show` payload carries must not reappear
	// here under any name.
	var raw map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &raw))
	meta, ok := raw["meta"].(map[string]any)
	require.True(t, ok)
	require.NotContains(t, meta, "cost")
	require.NotContains(t, meta, "prompt_tokens")
	rowsRaw := meta["cost_attribution"].(map[string]any)["by_model"].([]any)
	require.Len(t, rowsRaw, 3)
	for _, r := range rowsRaw {
		require.NotContains(t, r.(map[string]any), "cost")
	}
}

// TestSessionByModelReconcilesWithoutDescendants keeps the flat case honest:
// a session with no sub-sessions must still reconcile, and must not grow a
// sub-session section.
func TestSessionByModelReconcilesWithoutDescendants(t *testing.T) {
	t.Parallel()
	c := newCostStore(t)
	sess, err := c.sessions.Create(t.Context(), "flat")
	require.NoError(t, err)
	c.spend(t, sess.ID, provA, modelA, 100, 50, 0.01)
	c.spend(t, sess.ID, provA, modelA, 200, 80, 0.02)
	c.spend(t, sess.ID, provFree, modelFree, 10, 5, 0)
	sess, err = c.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)

	report := c.report(t, sess)
	require.Equal(t, 0, report.SubSessionCount)
	require.InDelta(t, 0.03, report.IncurredCostTotal, 1e-9)
	require.InDelta(t, sess.Cost, report.IncurredCostTotal, 1e-9)
	require.InDelta(t, 0, report.UnattributedCost, 1e-9)

	a := rowFor(t, report, provA, modelA)
	require.Equal(t, int64(2), a.MessageCount)
	require.Equal(t, int64(300), a.PromptTokens)
	require.Equal(t, int64(130), a.CompletionTokens)
	require.Equal(t, int64(430), a.TotalTokens)
	require.InDelta(t, 0.03, a.IncurredCostHere, 1e-9)
	require.Zero(t, rowFor(t, report, provFree, modelFree).IncurredCostTotal)

	var buf bytes.Buffer
	require.NoError(t, outputSessionByModelHuman(&buf, sess, report))
	require.NotContains(t, buf.String(), "Sessions in this subtree")
}

// TestSessionByModelReportsUnattributedCost proves the reconciliation is a
// real check and not a tautology: money on the session row with no per-step
// usage record behind it is surfaced rather than silently absorbed.
func TestSessionByModelReportsUnattributedCost(t *testing.T) {
	t.Parallel()
	c := newCostStore(t)
	sess, err := c.sessions.Create(t.Context(), "legacy")
	require.NoError(t, err)
	// A legacy assistant message with no Finish usage at all.
	msg, err := c.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.Assistant, Model: modelA, Provider: provA,
	})
	require.NoError(t, err)
	msg.AddFinish(message.FinishReasonEndTurn, "", "")
	require.NoError(t, c.messages.Update(t.Context(), msg))
	require.NoError(t, c.messages.Flush(t.Context(), msg.ID))
	sess.Cost = 0.42
	_, err = c.sessions.Save(t.Context(), sess)
	require.NoError(t, err)
	sess, err = c.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)

	report := c.report(t, sess)
	require.Zero(t, report.IncurredCostTotal)
	require.InDelta(t, 0.42, report.UnattributedCost, 1e-9)
	require.Equal(t, int64(1), rowFor(t, report, provA, modelA).MessageCount)
}

func TestAggregateSessionCostByModelIgnoresNonAssistantAndNilMessages(t *testing.T) {
	t.Parallel()
	withFinish := func(model string, cost float64) *message.Message {
		m := &message.Message{Role: message.Assistant, Model: model, Provider: provA}
		m.AddFinishWithUsage(message.FinishReasonEndTurn, "", "", 10, 5, cost)
		return m
	}
	rows := aggregateSessionCostByModel([]sessionSubtreeNode{
		{depth: 0, msgs: []*message.Message{
			withFinish(modelA, 0.01),
			nil,
			{Role: message.User, Model: modelA, Provider: provA},
			{Role: message.Assistant}, // no model/provider metadata at all
		}},
		{depth: 1, msgs: []*message.Message{withFinish(modelA, 0.02)}},
	})
	require.Len(t, rows, 2)
	require.Equal(t, modelA, rows[0].Model)
	require.InDelta(t, 0.01, rows[0].IncurredCostHere, 1e-9)
	require.InDelta(t, 0.02, rows[0].IncurredCostSubSessions, 1e-9)
	require.InDelta(t, 0.03, rows[0].IncurredCostTotal, 1e-9)
	require.Equal(t, "unknown", rows[1].Model)
	require.Equal(t, "unknown", rows[1].Provider)
	require.Equal(t, int64(1), rows[1].MessageCount)
}

// staticSessions is a session store whose parent/child links are supplied
// directly, so a walk can be driven over shapes SQLite would not normally
// produce (cycles, oversized fan-out).
type staticSessions struct {
	children map[string][]session.Session
}

func (s *staticSessions) ListChildren(_ context.Context, parentSessionID string) ([]session.Session, error) {
	return s.children[parentSessionID], nil
}

type emptyMessages struct{}

func (emptyMessages) List(context.Context, string) ([]message.Message, error) {
	return nil, nil
}

func TestCollectSessionSubtreeTerminatesOnCycle(t *testing.T) {
	t.Parallel()
	a := session.Session{ID: "a"}
	b := session.Session{ID: "b"}
	store := &staticSessions{children: map[string][]session.Session{
		"a": {b},
		"b": {a, b}, // points back at its own ancestor and at itself
	}}

	done := make(chan struct{})
	var nodes []sessionSubtreeNode
	go func() {
		defer close(done)
		var err error
		nodes, _, err = collectSessionSubtree(t.Context(), store, emptyMessages{}, a)
		if err != nil {
			panic(err)
		}
	}()
	select {
	case <-done:
	case <-t.Context().Done():
		t.Fatal("collectSessionSubtree did not terminate on a cyclic parent chain")
	}
	require.Len(t, nodes, 2)
}

func TestCollectSessionSubtreeIsBounded(t *testing.T) {
	t.Parallel()
	// A single root with far more children than the cap.
	root := session.Session{ID: "root"}
	kids := make([]session.Session, maxCostTreeSessions+50)
	for i := range kids {
		kids[i] = session.Session{ID: fmt.Sprintf("kid-%d", i)}
	}
	store := &staticSessions{children: map[string][]session.Session{"root": kids}}

	nodes, truncated, err := collectSessionSubtree(t.Context(), store, emptyMessages{}, root)
	require.NoError(t, err)
	require.True(t, truncated)
	require.Len(t, nodes, maxCostTreeSessions)

	var buf bytes.Buffer
	require.NoError(t, outputSessionByModelHuman(&buf, session.Session{}, buildSessionCostReport(nodes, truncated)))
	require.True(t, strings.Contains(buf.String(), "figures above are incomplete"),
		"a truncated walk must say so instead of reporting a wrong total")
}

type failingMessages struct{}

func (failingMessages) List(context.Context, string) ([]message.Message, error) {
	return nil, fmt.Errorf("boom")
}

func TestCollectSessionSubtreePropagatesMessageErrors(t *testing.T) {
	t.Parallel()
	_, _, err := collectSessionSubtree(t.Context(), &staticSessions{}, failingMessages{}, session.Session{ID: "a"})
	require.ErrorContains(t, err, "boom")
}
