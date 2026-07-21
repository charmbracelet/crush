package mcp

import (
	"context"
	"iter"
	"log/slog"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Prompt = mcp.Prompt

var allPrompts = csync.NewMap[string, []*Prompt]()

// Prompts returns all available MCP prompts.
func Prompts() iter.Seq2[string, []*Prompt] {
	return allPrompts.Seq2()
}

// GetPromptMessages retrieves the content of an MCP prompt with the given arguments.
func GetPromptMessages(ctx context.Context, cfg *config.ConfigStore, clientName, promptName string, args map[string]string) ([]string, error) {
	c, err := getOrRenewClient(ctx, cfg, clientName)
	if err != nil {
		return nil, err
	}
	result, err := c.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      promptName,
		Arguments: args,
	})
	if err != nil {
		return nil, err
	}

	var messages []string
	for _, msg := range result.Messages {
		// MCP prompts may seed either side of the conversation (the spec
		// allows both user and assistant roles); include text from every
		// message rather than only user-role ones, otherwise server-authored
		// context in assistant turns is silently dropped.
		if msg == nil {
			continue
		}
		if textContent, ok := msg.Content.(*mcp.TextContent); ok {
			messages = append(messages, textContent.Text)
			continue
		}
		// Non-text content (image, audio, embedded resources) has no string
		// form for the agent tool. Log it at debug rather than silently
		// swallowing the whole message so the skip is diagnosable, mirroring
		// read_mcp_resource's handling of mixed content.
		slog.Debug("MCP prompt message has non-text content, skipping", "prompt", promptName, "role", msg.Role)
	}
	return messages, nil
}

// RefreshPrompts gets the updated list of prompts from the MCP and updates the
// global state.
func RefreshPrompts(ctx context.Context, name string) {
	session, ok := sessions.Get(name)
	if !ok {
		slog.Warn("Refresh prompts: no session", "name", name)
		return
	}

	prompts, err := getPrompts(ctx, session)
	if err != nil {
		updateState(name, StateError, err, nil, Counts{})
		return
	}

	updatePrompts(name, prompts)

	prev, _ := states.Get(name)
	prev.Counts.Prompts = len(prompts)
	updateState(name, StateConnected, nil, session, prev.Counts)
}

func getPrompts(ctx context.Context, c *ClientSession) ([]*Prompt, error) {
	if c.InitializeResult().Capabilities.Prompts == nil {
		return nil, nil
	}
	result, err := c.ListPrompts(ctx, &mcp.ListPromptsParams{})
	if err != nil {
		// Handle "Method not found" errors from MCP servers that advertise
		// the prompts capability but reject prompts/list at call time
		// (partial implementations, proxies, older SDK versions). Degrade to
		// "no prompts" rather than bubbling the error, mirroring getResources.
		if isMethodNotFoundError(err) {
			slog.Warn("MCP server does not support prompts/list", "error", err)
			return nil, nil
		}
		return nil, err
	}
	return result.Prompts, nil
}

// updatePrompts updates the global mcpPrompts and mcpClient2Prompts maps
func updatePrompts(mcpName string, prompts []*Prompt) {
	if len(prompts) == 0 {
		allPrompts.Del(mcpName)
		return
	}
	allPrompts.Set(mcpName, prompts)
}
