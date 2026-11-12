package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]Level{
		"trace":   LevelTrace,
		"TRACE":   LevelTrace,
		"debug":   LevelDebug,
		"DEBUG":   LevelDebug,
		"info":    LevelInfo,
		"INFO":    LevelInfo,
		"":        LevelInfo,
		"warn":    LevelWarn,
		"warning": LevelWarn,
		"WARN":    LevelWarn,
		"error":   LevelError,
		"fatal":   LevelError,
		"panic":   LevelError,
		"unknown": LevelInfo,
	}

	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Fatalf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestInitHonorsLevel(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf)
	defer ResetOutput()

	SetLevel("warn")
	Debug("debug message")
	Info("info message")
	Warn("warn message")
	Error("error message")

	out := buf.String()
	if strings.Contains(out, "debug message") {
		t.Fatalf("debug message should be filtered out at warn level: %s", out)
	}
	if strings.Contains(out, "info message") {
		t.Fatalf("info message should be filtered out at warn level: %s", out)
	}
	if !strings.Contains(out, "warn message") || !strings.Contains(out, "error message") {
		t.Fatalf("warn and error messages should remain: %s", out)
	}
}

func TestDynamicLevelAndSlog(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf)
	defer ResetOutput()

	SetLevel("debug")
	if !IsDebug() {
		t.Fatalf("expected IsDebug to be true")
	}
	slog.Debug("slog debug message")
	Debug("logger debug message")

	out := buf.String()
	if !strings.Contains(out, "slog debug message") {
		t.Fatalf("slog debug should be output when level is debug: %s", out)
	}
	if !strings.Contains(out, "logger debug message") {
		t.Fatalf("logger debug should be output when level is debug: %s", out)
	}

	buf.Reset()
	SetLevel("info")
	if IsDebug() {
		t.Fatalf("expected IsDebug to be false")
	}
	slog.Debug("slog debug message 2")
	Debug("logger debug message 2")
	Info("logger info message 2")

	out = buf.String()
	if strings.Contains(out, "slog debug message 2") || strings.Contains(out, "logger debug message 2") {
		t.Fatalf("debug messages should be filtered at info level: %s", out)
	}
	if !strings.Contains(out, "logger info message 2") {
		t.Fatalf("info message should be present: %s", out)
	}
}

func TestConcurrentSetLevel(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if n%2 == 0 {
				SetLevel("debug")
				_ = IsDebug()
			} else {
				SetLevel("info")
				_ = GetLevel()
			}
		}(i)
	}
	wg.Wait()
}

func TestLoggerInitAndPrettyOutput(t *testing.T) {
	var buf bytes.Buffer
	handler := NewPrettyHandler(&buf, nil)
	l := handler.WithAttrs(nil)
	if l == nil {
		t.Fatal("handler.WithAttrs returned nil")
	}

	Init("development", "DEBUG", "pretty")
	Debug("test debug msg", "foo", "bar")

	// Test Context
	ctx := WithContext(context.Background(), Get())
	ctxL := FromContext(ctx)
	if ctxL == nil {
		t.Fatal("expected valid context logger")
	}
}

func TestLoggerJSONOutput(t *testing.T) {
	Init("production", "INFO", "json")

	ctx := WithAttrs(context.Background(), "user_id", 123)
	l := FromContext(ctx)
	if l == nil {
		t.Fatal("FromContext returned nil")
	}
}

func TestHTTPMiddleware(t *testing.T) {
	Init("development", "INFO", "pretty")

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			l := FromContext(r.Context())
			if l == nil {
				t.Error("logger not injected into context")
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mw := HTTPMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	respReqID := rec.Header().Get("X-Request-ID")
	if respReqID == "" || !strings.HasPrefix(respReqID, "req_") {
		t.Fatalf("expected generated req_ id in response header, got: %s", respReqID)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got: %d", rec.Code)
	}

	// Test custom X-Request-ID propagation
	customID := "custom-req-12345"
	req2 := httptest.NewRequest("POST", "/api/test", nil)
	req2.Header.Set("X-Request-ID", customID)
	rec2 := httptest.NewRecorder()
	mw.ServeHTTP(rec2, req2)

	if rec2.Header().Get("X-Request-ID") != customID {
		t.Fatalf("expected propagated %s, got %s", customID, rec2.Header().Get("X-Request-ID"))
	}
}

func TestPrintBootSummary(t *testing.T) {
	info := BootInfo{
		Version:     "v1.0.0-test",
		Environment: "testing",
		GoVersion:   "go1.27",
		PID:         1234,
		Port:        "8000",
		DataDir:     "/tmp/noder",
		DBDriver:    "SQLite",
		DBDSN:       "/tmp/noder/noder.db",
		MallSecret:  "secret-mall-key-12345",
		JWTSecret:   "jwt-key-xyz",
		Routers:     []string{"Auth", "AgentV1"},
		AgentCount:  2,
	}
	// Verify it does not panic
	PrintBootSummary(info)
}

func TestPrettyHandlerAttrs(t *testing.T) {
	var buf bytes.Buffer
	handler := NewPrettyHandler(&buf, nil)
	l := handler.WithAttrs(nil)
	if l == nil {
		t.Fatal("expected handler")
	}

	Init("production", "DEBUG", "json")
	Info("json test msg", "key1", "val1")

	m := map[string]interface{}{}
	jsonStr := `{"time":"2026-11-10T00:00:00Z","level":"INFO","msg":"test"}`
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		t.Fatal(err)
	}
}

func TestFormattedAndStructuredArgs(t *testing.T) {
	buf := &bytes.Buffer{}
	SetOutput(buf)
	defer ResetOutput()

	SetLevel("debug")
	SetFormat("text")

	// Printf style
	Info("hello %s, number %d", "world", 42)
	out := buf.String()
	if !strings.Contains(out, "hello world, number 42") {
		t.Fatalf("expected formatted output, got: %s", out)
	}

	// Slog structured style
	buf.Reset()
	Info("user login event", "user_id", "u_1001", "role", "admin")
	out = buf.String()
	if !strings.Contains(out, "user login event") || !strings.Contains(out, "user_id") || !strings.Contains(out, "u_1001") {
		t.Fatalf("expected structured output, got: %s", out)
	}
}
