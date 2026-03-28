package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestNextAutoModePromptType_NoPriorReminderStartsWithFull(t *testing.T) {
	t.Parallel()

	promptType, ok := nextAutoModePromptType(nil)
	require.True(t, ok)
	require.Equal(t, message.AutoModePromptTypeFull, promptType)
}

func TestNextAutoModePromptType_RespectsReminderTurnInterval(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		{
			Role: message.System,
			Parts: []message.ContentPart{
				message.TextContent{Text: message.AutoModePromptContent(message.AutoModePromptTypeFull)},
			},
		},
	}
	for range autoModeReminderTurnsBetween - 1 {
		msgs = append(msgs, assistantMsg("progress"))
	}

	promptType, ok := nextAutoModePromptType(msgs)
	require.False(t, ok)
	require.Equal(t, message.AutoModePromptType(""), promptType)
}

func TestNextAutoModePromptType_UsesSparseAfterInterval(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		{
			Role: message.System,
			Parts: []message.ContentPart{
				message.TextContent{Text: message.AutoModePromptContent(message.AutoModePromptTypeFull)},
			},
		},
	}
	for range autoModeReminderTurnsBetween {
		msgs = append(msgs, assistantMsg("progress"))
	}

	promptType, ok := nextAutoModePromptType(msgs)
	require.True(t, ok)
	require.Equal(t, message.AutoModePromptTypeSparse, promptType)
}

func TestNextAutoModePromptType_FullEveryFifthReminder(t *testing.T) {
	t.Parallel()

	msgs := make([]message.Message, 0)
	types := []message.AutoModePromptType{
		message.AutoModePromptTypeFull,
		message.AutoModePromptTypeSparse,
		message.AutoModePromptTypeSparse,
		message.AutoModePromptTypeSparse,
		message.AutoModePromptTypeSparse,
	}
	for _, promptType := range types {
		msgs = append(msgs, message.Message{
			Role: message.System,
			Parts: []message.ContentPart{
				message.TextContent{Text: message.AutoModePromptContent(promptType)},
			},
		})
	}
	for range autoModeReminderTurnsBetween {
		msgs = append(msgs, assistantMsg("progress"))
	}

	promptType, ok := nextAutoModePromptType(msgs)
	require.True(t, ok)
	require.Equal(t, message.AutoModePromptTypeFull, promptType)
}

func TestNextAutoModePromptType_ExitResetsReminderCycle(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		{
			Role: message.System,
			Parts: []message.ContentPart{
				message.TextContent{Text: message.AutoModePromptContent(message.AutoModePromptTypeFull)},
			},
		},
		assistantMsg("progress"),
		{
			Role: message.System,
			Parts: []message.ContentPart{
				message.TextContent{Text: message.AutoModePromptContent(message.AutoModePromptTypeExit)},
			},
		},
	}

	promptType, ok := nextAutoModePromptType(msgs)
	require.True(t, ok)
	require.Equal(t, message.AutoModePromptTypeFull, promptType)
}

func TestFilterAutoModePromptMessages_DropsPromptMarkersOutsideAuto(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		autoPromptMsg(message.AutoModePromptTypeFull),
		assistantMsg("progress"),
		autoPromptMsg(message.AutoModePromptTypeExit),
	}

	filtered := filterAutoModePromptMessages(msgs, session.PermissionModeDefault)
	require.Len(t, filtered, 1)
	require.Equal(t, message.Assistant, filtered[0].Role)
}

func TestFilterAutoModePromptMessages_KeepsOnlyLatestActiveMarkerInAuto(t *testing.T) {
	t.Parallel()

	msgs := []message.Message{
		autoPromptMsg(message.AutoModePromptTypeFull),
		assistantMsg("first"),
		autoPromptMsg(message.AutoModePromptTypeSparse),
		assistantMsg("second"),
		autoPromptMsg(message.AutoModePromptTypeExit),
		autoPromptMsg(message.AutoModePromptTypeFull),
	}

	filtered := filterAutoModePromptMessages(msgs, session.PermissionModeAuto)
	require.Len(t, filtered, 3)
	promptType, ok := message.ParseAutoModePrompt(filtered[2])
	require.True(t, ok)
	require.Equal(t, message.AutoModePromptTypeFull, promptType)
}

func autoPromptMsg(promptType message.AutoModePromptType) message.Message {
	return message.Message{
		Role: message.System,
		Parts: []message.ContentPart{
			message.TextContent{Text: message.AutoModePromptContent(promptType)},
		},
	}
}

func assistantMsg(text string) message.Message {
	return message.Message{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: text},
		},
	}
}
