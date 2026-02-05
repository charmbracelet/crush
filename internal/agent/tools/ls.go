package tools

import (
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/filepathext"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/permission"
)

type LSParams struct {
	Path   string   `json:"path,omitempty" description:"The path to the directory to list (defaults to current working directory)"`
	Ignore []string `json:"ignore,omitempty" description:"List of glob patterns to ignore"`
	Depth  int      `json:"depth,omitempty" description:"The maximum depth to traverse"`
}

type LSPermissionsParams struct {
	Path   string   `json:"path"`
	Ignore []string `json:"ignore"`
	Depth  int      `json:"depth"`
}

type NodeType string

const (
	NodeTypeFile      NodeType = "file"
	NodeTypeDirectory NodeType = "directory"
)

type TreeNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Type     NodeType    `json:"type"`
	Children []*TreeNode `json:"children,omitempty"`
}

type LSResponseMetadata struct {
	NumberOfFiles int  `json:"number_of_files"`
	Truncated     bool `json:"truncated"`
}

const (
	LSToolName = "ls"
	maxLSFiles = 1000
)

//go:embed ls.md
var lsDescription []byte

func NewLsTool(permissions permission.Service, workingDir string, lsConfig config.ToolLs) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		LSToolName,
		string(lsDescription),
		func(ctx context.Context, params LSParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			searchPath, err := fsext.Expand(cmp.Or(params.Path, workingDir))
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("error expanding path: %v", err)), nil
			}

			searchPath = filepathext.SmartJoin(workingDir, searchPath)

			// Check if directory is outside working directory and request permission if needed
			absWorkingDir, err := filepath.Abs(workingDir)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("error resolving working directory: %v", err)), nil
			}

			absSearchPath, err := filepath.Abs(searchPath)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("error resolving search path: %v", err)), nil
			}

			relPath, err := filepath.Rel(absWorkingDir, absSearchPath)
			if err != nil || strings.HasPrefix(relPath, "..") {
				// Directory is outside working directory, request permission
				sessionID := GetSessionFromContext(ctx)
				if sessionID == "" {
					return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for accessing directories outside working directory")
				}

				granted, err := permissions.Request(ctx,
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
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				if !granted {
					return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
				}
			}

			output, metadata, err := ListDirectoryTree(searchPath, params, lsConfig)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(output),
				metadata,
			), nil
		})
}

func ListDirectoryTree(searchPath string, params LSParams, lsConfig config.ToolLs) (string, LSResponseMetadata, error) {
	if _, err := os.Stat(searchPath); os.IsNotExist(err) {
		return "", LSResponseMetadata{}, fmt.Errorf("path does not exist: %s", searchPath)
	}

	depth, limit := lsConfig.Limits()
	maxFiles := cmp.Or(limit, maxLSFiles)
	files, truncated, err := fsext.ListDirectory(
		searchPath,
		params.Ignore,
		cmp.Or(params.Depth, depth),
		maxFiles,
	)
	if err != nil {
		return "", LSResponseMetadata{}, fmt.Errorf("error listing directory: %w", err)
	}

	metadata := LSResponseMetadata{
		NumberOfFiles: len(files),
		Truncated:     truncated,
	}
	tree := createFileTree(files, searchPath)

	var output string
	if truncated {
		output = fmt.Sprintf("There are more than %d files in the directory. Use a more specific path or use the Glob tool to find specific files. The first %[1]d files and directories are included below.\n", maxFiles)
	}
	if depth > 0 {
		output = fmt.Sprintf("The directory tree is shown up to a depth of %d. Use a higher depth and a specific path to see more levels.\n", cmp.Or(params.Depth, depth))
	}
	return output + "\n" + printTree(tree, searchPath), metadata, nil
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
			nodeType := NodeTypeFile
			if isDir {
				nodeType = NodeTypeDirectory
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
	result.WriteString(filepath.ToSlash(rootPath))
	if rootPath[len(rootPath)-1] != '/' {
		result.WriteByte('/')
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
	if node.Type == NodeTypeDirectory {
		nodeName = nodeName + "/"
	}

	fmt.Fprintf(builder, "%s- %s\n", indent, nodeName)

	if node.Type == NodeTypeDirectory && len(node.Children) > 0 {
		for _, child := range node.Children {
			printNode(builder, child, level+1)
		}
	}
}
