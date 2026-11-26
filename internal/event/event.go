package event

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/version"
	"github.com/posthog/posthog-go"
)

const (
	endpoint = "https://data.charm.land"
	key      = "phc_4zt4VgDWLqbYnJYEwLRxFoaTL2noNrQij0C6E8k3I0V"

	nonInteractiveAttrName       = "NonInteractive"
	nonInteractiveNestedAttrName = "NonInteractiveNested"
	continueSessionByIDAttrName  = "ContinueSessionByID"
	continueLastSessionAttrName  = "ContinueLastSession"
)

var (
	client posthog.Client

	baseProps = posthog.NewProperties().
			Set("GOOS", runtime.GOOS).
			Set("GOARCH", runtime.GOARCH).
			Set("TERM", os.Getenv("TERM")).
			Set("SHELL", filepath.Base(os.Getenv("SHELL"))).
			Set("Version", version.Version).
			Set("GoVersion", runtime.Version()).
			Set(nonInteractiveAttrName, false).
			Set(nonInteractiveNestedAttrName, false)
)

func SetNonInteractive(nonInteractive bool) {
	baseProps = baseProps.
		Set(nonInteractiveAttrName, nonInteractive).
		Set(nonInteractiveNestedAttrName, nonInteractive && os.Getenv("CRUSH") == "1")
}

func SetContinueBySessionID(continueBySessionID bool) {
	baseProps = baseProps.Set(continueSessionByIDAttrName, continueBySessionID)
}

func SetContinueLastSession(continueLastSession bool) {
	baseProps = baseProps.Set(continueLastSessionAttrName, continueLastSession)
}

// One goroutine sets the client up, which is slow (ioreg and sw_vers), then
// uploads queued events, so no caller ever waits on telemetry.
var (
	started atomic.Bool
	queue   = make(chan item, 256)
	ready   = make(chan struct{}) // Closed once client and distinctId are set.
)

// item is either an event to upload or a request to flush.
type item struct {
	build func(distinctID string) posthog.Message
	flush *flushRequest
}

type flushRequest struct {
	deadline time.Time
	done     chan struct{}
}

// Init starts the PostHog client in the background. Only the first call has
// any effect.
func Init() {
	if started.CompareAndSwap(false, true) {
		go run()
	}
}

func run() {
	c, err := posthog.NewWithConfig(key, posthog.Config{
		Endpoint:        endpoint,
		Logger:          logger{},
		ShutdownTimeout: flushTimeout,
	})
	if err != nil {
		slog.Error("Failed to initialize PostHog client", "error", err)
	}
	client, distinctId = c, getDistinctId()
	close(ready)

	for it := range queue {
		switch {
		case it.flush != nil:
			flushAndClose(it.flush.deadline)
			close(it.flush.done)
		case client != nil:
			if err := client.Enqueue(it.build(distinctId)); err != nil {
				slog.Error("Failed to enqueue PostHog event", "error", err)
			}
		}
	}
}

// flushAndClose uploads what is queued and closes the client. FlushWithContext
// returns as soon as the upload lands, so Close finds nothing left to wait on.
func flushAndClose(deadline time.Time) {
	if client == nil {
		return
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	if err := client.FlushWithContext(ctx); err != nil {
		slog.Warn("Failed to flush PostHog events", "error", err)
	}
	if err := client.Close(); err != nil {
		slog.Warn("Failed to close the PostHog client", "error", err)
	}
	client = nil
}

// enqueue hands an event to the uploader, dropping it when metrics are off or
// the queue is full.
func enqueue(build func(distinctID string) posthog.Message) {
	if !started.Load() {
		return
	}
	select {
	case queue <- item{build: build}:
	default:
		slog.Warn("Dropping a telemetry event: the queue is full")
	}
}

// idWait bounds how long GetID waits for Init to compute the ID.
const idWait = 2 * time.Second

// GetID returns the machine's anonymous ID, or "" if it is not ready in time.
func GetID() string {
	if !started.Load() {
		return ""
	}
	select {
	case <-ready:
		return distinctId
	case <-time.After(idWait):
		return ""
	}
}

// send logs an event to PostHog with the given event name and properties.
func send(event string, props ...any) {
	properties := pairsToProps(props...).Merge(baseProps)
	enqueue(func(id string) posthog.Message {
		return posthog.Capture{DistinctId: id, Event: event, Properties: properties}
	})
}

// Error logs an error event to PostHog with the error type and message.
func Error(errToLog any, props ...any) {
	if errToLog == nil {
		return
	}
	at := time.Now()
	errType, msg := reflect.TypeOf(errToLog).String(), fmt.Sprintf("%v", errToLog)
	properties := pairsToProps(props...)
	enqueue(func(id string) posthog.Message {
		exception := posthog.NewDefaultException(at, id, errType, msg)
		if exception.Properties == nil {
			exception.Properties = posthog.NewProperties()
		}
		exception.Properties = exception.Properties.Merge(properties)
		return exception
	})
}

// flushTimeout bounds how long exiting waits on telemetry, setup included.
const flushTimeout = 500 * time.Millisecond

// Flush uploads every queued event and closes the client, giving up after
// flushTimeout.
func Flush() {
	if !started.Load() {
		return
	}
	deadline := time.Now().Add(flushTimeout)
	timeout := time.NewTimer(flushTimeout)
	defer timeout.Stop()
	done := make(chan struct{})
	select {
	case queue <- item{flush: &flushRequest{deadline: deadline, done: done}}:
	case <-timeout.C:
		slog.Warn("Gave up flushing telemetry")
		return
	}
	select {
	case <-done:
	case <-timeout.C:
		slog.Warn("Gave up flushing telemetry")
	}
}

func pairsToProps(props ...any) posthog.Properties {
	p := posthog.NewProperties()

	if !isEven(len(props)) {
		slog.Error("Event properties must be provided as key-value pairs", "props", props)
		return p
	}

	for i := 0; i < len(props); i += 2 {
		key, ok := props[i].(string)
		if !ok {
			slog.Error("Event property key must be a string", "key", props[i], "index", i)
			continue
		}
		value := props[i+1]
		p = p.Set(key, value)
	}
	return p
}

func isEven(n int) bool {
	return n%2 == 0
}
