package agent

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"charm.land/fantasy"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
)

//go:embed templates/memory_search.md
var memorySearchToolDescription []byte

//go:embed templates/memory_search_prompt.md.tpl
var memorySearchPromptTmpl []byte

// memorySearchValidationResult holds the validated parameters from the tool call context.
type memorySearchValidationResult struct {
	SessionID      string
	AgentMessageID string
}

// validateMemorySearchParams validates the tool call parameters and extracts required context values.
func validateMemorySearchParams(ctx context.Context, params tools.MemorySearchParams) (memorySearchValidationResult, error) {
	if params.Query == "" {
		return memorySearchValidationResult{}, errors.New("query is required")
	}

	sessionID := tools.GetSessionFromContext(ctx)
	if sessionID == "" {
		return memorySearchValidationResult{}, errors.New("session id missing from context")
	}

	agentMessageID := tools.GetMessageFromContext(ctx)
	if agentMessageID == "" {
		return memorySearchValidationResult{}, errors.New("agent message id missing from context")
	}

	return memorySearchValidationResult{
		SessionID:      sessionID,
		AgentMessageID: agentMessageID,
	}, nil
}

func (c *coordinator) memorySearchTool(_ context.Context) (fantasy.AgentTool, error) {
	return fantasy.NewParallelAgentTool(
		tools.MemorySearchToolName,
		string(memorySearchToolDescription),
		func(ctx context.Context, params tools.MemorySearchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			validationResult, err := validateMemorySearchParams(ctx, params)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			// Get the parent session to find the transcript.
			parentSession, err := c.sessions.Get(ctx, validationResult.SessionID)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to get session: %s", err)), nil
			}

			// Check if session has been summarized.
			if parentSession.SummaryMessageID == "" {
				return fantasy.NewTextErrorResponse("This session has not been summarized yet. The memory_search tool is only available after summarization."), nil
			}

			// Find the transcript file.
			transcriptPath := TranscriptPath(parentSession.ID)
			if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Transcript file not found at %s. The session may have been summarized before this feature was available.", transcriptPath)), nil
			}

			// Build the sub-agent prompt.
			transcriptDir := filepath.Dir(transcriptPath)
			promptOpts := []prompt.Option{
				prompt.WithWorkingDir(transcriptDir),
			}

			promptTemplate, err := prompt.NewPrompt("memory_search", string(memorySearchPromptTmpl), promptOpts...)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error creating prompt: %s", err)
			}

			_, small, err := c.buildAgentModels(ctx, true)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error building models: %s", err)
			}

			systemPrompt, err := promptTemplate.Build(ctx, small.Model.Provider(), small.Model.Model(), *c.cfg)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error building system prompt: %s", err)
			}

			smallProviderCfg, ok := c.cfg.Providers.Get(small.ModelCfg.Provider)
			if !ok {
				return fantasy.ToolResponse{}, errors.New("small model provider not configured")
			}

			// Create sub-agent with read-only tools scoped to the transcript directory.
			searchTools := []fantasy.AgentTool{
				tools.NewGlobTool(transcriptDir),
				tools.NewGrepTool(transcriptDir),
				tools.NewViewTool(c.lspClients, c.permissions, transcriptDir),
			}

			agent := NewSessionAgent(SessionAgentOptions{
				LargeModel:           small, // Use small model for both (search doesn't need large)
				SmallModel:           small,
				SystemPromptPrefix:   smallProviderCfg.SystemPromptPrefix,
				SystemPrompt:         systemPrompt,
				DisableAutoSummarize: true, // Never summarize the sub-agent session
				IsYolo:               c.permissions.SkipRequests(),
				Sessions:             c.sessions,
				Messages:             c.messages,
				Tools:                searchTools,
			})

			agentToolSessionID := c.sessions.CreateAgentToolSessionID(validationResult.AgentMessageID, call.ID)
			session, err := c.sessions.CreateTaskSession(ctx, agentToolSessionID, validationResult.SessionID, "Memory Search")
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error creating session: %s", err)
			}

			c.permissions.AutoApproveSession(session.ID)

			// Build the full prompt including the transcript path.
			fullPrompt := fmt.Sprintf("%s\n\nThe session transcript is located at: %s\n\nUse grep and view to search this file for the requested information.", params.Query, transcriptPath)

			// Use small model for transcript search.
			maxTokens := small.CatwalkCfg.DefaultMaxTokens
			if small.ModelCfg.MaxTokens != 0 {
				maxTokens = small.ModelCfg.MaxTokens
			}

			result, err := agent.Run(ctx, SessionAgentCall{
				SessionID:        session.ID,
				Prompt:           fullPrompt,
				MaxOutputTokens:  maxTokens,
				ProviderOptions:  getProviderOptions(small, smallProviderCfg),
				Temperature:      small.ModelCfg.Temperature,
				TopP:             small.ModelCfg.TopP,
				TopK:             small.ModelCfg.TopK,
				FrequencyPenalty: small.ModelCfg.FrequencyPenalty,
				PresencePenalty:  small.ModelCfg.PresencePenalty,
			})
			if err != nil {
				return fantasy.NewTextErrorResponse("error generating response"), nil
			}

			// Update parent session cost.
			updatedSession, err := c.sessions.Get(ctx, session.ID)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error getting session: %s", err)
			}
			parentSession, err = c.sessions.Get(ctx, validationResult.SessionID)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error getting parent session: %s", err)
			}

			parentSession.Cost += updatedSession.Cost

			_, err = c.sessions.Save(ctx, parentSession)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error saving parent session: %s", err)
			}

			return fantasy.NewTextResponse(result.Response.Content.Text()), nil
		}), nil
}
