package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"log/slog"
	"strings"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Tool = mcp.Tool

var allTools = csync.NewMap[string, []*Tool]()

// Tools returns all available MCP tools.
func Tools() iter.Seq2[string, []*Tool] {
	return allTools.Seq2()
}

// RunTool runs an MCP tool with the given input parameters.
func RunTool(ctx context.Context, name, toolName string, input string) (string, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("error parsing parameters: %s", err)
	}

	c, err := getOrRenewClient(ctx, name)
	if err != nil {
		return "", err
	}
	result, err := c.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		return "", err
	}

	output := make([]string, 0, len(result.Content))
	for _, v := range result.Content {
		if vv, ok := v.(*mcp.TextContent); ok {
			output = append(output, vv.Text)
		} else {
			output = append(output, fmt.Sprintf("%v", v))
		}
	}
	return strings.Join(output, "\n"), nil
}

// RefreshTools gets the updated list of tools from the MCP and updates the
// global state.
func RefreshTools(ctx context.Context, name string) {
	session, ok := sessions.Get(name)
	if !ok {
		slog.Warn("refresh tools: no session", "name", name)
		return
	}

	tools, err := getTools(ctx, session)
	if err != nil {
		updateState(name, StateError, err, nil, Counts{})
		return
	}

	updateTools(name, tools)

	prev, _ := states.Get(name)
	prev.Counts.Tools = len(tools)
	updateState(name, StateConnected, nil, session, prev.Counts)
}

func getTools(ctx context.Context, session *mcp.ClientSession) ([]*Tool, error) {
	if session.InitializeResult().Capabilities.Tools == nil {
		return nil, nil
	}
	result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, err
	}
	return result.Tools, nil
}

func updateTools(name string, tools []*Tool) {
	if len(tools) == 0 {
		allTools.Del(name)
		return
	}
	allTools.Set(name, tools)
}
