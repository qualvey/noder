// Package logger 提供统一的高性能结构化日志抽象，基于 Go 标准库 log/slog，
// 支持终端美化彩色输出（Pretty）、标准 JSON 输出、全链路 RequestID Context 贯通与启动自检看板。
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Level 定义系统日志级别枚举。
type Level int

const (
	LevelTrace Level = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelTrace:
		return "TRACE"
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

func (l Level) SlogLevel() slog.Level {
	switch l {
	case LevelTrace, LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ParseLevel 将字符串转换为日志级别枚举。
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return LevelTrace
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error", "fatal", "panic":
		return LevelError
	case "info", "":
		return LevelInfo
	default:
		return LevelInfo
	}
}

var (
	mu            sync.RWMutex
	defaultLogger *slog.Logger
	currentLevel  Level          = LevelInfo
	slogLevelVar  *slog.LevelVar = new(slog.LevelVar)
	currentFormat string         = "pretty"
	defaultWriter io.Writer      = os.Stdout
	currentWriter io.Writer      = os.Stdout
	logFileHandle *os.File
)

func init() {
	slogLevelVar.Set(slog.LevelInfo)
	Init("", "", "")
}

// Init 初始化全局日志实例。
// 支持 Noder 风格签名：Init(env, levelStr, format string)
// 以及 Mall 历史兼容签名：Init(levelStr, output, outfile string)
func Init(args ...string) *slog.Logger {
	mu.Lock()
	defer mu.Unlock()

	var a1, a2, a3 string
	if len(args) > 0 {
		a1 = args[0]
	}
	if len(args) > 1 {
		a2 = args[1]
	}
	if len(args) > 2 {
		a3 = args[2]
	}

	// 清理旧的文件句柄
	if logFileHandle != nil {
		_ = logFileHandle.Close()
		logFileHandle = nil
	}

	var env, levelStr, format string
	var writers []io.Writer

	a2Lower := strings.ToLower(strings.TrimSpace(a2))
	isMallOutput := a2Lower == "stdout" || a2Lower == "stderr" || a2Lower == "file" || strings.HasSuffix(strings.ToLower(strings.TrimSpace(a3)), ".log")

	if isMallOutput {
		levelStr = a1
		switch a2Lower {
		case "stdout":
			writers = append(writers, os.Stdout)
		case "file":
			// 仅输出至文件
		default:
			writers = append(writers, os.Stderr)
		}

		if a3 != "" {
			if err := os.MkdirAll(filepath.Dir(a3), 0755); err == nil {
				if f, err := os.OpenFile(a3, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
					logFileHandle = f
					writers = append(writers, f)
				}
			}
		}

		if len(writers) == 0 {
			writers = append(writers, os.Stdout)
		}
		if len(writers) == 1 {
			currentWriter = writers[0]
		} else {
			currentWriter = io.MultiWriter(writers...)
		}

		env = os.Getenv("APP_ENV")
		if env == "" {
			env = "development"
		}
		format = os.Getenv("LOG_FORMAT")
		if format == "" {
			if env == "production" {
				format = "json"
			} else {
				format = "pretty"
			}
		}
	} else {
		env = a1
		levelStr = a2
		format = a3

		if env == "" {
			env = os.Getenv("APP_ENV")
			if env == "" {
				env = "development"
			}
		}
		if levelStr == "" {
			levelStr = os.Getenv("LOG_LEVEL")
		}
		if format == "" {
			format = os.Getenv("LOG_FORMAT")
			if format == "" {
				if env == "production" {
					format = "json"
				} else {
					format = "pretty"
				}
			}
		}
		currentWriter = os.Stdout
	}

	if levelStr == "" {
		levelStr = "INFO"
	}

	parsedLevel := ParseLevel(levelStr)
	currentLevel = parsedLevel
	slogLevelVar.Set(parsedLevel.SlogLevel())
	currentFormat = format

	rebuildLoggerLocked()
	return defaultLogger
}

func rebuildLoggerLocked() {
	opts := &slog.HandlerOptions{
		Level:     slogLevelVar,
		AddSource: os.Getenv("LOG_SOURCE") == "true",
	}

	out := currentWriter
	if out == nil {
		out = os.Stdout
	}

	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(currentFormat)) {
	case "json":
		handler = slog.NewJSONHandler(out, opts)
	case "raw", "plain":
		handler = slog.NewTextHandler(out, opts)
	default:
		handler = NewPrettyHandler(out, opts)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

// Get 返回当前全局 Logger。
func Get() *slog.Logger {
	mu.RLock()
	l := defaultLogger
	mu.RUnlock()
	if l == nil {
		return slog.Default()
	}
	return l
}

// SetFormat 动态切换日志输出格式 ("pretty", "json", "text")。
func SetFormat(format string) {
	mu.Lock()
	defer mu.Unlock()
	currentFormat = format
	rebuildLoggerLocked()
}

// SetLevel 动态调整日志级别（线程安全，底层基于 slog.LevelVar）。
func SetLevel(levelText string) {
	mu.Lock()
	defer mu.Unlock()

	parsed := ParseLevel(levelText)
	currentLevel = parsed
	slogLevelVar.Set(parsed.SlogLevel())
}

// GetLevel 获取当前日志级别。
func GetLevel() Level {
	mu.RLock()
	defer mu.RUnlock()
	return currentLevel
}

// IsDebug 检查当前是否开启了 Debug 或更低（Trace）日志级别。
func IsDebug() bool {
	mu.RLock()
	defer mu.RUnlock()
	return currentLevel <= LevelDebug
}

// SetOutput 重定向日志输出目的地。
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	currentWriter = w
	rebuildLoggerLocked()
}

// ResetOutput 还原输出目的地为默认输出。
func ResetOutput() {
	mu.Lock()
	defer mu.Unlock()
	if logFileHandle != nil {
		_ = logFileHandle.Close()
		logFileHandle = nil
	}
	currentWriter = defaultWriter
	rebuildLoggerLocked()
}

func isFormatString(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+1 < len(s) {
			if s[i+1] == '%' {
				i++
				continue
			}
			for j := i + 1; j < len(s) && j < i+10; j++ {
				switch s[j] {
				case 'v', 'T', 't', 'b', 'c', 'd', 'o', 'q', 'x', 'X', 'U', 'e', 'E', 'f', 'F', 'g', 'G', 's', 'p', 'w':
					return true
				case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.', '+', '-', ' ', '#':
					continue
				default:
					break
				}
				break
			}
		}
	}
	return false
}

func formatMsg(format string, a []any) string {
	return fmt.Sprintf(format, a...)
}

func prepareLogMsgAndArgs(msg string, args ...any) (string, []any) {
	if len(args) == 0 {
		return msg, nil
	}
	if isFormatString(msg) {
		return formatMsg(msg, args), nil
	}
	return msg, args
}

// 包级别日志便捷封装，支持 printf 格式化及结构化属性 (Key/Value pairs)
func Debug(msg string, args ...any) {
	m, a := prepareLogMsgAndArgs(msg, args...)
	Get().Debug(m, a...)
}

func Info(msg string, args ...any) {
	m, a := prepareLogMsgAndArgs(msg, args...)
	Get().Info(m, a...)
}

func Warn(msg string, args ...any) {
	m, a := prepareLogMsgAndArgs(msg, args...)
	Get().Warn(m, a...)
}

func Error(msg string, args ...any) {
	m, a := prepareLogMsgAndArgs(msg, args...)
	Get().Error(m, a...)
}

func Fatal(msg string, args ...any) {
	m, a := prepareLogMsgAndArgs(msg, args...)
	Get().Error(m, a...)
	os.Exit(1)
}

func Fatalf(format string, args ...any) {
	Fatal(format, args...)
}

func Trace(msg string, args ...any) {
	Debug(msg, args...)
}

func DebugContext(ctx context.Context, msg string, args ...any) {
	m, a := prepareLogMsgAndArgs(msg, args...)
	FromContext(ctx).DebugContext(ctx, m, a...)
}

func InfoContext(ctx context.Context, msg string, args ...any) {
	m, a := prepareLogMsgAndArgs(msg, args...)
	FromContext(ctx).InfoContext(ctx, m, a...)
}

func WarnContext(ctx context.Context, msg string, args ...any) {
	m, a := prepareLogMsgAndArgs(msg, args...)
	FromContext(ctx).WarnContext(ctx, m, a...)
}

func ErrorContext(ctx context.Context, msg string, args ...any) {
	m, a := prepareLogMsgAndArgs(msg, args...)
	FromContext(ctx).ErrorContext(ctx, m, a...)
}

// BootInfo holds metadata displayed in the startup banner.
type BootInfo struct {
	AppName     string
	Version     string
	Environment string
	GoVersion   string
	PID         int
	Port        string
	DataDir     string
	DBDriver    string
	DBDSN       string
	MallSecret  string // will be masked
	JWTSecret   string // will be masked
	Routers     []string
	AgentCount  int
}

// PrintBootSummary prints the standardized ASCII startup dashboard.
func PrintBootSummary(b BootInfo) {
	title := b.AppName
	if title == "" {
		if strings.Contains(strings.ToLower(b.DataDir), "noder") || b.AgentCount > 0 {
			title = fmt.Sprintf("Noder Control Plane (Go Edition %s)", b.Version)
		} else {
			title = fmt.Sprintf("Mall API Server (Go Edition %s)", b.Version)
		}
	} else if b.Version != "" && !strings.Contains(title, b.Version) {
		title = fmt.Sprintf("%s (Go Edition %s)", title, b.Version)
	}

	mallStatus := "Not Configured"
	if b.MallSecret != "" {
		mallStatus = fmt.Sprintf("Configured (len=%d)", len(b.MallSecret))
	}
	jwtStatus := "Not Configured"
	if b.JWTSecret != "" {
		jwtStatus = fmt.Sprintf("Configured (len=%d)", len(b.JWTSecret))
	}

	dbDSN := b.DBDSN
	if dbDSN == "" {
		dbDSN = "in-memory / default"
	}

	routers := strings.Join(b.Routers, ", ")
	if routers == "" {
		routers = "Default"
	}

	border := "================================================================================"
	fmt.Println(colorCyan + border + colorReset)
	fmt.Printf("  %s\n", title)
	fmt.Println(colorCyan + border + colorReset)
	fmt.Printf("  %-12s Profile: %s | Go: %s | PID: %d\n", "[ENV]", b.Environment, b.GoVersion, b.PID)
	fmt.Printf("  %-12s DataDir: %s | Listen: :%s\n", "[CONFIG]", b.DataDir, b.Port)
	fmt.Printf("  %-12s Driver: %s | DSN: %s\n", "[DATABASE]", b.DBDriver, dbDSN)
	if b.MallSecret != "" || b.JWTSecret != "" {
		fmt.Printf("  %-12s Mall Secret: %s | JWT: %s\n", "[SECURITY]", mallStatus, jwtStatus)
	}
	fmt.Printf("  %-12s %s\n", "[ROUTERS]", routers)
	if b.AgentCount > 0 {
		fmt.Printf("  %-12s Active Agents/Endpoints: %d | Heartbeat Timeout: 90s\n", "[AGENTS]", b.AgentCount)
	}
	fmt.Println(colorCyan + border + colorReset)
}
