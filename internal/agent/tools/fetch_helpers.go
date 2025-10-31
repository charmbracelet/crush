package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	md "github.com/JohannesKaufmann/html-to-markdown"
)

// FetchURLAndConvert fetches a URL and converts HTML content to markdown.
func FetchURLAndConvert(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "crush/1.0")

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
		markdown, err := ConvertHTMLToMarkdown(content)
		if err != nil {
			return "", fmt.Errorf("failed to convert HTML to markdown: %w", err)
		}
		content = markdown
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

// ConvertHTMLToMarkdown converts HTML content to markdown format.
func ConvertHTMLToMarkdown(html string) (string, error) {
	converter := md.NewConverter("", true, nil)

	markdown, err := converter.ConvertString(html)
	if err != nil {
		return "", err
	}

	return markdown, nil
}

// FormatJSON formats JSON content with proper indentation.
func FormatJSON(content string) (string, error) {
	var data interface{}
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
