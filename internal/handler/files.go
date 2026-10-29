package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"noder/internal/config"
	"noder/internal/db"
	"noder/internal/model"
	"noder/internal/service/file"
	templateService "noder/internal/service/template"
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

// resolveZipTemplateMetadata validates the selected member and determines the
// configuration core. Explicit types are checked against the file when it is
// recognizable; placeholder-only custom templates remain valid.
func resolveZipTemplateMetadata(content []byte, requestedName, requestedType string) (string, string, error) {
	zr, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", "", fmt.Errorf("无效的 ZIP 文件")
	}

	templateName := strings.TrimSpace(requestedName)
	if templateName == "" {
		for _, entry := range zr.File {
			lower := strings.ToLower(entry.Name)
			if strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".json") {
				templateName = entry.Name
				break
			}
		}
		if templateName == "" {
			return "", "", fmt.Errorf("ZIP 内未找到 .json/.yaml/.yml 模板，请指定模板文件名")
		}
	} else {
		matched := false
		baseName := filepath.Base(templateName)
		for _, entry := range zr.File {
			if entry.Name == templateName || filepath.Base(entry.Name) == baseName {
				matched = true
				break
			}
		}
		if !matched {
			return "", "", fmt.Errorf("ZIP 中不存在模板文件: %s", templateName)
		}
	}

	templateType := templateService.NormalizeTemplateType(requestedType)
	if templateType == "" {
		return "", "", fmt.Errorf("不支持的模板类型 %q，仅支持 auto、sing-box 或 xray", requestedType)
	}
	detectedType, detectErr := templateService.DetectTemplateTypeFromZip(content, templateName)
	if detectErr != nil {
		return "", "", detectErr
	}
	if templateType == templateService.TemplateTypeAuto {
		templateType = detectedType
		if templateType == "" {
			templateType = templateService.TemplateTypeSing
		}
	} else if detectedType != "" && detectedType != templateType {
		return "", "", fmt.Errorf("模板类型不匹配：选择了 %s，但内容分析为 %s", templateType, detectedType)
	}
	return templateName, templateType, nil
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

	// The endpoint accepts either a normal multipart file or text content.
	// Text uploads are used by the admin UI and do not include a `file` part.
	mrFile, header, fileErr := r.FormFile("file")
	var content []byte
	var originalName string
	fileType := r.FormValue("file_type")
	if fileErr == nil {
		defer mrFile.Close()
		content, fileErr = io.ReadAll(mrFile)
		if fileErr != nil {
			RespondError(w, http.StatusInternalServerError, "Failed to read file: "+fileErr.Error())
			return
		}
		originalName = header.Filename
	} else {
		text := r.FormValue("content_text")
		if text == "" {
			RespondError(w, http.StatusBadRequest, "Missing 'file' or 'content_text' field")
			return
		}
		content = []byte(text)
		fileType = "text"
		originalName = r.FormValue("name")
		if originalName == "" {
			originalName = "text.txt"
		} else if filepath.Ext(originalName) == "" {
			originalName += ".txt"
		}
	}

	name := r.FormValue("name")
	if name == "" {
		name = originalName
	}
	if fileType == "auto" || fileType == "" {
		fileType = file.DetectFileType(originalName)
	}
	remark := r.FormValue("remark")
	templateName := r.FormValue("template_name")
	templateType := r.FormValue("template_type")
	downloadName := r.FormValue("download_name")
	forceStr := r.FormValue("force")
	force := forceStr == "true" || forceStr == "1"

	if fileType == "zip" {
		var resolveErr error
		templateName, templateType, resolveErr = resolveZipTemplateMetadata(content, templateName, templateType)
		if resolveErr != nil {
			RespondError(w, http.StatusBadRequest, resolveErr.Error())
			return
		}
	}

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
		if fileType == "zip" {
			dup.TemplateType = templateType
		}
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
		TemplateType: templateType,
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
		TemplateName *string `json:"template_name"`
		TemplateType *string `json:"template_type"`
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
	templateName := ""
	templateType := ""
	if body.TemplateName != nil {
		templateName = *body.TemplateName
	}
	if body.TemplateType != nil {
		templateType = *body.TemplateType
	}
	if ft == "zip" {
		var resolveErr error
		templateName, templateType, resolveErr = resolveZipTemplateMetadata(content, templateName, templateType)
		if resolveErr != nil {
			RespondError(w, http.StatusBadRequest, resolveErr.Error())
			return
		}
	}
	name := origName
	if body.Name != nil && *body.Name != "" {
		name = *body.Name
	}

	cachedAt := time.Now().Format("2006-01-02 15:04:05")
	dist := &model.DistFile{
		Name:         name,
		FileType:     ft,
		TemplateType: templateType,
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
	if templateName != "" {
		dist.TemplateName = &templateName
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
	if tplType, ok := body["template_type"].(string); ok {
		normalized := templateService.NormalizeTemplateType(tplType)
		if normalized == "" {
			RespondError(w, http.StatusBadRequest, "不支持的模板类型，仅支持 auto、sing-box 或 xray")
			return
		}
		if dist.FileType == "zip" {
			content, err := file.ReadFileContent(&dist)
			if err != nil {
				RespondError(w, http.StatusInternalServerError, "读取 ZIP 文件失败: "+err.Error())
				return
			}
			name := ""
			if dist.TemplateName != nil {
				name = *dist.TemplateName
			}
			_, normalized, err = resolveZipTemplateMetadata(content, name, normalized)
			if err != nil {
				RespondError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		dist.TemplateType = normalized
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
