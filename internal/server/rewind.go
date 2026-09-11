package server

import (
	"encoding/json"
	"github.com/charmbracelet/crush/internal/filehistory"
	"net/http"
)

func (c *controllerV1) handleRewind(w http.ResponseWriter, r *http.Request) {
	ws, err := c.backend.GetWorkspace(r.PathValue("id"))
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	var request filehistory.Request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&request); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid rewind request")
		return
	}
	result, err := ws.Rewind(r.Context(), r.PathValue("sid"), request)
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	jsonEncode(w, result)
}
