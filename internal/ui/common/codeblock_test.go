package common

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"charm.land/glamour/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

const goCodeBlock = "```go\nfunc main() {}\n```\n"

// keywordStyles returns the default theme with the given code keyword color.
func keywordStyles(t *testing.T, color string) styles.Styles {
	t.Helper()
	s, err := styles.HostThemeStyles("", nil, map[string]string{"keyword": color})
	require.NoError(t, err)
	return s
}

// sgr returns the truecolor foreground sequence for "#rrggbb".
func sgr(color string) string {
	var r, g, b int
	_, _ = fmt.Sscanf(color, "#%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("38;2;%d;%d;%dm", r, g, b)
}

// renderCode renders a Go code block with a fresh renderer for s.
func renderCode(t *testing.T, s *styles.Styles) string {
	t.Helper()
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(codeBlockConfig(s.Markdown)),
		glamour.WithWordWrap(80),
		glamour.WithChromaFormatter(formatterName),
	)
	require.NoError(t, err)
	out, err := r.Render(goCodeBlock)
	require.NoError(t, err)
	return out
}

// TestCodeBlockColorsFollowTheStyles checks that every renderer highlights
// fenced code with its own theme's colors. Glamour alone keeps the colors
// of the first theme it ever rendered.
func TestCodeBlockColorsFollowTheStyles(t *testing.T) {
	t.Parallel()

	red, green := keywordStyles(t, "#ff0001"), keywordStyles(t, "#00ff02")
	require.Contains(t, renderCode(t, &red), sgr("#ff0001"))
	out := renderCode(t, &green)
	require.Contains(t, out, sgr("#00ff02"))
	require.NotContains(t, out, sgr("#ff0001"))
	// The first config keeps its colors too.
	require.Contains(t, renderCode(t, &red), sgr("#ff0001"))
}

// TestMarkdownRendererFollowsThemeSwitch mirrors an in-place theme switch:
// the shared *Styles is overwritten and the caches are invalidated.
func TestMarkdownRendererFollowsThemeSwitch(t *testing.T) {
	// Not parallel: the renderer caches are process-wide.
	s := keywordStyles(t, "#ff0003")
	InvalidateStyleCaches()
	t.Cleanup(InvalidateStyleCaches)

	render := func() string {
		r := MarkdownRenderer(&s, 60)
		mu := LockMarkdownRenderer(r)
		mu.Lock()
		defer mu.Unlock()
		out, err := r.Render(goCodeBlock)
		require.NoError(t, err)
		return out
	}
	require.Contains(t, render(), sgr("#ff0003"))

	s = keywordStyles(t, "#00ff04")
	InvalidateStyleCaches()
	out := render()
	require.Contains(t, out, sgr("#00ff04"))
	require.NotContains(t, out, sgr("#ff0003"))
}

// TestCodeBlockSlotsAreReused checks that more distinct themes than there
// are placeholders still render with the right colors.
func TestCodeBlockSlotsAreReused(t *testing.T) {
	// Not parallel: it cycles through every placeholder, which would hand
	// a concurrent test's placeholder to another theme mid-render.
	for i := range codeBlockSlots + 3 {
		color := fmt.Sprintf("#%02x00%02x", i+1, 0xaa)
		s := keywordStyles(t, color)
		require.Contains(t, renderCode(t, &s), sgr(color), "theme %d", i)
	}
}

// TestCodeBlockRendersConcurrently renders code blocks for different themes
// from several goroutines while new themes keep being registered. Run with
// -race: changing chroma's global registry at runtime would be reported.
func TestCodeBlockRendersConcurrently(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup
	// Fewer distinct themes than placeholders, so none is reused while a
	// goroutine still renders with it.
	for g := range 4 {
		wg.Go(func() {
			for i := range 4 {
				color := fmt.Sprintf("#%02x%02x01", g+1, i+1)
				s := keywordStyles(t, color)
				out := renderCode(t, &s)
				if !strings.Contains(out, sgr(color)) {
					t.Errorf("theme %d/%d rendered without its keyword color", g, i)
				}
			}
		})
	}
	wg.Wait()
}

// TestHostSyntaxReachesChromaStyle checks that a host theme's syntax colors
// land in the memoized style that file views and diffs highlight with.
func TestHostSyntaxReachesChromaStyle(t *testing.T) {
	t.Parallel()

	s := keywordStyles(t, "#123456")
	require.Equal(t, "#123456", ChromaStyle(&s, nil).Get(chroma.Keyword).Colour.String())
	out, err := SyntaxHighlight(&s, "func main() {}", "main.go", nil)
	require.NoError(t, err)
	require.Contains(t, out, "18;52;86")
}
