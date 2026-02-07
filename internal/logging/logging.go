package logging

import (
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
)

// Init configures the default slog logger for the given service.
// Format is determined by LOG_FORMAT env var: "text" for human-readable,
// "json" (default) for structured JSON output.
// Level is determined by LOG_LEVEL env var: "debug", "info" (default), "warn", "error".
func Init(service string, w io.Writer) *slog.Logger {
	if w == nil {
		w = os.Stderr
	}
	level := parseLevel(os.Getenv("LOG_LEVEL"))
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if strings.EqualFold(os.Getenv("LOG_FORMAT"), "text") {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}

	logger := slog.New(handler).With(slog.String("service", service))
	slog.SetDefault(logger)

	// Redirect stdlib log to slog so any transitive log.Printf calls
	// still produce structured output.
	log.SetFlags(0)
	log.SetOutput(&slogWriter{logger: logger})

	return logger
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// slogWriter adapts slog.Logger to io.Writer for stdlib log redirection.
type slogWriter struct {
	logger *slog.Logger
}

func (w *slogWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\n")
	w.logger.Info(msg, slog.String("source", "stdlib"))
	return len(p), nil
}
