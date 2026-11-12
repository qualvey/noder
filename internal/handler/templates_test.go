package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"noder/internal/config"
	"noder/internal/db"
	"noder/internal/model"
)

func TestTemplatesLifecycle(t *testing.T) {
	r, cleanup := setupServiceTest(t)
	defer cleanup()

	RegisterTemplateRoutes(r)

	// 1. Seed a template
	tpl := &model.Template{
		Name:          "singbox_test_tpl",
		Target:        "sing-box",
		ContentFormat: "json",
		Content:       `{"log":{"level":"info"},"outbounds":[]}`,
		Version:       1,
		UpdatedAt:     time.Now(),
	}
	if _, err := db.DB.NewInsert().Model(tpl).Exec(t.Context()); err != nil {
		t.Fatalf("failed to seed template: %v", err)
	}

	// Also insert initial version 1 history
	initHist := &model.TemplateHistory{
		TemplateID: tpl.ID,
		Target:     tpl.Target,
		Version:    1,
		Content:    tpl.Content,
		Remark:     "初始化版本",
		CreatedAt:  time.Now(),
	}
	_, _ = db.DB.NewInsert().Model(initHist).Exec(t.Context())

	// 2. List Templates
	reqList := httptest.NewRequest(http.MethodGet, "/api/templates/", nil)
	reqList.Header.Set("X-Admin-Token", config.AdminSecretToken)
	wList := httptest.NewRecorder()
	r.ServeHTTP(wList, reqList)
	if wList.Code != http.StatusOK {
		t.Fatalf("expected 200 on list templates, got %d", wList.Code)
	}
	var listResp []*model.Template
	_ = json.Unmarshal(wList.Body.Bytes(), &listResp)
	if len(listResp) == 0 {
		t.Fatal("expected at least 1 template in list")
	}

	// 3. Get single template
	reqGet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/templates/%d", tpl.ID), nil)
	reqGet.Header.Set("X-Admin-Token", config.AdminSecretToken)
	wGet := httptest.NewRecorder()
	r.ServeHTTP(wGet, reqGet)
	if wGet.Code != http.StatusOK {
		t.Fatalf("expected 200 on get template, got %d: %s", wGet.Code, wGet.Body.String())
	}
	var fetchedTpl model.Template
	_ = json.Unmarshal(wGet.Body.Bytes(), &fetchedTpl)
	if fetchedTpl.Name != "singbox_test_tpl" || fetchedTpl.Version != 1 {
		t.Fatalf("unexpected fetched template: %+v", fetchedTpl)
	}

	// 4. Update template (bumps to version 2)
	updatePayload := TemplateUpdateReq{
		Content: `{"log":{"level":"debug"},"outbounds":[]}`,
		Remark:  "调整日志等级为 debug",
	}
	updateBytes, _ := json.Marshal(updatePayload)
	reqUpdate := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/templates/%d", tpl.ID), bytes.NewReader(updateBytes))
	reqUpdate.Header.Set("X-Admin-Token", config.AdminSecretToken)
	reqUpdate.Header.Set("Content-Type", "application/json")
	wUpdate := httptest.NewRecorder()
	r.ServeHTTP(wUpdate, reqUpdate)
	if wUpdate.Code != http.StatusOK {
		t.Fatalf("expected 200 on update template, got %d: %s", wUpdate.Code, wUpdate.Body.String())
	}
	var updatedTpl model.Template
	_ = json.Unmarshal(wUpdate.Body.Bytes(), &updatedTpl)
	if updatedTpl.Version != 2 {
		t.Fatalf("expected version 2, got %d", updatedTpl.Version)
	}

	// 5. Get History
	reqHist := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/templates/%d/history", tpl.ID), nil)
	reqHist.Header.Set("X-Admin-Token", config.AdminSecretToken)
	wHist := httptest.NewRecorder()
	r.ServeHTTP(wHist, reqHist)
	if wHist.Code != http.StatusOK {
		t.Fatalf("expected 200 on get history, got %d", wHist.Code)
	}
	var histories []*model.TemplateHistory
	_ = json.Unmarshal(wHist.Body.Bytes(), &histories)
	if len(histories) < 2 {
		t.Fatalf("expected at least 2 versions in history, got %d", len(histories))
	}

	// 6. Rollback to version 1
	reqRollback := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/templates/%d/rollback/1", tpl.ID), nil)
	reqRollback.Header.Set("X-Admin-Token", config.AdminSecretToken)
	wRollback := httptest.NewRecorder()
	r.ServeHTTP(wRollback, reqRollback)
	if wRollback.Code != http.StatusOK {
		t.Fatalf("expected 200 on rollback template, got %d: %s", wRollback.Code, wRollback.Body.String())
	}
	var rolledBackTpl model.Template
	_ = json.Unmarshal(wRollback.Body.Bytes(), &rolledBackTpl)
	if rolledBackTpl.Version != 3 {
		t.Fatalf("expected version 3 after rollback, got %d", rolledBackTpl.Version)
	}
	if rolledBackTpl.Content != `{"log":{"level":"info"},"outbounds":[]}` {
		t.Fatalf("expected rolled back content from v1, got: %s", rolledBackTpl.Content)
	}
}
