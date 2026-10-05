package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"noder/internal/db"
	"noder/internal/model"
)

func RegisterTemplateRoutes(r chi.Router) {
	r.Route("/api/templates", func(r chi.Router) {
		r.Use(AdminAuth)
		r.Get("/", ListTemplates)
		r.Get("/{id}", GetTemplate)
		r.Put("/{id}", UpdateTemplate)
		r.Get("/{id}/history", GetTemplateHistory)
		r.Post("/{id}/rollback/{version}", RollbackTemplate)
	})

	// 模板分支管理
	r.Route("/api/template-branches", func(r chi.Router) {
		r.Use(AdminAuth)
		r.Get("/", ListTemplateBranches)
		r.Post("/", CreateTemplateBranch)
		r.Get("/{id}", GetTemplateBranch)
		r.Put("/{id}", UpdateTemplateBranch)
		r.Delete("/{id}", DeleteTemplateBranch)
	})
}

func ListTemplates(w http.ResponseWriter, r *http.Request) {
	var templates []*model.Template
	if err := db.DB.NewSelect().Model(&templates).Order("id ASC").Scan(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if templates == nil {
		templates = []*model.Template{}
	}
	RespondJSON(w, http.StatusOK, templates)
}

func GetTemplate(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid template ID")
		return
	}

	var tpl model.Template
	if err := db.DB.NewSelect().Model(&tpl).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusNotFound, "Template not found")
		return
	}

	RespondJSON(w, http.StatusOK, tpl)
}

type TemplateUpdateReq struct {
	Content string `json:"content"`
	Remark  string `json:"remark"`
}

func UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid template ID")
		return
	}

	var tpl model.Template
	if err := db.DB.NewSelect().Model(&tpl).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusNotFound, "Template not found")
		return
	}

	var req TemplateUpdateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	tpl.Content = req.Content
	tpl.Version += 1
	tpl.UpdatedAt = time.Now()

	if _, err := db.DB.NewUpdate().Model(&tpl).Where("id = ?", id).Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	remark := req.Remark
	if remark == "" {
		remark = fmt.Sprintf("更新版本 v%d", tpl.Version)
	}

	history := &model.TemplateHistory{
		TemplateID: tpl.ID,
		Target:     tpl.Target,
		Version:    tpl.Version,
		Content:    tpl.Content,
		Remark:     remark,
		CreatedAt:  time.Now(),
	}
	_, _ = db.DB.NewInsert().Model(history).Exec(r.Context())

	RespondJSON(w, http.StatusOK, tpl)
}

func GetTemplateHistory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid template ID")
		return
	}

	var histories []*model.TemplateHistory
	err = db.DB.NewSelect().
		Model(&histories).
		Where("template_id = ?", id).
		Order("version DESC").
		Scan(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if histories == nil {
		histories = []*model.TemplateHistory{}
	}

	RespondJSON(w, http.StatusOK, histories)
}

func RollbackTemplate(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid template ID")
		return
	}

	verStr := chi.URLParam(r, "version")
	version, err := strconv.Atoi(verStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid version")
		return
	}

	var tpl model.Template
	if err := db.DB.NewSelect().Model(&tpl).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusNotFound, "Template not found")
		return
	}

	var history model.TemplateHistory
	err = db.DB.NewSelect().
		Model(&history).
		Where("template_id = ? AND version = ?", id, version).
		Scan(r.Context())
	if err != nil {
		RespondError(w, http.StatusNotFound, fmt.Sprintf("History version v%d not found", version))
		return
	}

	tpl.Content = history.Content
	tpl.Version += 1
	tpl.UpdatedAt = time.Now()

	if _, err := db.DB.NewUpdate().Model(&tpl).Where("id = ?", id).Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	newHistory := &model.TemplateHistory{
		TemplateID: tpl.ID,
		Target:     tpl.Target,
		Version:    tpl.Version,
		Content:    tpl.Content,
		Remark:     fmt.Sprintf("回滚到历史版本 v%d", version),
		CreatedAt:  time.Now(),
	}
	_, _ = db.DB.NewInsert().Model(newHistory).Exec(r.Context())

	RespondJSON(w, http.StatusOK, tpl)
}

// TemplateBranch 请求结构
type TemplateBranchCreateReq struct {
	Name          string  `json:"name"`
	BaseTemplateID *int64 `json:"base_template_id"` // 可选，传入则复制该模板内容
	Target        string  `json:"target"`
	ContentFormat string  `json:"content_format"`
	Content       string  `json:"content"`
	Remark        *string `json:"remark,omitempty"`
}

type TemplateBranchUpdateReq struct {
	Name          string  `json:"name"`
	Content       string  `json:"content"`
	ContentFormat string  `json:"content_format"`
	Remark        *string `json:"remark,omitempty"`
	IsActive      *bool   `json:"is_active,omitempty"`
}

func ListTemplateBranches(w http.ResponseWriter, r *http.Request) {
	var branches []*model.TemplateBranch
	if err := db.DB.NewSelect().Model(&branches).Order("id ASC").Scan(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if branches == nil {
		branches = []*model.TemplateBranch{}
	}
	RespondJSON(w, http.StatusOK, branches)
}

func CreateTemplateBranch(w http.ResponseWriter, r *http.Request) {
	var req TemplateBranchCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		RespondError(w, http.StatusBadRequest, "Branch name is required")
		return
	}

	content := req.Content
	// 如果传入了 base_template_id，则复制该模板内容
	if req.BaseTemplateID != nil && *req.BaseTemplateID > 0 {
		var tpl model.Template
		if err := db.DB.NewSelect().Model(&tpl).Where("id = ?", *req.BaseTemplateID).Scan(r.Context()); err != nil {
			RespondError(w, http.StatusNotFound, "Base template not found")
			return
		}
		content = tpl.Content
		if req.Target == "" {
			req.Target = tpl.Target
		}
		if req.ContentFormat == "" {
			req.ContentFormat = tpl.ContentFormat
		}
	}

	if content == "" {
		RespondError(w, http.StatusBadRequest, "Content is required")
		return
	}
	if req.Target == "" {
		RespondError(w, http.StatusBadRequest, "Target is required")
		return
	}
	if req.ContentFormat == "" {
		RespondError(w, http.StatusBadRequest, "ContentFormat is required")
		return
	}

	now := time.Now()
	branch := &model.TemplateBranch{
		Name:          req.Name,
		Target:        req.Target,
		ContentFormat: req.ContentFormat,
		Content:       content,
		Remark:        req.Remark,
		IsActive:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if _, err := db.DB.NewInsert().Model(branch).Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusCreated, branch)
}

func GetTemplateBranch(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid branch ID")
		return
	}

	var branch model.TemplateBranch
	if err := db.DB.NewSelect().Model(&branch).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusNotFound, "Template branch not found")
		return
	}

	RespondJSON(w, http.StatusOK, branch)
}

func UpdateTemplateBranch(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid branch ID")
		return
	}

	var branch model.TemplateBranch
	if err := db.DB.NewSelect().Model(&branch).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusNotFound, "Template branch not found")
		return
	}

	var req TemplateBranchUpdateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if strings.TrimSpace(req.Name) != "" {
		branch.Name = req.Name
	}
	if req.Content != "" {
		branch.Content = req.Content
	}
	if req.ContentFormat != "" {
		branch.ContentFormat = req.ContentFormat
	}
	if req.Remark != nil {
		branch.Remark = req.Remark
	}
	if req.IsActive != nil {
		branch.IsActive = *req.IsActive
	}
	branch.UpdatedAt = time.Now()

	if _, err := db.DB.NewUpdate().Model(&branch).Where("id = ?", id).Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, branch)
}

func DeleteTemplateBranch(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid branch ID")
		return
	}

	// 检查是否有用户正在使用此分支
	count, err := db.DB.NewSelect().Model((*model.User)(nil)).Where("template_branch_id = ?", id).Count(r.Context())
	if err == nil && count > 0 {
		RespondError(w, http.StatusBadRequest, fmt.Sprintf("This branch is assigned to %d user(s). Please reassign them first.", count))
		return
	}

	res, err := db.DB.NewDelete().Model((*model.TemplateBranch)(nil)).Where("id = ?", id).Exec(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		RespondError(w, http.StatusNotFound, "Template branch not found")
		return
	}

	RespondJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("Template branch %d deleted successfully", id),
	})
}
