package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/memory"
)

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

	if len(infos) > memoryRelevanceMaxFiles {
		infos = infos[:memoryRelevanceMaxFiles]
	}

	manifest := buildMemoryManifest(infos)

	prompt := fmt.Sprintf("Query: %s\n\nMemory manifest:\n%s\n\nSelect relevant memory keys:", query, manifest)

	agent := fantasy.NewAgent(
		model.Model,
		fantasy.WithSystemPrompt(memoryRelevancePrompt),
		fantasy.WithMaxOutputTokens(512),
	)

	resp, err := agent.Stream(ctx, fantasy.AgentStreamCall{
		Prompt:          prompt,
		ProviderOptions: getProviderOptions(model, providerCfg),
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
			entries = append(entries, entry)
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
		sb.WriteString(fmt.Sprintf("%d. [%s] %s — %s", i+1, info.Type, info.Key, info.Description))
		if len(info.Tags) > 0 {
			sb.WriteString(fmt.Sprintf(" (#%s)", strings.Join(info.Tags, " #")))
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
