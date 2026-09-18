package themes

import (
	"testing"

	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestResolveDefault(t *testing.T) {
	t.Parallel()
	want := styles.CharmtonePantera()

	for _, name := range []string{"", "default"} {
		got, err := Resolve(name)
		require.NoError(t, err)
		require.Equal(t, want.Background, got.Background)
		require.Equal(t, want.ANSI, got.ANSI)
	}
}

func TestResolveHyper(t *testing.T) {
	t.Parallel()
	got, err := Resolve("hyper")
	require.NoError(t, err)
	require.Equal(t, styles.HypercrushObsidiana().Background, got.Background)
}

func TestResolveCatalogScheme(t *testing.T) {
	t.Parallel()
	schemes, err := catalogSchemes()
	require.NoError(t, err)
	require.NotEmpty(t, schemes)

	dracula := findScheme(schemes, "Dracula")
	require.NotNil(t, dracula)

	got, err := Resolve("Dracula")
	require.NoError(t, err)
	require.Equal(t, dracula.Background, got.Background)
	require.Equal(t, dracula.ANSI[0], got.ANSI[0])
}

func TestResolveCaseInsensitive(t *testing.T) {
	t.Parallel()
	got, err := Resolve("DRACULA")
	require.NoError(t, err)

	schemes, err := catalogSchemes()
	require.NoError(t, err)
	require.Equal(t, findScheme(schemes, "Dracula").Background, got.Background)
}

func TestResolveUnknown(t *testing.T) {
	t.Parallel()
	_, err := Resolve("not-a-theme")
	require.Error(t, err)
}

func TestResolveConfiguredFallback(t *testing.T) {
	t.Parallel()
	fallback := styles.CharmtonePantera()

	got := ResolveConfigured("", fallback)
	require.Equal(t, fallback.Background, got.Background)

	got = ResolveConfigured("not-a-theme", fallback)
	require.Equal(t, fallback.Background, got.Background)
}

func TestCatalogNotEmpty(t *testing.T) {
	t.Parallel()
	schemes := Catalog()
	require.NotEmpty(t, schemes)

	names := make(map[string]bool, len(schemes))
	for _, s := range schemes {
		require.NotEmpty(t, s.Name)
		require.False(t, names[s.Name], "duplicate scheme name %q", s.Name)
		names[s.Name] = true
	}
}

func findScheme(schemes []Scheme, name string) *Scheme {
	for i := range schemes {
		if schemes[i].Name == name {
			return &schemes[i]
		}
	}
	return nil
}
