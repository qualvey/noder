package handler

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"noder/internal/config"
	"noder/internal/db"
	"noder/internal/model"
)

func setupReleaseLogTest(t *testing.T) (*chi.Mux, func()) {
	tmpDir, err := os.MkdirTemp("", "noder-releaselog-test-*")
	if err != nil { t.Fatal(err) }
	config.Init(tmpDir)
	config.MallServiceSecret = "test-service-token"
	database, err := db.InitDB()
	if err != nil { os.RemoveAll(tmpDir); t.Fatalf("InitDB failed: %v", err) }
	r := chi.NewRouter()
	RegisterServiceRoutes(r)
	RegisterServiceReleaseLogRoutes(r)
	// also need nodes for endpoint
	cleanup := func() { _ = database.Close(); _ = os.RemoveAll(tmpDir) }
	return r, cleanup
}

func TestListServiceReleaseLogs_PaginationAndFilters(t *testing.T) {
	r, cleanup := setupReleaseLogTest(t)
	defer cleanup()

	// Seed a node and subscription to generate real release logs via release flow
	node := &model.Node{Tag: "vless-01", NodeName: "VLESS 01", Protocol: "vless", ServerAddress: "1.1.1.1", ServerPort: 443, IsActive: true, Status: "active"}
	if _, err := db.DB.NewInsert().Model(node).Exec(t.Context()); err != nil { t.Fatalf("insert node %v", err) }
	// create agent and binding so release succeeds
	agent := &model.AgentNode{ID: "ag_test_01", Name: "test-01", Status: "online"}
	// set heartbeat recent
	// use raw insert via bun; need LastHeartbeatAt
	// Insert agent directly
	if _, err := db.DB.NewInsert().Model(agent).Exec(t.Context()); err != nil { t.Fatalf("insert agent %v", err) }
	// binding
	binding := &model.EndpointAgentBinding{EndpointID: node.ID, AgentNodeID: agent.ID, Status: "active"}
	if _, err := db.DB.NewInsert().Model(binding).Exec(t.Context()); err != nil { t.Fatalf("insert binding %v", err) }

	// Activate subscription via service API (will create pending_activation grant)
	// We need to directly create a grant for faster test, then call release
	// Use the handler's Release flow: create user, credential, grant manually
	// Simpler: insert release log entries manually and test query
	for i := 0; i < 5; i++ {
		rec := &model.ServiceReleaseLog{
			SubscriptionID:  "sub_test_001",
			EndpointID:      node.ID,
			CallerIP:        "127.0.0.1",
			HttpStatus:      200,
			BusinessCode:    0,
			Delivered:       true,
			DeploymentCount: 1,
			AgentOnline:     true,
			LatencyMs:       10,
		}
		if i%2 == 0 {
			rec.ErrorCode = strPtr("BINDING_MISSING")
			rec.HttpStatus = 409
			rec.BusinessCode = 40902
			rec.Delivered = false
		}
		if _, err := db.DB.NewInsert().Model(rec).Exec(t.Context()); err != nil { t.Fatalf("insert log %v", err) }
	}

	// Test pagination: page=1 limit=2
	req := httptest.NewRequest("GET", "/api/admin/service-release-logs?page=1&limit=2", nil)
	req.Header.Set("X-Admin-Token", "admin-secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 { t.Fatalf("list page1 failed %d %s", w.Code, w.Body.String()) }
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items []model.ServiceReleaseLog `json:"items"`
			Total int `json:"total"`
			Page int `json:"page"`
			Limit int `json:"limit"`
			TotalPages int `json:"total_pages"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Total != 5 { t.Fatalf("expected total 5 got %d", resp.Data.Total) }
	if len(resp.Data.Items) != 2 { t.Fatalf("expected 2 items got %d", len(resp.Data.Items)) }
	if resp.Data.Page != 1 || resp.Data.Limit != 2 { t.Fatalf("page/limit mismatch %v", resp.Data) }

	// Filter delivered=true
	req2 := httptest.NewRequest("GET", "/api/admin/service-release-logs?delivered=true&limit=100", nil)
	req2.Header.Set("X-Admin-Token", "admin-secret")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != 200 { t.Fatalf("filter delivered failed %d %s", w2.Code, w2.Body.String()) }
	var resp2 struct {
		Code int `json:"code"`
		Data struct { Total int `json:"total"`; Items []model.ServiceReleaseLog `json:"items"` } `json:"data"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	// 5 items: 3 with false (i=0,2,4), 2 with true (i=1,3) -> delivered true should be 2
	if resp2.Data.Total != 2 { t.Fatalf("expected 2 delivered true got %d %s", resp2.Data.Total, w2.Body.String()) }

	// Test get by id
	id := resp.Data.Items[0].ID
	reqGet := httptest.NewRequest("GET", "/api/admin/service-release-logs/"+itoa(id), nil)
	reqGet.Header.Set("X-Admin-Token", "admin-secret")
	wGet := httptest.NewRecorder()
	r.ServeHTTP(wGet, reqGet)
	if wGet.Code != 200 { t.Fatalf("get by id failed %d %s", wGet.Code, wGet.Body.String()) }

	// Test stats
	reqStats := httptest.NewRequest("GET", "/api/admin/service-release-logs/stats", nil)
	reqStats.Header.Set("X-Admin-Token", "admin-secret")
	wStats := httptest.NewRecorder()
	r.ServeHTTP(wStats, reqStats)
	if wStats.Code != 200 { t.Fatalf("stats failed %d %s", wStats.Code, wStats.Body.String()) }

	// Auth check: without token should 401
	reqNoAuth := httptest.NewRequest("GET", "/api/admin/service-release-logs", nil)
	wNoAuth := httptest.NewRecorder()
	r.ServeHTTP(wNoAuth, reqNoAuth)
	if wNoAuth.Code != 401 { t.Fatalf("expected 401 without auth got %d", wNoAuth.Code) }
}

func strPtr(s string) *string { return &s }
func itoa(i int64) string { return strconv.FormatInt(i, 10) }
