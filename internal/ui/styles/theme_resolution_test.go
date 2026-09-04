package styles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// setTestThemeDirs overrides ThemeDirs for the duration of a test.
// Tests using this must NOT call t.Parallel() since the override is
// shared package-level state.
func setTestThemeDirs(t *testing.T, dirs []string) {
	t.Helper()
	orig := themeDirsOverride
	themeDirsOverride = dirs
	t.Cleanup(func() { themeDirsOverride = orig })
}

func TestLoadTheme_UserFileShadowsBuiltin(t *testing.T) {
	dir := t.TempDir()
	setTestThemeDirs(t, []string{dir})

	tf := &ThemeFile{
		Base:    "charmtone",
		Palette: Palette{Primary: "#ff0000"},
	}
	require.NoError(t, SaveThemeFile(filepath.Join(dir, "charmtone.json"), tf))

	s, err := LoadTheme("charmtone")
	require.NoError(t, err)
	require.NotNil(t, s.WorkingGradFromColor)
}

func TestLoadTheme_UserOnlyTheme(t *testing.T) {
	dir := t.TempDir()
	setTestThemeDirs(t, []string{dir})

	tf := &ThemeFile{
		Base: "gruvbox-dark",
		Palette: Palette{
			Primary: "#111111",
			BgBase:  "#222222",
		},
	}
	require.NoError(t, SaveThemeFile(filepath.Join(dir, "my-custom.json"), tf))

	s, err := LoadTheme("my-custom")
	require.NoError(t, err)
	require.NotNil(t, s.WorkingGradFromColor)
}

func TestLoadTheme_BuiltinFallback(t *testing.T) {
	dir := t.TempDir()
	setTestThemeDirs(t, []string{dir})

	s, err := LoadTheme("gruvbox-dark")
	require.NoError(t, err)
	require.NotNil(t, s.WorkingGradFromColor)
}

func TestLoadTheme_UnknownStillErrors(t *testing.T) {
	dir := t.TempDir()
	setTestThemeDirs(t, []string{dir})

	_, err := LoadTheme("nonexistent-theme-xyz")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown theme")
}

func TestListAllThemes_IncludesBuiltins(t *testing.T) {
	dir := t.TempDir()
	setTestThemeDirs(t, []string{dir})

	infos := ListAllThemes()
	require.NotEmpty(t, infos)

	foundCharmtone := false
	for _, info := range infos {
		if info.Name == "charmtone" {
			foundCharmtone = true
			require.Equal(t, ThemeSourceBuiltin, info.Source)
			require.False(t, info.Overridden)
		}
	}
	require.True(t, foundCharmtone, "expected charmtone in theme list")
}

func TestListAllThemes_ShowsOverridden(t *testing.T) {
	dir := t.TempDir()
	setTestThemeDirs(t, []string{dir})

	tf := &ThemeFile{Base: "gruvbox-dark", Palette: Palette{Primary: "#ff0000"}}
	require.NoError(t, SaveThemeFile(filepath.Join(dir, "gruvbox-dark.json"), tf))

	infos := ListAllThemes()
	for _, info := range infos {
		if info.Name == "gruvbox-dark" {
			require.Equal(t, ThemeSourceBuiltin, info.Source)
			require.True(t, info.Overridden)
			return
		}
	}
	t.Fatal("gruvbox-dark not found in theme list")
}

func TestRevertOverriddenTheme_RestoresBuiltin(t *testing.T) {
	dir := t.TempDir()
	setTestThemeDirs(t, []string{dir})

	// Override a built-in with a custom primary color.
	path := filepath.Join(dir, "gruvbox-dark.json")
	tf := &ThemeFile{Base: "gruvbox-dark", Palette: Palette{Primary: "#ff0000"}}
	require.NoError(t, SaveThemeFile(path, tf))

	// The built-in now reports as overridden.
	requireOverridden(t, "gruvbox-dark", true)

	// Reverting removes the shadowing file; the built-in resolves clean and
	// is no longer reported as overridden.
	require.NoError(t, os.Remove(path))
	requireOverridden(t, "gruvbox-dark", false)
	_, err := LoadTheme("gruvbox-dark")
	require.NoError(t, err)
}

func requireOverridden(t *testing.T, name string, want bool) {
	t.Helper()
	for _, info := range ListAllThemes() {
		if info.Name == name {
			require.Equal(t, want, info.Overridden)
			return
		}
	}
	t.Fatalf("theme %q not found", name)
}

func TestListAllThemes_IncludesUserOnlyThemes(t *testing.T) {
	projectDir := t.TempDir()
	userDir := t.TempDir()
	setTestThemeDirs(t, []string{projectDir, userDir})

	tf := &ThemeFile{Base: "charmtone"}
	require.NoError(t, SaveThemeFile(filepath.Join(userDir, "my-neon.json"), tf))

	infos := ListAllThemes()
	found := false
	for _, info := range infos {
		if info.Name == "my-neon" {
			found = true
			require.Equal(t, ThemeSourceUser, info.Source)
			require.False(t, info.Overridden)
		}
	}
	require.True(t, found, "expected my-neon in theme list")
}

func TestListAllThemes_Sorted(t *testing.T) {
	dir := t.TempDir()
	setTestThemeDirs(t, []string{dir})

	infos := ListAllThemes()
	for i := 1; i < len(infos); i++ {
		require.LessOrEqual(t, infos[i-1].Name, infos[i].Name,
			"themes not sorted: %q before %q", infos[i-1].Name, infos[i].Name)
	}
}

func TestExportResolvedPalette_Builtin(t *testing.T) {
	t.Parallel()
	tf, err := ExportResolvedPalette("charmtone")
	require.NoError(t, err)
	require.Equal(t, "charmtone", tf.Base)
	require.NotEmpty(t, tf.Primary)
	require.NotEmpty(t, tf.BgBase)
	require.NotEmpty(t, tf.FgBase)
	require.NotEmpty(t, tf.Success)
	require.NoError(t, tf.Validate())
}

func TestExportResolvedPalette_GruvboxDark(t *testing.T) {
	t.Parallel()
	tf, err := ExportResolvedPalette("gruvbox-dark")
	require.NoError(t, err)
	require.Equal(t, "gruvbox-dark", tf.Base)
	require.Equal(t, "#fabd2f", tf.Primary)
	require.Equal(t, "#282828", tf.BgBase)
}

func TestExportResolvedPalette_UserTheme(t *testing.T) {
	dir := t.TempDir()
	setTestThemeDirs(t, []string{dir})

	userTf := &ThemeFile{
		Base:    "charmtone",
		Palette: Palette{Primary: "#ff0000"},
	}
	require.NoError(t, SaveThemeFile(filepath.Join(dir, "custom.json"), userTf))

	exported, err := ExportResolvedPalette("custom")
	require.NoError(t, err)
	require.Equal(t, "charmtone", exported.Base)
	require.Equal(t, "#ff0000", exported.Primary)
	require.NotEmpty(t, exported.BgBase)
	require.NotEmpty(t, exported.FgBase)
}

func TestExportResolvedPalette_Unknown(t *testing.T) {
	dir := t.TempDir()
	setTestThemeDirs(t, []string{dir})

	_, err := ExportResolvedPalette("does-not-exist")
	require.Error(t, err)
}

func TestThemeSource_String(t *testing.T) {
	t.Parallel()
	require.Equal(t, "builtin", ThemeSourceBuiltin.String())
	require.Equal(t, "user", ThemeSourceUser.String())
	require.Equal(t, "project", ThemeSourceProject.String())
}

func TestIsBuiltinTheme(t *testing.T) {
	t.Parallel()
	require.True(t, IsBuiltinTheme("charmtone"))
	require.True(t, IsBuiltinTheme("Gruvbox-Dark"))
	require.False(t, IsBuiltinTheme("my-custom"))
}

func TestValidateThemeName_Valid(t *testing.T) {
	dir := t.TempDir()
	setTestThemeDirs(t, []string{dir})

	for _, name := range []string{"neon", "my-theme", "my_theme", "theme2", "a1-b2_c3"} {
		require.NoError(t, ValidateThemeName(name), "expected %q to be valid", name)
	}
}

func TestValidateThemeName_Empty(t *testing.T) {
	dir := t.TempDir()
	setTestThemeDirs(t, []string{dir})

	err := ValidateThemeName("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
}

func TestValidateThemeName_UnsafeCharacters(t *testing.T) {
	dir := t.TempDir()
	setTestThemeDirs(t, []string{dir})

	// Path traversal and separators must be rejected so a theme name can
	// never escape the themes directory.
	for _, name := range []string{"../evil", "a/b", "a\\b", "my theme", "Theme!", "-lead", "trail-", "a--b"} {
		require.Error(t, ValidateThemeName(name), "expected %q to be rejected", name)
	}
}

func TestValidateThemeName_RejectsBuiltin(t *testing.T) {
	dir := t.TempDir()
	setTestThemeDirs(t, []string{dir})

	err := ValidateThemeName("charmtone")
	require.Error(t, err)
	require.Contains(t, err.Error(), "built-in")
}

func TestValidateThemeName_RejectsExisting(t *testing.T) {
	dir := t.TempDir()
	setTestThemeDirs(t, []string{dir})

	require.NoError(t, SaveThemeFile(filepath.Join(dir, "taken.json"), &ThemeFile{Base: "charmtone"}))

	err := ValidateThemeName("taken")
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}
