package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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
