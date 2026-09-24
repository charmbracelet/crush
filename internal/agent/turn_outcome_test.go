package agent

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestRun_PostCancelAcceptedTurnRetainsSummarySequence(t *testing.T) {
	t.Parallel()
	env, sess, model, sa := newLifecycleTestAgent(t, "session")
	seedUserMessage(t, env.messages, sess.ID, "earlier")
	broker := pubsub.NewBroker[notify.RunComplete]()
	defer broker.Shutdown()
	sa.runComplete = broker
	events := broker.Subscribe(t.Context())
	pending := sa.BeginAccepted(sess.ID)
	defer pending.Close()
	sa.Cancel(sess.ID)
	calls := 0
	model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
		calls++
		require.NoError(t, ctx.Err())
		switch calls {
		case 1:
			return compactionStream, nil
		case 2:
			return (&finishStreamModel{text: "summary"}).Stream(ctx, call)
		default:
			return (&finishStreamModel{text: "B resumed"}).Stream(ctx, call)
		}
	}
	result, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID, RunID: "B", Prompt: "B", Accepted: sa.BeginAccepted(sess.ID),
	})
	require.NoError(t, err, "an older cancellation must not cover B's continuation")
	require.NotNil(t, result)
	require.Equal(t, fantasy.ResponseContent{fantasy.TextContent{Text: "B resumed"}}, result.Response.Content)
	require.Equal(t, 3, calls)
	require.Len(t, events, 1)
	complete := (<-events).Payload
	require.Equal(t, "B", complete.RunID)
	require.False(t, complete.Cancelled)
	require.Empty(t, complete.Error)
	require.Equal(t, "B resumed", complete.Text)

	// Keeping B alive must not erase the cancellation covering P.
	result, err = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "P", Prompt: "P", Accepted: pending})
	require.NoError(t, err)
	require.Nil(t, result)
	require.Equal(t, 3, calls, "P must not stream")
	require.Len(t, events, 1)
	complete = (<-events).Payload
	require.Equal(t, "P", complete.RunID)
	require.True(t, complete.Cancelled)
	require.False(t, sa.IsSessionBusy(sess.ID))
	require.Zero(t, sa.QueuedPrompts(sess.ID))
}

func TestRun_IndependentTurnOutcomes(t *testing.T) {
	t.Parallel()
	for _, ids := range [][2]string{{"A", "B"}, {"", "B"}, {"A", ""}, {"", ""}, {"same", "same"}} {
		for _, summarize := range []bool{false, true} {
			for _, failInitial := range []bool{false, true} {
				for _, failQueued := range []bool{false, true} {
					t.Run(fmt.Sprintf("ids=%v/summary=%t/initial-error=%t/queued-error=%t", ids, summarize, failInitial, failQueued), func(t *testing.T) {
						t.Parallel()
						_, sess, model, sa := newLifecycleTestAgent(t, "session")
						completions := pubsub.NewBroker[notify.RunComplete]()
						defer completions.Shutdown()
						sa.runComplete = completions
						queuedEvents := completions.Subscribe(t.Context())
						notifications := pubsub.NewBroker[notify.Notification]()
						defer notifications.Shutdown()
						sa.notify = notifications
						notificationEvents := notifications.Subscribe(t.Context())
						initialError := errors.New("initial failure")
						queuedError := errors.New("queued failure")
						calls := 0
						model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
							calls++
							if calls == 1 {
								result, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: ids[1], Prompt: "B"})
								require.NoError(t, err)
								require.Nil(t, result)
								if failInitial && !summarize {
									return nil, initialError
								}
								if summarize {
									return compactionStream, nil
								}
								return (&finishStreamModel{text: "A answer"}).Stream(ctx, call)
							}
							if summarize && calls == 2 {
								return (&finishStreamModel{text: "summary"}).Stream(ctx, call)
							}
							queuedCall := 2
							if summarize {
								queuedCall = 3
							}
							if calls == queuedCall {
								if failQueued {
									return nil, queuedError
								}
								return (&finishStreamModel{text: "B answer"}).Stream(ctx, call)
							}
							require.Equal(t, 4, calls, "only A's continuation remains")
							if failInitial {
								return nil, initialError
							}
							return (&finishStreamModel{text: "A resumed"}).Stream(ctx, call)
						}
						var initialEvents []notify.RunComplete
						result, err := sa.Run(t.Context(), SessionAgentCall{
							SessionID: sess.ID, RunID: ids[0], Prompt: "A",
							OnComplete: func(event notify.RunComplete) { initialEvents = append(initialEvents, event) },
						})
						wantText := "A answer"
						if summarize {
							wantText = "A resumed"
						}
						if failInitial {
							require.ErrorIs(t, err, initialError)
						} else {
							require.NoError(t, err)
							require.NotNil(t, result)
							require.Equal(t, fantasy.ResponseContent{fantasy.TextContent{Text: wantText}}, result.Response.Content)
						}
						require.Len(t, initialEvents, 1)
						require.Equal(t, ids[0], initialEvents[0].RunID)
						if failInitial {
							require.Contains(t, initialEvents[0].Error, initialError.Error())
						} else {
							require.Empty(t, initialEvents[0].Error)
							require.Equal(t, wantText, initialEvents[0].Text)
						}
						require.Len(t, queuedEvents, 1)
						queued := (<-queuedEvents).Payload
						require.Equal(t, ids[1], queued.RunID)
						var queuedErrors []notify.Notification
						for len(notificationEvents) > 0 {
							n := (<-notificationEvents).Payload
							if n.Type == notify.TypeAgentError {
								queuedErrors = append(queuedErrors, n)
							}
						}
						if failQueued {
							require.Contains(t, queued.Error, queuedError.Error())
							require.Len(t, queuedErrors, 1)
							require.Equal(t, ids[1], queuedErrors[0].RunID)
							require.Contains(t, queuedErrors[0].Message, queuedError.Error())
						} else {
							require.Empty(t, queued.Error)
							require.Equal(t, "B answer", queued.Text)
							require.Empty(t, queuedErrors)
						}
						require.False(t, sa.IsSessionBusy(sess.ID))
						require.Zero(t, sa.QueuedPrompts(sess.ID))
					})
				}
			}
		}
	}
}

type observingFlush struct {
	message.Service
	afterFlush func()
}

func (m observingFlush) FlushAll(ctx context.Context) error {
	err := m.Service.FlushAll(ctx)
	if err == nil {
		m.afterFlush()
	}
	return err
}

func TestRun_ClearSummaryContinuation(t *testing.T) {
	t.Parallel()
	for _, runID := range []string{"", "A"} {
		for _, phase := range []string{"before-enqueue", "queued", "after-dequeue", "preceding-failure"} {
			t.Run(runID+"/"+phase, func(t *testing.T) {
				t.Parallel()
				env, sess, model, sa := newLifecycleTestAgent(t, "session")
				var completed []notify.RunComplete
				clear := func() {
					sa.ClearQueue(sess.ID)
					require.Empty(t, completed, "queue clearing must not invoke the suspended caller's callback")
				}
				injected := false
				sa.messages = observingFlush{Service: env.messages, afterFlush: func() {
					if injected {
						return
					}
					injected = true
					// The summary has drained its queue but A has not yet
					// appended its continuation. C therefore precedes A.
					_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "C", Prompt: "C"})
					require.NoError(t, err)
					if phase == "before-enqueue" {
						clear()
					}
				}}
				calls := 0
				model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
					calls++
					switch calls {
					case 1:
						return compactionStream, nil
					case 2:
						return (&finishStreamModel{text: "summary"}).Stream(ctx, call)
					case 3:
						if phase != "before-enqueue" {
							require.Equal(t, 1, sa.QueuedPrompts(sess.ID), "anonymous continuations must not fold into C")
							if phase == "queued" {
								clear()
							}
							if phase == "preceding-failure" {
								return nil, errors.New("C failed with A's continuation still queued")
							}
							return (&finishStreamModel{text: "C answer"}).Stream(ctx, call)
						}
					case 4:
						require.Contains(t, []string{"after-dequeue", "preceding-failure"}, phase)
						if phase == "after-dequeue" {
							clear()
						}
					default:
						t.Fatal("unexpected continuation")
					}
					return (&finishStreamModel{text: "A resumed"}).Stream(ctx, call)
				}
				result, err := sa.Run(t.Context(), SessionAgentCall{
					SessionID: sess.ID, RunID: runID, Prompt: "A",
					OnComplete: func(event notify.RunComplete) { completed = append(completed, event) },
				})
				require.Len(t, completed, 1)
				require.Equal(t, runID, completed[0].RunID)
				if phase == "queued" {
					require.ErrorIs(t, err, context.Canceled)
					require.True(t, completed[0].Cancelled)
					require.Equal(t, 3, calls)
				} else {
					require.NoError(t, err)
					require.NotNil(t, result)
					require.Equal(t, fantasy.ResponseContent{fantasy.TextContent{Text: "A resumed"}}, result.Response.Content)
					require.Equal(t, "A resumed", completed[0].Text)
				}
				require.False(t, sa.IsSessionBusy(sess.ID))
				require.Zero(t, sa.QueuedPrompts(sess.ID))
			})
		}
	}
}

func TestRun_QueuedFailureDoesNotStrandContinuation(t *testing.T) {
	t.Parallel()
	for _, nested := range []bool{false, true} {
		t.Run(fmt.Sprint(nested), func(t *testing.T) {
			t.Parallel()
			env, sess, model, sa := newLifecycleTestAgent(t, "session")
			failure := errors.New("queued failure")
			if !nested {
				sa.messages = failedQueuedPreparation{Service: env.messages, err: failure}
			}
			broker := pubsub.NewBroker[notify.RunComplete]()
			defer broker.Shutdown()
			sa.runComplete = broker
			events := broker.Subscribe(t.Context())
			calls := 0
			model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
				calls++
				if calls == 1 {
					for _, prompt := range []string{"B", "C"} {
						_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: prompt, Prompt: prompt})
						require.NoError(t, err)
					}
					return compactionStream, nil
				}
				if nested {
					switch calls {
					case 3: // B also needs summarization.
						return compactionStream, nil
					case 5: // C fails inside B's summary drain.
						return nil, failure
					}
				}
				return (&finishStreamModel{text: fmt.Sprint("answer ", calls)}).Stream(ctx, call)
			}
			var initial []notify.RunComplete
			result, err := sa.Run(t.Context(), SessionAgentCall{
				SessionID: sess.ID, RunID: "A", Prompt: "A",
				OnComplete: func(event notify.RunComplete) { initial = append(initial, event) },
			})
			require.NoError(t, err)
			wantCalls, wantText := 4, "answer 4" // A, summary, C, A resumed; B failed preparation.
			if nested {
				wantCalls, wantText = 7, "answer 7" // A, summary, B, summary, C, B resumed, A resumed.
			}
			require.Equal(t, wantCalls, calls)
			require.NotNil(t, result)
			require.Equal(t, fantasy.ResponseContent{fantasy.TextContent{Text: wantText}}, result.Response.Content)
			require.Len(t, initial, 1)
			require.Equal(t, wantText, initial[0].Text)
			require.Empty(t, initial[0].Error)
			require.Len(t, events, 2)
			completed := make(map[string]notify.RunComplete)
			for range 2 {
				event := (<-events).Payload
				require.NotContains(t, completed, event.RunID)
				completed[event.RunID] = event
			}
			if nested {
				require.Contains(t, completed["C"].Error, failure.Error())
				require.Equal(t, "answer 6", completed["B"].Text)
			} else {
				require.Contains(t, completed["B"].Error, failure.Error())
				require.Equal(t, "answer 3", completed["C"].Text)
			}
			require.Zero(t, sa.QueuedPrompts(sess.ID))
		})
	}
}

func TestSummarize_DoesNotReturnQueuedFailure(t *testing.T) {
	t.Parallel()
	env, sess, model, sa := newLifecycleTestAgent(t, "session")
	seedUserMessage(t, env.messages, sess.ID, "earlier")
	failure := errors.New("queued preparation failed")
	sa.messages = failedQueuedPreparation{Service: env.messages, err: failure}
	broker := pubsub.NewBroker[notify.RunComplete]()
	defer broker.Shutdown()
	sa.runComplete = broker
	events := broker.Subscribe(t.Context())
	model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
		_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "B", Prompt: "B"})
		require.NoError(t, err)
		return (&finishStreamModel{text: "summary"}).Stream(ctx, call)
	}
	require.NoError(t, sa.Summarize(t.Context(), sess.ID, nil, nil))
	require.Len(t, events, 1)
	complete := (<-events).Payload
	require.Equal(t, "B", complete.RunID)
	require.Contains(t, complete.Error, failure.Error())
	saved, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.NotEmpty(t, saved.SummaryMessageID)
	require.False(t, sa.IsSessionBusy(sess.ID))
}

type observingSessionGet struct {
	session.Service
	beforeGet func(context.Context) error
}

func (s observingSessionGet) Get(ctx context.Context, id string) (session.Session, error) {
	if err := s.beforeGet(ctx); err != nil {
		return session.Session{}, err
	}
	return s.Service.Get(ctx, id)
}

func TestRun_NestedContinuationPreparationCompletion(t *testing.T) {
	t.Parallel()
	for _, cancel := range []bool{false, true} {
		t.Run(fmt.Sprint(cancel), func(t *testing.T) {
			t.Parallel()
			env, sess, model, sa := newLifecycleTestAgent(t, "session")
			seedUserMessage(t, env.messages, sess.ID, "earlier")
			injected, armed := false, false
			failure := errors.New("continuation preparation failed")
			sa.sessions = observingSessionGet{Service: env.sessions, beforeGet: func(ctx context.Context) error {
				if !armed {
					return nil
				}
				armed = false
				if cancel {
					sa.Cancel(sess.ID)
					return ctx.Err()
				}
				return failure
			}}
			sa.messages = observingFlush{Service: env.messages, afterFlush: func() {
				if !injected {
					injected = true
					_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "C", Prompt: "C"})
					require.NoError(t, err)
				}
			}}
			calls := 0
			model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
				calls++
				if calls == 1 || calls == 3 {
					return compactionStream, nil
				}
				if calls == 4 {
					armed = true // C's summary next promotes A's continuation.
				}
				return (&finishStreamModel{text: "done"}).Stream(ctx, call)
			}
			var completions []notify.RunComplete
			_, err := sa.Run(t.Context(), SessionAgentCall{
				SessionID: sess.ID, RunID: "A", Prompt: "A",
				OnComplete: func(event notify.RunComplete) { completions = append(completions, event) },
			})
			if cancel {
				require.ErrorIs(t, err, context.Canceled)
			} else {
				require.ErrorIs(t, err, failure)
			}
			require.Len(t, completions, 1)
			require.Equal(t, "A", completions[0].RunID)
			require.NotEmpty(t, completions[0].Error)
			require.Equal(t, cancel, completions[0].Cancelled)
		})
	}
}

func TestRun_NestedContinuationCancellationAgreesWithCompletion(t *testing.T) {
	t.Parallel()
	for _, cancelAtFinish := range []bool{false, true} {
		for _, successor := range []bool{false, true} {
			t.Run(fmt.Sprintf("cancel=%t/successor=%t", cancelAtFinish, successor), func(t *testing.T) {
				t.Parallel()
				env, sess, model, sa := newLifecycleTestAgent(t, "session")
				seedUserMessage(t, env.messages, sess.ID, "earlier")
				injected := false
				sa.messages = observingFlush{Service: env.messages, afterFlush: func() {
					if !injected {
						injected = true
						// C enters before A appends its summary continuation.
						_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "C", Prompt: "C"})
						require.NoError(t, err)
					}
				}}
				broker := pubsub.NewBroker[notify.Notification]()
				defer broker.Shutdown()
				completionBroker := pubsub.NewBroker[notify.RunComplete]()
				defer completionBroker.Shutdown()
				sa.runComplete = completionBroker
				events := completionBroker.Subscribe(t.Context())
				armed, canceled := false, false
				sa.notify = observingNotifications{Broker: broker, observe: func(n notify.Notification) {
					if armed && n.Type == notify.TypeAgentFinished {
						armed = false
						if cancelAtFinish {
							require.True(t, sa.IsSessionBusy(sess.ID))
							canceled = true
							sa.Cancel(sess.ID)
						}
					}
				}}
				calls := 0
				model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
					calls++
					require.LessOrEqual(t, calls, 7)
					if calls == 1 || calls == 3 {
						return compactionStream, nil
					}
					if calls == 5 {
						// C's summary has promoted A's continuation.
						armed = true
						if successor {
							_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "D", Prompt: "D"})
							require.NoError(t, err)
						}
						return (&finishStreamModel{text: "A resumed"}).Stream(ctx, call)
					}
					return (&finishStreamModel{text: "other"}).Stream(ctx, call)
				}
				var completions []notify.RunComplete
				result, err := sa.Run(t.Context(), SessionAgentCall{
					SessionID: sess.ID, RunID: "A", Prompt: "A",
					OnComplete: func(event notify.RunComplete) { completions = append(completions, event) },
				})
				require.Equal(t, cancelAtFinish, canceled)
				require.Len(t, completions, 1)
				require.Equal(t, "A", completions[0].RunID)
				require.Equal(t, cancelAtFinish, completions[0].Cancelled)
				require.False(t, sa.IsSessionBusy(sess.ID))
				require.Zero(t, sa.QueuedPrompts(sess.ID))
				wantDetached := 1
				if successor {
					wantDetached++
				}
				require.Len(t, events, wantDetached)
				seen := make(map[string]notify.RunComplete, wantDetached)
				for range wantDetached {
					complete := (<-events).Payload
					require.NotContains(t, seen, complete.RunID)
					seen[complete.RunID] = complete
				}
				require.Contains(t, seen, "C")
				if successor {
					require.Contains(t, seen, "D", "the already-dequeued successor must still complete")
					require.Equal(t, cancelAtFinish, seen["D"].Cancelled)
				}
				if cancelAtFinish {
					require.ErrorIs(t, err, context.Canceled, "A's return must agree with its canceled completion")
					require.Equal(t, err.Error(), completions[0].Error)
				} else {
					require.NoError(t, err)
					require.NotNil(t, result)
					require.Equal(t, fantasy.ResponseContent{fantasy.TextContent{Text: "A resumed"}}, result.Response.Content)
				}
			})
		}
	}
}

func finalCompactionStream(yield func(fantasy.StreamPart) bool) {
	for _, part := range []fantasy.StreamPart{
		{Type: fantasy.StreamPartTypeTextStart, ID: "answer"},
		{Type: fantasy.StreamPartTypeTextDelta, ID: "answer", Delta: "A answer"},
		{Type: fantasy.StreamPartTypeTextEnd, ID: "answer"},
		{
			Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop,
			Usage: fantasy.Usage{InputTokens: 190000, OutputTokens: 100},
		},
	} {
		if !yield(part) {
			return
		}
	}
}

func TestRun_FinishedAutomaticSummaryDoesNotInheritQueuedCancellation(t *testing.T) {
	t.Parallel()
	for _, needsContinuation := range []bool{false, true} {
		t.Run(fmt.Sprintf("continuation=%t", needsContinuation), func(t *testing.T) {
			t.Parallel()
			env, sess, model, sa := newLifecycleTestAgent(t, "session")
			seedUserMessage(t, env.messages, sess.ID, "earlier")
			broker := pubsub.NewBroker[notify.RunComplete]()
			defer broker.Shutdown()
			sa.runComplete = broker
			events := broker.Subscribe(t.Context())
			calls := 0
			model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
				calls++
				switch calls {
				case 1:
					_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "B", Prompt: "B"})
					require.NoError(t, err)
					if needsContinuation {
						return compactionStream, nil
					}
					return finalCompactionStream, nil
				case 2:
					return (&finishStreamModel{text: "summary"}).Stream(ctx, call)
				default:
					require.Equal(t, 3, calls, "only B should run after the summary")
					require.Empty(t, events, "A's completion must still wait for the summary's queue drain")
					saved, err := env.sessions.Get(ctx, sess.ID)
					require.NoError(t, err)
					require.NotEmpty(t, saved.SummaryMessageID)
					sa.Cancel(sess.ID)
					return nil, ctx.Err()
				}
			}
			result, runErr := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "A", Prompt: "A"})
			require.Equal(t, 3, calls)
			require.False(t, sa.IsSessionBusy(sess.ID))
			require.Zero(t, sa.QueuedPrompts(sess.ID))
			require.Len(t, events, 2)
			completed := make(map[string]notify.RunComplete)
			for _, runID := range []string{"B", "A"} {
				complete := (<-events).Payload
				require.Equal(t, runID, complete.RunID, "summary drain publication order must not change")
				completed[complete.RunID] = complete
			}
			require.True(t, completed["B"].Cancelled)
			if needsContinuation {
				require.ErrorIs(t, runErr, context.Canceled)
				require.True(t, completed["A"].Cancelled)
			} else {
				if completed["A"].Cancelled {
					t.Error("A's finished response and summary inherited B's cancellation")
				}
				require.NoError(t, runErr)
				require.NotNil(t, result)
				require.Equal(t, fantasy.ResponseContent{fantasy.TextContent{Text: "A answer"}}, result.Response.Content)
				require.Empty(t, completed["A"].Error)
				require.Equal(t, "A answer", completed["A"].Text)
			}
		})
	}
}

type failingSummarySave struct {
	session.Service
	err error
}

func (s failingSummarySave) Save(ctx context.Context, sess session.Session) (session.Session, error) {
	if sess.SummaryMessageID != "" {
		return session.Session{}, s.err
	}
	return s.Service.Save(ctx, sess)
}

func TestRun_FinalAnswerSummaryStillReportsOwnFailure(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"stream", "save", "before-handoff", "flush-without-queue"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			env, sess, model, sa := newLifecycleTestAgent(t, "session")
			seedUserMessage(t, env.messages, sess.ID, "earlier")
			failure := errors.New("summary save failed")
			switch phase {
			case "save":
				sa.sessions = failingSummarySave{Service: env.sessions, err: failure}
			case "before-handoff":
				sa.sessions = observingSummarySave{Service: env.sessions, afterSave: func() { sa.Cancel(sess.ID) }}
			case "flush-without-queue":
				sa.messages = observingFlush{Service: env.messages, afterFlush: func() { sa.Cancel(sess.ID) }}
			}
			calls := 0
			model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
				calls++
				if calls == 1 {
					if phase != "flush-without-queue" {
						_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "B", Prompt: "B"})
						require.NoError(t, err)
					}
					return finalCompactionStream, nil
				}
				if calls == 2 && phase == "stream" {
					sa.Cancel(sess.ID)
					return nil, ctx.Err()
				}
				return (&finishStreamModel{text: "summary"}).Stream(ctx, call)
			}
			var completions []notify.RunComplete
			_, runErr := sa.Run(t.Context(), SessionAgentCall{
				SessionID: sess.ID, RunID: "A", Prompt: "A",
				OnComplete: func(c notify.RunComplete) { completions = append(completions, c) },
			})
			require.Len(t, completions, 1)
			require.Equal(t, "A", completions[0].RunID)
			if phase == "save" {
				require.Equal(t, 3, calls, "a failed summary must still drain independent B")
				require.ErrorIs(t, runErr, failure)
				require.Contains(t, completions[0].Error, failure.Error())
				require.False(t, completions[0].Cancelled)
			} else {
				require.Equal(t, 2, calls)
				require.ErrorIs(t, runErr, context.Canceled)
				require.True(t, completions[0].Cancelled)
			}
			require.False(t, sa.IsSessionBusy(sess.ID))
			require.Zero(t, sa.QueuedPrompts(sess.ID))
		})
	}
}

func TestSummarize_DoesNotReturnQueuedCancellation(t *testing.T) {
	t.Parallel()
	env, sess, model, sa := newLifecycleTestAgent(t, "session")
	seedUserMessage(t, env.messages, sess.ID, "earlier")
	calls := 0
	model.stream = func(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
		calls++
		if calls == 1 {
			_, err := sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, RunID: "B", Prompt: "B"})
			require.NoError(t, err)
			return (&finishStreamModel{text: "summary"}).Stream(ctx, call)
		}
		sa.Cancel(sess.ID)
		return nil, ctx.Err()
	}
	require.NoError(t, sa.Summarize(t.Context(), sess.ID, nil, nil))
	require.Equal(t, 2, calls)
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, message.FinishReasonCanceled, msgs[len(msgs)-1].FinishReason())
}
