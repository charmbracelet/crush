package diffview_test

import (
	_ "embed"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/opencode-ai/opencode/internal/exp/diffview"
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
			Before("text.txt", TestMultipleHunksBefore).
			After("text.txt", TestMultipleHunksAfter).
			Width(40)
	}
	LargeWidthFunc = func(dv *diffview.DiffView) *diffview.DiffView {
		return dv.
			Before("text.txt", TestMultipleHunksBefore).
			After("text.txt", TestMultipleHunksAfter).
			Width(120)
	}

	LightModeFunc = func(dv *diffview.DiffView) *diffview.DiffView {
		return dv.Style(diffview.DefaultLightStyle)
	}
	DarkModeFunc = func(dv *diffview.DiffView) *diffview.DiffView {
		return dv.Style(diffview.DefaultDarkStyle)
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
							dv := diffview.New()
							dv = layoutFunc(dv)
							dv = behaviorFunc(dv)
							dv = themeFunc(dv)

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

func assertLineWidth(t *testing.T, expected int, output string) {
	var lineWidth int
	for line := range strings.SplitSeq(output, "\n") {
		lineWidth = max(lineWidth, ansi.StringWidth(line))
	}
	if lineWidth != expected {
		t.Errorf("expected output width to be == %d, got %d", expected, lineWidth)
	}
}
