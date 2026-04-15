package logging

import (
	"io"
	"log/slog"
	"os"
)

// Options controls the shared structured logging setup.
type Options struct {
	Level string
	JSON  bool
	Out   io.Writer
}

// New returns a configured slog logger for binaries and libraries.
func New(opts Options) *slog.Logger {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}

	level := new(slog.LevelVar)
	level.Set(parseLevel(opts.Level))

	handlerOptions := &slog.HandlerOptions{Level: level}
	if opts.JSON {
		return slog.New(slog.NewJSONHandler(opts.Out, handlerOptions))
	}
	return slog.New(slog.NewTextHandler(opts.Out, handlerOptions))
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
