package agent

import (
	"context"

	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/llm/prompt"
	"github.com/charmbracelet/crush/internal/llm/provider"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
)

func NewCoderAgent(
	ctx context.Context,
	cwd string,
	providers map[string]provider.Config,
	smallModel Model,
	largeModel Model,
	contextFiles []string,
	sessions session.Service,
	messages message.Service,
	permissions permission.Service,
	lspClients map[string]*lsp.Client,
	history history.Service,
	mcps map[string]MCPConfig,
) (Service, error) {
	systemPrompt := prompt.CoderPrompt(cwd, contextFiles...)
	tools := NewCoderTools(
		ctx,
		cwd,
		sessions,
		messages,
		permissions,
		lspClients,
		history,
		mcps,
	)

	return NewAgent(
		ctx,
		cwd,
		systemPrompt,
		tools,
		providers,
		smallModel,
		largeModel,
		sessions,
		messages,
	)
}
