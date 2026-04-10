package styles

import (
	"reflect"
	"strings"
	"testing"
)

func TestLoadTheme_Builtin(t *testing.T) {
	palette, err := LoadTheme("charmtone")
	if err != nil {
		t.Fatalf("LoadTheme(charmtone): %v", err)
	}
	if palette.Name != "Charmtone" {
		t.Errorf("Name = %q, want Charmtone", palette.Name)
	}
}

func TestLoadTheme_CaseInsensitive(t *testing.T) {
	palette, err := LoadTheme("Gruvbox-Dark")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	if palette.Name != "Gruvbox Dark" {
		t.Errorf("Name = %q, want Gruvbox Dark", palette.Name)
	}
}

func TestLoadTheme_Empty(t *testing.T) {
	palette, err := LoadTheme("")
	if err != nil {
		t.Fatalf("LoadTheme empty: %v", err)
	}
	if palette.Name != "Charmtone" {
		t.Errorf("expected default palette, got %q", palette.Name)
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
	if len(names) == 0 {
		t.Fatal("expected at least one builtin theme")
	}
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("names not sorted: %q before %q", names[i-1], names[i])
		}
	}
	found := false
	for _, n := range names {
		if n == "charmtone" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'charmtone' in builtin theme names")
	}
}

func TestAllBuiltinThemesValid(t *testing.T) {
	for _, name := range BuiltinThemeNames() {
		palette, err := LoadTheme(name)
		if err != nil {
			t.Errorf("builtin theme %q failed to load: %v", name, err)
			continue
		}
		if err := palette.Colors.validate(); err != nil {
			t.Errorf("builtin theme %q failed validation: %v", name, err)
		}
	}
}

func TestValidate_SortedMissingFields(t *testing.T) {
	tc := &ThemeColors{}
	err := tc.validate()
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	bgIdx := strings.Index(msg, "bg_base")
	priIdx := strings.Index(msg, "primary")
	if bgIdx < 0 || priIdx < 0 {
		t.Fatalf("expected both bg_base and primary in error: %s", msg)
	}
	if bgIdx > priIdx {
		t.Errorf("missing fields not sorted: bg_base at %d, primary at %d", bgIdx, priIdx)
	}
}

func TestValidate_InvalidHex(t *testing.T) {
	palette := validTestPalette()
	palette.Colors.Primary = "not-a-color"
	if err := palette.Colors.validate(); err == nil {
		t.Fatal("expected error for invalid hex color")
	}
}

func TestValidate_ValidHexFormats(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"6-digit lowercase", "#ff0000"},
		{"6-digit uppercase", "#FF0000"},
		{"3-digit", "#f00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			palette := validTestPalette()
			palette.Colors.Primary = tt.value
			if err := palette.Colors.validate(); err != nil {
				t.Fatalf("unexpected error for hex %q: %v", tt.value, err)
			}
		})
	}
}

func TestDiffDefaults_DeriveFromPalette(t *testing.T) {
	tc := validTestPalette().Colors
	tc.DiffInsertFg = ""
	tc.DiffInsertBg = ""
	tc.DiffInsertBgLight = ""
	tc.DiffDeleteFg = ""
	tc.DiffDeleteBg = ""
	tc.DiffDeleteBgLight = ""

	insertFg, insertBg, insertBgLight, deleteFg, deleteBg, deleteBgLight := tc.DiffDefaults()

	if insertFg != tc.Green {
		t.Errorf("insertFg = %q, want Green %q", insertFg, tc.Green)
	}
	if deleteFg != tc.Red {
		t.Errorf("deleteFg = %q, want Red %q", deleteFg, tc.Red)
	}
	for _, v := range []string{insertBg, insertBgLight, deleteBg, deleteBgLight} {
		if v == "" {
			t.Error("expected non-empty derived diff background color")
		}
		if !hexColorPattern.MatchString(v) {
			t.Errorf("derived diff color %q is not valid hex", v)
		}
	}
}

func TestDiffDefaults_PreservesExplicit(t *testing.T) {
	tc := validTestPalette().Colors
	tc.DiffInsertFg = "#112233"
	tc.DiffDeleteFg = "#445566"

	insertFg, _, _, deleteFg, _, _ := tc.DiffDefaults()
	if insertFg != "#112233" {
		t.Errorf("insertFg = %q, want #112233", insertFg)
	}
	if deleteFg != "#445566" {
		t.Errorf("deleteFg = %q, want #445566", deleteFg)
	}
}

func TestNewStyles_DoesNotPanic(t *testing.T) {
	for _, name := range BuiltinThemeNames() {
		t.Run(name, func(t *testing.T) {
			palette, err := LoadTheme(name)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			_ = NewStyles(palette)
		})
	}
}

func TestDefaultPaletteUsesCharmtone(t *testing.T) {
	p := DefaultPalette()
	if p.Name != "Charmtone" {
		t.Errorf("Name = %q, want Charmtone", p.Name)
	}
	if p.Colors.Primary == "" {
		t.Error("Primary should be set from charmtone")
	}
}

func TestRequiredColorFieldsCoversAllStructFields(t *testing.T) {
	tc := &ThemeColors{}
	required := requiredColorFields(tc)
	requiredNames := make(map[string]bool)
	for _, f := range required {
		requiredNames[f.Name] = true
	}

	optionalFields := map[string]bool{
		"diff_insert_fg":       true,
		"diff_insert_bg":       true,
		"diff_insert_bg_light": true,
		"diff_delete_fg":       true,
		"diff_delete_bg":       true,
		"diff_delete_bg_light": true,
	}

	typ := reflect.TypeOf(ThemeColors{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		name := jsonTag
		if idx := strings.Index(name, ","); idx >= 0 {
			name = name[:idx]
		}
		if optionalFields[name] {
			continue
		}
		if !requiredNames[name] {
			t.Errorf("ThemeColors field %q (json:%q) is not covered by requiredColorFields()", field.Name, name)
		}
	}
}

func TestCloneDoesNotAlias(t *testing.T) {
	s := DefaultStyles()
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

func TestBlendHex(t *testing.T) {
	result := blendHex("#000000", "#ffffff", 0.5)
	if result != "#7f7f7f" && result != "#808080" {
		t.Errorf("blend(black, white, 0.5) = %q, want ~#808080", result)
	}

	result = blendHex("#ff0000", "#0000ff", 0.0)
	if result != "#ff0000" {
		t.Errorf("blend at 0%% = %q, want #ff0000", result)
	}

	result = blendHex("#ff0000", "#0000ff", 1.0)
	if result != "#0000ff" {
		t.Errorf("blend at 100%% = %q, want #0000ff", result)
	}
}

func validTestPalette() ThemePalette {
	return ThemePalette{
		Name:   "Test",
		Author: "Test Author",
		Colors: ThemeColors{
			Primary:       "#6B50FF",
			Secondary:     "#FFD700",
			Tertiary:      "#00FF00",
			BgBase:        "#1a1a1a",
			BgBaseLighter: "#2a2a2a",
			BgSubtle:      "#3a3a3a",
			BgOverlay:     "#4a4a4a",
			FgBase:        "#ffffff",
			FgMuted:       "#888888",
			FgHalfMuted:   "#aaaaaa",
			FgSubtle:      "#666666",
			Border:        "#333333",
			BorderFocus:   "#6B50FF",
			Error:         "#ff0000",
			Warning:       "#ffaa00",
			Info:          "#00aaff",
			White:         "#ffffff",
			BlueLight:     "#aaddff",
			Blue:          "#0088ff",
			BlueDark:      "#003366",
			GreenLight:    "#aaffaa",
			Green:         "#00ff00",
			GreenDark:     "#006600",
			Red:           "#ff0000",
			RedDark:       "#880000",
			Yellow:        "#ffff00",
		},
	}
}
