package tools

import (
	"context"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

type mcpSourceEvidenceKey struct{}

type mcpSourceEvidence struct {
	mu         sync.RWMutex
	required   bool
	discovered map[string]struct{}
	fetched    map[string]struct{}
	rejected   map[string]string
}

var httpURLPattern = regexp.MustCompile(`https?://[^\s<>"']+`)

// WithMCPSourceEvidence adds a per-turn ledger of MCP configuration sources.
// It is intentionally transient and never persisted.
func WithMCPSourceEvidence(ctx context.Context, prompts ...string) context.Context {
	if _, ok := ctx.Value(mcpSourceEvidenceKey{}).(*mcpSourceEvidence); ok {
		return ctx
	}
	evidence := &mcpSourceEvidence{
		required:   len(prompts) == 0,
		discovered: make(map[string]struct{}),
		fetched:    make(map[string]struct{}),
		rejected:   make(map[string]string),
	}
	for _, prompt := range prompts {
		if strings.Contains(strings.ToLower(prompt), "mcp") {
			evidence.required = true
		}
		for _, rawURL := range httpURLPattern.FindAllString(prompt, -1) {
			evidence.discovered[normalizeMCPSourceURL(strings.TrimRight(rawURL, ".,;:!?)]}"))] = struct{}{}
		}
	}
	return context.WithValue(ctx, mcpSourceEvidenceKey{}, evidence)
}

func recordMCPSearchResults(ctx context.Context, results []SearchResult) {
	evidence, ok := ctx.Value(mcpSourceEvidenceKey{}).(*mcpSourceEvidence)
	if !ok {
		return
	}
	evidence.mu.Lock()
	defer evidence.mu.Unlock()
	for _, result := range results {
		if normalized := normalizeMCPSourceURL(result.Link); normalized != "" {
			evidence.discovered[normalized] = struct{}{}
		}
	}
}

func recordMCPSourceEvidence(ctx context.Context, sourceURL, content string) string {
	evidence, ok := ctx.Value(mcpSourceEvidenceKey{}).(*mcpSourceEvidence)
	if !ok {
		return "MCP source evidence ledger is unavailable"
	}
	key := normalizeMCPSourceURL(sourceURL)
	evidence.mu.Lock()
	defer evidence.mu.Unlock()
	if _, ok := evidence.discovered[key]; !ok {
		reason := "source URL was not supplied by the user or returned by web_search"
		evidence.rejected[key] = reason
		return reason
	}
	if reason := unusableMCPSourceReason(content); reason != "" {
		evidence.rejected[key] = reason
		return reason
	}
	evidence.fetched[key] = struct{}{}
	delete(evidence.rejected, key)
	return ""
}

func mcpSourceEvidenceError(ctx context.Context, sourceURL string) string {
	evidence, ok := ctx.Value(mcpSourceEvidenceKey{}).(*mcpSourceEvidence)
	if !ok {
		return "source evidence ledger is unavailable"
	}
	key := normalizeMCPSourceURL(sourceURL)
	evidence.mu.RLock()
	defer evidence.mu.RUnlock()
	if _, ok := evidence.fetched[key]; ok {
		return ""
	}
	if reason := evidence.rejected[key]; reason != "" {
		return reason
	}
	if _, ok := evidence.discovered[key]; ok {
		return "source URL was located but not successfully fetched"
	}
	return "source URL was not supplied by the user or returned by web_search"
}

func hasMCPSourceEvidence(ctx context.Context, sourceURL string) bool {
	return mcpSourceEvidenceError(ctx, sourceURL) == ""
}

func mcpSourceEvidenceRequired(ctx context.Context) bool {
	evidence, ok := ctx.Value(mcpSourceEvidenceKey{}).(*mcpSourceEvidence)
	return ok && evidence.required
}

func normalizeMCPSourceURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	return parsed.String()
}

func unusableMCPSourceReason(content string) string {
	lower := strings.ToLower(content)
	if strings.Contains(lower, "{{ message }}") && strings.Contains(lower, "uh oh") ||
		strings.Contains(lower, "# sign in to github") && strings.Contains(lower, "error while loading") ||
		strings.Contains(lower, "access denied") || strings.Contains(lower, "captcha") {
		return "fetched page is a login, access-denied, or error interstitial"
	}
	if !strings.Contains(lower, "mcp") && !strings.Contains(lower, "model context protocol") {
		return "fetched page does not describe an MCP server"
	}
	for _, marker := range []string{"install", "configuration", "command", "args", "stdio", "http", "endpoint", "package"} {
		if strings.Contains(lower, marker) {
			return ""
		}
	}
	return "fetched page does not contain MCP installation or configuration guidance"
}
