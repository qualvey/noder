package logger

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (w *responseWriterInterceptor) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriterInterceptor) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += int64(n)
	return n, err
}

func generateRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "req_" + hex.EncodeToString(b)
}

// HTTPMiddleware creates a structured access logger middleware for Chi / standard net/http.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = generateRequestID()
		}
		w.Header().Set("X-Request-ID", reqID)

		// Create contextual logger with request_id
		reqLogger := Get().With("req_id", reqID)
		ctx := WithContext(r.Context(), reqLogger)

		interceptor := &responseWriterInterceptor{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		start := time.Now()
		next.ServeHTTP(interceptor, r.WithContext(ctx))
		duration := time.Since(start)

		status := interceptor.statusCode
		durMs := float64(duration.Microseconds()) / 1000.0

		// Don't clutter logs with static asset fetches unless there's an error
		if isStaticAsset(r.URL.Path) && status < 400 {
			return
		}

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"dur_ms", durMs,
			"ip", clientIP(r),
			"bytes", interceptor.bytesWritten,
		}

		msg := "HTTP request completed"
		switch {
		case status >= 500:
			reqLogger.ErrorContext(ctx, msg, attrs...)
		case status >= 400:
			reqLogger.WarnContext(ctx, msg, attrs...)
		default:
			reqLogger.InfoContext(ctx, msg, attrs...)
		}
	})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}
	idx := strings.LastIndex(r.RemoteAddr, ":")
	if idx > 0 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}

func isStaticAsset(path string) bool {
	if strings.HasPrefix(path, "/static/") || strings.HasPrefix(path, "/assets/") ||
		strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".css") ||
		strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".svg") ||
		strings.HasSuffix(path, ".ico") {
		return true
	}
	return false
}
