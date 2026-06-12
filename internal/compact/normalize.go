package compact

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/crush/internal/message"
)

// Normalize converts a slice of Crush messages into a flat sequence of
// NormalizedBlocks. Each message may produce zero or more blocks depending
// on its parts (an assistant message with text + tool calls produces
// multiple blocks). A second pass correlates bash commands from ToolCall
// args into their corresponding BlockBash entries.
func Normalize(msgs []message.Message) []NormalizedBlock {
	var blocks []NormalizedBlock
	for i, msg := range msgs {
		blocks = append(blocks, normalizeOne(msg, i)...)
	}
	correlateBashCommands(blocks)
	return blocks
}

// correlateBashCommands fills in the Command field of BlockBash entries by
// scanning backwards for the matching BlockToolCall{Name: "bash"}. Crush
// stores the command in the ToolCall args and the output in the ToolResult;
// pi-vcc's model embeds both in one message, so we bridge the gap here.
func correlateBashCommands(blocks []NormalizedBlock) {
	for i := range blocks {
		if blocks[i].Kind != BlockBash {
			continue
		}
		for j := i - 1; j >= 0 && j >= i-4; j-- {
			if blocks[j].Kind == BlockToolCall && blocks[j].Name == "bash" {
				blocks[i].Command = blocks[j].Args["command"]
				break
			}
		}
	}
}

func normalizeOne(msg message.Message, msgIndex int) []NormalizedBlock {
	switch msg.Role {
	case message.User:
		return normalizeUser(msg, msgIndex)
	case message.Assistant:
		return normalizeAssistant(msg, msgIndex)
	case message.Tool:
		return normalizeTool(msg, msgIndex)
	default:
		return nil
	}
}

func normalizeUser(msg message.Message, idx int) []NormalizedBlock {
	var blocks []NormalizedBlock
	for _, part := range msg.Parts {
		switch p := part.(type) {
		case message.TextContent:
			text := strings.TrimSpace(p.Text)
			if text != "" {
				blocks = append(blocks, NormalizedBlock{
					Kind:        BlockUser,
					Text:        text,
					SourceIndex: idx,
				})
			}
		case message.ImageURLContent:
			blocks = append(blocks, NormalizedBlock{
				Kind:        BlockUser,
				Text:        "[image]",
				SourceIndex: idx,
			})
		case message.BinaryContent:
			blocks = append(blocks, NormalizedBlock{
				Kind:        BlockUser,
				Text:        "[binary: " + p.MIMEType + "]",
				SourceIndex: idx,
			})
		}
	}
	if len(blocks) == 0 {
		blocks = append(blocks, NormalizedBlock{
			Kind:        BlockUser,
			Text:        "",
			SourceIndex: idx,
		})
	}
	return blocks
}

func normalizeAssistant(msg message.Message, idx int) []NormalizedBlock {
	var blocks []NormalizedBlock
	for _, part := range msg.Parts {
		switch p := part.(type) {
		case message.TextContent:
			text := strings.TrimSpace(p.Text)
			if text != "" {
				blocks = append(blocks, NormalizedBlock{
					Kind:        BlockAssistant,
					Text:        text,
					SourceIndex: idx,
				})
			}
		case message.ToolCall:
			if p.Name == "" {
				continue
			}
			args := parseToolArgs(p.Input)
			blocks = append(blocks, NormalizedBlock{
				Kind:        BlockToolCall,
				Name:        p.Name,
				Args:        args,
				RawInput:    p.Input,
				SourceIndex: idx,
			})
		case message.ReasoningContent:
			// Thinking blocks are captured but will be filtered by noise filter.
			_ = p
		}
	}
	return blocks
}

func normalizeTool(msg message.Message, idx int) []NormalizedBlock {
	var blocks []NormalizedBlock
	for _, part := range msg.Parts {
		switch p := part.(type) {
		case message.ToolResult:
			// Detect bash results by name convention. The command is
			// populated later by correlateBashCommands from the
			// preceding ToolCall args.
			if p.Name == "bash" {
				exitCode := extractBashExitCode(p.Metadata)
				blocks = append(blocks, NormalizedBlock{
					Kind:        BlockBash,
					Output:      p.Content,
					ExitCode:    exitCode,
					SourceIndex: idx,
				})
			} else {
				blocks = append(blocks, NormalizedBlock{
					Kind:        BlockToolResult,
					Name:        p.Name,
					ResultText:  p.Content,
					IsError:     p.IsError,
					SourceIndex: idx,
				})
			}
		}
	}
	return blocks
}

// parseToolArgs extracts key-value pairs from a tool call's JSON input.
func parseToolArgs(input string) map[string]string {
	if input == "" {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		return nil
	}
	args := make(map[string]string, len(raw))
	for k, v := range raw {
		// Try string first, fall back to raw JSON.
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			args[k] = s
		} else {
			args[k] = string(v)
		}
	}
	return args
}

// extractBashExitCode parses the exit code from bash metadata.
func extractBashExitCode(metadata string) int {
	if metadata == "" {
		return -1
	}
	var meta struct {
		ExitCode int `json:"exit_code"`
	}
	if err := json.Unmarshal([]byte(metadata), &meta); err != nil {
		return -1
	}
	return meta.ExitCode
}
