package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"noder/internal/db"
	"noder/internal/model"
)

func TestServiceSubscriptionLifecycle(t *testing.T) {
	r, cleanup := setupServiceTest(t)
	defer cleanup()

	ctx := context.Background()

	// 1. Create a landing (vless) mother node
	landingNode := &model.Node{
		Tag:           "US-LA-Vless-01",
		NodeName:      "US-LA-Vless-01",
		Protocol:      "vless",
		ServerAddress: "198.51.100.1",
		ServerPort:    443,
		IsActive:      true,
		Status:        "active",
	}
	if _, err := db.DB.NewInsert().Model(landingNode).Exec(ctx); err != nil {
		t.Fatalf("failed to insert landing node: %v", err)
	}

	// Also create an agent node and binding for landingNode
	now := time.Now().UTC()
	agentNode := &model.AgentNode{
		ID:              "agent-uuid-001",
		Name:            "agent-la-01",
		Status:          "online",
		LastHeartbeatAt: &now,
	}
	if _, err := db.DB.NewInsert().Model(agentNode).Exec(ctx); err != nil {
		t.Fatalf("failed to insert agent node: %v", err)
	}
	binding := &model.EndpointAgentBinding{
		EndpointID:  landingNode.ID,
		AgentNodeID: agentNode.ID,
		Status:      "active",
	}
	if _, err := db.DB.NewInsert().Model(binding).Exec(ctx); err != nil {
		t.Fatalf("failed to insert endpoint agent binding: %v", err)
	}

	// 2. Activate subscription #1 (no tag_override passed)
	actReq1 := map[string]interface{}{
		"external_id":         "mall_u101",
		"endpoint_id":         landingNode.ID,
		"format":              "landing",
		"exit_route_tag":      "exit-us-la-01",
		"duration_seconds":   86400 * 30,
		"traffic_quota_bytes": 107374182400,
		"order_item_id":       "oi_sub_001",
	}
	body1, _ := json.Marshal(actReq1)
	req := httptest.NewRequest("POST", "/api/service/subscriptions/activate", bytes.NewReader(body1))
	req.Header.Set("X-Service-Token", "test-service-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("activate #1 failed: %d %s", w.Code, w.Body.String())
	}
	var resp1 struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp1)
	if resp1.Code != 0 {
		t.Fatalf("expected code 0, got %d", resp1.Code)
	}
	if resp1.Data["delivered"] != false {
		t.Fatalf("expected delivered=false on activate, got %v", resp1.Data["delivered"])
	}
	if resp1.Data["status"] != model.AccessGrantStatusPendingActivation {
		t.Fatalf("expected pending_activation, got %v", resp1.Data["status"])
	}
	subID1 := resp1.Data["subscription_id"].(string)

	// Check grant #1 in DB: DisplayName should NOT be email, TagOverride should be nil (1st subscription: no suffix)
	var g1 model.AccessGrant
	if err := db.DB.NewSelect().Model(&g1).Where("id = ?", subID1).Scan(ctx); err != nil {
		t.Fatalf("grant #1 not found: %v", err)
	}
	if g1.DisplayName != nil && *g1.DisplayName != "" {
		t.Fatalf("expected g1.DisplayName to be nil/empty, got %q", *g1.DisplayName)
	}
	if g1.TagOverride != nil {
		t.Fatalf("expected g1.TagOverride to be nil for first subscription, got %q", *g1.TagOverride)
	}

	// 3. Activate subscription #2 for same user, same endpoint and same exit_route_tag (Scheme A test)
	actReq2 := map[string]interface{}{
		"external_id":         "mall_u101",
		"endpoint_id":         landingNode.ID,
		"format":              "landing",
		"exit_route_tag":      "exit-us-la-01",
		"duration_seconds":   86400 * 30,
		"traffic_quota_bytes": 107374182400,
		"order_item_id":       "oi_sub_002",
	}
	body2, _ := json.Marshal(actReq2)
	req2 := httptest.NewRequest("POST", "/api/service/subscriptions/activate", bytes.NewReader(body2))
	req2.Header.Set("X-Service-Token", "test-service-token")
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("activate #2 failed: %d %s", w2.Code, w2.Body.String())
	}
	var resp2 struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	subID2 := resp2.Data["subscription_id"].(string)

	// Check grant #2 in DB: TagOverride should be "US-LA-Vless-01 #2"
	var g2 model.AccessGrant
	if err := db.DB.NewSelect().Model(&g2).Where("id = ?", subID2).Scan(ctx); err != nil {
		t.Fatalf("grant #2 not found: %v", err)
	}
	if g2.TagOverride == nil || *g2.TagOverride != "US-LA-Vless-01 #2" {
		t.Fatalf("expected TagOverride 'US-LA-Vless-01 #2', got %v", g2.TagOverride)
	}

	// 4. Test ReleaseSubscription
	relReq := httptest.NewRequest("POST", fmt.Sprintf("/api/service/subscriptions/%s/release", subID1), nil)
	relReq.Header.Set("X-Service-Token", "test-service-token")
	relW := httptest.NewRecorder()
	r.ServeHTTP(relW, relReq)

	if relW.Code != http.StatusOK {
		t.Fatalf("release failed: %d %s", relW.Code, relW.Body.String())
	}
	var relResp struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	_ = json.Unmarshal(relW.Body.Bytes(), &relResp)
	if relResp.Data["delivered"] != true || relResp.Data["status"] != "active" {
		t.Fatalf("expected delivered=true, status=active after release, got %v", relResp.Data)
	}

	// Idempotency check: release again should succeed with 200
	relReqAgain := httptest.NewRequest("POST", fmt.Sprintf("/api/service/subscriptions/%s/release", subID1), nil)
	relReqAgain.Header.Set("X-Service-Token", "test-service-token")
	relWAgain := httptest.NewRecorder()
	r.ServeHTTP(relWAgain, relReqAgain)
	if relWAgain.Code != http.StatusOK {
		t.Fatalf("expected 200 on repeat release, got %d", relWAgain.Code)
	}

	// Check credential deployment created
	var dep model.CredentialDeployment
	if err := db.DB.NewSelect().Model(&dep).Where("credential_id = ?", g1.CredentialID).Scan(ctx); err != nil {
		t.Fatalf("expected credential deployment created on release: %v", err)
	}
	if dep.DesiredState != model.CredentialStatusActive {
		t.Fatalf("expected desired_state active, got %s", dep.DesiredState)
	}

	// 5. Test GetSubscription
	getReq := httptest.NewRequest("GET", fmt.Sprintf("/api/service/subscriptions/%s", subID1), nil)
	getReq.Header.Set("X-Service-Token", "test-service-token")
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("get subscription failed: %d %s", getW.Code, getW.Body.String())
	}
	var getResp struct {
		Code int `json:"code"`
		Data struct {
			Status    string                 `json:"status"`
			ExpiresAt string                 `json:"expires_at"`
			Traffic   map[string]interface{} `json:"traffic"`
			SubURLs   map[string]string      `json:"sub_urls"`
		} `json:"data"`
	}
	_ = json.Unmarshal(getW.Body.Bytes(), &getResp)
	if getResp.Data.Status != "active" {
		t.Fatalf("expected active status, got %s", getResp.Data.Status)
	}
	if getResp.Data.Traffic["quota_bytes"].(float64) != 107374182400 {
		t.Fatalf("unexpected quota_bytes: %v", getResp.Data.Traffic)
	}

	// 6. Test ExtendSubscription
	extReqData := map[string]interface{}{
		"duration_seconds":   86400 * 10,
		"traffic_quota_bytes": 53687091200,
		"order_item_id":       "oi_renew_001",
	}
	extBody, _ := json.Marshal(extReqData)
	extReq := httptest.NewRequest("POST", fmt.Sprintf("/api/service/subscriptions/%s/extend", subID1), bytes.NewReader(extBody))
	extReq.Header.Set("X-Service-Token", "test-service-token")
	extReq.Header.Set("Content-Type", "application/json")
	extW := httptest.NewRecorder()
	r.ServeHTTP(extW, extReq)

	if extW.Code != http.StatusOK {
		t.Fatalf("extend failed: %d %s", extW.Code, extW.Body.String())
	}
	var extResp struct {
		Code int `json:"code"`
		Data struct {
			Status  string                 `json:"status"`
			Traffic map[string]interface{} `json:"traffic"`
		} `json:"data"`
	}
	_ = json.Unmarshal(extW.Body.Bytes(), &extResp)
	expectedTotalQuota := float64(107374182400 + 53687091200)
	if extResp.Data.Traffic["quota_bytes"].(float64) != expectedTotalQuota {
		t.Fatalf("expected quota %f, got %v", expectedTotalQuota, extResp.Data.Traffic["quota_bytes"])
	}

	// Extend idempotency on order_item_id
	extReqReplay := httptest.NewRequest("POST", fmt.Sprintf("/api/service/subscriptions/%s/extend", subID1), bytes.NewReader(extBody))
	extReqReplay.Header.Set("X-Service-Token", "test-service-token")
	extReqReplay.Header.Set("Content-Type", "application/json")
	extWReplay := httptest.NewRecorder()
	r.ServeHTTP(extWReplay, extReqReplay)
	if extWReplay.Code != http.StatusOK {
		t.Fatalf("expected 200 on extend replay, got %d", extWReplay.Code)
	}

	// 7. Test QuotaSubscription
	quotaReqData := map[string]interface{}{
		"delta_bytes":   10737418240,
		"order_item_id": "oi_quota_001",
	}
	quotaBody, _ := json.Marshal(quotaReqData)
	quotaReq := httptest.NewRequest("POST", fmt.Sprintf("/api/service/subscriptions/%s/quota", subID1), bytes.NewReader(quotaBody))
	quotaReq.Header.Set("X-Service-Token", "test-service-token")
	quotaReq.Header.Set("Content-Type", "application/json")
	quotaW := httptest.NewRecorder()
	r.ServeHTTP(quotaW, quotaReq)

	if quotaW.Code != http.StatusOK {
		t.Fatalf("quota failed: %d %s", quotaW.Code, quotaW.Body.String())
	}
	var quotaResp struct {
		Code int `json:"code"`
		Data struct {
			SubscriptionID    string `json:"subscription_id"`
			Status            string `json:"status"`
			TrafficQuotaBytes int64  `json:"traffic_quota_bytes"`
		} `json:"data"`
	}
	_ = json.Unmarshal(quotaW.Body.Bytes(), &quotaResp)
	if quotaResp.Data.TrafficQuotaBytes != int64(expectedTotalQuota)+10737418240 {
		t.Fatalf("expected quota %d, got %d", int64(expectedTotalQuota)+10737418240, quotaResp.Data.TrafficQuotaBytes)
	}

	// 8. Test RevokeSubscription
	revReqData := map[string]interface{}{"reason": "refund"}
	revBody, _ := json.Marshal(revReqData)
	revReq := httptest.NewRequest("POST", fmt.Sprintf("/api/service/subscriptions/%s/revoke", subID1), bytes.NewReader(revBody))
	revReq.Header.Set("X-Service-Token", "test-service-token")
	revReq.Header.Set("Content-Type", "application/json")
	revW := httptest.NewRecorder()
	r.ServeHTTP(revW, revReq)

	if revW.Code != http.StatusOK {
		t.Fatalf("revoke failed: %d %s", revW.Code, revW.Body.String())
	}
	// Revoke repeat is idempotent 200
	revW2 := httptest.NewRecorder()
	revReq2 := httptest.NewRequest("POST", fmt.Sprintf("/api/service/subscriptions/%s/revoke", subID1), bytes.NewReader(revBody))
	revReq2.Header.Set("X-Service-Token", "test-service-token")
	r.ServeHTTP(revW2, revReq2)
	if revW2.Code != http.StatusOK {
		t.Fatalf("expected 200 on repeat revoke, got %d", revW2.Code)
	}

	// Verify grant and credential status are revoked in DB
	var gRevoked model.AccessGrant
	_ = db.DB.NewSelect().Model(&gRevoked).Where("id = ?", subID1).Scan(ctx)
	if gRevoked.Status != model.AccessGrantStatusRevoked {
		t.Fatalf("expected grant revoked, got %s", gRevoked.Status)
	}

	// 9. Scheme A: Now buy a 3rd subscription for same user, endpoint and exit
	// Even though #1 is revoked, count of historical grants is 2 (#1, #2), so next must be #3 ("永不重用")
	actReq3 := map[string]interface{}{
		"external_id":         "mall_u101",
		"endpoint_id":         landingNode.ID,
		"format":              "landing",
		"exit_route_tag":      "exit-us-la-01",
		"duration_seconds":   86400 * 30,
		"traffic_quota_bytes": 107374182400,
		"order_item_id":       "oi_sub_003",
	}
	body3, _ := json.Marshal(actReq3)
	req3 := httptest.NewRequest("POST", "/api/service/subscriptions/activate", bytes.NewReader(body3))
	req3.Header.Set("X-Service-Token", "test-service-token")
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("activate #3 failed: %d %s", w3.Code, w3.Body.String())
	}
	var resp3 struct {
		Code int                    `json:"code"`
		Data map[string]interface{} `json:"data"`
	}
	_ = json.Unmarshal(w3.Body.Bytes(), &resp3)
	subID3 := resp3.Data["subscription_id"].(string)

	var g3 model.AccessGrant
	if err := db.DB.NewSelect().Model(&g3).Where("id = ?", subID3).Scan(ctx); err != nil {
		t.Fatalf("grant #3 not found: %v", err)
	}
	if g3.TagOverride == nil || *g3.TagOverride != "US-LA-Vless-01 #3" {
		t.Fatalf("expected TagOverride 'US-LA-Vless-01 #3' (永不重用), got %v", g3.TagOverride)
	}
}

func TestSocks5SubscriptionRetrieval(t *testing.T) {
	r, cleanup := setupServiceTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create socks5 mother node
	countryUS := "US"
	s5Node := &model.Node{
		Tag:           "US-Socks-Canonical-01",
		NodeName:      "US-Socks-Canonical-01",
		Protocol:      "socks5",
		ServerAddress: "198.51.100.99",
		ServerPort:    1080,
		Country:       &countryUS,
		IsActive:      true,
		Status:        "active",
	}
	if _, err := db.DB.NewInsert().Model(s5Node).Exec(ctx); err != nil {
		t.Fatalf("failed to insert socks5 node: %v", err)
	}

	// Activate socks5 subscription
	actReq := map[string]interface{}{
		"external_id":         "mall_u202",
		"endpoint_id":         s5Node.ID,
		"format":              "socks",
		"duration_seconds":   86400 * 30,
		"traffic_quota_bytes": 0, // unlimited
		"order_item_id":       "oi_socks_001",
	}
	actBody, _ := json.Marshal(actReq)
	req := httptest.NewRequest("POST", "/api/service/subscriptions/activate", bytes.NewReader(actBody))
	req.Header.Set("X-Service-Token", "test-service-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("activate socks5 failed: %d %s", w.Code, w.Body.String())
	}
	var actResp struct {
		Code int `json:"code"`
		Data struct {
			SubscriptionID string `json:"subscription_id"`
			UserID         int64  `json:"user_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &actResp)
	subID := actResp.Data.SubscriptionID

	// Release it
	relReq := httptest.NewRequest("POST", fmt.Sprintf("/api/service/subscriptions/%s/release", subID), nil)
	relReq.Header.Set("X-Service-Token", "test-service-token")
	relW := httptest.NewRecorder()
	r.ServeHTTP(relW, relReq)
	if relW.Code != http.StatusOK {
		t.Fatalf("release socks5 failed: %d %s", relW.Code, relW.Body.String())
	}

	// Fetch user token
	var u model.User
	if err := db.DB.NewSelect().Model(&u).Where("id = ?", actResp.Data.UserID).Scan(ctx); err != nil {
		t.Fatalf("user not found: %v", err)
	}

	// 1. Test GetGostNodeAuthList includes the canonical credential
	gostReq := httptest.NewRequest("GET", fmt.Sprintf("/api/service/socks5/nodes/%d/auth-list", s5Node.ID), nil)
	gostReq.Header.Set("X-Service-Token", "test-service-token")
	gostW := httptest.NewRecorder()
	r.ServeHTTP(gostW, gostReq)
	if gostW.Code != http.StatusOK {
		t.Fatalf("gost auth list failed: %d %s", gostW.Code, gostW.Body.String())
	}
	gostOut := gostW.Body.String()
	expectedUnamePrefix := fmt.Sprintf("u%d_", u.ID)
	if !strings.Contains(gostOut, expectedUnamePrefix) {
		t.Fatalf("expected gost auth list to contain canonical username prefix %s, got: %s", expectedUnamePrefix, gostOut)
	}

	// 2. Test GetUserSocks5List returns the canonical socks5 proxy
	userS5Req := httptest.NewRequest("GET", fmt.Sprintf("/api/service/users/%s/socks5/list?format=text", u.Token), nil)
	userS5Req.Header.Set("X-Service-Token", "test-service-token")
	userS5W := httptest.NewRecorder()
	r.ServeHTTP(userS5W, userS5Req)
	if userS5W.Code != http.StatusOK {
		t.Fatalf("user socks5 list failed: %d %s", userS5W.Code, userS5W.Body.String())
	}
	userS5Out := userS5W.Body.String()
	expectedLinePrefix := fmt.Sprintf("198.51.100.99:1080:%s", expectedUnamePrefix)
	if !strings.Contains(userS5Out, expectedLinePrefix) {
		t.Fatalf("expected socks5 text list to contain %s, got: %s", expectedLinePrefix, userS5Out)
	}
}
