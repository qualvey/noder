package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"noder/internal/config"
)

func TestSettingsRoutes(t *testing.T) {
	r, cleanup := setupServiceTest(t)
	defer cleanup()

	RegisterSettingsRoutes(r)
	RegisterDownloadRoutes(r)

	// 1. Update settings
	updatePayload := map[string]string{
		"site_name":     "Noder Management Center",
		"custom_banner": "Welcome",
	}
	body, _ := json.Marshal(updatePayload)
	reqPut := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(body))
	reqPut.Header.Set("X-Admin-Token", config.AdminSecretToken)
	reqPut.Header.Set("Content-Type", "application/json")
	wPut := httptest.NewRecorder()
	r.ServeHTTP(wPut, reqPut)
	if wPut.Code != http.StatusOK {
		t.Fatalf("expected 200 on update settings, got %d: %s", wPut.Code, wPut.Body.String())
	}

	// 2. Get all settings
	reqGet := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	reqGet.Header.Set("X-Admin-Token", config.AdminSecretToken)
	wGet := httptest.NewRecorder()
	r.ServeHTTP(wGet, reqGet)
	if wGet.Code != http.StatusOK {
		t.Fatalf("expected 200 on get settings, got %d", wGet.Code)
	}
	var settingsMap map[string]string
	_ = json.Unmarshal(wGet.Body.Bytes(), &settingsMap)
	if settingsMap["site_name"] != "Noder Management Center" || settingsMap["custom_banner"] != "Welcome" {
		t.Fatalf("unexpected settings: %v", settingsMap)
	}

	// 3. Reset shared token
	reqReset := httptest.NewRequest(http.MethodPost, "/api/settings/shared-token/reset", nil)
	reqReset.Header.Set("X-Admin-Token", config.AdminSecretToken)
	wReset := httptest.NewRecorder()
	r.ServeHTTP(wReset, reqReset)
	if wReset.Code != http.StatusOK {
		t.Fatalf("expected 200 on reset shared token, got %d: %s", wReset.Code, wReset.Body.String())
	}
	var resetResp map[string]string
	_ = json.Unmarshal(wReset.Body.Bytes(), &resetResp)
	tok := resetResp["token"]
	if len(tok) < 16 {
		t.Fatalf("invalid shared token generated: %s", tok)
	}

	// 4. Query shared token
	reqTok := httptest.NewRequest(http.MethodGet, "/api/settings/shared-token", nil)
	reqTok.Header.Set("X-Admin-Token", config.AdminSecretToken)
	wTok := httptest.NewRecorder()
	r.ServeHTTP(wTok, reqTok)
	if wTok.Code != http.StatusOK {
		t.Fatalf("expected 200 on query shared token, got %d", wTok.Code)
	}
	var queryResp map[string]string
	_ = json.Unmarshal(wTok.Body.Bytes(), &queryResp)
	if queryResp["token"] != tok {
		t.Fatalf("expected queried token %s, got %s", tok, queryResp["token"])
	}
}
