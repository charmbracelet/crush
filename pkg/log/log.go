package log

import (
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/charmbracelet/crush/pkg/config"

	"gopkg.in/natefinch/lumberjack.v2"
)

var initOnce sync.Once

func Init(cfg *config.Config) {
	initOnce.Do(func() {
		logRotator := &lumberjack.Logger{
			Filename:   filepath.Join(cfg.Options.DataDirectory, "logs", "crush.log"),
			MaxSize:    10,    // Max size in MB
			MaxBackups: 0,     // Number of backups
			MaxAge:     30,    // Days
			Compress:   false, // Enable compression
		}

		level := slog.LevelInfo
		if cfg.Options.Debug {
			level = slog.LevelDebug
		}

		logger := slog.NewJSONHandler(logRotator, &slog.HandlerOptions{
			Level:     level,
			AddSource: true,
		})

		slog.SetDefault(slog.New(logger))
	})
}
