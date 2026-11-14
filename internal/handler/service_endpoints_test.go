package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"noder/internal/db"
	"noder/internal/model"
)

func TestListServiceEndpoints(t *testing.T) {
	r, cleanup := setupServiceTest(t)
	defer cleanup()

	ctx := t.Context()
	baselineCount, err := db.DB.NewSelect().Model((*model.Node)(nil)).Count(ctx)
	if err != nil {
		t.Fatalf("count existing endpoints: %v", err)
	}
	noBinding := &model.Node{
		Tag:           "tuic-01",
		NodeName:      "TUIC 高速专线 01",
		Protocol:      "tuic",
		ServerAddress: "192.0.2.1",
		ServerPort:    443,
		IsActive:      true,
		Status:        "active",
		SortOrder:     1000000,
	}
	bound := &model.Node{
		Tag:           "vless-02",
		NodeName:      "VLESS REALITY 专线 02",
		Protocol:      "vless",
		ServerAddress: "192.0.2.2",
		ServerPort:    443,
		IsActive:      true,
		Status:        "active",
		SortOrder:     1000001,
	}
	inactive := &model.Node{
		Tag:           "disabled-03",
		NodeName:      "Disabled endpoint",
		Protocol:      "vless",
		ServerAddress: "192.0.2.3",
		ServerPort:    443,
		IsActive:      false,
		Status:        "disabled",
		SortOrder:     1000002,
	}
	for _, node := range []*model.Node{noBinding, bound, inactive} {
		if _, err := db.DB.NewInsert().Model(node).Exec(ctx); err != nil {
			t.Fatalf("insert endpoint: %v", err)
		}
	}
	if _, err := db.DB.NewUpdate().Model(inactive).Set("is_active = ?", false).Where("id = ?", inactive.ID).Exec(ctx); err != nil {
		t.Fatalf("disable endpoint: %v", err)
	}

	now := time.Now().UTC()
	onlineAgent := &model.AgentNode{
		ID:              "ag_hk_01",
		Name:            "香港线路机 01",
		Status:          "online",
		LastHeartbeatAt: &now,
	}
	offlineAgent := &model.AgentNode{
		ID:     "ag_hk_02",
		Name:   "香港线路机 02",
		Status: "offline",
	}
	for _, agent := range []*model.AgentNode{onlineAgent, offlineAgent} {
		if _, err := db.DB.NewInsert().Model(agent).Exec(ctx); err != nil {
			t.Fatalf("insert agent: %v", err)
		}
	}

	for _, binding := range []*model.EndpointAgentBinding{
		{EndpointID: bound.ID, AgentNodeID: onlineAgent.ID, Status: "active"},
		{EndpointID: bound.ID, AgentNodeID: offlineAgent.ID, Status: "active"},
		// Inactive bindings must not be returned or counted.
		{EndpointID: noBinding.ID, AgentNodeID: offlineAgent.ID, Status: "disabled"},
	} {
		if _, err := db.DB.NewInsert().Model(binding).Exec(ctx); err != nil {
			t.Fatalf("insert binding: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/service/endpoints", nil)
	req.Header.Set("X-Service-Token", "test-service-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		Code int `json:"code"`
		Data []struct {
			ID           int64                  `json:"id"`
			Tag          string                 `json:"tag"`
			NodeName     string                 `json:"node_name"`
			Protocol     string                 `json:"protocol"`
			IsActive     bool                   `json:"is_active"`
			Status       string                 `json:"status"`
			BindingCount int                    `json:"binding_count"`
			BoundAgents  []serviceEndpointAgent `json:"bound_agents"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != http.StatusOK {
		t.Fatalf("expected response code 200, got %d", response.Code)
	}
	if len(response.Data) != baselineCount+3 {
		t.Fatalf("expected %d endpoints, got %d", baselineCount+3, len(response.Data))
	}
	testEndpoints := response.Data[len(response.Data)-3:]
	if got := []int64{testEndpoints[0].ID, testEndpoints[1].ID, testEndpoints[2].ID}; !reflect.DeepEqual(got, []int64{noBinding.ID, bound.ID, inactive.ID}) {
		t.Fatalf("unexpected endpoint ordering: %v", got)
	}

	first := testEndpoints[0]
	if first.Tag != noBinding.Tag || first.NodeName != noBinding.NodeName ||
		first.Protocol != noBinding.Protocol || !first.IsActive ||
		first.Status != "active" || first.BindingCount != 0 ||
		first.BoundAgents == nil || len(first.BoundAgents) != 0 {
		t.Fatalf("unexpected no-binding endpoint response: %+v", first)
	}

	second := testEndpoints[1]
	if second.BindingCount != 2 || len(second.BoundAgents) != 2 {
		t.Fatalf("expected both active bindings, including offline agent: %+v", second)
	}
	if second.BoundAgents[0].ID != onlineAgent.ID ||
		second.BoundAgents[0].Name != onlineAgent.Name ||
		second.BoundAgents[0].Status != "online" {
		t.Fatalf("unexpected online agent response: %+v", second.BoundAgents[0])
	}
	if second.BoundAgents[1].ID != offlineAgent.ID ||
		second.BoundAgents[1].Status != "offline" {
		t.Fatalf("unexpected offline agent response: %+v", second.BoundAgents[1])
	}

	third := testEndpoints[2]
	if third.IsActive || third.Status != "disabled" {
		t.Fatalf("expected inactive endpoint to remain visible: %+v", third)
	}
}

func TestListServiceEndpointsRequiresServiceToken(t *testing.T) {
	r, cleanup := setupServiceTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/service/endpoints", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected HTTP 401 without service token, got %d: %s", rec.Code, rec.Body.String())
	}
}
