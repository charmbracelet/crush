package event

import (
	"log/slog"

	"github.com/posthog/posthog-go"
)

var _ posthog.Logger = logger{}

type logger struct{}

func (logger) Debugf(format string, args ...any) {
	slog.Debug(format, args...)
}

func (logger) Logf(format string, args ...any) {
	slog.Info(format, args...)
}

func (logger) Warnf(format string, args ...any) {
	slog.Warn(format, args...)
}

func (logger) Errorf(format string, args ...any) {
	slog.Error(format, args...)
}
