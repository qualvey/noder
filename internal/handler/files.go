package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"noder/internal/config"
	"noder/internal/db"
	"noder/internal/model"
	"noder/internal/service/file"
)

func RegisterFileRoutes(r chi.Router) {
	r.Route("/api/files", func(r chi.Router) {
		r.Use(AdminAuth)
		r.Get("/", ListFiles)
		r.Post("/", UploadFile)
		r.Post("/upload", UploadFile)
		r.Post("/remote", CreateRemoteFile)
		r.Post("/{id}/refresh", RefreshRemoteFile)
		r.Post("/text", CreateTextFile)
		r.Get("/{id}/text", GetTextFileContent)
		r.Put("/{id}/text", UpdateTextFileContent)
		r.Put("/{id}", UpdateFile)
		r.Delete("/{id}", DeleteFile)
	})
}

func ListFiles(w http.ResponseWriter, r *http.Request) {
	var files []*model.DistFile
	if err := db.DB.NewSelect().Model(&files).Order("id DESC").Scan(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if files == nil {
		files = []*model.DistFile{}
	}
	RespondJSON(w, http.StatusOK, files)
}

func UploadFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(int64(config.MaxFileSize)); err != nil {
		RespondError(w, http.StatusBadRequest, "Failed to parse form: "+err.Error())
		return
	}

	mrFile, header, err := r.FormFile("file")
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Missing 'file' field")
		return
	}
	defer mrFile.Close()

	content, err := io.ReadAll(mrFile)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to read file: "+err.Error())
		return
	}

	originalName := header.Filename
	name := r.FormValue("name")
	if name == "" {
		name = originalName
	}
	fileType := r.FormValue("file_type")
	if fileType == "" {
		fileType = file.DetectFileType(originalName)
	}
	remark := r.FormValue("remark")
	templateName := r.FormValue("template_name")
	downloadName := r.FormValue("download_name")
	forceStr := r.FormValue("force")
	force := forceStr == "true" || forceStr == "1"

	sha := file.ComputeSHA256(content)

	// 重复检查
	var existing []*model.DistFile
	_ = db.DB.NewSelect().Model(&existing).Where("original_name = ?", originalName).Scan(r.Context())

	var dup *model.DistFile
	for _, ef := range existing {
		if data, err := file.ReadFileContent(ef); err == nil {
			if file.ComputeSHA256(data) == sha {
				dup = ef
				break
			}
		}
	}

	if dup != nil {
		if !force {
			RespondError(w, http.StatusConflict, "文件已存在 (SHA-256 相同)，如需覆盖请开启强制上传")
			return
		}
		// 覆盖现有文件
		dup.Size = int64(len(content))
		dup.Name = name
		if templateName != "" {
			dup.TemplateName = &templateName
		}
		if downloadName != "" {
			dup.DownloadName = &downloadName
		}
		if remark != "" {
			dup.Remark = &remark
		}
		_ = file.SaveFileContent(dup, content)
		_, _ = db.DB.NewUpdate().Model(dup).Where("id = ?", dup.ID).Exec(r.Context())
		RespondJSON(w, http.StatusOK, dup)
		return
	}

	dist := &model.DistFile{
		Name:         name,
		FileType:     fileType,
		OriginalName: originalName,
		StoredName:   file.GenerateStoredName(originalName),
		Size:         int64(len(content)),
		IsActive:     true,
		CreatedAt:    time.Now().Format("2006-01-02 15:04:05"),
	}
	if templateName != "" {
		dist.TemplateName = &templateName
	}
	if downloadName != "" {
		dist.DownloadName = &downloadName
	}
	if remark != "" {
		dist.Remark = &remark
	}

	if _, err := db.DB.NewInsert().Model(dist).Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := file.SaveFileContent(dist, content); err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to write file: "+err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, dist)
}

func CreateRemoteFile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL          string  `json:"url"`
		Name         *string `json:"name"`
		Remark       *string `json:"remark"`
		DownloadName *string `json:"download_name"`
		FileType     *string `json:"file_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if body.URL == "" {
		RespondError(w, http.StatusBadRequest, "Missing url")
		return
	}

	content, err := file.FetchRemoteFile(body.URL)
	if err != nil {
		HandleAPIError(w, err)
		return
	}

	origName := file.ExtractFileNameFromURL(body.URL)
	ft := file.DetectFileType(origName)
	if body.FileType != nil && *body.FileType != "" {
		ft = *body.FileType
	}
	name := origName
	if body.Name != nil && *body.Name != "" {
		name = *body.Name
	}

	cachedAt := time.Now().Format("2006-01-02 15:04:05")
	dist := &model.DistFile{
		Name:         name,
		FileType:     ft,
		OriginalName: origName,
		StoredName:   file.GenerateStoredName(origName),
		Size:         int64(len(content)),
		IsActive:     true,
		SourceURL:    &body.URL,
		CachedAt:     &cachedAt,
		Remark:       body.Remark,
		DownloadName: body.DownloadName,
		CreatedAt:    cachedAt,
	}

	if _, err := db.DB.NewInsert().Model(dist).Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_ = file.SaveFileContent(dist, content)
	RespondJSON(w, http.StatusOK, dist)
}

func RefreshRemoteFile(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	var dist model.DistFile
	if err := db.DB.NewSelect().Model(&dist).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusNotFound, "File not found")
		return
	}

	if dist.SourceURL == nil || *dist.SourceURL == "" {
		RespondError(w, http.StatusBadRequest, "非远程文件无法刷新缓存")
		return
	}

	content, err := file.FetchRemoteFile(*dist.SourceURL)
	if err != nil {
		HandleAPIError(w, err)
		return
	}

	dist.Size = int64(len(content))
	cachedAt := time.Now().Format("2006-01-02 15:04:05")
	dist.CachedAt = &cachedAt

	_ = file.SaveFileContent(&dist, content)
	_, _ = db.DB.NewUpdate().Model(&dist).Where("id = ?", id).Exec(r.Context())

	RespondJSON(w, http.StatusOK, dist)
}

func CreateTextFile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name         string  `json:"name"`
		Content      string  `json:"content"`
		Remark       *string `json:"remark"`
		DownloadName *string `json:"download_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if body.Name == "" {
		RespondError(w, http.StatusBadRequest, "Name is required")
		return
	}

	contentBytes := []byte(body.Content)
	origName := body.Name
	if filepath.Ext(origName) == "" {
		origName += ".txt"
	}

	dist := &model.DistFile{
		Name:         body.Name,
		FileType:     "text",
		OriginalName: origName,
		StoredName:   file.GenerateStoredName(origName),
		Size:         int64(len(contentBytes)),
		IsActive:     true,
		Remark:       body.Remark,
		DownloadName: body.DownloadName,
		CreatedAt:    time.Now().Format("2006-01-02 15:04:05"),
	}

	if _, err := db.DB.NewInsert().Model(dist).Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_ = file.SaveFileContent(dist, contentBytes)
	RespondJSON(w, http.StatusOK, dist)
}

func GetTextFileContent(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	var dist model.DistFile
	if err := db.DB.NewSelect().Model(&dist).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusNotFound, "File not found")
		return
	}

	if dist.FileType != "text" {
		RespondError(w, http.StatusBadRequest, "非文本文件无法以文本形式查看")
		return
	}

	data, err := file.ReadFileContent(&dist)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to read text file: "+err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]string{"content": string(data)})
}

func UpdateTextFileContent(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	var dist model.DistFile
	if err := db.DB.NewSelect().Model(&dist).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusNotFound, "File not found")
		return
	}

	if dist.FileType != "text" {
		RespondError(w, http.StatusBadRequest, "非文本文件无法直接编辑内容")
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	contentBytes := []byte(body.Content)
	dist.Size = int64(len(contentBytes))
	_ = file.SaveFileContent(&dist, contentBytes)
	_, _ = db.DB.NewUpdate().Model(&dist).Where("id = ?", id).Exec(r.Context())

	RespondJSON(w, http.StatusOK, dist)
}

func UpdateFile(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	var dist model.DistFile
	if err := db.DB.NewSelect().Model(&dist).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusNotFound, "File not found")
		return
	}

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if name, ok := body["name"].(string); ok {
		dist.Name = name
	}
	if active, ok := body["is_active"].(bool); ok {
		dist.IsActive = active
	}
	if rem, ok := body["remark"].(string); ok {
		dist.Remark = &rem
	}
	if dlName, ok := body["download_name"].(string); ok {
		dist.DownloadName = &dlName
	}
	if tplName, ok := body["template_name"].(string); ok {
		dist.TemplateName = &tplName
	}

	_, _ = db.DB.NewUpdate().Model(&dist).Where("id = ?", id).Exec(r.Context())
	RespondJSON(w, http.StatusOK, dist)
}

func DeleteFile(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	var dist model.DistFile
	if err := db.DB.NewSelect().Model(&dist).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusNotFound, "File not found")
		return
	}

	_ = file.DeleteStoredFile(&dist)
	_, _ = db.DB.NewDelete().Model(&dist).Where("id = ?", id).Exec(r.Context())

	RespondJSON(w, http.StatusOK, map[string]string{"message": "File deleted"})
}
