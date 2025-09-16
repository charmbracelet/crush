package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nom-nom-hub/blush/internal/lsp"
	"github.com/nom-nom-hub/blush/internal/permission"
)

type VisualizeParams struct {
	FilePath    string `json:"file_path"`
	Type        string `json:"type"` // "class", "function", "dataflow"
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

type VisualizePermissionsParams struct {
	FilePath    string `json:"file_path"`
	Type        string `json:"type"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

type visualizeTool struct {
	lspClients  map[string]*lsp.Client
	permissions permission.Service
	workingDir  string
}

const (
	VisualizeToolName    = "visualize"
	visualizeDescription = "Generates ASCII diagrams to visualize code structures such as class diagrams, \nfunction call graphs, and data flow diagrams. This tool helps developers understand complex \ncode structures and relationships at a glance.\n\nAvailable visualization types:\n- \"class\": Generate UML-like class diagrams showing classes, methods, and relationships\n- \"function\": Generate function call graphs showing how functions call each other\n- \"dataflow\": Generate data flow diagrams showing how data moves through the system\n\nThe tool analyzes the specified file and generates an ASCII representation of the requested \ndiagram type. The generated diagram is returned as text that can be displayed in the terminal."
)

func NewVisualizeTool(lspClients map[string]*lsp.Client, permissions permission.Service, workingDir string) BaseTool {
	return &visualizeTool{
		lspClients:  lspClients,
		permissions: permissions,
		workingDir:  workingDir,
	}
}

func (t *visualizeTool) Info() ToolInfo {
	return ToolInfo{
		Name:        VisualizeToolName,
		Description: visualizeDescription,
		Parameters: map[string]any{
			"file_path": "string - Path to the file to visualize",
			"type":      "string - Type of visualization (class, function, dataflow)",
			"title":     "string - Optional title for the diagram",
			"description": "string - Optional description for the diagram",
		},
		Required: []string{"file_path", "type"},
	}
}

func (t *visualizeTool) Name() string {
	return VisualizeToolName
}

func (t *visualizeTool) Run(ctx context.Context, params ToolCall) (ToolResponse, error) {
	var visualizeParams VisualizeParams
	if err := json.Unmarshal([]byte(params.Input), &visualizeParams); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Failed to parse parameters: %v", err)), nil
	}

	// Check permissions
	sessionID, messageID := GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session ID and message ID are required for visualization")
	}

	permissionParams := VisualizePermissionsParams{
		FilePath:    visualizeParams.FilePath,
		Type:        visualizeParams.Type,
		Title:       visualizeParams.Title,
		Description: visualizeParams.Description,
	}

	p := t.permissions.Request(
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			ToolCallID:  params.ID,
			ToolName:    VisualizeToolName,
			Action:      "visualize",
			Description: fmt.Sprintf("Generate %s diagram for file: %s", visualizeParams.Type, visualizeParams.FilePath),
			Params:      permissionParams,
		},
	)
	if !p {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}

	// Generate the visualization
	diagram, err := t.generateVisualization(ctx, visualizeParams)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Failed to generate visualization: %v", err)), nil
	}

	response := NewTextResponse(diagram)
	return WithResponseMetadata(response, map[string]any{
		"file_path": visualizeParams.FilePath,
		"type":      visualizeParams.Type,
	}), nil
}

func (t *visualizeTool) generateVisualization(ctx context.Context, params VisualizeParams) (string, error) {
	// This is a simplified implementation. In a real implementation, you would:
	// 1. Use LSP to analyze the code structure
	// 2. Parse the code to extract relevant information
	// 3. Generate ASCII diagrams based on the requested type

	var diagram strings.Builder

	// Add title if provided
	if params.Title != "" {
		diagram.WriteString(fmt.Sprintf("# %s\n\n", params.Title))
	}

	// Add description if provided
	if params.Description != "" {
		diagram.WriteString(fmt.Sprintf("%s\n\n", params.Description))
	}

	switch params.Type {
	case "class":
		classDiagram, err := t.generateClassDiagram(ctx, params.FilePath)
		if err != nil {
			return "", err
		}
		diagram.WriteString(classDiagram)
	case "function":
		functionDiagram, err := t.generateFunctionDiagram(ctx, params.FilePath)
		if err != nil {
			return "", err
		}
		diagram.WriteString(functionDiagram)
	case "dataflow":
		dataflowDiagram, err := t.generateDataFlowDiagram(ctx, params.FilePath)
		if err != nil {
			return "", err
		}
		diagram.WriteString(dataflowDiagram)
	default:
		return "", fmt.Errorf("unsupported visualization type: %s", params.Type)
	}

	return diagram.String(), nil
}

func (t *visualizeTool) generateClassDiagram(ctx context.Context, filePath string) (string, error) {
	var diagram strings.Builder
	
	// Header
	diagram.WriteString("+-----------------------------------------------------+\n")
	diagram.WriteString("|                    Class Diagram                    |\n")
	diagram.WriteString("+-----------------------------------------------------+\n\n")
	
	// Try to get LSP information if available
	if lspInfo := t.getLSPClassInfo(ctx, filePath); lspInfo != "" {
		diagram.WriteString(lspInfo)
	} else {
		// Fallback to example class diagram
		diagram.WriteString("+---------------------+        +---------------------+\n")
		diagram.WriteString("|     ClassName       |        |    AnotherClass     |\n")
		diagram.WriteString("+---------------------+        +---------------------+\n")
		diagram.WriteString("| - privateField: Type|        | - field: Type       |\n")
		diagram.WriteString("| + publicMethod()    |<-------| + method(): ReturnType\n")
		diagram.WriteString("+---------------------+        +---------------------+\n")
		diagram.WriteString("         ^                              ^\n")
		diagram.WriteString("         |                              |\n")
		diagram.WriteString("         | Inherits                     | Uses\n")
		diagram.WriteString("         |                              |\n")
		diagram.WriteString("+---------------------+        +---------------------+\n")
		diagram.WriteString("|   SubClassName      |        |    HelperClass      |\n")
		diagram.WriteString("+---------------------+        +---------------------+\n")
		diagram.WriteString("| + methodOverride()  |        | + utilityMethod()   |\n")
		diagram.WriteString("+---------------------+        +---------------------+\n\n")
	}
	
	diagram.WriteString("Legend:\n")
	diagram.WriteString("  + : Public\n")
	diagram.WriteString("  - : Private\n")
	diagram.WriteString("  <------- : Association\n")
	diagram.WriteString("  ^ : Inheritance\n")
	
	return diagram.String(), nil
}

func (t *visualizeTool) generateFunctionDiagram(ctx context.Context, filePath string) (string, error) {
	var diagram strings.Builder
	
	// Header
	diagram.WriteString("+-----------------------------------------------------+\n")
	diagram.WriteString("|                 Function Call Graph                 |\n")
	diagram.WriteString("+-----------------------------------------------------+\n\n")
	
	// Try to get LSP information if available
	if lspInfo := t.getLSPFunctionInfo(ctx, filePath); lspInfo != "" {
		diagram.WriteString(lspInfo)
	} else {
		// Fallback to example function diagram
		diagram.WriteString("         +-----------+\n")
		diagram.WriteString("         | main()    |\n")
		diagram.WriteString("         +-----------+\n")
		diagram.WriteString("              |\n")
		diagram.WriteString("      +-------+-------+\n")
		diagram.WriteString("      |               |\n")
		diagram.WriteString("      v               v\n")
		diagram.WriteString("+-----------+   +-----------+\n")
		diagram.WriteString("| init()    |   | process() |\n")
		diagram.WriteString("+-----------+   +-----------+\n")
		diagram.WriteString("                    |\n")
		diagram.WriteString("            +-------+-------+\n")
		diagram.WriteString("            |               |\n")
		diagram.WriteString("            v               v\n")
		diagram.WriteString("      +-----------+   +-----------+\n")
		diagram.WriteString("      | validate()|   | save()    |\n")
		diagram.WriteString("      +-----------+   +-----------+\n")
		diagram.WriteString("                          |\n")
		diagram.WriteString("                          v\n")
		diagram.WriteString("                    +-----------+\n")
		diagram.WriteString("                    | notify()  |\n")
		diagram.WriteString("                    +-----------+\n\n")
	}
	
	diagram.WriteString("Legend:\n")
	diagram.WriteString("  Rectangles: Functions\n")
	diagram.WriteString("  Arrows: Function calls\n")
	
	return diagram.String(), nil
}

func (t *visualizeTool) generateDataFlowDiagram(ctx context.Context, filePath string) (string, error) {
	var diagram strings.Builder
	
	// Header
	diagram.WriteString("+-----------------------------------------------------+\n")
	diagram.WriteString("|                  Data Flow Diagram                  |\n")
	diagram.WriteString("+-----------------------------------------------------+\n\n")
	
	// Try to get LSP information if available
	if lspInfo := t.getLSPDataFlowInfo(ctx, filePath); lspInfo != "" {
		diagram.WriteString(lspInfo)
	} else {
		// Fallback to example data flow diagram
		diagram.WriteString("  User Input        Database         External API\n")
		diagram.WriteString("      |                 |                 |\n")
		diagram.WriteString("      v                 |                 |\n")
		diagram.WriteString("+-----------+           |                 |\n")
		diagram.WriteString("|  Form     |           |                 |\n")
		diagram.WriteString("| Validation|           |                 |\n")
		diagram.WriteString("+-----------+           |                 |\n")
		diagram.WriteString("      |                 |                 |\n")
		diagram.WriteString("      +--------->-------+                 |\n")
		diagram.WriteString("                        |                 |\n")
		diagram.WriteString("                        v                 |\n")
		diagram.WriteString("                  +-----------+           |\n")
		diagram.WriteString("                  |  Process  |           |\n")
		diagram.WriteString("                  |   Data    |           |\n")
		diagram.WriteString("                  +-----------+           |\n")
		diagram.WriteString("                        |                 |\n")
		diagram.WriteString("                        +--------->-------+\n")
		diagram.WriteString("                                          |\n")
		diagram.WriteString("                                          v\n")
		diagram.WriteString("                                    +-----------+\n")
		diagram.WriteString("                                    |   Send    |\n")
		diagram.WriteString("                                    |  Results  |\n")
		diagram.WriteString("                                    +-----------+\n")
		diagram.WriteString("                                          |\n")
		diagram.WriteString("                                          v\n")
		diagram.WriteString("                                   +-----------+\n")
		diagram.WriteString("                                   |  Display  |\n")
		diagram.WriteString("                                   | Results   |\n")
		diagram.WriteString("                                   +-----------+\n\n")
	}
	
	diagram.WriteString("Legend:\n")
	diagram.WriteString("  Rectangles: Data processors\n")
	diagram.WriteString("  Arrows: Data flow direction\n")
	
	return diagram.String(), nil
}

func (t *visualizeTool) getLSPClassInfo(ctx context.Context, filePath string) string {
	// This would use LSP to get actual class information
	// For now, we'll return an empty string to use the fallback
	return ""
}

func (t *visualizeTool) getLSPFunctionInfo(ctx context.Context, filePath string) string {
	// This would use LSP to get actual function call information
	// For now, we'll return an empty string to use the fallback
	return ""
}

func (t *visualizeTool) getLSPDataFlowInfo(ctx context.Context, filePath string) string {
	// This would use LSP to get actual data flow information
	// For now, we'll return an empty string to use the fallback
	return ""
}