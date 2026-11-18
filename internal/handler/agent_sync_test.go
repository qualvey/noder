package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

func TestAgentStateSyncFullySynced(t *testing.T) {
	r, agent, node, cleanup := setupAgentTest(t)
	defer cleanup()
	ctx := context.Background()

	credID := "cred_sync_001"
	dep := &model.CredentialDeployment{
		CredentialID: credID, EndpointID: node.ID, AgentNodeID: agent.ID,
		DesiredState: "active", AppliedState: "active", Revision: "100",
	}
	if _, err := db.DB.NewInsert().Model(dep).Exec(ctx); err != nil {
		t.Fatalf("insert dep: %v", err)
	}

	present := true
	body := map[string]interface{}{
		"items": []map[string]interface{}{
			{"credential_id": credID, "endpoint_id": node.ID, "applied_state": "active", "revision": "100", "present": present},
		},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/agent/v1/state/sync", bytes.NewReader(b))
	req.Header.Set("X-Agent-Token", *agent.ControlTokenCiphertext)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Drift       []map[string]interface{} `json:"drift"`
			Conflicts   []map[string]interface{} `json:"conflicts"`
			NextSyncSec int                      `json:"next_sync_sec"`
			ServerTime  string                   `json:"server_time"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Data.Drift) != 0 {
		t.Fatalf("expected 0 drift when fully synced, got %d: %+v", len(resp.Data.Drift), resp.Data.Drift)
	}
	if len(resp.Data.Conflicts) != 0 {
		t.Fatalf("expected 0 conflicts, got %+v", resp.Data.Conflicts)
	}
	if resp.Data.NextSyncSec != 30 {
		t.Fatalf("expected next_sync_sec 30, got %d", resp.Data.NextSyncSec)
	}
	if resp.Data.ServerTime == "" {
		t.Fatalf("expected server_time")
	}
	if _, err := time.Parse(time.RFC3339, resp.Data.ServerTime); err != nil {
		t.Fatalf("server_time not RFC3339: %v", err)
	}
	// verify DB last_sync_at updated and last_error cleared
	var updated model.CredentialDeployment
	if err := db.DB.NewSelect().Model(&updated).Where("credential_id = ? AND endpoint_id = ? AND agent_node_id = ?", credID, node.ID, agent.ID).Scan(ctx); err != nil {
		t.Fatalf("load dep: %v", err)
	}
	if updated.LastSyncAt == nil {
		t.Fatalf("expected last_sync_at set")
	}
	if updated.LastError != nil {
		t.Fatalf("expected last_error nil, got %v", *updated.LastError)
	}
}

func TestAgentStateSyncDrift(t *testing.T) {
	r, agent, node, cleanup := setupAgentTest(t)
	defer cleanup()
	ctx := context.Background()

	// desired active but agent missing -> add
	credAdd := "cred_drift_add"
	depAdd := &model.CredentialDeployment{CredentialID: credAdd, EndpointID: node.ID, AgentNodeID: agent.ID, DesiredState: "active", AppliedState: "unknown", Revision: "200"}
	// desired active but revision mismatch -> update
	credUpd := "cred_drift_upd"
	depUpd := &model.CredentialDeployment{CredentialID: credUpd, EndpointID: node.ID, AgentNodeID: agent.ID, DesiredState: "active", AppliedState: "active", Revision: "300"}
	// desired revoked but agent present active -> remove
	credRem := "cred_drift_rem"
	depRem := &model.CredentialDeployment{CredentialID: credRem, EndpointID: node.ID, AgentNodeID: agent.ID, DesiredState: "revoked", AppliedState: "active", Revision: "400"}
	for _, d := range []*model.CredentialDeployment{depAdd, depUpd, depRem} {
		if _, err := db.DB.NewInsert().Model(d).Exec(ctx); err != nil {
			t.Fatalf("insert %s: %v", d.CredentialID, err)
		}
	}

	body := map[string]interface{}{
		"items": []map[string]interface{}{
			// credAdd: present=false -> should trigger add
			{"credential_id": credAdd, "endpoint_id": node.ID, "applied_state": "active", "revision": "200", "present": false},
			// credUpd: wrong revision -> update
			{"credential_id": credUpd, "endpoint_id": node.ID, "applied_state": "active", "revision": "299", "present": true},
			// credRem: agent still active but desired revoked -> remove
			{"credential_id": credRem, "endpoint_id": node.ID, "applied_state": "active", "revision": "400", "present": true},
			// extra on agent with no desired -> remove
			{"credential_id": "cred_extra_unknown", "endpoint_id": node.ID, "applied_state": "active", "revision": "1", "present": true},
		},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/agent/v1/state/sync", bytes.NewReader(b))
	req.Header.Set("X-Agent-Token", *agent.ControlTokenCiphertext)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Drift       []map[string]interface{} `json:"drift"`
			Conflicts   []map[string]interface{} `json:"conflicts"`
			NextSyncSec int                      `json:"next_sync_sec"`
			ServerTime  string                   `json:"server_time"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Data.Drift) < 3 {
		t.Fatalf("expected at least 3 drift entries, got %d: %+v", len(resp.Data.Drift), resp.Data.Drift)
	}
	actions := map[string]int{}
	for _, d := range resp.Data.Drift {
		if a, ok := d["action"].(string); ok {
			actions[a]++
		}
	}
	if actions["add"] < 1 {
		t.Fatalf("expected at least 1 add drift, got %+v", actions)
	}
	if actions["remove"] < 1 {
		t.Fatalf("expected at least 1 remove drift, got %+v", actions)
	}
	if actions["update"] < 1 {
		t.Fatalf("expected at least 1 update drift, got %+v", actions)
	}
	if resp.Data.NextSyncSec != 30 {
		t.Fatalf("expected next_sync_sec 30, got %d", resp.Data.NextSyncSec)
	}
	// verify drift DB markers
	var upd model.CredentialDeployment
	if err := db.DB.NewSelect().Model(&upd).Where("credential_id = ? AND endpoint_id = ? AND agent_node_id = ?", credAdd, node.ID, agent.ID).Scan(ctx); err != nil {
		t.Fatalf("load add dep: %v", err)
	}
	if upd.LastSyncAt == nil {
		t.Fatalf("expected last_sync_at set for add drift")
	}
	if upd.LastError == nil || *upd.LastError == "" {
		t.Fatalf("expected last_error drift for missing present=false, got %+v", upd.LastError)
	}
}

func TestAgentStateSyncNotBound(t *testing.T) {
	r, agent, node, cleanup := setupAgentTest(t)
	defer cleanup()
	ctx := context.Background()

	// create another node without binding
	otherNode := &model.Node{ID: 999, Tag: "other-01", NodeName: "other", Protocol: "vless", ServerAddress: "5.5.5.5", ServerPort: 443, IsActive: true, Status: "active"}
	if _, err := db.DB.NewInsert().Model(otherNode).Exec(ctx); err != nil {
		t.Fatalf("insert other node: %v", err)
	}
	_ = node // keep bound node exists

	credID := "cred_notbound_001"
	body := map[string]interface{}{
		"items": []map[string]interface{}{
			{"credential_id": credID, "endpoint_id": otherNode.ID, "applied_state": "active", "revision": "1", "present": true},
		},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/agent/v1/state/sync", bytes.NewReader(b))
	req.Header.Set("X-Agent-Token", *agent.ControlTokenCiphertext)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Drift       []map[string]interface{} `json:"drift"`
			Conflicts   []map[string]interface{} `json:"conflicts"`
			NextSyncSec int                      `json:"next_sync_sec"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Data.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d: %+v", len(resp.Data.Conflicts), resp.Data.Conflicts)
	}
	if resp.Data.Conflicts[0]["reason"] != "not_bound" {
		t.Fatalf("expected reason not_bound, got %+v", resp.Data.Conflicts[0])
	}
	if len(resp.Data.Drift) != 0 {
		t.Fatalf("expected 0 drift for not_bound item, got %+v", resp.Data.Drift)
	}
}

func TestAgentGetNodeConfig(t *testing.T) {
	r, agent, _, cleanup := setupAgentTest(t)
	defer cleanup()

	ctx := context.Background()
	topoConfig := `{
		"inbounds": [
			{"tag": "in-443", "protocol": "vless", "port": 443, "security": "reality"}
		],
		"outbounds": [
			{"tag": "direct", "protocol": "freedom"}
		],
		"routing": {
			"domain_strategy": "AsIs",
			"rules": [
				{"rule_tag": "r1", "outbound_tag": "direct"}
			]
		}
	}`
	rev := "1710000000200"
	_, err := db.DB.NewUpdate().Model((*model.AgentNode)(nil)).
		Set("topology_config = ?", topoConfig).
		Set("config_revision = ?", rev).
		Where("id = ?", agent.ID).Exec(ctx)
	if err != nil {
		t.Fatalf("failed to update agent topo: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/agent/v1/config", nil)
	req.Header.Set("X-Agent-Token", *agent.ControlTokenCiphertext)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Revision  string            `json:"revision"`
			Inbounds  []json.RawMessage `json:"inbounds"`
			Outbounds []json.RawMessage `json:"outbounds"`
			Routing   json.RawMessage   `json:"routing"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}
	if resp.Data.Revision != rev {
		t.Fatalf("expected revision %s, got %s", rev, resp.Data.Revision)
	}
	if len(resp.Data.Inbounds) != 1 || len(resp.Data.Outbounds) != 1 {
		t.Fatalf("unexpected inbounds/outbounds count: %+v", resp.Data)
	}
	if len(resp.Data.Routing) == 0 {
		t.Fatalf("expected non-empty routing")
	}
}

func TestAgentGetNodeConfigEmpty(t *testing.T) {
	r, agent, _, cleanup := setupAgentTest(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/agent/v1/config", nil)
	req.Header.Set("X-Agent-Token", *agent.ControlTokenCiphertext)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Revision  string                 `json:"revision"`
			Inbounds  []json.RawMessage      `json:"inbounds"`
			Outbounds []json.RawMessage      `json:"outbounds"`
			Routing   map[string]interface{} `json:"routing"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}
	if resp.Data.Revision != "0" {
		t.Fatalf("expected revision 0, got %s", resp.Data.Revision)
	}
	if len(resp.Data.Inbounds) != 0 || len(resp.Data.Outbounds) != 0 {
		t.Fatalf("expected empty inbounds/outbounds, got %+v", resp.Data)
	}
	if resp.Data.Routing["domain_strategy"] != "AsIs" {
		t.Fatalf("expected default routing domain_strategy AsIs, got %v", resp.Data.Routing)
	}
}

func TestAgentReportHealth(t *testing.T) {
	r, agent, _, cleanup := setupAgentTest(t)
	defer cleanup()

	ctx := context.Background()
	body := map[string]interface{}{
		"agent_version":  "1.0.0",
		"xray_version":   "26.10.0",
		"hostname":       "line-hk-01",
		"overall_status": "degraded",
		"dimensions": map[string]string{
			"inbound":  "healthy",
			"user":     "healthy",
			"outbound": "degraded",
			"routing":  "healthy",
		},
		"anomalies": []map[string]interface{}{
			{
				"dimension":    "outbound",
				"resource_key": "landing-hk-01",
				"error_type":   "missing",
				"detail":       "outbound absent in xray runtime",
				"observed_at":  "2026-11-16T12:01:00Z",
			},
		},
		"observed_at": "2026-11-16T12:01:00Z",
	}

	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/agent/v1/health/report", bytes.NewReader(b))
	req.Header.Set("X-Agent-Token", *agent.ControlTokenCiphertext)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Accepted bool `json:"accepted"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if !resp.Data.Accepted {
		t.Fatalf("expected accepted: true")
	}

	// Verify agent_nodes snapshot
	var updatedAgent model.AgentNode
	if err := db.DB.NewSelect().Model(&updatedAgent).Where("id = ?", agent.ID).Scan(ctx); err != nil {
		t.Fatalf("load agent: %v", err)
	}
	if updatedAgent.OverallStatus != "degraded" {
		t.Fatalf("expected overall_status degraded, got %s", updatedAgent.OverallStatus)
	}
	if !strings.Contains(updatedAgent.HealthDimensions, `"outbound":"degraded"`) {
		t.Fatalf("expected health_dimensions with outbound degraded, got %s", updatedAgent.HealthDimensions)
	}

	// Verify agent_health_logs
	var logs []model.AgentHealthLog
	if err := db.DB.NewSelect().Model(&logs).Where("agent_node_id = ?", agent.ID).Scan(ctx); err != nil {
		t.Fatalf("load logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 health log, got %d", len(logs))
	}
	if logs[0].OverallStatus != "degraded" || logs[0].Hostname != "line-hk-01" {
		t.Fatalf("unexpected log entry: %+v", logs[0])
	}
}

func TestAgentReportHealthIdempotent(t *testing.T) {
	r, agent, _, cleanup := setupAgentTest(t)
	defer cleanup()

	ctx := context.Background()
	body := map[string]interface{}{
		"agent_version":  "1.0.0",
		"xray_version":   "26.10.0",
		"hostname":       "line-hk-01",
		"overall_status": "healthy",
		"dimensions": map[string]string{
			"inbound":  "healthy",
			"user":     "healthy",
			"outbound": "healthy",
			"routing":  "healthy",
		},
		"anomalies":   []map[string]interface{}{},
		"observed_at": "2026-11-16T12:00:00Z",
	}
	b, _ := json.Marshal(body)

	// Call 1 -> creates 1 log
	req1 := httptest.NewRequest("POST", "/api/agent/v1/health/report", bytes.NewReader(b))
	req1.Header.Set("X-Agent-Token", *agent.ControlTokenCiphertext)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w1.Code, w1.Body.String())
	}

	// Call 2 (same content) -> idempotent, should NOT create another log
	req2 := httptest.NewRequest("POST", "/api/agent/v1/health/report", bytes.NewReader(b))
	req2.Header.Set("X-Agent-Token", *agent.ControlTokenCiphertext)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}

	count, _ := db.DB.NewSelect().Model((*model.AgentHealthLog)(nil)).Where("agent_node_id = ?", agent.ID).Count(ctx)
	if count != 1 {
		t.Fatalf("expected exactly 1 log record after duplicate report, got %d", count)
	}

	// Call 3 (status changed to degraded) -> creates 2nd log
	body["overall_status"] = "degraded"
	body["dimensions"] = map[string]string{
		"inbound":  "healthy",
		"user":     "healthy",
		"outbound": "degraded",
		"routing":  "healthy",
	}
	b3, _ := json.Marshal(body)
	req3 := httptest.NewRequest("POST", "/api/agent/v1/health/report", bytes.NewReader(b3))
	req3.Header.Set("X-Agent-Token", *agent.ControlTokenCiphertext)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w3.Code, w3.Body.String())
	}

	countAfter, _ := db.DB.NewSelect().Model((*model.AgentHealthLog)(nil)).Where("agent_node_id = ?", agent.ID).Count(ctx)
	if countAfter != 2 {
		t.Fatalf("expected 2 logs after status change, got %d", countAfter)
	}
}

func TestAgentStateSyncV2Present(t *testing.T) {
	r, agent, node, cleanup := setupAgentTest(t)
	defer cleanup()
	ctx := context.Background()

	// 1. Desired active, agent missing -> drift missing (action: add)
	credMissing := "cred_v2_missing"
	depMissing := &model.CredentialDeployment{CredentialID: credMissing, EndpointID: node.ID, AgentNodeID: agent.ID, DesiredState: "active", AppliedState: "unknown", Revision: "200"}

	// 2. Desired active, agent present with old revision -> drift revision_mismatch (action: update)
	credRevMismatch := "cred_v2_rev_mismatch"
	depRevMismatch := &model.CredentialDeployment{CredentialID: credRevMismatch, EndpointID: node.ID, AgentNodeID: agent.ID, DesiredState: "active", AppliedState: "active", Revision: "300"}

	// 3. Desired revoked, agent present active -> drift state_mismatch (action: remove)
	credStateMismatch := "cred_v2_state_mismatch"
	depStateMismatch := &model.CredentialDeployment{CredentialID: credStateMismatch, EndpointID: node.ID, AgentNodeID: agent.ID, DesiredState: "revoked", AppliedState: "active", Revision: "400"}

	for _, d := range []*model.CredentialDeployment{depMissing, depRevMismatch, depStateMismatch} {
		if _, err := db.DB.NewInsert().Model(d).Exec(ctx); err != nil {
			t.Fatalf("insert %s: %v", d.CredentialID, err)
		}
	}

	// V2 format: last_revision + present list
	v2Req := map[string]interface{}{
		"last_revision": "1710000000300",
		"present": []map[string]interface{}{
			// credRevMismatch reported with revision 299
			{
				"credential_id": credRevMismatch,
				"endpoint_id":   node.ID,
				"revision":      "299",
				"applied_state": "active",
				"xray_user_key": "c_" + credRevMismatch + "@line-hk-01",
			},
			// credStateMismatch reported with active
			{
				"credential_id": credStateMismatch,
				"endpoint_id":   node.ID,
				"revision":      "400",
				"applied_state": "active",
				"xray_user_key": "c_" + credStateMismatch + "@line-hk-01",
			},
			// extra item present on agent but not desired
			{
				"credential_id": "cred_v2_extra",
				"endpoint_id":   node.ID,
				"revision":      "100",
				"applied_state": "active",
				"xray_user_key": "c_extra@line-hk-01",
			},
		},
	}

	b, _ := json.Marshal(v2Req)
	req := httptest.NewRequest("POST", "/api/agent/v1/state/sync", bytes.NewReader(b))
	req.Header.Set("X-Agent-Token", *agent.ControlTokenCiphertext)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Drift       []map[string]interface{} `json:"drift"`
			Conflicts   []map[string]interface{} `json:"conflicts"`
			NextSyncSec int                      `json:"next_sync_sec"`
			ServerTime  string                   `json:"server_time"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if resp.Data.NextSyncSec != 30 {
		t.Fatalf("expected next_sync_sec 30, got %d", resp.Data.NextSyncSec)
	}

	reasons := map[string]int{}
	actions := map[string]int{}
	for _, d := range resp.Data.Drift {
		if r, ok := d["reason"].(string); ok {
			reasons[r]++
		}
		if a, ok := d["action"].(string); ok {
			actions[a]++
		}
	}

	// Verify normalized reasons: missing, extra, revision_mismatch, state_mismatch
	if reasons["missing"] != 1 {
		t.Fatalf("expected 1 missing drift, got %+v", reasons)
	}
	if reasons["extra"] != 1 {
		t.Fatalf("expected 1 extra drift, got %+v", reasons)
	}
	if reasons["revision_mismatch"] != 1 {
		t.Fatalf("expected 1 revision_mismatch drift, got %+v", reasons)
	}
	if reasons["state_mismatch"] != 1 {
		t.Fatalf("expected 1 state_mismatch drift, got %+v", reasons)
	}

	if actions["add"] != 1 {
		t.Fatalf("expected 1 add action, got %+v", actions)
	}
	if actions["remove"] != 2 { // state_mismatch (revoked) + extra
		t.Fatalf("expected 2 remove actions, got %+v", actions)
	}
	if actions["update"] != 1 {
		t.Fatalf("expected 1 update action, got %+v", actions)
	}
}

func TestAgentHeartbeatEndpointProjection(t *testing.T) {
	r, agent, _, cleanup := setupAgentTest(t)
	defer cleanup()

	ctx := context.Background()
	topoConfig := `{
		"inbounds": [
			{
				"tag": "in-8443",
				"protocol": "trojan",
				"port": 8443,
				"network": "ws",
				"security": "tls",
				"server_names": ["hk-node.example.com"]
			}
		]
	}`
	_, _ = db.DB.NewUpdate().Model((*model.AgentNode)(nil)).
		Set("topology_config = ?", topoConfig).
		Where("id = ?", agent.ID).Exec(ctx)

	hbBody := map[string]interface{}{
		"agent_version": "1.0.0",
		"xray_version":  "1.8.24",
		"hostname":      "edge-hk-02.example.com",
		"capabilities":  []string{"xray.trojan"},
		"endpoints": []map[string]interface{}{
			{
				"endpoint_id": 0,
				"inbound_tag": "in-8443",
				"ready":       true,
			},
		},
	}
	b, _ := json.Marshal(hbBody)
	req := httptest.NewRequest("POST", "/api/agent/v1/heartbeat", bytes.NewReader(b))
	req.Header.Set("X-Agent-Token", *agent.ControlTokenCiphertext)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify Node was auto-projected
	var projectedNode model.Node
	err := db.DB.NewSelect().Model(&projectedNode).Where("inbound_tag = ?", "in-8443").Scan(ctx)
	if err != nil {
		t.Fatalf("expected projected node in DB: %v", err)
	}
	if projectedNode.Protocol != "trojan" || projectedNode.ServerPort != 8443 || projectedNode.Security != "tls" {
		t.Fatalf("unexpected node config: %+v", projectedNode)
	}
	if projectedNode.ServerAddress != "edge-hk-02.example.com" {
		t.Fatalf("expected ServerAddress edge-hk-02.example.com, got %s", projectedNode.ServerAddress)
	}

	// Verify EndpointAgentBinding was created
	var binding model.EndpointAgentBinding
	err = db.DB.NewSelect().Model(&binding).
		Where("endpoint_id = ? AND agent_node_id = ?", projectedNode.ID, agent.ID).
		Scan(ctx)
	if err != nil {
		t.Fatalf("expected binding created: %v", err)
	}
	if binding.Status != "active" || binding.LastSyncAt == nil {
		t.Fatalf("unexpected binding state: %+v", binding)
	}
}

func TestAgentStateSyncV1Compat(t *testing.T) {
	r, agent, node, cleanup := setupAgentTest(t)
	defer cleanup()
	ctx := context.Background()

	// 1. Desired active, agent reports present: false -> drift missing (action: add)
	credMissing := "cred_v1_missing"
	depMissing := &model.CredentialDeployment{CredentialID: credMissing, EndpointID: node.ID, AgentNodeID: agent.ID, DesiredState: "active", AppliedState: "unknown", Revision: "200"}

	// 2. Desired active, agent reports old revision -> drift revision_mismatch (action: update)
	credRevMismatch := "cred_v1_rev_mismatch"
	depRevMismatch := &model.CredentialDeployment{CredentialID: credRevMismatch, EndpointID: node.ID, AgentNodeID: agent.ID, DesiredState: "active", AppliedState: "active", Revision: "300"}

	// 3. Desired revoked, agent reports active -> drift state_mismatch (action: remove)
	credStateMismatch := "cred_v1_state_mismatch"
	depStateMismatch := &model.CredentialDeployment{CredentialID: credStateMismatch, EndpointID: node.ID, AgentNodeID: agent.ID, DesiredState: "revoked", AppliedState: "active", Revision: "400"}

	for _, d := range []*model.CredentialDeployment{depMissing, depRevMismatch, depStateMismatch} {
		if _, err := db.DB.NewInsert().Model(d).Exec(ctx); err != nil {
			t.Fatalf("insert %s: %v", d.CredentialID, err)
		}
	}

	// Legacy V1 format: "items" array with present boolean flags
	v1Req := map[string]interface{}{
		"items": []map[string]interface{}{
			// credMissing: present=false -> missing
			{
				"credential_id": credMissing,
				"endpoint_id":   node.ID,
				"revision":      "200",
				"applied_state": "active",
				"present":       false,
			},
			// credRevMismatch: present=true but revision 299 != 300 -> revision_mismatch
			{
				"credential_id": credRevMismatch,
				"endpoint_id":   node.ID,
				"revision":      "299",
				"applied_state": "active",
				"present":       true,
			},
			// credStateMismatch: desired revoked but present active -> state_mismatch
			{
				"credential_id": credStateMismatch,
				"endpoint_id":   node.ID,
				"revision":      "400",
				"applied_state": "active",
				"present":       true,
			},
			// extra on agent
			{
				"credential_id": "cred_v1_extra",
				"endpoint_id":   node.ID,
				"revision":      "100",
				"applied_state": "active",
				"present":       true,
			},
		},
	}

	b, _ := json.Marshal(v1Req)
	req := httptest.NewRequest("POST", "/api/agent/v1/state/sync", bytes.NewReader(b))
	req.Header.Set("X-Agent-Token", *agent.ControlTokenCiphertext)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Drift       []map[string]interface{} `json:"drift"`
			Conflicts   []map[string]interface{} `json:"conflicts"`
			NextSyncSec int                      `json:"next_sync_sec"`
			ServerTime  string                   `json:"server_time"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if resp.Data.NextSyncSec != 30 {
		t.Fatalf("expected next_sync_sec 30, got %d", resp.Data.NextSyncSec)
	}

	reasons := map[string]int{}
	actions := map[string]int{}
	for _, d := range resp.Data.Drift {
		if r, ok := d["reason"].(string); ok {
			reasons[r]++
		}
		if a, ok := d["action"].(string); ok {
			actions[a]++
		}
	}

	// Verify normalized reasons in V1 legacy mode
	if reasons["missing"] != 1 {
		t.Fatalf("expected 1 missing drift, got %+v", reasons)
	}
	if reasons["extra"] != 1 {
		t.Fatalf("expected 1 extra drift, got %+v", reasons)
	}
	if reasons["revision_mismatch"] != 1 {
		t.Fatalf("expected 1 revision_mismatch drift, got %+v", reasons)
	}
	if reasons["state_mismatch"] != 1 {
		t.Fatalf("expected 1 state_mismatch drift, got %+v", reasons)
	}

	if actions["add"] != 1 {
		t.Fatalf("expected 1 add action, got %+v", actions)
	}
	if actions["remove"] != 2 {
		t.Fatalf("expected 2 remove actions, got %+v", actions)
	}
	if actions["update"] != 1 {
		t.Fatalf("expected 1 update action, got %+v", actions)
	}
}

func TestAgentHeartbeatEndpointProjectionMultiAgentIsolation(t *testing.T) {
	r, agent1, _, cleanup := setupAgentTest(t)
	defer cleanup()
	ctx := context.Background()

	// Create Agent 2
	token2 := "agt_tok_agent_002"
	now := time.Now().UTC()
	agent2 := &model.AgentNode{
		ID:                     "ag_test_002",
		Name:                   "agent-2",
		ControlTokenCiphertext: &token2,
		Status:                 "offline",
		TopologyConfig: `{
			"inbounds": [
				{"tag": "in-shared-443", "protocol": "vless", "port": 443}
			]
		}`,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := db.DB.NewInsert().Model(agent2).Exec(ctx); err != nil {
		t.Fatalf("insert agent2: %v", err)
	}

	// Update Agent 1 topology
	_, _ = db.DB.NewUpdate().Model((*model.AgentNode)(nil)).
		Set("topology_config = ?", `{"inbounds": [{"tag": "in-shared-443", "protocol": "vless", "port": 443}]}`).
		Where("id = ?", agent1.ID).Exec(ctx)

	// Agent 1 heartbeat
	hb1 := map[string]interface{}{
		"agent_version": "1.0.0",
		"hostname":      "host-agent-1.example.com",
		"endpoints": []map[string]interface{}{
			{"endpoint_id": 0, "inbound_tag": "in-shared-443", "ready": true},
		},
	}
	b1, _ := json.Marshal(hb1)
	req1 := httptest.NewRequest("POST", "/api/agent/v1/heartbeat", bytes.NewReader(b1))
	req1.Header.Set("X-Agent-Token", *agent1.ControlTokenCiphertext)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("hb1 failed %d %s", w1.Code, w1.Body.String())
	}

	// Agent 2 heartbeat with identical inbound_tag
	hb2 := map[string]interface{}{
		"agent_version": "1.0.0",
		"hostname":      "host-agent-2.example.com",
		"endpoints": []map[string]interface{}{
			{"endpoint_id": 0, "inbound_tag": "in-shared-443", "ready": true},
		},
	}
	b2, _ := json.Marshal(hb2)
	req2 := httptest.NewRequest("POST", "/api/agent/v1/heartbeat", bytes.NewReader(b2))
	req2.Header.Set("X-Agent-Token", token2)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("hb2 failed %d %s", w2.Code, w2.Body.String())
	}

	// Both agents must have their own separate projected node
	var binding1 model.EndpointAgentBinding
	if err := db.DB.NewSelect().Model(&binding1).
		Where("agent_node_id = ?", agent1.ID).
		Where("endpoint_id IN (SELECT id FROM node WHERE inbound_tag = ?)", "in-shared-443").
		Scan(ctx); err != nil {
		t.Fatalf("agent1 binding missing: %v", err)
	}
	var binding2 model.EndpointAgentBinding
	if err := db.DB.NewSelect().Model(&binding2).
		Where("agent_node_id = ?", agent2.ID).
		Where("endpoint_id IN (SELECT id FROM node WHERE inbound_tag = ?)", "in-shared-443").
		Scan(ctx); err != nil {
		t.Fatalf("agent2 binding missing: %v", err)
	}

	if binding1.EndpointID == binding2.EndpointID {
		t.Fatalf("isolation violation: agent1 and agent2 projected the same endpoint ID %d", binding1.EndpointID)
	}

	var node1, node2 model.Node
	_ = db.DB.NewSelect().Model(&node1).Where("id = ?", binding1.EndpointID).Scan(ctx)
	_ = db.DB.NewSelect().Model(&node2).Where("id = ?", binding2.EndpointID).Scan(ctx)

	if node1.ServerAddress != "host-agent-1.example.com" {
		t.Fatalf("expected node1 host host-agent-1.example.com, got %s", node1.ServerAddress)
	}
	if node2.ServerAddress != "host-agent-2.example.com" {
		t.Fatalf("expected node2 host host-agent-2.example.com, got %s", node2.ServerAddress)
	}
}


