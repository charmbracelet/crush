package notebook

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"strings"

	"charm.land/fantasy"
)

//go:embed notebook_entry.md
var notebookEntryPrompt []byte

// llmGenerator implements the Generator interface using a small LLM
// model to produce structured notebook entries.
type llmGenerator struct {
	resolveModel   func() fantasy.LanguageModel
	maxEntryTokens int64
}

// NewLLMGenerator creates a Generator that uses the given model
// resolver to obtain the small model at generation time. The resolver
// is called lazily so the model can be configured after the notebook
// service is created.
func NewLLMGenerator(modelResolver func() fantasy.LanguageModel) Generator {
	return &llmGenerator{
		resolveModel:   modelResolver,
		maxEntryTokens: 1000, // Default; overridden by service.
	}
}

// computeMaxTokens returns a scaled max output token budget for a
// batched generation call. Each event may produce up to
// opts.MaxEntryTokens, so we scale linearly with a 25% buffer for
// markdown overhead and delimiters. Clamped to [2000, 16000].
func computeMaxTokens(numEvents int, maxEntryTokens int64) int64 {
	if numEvents <= 0 {
		numEvents = 1
	}
	tokens := int64(numEvents) * maxEntryTokens * 5 / 4
	if tokens < 2000 {
		return 2000
	}
	if tokens > 16000 {
		return 16000
	}
	return tokens
}

// Generate takes classified events and returns structured entry texts
// via a single batched LLM call.
func (g *llmGenerator) Generate(ctx context.Context, sessionID string, events []EntryInput) ([]GeneratedEntry, error) {
	if len(events) == 0 {
		return nil, nil
	}

	model := g.resolveModel()
	if model == nil {
		// Fallback: create simple entries without LLM.
		var entries []GeneratedEntry
		for _, event := range events {
			entries = append(entries, GeneratedEntry{
				EventType: event.EventType,
				Title:     event.Title,
				Text:      fmt.Sprintf("## %s\n\n%s\n", event.Title, truncate(event.Description, 800)),
				Tags:      defaultTagsForEvent(event),
			})
		}
		return entries, nil
	}

	// Build the prompt with all events.
	var promptSB strings.Builder
	promptSB.WriteString("Generate a notebook entry for each of the following events. ")
	promptSB.WriteString("Use the exact format from the instructions. ")
	promptSB.WriteString("Separate entries with '---' on its own line.\n\n")

	for i, event := range events {
		promptSB.WriteString(fmt.Sprintf("### Event %d\n", i+1))
		promptSB.WriteString("Type: ")
		promptSB.WriteString(event.EventType)
		promptSB.WriteString("\n")
		promptSB.WriteString("Title: ")
		promptSB.WriteString(event.Title)
		promptSB.WriteString("\n")
		promptSB.WriteString("Details:\n")
		promptSB.WriteString(event.Description)
		promptSB.WriteString("\n\n")
	}

	agent := fantasy.NewAgent(
		model,
		fantasy.WithSystemPrompt(string(notebookEntryPrompt)),
		fantasy.WithMaxOutputTokens(computeMaxTokens(len(events), g.maxEntryTokens)),
	)

	resp, err := agent.Stream(ctx, fantasy.AgentStreamCall{
		Prompt: promptSB.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate notebook entries: %w", err)
	}

	text := resp.Response.Content.Text()
	entries := parseGeneratedEntries(text, events)
	if len(entries) == 0 {
		// Fallback: create simple entries from the event inputs.
		slog.Warn("LLM returned no parseable notebook entries, using fallback")
		for _, event := range events {
			entries = append(entries, GeneratedEntry{
				EventType: event.EventType,
				Title:     event.Title,
				Text:      fmt.Sprintf("## %s\n\n%s\n", event.Title, truncate(event.Description, 800)),
				Tags:      defaultTagsForEvent(event),
			})
		}
	}
	return entries, nil
}

// parseGeneratedEntries splits the LLM output by '---' delimiters and
// extracts tags from each section.
func parseGeneratedEntries(text string, events []EntryInput) []GeneratedEntry {
	sections := strings.Split(text, "\n---\n")
	var entries []GeneratedEntry
	for i, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}
		tags := extractTags(section)
		title := extractTitle(section)
		eventType := EventGeneral
		if i < len(events) {
			eventType = events[i].EventType
		}
		entries = append(entries, GeneratedEntry{
			EventType: eventType,
			Title:     title,
			Text:      section,
			Tags:      tags,
		})
	}
	return entries
}

// extractTags finds all #tag patterns in the text. A tag is a token
// starting with a single # followed by a non-space character that is
// not itself a # (to avoid matching markdown headings like ## or ###).
// The leading # is stripped before returning so stored tags use a
// consistent format (e.g., "file:auth.go" not "#file:auth.go").
func extractTags(text string) []string {
	var tags []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		for _, f := range strings.Fields(line) {
			if len(f) < 2 || f[0] != '#' || f[1] == '#' {
				continue // Skip markdown headings (##, ###).
			}
			tag := strings.TrimPrefix(f, "#")
			if !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

// extractTitle extracts the title from a markdown heading.
func extractTitle(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			return strings.TrimPrefix(line, "## ")
		}
	}
	return "Entry"
}

// defaultTagsForEvent returns default tags for an event type.
func defaultTagsForEvent(event EntryInput) []string {
	tags := []string{"phase:" + event.EventType}
	if event.ToolCall != nil {
		path := extractPathFromInput(event.ToolCall.Input)
		if path != "" {
			// Use basename for the file tag so recall("file:auth.go")
			// matches regardless of the full path.
			basename := path
			if idx := strings.LastIndex(path, "/"); idx >= 0 {
				basename = path[idx+1:]
			}
			tags = append(tags, "file:"+basename)
		}
	}
	return tags
}
