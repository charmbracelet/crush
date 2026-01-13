package agent

import (
	"context"

	"github.com/charmbracelet/crush/internal/pubsub"
)

type SubagentEvent struct {
	Name  string
	Color string
}

var subagentBroker = pubsub.NewBroker[SubagentEvent]()

func SubscribeSubagentEvents(ctx context.Context) <-chan pubsub.Event[SubagentEvent] {
	return subagentBroker.Subscribe(ctx)
}

func publishSubagentStarted(name, color string) {
	subagentBroker.Publish(pubsub.CreatedEvent, SubagentEvent{
		Name:  name,
		Color: color,
	})
}

func publishSubagentStopped() {
	subagentBroker.Publish(pubsub.DeletedEvent, SubagentEvent{})
}
