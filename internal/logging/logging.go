package logging

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// Configure installs the process-wide structured logger. stderr is the default
// because it integrates naturally with systemd/journald.
func Configure(levelName, format, output string) error {
	var level slog.Level
	if err := level.UnmarshalText([]byte(levelName)); err != nil {
		return fmt.Errorf("invalid log level %q (expected debug, info, warn or error)", levelName)
	}

	var writer io.Writer
	switch output {
	case "", "stderr":
		writer = os.Stderr
	case "stdout":
		writer = os.Stdout
	default:
		return fmt.Errorf("invalid log output %q (expected stdout or stderr)", output)
	}

	options := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	switch format {
	case "", "text":
		handler = slog.NewTextHandler(writer, options)
	case "json":
		handler = slog.NewJSONHandler(writer, options)
	default:
		return fmt.Errorf("invalid log format %q (expected text or json)", format)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return nil
}

func L() *slog.Logger {
	return slog.Default()
}

type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytes += n
	return n, err
}

func (w *responseWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// RequestLogger records one structured event per request. Query strings are
// intentionally excluded because this application accepts tokens in queries.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			wrapped := &responseWriter{ResponseWriter: w}
			next.ServeHTTP(wrapped, r)

			status := wrapped.status
			if status == 0 {
				status = http.StatusOK
			}
			level := slog.LevelInfo
			if status >= http.StatusInternalServerError {
				level = slog.LevelError
			} else if status >= http.StatusBadRequest {
				level = slog.LevelWarn
			}
			logger.LogAttrs(r.Context(), level, "http request",
				slog.String("request_id", middleware.GetReqID(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Int("bytes", wrapped.bytes),
				slog.Int64("duration_ms", time.Since(started).Microseconds()/1000),
				slog.String("remote_ip", r.RemoteAddr),
			)
		})
	}
}

func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.ErrorContext(r.Context(), "http panic recovered",
						slog.Any("panic", recovered),
						slog.String("stack", string(debug.Stack())),
					)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
