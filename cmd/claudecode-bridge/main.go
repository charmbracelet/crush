package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

func main() {
	prompt := flag.String("p", "", "Prompt to send to Claude Code")
	model := flag.String("model", "sonnet", "Model to use (sonnet, opus, haiku)")
	tools := flag.String("tools", "", "Comma-separated list of allowed tools")
	output := flag.String("output", "text", "Output format: text, json, stream")
	verbose := flag.Bool("v", false, "Verbose output")
	flag.Parse()

	if *prompt == "" {
		fmt.Println("Error: -p flag is required")
		fmt.Println("Usage: claudecode-bridge -p 'your prompt here'")
		flag.Usage()
		os.Exit(1)
	}

	if !isClaudeCodeAvailable() {
		fmt.Println("Error: Claude Code CLI not found")
		fmt.Println("Please install Claude Code from: https://claude.ai/code")
		os.Exit(1)
	}

	runCLIPrompt(*prompt, *model, *tools, *output, *verbose)
}

func isClaudeCodeAvailable() bool {
	cmd := exec.Command("claude", "--version")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run() == nil
}

func runCLIPrompt(prompt, model, tools, output string, verbose bool) {
	args := []string{"-p", prompt, "--output-format", output}

	if model != "" {
		args = append(args, "--model", model)
	}

	if tools != "" {
		args = append(args, "--allowedTools", tools)
	}

	if verbose {
		args = append(args, "--verbose")
	}

	args = append(args, "--bare")

	cmd := exec.Command("claude", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalf("Claude Code failed: %v", err)
	}
}

// ClaudeCodeBridge provides a Go API for Claude Code communication
type ClaudeCodeBridge struct {
	mu       sync.Mutex
	sessions map[string]*ClaudeSession
}

// ClaudeSession represents a conversation session with Claude Code
type ClaudeSession struct {
	ID        string
	Model     string
	Tools     []string
	CreatedAt time.Time
	LastUse   time.Time
}

// NewClaudeCodeBridge creates a new bridge instance
func NewClaudeCodeBridge() *ClaudeCodeBridge {
	return &ClaudeCodeBridge{
		sessions: make(map[string]*ClaudeSession),
	}
}

// Query sends a prompt to Claude Code and returns the response
func (b *ClaudeCodeBridge) Query(ctx context.Context, prompt string, opts ...ClaudeOption) (string, error) {
	config := DefaultClaudeConfig()
	for _, opt := range opts {
		opt(config)
	}

	args := []string{"-p", prompt, "--output-format", "json", "--bare"}

	if config.Model != "" {
		args = append(args, "--model", config.Model)
	}

	if len(config.Tools) > 0 {
		args = append(args, "--allowedTools", strings.Join(config.Tools, ","))
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("Claude Code query failed: %w", err)
	}

	return string(output), nil
}

// QueryStream sends a prompt and streams the response via CLI
func (b *ClaudeCodeBridge) QueryStream(ctx context.Context, prompt string, opts ...ClaudeOption) (<-chan StreamChunk, error) {
	config := DefaultClaudeConfig()
	for _, opt := range opts {
		opt(config)
	}

	args := []string{"-p", prompt, "--output-format", "stream-json", "--include-partial-messages", "--bare"}

	if config.Model != "" {
		args = append(args, "--model", config.Model)
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	ch := make(chan StreamChunk, 100)
	go func() {
		defer close(ch)
		defer cmd.Wait()
	}()

	return ch, nil
}

// ClaudeConfig holds configuration for Claude Code queries
type ClaudeConfig struct {
	Model string
	Tools []string
}

// DefaultClaudeConfig returns default configuration
func DefaultClaudeConfig() *ClaudeConfig {
	return &ClaudeConfig{
		Model: "sonnet",
		Tools: []string{},
	}
}

// ClaudeOption is a functional option for configuring queries
type ClaudeOption func(*ClaudeConfig)

// WithModel sets the model to use
func WithModel(model string) ClaudeOption {
	return func(c *ClaudeConfig) {
		c.Model = model
	}
}

// WithTools sets the allowed tools
func WithTools(tools ...string) ClaudeOption {
	return func(c *ClaudeConfig) {
		c.Tools = tools
	}
}

// StreamChunk represents a chunk of streamed response
type StreamChunk struct {
	Type    string
	Content string
	Error   error
}

// DirectCLI provides simple CLI-based access
type DirectCLI struct{}

// Query executes a prompt and returns the response
func (d *DirectCLI) Query(ctx context.Context, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude", "-p", prompt, "--bare")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("Claude Code error: %s", string(exitErr.Stderr))
		}
		return "", err
	}
	return string(output), nil
}

// QueryWithOptions executes a prompt with options
func (d *DirectCLI) QueryWithOptions(ctx context.Context, prompt string, model string, tools []string) (string, error) {
	args := []string{"-p", prompt, "--bare", "--output-format", "json"}

	if model != "" {
		args = append(args, "--model", model)
	}

	if len(tools) > 0 {
		args = append(args, "--allowedTools", strings.Join(tools, ","))
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("Claude Code error: %s", string(exitErr.Stderr))
		}
		return "", err
	}
	return string(output), nil
}

// CheckClaudeCode checks if Claude Code CLI is installed
func CheckClaudeCode() error {
	cmd := exec.Command("claude", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Claude Code CLI not found: %w", err)
	}
	return nil
}

// MCPBridge provides MCP protocol integration with Claude Code
// Note: Claude Code MCP integration requires starting Claude Code as a server
// and connecting via stdio. This is a simplified implementation.
type MCPBridge struct {
	bridge *ClaudeCodeBridge
}

// NewMCPBridge creates a new MCP bridge for Claude Code
func NewMCPBridge() *MCPBridge {
	return &MCPBridge{
		bridge: NewClaudeCodeBridge(),
	}
}

// StartMCPServer starts Claude Code as an MCP server in background
func (b *MCPBridge) StartMCPServer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "claude", "mcp", "serve")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

// QueryMCPServer sends a query to a running MCP server
func (b *MCPBridge) QueryMCPServer(ctx context.Context, prompt string) (string, error) {
	return b.bridge.Query(ctx, prompt, WithModel("sonnet"))
}

// ClaudeCodeResponse represents the JSON response from Claude Code
type ClaudeCodeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	} `json:"content"`
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
}

// ParseResponse parses JSON response from Claude Code
func ParseResponse(data string) (*ClaudeCodeResponse, error) {
	var resp ClaudeCodeResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Session represents an extended session with Claude Code
type Session struct {
	ID         string
	Model      string
	MaxTurns   int
	Tools      []string
	cli        *DirectCLI
	ctx        context.Context
	cancel     context.CancelFunc
	history    []Message
}

// Message represents a message in the conversation
type Message struct {
	Role    string
	Content string
}

// NewSession creates a new Claude Code session
func NewSession(id string, model string, maxTurns int, tools []string) *Session {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	return &Session{
		ID:       id,
		Model:    model,
		MaxTurns: maxTurns,
		Tools:    tools,
		cli:      &DirectCLI{},
		ctx:      ctx,
		cancel:   cancel,
		history:  []Message{},
	}
}

// Send sends a message and returns the response
func (s *Session) Send(content string) (string, error) {
	resp, err := s.cli.QueryWithOptions(s.ctx, content, s.Model, s.Tools)
	if err != nil {
		return "", err
	}

	s.history = append(s.history, Message{Role: "user", Content: content})

	parsed, err := ParseResponse(resp)
	if err != nil {
		return resp, nil
	}

	var text string
	for _, c := range parsed.Content {
		text += c.Text
	}

	s.history = append(s.history, Message{Role: "assistant", Content: text})
	return text, nil
}

// Close closes the session
func (s *Session) Close() {
	s.cancel()
}

// GetHistory returns the conversation history
func (s *Session) GetHistory() []Message {
	return s.history
}
