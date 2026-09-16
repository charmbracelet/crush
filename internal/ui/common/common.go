package common

import (
	"errors"
	"fmt"
	"image"
	"os"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/clipboard"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
)

// MaxAttachmentSize defines the maximum allowed size for file attachments (5 MB).
const MaxAttachmentSize = int64(5 * 1024 * 1024)

// PlanStartMarker is the sentinel the plan agent emits on its own line at the
// very start of its final response, so the UI can tell the plan document
// apart from the agent's intermediate exploratory replies while it streams.
const PlanStartMarker = "<!-- CRUSH_PLAN_START -->"

// PlanReadyMarker is the sentinel the plan agent emits on its own line at the
// end of its final response to signal that the plan is ready for execution.
const PlanReadyMarker = "<!-- CRUSH_PLAN_READY -->"

// PlanStartMarkerPresent reports whether text contains the plan-start
// sentinel on a line by itself. See [PlanReadyMarkerPresent] for why the
// check is line-scoped.
func PlanStartMarkerPresent(text string) bool {
	return planMarkerPresent(text, PlanStartMarker)
}

// PlanReadyMarkerPresent reports whether text contains the plan-ready sentinel
// on a line by itself. An own-line check (rather than a substring match) avoids
// a false positive when the agent merely mentions the marker inside explanatory
// prose, while still tolerating trailing whitespace or notes after it. Inline
// code backticks around the marker count as a marker line too: some models wrap
// the marker despite the prompt asking for plain text.
func PlanReadyMarkerPresent(text string) bool {
	return planMarkerPresent(text, PlanReadyMarker)
}

func planMarkerPresent(text, marker string) bool {
	for line := range strings.SplitSeq(text, "\n") {
		if planMarkerLine(line, marker) {
			return true
		}
	}
	return false
}

// planMarkerLine reports whether the line consists solely of the given plan
// sentinel, optionally wrapped in inline-code backticks and whitespace.
func planMarkerLine(line, marker string) bool {
	trimmed := strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "`"))
	return trimmed == marker
}

// StripPlanReadyMarker removes lines that consist solely of the plan-ready
// sentinel (matching the detection rule of [PlanReadyMarkerPresent]) so the
// marker never shows up in rendered chat output. Mentions of the marker
// inside prose are left untouched. When the marker was wrapped in its own
// code fence — ``` markers on the lines directly before and after — the
// fence pair is removed with it so an empty code block is not left behind.
func StripPlanReadyMarker(text string) string {
	return stripPlanMarker(text, PlanReadyMarker)
}

// StripPlanMarkers removes both the plan-start and plan-ready sentinel lines.
// Use it wherever rendered text could contain either marker, such as the plan
// card (which strips the trailing ready marker and the leading start marker)
// or an interrupted plan whose start marker never got its ready companion.
func StripPlanMarkers(text string) string {
	return stripPlanMarker(stripPlanMarker(text, PlanStartMarker), PlanReadyMarker)
}

func stripPlanMarker(text, marker string) string {
	lines := strings.Split(text, "\n")
	kept := lines[:0]
	inFence := false
	for i := 0; i < len(lines); i++ {
		if planMarkerLine(lines[i], marker) {
			// When the marker sits in a block fenced on the lines directly
			// before and after, drop the whole block so an empty code
			// block is not left behind. inFence guards against eating the
			// closing fence of a real code block above the marker.
			if inFence && len(kept) > 0 && isCodeFenceLine(kept[len(kept)-1]) &&
				i+1 < len(lines) && isCodeFenceLine(lines[i+1]) {
				kept = kept[:len(kept)-1]
				i++
				inFence = false
			}
			continue
		}
		if isCodeFenceLine(lines[i]) {
			inFence = !inFence
		}
		kept = append(kept, lines[i])
	}
	return strings.Join(kept, "\n")
}

// isCodeFenceLine reports whether the line opens or closes a fenced code
// block, e.g. ``` or ```go.
func isCodeFenceLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "```") {
		return false
	}
	info := strings.TrimLeft(trimmed[3:], "`")
	return !strings.ContainsAny(info, "` ")
}

// AllowedImageTypes defines the permitted image file types.
var AllowedImageTypes = []string{".jpg", ".jpeg", ".png"}

// IsImagePath reports whether the given path has one of the allowed image
// file extensions.
func IsImagePath(path string) bool {
	lowerPath := strings.ToLower(path)
	return slices.ContainsFunc(AllowedImageTypes, func(ext string) bool {
		return strings.HasSuffix(lowerPath, ext)
	})
}

// Common defines common UI options and configurations.
type Common struct {
	Workspace workspace.Workspace
	Styles    *styles.Styles
}

// Config returns the pure-data configuration associated with this [Common] instance.
func (c *Common) Config() *config.Config {
	return c.Workspace.Config()
}

// DefaultCommon returns the default common UI configurations. When the
// workspace has a large model selected, the theme is chosen based on its
// provider; otherwise the default theme is used.
func DefaultCommon(ws workspace.Workspace) *Common {
	s := styles.ThemeForProvider(largeModelProviderID(ws))
	return &Common{
		Workspace: ws,
		Styles:    &s,
	}
}

// largeModelProviderID returns the provider ID of the currently selected
// large model, or the empty string if none is set or the workspace is nil.
func largeModelProviderID(ws workspace.Workspace) string {
	if ws == nil {
		return ""
	}
	cfg := ws.Config()
	if cfg == nil {
		return ""
	}
	return cfg.Models[config.SelectedModelTypeLarge].Provider
}

// IsHyper reports whether the currently selected large model is provided
// by Hyper.
func (c *Common) IsHyper() bool {
	return largeModelProviderID(c.Workspace) == "hyper"
}

// CenterRect returns a new [Rectangle] centered within the given area with the
// specified width and height.
func CenterRect(area uv.Rectangle, width, height int) uv.Rectangle {
	centerX := area.Min.X + area.Dx()/2
	centerY := area.Min.Y + area.Dy()/2
	minX := centerX - width/2
	minY := centerY - height/2
	maxX := minX + width
	maxY := minY + height
	return image.Rect(minX, minY, maxX, maxY)
}

// BottomLeftRect returns a new [Rectangle] positioned at the bottom-left within the given area with the
// specified width and height.
func BottomLeftRect(area uv.Rectangle, width, height int) uv.Rectangle {
	minX := area.Min.X
	maxX := minX + width
	maxY := area.Max.Y
	minY := maxY - height
	return image.Rect(minX, minY, maxX, maxY)
}

// IsFileTooBig checks if the file at the given path exceeds the specified size
// limit.
func IsFileTooBig(filePath string, sizeLimit int64) (bool, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return false, fmt.Errorf("error getting file info: %w", err)
	}

	if fileInfo.Size() > sizeLimit {
		return true, nil
	}

	return false, nil
}

// CopyToClipboard copies the given text to the clipboard using both OSC 52
// (terminal escape sequence) and native clipboard for maximum compatibility.
// Returns a command that reports the outcome to the user, using the given
// message on success.
func CopyToClipboard(text, successMessage string) tea.Cmd {
	return CopyToClipboardWithCallback(text, successMessage, nil)
}

// CopyToClipboardWithCallback copies text to clipboard and executes a callback
// before showing the success message.
// This is useful when you need to perform additional actions like clearing UI state.
//
// The callback and the success message only run when the copy is believed to
// have worked, so callers can safely use the callback to discard the copied
// state (a selection, say) without losing it on a failed copy.
func CopyToClipboardWithCallback(text, successMessage string, callback tea.Cmd) tea.Cmd {
	return tea.Sequence(
		tea.SetClipboard(text),
		func() tea.Msg {
			// OSC 52 above is fire and forget: the terminal never answers, so a
			// platform without a native clipboard (an SSH session, say) gets the
			// benefit of the doubt. Only a native clipboard that accepted the
			// write and then does not hold the text is a real failure.
			if err := clipboard.WriteText(text); errors.Is(err, clipboard.ErrWriteFailed) {
				return util.NewWarnMsg("Failed to copy to clipboard")
			}
			return tea.Sequence(callback, util.ReportInfo(successMessage))()
		},
	)
}
