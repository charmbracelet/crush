package common

import (
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestCapabilitiesUpdateTerminalColors(t *testing.T) {
	t.Parallel()

	var capabilities Capabilities
	foreground := color.NRGBA{R: 0xf0, G: 0xf0, B: 0xf0, A: 0xff}
	background := color.NRGBA{R: 0x10, G: 0x10, B: 0x10, A: 0xff}

	require.True(t, capabilities.Update(uv.ForegroundColorEvent{Color: foreground}))
	require.False(t, capabilities.Update(uv.ForegroundColorEvent{Color: foreground}))
	require.True(t, capabilities.Update(uv.BackgroundColorEvent{Color: background}))
	require.Equal(t, foreground, capabilities.TerminalPalette.Foreground)
	require.Equal(t, background, capabilities.TerminalPalette.Background)
	require.True(t, capabilities.TerminalPalette.IsUsable())
}

func TestCapabilitiesUpdateANSIColorResponse(t *testing.T) {
	t.Parallel()

	var capabilities Capabilities
	changed := capabilities.Update(uv.UnknownOscEvent("\x1b]4;6;rgb:00/cc/ff\a"))
	require.True(t, changed)
	require.False(t, capabilities.Update(uv.UnknownOscEvent("\x1b]4;6;rgb:00/cc/ff\a")))
	require.Equal(t, uint8(0), color.NRGBAModel.Convert(capabilities.TerminalPalette.ANSI[6]).(color.NRGBA).R)
	require.Equal(t, uint8(0xcc), color.NRGBAModel.Convert(capabilities.TerminalPalette.ANSI[6]).(color.NRGBA).G)
	require.Equal(t, uint8(0xff), color.NRGBAModel.Convert(capabilities.TerminalPalette.ANSI[6]).(color.NRGBA).B)
}

func TestCapabilitiesIgnoresUnrelatedOSC(t *testing.T) {
	t.Parallel()

	var capabilities Capabilities
	require.False(t, capabilities.Update(uv.UnknownOscEvent("\x1b]4;16;#ffffff\a")))
	require.False(t, capabilities.Update(uv.UnknownOscEvent("\x1b]99;i=0\a")))
	require.Nil(t, capabilities.TerminalPalette.ANSI[0])
}

func TestTerminalPaletteQueryUsesTmuxVersionStrategy(t *testing.T) {
	t.Parallel()

	tmuxEnv := uv.Environ{"TMUX=/tmp/tmux,1234,0", "TERM=screen-256color"}
	palette := ansiPaletteQuery()
	defaults := ansi.RequestForegroundColor + ansi.RequestBackgroundColor

	legacy := terminalPaletteQuery(tmuxEnv, "tmux 3.5")
	require.True(t, strings.HasPrefix(legacy, defaults))
	require.Contains(t, legacy, ansi.TmuxPassthrough(palette))
	require.NotContains(t, legacy, palette)

	modern := terminalPaletteQuery(tmuxEnv, "tmux 3.6")
	require.True(t, strings.HasPrefix(modern, defaults))
	require.Contains(t, modern, palette)
	require.NotContains(t, modern, "\x1bPtmux;")

	unknown := terminalPaletteQuery(tmuxEnv, "")
	require.True(t, strings.HasPrefix(unknown, defaults))
	require.Contains(t, unknown, palette)
	require.Contains(t, unknown, ansi.TmuxPassthrough(palette))
}

func TestTerminalPaletteQueryUsesTmuxVersionFromEnvironment(t *testing.T) {
	t.Parallel()

	env := uv.Environ{
		"TERM_PROGRAM=tmux",
		"TERM_PROGRAM_VERSION=3.6",
	}
	query := terminalPaletteQuery(env, "")
	require.Contains(t, query, ansiPaletteQuery())
	require.NotContains(t, query, "\x1bPtmux;")
}

func TestIsTmuxTerminalUsesEnvironmentAndVersion(t *testing.T) {
	t.Parallel()

	require.True(t, IsTmuxTerminal(uv.Environ{"TMUX=/tmp/tmux,1234,0"}, ""))
	require.True(t, IsTmuxTerminal(uv.Environ{"TERM=tmux-256color"}, ""))
	require.True(t, IsTmuxTerminal(nil, "tmux 3.6"))
	require.False(t, IsTmuxTerminal(uv.Environ{"TERM=xterm-256color"}, "xterm 381"))
}

func TestTerminalPaletteQueryUsesPlainOSCOutsideTmux(t *testing.T) {
	t.Parallel()

	query := terminalPaletteQuery(uv.Environ{"TERM=xterm-256color"}, "")
	require.Contains(t, query, ansi.RequestForegroundColor)
	require.Contains(t, query, ansi.RequestBackgroundColor)
	require.Contains(t, query, ansiPaletteQuery())
	require.NotContains(t, query, "\x1bPtmux;")
}

func TestQueryCmdRequestsTmuxVersionOverSSH(t *testing.T) {
	t.Parallel()

	cmd := QueryCmd(uv.Environ{"TMUX=/tmp/tmux,1234,0", "SSH_TTY=/dev/pts/1", "TERM=screen-256color"})
	msg := cmd().(tea.RawMsg)
	query := msg.Msg.(string)
	require.Contains(t, query, ansi.RequestNameVersion)
	require.NotContains(t, query, ansi.WindowOp(14))
}
