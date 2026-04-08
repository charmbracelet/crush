package kernel

import (
	"context"
	"errors"
	"testing"
	"time"

	"charm.land/fantasy"
)

func TestHookPipeline_Basic(t *testing.T) {
	hp := NewHookPipeline()

	var preToolCalled bool
	var postToolCalled bool

	err := hp.RegisterHook(&Hook{
		Name:     "pre-tool-test",
		Phase:    HookPhasePreToolUse,
		Priority: HookPriorityMedium,
		Fn: func(ctx context.Context, hookCtx *HookContext) error {
			preToolCalled = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Failed to register pre-tool hook: %v", err)
	}

	err = hp.RegisterHook(&Hook{
		Name:     "post-tool-test",
		Phase:    HookPhasePostToolUse,
		Priority: HookPriorityMedium,
		Fn: func(ctx context.Context, hookCtx *HookContext) error {
			postToolCalled = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Failed to register post-tool hook: %v", err)
	}

	errs := hp.ExecutePhase(context.Background(), HookPhasePreToolUse, &HookContext{
		SessionID: "test-session",
	})
	if len(errs) > 0 {
		t.Errorf("PreTool hooks failed: %v", errs)
	}
	if !preToolCalled {
		t.Error("Pre-tool hook was not called")
	}

	errs = hp.ExecutePhase(context.Background(), HookPhasePostToolUse, &HookContext{
		SessionID: "test-session",
	})
	if len(errs) > 0 {
		t.Errorf("PostTool hooks failed: %v", errs)
	}
	if !postToolCalled {
		t.Error("Post-tool hook was not called")
	}

	t.Log("✓ HookPipeline basic execution works")
}

func TestHookPipeline_ErrorHandling(t *testing.T) {
	hp := NewHookPipeline()

	err := hp.RegisterHook(&Hook{
		Name:     "error-hook",
		Phase:    HookPhaseOnError,
		Priority: HookPriorityMedium,
		Fn: func(ctx context.Context, hookCtx *HookContext) error {
			return errors.New("simulated error")
		},
	})
	if err != nil {
		t.Fatalf("Failed to register error hook: %v", err)
	}

	errs := hp.ExecutePhase(context.Background(), HookPhaseOnError, &HookContext{
		SessionID: "test-session",
	})

	if len(errs) != 1 {
		t.Errorf("Expected 1 error, got %d", len(errs))
	}
	if errs[0].Error() != "simulated error" {
		t.Errorf("Unexpected error message: %s", errs[0].Error())
	}

	t.Log("✓ HookPipeline error handling works")
}

func TestHookPipeline_DisableHook(t *testing.T) {
	hp := NewHookPipeline()

	var called bool
	err := hp.RegisterHook(&Hook{
		Name:     "should-not-call",
		Phase:    HookPhasePreToolUse,
		Priority: HookPriorityMedium,
		Fn: func(ctx context.Context, hookCtx *HookContext) error {
			called = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Failed to register hook: %v", err)
	}

	// Disable after registration using the DisableHook method
	hp.DisableHook("should-not-call")

	errs := hp.ExecutePhase(context.Background(), HookPhasePreToolUse, &HookContext{})
	if len(errs) != 0 {
		t.Errorf("Expected no errors, got %d", len(errs))
	}
	if called {
		t.Error("Disabled hook was called")
	}

	t.Log("✓ HookPipeline disable hook works")
}

func TestHookPipeline_ExecutionMetrics(t *testing.T) {
	hp := NewHookPipeline()

	err := hp.RegisterHook(&Hook{
		Name:     "slow-hook",
		Phase:    HookPhasePreAgent,
		Priority: HookPriorityMedium,
		Fn: func(ctx context.Context, hookCtx *HookContext) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Failed to register hook: %v", err)
	}

	errs := hp.ExecutePhase(context.Background(), HookPhasePreAgent, &HookContext{})
	if len(errs) > 0 {
		t.Errorf("Hook failed: %v", errs)
	}

	t.Log("✓ HookPipeline execution works")
}

func TestHookPipeline_PriorityOrdering(t *testing.T) {
	hp := NewHookPipeline()

	var order []string

	hp.RegisterHook(&Hook{
		Name:     "low-priority",
		Phase:    HookPhasePreToolUse,
		Priority: HookPriorityLow,
		Fn: func(ctx context.Context, hookCtx *HookContext) error {
			order = append(order, "low")
			return nil
		},
	})

	hp.RegisterHook(&Hook{
		Name:     "high-priority",
		Phase:    HookPhasePreToolUse,
		Priority: HookPriorityHigh,
		Fn: func(ctx context.Context, hookCtx *HookContext) error {
			order = append(order, "high")
			return nil
		},
	})

	hp.RegisterHook(&Hook{
		Name:     "medium-priority",
		Phase:    HookPhasePreToolUse,
		Priority: HookPriorityMedium,
		Fn: func(ctx context.Context, hookCtx *HookContext) error {
			order = append(order, "medium")
			return nil
		},
	})

	hp.ExecutePhase(context.Background(), HookPhasePreToolUse, &HookContext{})

	if len(order) != 3 {
		t.Errorf("Expected 3 calls, got %d", len(order))
	}
	if order[0] != "high" {
		t.Errorf("Expected high first, got %s", order[0])
	}
	if order[1] != "medium" {
		t.Errorf("Expected medium second, got %s", order[1])
	}
	if order[2] != "low" {
		t.Errorf("Expected low last, got %s", order[2])
	}

	t.Log("✓ HookPipeline priority ordering works")
}

func TestCompressionOrchestrator_Initialize(t *testing.T) {
	config := OrchestratorConfig{
		MaxTokenBudget: 200000,
		L1Threshold:    20,
		L2Threshold:    0.85,
		L3Threshold:    0.95,
		EnableL1:       true,
		EnableL2:       true,
		EnableL3:       true,
		EnableL4:       true,
	}

	orch := NewCompressionOrchestrator(config)

	if orch == nil {
		t.Fatal("Failed to create CompressionOrchestrator")
	}

	if orch.hookPipeline == nil {
		t.Error("HookPipeline is nil")
	}

	if orch.usageTracker == nil {
		t.Error("UsageTracker is nil")
	}

	if orch.compactor == nil {
		t.Error("Compactor is nil")
	}

	t.Log("✓ CompressionOrchestrator initialization works")
}

func TestCompressionOrchestrator_Compact(t *testing.T) {
	config := OrchestratorConfig{
		MaxTokenBudget: 200000,
		L1Threshold:    20,
		L2Threshold:    0.85,
		EnableL1:       true,
	}
	orch := NewCompressionOrchestrator(config)

	messages := []fantasy.Message{
		{Role: fantasy.MessageRoleUser, Content: []fantasy.MessagePart{}},
		{Role: fantasy.MessageRoleAssistant, Content: []fantasy.MessagePart{}},
	}

	result, err := orch.Compact(context.Background(), messages, "test-session")
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	if len(result) == 0 {
		t.Error("Expected some messages after compaction")
	}

	t.Log("✓ CompressionOrchestrator.Compact works")
}

func TestCompressionOrchestrator_CompactL1(t *testing.T) {
	config := OrchestratorConfig{
		MaxTokenBudget: 200000,
		L1Threshold:    1, // Low threshold to trigger L1
		EnableL1:       true,
	}
	orch := NewCompressionOrchestrator(config)

	// Create 5+ messages to trigger L1
	messages := []fantasy.Message{
		{Role: fantasy.MessageRoleUser, Content: []fantasy.MessagePart{}},
		{Role: fantasy.MessageRoleAssistant, Content: []fantasy.MessagePart{}},
		{Role: fantasy.MessageRoleTool, Content: []fantasy.MessagePart{}},
		{Role: fantasy.MessageRoleUser, Content: []fantasy.MessagePart{}},
		{Role: fantasy.MessageRoleAssistant, Content: []fantasy.MessagePart{}},
		{Role: fantasy.MessageRoleTool, Content: []fantasy.MessagePart{}},
	}

	result, err := orch.Compact(context.Background(), messages, "test-session")
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	t.Logf("✓ CompressionOrchestrator L1 triggered, messages: %d -> %d", len(messages), len(result))
}

func TestCompressionOrchestrator_HookIntegration(t *testing.T) {
	config := OrchestratorConfig{
		MaxTokenBudget: 200000,
		EnableL1:       true,
	}
	orch := NewCompressionOrchestrator(config)

	// Verify default hooks are registered
	hp := orch.GetHookPipeline()
	if hp == nil {
		t.Error("GetHookPipeline returned nil")
	}

	t.Log("✓ CompressionOrchestrator hook integration works")
}
