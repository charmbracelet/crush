package agent

import (
	"fmt"
	"sort"
	"unicode/utf8"

	"charm.land/fantasy"
)

// contextReserve leaves room for another response and tool results. Large
// windows need proportional headroom too, rather than a fixed 20k tokens.
func contextReserve(window int64) int64 {
	if window > largeContextWindowThreshold {
		return max(largeContextWindowBuffer, window/10)
	}
	return window / 5
}

// pendingToolResultTokens counts data that the last provider usage report
// cannot include: client tools execute after that response finishes.
func pendingToolResultTokens(steps []fantasy.StepResult) int64 {
	if len(steps) == 0 {
		return 0
	}
	var tokens int64
	for _, result := range steps[len(steps)-1].Content.ToolResults() {
		if !result.ProviderExecuted {
			tokens += estimateToolResultContentTokens(result.ToolCallID, result.ToolName, result.ClientMetadata, result.Result)
		}
	}
	return tokens
}

const summaryOmission = "\n[Tool output omitted to fit the summary context. The complete output remains in the session history.]"

// fitSummaryToolResults bounds tool text in the summary request, never in
// storage. Keep every message and call ID so calls and results remain paired.
// User text and instructions are never shortened to make a request fit.
func fitSummaryToolResults(messages []fantasy.Message, budget int64) ([]fantasy.Message, error) {
	total := estimateMessageTokens(messages)
	if total <= budget {
		return messages, nil
	}
	type candidate struct {
		message, part int
		text          string
		result        fantasy.ToolResultPart
	}
	var candidates []candidate
	for i, msg := range messages {
		for j, part := range msg.Content {
			var result fantasy.ToolResultPart
			switch p := part.(type) {
			case fantasy.ToolResultPart:
				result = p
			case *fantasy.ToolResultPart:
				result = *p
			default:
				continue
			}
			var text string
			switch output := result.Output.(type) {
			case fantasy.ToolResultOutputContentText:
				text = output.Text
			case *fantasy.ToolResultOutputContentText:
				text = output.Text
			default:
				continue
			}
			if approxTokenCount(text) > approxTokenCount(summaryOmission) {
				candidates = append(candidates, candidate{i, j, text, result})
			}
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return len(candidates[i].text) > len(candidates[j].text) })
	bounded := cloneFantasyMessages(messages)
	for _, candidate := range candidates {
		if total <= budget {
			break
		}
		oldTokens := approxTokenCount(candidate.text)
		target := max(approxTokenCount(summaryOmission), oldTokens-(total-budget))
		prefixBytes := min(len(candidate.text), int((target-approxTokenCount(summaryOmission))*4))
		for prefixBytes > 0 && !utf8.RuneStart(candidate.text[prefixBytes]) {
			prefixBytes--
		}
		text := candidate.text[:prefixBytes] + summaryOmission
		candidate.result.Output = fantasy.ToolResultOutputContentText{Text: text}
		bounded[candidate.message].Content[candidate.part] = candidate.result
		total -= oldTokens - approxTokenCount(text)
	}
	if total > budget {
		return nil, fmt.Errorf("summary context exceeds the model budget after shortening tool output (%d estimated tokens, %d available); user messages and tool calls were kept intact", total, budget)
	}
	return bounded, nil
}
