package dialog

import (
	"cmp"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// renderDialogHelp renders km's short help as one padded line that fits contentWidth, the dialog's inner width.
func renderDialogHelp(t *styles.Styles, h *help.Model, km help.KeyMap, contentWidth int) string {
	textWidth := max(0, contentWidth-t.Dialog.HelpView.GetHorizontalFrameSize())
	return t.Dialog.HelpView.Render(shortHelpLine(h, km.ShortHelp(), textWidth))
}

// shortHelpLine packs enabled bindings into width, ending in an ellipsis rather than overflowing or on a separator.
func shortHelpLine(h *help.Model, bindings []key.Binding, width int) string {
	if width <= 0 {
		return ""
	}
	sep := h.Styles.ShortSeparator.Inline(true).Render(h.ShortSeparator)
	ellipsis := h.Styles.Ellipsis.Inline(true).Render(cmp.Or(h.Ellipsis, "…"))

	var segs []string
	total := 0
	for _, kb := range bindings {
		if !kb.Enabled() {
			continue
		}
		seg := ""
		if len(segs) > 0 {
			seg = sep
		}
		seg += h.Styles.ShortKey.Inline(true).Render(kb.Help().Key) + " " +
			h.Styles.ShortDesc.Inline(true).Render(kb.Help().Desc)
		w := lipgloss.Width(seg)
		if total+w > width {
			// End with an ellipsis, dropping earlier hints (with their separators) until it fits.
			for {
				tail := ellipsis
				if len(segs) > 0 {
					tail = " " + ellipsis
				}
				if total+lipgloss.Width(tail) <= width {
					segs = append(segs, tail)
					break
				}
				if len(segs) == 0 {
					break
				}
				total -= lipgloss.Width(segs[len(segs)-1])
				segs = segs[:len(segs)-1]
			}
			break
		}
		total += w
		segs = append(segs, seg)
	}
	return strings.Join(segs, "")
}
