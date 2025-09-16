package tools

import (
	"testing"
)

func TestDepsTool(t *testing.T) {
	// This is a placeholder test to ensure the deps tool compiles correctly
	// In a real implementation, we would add actual tests here
	tool := &depsTool{}
	if tool.Name() != DepsToolName {
		t.Errorf("Expected tool name to be %s, got %s", DepsToolName, tool.Name())
	}
}

func TestAPITool(t *testing.T) {
	// This is a placeholder test to ensure the API tool compiles correctly
	// In a real implementation, we would add actual tests here
	tool := &apiTool{}
	if tool.Name() != APIToolName {
		t.Errorf("Expected tool name to be %s, got %s", APIToolName, tool.Name())
	}
}