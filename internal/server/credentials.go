package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/proto"
)

// credentialsAllowed reports whether the listener may accept credentials.
// The TCP API is unauthenticated, so it must never take them; every other
// transport (unix socket, named pipe, and later ones) is assumed to be
// reachable only by its owner.
func (c *controllerV1) credentialsAllowed(w http.ResponseWriter) bool {
	// tcp, tcp4 and tcp6 are all the same unauthenticated API.
	if c.server != nil && strings.HasPrefix(c.server.network, "tcp") {
		jsonError(w, http.StatusForbidden, "credentials cannot be pushed over a TCP listener")
		return false
	}
	return true
}

// handlePutCredential installs a managed credential for a provider. The
// token is never logged.
func (c *controllerV1) handlePutCredential(w http.ResponseWriter, r *http.Request) {
	if !c.credentialsAllowed(w) {
		return
	}
	var req proto.CredentialPush
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "failed to decode request")
		return
	}
	if strings.TrimSpace(req.AccessToken) == "" {
		jsonError(w, http.StatusBadRequest, "access_token is required")
		return
	}
	var expiresAt time.Time
	if req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "expires_at must be RFC 3339")
			return
		}
		expiresAt = t
	}
	kind, err := config.ParseManagedKind(req.Kind)
	if err != nil {
		jsonError(w, http.StatusBadRequest, `kind must be "oauth" or "api-key"`)
		return
	}
	providerID := r.PathValue("provider")
	err = c.backend.SetManagedCredential(r.Context(), providerID, config.ManagedCredential{
		AccessToken: req.AccessToken,
		ExpiresAt:   expiresAt,
		Kind:        kind,
	})
	if errors.Is(err, backend.ErrUnknownProvider) {
		jsonError(w, http.StatusNotFound, "unknown provider")
		return
	}
	if errors.Is(err, config.ErrUnsupportedManagedKind) {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteCredential drops a managed credential.
func (c *controllerV1) handleDeleteCredential(w http.ResponseWriter, r *http.Request) {
	if !c.credentialsAllowed(w) {
		return
	}
	providerID := r.PathValue("provider")
	err := c.backend.DeleteManagedCredential(r.Context(), providerID)
	if errors.Is(err, backend.ErrUnknownProvider) {
		jsonError(w, http.StatusNotFound, "provider has no managed credential")
		return
	}
	if err != nil {
		c.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
