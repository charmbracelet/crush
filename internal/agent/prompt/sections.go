package prompt

import (
	"io"
	"os"
	"sort"
	"strings"
	"unicode/utf8"
)

// CacheClass describes how stable a prompt section is across requests.
// It is used for cache-breakpoint planning: stable sections can share a
// prefix cache while volatile sections change every session or turn.
type CacheClass string

const (
	// CacheClassStable sections are identical across sessions and turns
	// (core policy, project context files, skills).
	CacheClassStable CacheClass = "stable"
	// CacheClassSession sections are stable within a session but differ
	// across sessions (MCP instructions).
	CacheClassSession CacheClass = "session"
	// CacheClassVolatile sections change frequently (date, git status,
	// notebook context).
	CacheClassVolatile CacheClass = "volatile"
)

// PromptSection is a named, measured component of a built prompt.
type PromptSection struct {
	Name       string
	Content    string
	Required   bool
	Bytes      int
	EstTokens  int64 // populated by approxTokenCount, same heuristic used elsewhere
	CacheClass CacheClass
}

// BuiltPrompt is the rendered system prompt plus per-section telemetry.
type BuiltPrompt struct {
	Text     string
	Sections []PromptSection
}

// taggedSection maps a rendered <tag>...</tag> region to its section
// name and cache class.
type taggedSection struct {
	name       string
	required   bool
	cacheClass CacheClass
}

// sectionTags lists the XML tags emitted by the prompt templates that
// are measured as standalone sections. Everything outside these tags is
// reported as core_policy.
var sectionTags = map[string]taggedSection{
	"env":              {"env", true, CacheClassVolatile},
	"project_context":  {"project_context", true, CacheClassStable},
	"user_preferences": {"user_context", true, CacheClassStable},
	"available_skills": {"skills", true, CacheClassStable},
}

// extractSections scans the rendered prompt for the known tagged
// regions and returns one PromptSection per region plus a core_policy
// section covering the remainder. core_policy is listed first and the
// rest follow in render order.
func extractSections(text string) []PromptSection {
	type found struct {
		section PromptSection
		start   int
	}
	var foundSections []found
	coreBytes := len(text)
	for tag, meta := range sectionTags {
		content, start, ok := extractTag(text, tag)
		if !ok {
			continue
		}
		coreBytes -= len(content)
		foundSections = append(foundSections, found{
			start: start,
			section: PromptSection{
				Name:       meta.name,
				Content:    content,
				Required:   meta.required,
				Bytes:      len(content),
				EstTokens:  approxTokenCount(content),
				CacheClass: meta.cacheClass,
			},
		})
	}
	sort.Slice(foundSections, func(i, j int) bool {
		return foundSections[i].start < foundSections[j].start
	})
	sections := []PromptSection{{
		Name:       "core_policy",
		Required:   true,
		Bytes:      coreBytes,
		EstTokens:  int64((coreBytes + 3) / 4),
		CacheClass: CacheClassStable,
	}}
	for _, f := range foundSections {
		sections = append(sections, f.section)
	}
	return sections
}

// extractTag returns the "<tag>...</tag>" region of text including the
// tags themselves, along with its start offset. The bool reports whether
// both markers were found in order.
//
// The open tag must sit at a line boundary (start of text or after a
// newline): rendered sections always start on their own line, while
// prose mentions like `<available_skills>` inside backticks appear
// mid-line and must not be mistaken for the real block.
//
// Known limitation: a context file that itself contains a literal
// line-anchored <tag> or </tag> will confuse extraction — a closing tag
// inside <project_context> truncates that section's measurement. This
// is acceptable for telemetry-only attribution.
func extractTag(text, tag string) (string, int, bool) {
	open := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	end := strings.Index(text, closeTag)
	if end < 0 {
		return "", 0, false
	}
	// Use the last line-anchored open tag before the close tag.
	start := -1
	searchFrom := 0
	for searchFrom < end {
		idx := strings.Index(text[searchFrom:end], "\n"+open)
		if idx < 0 {
			break
		}
		start = searchFrom + idx + 1 // Skip the newline.
		searchFrom = start + len(open)
	}
	if start < 0 && strings.HasPrefix(text, open) {
		start = 0
	}
	if start < 0 {
		return "", 0, false
	}
	return text[start : end+len(closeTag)], start, true
}

// approxTokenCount estimates a token count using the same ~4 bytes per
// token heuristic used by the agent's usage fallback.
func approxTokenCount(s string) int64 {
	if s == "" {
		return 0
	}
	return int64((len(s) + 3) / 4)
}

// truncateUTF8Prefix normalizes invalid UTF-8 and trims so that the
// result is valid UTF-8 no longer than maxBytes.
func truncateUTF8Prefix(s string, maxBytes int) string {
	s = strings.ToValidUTF8(s, "")
	if maxBytes >= len(s) {
		return s
	}
	for end := maxBytes; end > 0; end-- {
		if end == len(s) || utf8.RuneStart(s[end]) {
			return s[:end]
		}
	}
	return ""
}

// maxContextFileReadSize is the hard process-protection limit for a
// single context file. Files larger than this are truncated; the limit
// is measured in bytes and guards against unbounded context files
// exhausting memory or producing oversized requests.
const maxContextFileReadSize = 256 * 1024

// readBounded reads up to maxContextFileReadSize bytes from path. If
// the file is larger than the hard limit, it returns the truncated
// prefix and truncated=true. The result is always valid UTF-8.
func readBounded(path string) (content string, truncated bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()

	buf := make([]byte, maxContextFileReadSize+1)
	n, err := io.ReadFull(f, buf)
	switch {
	case err == io.EOF || err == io.ErrUnexpectedEOF:
		return truncateUTF8Prefix(string(buf[:n]), n), false, nil
	case err != nil:
		return "", false, err
	default:
		// Read succeeded for max+1 bytes — file is larger than the hard limit.
		return truncateUTF8Prefix(string(buf[:maxContextFileReadSize]), maxContextFileReadSize), true, nil
	}
}
