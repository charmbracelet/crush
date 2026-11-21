package proto

import (
	"time"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
)

// RunActivityType identifies a user-visible phase of an agent run.
type RunActivityType string

const (
	RunActivityThinkingStarted  RunActivityType = "thinking_started"
	RunActivityThinkingFinished RunActivityType = "thinking_finished"
	RunActivityToolStarted      RunActivityType = "tool_started"
	RunActivityToolFinished     RunActivityType = "tool_finished"
)

// RunActivity describes a state transition for non-interactive clients. The
// duration is in nanoseconds. Reasoning text and tool result contents are
// intentionally omitted.
type RunActivity struct {
	SessionID        string          `json:"session_id"`
	RunID            string          `json:"run_id"`
	MessageID        string          `json:"message_id"`
	Type             RunActivityType `json:"type"`
	ThinkingDuration time.Duration   `json:"thinking_duration,omitempty"`
	ToolCall         *ToolCall       `json:"tool_call,omitempty"`
	ToolResult       *ToolResult     `json:"tool_result,omitempty"`
	Cancelled        bool            `json:"cancelled,omitempty"`
}

func RunActivityToProto(activity notify.RunActivity) RunActivity {
	out := RunActivity{
		SessionID:        activity.SessionID,
		RunID:            activity.RunID,
		MessageID:        activity.MessageID,
		Type:             RunActivityType(activity.Type),
		ThinkingDuration: activity.ThinkingDuration,
		Cancelled:        activity.Cancelled,
	}
	if activity.Type == notify.RunActivityToolStarted || activity.Type == notify.RunActivityToolFinished {
		out.ToolCall = &ToolCall{
			ID:       activity.ToolCall.ID,
			Name:     activity.ToolCall.Name,
			Input:    activity.ToolCall.Input,
			Finished: activity.ToolCall.Finished,
		}
	}
	if activity.Type == notify.RunActivityToolFinished {
		out.ToolResult = &ToolResult{
			ToolCallID: activity.ToolResult.ToolCallID,
			Name:       activity.ToolResult.Name,
			IsError:    activity.ToolResult.IsError,
		}
	}
	return out
}

func RunActivitiesToProto(activities []notify.RunActivity) []RunActivity {
	if len(activities) == 0 {
		return nil
	}
	out := make([]RunActivity, len(activities))
	for i, activity := range activities {
		out[i] = RunActivityToProto(activity)
	}
	return out
}

func RunActivityFromProto(activity RunActivity) notify.RunActivity {
	out := notify.RunActivity{
		SessionID:        activity.SessionID,
		RunID:            activity.RunID,
		MessageID:        activity.MessageID,
		Type:             notify.RunActivityType(activity.Type),
		ThinkingDuration: activity.ThinkingDuration,
		Cancelled:        activity.Cancelled,
	}
	if activity.ToolCall != nil {
		out.ToolCall = message.ToolCall{
			ID:       activity.ToolCall.ID,
			Name:     activity.ToolCall.Name,
			Input:    activity.ToolCall.Input,
			Finished: activity.ToolCall.Finished,
		}
	}
	if activity.ToolResult != nil {
		out.ToolResult = message.ToolResult{
			ToolCallID: activity.ToolResult.ToolCallID,
			Name:       activity.ToolResult.Name,
			IsError:    activity.ToolResult.IsError,
		}
	}
	return out
}

func RunActivitiesFromProto(activities []RunActivity) []notify.RunActivity {
	if len(activities) == 0 {
		return nil
	}
	out := make([]notify.RunActivity, len(activities))
	for i, activity := range activities {
		out[i] = RunActivityFromProto(activity)
	}
	return out
}
