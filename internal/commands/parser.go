package commands

import (
	"strings"
	"unicode"
)

// ParseCommandInput parses slash command input into command name and arguments.
// This is the exported version of parseCommandInput for use by the editor.
//
// Input format: `\command-name arg1 arg2 "quoted arg3" arg4`
//
// Examples:
//   - `\review-pr` → name: "review-pr", args: []
//   - `\review-pr 123` → name: "review-pr", args: ["123"]
//   - `\review-pr 123 high priority` → name: "review-pr", args: ["123", "high", "priority"]
//   - `\review-pr 123 "high priority"` → name: "review-pr", args: ["123", "high priority"]
//   - `\frontend:review-pr 123` → name: "frontend:review-pr", args: ["123"]
//
// Returns the command name (without leading backslash) and a slice of arguments.
// If the input is empty or doesn't start with `\`, returns empty string and empty slice.
func ParseCommandInput(input string) (commandName string, args []string) {
	// Trim whitespace
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}

	// Check if input starts with backslash
	if !strings.HasPrefix(input, "\\") {
		return "", nil
	}

	// Remove leading backslash
	input = input[1:]

	// Split into parts, handling quoted arguments
	parts := parseArguments(input)

	if len(parts) == 0 {
		return "", []string{}
	}

	// First part is the command name
	commandName = parts[0]

	// Remaining parts are arguments
	if len(parts) > 1 {
		args = parts[1:]
	} else {
		args = []string{}
	}

	return commandName, args
}

// parseArguments parses a string into arguments, handling quoted strings.
// Supports both single and double quotes, and handles escaped quotes.
func parseArguments(input string) []string {
	var args []string
	var current strings.Builder
	var inQuotes bool
	var quoteChar rune

	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return []string{}
	}

	runes := []rune(trimmed)

	for i := 0; i < len(runes); i++ {
		char := runes[i]

		switch {
		case !inQuotes && (char == '"' || char == '\''):
			// Start of quoted string
			inQuotes = true
			quoteChar = char

		case inQuotes && char == quoteChar:
			// End of quoted string
			inQuotes = false
			quoteChar = 0
			// Save the quoted argument (even if empty)
			args = append(args, current.String())
			current.Reset()

		case inQuotes && char == '\\' && i+1 < len(runes):
			// Escaped character in quoted string
			next := runes[i+1]
			if next == quoteChar || next == '\\' {
				current.WriteRune(next)
				i++ // Skip the next character since we've processed it
			} else {
				current.WriteRune(char)
			}

		case !inQuotes && unicode.IsSpace(char):
			// Whitespace outside quotes - end of current argument
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			// Skip remaining whitespace
			for i+1 < len(runes) && unicode.IsSpace(runes[i+1]) {
				i++
			}

		default:
			// Regular character
			current.WriteRune(char)
		}
	}

	// Add the last argument if any (or if we're still in quotes)
	if current.Len() > 0 || inQuotes {
		args = append(args, current.String())
	}

	return args
}

