package dialog

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

// ListItemStyles are the styles a picker row renders with.
type ListItemStyles struct {
	ItemBlurred     lipgloss.Style
	ItemFocused     lipgloss.Style
	InfoTextBlurred lipgloss.Style
	InfoTextFocused lipgloss.Style
}

// defaultListItemStyles returns the row styles most pickers share.
func defaultListItemStyles(t *styles.Styles) ListItemStyles {
	return ListItemStyles{
		ItemBlurred:     t.Dialog.NormalItem,
		ItemFocused:     t.Dialog.SelectedItem,
		InfoTextBlurred: t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: t.Dialog.ListItem.InfoFocused,
	}
}

// rowCache holds a list row's last rendered line and the input it was
// rendered from. Styles, focus and match are not part of the key: changing
// them resets the cache.
type rowCache struct {
	key rowCacheKey
	out string
	ok  bool
}

// rowCacheKey is the input a row was rendered from. The info text is
// part of it because some of it changes on its own, like a session's
// "1 minute ago".
type rowCacheKey struct {
	width int
	title string
	info  string
}

// pickerRow is the state every picker list item shares: its version, its
// render cache, its fuzzy match and its focus. Items embed it and render
// through [pickerRow.render].
type pickerRow struct {
	*list.Versioned
	cache   rowCache
	m       fuzzy.Match
	focused bool
}

// newPickerRow returns a pickerRow at version zero.
func newPickerRow() pickerRow {
	return pickerRow{Versioned: list.NewVersioned()}
}

// invalidate drops the cached line and bumps the version so the list
// renders the row again.
func (r *pickerRow) invalidate() {
	r.cache = rowCache{}
	r.Bump()
}

// SetFocused implements [list.Focusable].
func (r *pickerRow) SetFocused(f bool) {
	if r.focused != f {
		r.focused = f
		r.invalidate()
	}
}

// SetMatch implements [list.MatchSettable].
func (r *pickerRow) SetMatch(m fuzzy.Match) {
	if !sameFuzzyMatch(r.m, m) {
		r.m = m
		r.invalidate()
	}
}

// HeightStable implements [list.HeightStable]. Focus, match and style
// changes restyle a row without changing its line count, so filtering
// does not have to render every row just to size the scrollbar.
func (r *pickerRow) HeightStable() bool {
	return true
}

// RefreshStyles implements [StyleRefresher].
func (r *pickerRow) RefreshStyles() {
	r.invalidate()
}

// render renders the row with its focus and match, reusing the cache.
func (r *pickerRow) render(t ListItemStyles, title, info string, w int) string {
	return renderItem(t, title, info, r.focused, w, &r.cache, &r.m)
}

// renderItem renders a list row, reusing the line in cache when it was
// rendered from the same input and storing it there otherwise.
func renderItem(t ListItemStyles, title string, info string, focused bool, width int, cache *rowCache, m *fuzzy.Match) string {
	key := rowCacheKey{width: width, title: title, info: info}
	if cache.ok && cache.key == key {
		return cache.out
	}

	style := t.ItemBlurred
	if focused {
		style = t.ItemFocused
	}

	// Build content to the width left after the item style's own padding,
	// so the final rendered line is exactly `width` wide. Otherwise the
	// padding pushes the line past the list width and it wraps.
	lineWidth := max(0, width-style.GetHorizontalFrameSize())

	var infoText string
	var infoWidth int
	if len(info) > 0 {
		// Cap the info column so a long value (e.g. a provider name) can
		// truncate instead of overflowing the row or squeezing the title
		// to nothing; the title keeps at least half the width.
		if maxInfo := lineWidth / 2; lipgloss.Width(info)+2 > maxInfo {
			info = ansi.Truncate(info, max(0, maxInfo-2), "…")
		}
		infoText = fmt.Sprintf(" %s ", info)
		if focused {
			infoText = t.InfoTextFocused.Render(infoText)
		} else {
			infoText = t.InfoTextBlurred.Render(infoText)
		}

		infoWidth = lipgloss.Width(infoText)
	}

	title = ansi.Truncate(title, max(0, lineWidth-infoWidth), "…")
	titleWidth := lipgloss.Width(title)
	gap := strings.Repeat(" ", max(0, lineWidth-titleWidth-infoWidth))
	content := title
	if m != nil && len(m.MatchedIndexes) > 0 {
		var lastPos int
		parts := make([]string, 0)
		ranges := matchedRanges(m.MatchedIndexes)
		for _, rng := range ranges {
			start, stop := bytePosToVisibleCharPos(title, rng)
			if start > lastPos {
				parts = append(parts, ansi.Cut(title, lastPos, start))
			}
			// NOTE: We're using [ansi.Style] here instead of [lipglosStyle]
			// because we can control the underline start and stop more
			// precisely via [ansi.AttrUnderline] and [ansi.AttrNoUnderline]
			// which only affect the underline attribute without interfering
			// with other style attributes.
			parts = append(
				parts,
				ansi.NewStyle().Underline(true).String(),
				ansi.Cut(title, start, stop+1),
				ansi.NewStyle().Underline(false).String(),
			)
			lastPos = stop + 1
		}
		if lastPos < ansi.StringWidth(title) {
			parts = append(parts, ansi.Cut(title, lastPos, ansi.StringWidth(title)))
		}

		content = strings.Join(parts, "")
	}

	content = style.Render(content + gap + infoText)
	*cache = rowCache{key: key, out: content, ok: true}
	return content
}
