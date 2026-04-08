package security

import (
	"strings"
)

// ShellQuote safely quotes a string for use in a POSIX shell command.
// It wraps the string in single quotes and escapes any embedded single
// quotes using the '\'' idiom.
func ShellQuote(s string) string {
	if s == "" {
		return "''"
	}
	// If the string contains no special characters, return as-is
	if !containsShellSpecial(s) {
		return s
	}
	// Replace single quotes with '\'' (end quote, escaped quote, begin quote)
	escaped := strings.ReplaceAll(s, "'", "'\\''")
	return "'" + escaped + "'"
}

// ShellQuoteSlice quotes each element in a slice and joins them with spaces.
func ShellQuoteSlice(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = ShellQuote(arg)
	}
	return strings.Join(quoted, " ")
}

// ValidateNoShellMeta returns an error message if the string contains
// dangerous shell metacharacters that could be used for injection.
// Returns empty string if safe.
func ValidateNoShellMeta(s string) string {
	dangerous := []struct {
		char string
		desc string
	}{
		{";", "command separator"},
		{"&&", "command chain"},
		{"||", "command chain"},
		{"|", "pipe"},
		{"`", "command substitution"},
		{"$(", "command substitution"},
		{">", "output redirection"},
		{"<", "input redirection"},
		{"\n", "newline (command separator)"},
		{"\r", "carriage return"},
	}

	for _, d := range dangerous {
		if strings.Contains(s, d.char) {
			return "input contains dangerous shell metacharacter: " + d.desc + " (" + d.char + ")"
		}
	}
	return ""
}

func containsShellSpecial(s string) bool {
	for _, c := range s {
		switch c {
		case '\'', '"', '\\', '$', '`', '!', '(', ')', '{', '}',
			'[', ']', '|', '&', ';', '<', '>', '~', '#', '*',
			'?', ' ', '\t', '\n', '\r':
			return true
		}
	}
	return false
}
