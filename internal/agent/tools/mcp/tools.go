package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strings"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Tool = mcp.Tool

var (
	allTools     = csync.NewMap[string, *Tool]()
	client2Tools = csync.NewMap[string, []*Tool]()
)

// GetTools returns all available MCP tools.
func GetTools() iter.Seq2[string, *Tool] {
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

// updateTools updates the global mcpTools and mcpClient2Tools maps
func updateTools(mcpName string, tools []*Tool) {
	if len(tools) == 0 {
		client2Tools.Del(mcpName)
	} else {
		client2Tools.Set(mcpName, tools)
	}
	for name, tools := range client2Tools.Seq2() {
		for _, t := range tools {
			allTools.Set(name, t)
		}
	}
}
