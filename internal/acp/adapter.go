package acp

import (
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
)

// AppAdapter wraps app.App (or a compatible struct) to satisfy the acp.App
// interface without importing the app package (which would create a cycle).
type AppAdapter struct {
	sessions    session.Service
	messages    message.Service
	coordinator agent.Coordinator
	permissions permission.Service
	cfg         *config.ConfigStore
}

// NewAppAdapter wraps the necessary services to satisfy the acp.App interface.
func NewAppAdapter(
	sessions session.Service,
	messages message.Service,
	coordinator agent.Coordinator,
	permissions permission.Service,
	cfg *config.ConfigStore,
) *AppAdapter {
	return &AppAdapter{
		sessions:    sessions,
		messages:    messages,
		coordinator: coordinator,
		permissions: permissions,
		cfg:         cfg,
	}
}

func (a *AppAdapter) GetSessions() session.Service       { return a.sessions }
func (a *AppAdapter) GetMessages() message.Service       { return a.messages }
func (a *AppAdapter) GetCoordinator() agent.Coordinator  { return a.coordinator }
func (a *AppAdapter) GetPermissions() permission.Service { return a.permissions }
func (a *AppAdapter) GetConfig() *config.ConfigStore     { return a.cfg }
