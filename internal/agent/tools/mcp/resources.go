package mcp

import (
	"context"
	"iter"
	"log/slog"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type (
	Resource         = mcp.Resource
	ResourceContents = mcp.ResourceContents
)

var allResources = csync.NewMap[string, []*Resource]()

// Resources returns all available MCP resources.
func Resources() iter.Seq2[string, []*Resource] {
	return allResources.Seq2()
}

// ReadResource retrieves the content of an MCP resource with the given arguments.
func ReadResource(ctx context.Context, clientName, uri string) ([]*ResourceContents, error) {
	c, err := getOrRenewClient(ctx, clientName)
	if err != nil {
		return nil, err
	}
	result, err := c.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: uri,
	})
	if err != nil {
		return nil, err
	}
	return result.Contents, nil
}

// RefreshResources gets the updated list of resources from the MCP and updates the
// global state.
func RefreshResources(ctx context.Context, name string) {
	session, ok := sessions.Get(name)
	if !ok {
		slog.Warn("refresh resources: no session", "name", name)
		return
	}

	resources, err := getResources(ctx, session)
	if err != nil {
		updateState(name, StateError, err, nil, Counts{})
		return
	}

	updateResources(name, resources)

	prev, _ := states.Get(name)
	prev.Counts.Resources = len(resources)
	updateState(name, StateConnected, nil, session, prev.Counts)
}

func getResources(ctx context.Context, c *mcp.ClientSession) ([]*Resource, error) {
	if c.InitializeResult().Capabilities.Resources == nil {
		return nil, nil
	}
	result, err := c.ListResources(ctx, &mcp.ListResourcesParams{})
	if err != nil {
		return nil, err
	}
	return result.Resources, nil
}

func updateResources(mcpName string, resources []*Resource) {
	if len(resources) == 0 {
		allResources.Del(mcpName)
		return
	}
	allResources.Set(mcpName, resources)
}
