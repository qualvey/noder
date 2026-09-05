package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

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

	downloadParam := r.URL.Query().Get("download")
	rawParam := r.URL.Query().Get("raw")
	isDirectDownload := downloadParam == "1" || downloadParam == "true" || rawParam == "1" || rawParam == "true"

	// 针对 ZIP 且由具体用户打开场景，若非直接下载，返回 Loading 引导页
	if matchedUser != nil && dist.FileType == "zip" && !isDirectDownload {
		html := loading.RenderLoadingPage(&dist, matchedUser)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
		return
	}

	data, err := file.ReadFileContent(&dist)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "读取文件失败: "+err.Error())
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
