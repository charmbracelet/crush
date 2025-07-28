package agent

import (
	"context"

	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
)

func NewCoderTools(
	ctx context.Context,
	cwd string,
	sessions session.Service,
	messages message.Service,
	permissions permission.Service,
	lspClients map[string]*lsp.Client,
	history history.Service,
	mcps map[string]MCPConfig,
) tools.Registry {
	toolFn := func() []tools.BaseTool {
		allTools := []tools.BaseTool{
			tools.NewBashTool(permissions, cwd),
			tools.NewDownloadTool(permissions, cwd),
			tools.NewEditTool(lspClients, permissions, history, cwd),
			tools.NewFetchTool(permissions, cwd),
			tools.NewGlobTool(cwd),
			tools.NewGrepTool(cwd),
			tools.NewLsTool(cwd),
			tools.NewSourcegraphTool(),
			tools.NewViewTool(lspClients, cwd),
			tools.NewWriteTool(lspClients, permissions, history, cwd),
		}
		mcpTools := GetMCPTools(ctx, cwd, mcps, permissions)
		allTools = append(allTools, mcpTools...)
		if len(lspClients) > 0 {
			allTools = append(allTools, tools.NewDiagnosticsTool(lspClients))
		}
		return allTools
	}
	return tools.NewRegistry(toolFn)
}

func NewTaskTools(cwd string) tools.Registry {
	return tools.NewRegistryFromTools([]tools.BaseTool{
		tools.NewGlobTool(cwd),
		tools.NewGrepTool(cwd),
		tools.NewLsTool(cwd),
		tools.NewSourcegraphTool(),
		// no need for LSP info here
		tools.NewViewTool(map[string]*lsp.Client{}, cwd),
	})
}
