// Package playground provides code execution capabilities for the playground
package playground

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Executor handles code execution in sandboxed environments
type Executor struct {
	timeout time.Duration
	tempDir string
}

// NewExecutor creates a new executor with default settings
func NewExecutor() *Executor {
	tempDir := os.TempDir()
	return &Executor{
		timeout: 30 * time.Second, // Default timeout of 30 seconds
		tempDir: tempDir,
	}
}

// ExecutionResult represents the result of code execution
type ExecutionResult struct {
	Stdout   string
	Stderr   string
	Duration time.Duration
	Success  bool
	Error    error
}

// Execute runs code in the specified language
func (e *Executor) Execute(ctx context.Context, language, code string) (*ExecutionResult, error) {
	start := time.Now()
	result := &ExecutionResult{}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	switch strings.ToLower(language) {
	case "go", "golang":
		result, err := e.executeGo(ctx, code)
		result.Duration = time.Since(start)
		return result, err
	case "javascript", "js":
		result, err := e.executeJavaScript(ctx, code)
		result.Duration = time.Since(start)
		return result, err
	case "python", "py":
		result, err := e.executePython(ctx, code)
		result.Duration = time.Since(start)
		return result, err
	default:
		result.Error = fmt.Errorf("unsupported language: %s", language)
		result.Success = false
		result.Duration = time.Since(start)
		return result, result.Error
	}
}

// executeGo runs Go code
func (e *Executor) executeGo(ctx context.Context, code string) (*ExecutionResult, error) {
	result := &ExecutionResult{}
	
	// Create a temporary directory for the Go module
	tempDir, err := os.MkdirTemp(e.tempDir, "blush-go-*")
	if err != nil {
		result.Error = fmt.Errorf("failed to create temp directory: %w", err)
		result.Success = false
		return result, result.Error
	}
	defer os.RemoveAll(tempDir) // Clean up
	
	// Write the Go code to a file
	filePath := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(filePath, []byte(code), 0644); err != nil {
		result.Error = fmt.Errorf("failed to write Go code: %w", err)
		result.Success = false
		return result, result.Error
	}
	
	// Run 'go run' on the file
	cmd := exec.CommandContext(ctx, "go", "run", filePath)
	
	// Capture output
	stdout, err := cmd.Output()
	if err != nil {
		// Check if it's an ExitError to get stderr
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Stderr = string(exitErr.Stderr)
			result.Success = false
			return result, nil
		}
		result.Error = fmt.Errorf("failed to execute Go code: %w", err)
		result.Success = false
		return result, result.Error
	}
	
	result.Stdout = string(stdout)
	result.Success = true
	return result, nil
}

// executeJavaScript runs JavaScript code
func (e *Executor) executeJavaScript(ctx context.Context, code string) (*ExecutionResult, error) {
	result := &ExecutionResult{}
	
	// Create a temporary file for the JavaScript code
	tempFile, err := os.CreateTemp(e.tempDir, "blush-js-*.js")
	if err != nil {
		result.Error = fmt.Errorf("failed to create temp file: %w", err)
		result.Success = false
		return result, result.Error
	}
	defer os.Remove(tempFile.Name()) // Clean up
	defer tempFile.Close()
	
	// Write the JavaScript code to the file
	if _, err := tempFile.Write([]byte(code)); err != nil {
		result.Error = fmt.Errorf("failed to write JavaScript code: %w", err)
		result.Success = false
		return result, result.Error
	}
	
	// Run 'node' on the file
	cmd := exec.CommandContext(ctx, "node", tempFile.Name())
	
	// Capture output
	stdout, err := cmd.Output()
	if err != nil {
		// Check if it's an ExitError to get stderr
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Stderr = string(exitErr.Stderr)
			result.Success = false
			return result, nil
		}
		result.Error = fmt.Errorf("failed to execute JavaScript code: %w", err)
		result.Success = false
		return result, result.Error
	}
	
	result.Stdout = string(stdout)
	result.Success = true
	return result, nil
}

// executePython runs Python code
func (e *Executor) executePython(ctx context.Context, code string) (*ExecutionResult, error) {
	result := &ExecutionResult{}
	
	// Create a temporary file for the Python code
	tempFile, err := os.CreateTemp(e.tempDir, "blush-py-*.py")
	if err != nil {
		result.Error = fmt.Errorf("failed to create temp file: %w", err)
		result.Success = false
		return result, result.Error
	}
	defer os.Remove(tempFile.Name()) // Clean up
	defer tempFile.Close()
	
	// Write the Python code to the file
	if _, err := tempFile.Write([]byte(code)); err != nil {
		result.Error = fmt.Errorf("failed to write Python code: %w", err)
		result.Success = false
		return result, result.Error
	}
	
	// Run 'python' on the file
	cmd := exec.CommandContext(ctx, "python", tempFile.Name())
	
	// Capture output
	stdout, err := cmd.Output()
	if err != nil {
		// Check if it's an ExitError to get stderr
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Stderr = string(exitErr.Stderr)
			result.Success = false
			return result, nil
		}
		result.Error = fmt.Errorf("failed to execute Python code: %w", err)
		result.Success = false
		return result, result.Error
	}
	
	result.Stdout = string(stdout)
	result.Success = true
	return result, nil
}