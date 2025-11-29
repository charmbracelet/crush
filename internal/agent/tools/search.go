package tools

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/PuerkitoBio/goquery"
	"github.com/charmbracelet/crush/internal/permission"
)

// SearchToolName is the name of the search tool.
const SearchToolName = "search"

//go:embed search.md
var searchDescription []byte

// SearchParams defines the parameters for the search tool.
type SearchParams struct {
	// Query is the search query string.
	Query string `json:"query" description:"The search query. Keep it concise and specific."`

	// Provider is the search engine provider to use: "duckduckgo", "brave", or "google". Defaults to "duckduckgo".
	Provider string `json:"provider,omitempty" description:"Search engine to use (duckduckgo, brave, google). Defaults to duckduckgo."`

	// MaxResults limits the number of results returned. Defaults to 10, max 20.
	MaxResults int `json:"max_results,omitempty" description:"Maximum results to return (1-20, default 10)."`

	// Site restricts search to a specific domain (e.g., "docs.python.org").
	Site string `json:"site,omitempty" description:"Restrict search to a specific domain (e.g., 'docs.python.org')."`
}

// SearchPermissionsParams defines the permission parameters for the search tool.
type SearchPermissionsParams struct {
	Query      string `json:"query"`
	Provider   string `json:"provider,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
	Site       string `json:"site,omitempty"`
}

// SearchResult represents a single search result.
type SearchResult struct {
	Title     string `json:"title"`
	URL       string `json:"url"`
	Snippet   string `json:"snippet"`
	Published string `json:"published,omitempty"`
}

// SearchResponseMetadata contains the search results.
type SearchResponseMetadata struct {
	Results []SearchResult `json:"results"`
	Provider string         `json:"provider"`
	Query    string         `json:"query"`
	Count    int            `json:"count"`
}

type searchTool struct {
	client  *http.Client
	rateLimiter *ratelimiter
}

type ratelimiter struct {
	lastRequest time.Time
	minInterval time.Duration
}

func (r *ratelimiter) Wait() {
	now := time.Now()
	elapsed := now.Sub(r.lastRequest)
	if elapsed < r.minInterval {
		time.Sleep(r.minInterval - elapsed)
	}
	r.lastRequest = time.Now()
}

// NewSearchTool creates a new search tool that can query search engines.
func NewSearchTool(permissions permission.Service, workingDir string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		SearchToolName,
		string(searchDescription),
		func(ctx context.Context, params SearchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			// Validate parameters
			if params.Query == "" {
				return fantasy.NewTextErrorResponse("query parameter is required"), nil
			}

			if params.MaxResults < 1 || params.MaxResults > 20 {
				params.MaxResults = 10
			}

			if params.Provider == "" {
				params.Provider = "duckduckgo"
			}

			// Request permissions
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for web search")
			}

			p := permissions.Request(
				permission.CreatePermissionRequest{
					SessionID:   sessionID,
					Path:        workingDir,
					ToolCallID:  call.ID,
					ToolName:    SearchToolName,
					Action:      "search",
					Description: fmt.Sprintf("Search for: %s (provider: %s)", params.Query, params.Provider),
					Params:      SearchPermissionsParams(params),
				},
			)

			if !p {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			// Initialize search tool
			tool := &searchTool{
				client: &http.Client{
					Timeout: 30 * time.Second,
				},
				rateLimiter: &ratelimiter{
					minInterval: 500 * time.Millisecond, // Rate limit to avoid overwhelming services
				},
			}

			// Perform search
			results, err := tool.search(ctx, params)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("search failed: %w", err)
			}

			if len(results) == 0 {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("No results found for \"%s\" using %s", params.Query, params.Provider)), nil
			}

			// Limit output in response
			output := fmt.Sprintf("Found %d results for \"%s\" using %s:\n\n", len(results), params.Query, params.Provider)
			for i, result := range results {
				output += fmt.Sprintf("%d. %s\n   %s\n   %s\n\n", i+1, result.Title, result.URL, result.Snippet)
			}

			// Prepare response metadata
			metadata := SearchResponseMetadata{
				Results:  results,
				Provider: params.Provider,
				Query:    params.Query,
				Count:    len(results),
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(output),
				metadata,
			), nil
		},
	)
}

// search performs the actual search using the configured provider.
func (s *searchTool) search(ctx context.Context, params SearchParams) ([]SearchResult, error) {
	s.rateLimiter.Wait()

	switch strings.ToLower(params.Provider) {
	case "duckduckgo":
		return s.searchDuckDuckGo(ctx, params)
	case "brave":
		return s.searchBrave(ctx, params)
	case "google":
		return s.searchGoogle(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported search provider: %s (supported: duckduckgo, brave, google)", params.Provider)
	}
}

// searchDuckDuckGo performs a search using DuckDuckGo's HTML interface.
func (s *searchTool) searchDuckDuckGo(ctx context.Context, params SearchParams) ([]SearchResult, error) {
	// Build query URL with site restriction if specified
	query := params.Query
	if params.Site != "" {
		query = fmt.Sprintf("%s site:%s", query, params.Site)
	}

	baseURL := "https://duckduckgo.com/html/"
	searchURL := fmt.Sprintf("%s?q=%s", baseURL, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "crush/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed with status: %d", resp.StatusCode)
	}

	// Parse DuckDuckGo HTML results
	results := parseDuckDuckGoResults(resp.Body)
	
	// Limit results
	if len(results) > params.MaxResults {
		results = results[:params.MaxResults]
	}

	return results, nil
}

// searchBrave performs a search using Brave Search API.
func (s *searchTool) searchBrave(ctx context.Context, params SearchParams) ([]SearchResult, error) {
	// Note: Brave Search API requires an API key
	// For now, we'll use a simpler approach or return an error
	// In production, you'd want to handle API key configuration
	
	// Attempt to use Brave's no-js interface as fallback
	query := params.Query
	if params.Site != "" {
		query = fmt.Sprintf("%s site:%s", query, params.Site)
	}

	searchURL := fmt.Sprintf("https://search.brave.com/search?q=%s", url.QueryEscape(query))
	
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "crush/1.0")
	
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed with status: %d", resp.StatusCode)
	}

	// Parse Brave results
	results := parseBraveResults(resp.Body)
	
	if len(results) > params.MaxResults {
		results = results[:params.MaxResults]
	}

	return results, nil
}

// searchGoogle performs a search using Google Custom Search API.
func (s *searchTool) searchGoogle(ctx context.Context, params SearchParams) ([]SearchResult, error) {
	// For now, return a helpful error since Google requires API keys
	return nil, fmt.Errorf("Google search requires an API key configuration. Try using 'duckduckgo' provider instead.")
}

func parseDuckDuckGoResults(body io.Reader) []SearchResult {
	var results []SearchResult
	
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return results
	}
	
	// DuckDuckGo HTML selector for search results
	doc.Find(".results .result").Each(func(i int, s *goquery.Selection) {
		// Extract title and URL
		titleLink := s.Find("a.result__a")
		title := strings.TrimSpace(titleLink.Text())
		href, exists := titleLink.Attr("href")
		if !exists || title == "" {
			return
		}
		
		// Extract snippet
		snippet := s.Find(".result__snippet").Text()
		snippet = strings.TrimSpace(snippet)
		
		// Only add results with content
		if title != "" && href != "" {
			result := SearchResult{
				Title:   sanitizeText(title),
				URL:     sanitizeURL(href),
				Snippet: sanitizeText(snippet),
			}
			results = append(results, result)
		}
	})
	
	return results
}

func parseBraveResults(body io.Reader) []SearchResult {
	var results []SearchResult
	
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return results
	}
	
	// Brave uses different selectors
	doc.Find(".snippet").Each(func(i int, s *goquery.Selection) {
		titleLink := s.Find("a").First()
		title := strings.TrimSpace(titleLink.Text())
		href, exists := titleLink.Attr("href")
		if !exists || title == "" || href == "" {
			return
		}
		
		snippet := s.Find(".snippet-content").Text()
		snippet = strings.TrimSpace(snippet)
		
		result := SearchResult{
			Title:   sanitizeText(title),
			URL:     sanitizeURL(href),
			Snippet: sanitizeText(snippet),
		}
		results = append(results, result)
	})
	
	return results
}

func sanitizeText(text string) string {
	// Remove extra whitespace and newlines
	text = strings.Join(strings.Fields(text), " ")
	// Limit length to avoid overly long snippets
	if len(text) > 500 {
		text = text[:497] + "..."
	}
	return text
}

func sanitizeURL(href string) string {
	// DuckDuckGo sometimes uses /l/?u=encodedurl
	if strings.HasPrefix(href, "//duckduckgo.com/l/?u=") {
		u, err := url.Parse(href)
		if err == nil {
			if targetURL := u.Query().Get("u"); targetURL != "" {
				return targetURL
			}
		}
	}
	
	// Ensure URL has scheme
	if strings.HasPrefix(href, "www.") {
		return "https://" + href
	}
	
	return href
}