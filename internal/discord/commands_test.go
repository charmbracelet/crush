package discord

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// newCommandBridge builds a launched-free bridge for command tests.
func newCommandBridge(t *testing.T) (*Bridge, *fakeEnv, *fakeAPI) {
	t.Helper()
	env := newFakeEnv()
	env.cfg.Options = &config.Options{DataDirectory: t.TempDir()}
	api := newFakeAPI()
	b := newBridge(env, nil, "sess-1", Config{Token: "t", ChannelIDs: []string{"chan-1"}}, "code", bridgeDeps{
		api:          api,
		now:          time.Now,
		tick:         10 * time.Millisecond,
		editThrottle: time.Millisecond,
	})
	b.threadID = "thread-1"
	b.ctx = context.Background()
	b.cancel = func() {}
	return b, env, api
}

func TestParseCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		name string
		args string
		ok   bool
	}{
		{"", "", "", false},
		{"hello world", "", "", false},
		{"/help", "help", "", true},
		{"/model gpt-4o", "model", "gpt-4o", true},
		{"  /MODEL   gpt 4o  ", "model", "gpt 4o", true},
		{"/", "", "", false},
		{"//path/to/file", "/path/to/file", "", true},
	}
	for _, tt := range tests {
		name, args, ok := parseCommand(tt.in)
		require.Equal(t, tt.ok, ok, "input %q", tt.in)
		require.Equal(t, tt.name, name, "input %q", tt.in)
		require.Equal(t, tt.args, args, "input %q", tt.in)
	}
}

func TestCommandHelp(t *testing.T) {
	t.Parallel()
	b, _, _ := newCommandBridge(t)
	help := b.handleCommand(b.ctx, "help", "")
	require.Contains(t, help, "/model")
	require.Contains(t, help, "/yolo")
	require.Contains(t, help, "/clear")
	require.Contains(t, help, "/goal")
	require.Contains(t, help, "/loop")
	require.Contains(t, help, "sent to the agent as a prompt")
}

func TestCommandAutomationForwardsToTUI(t *testing.T) {
	t.Parallel()
	b, _, _ := newCommandBridge(t)
	var got AutomationCommandMsg
	b.sendEvent = func(msg tea.Msg) {
		got, _ = msg.(AutomationCommandMsg)
		got.Reply <- "Goal set: get the tests green"
	}

	reply := b.handleCommand(b.ctx, "goal", "get the tests green")
	require.Equal(t, "Goal set: get the tests green", reply)
	require.Equal(t, "sess-1", got.SessionID)
	require.Equal(t, "goal", got.Name)
	require.Equal(t, "get the tests green", got.Args)
}

func TestCommandAutomationTimesOut(t *testing.T) {
	t.Parallel()
	b, _, _ := newCommandBridge(t)
	b.commandTimeout = 10 * time.Millisecond
	b.sendEvent = func(tea.Msg) {}

	reply := b.handleCommand(b.ctx, "loop", "status")
	require.Contains(t, reply, "did not answer")
}

func TestCommandAutomationUnavailable(t *testing.T) {
	t.Parallel()
	b, _, _ := newCommandBridge(t)
	b.sendEvent = nil

	reply := b.handleCommand(b.ctx, "goal", "status")
	require.Contains(t, reply, "unavailable")
}

func TestCommandStatus(t *testing.T) {
	t.Parallel()
	b, env, _ := newCommandBridge(t)
	env.setBusy(true, "queued prompt")

	status := b.handleCommand(b.ctx, "status", "")
	require.Contains(t, status, "Test Session")
	require.Contains(t, status, "openai/gpt-4o")
	require.Contains(t, status, "mode `code`")
	require.Contains(t, status, "queue 1")
	require.Contains(t, status, "1.2k")
}

func TestCommandModelToggles(t *testing.T) {
	t.Parallel()
	b, env, _ := newCommandBridge(t)

	reply := b.handleCommand(b.ctx, "model", "gpt-5")
	require.Contains(t, reply, "Model changed")
	require.Equal(t, []config.SelectedModel{{Provider: "openai", Model: "gpt-5"}}, env.preferred)
	require.Equal(t, 1, env.modelUpdate)

	// Exact provider/model form.
	reply = b.handleCommand(b.ctx, "model", "openai/gpt-4o")
	require.Contains(t, reply, "Model changed")
	require.Len(t, env.preferred, 2)

	// Ambiguous query lists candidates.
	reply = b.handleCommand(b.ctx, "model", "gpt")
	require.Contains(t, reply, "Multiple models match")
	require.Contains(t, reply, "openai/gpt-4o")
	require.Contains(t, reply, "openai/gpt-5")
	require.Len(t, env.preferred, 2, "ambiguous query must not switch")

	// Unconfigured providers are never offered.
	reply = b.handleCommand(b.ctx, "model", "claude")
	require.Contains(t, reply, "no configured model matches")

	// Unknown query.
	reply = b.handleCommand(b.ctx, "model", "zzz")
	require.Contains(t, reply, "no configured model matches")

	// A model declared in the config resolves even though the catalog
	// does not carry it.
	reply = b.handleCommand(b.ctx, "model", "gpt-5-canary")
	require.Contains(t, reply, "Model changed")
	require.Equal(t, "gpt-5-canary", env.preferred[len(env.preferred)-1].Model)
}

func TestCommandEffort(t *testing.T) {
	t.Parallel()
	b, env, _ := newCommandBridge(t)

	provider, ok := env.cfg.Providers.Get("openai")
	require.True(t, ok)
	for i := range provider.Models {
		if provider.Models[i].ID == "gpt-5" {
			provider.Models[i].Reasoning.EffortLevels = catwalk.NewEffortLevels("low", "medium", "high")
			provider.Models[i].Reasoning.DefaultEffortLevel = "medium"
		}
	}
	env.cfg.Providers.Set("openai", provider)

	// gpt-4o carries no reasoning levels.
	require.Contains(t, b.handleCommand(b.ctx, "effort", ""), "does not support reasoning effort")

	require.Contains(t, b.handleCommand(b.ctx, "model", "gpt-5"), "Model changed")

	// Bare /effort reports the current level and the accepted values.
	reply := b.handleCommand(b.ctx, "effort", "")
	require.Contains(t, reply, "medium")
	require.Contains(t, reply, "low, medium, high")

	reply = b.handleCommand(b.ctx, "effort", "high")
	require.Contains(t, reply, "Effort set to `high`")
	require.Equal(t, "high", env.preferred[len(env.preferred)-1].ReasoningEffort)
	require.Equal(t, 2, env.modelUpdate)

	reply = b.handleCommand(b.ctx, "effort", "ultra")
	require.Contains(t, reply, "accepted values: low, medium, high")

	reply = b.handleCommand(b.ctx, "effort", "auto")
	require.Contains(t, reply, "model default")
	require.Empty(t, env.preferred[len(env.preferred)-1].ReasoningEffort)
}

func TestCommandBTW(t *testing.T) {
	t.Parallel()
	b, env, api := newCommandBridge(t)
	env.askAnswer = "It is called `pool_size`."

	require.Empty(t, b.handleCommand(b.ctx, "btw", "what was the pooling option?"))
	require.Eventually(t, func() bool {
		return api.hasContentContaining("Side question")
	}, time.Second, 10*time.Millisecond)

	var posted string
	for _, p := range api.postsSnapshot() {
		if strings.Contains(p.content, "Side question") {
			posted = p.content
		}
	}
	require.Contains(t, posted, "what was the pooling option?")
	require.Contains(t, posted, "It is called `pool_size`.")
	require.Contains(t, posted, "Not added to the conversation")
	require.Empty(t, env.runs, "a side question must not run as a normal prompt")
	require.Equal(t, []string{"what was the pooling option?"}, env.asksSnapshot())

	// Bare /btw repeats the last answer.
	reply := b.handleCommand(b.ctx, "btw", "")
	require.Contains(t, reply, "Side question")
	require.Contains(t, reply, "pool_size")
}

func TestCommandBTWUsage(t *testing.T) {
	t.Parallel()
	b, _, _ := newCommandBridge(t)
	require.Contains(t, b.handleCommand(b.ctx, "btw", ""), "Usage")
}

func TestApplicationCommandsMatchHelp(t *testing.T) {
	t.Parallel()
	_, env, _ := newCommandBridge(t)
	cmds := applicationCommands(env.cfg)
	require.GreaterOrEqual(t, len(cmds), len(builtinCommands))
	for i, doc := range builtinCommands {
		cmd := cmds[i]
		require.Equal(t, doc.name, cmd.Name)
		require.LessOrEqual(t, len([]rune(cmd.Description)), 100)
		require.Equal(t, 1, cmd.Type)
		if doc.args == "" {
			require.Empty(t, cmd.Options)
			continue
		}
		require.Len(t, cmd.Options, 1)
		require.Equal(t, "args", cmd.Options[0].Name)
		require.Equal(t, 3, cmd.Options[0].Type)
	}
}

func TestDiscordCommandName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"user:deploy", "deploy", true},
		{"project:fix-bug", "fix-bug", true},
		{"deploy", "deploy", true},
		{"user:sub:thing", "sub:thing", false},
		{"user:has space", "has space", false},
		{strings.Repeat("x", 33), strings.Repeat("x", 33), false},
	}
	for _, tt := range tests {
		got, ok := discordCommandName(tt.in)
		require.Equal(t, tt.ok, ok, "input %q", tt.in)
		if tt.ok {
			require.Equal(t, tt.want, got, "input %q", tt.in)
		}
	}
}

func TestCommandModelsList(t *testing.T) {
	t.Parallel()
	b, _, _ := newCommandBridge(t)
	reply := b.handleCommand(b.ctx, "models", "")
	require.Contains(t, reply, "openai/gpt-4o")
	require.Contains(t, reply, "GPT-5")
	require.Contains(t, reply, "neuralwatt/nw-flash", "providers outside the catalog are listed")
	require.Contains(t, reply, "acme/acme-1", "providers outside the catalog are listed")
	require.NotContains(t, reply, "claude", "unconfigured providers are hidden")

	reply = b.handleCommand(b.ctx, "models", "gpt-5")
	require.Contains(t, reply, "openai/gpt-5")
	require.NotContains(t, reply, "gpt-4o")

	// A provider query lists every model of that provider, including the
	// ones whose own name does not contain the query.
	reply = b.handleCommand(b.ctx, "models", "neural")
	require.Contains(t, reply, "neuralwatt/nw-flash")
	require.Contains(t, reply, "neuralwatt/deepseek-v4.1-flash-canary")
	require.NotContains(t, reply, "openai/")

	// Models declared in the config but absent from the Catwalk catalog
	// are listed.
	reply = b.handleCommand(b.ctx, "models", "canary")
	require.Contains(t, reply, "openai/gpt-5-canary")
	require.Contains(t, reply, "neuralwatt/deepseek-v4.1-flash-canary")

	reply = b.handleCommand(b.ctx, "models", "acme")
	require.Contains(t, reply, "acme/acme-1")
}

func TestCommandModeSwitch(t *testing.T) {
	t.Parallel()
	b, env, _ := newCommandBridge(t)

	require.Contains(t, b.handleCommand(b.ctx, "plan", ""), "Plan mode enabled")
	require.Equal(t, config.AgentPlan, env.agent)
	require.Equal(t, "plan", b.modeName())

	require.Contains(t, b.handleCommand(b.ctx, "code", ""), "Code mode enabled")
	require.Equal(t, config.AgentCoder, env.agent)
	require.Equal(t, "code", b.modeName())
}

func TestCommandYolo(t *testing.T) {
	t.Parallel()
	b, env, _ := newCommandBridge(t)

	require.Contains(t, b.handleCommand(b.ctx, "yolo", "on"), "enabled")
	require.True(t, env.yolo)

	require.Contains(t, b.handleCommand(b.ctx, "yolo", "off"), "disabled")
	require.False(t, env.yolo)

	// Bare /yolo toggles.
	require.Contains(t, b.handleCommand(b.ctx, "yolo", ""), "enabled")
	require.True(t, env.yolo)

	require.Contains(t, b.handleCommand(b.ctx, "yolo", "maybe"), "Usage")
}

func TestCommandStop(t *testing.T) {
	t.Parallel()
	b, env, _ := newCommandBridge(t)
	reply := b.handleCommand(b.ctx, "stop", "")
	require.Contains(t, reply, "Cancelling")
	require.Equal(t, 1, env.cancelCount())
}

func TestCommandRcOff(t *testing.T) {
	t.Parallel()
	b, _, api := newCommandBridge(t)
	require.Empty(t, b.handleCommand(b.ctx, "rc", "off"))
	require.True(t, api.hasPostContaining("Remote control ended"))
	require.Contains(t, b.handleCommand(b.ctx, "rc", ""), "Usage")
}

func TestCommandQueueTokensTitle(t *testing.T) {
	t.Parallel()
	b, env, _ := newCommandBridge(t)
	env.setBusy(true, "first prompt", "second prompt")

	reply := b.handleCommand(b.ctx, "queue", "")
	require.Contains(t, reply, "2")
	require.Contains(t, reply, "first prompt")

	require.Contains(t, b.handleCommand(b.ctx, "tokens", ""), "1.2k")
	require.Contains(t, b.handleCommand(b.ctx, "title", ""), "Test Session")
}

func TestCommandPermissionFlow(t *testing.T) {
	t.Parallel()
	b, env, _ := newCommandBridge(t)

	require.Contains(t, b.handleCommand(b.ctx, "deny", "bash"), "no permission requests are pending")

	b.permissions["p1"] = permission.PermissionRequest{ID: "p1", SessionID: "sess-1", ToolCallID: "tc1", ToolName: "bash"}
	b.permissionOrder = []string{"p1"}
	require.Contains(t, b.handleCommand(b.ctx, "allow", ""), "Approved")
	require.Equal(t, 1, env.grantCount())

	// The resolution notification clears the pending entry.
	b.handleEvent(pubsub.Event[tea.Msg]{Payload: pubsub.Event[permission.PermissionNotification]{
		Type: pubsub.CreatedEvent, Payload: permission.PermissionNotification{ToolCallID: "tc1", Granted: true},
	}})
	require.Contains(t, b.handleCommand(b.ctx, "deny", "bash"), "no permission requests are pending")

	// Re-seed and deny by tool name.
	b.permissions["p2"] = permission.PermissionRequest{ID: "p2", SessionID: "sess-1", ToolCallID: "tc2", ToolName: "write"}
	b.permissionOrder = append(b.permissionOrder, "p2")
	require.Contains(t, b.handleCommand(b.ctx, "deny", "write"), "Denied")
	require.Len(t, env.denies, 1)

	// Newest is the default target.
	b.permissions["p3"] = permission.PermissionRequest{ID: "p3", SessionID: "sess-1", ToolCallID: "tc3", ToolName: "bash"}
	b.permissionOrder = append(b.permissionOrder, "p3")
	require.Contains(t, b.handleCommand(b.ctx, "allow-session", ""), "session")
	require.Len(t, env.grantSessions, 1)
}

func TestCommandAnswer(t *testing.T) {
	t.Parallel()
	b, env, _ := newCommandBridge(t)

	require.Contains(t, b.handleCommand(b.ctx, "answer", "yes"), "No question is pending")

	b.questions["q1"] = question.Request{
		ID:        "q1",
		SessionID: "sess-1",
		Questions: []question.Question{{ID: "qq1", Type: question.TypeYesNo, Text: "Continue?"}},
	}
	b.questionOrder = []string{"q1"}
	require.Contains(t, b.handleCommand(b.ctx, "answer", "yes"), "Answer sent")
	require.NotNil(t, env.answers[0].Yes)
	require.True(t, *env.answers[0].Yes)

	// Multi-question forms stay in the TUI.
	b.questions["q2"] = question.Request{
		ID:        "q2",
		SessionID: "sess-1",
		Questions: []question.Question{
			{ID: "a", Type: question.TypeFreeText, Text: "one"},
			{ID: "b", Type: question.TypeFreeText, Text: "two"},
		},
	}
	b.questionOrder = append(b.questionOrder, "q2")
	require.Contains(t, b.handleCommand(b.ctx, "answer", "x"), "multiple parts")
}

func TestAnswerForQuestion(t *testing.T) {
	t.Parallel()
	choice := question.Question{
		ID:      "q",
		Type:    question.TypeSingleChoice,
		Choices: []question.Choice{{ID: "c1", Label: "One"}, {ID: "c2", Label: "Two"}},
	}
	answer, err := answerForQuestion(choice, "#2")
	require.NoError(t, err)
	require.Equal(t, []string{"c2"}, answer.SelectedIDs)

	_, err = answerForQuestion(choice, "#9")
	require.ErrorContains(t, err, "out of range")

	answer, err = answerForQuestion(choice, "some text")
	require.NoError(t, err)
	require.Equal(t, "some text", answer.FillInText)

	multi := question.Question{
		ID:      "m",
		Type:    question.TypeMultiChoice,
		Choices: []question.Choice{{ID: "a"}, {ID: "b"}, {ID: "c"}},
	}
	answer, err = answerForQuestion(multi, "1, 3")
	require.NoError(t, err)
	require.Equal(t, []string{"a", "c"}, answer.SelectedIDs)

	_, err = answerForQuestion(question.Question{ID: "y", Type: question.TypeYesNo}, "banana")
	require.ErrorContains(t, err, "yes")
}

func TestExpandCustomCommand(t *testing.T) {
	t.Parallel()
	cmd := commands.CustomCommand{
		Name:    "deploy",
		Content: "Deploy $TARGET with $FLAGS please",
		Arguments: []commands.Argument{
			{ID: "TARGET"}, {ID: "FLAGS"},
		},
	}

	out, err := expandCustomCommand(cmd, "prod --dry-run")
	require.NoError(t, err)
	require.Equal(t, "Deploy prod with --dry-run please", out)

	// The last declared argument absorbs the remainder.
	out, err = expandCustomCommand(cmd, "prod --dry run")
	require.NoError(t, err)
	require.Contains(t, out, "with --dry run")

	// name=value overrides positionally.
	out, err = expandCustomCommand(cmd, "FLAGS=fast prod")
	require.NoError(t, err)
	require.Equal(t, "Deploy prod with fast please", out)

	_, err = expandCustomCommand(cmd, "prod")
	require.ErrorContains(t, err, "$FLAGS")

	noArgs := commands.CustomCommand{Name: "ping", Content: "pong"}
	out, err = expandCustomCommand(noArgs, "extra")
	require.NoError(t, err)
	require.Contains(t, out, "pong")
	require.Contains(t, out, "extra")
}

func TestCustomCommandMatches(t *testing.T) {
	t.Parallel()
	cmd := commands.CustomCommand{ID: "user:deploy", Name: "user:deploy"}
	require.True(t, customCommandMatches(cmd, "deploy"))
	require.True(t, customCommandMatches(cmd, "user:deploy"))
	require.False(t, customCommandMatches(cmd, "other"))
}

func TestCommandUnknown(t *testing.T) {
	t.Parallel()
	b, env, _ := newCommandBridge(t)
	reply := b.handleCommand(b.ctx, "frobnicate", "")
	require.Contains(t, reply, "Unknown command")
	require.Contains(t, reply, "escape")
	require.Zero(t, env.runCount(), "unknown commands must not reach the agent")
}

func TestResolveModelQuery(t *testing.T) {
	t.Parallel()
	providers := []catwalk.Provider{
		{
			ID: catwalk.InferenceProvider("openai"),
			Models: []catwalk.Model{
				{ID: "gpt-4o", Name: "GPT-4o"},
				{ID: "gpt-5", Name: "GPT-5"},
			},
		},
		{
			ID: catwalk.InferenceProvider("anthropic"),
			Models: []catwalk.Model{
				{ID: "claude-4", Name: "Claude 4"},
			},
		},
	}
	sel, candidates, err := resolveModelQuery(providers, "gpt-5")
	require.NoError(t, err)
	require.Empty(t, candidates)
	require.Equal(t, config.SelectedModel{Provider: "openai", Model: "gpt-5"}, sel)

	sel, _, err = resolveModelQuery(providers, "GPT-5")
	require.NoError(t, err)
	require.Equal(t, "gpt-5", sel.Model)

	_, candidates, err = resolveModelQuery(providers, "gpt")
	require.NoError(t, err)
	require.Len(t, candidates, 2)

	// The caller passes configured providers only, so anything in the
	// list resolves.
	sel, _, err = resolveModelQuery(providers, "claude")
	require.NoError(t, err)
	require.Equal(t, config.SelectedModel{Provider: "anthropic", Model: "claude-4"}, sel)

	_, _, err = resolveModelQuery(providers, "anchor-name")
	require.Error(t, err, "unknown substrings must not resolve")
}

// TestCommandClearStartsFreshSession pins the /clear contract: a new
// session is created with an empty title, the bridge moves to it without
// touching the thread, and no per-session state from the cleared session
// leaks into the new one.
func TestCommandClearStartsFreshSession(t *testing.T) {
	t.Parallel()
	b, env, api := newCommandBridge(t)
	env.newSession = session.Session{ID: "sess-2"}

	// Stale state from the cleared session must not survive the rebind.
	b.pendingMu.Lock()
	b.permissions["perm-1"] = permission.PermissionRequest{ID: "perm-1", SessionID: "sess-1", ToolName: "bash"}
	b.questionOrder = append(b.questionOrder, "q-1")
	b.pendingMu.Unlock()
	b.subMu.Lock()
	b.submissions = append(b.submissions, promptSubmission{hash: "h-1", when: time.Now()})
	b.subMu.Unlock()
	b.footer.mu.Lock()
	b.footer.state = turnThinking
	b.footer.mu.Unlock()

	reply := b.handleCommand(b.ctx, "clear", "")
	require.Contains(t, reply, "Context cleared")
	require.Equal(t, []string{""}, env.createTitles)
	require.Equal(t, "sess-2", b.currentSessionID())

	b.pendingMu.Lock()
	require.Empty(t, b.permissions)
	require.Empty(t, b.questionOrder)
	b.pendingMu.Unlock()
	b.subMu.Lock()
	require.Empty(t, b.submissions)
	b.subMu.Unlock()
	b.footer.mu.Lock()
	require.Equal(t, turnIdle, b.footer.state)
	b.footer.mu.Unlock()

	// The thread is reused: no new thread and no farewell.
	require.Empty(t, api.threadNames)
	require.False(t, api.hasContentContaining("Remote control ended"))

	// Events follow the rebind: the cleared session stops mirroring and
	// the new one starts.
	before := len(api.postsSnapshot())
	b.handleEvent(evMessage(pubsub.CreatedEvent, userMessage("stale prompt")))
	require.Equal(t, before, len(api.postsSnapshot()), "the cleared session must not mirror anymore")

	fresh := message.Message{
		ID: "u-fresh", SessionID: "sess-2", Role: message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "fresh prompt"}},
	}
	b.handleEvent(evMessage(pubsub.CreatedEvent, fresh))
	require.True(t, api.hasPostContaining("fresh prompt"), "the new session must mirror")
}

func TestCommandClearRefusedWhileBusy(t *testing.T) {
	t.Parallel()
	b, env, _ := newCommandBridge(t)
	env.setBusy(true)

	reply := b.handleCommand(b.ctx, "clear", "")
	require.Contains(t, reply, "busy")
	require.Zero(t, env.createCalls, "a busy session must not be cleared")
	require.Equal(t, "sess-1", b.currentSessionID())
}

func TestCommandClearNewAlias(t *testing.T) {
	t.Parallel()
	b, env, _ := newCommandBridge(t)
	env.newSession = session.Session{ID: "sess-2"}

	require.Contains(t, b.handleCommand(b.ctx, "new", ""), "Context cleared")
	require.Equal(t, "sess-2", b.currentSessionID())
}

func TestCommandClearCreateFailure(t *testing.T) {
	t.Parallel()
	b, env, _ := newCommandBridge(t)
	env.createErr = errors.New("boom")

	reply := b.handleCommand(b.ctx, "clear", "")
	require.Contains(t, reply, "boom")
	require.Equal(t, "sess-1", b.currentSessionID(), "a failed create must keep the current session")
}
