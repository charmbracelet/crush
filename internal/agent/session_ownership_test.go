package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

type lifecycleModel struct {
	finishStreamModel
	stream func(context.Context, fantasy.Call) (fantasy.StreamResponse, error)
}

func (m *lifecycleModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	return m.stream(ctx, call)
}

func newLifecycleTestAgent(t *testing.T, title string) (fakeEnv, session.Session, *lifecycleModel, *sessionAgent) {
	t.Helper()
	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), title)
	require.NoError(t, err)
	model := &lifecycleModel{}
	agent, ok := testSessionAgent(env, model, &finishStreamModel{text: "title"}, "system").(*sessionAgent)
	require.True(t, ok)
	return env, sess, model, agent
}

func seedUserMessage(t *testing.T, messages message.Service, sessionID, text string) {
	t.Helper()
	_, err := messages.Create(t.Context(), sessionID, message.CreateMessageParams{
		Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: text}},
	})
	require.NoError(t, err)
}

type observingNotifications struct {
	*pubsub.Broker[notify.Notification]
	observe func(notify.Notification)
}

func (p observingNotifications) Publish(kind pubsub.EventType, notification notify.Notification) {
	p.observe(notification)
	p.Broker.Publish(kind, notification)
}

func compactionStream(yield func(fantasy.StreamPart) bool) {
	if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: "tool-1", ToolCallName: "unused", ToolCallInput: "{}"}) {
		return
	}
	yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls, Usage: fantasy.Usage{InputTokens: 190000, OutputTokens: 100}})
}

type observingSummarySave struct {
	session.Service
	afterSave func()
}

func (s observingSummarySave) Save(ctx context.Context, sess session.Session) (session.Session, error) {
	saved, err := s.Service.Save(ctx, sess)
	if err == nil && sess.SummaryMessageID != "" {
		s.afterSave()
	}
	return saved, err
}

type observingUserCreate struct {
	message.Service
	afterCreate func(message.Message)
}

func (s observingUserCreate) Create(ctx context.Context, sessionID string, params message.CreateMessageParams) (message.Message, error) {
	created, err := s.Service.Create(ctx, sessionID, params)
	if err == nil && params.Role == message.User {
		s.afterCreate(created)
	}
	return created, err
}

func TestRun_PostCancelAnonymousSubmissionBeforeStep(t *testing.T) {
	t.Parallel()
	for _, pendingAccept := range []bool{false, true} {
		t.Run(fmt.Sprintf("pending=%t", pendingAccept), func(t *testing.T) {
			t.Parallel()
			env, sess, model, sa := newLifecycleTestAgent(t, "session")
			seedUserMessage(t, env.messages, sess.ID, "earlier")
			if pendingAccept {
				pending := sa.BeginAccepted(sess.ID)
				defer pending.Close()
			}
			sa.messages = observingUserCreate{Service: env.messages, afterCreate: func(created message.Message) {
				if created.Content().String() != "A" {
					return
				}
				// A has an owner but has not reached its first PrepareStep.
				sa.Cancel(sess.ID)
				result, err := sa.Run(t.Context(), SessionAgentCall{
					SessionID: sess.ID, Prompt: "B", Accepted: sa.BeginAccepted(sess.ID),
				})
				require.NoError(t, err)
				require.Nil(t, result)
				require.Equal(t, []string{"B"}, sa.QueuedPromptsList(sess.ID))
			}}
			var streamed [][]fantasy.MessagePart
			model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				require.NotEmpty(t, call.Prompt)
				streamed = append(streamed, call.Prompt[len(call.Prompt)-1].Content)
				return (&finishStreamModel{text: "done"}).Stream(ctx, call)
			}
			_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "A"})
			require.ErrorIs(t, err, context.Canceled)
			require.Equal(t, [][]fantasy.MessagePart{fantasy.NewUserMessage("B").Content}, streamed, "B must execute once under a fresh, uncanceled owner")
			msgs, err := env.messages.List(t.Context(), sess.ID)
			require.NoError(t, err)
			var prompts []string
			for _, msg := range msgs {
				if msg.Role == message.User {
					prompts = append(prompts, msg.Content().String())
				}
			}
			require.Equal(t, []string{"earlier", "A", "B"}, prompts)
			require.False(t, sa.IsSessionBusy(sess.ID))
			require.Zero(t, sa.QueuedPrompts(sess.ID))
		})
	}
}

func TestRun_CanceledAcceptedInteractiveAdmissionNotifiesIdle(t *testing.T) {
	t.Parallel()
	for _, finish := range []string{"before-dispatch", "during-cleanup", "after-cleanup"} {
		t.Run(finish, func(t *testing.T) {
			t.Parallel()
			env, sess, model, sa := newLifecycleTestAgent(t, "session")
			seedUserMessage(t, env.messages, sess.ID, "earlier")
			broker := pubsub.NewBroker[notify.Notification]()
			defer broker.Shutdown()
			var events []notify.Notification
			var busy []bool
			sa.notify = observingNotifications{Broker: broker, observe: func(n notify.Notification) {
				busy = append(busy, sa.IsSessionBusy(sess.ID))
				events = append(events, n)
			}}
			started := make(chan struct{})
			releaseOwner := make(chan struct{})
			ownerGone := make(chan struct{})
			release := sync.OnceFunc(func() { close(releaseOwner) })
			sa.messages = observingUserCreate{Service: env.messages, afterCreate: func(created message.Message) {
				if finish == "during-cleanup" && created.Content().String() == "B" {
					release()
					<-ownerGone
				}
			}}
			calls := 0
			model.stream = func(ctx context.Context, _ fantasy.Call) (fantasy.StreamResponse, error) {
				calls++
				if calls != 1 {
					return nil, errors.New("canceled B unexpectedly streamed")
				}
				close(started)
				<-releaseOwner
				return nil, ctx.Err()
			}
			t.Cleanup(func() {
				release()
				<-ownerGone
			})
			var ownerErr error
			go func() {
				_, ownerErr = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "A", NonInteractive: true})
				close(ownerGone)
			}()
			<-started
			accepted := sa.BeginAccepted(sess.ID)
			defer accepted.Close()
			sa.Cancel(sess.ID)
			if finish == "before-dispatch" {
				release()
				<-ownerGone
			}
			result, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "B", Accepted: accepted})
			require.NoError(t, err)
			require.Nil(t, result)
			release()
			<-ownerGone
			require.ErrorIs(t, ownerErr, context.Canceled)
			require.Equal(t, 1, calls, "B was covered by Cancel and must not stream")
			require.Len(t, events, 1)
			require.Equal(t, sess.ID, events[0].SessionID)
			require.Equal(t, "session", events[0].SessionTitle)
			require.Equal(t, notify.TypeAgentFinished, events[0].Type)
			require.Equal(t, []bool{false}, busy, "the terminal notification must observe idle")
			require.False(t, sa.IsSessionBusy(sess.ID))
		})
	}
}

// Exercise the handoff through a public notification callback, after the
// old implementation released its active request but before the next run.
func TestRun_CancelAtQueueHandoff(t *testing.T) {
	t.Parallel()
	for _, summarize := range []bool{false, true} {
		name := "ordinary"
		if summarize {
			name = "automatic-summary"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			env, sess, model, sa := newLifecycleTestAgent(t, "session")
			seedUserMessage(t, env.messages, sess.ID, "earlier")
			broker := pubsub.NewBroker[notify.Notification]()
			defer broker.Shutdown()
			completions := pubsub.NewBroker[notify.RunComplete]()
			defer completions.Shutdown()
			events := completions.Subscribe(t.Context())
			sa.runComplete = completions
			notified := false
			lastBusy := true
			sa.notify = observingNotifications{Broker: broker, observe: func(n notify.Notification) {
				lastBusy = sa.IsSessionBusy(sess.ID)
				if n.Type != notify.TypeAgentFinished || notified {
					return
				}
				notified = true
				require.True(t, sa.IsSessionBusy(sess.ID), "the handoff must retain cancellation ownership")
				sa.Cancel(sess.ID)
			}}
			calls := 0
			model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
				calls++
				if calls == 1 {
					result, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "B", Prompt: "queued"})
					require.NoError(t, err)
					require.Nil(t, result)
					if summarize {
						return compactionStream, nil
					}
				}
				return (&finishStreamModel{text: "done"}).Stream(ctx, call)
			}
			_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "A", Prompt: "initial"})
			if summarize {
				require.ErrorIs(t, err, context.Canceled)
				require.Equal(t, 3, calls, "A, summary, B; A must not resume after Cancel")
			} else {
				require.True(t, err == nil || errors.Is(err, context.Canceled))
				require.Equal(t, 1, calls, "the queued call must not stream after Cancel")
			}
			require.True(t, notified)
			require.False(t, lastBusy, "the final lifecycle notification must observe idle")
			require.False(t, sa.IsSessionBusy(sess.ID))
			require.Zero(t, sa.QueuedPrompts(sess.ID))
			require.Len(t, events, 2, "both requests must retain their completion")
			completed := make(map[string]notify.RunComplete)
			for range 2 {
				event := <-events
				require.NotContains(t, completed, event.Payload.RunID)
				completed[event.Payload.RunID] = event.Payload
			}
			require.Equal(t, summarize, completed["A"].Cancelled)
			require.True(t, completed["B"].Cancelled, "B has not delivered its terminal event when cancellation arrives")
		})
	}
}

func TestRun_CanceledHandoffPersistenceFailureIsReported(t *testing.T) {
	t.Parallel()
	for _, failPersistence := range []bool{false, true} {
		t.Run(fmt.Sprintf("persistence-fails=%t", failPersistence), func(t *testing.T) {
			t.Parallel()
			env, sess, model, sa := newLifecycleTestAgent(t, "session")
			seedUserMessage(t, env.messages, sess.ID, "earlier")
			failure := errors.New("B cancellation persistence failed")
			if failPersistence {
				sa.messages = failedQueuedPreparation{Service: env.messages, err: failure}
			}
			completions := pubsub.NewBroker[notify.RunComplete]()
			defer completions.Shutdown()
			sa.runComplete = completions
			events := completions.Subscribe(t.Context())
			notifications := pubsub.NewBroker[notify.Notification]()
			defer notifications.Shutdown()
			cancelled := false
			var reported []notify.Notification
			sa.notify = observingNotifications{Broker: notifications, observe: func(n notify.Notification) {
				if n.Type == notify.TypeAgentError {
					reported = append(reported, n)
				}
				if n.Type == notify.TypeAgentFinished && !cancelled {
					cancelled = true
					sa.Cancel(sess.ID)
				}
			}}
			calls := 0
			model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
				calls++
				require.Equal(t, 1, calls, "B must not stream after cancellation")
				_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "B", Prompt: "B"})
				require.NoError(t, err)
				return (&finishStreamModel{text: "A answer"}).Stream(ctx, call)
			}
			result, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "A", Prompt: "A"})
			require.NoError(t, err, "B's persistence failure must not replace A's success")
			require.NotNil(t, result)
			require.True(t, cancelled)
			require.Equal(t, 1, calls)
			require.False(t, sa.IsSessionBusy(sess.ID))
			require.Zero(t, sa.QueuedPrompts(sess.ID))
			require.Len(t, events, 2)
			completed := make(map[string]notify.RunComplete)
			for range 2 {
				complete := (<-events).Payload
				require.NotContains(t, completed, complete.RunID)
				completed[complete.RunID] = complete
			}
			require.False(t, completed["A"].Cancelled)
			require.Empty(t, completed["A"].Error)
			require.Equal(t, "A answer", completed["A"].Text)
			if failPersistence {
				require.Contains(t, completed["B"].Error, failure.Error())
				if completed["B"].Cancelled {
					t.Error("B's persistence failure is classified as benign cancellation")
				}
				require.Len(t, reported, 1, "the detached failure needs a correlated error notification")
				require.Equal(t, "B", reported[0].RunID)
				require.Contains(t, reported[0].Message, failure.Error())
			} else {
				require.True(t, completed["B"].Cancelled)
				require.Empty(t, completed["B"].Error)
				require.Empty(t, reported)
			}
		})
	}
}

func TestRun_PublicSubmissionCannotStealQueueHandoff(t *testing.T) {
	t.Parallel()
	env, sess, model, sa := newLifecycleTestAgent(t, "session")
	broker := pubsub.NewBroker[notify.Notification]()
	defer broker.Shutdown()
	var busy []bool
	sa.notify = observingNotifications{Broker: broker, observe: func(n notify.Notification) {
		if n.Type != notify.TypeAgentFinished {
			return
		}
		busy = append(busy, sa.IsSessionBusy(sess.ID))
		if len(busy) == 1 {
			result, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "C", Prompt: "C"})
			require.NoError(t, err)
			require.Nil(t, result, "a public submission during handoff must queue, not execute")
		}
	}}
	calls := 0
	model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
		calls++
		if calls == 1 {
			result, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "B", Prompt: "B"})
			require.NoError(t, err)
			require.Nil(t, result)
		}
		return (&finishStreamModel{text: "done"}).Stream(ctx, call)
	}
	_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "A", Prompt: "A"})
	require.NoError(t, err)
	require.Equal(t, []bool{true, true, false}, busy, "only the final turn may release the owner before notifying")
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var prompts []string
	for _, msg := range msgs {
		if msg.Role == message.User {
			prompts = append(prompts, msg.Content().String())
		}
	}
	require.Equal(t, []string{"A", "B", "C"}, prompts)
}

type summaryDeletionFailure struct {
	message.Service
	failure   error
	attempted bool
}

func (m *summaryDeletionFailure) Delete(ctx context.Context, id string) error {
	m.attempted = true
	if m.failure != nil {
		return m.failure
	}
	return m.Service.Delete(ctx, id)
}

func TestRun_CanceledSummaryCleanupFailureIsNotBenignCancellation(t *testing.T) {
	t.Parallel()
	for _, failCleanup := range []bool{false, true} {
		t.Run(fmt.Sprintf("cleanup-fails=%t", failCleanup), func(t *testing.T) {
			t.Parallel()
			env, sess, model, sa := newLifecycleTestAgent(t, "session")
			failure := errors.New("summary deletion failed")
			messages := &summaryDeletionFailure{Service: env.messages}
			if failCleanup {
				messages.failure = failure
			}
			sa.messages = messages
			calls := 0
			model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
				calls++
				if calls == 1 {
					return compactionStream, nil
				}
				require.Equal(t, 2, calls, "cancellation must not resume A")
				require.True(t, sa.IsSessionBusy(sess.ID))
				sa.Cancel(sess.ID)
				return nil, ctx.Err()
			}
			var completions []notify.RunComplete
			_, runErr := sa.Run(t.Context(), SessionAgentCall{
				SessionID: sess.ID, RunID: "A", Prompt: "A",
				OnComplete: func(complete notify.RunComplete) { completions = append(completions, complete) },
			})
			require.Equal(t, 2, calls)
			require.True(t, messages.attempted)
			require.False(t, sa.IsSessionBusy(sess.ID))
			require.Len(t, completions, 1)
			require.Equal(t, "A", completions[0].RunID)
			stored, err := env.messages.List(t.Context(), sess.ID)
			require.NoError(t, err)
			summaries := 0
			for _, msg := range stored {
				if msg.IsSummaryMessage {
					summaries++
					require.False(t, msg.IsFinished())
				}
			}
			if failCleanup {
				require.Equal(t, 1, summaries, "the failed deletion leaves an unfinished summary")
				require.ErrorIs(t, runErr, failure)
				require.Contains(t, completions[0].Error, failure.Error())
				require.False(t, completions[0].Cancelled, "cleanup failure must not be hidden as a benign cancellation")
				require.False(t, errors.Is(runErr, context.Canceled))
			} else {
				require.Zero(t, summaries)
				require.ErrorIs(t, runErr, context.Canceled)
				require.True(t, completions[0].Cancelled)
			}
		})
	}
}

func TestRun_PostCancelSubmissionGetsNewOwner(t *testing.T) {
	t.Parallel()
	for _, summarize := range []bool{false, true} {
		for _, pendingAccept := range []bool{false, true} {
			t.Run(fmt.Sprintf("summary=%t/pending=%t", summarize, pendingAccept), func(t *testing.T) {
				t.Parallel()
				_, sess, model, sa := newLifecycleTestAgent(t, "session")
				broker := pubsub.NewBroker[notify.Notification]()
				defer broker.Shutdown()
				var pending *AcceptedRun
				if pendingAccept {
					pending = sa.BeginAccepted(sess.ID)
					defer pending.Close()
				}
				cancelled := false
				lastBusy := true
				sa.notify = observingNotifications{Broker: broker, observe: func(n notify.Notification) {
					lastBusy = sa.IsSessionBusy(sess.ID)
					if cancelled || n.Type != notify.TypeAgentFinished {
						return
					}
					cancelled = true
					require.True(t, lastBusy)
					sa.Cancel(sess.ID)
					result, err := sa.Run(t.Context(), SessionAgentCall{
						SessionID: sess.ID, RunID: "C", Prompt: "C", Accepted: sa.BeginAccepted(sess.ID),
					})
					require.NoError(t, err)
					require.Nil(t, result)
				}}
				calls := 0
				model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
					calls++
					require.NoError(t, ctx.Err(), "C must not inherit A's cancellation")
					if calls == 1 {
						_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "B", Prompt: "B"})
						require.NoError(t, err)
						if summarize {
							return compactionStream, nil
						}
					}
					return (&finishStreamModel{text: "done"}).Stream(ctx, call)
				}
				_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "A", Prompt: "A"})
				expectedCalls := 2 // A, C. B was dequeued but cancelled before streaming.
				if summarize {
					require.ErrorIs(t, err, context.Canceled)
					expectedCalls = 4 // A, summary, B, C. A must not resume.
				} else {
					require.NoError(t, err, "B's cancellation must not replace A's completed outcome")
				}
				require.Equal(t, expectedCalls, calls)
				require.True(t, cancelled)
				require.False(t, lastBusy)
				require.Zero(t, sa.QueuedPrompts(sess.ID))
			})
		}
	}
}

func TestSummarize_PostCancelSubmissionBeforeDequeue(t *testing.T) {
	t.Parallel()
	for _, automatic := range []bool{false, true} {
		t.Run(fmt.Sprint(automatic), func(t *testing.T) {
			t.Parallel()
			env, sess, model, sa := newLifecycleTestAgent(t, "session")
			seedUserMessage(t, env.messages, sess.ID, "earlier")
			cancelled := false
			sa.sessions = observingSummarySave{Service: env.sessions, afterSave: func() {
				if cancelled {
					return
				}
				cancelled = true
				sa.Cancel(sess.ID)
				result, err := sa.Run(t.Context(), SessionAgentCall{
					SessionID: sess.ID, RunID: "C", Prompt: "C", Accepted: sa.BeginAccepted(sess.ID),
				})
				require.NoError(t, err)
				require.Nil(t, result)
			}}
			calls := 0
			model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
				calls++
				require.NoError(t, ctx.Err())
				if automatic && calls == 1 {
					return compactionStream, nil
				}
				return (&finishStreamModel{text: "done"}).Stream(ctx, call)
			}
			expectedCalls := 2 // Summary, C.
			var err error
			if automatic {
				_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "A", Prompt: "A"})
				expectedCalls = 3 // A, summary, C; A must not resume.
			} else {
				err = sa.Summarize(t.Context(), sess.ID, nil, nil)
			}
			require.ErrorIs(t, err, context.Canceled)
			require.True(t, cancelled)
			require.Equal(t, expectedCalls, calls)
			require.Zero(t, sa.QueuedPrompts(sess.ID))
		})
	}
}

func TestRun_MixedModeFinalIdleNotification(t *testing.T) {
	t.Parallel()
	for _, initialNonInteractive := range []bool{false, true} {
		for _, queuedNonInteractive := range []bool{false, true} {
			for _, cancel := range []bool{false, true} {
				t.Run(fmt.Sprintf("initial=%t/queued=%t/cancel=%t", initialNonInteractive, queuedNonInteractive, cancel), func(t *testing.T) {
					t.Parallel()
					_, sess, model, sa := newLifecycleTestAgent(t, "session")
					broker := pubsub.NewBroker[notify.Notification]()
					defer broker.Shutdown()
					var busy []bool
					sa.notify = observingNotifications{Broker: broker, observe: func(notify.Notification) {
						busy = append(busy, sa.IsSessionBusy(sess.ID))
					}}
					calls := 0
					model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
						calls++
						if calls == 1 {
							if cancel {
								sa.Cancel(sess.ID)
							}
							_, err := sa.Run(t.Context(), SessionAgentCall{
								SessionID: sess.ID, RunID: "B", Prompt: "B",
								NonInteractive: queuedNonInteractive, Accepted: sa.BeginAccepted(sess.ID),
							})
							require.NoError(t, err)
							if cancel {
								return nil, ctx.Err()
							}
						}
						return (&finishStreamModel{text: "done"}).Stream(ctx, call)
					}
					_, err := sa.Run(t.Context(), SessionAgentCall{
						SessionID: sess.ID, RunID: "A", Prompt: "A", NonInteractive: initialNonInteractive,
					})
					if cancel {
						require.ErrorIs(t, err, context.Canceled)
					} else {
						require.NoError(t, err)
					}
					require.Equal(t, 2, calls)
					if initialNonInteractive && queuedNonInteractive {
						require.Empty(t, busy)
					} else {
						require.NotEmpty(t, busy)
						require.False(t, busy[len(busy)-1], "a noninteractive successor must not suppress the interactive owner's idle edge")
					}
				})
			}
		}
	}
}

func TestRun_InteractiveAdmissionRetainsIdleNotification(t *testing.T) {
	t.Parallel()
	for _, cancel := range []bool{false, true} {
		t.Run(fmt.Sprint(cancel), func(t *testing.T) {
			t.Parallel()
			_, sess, model, sa := newLifecycleTestAgent(t, "session")
			sa.disableAutoSummarize = true
			broker := pubsub.NewBroker[notify.Notification]()
			defer broker.Shutdown()
			var busy []bool
			sa.notify = observingNotifications{Broker: broker, observe: func(notify.Notification) {
				busy = append(busy, sa.IsSessionBusy(sess.ID))
			}}
			calls := 0
			model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
				calls++
				if calls == 1 {
					_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "interactive follow-up"})
					require.NoError(t, err)
					if cancel {
						sa.Cancel(sess.ID)
						return nil, ctx.Err()
					}
					return compactionStream, nil
				}
				require.Zero(t, sa.QueuedPrompts(sess.ID), "anonymous follow-up should fold, not start a new turn")
				return (&finishStreamModel{text: "done"}).Stream(ctx, call)
			}
			_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "A", NonInteractive: true})
			if cancel {
				require.ErrorIs(t, err, context.Canceled)
			} else {
				require.NoError(t, err)
				require.Equal(t, 2, calls)
			}
			require.NotEmpty(t, busy)
			require.False(t, busy[len(busy)-1])
		})
	}
}

func TestSummarize_QueuedTurnToolsReceiveSession(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "summary session")
	require.NoError(t, err)
	seedUserMessage(t, env.messages, sess.ID, "earlier")
	model := &lifecycleModel{}
	sa := testSessionAgent(env, model, &finishStreamModel{text: "title"}, "system", tools.NewTodosTool(env.sessions))
	a, ok := sa.(*sessionAgent)
	require.True(t, ok)
	completions := pubsub.NewBroker[notify.RunComplete]()
	defer completions.Shutdown()
	a.runComplete = completions
	events := completions.Subscribe(t.Context())
	calls := 0
	model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
		calls++
		switch calls {
		case 1:
			result, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "B", Prompt: "create a todo"})
			require.NoError(t, err)
			require.Nil(t, result)
			return (&finishStreamModel{text: "summary"}).Stream(ctx, call)
		case 2:
			return func(yield func(fantasy.StreamPart) bool) {
				if !yield(fantasy.StreamPart{
					Type: fantasy.StreamPartTypeToolCall, ID: "todo-1", ToolCallName: tools.TodosToolName,
					ToolCallInput: `{"todos":[{"content":"Verify handoff","status":"pending","active_form":"Verifying handoff"}]}`,
				}) {
					return
				}
				yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
			}, nil
		default:
			return (&finishStreamModel{text: "done"}).Stream(ctx, call)
		}
	}
	require.Empty(t, tools.GetSessionFromContext(t.Context()))
	require.NoError(t, sa.Summarize(t.Context(), sess.ID, nil, nil))
	require.Equal(t, 3, calls, "summary, queued tool call, queued final answer")
	updated, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, []session.Todo{{Content: "Verify handoff", Status: session.TodoStatusPending, ActiveForm: "Verifying handoff"}}, updated.Todos)
	require.Len(t, events, 1)
	complete := (<-events).Payload
	require.Equal(t, "B", complete.RunID)
	require.Empty(t, complete.Error)
	require.False(t, complete.Cancelled)
	require.Equal(t, "done", complete.Text)
	require.False(t, sa.IsSessionBusy(sess.ID))
}

func TestSummarize_ClearedInteractiveAdmissionNotifiesIdle(t *testing.T) {
	t.Parallel()
	for _, cancel := range []bool{false, true} {
		t.Run(fmt.Sprint(cancel), func(t *testing.T) {
			t.Parallel()
			env, sess, model, sa := newLifecycleTestAgent(t, "summary session")
			seedUserMessage(t, env.messages, sess.ID, "earlier")
			broker := pubsub.NewBroker[notify.Notification]()
			defer broker.Shutdown()
			var events []notify.Notification
			sa.notify = observingNotifications{Broker: broker, observe: func(n notify.Notification) {
				require.False(t, sa.IsSessionBusy(sess.ID))
				events = append(events, n)
			}}
			model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
				_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "B"})
				require.NoError(t, err)
				if cancel {
					sa.Cancel(sess.ID)
					return nil, ctx.Err()
				}
				sa.ClearQueue(sess.ID)
				return (&finishStreamModel{text: "summary"}).Stream(ctx, call)
			}
			err := sa.Summarize(t.Context(), sess.ID, nil, nil)
			if cancel {
				require.ErrorIs(t, err, context.Canceled)
			} else {
				require.NoError(t, err)
			}
			state := notify.FinishIdleSuccess
			if cancel {
				state = notify.FinishIdleUnsuccessful
			}
			require.Equal(t, []notify.Notification{{SessionID: sess.ID, SessionTitle: "summary session", Type: notify.TypeAgentFinished, FinishState: state}}, events)
		})
	}
}

func TestRun_IdleOutcomeAcrossQueuedTurns(t *testing.T) {
	t.Parallel()
	for _, failure := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			t.Parallel()
			_, sess, model, sa := newLifecycleTestAgent(t, "session")
			broker := pubsub.NewBroker[notify.Notification]()
			defer broker.Shutdown()
			var states []notify.FinishState
			sa.notify = observingNotifications{Broker: broker, observe: func(n notify.Notification) {
				if n.Type == notify.TypeAgentFinished {
					states = append(states, n.FinishState)
					require.Equal(t, n.FinishState == notify.FinishContinuing, sa.IsSessionBusy(sess.ID))
				}
			}}
			calls := 0
			model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
				calls++
				if calls == 1 {
					_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "B", RunID: "B"})
					require.NoError(t, err)
				}
				if calls == failure {
					return nil, errors.New("model failed")
				}
				return (&finishStreamModel{text: "done"}).Stream(ctx, call)
			}
			_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "A", RunID: "A"})
			if failure == 1 {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, 2, calls)
			require.NotEmpty(t, states)
			want := notify.FinishIdleSuccess
			if failure != 0 {
				want = notify.FinishIdleUnsuccessful
			}
			require.Equal(t, want, states[len(states)-1])
			// A new busy interval does not inherit the previous failure.
			_, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "C"})
			require.NoError(t, err)
			require.Equal(t, notify.FinishIdleSuccess, states[len(states)-1])
		})
	}
}
