package tokenizer

import (
	"encoding/json"
	"strings"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/localit-io/tiktoken-go"
)

func encodingForModel(model catwalk.Model) (*tiktoken.Tiktoken, error) {
	enc, err := tiktoken.EncodingForModel(model.ID)
	if err == nil {
		return enc, nil
	}
	if model.ContextWindow >= 200000 {
		return tiktoken.GetEncoding("o200k_base")
	}
	return tiktoken.GetEncoding("cl100k_base")
}

func CountTokens(model catwalk.Model, systemPromptPrefix, systemMessage string, messages []message.Message, baseTools []tools.BaseTool) (int, error) {
	enc, err := encodingForModel(model)
	if err != nil {
		return 0, err
	}

	var b strings.Builder
	if systemPromptPrefix != "" {
		b.WriteString(systemPromptPrefix)
		b.WriteString("\n")
	}
	b.WriteString(systemMessage)

	for _, msg := range messages {
		if s := msg.Content().String(); s != "" {
			b.WriteString("\n\n")
			b.WriteString(s)
		}
		for _, tc := range msg.ToolCalls() {
			b.WriteString("\n\n")
			b.WriteString(tc.Name)
			b.WriteString(": ")
			b.WriteString(tc.Input)
		}
		for _, tr := range msg.ToolResults() {
			b.WriteString("\n\n")
			b.WriteString(tr.Content)
		}
	}

	if len(baseTools) > 0 {
		b.WriteString("\n\n")
		for i, tl := range baseTools {
			info := tl.Info()
			payload := map[string]any{
				"name":        info.Name,
				"description": info.Description,
				"parameters":  info.Parameters,
				"required":    info.Required,
			}
			data, _ := json.Marshal(payload)
			b.Write(data)
			if i < len(baseTools)-1 {
				b.WriteString("\n")
			}
		}
	}

	tokens := enc.Encode(b.String(), nil, nil)
	return len(tokens), nil
}
