package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// Fixed Unix timestamps for deterministic cache-equality tests. The
// thinking section's `extra` hash folds in ThinkingDuration, which
// in turn depends on (FinishedAt - StartedAt). Anchoring both
// timestamps removes any wall-clock dependency from the cache key
// so two builds across a second boundary still hit the cache.
const (
	testStartedAt  int64 = 1_700_000_000
	testFinishedAt int64 = 1_700_000_005
	testFinishTime int64 = 1_700_000_006
)

// thinkingMessage builds an assistant message with a fixed reasoning
// content and an optional text content. When text is empty the
// message represents a still-thinking turn (matches IsThinking()).
// Both reasoning timestamps are anchored to fixed Unix seconds so
// ThinkingDuration is deterministic and cache-equality assertions
// don't depend on wall-clock time.
func thinkingMessage(id, thinking, text string) *message.Message {
	parts := []message.ContentPart{
		message.ReasoningContent{
			Thinking:   thinking,
			StartedAt:  testStartedAt,
			FinishedAt: testFinishedAt,
		},
	}
	if text != "" {
		parts = append(parts, message.TextContent{Text: text})
	}
	return &message.Message{
		ID:    id,
		Role:  message.Assistant,
		Parts: parts,
	}
}

// errorMessage builds a finished assistant message whose finish part
// carries an error reason plus a custom message and details.
func errorMessage(id, errMsg, errDetails string) *message.Message {
	return &message.Message{
		ID:   id,
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "partial output"},
			message.Finish{
				Reason:  message.FinishReasonError,
				Message: errMsg,
				Details: errDetails,
				Time:    testFinishTime,
			},
		},
	}
}

// renderTwoSetMessages drives a SetMessage cycle and returns the
// section-cache identity (out string pointers via direct comparison
// of the cached fields). The test compares `out` strings; identical
// output across cycles is the cache-hit indicator we rely on.
type sectionSnapshot struct {
	thinking string
	content  string
	errSec   string
}

func snapshot(a *AssistantMessageItem) sectionSnapshot {
	return sectionSnapshot{
		thinking: a.thinkingSec.out,
		content:  a.contentSec.out,
		errSec:   a.errorSec.out,
	}
}

// TestAssistantSectionCache_ContentChangeDoesNotInvalidateThinking covers
// the central F4 invariant: streaming the main content through SetMessage
// must keep the cached thinking render intact, provided the inputs to
// the thinking section render (text, expanded flag, footer state) are
// unchanged. We seed an already-non-empty content so that IsThinking()
// is false on both renders — that's the steady streaming state where
// the thinking block has finished and content keeps growing.
func TestAssistantSectionCache_ContentChangeDoesNotInvalidateThinking(t *testing.T) {
	sty := styles.CharmtonePantera()
	thinking := "Step 1\nStep 2\nStep 3"
	msg := thinkingMessage("a1", thinking, "Initial answer.")
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)

	const width = 71

	_ = item.RawRender(width)
	first := snapshot(item)
	require.NotEmpty(t, first.thinking, "thinking section must be populated after first render")

	// Stream more content into the existing turn. Thinking text and
	// footer state are byte-identical between the two renders.
	updated := thinkingMessage("a1", thinking, "Initial answer. More streamed text.")
	item.SetMessage(updated)
	_ = item.RawRender(width)
	second := snapshot(item)

	require.Equal(t, first.thinking, second.thinking,
		"content streaming must not invalidate the thinking section render")
	require.NotEqual(t, first.content, second.content,
		"content section must have been re-rendered")
}

// TestAssistantSectionCache_ThinkingChangeDoesNotInvalidateContent is the
// mirror of the previous test: extending thinking text must not force a
// re-render of the content section.
func TestAssistantSectionCache_ThinkingChangeDoesNotInvalidateContent(t *testing.T) {
	sty := styles.CharmtonePantera()
	content := "Final answer goes here."
	msg := thinkingMessage("a2", "Step 1", content)
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)

	const width = 73

	_ = item.RawRender(width)
	first := snapshot(item)
	require.NotEmpty(t, first.content)

	updated := thinkingMessage("a2", "Step 1\nStep 2", content)
	item.SetMessage(updated)
	_ = item.RawRender(width)
	second := snapshot(item)

	require.Equal(t, first.content, second.content,
		"thinking streaming must not invalidate the content section render")
	require.NotEqual(t, first.thinking, second.thinking,
		"thinking text changed; thinking section must have re-rendered")
}

// TestAssistantSectionCache_HashKeyDiscrimination asserts that two
// messages with different source text hash to different per-section
// keys, and that messages with identical source text hit the cache.
func TestAssistantSectionCache_HashKeyDiscrimination(t *testing.T) {
	sty := styles.CharmtonePantera()
	msgA := thinkingMessage("a3", "thinking A", "content A")
	msgB := thinkingMessage("a3", "thinking B", "content B")

	itemA := NewAssistantMessageItem(&sty, msgA).(*AssistantMessageItem)
	itemB := NewAssistantMessageItem(&sty, msgB).(*AssistantMessageItem)

	thinkSrcA, _ := itemA.thinkingKey()
	thinkSrcB, _ := itemB.thinkingKey()
	require.NotEqual(t, thinkSrcA, thinkSrcB,
		"distinct thinking text must produce distinct FNV-64 source hashes")

	contentSrcA, _ := itemA.contentKey()
	contentSrcB, _ := itemB.contentKey()
	require.NotEqual(t, contentSrcA, contentSrcB,
		"distinct content text must produce distinct FNV-64 source hashes")

	// Identical source text on a fresh item must produce the same
	// hashes — keying invariant for cache hits.
	itemAClone := NewAssistantMessageItem(&sty, thinkingMessage("a3", "thinking A", "content A")).(*AssistantMessageItem)
	thinkSrcAClone, _ := itemAClone.thinkingKey()
	contentSrcAClone, _ := itemAClone.contentKey()
	require.Equal(t, thinkSrcA, thinkSrcAClone)
	require.Equal(t, contentSrcA, contentSrcAClone)
}

// TestAssistantSectionCache_CloneRoundTrip guards the contract that
// message.Clone() does not invalidate any section cache: re-keying off
// the cloned message must produce identical hashes and the section
// caches must serve byte-identical renders.
func TestAssistantSectionCache_CloneRoundTrip(t *testing.T) {
	sty := styles.CharmtonePantera()
	orig := thinkingMessage("a4", "Reasoning step.", "Answer text.")
	item := NewAssistantMessageItem(&sty, orig).(*AssistantMessageItem)

	const width = 75
	_ = item.RawRender(width)
	first := snapshot(item)

	cloned := orig.Clone()
	item.SetMessage(&cloned)
	_ = item.RawRender(width)
	second := snapshot(item)

	require.Equal(t, first.thinking, second.thinking, "clone must hit the thinking cache")
	require.Equal(t, first.content, second.content, "clone must hit the content cache")
}

// TestAssistantSectionCache_ResizeInvalidatesAll asserts that a width
// change forces a re-render of every section.
func TestAssistantSectionCache_ResizeInvalidatesAll(t *testing.T) {
	sty := styles.CharmtonePantera()
	msg := errorMessage("a5", "boom", strings.Repeat("detail line\n", 5))
	// errorMessage returns FinishReasonError; combine with thinking
	// content so all three sections are exercised.
	msg.Parts = append([]message.ContentPart{
		message.ReasoningContent{
			Thinking:   "Considering options.",
			StartedAt:  testStartedAt,
			FinishedAt: testFinishedAt,
		},
	}, msg.Parts...)
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)

	_ = item.RawRender(77)
	first := snapshot(item)
	require.NotEmpty(t, first.thinking)
	require.NotEmpty(t, first.content)
	require.NotEmpty(t, first.errSec)

	_ = item.RawRender(117)
	second := snapshot(item)

	require.NotEqual(t, first.thinking, second.thinking, "resize must re-render the thinking section")
	require.NotEqual(t, first.content, second.content, "resize must re-render the content section")
	require.NotEqual(t, first.errSec, second.errSec, "resize must re-render the error section")
}

// TestAssistantSectionCache_ErrorIndependentOfThinkingAndContent guards
// that the error section caches independently. Editing the error
// message must not invalidate the other two sections, and editing the
// content must not invalidate the error section.
func TestAssistantSectionCache_ErrorIndependentOfThinkingAndContent(t *testing.T) {
	sty := styles.CharmtonePantera()
	build := func(thinking, content, errMsg, errDetails string) *message.Message {
		return &message.Message{
			ID:   "a6",
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.ReasoningContent{
					Thinking:   thinking,
					StartedAt:  testStartedAt,
					FinishedAt: testFinishedAt,
				},
				message.TextContent{Text: content},
				message.Finish{
					Reason:  message.FinishReasonError,
					Message: errMsg,
					Details: errDetails,
					Time:    testFinishTime,
				},
			},
		}
	}

	item := NewAssistantMessageItem(&sty, build("think", "content", "boom", "details")).(*AssistantMessageItem)
	_ = item.RawRender(79)
	first := snapshot(item)

	// Change only the error text. Thinking and content caches must
	// survive; error cache must miss and re-render.
	item.SetMessage(build("think", "content", "different boom", "different details"))
	_ = item.RawRender(79)
	second := snapshot(item)

	require.Equal(t, first.thinking, second.thinking, "error change must not invalidate thinking")
	require.Equal(t, first.content, second.content, "error change must not invalidate content")
	require.NotEqual(t, first.errSec, second.errSec, "error change must re-render the error section")

	// Now change only the content; error cache must survive.
	item.SetMessage(build("think", "different content", "different boom", "different details"))
	_ = item.RawRender(79)
	third := snapshot(item)

	require.Equal(t, second.thinking, third.thinking)
	require.NotEqual(t, second.content, third.content)
	require.Equal(t, second.errSec, third.errSec, "content change must not invalidate the error section")
}

// TestAssistantSectionCache_PrefixCacheRespectsSectionChanges guards
// the F3/F4 boundary: the prefix cache must invalidate when any
// underlying section changes. We verify by comparing the F3-cached
// Render output across SetMessage cycles.
func TestAssistantSectionCache_PrefixCacheRespectsSectionChanges(t *testing.T) {
	sty := styles.CharmtonePantera()
	build := func(content string) *message.Message {
		return &message.Message{
			ID:   "a7",
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: content},
				message.Finish{Reason: message.FinishReasonEndTurn, Time: testFinishTime},
			},
		}
	}

	item := NewAssistantMessageItem(&sty, build("first content")).(*AssistantMessageItem)
	item.SetFocused(true)

	const width = 81
	first := item.Render(width)

	item.SetMessage(build("second content"))
	second := item.Render(width)
	require.NotEqual(t, first, second,
		"prefix cache must invalidate when the content section changes")

	// Re-set to the original content; the prefix cache should
	// produce identical output again.
	item.SetMessage(build("first content"))
	third := item.Render(width)
	require.Equal(t, first, third)
}

// TestAssistantSectionCache_ByteIdenticalToFreshRender asserts that the
// F4 cached path produces the same bytes as a fresh-instance render of
// the equivalent message — i.e. caching is invisible from the outside.
// Drives a sequence of mutations (thinking change, content change,
// finish) and compares every step against an independent item rendered
// from scratch.
func TestAssistantSectionCache_ByteIdenticalToFreshRender(t *testing.T) {
	sty := styles.CharmtonePantera()
	const width = 83

	type step struct {
		name string
		msg  *message.Message
	}
	startedAt := testStartedAt
	finishedAt := testFinishedAt
	finishTime := testFinishTime
	steps := []step{
		{
			name: "thinking-only",
			msg: &message.Message{
				ID: "iso", Role: message.Assistant,
				Parts: []message.ContentPart{
					message.ReasoningContent{Thinking: "first reasoning", StartedAt: startedAt},
				},
			},
		},
		{
			name: "thinking-grew",
			msg: &message.Message{
				ID: "iso", Role: message.Assistant,
				Parts: []message.ContentPart{
					message.ReasoningContent{Thinking: "first reasoning more", StartedAt: startedAt},
				},
			},
		},
		{
			name: "content-arrived",
			msg: &message.Message{
				ID: "iso", Role: message.Assistant,
				Parts: []message.ContentPart{
					message.ReasoningContent{Thinking: "first reasoning more", StartedAt: startedAt, FinishedAt: finishedAt},
					message.TextContent{Text: "the answer"},
				},
			},
		},
		{
			name: "finished-end-turn",
			msg: &message.Message{
				ID: "iso", Role: message.Assistant,
				Parts: []message.ContentPart{
					message.ReasoningContent{Thinking: "first reasoning more", StartedAt: startedAt, FinishedAt: finishedAt},
					message.TextContent{Text: "the answer"},
					message.Finish{Reason: message.FinishReasonEndTurn, Time: finishTime},
				},
			},
		},
	}

	first := steps[0].msg.Clone()
	cached := NewAssistantMessageItem(&sty, &first).(*AssistantMessageItem)
	for _, s := range steps {
		cached.SetMessage(s.msg)
		freshMsg := s.msg.Clone()
		fresh := NewAssistantMessageItem(&sty, &freshMsg).(*AssistantMessageItem)
		require.Equal(t, fresh.RawRender(width), cached.RawRender(width),
			"step %q: cached path must match fresh render byte-for-byte", s.name)
	}
}

// TestAssistantSectionCache_PrefixCacheInvalidatesOnCompositionOnlyChange
// guards the F3 prefix cache against composition-only changes:
// flipping the finish reason from EndTurn to Canceled appends a
// constant "Canceled" line via renderMessageContent, but no
// section's own source text changes. The prefix cache must observe
// the difference (compositionKey is folded into prefixCacheKey) and
// the resulting bytes must differ. As a second guarantee, a fresh
// item built with the same final state must produce byte-equal
// output to the cached item — caching must never produce stale or
// divergent renders.
func TestAssistantSectionCache_PrefixCacheInvalidatesOnCompositionOnlyChange(t *testing.T) {
	sty := styles.CharmtonePantera()
	const width = 87

	build := func(reason message.FinishReason) *message.Message {
		return &message.Message{
			ID:   "comp",
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: "hi"},
				message.Finish{Reason: reason, Time: testFinishTime},
			},
		}
	}

	item := NewAssistantMessageItem(&sty, build(message.FinishReasonEndTurn)).(*AssistantMessageItem)
	endTurnOut := item.Render(width)

	// Flip only the finish reason. Thinking is empty and content
	// text is unchanged, so no section's source hash moves; only
	// compositionKey shifts. The prefix cache must miss.
	item.SetMessage(build(message.FinishReasonCanceled))
	canceledOut := item.Render(width)
	require.NotEqual(t, endTurnOut, canceledOut,
		"prefix cache must invalidate on composition-only change (finish reason)")

	// A fresh item built with the same final state must match the
	// cached item byte-for-byte — caching is invisible from the
	// outside and never serves stale output.
	fresh := NewAssistantMessageItem(&sty, build(message.FinishReasonCanceled)).(*AssistantMessageItem)
	require.Equal(t, fresh.Render(width), canceledOut,
		"cached output must equal a fresh render of the same final state")
}

// TestAssistantSectionCache_ThinkingBoxHeightSurvivesCacheHit guards
// click-detection geometry across thinking-section cache hits. The
// thinking box height feeds HandleMouseClick; it is recomputed
// inside renderThinking and must be restored from
// assistantSection.aux when the thinking cache hits. We render once
// to capture the original height, trigger a content-only change so
// thinkingKey stays identical (thinking text, expanded flag, and
// footer state all unchanged), render again, and assert the
// thinkingBoxHeight field is preserved.
func TestAssistantSectionCache_ThinkingBoxHeightSurvivesCacheHit(t *testing.T) {
	sty := styles.CharmtonePantera()
	const width = 71

	thinking := strings.Join([]string{
		"Considering the request.",
		"Looking at the relevant files.",
		"Drafting a plan.",
		"Verifying constraints.",
	}, "\n")
	msg := thinkingMessage("hbox", thinking, "initial answer")
	item := NewAssistantMessageItem(&sty, msg).(*AssistantMessageItem)
	item.thinkingViewMode = thinkingFullExpanded

	_ = item.RawRender(width)
	originalHeight := item.thinkingBoxHeight
	require.Greater(t, originalHeight, 0,
		"thinking box height must be populated after first render")

	// Stomp the field so a stale read (cache hit that fails to
	// restore aux) is detectable. Then trigger a content-only
	// change: thinkingKey is byte-identical between renders, so
	// the thinking section cache must hit and restore the
	// preserved height via assistantSection.aux.
	item.thinkingBoxHeight = -1
	updated := thinkingMessage("hbox", thinking, "initial answer with more streamed text")
	item.SetMessage(updated)
	_ = item.RawRender(width)

	require.Equal(t, originalHeight, item.thinkingBoxHeight,
		"thinkingBoxHeight must be preserved across thinking section cache hits "+
			"so HandleMouseClick keeps targeting the right rows")
}
