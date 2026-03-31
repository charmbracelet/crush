package app

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/plugin"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/timeline"
	"github.com/charmbracelet/crush/internal/toolruntime"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestSetupSubscriber_NormalFlow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newSubscriberFixture(t, 10)

		time.Sleep(10 * time.Millisecond)
		synctest.Wait()

		f.broker.Publish(pubsub.CreatedEvent, "event1")
		f.broker.Publish(pubsub.CreatedEvent, "event2")

		for range 2 {
			select {
			case <-f.outputCh:
			case <-time.After(5 * time.Second):
				t.Fatal("Timed out waiting for messages")
			}
		}

		f.cancel()
		f.wg.Wait()
	})
}

func TestSetupSubscriber_SlowConsumer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newSubscriberFixture(t, 0)

		const numEvents = 5

		var pubWg sync.WaitGroup
		pubWg.Go(func() {
			for range numEvents {
				f.broker.Publish(pubsub.CreatedEvent, "event")
				time.Sleep(10 * time.Millisecond)
				synctest.Wait()
			}
		})

		time.Sleep(time.Duration(numEvents) * (subscriberSendTimeout + 20*time.Millisecond))
		synctest.Wait()

		received := 0
		for {
			select {
			case <-f.outputCh:
				received++
			default:
				pubWg.Wait()
				f.cancel()
				f.wg.Wait()
				require.Less(t, received, numEvents, "Slow consumer should have dropped some messages")
				return
			}
		}
	})
}

func TestSetupSubscriber_ContextCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newSubscriberFixture(t, 10)

		f.broker.Publish(pubsub.CreatedEvent, "event1")
		time.Sleep(100 * time.Millisecond)
		synctest.Wait()

		f.cancel()
		f.wg.Wait()
	})
}

func TestSetupSubscriber_DrainAfterDrop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newSubscriberFixture(t, 0)

		time.Sleep(10 * time.Millisecond)
		synctest.Wait()

		// First event: nobody reads outputCh so the timer fires (message dropped).
		f.broker.Publish(pubsub.CreatedEvent, "event1")
		time.Sleep(subscriberSendTimeout + 25*time.Millisecond)
		synctest.Wait()

		// Second event: triggers Stop()==false path; without the fix this deadlocks.
		f.broker.Publish(pubsub.CreatedEvent, "event2")

		// If the timer drain deadlocks, wg.Wait never returns.
		done := make(chan struct{})
		go func() {
			f.cancel()
			f.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("setupSubscriber goroutine hung — likely timer drain deadlock")
		}
	})
}

func TestSetupSubscriber_NoTimerLeak(t *testing.T) {
	defer goleak.VerifyNone(t)
	synctest.Test(t, func(t *testing.T) {
		f := newSubscriberFixture(t, 100)

		for range 100 {
			f.broker.Publish(pubsub.CreatedEvent, "event")
			time.Sleep(5 * time.Millisecond)
			synctest.Wait()
		}

		f.cancel()
		f.wg.Wait()
	})
}

type subscriberFixture struct {
	broker   *pubsub.Broker[string]
	wg       sync.WaitGroup
	outputCh chan tea.Msg
	cancel   context.CancelFunc
}

type messageCreatedPlugin struct {
	called atomic.Int32
}

func (p *messageCreatedPlugin) Name() string {
	return "message-created-plugin"
}

func (p *messageCreatedPlugin) Close(ctx context.Context) error {
	return nil
}

func (p *messageCreatedPlugin) Init(ctx context.Context, input plugin.PluginInput) (plugin.Hooks, error) {
	return plugin.Hooks{
		MessageCreated: func(ctx context.Context, msg message.Message) error {
			p.called.Add(1)
			return nil
		},
	}, nil
}

func TestSetupMessageSubscriber_TriggersMessageCreatedHook(t *testing.T) {
	plugin.Reset()
	defer plugin.Reset()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	testPlugin := &messageCreatedPlugin{}
	plugin.Register(testPlugin)

	conn, store := setupMessageSubscriberDependencies(t)
	defer func() {
		require.NoError(t, conn.Close())
	}()

	sessions := session.NewService(db.New(conn), conn)
	messages := message.NewService(db.New(conn))

	require.NoError(t, plugin.Init(ctx, plugin.PluginInput{
		Config:     store,
		Sessions:   sessions,
		Messages:   messages,
		WorkingDir: store.WorkingDir(),
	}))

	var wg sync.WaitGroup
	outputCh := make(chan tea.Msg, 8)
	setupMessageSubscriber(ctx, &wg, messages.Subscribe, outputCh)

	testSession, err := sessions.Create(ctx, "message hook")
	require.NoError(t, err)
	_, err = messages.Create(ctx, testSession.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "hello"}},
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return testPlugin.called.Load() == 1
	}, time.Second, 10*time.Millisecond)

	cancel()
	wg.Wait()
}

func setupMessageSubscriberDependencies(t *testing.T) (*sql.DB, *config.ConfigStore) {
	t.Helper()
	workingDir := t.TempDir()
	dataDir := t.TempDir()
	t.Cleanup(func() {
		require.NoError(t, log.ResetForTesting())
	})
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "crush.json"), []byte(`{"options":{"disable_provider_auto_update":true}}`), 0o644))

	store, err := config.Init(workingDir, dataDir, false)
	require.NoError(t, err)

	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	return conn, store
}

func newSubscriberFixture(t *testing.T, bufSize int) *subscriberFixture {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	f := &subscriberFixture{
		broker:   pubsub.NewBroker[string](),
		outputCh: make(chan tea.Msg, bufSize),
		cancel:   cancel,
	}
	t.Cleanup(f.broker.Shutdown)

	setupSubscriber(ctx, &f.wg, "test", func(ctx context.Context) <-chan pubsub.Event[string] {
		return f.broker.Subscribe(ctx)
	}, f.outputCh)

	return f
}

func TestSetupTimelineFromToolRuntimePublishesTimelineEvents(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	app := &App{
		ToolRuntime:     toolruntime.NewService(),
		Timeline:        timeline.NewService(),
		serviceEventsWG: &sync.WaitGroup{},
	}
	app.setupTimelineFromToolRuntime(ctx)

	sub := app.Timeline.Subscribe(ctx)
	app.ToolRuntime.Publish(toolruntime.State{SessionID: "sess-1", ToolCallID: "tool-1", ToolName: "bash", Status: toolruntime.StatusRunning, SnapshotText: "first"})
	app.ToolRuntime.Publish(toolruntime.State{SessionID: "sess-1", ToolCallID: "tool-1", ToolName: "bash", Status: toolruntime.StatusCompleted, SnapshotText: "done"})

	seen := make([]timeline.EventType, 0, 3)
	for len(seen) < 3 {
		select {
		case event := <-sub:
			seen = append(seen, event.Payload.Type)
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for timeline events: %v", seen)
		}
	}

	require.Equal(t, []timeline.EventType{timeline.EventToolStarted, timeline.EventToolProgress, timeline.EventToolFinished}, seen)
	cancel()
	app.serviceEventsWG.Wait()
}
