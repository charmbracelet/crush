package chat

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/compaction"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func checkpointItem(t *testing.T, sty *styles.Styles, info *compaction.Info, body string, finished bool) *AssistantMessageItem {
	t.Helper()

	parts := []message.ContentPart{message.TextContent{Text: body}}
	if info != nil {
		parts[0] = message.TextContent{Text: body + "\n\n" + info.Render()}
	}
	if finished {
		parts = append(parts, message.Finish{Reason: message.FinishReasonEndTurn, Time: 1})
	}
	msg := &message.Message{
		ID:               "checkpoint",
		Role:             message.Assistant,
		Parts:            parts,
		IsSummaryMessage: true,
	}
	item := NewAssistantMessageItem(sty, msg)
	return item.(*AssistantMessageItem)
}

// footedItem builds a finished summary message carrying an arbitrary footer, so
// the fallback can be exercised with text this build would never write.
func footedItem(t *testing.T, sty *styles.Styles, footer string) *AssistantMessageItem {
	t.Helper()

	body := "Summary\n\ncheckpoint prose."
	if footer != "" {
		body += "\n\n" + footer
	}
	msg := &message.Message{
		ID:   "checkpoint",
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: body},
			message.Finish{Reason: message.FinishReasonEndTurn, Time: 1},
		},
		IsSummaryMessage: true,
	}
	return NewAssistantMessageItem(sty, msg).(*AssistantMessageItem)
}

var sampleInfo = compaction.Info{
	ReplacedMessages: 332,
	ReplacedTokens:   756_906,
	KeptMessages:     231,
	KeptTokens:       573_814,
	TranscriptPath:   "/var/tmp/region.txt",
}

// TestCompactionCardSaysWhatItIs is the point of the box: a checkpoint in the
// middle of a conversation otherwise reads as another reply about the last
// message, when it is actually a stand-in for everything above it.
func TestCompactionCardSaysWhatItIs(t *testing.T) {
	sty := styles.CharmtonePantera()
	item := checkpointItem(t, &sty, &sampleInfo, "# Session Checkpoint\n\nShip the parser.", true)

	// Styling is dropped because the markdown renderer splits prose freely
	// across spans; the assertions are about what a person reads.
	out := ansi.Strip(item.RawRender(76))

	require.Contains(t, out, "Session Summary")
	require.Contains(t, out, "Replaced 332 earlier messages (~756.9K tokens)")
	require.Contains(t, out, "Kept the 231 most recent (~573.8K) verbatim")
	require.Contains(t, out, "Full transcript: /var/tmp/region.txt")
	require.Contains(t, out, "Ship the parser", "the checkpoint itself still has to be readable")
	require.NotContains(t, out, "compaction_info",
		"the footer is machine-readable bookkeeping, not something to show a person")
}

// TestCompactionCardIsBoxed checks the card is a card: a rounded frame around
// the content, the same treatment as the plan card.
func TestCompactionCardIsBoxed(t *testing.T) {
	sty := styles.CharmtonePantera()
	item := checkpointItem(t, &sty, &sampleInfo, "checkpoint body", true)

	rendered := item.RawRender(76)
	out := ansi.Strip(rendered)

	require.Contains(t, out, "╭")
	require.Contains(t, out, "╯")
	require.LessOrEqual(t, lipgloss.Width(rendered), cappedMessageWidth(76),
		"a checkpoint card must fit the message column")
	require.Equal(t, rendered, item.RawRender(76), "a cached render must be byte-stable")
}

// TestCompactionCardStreamsAsAnOpenBox mirrors the plan card: no bottom edge
// until the checkpoint is finished, so an unfinished summary is never
// presented as settled.
func TestCompactionCardStreamsAsAnOpenBox(t *testing.T) {
	sty := styles.CharmtonePantera()
	item := checkpointItem(t, &sty, nil, "writing the checkpoint", false)

	out := ansi.Strip(item.RawRender(76))

	require.Contains(t, out, "Summarizing")
	require.NotContains(t, out, "╯", "an open box has no bottom border")
	require.NotContains(t, out, "Replaced ", "nothing has been replaced until the footer lands")
}

// TestCompactionCardWithoutFooter keeps a checkpoint written before the footer
// existed legible as one, rather than falling back to plain assistant text.
func TestCompactionCardWithoutFooter(t *testing.T) {
	sty := styles.CharmtonePantera()
	item := checkpointItem(t, &sty, nil, "an older checkpoint", true)

	out := ansi.Strip(item.RawRender(76))

	require.Contains(t, out, "Session Summary")
	require.Contains(t, out, "╯")
	require.NotContains(t, out, "Replaced ")
}

func TestFormatTokenCount(t *testing.T) {
	t.Parallel()

	require.Equal(t, "0", formatTokenCount(0))
	require.Equal(t, "999", formatTokenCount(999))
	require.Equal(t, "1K", formatTokenCount(1000))
	require.Equal(t, "8.2K", formatTokenCount(8192))
	require.Equal(t, "1M", formatTokenCount(1_000_000))
	require.Equal(t, "1.5M", formatTokenCount(1_500_000))
}

// TestCompactionCardStripsOnlyTheFooter keeps the machine footer out of the
// card while the checkpoint's own text survives: the card states the same
// numbers in its own words, and showing both would read as a doubled note.
func TestCompactionCardStripsOnlyTheFooter(t *testing.T) {
	sty := styles.CharmtonePantera()
	item := checkpointItem(t, &sty, &sampleInfo, "Summary\n\nThe deploy pin is harbor-lantern.", true)

	out := ansi.Strip(item.RawRender(76))

	require.Contains(t, out, "harbor-lantern")
	require.NotContains(t, out, "This checkpoint replaces",
		"the footer prose is superseded by the card's note")
	require.NotContains(t, out, "read that file instead of guessing",
		"the instruction to the model is not a message to the user")
}

// TestCompactionCardDoesNotInventCounts covers a footer that is present but
// unreadable - reworded by a future build, cut short, or imitated - where the
// prose fallback recovers nothing. Zeros would be a statement about the session.
func TestCompactionCardDoesNotInventCounts(t *testing.T) {
	sty := styles.CharmtonePantera()
	item := footedItem(t, &sty, "<compaction_info>\nsomething changed here\n</compaction_info>")

	out := ansi.Strip(item.RawRender(76))

	require.Contains(t, out, "Session Summary")
	require.NotContains(t, out, "Replaced 0")
	require.NotContains(t, out, "Kept the 0")
}

// TestCompactionCardShowsWhatSurvivesOfAPartialFooter: the counts are reported
// independently, so a footer that only got as far as the first sentence says
// that much and nothing more.
func TestCompactionCardShowsWhatSurvivesOfAPartialFooter(t *testing.T) {
	sty := styles.CharmtonePantera()
	item := footedItem(t, &sty, "<compaction_info>\n"+
		"This checkpoint replaces 12 earlier messages (~4000 tokens).\n"+
		"</compaction_info>")

	out := ansi.Strip(item.RawRender(76))

	require.Contains(t, out, "Replaced 12 earlier messages (~4K tokens)")
	require.NotContains(t, out, "Kept the")
}
