package acp

import (
	"context"
	"fmt"
	"github.com/charmbracelet/crush/internal/llm/prompt"
	"github.com/charmbracelet/crush/internal/message"
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
	done, err := agent.app.CoderAgent.Run(context.Background(), string(params.SessionId), prompt.Initialize())
	if err != nil {
		return err
	}

	events := agent.app.Messages.Subscribe(ctx)

	// track what we already sent
	var lastText, lastThinking string

	for {
		select {
		case <-done: // agent finished (success or cancel)
			return nil // error (if any) was already returned above

		case ev, ok := <-events:
			if !ok { // stream closed → agent finished
				return nil
			}
			if ev.Payload.Role != message.Assistant {
				continue
			}

			for _, part := range ev.Payload.Parts {
				switch p := part.(type) {
				case message.TextContent:
					if delta := p.Text[len(lastText):]; delta != "" {
						_ = agent.conn.SessionUpdate(ctx, acp.SessionNotification{
							SessionId: params.SessionId,
							Update: acp.SessionUpdate{
								AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{
									SessionUpdate: "agent_message_chunk",
									Content: acp.ContentBlock{
										Text: &acp.ContentBlockText{Type: "text", Text: delta},
									},
								},
							},
						})
						lastText = p.Text
					}

				case message.ReasoningContent:
					if delta := p.Thinking[len(lastThinking):]; delta != "" {
						_ = agent.conn.SessionUpdate(ctx, acp.SessionNotification{
							SessionId: params.SessionId,
							Update: acp.SessionUpdate{
								AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{
									SessionUpdate: "agent_thought_chunk",
									Content: acp.ContentBlock{
										Text: &acp.ContentBlockText{Type: "text", Text: delta},
									},
								},
							},
						})
						lastThinking = p.Thinking
					}
				}
			}
		}
	}
}
