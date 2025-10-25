package acp

import (
	"context"
	"fmt"
	"github.com/charmbracelet/crush/internal/llm/prompt"
	"github.com/coder/acp-go-sdk"
)

type slashCmd interface {
	Help() string
	Exec(ctx context.Context, agent *Agent, text string, params acp.PromptRequest) error
}

var slashRegistry = map[string]slashCmd{
	"yolo": yoloCmd{},
	"init": initCmd{},
}

type yoloCmd struct{}

func (yoloCmd) Help() string { return "Toggle Yolo Mode" }
func (yoloCmd) Exec(ctx context.Context, agent *Agent, text string, params acp.PromptRequest) error {
	agent.app.Permissions.SetSkipRequests(!agent.app.Permissions.SkipRequests())
	status := agent.app.Permissions.SkipRequests()

	return agent.conn.SessionUpdate(ctx, acp.SessionNotification{
		SessionId: params.SessionId,
		Update: acp.SessionUpdate{
			AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{
				SessionUpdate: "agent_message_chunk", // TODO: double check this type for such cases - no LLM runs or tools
				Content: acp.ContentBlock{
					Text: &acp.ContentBlockText{
						Text: fmt.Sprintf("YOLO mode is now **%s**.", map[bool]string{
							true:  "ON",
							false: "OFF",
						}[status]),
						Type: "text",
					},
				},
			},
		},
	})
}

type initCmd struct{}

func (initCmd) Help() string { return "Initialize Project" }
func (initCmd) Exec(ctx context.Context, agent *Agent, text string, params acp.PromptRequest) error {
	return agent.streamAgentRun(ctx, params.SessionId, prompt.Initialize(), true)
}
