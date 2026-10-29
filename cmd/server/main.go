package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"noder"
	"noder/internal/config"
	"noder/internal/db"
	"noder/internal/handler"
	"noder/internal/logging"
)

var Version = "dev"

func main() {
	if err := config.Load(""); err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}
	if err := logging.Configure(config.LogLevel, config.LogFormat, config.LogOutput); err != nil {
		fmt.Fprintf(os.Stderr, "logging configuration error: %v\n", err)
		os.Exit(1)
	}
	logger := logging.L()

	// 初始化数据库及迁移
	_, err := db.InitDB()
	if err != nil {
		logger.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}

	r := chi.NewRouter()

	// 基础中间件
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(logging.RequestLogger(logger))
	r.Use(logging.Recoverer(logger))

	// CORS 跨域配置
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Admin-Token", "X-Service-Token"},
		ExposedHeaders:   []string{"Link", "Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// 注册业务 API 路由
	handler.RegisterAuthRoutes(r)
	handler.RegisterNodeRoutes(r)
	handler.RegisterUserRoutes(r)
	handler.RegisterTemplateRoutes(r)
	handler.RegisterSettingsRoutes(r)
	handler.RegisterFileRoutes(r)
	handler.RegisterDownloadRoutes(r)
	handler.RegisterSubscriptionRoutes(r)
	handler.RegisterServiceRoutes(r)

	// 挂载 SPA 前端静态资源
	mountSPA(r)

	server := &http.Server{
		Addr:    net.JoinHostPort(config.ServerHost, fmt.Sprint(config.ServerPort)),
		Handler: r,
	}

	// 优雅停机监听
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("server listening",
			"version", Version,
			"address", server.Addr,
			"config", config.ConfigPath,
			"log_level", config.LogLevel,
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-stop
	logger.Info("shutting down server gracefully")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
	}

	logger.Info("server exited successfully")
}

func mountSPA(r *chi.Mux) {
	subFS, err := fs.Sub(noder.StaticFS, "static")
	if err != nil {
		logging.L().Warn("failed to load embedded static filesystem", "error", err)
		return
	}

	fileServer := http.FileServer(http.FS(subFS))

	// 处理所有其它路由回退到 SPA index.html
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path

		// 如果是 API 或下载/订阅前缀且 404，返回标准 JSON 404
		if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/dl") ||
			path == "/sub" || path == "/node" || path == "/mihomo" || path == "/xray" {
			handler.RespondError(w, http.StatusNotFound, fmt.Sprintf("Path '%s' not found", path))
			return
		}

		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath != "" {
			if f, err := subFS.Open(cleanPath); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, req)
				return
			}
		}

		// Fallback index.html
		req.URL.Path = "/"
		fileServer.ServeHTTP(w, req)
	})
}
