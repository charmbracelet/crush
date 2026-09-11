// Package notebooktools provides the notebook retrieval tools (recall
// and notebook_search) that let the model query the per-event context
// notebook.
package notebooktools

import (
	"charm.land/fantasy"
	notebooktool "github.com/charmbracelet/crush/internal/agent/tools/notebook"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/notebook"
)

// Build returns the notebook retrieval tools for the given service.
// When mem0 sync is enabled, the recall tool supports cross-session
// search via the "cross:" prefix.
func Build(svc notebook.Service, messages message.Service, cfg *config.ConfigStore, mem0Server string, syncMem0 bool) []fantasy.AgentTool {
	return []fantasy.AgentTool{
		notebooktool.NewRecallTool(svc, messages, cfg, mem0Server, syncMem0),
		notebooktool.NewSearchTool(svc),
	}
}
