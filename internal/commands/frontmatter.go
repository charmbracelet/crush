package commands

import (
	"log/slog"
	"strings"

	"go.yaml.in/yaml/v4"
)

// Frontmatter represents the YAML metadata parsed from command files.
type Frontmatter struct {
	Description  string   `yaml:"description"`
	ArgumentHint string   `yaml:"argument-hint"`
	AllowedTools []string `yaml:"allowed-tools"`
}

// ParseFrontmatter extracts and parses YAML frontmatter from a command file.
// The frontmatter must be delimited by `---` markers at the start of the file.
// Returns the parsed frontmatter and the remaining content (after frontmatter removal).
// If no frontmatter is present, returns empty Frontmatter and the original content.
// Invalid YAML is logged but doesn't cause the function to fail - it returns empty frontmatter.
// This function never panics and gracefully handles all edge cases.
func ParseFrontmatter(content string) (Frontmatter, string, error) {
	// Defer recover to ensure function never panics
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic in ParseFrontmatter",
				"panic", r,
			)
		}
	}()

	// Handle empty content
	if content == "" {
		return Frontmatter{}, "", nil
	}

	content = strings.TrimPrefix(content, "\ufeff") // Remove BOM if present

	// Check if content starts with frontmatter delimiter
	if !strings.HasPrefix(content, "---") {
		// No frontmatter, return empty and original content
		return Frontmatter{}, content, nil
	}

	// Find the closing delimiter (must be on its own line with newlines)
	// Look for "\n---\n" or "\n---" at end
	closingIndex := strings.Index(content, "\n---\n")
	if closingIndex == -1 {
		// Try "\n---" at end of content
		if strings.HasSuffix(content, "\n---") {
			closingIndex = len(content) - 4
		} else {
			// No closing delimiter found, treat as no frontmatter
			return Frontmatter{}, content, nil
		}
	}

	// Extract YAML content (between delimiters)
	// Skip opening "---" and newline, go until closing delimiter
	yamlStart := strings.Index(content, "\n") + 1
	if yamlStart == 0 || yamlStart > len(content) {
		return Frontmatter{}, content, nil
	}
	if closingIndex < yamlStart {
		// Invalid structure, treat as no frontmatter
		return Frontmatter{}, content, nil
	}
	yamlEnd := closingIndex
	if yamlEnd > len(content) {
		yamlEnd = len(content)
	}
	yamlContent := strings.TrimSpace(content[yamlStart:yamlEnd])

	// If YAML content is empty, treat as no frontmatter
	if yamlContent == "" {
		return Frontmatter{}, content, nil
	}

	// Extract remaining content (after closing delimiter)
	remainingStart := closingIndex + 5 // Skip "\n---\n"
	if remainingStart > len(content) {
		remainingStart = len(content)
	}
	remainingContent := strings.TrimSpace(content[remainingStart:])

	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		// Log error but don't crash - return empty frontmatter and original content
		slog.Warn("Failed to parse frontmatter YAML",
			"error", err,
			"yaml_content", yamlContent,
		)
		// Return empty frontmatter and original content (treat as no frontmatter)
		return Frontmatter{}, content, nil
	}

	// Handle allowed-tools: if it's a comma-separated string, split it
	// The YAML parser handles []string directly, but supports comma-separated strings too
	if len(fm.AllowedTools) == 1 && strings.Contains(fm.AllowedTools[0], ",") {
		tools := strings.Split(fm.AllowedTools[0], ",")
		fm.AllowedTools = make([]string, 0, len(tools))
		for _, tool := range tools {
			if trimmed := strings.TrimSpace(tool); trimmed != "" {
				fm.AllowedTools = append(fm.AllowedTools, trimmed)
			}
		}
	}

	return fm, remainingContent, nil
}

