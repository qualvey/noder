package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// ANSI color codes for terminal formatting
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

// PrettyHandler formats slog records for human readability in development/console.
type PrettyHandler struct {
	opts  slog.HandlerOptions
	out   io.Writer
	mu    *sync.Mutex
	attrs []slog.Attr
}

func NewPrettyHandler(out io.Writer, opts *slog.HandlerOptions) *PrettyHandler {
	if out == nil {
		out = os.Stdout
	}
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &PrettyHandler{
		opts: *opts,
		out:  out,
		mu:   &sync.Mutex{},
	}
}

func (h *PrettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *PrettyHandler) Handle(_ context.Context, r slog.Record) error {
	levelStr := h.formatLevel(r.Level)
	timeStr := r.Time.Format("2006-01-02 15:04:05.000")

	var b strings.Builder
	b.WriteString(colorGray)
	b.WriteString(timeStr)
	b.WriteString(colorReset)
	b.WriteString(" ")
	b.WriteString(levelStr)
	b.WriteString(" ")
	b.WriteString(r.Message)

	// Append handler-level attributes
	for _, a := range h.attrs {
		b.WriteString(" ")
		h.appendAttr(&b, a)
	}

	// Append record-level attributes
	r.Attrs(func(a slog.Attr) bool {
		b.WriteString(" ")
		h.appendAttr(&b, a)
		return true
	})

	b.WriteString("\n")

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.out.Write([]byte(b.String()))
	return err
}

func (h *PrettyHandler) formatLevel(level slog.Level) string {
	switch {
	case level < slog.LevelInfo:
		return colorCyan + "[DEBUG]" + colorReset
	case level < slog.LevelWarn:
		return colorGreen + "[INFO] " + colorReset
	case level < slog.LevelError:
		return colorYellow + "[WARN] " + colorReset
	default:
		return colorRed + "[ERROR]" + colorReset
	}
}

func (h *PrettyHandler) appendAttr(b *strings.Builder, a slog.Attr) {
	b.WriteString(colorCyan)
	b.WriteString(a.Key)
	b.WriteString(colorReset)
	b.WriteString("=")
	b.WriteString(fmt.Sprintf("%v", a.Value.Any()))
}

func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &PrettyHandler{
		opts:  h.opts,
		out:   h.out,
		mu:    h.mu,
		attrs: newAttrs,
	}
}

func (h *PrettyHandler) WithGroup(_ string) slog.Handler {
	// Simple flat grouping for pretty terminal output
	return h
}
