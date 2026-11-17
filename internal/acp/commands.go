package acp

import (
	"strings"

	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/skills"
)

// commandMatch is one advertised slash command reduced to what resolving it
// needs. The same three sources availableCommandsUpdate enumerates populate it,
// so the palette a client shows and the commands a prompt can invoke can never
// disagree.
type commandMatch struct {
	name    string
	skill   *skills.Skill
	content string
	args    []commands.Argument
	// prompt is set for MCP prompts, which resolve through the MCP client
	// rather than from local text.
	prompt *commands.MCPPrompt
}

// resolveCommand replaces a leading /name with the text that command stands
// for. A prompt that names no advertised command is returned unchanged, so
// ordinary prose starting with a slash still reaches the model.
//
// This is an ACP-local copy of the resolution the TUI performs (see plan Q1):
// the TUI drives a dialog to collect arguments, which a prompt has no
// equivalent of, so arguments are bound positionally from the trailing text.
func (a *crushAgent) resolveCommand(prompt string) string {
	return a.resolveAgainst(a.advertisedCommands(), prompt)
}

// resolveAgainst is resolveCommand against a supplied command index, so the
// resolution rules are testable without an app or a config store.
func (a *crushAgent) resolveAgainst(matches map[string]commandMatch, prompt string) string {
	name, rest, ok := splitCommand(prompt)
	if !ok {
		return prompt
	}
	match, ok := lookupCommand(matches, name)
	if !ok {
		return prompt
	}

	args := bindArgs(match.args, rest)
	if len(match.args) > 0 && len(args) < countRequired(match.args) {
		// Sending the template with $ARG_* still in it would be worse than
		// passing the user's text through unresolved.
		return prompt
	}

	switch {
	case match.skill != nil:
		return joinNonEmpty(match.skill.FormatInvocation(), rest)
	case match.prompt != nil:
		return a.resolveMCPPrompt(match, args, prompt)
	case match.content != "":
		return substituteArgs(match.content, args)
	default:
		return prompt
	}
}

// resolveMCPPrompt fetches an MCP prompt's text, falling back to the original
// prompt when the MCP server is unreachable.
func (a *crushAgent) resolveMCPPrompt(match commandMatch, args map[string]string, prompt string) string {
	if a.app == nil {
		return prompt
	}
	store := a.app.Store()
	if store == nil {
		return prompt
	}
	text, err := commands.GetMCPPrompt(store, match.prompt.ClientID, match.prompt.PromptID, args)
	if err != nil {
		a.log.Warn("Failed to resolve MCP prompt", "command", match.name, "error", err)
		return prompt
	}
	return text
}

// splitCommand separates a leading "/name" from the text after it. A prompt
// that does not start with a slash, or is a bare slash, is not a command.
func splitCommand(prompt string) (name, rest string, ok bool) {
	if !strings.HasPrefix(prompt, "/") {
		return "", "", false
	}
	body := prompt[1:]
	if body == "" {
		return "", "", false
	}
	if i := strings.IndexAny(body, " \t\n"); i >= 0 {
		return body[:i], strings.TrimSpace(body[i:]), true
	}
	return body, "", true
}

// lookupCommand finds the advertised command a name refers to, preferring the
// longest match.
//
// Longest-match matters because custom command ids embed path separators and a
// ":" prefix (user:foo, project:sub/bar), so "user:foo/bar" and "user:foo" can
// both be advertised. Names are matched after any leading slash is stripped,
// and a name is also accepted with its leading slash still attached, since
// command ids are built from file paths and may themselves begin with one.
func lookupCommand(matches map[string]commandMatch, name string) (commandMatch, bool) {
	if m, ok := matches[name]; ok {
		return m, true
	}
	// Fall back to the longest advertised name that the requested name starts
	// with, so subcommand paths resolve to their subcommand.
	best := ""
	for candidate := range matches {
		if strings.HasPrefix(name, candidate) && len(candidate) > len(best) {
			best = candidate
		}
	}
	if best == "" {
		return commandMatch{}, false
	}
	return matches[best], true
}

// advertisedCommands indexes the commands availableCommandsUpdate advertises.
func (a *crushAgent) advertisedCommands() map[string]commandMatch {
	out := map[string]commandMatch{}

	add := func(m commandMatch) {
		if m.name == "" {
			return
		}
		if _, exists := out[m.name]; exists {
			return
		}
		out[m.name] = m
	}

	if a.app != nil && a.app.Skills != nil {
		entries := skills.Catalog(a.app.Skills.ActiveSkills(), a.app.Skills.ResolvedPaths(), a.workingDir())
		for _, c := range commands.FromSkillCatalog(entries) {
			add(commandMatch{name: c.Name, skill: c.Skill, content: c.Content, args: c.Arguments})
		}
	}

	if cfg := a.config(); cfg != nil {
		if custom, err := commands.LoadCustomCommands(cfg); err == nil {
			for _, c := range custom {
				add(commandMatch{name: c.Name, skill: c.Skill, content: c.Content, args: c.Arguments})
			}
		}
	}

	if prompts, err := commands.LoadMCPPrompts(); err == nil {
		for i := range prompts {
			p := prompts[i]
			add(commandMatch{name: p.ID, prompt: &p, args: p.Arguments})
		}
	}

	return out
}

// bindArgs pairs the trailing text with the command's declared arguments in
// declaration order, after $NAME substitution. Commands without declared
// arguments bind nothing, which is what their templates expect.
func bindArgs(declared []commands.Argument, rest string) map[string]string {
	if len(declared) == 0 {
		return nil
	}
	fields := strings.Fields(rest)
	args := make(map[string]string, len(declared))
	for i, arg := range declared {
		if i >= len(fields) {
			break
		}
		args[arg.ID] = fields[i]
	}
	return args
}

// countRequired reports how many of the declared arguments must be supplied.
func countRequired(declared []commands.Argument) int {
	n := 0
	for _, arg := range declared {
		if arg.Required {
			n++
		}
	}
	return n
}

// substituteArgs replaces $NAME placeholders in content with their values.
func substituteArgs(content string, args map[string]string) string {
	for name, value := range args {
		content = strings.ReplaceAll(content, "$"+name, value)
	}
	return content
}

// joinNonEmpty joins the parts that carry text with a blank line.
func joinNonEmpty(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "\n\n")
}
