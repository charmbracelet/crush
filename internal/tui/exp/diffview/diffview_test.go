package diffview_test

import (
	_ "embed"
	"fmt"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/crush/internal/tui/exp/diffview"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
)

//go:embed testdata/TestDefault.before
var TestDefaultBefore string

//go:embed testdata/TestDefault.after
var TestDefaultAfter string

//go:embed testdata/TestMultipleHunks.before
var TestMultipleHunksBefore string

//go:embed testdata/TestMultipleHunks.after
var TestMultipleHunksAfter string

//go:embed testdata/TestNarrow.before
var TestNarrowBefore string

//go:embed testdata/TestNarrow.after
var TestNarrowAfter string

//go:embed testdata/TestTabs.before
var TestTabsBefore string

//go:embed testdata/TestTabs.after
var TestTabsAfter string

//go:embed testdata/TestLineBreakIssue.before
var TestLineBreakIssueBefore string

//go:embed testdata/TestLineBreakIssue.after
var TestLineBreakIssueAfter string

type (
	TestFunc  func(dv *diffview.DiffView) *diffview.DiffView
	TestFuncs map[string]TestFunc
)

var (
	UnifiedFunc = func(dv *diffview.DiffView) *diffview.DiffView {
		return dv.Unified()
	}
	SplitFunc = func(dv *diffview.DiffView) *diffview.DiffView {
		return dv.Split()
	}

	DefaultFunc = func(dv *diffview.DiffView) *diffview.DiffView {
		return dv.
			Before("main.go", TestDefaultBefore).
			After("main.go", TestDefaultAfter)
	}
	NoLineNumbersFunc = func(dv *diffview.DiffView) *diffview.DiffView {
		return dv.
			Before("main.go", TestDefaultBefore).
			After("main.go", TestDefaultAfter).
			LineNumbers(false)
	}
	MultipleHunksFunc = func(dv *diffview.DiffView) *diffview.DiffView {
		return dv.
			Before("main.go", TestMultipleHunksBefore).
			After("main.go", TestMultipleHunksAfter)
	}
	CustomContextLinesFunc = func(dv *diffview.DiffView) *diffview.DiffView {
		return dv.
			Before("main.go", TestMultipleHunksBefore).
			After("main.go", TestMultipleHunksAfter).
			ContextLines(4)
	}
	NarrowFunc = func(dv *diffview.DiffView) *diffview.DiffView {
		return dv.
			Before("text.txt", TestNarrowBefore).
			After("text.txt", TestNarrowAfter)
	}
	SmallWidthFunc = func(dv *diffview.DiffView) *diffview.DiffView {
		return dv.
			Before("main.go", TestMultipleHunksBefore).
			After("main.go", TestMultipleHunksAfter).
			Width(40)
	}
	LargeWidthFunc = func(dv *diffview.DiffView) *diffview.DiffView {
		return dv.
			Before("main.go", TestMultipleHunksBefore).
			After("main.go", TestMultipleHunksAfter).
			Width(120)
	}
	NoSyntaxHighlightFunc = func(dv *diffview.DiffView) *diffview.DiffView {
		return dv.
			Before("main.go", TestMultipleHunksBefore).
			After("main.go", TestMultipleHunksAfter).
			ChromaStyle(nil)
	}

	LightModeFunc = func(dv *diffview.DiffView) *diffview.DiffView {
		return dv.
			Style(diffview.DefaultLightStyle()).
			ChromaStyle(styles.Get("catppuccin-latte"))
	}
	DarkModeFunc = func(dv *diffview.DiffView) *diffview.DiffView {
		return dv.
			Style(diffview.DefaultDarkStyle()).
			ChromaStyle(styles.Get("catppuccin-macchiato"))
	}

	LayoutFuncs = TestFuncs{
		"Unified": UnifiedFunc,
		"Split":   SplitFunc,
	}
	BehaviorFuncs = TestFuncs{
		"Default":            DefaultFunc,
		"NoLineNumbers":      NoLineNumbersFunc,
		"MultipleHunks":      MultipleHunksFunc,
		"CustomContextLines": CustomContextLinesFunc,
		"Narrow":             NarrowFunc,
		"SmallWidth":         SmallWidthFunc,
		"LargeWidth":         LargeWidthFunc,
		"NoSyntaxHighlight":  NoSyntaxHighlightFunc,
	}
	ThemeFuncs = TestFuncs{
		"LightMode": LightModeFunc,
		"DarkMode":  DarkModeFunc,
	}
)

func TestDiffView(t *testing.T) {
	for layoutName, layoutFunc := range LayoutFuncs {
		t.Run(layoutName, func(t *testing.T) {
			for behaviorName, behaviorFunc := range BehaviorFuncs {
				t.Run(behaviorName, func(t *testing.T) {
					for themeName, themeFunc := range ThemeFuncs {
						t.Run(themeName, func(t *testing.T) {
							t.Parallel()

							dv := diffview.New()
							dv = layoutFunc(dv)
							dv = themeFunc(dv)
							dv = behaviorFunc(dv)

							output := dv.String()
							golden.RequireEqual(t, []byte(output))

							switch behaviorName {
							case "SmallWidth":
								assertLineWidth(t, 40, output)
							case "LargeWidth":
								assertLineWidth(t, 120, output)
							}
						})
					}
				})
			}
		})
	}
}

func TestDiffViewTabs(t *testing.T) {
	t.Parallel()

	for layoutName, layoutFunc := range LayoutFuncs {
		t.Run(layoutName, func(t *testing.T) {
			t.Parallel()

			dv := diffview.New().
				Before("main.go", TestTabsBefore).
				After("main.go", TestTabsAfter).
				Style(diffview.DefaultLightStyle()).
				ChromaStyle(styles.Get("catppuccin-latte"))
			dv = layoutFunc(dv)

			output := dv.String()
			golden.RequireEqual(t, []byte(output))
		})
	}
}

func TestDiffViewLineBreakIssue(t *testing.T) {
	t.Parallel()

	for layoutName, layoutFunc := range LayoutFuncs {
		t.Run(layoutName, func(t *testing.T) {
			t.Parallel()

			dv := diffview.New().
				Before("index.js", TestLineBreakIssueBefore).
				After("index.js", TestLineBreakIssueAfter).
				Style(diffview.DefaultLightStyle()).
				ChromaStyle(styles.Get("catppuccin-latte"))
			dv = layoutFunc(dv)

			output := dv.String()
			golden.RequireEqual(t, []byte(output))
		})
	}
}

func TestDiffViewWidth(t *testing.T) {
	for layoutName, layoutFunc := range LayoutFuncs {
		t.Run(layoutName, func(t *testing.T) {
			for width := 1; width <= 110; width++ {
				if layoutName == "Unified" && width > 60 {
					continue
				}

				t.Run(fmt.Sprintf("WidthOf%03d", width), func(t *testing.T) {
					t.Parallel()

					dv := diffview.New().
						Before("main.go", TestMultipleHunksBefore).
						After("main.go", TestMultipleHunksAfter).
						Width(width).
						Style(diffview.DefaultLightStyle()).
						ChromaStyle(styles.Get("catppuccin-latte"))
					dv = layoutFunc(dv)

					output := dv.String()
					golden.RequireEqual(t, []byte(output))

					assertLineWidth(t, width, output)
				})
			}
		})
	}
}

func TestDiffViewHeight(t *testing.T) {
	for layoutName, layoutFunc := range LayoutFuncs {
		t.Run(layoutName, func(t *testing.T) {
			for height := 1; height <= 20; height++ {
				t.Run(fmt.Sprintf("HeightOf%03d", height), func(t *testing.T) {
					t.Parallel()

					dv := diffview.New().
						Before("main.go", TestMultipleHunksBefore).
						After("main.go", TestMultipleHunksAfter).
						Height(height).
						Style(diffview.DefaultLightStyle()).
						ChromaStyle(styles.Get("catppuccin-latte"))
					dv = layoutFunc(dv)

					output := dv.String()
					golden.RequireEqual(t, []byte(output))
				})
			}
		})
	}
}

func TestDiffViewXOffset(t *testing.T) {
	for layoutName, layoutFunc := range LayoutFuncs {
		t.Run(layoutName, func(t *testing.T) {
			for xOffset := range 21 {
				t.Run(fmt.Sprintf("XOffsetOf%02d", xOffset), func(t *testing.T) {
					t.Parallel()

					dv := diffview.New().
						Before("main.go", TestDefaultBefore).
						After("main.go", TestDefaultAfter).
						Style(diffview.DefaultLightStyle()).
						ChromaStyle(styles.Get("catppuccin-latte")).
						Width(60).
						XOffset(xOffset)
					dv = layoutFunc(dv)

					output := dv.String()
					golden.RequireEqual(t, []byte(output))

					assertLineWidth(t, 60, output)
				})
			}
		})
	}
}

func TestDiffViewYOffset(t *testing.T) {
	for layoutName, layoutFunc := range LayoutFuncs {
		t.Run(layoutName, func(t *testing.T) {
			for yOffset := range 17 {
				t.Run(fmt.Sprintf("YOffsetOf%02d", yOffset), func(t *testing.T) {
					t.Parallel()

					dv := diffview.New().
						Before("main.go", TestMultipleHunksBefore).
						After("main.go", TestMultipleHunksAfter).
						Style(diffview.DefaultLightStyle()).
						ChromaStyle(styles.Get("catppuccin-latte")).
						Height(5).
						YOffset(yOffset)
					dv = layoutFunc(dv)

					output := dv.String()
					golden.RequireEqual(t, []byte(output))
				})
			}
		})
	}
}

func TestDiffViewYOffsetInfinite(t *testing.T) {
	for layoutName, layoutFunc := range LayoutFuncs {
		t.Run(layoutName, func(t *testing.T) {
			for yOffset := range 17 {
				t.Run(fmt.Sprintf("YOffsetOf%02d", yOffset), func(t *testing.T) {
					t.Parallel()

					dv := diffview.New().
						Before("main.go", TestMultipleHunksBefore).
						After("main.go", TestMultipleHunksAfter).
						Style(diffview.DefaultLightStyle()).
						ChromaStyle(styles.Get("catppuccin-latte")).
						Height(5).
						YOffset(yOffset).
						InfiniteYScroll(true)
					dv = layoutFunc(dv)

					output := dv.String()
					golden.RequireEqual(t, []byte(output))
				})
			}
		})
	}
}

func assertLineWidth(t *testing.T, expected int, output string) {
	var lineWidth int
	for line := range strings.SplitSeq(output, "\n") {
		lineWidth = max(lineWidth, ansi.StringWidth(line))
	}
	if lineWidth != expected {
		t.Errorf("expected output width to be == %d, got %d", expected, lineWidth)
	}
}

func assertHeight(t *testing.T, expected int, output string) {
	output = strings.TrimSuffix(output, "\n")
	lines := strings.Count(output, "\n") + 1
	if lines != expected {
		t.Errorf("expected output height to be == %d, got %d", expected, lines)
	}
}
