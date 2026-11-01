package model

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestQueueItemRendersSubagentReport(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	report := message.SubAgentReport{
		Label:  "nancy-roadrunner-recheck",
		Output: "Verification complete.\n\nFinal verdict below.",
	}
	got := ansi.Strip(queueList([]string{report.PromptText()}, u.com.Styles))

	require.Equal(t, "Subagent Report → Nancy roadrunner-recheck", strings.TrimSpace(strings.TrimLeft(got, "•▶ ")))
}

func TestQueueItemKeepsMultilinePromptOnOneRow(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	got := ansi.Strip(queueList([]string{"first\nsecond", "third"}, u.com.Styles))
	require.Equal(t, 2, strings.Count(got, "\n")+1)
	require.Contains(t, got, "first second")
}

func TestExpandedPillsLeaveRoomForAttachments(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.attachments = attachments.New(attachments.NewRenderer(u.com.Styles.Attachments), attachments.Keymap{})
	u.session = &session.Session{ID: "s1"}
	u.promptQueue = 1
	u.promptQueueItems = []string{"queued"}
	u.pillsExpanded = true
	u.focusedPillSection = pillSectionQueue

	u.updateLayoutAndSize()
	without := u.layout.pills.Dy()
	u.changeAttachments(func() bool {
		return u.attachments.Update(message.Attachment{FileName: "paste_1.txt", MimeType: "text/plain", Content: []byte("x")})
	})
	require.Equal(t, without+1, u.layout.pills.Dy(), "adding an attachment relays out")
	require.Equal(t, u.pillsAreaHeight(), strings.Count(u.pillsView, "\n")+1)

	u.pillsExpanded = false
	require.Equal(t, pillHeightWithBorder, u.pillsAreaHeight(), "a closed list needs no gap")
}
