package model

import "testing"

func TestFormatWorkingDir(t *testing.T) {
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
			got := formatWorkingDir(tt.format, tt.cwd, tt.user, tt.host)
			if got != tt.want {
				t.Errorf("formatWorkingDir(%q, %q, %q, %q) = %q, want %q",
					tt.format, tt.cwd, tt.user, tt.host, got, tt.want)
			}
		})
	}
}

func TestShortHost(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		{"lir", "lir"},
		{"lir.stump.rocks", "lir"},
		{"lir.stump.rocks.", "lir"},
		{".leading", ".leading"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := shortHost(tt.host); got != tt.want {
			t.Errorf("shortHost(%q) = %q, want %q", tt.host, got, tt.want)
		}
	}
}

func TestCurrentUserHost(t *testing.T) {
	uh := currentUserHost()
	if uh.name == "" {
		t.Error("currentUserHost returned an empty username")
	}
	if uh.host == "" {
		t.Error("currentUserHost returned an empty hostname")
	}
}
