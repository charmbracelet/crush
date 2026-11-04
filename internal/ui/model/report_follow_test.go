package model

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func subagentReportMessage(id string) message.Message {
	return message.Message{
		ID:        id,
		Role:      message.User,
		SessionID: "s1",
		Parts: []message.ContentPart{
			message.SubagentReport{Name: "reviewer-1", Status: "completed", Output: "done"},
		},
	}
}

// A report arrives in the background, so it must not yank the view to the
// bottom while the user is scrolled up reading earlier output.
func TestAppendSessionMessage_ReportWhileScrolledUpKeepsFollowOff(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.chat.ScrollToTop()
	require.False(t, u.chat.Follow())
	offset := u.chat.Offset()

	u.appendSessionMessage(subagentReportMessage("r1"))

	require.NotNil(t, u.chat.MessageItem("r1"), "the report is still appended")
	require.False(t, u.chat.Follow(), "a report must not turn follow on")
	require.Equal(t, offset, u.chat.Offset(), "the viewport must stay where it was")
}

// A user already following the transcript keeps following when a report
// arrives.
func TestAppendSessionMessage_ReportWhileFollowingStaysAtBottom(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.chat.ScrollToBottom()
	require.True(t, u.chat.Follow())

	u.appendSessionMessage(subagentReportMessage("r1"))

	require.NotNil(t, u.chat.MessageItem("r1"))
	require.True(t, u.chat.Follow())
	require.True(t, u.chat.AtBottom())
}

// A prompt the user typed always brings the view to the bottom.
func TestAppendSessionMessage_TypedPromptScrollsToBottomAndFollows(t *testing.T) {
	t.Parallel()
	u := newFrameTestUI(t)
	u.chat.ScrollToTop()
	require.False(t, u.chat.Follow())

	u.appendSessionMessage(userMessage("u1", "s1", "hello"))

	require.NotNil(t, u.chat.MessageItem("u1"))
	require.True(t, u.chat.Follow())
	require.True(t, u.chat.AtBottom())
}
