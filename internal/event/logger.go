package event

import (
	"fmt"
	"log/slog"

	"github.com/posthog/posthog-go"
)

var _ posthog.Logger = logger{}

type logger struct{}

func (logger) Debugf(format string, args ...any) {
	slog.Debug(fmt.Sprintf(format, args...))
}

func (logger) Logf(format string, args ...any) {
	slog.Info(fmt.Sprintf(format, args...))
}

func (logger) Warnf(format string, args ...any) {
	slog.Warn(fmt.Sprintf(format, args...))
}

func (logger) Errorf(format string, args ...any) {
	slog.Error(fmt.Sprintf(format, args...))
}
