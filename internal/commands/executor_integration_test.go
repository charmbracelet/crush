package commands

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCoordinator is a mock implementation of agent.Coordinator for testing
type mockCoordinator struct {
	mu              sync.Mutex
	calls           []coordinatorCall
	runShouldError  bool
	runError        error
}

type coordinatorCall struct {
	SessionID   string
	Prompt      string
	Attachments []message.Attachment
}

func newMockCoordinator() *mockCoordinator {
	return &mockCoordinator{
		calls: make([]coordinatorCall, 0),
	}
}

func (m *mockCoordinator) Run(ctx context.Context, sessionID string, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, coordinatorCall{
		SessionID:   sessionID,
		Prompt:      prompt,
		Attachments: attachments,
	})

	if m.runShouldError {
		return nil, m.runError
	}

	return &fantasy.AgentResult{}, nil
}

func (m *mockCoordinator) Cancel(sessionID string)                              {}
func (m *mockCoordinator) CancelAll()                                            {}
func (m *mockCoordinator) IsSessionBusy(sessionID string) bool                   { return false }
func (m *mockCoordinator) IsBusy() bool                                          { return false }
func (m *mockCoordinator) QueuedPrompts(sessionID string) int                   { return 0 }
func (m *mockCoordinator) ClearQueue(sessionID string)                           {}
func (m *mockCoordinator) Summarize(ctx context.Context, sessionID string) error { return nil }
func (m *mockCoordinator) Model() agent.Model                                     { return agent.Model{} }
func (m *mockCoordinator) UpdateModels(ctx context.Context) error                 { return nil }

func (m *mockCoordinator) GetCalls() []coordinatorCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]coordinatorCall, len(m.calls))
	copy(calls, m.calls)
	return calls
}

// mockMessageService is a mock implementation of message.Service for testing
type mockMessageService struct {
	mu     sync.Mutex
	msgs   []messageCall
	createShouldError bool
	createError       error
}

type messageCall struct {
	SessionID string
	Params    message.CreateMessageParams
}

func newMockMessageService() *mockMessageService {
	return &mockMessageService{
		msgs: make([]messageCall, 0),
	}
}

func (m *mockMessageService) Create(ctx context.Context, sessionID string, params message.CreateMessageParams) (message.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.msgs = append(m.msgs, messageCall{
		SessionID: sessionID,
		Params:    params,
	})

	if m.createShouldError {
		return message.Message{}, m.createError
	}

	return message.Message{
		ID:        "mock-msg-" + sessionID,
		SessionID: sessionID,
		Role:      params.Role,
		Parts:     params.Parts,
	}, nil
}

func (m *mockMessageService) GetCalls() []messageCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]messageCall, len(m.msgs))
	copy(calls, m.msgs)
	return calls
}

// Implement other message.Service methods (not used in tests)
func (m *mockMessageService) Update(ctx context.Context, msg message.Message) error { return nil }
func (m *mockMessageService) Get(ctx context.Context, id string) (message.Message, error) {
	return message.Message{}, nil
}
func (m *mockMessageService) List(ctx context.Context, sessionID string) ([]message.Message, error) {
	return nil, nil
}
func (m *mockMessageService) Delete(ctx context.Context, id string) error { return nil }
func (m *mockMessageService) DeleteSessionMessages(ctx context.Context, sessionID string) error { return nil }
func (m *mockMessageService) Subscribe(ctx context.Context) <-chan pubsub.Event[message.Message] {
	ch := make(chan pubsub.Event[message.Message])
	close(ch)
	return ch
}

func TestIntegration_FullCommandExecution(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir
	workingDir := tmpDir

	// Create commands directory
	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	// Create a test file for file reference
	testFile := filepath.Join(workingDir, "test-file.txt")
	testFileContent := "This is test file content"
	require.NoError(t, os.WriteFile(testFile, []byte(testFileContent), 0o644))

	// Create a command file with arguments, file references, and allowed-tools
	cmdFile := filepath.Join(commandsDir, "test-cmd.md")
	cmdContent := `---
description: Test command for integration testing
argument-hint: "[pr-number] [priority]"
allowed-tools: ["view", "edit"]
---
Review PR $1 with priority $2.

Please check @test-file.txt for additional context.

All arguments: $ARGS
`
	require.NoError(t, os.WriteFile(cmdFile, []byte(cmdContent), 0o644))

	// Create registry and load commands
	registry := NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	// Create mock coordinator and message service
	mockCoord := newMockCoordinator()
	mockMessages := newMockMessageService()

	// Create executor
	executor := NewExecutor(registry, mockCoord, mockMessages, workingDir)

	// Execute command with arguments
	ctx := context.Background()
	sessionID := "test-session-123"
	args := []string{"123", "high"}

	err = executor.Execute(ctx, sessionID, "test-cmd", args)
	require.NoError(t, err)

	// Verify coordinator was called
	calls := mockCoord.GetCalls()
	require.Len(t, calls, 1, "Coordinator should be called exactly once")

	call := calls[0]

	// Verify session ID
	assert.Equal(t, sessionID, call.SessionID, "Session ID should match")

	// Verify argument substitution worked
	assert.Contains(t, call.Prompt, "Review PR 123 with priority high", "Prompt should contain substituted arguments")
	assert.Contains(t, call.Prompt, "All arguments: 123 high", "Prompt should contain $ARGS substitution")

	// Verify file reference was resolved and attached
	require.Len(t, call.Attachments, 1, "Should have one file attachment")
	attachment := call.Attachments[0]
	assert.Equal(t, testFile, attachment.FilePath, "Attachment file path should match")
	assert.Equal(t, "test-file.txt", attachment.FileName, "Attachment file name should match")
	assert.Equal(t, testFileContent, string(attachment.Content), "Attachment content should match")

	// Verify file reference is preserved in prompt (not removed)
	assert.Contains(t, call.Prompt, "@test-file.txt", "File reference should be preserved in prompt")
}

func TestIntegration_CommandExecutionWithMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir
	workingDir := tmpDir

	// Create commands directory
	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	// Create multiple test files
	file1 := filepath.Join(workingDir, "file1.txt")
	file2 := filepath.Join(workingDir, "file2.go")
	require.NoError(t, os.WriteFile(file1, []byte("Content 1"), 0o644))
	require.NoError(t, os.WriteFile(file2, []byte("Content 2"), 0o644))

	// Create command with multiple file references
	cmdFile := filepath.Join(commandsDir, "multi-file.md")
	cmdContent := `---
description: Command with multiple file references
---
Review @file1.txt and @file2.go
`
	require.NoError(t, os.WriteFile(cmdFile, []byte(cmdContent), 0o644))

	// Create registry and load commands
	registry := NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	// Create mock coordinator and message service
	mockCoord := newMockCoordinator()
	mockMessages := newMockMessageService()

	// Create executor
	executor := NewExecutor(registry, mockCoord, mockMessages, workingDir)

	// Execute command
	ctx := context.Background()
	err = executor.Execute(ctx, "session-1", "multi-file", []string{})
	require.NoError(t, err)

	// Verify coordinator was called with both files
	calls := mockCoord.GetCalls()
	require.Len(t, calls, 1)
	call := calls[0]

	// Should have 2 attachments
	require.Len(t, call.Attachments, 2, "Should have two file attachments")

	// Verify both files are attached
	filePaths := make(map[string]bool)
	for _, att := range call.Attachments {
		filePaths[att.FilePath] = true
	}
	assert.True(t, filePaths[file1], "file1.txt should be attached")
	assert.True(t, filePaths[file2], "file2.go should be attached")
}

func TestIntegration_CommandExecutionWithNoAllowedTools(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir
	workingDir := tmpDir

	// Create commands directory
	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	// Create command without allowed-tools (should allow all tools)
	cmdFile := filepath.Join(commandsDir, "no-tools-restriction.md")
	cmdContent := `---
description: Command without tool restrictions
---
Simple command without tool restrictions.
`
	require.NoError(t, os.WriteFile(cmdFile, []byte(cmdContent), 0o644))

	// Create registry and load commands
	registry := NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	// Create mock coordinator and message service
	mockCoord := newMockCoordinator()
	mockMessages := newMockMessageService()

	// Create executor
	executor := NewExecutor(registry, mockCoord, mockMessages, workingDir)

	// Execute command
	ctx := context.Background()
	err = executor.Execute(ctx, "session-1", "no-tools-restriction", []string{})
	require.NoError(t, err)

	// Verify coordinator was called (tool filtering is noted but not enforced in current implementation)
	calls := mockCoord.GetCalls()
	require.Len(t, calls, 1)
}

func TestIntegration_HelpCommandExecution(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir
	workingDir := tmpDir

	// Create commands directory
	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	// Create test commands
	cmd1 := filepath.Join(commandsDir, "cmd1.md")
	cmd2 := filepath.Join(commandsDir, "frontend", "cmd2.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(cmd2), 0o755))
	
	require.NoError(t, os.WriteFile(cmd1, []byte(`---
description: First command
---
# Command 1
`), 0o644))
	require.NoError(t, os.WriteFile(cmd2, []byte(`---
description: Frontend command
---
# Command 2
`), 0o644))

	// Create registry and load commands
	registry := NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	// Create mock coordinator and message service
	mockCoord := newMockCoordinator()
	mockMessages := newMockMessageService()

	// Create executor
	executor := NewExecutor(registry, mockCoord, mockMessages, workingDir)

	// Execute help command
	ctx := context.Background()
	err = executor.Execute(ctx, "session-1", "help", []string{})
	require.NoError(t, err)

	// Verify coordinator was NOT called (help command creates message directly)
	calls := mockCoord.GetCalls()
	assert.Empty(t, calls, "Coordinator should NOT be called for help command")

	// Verify message service was called instead
	msgCalls := mockMessages.GetCalls()
	require.Len(t, msgCalls, 1, "Message service should be called exactly once")

	msgCall := msgCalls[0]

	// Verify session ID
	assert.Equal(t, "session-1", msgCall.SessionID, "Session ID should match")

	// Verify message is assistant role
	assert.Equal(t, message.Assistant, msgCall.Params.Role, "Message should be assistant role")

	// Verify help output contains expected content
	helpText := ""
	for _, part := range msgCall.Params.Parts {
		if textPart, ok := part.(message.TextContent); ok {
			helpText = textPart.Text
			break
		}
	}
	require.NotEmpty(t, helpText, "Help output should contain text")
	assert.Contains(t, helpText, "Available Commands", "Help output should contain header")
	assert.Contains(t, helpText, "\\cmd1", "Help output should contain cmd1")
	assert.Contains(t, helpText, "\\frontend:cmd2", "Help output should contain namespaced command")
	assert.Contains(t, helpText, "First command", "Help output should contain description")
	assert.Contains(t, helpText, "Frontend command", "Help output should contain frontend command description")
}

func TestIntegration_HelpCommandExecution_EmptyRegistry(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir
	workingDir := tmpDir

	// Create registry (no commands)
	registry := NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	// Create mock coordinator and message service
	mockCoord := newMockCoordinator()
	mockMessages := newMockMessageService()

	// Create executor
	executor := NewExecutor(registry, mockCoord, mockMessages, workingDir)

	// Execute help command
	ctx := context.Background()
	err = executor.Execute(ctx, "session-1", "help", []string{})
	require.NoError(t, err)

	// Verify coordinator was NOT called
	calls := mockCoord.GetCalls()
	assert.Empty(t, calls, "Coordinator should NOT be called for help command")

	// Verify message service was called
	msgCalls := mockMessages.GetCalls()
	require.Len(t, msgCalls, 1)

	msgCall := msgCalls[0]

	// Verify help output for empty registry
	// Help command should always be shown, even when registry is empty
	helpText := ""
	for _, part := range msgCall.Params.Parts {
		if textPart, ok := part.(message.TextContent); ok {
			helpText = textPart.Text
			break
		}
	}
	require.NotEmpty(t, helpText, "Help output should contain text")
	assert.Contains(t, helpText, "Available Commands", "Help output should contain header")
	assert.Contains(t, helpText, "\\help", "Help output should contain help command")
	assert.Contains(t, helpText, "Show a list of all available commands", "Help output should contain help description")
}

func TestIntegration_CommandExecutionErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := tmpDir
	workingDir := tmpDir

	// Create commands directory
	commandsDir := filepath.Join(projectDir, ".crush", "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0o755))

	// Create command file
	cmdFile := filepath.Join(commandsDir, "error-cmd.md")
	cmdContent := `---
description: Command that will cause coordinator error
---
This command will fail execution.
`
	require.NoError(t, os.WriteFile(cmdFile, []byte(cmdContent), 0o644))

	// Create registry and load commands
	registry := NewRegistry(projectDir)
	_, err := registry.LoadCommands()
	require.NoError(t, err)

	// Create mock coordinator that returns error
	mockCoord := newMockCoordinator()
	mockCoord.runShouldError = true
	mockCoord.runError = assert.AnError

	// Create mock message service
	mockMessages := newMockMessageService()

	// Create executor
	executor := NewExecutor(registry, mockCoord, mockMessages, workingDir)

	// Execute command - should return error
	ctx := context.Background()
	err = executor.Execute(ctx, "session-1", "error-cmd", []string{})
	require.Error(t, err, "Executor should return error when coordinator fails")
	assert.Contains(t, err.Error(), "failed to execute command", "Error message should indicate execution failure")
}

