package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"noder/internal/config"
)

func TestAuthRoutes(t *testing.T) {
	r, cleanup := setupServiceTest(t)
	defer cleanup()

	RegisterAuthRoutes(r)

	// 1. Check without token -> 401
	req1 := httptest.NewRequest(http.MethodGet, "/api/auth/check", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", w1.Code)
	}

	// 2. Check with invalid token -> 401
	req2 := httptest.NewRequest(http.MethodGet, "/api/auth/check", nil)
	req2.Header.Set("X-Admin-Token", "wrong-secret")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with invalid token, got %d", w2.Code)
	}

	// 3. Check with valid X-Admin-Token -> 200
	req3 := httptest.NewRequest(http.MethodGet, "/api/auth/check", nil)
	req3.Header.Set("X-Admin-Token", config.AdminSecretToken)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid token, got %d: %s", w3.Code, w3.Body.String())
	}
	var res map[string]bool
	_ = json.Unmarshal(w3.Body.Bytes(), &res)
	if !res["valid"] {
		t.Fatalf("expected valid: true, got %v", res)
	}

	// 4. Check with Bearer authorization header -> 200
	req4 := httptest.NewRequest(http.MethodGet, "/api/auth/check", nil)
	req4.Header.Set("Authorization", "Bearer "+config.AdminSecretToken)
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Fatalf("expected 200 with Bearer token, got %d", w4.Code)
	}

	// 5. Change token via /api/auth/change
	changeBody, _ := json.Marshal(map[string]string{"token": "new-admin-secret-999"})
	reqChange := httptest.NewRequest(http.MethodPost, "/api/auth/change", bytes.NewReader(changeBody))
	reqChange.Header.Set("X-Admin-Token", config.AdminSecretToken)
	reqChange.Header.Set("Content-Type", "application/json")
	wChange := httptest.NewRecorder()
	r.ServeHTTP(wChange, reqChange)
	if wChange.Code != http.StatusOK {
		t.Fatalf("expected 200 on change token, got %d: %s", wChange.Code, wChange.Body.String())
	}

	// 6. Force change token non-localhost rejected
	reqForceNonLocal := httptest.NewRequest(http.MethodPost, "/api/auth/force-change", bytes.NewReader(changeBody))
	reqForceNonLocal.RemoteAddr = "192.168.1.100:54321"
	wForceNonLocal := httptest.NewRecorder()
	r.ServeHTTP(wForceNonLocal, reqForceNonLocal)
	if wForceNonLocal.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on force-change from non-localhost, got %d", wForceNonLocal.Code)
	}

	// 7. Force change token from localhost accepted
	reqForceLocal := httptest.NewRequest(http.MethodPost, "/api/auth/force-change", bytes.NewReader(changeBody))
	reqForceLocal.RemoteAddr = "127.0.0.1:54321"
	wForceLocal := httptest.NewRecorder()
	r.ServeHTTP(wForceLocal, reqForceLocal)
	if wForceLocal.Code != http.StatusOK {
		t.Fatalf("expected 200 on force-change from localhost, got %d: %s", wForceLocal.Code, wForceLocal.Body.String())
	}
}
