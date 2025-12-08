package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"golang.org/x/net/html"
)

// BrowserUserAgent is a realistic browser User-Agent for better compatibility.
const BrowserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// FetchURLAndConvert fetches a URL and converts HTML content to markdown.
func FetchURLAndConvert(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Use realistic browser headers for better compatibility.
	req.Header.Set("User-Agent", BrowserUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status code: %d", resp.StatusCode)
	}

	maxSize := int64(5 * 1024 * 1024) // 5MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	content := string(body)

	if !utf8.ValidString(content) {
		return "", errors.New("response content is not valid UTF-8")
	}

	contentType := resp.Header.Get("Content-Type")

	// Convert HTML to markdown for better AI processing.
	if strings.Contains(contentType, "text/html") {
		// Remove noisy elements before conversion.
		cleanedHTML := removeNoisyElements(content)
		markdown, err := ConvertHTMLToMarkdown(cleanedHTML)
		if err != nil {
			return "", fmt.Errorf("failed to convert HTML to markdown: %w", err)
		}
		content = cleanupMarkdown(markdown)
	} else if strings.Contains(contentType, "application/json") || strings.Contains(contentType, "text/json") {
		// Format JSON for better readability.
		formatted, err := FormatJSON(content)
		if err == nil {
			content = formatted
		}
		// If formatting fails, keep original content.
	}

	return content, nil
}

// removeNoisyElements removes script, style, nav, header, footer, and other
// noisy elements from HTML to improve content extraction.
func removeNoisyElements(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		// If parsing fails, return original content.
		return htmlContent
	}

	// Elements to remove entirely.
	noisyTags := map[string]bool{
		"script":   true,
		"style":    true,
		"nav":      true,
		"header":   true,
		"footer":   true,
		"aside":    true,
		"noscript": true,
		"iframe":   true,
		"svg":      true,
	}

	var removeNodes func(*html.Node)
	removeNodes = func(n *html.Node) {
		var toRemove []*html.Node

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && noisyTags[c.Data] {
				toRemove = append(toRemove, c)
			} else {
				removeNodes(c)
			}
		}

		for _, node := range toRemove {
			n.RemoveChild(node)
		}
	}

	removeNodes(doc)

	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return htmlContent
	}

	return buf.String()
}

// cleanupMarkdown removes excessive whitespace and blank lines from markdown.
func cleanupMarkdown(content string) string {
	// Collapse multiple blank lines into at most two.
	multipleNewlines := regexp.MustCompile(`\n{3,}`)
	content = multipleNewlines.ReplaceAllString(content, "\n\n")

	// Remove trailing whitespace from each line.
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	content = strings.Join(lines, "\n")

	// Trim leading/trailing whitespace.
	content = strings.TrimSpace(content)

	return content
}

// ConvertHTMLToMarkdown converts HTML content to markdown format.
func ConvertHTMLToMarkdown(htmlContent string) (string, error) {
	converter := md.NewConverter("", true, nil)

	markdown, err := converter.ConvertString(htmlContent)
	if err != nil {
		return "", err
	}

	return markdown, nil
}

// FormatJSON formats JSON content with proper indentation.
func FormatJSON(content string) (string, error) {
	var data any
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return "", err
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
