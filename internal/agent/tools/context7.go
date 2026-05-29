package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/taigrr/fantasy"
)

const (
	Context7ToolName    = "context7"
	context7BaseURL     = "https://context7.com/api"
	context7Source      = "crush"
	context7HTTPTimeout = 30 * time.Second
)

//go:embed context7.md
var context7Description string

// Context7Params lets the model either look up a library by name (and
// have us resolve the best match) or pin a known library_id directly.
// Either form must be paired with a focused query.
type Context7Params struct {
	Library   string `json:"library,omitempty" description:"Library name to resolve via context7 (e.g. 'react', 'Next.js'). Mutually exclusive with library_id."`
	LibraryID string `json:"library_id,omitempty" description:"Pinned context7 library ID like '/vercel/next.js' or '/vercel/next.js/v15.1.8'. Skips resolution."`
	Query     string `json:"query" description:"Specific question or task; the answer is sliced from the docs to match this."`
}

// context7SearchResult mirrors the fields we use from the v2 search API.
// Trimmed: the API returns more, but the model only needs identity +
// signal of relevance.
type context7SearchResult struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	TrustScore  int    `json:"trustScore"`
	TotalTokens int    `json:"totalTokens"`
	State       string `json:"state"`
}

type context7SearchResponse struct {
	Results []context7SearchResult `json:"results"`
}

// NewContext7Tool returns the context7 tool. The HTTP client can be
// injected for tests; nil falls back to a sensible default.
func NewContext7Tool(httpClient *http.Client) fantasy.AgentTool {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: context7HTTPTimeout}
	}
	return fantasy.NewParallelAgentTool(
		Context7ToolName,
		context7Description,
		func(ctx context.Context, params Context7Params, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if strings.TrimSpace(params.Query) == "" {
				return fantasy.NewTextErrorResponse("query is required"), nil
			}
			if params.Library == "" && params.LibraryID == "" {
				return fantasy.NewTextErrorResponse("library or library_id is required"), nil
			}

			libraryID := params.LibraryID
			var resolutionNote string
			if libraryID == "" {
				match, err := context7Resolve(ctx, httpClient, params.Library, params.Query)
				if err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("context7 resolve failed: %s", err)), nil
				}
				if match == nil {
					return fantasy.NewTextResponse(fmt.Sprintf("No context7 library found for %q", params.Library)), nil
				}
				libraryID = match.ID
				resolutionNote = fmt.Sprintf("# Resolved %q to %s (%s, trust=%d)\n\n", params.Library, match.ID, match.Title, match.TrustScore)
			}

			docs, err := context7Query(ctx, httpClient, libraryID, params.Query)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("context7 query failed: %s", err)), nil
			}

			return fantasy.NewTextResponse(resolutionNote + docs), nil
		},
	)
}

// context7Resolve calls /v2/libs/search and returns the highest-quality
// match, or nil when there are no results. We rank by (state=finalized,
// trustScore desc, totalTokens desc) — the same heuristic the upstream
// MCP server's prompt suggests.
func context7Resolve(ctx context.Context, hc *http.Client, library, query string) (*context7SearchResult, error) {
	u, _ := url.Parse(context7BaseURL + "/v2/libs/search")
	q := u.Query()
	q.Set("libraryName", library)
	q.Set("query", query)
	u.RawQuery = q.Encode()

	body, err := context7Get(ctx, hc, u.String())
	if err != nil {
		return nil, err
	}

	var resp context7SearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}
	if len(resp.Results) == 0 {
		return nil, nil
	}

	best := pickBestResult(resp.Results)
	return best, nil
}

// pickBestResult applies the same ranking the official MCP recommends in
// its prompt: prefer finalized libraries, then higher trust score, then
// larger doc footprint. Stable for tests.
func pickBestResult(results []context7SearchResult) *context7SearchResult {
	var best *context7SearchResult
	for i := range results {
		r := &results[i]
		if best == nil || better(r, best) {
			best = r
		}
	}
	return best
}

func better(a, b *context7SearchResult) bool {
	if (a.State == "finalized") != (b.State == "finalized") {
		return a.State == "finalized"
	}
	if a.TrustScore != b.TrustScore {
		return a.TrustScore > b.TrustScore
	}
	return a.TotalTokens > b.TotalTokens
}

// context7Query calls /v2/context with type=txt so we can pass the
// markdown body straight back to the model without round-tripping JSON.
func context7Query(ctx context.Context, hc *http.Client, libraryID, query string) (string, error) {
	u, _ := url.Parse(context7BaseURL + "/v2/context")
	q := u.Query()
	q.Set("libraryId", libraryID)
	q.Set("query", query)
	q.Set("type", "txt")
	u.RawQuery = q.Encode()

	body, err := context7Get(ctx, hc, u.String())
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// context7Get is the shared GET helper. It applies authentication when
// CONTEXT7_API_KEY is set and converts non-2xx into a clear error so
// the model sees the actual reason (auth, rate limit, library not yet
// finalized).
func context7Get(ctx context.Context, hc *http.Client, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Context7-Source", context7Source)
	if key := os.Getenv("CONTEXT7_API_KEY"); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return body, nil
	case http.StatusAccepted:
		return nil, fmt.Errorf("library is still being indexed by context7; retry later")
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("context7 rejected the API key (check CONTEXT7_API_KEY)")
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("context7 rate limit hit; retry-after=%s", resp.Header.Get("Retry-After"))
	default:
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "…"
		}
		return nil, fmt.Errorf("context7 returned HTTP %d: %s", resp.StatusCode, snippet)
	}
}
