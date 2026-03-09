package tools

import (
	"bufio"
	"context"
	_ "embed"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"charm.land/fantasy"
)

//go:embed crush_instance.md
var crushInstanceDescription []byte

const CrushInstanceToolName = "crush"

type CrushInstanceParams struct {
	Prompt string `json:"prompt" description:"The task for the Crush instance to perform"`
	Model  string `json:"model" description:"Optional model to use (e.g., 'gpt-4' or 'openai/gpt-4')"`
}

type CrushInstanceRequest struct {
	Prompt string `json:"prompt"`
}

type CrushInstanceResponse struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

type CrushInstanceMetadata struct {
	ProcessID    string `json:"process_id"`
	IsRunning    bool   `json:"is_running"`
	Completed    bool   `json:"completed"`
	OutputLength int    `json:"output_length"`
}

// NewCrushInstanceTool creates a tool that spawns Crush subprocess instances
func NewCrushInstanceTool() fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		CrushInstanceToolName,
		string(crushInstanceDescription),
		func(ctx context.Context, params CrushInstanceParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Prompt == "" {
				return fantasy.NewTextErrorResponse("prompt is required"), nil
			}

			// Build crush run command
			args := []string{"run"}
			if params.Model != "" {
				args = append(args, "--model", params.Model)
			}
			args = append(args, params.Prompt)

			// Create the subprocess
			cmd := exec.CommandContext(ctx, "crush", args...)

			// Set up pipes for communication
			stdin, err := cmd.StdinPipe()
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to create stdin pipe: %s", err)), nil
			}
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to create stdout pipe: %s", err)), nil
			}
			stderr, err := cmd.StderrPipe()
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to create stderr pipe: %s", err)), nil
			}

			// Start the process
			if err := cmd.Start(); err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to start crush process: %s", err)), nil
			}

			processID := cmd.Process.Pid

			// Close stdin since we're not sending input
			stdin.Close()

			// Read output in goroutine
			outputChan := make(chan string)
			errChan := make(chan error)

			go func() {
				scanner := bufio.NewScanner(stdout)
				var output strings.Builder
				for scanner.Scan() {
					line := scanner.Text()
					output.WriteString(line)
					output.WriteString("\n")
					// For now, just collect output
					// TODO: Support streaming results back
				}
				outputChan <- output.String()
			}()

			go func() {
				errText, _ := io.ReadAll(stderr)
				if errText != nil && len(errText) > 0 {
					errChan <- fmt.Errorf("crush error: %s", string(errText))
				}
				close(errChan)
			}()

			// Wait for completion or context cancellation
			resultChan := make(chan *CrushInstanceResponse)
			go func() {
				var output string
				select {
				case output = <-outputChan:
					// Got output
				case <-ctx.Done():
					// Context cancelled
					cmd.Process.Kill()
					return
				}

				select {
				case err := <-errChan:
					if err != nil {
						resultChan <- &CrushInstanceResponse{
							Error: err.Error(),
						}
						return
					}
				default:
				}

				// Wait for process to complete
				if err := cmd.Wait(); err != nil {
					resultChan <- &CrushInstanceResponse{
						Content: output,
						Error:   fmt.Sprintf("process error: %s", err),
					}
					return
				}

				resultChan <- &CrushInstanceResponse{
					Content: output,
				}
			}()

			// Wait for result with timeout
			select {
			case result := <-resultChan:
				metadata := CrushInstanceMetadata{
					ProcessID:    fmt.Sprintf("%d", processID),
					IsRunning:    false,
					Completed:    result.Error == "",
					OutputLength: len(result.Content),
				}

				if result.Error != "" {
					return fantasy.WithResponseMetadata(
						fantasy.NewTextErrorResponse(result.Error),
						metadata,
					), nil
				}

				return fantasy.WithResponseMetadata(
					fantasy.NewTextResponse(result.Content),
					metadata,
				), nil

			case <-ctx.Done():
				cmd.Process.Kill()
				return fantasy.NewTextErrorResponse("operation cancelled"), nil
			}
		})
}
