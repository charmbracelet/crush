package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/lsp/protocol"
)

type DiagnosticsParams struct {
	FilePath string `json:"file_path"`
}
type diagnosticsTool struct {
	lspClients map[string]*lsp.Client
}

const (
	DiagnosticsToolName    = "diagnostics"
	diagnosticsDescription = `Get diagnostics for a file and/or project.
WHEN TO USE THIS TOOL:
- Use when you need to check for errors or warnings in your code
- Helpful for debugging and ensuring code quality
- Good for getting a quick overview of issues in a file or project
HOW TO USE:
- Provide a path to a file to get diagnostics for that file
- Leave the path empty to get diagnostics for the entire project
- Results are displayed in a structured format with severity levels
FEATURES:
- Displays errors, warnings, and hints
- Groups diagnostics by severity
- Provides detailed information about each diagnostic
LIMITATIONS:
- Results are limited to the diagnostics provided by the LSP clients
- May not cover all possible issues in the code
- Does not provide suggestions for fixing issues
TIPS:
- Use in conjunction with other tools for a comprehensive code review
- Combine with the LSP client for real-time diagnostics
`
)

func NewDiagnosticsTool(lspClients map[string]*lsp.Client) BaseTool {
	return &diagnosticsTool{
		lspClients,
	}
}

func (b *diagnosticsTool) Name() string {
	return DiagnosticsToolName
}

func (b *diagnosticsTool) Info() ToolInfo {
	return ToolInfo{
		Name:        DiagnosticsToolName,
		Description: diagnosticsDescription,
		Parameters: map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The path to the file to get diagnostics for (leave w empty for project diagnostics)",
			},
		},
		Required: []string{},
	}
}

func (b *diagnosticsTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params DiagnosticsParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	lsps := b.lspClients

	if len(lsps) == 0 {
		return NewTextErrorResponse("no LSP clients available"), nil
	}

	if params.FilePath != "" {
		notifyLspOpenFile(ctx, params.FilePath, lsps)
		waitForLspDiagnostics(ctx, params.FilePath, lsps)
	}

	output := getDiagnostics(params.FilePath, lsps)

	return NewTextResponse(output), nil
}

func notifyLspOpenFile(ctx context.Context, filePath string, lsps map[string]*lsp.Client) {
	for _, client := range lsps {
		err := client.OpenFile(ctx, filePath)
		if err != nil {
			continue
		}
	}
}

func waitForLspDiagnostics(ctx context.Context, filePath string, lsps map[string]*lsp.Client) {
	if len(lsps) == 0 {
		return
	}

	diagChan := make(chan struct{}, 1)

	for _, client := range lsps {
		originalDiags := client.GetDiagnostics()

		handler := func(params json.RawMessage) {
			lsp.HandleDiagnostics(client, params)
			var diagParams protocol.PublishDiagnosticsParams
			if err := json.Unmarshal(params, &diagParams); err != nil {
				return
			}

			path, err := diagParams.URI.Path()
			if err != nil {
				slog.Error("Failed to convert diagnostic URI to path", "uri", diagParams.URI, "error", err)
				return
			}

			if path == filePath || hasDiagnosticsChanged(client.GetDiagnostics(), originalDiags) {
				select {
				case diagChan <- struct{}{}:
				default:
				}
			}
		}

		client.RegisterNotificationHandler("textDocument/publishDiagnostics", handler)

		if client.IsFileOpen(filePath) {
			err := client.NotifyChange(ctx, filePath)
			if err != nil {
				continue
			}
		} else {
			err := client.OpenFile(ctx, filePath)
			if err != nil {
				continue
			}
		}
	}

	select {
	case <-diagChan:
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
	}
}

func hasDiagnosticsChanged(current, original map[protocol.DocumentURI][]protocol.Diagnostic) bool {
	for uri, diags := range current {
		origDiags, exists := original[uri]
		if !exists || len(diags) != len(origDiags) {
			return true
		}
	}
	return false
}

func getDiagnostics(filePath string, lsps map[string]*lsp.Client) string {
	fileDiagnostics := []string{}
	projectDiagnostics := []string{}

	formatDiagnostic := func(pth string, diagnostic protocol.Diagnostic, source string) string {
		severity := "Info"
		switch diagnostic.Severity {
		case protocol.SeverityError:
			severity = "Error"
		case protocol.SeverityWarning:
			severity = "Warn"
		case protocol.SeverityHint:
			severity = "Hint"
		}

		location := fmt.Sprintf("%s:%d:%d", pth, diagnostic.Range.Start.Line+1, diagnostic.Range.Start.Character+1)

		sourceInfo := ""
		if diagnostic.Source != "" {
			sourceInfo = diagnostic.Source
		} else if source != "" {
			sourceInfo = source
		}

		codeInfo := ""
		if diagnostic.Code != nil {
			codeInfo = fmt.Sprintf("[%v]", diagnostic.Code)
		}

		tagsInfo := ""
		if len(diagnostic.Tags) > 0 {
			tags := []string{}
			for _, tag := range diagnostic.Tags {
				switch tag {
				case protocol.Unnecessary:
					tags = append(tags, "unnecessary")
				case protocol.Deprecated:
					tags = append(tags, "deprecated")
				}
			}
			if len(tags) > 0 {
				tagsInfo = fmt.Sprintf(" (%s)", strings.Join(tags, ", "))
			}
		}

		return fmt.Sprintf("%s: %s [%s]%s%s %s",
			severity,
			location,
			sourceInfo,
			codeInfo,
			tagsInfo,
			diagnostic.Message)
	}

	for lspName, client := range lsps {
		diagnostics := client.GetDiagnostics()
		if len(diagnostics) > 0 {
			for location, diags := range diagnostics {
				path, err := location.Path()
				if err != nil {
					slog.Error("Failed to convert diagnostic location URI to path", "uri", location, "error", err)
					continue
				}
				isCurrentFile := path == filePath

				for _, diag := range diags {
					formattedDiag := formatDiagnostic(path, diag, lspName)

					if isCurrentFile {
						fileDiagnostics = append(fileDiagnostics, formattedDiag)
					} else {
						projectDiagnostics = append(projectDiagnostics, formattedDiag)
					}
				}
			}
		}
	}

	sort.Slice(fileDiagnostics, func(i, j int) bool {
		iIsError := strings.HasPrefix(fileDiagnostics[i], "Error")
		jIsError := strings.HasPrefix(fileDiagnostics[j], "Error")
		if iIsError != jIsError {
			return iIsError // Errors come first
		}
		return fileDiagnostics[i] < fileDiagnostics[j] // Then alphabetically
	})

	sort.Slice(projectDiagnostics, func(i, j int) bool {
		iIsError := strings.HasPrefix(projectDiagnostics[i], "Error")
		jIsError := strings.HasPrefix(projectDiagnostics[j], "Error")
		if iIsError != jIsError {
			return iIsError
		}
		return projectDiagnostics[i] < projectDiagnostics[j]
	})

	var output strings.Builder

	if len(fileDiagnostics) > 0 {
		output.WriteString("\n<file_diagnostics>\n")
		if len(fileDiagnostics) > 10 {
			output.WriteString(strings.Join(fileDiagnostics[:10], "\n"))
			fmt.Fprintf(&output, "\n... and %d more diagnostics", len(fileDiagnostics)-10)
		} else {
			output.WriteString(strings.Join(fileDiagnostics, "\n"))
		}
		output.WriteString("\n</file_diagnostics>\n")
	}

	if len(projectDiagnostics) > 0 {
		output.WriteString("\n<project_diagnostics>\n")
		if len(projectDiagnostics) > 10 {
			output.WriteString(strings.Join(projectDiagnostics[:10], "\n"))
			fmt.Fprintf(&output, "\n... and %d more diagnostics", len(projectDiagnostics)-10)
		} else {
			output.WriteString(strings.Join(projectDiagnostics, "\n"))
		}
		output.WriteString("\n</project_diagnostics>\n")
	}

	if len(fileDiagnostics) > 0 || len(projectDiagnostics) > 0 {
		fileErrors := countSeverity(fileDiagnostics, "Error")
		fileWarnings := countSeverity(fileDiagnostics, "Warn")
		projectErrors := countSeverity(projectDiagnostics, "Error")
		projectWarnings := countSeverity(projectDiagnostics, "Warn")

		output.WriteString("\n<diagnostic_summary>\n")
		fmt.Fprintf(&output, "Current file: %d errors, %d warnings\n", fileErrors, fileWarnings)
		fmt.Fprintf(&output, "Project: %d errors, %d warnings\n", projectErrors, projectWarnings)
		output.WriteString("</diagnostic_summary>\n")
	}

	return output.String()
}

func countSeverity(diagnostics []string, severity string) int {
	count := 0
	for _, diag := range diagnostics {
		if strings.HasPrefix(diag, severity) {
			count++
		}
	}
	return count
}
