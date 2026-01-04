package acp

import (
	"log/slog"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/uicmd"
	"github.com/coder/acp-go-sdk"
)

const (
	systemCommandPrefix = "system:"
	mcpCommandPrefix    = "mcp:"
)

// HandleMCPEvent processes MCP events and republishes commands when prompts
// change.
func (s *Sink) HandleMCPEvent(event pubsub.Event[mcp.Event]) {
	switch event.Payload.Type {
	case mcp.EventPromptsListChanged, mcp.EventStateChanged:
		s.PublishCommands()
	}
}

// PublishCommands aggregates commands from all sources and sends an
// AvailableCommandsUpdate to the ACP client.
func (s *Sink) PublishCommands() {
	var commands []acp.AvailableCommand

	// System/built-in commands.
	commands = append(commands, s.builtinCommands()...)

	// User and project commands (already prefixed by uicmd).
	if userCmds, err := uicmd.LoadCustomCommandsFromConfig(config.Get()); err == nil {
		commands = append(commands, translateCommands(userCmds, "")...)
	}

	// MCP prompts.
	mcpCmds := uicmd.LoadMCPPrompts()
	commands = append(commands, translateCommands(mcpCmds, mcpCommandPrefix)...)

	if err := s.conn.SessionUpdate(s.ctx, acp.SessionNotification{
		SessionId: acp.SessionId(s.sessionID),
		Update: acp.SessionUpdate{
			AvailableCommandsUpdate: &acp.SessionAvailableCommandsUpdate{
				AvailableCommands: commands,
			},
		},
	}); err != nil {
		slog.Error("Failed to send available commands update", "error", err)
	}
}

// builtinCommands returns ACP-compatible built-in commands.
func (s *Sink) builtinCommands() []acp.AvailableCommand {
	return []acp.AvailableCommand{
		{
			Name:        systemCommandPrefix + "new_session",
			Description: "Start a new session",
		},
		{
			Name:        systemCommandPrefix + "switch_session",
			Description: "Switch to a different session",
		},
		{
			Name:        systemCommandPrefix + "switch_model",
			Description: "Switch to a different model",
		},
		{
			Name:        systemCommandPrefix + "summarize",
			Description: "Summarize the current session and create a new one with the summary",
		},
		{
			Name:        systemCommandPrefix + "toggle_thinking",
			Description: "Toggle model thinking for reasoning-capable models",
		},
		{
			Name:        systemCommandPrefix + "toggle_yolo",
			Description: "Toggle yolo mode (auto-approve tool calls)",
		},
		{
			Name:        systemCommandPrefix + "help",
			Description: "Show available commands and shortcuts",
		},
	}
}

// translateCommands converts uicmd.Command slice to acp.AvailableCommand
// slice, optionally adding a prefix.
func translateCommands(cmds []uicmd.Command, prefix string) []acp.AvailableCommand {
	result := make([]acp.AvailableCommand, 0, len(cmds))
	for _, cmd := range cmds {
		acpCmd := acp.AvailableCommand{
			Name:        prefix + cmd.ID,
			Description: cmd.Description,
		}

		// If the command has a title different from ID, use it as a hint.
		if cmd.Title != "" && cmd.Title != cmd.ID {
			acpCmd.Input = &acp.AvailableCommandInput{
				UnstructuredCommandInput: &acp.AvailableCommandUnstructuredCommandInput{
					Hint: cmd.Title,
				},
			}
		}

		result = append(result, acpCmd)
	}
	return result
}
