package security

import (
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "''"},
		{"hello", "hello"},
		{"hello world", "'hello world'"},
		{"it's a test", "'it'\\''s a test'"},
		{"path/to/file", "path/to/file"},
		{"/etc/passwd", "/etc/passwd"},
		{"value with $var", "'value with $var'"},
		{"cmd; rm -rf /", "'cmd; rm -rf /'"},
		{"backtick`cmd`", "'backtick`cmd`'"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ShellQuote(tt.input)
			if got != tt.want {
				t.Errorf("ShellQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestShellQuoteSlice(t *testing.T) {
	args := []string{"ls", "-la", "/home/user name"}
	got := ShellQuoteSlice(args)
	if !strings.Contains(got, "'") {
		t.Error("expected quoted output for arg with space")
	}
	// Ensure all parts are joined with spaces
	parts := strings.Fields(got)
	if len(parts) < 2 {
		t.Errorf("expected at least 2 parts, got %q", got)
	}
}

func TestValidateNoShellMeta(t *testing.T) {
	safe := []string{
		"hostname",
		"/var/log/syslog",
		"192.168.1.1",
		"my-container",
		"nginx.service",
	}
	for _, s := range safe {
		if msg := ValidateNoShellMeta(s); msg != "" {
			t.Errorf("ValidateNoShellMeta(%q) returned error %q for safe input", s, msg)
		}
	}

	dangerous := []struct {
		input string
		desc  string
	}{
		{"; rm -rf /", "semicolon"},
		{"foo && bar", "double ampersand"},
		{"foo || bar", "double pipe"},
		{"foo | bar", "pipe"},
		{"foo`cmd`", "backtick"},
		{"$(cmd)", "dollar paren"},
		{"foo > /dev/null", "redirect"},
		{"foo < file", "input redirect"},
		{"line1\nline2", "newline"},
	}
	for _, tt := range dangerous {
		t.Run(tt.desc, func(t *testing.T) {
			if msg := ValidateNoShellMeta(tt.input); msg == "" {
				t.Errorf("ValidateNoShellMeta(%q) expected error for dangerous input (%s)", tt.input, tt.desc)
			}
		})
	}
}
