package model

import (
	"testing"

	"charm.land/x/nerdfont"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestUIGitBranchLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		branch    string
		nerdFonts bool
		want      string
	}{
		{name: "empty outside a repository", nerdFonts: true, want: ""},
		{name: "glyph with nerd fonts", branch: "main", nerdFonts: true, want: styles.GitBranchIcon + " main"},
		{name: "bare branch without nerd fonts", branch: "main", want: "main"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ui := &UI{gitBranch: tt.branch, nerdFonts: tt.nerdFonts}
			require.Equal(t, tt.want, ui.gitBranchLabel())
		})
	}
}

func TestResolveNerdFonts(t *testing.T) {
	t.Parallel()

	on, off := true, false
	detected := func() nerdfont.Result { return nerdfont.Result{Supported: true, Reason: "detected"} }

	tests := []struct {
		name string
		tui  *config.TUIOptions
		env  string
		want nerdfont.Result
	}{
		{name: "no options detects", want: detected()},
		{name: "unset option detects", tui: &config.TUIOptions{}, want: detected()},
		{name: "option forces off", tui: &config.TUIOptions{NerdFonts: &off}, want: nerdfont.Result{Supported: false, Reason: "tui.nerd_fonts option"}},
		{name: "option forces on", tui: &config.TUIOptions{NerdFonts: &on}, want: nerdfont.Result{Supported: true, Reason: "tui.nerd_fonts option"}},
		{name: "auto env keeps the option", tui: &config.TUIOptions{NerdFonts: &off}, env: "auto", want: nerdfont.Result{Supported: false, Reason: "tui.nerd_fonts option"}},
		{name: "env beats the option", tui: &config.TUIOptions{NerdFonts: &off}, env: "1", want: detected()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, resolveNerdFonts(tt.tui, tt.env, detected))
		})
	}
}
