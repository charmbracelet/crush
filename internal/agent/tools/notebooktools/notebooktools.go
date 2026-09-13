// Package notebooktools provides the notebook retrieval tools (recall
// and notebook_search) that let the model query the per-event context
// notebook.
package notebooktools

import (
	"charm.land/fantasy"
	notebooktool "github.com/charmbracelet/crush/internal/agent/tools/notebook"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/notebook"
)

// Build returns the notebook retrieval tools for the given service.
// When mem0 sync is enabled, the recall tool supports cross-session
// search via the "cross:" prefix. stats, when non-nil, accumulates
// per-session recall counts for sufficiency telemetry.
func Build(svc notebook.Service, messages message.Service, cfg *config.ConfigStore, mem0Server string, syncMem0 bool, stats *csync.Map[string, notebook.Stats]) []fantasy.AgentTool {
	return []fantasy.AgentTool{
		notebooktool.NewRecallTool(svc, messages, cfg, mem0Server, syncMem0, stats),
		notebooktool.NewSearchTool(svc),
	}
}
