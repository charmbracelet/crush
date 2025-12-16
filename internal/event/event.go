package event

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"

	"github.com/charmbracelet/crush/internal/version"
	"github.com/posthog/posthog-go"
)

const (
	endpoint = "https://data.charm.land"
	key      = "phc_4zt4VgDWLqbYnJYEwLRxFoaTL2noNrQij0C6E8k3I0V"
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
			Set("Interactive", false)
)

func SetInteractive(interactive bool) {
	baseProps = baseProps.Set("interactive", interactive)
}

func Init() {
	c, err := posthog.NewWithConfig(key, posthog.Config{
		Endpoint: endpoint,
		Logger:   logger{},
	})
	if err != nil {
		slog.Error("Failed to initialize PostHog client", "error", err)
	}
	client = c
	distinctId = getDistinctId()
}

func GetID() string { return distinctId }

func Alias(userID string) {
	if client == nil || distinctId == fallbackId || distinctId == "" || userID == "" {
		return
	}
	if err := client.Enqueue(posthog.Alias{
		DistinctId: distinctId,
		Alias:      userID,
	}); err != nil {
		slog.Error("Failed to enqueue PostHog alias event", "error", err)
		return
	}
	slog.Info("Aliased in PostHog", "machine_id", distinctId, "user_id", userID)
}

// send logs an event to PostHog with the given event name and properties.
func send(event string, props ...any) {
	if client == nil {
		return
	}
	err := client.Enqueue(posthog.Capture{
		DistinctId: distinctId,
		Event:      event,
		Properties: pairsToProps(props...).Merge(baseProps),
	})
	if err != nil {
		slog.Error("Failed to enqueue PostHog event", "event", event, "props", props, "error", err)
		return
	}
}

// Error logs an error event to PostHog with the error type and message.
func Error(err any, props ...any) {
	if client == nil {
		return
	}
	// The PostHog Go client does not yet support sending exceptions.
	// We're mimicking the behavior by sending the minimal info required
	// for PostHog to recognize this as an exception event.
	props = append(
		[]any{
			"$exception_list",
			[]map[string]string{
				{"type": reflect.TypeOf(err).String(), "value": fmt.Sprintf("%v", err)},
			},
		},
		props...,
	)
	send("$exception", props...)
}

func Flush() {
	if client == nil {
		return
	}
	if err := client.Close(); err != nil {
		slog.Error("Failed to flush PostHog events", "error", err)
	}
}

func pairsToProps(props ...any) posthog.Properties {
	p := posthog.NewProperties()

	if !isEven(len(props)) {
		slog.Error("Event properties must be provided as key-value pairs", "props", props)
		return p
	}

	for i := 0; i < len(props); i += 2 {
		key := props[i].(string)
		value := props[i+1]
		p = p.Set(key, value)
	}
	return p
}

func isEven(n int) bool {
	return n%2 == 0
}
