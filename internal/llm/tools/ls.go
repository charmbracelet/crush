package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/permission"
)

type LSParams struct {
	Path   string   `json:"path"`
	Ignore []string `json:"ignore"`
}

type LSPermissionsParams struct {
	Path   string   `json:"path"`
	Ignore []string `json:"ignore"`
}

type TreeNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Type     string      `json:"type"` // "file" or "directory"
	Children []*TreeNode `json:"children,omitempty"`
}

type LSResponseMetadata struct {
	NumberOfFiles int  `json:"number_of_files"`
	Truncated     bool `json:"truncated"`
}

type lsTool struct {
	workingDir  string
	permissions permission.Service
}

const (
	LSToolName = "ls"
	MaxLSFiles = 1000
)

//go:embed ls.md
var lsDescription []byte

func NewLsTool(permissions permission.Service, workingDir string) BaseTool {
	return &lsTool{
		workingDir:  workingDir,
		permissions: permissions,
	}
}

func (l *lsTool) Name() string {
	return LSToolName
}

func (l *lsTool) Info() ToolInfo {
	return ToolInfo{
		Name:        LSToolName,
		Description: string(lsDescription),
		Parameters: map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The path to the directory to list (defaults to current working directory)",
			},
			"ignore": map[string]any{
				"type":        "array",
				"description": "List of glob patterns to ignore",
				"items": map[string]any{
					"type": "string",
				},
			},
		},
		Required: []string{"path"},
	}
}

func (l *lsTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params LSParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	searchPath := params.Path
	if searchPath == "" {
		searchPath = l.workingDir
	}

	var err error
	searchPath, err = fsext.Expand(searchPath)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("error expanding path: %w", err)
	}

	if !filepath.IsAbs(searchPath) {
		searchPath = filepath.Join(l.workingDir, searchPath)
	}

	// Check if directory is outside working directory and request permission if needed
	absWorkingDir, err := filepath.Abs(l.workingDir)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("error resolving working directory: %w", err)
	}

	absSearchPath, err := filepath.Abs(searchPath)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("error resolving search path: %w", err)
	}

	relPath, err := filepath.Rel(absWorkingDir, absSearchPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		// Directory is outside working directory, request permission
		sessionID, messageID := GetContextValues(ctx)
		if sessionID == "" || messageID == "" {
			return ToolResponse{}, fmt.Errorf("session ID and message ID are required for accessing directories outside working directory")
		}

		granted := l.permissions.Request(
			permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        absSearchPath,
				ToolCallID:  call.ID,
				ToolName:    LSToolName,
				Action:      "list",
				Description: fmt.Sprintf("List directory outside working directory: %s", absSearchPath),
				Params:      LSPermissionsParams(params),
			},
		)

		if !granted {
			return ToolResponse{}, permission.ErrorPermissionDenied
		}
	}

	output, err := ListDirectoryTree(searchPath, params.Ignore)
	if err != nil {
		return ToolResponse{}, err
	}

	// Get file count for metadata
	files, truncated, err := fsext.ListDirectory(searchPath, params.Ignore, MaxLSFiles)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("error listing directory for metadata: %w", err)
	}

	return WithResponseMetadata(
		NewTextResponse(output),
		LSResponseMetadata{
			NumberOfFiles: len(files),
			Truncated:     truncated,
		},
	), nil
}

func ListDirectoryTree(searchPath string, ignore []string) (string, error) {
	if _, err := os.Stat(searchPath); os.IsNotExist(err) {
		return "", fmt.Errorf("path does not exist: %s", searchPath)
	}

	files, truncated, err := fsext.ListDirectory(searchPath, ignore, MaxLSFiles)
	if err != nil {
		return "", fmt.Errorf("error listing directory: %w", err)
	}

	tree := createFileTree(files, searchPath)
	output := printTree(tree, searchPath)

	if truncated {
		output = fmt.Sprintf("There are more than %d files in the directory. Use a more specific path or use the Glob tool to find specific files. The first %d files and directories are included below:\n\n%s", MaxLSFiles, MaxLSFiles, output)
	}

	return output, nil
}

func createFileTree(sortedPaths []string, rootPath string) []*TreeNode {
	root := []*TreeNode{}
	pathMap := make(map[string]*TreeNode)

	for _, path := range sortedPaths {
		relativePath := strings.TrimPrefix(path, rootPath)
		parts := strings.Split(relativePath, string(filepath.Separator))
		currentPath := ""
		var parentPath string

		var cleanParts []string
		for _, part := range parts {
			if part != "" {
				cleanParts = append(cleanParts, part)
			}
		}
		parts = cleanParts

		if len(parts) == 0 {
			continue
		}

		for i, part := range parts {
			if currentPath == "" {
				currentPath = part
			} else {
				currentPath = filepath.Join(currentPath, part)
			}

			if _, exists := pathMap[currentPath]; exists {
				parentPath = currentPath
				continue
			}

			isLastPart := i == len(parts)-1
			isDir := !isLastPart || strings.HasSuffix(relativePath, string(filepath.Separator))
			nodeType := "file"
			if isDir {
				nodeType = "directory"
			}
			newNode := &TreeNode{
				Name:     part,
				Path:     currentPath,
				Type:     nodeType,
				Children: []*TreeNode{},
			}

			pathMap[currentPath] = newNode

			if i > 0 && parentPath != "" {
				if parent, ok := pathMap[parentPath]; ok {
					parent.Children = append(parent.Children, newNode)
				}
			} else {
				root = append(root, newNode)
			}

			parentPath = currentPath
		}
	}

	return root
}

func printTree(tree []*TreeNode, rootPath string) string {
	var result strings.Builder

	result.WriteString("- ")
	result.WriteString(rootPath)
	if rootPath[len(rootPath)-1] != '/' {
		result.WriteByte(filepath.Separator)
	}
	result.WriteByte('\n')

	for _, node := range tree {
		printNode(&result, node, 1)
	}

	return result.String()
}

func printNode(builder *strings.Builder, node *TreeNode, level int) {
	indent := strings.Repeat("  ", level)

	nodeName := node.Name
	if node.Type == "directory" {
		nodeName = nodeName + string(filepath.Separator)
	}

	fmt.Fprintf(builder, "%s- %s\n", indent, nodeName)

	if node.Type == "directory" && len(node.Children) > 0 {
		for _, child := range node.Children {
			printNode(builder, child, level+1)
		}
	}
}
