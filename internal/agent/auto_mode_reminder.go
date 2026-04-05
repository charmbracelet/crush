package agent

import (
	"context"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
)

const (
	autoModeReminderTurnsBetween = 5
	autoModeFullReminderEvery    = 5
)

func (c *coordinator) maybeAppendAutoModeReminder(ctx context.Context, sessionID string, permissionMode session.PermissionMode) error {
	if permissionMode != session.PermissionModeAuto {
		return nil
	}

	msgs, err := c.messages.List(ctx, sessionID)
	if err != nil {
		return err
	}

	promptType, ok := nextAutoModePromptType(msgs)
	if !ok {
		return nil
	}

	_, err = c.messages.Create(ctx, sessionID, message.NewAutoModePromptMessage(promptType))
	return err
}

func nextAutoModePromptType(msgs []message.Message) (message.AutoModePromptType, bool) {
	reminderCount := 0
	assistantTurnsSinceLastReminder := 0
	foundReminder := false

	for _, msg := range msgs {
		if promptType, ok := message.ParseAutoModePrompt(msg); ok {
			switch promptType {
			case message.AutoModePromptTypeExit:
				reminderCount = 0
				assistantTurnsSinceLastReminder = 0
				foundReminder = false
			case message.AutoModePromptTypeFull, message.AutoModePromptTypeSparse:
				reminderCount++
				assistantTurnsSinceLastReminder = 0
				foundReminder = true
			}
			continue
		}

		if msg.Role == message.Assistant && isMeaningfulAssistantTurn(msg) {
			assistantTurnsSinceLastReminder++
		}
	}

	if !foundReminder {
		return message.AutoModePromptTypeFull, true
	}
	if assistantTurnsSinceLastReminder < autoModeReminderTurnsBetween {
		return "", false
	}
	if (reminderCount+1)%autoModeFullReminderEvery == 1 {
		return message.AutoModePromptTypeFull, true
	}
	return message.AutoModePromptTypeSparse, true
}

func filterAutoModePromptMessages(msgs []message.Message, permissionMode session.PermissionMode) []message.Message {
	if len(msgs) == 0 {
		return nil
	}

	pendingPromptIndex := latestPendingAutoModePromptIndex(msgs)

	filtered := make([]message.Message, 0, len(msgs))
	for i, msg := range msgs {
		if promptType, ok := message.ParseAutoModePrompt(msg); !ok {
			filtered = append(filtered, msg)
			continue
		} else if i == pendingPromptIndex && (promptType != message.AutoModePromptTypeExit || permissionMode == session.PermissionModeAuto) {
			filtered = append(filtered, msg)
		}
	}
	return filtered
}

func pendingAutoModePromptText(msgs []message.Message, permissionMode session.PermissionMode) (string, bool) {
	if permissionMode != session.PermissionModeAuto && permissionMode != session.PermissionModeDefault {
		return "", false
	}
	index := latestPendingAutoModePromptIndex(msgs)
	if index < 0 {
		return "", false
	}
	promptType, ok := message.ParseAutoModePrompt(msgs[index])
	if !ok {
		return "", false
	}
	if permissionMode == session.PermissionModeAuto && promptType == message.AutoModePromptTypeExit {
		return "", false
	}
	return message.AutoModePromptSystemText(promptType), true
}

func latestPendingAutoModePromptIndex(msgs []message.Message) int {
	lastPromptIndex := -1
	hasAssistantTurnAfterPrompt := false

	for i, msg := range msgs {
		if _, ok := message.ParseAutoModePrompt(msg); ok {
			lastPromptIndex = i
			hasAssistantTurnAfterPrompt = false
			continue
		}
		if lastPromptIndex >= 0 && msg.Role == message.Assistant && isMeaningfulAssistantTurn(msg) {
			hasAssistantTurnAfterPrompt = true
		}
	}

	if lastPromptIndex < 0 || hasAssistantTurnAfterPrompt {
		return -1
	}
	return lastPromptIndex
}

func isMeaningfulAssistantTurn(msg message.Message) bool {
	return msg.Content().Text != "" ||
		msg.ReasoningContent().Thinking != "" ||
		len(msg.ToolCalls()) > 0 ||
		msg.FinishPart() != nil
}
