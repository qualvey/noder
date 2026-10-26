package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestResolveEndpointUUIDUsesUserUUIDByDefault(t *testing.T) {
	if got := resolveEndpointUUID(nil, "user-uuid"); got != "user-uuid" {
		t.Fatalf("resolveEndpointUUID(nil, ...) = %q, want user UUID", got)
	}

	blank := "  "
	if got := resolveEndpointUUID(&blank, "user-uuid"); got != "user-uuid" {
		t.Fatalf("resolveEndpointUUID(blank, ...) = %q, want user UUID", got)
	}

	custom := "  endpoint-uuid  "
	if got := resolveEndpointUUID(&custom, "user-uuid"); got != "endpoint-uuid" {
		t.Fatalf("resolveEndpointUUID(custom, ...) = %q, want trimmed custom UUID", got)
	}
}

func TestUserEndpointsDefaultToUserUUID(t *testing.T) {
	r, cleanup := setupServiceTest(t)
	defer cleanup()
	RegisterUserRoutes(r)

	createBody, _ := json.Marshal(map[string]interface{}{
		"name":      "endpoint-uuid-default-test",
		"uuid":      "user-uuid-default",
		"endpoints": []map[string]interface{}{{"node_id": 1, "tag": "default-endpoint"}},
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/users/", bytes.NewReader(createBody))
	createReq.Header.Set("X-Admin-Token", "admin-secret")
	createResp := httptest.NewRecorder()
	r.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusOK {
		t.Fatalf("create user returned %d: %s", createResp.Code, createResp.Body.String())
	}

	var created UserReadResp
	if err := json.Unmarshal(createResp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created user: %v", err)
	}
	if len(created.Endpoints) != 1 || created.Endpoints[0].UUID != "user-uuid-default" {
		t.Fatalf("created endpoint did not inherit user UUID: %#v", created.Endpoints)
	}

	addBody, _ := json.Marshal(map[string]interface{}{"node_id": 2, "tag": "added-endpoint"})
	addReq := httptest.NewRequest(http.MethodPost, "/api/users/"+userTestFormatID(created.ID)+"/endpoints", bytes.NewReader(addBody))
	addReq.Header.Set("X-Admin-Token", "admin-secret")
	addResp := httptest.NewRecorder()
	r.ServeHTTP(addResp, addReq)
	if addResp.Code != http.StatusOK {
		t.Fatalf("add endpoint returned %d: %s", addResp.Code, addResp.Body.String())
	}

	var added EndpointReadResp
	if err := json.Unmarshal(addResp.Body.Bytes(), &added); err != nil {
		t.Fatalf("decode added endpoint: %v", err)
	}
	if added.UUID != "user-uuid-default" {
		t.Fatalf("added endpoint UUID = %q, want user UUID", added.UUID)
	}

	if added.UserID != created.ID || added.Node == nil {
		t.Fatalf("unexpected endpoint relation: %#v", added)
	}
}

func userTestFormatID(id int64) string {
	return strconv.FormatInt(id, 10)
}
