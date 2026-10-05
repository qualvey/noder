package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"noder/internal/config"
)

func TestServiceAuthMiddleware(t *testing.T) {
	origSecret := config.MallServiceSecret
	config.MallServiceSecret = "test-secret-123"
	defer func() { config.MallServiceSecret = origSecret }()

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		RespondServiceSuccess(w, map[string]string{"result": "ok"})
	})

	handler := ServiceAuth(nextHandler)

	// 1. Missing token
	req := httptest.NewRequest("GET", "/api/service/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	// 2. Wrong token
	req = httptest.NewRequest("GET", "/api/service/test", nil)
	req.Header.Set("X-Service-Token", "wrong-secret")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	// 3. Valid token via X-Service-Token
	req = httptest.NewRequest("GET", "/api/service/test", nil)
	req.Header.Set("X-Service-Token", "test-secret-123")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}
	if resp["code"].(float64) != 0 {
		t.Fatalf("expected code 0, got %v", resp["code"])
	}

	// 4. Valid token via Authorization: Bearer
	req = httptest.NewRequest("GET", "/api/service/test", nil)
	req.Header.Set("Authorization", "Bearer test-secret-123")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
