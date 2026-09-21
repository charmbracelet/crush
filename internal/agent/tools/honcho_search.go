package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"unicode/utf8"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/honcho"
)

// Tool names for the memory tools. They share the honcho_ prefix so a
// user scanning the tool list can see at a glance which capabilities
// depend on a configured memory backend.
const (
	HonchoSearchToolName   = "honcho_search"
	HonchoChatToolName     = "honcho_chat"
	HonchoRememberToolName = "honcho_remember"
	HonchoStatusToolName   = "honcho_status"
)

//go:embed honcho_search.md
var honchoSearchDescription string

// memoryNotConfigured is the response every memory tool returns when
// it was built without a service.
//
// This is an error response rather than a Go error because the model
// can act on it: the fix is a user-side configuration change, not a
// retry, and saying so plainly stops the model from trying the same
// call again.
func memoryNotConfigured() fantasy.ToolResponse {
	return fantasy.NewTextErrorResponse(
		"Memory is not configured, so there is nothing to search or recall. " +
			"Enable it by setting HONCHO_API_KEY (or HONCHO_URL for a self-hosted " +
			"deployment), or by adding a honcho section to the Crush config.",
	)
}

// honchoSearchDefaultLimit and honchoSearchMaxLimit bound how many
// past messages a single search returns. The default is small because
// search results land in the transcript verbatim; the maximum keeps a
// curious model from pulling a whole session back into context.
const (
	honchoSearchDefaultLimit = 5
	honchoSearchMaxLimit     = 50
)

// honchoSearchSnippet bounds a single result. Stored messages can be
// long tool outputs, and a search is meant to locate a memory rather
// than replay it.
const honchoSearchSnippet = 500

type HonchoSearchParams struct {
	Query string `json:"query" description:"What to look for in past conversation. Plain language works better than keywords."`
	Limit int    `json:"limit,omitempty" description:"Maximum number of messages to return. Defaults to 5, maximum 50."`
}

type HonchoSearchResponseMetadata struct {
	Query   string `json:"query"`
	Limit   int    `json:"limit"`
	Matches int    `json:"matches"`
}

// NewHonchoSearchTool builds the message search tool. A nil service
// yields a tool that explains memory is unconfigured rather than one
// that is absent, so the failure is visible if it is ever registered
// by mistake.
func NewHonchoSearchTool(svc *honcho.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		HonchoSearchToolName,
		honchoSearchDescription,
		func(ctx context.Context, params HonchoSearchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if svc == nil {
				return memoryNotConfigured(), nil
			}
			query := strings.TrimSpace(params.Query)
			if query == "" {
				return fantasy.NewTextErrorResponse("query is required"), nil
			}

			limit := params.Limit
			if limit <= 0 {
				limit = honchoSearchDefaultLimit
			}
			limit = min(limit, honchoSearchMaxLimit)

			id := svc.Identity()
			msgs, err := svc.Client().SearchSession(ctx, id.SessionKey, honcho.SearchQuery{
				Query: query,
				Limit: limit,
			})
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("memory search failed: %v", err)), nil
			}

			metadata := HonchoSearchResponseMetadata{
				Query:   query,
				Limit:   limit,
				Matches: len(msgs),
			}

			if len(msgs) == 0 {
				result := fmt.Sprintf(
					"No past messages matched %q in session %s. Nothing was recorded on this topic yet.",
					query, id.SessionKey,
				)
				return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result), metadata), nil
			}

			var out strings.Builder
			fmt.Fprintf(&out, "Found %d past message(s) matching %q:\n", len(msgs), query)
			for i, m := range msgs {
				speaker := m.PeerID
				if speaker == "" {
					speaker = "unknown"
				}
				fmt.Fprintf(&out, "\n%d. [%s] %s\n", i+1, speaker, honchoTruncate(m.Content, honchoSearchSnippet))
			}
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(out.String()), metadata), nil
		},
	)
}

// honchoTruncate shortens s to at most n bytes on a rune boundary,
// marking that it was cut so the model does not read a clipped
// sentence as the whole thought.
func honchoTruncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || len(s) <= n {
		return s
	}
	// Drop a trailing partial rune so the result stays valid UTF-8.
	cut := s[:n]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return strings.TrimSpace(cut) + "..."
}
