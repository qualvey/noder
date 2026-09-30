package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"noder/internal/db"
	"noder/internal/model"
	"noder/internal/service/file"
	"noder/internal/service/loading"
	"noder/internal/service/template"
)

func RegisterDownloadRoutes(r chi.Router) {
	r.Route("/dl", func(r chi.Router) {
		r.Get("/{id}", HandleDownload)

		// 共享 Token 管理
		r.Group(func(r chi.Router) {
			r.Use(AdminAuth)
			r.Get("/token", GetSharedToken)
			r.Post("/token/reset", ResetSharedToken)
		})
	})
}

func getSharedTokenFromDB(ctx context.Context) string {
	var s model.AppSetting
	if err := db.DB.NewSelect().Model(&s).Where("key = ?", "shared_download_token").Scan(ctx); err == nil {
		return s.Value
	}
	return ""
}

func GetSharedToken(w http.ResponseWriter, r *http.Request) {
	tok := getSharedTokenFromDB(r.Context())
	RespondJSON(w, http.StatusOK, map[string]string{"token": tok})
}

func ResetSharedToken(w http.ResponseWriter, r *http.Request) {
	newToken := randomHex(16)
	_, _ = db.DB.NewInsert().
		Model(&model.AppSetting{Key: "shared_download_token", Value: newToken}).
		On("CONFLICT (key) DO UPDATE").
		Set("value = EXCLUDED.value").
		Exec(r.Context())

	RespondJSON(w, http.StatusOK, map[string]string{"token": newToken})
}

func HandleDownload(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		RespondError(w, http.StatusUnauthorized, "缺少鉴权 Token (请使用 ?token=xxx)")
		return
	}

	var dist model.DistFile
	if err := db.DB.NewSelect().Model(&dist).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusNotFound, "文件不存在或已被删除")
		return
	}

	if !dist.IsActive {
		RespondError(w, http.StatusNotFound, "文件不存在或已被停用")
		return
	}

	sharedTok := getSharedTokenFromDB(r.Context())
	isSharedAuth := sharedTok != "" && token == sharedTok

	var matchedUser *model.User
	if !isSharedAuth {
		var u model.User
		err := db.DB.NewSelect().Model(&u).Relation("Nodes").Where("user.token = ?", token).Scan(r.Context())
		if err == nil && u.IsActive {
			matchedUser = &u
		}
	}

	if !isSharedAuth && matchedUser == nil {
		RespondError(w, http.StatusUnauthorized, "无效或已过期的下载凭证")
		return
	}
	if r.URL.Query().Get("progress_stream") == "1" {
		streamDownloadProgress(w, r, progressIDFromRequest(r))
		return
	}

	downloadParam := r.URL.Query().Get("download")
	rawParam := r.URL.Query().Get("raw")
	isDirectDownload := downloadParam == "1" || downloadParam == "true" || rawParam == "1" || rawParam == "true"
	if matchedUser != nil && dist.FileType == "zip" && !isDirectDownload {
		html := loading.RenderLoadingPage(&dist, matchedUser)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
		return
	}
	progressID := progressIDFromRequest(r)
	if progressID != "" {
		publishDownloadProgress(progressID, 3, "准备下载文件", false)
	}

	// 远程模式：缓存过期则自动刷新，失败时保留旧缓存继续服务。
	// Keep this after returning the loading page so the original URL remains responsive.
	if dist.SourceURL != nil && *dist.SourceURL != "" && file.IsRemoteCacheExpired(&dist) {
		if progressID != "" {
			publishDownloadProgress(progressID, 5, "检查远程文件缓存", false)
		}
		if content, err := file.FetchRemoteFile(*dist.SourceURL); err == nil {
			_ = file.SaveFileContent(&dist, content)
			dist.Size = int64(len(content))
			now := time.Now().Format("2006-01-02 15:04:05")
			dist.CachedAt = &now
			_, _ = db.DB.NewUpdate().Model(&dist).Where("id = ?", dist.ID).Exec(r.Context())
		}
	}

	data, err := file.ReadFileContent(&dist)
	if err != nil {
		if progressID != "" {
			publishDownloadProgress(progressID, 5, "读取源文件失败", true)
		}
		RespondError(w, http.StatusInternalServerError, "读取文件失败: "+err.Error())
		return
	}

	outFileName := dist.OriginalName
	if dist.DownloadName != nil && *dist.DownloadName != "" {
		outFileName = *dist.DownloadName
	}

	// 用户下载 ZIP：进行模板渲染
	if matchedUser != nil && dist.FileType == "zip" {
		if progressID != "" {
			publishDownloadProgress(progressID, 10, "准备模板与用户数据", false)
		}
		var allUsers []*model.User
		_ = db.DB.NewSelect().Model(&allUsers).Column("token").Scan(r.Context())
		var knownTokens []string
		for _, au := range allUsers {
			if au.Token != "" {
				knownTokens = append(knownTokens, au.Token)
			}
		}

		renderedZip, err := template.RenderZipForUserWithProgress(data, dist.TemplateName, matchedUser, matchedUser.Nodes, knownTokens, func(processed, total uint64) {
			if progressID == "" {
				return
			}
			percent := 10
			if total > 0 {
				percent += int(float64(processed) / float64(total) * 80)
			}
			publishDownloadProgress(progressID, percent, fmt.Sprintf("正在处理压缩包文件（%d/%d）", processed, total), false)
		})
		if err != nil {
			if progressID != "" {
				publishDownloadProgress(progressID, 10, "配置包生成失败", true)
			}
			RespondError(w, http.StatusInternalServerError, "渲染个性化 ZIP 失败: "+err.Error())
			return
		}
		data = renderedZip
		if progressID != "" {
			publishDownloadProgress(progressID, 95, "配置包已生成，启动下载", false)
		}
	}
	contentType := "application/octet-stream"
	switch dist.FileType {
	case "apk":
		contentType = "application/vnd.android.package-archive"
	case "zip":
		contentType = "application/zip"
	case "text":
		contentType = "text/plain; charset=utf-8"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	if progressID != "" {
		// Allow reverse proxies such as nginx to forward ZIP chunks as they arrive.
		w.Header().Set("X-Accel-Buffering", "no")
	}

	// 对普通下载添加 Content-Disposition
	escapedName := url.PathEscape(outFileName)
	cd := fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, outFileName, escapedName)
	w.Header().Set("Content-Disposition", cd)

	w.WriteHeader(http.StatusOK)
	written := 0
	var writeErr error
	if progressID != "" && len(data) > 0 {
		written, writeErr = w.Write(data[:1])
		if writeErr == nil && written == 1 {
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			publishDownloadProgress(progressID, 100, "浏览器下载已启动", false)
		}
	}
	if writeErr == nil && written < len(data) {
		var n int
		n, writeErr = w.Write(data[written:])
		written += n
	}
	if progressID != "" {
		if writeErr != nil || written != len(data) {
			publishDownloadProgress(progressID, 100, "下载传输中断", true)
		} else {
			publishDownloadProgress(progressID, 100, "浏览器下载已启动", true)
		}
	}
	if matchedUser != nil && writeErr == nil && written == len(data) {
		_, logErr := db.DB.NewInsert().Model(&model.UserDownloadLog{
			UserID:       matchedUser.ID,
			UserName:     matchedUser.Name,
			FileID:       dist.ID,
			FileName:     outFileName,
			DownloadedAt: time.Now().UTC(),
		}).Exec(r.Context())
		if logErr != nil {
			log.Printf("failed to record download for user %d and file %d: %v", matchedUser.ID, dist.ID, logErr)
		}
	}
}
