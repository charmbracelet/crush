package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestPendingChecksForEdit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "pkg")
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "foo_test.go"), []byte("package pkg"), 0o644))
	plain := filepath.Join(dir, "plain")
	require.NoError(t, os.MkdirAll(plain, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(plain, "bar.go"), []byte("package plain"), 0o644))

	cfgWithVerify := &config.Config{Verify: []config.VerifyConfig{
		{Name: "build", Command: "go build ./...", Timeout: 60},
	}}
	cfgEmpty := &config.Config{}

	tests := []struct {
		name string
		cfg  *config.Config
		path string
		want []string // expected check names
	}{
		{name: "non-source file still gets declared checks", cfg: cfgWithVerify, path: filepath.Join(dir, "README.md"), want: []string{"verify:build"}},
		{name: "manifest edit gets declared checks", cfg: cfgWithVerify, path: filepath.Join(dir, "go.mod"), want: []string{"verify:build"}},
		{name: "source gets verify commands", cfg: cfgWithVerify, path: filepath.Join(plain, "bar.go"), want: []string{"verify:build"}},
		{name: "no verify config yields no pending", cfg: cfgEmpty, path: filepath.Join(plain, "bar.go"), want: nil},
		{name: "non-source without config selects nothing", cfg: cfgEmpty, path: filepath.Join(dir, "README.md"), want: nil},
		{name: "go file in tested dir gets package test", cfg: cfgEmpty, path: filepath.Join(pkgDir, "foo.go"), want: []string{"package-test:pkg"}},
		{name: "verify and package test compose", cfg: cfgWithVerify, path: filepath.Join(pkgDir, "foo.go"), want: []string{"verify:build", "package-test:pkg"}},
		{name: "file outside working dir gets no package test", cfg: cfgEmpty, path: filepath.Join(t.TempDir(), "x.go"), want: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			checks := pendingChecksForEdit(tc.cfg, dir, tc.path)
			var got []string
			for _, c := range checks {
				require.Equal(t, message.VerificationPending, c.State)
				require.NotEmpty(t, c.Command)
				got = append(got, c.Check)
			}
			require.Equal(t, tc.want, got)
		})
	}
}

// stepWith builds a fantasy step carrying the given content parts.
func stepWith(finish fantasy.FinishReason, parts ...fantasy.Content) fantasy.StepResult {
	return fantasy.StepResult{
		Response: fantasy.Response{
			Content:      fantasy.ResponseContent(parts),
			FinishReason: finish,
		},
	}
}

func TestScanVerification(t *testing.T) {
	t.Parallel()
	steps := []fantasy.StepResult{
		stepWith(fantasy.FinishReasonToolCalls,
			fantasy.ToolResultContent{
				ToolCallID: "tc-edit",
				ToolName:   "edit",
				Result:     fantasy.ToolResultOutputContentText{Text: "ok"},
				ClientMetadata: `{"verification":[` +
					`{"check":"diagnostics","state":"passed"},` +
					`{"check":"package-test:pkg","state":"pending","command":"go test ./pkg"}]}`,
			},
		),
		stepWith(fantasy.FinishReasonToolCalls,
			fantasy.ToolCallContent{ToolCallID: "tc-bash", ToolName: "bash", Input: `{"command":"go test ./pkg"}`},
			fantasy.ToolResultContent{
				ToolCallID:     "tc-bash",
				ToolName:       "bash",
				Result:         fantasy.ToolResultOutputContentText{Text: "ok pkg\nPASS"},
				ClientMetadata: `{"done":true,"exit_code":0}`,
			},
			fantasy.ToolCallContent{ToolCallID: "tc-bash2", ToolName: "bash", Input: `{"command":"go test ./bad"}`},
			fantasy.ToolResultContent{
				ToolCallID:     "tc-bash2",
				ToolName:       "bash",
				Result:         fantasy.ToolResultOutputContentText{Text: "FAIL\nExit code 1"},
				ClientMetadata: `{"done":true,"exit_code":1}`,
			},
			// A still-running background command is not a verdict.
			fantasy.ToolCallContent{ToolCallID: "tc-bash3", ToolName: "bash", Input: `{"command":"go test ./slow"}`},
			fantasy.ToolResultContent{
				ToolCallID:     "tc-bash3",
				ToolName:       "bash",
				Result:         fantasy.ToolResultOutputContentText{Text: "Background shell started with ID: bg1"},
				ClientMetadata: `{"background":true,"shell_id":"bg1"}`,
			},
			// A completed background job inspected via job_output is a
			// verdict — the command comes from the result metadata.
			fantasy.ToolCallContent{ToolCallID: "tc-job", ToolName: "job_output", Input: `{"shell_id":"bg2","wait":true}`},
			fantasy.ToolResultContent{
				ToolCallID:     "tc-job",
				ToolName:       "job_output",
				Result:         fantasy.ToolResultOutputContentText{Text: "Status: completed\nFAIL"},
				ClientMetadata: `{"command":"go test ./job","done":true,"exit_code":1,"shell_id":"bg2"}`,
			},
			// A job launched this run and polled later: the verdict's
			// step for postdating is the launch, not the poll.
			fantasy.ToolCallContent{ToolCallID: "tc-bash4", ToolName: "bash", Input: `{"command":"go test ./bg","run_in_background":true}`},
			fantasy.ToolResultContent{
				ToolCallID:     "tc-bash4",
				ToolName:       "bash",
				Result:         fantasy.ToolResultOutputContentText{Text: "Background shell started with ID: bg9"},
				ClientMetadata: `{"background":true,"shell_id":"bg9"}`,
			},
		),
		stepWith(fantasy.FinishReasonToolCalls,
			fantasy.ToolCallContent{ToolCallID: "tc-job2", ToolName: "job_output", Input: `{"shell_id":"bg9","wait":true}`},
			fantasy.ToolResultContent{
				ToolCallID:     "tc-job2",
				ToolName:       "job_output",
				Result:         fantasy.ToolResultOutputContentText{Text: "Status: completed\nok"},
				ClientMetadata: `{"command":"go test ./bg","done":true,"exit_code":0,"shell_id":"bg9"}`,
			},
		),
		stepWith(fantasy.FinishReasonStop, fantasy.TextContent{Text: "done"}),
	}

	failed, pending, observed := scanVerification(steps)
	require.Empty(t, failed)
	require.Len(t, pending, 1)
	require.Equal(t, "package-test:pkg", pending[0].check.Check)
	require.Equal(t, "tc-edit", pending[0].toolCallID)
	require.Len(t, observed, 4)

	require.Equal(t, "go test ./pkg", observed[0].command)
	require.False(t, observed[0].isError)

	// A non-zero exit is a text response — the verdict comes from
	// exit_code in the metadata, not the result type.
	require.Equal(t, "go test ./bad", observed[1].command)
	require.True(t, observed[1].isError)

	// A job launched before this run predates every write it made —
	// the poll's step must not count.
	require.Equal(t, "go test ./job", observed[2].command)
	require.True(t, observed[2].isError)
	require.Equal(t, -1, observed[2].stepIdx)

	// Launched at step 1 (with the edit), polled at step 2 — the
	// verdict postdates from the launch step, not the poll step.
	require.Equal(t, "go test ./bg", observed[3].command)
	require.False(t, observed[3].isError)
	require.Equal(t, 1, observed[3].stepIdx)
}

func TestRunGateChecks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := &sessionAgent{}

	mkPending := func(check, command string, step int) gateCheckOutcome {
		return gateCheckOutcome{
			toolCallID: "tc-" + check,
			stepIndex:  step,
			check: message.VerificationCheck{
				Check: check, State: message.VerificationPending, Command: command, Timeout: 30,
			},
		}
	}

	t.Run("runs command and records exit code", func(t *testing.T) {
		res := a.runGateChecks(t.Context(), dir, []gateCheckOutcome{
			mkPending("verify:fail", "exit 3", 0),
			mkPending("verify:pass", "echo ok", 0),
		}, nil)
		require.Equal(t, message.VerificationFailed, res["verify:fail"].state)
		require.Equal(t, "exit code 3", res["verify:fail"].detail)
		require.Equal(t, message.VerificationPassed, res["verify:pass"].state)
	})

	t.Run("dedups identical pending checks", func(t *testing.T) {
		res := a.runGateChecks(t.Context(), dir, []gateCheckOutcome{
			mkPending("verify:x", "echo hi", 0),
			mkPending("verify:x", "echo hi", 1),
		}, nil)
		require.Len(t, res, 1)
	})

	t.Run("observed bash run satisfies pending check", func(t *testing.T) {
		res := a.runGateChecks(t.Context(), dir, []gateCheckOutcome{
			mkPending("verify:test", "go test ./pkg", 0),
		}, []observedBash{
			{stepIdx: 2, command: "go test ./pkg", isError: false, output: "PASS"},
		})
		require.Equal(t, message.VerificationPassed, res["verify:test"].state)
	})

	t.Run("observed failure resolves pending as failed", func(t *testing.T) {
		res := a.runGateChecks(t.Context(), dir, []gateCheckOutcome{
			mkPending("verify:test", "go test ./pkg", 0),
		}, []observedBash{
			{stepIdx: 2, command: "go test ./pkg", isError: true, output: "FAIL"},
		})
		require.Equal(t, message.VerificationFailed, res["verify:test"].state)
	})

	t.Run("observed run before the writes does not satisfy", func(t *testing.T) {
		res := a.runGateChecks(t.Context(), dir, []gateCheckOutcome{
			mkPending("verify:test", "echo ran", 5),
		}, []observedBash{
			{stepIdx: 2, command: "echo ran", isError: false},
		})
		// Not satisfied by the earlier run — the command executes
		// harness-side instead.
		require.Equal(t, message.VerificationPassed, res["verify:test"].state)
		require.Equal(t, "ran\n", res["verify:test"].output)
	})

	t.Run("observed run between two writes does not satisfy", func(t *testing.T) {
		// The same check pending on writes at steps 0 and 4 must not be
		// satisfied by a matching bash run at step 2 — the step-4 write
		// is not covered.
		res := a.runGateChecks(t.Context(), dir, []gateCheckOutcome{
			mkPending("verify:test", "echo ran", 0),
			mkPending("verify:test", "echo ran", 4),
		}, []observedBash{
			{stepIdx: 2, command: "echo ran", isError: false},
		})
		require.Equal(t, "ran\n", res["verify:test"].output)
	})

	t.Run("prefix command does not match observed run", func(t *testing.T) {
		res := a.runGateChecks(t.Context(), dir, []gateCheckOutcome{
			mkPending("verify:test", "echo safe", 0),
		}, []observedBash{
			{stepIdx: 2, command: "echo safe && exit 1", isError: true},
		})
		require.Equal(t, message.VerificationPassed, res["verify:test"].state)
	})

	t.Run("hung check times out as failed", func(t *testing.T) {
		res := a.runGateChecks(t.Context(), dir, []gateCheckOutcome{
			{toolCallID: "tc-hang", stepIndex: 0, check: message.VerificationCheck{
				Check: "verify:slow", State: message.VerificationPending,
				Command: "sleep 60", Timeout: 1,
			}},
		}, nil)
		require.Equal(t, message.VerificationFailed, res["verify:slow"].state)
		require.Contains(t, res["verify:slow"].detail, "timed out")
	})

	t.Run("run cancel aborts remaining checks", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel() // cancelled before the first check runs
		res := a.runGateChecks(ctx, dir, []gateCheckOutcome{
			mkPending("verify:x", "exit 1", 0),
		}, nil)
		require.Empty(t, res, "a cancelled run records no verdict for unrunnable checks")
	})
}

func TestMergeVerificationResolved(t *testing.T) {
	t.Parallel()
	existing := `{"hook":{"allow":true},"verification":[{"check":"diagnostics","state":"passed"},{"check":"verify:x","state":"pending"}]}`
	merged := mergeVerificationResolved(existing, []message.VerificationCheck{
		{Check: "verify:x", State: message.VerificationFailed, Detail: "exit code 1"},
	})
	require.Contains(t, merged, `"allow":true`)
	require.Contains(t, merged, `"check":"verify:x","state":"failed"`)
	require.NotContains(t, merged, "pending")
}

func TestUnionToolMetadata(t *testing.T) {
	t.Parallel()
	stored := `{"verification":[{"check":"verify:x","state":"failed"}]}`
	incoming := `{"hook":{"deny":false}}`
	merged := unionToolMetadata(stored, incoming)
	require.Contains(t, merged, `"verification"`)
	require.Contains(t, merged, `"hook"`)

	// A resolved stored verdict does not regress to pending when the
	// incoming copy was snapshotted before the gate wrote outcomes.
	merged = unionToolMetadata(
		`{"verification":[{"check":"verify:x","state":"failed"}]}`,
		`{"verification":[{"check":"verify:x","state":"pending"}]}`,
	)
	require.Contains(t, merged, `"state":"failed"`)
	require.NotContains(t, merged, `"state":"pending"`)

	// Incoming can still resolve a stored pending.
	merged = unionToolMetadata(
		`{"verification":[{"check":"verify:x","state":"pending"}]}`,
		`{"verification":[{"check":"verify:x","state":"passed"}]}`,
	)
	require.Contains(t, merged, `"state":"passed"`)
}

// newGateTestAgent builds a sessionAgent with the pieces the gate
// touches: config store, message service, and the queue/dispatch maps.
func newGateTestAgent(t *testing.T, cfg *config.Config) (*sessionAgent, message.Service, string) {
	t.Helper()
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	sess, err := sessions.Create(t.Context(), "test")
	require.NoError(t, err)

	svc := message.NewService(q)
	return &sessionAgent{
		configStore:  config.NewTestStore(cfg),
		messages:     svc,
		messageQueue: csync.NewMap[string, []SessionAgentCall](),
		dispatchMu:   csync.NewMap[string, *sync.Mutex](),
	}, svc, sess.ID
}

func TestRunVerificationGate(t *testing.T) {
	t.Parallel()

	editWith := func(meta string) fantasy.ToolResultContent {
		return fantasy.ToolResultContent{
			ToolCallID:     "tc-edit",
			ToolName:       "edit",
			Result:         fantasy.ToolResultOutputContentText{Text: "edited"},
			ClientMetadata: meta,
		}
	}
	assistantMsg := func() *message.Message {
		return &message.Message{Role: message.Assistant}
	}

	t.Run("failed decorator verdict queues a retry", func(t *testing.T) {
		t.Parallel()
		a, _, sessionID := newGateTestAgent(t, &config.Config{})
		result := &fantasy.AgentResult{Steps: []fantasy.StepResult{
			stepWith(fantasy.FinishReasonToolCalls, editWith(`{"verification":[{"check":"diagnostics","state":"failed","detail":"2 new error(s)"}]}`)),
			stepWith(fantasy.FinishReasonStop, fantasy.TextContent{Text: "done"}),
		}}

		queued := a.runVerificationGate(t.Context(), SessionAgentCall{
			SessionID: sessionID, RunID: "run-1", Prompt: "do it",
		}, result, assistantMsg())
		require.True(t, queued)
		q, ok := a.messageQueue.Get(sessionID)
		require.True(t, ok)
		require.Len(t, q, 1)
		require.Equal(t, "run-1", q[0].RunID)
		require.Equal(t, 1, q[0].VerificationAttempts)
		require.Contains(t, q[0].Prompt, "diagnostics")
	})

	t.Run("stop-turn ending does not gate", func(t *testing.T) {
		t.Parallel()
		a, _, sessionID := newGateTestAgent(t, &config.Config{})
		result := &fantasy.AgentResult{Steps: []fantasy.StepResult{
			stepWith(fantasy.FinishReasonToolCalls, editWith(`{"verification":[{"check":"diagnostics","state":"failed"}]}`)),
			stepWith(fantasy.FinishReasonStop, fantasy.ToolResultContent{
				ToolCallID: "tc-q", ToolName: "question", StopTurn: true,
				Result: fantasy.ToolResultOutputContentText{Text: "denied"},
			}),
		}}
		queued := a.runVerificationGate(t.Context(), SessionAgentCall{SessionID: sessionID}, result, assistantMsg())
		require.False(t, queued)
	})

	t.Run("non-stop terminal step does not gate", func(t *testing.T) {
		t.Parallel()
		a, _, sessionID := newGateTestAgent(t, &config.Config{})
		result := &fantasy.AgentResult{Steps: []fantasy.StepResult{
			stepWith(fantasy.FinishReasonToolCalls, editWith(`{"verification":[{"check":"diagnostics","state":"failed"}]}`)),
			stepWith(fantasy.FinishReasonLength, fantasy.TextContent{Text: "cut off"}),
		}}
		require.False(t, a.runVerificationGate(t.Context(), SessionAgentCall{SessionID: sessionID}, result, assistantMsg()))
	})

	t.Run("observed failing bash resolves pending and retries", func(t *testing.T) {
		t.Parallel()
		a, svc, sessionID := newGateTestAgent(t, &config.Config{})
		// The stored tool-result row the outcome lands on.
		mkMsg(t, svc, sessionID, message.Tool, message.ToolResult{
			ToolCallID: "tc-edit", Name: "edit", Content: "edited",
			Metadata: `{"verification":[{"check":"verify:test","state":"pending","command":"go test ./x"}]}`,
		})
		result := &fantasy.AgentResult{Steps: []fantasy.StepResult{
			stepWith(fantasy.FinishReasonToolCalls, editWith(`{"verification":[{"check":"verify:test","state":"pending","command":"go test ./x"}]}`)),
			stepWith(fantasy.FinishReasonToolCalls,
				fantasy.ToolCallContent{ToolCallID: "tc-bash", ToolName: "bash", Input: `{"command":"go test ./x"}`},
				// A real bash failure is a TEXT result — the exit_code
				// metadata is the verdict.
				fantasy.ToolResultContent{
					ToolCallID: "tc-bash", ToolName: "bash",
					Result:         fantasy.ToolResultOutputContentText{Text: "FAIL\nExit code 1"},
					ClientMetadata: `{"done":true,"exit_code":1}`,
				},
			),
			stepWith(fantasy.FinishReasonStop, fantasy.TextContent{Text: "done"}),
		}}

		queued := a.runVerificationGate(t.Context(), SessionAgentCall{SessionID: sessionID, RunID: "r"}, result, assistantMsg())
		require.True(t, queued)

		// The stored row's pending entry resolved to failed.
		msgs, err := svc.List(t.Context(), sessionID)
		require.NoError(t, err)
		require.Contains(t, msgs[0].ToolResults()[0].Metadata, `"state":"failed"`)
	})

	t.Run("retry prepends ahead of a queued user prompt", func(t *testing.T) {
		t.Parallel()
		a, _, sessionID := newGateTestAgent(t, &config.Config{})
		// A user prompt submitted during the run sits behind the gate's
		// retry — the repair turn runs first.
		a.messageQueue.Set(sessionID, []SessionAgentCall{{
			SessionID: sessionID, Prompt: "user follow-up",
		}})
		result := &fantasy.AgentResult{Steps: []fantasy.StepResult{
			stepWith(fantasy.FinishReasonToolCalls, editWith(`{"verification":[{"check":"diagnostics","state":"failed"}]}`)),
			stepWith(fantasy.FinishReasonStop, fantasy.TextContent{Text: "done"}),
		}}
		queued := a.runVerificationGate(t.Context(), SessionAgentCall{
			SessionID: sessionID, RunID: "run-1",
		}, result, assistantMsg())
		require.True(t, queued)
		q, _ := a.messageQueue.Get(sessionID)
		require.Len(t, q, 2)
		require.Contains(t, q[0].Prompt, "Verification failed", "retry is prepended")
		require.Equal(t, "user follow-up", q[1].Prompt)
	})

	t.Run("exhausted budget surfaces on the assistant message", func(t *testing.T) {
		t.Parallel()
		a, _, sessionID := newGateTestAgent(t, &config.Config{})
		asst := assistantMsg()
		result := &fantasy.AgentResult{Steps: []fantasy.StepResult{
			stepWith(fantasy.FinishReasonToolCalls, editWith(`{"verification":[{"check":"diagnostics","state":"failed","detail":"1 new error(s)"}]}`)),
			stepWith(fantasy.FinishReasonStop, fantasy.TextContent{Text: "done"}),
		}}
		queued := a.runVerificationGate(t.Context(), SessionAgentCall{
			SessionID: sessionID, VerificationAttempts: maxVerificationAttempts,
		}, result, asst)
		require.False(t, queued)
		require.Contains(t, asst.Content().Text, "still failing")
	})
}

// gateScriptModel drives sessionAgent.Run end-to-end: the first Stream
// emits an edit tool call (whose decorator records a pending check), the
// second ends turn one on a clean stop, and the third is the gate's
// retry turn. Prompts are captured per call.
type gateScriptModel struct {
	calls    atomic.Int32
	mu       sync.Mutex
	prompts  []string
	editPath string
}

func (m *gateScriptModel) Provider() string { return "fake" }
func (m *gateScriptModel) Model() string    { return "fake-model" }

func (m *gateScriptModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not implemented")
}

func (m *gateScriptModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *gateScriptModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

// promptText flattens a fantasy prompt's text for assertions.
func promptText(p fantasy.Prompt) string {
	var b strings.Builder
	for _, msg := range p {
		for _, part := range msg.Content {
			if t, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok {
				b.WriteString(t.Text)
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

func (m *gateScriptModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	text := promptText(call.Prompt)
	if strings.Contains(text, "Generate a concise title") {
		// Title generation shares the model — answer it without
		// consuming a main-turn call.
		return gateTextStream("title"), nil
	}

	n := m.calls.Add(1)
	m.mu.Lock()
	m.prompts = append(m.prompts, text)
	m.mu.Unlock()

	if n != 1 {
		return gateTextStream("done"), nil
	}

	input := fmt.Sprintf(`{"file_path":%q,"old_string":"a","new_string":"b"}`, m.editPath)
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputStart, ID: "tc1", ToolCallName: "edit"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputDelta, ID: "tc1", Delta: input}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputEnd, ID: "tc1"}) {
			return
		}
		if !yield(fantasy.StreamPart{
			Type:          fantasy.StreamPartTypeToolCall,
			ID:            "tc1",
			ToolCallName:  "edit",
			ToolCallInput: input,
		}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
	}, nil
}

// gateTextStream emits a trivial text response ending on Stop.
func gateTextStream(text string) fantasy.StreamResponse {
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "t"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "t", Delta: text}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "t"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}
}

// TestRunGate_EndToEnd exercises the full recursion path: an edit
// records a pending check, the gate runs it (exit 1 → failed), a retry
// call is prepended under the same RunID, the recursive Run executes it,
// and exactly one RunComplete publishes for the lifecycle.
func TestRunGate_EndToEnd(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	dir := env.workingDir
	editPath := filepath.Join(dir, "x.go")

	model := &gateScriptModel{editPath: editPath}
	tool := &verifyingTool{
		inner:      &fakeTool{name: "edit", resp: fantasy.NewTextResponse("edited")},
		workingDir: dir,
		pendingChecks: func(string) []message.VerificationCheck {
			return []message.VerificationCheck{{
				Check:   "verify:test",
				State:   message.VerificationPending,
				Command: "exit 1",
				Timeout: 30,
			}}
		},
	}

	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)
	sa := testSessionAgent(env, model, model, "system", tool).(*sessionAgent)
	sa.configStore = config.NewTestStoreWithDir(&config.Config{}, dir)
	sa.runComplete = broker

	sess, err := env.sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	events := broker.Subscribe(ctx)

	_, err = sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		RunID:     "run-e2e",
		Prompt:    "edit the file",
	})
	require.NoError(t, err)

	// Three Stream calls: edit step, turn-one stop step, retry turn.
	require.Equal(t, int32(3), model.calls.Load())
	model.mu.Lock()
	for i, p := range model.prompts {
		t.Logf("prompt[%d]: %q", i, p)
	}
	retry := model.prompts[2]
	require.Contains(t, retry, "verify:test")
	require.Contains(t, retry, "exit code")
	model.mu.Unlock()

	// Exactly one RunComplete for the shared RunID — the outer turn's
	// publish is suppressed while the retry is queued.
	var completes []notify.RunComplete
drain:
	for {
		select {
		case ev := <-events:
			if ev.Payload.RunID == "run-e2e" {
				completes = append(completes, ev.Payload)
			}
		default:
			break drain
		}
	}
	require.Len(t, completes, 1)

	// The stored tool result carries the gate-resolved verdict.
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	found := false
	for _, m := range msgs {
		for _, tr := range m.ToolResults() {
			if tr.ToolCallID == "tc1" {
				found = true
				require.Contains(t, tr.Metadata, `"check":"verify:test"`)
				require.Contains(t, tr.Metadata, `"state":"failed"`)
			}
		}
	}
	require.True(t, found, "stored tool result for tc1 not found")
}
