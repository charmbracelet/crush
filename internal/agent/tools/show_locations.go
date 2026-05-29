package tools

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/taigrr/fantasy"

	"github.com/taigrr/crush/internal/editor"
)

const ShowLocationsToolName = "show_locations"

//go:embed show_locations.md
var showLocationsDescription string

// ShowLocationsParams matches the shape neocrush.nvim's picker expects.
type ShowLocationsParams struct {
	Title string              `json:"title,omitempty" description:"Optional picker title"`
	Items []ShowLocationsItem `json:"items" description:"List of locations to display"`
}

// ShowLocationsItem is a single picker entry.
type ShowLocationsItem struct {
	Filename string `json:"filename" description:"Absolute or workspace-relative path"`
	Line     int    `json:"lnum" description:"1-indexed line number"`
	Col      int    `json:"col,omitempty" description:"1-indexed column (default 1)"`
	Text     string `json:"text" description:"Code snippet at this location"`
	Note     string `json:"note" description:"Why this location is relevant; shown in the explanation pane"`
	Type     string `json:"type,omitempty" description:"N=note (default), I=info, W=warning, E=error"`
}

// NewShowLocationsTool returns the show_locations tool, backed by the
// supplied bridge.
func NewShowLocationsTool(bridge editor.Bridge) fantasy.AgentTool {
	if bridge == nil {
		bridge = editor.Noop{}
	}
	return fantasy.NewAgentTool(
		ShowLocationsToolName,
		showLocationsDescription,
		func(ctx context.Context, params ShowLocationsParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if len(params.Items) == 0 {
				return fantasy.NewTextErrorResponse("show_locations requires at least one item"), nil
			}
			items := make([]editor.Location, len(params.Items))
			for i, it := range params.Items {
				items[i] = editor.Location{
					Filename: it.Filename,
					Line:     it.Line,
					Col:      it.Col,
					Text:     it.Text,
					Note:     it.Note,
					Type:     it.Type,
				}
			}
			if err := bridge.ShowLocations(ctx, params.Title, items); err != nil {
				if errors.Is(err, editor.ErrUnavailable) {
					return fantasy.NewTextErrorResponse("Editor bridge is not available; no editor is attached."), nil
				}
				return fantasy.NewTextErrorResponse(fmt.Sprintf("show_locations failed: %v", err)), nil
			}
			return fantasy.NewTextResponse(fmt.Sprintf("Displayed %d location(s) in editor.", len(items))), nil
		},
	)
}
