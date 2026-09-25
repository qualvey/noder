package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
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
)

var Version = "dev"

func main() {
	config.Init("")

	// 初始化数据库及迁移
	_, err := db.InitDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	r := chi.NewRouter()

	// 基础中间件
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// CORS 跨域配置
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Admin-Token"},
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

	// 挂载 SPA 前端静态资源
	mountSPA(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// 优雅停机监听
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Noder (Go Edition %s) server listening on http://0.0.0.0:%s", Version, port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited successfully")
}

func mountSPA(r *chi.Mux) {
	subFS, err := fs.Sub(noder.StaticFS, "static")
	if err != nil {
		log.Printf("Warning: failed to load embedded static fs: %v", err)
		return
	}

	fileServer := http.FileServer(http.FS(subFS))

	// 处理所有其它路由回退到 SPA index.html
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path

		// 如果是 API 或下载/订阅前缀且 404，返回标准 JSON 404
		if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/dl") ||
			path == "/sub" || path == "/node" || path == "/mihomo" {
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
