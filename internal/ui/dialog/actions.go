package dialog

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/memory"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/workspace"
)

// ActionClose is a message to close the current dialog.
type ActionClose struct{}

// ActionQuit is a message to quit the application.
type ActionQuit = tea.QuitMsg

// ActionOpenDialog is a message to open a dialog.
type ActionOpenDialog struct {
	DialogID string
}

// ActionOpenLMStudioSetup opens the custom LM Studio provider setup dialog.
type ActionOpenLMStudioSetup struct{}

// ActionSelectSession is a message indicating a session has been selected.
type ActionSelectSession struct {
	Session session.Session
}

// ActionSelectModel is a message indicating a model has been selected.
type ActionSelectModel struct {
	Provider       catwalk.Provider
	Model          config.SelectedModel
	ModelType      config.SelectedModelType
	ReAuthenticate bool
}

// ActionConfigureLMStudio is sent after the LM Studio setup dialog validates
// and discovers at least one model.
type ActionConfigureLMStudio struct {
	Provider config.ProviderConfig
	Model    config.SelectedModel
}

// Messages for commands
type (
	ActionNewSession        struct{}
	ActionToggleHelp        struct{}
	ActionToggleCompactMode struct{}
	ActionToggleThinking    struct{}
	ActionTogglePills       struct{}
	ActionExternalEditor    struct{}
	ActionToggleYoloMode    struct{}
	ActionSetAgentMode      struct {
		AgentID string
	}
	ActionActivateAgentMode struct {
		AgentID   string
		ModelType config.SelectedModelType
	}
	ActionToggleNotifications     struct{}
	ActionSelectNotificationStyle struct {
		Style string
	}
	ActionToggleTransparentBackground struct{}
	ActionInitializeProject           struct{}
	ActionSummarize                   struct {
		SessionID string
	}
	// ActionSelectReasoningEffort is a message indicating a reasoning effort
	// has been selected.
	ActionSelectReasoningEffort struct {
		Effort string
	}
	ActionPermissionResponse struct {
		Permission permission.PermissionRequest
		Action     PermissionAction
	}
	// ActionRunCustomCommand is a message to run a custom command.
	ActionRunCustomCommand struct {
		Content   string
		Arguments []commands.Argument
		Args      map[string]string // Actual argument values
		Skill     *skills.Skill     // Set when this is a skill command
	}
	// ActionAttachSkill is sent when a skill is selected from the commands
	// dialog to be attached to the conversation as a markdown attachment.
	ActionAttachSkill struct {
		ID   string
		Name string
	}
	// ActionRunMCPPrompt is a message to run a custom command.
	ActionRunMCPPrompt struct {
		Title       string
		Description string
		PromptID    string
		ClientID    string
		Arguments   []commands.Argument
		Args        map[string]string // Actual argument values
	}
	// ActionEnableDockerMCP is a message to enable Docker MCP.
	ActionEnableDockerMCP struct{}
	// ActionDisableDockerMCP is a message to disable Docker MCP.
	ActionDisableDockerMCP   struct{}
	ActionOpenMemoryRemember struct{}
	ActionMemoryRemember     struct {
		Args map[string]string
	}
	ActionMemorySetStatus struct {
		ID     string
		Status memory.Status
	}
	ActionMemorySetPinned struct {
		ID     string
		Pinned bool
	}
	ActionMemorySetFeature struct {
		Feature workspace.MemoryFeature
		Enabled bool
	}
	ActionMemorySetSessionRecording struct {
		SessionID string
		Enabled   bool
	}
	ActionMemoryMaintain struct{}
	ActionOpenSourceAdd  struct{}
	ActionSourceAdd      struct {
		Args map[string]string
	}
	ActionSourceRemove struct {
		ID string
	}
	ActionSourceView struct {
		Source session.Source
	}
)

// Messages for API key input dialog.
type (
	ActionChangeAPIKeyState struct {
		State APIKeyInputState
	}
	ActionChangeLMStudioSetupState struct {
		State LMStudioSetupState
	}
	ActionLMStudioSetupResult struct {
		Provider config.ProviderConfig
		Model    config.SelectedModel
		Error    error
	}
)

// Messages for OAuth2 device flow dialog.
type (
	// ActionInitiateOAuth is sent when the device auth is initiated
	// successfully.
	ActionInitiateOAuth struct {
		DeviceCode      string
		UserCode        string
		ExpiresIn       int
		VerificationURL string
		Interval        int
	}

	// ActionCompleteOAuth is sent when the device flow completes successfully.
	ActionCompleteOAuth struct {
		Token *oauth.Token
	}

	// ActionOAuthErrored is sent when the device flow encounters an error.
	ActionOAuthErrored struct {
		Error error
	}
)

// ActionCmd represents an action that carries a [tea.Cmd] to be passed to the
// Bubble Tea program loop.
type ActionCmd struct {
	Cmd tea.Cmd
}

// ActionFilePickerSelected is a message indicating a file has been selected in
// the file picker dialog.
type ActionFilePickerSelected struct {
	Path string
}

// ActionClipboardImageSelected is a message indicating the clipboard image
// option has been selected in the file picker dialog.
type ActionClipboardImageSelected struct{}

// Cmd returns a command that reads the file at path and sends a
// [message.Attachement] to the program.
func (a ActionFilePickerSelected) Cmd() tea.Cmd {
	path := a.Path
	if path == "" {
		return nil
	}
	return func() tea.Msg {
		isFileLarge, err := common.IsFileTooBig(path, common.MaxAttachmentSize)
		if err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  fmt.Sprintf("unable to read the image: %v", err),
			}
		}
		if isFileLarge {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  "file too large, max 5MB",
			}
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  fmt.Sprintf("unable to read the image: %v", err),
			}
		}

		mimeBufferSize := min(512, len(content))
		mimeType := http.DetectContentType(content[:mimeBufferSize])
		fileName := filepath.Base(path)

		return message.Attachment{
			FilePath: path,
			FileName: fileName,
			MimeType: mimeType,
			Content:  content,
		}
	}
}
