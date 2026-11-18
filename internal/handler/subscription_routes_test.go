package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"noder/internal/db"
	"noder/internal/model"
)

func TestSubscriptionAndUserRoutesQueryUser(t *testing.T) {
	r, cleanup := setupServiceTest(t)
	defer cleanup()
	RegisterSubscriptionRoutes(r)
	RegisterUserRoutes(r)

	adminToken := "admin-secret"
	var setting model.AppSetting
	if err := db.DB.NewSelect().Model(&setting).Where("key = ?", "admin_secret_token").Scan(t.Context()); err == nil && setting.Value != "" {
		adminToken = setting.Value
	}

	// 1. /sub with valid token
	req := httptest.NewRequest(http.MethodGet, "/sub?token=my-secret-token", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("/sub returned code %d: %s", resp.Code, resp.Body.String())
	}

	// 2. /api/users/1 (GetUser)
	req2 := httptest.NewRequest(http.MethodGet, "/api/users/1", nil)
	req2.Header.Set("X-Admin-Token", adminToken)
	resp2 := httptest.NewRecorder()
	r.ServeHTTP(resp2, req2)
	if resp2.Code != http.StatusOK {
		t.Fatalf("/api/users/1 returned code %d: %s", resp2.Code, resp2.Body.String())
	}

	// 3. /api/users/1 (UpdateUser)
	updateBody, _ := json.Marshal(map[string]interface{}{"remark": "updated remark"})
	req3 := httptest.NewRequest(http.MethodPut, "/api/users/1", bytes.NewReader(updateBody))
	req3.Header.Set("X-Admin-Token", adminToken)
	resp3 := httptest.NewRecorder()
	r.ServeHTTP(resp3, req3)
	if resp3.Code != http.StatusOK {
		t.Fatalf("/api/users/1 PUT returned code %d: %s", resp3.Code, resp3.Body.String())
	}

	// 4. /sub with non-existent token (clean 401 without SQL syntax error)
	req4 := httptest.NewRequest(http.MethodGet, "/sub?token=97347ae4-1abc-478b-8577-4bd5829eaa92", nil)
	resp4 := httptest.NewRecorder()
	r.ServeHTTP(resp4, req4)
	if resp4.Code != http.StatusUnauthorized {
		t.Fatalf("/sub with unknown token returned code %d, want 401: %s", resp4.Code, resp4.Body.String())
	}

	// 5. Download route user lookup
	RegisterDownloadRoutes(r)
	req5 := httptest.NewRequest(http.MethodGet, "/dl/999999?token=my-secret-token", nil)
	resp5 := httptest.NewRecorder()
	r.ServeHTTP(resp5, req5)
	// non-existent file returns 404, proving token authentication succeeded and query worked
	if resp5.Code != http.StatusNotFound {
		t.Fatalf("/dl with valid user token returned code %d, want 404 for missing file: %s", resp5.Code, resp5.Body.String())
	}
}

func TestPostgresUserQueryQuoting(t *testing.T) {
	pgDB := bun.NewDB(nil, pgdialect.New())
	pgDB.RegisterModel(
		(*model.UserNodeLink)(nil), (*model.User)(nil), (*model.Node)(nil), (*model.Endpoint)(nil),
	)

	var user model.User

	// subscription.go / download.go query
	subQuery := pgDB.NewSelect().
		Model(&user).
		Relation("Endpoints", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Order("endpoint.sort_order ASC", "endpoint.id ASC")
		}).
		Relation("Endpoints.Node").
		Relation("Nodes").
		Where("?TableAlias.token = ?", "test-token")

	renderedSub := subQuery.String()
	if !bytes.Contains([]byte(renderedSub), []byte(`WHERE ("user".token = 'test-token')`)) {
		t.Fatalf("expected quoted \"user\".token in postgres query, got: %s", renderedSub)
	}

	// users.go query
	userQuery := pgDB.NewSelect().
		Model(&user).
		Relation("Nodes").
		Relation("Endpoints", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Order("endpoint.sort_order ASC", "endpoint.id ASC")
		}).
		Relation("Endpoints.Node").
		Where("?TableAlias.id = ?", int64(42))

	renderedUser := userQuery.String()
	if !bytes.Contains([]byte(renderedUser), []byte(`WHERE ("user".id = 42)`)) {
		t.Fatalf("expected quoted \"user\".id in postgres query, got: %s", renderedUser)
	}
}
