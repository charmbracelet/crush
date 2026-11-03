package dialog

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// OnboardingStep numbers the first-run setup screens. Authentication is a
// sub-step of the provider screen, so it does not get its own step.
type OnboardingStep int

// Possible OnboardingStep values.
const (
	OnboardingStepProvider OnboardingStep = iota
	OnboardingStepModel
	OnboardingStepHelp

	onboardingStepCount = 3
)

// onboardingProgress renders the setup progress shown at the right of a
// setup screen's title, e.g. "1/3 ···". The active step is the dot drawn in
// the text color.
func onboardingProgress(t *styles.Styles, step OnboardingStep) string {
	if step < 0 || int(step) >= onboardingStepCount {
		return ""
	}

	dots := make([]string, 0, onboardingStepCount)
	for i := range onboardingStepCount {
		if i == int(step) {
			dots = append(dots, t.Dialog.Onboarding.StepDotOn.Render())
			continue
		}
		dots = append(dots, t.Dialog.Onboarding.StepDot.Render())
	}

	count := t.Dialog.Onboarding.Step.Render(fmt.Sprintf("%d/%d", int(step)+1, onboardingStepCount))
	return " " + count + "  " + strings.Join(dots, "")
}

// padRight pads s with spaces to width, so a row's selection background
// spans the full list width instead of ending at the last glyph.
func padRight(s string, width int) string {
	if gap := width - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// sizeOnboardingList sizes a setup screen's list to its content, capped by
// the rows left over after chrome: the lines used by the step label, the copy,
// the input, and the help footer. A scrollbar column is reserved only when
// the list actually overflows.
func sizeOnboardingList(t *styles.Styles, l *list.FilterableList, innerWidth, maxHeight, chrome int) (listHeight, listTotalHeight int) {
	listTotalHeight = l.TotalHeight()
	listHeight = max(1, min(listTotalHeight, max(1, maxHeight-chrome)))

	width := innerWidth
	if listTotalHeight > listHeight {
		width--
	}
	l.SetSize(max(0, width), listHeight)
	return listHeight, listTotalHeight
}
