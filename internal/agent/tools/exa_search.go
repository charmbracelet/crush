package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/crush/internal/version"
)

const (
	exaEndpoint       = "https://mcp.exa.ai/mcp"
	exaToolName       = "web_search_exa"
	exaRequestTimeout = 25 * time.Second
	exaResponseLimit  = 4 << 20
	exaResultLimit    = 1000
	exaOutputLimit    = 12000
)

type exaSearchRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  exaSearchParams `json:"params"`
}

type exaSearchParams struct {
	Name      string             `json:"name"`
	Arguments exaSearchArguments `json:"arguments"`
}

type exaSearchArguments struct {
	Query      string `json:"query"`
	NumResults int    `json:"numResults"`
}

type exaSearchResponse struct {
	Result *struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	} `json:"result"`
	Error *exaRPCError `json:"error"`
}

type exaRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func searchExa(ctx context.Context, client *http.Client, apiKey, query string, maxResults int) ([]SearchResult, error) {
	if maxResults <= 0 {
		maxResults = 10
	}
	endpoint := exaEndpoint
	if apiKey != "" {
		endpoint += "?" + url.Values{"exaApiKey": {apiKey}}.Encode()
	}

	body, err := json.Marshal(exaSearchRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: exaSearchParams{
			Name: exaToolName,
			Arguments: exaSearchArguments{
				Query:      query,
				NumResults: maxResults,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode Exa request: %w", err)
	}

	callCtx, cancel := context.WithTimeout(ctx, exaRequestTimeout)
	defer cancel()

	var responseBody []byte
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(callCtx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, errors.New("failed to create Exa request")
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("User-Agent", "crush/"+version.Version)
		req.Header.Set("x-exa-integration", "crush")

		resp, err := client.Do(req)
		if err != nil {
			var urlErr *url.Error
			if errors.As(err, &urlErr) && urlErr.Err != nil {
				return nil, fmt.Errorf("failed to execute Exa search: %w", urlErr.Err)
			}
			return nil, errors.New("failed to execute Exa search")
		}
		responseBody, err = io.ReadAll(io.LimitReader(resp.Body, exaResponseLimit+1))
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read Exa response: %w", err)
		}
		if len(responseBody) > exaResponseLimit {
			return nil, fmt.Errorf("exa response exceeded %d bytes", exaResponseLimit)
		}
		if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
			if resp.StatusCode < 400 {
				break
			}
			return nil, fmt.Errorf("exa search failed with status code %d", resp.StatusCode)
		}
		if attempt == 1 {
			return nil, fmt.Errorf("exa search failed with status code %d", resp.StatusCode)
		}
		if err := waitExaRetry(callCtx, resp.Header.Get("Retry-After")); err != nil {
			return nil, err
		}
	}

	text, err := parseExaResponse(responseBody)
	if err != nil {
		return nil, err
	}
	return parseExaSearchResults(text, maxResults), nil
}

func waitExaRetry(ctx context.Context, retryAfter string) error {
	delay := 100 * time.Millisecond
	if seconds, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && seconds >= 0 && seconds <= 5 {
		delay = time.Duration(seconds) * time.Second
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func parseExaResponse(body []byte) (string, error) {
	if text, ok, err := parseExaPayload(bytes.TrimSpace(body)); ok {
		if err != nil {
			return "", err
		}
		return text, nil
	}

	var texts []string
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 1024), exaResponseLimit)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		if text, ok, err := parseExaPayload([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data: ")))); ok {
			if err != nil {
				return "", err
			}
			texts = append(texts, text)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to parse Exa response: %w", err)
	}
	if len(texts) == 0 {
		return "", errors.New("exa response contained no search results")
	}
	return strings.Join(texts, "\n"), nil
}

func parseExaPayload(body []byte) (string, bool, error) {
	var response exaSearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", false, nil
	}
	if response.Error != nil {
		return "", true, fmt.Errorf("exa JSON-RPC error %d: %s", response.Error.Code, response.Error.Message)
	}
	if response.Result == nil {
		return "", false, nil
	}
	if response.Result.IsError {
		var messages []string
		for _, content := range response.Result.Content {
			if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
				messages = append(messages, strings.TrimSpace(content.Text))
			}
		}
		if len(messages) == 0 {
			return "", true, errors.New("exa tool call failed")
		}
		return "", true, fmt.Errorf("exa tool call failed: %s", strings.Join(messages, " "))
	}
	if len(response.Result.Content) == 0 {
		return "", false, nil
	}
	for _, content := range response.Result.Content {
		if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
			return content.Text, true, nil
		}
	}
	return "", false, nil
}

func parseExaSearchResults(blob string, maxResults int) []SearchResult {
	if maxResults <= 0 {
		maxResults = 10
	}
	lines := strings.Split(blob, "\n")
	var starts []int
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "Title: ") {
			starts = append(starts, i)
		}
	}
	if len(starts) == 0 {
		if strings.TrimSpace(blob) == "" {
			return nil
		}
		return []SearchResult{{
			Title:    "Exa search results",
			Snippet:  truncateRunes(cleanExaText(blob), exaResultLimit),
			Position: 1,
		}}
	}

	results := make([]SearchResult, 0, min(len(starts), maxResults))
	for i, start := range starts {
		if len(results) >= maxResults {
			break
		}
		end := len(lines)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		result := parseExaEntry(lines[start:end])
		if result.Title == "" {
			continue
		}
		result.Position = len(results) + 1
		result.Snippet = truncateRunes(result.Snippet, exaResultLimit)
		results = append(results, result)
	}
	return results
}

func parseExaEntry(lines []string) SearchResult {
	var result SearchResult
	var highlights []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Title: "):
			result.Title = strings.TrimSpace(strings.TrimPrefix(line, "Title: "))
		case strings.HasPrefix(line, "URL: "):
			result.Link = strings.TrimSpace(strings.TrimPrefix(line, "URL: "))
		case strings.HasPrefix(line, "Highlights:"):
			highlights = append(highlights, strings.TrimSpace(strings.TrimPrefix(line, "Highlights:")))
		case line != "" && len(highlights) > 0:
			highlights = append(highlights, line)
		}
	}
	result.Snippet = cleanExaText(strings.Join(highlights, " "))
	return result
}

func cleanExaText(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, " ... ", " ")
	value = strings.ReplaceAll(value, " … ", " ")
	lines := strings.Split(value, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "> "))
		if line == "" || line == "..." || line == "…" {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.Join(strings.Fields(strings.Join(cleaned, " ")), " ")
}

func truncateRunes(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 1 {
		return "…"
	}
	return string(runes[:limit-1]) + "…"
}

func truncateBytes(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	if limit <= len("…") {
		return "…"
	}
	keep := limit - len("…")
	if keep <= 0 {
		return "…"
	}
	if keep > len(value) {
		keep = len(value)
	}
	for keep > 0 && !utf8.RuneStart(value[keep-1]) {
		keep--
	}
	return value[:keep] + "…"
}
