package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"noder/internal/db"
	"noder/internal/model"
)

func TestDeleteNodeRemovesUserEndpointsAndLinks(t *testing.T) {
	r, cleanup := setupServiceTest(t)
	defer cleanup()
	RegisterNodeRoutes(r)

	request := httptest.NewRequest(http.MethodDelete, "/api/nodes/1", nil)
	request.Header.Set("X-Admin-Token", "admin-secret")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("delete node returned %d: %s", response.Code, response.Body.String())
	}

	var node model.Node
	if err := db.DB.NewSelect().Model(&node).Where("id = ?", 1).Scan(t.Context()); err == nil {
		t.Fatal("deleted node is still present")
	}

	endpointCount, err := db.DB.NewSelect().Model((*model.Endpoint)(nil)).Where("node_id = ?", 1).Count(t.Context())
	if err != nil {
		t.Fatalf("count endpoints: %v", err)
	}
	if endpointCount != 0 {
		t.Fatalf("expected no endpoints for deleted node, got %d", endpointCount)
	}

	linkCount, err := db.DB.NewSelect().Model((*model.UserNodeLink)(nil)).Where("node_id = ?", 1).Count(t.Context())
	if err != nil {
		t.Fatalf("count user-node links: %v", err)
	}
	if linkCount != 0 {
		t.Fatalf("expected no user-node links for deleted node, got %d", linkCount)
	}
}
