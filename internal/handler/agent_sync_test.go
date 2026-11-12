package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"noder/internal/config"
	"noder/internal/db"
	"noder/internal/model"
)

func setupAgentTest(t *testing.T) (*chi.Mux, *model.AgentNode, *model.Node, func()) {
	tmpDir, err := os.MkdirTemp("", "noder-agent-test-*")
	if err != nil {
		t.Fatal(err)
	}

	config.Init(tmpDir)

	database, err := db.InitDB()
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("InitDB failed: %v", err)
	}

	ctx := context.Background()

	// Seed AgentNode
	token := "secret-agent-token-123"
	agent := &model.AgentNode{
		ID:                     "ag_test_001",
		Name:                   "Test Agent HK",
		ControlTokenCiphertext: &token,
		Status:                 "offline",
	}
	if _, err := db.DB.NewInsert().Model(agent).Exec(ctx); err != nil {
		t.Fatalf("Failed to seed agent: %v", err)
	}

	// Seed Node (Endpoint)
	nodeTag := "line-hk-01"
	inboundTag := "in-443"
	flow := "xtls-rprx-vision"
	node := &model.Node{
		ID:            101,
		Tag:           nodeTag,
		InboundTag:    &inboundTag,
		NodeName:      "HK 01",
		Protocol:      "vless",
		ServerAddress: "1.2.3.4",
		ServerPort:    443,
		Flow:          &flow,
		IsActive:      true,
		Status:        "active",
	}
	if _, err := db.DB.NewInsert().Model(node).Exec(ctx); err != nil {
		t.Fatalf("Failed to seed node: %v", err)
	}

	// Seed Binding
	binding := &model.EndpointAgentBinding{
		EndpointID:  node.ID,
		AgentNodeID: agent.ID,
		Status:      "active",
	}
	if _, err := db.DB.NewInsert().Model(binding).Exec(ctx); err != nil {
		t.Fatalf("Failed to seed binding: %v", err)
	}

	r := chi.NewRouter()
	RegisterAgentRoutes(r)

	cleanup := func() {
		_ = database.Close()
		_ = os.RemoveAll(tmpDir)
	}

	return r, agent, node, cleanup
}

func TestAgentHeartbeat(t *testing.T) {
	r, agent, node, cleanup := setupAgentTest(t)
	defer cleanup()

	// 1. Missing Token -> 401
	req := httptest.NewRequest("POST", "/api/agent/v1/heartbeat", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing token, got %d", w.Code)
	}

	// 2. Invalid Token -> 401
	req = httptest.NewRequest("POST", "/api/agent/v1/heartbeat", bytes.NewReader([]byte("{}")))
	req.Header.Set("X-Agent-Token", "wrong-token")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong token, got %d", w.Code)
	}

	// 3. Valid Heartbeat -> 200
	hbBody := map[string]interface{}{
		"agent_version": "1.0.0",
		"xray_version":  "1.8.24",
		"hostname":      "line-hk-01",
		"capabilities":  []string{"xray.vless", "traffic"},
		"endpoints": []map[string]interface{}{
			{
				"endpoint_id": node.ID,
				"inbound_tag": "in-443",
				"ready":       true,
			},
		},
	}
	bodyBytes, _ := json.Marshal(hbBody)
	req = httptest.NewRequest("POST", "/api/agent/v1/heartbeat", bytes.NewReader(bodyBytes))
	req.Header.Set("X-Agent-Token", *agent.ControlTokenCiphertext)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for heartbeat, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			ServerTime      string `json:"server_time"`
			SyncIntervalSec int    `json:"sync_interval_sec"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Code != 0 || resp.Data.SyncIntervalSec != 15 {
		t.Fatalf("unexpected heartbeat resp: %+v", resp)
	}

	// Verify agent online
	ctx := context.Background()
	var updatedAgent model.AgentNode
	_ = db.DB.NewSelect().Model(&updatedAgent).Where("id = ?", agent.ID).Scan(ctx)
	if updatedAgent.Status != "online" || updatedAgent.LastHeartbeatAt == nil {
		t.Fatalf("agent not updated to online: %+v", updatedAgent)
	}

	// Verify isAgentOnlineForEndpoint
	if !isAgentOnlineForEndpoint(ctx, node.ID) {
		t.Fatalf("isAgentOnlineForEndpoint expected true")
	}
}

func TestAgentDeploymentsAndReport(t *testing.T) {
	r, agent, node, cleanup := setupAgentTest(t)
	defer cleanup()

	ctx := context.Background()

	// Seed User, Credential, Grant, Deployment
	user := &model.User{ID: 1, Token: "tok-user-1", Status: "active"}
	_, _ = db.DB.NewInsert().Model(user).Exec(ctx)

	uuidVal := "8b9e59d9-2e11-4712-a7d1-0f7236da4cfb"
	cred := &model.Credential{
		ID:        "cred_001",
		UserID:    user.ID,
		Scheme:    "vless",
		UUIDValue: &uuidVal,
		Status:    "active",
	}
	_, _ = db.DB.NewInsert().Model(cred).Exec(ctx)

	grantFormat := "landing"
	grant := &model.AccessGrant{
		ID:                "grant_001",
		UserID:            user.ID,
		EndpointID:        node.ID,
		CredentialID:      cred.ID,
		Status:            "active",
		Format:            &grantFormat,
		TrafficQuotaBytes: 1000000,
		TrafficUsedBytes:  200000,
	}
	_, _ = db.DB.NewInsert().Model(grant).Exec(ctx)

	dep := &model.CredentialDeployment{
		CredentialID: cred.ID,
		EndpointID:   node.ID,
		AgentNodeID:  agent.ID,
		DesiredState: "active",
		AppliedState: "unknown",
		Revision:     "1710000000100",
	}
	_, _ = db.DB.NewInsert().Model(dep).Exec(ctx)

	// 1. GET Deployments
	req := httptest.NewRequest("GET", "/api/agent/v1/deployments", nil)
	req.Header.Set("X-Agent-Token", *agent.ControlTokenCiphertext)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var depResp struct {
		Code int `json:"code"`
		Data struct {
			AgentID string `json:"agent_id"`
			Items   []struct {
				DeploymentKey struct {
					CredentialID string `json:"credential_id"`
					EndpointID   int64  `json:"endpoint_id"`
					AgentNodeID  string `json:"agent_node_id"`
				} `json:"deployment_key"`
				DesiredState string `json:"desired_state"`
				Revision     string `json:"revision"`
				Endpoint     struct {
					EndpointID int64  `json:"endpoint_id"`
					InboundTag string `json:"inbound_tag"`
				} `json:"endpoint"`
				Credential struct {
					CredentialID string  `json:"credential_id"`
					Scheme       string  `json:"scheme"`
					Email        string  `json:"email"`
					UUID         *string `json:"uuid"`
				} `json:"credential"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &depResp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(depResp.Data.Items) != 1 {
		t.Fatalf("expected 1 deployment item, got %d", len(depResp.Data.Items))
	}
	item := depResp.Data.Items[0]
	if item.DeploymentKey.CredentialID != cred.ID || item.Endpoint.InboundTag != "in-443" {
		t.Fatalf("unexpected item fields: %+v", item)
	}
	if item.Credential.UUID == nil || *item.Credential.UUID != uuidVal {
		t.Fatalf("expected uuid %s, got %+v", uuidVal, item.Credential.UUID)
	}

	// 2. Incremental GET with since_revision=1710000000100 -> should return 0 items
	req = httptest.NewRequest("GET", "/api/agent/v1/deployments?since_revision=1710000000100", nil)
	req.Header.Set("X-Agent-Token", *agent.ControlTokenCiphertext)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if err := json.Unmarshal(w.Body.Bytes(), &depResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(depResp.Data.Items) != 0 {
		t.Fatalf("expected 0 items with since_revision, got %d", len(depResp.Data.Items))
	}

	// 3. Report Deployments: Stale Revision Conflict
	staleReport := map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"credential_id": cred.ID,
				"endpoint_id":   node.ID,
				"applied_state": "active",
				"revision":      "1710000000050", // older than 1710000000100
				"applied_at":    time.Now().UTC().Format(time.RFC3339),
				"xray_user_key": "c_cred_001@line-hk-01",
			},
		},
	}
	repBytes, _ := json.Marshal(staleReport)
	req = httptest.NewRequest("POST", "/api/agent/v1/deployments/report", bytes.NewReader(repBytes))
	req.Header.Set("X-Agent-Token", *agent.ControlTokenCiphertext)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var reportResp struct {
		Code int `json:"code"`
		Data struct {
			Accepted  int `json:"accepted"`
			Conflicts []struct {
				CredentialID    string `json:"credential_id"`
				Reason          string `json:"reason"`
				CurrentRevision string `json:"current_revision"`
			} `json:"conflicts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &reportResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if reportResp.Data.Accepted != 0 || len(reportResp.Data.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %+v", reportResp.Data)
	}
	if reportResp.Data.Conflicts[0].Reason != "stale_revision" {
		t.Fatalf("expected stale_revision conflict reason, got %s", reportResp.Data.Conflicts[0].Reason)
	}

	// 4. Report Deployments: Successful Current Revision
	okReport := map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"credential_id": cred.ID,
				"endpoint_id":   node.ID,
				"applied_state": "active",
				"revision":      "1710000000100",
				"applied_at":    time.Now().UTC().Format(time.RFC3339),
				"xray_user_key": "c_cred_001@line-hk-01",
			},
		},
	}
	repBytes, _ = json.Marshal(okReport)
	req = httptest.NewRequest("POST", "/api/agent/v1/deployments/report", bytes.NewReader(repBytes))
	req.Header.Set("X-Agent-Token", *agent.ControlTokenCiphertext)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if err := json.Unmarshal(w.Body.Bytes(), &reportResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if reportResp.Data.Accepted != 1 || len(reportResp.Data.Conflicts) != 0 {
		t.Fatalf("expected accepted: 1, got %+v", reportResp.Data)
	}

	// Verify in DB
	var depAfter model.CredentialDeployment
	_ = db.DB.NewSelect().Model(&depAfter).
		Where("credential_id = ? AND endpoint_id = ? AND agent_node_id = ?", cred.ID, node.ID, agent.ID).
		Scan(ctx)
	if depAfter.AppliedState != "active" {
		t.Fatalf("expected applied_state active, got %s", depAfter.AppliedState)
	}
}

func TestAgentTrafficReportIdempotency(t *testing.T) {
	r, agent, node, cleanup := setupAgentTest(t)
	defer cleanup()

	ctx := context.Background()

	// Seed user, cred, grant
	user := &model.User{ID: 2, Token: "tok-user-2", Status: "active"}
	_, _ = db.DB.NewInsert().Model(user).Exec(ctx)

	cred := &model.Credential{ID: "cred_002", UserID: user.ID, Scheme: "vless", Status: "active"}
	_, _ = db.DB.NewInsert().Model(cred).Exec(ctx)

	grant := &model.AccessGrant{
		ID:                "grant_002",
		UserID:            user.ID,
		EndpointID:        node.ID,
		CredentialID:      cred.ID,
		Status:            "active",
		TrafficQuotaBytes: 1000000,
		TrafficUsedBytes:  1000,
	}
	_, _ = db.DB.NewInsert().Model(grant).Exec(ctx)

	trafficReport := map[string]interface{}{
		"report_id":   "rep-uuid-0001",
		"endpoint_id": node.ID,
		"observed_at": time.Now().UTC().Format(time.RFC3339),
		"items": []map[string]interface{}{
			{
				"credential_id": cred.ID,
				"grant_id":      grant.ID,
				"uplink_bytes":  500,
				"downlink_bytes": 1500,
			},
		},
	}
	bodyBytes, _ := json.Marshal(trafficReport)

	// First call -> success, increment traffic by 2000
	req := httptest.NewRequest("POST", "/api/agent/v1/traffic/report", bytes.NewReader(bodyBytes))
	req.Header.Set("X-Agent-Token", *agent.ControlTokenCiphertext)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("first traffic report expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var g1 model.AccessGrant
	_ = db.DB.NewSelect().Model(&g1).Where("id = ?", grant.ID).Scan(ctx)
	if g1.TrafficUsedBytes != 3000 {
		t.Fatalf("expected traffic 3000, got %d", g1.TrafficUsedBytes)
	}

	// Retry identical report_id -> idempotent accepted, traffic MUST NOT increment again
	req = httptest.NewRequest("POST", "/api/agent/v1/traffic/report", bytes.NewReader(bodyBytes))
	req.Header.Set("X-Agent-Token", *agent.ControlTokenCiphertext)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("retry traffic report expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var g2 model.AccessGrant
	_ = db.DB.NewSelect().Model(&g2).Where("id = ?", grant.ID).Scan(ctx)
	if g2.TrafficUsedBytes != 3000 {
		t.Fatalf("idempotency violated: traffic incremented again to %d", g2.TrafficUsedBytes)
	}
}

func TestCreateDesiredDeployments(t *testing.T) {
	_, agent, node, cleanup := setupAgentTest(t)
	defer cleanup()
	ctx := context.Background()

	createDesiredDeployments(ctx, "cred_test_123", node.ID, "active")

	var deps []model.CredentialDeployment
	err := db.DB.NewSelect().Model(&deps).Scan(ctx)
	t.Logf("deps count: %d, err: %v", len(deps), err)
	if len(deps) == 0 {
		t.Fatalf("expected at least 1 deployment, got 0. Agent: %s, Node: %d", agent.ID, node.ID)
	}
}

