package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatWorkingDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		format string
		cwd    string
		user   string
		host   string
		want   string
	}{
		{
			name:   "default format when empty",
			format: "",
			cwd:    "~/src/crush",
			user:   "joestump",
			host:   "lir",
			want:   "joestump@lir:~/src/crush",
		},
		{
			name:   "default format when blank",
			format: "   ",
			cwd:    "~/src",
			user:   "u",
			host:   "h",
			want:   "u@h:~/src",
		},
		{
			name:   "path only restores previous behavior",
			format: "{cwd}",
			cwd:    "~/src/crush",
			user:   "joestump",
			host:   "lir",
			want:   "~/src/crush",
		},
		{
			name:   "host and cwd only",
			format: "{host}:{cwd}",
			cwd:    "/tmp",
			user:   "joestump",
			host:   "nyma",
			want:   "nyma:/tmp",
		},
		{
			name:   "unknown placeholders left verbatim",
			format: "{cwd} {unknown}",
			cwd:    "/tmp",
			user:   "u",
			host:   "h",
			want:   "/tmp {unknown}",
		},
		{
			name:   "placeholder-like literals around values",
			format: "{user}@{host}:{cwd}$",
			cwd:    "~/x",
			user:   "agent",
			host:   "pidge",
			want:   "agent@pidge:~/x$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatWorkingDir(tt.format, tt.cwd, tt.user, tt.host)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestShortHostname(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"":                  "localhost",
		"lir":               "lir",
		"lir.stump.rocks":   "lir",
		"192.168.1.5":       "192.168.1.5",
		"::1":               "::1",
		".weird":            ".weird",
		"DESKTOP-8F2A1C":    "DESKTOP-8F2A1C",
		"host.example.com.": "host",
	}
	for in, want := range tests {
		require.Equal(t, want, shortHostname(in), "input %q", in)
	}
}

func TestShortUsername(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"":              "",
		"joestump":      "joestump",
		`MYCORP\jstump`: "jstump",
		`\`:             `\`,
	}
	for in, want := range tests {
		require.Equal(t, want, shortUsername(in), "input %q", in)
	}
}

func TestCurrentUserHost(t *testing.T) {
	t.Parallel()

	// The username depends on the ambient environment and may legitimately
	// be empty (e.g. containers with no passwd entry), but the hostname
	// always falls back to "localhost".
	uh := currentUserHost()
	require.NotEmpty(t, uh.host)
}
