// Package anim provides an animated spinner.
package anim

import (
	"fmt"
	"image/color"
	"math/rand/v2"
	"strings"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/lucasb-eyer/go-colorful"
)

const (
	fps           = 20
	initialChar   = '.'
	labelGap      = " "
	labelGapWidth = 1

	// Periods of ellipsis animation speed in steps.
	//
	// If the FPS is 20 (50 milliseconds) this means that the ellipsis will
	// change every 8 frames (400 milliseconds).
	ellipsisAnimSpeed = 8

	// The maximum amount of time that can pass before a character appears.
	// This is used to create a staggered entrance effect.
	maxBirthOffset = time.Second

	// Number of frames to prerender for the animation. After this number
	// of frames, the animation will loop.
	prerenderedFrames = 10
)

var (
	availableRunes = []rune("0123456789abcdefABCDEF~!@#$£€%^&*()+=_")
	ellipsisFrames = []string{".", "..", "...", ""}
)

// Internal ID management. Used during animating to ensure that frame messages
// are received only by spinner components that sent them.
var lastID int64

func nextID() int {
	return int(atomic.AddInt64(&lastID, 1))
}

// StepMsg is a message type used to trigger the next step in the animation.
type StepMsg struct{ id int }

// Anim is a Bubble for an animated spinner.
type Anim struct {
	width            int
	cyclingCharWidth int
	label            []string
	labelWidth       int
	startTime        time.Time
	birthOffsets     []time.Duration
	initialChars     []string
	initialized      bool
	cyclingFrames    [][]string // frames for the cycling characters
	step             int        // current main frame step
	ellipsisStep     int        // current ellipsis frame step
	ellipsisFrames   []string   // ellipsis animation frames
	id               int
}

// New creates a new Anim instance with the specified width and label.
func New(numChars int, label string, t *styles.Theme) (a Anim) {
	a.id = nextID()

	a.startTime = time.Now()
	a.cyclingCharWidth = numChars
	a.labelWidth = lipgloss.Width(label)

	// Total width of anim, in cells.
	a.width = numChars
	if label != "" {
		a.width += labelGapWidth + lipgloss.Width(label)
	}

	// Pre-render the label.
	// XXX: We should really get the graphemes for the label, not the runes.
	labelRunes := []rune(label)
	a.label = make([]string, len(labelRunes))
	for i := range a.label {
		a.label[i] = lipgloss.NewStyle().
			Foreground(t.FgBase).
			Render(string(labelRunes[i]))
	}

	// Pre-generate gradient.
	ramp := makeGradientRamp(a.width, t.Primary, t.Secondary)

	// Pre-render initial characters.
	a.initialChars = make([]string, a.width)
	for i := range a.initialChars {
		a.initialChars[i] = lipgloss.NewStyle().
			Foreground(ramp[i]).
			Render(string(initialChar))
	}

	// Pre-render the ellipsis frames.
	a.ellipsisFrames = make([]string, len(ellipsisFrames))
	for i, frame := range ellipsisFrames {
		a.ellipsisFrames[i] = lipgloss.NewStyle().
			Foreground(t.FgBase).
			Render(frame)
	}

	// Prerender scrambled rune frames for the animation.
	a.cyclingFrames = make([][]string, prerenderedFrames)
	for i := range a.cyclingFrames {
		a.cyclingFrames[i] = make([]string, a.width)
		for j := range a.cyclingFrames[i] {
			// NB: we also prerender the color with Lip Gloss here to avoid
			// processing in the render loop.
			r := availableRunes[rand.IntN(len(availableRunes))]
			a.cyclingFrames[i][j] = lipgloss.NewStyle().
				Foreground(ramp[j]).
				Render(string(r))
		}
	}

	// Random assign a birth to each character for a stagged entrance effect.
	a.birthOffsets = make([]time.Duration, a.width)
	for i := range a.birthOffsets {
		a.birthOffsets[i] = time.Duration(rand.N(int64(maxBirthOffset))) * time.Nanosecond
	}

	return a
}

// Init starts the animation.
func (a Anim) Init() tea.Cmd {
	return a.Step()
}

// Update processes animation steps (or not).
func (a Anim) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case StepMsg:
		if msg.id != a.id {
			// Reject messages that are not for this instance.
			return a, nil
		}

		a.step++
		if a.step >= len(a.cyclingFrames) {
			a.step = 0
		}

		if a.initialized {
			// Manage the ellipsis animation.
			a.ellipsisStep++
			if a.ellipsisStep >= ellipsisAnimSpeed*len(ellipsisFrames) {
				a.ellipsisStep = 0
			}
		} else if !a.initialized && time.Since(a.startTime) >= maxBirthOffset {
			a.initialized = true
		}
		return a, a.Step()
	default:
		return a, nil
	}
}

// View renders the current state of the animation.
func (a Anim) View() tea.View {
	var b strings.Builder
	for i := range a.width {
		switch {
		case !a.initialized && time.Since(a.startTime) < a.birthOffsets[i]:
			// Birth offset not reached: render initial character.
			b.WriteString(a.initialChars[i])
		case i < a.cyclingCharWidth:
			// Render a cycling character.
			b.WriteString(a.cyclingFrames[a.step][i])
		case i == a.cyclingCharWidth:
			// Render label gap.
			b.WriteString(labelGap)
		case i > a.cyclingCharWidth:
			// Label.
			b.WriteString(a.label[i-a.cyclingCharWidth-labelGapWidth])
		}
	}
	// Render animated ellipsis at the end of the label if all characters
	// have been initialized.
	if a.initialized {
		b.WriteString(a.ellipsisFrames[a.ellipsisStep/ellipsisAnimSpeed])
	}
	return tea.NewView(b.String())
}

// Step is a command that triggers the next step in the animation.
func (a Anim) Step() tea.Cmd {
	return tea.Tick(time.Second/time.Duration(fps), func(t time.Time) tea.Msg {
		return StepMsg{id: a.id}
	})
}

func colorToHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

func makeGradientRamp(length int, from, to color.Color) []color.Color {
	startColor := colorToHex(from)
	endColor := colorToHex(to)
	var (
		c        = make([]color.Color, length)
		start, _ = colorful.Hex(startColor)
		end, _   = colorful.Hex(endColor)
	)
	for i := range length {
		step := start.BlendLuv(end, float64(i)/float64(length))
		c[i] = lipgloss.Color(step.Hex())
	}
	return c
}
