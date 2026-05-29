package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/taigrr/fantasy"

	"github.com/taigrr/crush/internal/editor"
)

const EditorContextToolName = "editor_context"

//go:embed editor_context.md
var editorContextDescription string

// EditorContextParams is intentionally empty: the editor is the source of
// truth, all state is pulled at call time.
type EditorContextParams struct{}

// NewEditorContextTool returns the editor_context tool, backed by the
// supplied bridge. When the bridge is unavailable the tool surfaces a
// clear error so the model can adapt rather than retry blindly.
func NewEditorContextTool(bridge editor.Bridge) fantasy.AgentTool {
	if bridge == nil {
		bridge = editor.Noop{}
	}
	return fantasy.NewAgentTool(
		EditorContextToolName,
		editorContextDescription,
		func(ctx context.Context, _ EditorContextParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			ec, err := bridge.Context(ctx)
			if err != nil {
				if errors.Is(err, editor.ErrUnavailable) {
					return fantasy.NewTextErrorResponse("Editor bridge is not available; no editor is attached."), nil
				}
				return fantasy.NewTextErrorResponse(fmt.Sprintf("editor_context failed: %v", err)), nil
			}
			out, err := json.Marshal(ec)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("marshal editor context: %w", err)
			}
			return fantasy.NewTextResponse(string(out)), nil
		},
	)
}
