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
		return styles.ToolCancel
	case ToolCallStateFailed:
		return styles.ToolError
	default:
		// In case of unknown states we also return the error Icon
		return styles.ToolError
	}
}

func (state ToolCallState) ToColor() color.Color {
	t := styles.CurrentTheme()
	switch state {
	case ToolCallStatePending:
		// TODO: random color must be replace with some kind of Gray.
		return t.Info // TODO: not sure if this is a shade of gray
	case ToolCallStatePermission:
		return t.Paprika
	case ToolCallStateRunning:
		return t.GreenDark // Use darker green for active running state
	case ToolCallStateCompleted:
		return t.Green // Use bright green for successful completion
	case ToolCallStateCancelled:
		return t.FgMuted // Muted is appropriate for user-initiated cancellation
	case ToolCallStateFailed:
		return t.Error // Use error color for failed operations
	default:
		// In case of unknown states we also return the error Icon
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
	case ToolCallStateCompleted:
		return "" // Final states don't need status messages
	default:
		return ""
	}
}

func (state ToolCallState) renderTUIMessage(permissionStatus permission.PermissionStatus) (string, error) {
	// TODO: revisit logic, now that we have more ToolCallStates
	switch state {
	case ToolCallStateFailed:
		return "Tool call failed.", nil
	case ToolCallStateCancelled:
		return "Cancelled.", nil
	case ToolCallStateCompleted:
		return "Done", nil
	case ToolCallStateRunning:
		return "Running...", nil
	case ToolCallStatePending:
		return "Waiting for tool to start...", nil
	case ToolCallStatePermission:
		return permissionStatus.ToMessage()
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

	switch state {
	case ToolCallStateFailed:
		{
			messageBaseStyle = messageBaseStyle.Padding(0, 1).Background(t.Red).Foreground(t.White)
			//TODO: ERROR content? Most likely not in this function.
			//err := strings.ReplaceAll(v.result.Content, "\n", " ")
			//err = fmt.Sprintf("%s %s", errTag, t.S().Base.Foreground(t.FgHalfMuted).Render(v.fit(err, v.textWidth()-2-lipgloss.Width(errTag))))
		}
	case ToolCallStateCompleted:
		{
			// This should make the background light gray in case of ToolCallStateCompleted, see: https://github.com/charmbracelet/crush/pull/1385#issuecomment-3504123709
			messageBaseStyle = messageBaseStyle.Padding(0, 1).Background(t.BgBaseLighter).Foreground(t.FgSubtle)
		}

	}

	return messageBaseStyle.Render(message), nil
}
