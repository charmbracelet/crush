package model

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
)

// BenchmarkThemeSwitchDraw measures the first frame after a theme swap on a
// scrolled, warmed session with a visible scrollbar, alternating two themes
// with different colours.
func BenchmarkThemeSwitchDraw(b *testing.B) {
	gruvbox, err := styles.LoadTheme("gruvbox-dark")
	if err != nil {
		b.Fatal(err)
	}
	themes := []styles.Styles{gruvbox, styles.CharmtonePantera()}
	for _, n := range []int{20, 200, 800} {
		b.Run(fmt.Sprintf("msgs=%d", n*2), func(b *testing.B) {
			u := newHostThemeTestUI(b, nil)
			u.chat.scrollbarMode = config.ScrollbarAlways
			const width, height = 140, 40
			u.chat.SetSize(width, height)
			u.chat.SetMessages(buildSession(n)...)
			scr := uv.NewScreenBuffer(width, height)
			area := uv.Rect(0, 0, width, height)
			u.chat.Draw(scr, area)

			b.ReportAllocs()
			for i := 0; b.Loop(); i++ {
				u.applyTheme(themes[i%2])
				u.chat.Draw(scr, area)
			}
		})
	}
}
