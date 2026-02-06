package agent

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
)

//go:embed templates/past_memory.md
var pastMemoryToolDescription []byte

//go:embed templates/past_memory_prompt.md.tpl
var pastMemoryPromptTmpl []byte

const PastMemorySearchToolName = "past_memory_search"

// PastMemorySearchParams holds the parameters for the past_memory_search tool.
type PastMemorySearchParams struct {
	Query string `json:"query" description:"The search query about past session context"`
}

// PastMemoryMiniTools provides mini-tools for searching past memory.
type PastMemoryMiniTools struct {
	pastMemory string
}

// NewPastMemoryMiniTools creates a new instance of past memory mini-tools.
func NewPastMemoryMiniTools(pastMemory string) *PastMemoryMiniTools {
	return &PastMemoryMiniTools{pastMemory: pastMemory}
}

// GrepPastMemory searches for a pattern in the past memory.
func (p *PastMemoryMiniTools) GrepPastMemory(pattern string) string {
	if p.pastMemory == "" {
		return "No past memory available"
	}
	lines := strings.Split(p.pastMemory, "\n")
	var matches []string
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), strings.ToLower(pattern)) {
			matches = append(matches, fmt.Sprintf("%d: %s", i+1, line))
		}
	}
	if len(matches) == 0 {
		return fmt.Sprintf("No matches found for pattern: %s", pattern)
	}
	return strings.Join(matches, "\n")
}

// StatsPastMemory returns statistics about the past memory.
func (p *PastMemoryMiniTools) StatsPastMemory() string {
	if p.pastMemory == "" {
		return "No past memory available"
	}
	lines := strings.Split(p.pastMemory, "\n")
	charCount := len(p.pastMemory)
	lineCount := len(lines)
	return fmt.Sprintf("Past memory statistics:\n- Characters: %d\n- Lines: %d", charCount, lineCount)
}

// ReadRangePastMemory reads a specific range of lines from past memory.
func (p *PastMemoryMiniTools) ReadRangePastMemory(startLine, endLine int) string {
	if p.pastMemory == "" {
		return "No past memory available"
	}
	lines := strings.Split(p.pastMemory, "\n")
	if startLine < 1 {
		startLine = 1
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	if startLine > endLine {
		return "Invalid line range"
	}
	var result []string
	for i := startLine - 1; i < endLine && i < len(lines); i++ {
		result = append(result, fmt.Sprintf("%d: %s", i+1, lines[i]))
	}
	return strings.Join(result, "\n")
}

func (c *coordinator) pastMemorySearchTool(ctx context.Context) (fantasy.AgentTool, error) {
	return fantasy.NewParallelAgentTool(
		PastMemorySearchToolName,
		string(pastMemoryToolDescription),
		func(ctx context.Context, params PastMemorySearchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Query == "" {
				return fantasy.NewTextErrorResponse("query is required"), nil
			}

			sessionID := tools.GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("session ID is required"), nil
			}

			currentSession, err := c.sessions.Get(ctx, sessionID)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to get session: %s", err)), nil
			}

			pastMemory := currentSession.PastMemory
			if pastMemory == "" {
				return fantasy.NewTextResponse("No past memory available for this session."), nil
			}

			miniTools := NewPastMemoryMiniTools(pastMemory)

			// Create the mini-tools for the subagent
			grepTool := fantasy.NewParallelAgentTool(
				"grep_past_memory",
				"Search for a pattern in the past memory. Parameters: pattern (string) - the search pattern",
				func(ctx context.Context, grepParams struct {
					Pattern string `json:"pattern"`
				}, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
					result := miniTools.GrepPastMemory(grepParams.Pattern)
					return fantasy.NewTextResponse(result), nil
				},
			)

			statsTool := fantasy.NewParallelAgentTool(
				"stats_past_memory",
				"Get statistics about the past memory (character count, line count). No parameters needed.",
				func(ctx context.Context, _ struct{}, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
					result := miniTools.StatsPastMemory()
					return fantasy.NewTextResponse(result), nil
				},
			)

			readRangeTool := fantasy.NewParallelAgentTool(
				"read_range_past_memory",
				"Read a specific range of lines from past memory. Parameters: start_line (int), end_line (int)",
				func(ctx context.Context, rangeParams struct {
					StartLine int `json:"start_line"`
					EndLine   int `json:"end_line"`
				}, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
					result := miniTools.ReadRangePastMemory(rangeParams.StartLine, rangeParams.EndLine)
					return fantasy.NewTextResponse(result), nil
				},
			)

			// Build prompt template
			promptTemplate, err := prompt.NewPrompt("past_memory", string(pastMemoryPromptTmpl))
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("error creating prompt: %s", err)), nil
			}

			// Get small model for subagent
			_, small, err := c.buildAgentModels(ctx, true)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("error building models: %s", err)), nil
			}

			systemPrompt, err := promptTemplate.Build(ctx, small.Model.Provider(), small.Model.Model(), *c.cfg)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("error building system prompt: %s", err)), nil
			}

			smallProviderCfg, ok := c.cfg.Providers.Get(small.ModelCfg.Provider)
			if !ok {
				return fantasy.NewTextErrorResponse("small model provider not configured"), nil
			}

			// Create a task session for the subagent
			agentMessageID := tools.GetMessageFromContext(ctx)
			if agentMessageID == "" {
				return fantasy.NewTextErrorResponse("agent message ID is required"), nil
			}

			agentToolSessionID := c.sessions.CreateAgentToolSessionID(agentMessageID, call.ID)
			taskSession, err := c.sessions.CreateTaskSession(ctx, agentToolSessionID, sessionID, "Past Memory Search")
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("error creating session: %s", err)), nil
			}

			c.permissions.AutoApproveSession(taskSession.ID)

			// Create subagent with mini-tools
			subAgent := NewSessionAgent(SessionAgentOptions{
				LargeModel:           small,
				SmallModel:           small,
				SystemPromptPrefix:   smallProviderCfg.SystemPromptPrefix,
				SystemPrompt:         systemPrompt,
				DisableAutoSummarize: c.cfg.Options.DisableAutoSummarize,
				IsYolo:               c.permissions.SkipRequests(),
				Sessions:             c.sessions,
				Messages:             c.messages,
				Tools:                []fantasy.AgentTool{grepTool, statsTool, readRangeTool},
			})

			// Run the subagent to answer the query
			maxTokens := small.CatwalkCfg.DefaultMaxTokens
			if small.ModelCfg.MaxTokens != 0 {
				maxTokens = small.ModelCfg.MaxTokens
			}

			result, err := subAgent.Run(ctx, SessionAgentCall{
				SessionID:        taskSession.ID,
				Prompt:           params.Query,
				MaxOutputTokens:  maxTokens,
				ProviderOptions:  getProviderOptions(small, smallProviderCfg),
				Temperature:      small.ModelCfg.Temperature,
				TopP:             small.ModelCfg.TopP,
				TopK:             small.ModelCfg.TopK,
				FrequencyPenalty: small.ModelCfg.FrequencyPenalty,
				PresencePenalty:  small.ModelCfg.PresencePenalty,
			})
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Error searching past memory: %s", err)), nil
			}

			// Update parent session cost
			updatedSession, err := c.sessions.Get(ctx, taskSession.ID)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("error getting session: %s", err)), nil
			}

			parentSession, err := c.sessions.Get(ctx, sessionID)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("error getting parent session: %s", err)), nil
			}

			parentSession.Cost += updatedSession.Cost
			_, err = c.sessions.Save(ctx, parentSession)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("error saving parent session: %s", err)), nil
			}

			return fantasy.NewTextResponse(result.Response.Content.Text()), nil
		},
	), nil
}
