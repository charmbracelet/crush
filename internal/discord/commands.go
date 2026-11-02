package discord

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/session"
)

// commandEnv is the slice of workspace.Workspace the bridge drives.
// workspace.Workspace satisfies it in production; tests provide a fake.
type commandEnv interface {
	AgentRun(ctx context.Context, sessionID, prompt string, attachments ...message.Attachment) error
	AgentAsk(ctx context.Context, sessionID, question string) (string, error)
	AgentCancel(sessionID string)
	AgentIsSessionBusy(sessionID string) bool
	AgentQueuedPrompts(sessionID string) int
	AgentQueuedPromptsList(sessionID string) []string
	AgentSetMain(agentID string) error
	AgentSummarize(ctx context.Context, sessionID string) error
	UpdateAgentModel(ctx context.Context) error
	UpdatePreferredModel(scope config.Scope, modelType config.SelectedModelType, model config.SelectedModel) error
	GetSession(ctx context.Context, sessionID string) (session.Session, error)
	CreateSession(ctx context.Context, title string) (session.Session, error)
	PermissionGrant(perm permission.PermissionRequest) bool
	PermissionGrantPersistent(perm permission.PermissionRequest) bool
	PermissionDeny(perm permission.PermissionRequest) bool
	PermissionSetSkipRequests(skip bool)
	PermissionSkipRequests() bool
	QuestionAnswer(responses []question.Answer) bool
	Config() *config.Config
	WorkingDir() string
}

// commandDoc documents one built-in Discord command for /help.
type commandDoc struct {
	name string
	args string
	desc string
}

// builtinCommands is the /help surface, in display order.
var builtinCommands = []commandDoc{
	{"help", "", "show this list"},
	{"status", "", "session, model, effort, mode, queue, tokens"},
	{"models", "[query]", "list available models"},
	{"model", "<query>", "switch the main model (fuzzy match)"},
	{"effort", "[level|auto]", "show or set reasoning effort"},
	{"btw", "<question>", "ask a side question without adding it to the conversation"},
	{"goal", "[objective|status|clear]", "pursue a standing objective across turns"},
	{"loop", "[interval] <task>|status|clear|cancel <id>", "run a task on a schedule"},
	{"plan", "", "plan mode"},
	{"code", "", "code mode"},
	{"yolo", "[on|off]", "toggle permission prompts"},
	{"stop", "", "cancel the running turn"},
	{"compact", "", "compact the session (alias: /summarize)"},
	{"clear", "", "start a fresh session (alias: /new)"},
	{"queue", "", "list queued prompts"},
	{"tokens", "", "token usage for this session"},
	{"title", "", "show the session title"},
	{"allow", "[tool]", "approve the pending permission"},
	{"allow-session", "[tool]", "approve for the rest of the session"},
	{"deny", "[tool]", "deny the pending permission"},
	{"answer", "<text|#n>", "answer a pending question"},
	{"rc", "off", "stop remote control"},
}

// parseCommand splits thread text into a command name and its argument
// text. ok is false for anything that should be treated as a prompt.
func parseCommand(content string) (name, args string, ok bool) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "/") {
		return "", "", false
	}
	trimmed = strings.TrimPrefix(trimmed, "/")
	// Strip an optional "crush" or bot-mention style prefix is not
	// needed: any "/x y" is a command; authors escape with "\/".
	name, args, _ = strings.Cut(trimmed, " ")
	name = strings.ToLower(strings.TrimSpace(name))
	args = strings.TrimSpace(args)
	if name == "" {
		return "", "", false
	}
	return name, args, true
}

// handleCommand executes one slash command and returns the reply to post
// in the thread ("" posts nothing; some commands reply asynchronously or
// not at all).
func (b *Bridge) handleCommand(ctx context.Context, name, args string) string {
	switch name {
	case "help", "commands":
		return b.helpText()
	case "status":
		return b.commandStatus(ctx)
	case "models":
		return b.commandModels(args)
	case "model":
		return b.commandModel(ctx, args)
	case "effort":
		return b.commandEffort(ctx, args)
	case "btw":
		return b.commandBTW(ctx, args)
	case "goal":
		return b.commandAutomation("goal", args)
	case "loop":
		return b.commandAutomation("loop", args)
	case "plan":
		return b.commandMode(ctx, true)
	case "code":
		return b.commandMode(ctx, false)
	case "yolo":
		return b.commandYolo(args)
	case "stop":
		b.env.AgentCancel(b.currentSessionID())
		return iconCancelled + " Cancelling the running turn…"
	case "compact", "summarize":
		go func() {
			// Background context: a bridge stop must not cancel the
			// compaction the local session is still running.
			if err := b.env.AgentSummarize(context.Background(), b.currentSessionID()); err != nil && b.ctx.Err() == nil {
				b.post(iconError + " Compaction failed: " + err.Error())
			}
		}()
		return ""
	case "clear", "new":
		return b.commandClear(ctx)
	case "queue":
		return b.commandQueue()
	case "tokens":
		return b.commandTokens(ctx)
	case "title":
		return b.commandTitle(ctx)
	case "allow":
		return b.commandPermission(args, permissionGrant)
	case "allow-session":
		return b.commandPermission(args, permissionGrantSession)
	case "deny":
		return b.commandPermission(args, permissionDeny)
	case "answer":
		return b.commandAnswer(args)
	case "rc":
		if args == "off" || args == "stop" {
			b.Stop("/rc off from Discord")
			return ""
		}
		return "Usage: `/rc off` stops remote control."
	}
	// Fall through to custom commands so "/my-command args" works like
	// the TUI palette.
	if reply, handled := b.commandCustom(ctx, name, args); handled {
		return reply
	}
	return fmt.Sprintf("Unknown command `/%s` — try `/help`. To send a literal slash, escape it with `\\`.", name)
}

// applicationCommands builds the Discord application command definitions
// from the same list that backs /help (built-ins, then custom commands
// found in the config directories), so typing "/" in Discord lists every
// command the bridge understands.
func applicationCommands(cfg *config.Config) []applicationCommand {
	cmds := make([]applicationCommand, 0, len(builtinCommands))
	seen := make(map[string]bool, len(builtinCommands))
	add := func(cmd applicationCommand) {
		key := strings.ToLower(cmd.Name)
		if seen[key] {
			return
		}
		seen[key] = true
		cmds = append(cmds, cmd)
	}

	for _, doc := range builtinCommands {
		cmd := applicationCommand{
			Name:        doc.name,
			Description: truncateRunes(doc.desc, 100),
			Type:        1, // CHAT_INPUT.
		}
		if doc.args != "" {
			cmd.Options = []applicationCommandOption{{
				Name:        "args",
				Description: truncateRunes("Arguments for /"+doc.name, 100),
				Type:        3, // STRING.
			}}
		}
		add(cmd)
	}

	if cfg == nil {
		return cmds
	}
	custom, err := commands.LoadCustomCommands(cfg)
	if err != nil {
		return cmds
	}
	for _, cmd := range custom {
		name, ok := discordCommandName(cmd.Name)
		if !ok {
			continue
		}
		description := "Custom command"
		if cmd.Skill != nil && cmd.Skill.Description != "" {
			description = cmd.Skill.Description
		} else if line := firstMeaningfulLine(cmd.Content); line != "" {
			description = line
		}
		add(applicationCommand{
			Name:        name,
			Description: truncateRunes(description, 100),
			Type:        1, // CHAT_INPUT.
			Options: []applicationCommandOption{{
				Name:        "args",
				Description: "Arguments for the command",
				Type:        3, // STRING.
			}},
		})
	}
	return cmds
}

// discordCommandName reduces a typed command name ("user:deploy") to the
// bare name Discord accepts ("deploy"). Names with nested segments or
// characters Discord rejects are skipped rather than mangled, since the
// typed form would no longer match.
func discordCommandName(name string) (string, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if _, suffix, ok := strings.Cut(name, ":"); ok {
		name = suffix
	}
	if name == "" || len([]rune(name)) > 32 {
		return "", false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return "", false
		}
	}
	return name, true
}

// firstMeaningfulLine returns the first non-empty content line, skipping
// markdown frontmatter delimiters.
func firstMeaningfulLine(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "---" {
			continue
		}
		return line
	}
	return ""
}

// helpText builds the /help card, including custom commands.
func (b *Bridge) helpText() string {
	var sb strings.Builder
	sb.WriteString(iconBot + " **Crush remote control commands**\n")
	for _, c := range builtinCommands {
		usage := "/" + c.name
		if c.args != "" {
			usage += " " + c.args
		}
		fmt.Fprintf(&sb, "`%-22s` %s\n", usage, c.desc)
	}
	cfg := b.env.Config()
	if cfg != nil {
		if custom, err := commands.LoadCustomCommands(cfg); err == nil && len(custom) > 0 {
			sb.WriteString("\n**Custom commands**\n")
			for _, c := range custom {
				fmt.Fprintf(&sb, "`/%s`\n", c.Name)
			}
		}
	}
	sb.WriteString("\nAnything else you type is sent to the agent as a prompt.")
	return sb.String()
}

// commandStatus renders the /status card.
func (b *Bridge) commandStatus(ctx context.Context) string {
	var sb strings.Builder

	title := "(untitled)"
	if sess, err := b.env.GetSession(ctx, b.currentSessionID()); err == nil {
		if sess.Title != "" {
			title = sess.Title
		}
	} else {
		return iconError + " Failed to read the session: " + err.Error()
	}

	sb.WriteString(iconBot + " **" + title + "**\n")
	model := "unknown"
	var selected config.SelectedModel
	if cfg := b.env.Config(); cfg != nil {
		if sel, ok := cfg.Models[config.SelectedModelTypeLarge]; ok {
			selected = sel
			model = sel.Provider + "/" + sel.Model
		}
	}
	effort := currentEffort(b.env.Config(), selected)
	if effort == "" {
		effort = "none"
	}
	fmt.Fprintf(&sb, "-# model `%s` · effort `%s` · mode `%s` · yolo `%s`\n", model, effort, b.modeName(), onOff(b.env.PermissionSkipRequests()))
	busy := "no"
	if b.env.AgentIsSessionBusy(b.currentSessionID()) {
		busy = fmt.Sprintf("yes (queue %d)", b.env.AgentQueuedPrompts(b.currentSessionID()))
	}
	fmt.Fprintf(&sb, "-# busy `%s` · dir `%s`", busy, truncateRunes(b.env.WorkingDir(), 80))
	fmt.Fprintf(&sb, "\n-# %s", b.usageLine(ctx))
	return sb.String()
}

// commandModels lists configured models, optionally filtered by query.
func (b *Bridge) commandModels(query string) string {
	cfg := b.env.Config()
	if cfg == nil {
		return iconError + " No configuration loaded."
	}
	q := strings.ToLower(strings.TrimSpace(query))

	var lines []string
	seen := 0
	for _, p := range configuredProviders(cfg) {
		for _, m := range p.Models {
			if q != "" && !modelMatchesQuery(p, m, q) {
				continue
			}
			lines = append(lines, fmt.Sprintf("`%s/%s` — %s", p.ID, m.ID, m.Name))
			seen++
			if seen >= 40 {
				lines = append(lines, "-# (truncated; narrow with /models <query>)")
				return strings.Join(lines, "\n")
			}
		}
	}
	if len(lines) == 0 {
		return fmt.Sprintf("No models match `%s`.", query)
	}
	return "**Available models**\n" + strings.Join(lines, "\n") + "\n\nSwitch with `/model <query>`."
}

// commandModel switches the main model using a fuzzy query.
func (b *Bridge) commandModel(ctx context.Context, query string) string {
	if query == "" {
		return "Usage: `/model <query>` (try `/models` to list)."
	}
	cfg := b.env.Config()
	if cfg == nil {
		return iconError + " No configuration loaded."
	}
	sel, candidates, err := resolveModelQuery(configuredProviders(cfg), query)
	if err != nil {
		return iconError + " " + err.Error()
	}
	if len(candidates) > 0 {
		shown := candidates
		note := ""
		if len(shown) > 10 {
			shown = shown[:10]
			note = fmt.Sprintf("\n-# and %d more…", len(candidates)-10)
		}
		return "Multiple models match `" + query + "`:\n`" + strings.Join(shown, "`\n`") + "`" + note
	}
	if err := b.changeModel(ctx, sel); err != nil {
		return iconError + " Model switch failed: " + err.Error()
	}
	return fmt.Sprintf("%s Model changed to `%s/%s`.", iconDone, sel.Provider, sel.Model)
}

// currentModelType returns the model slot the coder agent uses.
func currentModelType(cfg *config.Config) config.SelectedModelType {
	if cfg != nil {
		if agentCfg, ok := cfg.Agents[config.AgentCoder]; ok && agentCfg.Model != "" {
			return agentCfg.Model
		}
	}
	return config.SelectedModelTypeLarge
}

// currentEffort returns the effort that applies to the selected model.
func currentEffort(cfg *config.Config, sel config.SelectedModel) string {
	if sel.ReasoningEffort != "" {
		return sel.ReasoningEffort
	}
	if cfg == nil {
		return ""
	}
	if model := cfg.GetModel(sel.Provider, sel.Model); model != nil {
		return model.Reasoning.DefaultEffortLevel
	}
	return ""
}

// commandEffort shows or sets the reasoning effort for the current model.
func (b *Bridge) commandEffort(ctx context.Context, args string) string {
	cfg := b.env.Config()
	if cfg == nil {
		return iconError + " No configuration loaded."
	}
	modelType := currentModelType(cfg)
	sel, ok := cfg.Models[modelType]
	if !ok {
		return iconError + " No model is selected."
	}
	model := cfg.GetModel(sel.Provider, sel.Model)
	if model == nil {
		return iconError + " The selected model is not in the catalog."
	}
	if len(model.ReasoningEffortLevels()) == 0 {
		return fmt.Sprintf("`%s/%s` does not support reasoning effort.", sel.Provider, sel.Model)
	}

	levels := strings.Join(model.ReasoningEffortLevels(), ", ")
	arg := strings.ToLower(strings.TrimSpace(args))
	switch arg {
	case "", "status":
		current := currentEffort(cfg, sel)
		if current == "" {
			current = "(none)"
		}
		return fmt.Sprintf(
			"Effort for `%s/%s`: `%s`\nLevels: %s\nUse `/effort <level>` to change it, or `/effort auto` for the model default.",
			sel.Provider, sel.Model, current, levels,
		)
	case "auto", "default":
		sel.ReasoningEffort = ""
		if err := b.env.UpdatePreferredModel(config.ScopeGlobal, modelType, sel); err != nil {
			return iconError + " Effort update failed: " + err.Error()
		}
		if err := b.env.UpdateAgentModel(ctx); err != nil {
			return iconError + " Effort update failed: " + err.Error()
		}
		def := model.Reasoning.DefaultEffortLevel
		if def == "" {
			def = "(none)"
		}
		return fmt.Sprintf("%s Effort reset to the model default: `%s`.", iconDone, def)
	}

	if err := cfg.ValidateReasoningEffort(sel.Provider, sel.Model, arg); err != nil {
		return iconError + " " + err.Error()
	}
	sel.ReasoningEffort = arg
	if err := b.env.UpdatePreferredModel(config.ScopeGlobal, modelType, sel); err != nil {
		return iconError + " Effort update failed: " + err.Error()
	}
	if err := b.env.UpdateAgentModel(ctx); err != nil {
		return iconError + " Effort update failed: " + err.Error()
	}
	return fmt.Sprintf("%s Effort set to `%s` for `%s/%s`.", iconDone, arg, sel.Provider, sel.Model)
}

// commandBTW answers a side question about the session without adding it
// to the conversation. The answer runs in the background so the thread
// stays responsive, and bare /btw repeats the last answer.
func (b *Bridge) commandBTW(ctx context.Context, question string) string {
	question = strings.TrimSpace(question)
	if question == "" {
		b.btwMu.Lock()
		last := b.lastBTW
		b.btwMu.Unlock()
		if last == "" {
			return "Usage: `/btw <question>` asks a side question without adding it to the conversation."
		}
		return last
	}

	go func() {
		answer, err := b.env.AgentAsk(ctx, b.currentSessionID(), question)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			b.post(iconError + " Side question failed: " + err.Error())
			return
		}
		if strings.TrimSpace(answer) == "" {
			answer = "(the model returned no answer)"
		}
		card := "💬 **Side question**\n> " + question + "\n\n" + answer +
			"\n\n-# Not added to the conversation."
		b.btwMu.Lock()
		b.lastBTW = card
		b.btwMu.Unlock()
		b.post(card)
	}()
	return ""
}

// commandAutomation forwards /goal and /loop to the TUI, which owns the
// per-session automation state and the scheduler that fires loop turns.
// The bridge cannot run them itself: the TUI's automation state is the
// source of truth, and a second scheduler would fire the same loop twice.
func (b *Bridge) commandAutomation(name, args string) string {
	if b.sendEvent == nil {
		return iconError + " Automation is unavailable in this bridge."
	}
	reply := make(chan string, 1)
	b.sendEvent(AutomationCommandMsg{
		SessionID: b.currentSessionID(),
		Name:      name,
		Args:      args,
		Reply:     reply,
	})
	timer := time.NewTimer(b.commandTimeout)
	defer timer.Stop()
	select {
	case text := <-reply:
		if strings.TrimSpace(text) == "" {
			return iconBot + " /" + name + " handled."
		}
		return text
	case <-timer.C:
		return iconError + " The TUI did not answer /" + name + "."
	case <-b.ctx.Done():
		return ""
	}
}

// commandMode switches between plan and code mode.
func (b *Bridge) commandMode(ctx context.Context, plan bool) string {
	if err := b.setMode(ctx, plan); err != nil {
		return iconError + " Mode switch failed: " + err.Error()
	}
	if plan {
		return "🧭 Plan mode enabled."
	}
	return "⌨️ Code mode enabled."
}

// commandYolo toggles or sets YOLO mode.
func (b *Bridge) commandYolo(args string) string {
	var on bool
	switch strings.ToLower(args) {
	case "":
		on = !b.env.PermissionSkipRequests()
	case "on", "true", "yes", "1":
		on = true
	case "off", "false", "no", "0":
		on = false
	default:
		return "Usage: `/yolo [on|off]`."
	}
	b.setYolo(on)
	if on {
		return "🔓 YOLO mode enabled — permission prompts are skipped."
	}
	return "🔒 YOLO mode disabled."
}

// commandClear starts a fresh session with an empty context and moves
// the thread to it, keeping the same Discord thread. The previous
// session stays on disk and remains resumable from the TUI.
func (b *Bridge) commandClear(ctx context.Context) string {
	if b.env.AgentIsSessionBusy(b.currentSessionID()) {
		return "⚠️ The agent is busy — wait for the current turn to finish (or `/stop`), then try again."
	}
	sess, err := b.env.CreateSession(ctx, "")
	if err != nil {
		return iconError + " Failed to start a new session: " + err.Error()
	}
	b.rebindSession(ctx, sess)
	return iconDone + " Context cleared — started a new session. The previous one stays on disk."
}

// commandQueue lists queued prompts.
func (b *Bridge) commandQueue() string {
	queue := b.env.AgentQueuedPromptsList(b.currentSessionID())
	if len(queue) == 0 {
		return "The queue is empty."
	}
	var lines []string
	for i, prompt := range queue {
		if i >= 10 {
			lines = append(lines, fmt.Sprintf("-# and %d more…", len(queue)-10))
			break
		}
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, truncateRunes(compactLine(prompt, 120), 120)))
	}
	return fmt.Sprintf("**Queued prompts (%d)**\n%s", len(queue), strings.Join(lines, "\n"))
}

// commandTokens renders token usage from the session row.
func (b *Bridge) commandTokens(ctx context.Context) string {
	return b.usageLine(ctx)
}

// usageLine formats the session's accumulated usage.
func (b *Bridge) usageLine(ctx context.Context) string {
	sess, err := b.env.GetSession(ctx, b.currentSessionID())
	if err != nil {
		return "tokens: unavailable"
	}
	approx := ""
	if sess.EstimatedUsage {
		approx = "~"
	}
	line := fmt.Sprintf("tokens: %s↑%s %s↓%s", approx, formatTokens(sess.PromptTokens), approx, formatTokens(sess.CompletionTokens))
	if sess.Cost > 0 {
		line += fmt.Sprintf(" · cost $%.4f", sess.Cost)
	}
	return line
}

// commandTitle shows the session title.
func (b *Bridge) commandTitle(ctx context.Context) string {
	sess, err := b.env.GetSession(ctx, b.currentSessionID())
	if err != nil {
		return iconError + " Failed to read the session: " + err.Error()
	}
	if sess.Title == "" {
		return "This session has no title yet."
	}
	return "**" + sess.Title + "**"
}

// permissionAction is how a permission command resolves a request.
type permissionAction int

const (
	permissionGrant permissionAction = iota
	permissionGrantSession
	permissionDeny
)

// commandPermission approves or denies a pending permission request.
func (b *Bridge) commandPermission(target string, action permissionAction) string {
	perm, err := b.pickPendingPermission(target)
	if err != nil {
		return iconError + " " + err.Error()
	}
	var ok bool
	var verb string
	switch action {
	case permissionGrant:
		ok, verb = b.env.PermissionGrant(perm), "Approved"
	case permissionGrantSession:
		ok, verb = b.env.PermissionGrantPersistent(perm), "Approved for the session"
	case permissionDeny:
		ok, verb = b.env.PermissionDeny(perm), "Denied"
	}
	if !ok {
		return "That permission request was already resolved."
	}
	return fmt.Sprintf("%s %s `%s`.", iconDone, verb, perm.ToolName)
}

// pickPendingPermission returns the newest pending permission request,
// optionally filtered by tool name or id prefix.
func (b *Bridge) pickPendingPermission(target string) (permission.PermissionRequest, error) {
	pending := b.pendingPermissions()
	if len(pending) == 0 {
		return permission.PermissionRequest{}, errors.New("no permission requests are pending")
	}
	if target == "" {
		return pending[len(pending)-1], nil
	}
	target = strings.ToLower(target)
	var matches []permission.PermissionRequest
	for _, p := range pending {
		switch {
		case strings.ToLower(p.ToolName) == target,
			strings.HasPrefix(strings.ToLower(p.ID), target),
			strings.HasPrefix(strings.ToLower(p.ToolCallID), target):
			matches = append(matches, p)
		}
	}
	switch len(matches) {
	case 0:
		return permission.PermissionRequest{}, fmt.Errorf("no pending permission matches `%s`", target)
	case 1:
		return matches[0], nil
	default:
		return permission.PermissionRequest{}, fmt.Errorf("%d pending permissions match `%s`; use the request id", len(matches), target)
	}
}

// commandAnswer answers the newest pending question when it carries a
// single question. Multi-question forms stay in the TUI.
func (b *Bridge) commandAnswer(args string) string {
	req, ok := b.pendingQuestion()
	if !ok {
		return iconError + " No question is pending."
	}
	if len(req.Questions) != 1 {
		return iconError + " This question has multiple parts; answer it in the terminal."
	}
	if strings.TrimSpace(args) == "" {
		return "Usage: `/answer <text>` or `/answer #1` for a choice."
	}
	answer, err := answerForQuestion(req.Questions[0], args)
	if err != nil {
		return iconError + " " + err.Error()
	}
	if !b.env.QuestionAnswer([]question.Answer{answer}) {
		return "That question was already resolved."
	}
	return iconDone + " Answer sent."
}

// answerForQuestion converts thread text into a question answer: yes/no
// for confirmations, choice indices ("#2", "1,3") for choices, and free
// text otherwise.
func answerForQuestion(q question.Question, text string) (question.Answer, error) {
	text = strings.TrimSpace(text)
	switch q.Type {
	case question.TypeYesNo:
		yes, err := parseYesNo(text)
		if err != nil {
			return question.Answer{}, err
		}
		return question.Answer{QuestionID: q.ID, Yes: &yes}, nil
	case question.TypeSingleChoice:
		if idx, ok := choiceIndex(text); ok {
			if idx >= len(q.Choices) {
				return question.Answer{}, fmt.Errorf("choice #%d is out of range (1-%d)", idx+1, len(q.Choices))
			}
			return question.Answer{QuestionID: q.ID, SelectedIDs: []string{q.Choices[idx].ID}}, nil
		}
		return question.Answer{QuestionID: q.ID, FillInText: text}, nil
	case question.TypeMultiChoice:
		if ids, ok := choiceIndices(text, q.Choices); ok {
			return question.Answer{QuestionID: q.ID, SelectedIDs: ids}, nil
		}
		return question.Answer{QuestionID: q.ID, FillInText: text}, nil
	default:
		return question.Answer{QuestionID: q.ID, FillInText: text}, nil
	}
}

// parseYesNo accepts the usual confirmations.
func parseYesNo(text string) (bool, error) {
	switch strings.ToLower(text) {
	case "y", "yes", "true", "1":
		return true, nil
	case "n", "no", "false", "0":
		return false, nil
	}
	return false, fmt.Errorf("answer `yes` or `no`")
}

// choiceIndex parses "1", "#1", or "1." into a zero-based index.
func choiceIndex(text string) (int, bool) {
	text = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(text), "#"), ".")
	n, err := strconv.Atoi(text)
	if err != nil || n < 1 {
		return 0, false
	}
	return n - 1, true
}

// choiceIndices parses "1,3 #5" into zero-based indices for a
// multi-choice question.
func choiceIndices(text string, choices []question.Choice) ([]string, bool) {
	parts := strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ' ' || r == '#' })
	if len(parts) == 0 {
		return nil, false
	}
	var ids []string
	for _, part := range parts {
		idx, ok := choiceIndex(part)
		if !ok || idx >= len(choices) {
			return nil, false
		}
		ids = append(ids, choices[idx].ID)
	}
	return ids, len(ids) > 0
}

// commandCustom finds and runs a custom command (or user-invocable
// skill) matching the typed name.
func (b *Bridge) commandCustom(ctx context.Context, name, args string) (string, bool) {
	cfg := b.env.Config()
	if cfg == nil {
		return "", false
	}
	custom, err := commands.LoadCustomCommands(cfg)
	if err != nil {
		return "", false
	}
	for _, cmd := range custom {
		if !customCommandMatches(cmd, name) {
			continue
		}
		if cmd.Skill != nil {
			b.launchRun(cmd.Skill.FormatInvocation())
			return "", true
		}
		prompt, err := expandCustomCommand(cmd, args)
		if err != nil {
			return iconError + " " + err.Error(), true
		}
		b.launchRun(prompt)
		return "", true
	}
	return "", false
}

// customCommandMatches reports whether a typed name selects a custom
// command, accepting the bare name and the "user:"/"project:" prefixed
// display name.
func customCommandMatches(cmd commands.CustomCommand, typed string) bool {
	typed = strings.ToLower(typed)
	name := strings.ToLower(cmd.Name)
	if name == typed {
		return true
	}
	if _, suffix, ok := strings.Cut(name, ":"); ok && suffix == typed {
		return true
	}
	return strings.ToLower(cmd.ID) == typed
}

// expandCustomCommand substitutes typed arguments into a custom
// command's markdown body. Arguments map positionally onto the $NAME
// placeholders (the last one absorbs the remainder); "name=value" pairs
// override positionally.
func expandCustomCommand(cmd commands.CustomCommand, argsText string) (string, error) {
	if len(cmd.Arguments) == 0 {
		if strings.TrimSpace(argsText) != "" {
			return cmd.Content + "\n\n" + argsText, nil
		}
		return cmd.Content, nil
	}

	values := map[string]string{}
	var positional []string
	for _, field := range strings.Fields(argsText) {
		if key, value, ok := strings.Cut(field, "="); ok && customCommandHasArg(cmd, key) {
			values[key] = value
			continue
		}
		positional = append(positional, field)
	}

	for i, arg := range cmd.Arguments {
		if _, set := values[arg.ID]; set {
			continue
		}
		if i == len(cmd.Arguments)-1 {
			values[arg.ID] = strings.Join(positional, " ")
			positional = nil
			continue
		}
		if len(positional) > 0 {
			values[arg.ID] = positional[0]
			positional = positional[1:]
		}
	}

	var missing []string
	for _, arg := range cmd.Arguments {
		if strings.TrimSpace(values[arg.ID]) == "" {
			missing = append(missing, "$"+arg.ID)
		}
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("missing arguments: %s (usage: /%s %s)", strings.Join(missing, ", "), cmd.Name, strings.Join(missing, " "))
	}

	content := cmd.Content
	for name, value := range values {
		content = strings.ReplaceAll(content, "$"+name, value)
	}
	return content, nil
}

// customCommandHasArg reports whether an argument name is declared.
func customCommandHasArg(cmd commands.CustomCommand, name string) bool {
	for _, arg := range cmd.Arguments {
		if strings.EqualFold(arg.ID, name) {
			return true
		}
	}
	return false
}

// configuredProviders returns the providers declared in the config,
// sorted by id, with the model lists merged at load time: the models
// the user declared plus the catalog's, including providers the
// catalog does not know. Providers that were disabled, or that failed
// to configure, are absent, so only usable models are offered.
func configuredProviders(cfg *config.Config) []catwalk.Provider {
	var providers []catwalk.Provider
	for id, pc := range cfg.Providers.Seq2() {
		if pc.Disable {
			continue
		}
		p := pc.ToProvider()
		if p.ID == "" {
			p.ID = catwalk.InferenceProvider(id)
		}
		if p.Name == "" {
			p.Name = id
		}
		providers = append(providers, p)
	}
	slices.SortFunc(providers, func(a, b catwalk.Provider) int {
		return cmp.Compare(a.ID, b.ID)
	})
	return providers
}

// modelMatchesQuery reports whether the query matches the provider or
// model, by id or display name, so `/models neural` lists every model
// of the Neuralwatt provider even when a model's own name lacks the
// term.
func modelMatchesQuery(p catwalk.Provider, m catwalk.Model, q string) bool {
	for _, s := range []string{string(p.ID), p.Name, m.ID, m.Name} {
		if strings.Contains(strings.ToLower(s), q) {
			return true
		}
	}
	return false
}

// resolveModelQuery fuzzy-matches a query against the models of the
// given providers. Exact "provider/model" and exact model ids win;
// otherwise case-insensitive substrings are collected, and multiple
// matches are returned as candidates for the caller to disambiguate.
func resolveModelQuery(providers []catwalk.Provider, query string) (config.SelectedModel, []string, error) {
	q := strings.ToLower(strings.TrimSpace(query))

	var candidates []string
	for _, p := range providers {
		providerID := string(p.ID)
		for _, m := range p.Models {
			full := providerID + "/" + m.ID
			switch {
			case strings.EqualFold(full, q), strings.EqualFold(m.ID, q):
				return config.SelectedModel{Model: m.ID, Provider: providerID}, nil, nil
			case strings.Contains(strings.ToLower(full), q),
				strings.Contains(strings.ToLower(m.ID), q),
				strings.Contains(strings.ToLower(m.Name), q):
				candidates = append(candidates, full)
			}
		}
	}
	switch len(candidates) {
	case 0:
		return config.SelectedModel{}, nil, fmt.Errorf("no configured model matches `%s`", query)
	case 1:
		full := candidates[0]
		provider, model, _ := strings.Cut(full, "/")
		return config.SelectedModel{Model: model, Provider: provider}, nil, nil
	default:
		return config.SelectedModel{}, candidates, nil
	}
}

// onOff renders a boolean for display.
func onOff(v bool) string {
	if v {
		return "on"
	}
	return "off"
}
