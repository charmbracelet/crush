package diffview_test

import (
	_ "embed"
	"testing"

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

func TestDefault(t *testing.T) {
	dv := diffview.New().
		Before("main.go", TestDefaultBefore).
		After("main.go", TestDefaultAfter)

	t.Run("LightMode", func(t *testing.T) {
		dv = dv.Style(diffview.DefaultLightStyle)
		golden.RequireEqual(t, []byte(dv.String()))
	})

	t.Run("DarkMode", func(t *testing.T) {
		dv = dv.Style(diffview.DefaultDarkStyle)
		golden.RequireEqual(t, []byte(dv.String()))
	})
}

func TestNoLineNumbers(t *testing.T) {
	dv := diffview.New().
		Before("main.go", TestDefaultBefore).
		After("main.go", TestDefaultAfter).
		LineNumbers(false)

	t.Run("LightMode", func(t *testing.T) {
		dv = dv.Style(diffview.DefaultLightStyle)
		golden.RequireEqual(t, []byte(dv.String()))
	})

	t.Run("DarkMode", func(t *testing.T) {
		dv = dv.Style(diffview.DefaultDarkStyle)
		golden.RequireEqual(t, []byte(dv.String()))
	})
}

func TestMultipleHunks(t *testing.T) {
	dv := diffview.New().
		Before("main.go", TestMultipleHunksBefore).
		After("main.go", TestMultipleHunksAfter)

	t.Run("LightMode", func(t *testing.T) {
		dv = dv.Style(diffview.DefaultLightStyle)
		golden.RequireEqual(t, []byte(dv.String()))
	})

	t.Run("DarkMode", func(t *testing.T) {
		dv = dv.Style(diffview.DefaultDarkStyle)
		golden.RequireEqual(t, []byte(dv.String()))
	})
}
