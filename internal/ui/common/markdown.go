package common

import (
	"fmt"
	"image/color"
	"io"
	"maps"
	"slices"
	"strings"
	"sync"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/xchroma"
)

const formatterName = "crush"

// Code block colors.
//
// Glamour turns a style config's code-block chroma settings into a chroma
// style registered globally under one fixed name, and only the first time:
// renderers built after a theme switch kept highlighting fenced code with
// the colors of the first theme ever rendered. Registering again is no fix,
// because chroma's registry is a plain map that every highlight reads
// without a lock, so writing it while a renderer runs on another goroutine
// is a data race.
//
// So the registry is only written here, at init: it gets a fixed set of
// empty placeholder styles. Each renderer's config names one placeholder
// instead of carrying chroma settings ([codeBlockConfig]), a placeholder
// stands for one distinct set of settings, and the "crush" formatter every
// renderer uses swaps the placeholder for the real style while it formats.
// Placeholders are reused round-robin once all are taken; renderers are
// rebuilt on every theme change, so the live ones always share the current
// settings.
const codeBlockSlots = 32

var (
	// codeBlockSlotOf maps each placeholder style to its slot. It is
	// written only at init.
	codeBlockSlotOf = map[*chroma.Style]int{}

	// codeBlockMu guards the slot assignments below.
	codeBlockMu     sync.Mutex
	codeBlockByKey  = map[string]int{}
	codeBlockKeys   [codeBlockSlots]string
	codeBlockStyles [codeBlockSlots]*chroma.Style
	codeBlockNext   int
)

func init() {
	// NOTE: Glamour does not offer us an option to pass the formatter
	// implementation directly. We need to register and use by name.
	var zero color.Color
	inner := xchroma.Formatter(zero, nil)
	formatters.Register(formatterName, chroma.FormatterFunc(
		func(w io.Writer, style *chroma.Style, it chroma.Iterator) error {
			return inner.Format(w, codeBlockStyle(style), it)
		},
	))
	for i := range codeBlockSlots {
		placeholder := chromastyles.Register(chroma.MustNewStyle(codeBlockSlotName(i), chroma.StyleEntries{}))
		codeBlockSlotOf[placeholder] = i
	}
}

// codeBlockSlotName is the registry name of a code block placeholder.
func codeBlockSlotName(slot int) string {
	return fmt.Sprintf("crush-code-block-%d", slot)
}

// codeBlockConfig returns cfg ready to hand to glamour: chroma settings in
// its code block are replaced by the name of the placeholder that stands
// for them. See the comment on [codeBlockSlots].
func codeBlockConfig(cfg ansi.StyleConfig) ansi.StyleConfig {
	if cfg.CodeBlock.Chroma == nil {
		return cfg
	}
	cfg.CodeBlock.Theme = codeBlockSlotName(codeBlockSlot(cfg.CodeBlock.Chroma))
	cfg.CodeBlock.Chroma = nil
	return cfg
}

// codeBlockSlot returns the placeholder slot for the given chroma settings,
// assigning one (and building its style) the first time they are seen.
func codeBlockSlot(c *ansi.Chroma) int {
	entries := styles.ChromaEntries(c)
	var key strings.Builder
	for _, t := range slices.Sorted(maps.Keys(entries)) {
		fmt.Fprintf(&key, "%d=%s;", t, entries[t])
	}

	codeBlockMu.Lock()
	defer codeBlockMu.Unlock()
	if slot, ok := codeBlockByKey[key.String()]; ok {
		return slot
	}
	slot := codeBlockNext
	codeBlockNext = (codeBlockNext + 1) % codeBlockSlots
	if old := codeBlockKeys[slot]; old != "" {
		delete(codeBlockByKey, old)
	}
	codeBlockKeys[slot] = key.String()
	codeBlockStyles[slot] = chroma.MustNewStyle("crush", entries)
	codeBlockByKey[key.String()] = slot
	return slot
}

// codeBlockStyle returns the real style for a code block placeholder, or
// style itself when it is not one.
func codeBlockStyle(style *chroma.Style) *chroma.Style {
	slot, ok := codeBlockSlotOf[style]
	if !ok {
		return style
	}
	codeBlockMu.Lock()
	defer codeBlockMu.Unlock()
	if s := codeBlockStyles[slot]; s != nil {
		return s
	}
	return style
}

// mdCacheMu guards mdCache, quietMDCache, userMDCache, and planMDCache.
//
// Lock ordering: when both mdCacheMu and rendererLocksMu are
// needed (only in InvalidateMarkdownRendererCache), acquire
// mdCacheMu FIRST, then rendererLocksMu. No other call site may
// hold rendererLocksMu while acquiring mdCacheMu.
var (
	mdCacheMu    sync.Mutex
	mdCache      = map[int]*glamour.TermRenderer{}
	quietMDCache = map[int]*glamour.TermRenderer{}
	userMDCache  = map[int]*glamour.TermRenderer{}
	planMDCache  = map[int]*glamour.TermRenderer{}
)

// MarkdownRenderer returns a glamour [glamour.TermRenderer] configured with
// the given styles and width. Renderers are memoized per width and shared
// across callers; call InvalidateMarkdownRendererCache when the active
// styles change.
//
// The returned renderer is NOT safe for concurrent Render calls
// (goldmark's BlockStack carries state across the public Render
// API). Crush's TUI is single-threaded so production never
// contends, but parallel callers (most notably parallel tests)
// must serialize via [LockMarkdownRenderer]. Treat the renderer
// as effectively pinned to one goroutine at a time.
func MarkdownRenderer(sty *styles.Styles, width int) *glamour.TermRenderer {
	mdCacheMu.Lock()
	defer mdCacheMu.Unlock()
	if r, ok := mdCache[width]; ok {
		return r
	}
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(codeBlockConfig(sty.Markdown)),
		glamour.WithWordWrap(width),
		glamour.WithChromaFormatter(formatterName),
	)
	mdCache[width] = r
	return r
}

// UserMarkdownRenderer returns a glamour [glamour.TermRenderer] configured like
// [MarkdownRenderer] but with single line breaks preserved. Renderers are
// memoized per width and shared across callers; call
// InvalidateMarkdownRendererCache when the active styles change. Same
// concurrency contract as [MarkdownRenderer]: serialize via
// [LockMarkdownRenderer].
//
// User input is authored in a plain textarea, not written as Markdown source,
// so a lone newline is a line the user deliberately typed. Standard Markdown
// treats it as a soft break and joins the lines when rendering, which makes a
// submitted message display differently from what was typed (see
// charmbracelet/crush#3502). Preserving newlines keeps the display faithful.
//
// This is deliberately NOT applied to [MarkdownRenderer]: assistant output and
// dialog copy are genuine Markdown, where soft-wrapping a paragraph across
// source lines is normal and collapsing it is the correct rendering.
func UserMarkdownRenderer(sty *styles.Styles, width int) *glamour.TermRenderer {
	mdCacheMu.Lock()
	defer mdCacheMu.Unlock()
	if r, ok := userMDCache[width]; ok {
		return r
	}
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(codeBlockConfig(sty.Markdown)),
		glamour.WithWordWrap(width),
		glamour.WithChromaFormatter(formatterName),
		glamour.WithPreservedNewLines(),
	)
	userMDCache[width] = r
	return r
}

// QuietMarkdownRenderer returns a glamour [glamour.TermRenderer] with no colors
// (plain text with structure) and the given width. Renderers are memoized per
// width and shared across callers. Same concurrency contract as
// [MarkdownRenderer]: serialize via [LockMarkdownRenderer].
func QuietMarkdownRenderer(sty *styles.Styles, width int) *glamour.TermRenderer {
	mdCacheMu.Lock()
	defer mdCacheMu.Unlock()
	if r, ok := quietMDCache[width]; ok {
		return r
	}
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(codeBlockConfig(sty.QuietMarkdown)),
		glamour.WithWordWrap(width),
		glamour.WithChromaFormatter(formatterName),
	)
	quietMDCache[width] = r
	return r
}

// PlanMarkdownRenderer returns a glamour [glamour.TermRenderer] that keeps the
// rich markdown colors but paints the plan-card background under every
// primitive, for the final plan message card. Renderers are memoized per width
// and shared across callers. Same concurrency contract as [MarkdownRenderer]:
// serialize via [LockMarkdownRenderer].
func PlanMarkdownRenderer(sty *styles.Styles, width int) *glamour.TermRenderer {
	mdCacheMu.Lock()
	defer mdCacheMu.Unlock()
	if r, ok := planMDCache[width]; ok {
		return r
	}
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(codeBlockConfig(sty.PlanMarkdown)),
		glamour.WithWordWrap(width),
		glamour.WithChromaFormatter(formatterName),
	)
	planMDCache[width] = r
	return r
}

// InvalidateMarkdownRendererCache drops every cached renderer
// AND every per-renderer mutex in a single atomic critical
// section so the two maps cannot disagree mid-toggle. Call this
// whenever the active styles change so subsequent renderers
// pick up the new ansi.StyleConfig.
//
// Existing holders of an old mutex (mid-Render goroutines) keep
// their reference safely; new renderers minted after the
// invalidation get freshly minted mutexes.
//
// Lock ordering: mdCacheMu is acquired first, then
// rendererLocksMu — see the comments on each mutex.
func InvalidateMarkdownRendererCache() {
	mdCacheMu.Lock()
	defer mdCacheMu.Unlock()
	rendererLocksMu.Lock()
	defer rendererLocksMu.Unlock()

	mdCache = map[int]*glamour.TermRenderer{}
	quietMDCache = map[int]*glamour.TermRenderer{}
	userMDCache = map[int]*glamour.TermRenderer{}
	planMDCache = map[int]*glamour.TermRenderer{}
	rendererLocks = map[*glamour.TermRenderer]*sync.Mutex{}
}

// rendererLocksMu guards rendererLocks. We key per-renderer
// mutexes by pointer so the lock granularity matches the
// renderer cache granularity (one mutex per (width, palette)
// renderer instance, not one mutex for the entire cache).
//
// Lock ordering: when both mdCacheMu and rendererLocksMu are
// needed (only in InvalidateMarkdownRendererCache), acquire
// mdCacheMu FIRST, then rendererLocksMu.
var (
	rendererLocksMu sync.Mutex
	rendererLocks   = map[*glamour.TermRenderer]*sync.Mutex{}
)

// LockMarkdownRenderer returns the per-renderer mutex used to
// serialize concurrent Render calls on a shared
// [glamour.TermRenderer] instance. The returned [*sync.Mutex] is
// stable for the lifetime of the renderer (i.e. until
// [InvalidateMarkdownRendererCache] is called).
//
// Callers that issue more than one Render call in the same
// logical operation should hold the mutex for the entire
// sequence so other goroutines do not interleave their own
// Render calls and corrupt the renderer state. F8's
// streamingMarkdown is the immediate consumer; other call
// sites that today issue exactly one Render call per item
// render are safe without locking under the single-threaded
// TUI Update loop, but should adopt this lock if they ever run
// in parallel (e.g. background prerender workers).
func LockMarkdownRenderer(r *glamour.TermRenderer) *sync.Mutex {
	rendererLocksMu.Lock()
	defer rendererLocksMu.Unlock()
	if mu, ok := rendererLocks[r]; ok {
		return mu
	}
	mu := &sync.Mutex{}
	rendererLocks[r] = mu
	return mu
}
