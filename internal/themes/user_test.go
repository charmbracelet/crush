package themes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeUserTheme(t *testing.T, configHome, name, content string) {
	t.Helper()
	dir := filepath.Join(configHome, "crush", "themes")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name+".json"), []byte(content), 0o644))
}

func TestUserThemeAppearsInCatalog(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	writeUserTheme(t, configHome, "MyTheme", `{
		"background": "#111111",
		"foreground": "#eeeeee",
		"black": "#000000", "red": "#ff0000", "green": "#00ff00", "yellow": "#ffff00",
		"blue": "#0000ff", "purple": "#ff00ff", "cyan": "#00ffff", "white": "#ffffff",
		"brightBlack": "#333333", "brightRed": "#ff6666", "brightGreen": "#66ff66",
		"brightYellow": "#ffff66", "brightBlue": "#6666ff", "brightPurple": "#ff66ff",
		"brightCyan": "#66ffff", "brightWhite": "#eeeeee"
	}`)

	catalog := Catalog()
	s := findScheme(catalog, "MyTheme")
	require.NotNil(t, s)

	got, err := Resolve("MyTheme")
	require.NoError(t, err)
	require.Equal(t, s.Background, got.Background)
}

func TestUserThemeShadowsEmbedded(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	writeUserTheme(t, configHome, "Dracula", `{
		"background": "#123456",
		"foreground": "#ffffff",
		"black": "#000000", "red": "#ff0000", "green": "#00ff00", "yellow": "#ffff00",
		"blue": "#0000ff", "purple": "#ff00ff", "cyan": "#00ffff", "white": "#ffffff",
		"brightBlack": "#333333", "brightRed": "#ff6666", "brightGreen": "#66ff66",
		"brightYellow": "#ffff66", "brightBlue": "#6666ff", "brightPurple": "#ff66ff",
		"brightCyan": "#66ffff", "brightWhite": "#eeeeee"
	}`)

	got, err := Resolve("Dracula")
	require.NoError(t, err)
	require.Equal(t, "#123456", hexOf(got.Background)[:7])
}

func TestUserThemeShadowsEmbeddedCaseInsensitive(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	// Lowercase filename must shadow the embedded "Dracula" scheme.
	writeUserTheme(t, configHome, "dracula", `{
		"background": "#123456",
		"foreground": "#ffffff"
	}`)

	catalog := Catalog()
	require.NotNil(t, findScheme(catalog, "dracula"))
	require.Nil(t, findScheme(catalog, "Dracula"))

	got, err := Resolve("Dracula")
	require.NoError(t, err)
	require.Equal(t, "#123456", hexOf(got.Background)[:7])
}

func TestMalformedUserThemeIsSkipped(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	writeUserTheme(t, configHome, "Bad", `{not json`)

	catalog := Catalog()
	require.NotNil(t, findScheme(catalog, "Dracula"))
	require.Nil(t, findScheme(catalog, "Bad"))
}
