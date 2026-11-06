package server

import (
	"net/http"

	"github.com/charmbracelet/crush/internal/apigen"
)

// apiInfo is the top-level metadata for the Crush API documentation.
var apiInfo = apigen.Info{
	Title:   "Crush API",
	Version: "1.0",
}

// apiBase is the base path shared by all v1 endpoints.
const apiBase = "/v1"

// handleDocsIndex serves the plain HTML documentation page.
func (c *controllerV1) handleDocsIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != apiBase+"/docs/" {
		http.Redirect(w, r, apiBase+"/docs/", http.StatusMovedPermanently)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := apigen.RenderDocs(w, apiInfo, apiBase, c.endpoints()); err != nil {
		c.server.logError(r, "Failed to render docs", "error", err)
		jsonError(w, http.StatusInternalServerError, "failed to render docs")
	}
}

// handleDocsSpec serves the OpenAPI 3.1 specification as JSON.
func (c *controllerV1) handleDocsSpec(w http.ResponseWriter, r *http.Request) {
	data, err := apigen.SpecJSON(apiInfo, apiBase, c.endpoints())
	if err != nil {
		c.server.logError(r, "Failed to render spec", "error", err)
		jsonError(w, http.StatusInternalServerError, "failed to render spec")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}
