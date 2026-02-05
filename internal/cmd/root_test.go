package cmd

import (
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	xstrings "github.com/charmbracelet/x/exp/strings"
	"github.com/stretchr/testify/require"
)

type mockEnviron []string

func (m mockEnviron) Getenv(key string) string {
	v, _ := m.LookupEnv(key)
	return v
}

func (m mockEnviron) LookupEnv(key string) (string, bool) {
	for _, env := range m {
		kv := strings.SplitN(env, "=", 2)
		if len(kv) == 2 && kv[0] == key {
			return kv[1], true
		}
	}
	return "", false
}

func (m mockEnviron) ExpandEnv(s string) string {
	return s // Not implemented for tests
}

func (m mockEnviron) Slice() []string {
	return []string(m)
}

func TestShouldQueryImageCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  mockEnviron
		want bool
	}{
		{
			name: "kitty terminal",
			env:  mockEnviron{"TERM=xterm-kitty"},
			want: true,
		},
		{
			name: "wezterm terminal",
			env:  mockEnviron{"TERM=xterm-256color"},
			want: true,
		},
		{
			name: "wezterm with WEZTERM env",
			env:  mockEnviron{"TERM=xterm-256color", "WEZTERM_EXECUTABLE=/Applications/WezTerm.app/Contents/MacOS/wezterm-gui"},
			want: true, // Not detected via TERM, only via stringext.ContainsAny which checks TERM
		},
		{
			name: "Apple Terminal",
			env:  mockEnviron{"TERM_PROGRAM=Apple_Terminal", "TERM=xterm-256color"},
			want: false,
		},
		{
			name: "alacritty",
			env:  mockEnviron{"TERM=alacritty"},
			want: true,
		},
		{
			name: "ghostty",
			env:  mockEnviron{"TERM=xterm-ghostty"},
			want: true,
		},
		{
			name: "rio",
			env:  mockEnviron{"TERM=rio"},
			want: true,
		},
		{
			name: "wezterm (detected via TERM)",
			env:  mockEnviron{"TERM=wezterm"},
			want: true,
		},
		{
			name: "SSH session",
			env:  mockEnviron{"SSH_TTY=/dev/pts/0", "TERM=xterm-256color"},
			want: false,
		},
		{
			name: "generic terminal",
			env:  mockEnviron{"TERM=xterm-256color"},
			want: true,
		},
		{
			name: "kitty over SSH",
			env:  mockEnviron{"SSH_TTY=/dev/pts/0", "TERM=xterm-kitty"},
			want: true,
		},
		{
			name: "Apple Terminal with kitty TERM (should still be false due to TERM_PROGRAM)",
			env:  mockEnviron{"TERM_PROGRAM=Apple_Terminal", "TERM=xterm-kitty"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shouldQueryCapabilities(uv.Environ(tt.env))
			require.Equal(t, tt.want, got, "shouldQueryImageCapabilities() = %v, want %v", got, tt.want)
		})
	}
}

// This is a helper to test the underlying logic of stringext.ContainsAny
// which is used by shouldQueryImageCapabilities
func TestStringextContainsAny(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		s      string
		substr []string
		want   bool
	}{
		{
			name:   "kitty in TERM",
			s:      "xterm-kitty",
			substr: kittyTerminals,
			want:   true,
		},
		{
			name:   "wezterm in TERM",
			s:      "wezterm",
			substr: kittyTerminals,
			want:   true,
		},
		{
			name:   "alacritty in TERM",
			s:      "alacritty",
			substr: kittyTerminals,
			want:   true,
		},
		{
			name:   "generic terminal not in list",
			s:      "xterm-256color",
			substr: kittyTerminals,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := xstrings.ContainsAnyOf(tt.s, tt.substr...)
			require.Equal(t, tt.want, got)
		})
	}
}
