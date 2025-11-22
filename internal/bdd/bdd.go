package bdd

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/errors"
)

// TestScenario represents a BDD test scenario
type TestScenario struct {
	Name        string
	Description string
	Given       func(ctx *TestContext) error
	When        func(ctx *TestContext) error
	Then        func(ctx *TestContext, err error) error
	Cleanup     func(ctx *TestContext) error
	Timeout     time.Duration
}

// TestContext provides shared context for BDD scenarios
type TestContext struct {
	T           *testing.T
	Config      *config.Config
	Agent       agent.SessionAgent
	TUI         interface{} // Using interface to avoid import issues
	Clipboard   MockClipboard
	FileSystem  MockFileSystem
	Network     MockNetwork
	StartTime   time.Time
	Data        map[string]interface{}
}

// Mock implementations for testing
type MockClipboard struct {
	Content string
	Error   error
	Calls   int
}

func (m *MockClipboard) WriteAll(content string) error {
	m.Calls++
	m.Content = content
	return m.Error
}

func (m *MockClipboard) ReadAll() (string, error) {
	m.Calls++
	return m.Content, m.Error
}

type MockFileSystem struct {
	Files map[string]string
	Error error
}

func (m *MockFileSystem) WriteFile(path, content string) error {
	if m.Files == nil {
		m.Files = make(map[string]string)
	}
	m.Files[path] = content
	return m.Error
}

func (m *MockFileSystem) ReadFile(path string) (string, error) {
	content, exists := m.Files[path]
	if !exists {
		return "", fmt.Errorf("file not found: %s", path)
	}
	return content, m.Error
}

type MockNetwork struct {
	ShouldFail bool
	Latency   time.Duration
	Calls      int
}

func (m *MockNetwork) Call(endpoint string, data interface{}) error {
	m.Calls++
	time.Sleep(m.Latency)
	if m.ShouldFail {
		return errors.NetworkError("Simulated network failure")
	}
	return nil
}

// BDDTestRunner executes BDD scenarios
type BDDTestRunner struct {
	scenarios []TestScenario
	setup     func(ctx *TestContext) error
	teardown  func(ctx *TestContext) error
}

// NewBDDTestRunner creates a new BDD test runner
func NewBDDTestRunner() *BDDTestRunner {
	return &BDDTestRunner{
		scenarios: make([]TestScenario, 0),
	}
}

// AddScenario adds a test scenario to the runner
func (r *BDDTestRunner) AddScenario(scenario TestScenario) {
	if scenario.Timeout == 0 {
		scenario.Timeout = 30 * time.Second
	}
	r.scenarios = append(r.scenarios, scenario)
}

// SetSetup sets up global test setup function
func (r *BDDTestRunner) SetSetup(setup func(ctx *TestContext) error) {
	r.setup = setup
}

// SetTeardown sets up global test teardown function
func (r *BDDTestRunner) SetTeardown(teardown func(ctx *TestContext) error) {
	r.teardown = teardown
}

// RunAll executes all scenarios
func (r *BDDTestRunner) RunAll(t *testing.T) {
	for _, scenario := range r.scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			r.runScenario(t, scenario)
		})
	}
}

// runScenario executes a single BDD scenario
func (r *BDDTestRunner) runScenario(t *testing.T, scenario TestScenario) {
	ctx := &TestContext{
		T:          t,
		StartTime:   time.Now(),
		Data:        make(map[string]interface{}),
		Clipboard:   MockClipboard{},
		FileSystem:  MockFileSystem{Files: make(map[string]string)},
		Network:     MockNetwork{Latency: 10 * time.Millisecond},
	}

	// Set up test context
	if r.setup != nil {
		if err := r.setup(ctx); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
	}

	// Execute scenario with timeout
	timeoutCtx, cancel := context.WithTimeout(context.Background(), scenario.Timeout)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		// Execute Given
		if scenario.Given != nil {
			if err := scenario.Given(ctx); err != nil {
				done <- errors.BDDTestErrorWithCause("Given step failed", err)
				return
			}
		}

		// Execute When
		if scenario.When != nil {
			if err := scenario.When(ctx); err != nil {
				done <- errors.BDDTestErrorWithCause("When step failed", err)
				return
			}
		}

		done <- nil
	}()

	var testErr error
	select {
	case err := <-done:
		testErr = err
	case <-timeoutCtx.Done():
		testErr = errors.BDDTestError(fmt.Sprintf("Test timed out after %v", scenario.Timeout))
	}

	// Execute Then
	if scenario.Then != nil {
		if err := scenario.Then(ctx, testErr); err != nil {
			t.Errorf("Then step failed: %v", err)
		}
	}

	// Cleanup
	if scenario.Cleanup != nil {
		if err := scenario.Cleanup(ctx); err != nil {
			t.Errorf("Cleanup failed: %v", err)
		}
	}

	// Global teardown
	if r.teardown != nil {
		if err := r.teardown(ctx); err != nil {
			t.Errorf("Teardown failed: %v", err)
		}
	}
}

// Helper functions for common BDD patterns

// GivenUserInEditor creates a user in editor context
func GivenUserInEditor(ctx *TestContext, content string) error {
	ctx.Data["editor_content"] = content
	ctx.Data["editor_cursor"] = 0
	return nil
}

// GivenActiveSession creates an active session context
func GivenActiveSession(ctx *TestContext, sessionID string) error {
	ctx.Data["active_session_id"] = sessionID
	ctx.Data["session_busy"] = false
	return nil
}

// GivenClipboardContent sets clipboard content
func GivenClipboardContent(ctx *TestContext, content string) error {
	ctx.Clipboard.Content = content
	ctx.Data["clipboard_content"] = content
	return nil
}

// GivenNetworkFailure simulates network issues
func GivenNetworkFailure(ctx *TestContext, shouldFail bool) error {
	ctx.Network.ShouldFail = shouldFail
	ctx.Data["network_failure"] = shouldFail
	return nil
}

// WhenUserPressesCtrlA simulates Ctrl+A key press
func WhenUserPressesCtrlA(ctx *TestContext) error {
	ctx.Data["last_key"] = "ctrl+a"
	ctx.Data["selection_triggered"] = true
	return nil
}

// WhenAgentExecutes simulates agent execution
func WhenAgentExecutes(ctx *TestContext, prompt string) error {
	ctx.Data["agent_prompt"] = prompt
	ctx.Data["agent_executing"] = true
	return nil
}

// WhenNetworkFails simulates network failure during operation
func WhenNetworkFails(ctx *TestContext) error {
	ctx.Network.ShouldFail = true
	ctx.Data["network_failed"] = true
	return nil
}

// ThenSelectionExists verifies selection exists
func ThenSelectionExists(ctx *TestContext, err error) error {
	if err != nil {
		return errors.BDDTestErrorWithCause("Selection failed unexpectedly", err)
	}
	
	if !ctx.Data["selection_triggered"].(bool) {
		return errors.BDDTestError("Selection was not triggered")
	}
	
	return nil
}

// ThenErrorIsType verifies error type
func ThenErrorIsType(ctx *TestContext, expected errors.ErrorType, err error) error {
	if err == nil {
		return errors.BDDTestError(fmt.Sprintf("Expected error of type %s but got nil", expected))
	}
	
	if !errors.IsBDDTestError(err) || !errors.IsNetworkError(err) || !errors.IsValidationError(err) {
		return errors.BDDTestError(fmt.Sprintf("Expected error type %s but got %T", expected, err))
	}
	
	return nil
}

// ThenClipboardContains verifies clipboard content
func ThenClipboardContains(ctx *TestContext, expected string) error {
	if ctx.Clipboard.Content != expected {
		return errors.BDDTestError(fmt.Sprintf("Expected clipboard content '%s' but got '%s'", expected, ctx.Clipboard.Content))
	}
	return nil
}

// ThenSessionStateMatches verifies session state
func ThenSessionStateMatches(ctx *TestContext, expectedState string) error {
	actualState, exists := ctx.Data["session_state"]
	if !exists {
		return errors.BDDTestError("Session state not set")
	}
	
	if actualState != expectedState {
		return errors.BDDTestError(fmt.Sprintf("Expected session state '%s' but got '%s'", expectedState, actualState))
	}
	
	return nil
}

// ThenNetworkCallsCount verifies network call count
func ThenNetworkCallsCount(ctx *TestContext, expectedCount int) error {
	if ctx.Network.Calls != expectedCount {
		return errors.BDDTestError(fmt.Sprintf("Expected %d network calls but got %d", expectedCount, ctx.Network.Calls))
	}
	return nil
}

// ThenOperationCompletesWithin verifies operation completes within time limit
func ThenOperationCompletesWithin(ctx *TestContext, duration time.Duration) error {
	elapsed := time.Since(ctx.StartTime)
	if elapsed > duration {
		return errors.BDDTestError(fmt.Sprintf("Expected operation to complete within %v but took %v", duration, elapsed))
	}
	return nil
}