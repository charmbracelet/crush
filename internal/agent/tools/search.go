package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"golang.org/x/net/html"
)

// SearchResult represents a single search result from DuckDuckGo.
type SearchResult struct {
	Title    string
	Link     string
	Snippet  string
	Position int
}

// searchDuckDuckGo performs a web search using DuckDuckGo's HTML endpoint.
func searchDuckDuckGo(ctx context.Context, client *http.Client, query string, maxResults int) ([]SearchResult, error) {
	if maxResults <= 0 {
		maxResults = 10
	}

	formData := url.Values{}
	formData.Set("q", query)
	formData.Set("b", "")
	formData.Set("kl", "")

	req, err := http.NewRequestWithContext(ctx, "POST", "https://html.duckduckgo.com/html", strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", BrowserUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return parseSearchResults(string(body), maxResults)
}

// parseSearchResults extracts search results from DuckDuckGo HTML response.
func parseSearchResults(htmlContent string, maxResults int) ([]SearchResult, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var results []SearchResult
	var traverse func(*html.Node)

	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "result") {
			result := extractResult(n)
			if result != nil && result.Link != "" && !strings.Contains(result.Link, "y.js") {
				result.Position = len(results) + 1
				results = append(results, *result)
				if len(results) >= maxResults {
					return
				}
			}
		}
		for c := n.FirstChild; c != nil && len(results) < maxResults; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)
	return results, nil
}

// hasClass checks if an HTML node has a specific class.
func hasClass(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			return slices.Contains(strings.Fields(attr.Val), class)
		}
	}
	return false
}

// extractResult extracts a search result from a result div node.
func extractResult(n *html.Node) *SearchResult {
	result := &SearchResult{}

	var traverse func(*html.Node)
	traverse = func(node *html.Node) {
		if node.Type == html.ElementNode {
			// Look for title link.
			if node.Data == "a" && hasClass(node, "result__a") {
				result.Title = getTextContent(node)
				for _, attr := range node.Attr {
					if attr.Key == "href" {
						result.Link = cleanDuckDuckGoURL(attr.Val)
						break
					}
				}
			}
			// Look for snippet.
			if node.Data == "a" && hasClass(node, "result__snippet") {
				result.Snippet = getTextContent(node)
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(n)
	return result
}

// getTextContent extracts all text content from a node and its children.
func getTextContent(n *html.Node) string {
	var text strings.Builder
	var traverse func(*html.Node)

	traverse = func(node *html.Node) {
		if node.Type == html.TextNode {
			text.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(n)
	return strings.TrimSpace(text.String())
}

// cleanDuckDuckGoURL extracts the actual URL from DuckDuckGo's redirect URL.
func cleanDuckDuckGoURL(rawURL string) string {
	if strings.HasPrefix(rawURL, "//duckduckgo.com/l/?uddg=") {
		// Extract the actual URL from the redirect.
		if idx := strings.Index(rawURL, "uddg="); idx != -1 {
			encoded := rawURL[idx+5:]
			if ampIdx := strings.Index(encoded, "&"); ampIdx != -1 {
				encoded = encoded[:ampIdx]
			}
			decoded, err := url.QueryUnescape(encoded)
			if err == nil {
				return decoded
			}
		}
	}
	return rawURL
}

// formatSearchResults formats search results for LLM consumption.
func formatSearchResults(results []SearchResult) string {
	if len(results) == 0 {
		return "No results were found for your search query. This could be due to DuckDuckGo's bot detection or the query returned no matches. Please try rephrasing your search or try again in a few minutes."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d search results:\n\n", len(results)))

	for _, result := range results {
		sb.WriteString(fmt.Sprintf("%d. %s\n", result.Position, result.Title))
		sb.WriteString(fmt.Sprintf("   URL: %s\n", result.Link))
		sb.WriteString(fmt.Sprintf("   Summary: %s\n\n", result.Snippet))
	}

	return sb.String()
}
