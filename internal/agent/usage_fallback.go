package agent

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"charm.land/fantasy"
)

const messageFramingTokens = 4

func usageIsZero(usage fantasy.Usage) bool {
	return usage.InputTokens == 0 &&
		usage.OutputTokens == 0 &&
		usage.TotalTokens == 0 &&
		usage.ReasoningTokens == 0 &&
		usage.CacheCreationTokens == 0 &&
		usage.CacheReadTokens == 0
}

func fallbackStepUsage(messages []fantasy.Message, step fantasy.StepResult) (fantasy.Usage, bool) {
	if !usageIsZero(step.Usage) {
		return step.Usage, false
	}

	inputTokens := estimateMessageTokens(messages)
	outputTokens := estimateStepCompletionTokens(step)
	if inputTokens == 0 && outputTokens == 0 {
		return fantasy.Usage{}, false
	}

	return fantasy.Usage{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
	}, true
}

func cloneFantasyMessages(messages []fantasy.Message) []fantasy.Message {
	cloned := make([]fantasy.Message, len(messages))
	for i, msg := range messages {
		cloned[i] = msg
		cloned[i].Content = append([]fantasy.MessagePart(nil), msg.Content...)
	}
	return cloned
}

func estimateMessageTokens(messages []fantasy.Message) int64 {
	var tokens int64
	for _, msg := range messages {
		// Chat templates add fixed start/end/newline markers around every
		// message. Four tokens covers the Qwen ChatML framing and prevents
		// long histories of tiny tool messages from being underestimated.
		tokens += messageFramingTokens + approxTokenCount(string(msg.Role))
		for _, part := range msg.Content {
			tokens += estimateMessagePartTokens(part)
		}
	}
	return tokens
}

func estimateStepCompletionTokens(step fantasy.StepResult) int64 {
	var tokens int64
	for _, content := range step.Content {
		switch c := content.(type) {
		case fantasy.TextContent:
			tokens += approxTokenCount(c.Text)
		case *fantasy.TextContent:
			tokens += approxTokenCount(c.Text)
		case fantasy.ReasoningContent:
			tokens += approxTokenCount(c.Text)
		case *fantasy.ReasoningContent:
			tokens += approxTokenCount(c.Text)
		case fantasy.FileContent:
			tokens += estimateGeneratedFileTokens(c)
		case *fantasy.FileContent:
			tokens += estimateGeneratedFileTokens(*c)
		case fantasy.SourceContent:
			tokens += estimateSourceTokens(c)
		case *fantasy.SourceContent:
			tokens += estimateSourceTokens(*c)
		case fantasy.ToolCallContent:
			tokens += estimateToolCallTokens(c.ToolName, c.Input)
		case *fantasy.ToolCallContent:
			tokens += estimateToolCallTokens(c.ToolName, c.Input)
		case fantasy.ToolResultContent:
			if c.ProviderExecuted {
				tokens += estimateToolResultContentTokens(c.ToolCallID, c.ToolName, c.ClientMetadata, c.Result)
			}
		case *fantasy.ToolResultContent:
			if c.ProviderExecuted {
				tokens += estimateToolResultContentTokens(c.ToolCallID, c.ToolName, c.ClientMetadata, c.Result)
			}
		}
	}
	return tokens
}

func estimateMessagePartTokens(part fantasy.MessagePart) int64 {
	switch p := part.(type) {
	case fantasy.TextPart:
		return approxTokenCount(p.Text)
	case *fantasy.TextPart:
		return approxTokenCount(p.Text)
	case fantasy.ReasoningPart:
		return approxTokenCount(p.Text)
	case *fantasy.ReasoningPart:
		return approxTokenCount(p.Text)
	case fantasy.FilePart:
		return estimateFilePartTokens(p)
	case *fantasy.FilePart:
		return estimateFilePartTokens(*p)
	case fantasy.ToolCallPart:
		return estimateToolCallTokens(p.ToolName, p.Input)
	case *fantasy.ToolCallPart:
		return estimateToolCallTokens(p.ToolName, p.Input)
	case fantasy.ToolResultPart:
		return estimateToolResultContentTokens(p.ToolCallID, "", "", p.Output)
	case *fantasy.ToolResultPart:
		return estimateToolResultContentTokens(p.ToolCallID, "", "", p.Output)
	default:
		return 0
	}
}

func estimateToolCallTokens(toolName, input string) int64 {
	return approxTokenCount(toolName) + approxTokenCount(input)
}

func estimateToolResultContentTokens(toolCallID, toolName, metadata string, output fantasy.ToolResultOutputContent) int64 {
	tokens := approxTokenCount(toolCallID) + approxTokenCount(toolName) + approxTokenCount(metadata)
	switch result := output.(type) {
	case fantasy.ToolResultOutputContentText:
		tokens += approxTokenCount(result.Text)
	case *fantasy.ToolResultOutputContentText:
		tokens += approxTokenCount(result.Text)
	case fantasy.ToolResultOutputContentError:
		if result.Error != nil {
			tokens += approxTokenCount(result.Error.Error())
		}
	case *fantasy.ToolResultOutputContentError:
		if result.Error != nil {
			tokens += approxTokenCount(result.Error.Error())
		}
	case fantasy.ToolResultOutputContentMedia:
		tokens += estimateMediaTokens(result.MediaType, result.Text, len(result.Data))
	case *fantasy.ToolResultOutputContentMedia:
		tokens += estimateMediaTokens(result.MediaType, result.Text, len(result.Data))
	}
	return tokens
}

func estimateFilePartTokens(file fantasy.FilePart) int64 {
	return estimateMediaTokens(file.MediaType, file.Filename, len(file.Data))
}

func estimateGeneratedFileTokens(file fantasy.FileContent) int64 {
	return estimateMediaTokens(file.MediaType, "", len(file.Data))
}

func estimateMediaTokens(mediaType, text string, dataBytes int) int64 {
	if dataBytes == 0 {
		return approxTokenCount(mediaType) + approxTokenCount(text)
	}
	return approxTokenCount(fmt.Sprintf("%s %s %d bytes", mediaType, text, dataBytes))
}

func estimateSourceTokens(source fantasy.SourceContent) int64 {
	return approxTokenCount(string(source.SourceType)) +
		approxTokenCount(source.ID) +
		approxTokenCount(source.URL) +
		approxTokenCount(source.Title) +
		approxTokenCount(source.MediaType) +
		approxTokenCount(source.Filename)
}

func approxTokenCount(s string) int64 {
	if s == "" {
		return 0
	}
	var counter approxTokenCounter
	for _, r := range s {
		counter.add(r)
	}
	return counter.total()
}

type approxTokenCounter struct {
	tokens    int64
	runLength int64
	runKind   uint8
}

func (c *approxTokenCounter) add(r rune) {
	const (
		runAlphaNumeric uint8 = iota + 1
		runWhitespace
	)

	var kind uint8
	if r < utf8.RuneSelf {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			kind = runAlphaNumeric
		case unicode.IsSpace(r):
			kind = runWhitespace
		}
	}
	if kind != 0 {
		if c.runKind != kind {
			c.flushRun()
			c.runKind = kind
		}
		c.runLength++
		return
	}

	c.flushRun()
	if r < utf8.RuneSelf {
		// ASCII punctuation and separators are frequently individual tokens
		// in code, paths, JSON, and tool output. Counting each byte prevents
		// the old bytes/4 heuristic from systematically underestimating them.
		c.tokens++
		return
	}
	if unicode.IsLetter(r) || unicode.IsNumber(r) {
		// CJK tokenizers commonly encode one or more characters per token;
		// one token per rune is intentionally conservative for prose.
		c.tokens++
		return
	}
	// Emoji and other symbols often decompose into multiple byte-level BPE
	// tokens. UTF-8 width is a portable upper estimate for that decomposition.
	c.tokens += int64(utf8.RuneLen(r))
}

func (c *approxTokenCounter) flushRun() {
	if c.runLength == 0 {
		return
	}
	if c.runKind == 1 {
		if c.runLength <= 16 {
			c.tokens += (c.runLength + 2) / 3
		} else {
			// Long unbroken alphanumeric runs are commonly hashes, minified
			// data, random identifiers, or base64. They tokenize far more
			// densely than natural-language words; Qwen3.6 is approximately
			// 0.72 tokens/byte for representative base64 and random data.
			c.tokens += (c.runLength*3 + 3) / 4
		}
	} else {
		c.tokens += (c.runLength + 3) / 4
	}
	c.runLength = 0
	c.runKind = 0
}

func (c approxTokenCounter) total() int64 {
	c.flushRun()
	return c.tokens
}
