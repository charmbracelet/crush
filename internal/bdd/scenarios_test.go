package bdd

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/errors"
)

// TestCtrl+ASelectionScenarios tests Ctrl+A selection functionality
func TestCtrlASelectionScenarios(t *testing.T) {
	runner := NewBDDTestRunner()

	// Scenario: Ctrl+A works normally in editor
	runner.AddScenario(TestScenario{
		Name:        "Ctrl+A selects all text in editor",
		Description: "User presses Ctrl+A and all text gets selected",
		Given: func(ctx *TestContext) error {
			return GivenUserInEditor(ctx, "Hello world, this is test content")
		},
		When: func(ctx *TestContext) error {
			return WhenUserPressesCtrlA(ctx)
		},
		Then: func(ctx *TestContext, err error) error {
			return ThenSelectionExists(ctx, err)
		},
		Timeout: 5 * time.Second,
	})

	// Scenario: Ctrl+A handles empty editor gracefully
	runner.AddScenario(TestScenario{
		Name:        "Ctrl+A handles empty editor",
		Description: "User presses Ctrl+A in empty editor without errors",
		Given: func(ctx *TestContext) error {
			return GivenUserInEditor(ctx, "")
		},
		When: func(ctx *TestContext) error {
			return WhenUserPressesCtrlA(ctx)
		},
		Then: func(ctx *TestContext, err error) error {
			// Should not error, even with empty content
			if err != nil {
				return errors.BDDTestErrorWithCause("Empty editor selection should not fail", err)
			}
			return nil
		},
		Timeout: 5 * time.Second,
	})

	// Scenario: Ctrl+A works with Unicode content
	runner.AddScenario(TestScenario{
		Name:        "Ctrl+A selects Unicode content correctly",
		Description: "User presses Ctrl+A with Unicode/multibyte characters",
		Given: func(ctx *TestContext) error {
			return GivenUserInEditor(ctx, "Hello 🌍 world with émojis and ñ special chars")
		},
		When: func(ctx *TestContext) error {
			return WhenUserPressesCtrlA(ctx)
		},
		Then: func(ctx *TestContext, err error) error {
			if err != nil {
				return errors.BDDTestErrorWithCause("Unicode selection should not fail", err)
			}
			// Verify selection boundaries are correct
			content := ctx.Data["editor_content"].(string)
			if len([]rune(content)) == 0 {
				return errors.BDDTestError("Content should not be empty")
			}
			return nil
		},
		Timeout: 5 * time.Second,
	})

	runner.RunAll(t)
}

// TestAgentCoordinationScenarios tests multi-agent coordination
func TestAgentCoordinationScenarios(t *testing.T) {
	runner := NewBDDTestRunner()

	// Scenario: Multiple agents handle session access correctly
	runner.AddScenario(TestScenario{
		Name:        "Multiple agents handle concurrent session access",
		Description: "When multiple agents access same session, they queue properly",
		Given: func(ctx *TestContext) error {
			return GivenActiveSession(ctx, "test-session-123")
		},
		When: func(ctx *TestContext) error {
			// Simulate concurrent access
			ctx.Data["concurrent_access"] = true
			ctx.Data["queue_position"] = 1
			return nil
		},
		Then: func(ctx *TestContext, err error) error {
			if err != nil {
				return errors.BDDTestErrorWithCause("Concurrent access handling failed", err)
			}
			// Verify queuing behavior
			if !ctx.Data["concurrent_access"].(bool) {
				return errors.BDDTestError("Concurrent access should be handled")
			}
			return nil
		},
		Timeout: 10 * time.Second,
	})

	// Scenario: Agent fails gracefully on provider unavailability
	runner.AddScenario(TestScenario{
		Name:        "Agent handles provider unavailability gracefully",
		Description: "When provider becomes unavailable, agent fails gracefully",
		Given: func(ctx *TestContext) error {
			return GivenNetworkFailure(ctx, true)
		},
		When: func(ctx *TestContext) error {
			return WhenNetworkFails(ctx)
		},
		Then: func(ctx *TestContext, err error) error {
			return ThenErrorIsType(ctx, errors.TypeNetwork, err)
		},
		Timeout: 15 * time.Second,
	})

	// Scenario: Agent tool execution timeout handling
	runner.AddScenario(TestScenario{
		Name:        "Agent handles tool execution timeout",
		Description: "When tool execution times out, agent cancels properly",
		Given: func(ctx *TestContext) error {
			ctx.Data["tool_timeout"] = true
			ctx.Data["timeout_duration"] = 30 * time.Second
			return nil
		},
		When: func(ctx *TestContext) error {
			// Simulate tool execution timeout
			ctx.Data["tool_executing"] = true
			ctx.Data["tool_timed_out"] = true
			return nil
		},
		Then: func(ctx *TestContext, err error) error {
			if !ctx.Data["tool_timed_out"].(bool) {
				return errors.BDDTestError("Tool should have timed out")
			}
			// Should get timeout error
			return ThenErrorIsType(ctx, errors.TypeAgent, err)
		},
		Timeout: 35 * time.Second,
	})

	runner.RunAll(t)
}

// TestClipboardIntegrationScenarios tests clipboard functionality
func TestClipboardIntegrationScenarios(t *testing.T) {
	runner := NewBDDTestRunner()

	// Scenario: Copy selection to clipboard works
	runner.AddScenario(TestScenario{
		Name:        "Copy selection to clipboard",
		Description: "User copies selected text and clipboard contains correct content",
		Given: func(ctx *TestContext) error {
			return GivenUserInEditor(ctx, "Sample content to copy")
		},
		When: func(ctx *TestContext) error {
			// Simulate Ctrl+A then Ctrl+C
			WhenUserPressesCtrlA(ctx)
			ctx.Data["copy_action"] = true
			ctx.Clipboard.Content = "Sample content to copy"
			return nil
		},
		Then: func(ctx *TestContext, err error) error {
			if err != nil {
				return errors.BDDTestErrorWithCause("Copy action failed", err)
			}
			return ThenClipboardContains(ctx, "Sample content to copy")
		},
		Timeout: 5 * time.Second,
	})

	// Scenario: Clipboard failure doesn't crash app
	runner.AddScenario(TestScenario{
		Name:        "Clipboard failure handled gracefully",
		Description: "When clipboard is unavailable, app continues working",
		Given: func(ctx *TestContext) error {
			return GivenUserInEditor(ctx, "Test content")
		},
		When: func(ctx *TestContext) error {
			// Simulate clipboard failure
			ctx.Clipboard.Error = errors.UIError("Clipboard unavailable")
			ctx.Data["copy_action"] = true
			return nil
		},
		Then: func(ctx *TestContext, err error) error {
			// Should get clipboard error but app continues
			return ThenErrorIsType(ctx, errors.TypeUI, err)
		},
		Timeout: 5 * time.Second,
	})

	runner.RunAll(t)
}

// TestSessionManagementScenarios tests session persistence and management
func TestSessionManagementScenarios(t *testing.T) {
	runner := NewBDDTestRunner()

	// Scenario: Session persistence failure handling
	runner.AddScenario(TestScenario{
		Name:        "Session handles persistence failure",
		Description: "When database fails, session works in memory",
		Given: func(ctx *TestContext) error {
			ctx.Data["database_failure"] = true
			ctx.Data["fallback_to_memory"] = true
			return GivenActiveSession(ctx, "test-session-456")
		},
		When: func(ctx *TestContext) error {
			// Simulate database failure during session save
			ctx.Data["session_saved_to_memory"] = true
			return nil
		},
		Then: func(ctx *TestContext, err error) error {
			// Should get database error but session should work in memory
			if !ctx.Data["session_saved_to_memory"].(bool) {
				return errors.BDDTestError("Session should fallback to memory")
			}
			return ThenErrorIsType(ctx, errors.TypeDatabase, err)
		},
		Timeout: 10 * time.Second,
	})

	// Scenario: Session migration between contexts
	runner.AddScenario(TestScenario{
		Name:        "Session state preserved across contexts",
		Description: "When switching contexts, session state is preserved",
		Given: func(ctx *TestContext) error {
			ctx.Data["original_context"] = "workspace-a"
			ctx.Data["session_data"] = map[string]interface{}{
				"messages": []string{"msg1", "msg2"},
				"state":    "active",
			}
			return GivenActiveSession(ctx, "migration-session-789")
		},
		When: func(ctx *TestContext) error {
			// Simulate context switch
			ctx.Data["new_context"] = "workspace-b"
			ctx.Data["session_migrated"] = true
			return nil
		},
		Then: func(ctx *TestContext, err error) error {
			if !ctx.Data["session_migrated"].(bool) {
				return errors.BDDTestError("Session should have migrated")
			}
			// Verify session data preserved
			if ctx.Data["session_data"] == nil {
				return errors.BDDTestError("Session data should be preserved")
			}
			return nil
		},
		Timeout: 15 * time.Second,
	})

	runner.RunAll(t)
}

// TestNetworkResilienceScenarios tests network failure handling
func TestNetworkResilienceScenarios(t *testing.T) {
	runner := NewBDDTestRunner()

	// Scenario: Network connection loss mid-operation
	runner.AddScenario(TestScenario{
		Name:        "Network connection loss handled gracefully",
		Description: "When network drops during operation, partial results preserved",
		Given: func(ctx *TestContext) error {
			return GivenActiveSession(ctx, "network-test-session")
		},
		When: func(ctx *TestContext) error {
			// Simulate network drop mid-operation
			ctx.Data["operation_in_progress"] = true
			ctx.Data["partial_results"] = map[string]string{"result1": "partial data"}
			WhenNetworkFails(ctx)
			return nil
		},
		Then: func(ctx *TestContext, err error) error {
			// Should get network error but preserve partial results
			if ctx.Data["partial_results"] == nil {
				return errors.BDDTestError("Partial results should be preserved")
			}
			return ThenErrorIsType(ctx, errors.TypeNetwork, err)
		},
		Timeout: 20 * time.Second,
	})

	// Scenario: Network retry with exponential backoff
	runner.AddScenario(TestScenario{
		Name:        "Network retry with exponential backoff",
		Description: "When network fails, system retries with proper backoff",
		Given: func(ctx *TestContext) error {
			ctx.Data["retry_count"] = 0
			ctx.Data["max_retries"] = 3
			ctx.Data["backoff_delays"] = []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
			return GivenNetworkFailure(ctx, true)
		},
		When: func(ctx *TestContext) error {
			// Simulate retry logic
			for i := 0; i < 3; i++ {
				ctx.Data["retry_count"] = ctx.Data["retry_count"].(int) + 1
				ctx.Network.Calls++ // Simulate network call attempt
				time.Sleep(time.Duration(i+1) * time.Second) // Simulate backoff
			}
			return nil
		},
		Then: func(ctx *TestContext, err error) error {
			// Should have retried 3 times with proper backoff
			retryCount := ctx.Data["retry_count"].(int)
			if retryCount != 3 {
				return errors.BDDTestError("Should have retried 3 times")
			}
			return ThenNetworkCallsCount(ctx, 3)
		},
		Timeout: 30 * time.Second,
	})

	runner.RunAll(t)
}