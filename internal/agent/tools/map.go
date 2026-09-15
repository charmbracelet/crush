package tools

import (
	"context"
	_ "embed"
	"log/slog"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/index"
)

const MapToolName = "map"

//go:embed map.md
var mapDescription string

// MapParams selects the query mode. All fields are optional; an empty
// call renders the ranked project skeleton.
type MapParams struct {
	Path      string `json:"path,omitempty" description:"List indexed files and top-level symbols under this directory (project-relative), e.g. internal/agent"`
	Symbol    string `json:"symbol,omitempty" description:"Find where a symbol is defined and which files reference it"`
	Semantic  string `json:"semantic,omitempty" description:"Reserved for a future semantic index — not yet implemented; returns guidance to use symbol=, path=, grep, or glob"`
	MaxTokens int    `json:"max_tokens,omitempty" description:"Cap on response size in approximate tokens, default 500"`
}

// NewMapTool exposes the persistent project index. The shared Service
// opens the sidecar database lazily on first call; the index build
// runs in the background, so early calls return partial results. The
// tool itself stays read-only against the project tree.
func NewMapTool(cfg *config.ConfigStore) fantasy.AgentTool {
	svc := index.Shared(
		cfg.Config().Options.DataDirectory,
		cfg.WorkingDir(),
	)
	return fantasy.NewAgentTool(
		MapToolName,
		mapDescription,
		func(ctx context.Context, params MapParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if err := svc.Ready(); err != nil {
				slog.Warn("Project index unavailable", "error", err)
				return fantasy.NewTextErrorResponse("project index unavailable — fall back to grep/glob"), nil
			}

			var (
				out string
				err error
			)
			switch {
			case params.Semantic != "":
				return fantasy.NewTextResponse("Semantic map search is not enabled: it requires the semantic index option (v2, not built). Use symbol=, path=, grep, or glob instead."), nil
			case params.Symbol != "":
				out, err = svc.Symbol(ctx, params.Symbol, params.MaxTokens)
			case params.Path != "":
				out, err = svc.Subtree(ctx, params.Path, params.MaxTokens)
			default:
				out, err = svc.Skeleton(ctx, params.MaxTokens)
			}
			if err != nil {
				return fantasy.NewTextErrorResponse("map query failed: " + err.Error()), nil
			}
			return fantasy.NewTextResponse(out), nil
		},
	)
}
