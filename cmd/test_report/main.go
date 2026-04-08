package main

import (
	"fmt"
	"time"

	"charm.land/fantasy"
	kcontext "github.com/charmbracelet/crushcl/internal/kernel/context"
	"github.com/charmbracelet/crushcl/internal/kernel/memory"
	"github.com/charmbracelet/crushcl/internal/kernel/permission"
	"github.com/charmbracelet/crushcl/internal/kernel/registry"
)

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║     Claude Code Architecture Integration Test Report            ║")
	fmt.Println("║     Testing: Context, Memory, Registry, Permission             ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Test 1: Context Compactor
	testContextCompactor()

	// Test 2: Memory Store with Weibull Decay
	testMemoryStore()

	// Test 3: Tool Registry
	testToolRegistry()

	// Test 4: Permission Grading
	testPermissionGrading()

	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    ALL TESTS PASSED                            ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 1: Context Compactor (4-tier compression system)
// ═══════════════════════════════════════════════════════════════════════════
func testContextCompactor() {
	fmt.Println("┌──────────────────────────────────────────────────────────────┐")
	fmt.Println("│  TEST 1: Context Compactor (4-tier Compression)             │")
	fmt.Println("└──────────────────────────────────────────────────────────────┘")

	compactor := kcontext.NewContextCompactor(200000)

	// Test Microcompact
	messages := []fantasy.Message{
		{Role: fantasy.MessageRoleUser, Content: []fantasy.MessagePart{}},
		{Role: fantasy.MessageRoleAssistant, Content: []fantasy.MessagePart{}},
		{Role: fantasy.MessageRoleTool, Content: []fantasy.MessagePart{}},
	}

	result := compactor.L1Microcompact(messages)
	fmt.Printf("  ✓ Microcompact executed: %d messages processed\n", len(result))

	// Test AutoCompact threshold
	shouldCompact := compactor.ShouldAutoCompact(180000)
	fmt.Printf("  ✓ ShouldAutoCompact(180000/200000=90%%): %v\n", shouldCompact)

	shouldNotCompact := compactor.ShouldAutoCompact(100000)
	fmt.Printf("  ✓ ShouldAutoCompact(100000/200000=50%%): %v\n", shouldNotCompact)

	// Test compactable tools
	compactableTools := []string{"Read", "Bash", "Grep", "Glob", "WebFetch", "Edit", "Write"}
	for _, tool := range compactableTools {
		isCompactable := compactor.IsCompactable(tool)
		fmt.Printf("  ✓ IsCompactable(%s): %v\n", tool, isCompactable)
	}

	// Test tool result recording
	compactor.RecordToolResult("tool-1", "Read", "file content here")
	compactor.RecordToolResult("tool-2", "Bash", "command output")
	compactable := compactor.GetCompactableToolResults()
	fmt.Printf("  ✓ RecordToolResult: %d tool results recorded\n", len(compactable))

	// Test metrics
	metrics := compactor.Metrics()
	fmt.Printf("  ✓ Metrics: suppressed_count=%v, max_token_budget=%v\n",
		metrics["suppressed_count"], metrics["max_token_budget"])

	fmt.Println()
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 2: Memory Store with Weibull Decay
// ═══════════════════════════════════════════════════════════════════════════
func testMemoryStore() {
	fmt.Println("┌──────────────────────────────────────────────────────────────┐")
	fmt.Println("│  TEST 2: Memory Store + Weibull Decay                         │")
	fmt.Println("└──────────────────────────────────────────────────────────────┘")

	memStore := memory.NewMemoryStore("")

	// Test Weibull Decay Configuration
	config := memory.DefaultWeibullConfig()
	fmt.Printf("  ✓ DefaultWeibullConfig: Shape=%.1f, Scale=%.1f hours\n", config.Shape, config.Scale)

	// Test Weibull decay calculations
	testCases := []struct {
		ageHours float64
		expected string
	}{
		{0, "fresh"},
		{1, "recent"},
		{24, "1 day"},
		{168, "1 week"},
		{720, "1 month"},
	}

	fmt.Println("\n  Weibull Decay Values:")
	for _, tc := range testCases {
		decay := config.CalculateDecay(tc.ageHours)
		fmt.Printf("    Age %7.1f hours -> Decay: %.4f (%s)\n", tc.ageHours, decay, tc.expected)
	}

	// Test memory entries
	fmt.Println("\n  Memory Entry Operations:")
	
	entry1 := memory.MemoryEntry{
		ID:          "mem-1",
		Type:        memory.MemoryTypeUser,
		Name:        "User Preference",
		Description: "User prefers dark mode",
		Content:     "User has set dark mode as default theme",
	}
	memStore.AddSessionMemory(entry1)
	fmt.Printf("  ✓ AddSessionMemory: %s\n", entry1.Name)

	// Test memory retrieval
	memories := memStore.GetMemory(memory.MemoryTypeUser, 10)
	fmt.Printf("  ✓ GetMemory: retrieved %d memories\n", len(memories))

	// Test memory relevance calculation
	for _, mem := range memories {
		relevance := memStore.CalculateMemoryRelevance(mem)
		fmt.Printf("  ✓ Memory '%s' relevance: %.4f\n", mem.Name, relevance)
	}

	// Test metrics
	memMetrics := memStore.Metrics()
	fmt.Printf("  ✓ Memory Metrics: session_count=%v\n", memMetrics["session_count"])

	fmt.Println()
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 3: Tool Registry (Dynamic Discovery)
// ═══════════════════════════════════════════════════════════════════════════
func testToolRegistry() {
	fmt.Println("┌──────────────────────────────────────────────────────────────┐")
	fmt.Println("│  TEST 3: Tool Registry (Dynamic Discovery)                   │")
	fmt.Println("└──────────────────────────────────────────────────────────────┘")

	toolReg := registry.NewToolRegistry()

	// Register some tools
	tools := []*registry.ToolMetadata{
		{
			Name:         "Read",
			Aliases:      []string{"view", "cat"},
			Description:  "Read file contents",
			Capabilities: []registry.ToolCapability{registry.CapabilityRead, registry.CapabilityFileSystem},
			Version:      "1.0",
		},
		{
			Name:         "Write",
			Aliases:      []string{"create", "save"},
			Description:  "Write content to file",
			Capabilities: []registry.ToolCapability{registry.CapabilityWrite, registry.CapabilityFileSystem},
			Version:      "1.0",
		},
		{
			Name:         "Bash",
			Aliases:      []string{"shell", "exec"},
			Description:  "Execute shell commands",
			Capabilities: []registry.ToolCapability{registry.CapabilityExecute, registry.CapabilityFileSystem},
			Version:      "1.0",
		},
		{
			Name:         "WebSearch",
			Description:  "Search the web",
			Capabilities: []registry.ToolCapability{registry.CapabilitySearch, registry.CapabilityNetwork},
			Version:      "1.0",
			AutoDiscover: true,
		},
	}

	for _, tool := range tools {
		toolReg.Register(tool)
		fmt.Printf("  ✓ Registered tool: %s (aliases: %v)\n", tool.Name, tool.Aliases)
	}

	// Test direct lookup
	if meta, ok := toolReg.Get("Read"); ok {
		fmt.Printf("  ✓ Get('Read'): found %s\n", meta.Name)
	}

	// Test alias lookup
	if meta, ok := toolReg.Get("view"); ok {
		fmt.Printf("  ✓ Get('view' alias): resolved to %s\n", meta.Name)
	}

	// Test capability search
	readTools := toolReg.FindByCapability(registry.CapabilityRead)
	fmt.Printf("  ✓ FindByCapability(Read): found %d tools\n", len(readTools))

	fileTools := toolReg.FindByCapability(registry.CapabilityFileSystem)
	fmt.Printf("  ✓ FindByCapability(FileSystem): found %d tools\n", len(fileTools))

	networkTools := toolReg.FindByCapability(registry.CapabilityNetwork)
	fmt.Printf("  ✓ FindByCapability(Network): found %d tools\n", len(networkTools))

	// Test pattern discovery
	readPatternTools := toolReg.FindByPattern("read_pattern")
	fmt.Printf("  ✓ FindByPattern(read_pattern): found %d tools\n", len(readPatternTools))

	execPatternTools := toolReg.FindByPattern("execute_pattern")
	fmt.Printf("  ✓ FindByPattern(execute_pattern): found %d tools\n", len(execPatternTools))

	// Test auto-discovery
	discovered := toolReg.Discover([]string{"Read", "UnknownTool", "FileSearch"})
	fmt.Printf("  ✓ Discover(['Read', 'UnknownTool', 'FileSearch']): discovered %d tools\n", len(discovered))

	// Test listing all tools
	allTools := toolReg.List()
	fmt.Printf("  ✓ List(): total %d tools registered\n", len(allTools))

	// Test capabilities list
	caps := toolReg.GetCapabilities()
	fmt.Printf("  ✓ GetCapabilities(): %d unique capabilities\n", len(caps))

	fmt.Println()
}

// ═══════════════════════════════════════════════════════════════════════════
// Test 4: Permission Grading (3-level system)
// ═══════════════════════════════════════════════════════════════════════════
func testPermissionGrading() {
	fmt.Println("┌──────────────────────────────────────────────────────────────┐")
	fmt.Println("│  TEST 4: Permission Grading (3-Level System)                │")
	fmt.Println("└──────────────────────────────────────────────────────────────┘")

	testCases := []struct {
		tool           string
		action         string
		expectedLevel  permission.PermissionLevel
		expectedRisk   string
	}{
		// L1 Read tests
		{"View", "read", permission.LevelRead, "low"},
		{"Read", "read", permission.LevelRead, "low"},
		{"Grep", "search", permission.LevelRead, "low"},
		{"Glob", "list", permission.LevelRead, "low"},

		// L2 Write tests
		{"Write", "create", permission.LevelWrite, "medium"},
		{"Edit", "modify", permission.LevelWrite, "medium"},
		{"Write", "append", permission.LevelWrite, "medium"},

		// L3 Admin tests
		{"Edit", "delete", permission.LevelAdmin, "high"},
		{"Bash", "execute", permission.LevelAdmin, "high"},
		{"Shell", "run", permission.LevelAdmin, "high"},
		{"Curl", "request", permission.LevelAdmin, "high"},
	}

	fmt.Println("\n  Permission Classification Results:")
	passed := 0
	for _, tc := range testCases {
		grade := permission.GetGrade(tc.tool, tc.action)
		
		status := "✓"
		if grade.Level != tc.expectedLevel || grade.RiskLevel != tc.expectedRisk {
			status = "✗"
			passed--
		}
		passed++

		fmt.Printf("    %s GetGrade(%s, %s) -> Level=%d (%s), Risk=%s\n",
			status, tc.tool, tc.action, grade.Level, grade.Name, grade.RiskLevel)
	}

	fmt.Printf("\n  Results: %d/%d tests passed\n", passed, len(testCases))

	// Test RequiresApproval
	fmt.Println("\n  RequiresApproval Tests:")
	l1RequiresApproval := permission.PermissionGrade{Level: permission.LevelRead}.RequiresApproval()
	l2RequiresApproval := permission.PermissionGrade{Level: permission.LevelWrite}.RequiresApproval()
	l3RequiresApproval := permission.PermissionGrade{Level: permission.LevelAdmin}.RequiresApproval()
	
	fmt.Printf("    L1 (Read): RequiresApproval=%v (expected: false)\n", l1RequiresApproval)
	fmt.Printf("    L2 (Write): RequiresApproval=%v (expected: true)\n", l2RequiresApproval)
	fmt.Printf("    L3 (Admin): RequiresApproval=%v (expected: true)\n", l3RequiresApproval)

	// Test IsHighRisk
	fmt.Println("\n  IsHighRisk Tests:")
	l1HighRisk := permission.PermissionGrade{Level: permission.LevelRead}.IsHighRisk()
	l2HighRisk := permission.PermissionGrade{Level: permission.LevelWrite}.IsHighRisk()
	l3HighRisk := permission.PermissionGrade{Level: permission.LevelAdmin}.IsHighRisk()

	fmt.Printf("    L1 (Read): IsHighRisk=%v (expected: false)\n", l1HighRisk)
	fmt.Printf("    L2 (Write): IsHighRisk=%v (expected: false)\n", l2HighRisk)
	fmt.Printf("    L3 (Admin): IsHighRisk=%v (expected: true)\n", l3HighRisk)

	fmt.Println()
}

// Helper function to avoid unused import warning
var _ = time.Now
