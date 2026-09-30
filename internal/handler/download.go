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

	// 远程模式：缓存过期则自动刷新，失败时保留旧缓存继续服务
	if dist.SourceURL != nil && *dist.SourceURL != "" && file.IsRemoteCacheExpired(&dist) {
		if content, err := file.FetchRemoteFile(*dist.SourceURL); err == nil {
			_ = file.SaveFileContent(&dist, content)
			dist.Size = int64(len(content))
			now := time.Now().Format("2006-01-02 15:04:05")
			dist.CachedAt = &now
			_, _ = db.DB.NewUpdate().Model(&dist).Where("id = ?", dist.ID).Exec(r.Context())
		}
	}

	downloadParam := r.URL.Query().Get("download")
	rawParam := r.URL.Query().Get("raw")
	isDirectDownload := downloadParam == "1" || downloadParam == "true" || rawParam == "1" || rawParam == "true"

	data, err := file.ReadFileContent(&dist)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "读取文件失败: "+err.Error())
		return
	}

	// ZIP 个性化配置直接流式返回模板文本；只有显式下载时才重新生成 ZIP。
	if matchedUser != nil && dist.FileType == "zip" && !isDirectDownload && dist.TemplateName != nil && *dist.TemplateName != "" {
		var allUsers []*model.User
		_ = db.DB.NewSelect().Model(&allUsers).Column("token").Scan(r.Context())
		var knownTokens []string
		for _, au := range allUsers {
			if au.Token != "" {
				knownTokens = append(knownTokens, au.Token)
			}
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", "inline")
		w.WriteHeader(http.StatusOK)
		if err := template.RenderZipForUserTo(data, dist.TemplateName, matchedUser, matchedUser.Nodes, knownTokens, w); err != nil {
			log.Printf("渲染个性化模板失败: %v", err)
		}
		return
	}

	outFileName := dist.OriginalName
	if dist.DownloadName != nil && *dist.DownloadName != "" {
		outFileName = *dist.DownloadName
	}

	// 用户下载 ZIP：进行模板渲染
	if matchedUser != nil && dist.FileType == "zip" {
		var allUsers []*model.User
		_ = db.DB.NewSelect().Model(&allUsers).Column("token").Scan(r.Context())
		var knownTokens []string
		for _, au := range allUsers {
			if au.Token != "" {
				knownTokens = append(knownTokens, au.Token)
			}
		}

		renderedZip, err := template.RenderZipForUser(data, dist.TemplateName, matchedUser, matchedUser.Nodes, knownTokens)
		if err != nil {
			RespondError(w, http.StatusInternalServerError, "渲染个性化 ZIP 失败: "+err.Error())
			return
		}
		data = renderedZip
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

	// 对普通下载添加 Content-Disposition
	escapedName := url.PathEscape(outFileName)
	cd := fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, outFileName, escapedName)
	w.Header().Set("Content-Disposition", cd)

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
