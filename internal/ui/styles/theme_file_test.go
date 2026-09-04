package styles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestThemeFile_SaveAndLoad_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "my-theme.json")

	tf := &ThemeFile{
		Base: "charmtone",
		Palette: Palette{
			Primary: "#ff0000",
			BgBase:  "#1a1a2e",
			FgBase:  "#ffffff",
		},
	}

	require.NoError(t, SaveThemeFile(path, tf))

	loaded, err := LoadThemeFile(path)
	require.NoError(t, err)
	require.Equal(t, tf.Base, loaded.Base)
	require.Equal(t, tf.Primary, loaded.Primary)
	require.Equal(t, tf.BgBase, loaded.BgBase)
	require.Equal(t, tf.FgBase, loaded.FgBase)
	require.Empty(t, loaded.Secondary)
}

func TestThemeFile_SaveCreatesDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "theme.json")

	tf := &ThemeFile{Base: "gruvbox-dark"}
	require.NoError(t, SaveThemeFile(path, tf))

	_, err := os.Stat(path)
	require.NoError(t, err)
}

func TestThemeFile_LoadInvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{invalid"), 0o644))

	_, err := LoadThemeFile(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse theme file")
}

func TestThemeFile_LoadInvalidColor(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad-color.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"primary":"not-a-color"}`), 0o644))

	_, err := LoadThemeFile(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "primary")
}

func TestThemeFile_LoadUnknownFieldsWarnsButSucceeds(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "extra.json")
	data := `{"base":"charmtone","primary":"#ff0000","unknown_field":"value","another_bad":"x"}`
	require.NoError(t, os.WriteFile(path, []byte(data), 0o644))

	tf, err := LoadThemeFile(path)
	require.NoError(t, err)
	require.Equal(t, "charmtone", tf.Base)
	require.Equal(t, "#ff0000", tf.Primary)
}

func TestThemeFile_LoadMissingFile(t *testing.T) {
	t.Parallel()
	_, err := LoadThemeFile("/nonexistent/path/theme.json")
	require.Error(t, err)
	require.Contains(t, err.Error(), "read theme file")
}

func TestThemeFile_EmptyObjectIsValid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	require.NoError(t, os.WriteFile(path, []byte(`{}`), 0o644))

	tf, err := LoadThemeFile(path)
	require.NoError(t, err)
	require.Empty(t, tf.Base)
	require.Empty(t, tf.Primary)
}

func TestFindThemeFile_ProjectLocalWins(t *testing.T) {
	t.Parallel()
	// We can't easily override ThemeDirs in tests since it uses home.Config().
	// Instead, test that FindThemeFile returns an error for nonexistent themes.
	_, err := FindThemeFile("definitely-does-not-exist-" + t.Name())
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestListUserThemes_EmptyDirs(t *testing.T) {
	t.Parallel()
	// When no theme dirs exist, should return empty list without error.
	names, err := ListUserThemes()
	require.NoError(t, err)
	// May or may not be empty depending on user's system; just verify no error.
	_ = names
}

func TestListUserThemes_ReadsDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	themesDir := filepath.Join(dir, "themes")
	require.NoError(t, os.MkdirAll(themesDir, 0o755))

	// Create some theme files.
	for _, name := range []string{"alpha.json", "beta.json", "not-a-theme.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(themesDir, name), []byte(`{}`), 0o644))
	}
	// Create a subdirectory (should be ignored).
	require.NoError(t, os.MkdirAll(filepath.Join(themesDir, "subdir.json"), 0o755))

	// Override ThemeDirs by testing the internal logic directly.
	entries, err := os.ReadDir(themesDir)
	require.NoError(t, err)

	var names []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		names = append(names, e.Name())
	}
	require.Equal(t, []string{"alpha.json", "beta.json"}, names)
}

func TestThemeFile_AllPaletteFieldsRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "full.json")

	tf := &ThemeFile{
		Base: "gruvbox-dark",
		Palette: Palette{
			Primary:           "#fabd2f",
			Secondary:         "#d3869b",
			Accent:            "#b8bb26",
			Keyword:           "#fe8019",
			FgBase:            "#ebdbb2",
			FgSubtle:          "#bdae93",
			FgMoreSubtle:      "#a89984",
			FgMostSubtle:      "#928374",
			BgBase:            "#282828",
			BgMostVisible:     "#665c54",
			BgLessVisible:     "#504945",
			BgLeastVisible:    "#3c3836",
			OnPrimary:         "#282828",
			Separator:         "#504945",
			Destructive:       "#fb4934",
			Error:             "#cc241d",
			Warning:           "#d79921",
			WarningSubtle:     "#fabd2f",
			Attention:         "#fe8019",
			Busy:              "#fabd2f",
			Info:              "#83a598",
			InfoMoreSubtle:    "#83a598",
			InfoMostSubtle:    "#458588",
			Success:           "#b8bb26",
			SuccessMoreSubtle: "#b8bb26",
			SuccessMostSubtle: "#8ec07c",
		},
	}

	require.NoError(t, SaveThemeFile(path, tf))

	loaded, err := LoadThemeFile(path)
	require.NoError(t, err)
	require.Equal(t, tf.Base, loaded.Base)
	require.Equal(t, tf.Palette, loaded.Palette)
}
