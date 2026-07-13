package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/session"
)

//go:embed add_source.md
var addSourceDescription string

//go:embed sources.md
var sourcesDescription string

//go:embed remove_source.md
var removeSourceDescription string

const (
	AddSourceToolName    = "add_source"
	RemoveSourceToolName = "remove_source"
	SourcesToolName      = "sources"
)

type AddSourceItem struct {
	Value string `json:"value" description:"File path, http(s) URL, or text to attach"`
	Kind  string `json:"kind,omitempty" description:"Optional type: file, url, or text. Omit to detect it"`
	Label string `json:"label,omitempty" description:"Optional short display label"`
}

type AddSourceParams struct {
	Items []AddSourceItem `json:"items" description:"All sources to attach in this batch"`
}

type AddSourceResponseMetadata struct {
	Added   []session.Source `json:"added"`
	Skipped []session.Source `json:"skipped,omitempty"`
}

type SourcesParams struct {
	Action string `json:"action,omitempty" description:"Action: list or resolve. Defaults to list"`
	ID     string `json:"id,omitempty" description:"Source ID required for resolve"`
}

type RemoveSourceParams struct {
	Items []string `json:"items" description:"Source IDs, labels, paths, URLs, or unique descriptive fragments to detach"`
}

func NewAddSourceTool(sessions session.Service, workingDir string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		AddSourceToolName,
		addSourceDescription,
		func(ctx context.Context, params AddSourceParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if len(params.Items) == 0 {
				return fantasy.NewTextErrorResponse("items must contain at least one source"), nil
			}
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for adding sources")
			}
			current, err := sessions.Get(ctx, sessionID)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to get session: %w", err)
			}

			metadata := AddSourceResponseMetadata{}
			for _, item := range params.Items {
				source, normalizeErr := normalizeSource(item, workingDir)
				if normalizeErr != nil {
					return fantasy.NewTextErrorResponse(normalizeErr.Error()), nil
				}
				if existing, ok := matchingSource(current.Sources, source); ok {
					metadata.Skipped = append(metadata.Skipped, existing)
					continue
				}
				current.Sources = append(current.Sources, source)
				metadata.Added = append(metadata.Added, source)
			}

			if len(metadata.Added) > 0 {
				if _, err := sessions.Save(ctx, current); err != nil {
					return fantasy.ToolResponse{}, fmt.Errorf("failed to save sources: %w", err)
				}
			}

			text := fmt.Sprintf("Attached %d source(s).", len(metadata.Added))
			if len(metadata.Skipped) > 0 {
				text += fmt.Sprintf(" %d duplicate(s) were already attached.", len(metadata.Skipped))
			}
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(text), metadata), nil
		},
	)
}

func NewRemoveSourceTool(sessions session.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		RemoveSourceToolName,
		removeSourceDescription,
		func(ctx context.Context, params RemoveSourceParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if len(params.Items) == 0 {
				return fantasy.NewTextErrorResponse("items must contain at least one source ID or label"), nil
			}
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for removing sources")
			}
			current, err := sessions.Get(ctx, sessionID)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to get session: %w", err)
			}

			removed, ambiguous := matchSourcesForRemoval(current.Sources, params.Items)
			if len(ambiguous) > 0 {
				return fantasy.NewTextErrorResponse("source selector is ambiguous; use a more specific label, path, URL, or ID:\n" + strings.Join(ambiguous, "\n")), nil
			}
			if len(removed) == 0 {
				return fantasy.NewTextErrorResponse("no attached source matched the supplied descriptions; call sources with action=list to inspect the current references"), nil
			}
			removedIDs := make(map[string]struct{}, len(removed))
			for _, source := range removed {
				removedIDs[source.ID] = struct{}{}
			}
			kept := make([]session.Source, 0, len(current.Sources)-len(removed))
			for _, source := range current.Sources {
				if _, ok := removedIDs[source.ID]; !ok {
					kept = append(kept, source)
				}
			}
			current.Sources = kept
			if _, err := sessions.Save(ctx, current); err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to save sources: %w", err)
			}
			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(fmt.Sprintf("Detached %d source(s). The underlying files and URLs were not changed.", len(removed))),
				removed,
			), nil
		},
	)
}

func matchSourcesForRemoval(sources []session.Source, selectors []string) (matched []session.Source, ambiguous []string) {
	selected := make(map[string]session.Source)
	for _, selector := range selectors {
		selector = strings.ToLower(strings.TrimSpace(selector))
		if selector == "" {
			continue
		}
		var exact []session.Source
		var partial []session.Source
		for _, source := range sources {
			id := strings.ToLower(source.ID)
			label := strings.ToLower(source.Label)
			location := strings.ToLower(source.Location)
			if selector == id || selector == label || selector == location {
				exact = append(exact, source)
				continue
			}
			if strings.Contains(label, selector) || strings.Contains(location, selector) {
				partial = append(partial, source)
			}
		}
		candidates := exact
		if len(candidates) == 0 {
			candidates = partial
		}
		switch len(candidates) {
		case 0:
			continue
		case 1:
			selected[candidates[0].ID] = candidates[0]
		default:
			var labels []string
			for _, source := range candidates {
				labels = append(labels, fmt.Sprintf("%s [%s] %s", source.ID, source.Kind, source.Label))
			}
			ambiguous = append(ambiguous, fmt.Sprintf("%q matches: %s", selector, strings.Join(labels, "; ")))
		}
	}
	for _, source := range sources {
		if _, ok := selected[source.ID]; ok {
			matched = append(matched, source)
		}
	}
	return matched, ambiguous
}

func NewSourcesTool(sessions session.Service) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		SourcesToolName,
		sourcesDescription,
		func(ctx context.Context, params SourcesParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for listing sources")
			}
			current, err := sessions.Get(ctx, sessionID)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to get session: %w", err)
			}

			switch strings.ToLower(strings.TrimSpace(params.Action)) {
			case "", "list":
				return fantasy.WithResponseMetadata(
					fantasy.NewTextResponse(formatSources(current.Sources)),
					current.Sources,
				), nil
			case "resolve":
				for _, source := range current.Sources {
					if source.ID != params.ID {
						continue
					}
					return fantasy.WithResponseMetadata(
						fantasy.NewTextResponse(resolveSource(source)),
						source,
					), nil
				}
				return fantasy.NewTextErrorResponse(fmt.Sprintf("source %q was not found", params.ID)), nil
			default:
				return fantasy.NewTextErrorResponse("action must be list or resolve"), nil
			}
		},
	)
}

func normalizeSource(item AddSourceItem, workingDir string) (session.Source, error) {
	return session.NewSource(item.Value, session.SourceKind(item.Kind), item.Label, workingDir)
}

func matchingSource(sources []session.Source, candidate session.Source) (session.Source, bool) {
	for _, source := range sources {
		if source.Kind != candidate.Kind {
			continue
		}
		if source.Location != "" && source.Location == candidate.Location {
			return source, true
		}
		if source.Content != "" && source.Content == candidate.Content {
			return source, true
		}
	}
	return session.Source{}, false
}

func formatSources(sources []session.Source) string {
	if len(sources) == 0 {
		return "No sources are attached to this session."
	}
	var lines []string
	for _, source := range sources {
		line := fmt.Sprintf("%s [%s] %s", source.ID, source.Kind, source.Label)
		if source.Location != "" {
			line += " -> " + source.Location
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func resolveSource(source session.Source) string {
	switch source.Kind {
	case session.SourceKindFile:
		return fmt.Sprintf("File source %q selected: %s\nImages and PDFs are attached to the next model step. Use view with this exact path for text or code files.", source.Label, source.Location)
	case session.SourceKindURL:
		return fmt.Sprintf("URL source %q: %s\nUse web_fetch with this exact URL when its content is needed.", source.Label, source.Location)
	default:
		return fmt.Sprintf("Text source %q:\n\n%s", source.Label, source.Content)
	}
}
