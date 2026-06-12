package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"github.com/charmbracelet/crush/internal/compact"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
)

// AlgorithmicSummarize produces a hybrid compaction: algorithmic extraction
// followed by a lightweight LLM polish pass. The algorithmic step extracts
// structured facts (goals, files, errors, commits, type catalog) in ~30ms
// with zero API cost. The LLM step then synthesizes those facts into a
// concise handoff summary using the same template as the full LLM path,
// but seeing ~100× fewer input tokens.
func (a *sessionAgent) AlgorithmicSummarize(ctx context.Context, sessionID string) error {
	if a.IsSessionBusy(sessionID) {
		return ErrSessionBusy
	}

	largeModel := a.largeModel.Get()
	systemPromptPrefix := a.systemPromptPrefix.Get()

	currentSession, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	msgs, err := a.getSessionMessages(ctx, currentSession)
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		return nil
	}

	// Step 1: Algorithmic extraction.
	var previousSummary string
	if currentSession.SummaryMessageID != "" {
		prevMsg, err := a.messages.Get(ctx, currentSession.SummaryMessageID)
		if err != nil {
			slog.Warn("Failed to load previous summary for merge; compaction will not accumulate prior context",
				"session_id", sessionID,
				"error", err,
			)
		} else {
			previousSummary = prevMsg.Content().Text
		}
	}

	extracted := compact.Compact(compact.Input{
		Messages:        msgs,
		PreviousSummary: previousSummary,
	})
	if extracted == "" {
		return nil
	}

	slog.Info("Algorithmic extraction completed, starting LLM polish",
		"session_id", sessionID,
		"extracted_chars", len(extracted),
	)

	// Step 2: LLM polish pass. Feed the extracted summary as context
	// and ask the model to produce a concise handoff using the standard
	// template. This is much cheaper than feeding the full conversation.
	genCtx, cancel := context.WithCancel(ctx)
	a.activeRequests.Set(sessionID, cancel)
	defer a.activeRequests.Del(sessionID)
	defer cancel()
	defer func() {
		if flushErr := a.messages.FlushAll(ctx); flushErr != nil {
			slog.Error("Failed to flush pending message updates after algorithmic summarize", "error", flushErr)
		}
	}()

	agent := fantasy.NewAgent(
		largeModel.Model,
		fantasy.WithSystemPrompt(string(summaryPrompt)),
		fantasy.WithUserAgent(userAgent),
	)

	summaryMessage, err := a.messages.Create(ctx, sessionID, message.CreateMessageParams{
		Role:             message.Assistant,
		Model:            largeModel.ModelCfg.Model,
		Provider:         largeModel.ModelCfg.Provider,
		IsSummaryMessage: true,
	})
	if err != nil {
		return err
	}

	thinkingEnabled := largeModel.CatwalkCfg.CanReason && largeModel.ModelCfg.Think
	userPrompt := buildHybridPrompt(extracted, currentSession.Todos, thinkingEnabled)

	router := newAnalysisRouter(
		func(text string) error {
			summaryMessage.AppendReasoningContent(text)
			return a.messages.Update(genCtx, summaryMessage)
		},
		func(text string) error {
			summaryMessage.AppendContent(text)
			return a.messages.Update(genCtx, summaryMessage)
		},
	)

	streamCall := fantasy.AgentStreamCall{
		Prompt: userPrompt,
		PrepareStep: func(callContext context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			prepared.Messages = options.Messages
			if systemPromptPrefix != "" {
				prepared.Messages = append([]fantasy.Message{fantasy.NewSystemMessage(systemPromptPrefix)}, prepared.Messages...)
			}
			return callContext, prepared, nil
		},
		OnReasoningDelta: func(id string, text string) error {
			summaryMessage.AppendReasoningContent(text)
			return a.messages.Update(genCtx, summaryMessage)
		},
		OnReasoningEnd: func(id string, reasoning fantasy.ReasoningContent) error {
			if anthropicData, ok := reasoning.ProviderMetadata["anthropic"]; ok {
				if signature, ok := anthropicData.(*anthropic.ReasoningOptionMetadata); ok && signature.Signature != "" {
					summaryMessage.AppendReasoningSignature(signature.Signature)
				}
			}
			summaryMessage.FinishThinking()
			return a.messages.Update(genCtx, summaryMessage)
		},
		OnTextDelta: func(id, text string) error {
			return router.write(text)
		},
	}

	resp, err := agent.Stream(genCtx, streamCall)
	if flushErr := router.flush(); flushErr != nil && err == nil {
		err = flushErr
	}
	if err != nil {
		isCancelErr := errors.Is(err, context.Canceled)
		if isCancelErr {
			deleteErr := a.messages.Delete(ctx, summaryMessage.ID)
			return deleteErr
		}
		summaryMessage.AddFinish(message.FinishReasonError, "Algorithmic Summarization Error", err.Error())
		if updateErr := a.messages.Update(ctx, summaryMessage); updateErr != nil {
			return updateErr
		}
		return err
	}

	summaryMessage.AddFinish(message.FinishReasonEndTurn, "", "")
	err = a.messages.Update(genCtx, summaryMessage)
	if err != nil {
		return err
	}

	var openrouterCost *float64
	for _, step := range resp.Steps {
		stepCost := a.openrouterCost(step.ProviderMetadata)
		if stepCost != nil {
			newCost := *stepCost
			if openrouterCost != nil {
				newCost += *openrouterCost
			}
			openrouterCost = &newCost
		}
	}

	a.updateSessionUsage(largeModel, &currentSession, resp.TotalUsage, openrouterCost, false)

	usage := resp.Response.Usage
	currentSession.SummaryMessageID = summaryMessage.ID
	currentSession.CompletionTokens = summaryCompletionTokens(usage, summaryMessage)
	currentSession.PromptTokens = 0
	currentSession.EstimatedUsage = usageIsZero(usage)
	_, err = a.sessions.Save(genCtx, currentSession)
	if err != nil {
		return err
	}

	a.activeRequests.Del(sessionID)
	cancel()

	slog.Info("Hybrid compaction completed",
		"session_id", sessionID,
		"extracted_chars", len(extracted),
		"summary_chars", len(summaryMessage.Content().Text),
	)
	return nil
}

// buildHybridPrompt constructs the user prompt for the LLM polish pass.
// It includes the algorithmically extracted summary as structured context
// plus the current todo list status.
func buildHybridPrompt(extracted string, todos []session.Todo, thinkingEnabled bool) string {
	var sb strings.Builder

	sb.WriteString("Below is a structured extraction from our conversation. ")
	sb.WriteString("Use it as your sole source of truth to write a handoff summary following the system prompt instructions.\n\n")
	sb.WriteString("## Extracted Context\n\n")
	sb.WriteString(extracted)

	if len(todos) > 0 {
		sb.WriteString("\n\n## Current Todo List\n\n")
		for _, t := range todos {
			fmt.Fprintf(&sb, "- [%s] %s\n", t.Status, t.Content)
		}
		sb.WriteString("\nInclude these tasks and their statuses in your summary. ")
		sb.WriteString("Instruct the resuming assistant to use the `todos` tool to continue tracking progress on these tasks.")
	}

	if thinkingEnabled {
		sb.WriteString("\n\nUse your reasoning/thinking block to analyze what to preserve before writing the summary.")
	} else {
		sb.WriteString("\n\nWrap your analysis in <analysis> tags before writing the summary. These will be displayed as thinking.")
	}

	return sb.String()
}
