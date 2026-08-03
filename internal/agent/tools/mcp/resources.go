package mcp

import (
	"context"
	"errors"
	"iter"
	"log/slog"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Resource = mcp.Resource

type ResourceTemplate = mcp.ResourceTemplate

type ResourceContents = mcp.ResourceContents

var (
	allResources         = csync.NewMap[string, []*Resource]()
	allResourceTemplates = csync.NewMap[string, []*ResourceTemplate]()
)

// Resources returns all available MCP resources.
func Resources() iter.Seq2[string, []*Resource] {
	return allResources.Seq2()
}

// ResourceTemplates returns all available MCP resource templates.
func ResourceTemplates() iter.Seq2[string, []*ResourceTemplate] {
	return allResourceTemplates.Seq2()
}

// ListResources returns the current resources (including resource templates) for an MCP server.
func ListResources(ctx context.Context, cfg *config.ConfigStore, name string) ([]*Resource, []*ResourceTemplate, error) {
	session, err := getOrRenewClient(ctx, cfg, name)
	if err != nil {
		return nil, nil, err
	}

	resources, err := getResources(ctx, session)
	if err != nil {
		return nil, nil, err
	}

	templates := listTemplatesBestEffort(ctx, session, name)

	resourceCount := updateResources(name, resources)
	templateCount := updateResourceTemplates(name, templates)
	prev, _ := states.Get(name)
	prev.Counts.Resources = resourceCount + templateCount
	updateState(name, StateConnected, nil, session, prev.Counts)
	return resources, templates, nil
}

// ReadResource reads the contents of a resource from an MCP server.
func ReadResource(ctx context.Context, cfg *config.ConfigStore, name, uri string) ([]*ResourceContents, error) {
	session, err := getOrRenewClient(ctx, cfg, name)
	if err != nil {
		return nil, err
	}
	result, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		return nil, err
	}
	return result.Contents, nil
}

// RefreshResources gets the updated list of resources and resource templates from the MCP and updates the
// global state.
func RefreshResources(ctx context.Context, name string) {
	// Runs under the per-name lifecycle lock so a concurrent renewal can't
	// swap the session between our Get and the state update below.
	mu := renewLock(name)
	mu.Lock()
	defer mu.Unlock()

	session, ok := sessions.Get(name)
	if !ok {
		slog.Warn("Refresh resources: no session", "name", name)
		return
	}

	resources, err := getResources(ctx, session)
	if err != nil {
		updateState(name, StateError, err, session, Counts{})
		return
	}

	templates := listTemplatesBestEffort(ctx, session, name)

	resourceCount := updateResources(name, resources)
	templateCount := updateResourceTemplates(name, templates)

	prev, _ := states.Get(name)
	prev.Counts.Resources = resourceCount + templateCount
	updateState(name, StateConnected, nil, session, prev.Counts)
}

func getResources(ctx context.Context, c *ClientSession) ([]*Resource, error) {
	if c.InitializeResult().Capabilities.Resources == nil {
		return nil, nil
	}
	result, err := c.ListResources(ctx, &mcp.ListResourcesParams{})
	if err != nil {
		// Handle "Method not found" errors from MCP servers that don't support resources/list.
		if isMethodNotFoundError(err) {
			slog.Warn("MCP server does not support resources/list", "error", err)
			return nil, nil
		}
		return nil, err
	}
	return result.Resources, nil
}

// refreshSessionResources re-fetches a session's resources and resource
// templates (best-effort: a listing failure degrades to an empty registry
// and a warn, never an error) and installs them in the registries. It
// returns the combined count for ClientInfo.Counts.Resources, so callers
// publishing a StateConnected transition report exactly what the
// registries hold.
func refreshSessionResources(ctx context.Context, name string, session *ClientSession) int {
	resources, err := getResources(ctx, session)
	if err != nil {
		slog.Warn("Error listing MCP resources", "name", name, "error", err)
		resources = nil
	}
	templates := listTemplatesBestEffort(ctx, session, name)
	return updateResources(name, resources) + updateResourceTemplates(name, templates)
}

// listTemplatesBestEffort lists a session's resource templates, treating any
// failure as "no templates". getResourceTemplates already swallows
// method-not-found, so an error reaching here is a timeout, transport
// failure, or server bug — never lack of support — and is logged as such,
// with the server name so a multi-server config stays debuggable.
func listTemplatesBestEffort(ctx context.Context, session *ClientSession, name string) []*ResourceTemplate {
	templates, err := getResourceTemplates(ctx, session)
	if err != nil {
		slog.Warn("Error listing MCP resource templates", "name", name, "error", err)
		return nil
	}
	return templates
}

func getResourceTemplates(ctx context.Context, c *ClientSession) ([]*ResourceTemplate, error) {
	if c.InitializeResult().Capabilities.Resources == nil {
		return nil, nil
	}
	result, err := c.ListResourceTemplates(ctx, &mcp.ListResourceTemplatesParams{})
	if err != nil {
		if isMethodNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}
	return result.ResourceTemplates, nil
}

// isMethodNotFoundError checks if the error is a JSON-RPC "Method not found" error.
func isMethodNotFoundError(err error) bool {
	var rpcErr *jsonrpc.Error
	return errors.As(err, &rpcErr) && rpcErr != nil && rpcErr.Code == jsonrpc.CodeMethodNotFound
}

func updateResources(name string, resources []*Resource) int {
	if len(resources) == 0 {
		allResources.Del(name)
		return 0
	}
	allResources.Set(name, resources)
	return len(resources)
}

func updateResourceTemplates(name string, templates []*ResourceTemplate) int {
	if len(templates) == 0 {
		allResourceTemplates.Del(name)
		return 0
	}
	allResourceTemplates.Set(name, templates)
	return len(templates)
}
