package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
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

	handlerOptions := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			switch attr.Key {
			case slog.TimeKey:
				return slog.String("timestamp", attr.Value.Time().UTC().Format(time.RFC3339))
			case slog.LevelKey:
				return slog.String("level", strings.ToLower(attr.Value.Any().(slog.Level).String()))
			case slog.MessageKey:
				return slog.String("msg", attr.Value.String())
			default:
				return attr
			}
		},
	}
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
