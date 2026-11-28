package mcp

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Resource = mcp.Resource

type ResourceContents = mcp.ResourceContents

var allResources = csync.NewMap[string, []*Resource]()

// Resources returns all available MCP resources.
func Resources() iter.Seq2[string, []*Resource] {
	return allResources.Seq2()
}

// ListResources returns the current resources for an MCP server.
func ListResources(ctx context.Context, cfg *config.ConfigStore, name string) ([]*Resource, error) {
	session, err := getOrRenewClient(ctx, cfg, name)
	if err != nil {
		return nil, err
	}

	resources, err := getResources(ctx, session)
	session.noteResult(err)
	if err != nil {
		return nil, err
	}

	resourceCount := updateResources(name, resources)
	prev, _ := states.Get(name)
	prev.Counts.Resources = resourceCount
	updateState(name, StateConnected, nil, session, prev.Counts)
	return resources, nil
}

// ReadResource reads the contents of a resource from an MCP server.
func ReadResource(ctx context.Context, cfg *config.ConfigStore, name, uri string) ([]*ResourceContents, error) {
	session, err := getOrRenewClient(ctx, cfg, name)
	if err != nil {
		return nil, err
	}
	result, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
	session.noteResult(err)
	if err != nil {
		return nil, err
	}
	return result.Contents, nil
}

// ResourceText joins the contents of a resource read into the text placed in
// the conversation. Binary blobs are replaced by a short placeholder, and the
// joined text is bounded like tool output, with oversized text spilled into
// spillDir.
func ResourceText(contents []*ResourceContents, spillDir string) string {
	var textParts []string
	for _, content := range contents {
		if content == nil {
			continue
		}
		if content.Text != "" {
			textParts = append(textParts, content.Text)
			continue
		}
		if len(content.Blob) > 0 {
			textParts = append(textParts, blobText(content))
			continue
		}
		slog.Debug("MCP resource content missing text/blob", "uri", content.URI)
	}
	return capResult(strings.Join(textParts, "\n"), spillDir)
}

// blobText returns a blob's bytes as text when they are text, and a short
// placeholder otherwise, so binary data never reaches the conversation.
func blobText(content *ResourceContents) string {
	if utf8.Valid(content.Blob) && !bytes.Contains(content.Blob, []byte{0}) {
		return string(content.Blob)
	}
	return fmt.Sprintf(
		"[binary resource %s, %s, %d bytes omitted]",
		content.URI, cmp.Or(content.MIMEType, "unknown type"), len(content.Blob),
	)
}

// RefreshResources gets the updated list of resources from the MCP and updates the
// global state.
func RefreshResources(ctx context.Context, name string) {
	// Serialize with session renewal so the registered session can't be
	// swapped between the Get and the state update below — a stale error
	// transition would otherwise tear down the healthy replacement.
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

	resourceCount := updateResources(name, resources)

	prev, _ := states.Get(name)
	prev.Counts.Resources = resourceCount
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
