package styles

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestLoadTheme_Builtin(t *testing.T) {
	_, err := LoadTheme("charmtone-panther")
	if err != nil {
		t.Fatalf("LoadTheme(charmtone-panther): %v", err)
	}
}

func TestLoadTheme_DeprecatedAlias(t *testing.T) {
	_, err := LoadTheme("charmtone")
	if err != nil {
		t.Fatalf("LoadTheme(charmtone) via deprecated alias: %v", err)
	}
	newStyles, err := LoadTheme("charmtone-panther")
	if err != nil {
		t.Fatalf("LoadTheme(charmtone-panther): %v", err)
	}
	alias, err := LoadTheme("charmtone")
	if err != nil {
		t.Fatalf("LoadTheme(charmtone): %v", err)
	}
	if *alias.Markdown.Document.Color != *newStyles.Markdown.Document.Color {
		t.Error("deprecated alias should resolve to the same styles")
	}
}

func TestLoadTheme_CaseInsensitive(t *testing.T) {
	_, err := LoadTheme("Gruvbox-Dark")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	_, err = LoadTheme("Keanu")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
}

func TestLoadTheme_Empty(t *testing.T) {
	s, err := LoadTheme("")
	if err != nil {
		t.Fatalf("LoadTheme empty: %v", err)
	}
	if s.WorkingGradFromColor == nil {
		t.Error("expected non-nil WorkingGradFromColor in default theme")
	}
}

func TestLoadTheme_Unknown(t *testing.T) {
	_, err := LoadTheme("nonexistent-theme")
	if err == nil {
		t.Fatal("expected error for unknown theme")
	}
}

func TestBuiltinThemeNames(t *testing.T) {
	names := BuiltinThemeNames()
	if len(names) < 2 {
		t.Fatal("expected at least two builtin themes")
	}
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("names not sorted: %q before %q", names[i-1], names[i])
		}
	}
}

func TestAllBuiltinThemes_DoNotPanic(t *testing.T) {
	for _, name := range BuiltinThemeNames() {
		t.Run(name, func(t *testing.T) {
			_, err := LoadTheme(name)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
		})
	}
}

func TestCloneDoesNotAlias(t *testing.T) {
	s := CharmtonePantera()
	clone := s.Clone()

	origColor := s.Markdown.Document.Color
	if origColor == nil {
		t.Fatal("expected non-nil Document.Color in default styles")
	}

	newColor := "#ff0000"
	clone.Markdown.Document.Color = &newColor

	if s.Markdown.Document.Color == clone.Markdown.Document.Color {
		t.Error("Clone() aliased Markdown.Document.Color pointer")
	}
	if *s.Markdown.Document.Color == "#ff0000" {
		t.Error("modifying clone mutated original")
	}
}

// relativeLuminance and contrastRatio implement the WCAG 2.x formulas so we
// can assert that inline code stays legible in built-in themes.
func relativeLuminance(hexColor string) float64 {
	h := strings.TrimPrefix(hexColor, "#")
	if len(h) != 6 {
		return 0
	}
	channel := func(offset int) float64 {
		v, _ := strconv.ParseInt(h[offset:offset+2], 16, 0)
		c := float64(v) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	r, g, b := channel(0), channel(2), channel(4)
	return 0.2126*r + 0.7152*g + 0.0722*b
}

func contrastRatio(fg, bg string) float64 {
	lf, lb := relativeLuminance(fg), relativeLuminance(bg)
	hi, lo := math.Max(lf, lb), math.Min(lf, lb)
	return (hi + 0.05) / (lo + 0.05)
}

func TestGruvboxDark_InlineCodeContrast(t *testing.T) {
	// Regression test for the reported low-contrast inline code in Gruvbox
	// Dark (bright red on the code background was only ~2.6:1).
	const minAA = 4.5
	s, err := LoadTheme("gruvbox-dark")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	code := s.Markdown.Code
	if code.Color == nil || code.BackgroundColor == nil {
		t.Fatal("inline code style is missing fg/bg colors")
	}
	if ratio := contrastRatio(*code.Color, *code.BackgroundColor); ratio < minAA {
		t.Errorf("gruvbox-dark inline code contrast %.2f is below WCAG AA (%.1f)", ratio, minAA)
	}
}

func TestKeanu_BrightOnBrightContrast(t *testing.T) {
	// Keanu is monochrome green: fgBase and primary are both neon
	// #00ff41, so the shared "fgBase on primary" pairings would render
	// invisible text. The overrides must swap those foregrounds to the
	// dark ink.
	const minAA = 4.5
	s, err := LoadTheme("keanu")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	fg := *hex(s.FilePicker.Selected.GetForeground())
	if ratio := contrastRatio(fg, "#00ff41"); ratio < minAA {
		t.Errorf("keanu selected-file contrast %.2f is below WCAG AA (%.1f)", ratio, minAA)
	}

	if s.Markdown.H1.Color == nil || s.Markdown.H1.BackgroundColor == nil {
		t.Fatal("H1 style is missing fg/bg colors")
	}
	if ratio := contrastRatio(*s.Markdown.H1.Color, *s.Markdown.H1.BackgroundColor); ratio < minAA {
		t.Errorf("keanu H1 contrast %.2f is below WCAG AA (%.1f)", ratio, minAA)
	}

	code := s.Markdown.Code
	if code.Color == nil || code.BackgroundColor == nil {
		t.Fatal("inline code style is missing fg/bg colors")
	}
	if ratio := contrastRatio(*code.Color, *code.BackgroundColor); ratio < minAA {
		t.Errorf("keanu inline code contrast %.2f is below WCAG AA (%.1f)", ratio, minAA)
	}

	// The inline code chip must stand out from the message background,
	// like it does inside the thinking block.
	if bg := *hex(s.Background); *code.BackgroundColor == bg {
		t.Errorf("inline code background %s matches the message background; the chip is invisible", bg)
	}
}

func TestKeanu_ModeBadges(t *testing.T) {
	// Mode badges and banners pair a foreground with a bright mode
	// background; in a monochrome green theme the default plan pairing
	// collapses into green-on-green. The banner badge is copied from the
	// badge inside quickStyle before overrides run, so check both.
	const minAA = 4.5
	s, err := LoadTheme("keanu")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	badges := map[string]lipgloss.Style{
		"ModeBadgePlan":       s.Status.ModeBadgePlan,
		"ModeBannerPlanBadge": s.Status.ModeBannerPlanBadge,
		"ModeBannerPlan":      s.Status.ModeBannerPlan,
		"ModeBadgeYolo":       s.Status.ModeBadgeYolo,
		"ModeBannerYoloBadge": s.Status.ModeBannerYoloBadge,
		"ModeBannerYolo":      s.Status.ModeBannerYolo,
	}
	for name, style := range badges {
		fg := *hex(style.GetForeground())
		bg := *hex(style.GetBackground())
		if ratio := contrastRatio(fg, bg); ratio < minAA {
			t.Errorf("keanu %s contrast %.2f is below WCAG AA (%.1f)", name, ratio, minAA)
		}
	}
}

func TestKeanu_BodyTextContrast(t *testing.T) {
	// The main text color must stay legible on the pure black background.
	const minAA = 4.5
	s, err := LoadTheme("keanu")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	fg := *hex(s.Messages.NoContent.GetForeground())
	bg := *hex(s.Background)
	if ratio := contrastRatio(fg, bg); ratio < minAA {
		t.Errorf("keanu body text contrast %.2f is below WCAG AA (%.1f)", ratio, minAA)
	}
}

func TestLoadTheme_CharmtoneLight(t *testing.T) {
	_, err := LoadTheme("charmtone-light")
	if err != nil {
		t.Fatalf("LoadTheme(charmtone-light): %v", err)
	}
}

func TestCharmtoneLight_IsLight(t *testing.T) {
	s, err := LoadTheme("charmtone-light")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// The base background must be a light surface and the base ink dark,
	// which is what makes the theme usable in a black-on-white terminal.
	bgHex := *hex(s.Background)
	if lum := relativeLuminance(bgHex); lum < 0.5 {
		t.Errorf("light theme background %s has luminance %.2f; want >= 0.5", bgHex, lum)
	}
	fgHex := *hex(s.Messages.NoContent.GetForeground())
	if lum := relativeLuminance(fgHex); lum > 0.5 {
		t.Errorf("light theme base ink %s has luminance %.2f; want <= 0.5", fgHex, lum)
	}
}

func TestCharmtoneLight_BaseContrast(t *testing.T) {
	// The base ink on the base background must clear WCAG AA, so ordinary
	// text stays legible in a black-on-white terminal.
	const minAA = 4.5
	s, err := LoadTheme("charmtone-light")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	bgHex := *hex(s.Background)
	fgHex := *hex(s.Messages.NoContent.GetForeground())
	if ratio := contrastRatio(fgHex, bgHex); ratio < minAA {
		t.Errorf("light theme base ink contrast %.2f is below WCAG AA (%.1f)", ratio, minAA)
	}
}

func TestCharmtoneLight_ANSIWhiteIsDarkInk(t *testing.T) {
	// In a light theme the ANSI "white" (index 7) must remap to a dark ink
	// rather than a near-white, or raw terminal output that uses white
	// would be invisible on the white background.
	s, err := LoadTheme("charmtone-light")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	whiteHex := *hex(s.ANSI[7])
	if lum := relativeLuminance(whiteHex); lum > 0.5 {
		t.Errorf("light theme ANSI white %s has luminance %.2f; want dark ink (<= 0.5)", whiteHex, lum)
	}
	brightWhiteHex := *hex(s.ANSI[15])
	if lum := relativeLuminance(brightWhiteHex); lum > 0.5 {
		t.Errorf("light theme ANSI bright white %s has luminance %.2f; want dark ink (<= 0.5)", brightWhiteHex, lum)
	}
}
