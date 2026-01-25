package cmd

import (
	"testing"

	"github.com/charmbracelet/crush/internal/stringext"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

func TestShouldQueryCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  uv.Environ
		want bool
	}{
		{
			name: "kitty terminal",
			env:  uv.Environ{"TERM=xterm-kitty"},
			want: true,
		},
		{
			name: "generic terminal (xterm-256color)",
			env:  uv.Environ{"TERM=xterm-256color"},
			want: true,
		},
		{
			name: "generic terminal with WEZTERM_EXECUTABLE env var not checked",
			env:  uv.Environ{"TERM=xterm-256color", "WEZTERM_EXECUTABLE=/Applications/WezTerm.app/Contents/MacOS/wezterm-gui"},
			want: true, // WEZTERM_EXECUTABLE is not checked by shouldQueryCapabilities
		},
		{
			name: "Apple Terminal",
			env:  uv.Environ{"TERM_PROGRAM=Apple_Terminal", "TERM=xterm-256color"},
			want: false,
		},
		{
			name: "alacritty",
			env:  uv.Environ{"TERM=alacritty"},
			want: true,
		},
		{
			name: "ghostty",
			env:  uv.Environ{"TERM=xterm-ghostty"},
			want: true,
		},
		{
			name: "rio",
			env:  uv.Environ{"TERM=rio"},
			want: true,
		},
		{
			name: "wezterm detected via TERM",
			env:  uv.Environ{"TERM=wezterm"},
			want: true,
		},
		{
			name: "SSH session",
			env:  uv.Environ{"SSH_TTY=/dev/pts/0", "TERM=xterm-256color"},
			want: false,
		},
		{
			name: "generic terminal",
			env:  uv.Environ{"TERM=xterm-256color"},
			want: true,
		},
		{
			name: "kitty over SSH",
			env:  uv.Environ{"SSH_TTY=/dev/pts/0", "TERM=xterm-kitty"},
			want: true,
		},
		{
			name: "Apple Terminal with kitty TERM (should still be false due to TERM_PROGRAM)",
			env:  uv.Environ{"TERM_PROGRAM=Apple_Terminal", "TERM=xterm-kitty"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shouldQueryCapabilities(tt.env)
			require.Equal(t, tt.want, got, "shouldQueryCapabilities() = %v, want %v", got, tt.want)
		})
	}
}

// This is a helper to test the underlying logic of stringext.ContainsAny
// which is used by shouldQueryCapabilities
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
			got := stringext.ContainsAny(tt.s, tt.substr...)
			require.Equal(t, tt.want, got)
		})
	}
}
