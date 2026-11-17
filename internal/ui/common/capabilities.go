package common

import (
	"image/color"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	xstrings "github.com/charmbracelet/x/exp/strings"

	"github.com/charmbracelet/crush/internal/ui/notification"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// Capabilities define different terminal capabilities supported.
type Capabilities struct {
	// Profile is the terminal color profile used to determine how colors are
	// rendered.
	Profile colorprofile.Profile
	// Columns is the number of character columns in the terminal.
	Columns int
	// Rows is the number of character rows in the terminal.
	Rows int
	// PixelX is the width of the terminal in pixels.
	PixelX int
	// PixelY is the height of the terminal in pixels.
	PixelY int
	// KittyGraphics indicates whether the terminal supports the Kitty graphics
	// protocol.
	KittyGraphics bool
	// SixelGraphics indicates whether the terminal supports Sixel graphics.
	SixelGraphics bool
	// Env is the terminal environment variables.
	Env uv.Environ
	// TerminalVersion is the terminal version string.
	TerminalVersion string
	// ReportFocusEvents indicates whether the terminal supports focus events.
	ReportFocusEvents bool
	// OSC99Notifications indicates whether the terminal supports OSC 99 notifications.
	OSC99Notifications bool
	// TerminalPalette holds colors reported by OSC 10, 11, and 4 queries.
	TerminalPalette styles.TerminalPalette
}

// Update updates the capabilities and reports whether the terminal palette
// changed.
func (c *Capabilities) Update(msg any) bool {
	switch m := msg.(type) {
	case tea.EnvMsg:
		c.Env = uv.Environ(m)
	case tea.ColorProfileMsg:
		c.Profile = m.Profile
	case tea.WindowSizeMsg:
		c.Columns = m.Width
		c.Rows = m.Height
	case uv.PixelSizeEvent:
		c.PixelX = m.Width
		c.PixelY = m.Height
	case uv.KittyGraphicsEvent:
		c.KittyGraphics = true
	case uv.PrimaryDeviceAttributesEvent:
		if slices.Contains(m, 4) {
			c.SixelGraphics = true
		}
	case tea.TerminalVersionMsg:
		c.TerminalVersion = m.Name
	case tea.ModeReportMsg:
		switch m.Mode {
		case ansi.ModeFocusEvent:
			c.ReportFocusEvents = modeSupported(m.Value)
		}
	case uv.UnknownOscEvent:
		if notification.DetectOSC99Support(string(m)) {
			c.OSC99Notifications = true
		}
		if index, color, ok := parseANSIColorResponse(string(m)); ok {
			if sameTerminalColor(c.TerminalPalette.ANSI[index], color) {
				return false
			}
			c.TerminalPalette.ANSI[index] = color
			return true
		}
	case uv.ForegroundColorEvent:
		if m.Color != nil && !sameTerminalColor(c.TerminalPalette.Foreground, m.Color) {
			c.TerminalPalette.Foreground = m.Color
			return true
		}
	case uv.BackgroundColorEvent:
		if m.Color != nil && !sameTerminalColor(c.TerminalPalette.Background, m.Color) {
			c.TerminalPalette.Background = m.Color
			return true
		}
	}
	return false
}

func sameTerminalColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

// QueryCmd returns a [tea.Cmd] that queries the terminal for different
// capabilities.
func QueryCmd(env uv.Environ) tea.Cmd {
	var sb strings.Builder
	sb.WriteString(ansi.RequestPrimaryDeviceAttributes)
	sb.WriteString(ansi.QueryModifyOtherKeys)
	sb.WriteString(ansi.RequestModeFocusEvent)
	sb.WriteString(notification.OSC99QuerySequence())

	// Queries that should only be sent to "smart" normal terminals.
	shouldQueryFor := shouldQueryCapabilities(env)
	if shouldQueryFor || IsTmuxTerminal(env, "") {
		sb.WriteString(ansi.RequestNameVersion)
	}
	if shouldQueryFor {
		sb.WriteString(ansi.WindowOp(14)) // Window size in pixels
		kittyReq := ansi.KittyGraphics([]byte("AAAA"), "i=31", "s=1", "v=1", "a=q", "t=d", "f=24")
		if _, isTmux := env.LookupEnv("TMUX"); isTmux {
			kittyReq = ansi.TmuxPassthrough(kittyReq)
		}
		sb.WriteString(kittyReq)
	}

	return tea.Raw(sb.String())
}

// QueryTerminalPaletteCmd requests the terminal's default foreground and
// background plus its 16 ANSI colors. tmux palette queries use passthrough
// only before tmux 3.6. Responses arrive as terminal events.
func QueryTerminalPaletteCmd(env uv.Environ, terminalVersion string) tea.Cmd {
	return tea.Raw(terminalPaletteQuery(env, terminalVersion))
}

func terminalPaletteQuery(env uv.Environ, terminalVersion string) string {
	var sb strings.Builder
	sb.WriteString(ansi.RequestForegroundColor)
	sb.WriteString(ansi.RequestBackgroundColor)
	palette := ansiPaletteQuery()
	if !IsTmuxTerminal(env, terminalVersion) {
		sb.WriteString(palette)
		return sb.String()
	}
	major, minor, known := tmuxVersion(env, terminalVersion)
	if !known {
		sb.WriteString(palette)
		sb.WriteString(ansi.TmuxPassthrough(palette))
		return sb.String()
	}
	if major < 3 || major == 3 && minor < 6 {
		sb.WriteString(ansi.TmuxPassthrough(palette))
	} else {
		sb.WriteString(palette)
	}
	return sb.String()
}

func ansiPaletteQuery() string {
	var sb strings.Builder
	for index := range 16 {
		sb.WriteString("\x1b]4;")
		sb.WriteString(strconv.Itoa(index))
		sb.WriteString(";?\x1b\\")
	}
	return sb.String()
}

// IsTmuxTerminal reports whether the environment or terminal version identifies
// tmux.
func IsTmuxTerminal(env uv.Environ, terminalVersion string) bool {
	if _, ok := env.LookupEnv("TMUX"); ok {
		return true
	}
	if strings.HasPrefix(strings.ToLower(env.Getenv("TERM")), "tmux") {
		return true
	}
	termProgram, ok := env.LookupEnv("TERM_PROGRAM")
	if ok && strings.EqualFold(termProgram, "tmux") {
		return true
	}
	return strings.Contains(strings.ToLower(terminalVersion), "tmux")
}

func tmuxVersion(env uv.Environ, terminalVersion string) (int, int, bool) {
	if termProgram, ok := env.LookupEnv("TERM_PROGRAM"); ok && strings.EqualFold(termProgram, "tmux") {
		if version, ok := env.LookupEnv("TERM_PROGRAM_VERSION"); ok {
			if major, minor, parsed := parseVersion(version); parsed {
				return major, minor, true
			}
		}
	}
	return parseVersion(terminalVersion)
}

func parseVersion(version string) (int, int, bool) {
	if tmuxIndex := strings.Index(strings.ToLower(version), "tmux"); tmuxIndex >= 0 {
		version = version[tmuxIndex+len("tmux"):]
	}
	version = strings.TrimLeft(strings.TrimSpace(version), "vV")
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, majorErr := strconv.Atoi(leadingDigits(parts[0]))
	minor, minorErr := strconv.Atoi(leadingDigits(parts[1]))
	if majorErr != nil || minorErr != nil {
		return 0, 0, false
	}
	return major, minor, true
}

func leadingDigits(value string) string {
	for index, char := range value {
		if char < '0' || char > '9' {
			return value[:index]
		}
	}
	return value
}

func parseANSIColorResponse(sequence string) (int, color.Color, bool) {
	sequence = strings.TrimPrefix(sequence, "\x1b]")
	sequence = strings.TrimPrefix(sequence, "\x9d")
	sequence = strings.TrimSuffix(sequence, "\x07")
	sequence = strings.TrimSuffix(sequence, "\x1b\\")
	sequence = strings.TrimSuffix(sequence, "\x9c")
	parts := strings.Split(sequence, ";")
	if len(parts) != 3 || parts[0] != "4" {
		return 0, nil, false
	}
	index, err := strconv.Atoi(parts[1])
	if err != nil || index < 0 || index >= 16 {
		return 0, nil, false
	}
	parsed := ansi.XParseColor(parts[2])
	if parsed == nil {
		return 0, nil, false
	}
	return index, parsed, true
}

// SupportsTrueColor returns true if the terminal supports true color.
func (c Capabilities) SupportsTrueColor() bool {
	return c.Profile == colorprofile.TrueColor
}

// SupportsKittyGraphics returns true if the terminal supports Kitty graphics.
func (c Capabilities) SupportsKittyGraphics() bool {
	return c.KittyGraphics
}

// SupportsSixelGraphics returns true if the terminal supports Sixel graphics.
func (c Capabilities) SupportsSixelGraphics() bool {
	return c.SixelGraphics
}

// CellSize returns the size of a single terminal cell in pixels.
func (c Capabilities) CellSize() (width, height int) {
	if c.Columns == 0 || c.Rows == 0 {
		return 0, 0
	}
	return c.PixelX / c.Columns, c.PixelY / c.Rows
}

func modeSupported(v ansi.ModeSetting) bool {
	return v.IsSet() || v.IsReset()
}

// kittyTerminals defines terminals supporting querying capabilities.
var kittyTerminals = []string{"alacritty", "ghostty", "kitty", "rio", "wezterm"}

func shouldQueryCapabilities(env uv.Environ) bool {
	const osVendorTypeApple = "Apple"
	termType := env.Getenv("TERM")
	termProg, okTermProg := env.LookupEnv("TERM_PROGRAM")
	_, okSSHTTY := env.LookupEnv("SSH_TTY")
	if okTermProg && strings.Contains(termProg, osVendorTypeApple) {
		return false
	}
	return (!okTermProg && !okSSHTTY) ||
		(!strings.Contains(termProg, osVendorTypeApple) && !okSSHTTY) ||
		// Terminals that do support XTVERSION.
		xstrings.ContainsAnyOf(termType, kittyTerminals...)
}
