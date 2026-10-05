package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"noder/internal/config"
	"noder/internal/db"
	"noder/internal/model"
)

func setupServiceTest(t *testing.T) (*chi.Mux, func()) {
	tmpDir, err := os.MkdirTemp("", "noder-service-test-*")
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
	RegisterServiceRoutes(r)

	cleanup := func() {
		_ = database.Close()
		_ = os.RemoveAll(tmpDir)
	}

	return r, cleanup
}

func TestServiceUserRegister(t *testing.T) {
	r, cleanup := setupServiceTest(t)
	defer cleanup()

	// 1. Success registration
	body := map[string]interface{}{
		"external_id": "mall_user_10086",
		"name":        "user@example.com",
		"token":       "custom_token_123",
		"remark":      "定制独享套餐",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/service/users/register", bytes.NewReader(bodyBytes))
	req.Header.Set("X-Service-Token", "test-service-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json parse error: %v", err)
	}
	if resp["code"].(float64) != 0 {
		t.Fatalf("expected code 0, got %v", resp["code"])
	}
	data := resp["data"].(map[string]interface{})
	if data["external_id"] != "mall_user_10086" || data["token"] != "custom_token_123" {
		t.Fatalf("unexpected data: %v", data)
	}
	subURLs := data["subscription_urls"].(map[string]interface{})
	if !strings.Contains(subURLs["singbox"].(string), "/sub?token=custom_token_123") {
		t.Fatalf("invalid singbox url: %v", subURLs["singbox"])
	}

	// 2. Duplicate external_id should return 400
	req2 := httptest.NewRequest("POST", "/api/service/users/register", bytes.NewReader(bodyBytes))
	req2.Header.Set("X-Service-Token", "test-service-token")
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on duplicate external_id, got %d", w2.Code)
	}
	if !strings.Contains(w2.Body.String(), "mall_user_10086") {
		t.Fatalf("expected error message to mention external_id, got: %s", w2.Body.String())
	}
}

func TestServiceAddUserCredential(t *testing.T) {
	r, cleanup := setupServiceTest(t)
	defer cleanup()

	// 1. Create a user first
	regBody, _ := json.Marshal(map[string]string{
		"external_id": "mall_u1",
		"token":       "token_u1",
	})
	regReq := httptest.NewRequest("POST", "/api/service/users/register", bytes.NewReader(regBody))
	regReq.Header.Set("X-Service-Token", "test-service-token")
	regW := httptest.NewRecorder()
	r.ServeHTTP(regW, regReq)
	if regW.Code != http.StatusOK {
		t.Fatalf("failed to register user: %s", regW.Body.String())
	}

	// 2. Add credential
	credBody, _ := json.Marshal(map[string]string{
		"node_tag": "HK-Dedicated-01",
		"uuid":     "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d",
		"password": "pwd_secret_123",
		"remark":   "张三定制香港 100M",
	})
	credReq := httptest.NewRequest("POST", "/api/service/users/token_u1/credentials", bytes.NewReader(credBody))
	credReq.Header.Set("X-Service-Token", "test-service-token")
	credW := httptest.NewRecorder()
	r.ServeHTTP(credW, credReq)

	if credW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", credW.Code, credW.Body.String())
	}

	// Verify in DB
	var ep model.Endpoint
	err := db.DB.NewSelect().Model(&ep).Where("tag = ?", "HK-Dedicated-01").Scan(t.Context())
	if err != nil {
		t.Fatalf("endpoint not found in DB: %v", err)
	}
	if ep.NodeID != nil {
		t.Fatalf("expected node_id to be nil (pending binding), got %v", *ep.NodeID)
	}
	if ep.Status != "pending_binding" {
		t.Fatalf("expected status 'pending_binding', got %s", ep.Status)
	}

	// 3. Duplicate tag should return 400
	credReq2 := httptest.NewRequest("POST", "/api/service/users/token_u1/credentials", bytes.NewReader(credBody))
	credReq2.Header.Set("X-Service-Token", "test-service-token")
	credW2 := httptest.NewRecorder()
	r.ServeHTTP(credW2, credReq2)

	if credW2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate tag, got %d", credW2.Code)
	}
}

func TestRenderConfigStateless(t *testing.T) {
	r, cleanup := setupServiceTest(t)
	defer cleanup()

	// 1. Sing-box render
	reqBody, _ := json.Marshal(map[string]interface{}{
		"platform": "sing-box",
		"nodes": []map[string]interface{}{
			{
				"tag":            "HK Reality 01",
				"protocol":       "vless",
				"server_address": "hk01.edge.net",
				"server_port":    443,
				"uuid":           "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d",
				"security":       "reality",
				"sni":            "gateway.icloud.com",
				"public_key":     "x5vK7aAbCdEfGhIjKlMnOpQrStUvWxYz0123456789a=",
				"short_id":       "0123456789abcdef",
				"fingerprint":    "chrome",
				"flow":           "xtls-rprx-vision",
			},
		},
	})

	req := httptest.NewRequest("POST", "/api/service/render-config", bytes.NewReader(reqBody))
	req.Header.Set("X-Service-Token", "test-service-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["platform"] != "sing-box" {
		t.Fatalf("expected platform sing-box, got %v", resp["platform"])
	}
	cfg := resp["config"].(map[string]interface{})
	outbounds := cfg["outbounds"].([]interface{})
	if len(outbounds) == 0 {
		t.Fatal("expected outbounds in config")
	}

	// 2. Xray render
	xrayBody, _ := json.Marshal(map[string]interface{}{
		"platform": "xray",
		"nodes": []map[string]interface{}{
			{
				"tag":            "HK Reality Xray",
				"protocol":       "vless",
				"server_address": "hk01.edge.net",
				"server_port":    443,
				"uuid":           "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d",
				"security":       "reality",
				"sni":            "gateway.icloud.com",
				"public_key":     "x5vK7aAbCdEfGhIjKlMnOpQrStUvWxYz0123456789a=",
				"short_id":       "0123456789abcdef",
				"fingerprint":    "chrome",
				"flow":           "xtls-rprx-vision",
			},
		},
	})
	reqXray := httptest.NewRequest("POST", "/api/service/render-config", bytes.NewReader(xrayBody))
	reqXray.Header.Set("X-Service-Token", "test-service-token")
	wXray := httptest.NewRecorder()
	r.ServeHTTP(wXray, reqXray)

	if wXray.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", wXray.Code, wXray.Body.String())
	}
}

func TestSocks5AntiAffinityAllocationAndGostAuth(t *testing.T) {
	r, cleanup := setupServiceTest(t)
	defer cleanup()

	// 1. Create 2 Socks5 mother machines with different IPs in US
	countryUS := "US"
	node1 := &model.Node{
		Tag:           "US-Socks-01",
		NodeName:      "US-Socks-01",
		Protocol:      "socks5",
		ServerAddress: "45.76.12.34",
		ServerPort:    1080,
		Country:       &countryUS,
		IsActive:      true,
	}
	node2 := &model.Node{
		Tag:           "US-Socks-02",
		NodeName:      "US-Socks-02",
		Protocol:      "socks5",
		ServerAddress: "104.28.99.88",
		ServerPort:    1080,
		Country:       &countryUS,
		IsActive:      true,
	}
	if _, err := db.DB.NewInsert().Model(&[]*model.Node{node1, node2}).Exec(t.Context()); err != nil {
		t.Fatalf("failed to insert test mother nodes: %v", err)
	}

	// 2. Register user
	user := &model.User{
		Name:     "Socks User",
		Token:    "socks-token-101",
		IsActive: true,
	}
	if _, err := db.DB.NewInsert().Model(user).Exec(t.Context()); err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	// 3. User requests 2 US nodes
	allocBody, _ := json.Marshal(map[string]interface{}{
		"country": "US",
		"count":   2,
		"remark":  "Shop",
	})
	allocReq := httptest.NewRequest("POST", "/api/service/users/socks-token-101/socks5/allocate", bytes.NewReader(allocBody))
	allocReq.Header.Set("X-Service-Token", "test-service-token")
	allocW := httptest.NewRecorder()
	r.ServeHTTP(allocW, allocReq)

	if allocW.Code != http.StatusOK {
		t.Fatalf("allocate failed: %d %s", allocW.Code, allocW.Body.String())
	}

	var allocResp struct {
		Code int `json:"code"`
		Data struct {
			AllocatedCount int              `json:"allocated_count"`
			Items          []Socks5ItemResp `json:"items"`
		} `json:"data"`
	}
	_ = json.Unmarshal(allocW.Body.Bytes(), &allocResp)

	if allocResp.Data.AllocatedCount != 2 {
		t.Fatalf("expected 2 allocated, got %d", allocResp.Data.AllocatedCount)
	}
	items := allocResp.Data.Items
	if items[0].IP == items[1].IP {
		t.Fatalf("Anti-affinity violation! Both items allocated same IP: %s", items[0].IP)
	}
	if items[0].Status != "active" || items[1].Status != "active" {
		t.Fatalf("expected both items active, got %s, %s", items[0].Status, items[1].Status)
	}
	if !strings.HasPrefix(items[0].ProxyURL, "socks5://") {
		t.Fatalf("invalid proxy url: %s", items[0].ProxyURL)
	}

	// 4. Request a 3rd node for the same user in US: pool should be exhausted!
	allocBody3, _ := json.Marshal(map[string]interface{}{
		"country": "US",
		"count":   1,
	})
	allocReq3 := httptest.NewRequest("POST", "/api/service/users/socks-token-101/socks5/allocate", bytes.NewReader(allocBody3))
	allocReq3.Header.Set("X-Service-Token", "test-service-token")
	allocW3 := httptest.NewRecorder()
	r.ServeHTTP(allocW3, allocReq3)

	var allocResp3 struct {
		Code int `json:"code"`
		Data struct {
			AllocatedCount int              `json:"allocated_count"`
			Items          []Socks5ItemResp `json:"items"`
		} `json:"data"`
	}
	_ = json.Unmarshal(allocW3.Body.Bytes(), &allocResp3)
	if len(allocResp3.Data.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(allocResp3.Data.Items))
	}
	if allocResp3.Data.Items[0].Status != "pending_pool_exhausted" {
		t.Fatalf("expected pending_pool_exhausted, got %s", allocResp3.Data.Items[0].Status)
	}

	// 5. Test user socks5 list format=text
	listTextReq := httptest.NewRequest("GET", "/api/service/users/socks-token-101/socks5/list?format=text", nil)
	listTextW := httptest.NewRecorder()
	r.ServeHTTP(listTextW, listTextReq)

	if listTextW.Code != http.StatusOK {
		t.Fatalf("list text failed: %d", listTextW.Code)
	}
	lines := strings.Split(strings.TrimSpace(listTextW.Body.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 active lines in text list, got %d: %q", len(lines), listTextW.Body.String())
	}
	if !strings.Contains(lines[0], ":1080:") || !strings.Contains(lines[1], ":1080:") {
		t.Fatalf("unexpected line format: %v", lines)
	}

	// 6. Test GOST mother machine auth reload list
	gostReq := httptest.NewRequest("GET", fmt.Sprintf("/api/service/socks5/nodes/%d/auth-list", node1.ID), nil)
	gostW := httptest.NewRecorder()
	r.ServeHTTP(gostW, gostReq)

	if gostW.Code != http.StatusOK {
		t.Fatalf("gost auth-list failed: %d %s", gostW.Code, gostW.Body.String())
	}
	gostBody := gostW.Body.String()
	if !strings.Contains(gostBody, "# username password") {
		t.Fatalf("gost auth header missing: %s", gostBody)
	}
	gostLines := strings.Split(strings.TrimSpace(gostBody), "\n")
	// Header + 1 user endpoint allocated to node1
	if len(gostLines) != 2 {
		t.Fatalf("expected 2 lines in gost auth list (1 comment + 1 auth), got %d: %q", len(gostLines), gostBody)
	}
}
