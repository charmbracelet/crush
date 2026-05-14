package chat

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/anim"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// versionedItem is the cross-cutting interface every chat item type
// must satisfy under F6: every documented mutator must bump the
// shared version counter so the list-level memo invalidates.
type versionedItem interface {
	list.Item
	Version() uint64
}

// requireBump asserts that the supplied mutator advances the item's
// Version(). The mutator runs once; an absent bump is a regression
// (a finished item would keep serving stale frozen output to the
// list cache).
func requireBump(t *testing.T, name string, item versionedItem, mutate func()) {
	t.Helper()
	before := item.Version()
	mutate()
	after := item.Version()
	require.Greaterf(t, after, before, "%s must bump Version() (before=%d, after=%d)", name, before, after)
}

// TestAssistantMessageItem_MutatorsBumpVersion enumerates every
// documented mutator on AssistantMessageItem and asserts each one
// advances Version().
func TestAssistantMessageItem_MutatorsBumpVersion(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	build := func(thinking, content string) *message.Message {
		parts := []message.ContentPart{
			message.ReasoningContent{
				Thinking:   thinking,
				StartedAt:  testStartedAt,
				FinishedAt: testFinishedAt,
			},
		}
		if content != "" {
			parts = append(parts, message.TextContent{Text: content})
		}
		return &message.Message{ID: "a-mut", Role: message.Assistant, Parts: parts}
	}

	item := NewAssistantMessageItem(&sty, build("thinking", "content")).(*AssistantMessageItem)

	requireBump(t, "SetMessage", item, func() {
		item.SetMessage(build("thinking", "more content"))
	})
	requireBump(t, "SetFocused", item, func() {
		item.SetFocused(true)
	})
	requireBump(t, "SetHighlight", item, func() {
		item.SetHighlight(0, 0, 0, 5)
	})
	// ToggleExpanded only mutates state when there is non-empty
	// thinking text — which the build helper provides.
	requireBump(t, "ToggleExpanded", item, func() {
		item.ToggleExpanded()
	})
}

// TestUserMessageItem_MutatorsBumpVersion enumerates UserMessageItem
// mutators.
func TestUserMessageItem_MutatorsBumpVersion(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	r := attachments.NewRenderer(
		sty.Attachments.Normal,
		sty.Attachments.Deleting,
		sty.Attachments.Image,
		sty.Attachments.Text,
	)
	msg := &message.Message{
		ID:   "u-mut",
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "Hello"},
		},
	}
	item := NewUserMessageItem(&sty, msg, r).(*UserMessageItem)

	requireBump(t, "SetFocused", item, func() {
		item.SetFocused(true)
	})
	requireBump(t, "SetHighlight", item, func() {
		item.SetHighlight(0, 0, 0, 3)
	})
}

// TestAssistantInfoItem_VersionedAndFinished sanity-checks the
// AssistantInfoItem wiring. The item carries only immutable data
// after construction; we still assert Version() is callable and
// Finished() returns true.
func TestAssistantInfoItem_VersionedAndFinished(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	cfg := &config.Config{}
	msg := &message.Message{
		ID:    "info",
		Role:  message.Assistant,
		Parts: []message.ContentPart{message.Finish{Reason: message.FinishReasonEndTurn, Time: time.Now().Unix()}},
	}
	item := NewAssistantInfoItem(&sty, msg, cfg, time.Unix(0, 0)).(*AssistantInfoItem)

	require.True(t, item.Finished(), "AssistantInfoItem must be Finished()")
	// Version() is callable and starts at zero.
	require.Equal(t, uint64(0), item.Version())
}

// TestBaseToolMessageItem_MutatorsBumpVersion enumerates the base
// tool item mutators. Specific tool types layer on top of this
// base; the base bumps cover the shared mutator surface.
func TestBaseToolMessageItem_MutatorsBumpVersion(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tc := message.ToolCall{ID: "tc1", Name: "bash", Input: "{}", Finished: false}
	item := NewToolMessageItem(&sty, "msg", tc, nil, false)

	v := item.(versionedItem)

	requireBump(t, "SetFocused", v, func() {
		if f, ok := item.(list.Focusable); ok {
			f.SetFocused(true)
		}
	})
	requireBump(t, "SetHighlight", v, func() {
		if h, ok := item.(list.Highlightable); ok {
			h.SetHighlight(0, 0, 0, 3)
		}
	})
	requireBump(t, "SetToolCall", v, func() {
		tc2 := tc
		tc2.Input = `{"command":"echo"}`
		item.SetToolCall(tc2)
	})
	requireBump(t, "SetResult", v, func() {
		item.SetResult(&message.ToolResult{ToolCallID: "tc1", Content: "ok"})
	})
	requireBump(t, "SetStatus", v, func() {
		item.SetStatus(ToolStatusSuccess)
	})
	requireBump(t, "ToggleExpanded", v, func() {
		if e, ok := item.(Expandable); ok {
			e.ToggleExpanded()
		}
	})
	requireBump(t, "SetCompact", v, func() {
		if c, ok := item.(Compactable); ok {
			c.SetCompact(true)
		}
	})
}

// TestAssistantMessageItem_AnimateBumpsVersion covers the spinner
// regression: while the assistant message is spinning, every
// anim.StepMsg fed through Animate must bump Version() so the
// list-level cache invalidates and the next draw re-renders the
// advanced spinner frame. Without this bump the cached entry's
// version stays put and the spinner appears frozen.
func TestAssistantMessageItem_AnimateBumpsVersion(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	streaming := &message.Message{
		ID:   "spin",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ReasoningContent{Thinking: "thinking..."},
		},
	}
	item := NewAssistantMessageItem(&sty, streaming).(*AssistantMessageItem)

	requireBump(t, "Animate", item, func() {
		item.Animate(anim.StepMsg{})
	})

	// A non-spinning item must not bump on Animate: the bump only
	// makes sense while the spinner is live, and a stray bump on a
	// finished item would needlessly invalidate frozen entries.
	finished := &message.Message{
		ID:   "spin",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "done"},
			message.Finish{Reason: message.FinishReasonEndTurn, Time: testFinishTime},
		},
	}
	item.SetMessage(finished)
	require.True(t, item.Finished(), "item must report Finished() once the message finishes")
	before := item.Version()
	item.Animate(anim.StepMsg{})
	require.Equal(t, before, item.Version(), "Animate must not bump Version() on a non-spinning item")
}

// TestAssistantMessageItem_FinishedTransition covers §4.5.1: a
// streaming assistant message reports Finished() == false; once the
// message reports IsFinished() and stops spinning, Finished() must
// return true.
func TestAssistantMessageItem_FinishedTransition(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()

	// Streaming: no finish part, no content yet — isSpinning == true.
	streaming := &message.Message{
		ID:   "stream",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ReasoningContent{Thinking: "thinking..."},
		},
	}
	item := NewAssistantMessageItem(&sty, streaming).(*AssistantMessageItem)
	require.False(t, item.Finished(), "streaming assistant message must not be Finished()")

	// Finished with content.
	finished := &message.Message{
		ID:   "stream",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ReasoningContent{Thinking: "thinking", StartedAt: testStartedAt, FinishedAt: testFinishedAt},
			message.TextContent{Text: "the answer"},
			message.Finish{Reason: message.FinishReasonEndTurn, Time: testFinishTime},
		},
	}
	item.SetMessage(finished)
	require.True(t, item.Finished(), "finished assistant message must be Finished()")
}

// TestUserMessageItem_FinishedAlwaysTrue locks in the freezable
// contract: user messages are never spinning.
func TestUserMessageItem_FinishedAlwaysTrue(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	r := attachments.NewRenderer(
		sty.Attachments.Normal,
		sty.Attachments.Deleting,
		sty.Attachments.Image,
		sty.Attachments.Text,
	)
	msg := &message.Message{
		ID:    "u-fin",
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "hi"}},
	}
	item := NewUserMessageItem(&sty, msg, r).(*UserMessageItem)
	require.True(t, item.Finished())
}

// TestAgentToolMessageItem_NestedToolMutatorsBumpVersion covers B1:
// the nested-tool mutators on AgentToolMessageItem must bump
// Version() so the list cache invalidates frozen entries when a
// nested tool is added or the slice changes. SetNestedTools always
// bumps unconditionally — the live update path in
// internal/ui/model/ui.go mutates existing children in place and
// then re-passes the same slice, so a pointer-equality dedupe would
// hide observable child-render changes. AddNestedTool also always
// observably mutates state and always bumps.
func TestAgentToolMessageItem_NestedToolMutatorsBumpVersion(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	parent := message.ToolCall{ID: "agent-parent", Name: "agent", Input: `{}`, Finished: false}
	item := NewAgentToolMessageItem(&sty, parent, nil, false)

	mkChild := func(id string) ToolMessageItem {
		tc := message.ToolCall{ID: id, Name: "bash", Input: `{}`, Finished: false}
		return NewToolMessageItem(&sty, "msg", tc, nil, false)
	}

	// AddNestedTool always bumps.
	requireBump(t, "AddNestedTool", item, func() {
		item.AddNestedTool(mkChild("c1"))
	})

	// SetNestedTools always bumps, even with a pointer-equal slice.
	current := append([]ToolMessageItem(nil), item.NestedTools()...)
	requireBump(t, "SetNestedTools[pointer-equal]", item, func() {
		item.SetNestedTools(current)
	})

	// SetNestedTools with a different slice (extra element) bumps.
	requireBump(t, "SetNestedTools[different]", item, func() {
		item.SetNestedTools(append(current, mkChild("c2")))
	})

	// SetNestedTools to an empty slice from a non-empty state bumps.
	requireBump(t, "SetNestedTools[empty]", item, func() {
		item.SetNestedTools(nil)
	})
}

// TestAgenticFetchToolMessageItem_NestedToolMutatorsBumpVersion is
// the agentic-fetch counterpart to the agent-tool nested mutator
// bump test above.
func TestAgenticFetchToolMessageItem_NestedToolMutatorsBumpVersion(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	parent := message.ToolCall{ID: "fetch-parent", Name: "agentic_fetch", Input: `{}`, Finished: false}
	item := NewAgenticFetchToolMessageItem(&sty, parent, nil, false)

	mkChild := func(id string) ToolMessageItem {
		tc := message.ToolCall{ID: id, Name: "fetch", Input: `{}`, Finished: false}
		return NewToolMessageItem(&sty, "msg", tc, nil, false)
	}

	requireBump(t, "AddNestedTool", item, func() {
		item.AddNestedTool(mkChild("c1"))
	})

	current := append([]ToolMessageItem(nil), item.NestedTools()...)
	requireBump(t, "SetNestedTools[pointer-equal]", item, func() {
		item.SetNestedTools(current)
	})

	requireBump(t, "SetNestedTools[different]", item, func() {
		item.SetNestedTools(append(current, mkChild("c2")))
	})

	requireBump(t, "SetNestedTools[empty]", item, func() {
		item.SetNestedTools(nil)
	})
}

// TestAgentToolMessageItem_NestedChildInPlaceMutationBumpsParent is
// the T5 regression test: it mirrors the live update flow at
// internal/ui/model/ui.go:1242-1281 where nested tool calls are
// updated in place (SetToolCall / SetResult on the same child
// pointers) and then the same slice is handed back to the parent
// via SetNestedTools. The parent must still bump its version so
// the list cache invalidates the parent's pre-rendered string and
// the freshly-rendered child output becomes visible.
func TestAgentToolMessageItem_NestedChildInPlaceMutationBumpsParent(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	parent := message.ToolCall{ID: "agent-parent", Name: "agent", Input: `{}`, Finished: false}
	item := NewAgentToolMessageItem(&sty, parent, nil, false)

	childTC := message.ToolCall{ID: "c1", Name: "bash", Input: `{}`, Finished: false}
	child := NewToolMessageItem(&sty, "msg", childTC, nil, false)
	item.AddNestedTool(child)

	v0 := item.Version()
	childVersionBefore := child.(versionedItem).Version()

	// In-place mutate the existing child, exactly like the live
	// flow in ui.go:1271-1278 does.
	child.SetResult(&message.ToolResult{ToolCallID: "c1", Content: "ok"})
	require.Greaterf(t, child.(versionedItem).Version(), childVersionBefore,
		"child SetResult must bump child version")

	// Hand the same slice back to the parent (pointers unchanged).
	same := item.NestedTools()
	item.SetNestedTools(same)
	require.Greaterf(t, item.Version(), v0,
		"parent SetNestedTools must bump even when child pointers are unchanged (in-place child mutation invalidates parent's pre-rendered output)")
}

// TestAgenticFetchToolMessageItem_NestedChildInPlaceMutationBumpsParent
// is the agentic-fetch counterpart of the T5 regression test.
func TestAgenticFetchToolMessageItem_NestedChildInPlaceMutationBumpsParent(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	parent := message.ToolCall{ID: "fetch-parent", Name: "agentic_fetch", Input: `{}`, Finished: false}
	item := NewAgenticFetchToolMessageItem(&sty, parent, nil, false)

	childTC := message.ToolCall{ID: "c1", Name: "fetch", Input: `{}`, Finished: false}
	child := NewToolMessageItem(&sty, "msg", childTC, nil, false)
	item.AddNestedTool(child)

	v0 := item.Version()
	childVersionBefore := child.(versionedItem).Version()

	child.SetResult(&message.ToolResult{ToolCallID: "c1", Content: "ok"})
	require.Greaterf(t, child.(versionedItem).Version(), childVersionBefore,
		"child SetResult must bump child version")

	same := item.NestedTools()
	item.SetNestedTools(same)
	require.Greaterf(t, item.Version(), v0,
		"parent SetNestedTools must bump even when child pointers are unchanged")
}

// requireNoBump asserts the supplied mutator leaves the item's
// Version() unchanged. The mutator runs once; an unexpected bump
// would force the F6 list memo to re-render an item whose output
// did not change, churning the cache.
func requireNoBump(t *testing.T, name string, item versionedItem, mutate func()) {
	t.Helper()
	before := item.Version()
	mutate()
	after := item.Version()
	require.Equalf(t, before, after,
		"%s must not bump Version() (before=%d, after=%d)", name, before, after)
}

// TestBaseToolMessageItem_AnimateBumpsVersion is the spinner
// regression test for non-agent tools: while the tool is spinning,
// every anim.StepMsg whose ID matches the tool must bump Version()
// so the list-level cache invalidates and the next draw re-renders
// the advanced spinner frame. Foreign IDs must not bump (they would
// churn the cache on every frame), and a finished tool must not
// bump on any ID (the entry is frozen and stays frozen).
func TestBaseToolMessageItem_AnimateBumpsVersion(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tc := message.ToolCall{ID: "tc-spin", Name: "bash", Input: "{}", Finished: false}
	item := NewToolMessageItem(&sty, "msg", tc, nil, false)
	v := item.(versionedItem)
	a, ok := item.(Animatable)
	require.True(t, ok, "base tool message item must implement Animatable")

	// Spinning + matching ID → bump.
	requireBump(t, "Animate[spinning,own ID]", v, func() {
		a.Animate(anim.StepMsg{ID: tc.ID})
	})

	// Spinning + foreign ID → no bump. Routing this StepMsg here at
	// all would mean a future chat.Animate refactor; the item must
	// be defensive against it so we don't churn the list cache.
	requireNoBump(t, "Animate[spinning,foreign ID]", v, func() {
		a.Animate(anim.StepMsg{ID: "some-other-tool"})
	})

	// Finished → no bump on any ID. The entry is frozen; a stray
	// bump would needlessly invalidate frozen entries.
	tcFinished := tc
	tcFinished.Finished = true
	item.SetToolCall(tcFinished)
	item.SetResult(&message.ToolResult{ToolCallID: tc.ID, Content: "ok"})
	require.True(t, item.Finished(), "tool must report Finished() once the result lands")

	requireNoBump(t, "Animate[finished,own ID]", v, func() {
		a.Animate(anim.StepMsg{ID: tc.ID})
	})
	requireNoBump(t, "Animate[finished,foreign ID]", v, func() {
		a.Animate(anim.StepMsg{ID: "some-other-tool"})
	})
}

// TestAgentToolMessageItem_AnimateBumpsVersion is the spinner
// regression test for agent tools. The parent must bump on both
// the parent-tick branch (msg.ID == parent.ID()) and the
// nested-tick branch (msg.ID == nested.ID()) because the list
// only checks the parent's version — nested tools are not list
// entries of their own. Unrelated IDs must not bump, and a parent
// with a result must not bump on any ID.
func TestAgentToolMessageItem_AnimateBumpsVersion(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	parentTC := message.ToolCall{ID: "agent-parent", Name: "agent", Input: `{}`, Finished: false}
	parent := NewAgentToolMessageItem(&sty, parentTC, nil, false)

	childTC := message.ToolCall{ID: "agent-child", Name: "bash", Input: `{}`, Finished: false}
	child := NewToolMessageItem(&sty, "msg", childTC, nil, false)
	parent.AddNestedTool(child)

	// Spinning + parent's own ID → parent bumps.
	requireBump(t, "Animate[spinning,parent ID]", parent, func() {
		parent.Animate(anim.StepMsg{ID: parentTC.ID})
	})

	// Spinning + nested child ID → parent bumps. The list only
	// invalidates on the parent; without this the nested
	// spinner's frame would never reach the screen even though
	// the nested anim's step has advanced.
	requireBump(t, "Animate[spinning,nested ID]", parent, func() {
		parent.Animate(anim.StepMsg{ID: childTC.ID})
	})

	// Spinning + unrelated ID → no bump.
	requireNoBump(t, "Animate[spinning,foreign ID]", parent, func() {
		parent.Animate(anim.StepMsg{ID: "unrelated"})
	})

	// Once the parent has a result, neither branch bumps.
	parent.SetResult(&message.ToolResult{ToolCallID: parentTC.ID, Content: "done"})
	requireNoBump(t, "Animate[finished,parent ID]", parent, func() {
		parent.Animate(anim.StepMsg{ID: parentTC.ID})
	})
	requireNoBump(t, "Animate[finished,nested ID]", parent, func() {
		parent.Animate(anim.StepMsg{ID: childTC.ID})
	})
}

// TestAgenticFetchToolMessageItem_AnimateBumpsVersion is the
// agentic-fetch counterpart of the agent-tool Animate bump test.
// Without an explicit override the embedded base Animate would
// drop nested-child StepMsgs at anim.Animate's ID check and never
// bump the parent on its own ticks; this test locks in the
// override.
func TestAgenticFetchToolMessageItem_AnimateBumpsVersion(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	parentTC := message.ToolCall{ID: "fetch-parent", Name: "agentic_fetch", Input: `{}`, Finished: false}
	parent := NewAgenticFetchToolMessageItem(&sty, parentTC, nil, false)

	childTC := message.ToolCall{ID: "fetch-child", Name: "fetch", Input: `{}`, Finished: false}
	child := NewToolMessageItem(&sty, "msg", childTC, nil, false)
	parent.AddNestedTool(child)

	requireBump(t, "Animate[spinning,parent ID]", parent, func() {
		parent.Animate(anim.StepMsg{ID: parentTC.ID})
	})
	requireBump(t, "Animate[spinning,nested ID]", parent, func() {
		parent.Animate(anim.StepMsg{ID: childTC.ID})
	})
	requireNoBump(t, "Animate[spinning,foreign ID]", parent, func() {
		parent.Animate(anim.StepMsg{ID: "unrelated"})
	})

	parent.SetResult(&message.ToolResult{ToolCallID: parentTC.ID, Content: "done"})
	requireNoBump(t, "Animate[finished,parent ID]", parent, func() {
		parent.Animate(anim.StepMsg{ID: parentTC.ID})
	})
	requireNoBump(t, "Animate[finished,nested ID]", parent, func() {
		parent.Animate(anim.StepMsg{ID: childTC.ID})
	})
}

// TestBaseToolMessageItem_FinishedTransition covers §4.5.1 for
// tools: a still-running tool reports Finished() == false; once the
// tool call is marked finished and a result lands, Finished()
// returns true. Cancelled tools also become Finished.
func TestBaseToolMessageItem_FinishedTransition(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	tc := message.ToolCall{ID: "tc-fin", Name: "bash", Input: "{}", Finished: false}
	item := NewToolMessageItem(&sty, "msg", tc, nil, false)
	require.False(t, item.Finished(), "running tool must not be Finished()")

	tcFinished := tc
	tcFinished.Finished = true
	item.SetToolCall(tcFinished)
	item.SetResult(&message.ToolResult{ToolCallID: "tc-fin", Content: "ok"})
	require.True(t, item.Finished(), "finished tool with result must be Finished()")

	// Canceled tool with no result is also Finished.
	tcCanceled := message.ToolCall{ID: "tc-cancel", Name: "bash", Input: "{}", Finished: false}
	canceled := NewToolMessageItem(&sty, "msg", tcCanceled, nil, true)
	require.True(t, canceled.Finished(), "canceled tool must be Finished()")
}
