package enum

import (
	"errors"
	"image/color"

	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/tui/styles"
)

type ToolCallState string

const (
	// ToolCallStatePending Tool has been created but not yet started execution
	// e.g. multiple tool calls at once
	ToolCallStatePending ToolCallState = "pending"

	// ToolCallStatePermission Tool is in a Permissions related state
	ToolCallStatePermission ToolCallState = "permission"

	// ToolCallStateRunning Tool is actively executing
	ToolCallStateRunning ToolCallState = "running"

	// ToolCallStateCompleted Tool completed successfully
	ToolCallStateCompleted ToolCallState = "completed"

	// ToolCallStateFailed Tool failed during execution
	ToolCallStateFailed ToolCallState = "failed"

	// ToolCallStateCancelled Tool was explicitly cancelled by user
	ToolCallStateCancelled ToolCallState = "cancelled"
)

func (state ToolCallState) IsFinalState(permissionStatus permission.PermissionStatus) bool {
	return state == ToolCallStateCompleted ||
		state == ToolCallStateFailed ||
		state == ToolCallStateCancelled ||
		(state == ToolCallStatePermission && permissionStatus == permission.PermissionDenied)
}

func (state ToolCallState) IsNonFinalState(permissionStatus permission.PermissionStatus) bool {
	return !state.IsFinalState(permissionStatus)
}

func (state ToolCallState) ToIcon() string {
	switch state {
	case ToolCallStatePending:
		return styles.ToolPending
	case ToolCallStateRunning:
		return styles.ToolPending
	case ToolCallStatePermission:
		return styles.ToolPending
	case ToolCallStateCompleted:
		return styles.ToolSuccess
	case ToolCallStateCancelled:
		return styles.ToolPending
	case ToolCallStateFailed:
		return styles.ToolError
	default:
		//In case of unknown states we also return the error Icon
		return styles.ToolError
	}
}

func (state ToolCallState) ToColor() color.Color {
	t := styles.CurrentTheme()
	switch state {
	case ToolCallStatePending:
		//TODO: random color must be replace with some kind of Gray.
		return t.Info //TODO: not sure if this is a shade of gray
	case ToolCallStatePermission:
		return t.Paprika
	case ToolCallStateRunning:
		// TODO: I am for now sticking with GreenDark instead of Success since that was used before.
		return t.GreenDark //TODO consider: t.Success
	case ToolCallStateCompleted:
		return t.Green //TODO consider: t.Success
	case ToolCallStateCancelled:
		return t.FgMuted //TODO: consider: t.Error
	case ToolCallStateFailed:
		return t.RedDark //TODO: consider: t.Error
	default:
		//In case of unknown states we also return the error Icon
		return t.Error
	}
}

func (state ToolCallState) ToIconColored() string {
	t := styles.CurrentTheme()
	return t.S().Base.Foreground(state.ToColor()).Render(state.ToIcon())
}

func (state ToolCallState) FormatToolForCopy() string {
	switch state {
	case ToolCallStatePending:
		return "Pending..."
	case ToolCallStateRunning:
		return "Running..."
	case ToolCallStatePermission:
		return "Permissions..."
	case ToolCallStateCancelled:
		return "Cancelled"
	case ToolCallStateFailed:
		return "Failed"
	default:
		return ""
	}
}

func (state ToolCallState) renderTUIMessage(permissionStatus permission.PermissionStatus) (string, error) {
	//TODO: revisit logic, now that we have more ToolCallStates
	switch state {
	case ToolCallStateFailed:
		return "Tool call failed.", nil
	case ToolCallStateCancelled:
		return "Done", nil
	case ToolCallStateCompleted:
		return "Completed.", nil
	case ToolCallStateRunning:
		return "Running...", nil
	case ToolCallStatePending:
		return "Waiting for tool to start...", nil
	case ToolCallStatePermission:
		return permissionStatus.ToMessage(), nil
	default:
		return "", errors.New("unknown state: tool call related rendering issue")
	}
}

func (state ToolCallState) RenderTUIMessageColored(permissionStatus permission.PermissionStatus) (string, error) {
	t := styles.CurrentTheme()
	messageBaseStyle := t.S().Base.Foreground(t.FgSubtle)
	message, err := state.renderTUIMessage(permissionStatus)
	if err != nil {
		return "", err
	}
	// TODO: make the background light gray in case of ToolCallStateCompleted, see: https://github.com/charmbracelet/crush/pull/1385#issuecomment-3504123709
	return messageBaseStyle.Render(message), nil
}
