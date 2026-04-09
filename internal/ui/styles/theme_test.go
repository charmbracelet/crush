package styles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseTheme_ValidMinimal(t *testing.T) {
	palette := validTestPalette()
	data, err := json.Marshal(palette)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := ParseTheme(data)
	if err != nil {
		t.Fatalf("ParseTheme: %v", err)
	}
	if got.Colors.Primary != palette.Colors.Primary {
		t.Errorf("Primary = %q, want %q", got.Colors.Primary, palette.Colors.Primary)
	}
}

func TestParseTheme_MissingFields(t *testing.T) {
	data := []byte(`{"colors": {"primary": "#ff0000"}}`)
	_, err := ParseTheme(data)
	if err == nil {
		t.Fatal("expected error for missing required fields")
	}
}

func TestParseTheme_InvalidHex(t *testing.T) {
	palette := validTestPalette()
	palette.Colors.Primary = "not-a-color"
	data, _ := json.Marshal(palette)
	_, err := ParseTheme(data)
	if err == nil {
		t.Fatal("expected error for invalid hex color")
	}
}

func TestParseTheme_InvalidHexFormat(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"no hash", "FF0000"},
		{"too short", "#FF00"},
		{"too long", "#FF00001"},
		{"invalid chars", "#GGHHII"},
		{"empty", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			palette := validTestPalette()
			palette.Colors.Primary = tt.value
			data, _ := json.Marshal(palette)
			_, err := ParseTheme(data)
			if err == nil {
				t.Fatalf("expected error for hex %q", tt.value)
			}
		})
	}
}

func TestParseTheme_ValidHexFormats(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"6-digit lowercase", "#ff0000"},
		{"6-digit uppercase", "#FF0000"},
		{"6-digit mixed", "#aAbBcC"},
		{"3-digit", "#f00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			palette := validTestPalette()
			palette.Colors.Primary = tt.value
			data, _ := json.Marshal(palette)
			_, err := ParseTheme(data)
			if err != nil {
				t.Fatalf("unexpected error for hex %q: %v", tt.value, err)
			}
		})
	}
}

func TestParseTheme_MalformedJSON(t *testing.T) {
	_, err := ParseTheme([]byte(`{invalid`))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestParseTheme_EmptyJSON(t *testing.T) {
	_, err := ParseTheme([]byte(`{}`))
	if err == nil {
		t.Fatal("expected error for empty JSON (missing colors)")
	}
}

func TestParseTheme_OptionalDiffColors(t *testing.T) {
	palette := validTestPalette()
	// No diff colors set — should still parse successfully.
	palette.Colors.DiffInsertFg = ""
	palette.Colors.DiffInsertBg = ""
	palette.Colors.DiffDeleteFg = ""
	data, _ := json.Marshal(palette)
	_, err := ParseTheme(data)
	if err != nil {
		t.Fatalf("ParseTheme with no diff colors: %v", err)
	}
}

func TestParseTheme_InvalidOptionalDiffColor(t *testing.T) {
	palette := validTestPalette()
	palette.Colors.DiffInsertFg = "bad-color"
	data, _ := json.Marshal(palette)
	_, err := ParseTheme(data)
	if err == nil {
		t.Fatal("expected error for invalid optional diff color")
	}
}

func TestValidate_SortedMissingFields(t *testing.T) {
	// Verify error message lists fields in sorted order.
	tc := &ThemeColors{} // all empty
	err := tc.validate()
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	// "bg_base" should come before "primary" in sorted order.
	bgIdx := indexOf(msg, "bg_base")
	priIdx := indexOf(msg, "primary")
	if bgIdx < 0 || priIdx < 0 {
		t.Fatalf("expected both bg_base and primary in error: %s", msg)
	}
	if bgIdx > priIdx {
		t.Errorf("missing fields not sorted: bg_base at %d, primary at %d", bgIdx, priIdx)
	}
}

func TestLoadTheme_Builtin(t *testing.T) {
	palette, err := LoadTheme("charm")
	if err != nil {
		t.Fatalf("LoadTheme(charm): %v", err)
	}
	if palette.Name != "Charm" {
		t.Errorf("Name = %q, want Charm", palette.Name)
	}
}

func TestLoadTheme_CaseInsensitive(t *testing.T) {
	palette, err := LoadTheme("Catppuccin-Mocha")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	if palette.Name != "Catppuccin Mocha" {
		t.Errorf("Name = %q, want Catppuccin Mocha", palette.Name)
	}
}

func TestLoadTheme_Empty(t *testing.T) {
	palette, err := LoadTheme("")
	if err != nil {
		t.Fatalf("LoadTheme empty: %v", err)
	}
	if palette.Name != "Charm" {
		t.Errorf("expected default palette, got %q", palette.Name)
	}
}

func TestLoadTheme_MissingFile(t *testing.T) {
	_, err := LoadTheme("/nonexistent/path/theme.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadTheme_InvalidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte(`{invalid`), 0o644)
	_, err := LoadTheme(path)
	if err == nil {
		t.Fatal("expected error for invalid file")
	}
}

func TestLoadTheme_ValidFile(t *testing.T) {
	palette := validTestPalette()
	palette.Name = "Custom Test"
	data, _ := json.Marshal(palette)

	dir := t.TempDir()
	path := filepath.Join(dir, "test-theme.json")
	os.WriteFile(path, data, 0o644)

	got, err := LoadTheme(path)
	if err != nil {
		t.Fatalf("LoadTheme file: %v", err)
	}
	if got.Name != "Custom Test" {
		t.Errorf("Name = %q, want Custom Test", got.Name)
	}
}

func TestBuiltinThemeNames(t *testing.T) {
	names := BuiltinThemeNames()
	if len(names) == 0 {
		t.Fatal("expected at least one builtin theme")
	}
	// Verify sorted order.
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("names not sorted: %q before %q", names[i-1], names[i])
		}
	}
	// Verify charm is included.
	found := false
	for _, n := range names {
		if n == "charm" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'charm' in builtin theme names")
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

func TestDiscoverThemes_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	// Create the themes dir but leave it empty.
	os.MkdirAll(filepath.Join(dir, "crush", "themes"), 0o755)

	themes := DiscoverThemes()
	if len(themes) != 0 {
		t.Errorf("expected 0 themes in empty dir, got %d", len(themes))
	}
}

func TestDiscoverThemes_ValidCustomTheme(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	themeDir := filepath.Join(dir, "crush", "themes")
	os.MkdirAll(themeDir, 0o755)

	palette := validTestPalette()
	palette.Name = "My Custom"
	data, _ := json.Marshal(palette)
	os.WriteFile(filepath.Join(themeDir, "my-custom.json"), data, 0o644)

	themes := DiscoverThemes()
	if len(themes) != 1 {
		t.Fatalf("expected 1 theme, got %d", len(themes))
	}
	if themes[0].Name != "My Custom" {
		t.Errorf("Name = %q, want My Custom", themes[0].Name)
	}
}

func TestDiscoverThemes_ShadowBuiltin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	themeDir := filepath.Join(dir, "crush", "themes")
	os.MkdirAll(themeDir, 0o755)

	palette := validTestPalette()
	palette.Name = "Charm" // shadows builtin
	data, _ := json.Marshal(palette)
	os.WriteFile(filepath.Join(themeDir, "charm.json"), data, 0o644)

	themes := DiscoverThemes()
	if len(themes) != 0 {
		t.Errorf("expected shadowed theme to be skipped, got %d", len(themes))
	}
}

func TestDiscoverThemes_SkipsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	themeDir := filepath.Join(dir, "crush", "themes")
	os.MkdirAll(themeDir, 0o755)

	os.WriteFile(filepath.Join(themeDir, "bad.json"), []byte(`{invalid`), 0o644)

	themes := DiscoverThemes()
	if len(themes) != 0 {
		t.Errorf("expected 0 valid themes, got %d", len(themes))
	}
}

func TestDiscoverThemes_SkipsNonJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	themeDir := filepath.Join(dir, "crush", "themes")
	os.MkdirAll(themeDir, 0o755)

	os.WriteFile(filepath.Join(themeDir, "readme.txt"), []byte("not a theme"), 0o644)

	themes := DiscoverThemes()
	if len(themes) != 0 {
		t.Errorf("expected 0 themes for non-JSON files, got %d", len(themes))
	}
}

func TestDiffDefaults_DeriveFromPalette(t *testing.T) {
	tc := validTestPalette().Colors
	// Clear optional diff fields.
	tc.DiffInsertFg = ""
	tc.DiffInsertBg = ""
	tc.DiffInsertBgLight = ""
	tc.DiffDeleteFg = ""
	tc.DiffDeleteBg = ""
	tc.DiffDeleteBgLight = ""

	insertFg, insertBg, insertBgLight, deleteFg, deleteBg, deleteBgLight := tc.DiffDefaults()

	// InsertFg should fall back to Green.
	if insertFg != tc.Green {
		t.Errorf("insertFg = %q, want Green %q", insertFg, tc.Green)
	}
	// DeleteFg should fall back to Red.
	if deleteFg != tc.Red {
		t.Errorf("deleteFg = %q, want Red %q", deleteFg, tc.Red)
	}
	// Bg values should be derived blends (not empty).
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

func TestDefaultPaletteRoundTrip(t *testing.T) {
	palette := DefaultPalette()
	if err := palette.Colors.validate(); err != nil {
		t.Fatalf("default palette fails validation: %v", err)
	}

	data, err := json.Marshal(palette)
	if err != nil {
		t.Fatalf("marshal default palette: %v", err)
	}

	got, err := ParseTheme(data)
	if err != nil {
		t.Fatalf("parse round-tripped default palette: %v", err)
	}

	if got.Colors.Primary != palette.Colors.Primary {
		t.Errorf("Primary mismatch after round-trip: %q vs %q", got.Colors.Primary, palette.Colors.Primary)
	}
	if got.Colors.BgBase != palette.Colors.BgBase {
		t.Errorf("BgBase mismatch after round-trip: %q vs %q", got.Colors.BgBase, palette.Colors.BgBase)
	}
}

func TestNewStyles_DoesNotPanic(t *testing.T) {
	for _, name := range BuiltinThemeNames() {
		t.Run(name, func(t *testing.T) {
			palette, err := LoadTheme(name)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			// Should not panic.
			_ = NewStyles(palette)
		})
	}
}

func TestThemeConfigDir_XDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	dir := themeConfigDir()
	want := "/custom/config/crush/themes"
	if dir != want {
		t.Errorf("themeConfigDir = %q, want %q", dir, want)
	}
}

func TestBlendHex(t *testing.T) {
	// Blending black (#000000) with white (#ffffff) at 50% should give ~#808080.
	result := blendHex("#000000", "#ffffff", 0.5)
	// Allow small rounding differences.
	if result != "#7f7f7f" && result != "#808080" {
		t.Errorf("blend(black, white, 0.5) = %q, want ~#808080", result)
	}

	// Blending at 0% should return base.
	result = blendHex("#ff0000", "#0000ff", 0.0)
	if result != "#ff0000" {
		t.Errorf("blend at 0%% = %q, want #ff0000", result)
	}

	// Blending at 100% should return accent.
	result = blendHex("#ff0000", "#0000ff", 1.0)
	if result != "#0000ff" {
		t.Errorf("blend at 100%% = %q, want #0000ff", result)
	}
}

// validTestPalette returns a valid ThemePalette with all required fields set.
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

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
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
		if idx := indexOf(name, ","); idx >= 0 {
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
		t.Error("Clone() aliased Markdown.Document.Color pointer — shallow copy detected")
	}
	if *s.Markdown.Document.Color == "#ff0000" {
		t.Error("modifying clone mutated original — pointer aliasing")
	}
}
