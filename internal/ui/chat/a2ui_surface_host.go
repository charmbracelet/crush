package chat

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/joestump-agent/a2tea"
	"github.com/joestump-agent/a2tea/render"
)

// a2uiSurfaceHost holds the live a2tea surface models for one message item —
// an assistant message whose content scanned as A2UI, or a tool result
// that delivered an MCP A2UI surface. The slice is indexed by
// scan-part (assistant) or surface (tool result); entries without a
// renderable surface hold nil.
//
// Assistant surfaces stream and rebuild as deltas arrive, so the assistant
// item keys its models by source hash and retires them on first
// submission. MCP tool-result surfaces are long-lived app UIs: they build once
// from a fixed payload, are never retired on interaction, and feed a button
// back to the owning server. The host carries the shared state and
// behavior both kinds need; the items own their lifecycle.
type a2uiSurfaceHost struct {
	// surfaces holds the live a2tea models so they can receive focus and
	// key input instead of being frozen to a rendered string.
	surfaces []render.Model
	// surfaceIDs holds the A2UI surface ID for each entry (empty where the
	// part has no renderable surface), so a ButtonClicked event's
	// SurfaceID can be routed back to the model that emitted it.
	surfaceIDs []string
	// surfaceOwners holds the MCP server that served each entry (empty for
	// chat-scanned surfaces). It is the item-scoped provenance the action
	// round-trip resolves through, so two servers using the same surface ID
	// cannot route each other's clicks.
	surfaceOwners []string
	// sty themes the surfaces with the crush palette.
	sty *styles.Styles
}

// ownerFor returns the MCP server that served the surface at index i, and
// whether one is known.
func (h *a2uiSurfaceHost) ownerFor(i int) (string, bool) {
	if i < 0 || i >= len(h.surfaceOwners) {
		return "", false
	}
	name := h.surfaceOwners[i]
	return name, name != ""
}

// drop releases the surface at index i, which the server deleted. The entry
// is kept (indexes stay parallel to surfaceIDs) but nils out, so hasLive and
// findByID stop reporting it and it can no longer take focus or keys.
func (h *a2uiSurfaceHost) drop(i int) {
	if i < 0 || i >= len(h.surfaces) {
		return
	}
	h.surfaces[i] = nil
}

// buildSurfaces renders each part's messages into a live a2tea model,
// returning the surfaces and their IDs. Parts with no messages or nothing
// to draw (e.g. a bare data-model update) leave nil/empty entries.
func buildA2UISurfaces(sty *styles.Styles, parts []a2tea.Part) ([]render.Model, []string) {
	surfaces := make([]render.Model, len(parts))
	ids := make([]string, len(parts))
	for i, p := range parts {
		if len(p.Messages) == 0 {
			continue
		}
		model, err := a2tea.Render(p.Messages, render.WithStyles(a2uiThemeStyles(sty)))
		if err != nil {
			continue
		}
		if rm, ok := model.(render.Model); ok {
			surfaces[i] = rm
			ids[i] = a2uiPartSurfaceID(p.Messages)
		}
	}
	return surfaces, ids
}

// hasLive reports whether the host holds at least one live surface model.
// While true, the item's content-hash render caches are bypassed so surface
// interaction is never served a frozen frame.
func (h *a2uiSurfaceHost) hasLive() bool {
	for _, s := range h.surfaces {
		if s != nil {
			return true
		}
	}
	return false
}

// surfaceAt returns the live surface at index i, or nil.
func (h *a2uiSurfaceHost) surfaceAt(i int) render.Model {
	if i < 0 || i >= len(h.surfaces) {
		return nil
	}
	return h.surfaces[i]
}

// focusedIndex returns the index of the surface currently holding keyboard
// focus, or -1 when none does.
func (h *a2uiSurfaceHost) focusedIndex() int {
	for i, s := range h.surfaces {
		if s != nil && s.Focused() {
			return i
		}
	}
	return -1
}

// update forwards msg into surface i's Update and stores the returned model.
func (h *a2uiSurfaceHost) update(i int, msg tea.Msg) tea.Cmd {
	model, cmd := h.surfaces[i].Update(msg)
	if rm, ok := model.(render.Model); ok {
		h.surfaces[i] = rm
	}
	return cmd
}

// fieldValues reads the surface's current values, if it supports them.
func (h *a2uiSurfaceHost) fieldValues(i int) map[string]any {
	s := h.surfaceAt(i)
	if s == nil {
		return nil
	}
	if fv, ok := s.(interface{ FieldValues() map[string]any }); ok {
		return fv.FieldValues()
	}
	return nil
}

// surfaceIDFor returns the A2UI surface ID at index i.
func (h *a2uiSurfaceHost) surfaceIDFor(i int) string {
	if i < 0 || i >= len(h.surfaceIDs) {
		return ""
	}
	return h.surfaceIDs[i]
}

// findByID returns the index of the surface with the given A2UI surface ID,
// or -1.
func (h *a2uiSurfaceHost) findByID(surfaceID string) int {
	for i, id := range h.surfaceIDs {
		if id == surfaceID && h.surfaceAt(i) != nil {
			return i
		}
	}
	return -1
}

// focus grants keyboard focus to the surface at index i and blurs the rest,
// honoring the a2tea composition contract of at most one focused child.
func (h *a2uiSurfaceHost) focus(i int) {
	for j, s := range h.surfaces {
		if s == nil {
			continue
		}
		if j == i {
			_ = s.Focus()
		} else {
			s.Blur()
		}
	}
}

// blurAll revokes keyboard focus from every live surface.
func (h *a2uiSurfaceHost) blurAll() {
	for _, s := range h.surfaces {
		if s != nil {
			s.Blur()
		}
	}
}

// --- MCP surface provenance registry ---

// a2uiMCPProvenance maps a rendered A2UI surface ID to the MCP server that
// served it, so a later interaction event (button press, render failure)
// can route back to the owning server as an a2ui_action / a2ui_error tool
// call. Entries live for the process lifetime: surface IDs are
// server-scoped (typically "default"), so re-rendering a surface from the
// same server simply overwrites the same key. Two servers emitting the SAME
// surface ID would clobber each other here — the action round-trip should
// resolve provenance through the owning item before falling back to this
// registry.
var a2uiMCPProvenance = csync.NewMap[string, string]()

// registerA2UISurfaceProvenance records that surfaceID came from mcpName.
// An empty surfaceID or mcpName is not registered.
func registerA2UISurfaceProvenance(surfaceID, mcpName string) {
	if surfaceID == "" || mcpName == "" {
		return
	}
	a2uiMCPProvenance.Set(surfaceID, mcpName)
}

// A2UISurfaceProvenance returns the MCP server that served surfaceID, and
// whether the surface is MCP-owned at all.
func A2UISurfaceProvenance(surfaceID string) (string, bool) {
	return a2uiMCPProvenance.Get(surfaceID)
}
