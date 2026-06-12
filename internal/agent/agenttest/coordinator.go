// Package agenttest provides test-only constructors for wiring a real
// production agent.Coordinator without booting a full app.App. It is
// imported only from _test.go files (e.g. internal/backend integration
// tests) and is never referenced by production code, so it is compiled
// only under tests and never ships in the production binary or API.
package agenttest

import (
	"context"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy/providers/openaicompat"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
)

// NewCoordinator builds a real agent.Coordinator through the production
// agent.NewCoordinator constructor so the RunAccepted / BeginAccepted /
// run path (including UpdateModels) is the actual code under test.
//
// It installs a minimal config with a single openai-compatible provider
// whose model resolves offline. run rebuilds the model on every call, so
// the provider must construct without network I/O; the cancel-on-entry
// path this helper is built to exercise returns before any model call,
// so no request is ever issued. The coder agent's allowed-tools list is
// cleared to keep tool construction cheap and free of sub-agent wiring.
//
// The optional coordinator dependencies (history, filetracker, LSP,
// notify, runComplete, skills) are nil: run guards the publisher fields
// and the cancel-on-entry path never touches the others.
func NewCoordinator(
	ctx context.Context,
	workingDir string,
	sessions session.Service,
	messages message.Service,
) (agent.Coordinator, error) {
	cfg, err := config.Init(workingDir, "", false)
	if err != nil {
		return nil, err
	}

	const (
		providerID = "test-openai-compat"
		modelID    = "test-model"
	)
	cfg.Config().Providers.Set(providerID, config.ProviderConfig{
		ID:      providerID,
		Name:    "Test",
		Type:    openaicompat.Name,
		BaseURL: "http://127.0.0.1:0/v1",
		APIKey:  "test",
		Models:  []catwalk.Model{{ID: modelID, DefaultMaxTokens: 4096}},
	})
	selected := config.SelectedModel{Provider: providerID, Model: modelID}
	cfg.OverridePreferredModel(config.SelectedModelTypeLarge, selected)
	cfg.OverridePreferredModel(config.SelectedModelTypeSmall, selected)
	cfg.SetupAgents()

	// Keep buildTools light: no sub-agent or agentic-fetch construction.
	coderCfg := cfg.Config().Agents[config.AgentCoder]
	coderCfg.AllowedTools = nil
	cfg.Config().Agents[config.AgentCoder] = coderCfg

	return agent.NewCoordinator(ctx, agent.CoordinatorOptions{
		Config:      cfg,
		Sessions:    sessions,
		Messages:    messages,
		Permissions: permission.NewPermissionService(workingDir, true, nil),
	})
}
