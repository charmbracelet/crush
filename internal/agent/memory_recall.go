package agent

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/memory"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/charmbracelet/crush/internal/version"
)

var memoryUserAgent = fmt.Sprintf("Charm-Crush/%s (https://charm.land/crush)", version.Version)

const (
	memoryRelevanceMaxSelected = 5
	memoryRelevanceMaxFiles    = 50
)

// memoryRelevancePrompt is the system prompt for the memory relevance selection model.
const memoryRelevancePrompt = `You are a memory relevance selector. Given a user query and a manifest of available memory entries, select up to 5 most relevant memories that would help answer the query.

Rules:
- Only select memories that are directly relevant to the query
- Prefer recent memories (higher updated_at timestamps)
- Prefer specific, actionable memories over vague ones
- Return ONLY a JSON array of memory keys, nothing else
- Return an empty array [] if no memories are relevant

Example output:
["project/goal", "user/preferred-language"]`

// selectRelevantMemories uses the background model to select the most relevant
// memories for a given query, replacing simple string matching.
func selectRelevantMemories(ctx context.Context, memorySvc memory.Service, model Model, providerCfg config.ProviderConfig, query string, scope string) []memory.Entry {
	if memorySvc == nil {
		return nil
	}

	infos, err := memorySvc.ListMemoryFiles()
	if err != nil {
		slog.Warn("Failed to list memory files for relevance selection", "error", err)
		return nil
	}

	if len(infos) == 0 {
		return nil
	}

	if scope != "" {
		filtered := make([]memory.MemoryFileInfo, 0, len(infos))
		for _, info := range infos {
			if strings.EqualFold(info.Scope, scope) {
				filtered = append(filtered, info)
			}
		}
		infos = filtered
	}

	if len(infos) == 0 {
		return nil
	}

	if len(infos) > memoryRelevanceMaxFiles {
		infos = infos[:memoryRelevanceMaxFiles]
	}

	manifest := buildMemoryManifest(infos)

	prompt := fmt.Sprintf("Query: %s\n\nMemory manifest:\n%s\n\nSelect relevant memory keys:", query, manifest)

	agent := fantasy.NewAgent(
		model.Model,
		fantasy.WithSystemPrompt(memoryRelevancePrompt),
		fantasy.WithMaxOutputTokens(512),
		fantasy.WithUserAgent(memoryUserAgent),
	)

	resp, err := agent.Stream(copilot.ContextWithInitiatorType(ctx, copilot.InitiatorAgent), fantasy.AgentStreamCall{
		Prompt:          prompt,
		ProviderOptions: getProviderOptions(model, providerCfg),
		PrepareStep: func(callCtx context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			callCtx = copilot.ContextWithInitiatorType(callCtx, copilot.InitiatorAgent)
			prepared.Messages = options.Messages
			if providerCfg.SystemPromptPrefix != "" {
				prepared.Messages = append([]fantasy.Message{
					fantasy.NewSystemMessage(providerCfg.SystemPromptPrefix),
				}, prepared.Messages...)
			}
			return callCtx, prepared, nil
		},
	})
	if err != nil {
		slog.Warn("LLM memory relevance selection failed, falling back to string matching", "error", err)
		return fallbackMemorySearch(ctx, memorySvc, query, scope)
	}
	if resp == nil {
		return fallbackMemorySearch(ctx, memorySvc, query, scope)
	}

	content := resp.Response.Content.Text()
	selectedKeys := parseMemorySelectionResponse(content)
	if len(selectedKeys) == 0 {
		return fallbackMemorySearch(ctx, memorySvc, query, scope)
	}

	entries := make([]memory.Entry, 0, len(selectedKeys))
	for _, key := range selectedKeys {
		entry, err := memorySvc.Get(ctx, key)
		if err == nil {
			if scope == "" || strings.EqualFold(entry.Scope, scope) {
				entries = append(entries, entry)
			}
		}
	}

	if len(entries) == 0 {
		return fallbackMemorySearch(ctx, memorySvc, query, scope)
	}

	return entries
}

func buildMemoryManifest(infos []memory.MemoryFileInfo) string {
	var sb strings.Builder
	for i, info := range infos {
		fmt.Fprintf(&sb, "%d. [%s] %s — %s", i+1, info.Type, info.Key, info.Description)
		if len(info.Tags) > 0 {
			fmt.Fprintf(&sb, " (#%s)", strings.Join(info.Tags, " #"))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func parseMemorySelectionResponse(content string) []string {
	content = strings.TrimSpace(content)

	startIdx := strings.Index(content, "[")
	endIdx := strings.LastIndex(content, "]")
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		return nil
	}

	jsonArray := content[startIdx : endIdx+1]
	var keys []string
	if err := json.Unmarshal([]byte(jsonArray), &keys); err != nil {
		return nil
	}

	result := make([]string, 0, len(keys))
	seen := make(map[string]bool)
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, key)
		if len(result) >= memoryRelevanceMaxSelected {
			break
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func fallbackMemorySearch(ctx context.Context, memorySvc memory.Service, query, scope string) []memory.Entry {
	params := memory.SearchParams{
		Query: query,
		Limit: autoRecallMemoryLimit,
	}
	if scope != "" {
		params.Scope = scope
	}
	entries, err := memorySvc.Search(ctx, params)
	if err != nil {
		return nil
	}
	return entries
}

const memoryExtractMaxTurns = 5

const memoryExtractionThrottleTurns = 1

const memoryExtractPrompt = `You are a memory extraction agent. Analyze the conversation transcript and extract durable knowledge that should be remembered for future sessions.

Rules:
- Extract user preferences, project context, coding patterns, important decisions
- Do NOT extract transient information (file contents, temporary state)
- Format each memory as a markdown file with YAML frontmatter
- Create new memory files only when genuinely useful information is found
- Return JSON with array of memories: [{"key": "...", "description": "...", "content": "..."}]
- Return empty array [] if nothing worth remembering

Example output:
[{"key": "user/preferred-style", "description": "User prefers concise code", "content": "# User Preferred Style\\n\\nThe user prefers..."}]`

type extractedMemory struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Type        string `json:"type,omitempty"`
	Scope       string `json:"scope,omitempty"`
}

type memoryExtractionState struct {
	turnsSinceLastExtraction int
	inProgress               bool
	pendingHistory           []string
}

func shouldExtractMemories(turnsSinceLastExtraction int) bool {
	return turnsSinceLastExtraction >= memoryExtractionThrottleTurns
}

func hasMemoryWritesInHistory(history []string) bool {
	for _, h := range history {
		if strings.Contains(h, "memory") && strings.Contains(h, "store") {
			return true
		}
	}
	return false
}

func extractMemories(ctx context.Context, memorySvc memory.Service, bgModel *backgroundModel, sessionID, prompt string, history []string) {
	if memorySvc == nil || bgModel == nil {
		return
	}

	if len(history) < 2 {
		slog.Debug("Not enough conversation history for memory extraction", "session_id", sessionID)
		return
	}

	if hasMemoryWritesInHistory(history) {
		slog.Debug("Skipping extraction - memory writes detected in conversation", "session_id", sessionID)
		return
	}

	historyStr := strings.Join(history, "\n\n")
	if len(historyStr) < 200 {
		slog.Debug("Conversation too short for memory extraction", "session_id", sessionID, "chars", len(historyStr))
		return
	}

	existingMemories, err := memorySvc.ListMemoryFiles()
	if err != nil {
		slog.Warn("Failed to list existing memories for context", "error", err)
	} else if len(existingMemories) > 0 {
		manifest := buildMemoryManifest(existingMemories[:min(len(existingMemories), 20)])
		historyStr = "Existing memories:\n" + manifest + "\n\nConversation:\n" + historyStr
	}

	extractPrompt := fmt.Sprintf("Initial prompt: %s\n\nConversation transcript:\n%s\n\nExtract any durable memories worth saving (avoid duplicates with existing memories):", prompt, historyStr)

	agent := fantasy.NewAgent(
		bgModel.model.Model,
		fantasy.WithSystemPrompt(memoryExtractPrompt),
		fantasy.WithMaxOutputTokens(2048),
		fantasy.WithUserAgent(memoryUserAgent),
	)

	extractCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := agent.Stream(copilot.ContextWithInitiatorType(extractCtx, copilot.InitiatorAgent), fantasy.AgentStreamCall{
		Prompt:          extractPrompt,
		ProviderOptions: getProviderOptions(bgModel.model, bgModel.provider),
		PrepareStep: func(callCtx context.Context, options fantasy.PrepareStepFunctionOptions) (_ context.Context, prepared fantasy.PrepareStepResult, err error) {
			callCtx = copilot.ContextWithInitiatorType(callCtx, copilot.InitiatorAgent)
			prepared.Messages = options.Messages
			if bgModel.provider.SystemPromptPrefix != "" {
				prepared.Messages = append([]fantasy.Message{
					fantasy.NewSystemMessage(bgModel.provider.SystemPromptPrefix),
				}, prepared.Messages...)
			}
			return callCtx, prepared, nil
		},
	})
	if err != nil {
		slog.Warn("Memory extraction failed", "error", err, "session_id", sessionID)
		return
	}
	if resp == nil {
		return
	}

	content := resp.Response.Content.Text()
	memories := parseExtractedMemories(content)
	if len(memories) == 0 {
		slog.Debug("No memories extracted from conversation", "session_id", sessionID)
		return
	}

	for _, mem := range memories {
		fullContent := fmt.Sprintf("# %s\n\n%s", mem.Description, mem.Content)
		params := memory.StoreParams{
			Key:   mem.Key,
			Value: fullContent,
			Type:  cmp.Or(mem.Type, "general"),
		}
		if mem.Scope != "" {
			params.Scope = mem.Scope
		}
		if err := memorySvc.Store(ctx, params); err != nil {
			slog.Warn("Failed to store extracted memory", "error", err, "key", mem.Key)
		} else {
			slog.Info("Stored extracted memory", "key", mem.Key, "session_id", sessionID)
		}
	}
}

func drainPendingExtractions(pendingExtractions *map[string][]context.CancelFunc, timeout time.Duration) {
	allCancels := make([]context.CancelFunc, 0)
	for _, cancels := range *pendingExtractions {
		allCancels = append(allCancels, cancels...)
	}
	if len(allCancels) == 0 {
		return
	}

	time.AfterFunc(timeout, func() {
		for _, cancel := range allCancels {
			cancel()
		}
	})
}

func parseExtractedMemories(content string) []extractedMemory {
	content = strings.TrimSpace(content)

	startIdx := strings.Index(content, "[")
	endIdx := strings.LastIndex(content, "]")
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		return nil
	}

	jsonArray := content[startIdx : endIdx+1]
	var memories []extractedMemory
	if err := json.Unmarshal([]byte(jsonArray), &memories); err != nil {
		return nil
	}

	result := make([]extractedMemory, 0, len(memories))
	for _, mem := range memories {
		if mem.Key == "" || mem.Content == "" {
			continue
		}
		mem.Key = strings.TrimSpace(mem.Key)
		mem.Description = strings.TrimSpace(mem.Description)
		if mem.Description == "" {
			mem.Description = "Extracted from conversation"
		}
		result = append(result, mem)
	}

	return result
}
