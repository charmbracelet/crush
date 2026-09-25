package common

import (
	"fmt"
	"math"
	"strings"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/oauth/openai"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// A subscription plan meters usage over rolling windows and reports where it
// stands. Providers differ in how many windows they run, what those windows
// are called, and what mark identifies them, so a readout carries all three
// as data and the header and sidebar render it without knowing whose plan it
// is.

// PlanWindow is one rolling usage window, reduced to what gets drawn.
type PlanWindow struct {
	// Spent is the window's utilization from 0 to 1.
	Spent float64
	// Label names the window, "5h" or "weekly". Empty when the provider
	// reported a window length with no name worth giving it.
	Label string
}

// PlanReadout is a plan's usage as the UI needs it.
type PlanReadout struct {
	// Provider is whose plan this is, which decides the mark's colour.
	Provider string
	// Icon marks the readout as this provider's.
	Icon string
	// Windows are the plan's usage windows, shortest first.
	Windows []PlanWindow
	// LimitReached is set once the provider has stopped serving the plan.
	LimitReached bool
	// ResetsAt is when a blocked plan starts serving again.
	ResetsAt time.Time
}

// PlanUsageFor returns the plan usage of providerID, and false when that
// provider meters no plan or has reported nothing yet.
//
// Usage is reported only for the provider actually in use. Signing in to one
// plan and then working against another should not leave the first one's
// figures sitting in the header, where they would look current.
func PlanUsageFor(providerID string) (PlanReadout, bool) {
	switch providerID {
	case string(catwalk.InferenceProviderOpenAI):
		usage, ok := openai.LatestUsage()
		if !ok || len(usage.Windows) == 0 {
			return PlanReadout{}, false
		}
		out := PlanReadout{
			Provider:     providerID,
			Icon:         styles.ChatGPTUsageIcon,
			LimitReached: usage.LimitReached,
			ResetsAt:     usage.ResetsAt(),
			Windows:      make([]PlanWindow, 0, len(usage.Windows)),
		}
		for _, w := range usage.Windows {
			out.Windows = append(out.Windows, PlanWindow{Spent: w.Spent(), Label: w.Label()})
		}
		return out, true
	default:
		return PlanReadout{}, false
	}
}

// mark returns the style the readout's glyph is drawn in, so each provider's
// plan is recognisable before the figures are read.
func (r PlanReadout) mark(p styles.PlanUsage) lipgloss.Style {
	if r.Provider == string(catwalk.InferenceProviderOpenAI) {
		return p.ChatGPTIcon
	}
	return p.Icon
}

// Compact renders the readout for the header, which has no room to name the
// windows: "✻ 100/64%". Only the last figure carries the sign, so the pair
// reads as one figure rather than two unrelated ones.
func (r PlanReadout) Compact(p styles.PlanUsage) string {
	if len(r.Windows) == 0 {
		return ""
	}
	parts := make([]string, 0, len(r.Windows))
	for i, w := range r.Windows {
		unit := ""
		if i == len(r.Windows)-1 {
			unit = "%"
		}
		parts = append(parts, PlanUsageWindow(p, w.Spent, unit))
	}
	return r.mark(p).Render(r.Icon) + " " + strings.Join(parts, p.Label.Render("/"))
}

// Detail renders the readout for the sidebar, which has room to name each
// window and to say when a spent plan starts serving again.
func (r PlanReadout) Detail(p styles.PlanUsage) string {
	if len(r.Windows) == 0 {
		return ""
	}
	parts := make([]string, 0, len(r.Windows)+1)
	for _, w := range r.Windows {
		part := PlanUsageWindow(p, w.Spent, "%")
		if w.Label != "" {
			part += p.Label.Render(" " + w.Label)
		}
		parts = append(parts, part)
	}

	// A window at 100% still serves requests while credits or the other
	// window have room, so only the provider refusing is worth saying. When
	// it does, the refill time is the actionable part.
	if r.LimitReached {
		notice := "limit reached"
		if !r.ResetsAt.IsZero() {
			notice = "blocked until " + r.ResetsAt.Format("3:04pm")
		}
		parts = append(parts, p.Critical.Render(notice))
	}
	return r.mark(p).Render(r.Icon) + " " + strings.Join(parts, " ")
}

// PlanUsageWindow renders one window as a percentage, coloured by how spent
// it is.
func PlanUsageWindow(p styles.PlanUsage, spent float64, unit string) string {
	spent = min(max(spent, 0), 1)
	return p.Window(spent).Render(fmt.Sprintf("%d%s", int(math.Round(spent*100)), unit))
}
