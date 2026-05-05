package common

import (
	"image/color"
	"sync"

	"charm.land/glamour/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/xchroma"
)

const formatterName = "crush"

func init() {
	// NOTE: Glamour does not offer us an option to pass the formatter
	// implementation directly. We need to register and use by name.
	var zero color.Color
	formatters.Register(formatterName, xchroma.Formatter(zero, nil))
}

var (
	mdCacheMu    sync.Mutex
	mdCache      = map[int]*glamour.TermRenderer{}
	quietMDCache = map[int]*glamour.TermRenderer{}
)

// MarkdownRenderer returns a glamour [glamour.TermRenderer] configured with
// the given styles and width. Renderers are memoized per width and shared
// across callers; call InvalidateMarkdownRendererCache when the active
// styles change.
func MarkdownRenderer(sty *styles.Styles, width int) *glamour.TermRenderer {
	mdCacheMu.Lock()
	defer mdCacheMu.Unlock()
	if r, ok := mdCache[width]; ok {
		return r
	}
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(sty.Markdown),
		glamour.WithWordWrap(width),
		glamour.WithChromaFormatter(formatterName),
	)
	mdCache[width] = r
	return r
}

// QuietMarkdownRenderer returns a glamour [glamour.TermRenderer] with no colors
// (plain text with structure) and the given width. Renderers are memoized per
// width and shared across callers.
func QuietMarkdownRenderer(sty *styles.Styles, width int) *glamour.TermRenderer {
	mdCacheMu.Lock()
	defer mdCacheMu.Unlock()
	if r, ok := quietMDCache[width]; ok {
		return r
	}
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(sty.QuietMarkdown),
		glamour.WithWordWrap(width),
		glamour.WithChromaFormatter(formatterName),
	)
	quietMDCache[width] = r
	return r
}

// InvalidateMarkdownRendererCache drops every cached renderer. Call this
// whenever the active styles change so subsequent renderers pick up the new
// ansi.StyleConfig.
func InvalidateMarkdownRendererCache() {
	mdCacheMu.Lock()
	defer mdCacheMu.Unlock()
	mdCache = map[int]*glamour.TermRenderer{}
	quietMDCache = map[int]*glamour.TermRenderer{}
}
