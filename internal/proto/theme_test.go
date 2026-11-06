package proto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHostThemeEqual(t *testing.T) {
	t.Parallel()

	base := func() *HostTheme {
		return &HostTheme{
			Name:        "n",
			Source:      "s",
			Base:        "b",
			Palette:     map[string]string{"primary": "#6B50FF"},
			Syntax:      map[string]string{"keyword": "#00A4FF"},
			Transparent: new(true),
		}
	}
	require.True(t, (*HostTheme)(nil).Equal(nil))
	require.False(t, base().Equal(nil))
	require.False(t, (*HostTheme)(nil).Equal(base()))
	require.True(t, base().Equal(base()))
	require.True(t, (&HostTheme{}).Equal(&HostTheme{Palette: map[string]string{}, Syntax: map[string]string{}}))

	for name, change := range map[string]func(*HostTheme){
		"name":              func(h *HostTheme) { h.Name = "x" },
		"source":            func(h *HostTheme) { h.Source = "x" },
		"base":              func(h *HostTheme) { h.Base = "x" },
		"palette value":     func(h *HostTheme) { h.Palette["primary"] = "#000000" },
		"palette key":       func(h *HostTheme) { h.Palette["accent"] = "#000000" },
		"syntax":            func(h *HostTheme) { h.Syntax["keyword"] = "#000000" },
		"transparent value": func(h *HostTheme) { h.Transparent = new(false) },
		"transparent unset": func(h *HostTheme) { h.Transparent = nil },
	} {
		h := base()
		change(h)
		require.False(t, base().Equal(h), name)
		require.False(t, h.Equal(base()), name)
	}
}

func TestHostThemeClone(t *testing.T) {
	t.Parallel()

	require.Nil(t, (*HostTheme)(nil).Clone())
	orig := &HostTheme{Name: "n", Palette: map[string]string{"primary": "#6B50FF"}, Transparent: new(true)}
	c := orig.Clone()
	require.True(t, orig.Equal(c))
	c.Palette["primary"] = "#000000"
	*c.Transparent = false
	require.Equal(t, "#6B50FF", orig.Palette["primary"])
	require.True(t, *orig.Transparent)
}

func TestHostThemeJSON(t *testing.T) {
	t.Parallel()

	var req HostThemeRequest
	require.NoError(t, json.Unmarshal([]byte(`{"theme":{"name":"charmtone-dark","source":"superjoy","base":"",`+
		`"palette":{"primary":"#6B50FF"},"syntax":{"keyword":"#00A4FF"},"transparent":false}}`), &req))
	require.Equal(t, "charmtone-dark", req.Theme.Name)
	require.NotNil(t, req.Theme.Transparent)
	require.False(t, *req.Theme.Transparent)

	require.NoError(t, json.Unmarshal([]byte(`{"theme":null}`), &req))
	require.Nil(t, req.Theme)

	data, err := json.Marshal(HostThemeState{WorkspaceID: "w"})
	require.NoError(t, err)
	require.JSONEq(t, `{"workspace_id":"w","theme":null}`, string(data))
}
