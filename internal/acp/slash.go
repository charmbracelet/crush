package acp

import (
	"context"
	"fmt"
	"github.com/charmbracelet/crush/internal/llm/prompt"
	"github.com/coder/acp-go-sdk"
)

type SlashCommand interface {
	Name() string
	Help() string
	Exec(ctx context.Context, agent *Agent, text string, params acp.PromptRequest) error
}

type SlashCommandRegistry []SlashCommand

func (r SlashCommandRegistry) Get(name string) SlashCommand {
	for _, cmd := range defaultSlashCommands {
		if cmd.Name() == name {
			return cmd
		}
	}

	return nil
}

// AvailableCommands generates a slice of acp.AvailableCommand from a slice of SlashCommand
func AvailableCommands(commands SlashCommandRegistry) []acp.AvailableCommand {
	out := make([]acp.AvailableCommand, 0, len(commands))
	for _, cmd := range commands {
		out = append(out, acp.AvailableCommand{
			Name: cmd.Name(),
			Input: &acp.AvailableCommandInput{
				&acp.UnstructuredCommandInput{
					Hint: cmd.Help(),
				},
			},
		})
	}
	return out
}

var defaultSlashCommands = SlashCommandRegistry{
	yoloCmd{},
	initCmd{},
}

type yoloCmd struct{}

func (yoloCmd) Name() string { return "yolo" }
func (yoloCmd) Help() string { return "Toggle Yolo Mode" }
func (yoloCmd) Exec(ctx context.Context, agent *Agent, text string, params acp.PromptRequest) error {
	agent.app.Permissions.SetSkipRequests(!agent.app.Permissions.SkipRequests())
	status := agent.app.Permissions.SkipRequests()

	return agent.conn.SessionUpdate(ctx, acp.SessionNotification{
		SessionId: params.SessionId,
		Update: acp.UpdateAgentMessage(acp.ContentBlock{
			Text: &acp.ContentBlockText{
				Text: fmt.Sprintf("YOLO mode is now **%s**.", map[bool]string{
					true:  "ON",
					false: "OFF",
				}[status]),
				Type: "text",
			},
		}),
	})
}

type initCmd struct{}

func (initCmd) Name() string { return "init" }
func (initCmd) Help() string { return "Initialize Project" }
func (initCmd) Exec(ctx context.Context, agent *Agent, text string, params acp.PromptRequest) error {
	return agent.RunPrompt(ctx, prompt.Initialize(), params)
}
