package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"noder/internal/config"
	"noder/internal/db"
	"noder/internal/model"
)

func setupAgentAdminTest(t *testing.T) (*chi.Mux, func()) {
	tmpDir, err := os.MkdirTemp("", "noder-agent-admin-test-*")
	if err != nil {
		t.Fatal(err)
	}
	config.Init(tmpDir)
	config.MallServiceSecret = "test-service-token"
	database, err := db.InitDB()
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("InitDB failed: %v", err)
	}
	r := chi.NewRouter()
	RegisterAgentAdminRoutes(r)
	RegisterNodeRoutes(r)
	RegisterServiceRoutes(r)
	cleanup := func() {
		_ = database.Close()
		_ = os.RemoveAll(tmpDir)
	}
	return r, cleanup
}

func TestAgentAdminCRUD(t *testing.T) {
	r, cleanup := setupAgentAdminTest(t)
	defer cleanup()

	// create agent without id
	body, _ := json.Marshal(map[string]string{"name": "HK-01"})
	req := httptest.NewRequest("POST", "/api/agents", bytes.NewReader(body))
	req.Header.Set("X-Admin-Token", "admin-secret")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create agent failed %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			ControlToken string `json:"control_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.ID == "" || resp.Data.ControlToken == "" {
		t.Fatalf("missing id/token %v", resp.Data)
	}
	agentID := resp.Data.ID

	// create with explicit id
	body2, _ := json.Marshal(map[string]string{"id": "ag_hk_01", "name": "HK explicit"})
	req2 := httptest.NewRequest("POST", "/api/agents", bytes.NewReader(body2))
	req2.Header.Set("X-Admin-Token", "admin-secret")
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("create explicit id failed %d %s", w2.Code, w2.Body.String())
	}
	// duplicate id should 409
	reqDup := httptest.NewRequest("POST", "/api/agents", bytes.NewReader(body2))
	reqDup.Header.Set("X-Admin-Token", "admin-secret")
	reqDup.Header.Set("Content-Type", "application/json")
	wDup := httptest.NewRecorder()
	r.ServeHTTP(wDup, reqDup)
	if wDup.Code != http.StatusConflict {
		t.Fatalf("expected 409 duplicate id got %d %s", wDup.Code, wDup.Body.String())
	}

	// list
	reqList := httptest.NewRequest("GET", "/api/agents", nil)
	reqList.Header.Set("X-Admin-Token", "admin-secret")
	wList := httptest.NewRecorder()
	r.ServeHTTP(wList, reqList)
	if wList.Code != http.StatusOK {
		t.Fatalf("list failed %d %s", wList.Code, wList.Body.String())
	}
	var listResp struct {
		Code int `json:"code"`
		Data struct {
			Total int `json:"total"`
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wList.Body.Bytes(), &listResp)
	if listResp.Data.Total < 2 {
		t.Fatalf("expected total >=2 got %d", listResp.Data.Total)
	}

	// get
	reqGet := httptest.NewRequest("GET", "/api/agents/"+agentID, nil)
	reqGet.Header.Set("X-Admin-Token", "admin-secret")
	wGet := httptest.NewRecorder()
	r.ServeHTTP(wGet, reqGet)
	if wGet.Code != http.StatusOK {
		t.Fatalf("get failed %d %s", wGet.Code, wGet.Body.String())
	}

	// update
	updBody, _ := json.Marshal(map[string]string{"name": "HK-renamed", "status": "disabled"})
	reqUpd := httptest.NewRequest("PUT", "/api/agents/"+agentID, bytes.NewReader(updBody))
	reqUpd.Header.Set("X-Admin-Token", "admin-secret")
	reqUpd.Header.Set("Content-Type", "application/json")
	wUpd := httptest.NewRecorder()
	r.ServeHTTP(wUpd, reqUpd)
	if wUpd.Code != http.StatusOK {
		t.Fatalf("update failed %d %s", wUpd.Code, wUpd.Body.String())
	}

	// rotate token
	reqRot := httptest.NewRequest("POST", "/api/agents/"+agentID+"/token/rotate", nil)
	reqRot.Header.Set("X-Admin-Token", "admin-secret")
	wRot := httptest.NewRecorder()
	r.ServeHTTP(wRot, reqRot)
	if wRot.Code != http.StatusOK {
		t.Fatalf("rotate failed %d %s", wRot.Code, wRot.Body.String())
	}
	var rotResp struct {
		Code int `json:"code"`
		Data struct {
			ControlToken string `json:"control_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wRot.Body.Bytes(), &rotResp)
	if rotResp.Data.ControlToken == resp.Data.ControlToken {
		t.Fatalf("token should change")
	}

	// delete without force (no binding) should succeed
	reqDel := httptest.NewRequest("DELETE", "/api/agents/"+agentID, nil)
	reqDel.Header.Set("X-Admin-Token", "admin-secret")
	wDel := httptest.NewRecorder()
	r.ServeHTTP(wDel, reqDel)
	if wDel.Code != http.StatusOK {
		t.Fatalf("delete failed %d %s", wDel.Code, wDel.Body.String())
	}
}

func TestAgentBindingFlow(t *testing.T) {
	r, cleanup := setupAgentAdminTest(t)
	defer cleanup()

	// create node
	node := &model.Node{Tag: "vless-01", NodeName: "VLESS 01", Protocol: "vless", ServerAddress: "1.1.1.1", ServerPort: 443, IsActive: true, Status: "active"}
	if _, err := db.DB.NewInsert().Model(node).Exec(t.Context()); err != nil {
		t.Fatalf("insert node %v", err)
	}
	// create agent
	body, _ := json.Marshal(map[string]string{"id": "ag_test_01", "name": "test-01"})
	req := httptest.NewRequest("POST", "/api/agents", bytes.NewReader(body))
	req.Header.Set("X-Admin-Token", "admin-secret")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create agent %d %s", w.Code, w.Body.String())
	}

	// bind via agent perspective
	bindBody, _ := json.Marshal(map[string]interface{}{"endpoint_id": node.ID})
	reqBind := httptest.NewRequest("POST", "/api/agents/ag_test_01/bindings", bytes.NewReader(bindBody))
	reqBind.Header.Set("X-Admin-Token", "admin-secret")
	reqBind.Header.Set("Content-Type", "application/json")
	wBind := httptest.NewRecorder()
	r.ServeHTTP(wBind, reqBind)
	if wBind.Code != http.StatusOK {
		t.Fatalf("bind failed %d %s", wBind.Code, wBind.Body.String())
	}
	// duplicate bind should be idempotent 200
	wBind2 := httptest.NewRecorder()
	reqBind2 := httptest.NewRequest("POST", "/api/agents/ag_test_01/bindings", bytes.NewReader(bindBody))
	reqBind2.Header.Set("X-Admin-Token", "admin-secret")
	reqBind2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wBind2, reqBind2)
	if wBind2.Code != http.StatusOK {
		t.Fatalf("duplicate bind expected 200 got %d %s", wBind2.Code, wBind2.Body.String())
	}

	// list agent bindings
	reqList := httptest.NewRequest("GET", "/api/agents/ag_test_01/bindings", nil)
	reqList.Header.Set("X-Admin-Token", "admin-secret")
	wList := httptest.NewRecorder()
	r.ServeHTTP(wList, reqList)
	if wList.Code != http.StatusOK {
		t.Fatalf("list agent bindings %d %s", wList.Code, wList.Body.String())
	}
	var listData struct {
		Code int `json:"code"`
		Data []struct {
			EndpointID int64 `json:"endpoint_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wList.Body.Bytes(), &listData)
	if len(listData.Data) != 1 || listData.Data[0].EndpointID != node.ID {
		t.Fatalf("expected 1 binding %v", listData.Data)
	}

	// list endpoint bindings symmetry
	reqEpList := httptest.NewRequest("GET", fmt.Sprintf("/api/endpoints/%d/bindings", node.ID), nil)
	reqEpList.Header.Set("X-Admin-Token", "admin-secret")
	wEpList := httptest.NewRecorder()
	r.ServeHTTP(wEpList, reqEpList)
	if wEpList.Code != http.StatusOK {
		t.Fatalf("list endpoint bindings %d %s", wEpList.Code, wEpList.Body.String())
	}

	// delete should be blocked without force if has binding
	reqDel := httptest.NewRequest("DELETE", "/api/agents/ag_test_01", nil)
	reqDel.Header.Set("X-Admin-Token", "admin-secret")
	wDel := httptest.NewRecorder()
	r.ServeHTTP(wDel, reqDel)
	if wDel.Code != http.StatusConflict {
		t.Fatalf("expected 409 when deleting bound agent got %d %s", wDel.Code, wDel.Body.String())
	}
	// force delete
	reqDelForce := httptest.NewRequest("DELETE", "/api/agents/ag_test_01?force=true", nil)
	reqDelForce.Header.Set("X-Admin-Token", "admin-secret")
	wDelForce := httptest.NewRecorder()
	r.ServeHTTP(wDelForce, reqDelForce)
	if wDelForce.Code != http.StatusOK {
		t.Fatalf("force delete failed %d %s", wDelForce.Code, wDelForce.Body.String())
	}

	// unbind test via endpoint perspective with batch
	// recreate agent and test batch
	body2, _ := json.Marshal(map[string]string{"id": "ag_test_02", "name": "test-02"})
	reqC2 := httptest.NewRequest("POST", "/api/agents", bytes.NewReader(body2))
	reqC2.Header.Set("X-Admin-Token", "admin-secret")
	reqC2.Header.Set("Content-Type", "application/json")
	wC2 := httptest.NewRecorder()
	r.ServeHTTP(wC2, reqC2)
	if wC2.Code != http.StatusOK {
		t.Fatalf("create ag2 %d %s", wC2.Code, wC2.Body.String())
	}
	node2 := &model.Node{Tag: "vless-02", NodeName: "VLESS 02", Protocol: "vless", ServerAddress: "2.2.2.2", ServerPort: 443, IsActive: true, Status: "active"}
	if _, err := db.DB.NewInsert().Model(node2).Exec(t.Context()); err != nil {
		t.Fatalf("insert node2 %v", err)
	}
	batchBind, _ := json.Marshal(map[string]interface{}{"endpoint_ids": []int64{node.ID, node2.ID}})
	reqBatch := httptest.NewRequest("POST", "/api/agents/ag_test_02/bindings", bytes.NewReader(batchBind))
	reqBatch.Header.Set("X-Admin-Token", "admin-secret")
	reqBatch.Header.Set("Content-Type", "application/json")
	wBatch := httptest.NewRecorder()
	r.ServeHTTP(wBatch, reqBatch)
	if wBatch.Code != http.StatusOK {
		t.Fatalf("batch bind failed %d %s", wBatch.Code, wBatch.Body.String())
	}
	// unbind single
	reqUnbind := httptest.NewRequest("DELETE", fmt.Sprintf("/api/agents/ag_test_02/bindings/%d", node.ID), nil)
	reqUnbind.Header.Set("X-Admin-Token", "admin-secret")
	wUnbind := httptest.NewRecorder()
	r.ServeHTTP(wUnbind, reqUnbind)
	if wUnbind.Code != http.StatusOK {
		t.Fatalf("unbind failed %d %s", wUnbind.Code, wUnbind.Body.String())
	}
}

