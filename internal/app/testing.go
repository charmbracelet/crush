package app

import (
	"context"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// NewForTest constructs a minimal [App] suitable for in-process tests
// that need a working event broker and permission service without
// booting a real config, database, LSP, MCP, or agent coordinator.
//
// The returned App has:
//
//   - A live `events` broker that [App.SendEvent] publishes to and
//     [App.Events] subscribes from.
//   - A real [permission.Service] whose request and notification
//     brokers are fanned into the events broker, so subscribers to
//     [App.Events] observe the same permission events the production
//     wiring would deliver to SSE clients.
//   - An [App.agentNotifications] broker.
//
// The caller owns lifetime: cancel ctx (or call [App.Shutdown]) to
// tear down the fan-in goroutines and the events broker.
func NewForTest(ctx context.Context) *App {
	app := &App{
		Permissions:        permission.NewPermissionService("", false, nil),
		globalCtx:          ctx,
		events:             pubsub.NewBroker[tea.Msg](),
		serviceEventsWG:    &sync.WaitGroup{},
		tuiWG:              &sync.WaitGroup{},
		agentNotifications: pubsub.NewBroker[notify.Notification](),
	}

	eventsCtx, cancel := context.WithCancel(ctx)
	app.eventsCtx = eventsCtx
	setupSubscriber(eventsCtx, app.serviceEventsWG, "permissions",
		app.Permissions.Subscribe, app.events)
	setupSubscriber(eventsCtx, app.serviceEventsWG, "permissions-notifications",
		app.Permissions.SubscribeNotifications, app.events)
	setupSubscriber(eventsCtx, app.serviceEventsWG, "agent-notifications",
		app.agentNotifications.Subscribe, app.events)
	app.cleanupFuncs = append(app.cleanupFuncs, func(context.Context) error {
		cancel()
		app.serviceEventsWG.Wait()
		app.events.Shutdown()
		return nil
	})
	return app
}

// ShutdownForTest tears down the App's event broker and fan-in
// goroutines. It is safe to call multiple times.
//
// Use this in tests instead of [App.Shutdown], which drives a full
// production shutdown path (database release, LSP teardown, MCP
// shutdown) that synthetic test apps cannot satisfy.
func (app *App) ShutdownForTest() {
	for _, cleanup := range app.cleanupFuncs {
		if cleanup != nil {
			_ = cleanup(context.Background())
		}
	}
	app.cleanupFuncs = nil
}
