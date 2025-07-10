package format

import (
	"encoding/json"
	"fmt"
	"strings"
)

// OutputFormat represents the output format type for non-interactive mode
type OutputFormat string

const (
	// Text format outputs the AI response as plain text.
	Text OutputFormat = "text"

	// JSON format outputs the AI response wrapped in a JSON object.
	JSON OutputFormat = "json"
)

// String returns the string representation of the OutputFormat
func (f OutputFormat) String() string {
	return string(f)
}

// SupportedFormats is a list of all supported output formats as strings
var SupportedFormats = []string{
	string(Text),
	string(JSON),
}

// Parse converts a string to an OutputFormat
func Parse(s string) (OutputFormat, error) {
	s = strings.ToLower(strings.TrimSpace(s))

	switch s {
	case string(Text):
		return Text, nil
	case string(JSON):
		return JSON, nil
	default:
		return "", fmt.Errorf("invalid format: %s", s)
	}
}

// IsValid checks if the provided format string is supported
func IsValid(s string) bool {
	_, err := Parse(s)
	return err == nil
}

// GetHelpText returns a formatted string describing all supported formats
func GetHelpText() string {
	return fmt.Sprintf(`Supported output formats:
- %s: Plain text output (default)
- %s: Output wrapped in a JSON object`,
		Text, JSON)
}

// FormatOutput formats the AI response according to the specified format
func FormatOutput(content string, formatStr string) (string, error) {
	format, err := Parse(formatStr)
	if err != nil {
		format = Text
	}

	switch format {
	case JSON:
		return formatAsJSON(content)
	case Text:
		fallthrough
	default:
		// ensure final new line
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		return content, nil
	}
}

// formatAsJSON wraps the content in a simple JSON object
func formatAsJSON(content string) (string, error) {
	// Use the JSON package to properly escape the content
	response := struct {
		Response string `json:"response"`
	}{
		Response: content,
	}

	jsonBytes, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal output into JSON: %w", err)
	}

	return string(jsonBytes), nil
}
