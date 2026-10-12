package tools

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/version"
	"mvdan.cc/sh/v3/syntax"
)

// customToolAbandonGrace is how long a call waits, after its deadline, for
// the interpreter to yield before the agent stops waiting on it.
const customToolAbandonGrace = time.Second

// runCustomShell is the executor behind a custom tool; tests replace it.
var runCustomShell = shell.Run

// GetCustomTools returns an agent tool for every enabled custom tool in
// config, sorted by name.
func GetCustomTools(permissions permission.Service, cfg *config.ConfigStore, wd, spillDir string) []fantasy.AgentTool {
	declared := cfg.Config().CustomTools
	var result []fantasy.AgentTool
	for _, name := range slices.Sorted(maps.Keys(declared)) {
		t := declared[name]
		if t.Disabled {
			continue
		}
		result = append(result, NewCustomTool(name, t, permissions, wd, spillDir))
	}
	return result
}

// NewCustomTool returns the agent tool for one custom tool declaration.
func NewCustomTool(name string, cfg config.CustomToolConfig, permissions permission.Service, wd, spillDir string) *CustomTool {
	return &CustomTool{name: name, cfg: cfg, permissions: permissions, workingDir: wd, spillDir: spillDir}
}

// CustomTool is an agent tool implemented by a shell command, usually
// declared by a plugin with `tool add`.
type CustomTool struct {
	name            string
	cfg             config.CustomToolConfig
	permissions     permission.Service
	workingDir      string
	spillDir        string
	providerOptions fantasy.ProviderOptions
}

func (c *CustomTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	c.providerOptions = opts
}

func (c *CustomTool) ProviderOptions() fantasy.ProviderOptions {
	return c.providerOptions
}

func (c *CustomTool) Info() fantasy.ToolInfo {
	parameters := map[string]any{}
	if props, ok := c.cfg.Schema["properties"].(map[string]any); ok {
		parameters = maps.Clone(props)
	}
	for name, desc := range c.cfg.Params {
		parameters[name] = map[string]any{"type": "string", "description": desc}
	}
	required := slices.Clone(c.cfg.Required)
	if required == nil {
		required = []string{}
	}
	description := c.cfg.Description

	if len(c.cfg.Actions) > 0 {
		names := slices.Sorted(maps.Keys(c.cfg.Actions))
		parameters["action"] = map[string]any{
			"type":        "string",
			"enum":        names,
			"description": "The operation to run, from the list in the tool description",
		}
		required = append([]string{"action"}, required...)

		// An action's own parameters are listed once, noting which actions
		// use them, so the model can tell what goes with what.
		usedBy := map[string][]string{}
		var list strings.Builder
		list.WriteString("\n\nSet \"action\" to one of:")
		for _, name := range names {
			a := c.cfg.Actions[name]
			fmt.Fprintf(&list, "\n- %s: %s", name, a.Description)
			if args := actionArgs(a); args != "" {
				fmt.Fprintf(&list, " (%s)", args)
			}
			for p, desc := range a.Params {
				if _, ok := parameters[p]; !ok {
					parameters[p] = map[string]any{"type": "string", "description": desc}
				}
				usedBy[p] = append(usedBy[p], name)
			}
		}
		for p, actions := range usedBy {
			if _, shared := c.cfg.Params[p]; shared {
				continue
			}
			param := parameters[p].(map[string]any)
			param["description"] = fmt.Sprintf("%s (for %s)", param["description"], strings.Join(actions, ", "))
		}
		description += list.String()
	}

	return fantasy.ToolInfo{
		Name:        c.name,
		Description: description,
		Parameters:  parameters,
		Required:    required,
	}
}

// actionArgs lists an action's parameters for the tool description, marking
// the required ones.
func actionArgs(a config.CustomToolAction) string {
	var args []string
	for _, p := range slices.Sorted(maps.Keys(a.Params)) {
		if slices.Contains(a.Required, p) {
			p += " required"
		}
		args = append(args, p)
	}
	return strings.Join(args, ", ")
}

// resolve picks the command, permission action, and prompt policy for one
// call. A non-empty problem is returned to the model as an error.
func (c *CustomTool) resolve(input map[string]any) (command, action string, skip bool, problem string) {
	if len(c.cfg.Actions) == 0 {
		return c.cfg.Command, "execute", c.cfg.SkipPermission, missing(input, c.cfg.Required)
	}
	action, _ = input["action"].(string)
	a, ok := c.cfg.Actions[action]
	if !ok {
		names := slices.Sorted(maps.Keys(c.cfg.Actions))
		return "", "", false, fmt.Sprintf("unknown action %q, expected one of: %s", action, strings.Join(names, ", "))
	}
	command = cmp.Or(a.Command, c.cfg.Command)
	return command, action, c.cfg.SkipPermission || a.SkipPermission, missing(input, append(slices.Clone(c.cfg.Required), a.Required...))
}

// missing reports the first required parameter the input lacks.
func missing(input map[string]any, required []string) string {
	for _, r := range required {
		if v, ok := input[r]; !ok || v == nil || v == "" {
			return fmt.Sprintf("missing required parameter %q", r)
		}
	}
	return ""
}

// prelude assigns every declared parameter to a shell variable of the same
// name, empty when absent, so a command reads "$number" instead of parsing
// stdin. Values are quoted, so input never becomes code.
func prelude(params map[string]any, input map[string]any) string {
	var b strings.Builder
	for _, name := range slices.Sorted(maps.Keys(params)) {
		if !shellIdentifier.MatchString(name) {
			continue
		}
		var value string
		switch v := input[name].(type) {
		case nil:
		case string:
			value = v
		default:
			raw, _ := json.Marshal(v)
			value = string(raw)
		}
		quoted, err := syntax.Quote(value, syntax.LangBash)
		if err != nil {
			quoted = "''"
		}
		fmt.Fprintf(&b, "%s=%s\n", name, quoted)
	}
	return b.String()
}

var shellIdentifier = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func (c *CustomTool) Run(ctx context.Context, params fantasy.ToolCall) (fantasy.ToolResponse, error) {
	sessionID := GetSessionFromContext(ctx)
	if sessionID == "" {
		return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for running a custom tool")
	}

	input := map[string]any{}
	if strings.TrimSpace(params.Input) != "" {
		if err := json.Unmarshal([]byte(params.Input), &input); err != nil {
			return fantasy.NewTextErrorResponse(fmt.Sprintf("%s: input must be a JSON object: %s", c.name, err)), nil
		}
	}
	command, action, skip, problem := c.resolve(input)
	if problem != "" {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("%s: %s", c.name, problem)), nil
	}

	if !skip {
		ok, err := c.permissions.Request(ctx, permission.CreatePermissionRequest{
			SessionID:   sessionID,
			ToolCallID:  params.ID,
			Path:        c.workingDir,
			ToolName:    c.name,
			Action:      action,
			Description: c.permissionSummary(action, params.Input),
			Params:      c.permissionDetails(action, input, command),
		})
		if err != nil {
			return fantasy.ToolResponse{}, err
		}
		if !ok {
			return NewPermissionDeniedResponse(), nil
		}
	}

	timeout := c.cfg.TimeoutDuration()
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	run := runCustomShell
	script := prelude(c.Info().Parameters, input) + command
	env := c.env(sessionID, params.Input)
	if len(c.cfg.Actions) > 0 {
		env = append(env, "CRUSH_TOOL_ACTION="+action)
	}
	done := make(chan error, 1)
	go func() {
		done <- run(runCtx, shell.RunOptions{
			Command: script,
			Cwd:     c.workingDir,
			Env:     env,
			Stdin:   strings.NewReader(params.Input),
			Stdout:  &stdout,
			Stderr:  &stderr,
		})
	}()

	var err error
	// Same ownership rule as hooks: the buffers are only read once the
	// goroutine has delivered on done.
	select {
	case err = <-done:
	case <-runCtx.Done():
		select {
		case err = <-done:
		case <-time.After(customToolAbandonGrace):
			slog.Warn("Custom tool did not yield after cancel; abandoning goroutine", "tool", c.name)
			return c.interrupted(ctx, timeout), nil
		}
	}

	if shell.IsInterrupt(err) || (err != nil && runCtx.Err() != nil) {
		return c.interrupted(ctx, timeout), nil
	}

	out := TruncateOutput(strings.TrimRight(stdout.String(), "\n"), c.spillDir)
	errOut := TruncateOutput(strings.TrimRight(stderr.String(), "\n"), c.spillDir)
	if err != nil {
		msg := fmt.Sprintf("%s failed with exit code %d", c.name, shell.ExitCode(err))
		if errOut != "" {
			msg += "\n\nstderr:\n" + errOut
		}
		if out != "" {
			msg += "\n\nstdout:\n" + out
		}
		return fantasy.NewTextErrorResponse(msg), nil
	}
	if out == "" {
		out = "(no output)"
	}
	return fantasy.NewTextResponse(out), nil
}

func (c *CustomTool) interrupted(ctx context.Context, timeout time.Duration) fantasy.ToolResponse {
	if ctx.Err() != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("%s was cancelled", c.name))
	}
	return fantasy.NewTextErrorResponse(fmt.Sprintf("%s timed out after %s", c.name, timeout))
}

// env is the command's environment: Crush's own, the declared extras, and
// the call's context, so a command can read its input without stdin.
func (c *CustomTool) env(sessionID, input string) []string {
	env := os.Environ()
	for _, k := range slices.Sorted(maps.Keys(c.cfg.Env)) {
		env = append(env, k+"="+c.cfg.Env[k])
	}
	env = append(env,
		"CRUSH_TOOL_NAME="+c.name,
		"CRUSH_TOOL_INPUT="+input,
		"CRUSH_SESSION_ID="+sessionID,
		"CRUSH_PROJECT_DIR="+c.workingDir,
		"CRUSH_VERSION="+version.Version,
	)
	if c.cfg.Source != "" {
		env = append(env,
			"CRUSH_PLUGIN_FILE="+c.cfg.Source,
			"CRUSH_PLUGIN_DIR="+filepath.Dir(c.cfg.Source),
		)
	}
	return env
}

// CustomToolDisplay renders how a call to a custom tool is shown in the chat,
// from the Title and Summary templates of the chosen action or the tool.
// ok is false when nothing is declared or the input does not parse yet (it
// may still be streaming), so the caller falls back to the plain rendering.
func CustomToolDisplay(cfg config.CustomToolConfig, name, inputJSON string) (title, summary string, ok bool) {
	input := map[string]any{}
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return "", "", false
	}
	titleTpl, summaryTpl := cfg.Title, cfg.Summary
	if action, _ := input["action"].(string); action != "" {
		if a, found := cfg.Actions[action]; found {
			titleTpl, summaryTpl = cmp.Or(a.Title, titleTpl), cmp.Or(a.Summary, summaryTpl)
		}
	}
	if titleTpl == "" && summaryTpl == "" {
		return "", "", false
	}

	// Every declared parameter is present, so a template can test any of
	// them without tripping over a missing key.
	data := map[string]string{}
	for p := range NewCustomTool(name, cfg, nil, "", "").Info().Parameters {
		data[p] = ""
	}
	for k, v := range input {
		switch v := v.(type) {
		case nil:
		case string:
			data[k] = v
		default:
			raw, _ := json.Marshal(v)
			data[k] = string(raw)
		}
	}

	render := func(text string) string {
		tpl, err := config.CustomToolTemplate(text)
		if err != nil {
			return ""
		}
		var b strings.Builder
		if err := tpl.Execute(&b, data); err != nil {
			return ""
		}
		return strings.Join(strings.Fields(b.String()), " ")
	}
	title, summary = render(titleTpl), render(summaryTpl)
	return title, summary, title != "" || summary != ""
}

// permissionSummary is what the permission prompt leads with: what the call
// does, in the plugin's words, rather than the tool's internal name.
func (c *CustomTool) permissionSummary(action, inputJSON string) string {
	what := c.cfg.Description
	if a, ok := c.cfg.Actions[action]; ok {
		what = a.Description
	}
	headline := c.name
	if action != "execute" {
		headline += " " + action
	}
	if title, summary, ok := CustomToolDisplay(c.cfg, c.name, inputJSON); ok {
		headline = strings.TrimSpace(title + "\n" + summary)
	}
	return headline + "\n\n" + what
}

// permissionDetails lists the call's parameters, one per line with long or
// multi-line values indented under their name, followed by the command that
// will run, so the prompt shows exactly what is being approved.
func (c *CustomTool) permissionDetails(action string, input map[string]any, command string) string {
	var required []string
	if a, ok := c.cfg.Actions[action]; ok {
		required = a.Required
	}
	required = append(slices.Clone(c.cfg.Required), required...)
	names := slices.Collect(maps.Keys(input))
	slices.SortFunc(names, func(a, b string) int {
		ai, bi := slices.Index(required, a), slices.Index(required, b)
		switch {
		case ai >= 0 && bi >= 0:
			return cmp.Compare(ai, bi)
		case ai >= 0:
			return -1
		case bi >= 0:
			return 1
		}
		return strings.Compare(a, b)
	})

	var b strings.Builder
	for _, name := range names {
		if name == "action" && len(c.cfg.Actions) > 0 {
			continue
		}
		value, ok := input[name].(string)
		if !ok {
			raw, _ := json.Marshal(input[name])
			value = string(raw)
		}
		if value == "" {
			continue
		}
		if strings.Contains(value, "\n") || len(value) > 60 {
			fmt.Fprintf(&b, "%s:\n  %s\n", name, strings.ReplaceAll(strings.TrimRight(value, "\n"), "\n", "\n  "))
		} else {
			fmt.Fprintf(&b, "%s: %s\n", name, value)
		}
	}
	fmt.Fprintf(&b, "\nRuns:\n  %s", strings.ReplaceAll(strings.TrimSpace(command), "\n", "\n  "))
	return b.String()
}
