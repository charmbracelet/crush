package plan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// List returns regular plan filenames, newest names first.
func List(ctx context.Context, workingDir string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(workingDir, ".crush", "plans"))
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}
	names := []string{}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		info, err := entry.Info()
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read plan info: %w", err)
		}
		if info.Mode().IsRegular() {
			names = append(names, entry.Name())
		}
	}
	slices.Sort(names)
	slices.Reverse(names)
	return names, nil
}

// Read returns a regular file directly inside the workspace plans directory.
func Read(ctx context.Context, workingDir, name string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid plan filename")
	}
	root, err := os.OpenRoot(filepath.Join(workingDir, ".crush", "plans"))
	if err != nil {
		return "", fmt.Errorf("open plans directory: %w", err)
	}
	defer root.Close()
	info, err := root.Lstat(name)
	if err != nil {
		return "", fmt.Errorf("read plan info: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("plan is not a regular file")
	}
	content, err := root.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("read plan: %w", err)
	}
	return string(content), nil
}
