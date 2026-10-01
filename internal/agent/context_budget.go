package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"charm.land/fantasy"
)

var errContextBudget = errors.New("conversation exceeds the available context window")

func isContextOverflow(err error) bool {
	var providerErr *fantasy.ProviderError
	if errors.As(err, &providerErr) {
		if providerErr.IsContextTooLarge() {
			return true
		}
		if contextOverflowText(providerErr.Message) {
			return true
		}
		if providerErr.StatusCode == 400 {
			return contextOverflowText(string(providerErr.ResponseBody))
		}
	}
	var fantasyErr *fantasy.Error
	if errors.As(err, &fantasyErr) {
		return contextOverflowText(fantasyErr.Message)
	}
	return false
}

func contextOverflowText(text string) bool {
	text = strings.ToLower(text)
	return strings.Contains(text, "exceeds the context window") ||
		strings.Contains(text, "maximum context length") ||
		strings.Contains(text, "context_length_exceeded") ||
		strings.Contains(text, "context window exceeded")
}

func summaryOutputLimit(window int) int64 {
	if window <= 0 {
		return 4096
	}
	return min(4096, max(256, int64(window)/8))
}

func contextBudgetExceeded(model Model, system, prefix string, history []fantasy.Message, prompt string, files []fantasy.FilePart, tools []fantasy.AgentTool, maxOutput int64) bool {
	window := int64(model.CatwalkCfg.ContextWindow)
	if window <= 0 {
		return false
	}
	reserve := maxOutput
	if reserve <= 0 {
		reserve = model.CatwalkCfg.DefaultMaxTokens
	}
	if reserve <= 0 {
		reserve = 4096
	}
	reserve = min(reserve, window)
	margin := max(256, window/20)
	budget := window - reserve - margin
	if budget <= 0 {
		return true
	}

	characters := int64(len(system) + len(prefix) + len(prompt))
	binaryTokens := int64(0)
	for _, item := range history {
		characters += 64
		for _, part := range item.Content {
			switch content := part.(type) {
			case fantasy.TextPart:
				characters += int64(len(content.Text))
			case fantasy.ReasoningPart:
				characters += int64(len(content.Text))
			case fantasy.ToolCallPart:
				characters += int64(len(content.ToolName) + len(content.Input))
			case fantasy.ToolResultPart:
				data, err := json.Marshal(content.Output)
				if err != nil {
					return true
				}
				characters += int64(len(data))
			case fantasy.FilePart:
				binaryTokens += max(1024, int64(len(content.Data))/2)
			default:
				return true
			}
		}
	}
	for _, file := range files {
		binaryTokens += max(1024, int64(len(file.Data))/2)
	}
	for _, tool := range tools {
		info := tool.Info()
		characters += int64(len(info.Name) + len(info.Description))
		data, err := json.Marshal(info.Parameters)
		if err != nil {
			return true
		}
		characters += int64(len(data))
	}
	if characters > math.MaxInt64/2 {
		return true
	}
	return characters/2+binaryTokens > budget
}

func summaryHistoryBudget(window int) int64 {
	if window <= 0 {
		return 32_000
	}
	return max(1024, int64(window)/2)
}

func summaryChunks(messages []fantasy.Message, window int) ([][]fantasy.Message, error) {
	limit := summaryHistoryBudget(window)
	var chunks [][]fantasy.Message
	var chunk []fantasy.Message
	var size int64
	for index := 0; index < len(messages); {
		end := index + 1
		if messages[index].Role == fantasy.MessageRoleAssistant {
			for end < len(messages) && messages[end].Role == fantasy.MessageRoleTool {
				end++
			}
		}
		var groupSize int64
		for _, item := range messages[index:end] {
			data, err := json.Marshal(item.Content)
			if err != nil {
				return nil, fmt.Errorf("cannot estimate summary input: %w", err)
			}
			groupSize += int64(len(data))/2 + 64
		}
		if groupSize > limit {
			return nil, fmt.Errorf("%w: a single message or tool exchange cannot fit in a summary request", errContextBudget)
		}
		if size+groupSize > limit && len(chunk) > 0 {
			chunks = append(chunks, chunk)
			chunk = nil
			size = 0
		}
		chunk = append(chunk, messages[index:end]...)
		size += groupSize
		index = end
	}
	if len(chunk) > 0 {
		chunks = append(chunks, chunk)
	}
	return chunks, nil
}
