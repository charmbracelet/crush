package app

import (
	"context"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/timeline"
	"github.com/charmbracelet/crush/internal/toolruntime"
)

func (app *App) setupTimeline(ctx context.Context) {
	app.setupTimelineFromSessions(ctx)
	app.setupTimelineFromToolRuntime(ctx)
}

func (app *App) setupTimelineFromSessions(ctx context.Context) {
	app.serviceEventsWG.Go(func() {
		sub := app.Sessions.Subscribe(ctx)
		modeStates := make(map[string]session.ModeState)
		if existing, err := app.Sessions.List(ctx); err == nil {
			for _, sess := range existing {
				modeStates[sess.ID] = session.ModeStateFromSession(sess)
			}
		}
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-sub:
				if !ok {
					return
				}
				sess := event.Payload
				if event.Type == pubsub.DeletedEvent {
					delete(modeStates, sess.ID)
					continue
				}
				current := session.ModeStateFromSession(sess)
				if previous, ok := modeStates[sess.ID]; ok {
					transition := session.ModeTransition{Previous: previous, Current: current}
					if transition.Changed() {
						app.Timeline.Publish(timeline.ModeChangedEvent(sess.ID, transition))
					}
				}
				modeStates[sess.ID] = current
				if event.Type == pubsub.CreatedEvent && sess.ParentSessionID != "" {
					if events := app.Timeline.ListBySession(sess.ParentSessionID); len(events) == 0 || events[len(events)-1].ChildSessionID != sess.ID || events[len(events)-1].Type != timeline.EventChildSessionStarted {
						app.Timeline.Publish(timeline.ChildSessionStartedEvent(sess.ParentSessionID, sess.ID, sess.Title))
					}
				}
			}
		}
	})
}

func (app *App) setupTimelineFromToolRuntime(ctx context.Context) {
	app.serviceEventsWG.Go(func() {
		sub := app.ToolRuntime.Subscribe(ctx)
		states := make(map[string]map[string]toolruntime.State)
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-sub:
				if !ok {
					return
				}
				state := event.Payload
				if state.SessionID == "" || state.ToolCallID == "" {
					continue
				}
				if _, ok := states[state.SessionID]; !ok {
					states[state.SessionID] = make(map[string]toolruntime.State)
				}
				if event.Type == pubsub.DeletedEvent {
					delete(states[state.SessionID], state.ToolCallID)
					if len(states[state.SessionID]) == 0 {
						delete(states, state.SessionID)
					}
					continue
				}
				var previous *toolruntime.State
				if prev, ok := states[state.SessionID][state.ToolCallID]; ok {
					previous = &prev
				}
				for _, timelineEvent := range timeline.ToolEventsFromRuntime(previous, state) {
					app.Timeline.Publish(timelineEvent)
				}
				states[state.SessionID][state.ToolCallID] = state
			}
		}
	})
}
