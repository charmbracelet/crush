package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
)

// delegationTimeout bounds a single delegation. The external CLIs
// regularly need several minutes for coding tasks, so this is far more
// generous than the bash tool timeout.
const delegationTimeout = 15 * time.Minute

// DelegationParams is the shared input of every delegation tool: a
// self-contained task and optionally the session to continue.
type DelegationParams struct {
	Prompt string `json:"prompt" description:"The complete, self-contained task to delegate, including goal, file paths, and constraints"`
	Resume string `json:"resume,omitempty" description:"Session ID returned by a previous call to this tool to continue that conversation instead of starting a new one"`
}

// delegationOutput is the parsed outcome of one headless CLI run.
type delegationOutput struct {
	text      string // final report from the CLI
	sessionID string // identifier for resuming the conversation
}

// delegationError marks a failure the CLI itself reported in its output
// stream. Its message surfaces verbatim and takes precedence over the
// process exit code.
type delegationError struct{ msg string }

func (e *delegationError) Error() string { return e.msg }

// strategy is the per-CLI part of a delegation tool: everything that
// differs between external agents. The shared runner owns prompt
// validation, binary availability, timeout, execution, and the error
// precedence rules, so every provider behaves identically.
type strategy interface {
	// name is the tool name and the session label in responses.
	name() string
	// describe is the embedded tool description for the model.
	describe() string
	// binary is the CLI executable; a package var so tests can inject
	// fakes.
	binary() string
	// args assembles the headless invocation for one run.
	args(prompt, resume string) []string
	// parse extracts the outcome from captured stdout. A failure the CLI
	// reported in its own output comes back as a *delegationError; output
	// that cannot be interpreted comes back as any other error.
	parse(stdout []byte) (delegationOutput, error)
}

// newDelegationTool wires a strategy into the shared delegation flow:
// validate the prompt, check the binary, run it headlessly in workingDir,
// then let the structured output — not the exit code — decide the outcome.
func newDelegationTool(workingDir string, s strategy) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		s.name(),
		s.describe(),
		func(ctx context.Context, params DelegationParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if strings.TrimSpace(params.Prompt) == "" {
				return fantasy.ToolResponse{}, errors.New("prompt is required")
			}

			ctx, cancel := context.WithTimeout(ctx, delegationTimeout)
			defer cancel()

			stdout, runErr := runCommand(ctx, workingDir, s.binary(), s.args(params.Prompt, params.Resume)...)
			out, parseErr := s.parse(stdout)
			switch {
			case parseErr == nil:
				return fantasy.NewTextResponse(responseText(s.name(), out)), nil
			case isCLIReported(parseErr):
				return fantasy.ToolResponse{}, parseErr
			case runErr != nil:
				return fantasy.ToolResponse{}, runErr
			default:
				return fantasy.ToolResponse{}, parseErr
			}
		},
	)
}

// isCLIReported reports whether err is a failure the CLI stated in its own
// output rather than a broken invocation or unparseable output.
func isCLIReported(err error) bool {
	var delErr *delegationError
	return errors.As(err, &delErr)
}

// responseText renders the report with the resume hint appended.
func responseText(name string, out delegationOutput) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(out.text))
	if out.sessionID != "" {
		fmt.Fprintf(&b, "\n\n%s session: %s (pass as resume to continue this conversation)", name, out.sessionID)
	}
	return b.String()
}
