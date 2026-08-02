package chat

import (
	"encoding/json"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/joestump-agent/a2tea"
	"github.com/joestump-agent/a2tea/render"
)

// syncToolA2UISurfaces builds — or reuses — the live a2tea models for the
// surfaces carried in the tool result's metadata. Models are keyed by
// a hash of the metadata: a re-render with the same result reuses the live
// models (preserving focus and edited field values), while a result swap
// (SetResult) rebuilds them. Each surface's provenance is registered so a
// later interaction routes back to the owning MCP server.
func (t *baseToolMessageItem) syncToolA2UISurfaces() {
	if t.result == nil || t.result.Metadata == "" {
		t.a2ui.surfaces = nil
		t.a2ui.surfaceIDs = nil
		t.surfaceScanned = false
		return
	}
	var meta tools.ReadMCPResourceResponseMetadata
	if err := json.Unmarshal([]byte(t.result.Metadata), &meta); err != nil || len(meta.A2UISurfaces) == 0 {
		t.a2ui.surfaces = nil
		t.a2ui.surfaceIDs = nil
		t.surfaceScanned = false
		return
	}
	h := fnv64(t.result.Metadata)
	if t.surfaceScanned && t.surfaceSrcHash == h {
		return
	}
	var surfaces []render.Model
	var ids []string
	failed := 0
	for i, surfJSON := range meta.A2UISurfaces {
		parts, err := a2tea.Scan(surfJSON)
		if err != nil {
			failed++
			continue
		}
		built, builtIDs := buildA2UISurfaces(t.sty, parts)
		payloadLive := 0
		for j, m := range built {
			if m == nil {
				continue
			}
			payloadLive++
			surfaces = append(surfaces, m)
			id := builtIDs[j]
			ids = append(ids, id)
			// Provenance is parallel to A2UISurfaces; a missing or short
			// slice (older persisted results) means unknown origin.
			if i < len(meta.MCPSurfaceProvenance) {
				registerA2UISurfaceProvenance(id, meta.MCPSurfaceProvenance[i])
			}
		}
		// A payload that scanned but drew nothing (unsupported components,
		// bare data-model update) failed just like a scan error: the model
		// was told the user can see it.
		if payloadLive == 0 {
			failed++
		}
	}
	t.a2ui.surfaces = surfaces
	t.a2ui.surfaceIDs = ids
	t.surfaceBuildFailed = failed
	t.surfaceSrcHash = h
	t.surfaceScanned = true
	if t.focusableMessageItem.isFocused() {
		t.focusToolA2UISurfaces()
	}
}

// hasToolA2UISurfaces reports whether this item carries live MCP surfaces.
func (t *baseToolMessageItem) hasToolA2UISurfaces() bool {
	return t.a2ui.hasLive()
}

// focusToolA2UISurfaces grants focus to the first live surface and blurs the
// rest. MCP surfaces are long-lived app UIs — unlike chat-scanned
// forms they are never retired on submission, so every live surface is
// focusable.
func (t *baseToolMessageItem) focusToolA2UISurfaces() {
	for i, s := range t.a2ui.surfaces {
		if s != nil {
			t.a2ui.focus(i)
			return
		}
	}
}

// blurToolA2UISurfaces revokes keyboard focus from every live surface.
func (t *baseToolMessageItem) blurToolA2UISurfaces() {
	t.a2ui.blurAll()
}

// renderToolA2UISurfaces renders each live surface from its model at the
// given width, joined by blank lines. It also reports how many surfaces
// failed — payloads that never built a model (surfaceBuildFailed) plus live
// models that drew nothing — so the renderer can show the same alert the
// static path shows: the model was told the user can see every surface.
func (t *baseToolMessageItem) renderToolA2UISurfaces(width int) (string, int) {
	var b strings.Builder
	failed := t.surfaceBuildFailed
	for _, s := range t.a2ui.surfaces {
		if s == nil {
			continue
		}
		s.SetSize(width, 0)
		rendered := strings.TrimRight(s.View().Content, "\n")
		if rendered == "" {
			failed++
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(rendered)
	}
	return b.String(), failed
}

// HandleKeyEvent routes keys to the focused live MCP surface first
// — Tab/Shift+Tab cycle its focus ring, Enter activates a button,
// printable keys edit a focused field (see a2uiSurfaceWantsKey) — then falls
// through to the copy shortcut. A button activation surfaces as an
// a2uievent.ButtonClicked tea.Cmd, which the UI model routes per the
// surface's provenance.
func (t *baseToolMessageItem) handleA2UIKeyEvent(key tea.KeyMsg) (bool, tea.Cmd) {
	if idx := t.a2ui.focusedIndex(); idx >= 0 && a2uiSurfaceWantsKey(t.a2ui.surfaces[idx], key) {
		t.Bump()
		return true, t.a2ui.update(idx, key)
	}
	return false, nil
}
