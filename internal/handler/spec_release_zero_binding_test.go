package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"noder/internal/db"
	"noder/internal/model"
)

// Spec §7.1.1 + §5 + domain_model_er §6: release 必须产生 deployment，0 binding 不得静默 200
func TestSpec_7_1_1_Release_ZeroBinding(t *testing.T) {
	r, cleanup := setupServiceTest(t)
	defer cleanup()

	// endpoint 无任何 agent binding
	node := &model.Node{Tag: "spec-7.1.1-tuic", Protocol: "tuic", ServerAddress: "127.0.0.1", ServerPort: 443, IsActive: true, Status: "active"}
	if _, err := db.DB.NewInsert().Model(node).Exec(t.Context()); err != nil {
		t.Fatalf("insert node: %v", err)
	}
	// activate -> pending_activation (landing 需绑定，否则 release 应 409 BINDING_MISSING)
	body, _ := json.Marshal(map[string]interface{}{
		"external_id": "spec_u_711", "endpoint_id": node.ID, "format": "landing", "exit_route_tag": "exit-spec-01", "duration_seconds": 86400, "traffic_quota_bytes": 1073741824, "order_item_id": "spec_oi_711",
	})
	req := httptest.NewRequest("POST", "/api/service/subscriptions/activate", bytes.NewReader(body))
	req.Header.Set("X-Service-Token", "test-service-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("activate %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			SubscriptionID string `json:"subscription_id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	// release：当前实现会 200 但 0 deployment -> 本测试期望失败，驱动修复
	rel := httptest.NewRequest("POST", "/api/service/subscriptions/"+resp.Data.SubscriptionID+"/release", nil)
	rel.Header.Set("X-Service-Token", "test-service-token")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, rel)

	c, _ := db.DB.NewSelect().Model((*model.CredentialDeployment)(nil)).Where("endpoint_id=?", node.ID).Count(t.Context())
	if rw.Code != http.StatusConflict {
		t.Fatalf("SPEC §7.1.1: landing release with 0 binding must return 409 BINDING_MISSING, got %d body=%s deployments=%d", rw.Code, rw.Body.String(), c)
	}
	var respBody map[string]interface{}
	_ = json.Unmarshal(rw.Body.Bytes(), &respBody)
	if respBody["error"] != "BINDING_MISSING" {
		t.Fatalf("expected BINDING_MISSING, got %v", respBody)
	}
	if c != 0 {
		t.Fatalf("expected 0 deployments, got %d", c)
	}
}
