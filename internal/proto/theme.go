package proto

import "maps"

// HostTheme is a theme supplied by a host application that runs Crush in
// one of its panes (an editor, for example). It is written in Crush's own
// vocabulary, so any host can send one: Palette keys are theme-file keys
// (see styles.PaletteFields) and Syntax keys name code highlighting tokens
// (see styles.SyntaxFields). Colors the host leaves out come from Base.
// Unknown keys are ignored, so a host can target newer Crush versions.
//
// A host theme is never persisted: it lives in the server's memory for as
// long as the workspace does, or until the host clears it.
type HostTheme struct {
	// Name is the host's name for the theme. It is informational only.
	Name string `json:"name,omitempty"`
	// Source names the host application. It is informational only.
	Source string `json:"source,omitempty"`
	// Base is the Crush built-in or user theme that supplies every color
	// the host leaves out. Empty means the built-in charmtone-panther.
	Base string `json:"base,omitempty"`
	// Palette maps theme-file keys (such as "primary" or "bg_base") to
	// colors: "#rgb", "#rrggbb", an ANSI index (0-255) or a CharmTone name.
	Palette map[string]string `json:"palette,omitempty"`
	// Syntax maps code highlighting token keys (such as "keyword" or
	// "literal_string") to foreground colors, in the same formats.
	Syntax map[string]string `json:"syntax,omitempty"`
	// Transparent, when set, overrides Crush's own options.tui.transparent
	// while the host theme is applied. Nil keeps Crush's setting.
	Transparent *bool `json:"transparent,omitempty"`
}

// Equal reports whether t and o describe the same theme. Two nil themes
// are equal, and a nil map equals an empty one.
func (t *HostTheme) Equal(o *HostTheme) bool {
	if t == nil || o == nil {
		return t == o
	}
	if (t.Transparent == nil) != (o.Transparent == nil) ||
		(t.Transparent != nil && *t.Transparent != *o.Transparent) {
		return false
	}
	return t.Name == o.Name &&
		t.Source == o.Source &&
		t.Base == o.Base &&
		maps.Equal(t.Palette, o.Palette) &&
		maps.Equal(t.Syntax, o.Syntax)
}

// Clone returns a deep copy of t, or nil when t is nil.
func (t *HostTheme) Clone() *HostTheme {
	if t == nil {
		return nil
	}
	c := *t
	c.Palette = maps.Clone(t.Palette)
	c.Syntax = maps.Clone(t.Syntax)
	if t.Transparent != nil {
		c.Transparent = new(*t.Transparent)
	}
	return &c
}

// HostThemeRequest is the body of POST /v1/workspaces/{id}/theme. A nil
// Theme clears the host theme.
type HostThemeRequest struct {
	Theme *HostTheme `json:"theme"`
}

// HostThemeState is a workspace's host theme. It is the response of the
// theme endpoints and the payload of host_theme events (an updated event
// when a theme is set, a deleted one when it is cleared). A nil Theme
// means no host theme is set.
type HostThemeState struct {
	WorkspaceID string     `json:"workspace_id"`
	Theme       *HostTheme `json:"theme"`
}
