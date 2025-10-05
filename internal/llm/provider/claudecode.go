package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/message"
)

type claudeCodeClient struct {
	providerOptions providerClientOptions
	cliPath         string
}

type ClaudeCodeClient ProviderClient

func newClaudeCodeClient(opts providerClientOptions) ClaudeCodeClient {
	// Find claude binary path
	cliPath, err := exec.LookPath("claude")
	if err != nil {
		// Fallback to common paths
		cliPath = "/usr/local/bin/claude"
	}

	return &claudeCodeClient{
		providerOptions: opts,
		cliPath:         cliPath,
	}
}

func (c *claudeCodeClient) Model() catwalk.Model {
	return c.providerOptions.model(c.providerOptions.modelType)
}

// ClaudeCodeStreamResponse represents the JSON output from claude --output-format stream-json
type ClaudeCodeStreamResponse struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype,omitempty"`
	Result  string `json:"result,omitempty"`
	Message struct {
		ID      string `json:"id"`
		Role    string `json:"role"`
		Content []struct {
			Type        string          `json:"type"`
			Text        string          `json:"text,omitempty"`
			ID          string          `json:"id,omitempty"`
			Name        string          `json:"name,omitempty"`
			Input       json.RawMessage `json:"input,omitempty"`
			ToolUseID   string          `json:"tool_use_id,omitempty"`
			Content     string          `json:"content,omitempty"`
			IsError     bool            `json:"is_error,omitempty"`
		} `json:"content"`
		Usage struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		} `json:"usage,omitempty"`
	} `json:"message,omitempty"`
}

func (c *claudeCodeClient) buildPrompt(messages []message.Message) string {
	var prompt strings.Builder

	for _, msg := range messages {
		switch msg.Role {
		case message.User:
			prompt.WriteString(msg.Content().String())
			prompt.WriteString("\n")
		case message.Assistant:
			// Include assistant messages as context
			prompt.WriteString("Assistant: ")
			prompt.WriteString(msg.Content().String())
			prompt.WriteString("\n")
		case message.Tool:
			// Include tool results
			prompt.WriteString("Tool result: ")
			prompt.WriteString(msg.Content().String())
			prompt.WriteString("\n")
		}
	}

	return prompt.String()
}

func (c *claudeCodeClient) send(ctx context.Context, messages []message.Message, toolsList []tools.BaseTool) (*ProviderResponse, error) {
	// Collect all output from streaming
	var fullContent strings.Builder
	var toolCalls []message.ToolCall
	var usage TokenUsage
	var finishReason message.FinishReason = message.FinishReasonEndTurn

	eventChan := c.stream(ctx, messages, toolsList)

	for event := range eventChan {
		if event.Error != nil {
			return nil, event.Error
		}

		switch event.Type {
		case EventContentDelta:
			fullContent.WriteString(event.Content)
		case EventToolUseStart, EventToolUseStop:
			if event.ToolCall != nil {
				toolCalls = append(toolCalls, *event.ToolCall)
			}
		case EventComplete:
			if event.Response != nil {
				usage = event.Response.Usage
				finishReason = event.Response.FinishReason
			}
		}
	}

	return &ProviderResponse{
		Content:      fullContent.String(),
		ToolCalls:    toolCalls,
		Usage:        usage,
		FinishReason: finishReason,
	}, nil
}

func (c *claudeCodeClient) stream(ctx context.Context, messages []message.Message, toolsList []tools.BaseTool) <-chan ProviderEvent {
	eventChan := make(chan ProviderEvent)

	go func() {
		defer close(eventChan)

		prompt := c.buildPrompt(messages)

		// Build claude command
		args := []string{
			"--print",
			"--output-format", "stream-json",
			"--verbose", // Required for stream-json format
			"--dangerously-skip-permissions",
		}

		// Add model if specified from the config
		model := c.Model()
		if model.ID != "" {
			args = append(args, "--model", model.ID)
		}

		// Add the prompt
		args = append(args, prompt)

		cmd := exec.CommandContext(ctx, c.cliPath, args...)
		// Set working directory to current directory so Claude Code has context
		cmd.Dir = c.providerOptions.config.ExtraParams["working_dir"]
		if cmd.Dir == "" {
			// Default to current working directory
			cmd.Dir = "."
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			eventChan <- ProviderEvent{
				Type:  EventError,
				Error: fmt.Errorf("failed to create stdout pipe: %w", err),
			}
			return
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			eventChan <- ProviderEvent{
				Type:  EventError,
				Error: fmt.Errorf("failed to create stderr pipe: %w", err),
			}
			return
		}

		if err := cmd.Start(); err != nil {
			eventChan <- ProviderEvent{
				Type:  EventError,
				Error: fmt.Errorf("failed to start claude CLI: %w", err),
			}
			return
		}

		// Read stderr in background for error reporting
		var stderrOutput strings.Builder
		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				line := scanner.Text()
				stderrOutput.WriteString(line)
				stderrOutput.WriteString("\n")
			}
		}()

		// Parse streaming JSON output
		scanner := bufio.NewScanner(stdout)
		var totalUsage TokenUsage

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			var streamResp ClaudeCodeStreamResponse
			if err := json.Unmarshal([]byte(line), &streamResp); err != nil {
				// Skip malformed JSON lines
				continue
			}

			switch streamResp.Type {
			case "system":
				// System initialization message - ignore
				continue

			case "assistant":
				// Assistant message with complete response
				eventChan <- ProviderEvent{
					Type: EventContentStart,
				}

				// Process message content (could be text, tool_use, or both)
				for _, content := range streamResp.Message.Content {
					switch content.Type {
					case "text":
						if content.Text != "" {
							eventChan <- ProviderEvent{
								Type:    EventContentDelta,
								Content: content.Text,
							}
						}

					case "tool_use":
						// Emit tool use start event
						eventChan <- ProviderEvent{
							Type: EventToolUseStart,
							ToolCall: &message.ToolCall{
								ID:       content.ID,
								Name:     content.Name,
								Input:    string(content.Input),
								Finished: false,
							},
						}
						// Emit tool use stop event (the call is complete)
						eventChan <- ProviderEvent{
							Type: EventToolUseStop,
							ToolCall: &message.ToolCall{
								ID:       content.ID,
								Name:     content.Name,
								Input:    string(content.Input),
								Finished: true,
							},
						}
					}
				}

				// Update usage stats
				if streamResp.Message.Usage.InputTokens > 0 {
					totalUsage.InputTokens = streamResp.Message.Usage.InputTokens
				}
				if streamResp.Message.Usage.OutputTokens > 0 {
					totalUsage.OutputTokens = streamResp.Message.Usage.OutputTokens
				}
				if streamResp.Message.Usage.CacheReadInputTokens > 0 {
					totalUsage.CacheReadTokens = streamResp.Message.Usage.CacheReadInputTokens
				}
				if streamResp.Message.Usage.CacheCreationInputTokens > 0 {
					totalUsage.CacheCreationTokens = streamResp.Message.Usage.CacheCreationInputTokens
				}

				eventChan <- ProviderEvent{
					Type: EventContentStop,
				}

			case "user":
				// User message contains tool results - emit as content
				for _, content := range streamResp.Message.Content {
					if content.Type == "tool_result" {
						// Display tool result as informational content
						eventChan <- ProviderEvent{
							Type:    EventContentDelta,
							Content: fmt.Sprintf("\n[Tool %s output: %s]\n", content.ToolUseID, content.Content),
						}
					}
				}

			case "result":
				// Final result message - extract the result text
				if streamResp.Result != "" && streamResp.Subtype == "success" {
					// Result already handled in assistant message
				}
			}
		}

		if err := scanner.Err(); err != nil && err != io.EOF {
			eventChan <- ProviderEvent{
				Type:  EventError,
				Error: fmt.Errorf("error reading claude output: %w", err),
			}
		}

		if err := cmd.Wait(); err != nil {
			errMsg := fmt.Sprintf("claude CLI failed: %v", err)
			if stderrOutput.Len() > 0 {
				errMsg = fmt.Sprintf("%s\nstderr: %s", errMsg, stderrOutput.String())
			}
			eventChan <- ProviderEvent{
				Type:  EventError,
				Error: fmt.Errorf("%s", errMsg),
			}
			return
		}

		// Send completion event
		eventChan <- ProviderEvent{
			Type: EventComplete,
			Response: &ProviderResponse{
				Usage:        totalUsage,
				FinishReason: message.FinishReasonEndTurn,
			},
		}
	}()

	return eventChan
}
