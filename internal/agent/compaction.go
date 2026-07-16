package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/session"
)

const (
	compactionSafetyMarginTokens = 1_024
	maxCompactionPasses          = 32
	// Bound automatic paid work independently of provider pricing. The call
	// limit caps request count; the token multiplier allows the source pass,
	// reserved outputs, and merge passes without unbounded amplification.
	maxCompactionCalls        = 64
	compactionTokenMultiplier = 3
)

type compactionResult struct {
	summary        string
	totalUsage     fantasy.Usage
	finalUsage     fantasy.Usage
	openrouterCost *float64
	calls          int
	passes         int
	estimatedSpend int64
}

func (a *sessionAgent) compactMessages(
	ctx context.Context,
	model Model,
	messages []fantasy.Message,
	todos []session.Todo,
	providerOptions fantasy.ProviderOptions,
	systemPromptPrefix string,
	sessionID string,
) (result compactionResult, retErr error) {
	transcript := renderCompactionTranscript(messages)
	if strings.TrimSpace(transcript) == "" {
		return compactionResult{}, fmt.Errorf("cannot compact an empty conversation")
	}

	outputTokens := compactionOutputTokens(model.CatwalkCfg.ContextWindow, model.CatwalkCfg.DefaultMaxTokens)
	inputBudget, err := compactionInputBudget(
		model.CatwalkCfg.ContextWindow,
		outputTokens,
		systemPromptPrefix,
		buildCompactionChunkPrompt("", 0, 1, 1, todos),
	)
	if err != nil {
		return compactionResult{}, err
	}
	sourceTokens := approxTokenCount(transcript)
	tokenBudget := compactionTokenBudget(sourceTokens, model.CatwalkCfg.ContextWindow)
	startedAt := time.Now()
	slog.Info("Conversation compaction started",
		"session_id", sessionID,
		"estimated_source_tokens", sourceTokens,
		"context_window", model.CatwalkCfg.ContextWindow,
		"chunk_input_budget", inputBudget,
		"chunk_output_limit", outputTokens,
		"max_calls", maxCompactionCalls,
		"estimated_token_budget", tokenBudget,
	)

	input := transcript
	defer func() {
		fields := []any{
			"session_id", sessionID,
			"passes", result.passes,
			"calls", result.calls,
			"estimated_token_spend", result.estimatedSpend,
			"input_tokens", result.totalUsage.InputTokens,
			"output_tokens", result.totalUsage.OutputTokens,
			"summary_tokens", approxTokenCount(result.summary),
			"duration", time.Since(startedAt).Round(time.Millisecond),
		}
		if retErr != nil {
			fields = append(fields, "error", retErr)
			slog.Warn("Conversation compaction failed", fields...)
			return
		}
		slog.Info("Conversation compaction completed", fields...)
	}()
	for pass := range maxCompactionPasses {
		chunks := splitTextByApproxTokens(input, inputBudget)
		if len(chunks) == 0 {
			return result, fmt.Errorf("cannot compact an empty conversation")
		}
		if len(chunks) > maxCompactionCalls-result.calls {
			return result, fmt.Errorf(
				"conversation compaction exceeds the %d-call safety limit: %d calls already used and the next pass requires %d",
				maxCompactionCalls,
				result.calls,
				len(chunks),
			)
		}

		type plannedCall struct {
			prompt         string
			estimatedSpend int64
		}
		planned := make([]plannedCall, 0, len(chunks))
		var passSpend int64
		for i, chunk := range chunks {
			prompt := buildCompactionChunkPrompt(chunk, pass, i+1, len(chunks), todos)
			spend := estimateCompactionCallSpend(prompt, systemPromptPrefix, outputTokens)
			passSpend = saturatingAdd(passSpend, spend)
			planned = append(planned, plannedCall{prompt: prompt, estimatedSpend: spend})
		}
		if passSpend > tokenBudget-result.estimatedSpend {
			return result, fmt.Errorf(
				"conversation compaction exceeds its estimated token-spend safety budget: %d tokens already reserved, next pass requires %d, limit is %d",
				result.estimatedSpend,
				passSpend,
				tokenBudget,
			)
		}
		result.passes = pass + 1
		slog.Debug("Conversation compaction pass planned",
			"session_id", sessionID,
			"pass", result.passes,
			"chunks", len(planned),
			"estimated_token_spend", passSpend,
		)

		summaries := make([]string, 0, len(chunks))
		for i, plannedCall := range planned {
			result.calls++
			result.estimatedSpend = saturatingAdd(result.estimatedSpend, plannedCall.estimatedSpend)
			callResult, err := a.summarizeCompactionChunk(
				ctx,
				model,
				plannedCall.prompt,
				outputTokens,
				providerOptions,
				systemPromptPrefix,
				sessionID,
			)
			if err != nil {
				return result, fmt.Errorf("compact conversation chunk %d of %d: %w", i+1, len(chunks), err)
			}
			result.totalUsage = addTokenUsage(result.totalUsage, callResult.totalUsage)
			result.finalUsage = callResult.finalUsage
			result.openrouterCost = addOptionalCost(result.openrouterCost, callResult.openrouterCost)
			if strings.TrimSpace(callResult.summary) == "" {
				return result, fmt.Errorf("compact conversation chunk %d of %d: model returned an empty summary", i+1, len(chunks))
			}
			summaries = append(summaries, callResult.summary)
		}

		if len(summaries) == 1 {
			result.summary = summaries[0]
			return result, nil
		}
		input = joinCompactionSummaries(summaries)
	}

	return result, fmt.Errorf("conversation compaction did not converge after %d passes", maxCompactionPasses)
}

func compactionTokenBudget(sourceTokens, contextWindow int64) int64 {
	baseline := max(sourceTokens, contextWindow)
	if baseline > math.MaxInt64/compactionTokenMultiplier {
		return math.MaxInt64
	}
	return baseline * compactionTokenMultiplier
}

func estimateCompactionCallSpend(prompt, systemPromptPrefix string, outputTokens int64) int64 {
	messages := []fantasy.Message{fantasy.NewSystemMessage(string(summaryPrompt))}
	if systemPromptPrefix != "" {
		messages = append([]fantasy.Message{fantasy.NewSystemMessage(systemPromptPrefix)}, messages...)
	}
	messages = append(messages, fantasy.NewUserMessage(prompt))
	return saturatingAdd(estimateMessageTokens(messages), outputTokens)
}

func saturatingAdd(left, right int64) int64 {
	if right > 0 && left > math.MaxInt64-right {
		return math.MaxInt64
	}
	return left + right
}

func (a *sessionAgent) summarizeCompactionChunk(
	ctx context.Context,
	model Model,
	prompt string,
	maxOutputTokens int64,
	providerOptions fantasy.ProviderOptions,
	systemPromptPrefix string,
	sessionID string,
) (compactionResult, error) {
	agent := fantasy.NewAgent(
		withContextWindowLimit(model.Model, model.CatwalkCfg.ContextWindow),
		fantasy.WithSystemPrompt(string(summaryPrompt)),
		fantasy.WithUserAgent(userAgent),
	)

	call := fantasy.AgentStreamCall{
		Prompt:          prompt,
		Headers:         sessionHeaders(sessionID),
		ProviderOptions: providerOptions,
		PrepareStep: func(callContext context.Context, options fantasy.PrepareStepFunctionOptions) (context.Context, fantasy.PrepareStepResult, error) {
			messages := options.Messages
			if systemPromptPrefix != "" {
				messages = append([]fantasy.Message{fantasy.NewSystemMessage(systemPromptPrefix)}, messages...)
			}
			return callContext, fantasy.PrepareStepResult{Messages: messages}, nil
		},
	}
	if maxOutputTokens > 0 {
		call.MaxOutputTokens = &maxOutputTokens
	}

	var summary strings.Builder
	call.OnTextDelta = func(_, text string) error {
		summary.WriteString(text)
		return nil
	}

	response, err := agent.Stream(ctx, call)
	if err != nil {
		return compactionResult{}, err
	}
	if summary.Len() == 0 {
		summary.WriteString(response.Response.Content.Text())
	}

	var cost *float64
	for _, step := range response.Steps {
		cost = addOptionalCost(cost, a.openrouterCost(step.ProviderMetadata))
		extractHyperCredits(step.ProviderMetadata)
	}

	return compactionResult{
		summary:        summary.String(),
		totalUsage:     response.TotalUsage,
		finalUsage:     response.Response.Usage,
		openrouterCost: cost,
	}, nil
}

func compactionOutputTokens(contextWindow, defaultMaxTokens int64) int64 {
	if contextWindow <= 0 {
		return defaultMaxTokens
	}
	reserve := contextWindowReserve(contextWindow)
	if defaultMaxTokens > 0 {
		return min(reserve, defaultMaxTokens)
	}
	return reserve
}

func compactionInputBudget(contextWindow, outputTokens int64, systemPromptPrefix, promptWrapper string) (int64, error) {
	if contextWindow <= 0 {
		return 1<<62 - 1, nil
	}
	overhead := approxTokenCount(string(summaryPrompt)) +
		approxTokenCount(systemPromptPrefix) +
		approxTokenCount(promptWrapper) +
		compactionSafetyMarginTokens
	budget := contextWindow - outputTokens - overhead
	if budget <= 0 {
		return 0, fmt.Errorf(
			"context window is too small for compaction: %d tokens available, %d required for output and prompt overhead",
			contextWindow,
			outputTokens+overhead,
		)
	}
	return budget, nil
}

func splitTextByApproxTokens(text string, maxTokens int64) []string {
	if text == "" || maxTokens <= 0 {
		return nil
	}

	var chunks []string
	var chunk strings.Builder
	var counter approxTokenCounter
	flush := func() {
		if chunk.Len() == 0 {
			return
		}
		chunks = append(chunks, chunk.String())
		chunk.Reset()
		counter = approxTokenCounter{}
	}

	for len(text) > 0 {
		r, size := utf8.DecodeRuneInString(text)
		piece := text[:size]
		next := counter
		next.add(r)
		if chunk.Len() > 0 && next.total() > maxTokens {
			flush()
			next = approxTokenCounter{}
			next.add(r)
		}
		// Write the original bytes rather than encoding r again. This keeps
		// malformed UTF-8 from tool output byte-for-byte intact while still
		// avoiding splits inside valid multi-byte runes.
		chunk.WriteString(piece)
		counter = next
		text = text[size:]
	}
	flush()
	return chunks
}

func renderCompactionTranscript(messages []fantasy.Message) string {
	var transcript strings.Builder
	for _, msg := range messages {
		fmt.Fprintf(&transcript, "\n<message role=%q>\n", msg.Role)
		for _, part := range msg.Content {
			renderCompactionPart(&transcript, part)
		}
		transcript.WriteString("\n</message>\n")
	}
	return transcript.String()
}

func renderCompactionPart(transcript *strings.Builder, part fantasy.MessagePart) {
	if text, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok {
		transcript.WriteString(text.Text)
		return
	}
	if reasoning, ok := fantasy.AsMessagePart[fantasy.ReasoningPart](part); ok {
		transcript.WriteString("\n[reasoning]\n")
		transcript.WriteString(reasoning.Text)
		return
	}
	if toolCall, ok := fantasy.AsMessagePart[fantasy.ToolCallPart](part); ok {
		fmt.Fprintf(transcript, "\n[tool call %s id=%s]\n%s", toolCall.ToolName, toolCall.ToolCallID, toolCall.Input)
		return
	}
	if toolResult, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part); ok {
		fmt.Fprintf(transcript, "\n[tool result id=%s]\n", toolResult.ToolCallID)
		renderCompactionToolResult(transcript, toolResult.Output)
		return
	}
	if file, ok := fantasy.AsMessagePart[fantasy.FilePart](part); ok {
		fmt.Fprintf(transcript, "\n[file %q media_type=%q bytes=%d]\n", file.Filename, file.MediaType, len(file.Data))
		return
	}
	if encoded, err := json.Marshal(part); err == nil {
		transcript.Write(encoded)
	}
}

func renderCompactionToolResult(transcript *strings.Builder, output fantasy.ToolResultOutputContent) {
	if text, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](output); ok {
		transcript.WriteString(text.Text)
		return
	}
	if resultErr, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentError](output); ok {
		if resultErr.Error != nil {
			transcript.WriteString(resultErr.Error.Error())
		}
		return
	}
	if media, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](output); ok {
		fmt.Fprintf(transcript, "[media media_type=%q bytes=%d] %s", media.MediaType, len(media.Data), media.Text)
	}
}

func buildCompactionChunkPrompt(chunk string, pass, index, total int, todos []session.Todo) string {
	var prompt strings.Builder
	if pass == 0 {
		fmt.Fprintf(&prompt, "Summarize conversation segment %d of %d faithfully. Preserve decisions, file paths, commands, errors, completed work, and exact next steps.\n\n", index, total)
	} else {
		fmt.Fprintf(&prompt, "Merge partial continuation summaries %d of %d into a single faithful continuation summary. Remove duplication without dropping decisions, file paths, commands, errors, completed work, or exact next steps.\n\n", index, total)
	}
	prompt.WriteString(buildSummaryPrompt(todos))
	prompt.WriteString("\n\n<conversation_segment>\n")
	prompt.WriteString(chunk)
	prompt.WriteString("\n</conversation_segment>")
	return prompt.String()
}

func joinCompactionSummaries(summaries []string) string {
	var joined strings.Builder
	for i, summary := range summaries {
		fmt.Fprintf(&joined, "\n<partial_summary index=%d>\n%s\n</partial_summary>\n", i+1, summary)
	}
	return joined.String()
}

func addTokenUsage(a, b fantasy.Usage) fantasy.Usage {
	return fantasy.Usage{
		InputTokens:         a.InputTokens + b.InputTokens,
		OutputTokens:        a.OutputTokens + b.OutputTokens,
		TotalTokens:         a.TotalTokens + b.TotalTokens,
		ReasoningTokens:     a.ReasoningTokens + b.ReasoningTokens,
		CacheCreationTokens: a.CacheCreationTokens + b.CacheCreationTokens,
		CacheReadTokens:     a.CacheReadTokens + b.CacheReadTokens,
	}
}

func addOptionalCost(a, b *float64) *float64 {
	if b == nil {
		return a
	}
	if a == nil {
		value := *b
		return &value
	}
	value := *a + *b
	return &value
}
