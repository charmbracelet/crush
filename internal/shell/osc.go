package shell

import (
	"encoding/base64"
	"fmt"
	"image/color"
	"log/slog"
	"strconv"
	"strings"

	"github.com/charmbracelet/crush/internal/clipboard"
	"github.com/charmbracelet/x/ansi"
)

// OSC commands the session answers itself. vt handles the ones that are
// purely internal (title, working directory, default colors); these need
// access to Crush's clipboard and theme palette, which vt does not have.
const (
	oscSetColorPalette = 4
	oscClipboard       = 52
	oscResetPalette    = 104
)

// registerOscHandlers teaches the emulator the OSC sequences that reach
// outside the terminal: the color palette (4/104) and the system clipboard
// (52). vt leaves these unhandled, which would make programs that probe the
// palette guess wrong, and copy operations inside the embedded terminal
// disappear silently.
func registerOscHandlers(session *InteractiveSession) {
	emu := session.Emulator()
	emu.RegisterOscHandler(oscSetColorPalette, session.handlePalette)
	emu.RegisterOscHandler(oscResetPalette, session.handlePaletteReset)
	emu.RegisterOscHandler(oscClipboard, session.handleClipboard)
}

// handlePalette implements OSC 4: "4;N;spec;N2;spec2..." sets palette
// entries, and a spec of "?" asks the terminal to report them. Programs use
// the replies to adapt their colors to the theme. The payload carries the
// command number first, so the index/spec pairs start at parts[1].
func (s *InteractiveSession) handlePalette(data []byte) bool {
	parts := strings.Split(string(data), ";")
	if len(parts) < 3 {
		return true
	}

	var reply strings.Builder
	for i := 1; i+1 < len(parts); i += 2 {
		index, err := strconv.Atoi(parts[i])
		if err != nil || index < 0 || index > 255 {
			continue
		}

		spec := parts[i+1]
		if spec == "?" {
			// Report what this index currently resolves to, so the reply
			// matches what the panel is painting.
			fmt.Fprintf(&reply, ";%d;%s", index, oscColorSpec(s.effectiveColor(index)))
			continue
		}
		if c := ansi.XParseColor(spec); c != nil {
			s.setOverride(index, c)
		}
	}

	if reply.Len() > 0 {
		s.replyToChild("\x1b]4" + reply.String() + "\x1b\\")
	}
	return true
}

// handlePaletteReset implements OSC 104: "104" restores every palette entry
// a program changed, "104;N;N2" restores just those.
func (s *InteractiveSession) handlePaletteReset(data []byte) bool {
	parts := strings.Split(string(data), ";")
	if len(parts) < 2 {
		s.clearOverrides()
		return true
	}

	for _, part := range parts[1:] {
		index, err := strconv.Atoi(part)
		if err != nil || index < 0 || index > 255 {
			continue
		}
		s.clearOverride(index)
	}
	return true
}

// handleClipboard implements OSC 52 writes: programs that yank inside the
// embedded terminal (vim, tmux, ssh) send the selection here. Queries ("?")
// are refused: letting a child read the user's clipboard is a privacy hole
// with no upside for a terminal the agent drives.
func (s *InteractiveSession) handleClipboard(data []byte) bool {
	parts := strings.SplitN(string(data), ";", 3)
	if len(parts) < 3 {
		return true
	}

	payload := parts[2]
	if payload == "?" || strings.HasPrefix(payload, "?") {
		return true
	}

	text, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		text, err = base64.RawStdEncoding.DecodeString(payload)
	}
	if err != nil {
		slog.Debug("Ignored invalid OSC 52 clipboard payload", "error", err)
		return true
	}

	// The write forks a helper (pbcopy, wl-copy, ...), so keep it off the
	// PTY reader.
	selection := string(text)
	go func() {
		if err := clipboard.WriteText(selection); err != nil {
			slog.Debug("Failed to write OSC 52 clipboard selection", "error", err)
		}
	}()
	return true
}

// replyToChild writes a response the child asked for back into its input.
// It is best effort: a child that is not reading its input should not stall
// the PTY reader.
func (s *InteractiveSession) replyToChild(reply string) {
	s.tryEnqueue(func() {
		if _, err := s.pty.Write([]byte(reply)); err != nil {
			slog.Debug("Failed to reply to interactive session", "error", err)
		}
	})
}

// oscColorSpec formats a color the way terminals answer palette queries:
// 16 bits per channel in an X11 rgb string.
func oscColorSpec(c color.Color) string {
	if c == nil {
		return "rgb:0000/0000/0000"
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("rgb:%04x/%04x/%04x", r, g, b)
}
