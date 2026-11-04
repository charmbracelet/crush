package message

import (
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestSubagentReportAccessor(t *testing.T) {
	t.Parallel()

	report := SubagentReport{Name: "code-reviewer-1", Status: "completed", Output: "ok"}

	t.Run("returns the report part", func(t *testing.T) {
		t.Parallel()

		msg := &Message{Role: User, Parts: []ContentPart{report}}
		got, ok := msg.SubagentReport()
		require.True(t, ok)
		require.Equal(t, report, got)
	})

	t.Run("reports absence", func(t *testing.T) {
		t.Parallel()

		msg := &Message{Role: User, Parts: []ContentPart{TextContent{Text: "hi"}}}
		_, ok := msg.SubagentReport()
		require.False(t, ok)
	})
}

func TestIsPromptExcludesSubagentReports(t *testing.T) {
	t.Parallel()

	typed := &Message{Role: User, Parts: []ContentPart{TextContent{Text: "hi"}}}
	require.True(t, typed.IsPrompt())

	report := &Message{Role: User, Parts: []ContentPart{
		SubagentReport{Name: "a-1", Status: "completed", Output: "done"},
	}}
	require.False(t, report.IsPrompt())
}

func TestToAIMessageSubagentReport(t *testing.T) {
	t.Parallel()

	msg := &Message{Role: User, Parts: []ContentPart{
		SubagentReport{Name: "code-reviewer-2", Status: "failed", Output: "boom"},
	}}

	got := msg.ToAIMessage()
	require.Len(t, got, 1)
	require.Equal(t, fantasy.MessageRoleUser, got[0].Role)
	require.Len(t, got[0].Content, 1)
	text, ok := got[0].Content[0].(fantasy.TextPart)
	require.True(t, ok)
	require.Equal(t,
		"<subagent-report name=\"code-reviewer-2\" status=\"failed\">\nboom\n</subagent-report>",
		text.Text,
	)
}
