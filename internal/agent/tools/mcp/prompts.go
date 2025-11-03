package mcp

import (
	"context"
	"iter"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Prompt = mcp.Prompt

var (
	allPrompts     = csync.NewMap[string, *Prompt]()
	client2Prompts = csync.NewMap[string, []*Prompt]()
)

// GetPrompts returns all available MCP prompts.
func GetPrompts() iter.Seq2[string, *Prompt] {
	return allPrompts.Seq2()
}

// GetPrompt returns a specific MCP prompt by name.
func GetPrompt(name string) (*Prompt, bool) {
	return allPrompts.Get(name)
}

// GetPromptsByClient returns all prompts for a specific MCP client.
func GetPromptsByClient(clientName string) ([]*Prompt, bool) {
	return client2Prompts.Get(clientName)
}

// GetPromptMessages retrieves the content of an MCP prompt with the given arguments.
func GetPromptMessages(ctx context.Context, clientName, promptName string, args map[string]string) ([]string, error) {
	c, err := getOrRenewClient(ctx, clientName)
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
		if msg.Role != "user" {
			continue
		}
		if textContent, ok := msg.Content.(*mcp.TextContent); ok {
			messages = append(messages, textContent.Text)
		}
	}
	return messages, nil
}

func getPrompts(ctx context.Context, c *mcp.ClientSession) ([]*Prompt, error) {
	if c.InitializeResult().Capabilities.Prompts == nil {
		return nil, nil
	}
	result, err := c.ListPrompts(ctx, &mcp.ListPromptsParams{})
	if err != nil {
		return nil, err
	}
	return result.Prompts, nil
}

// updatePrompts updates the global mcpPrompts and mcpClient2Prompts maps
func updatePrompts(mcpName string, prompts []*Prompt) {
	if len(prompts) == 0 {
		client2Prompts.Del(mcpName)
	} else {
		client2Prompts.Set(mcpName, prompts)
	}
	for mcpName, prompts := range client2Prompts.Seq2() {
		for _, p := range prompts {
			key := mcpName + ":" + p.Name
			allPrompts.Set(key, p)
		}
	}
}
