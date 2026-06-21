// Package anim provides an animated spinner.
package anim

import (
	"fmt"
	"image/color"
	"math/rand/v2"
	"strings"
	"sync/atomic"
	"time"

	"github.com/zeebo/xxh3"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/lucasb-eyer/go-colorful"

	"github.com/charmbracelet/crush/internal/csync"
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

	// The maximum number of animation steps that can pass before a
	// character appears. With fps == 20 this is ~1s of staggered
	// entrance, identical to the previous wall-clock-driven value.
	// Switching from wall-clock + rand to a step-driven birth schedule
	// keeps Render() deterministic: two Anim instances built from the
	// same Settings produce byte-identical output when no Animate ticks
	// have advanced their step counter.
	maxBirthSteps = 20

	// Number of frames to prerender for the animation. After this number
	// of frames, the animation will loop. This only applies when color
	// cycling is disabled.
	prerenderedFrames = 10

	// Default number of cycling chars.
	defaultNumCyclingChars = 10
)

// Default colors for gradient.
var (
	defaultGradColorA = color.RGBA{R: 0xff, G: 0, B: 0, A: 0xff}
	defaultGradColorB = color.RGBA{R: 0, G: 0, B: 0xff, A: 0xff}
	defaultLabelColor = color.RGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff}
)

var (
	availableRunes = []rune("0123456789abcdefABCDEF~!@#$£€%^&*()+=_")
	ellipsisFrames = []string{".", "..", "...", ""}
)

// Internal ID management. Used during animating to ensure that frame messages
// are received only by spinner components that sent them.
var lastID atomic.Int64

func nextID() int {
	return int(lastID.Add(1))
}

// Cache for expensive animation calculations
type animCache struct {
	initialFrames  [][]string
	cyclingFrames  [][]string
	width          int
	labelWidth     int
	label          []string
	ellipsisFrames []string
}

var animCacheMap = csync.NewMap[string, *animCache]()

// settingsHash creates a hash key for the settings to use for caching
func settingsHash(opts Settings) string {
	h := xxh3.New()
	fmt.Fprintf(h, "%d-%s-%v-%v-%v-%t",
		opts.Size, opts.Label, opts.LabelColor, opts.GradColorA, opts.GradColorB, opts.CycleColors)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// StepMsg is a message type used to trigger the next step in the animation.
type StepMsg struct{ ID string }

// Settings defines settings for the animation.
type Settings struct {
	ID          string
	Size        int
	Label       string
	LabelColor  color.Color
	GradColorA  color.Color
	GradColorB  color.Color
	CycleColors bool
}

// Default settings.
const ()

// Anim is a Bubble for an animated spinner.
type Anim struct {
	width            int
	cyclingCharWidth int
	label            *csync.Slice[string]
	labelWidth       int
	labelColor       color.Color
	birthSteps       []int
	initialFrames    [][]string // frames for the initial characters
	initialized      atomic.Bool
	cyclingFrames    [][]string           // frames for the cycling characters
	step             atomic.Int64         // current main frame step (wraps)
	framesSinceStart atomic.Int64         // total Animate ticks (does not wrap)
	ellipsisStep     atomic.Int64         // current ellipsis frame step
	ellipsisFrames   *csync.Slice[string] // ellipsis animation frames
	id               string
}

// New creates a new Anim instance with the specified width and label.
func New(opts Settings) *Anim {
	a := &Anim{}
	// Validate settings.
	if opts.Size < 1 {
		opts.Size = defaultNumCyclingChars
	}
	if colorIsUnset(opts.GradColorA) {
		opts.GradColorA = defaultGradColorA
	}
	if colorIsUnset(opts.GradColorB) {
		opts.GradColorB = defaultGradColorB
	}
	if colorIsUnset(opts.LabelColor) {
		opts.LabelColor = defaultLabelColor
	}

	if opts.ID != "" {
		a.id = opts.ID
	} else {
		a.id = fmt.Sprintf("%d", nextID())
	}
	a.cyclingCharWidth = opts.Size
	a.labelColor = opts.LabelColor

	// Check cache first
	cacheKey := settingsHash(opts)
	cached, exists := animCacheMap.Get(cacheKey)

	if exists {
		// Use cached values
		a.width = cached.width
		a.labelWidth = cached.labelWidth
		a.label = csync.NewSliceFrom(cached.label)
		a.ellipsisFrames = csync.NewSliceFrom(cached.ellipsisFrames)
		a.initialFrames = cached.initialFrames
		a.cyclingFrames = cached.cyclingFrames
	} else {
		// Generate new values and cache them
		a.labelWidth = lipgloss.Width(opts.Label)

		// Total width of anim, in cells.
		a.width = opts.Size
		if opts.Label != "" {
			a.width += labelGapWidth + lipgloss.Width(opts.Label)
		}

		// Render the label
		a.renderLabel(opts.Label)

		// Pre-generate gradient.
		var ramp []color.Color
		numFrames := prerenderedFrames
		if opts.CycleColors {
			ramp = makeGradientRamp(a.width*3, opts.GradColorA, opts.GradColorB, opts.GradColorA, opts.GradColorB)
			numFrames = a.width * 2
		} else {
			ramp = makeGradientRamp(a.width, opts.GradColorA, opts.GradColorB)
		}

		// Pre-render initial characters.
		a.initialFrames = make([][]string, numFrames)
		offset := 0
		for i := range a.initialFrames {
			a.initialFrames[i] = make([]string, a.width+labelGapWidth+a.labelWidth)
			for j := range a.initialFrames[i] {
				if j+offset >= len(ramp) {
					continue // skip if we run out of colors
				}

				var c color.Color
				if j <= a.cyclingCharWidth {
					c = ramp[j+offset]
				} else {
					c = opts.LabelColor
				}

				// Also prerender the initial character with Lip Gloss to avoid
				// processing in the render loop.
				a.initialFrames[i][j] = lipgloss.NewStyle().
					Foreground(c).
					Render(string(initialChar))
			}
			if opts.CycleColors {
				offset++
			}
		}

		// Prerender scrambled rune frames for the animation. Seed
		// the rune picker off the settings hash so cyclingFrames is
		// a pure function of Settings: two processes with identical
		// Settings populate the cache with the same glyphs, which
		// keeps any cross-process golden-file comparison stable.
		seed := xxh3.HashString(cacheKey)
		rng := rand.New(rand.NewPCG(seed, ^seed))
		a.cyclingFrames = make([][]string, numFrames)
		offset = 0
		for i := range a.cyclingFrames {
			a.cyclingFrames[i] = make([]string, a.width)
			for j := range a.cyclingFrames[i] {
				if j+offset >= len(ramp) {
					continue // skip if we run out of colors
				}

				// Also prerender the color with Lip Gloss here to avoid processing
				// in the render loop.
				r := availableRunes[rng.IntN(len(availableRunes))]
				a.cyclingFrames[i][j] = lipgloss.NewStyle().
					Foreground(ramp[j+offset]).
					Render(string(r))
			}
			if opts.CycleColors {
				offset++
			}
		}

		// Cache the results
		labelSlice := make([]string, a.label.Len())
		for i, v := range a.label.Seq2() {
			labelSlice[i] = v
		}
		ellipsisSlice := make([]string, a.ellipsisFrames.Len())
		for i, v := range a.ellipsisFrames.Seq2() {
			ellipsisSlice[i] = v
		}
		cached = &animCache{
			initialFrames:  a.initialFrames,
			cyclingFrames:  a.cyclingFrames,
			width:          a.width,
			labelWidth:     a.labelWidth,
			label:          labelSlice,
			ellipsisFrames: ellipsisSlice,
		}
		animCacheMap.Set(cacheKey, cached)
	}

	// Assign a deterministic birth step to each column for a
	// staggered entrance effect. The schedule is seeded off the
	// spinner id and the settings hash, so two spinners with the
	// same role and identity stagger identically (this is what
	// keeps Render() byte-equal across cache hits and across
	// processes for the same Settings+ID) while spinners with
	// different ids — distinct assistant messages, different tool
	// calls, "Thinking" vs "Generating" labels — fade in with
	// different patterns instead of marching in lock-step.
	birthSeed := xxh3.HashString(a.id + "|" + cacheKey)
	birthRng := rand.New(rand.NewPCG(birthSeed, ^birthSeed))
	a.birthSteps = make([]int, a.width)
	for i := range a.birthSteps {
		a.birthSteps[i] = birthRng.IntN(maxBirthSteps)
	}

	return a
}

// SetLabel updates the label text and re-renders it.
func (a *Anim) SetLabel(newLabel string) {
	a.labelWidth = lipgloss.Width(newLabel)

	// Update total width
	a.width = a.cyclingCharWidth
	if newLabel != "" {
		a.width += labelGapWidth + a.labelWidth
	}

	// Re-render the label
	a.renderLabel(newLabel)
}

// renderLabel renders the label with the current label color.
func (a *Anim) renderLabel(label string) {
	if a.labelWidth > 0 {
		// Pre-render the label.
		labelRunes := []rune(label)
		a.label = csync.NewSlice[string]()
		for i := range labelRunes {
			rendered := lipgloss.NewStyle().
				Foreground(a.labelColor).
				Render(string(labelRunes[i]))
			a.label.Append(rendered)
		}

		// Pre-render the ellipsis frames which come after the label.
		a.ellipsisFrames = csync.NewSlice[string]()
		for _, frame := range ellipsisFrames {
			rendered := lipgloss.NewStyle().
				Foreground(a.labelColor).
				Render(frame)
			a.ellipsisFrames.Append(rendered)
		}
	} else {
		a.label = csync.NewSlice[string]()
		a.ellipsisFrames = csync.NewSlice[string]()
	}
}

// Width returns the total width of the animation.
func (a *Anim) Width() (w int) {
	w = a.width
	if a.labelWidth > 0 {
		w += labelGapWidth + a.labelWidth

		var widestEllipsisFrame int
		for _, f := range ellipsisFrames {
			fw := lipgloss.Width(f)
			if fw > widestEllipsisFrame {
				widestEllipsisFrame = fw
			}
		}
		w += widestEllipsisFrame
	}
	return w
}

// Start starts the animation.
func (a *Anim) Start() tea.Cmd {
	return a.Step()
}

// Animate advances the animation to the next step.
func (a *Anim) Animate(msg StepMsg) tea.Cmd {
	if msg.ID != a.id {
		return nil
	}

	step := a.step.Add(1)
	if int(step) >= len(a.cyclingFrames) {
		a.step.Store(0)
	}

	frames := a.framesSinceStart.Add(1)
	if a.initialized.Load() && a.labelWidth > 0 {
		// Manage the ellipsis animation.
		ellipsisStep := a.ellipsisStep.Add(1)
		if int(ellipsisStep) >= ellipsisAnimSpeed*len(ellipsisFrames) {
			a.ellipsisStep.Store(0)
		}
	} else if !a.initialized.Load() && int(frames) >= maxBirthSteps {
		a.initialized.Store(true)
	}
	return a.Step()
}

// Render renders the current state of the animation.
func (a *Anim) Render() string {
	var b strings.Builder
	step := int(a.step.Load())
	frames := int(a.framesSinceStart.Load())
	for i := range a.width {
		switch {
		case !a.initialized.Load() && i < len(a.birthSteps) && frames < a.birthSteps[i]:
			// Birth step not reached: render initial character.
			b.WriteString(a.initialFrames[step][i])
		case i < a.cyclingCharWidth:
			// Render a cycling character.
			b.WriteString(a.cyclingFrames[step][i])
		case i == a.cyclingCharWidth:
			// Render label gap.
			b.WriteString(labelGap)
		case i > a.cyclingCharWidth:
			// Label.
			if labelChar, ok := a.label.Get(i - a.cyclingCharWidth - labelGapWidth); ok {
				b.WriteString(labelChar)
			}
		}
	}
	// Render animated ellipsis at the end of the label if all characters
	// have been initialized.
	if a.initialized.Load() && a.labelWidth > 0 {
		ellipsisStep := int(a.ellipsisStep.Load())
		if ellipsisFrame, ok := a.ellipsisFrames.Get(ellipsisStep / ellipsisAnimSpeed); ok {
			b.WriteString(ellipsisFrame)
		}
	}

	return b.String()
}

// Step is a command that triggers the next step in the animation.
func (a *Anim) Step() tea.Cmd {
	return tea.Tick(time.Second/time.Duration(fps), func(t time.Time) tea.Msg {
		return StepMsg{ID: a.id}
	})
}

// makeGradientRamp() returns a slice of colors blended between the given keys.
// Blending is done as Hcl to stay in gamut.
func makeGradientRamp(size int, stops ...color.Color) []color.Color {
	if len(stops) < 2 {
		return nil
	}

	points := make([]colorful.Color, len(stops))
	for i, k := range stops {
		points[i], _ = colorful.MakeColor(k)
	}

	numSegments := len(stops) - 1
	if numSegments == 0 {
		return nil
	}
	blended := make([]color.Color, 0, size)

	// Calculate how many colors each segment should have.
	segmentSizes := make([]int, numSegments)
	baseSize := size / numSegments
	remainder := size % numSegments

	// Distribute the remainder across segments.
	for i := range numSegments {
		segmentSizes[i] = baseSize
		if i < remainder {
			segmentSizes[i]++
		}
	}

	// Generate colors for each segment.
	for i := range numSegments {
		c1 := points[i]
		c2 := points[i+1]
		segmentSize := segmentSizes[i]

		for j := range segmentSize {
			if segmentSize == 0 {
				continue
			}
			t := float64(j) / float64(segmentSize)
			c := c1.BlendHcl(c2, t)
			blended = append(blended, c)
		}
	}

	return blended
}

func colorIsUnset(c color.Color) bool {
	if c == nil {
		return true
	}
	_, _, _, a := c.RGBA()
	return a == 0
}
