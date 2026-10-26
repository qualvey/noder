package singbox

import (
	"encoding/json"
	"testing"

	"noder/internal/model"
)

func TestBuildSingboxOutbound(t *testing.T) {
	node := &model.Node{
		Tag:           "hk-tuic",
		Protocol:      "tuic",
		ServerAddress: "1.2.3.4",
		ServerPort:    8443,
		Security:      "tls",
		IsActive:      true,
	}
	user := &model.User{
		Token: "test-token",
	}

	ob, err := BuildSingboxOutbound(node, user)
	if err != nil {
		t.Fatalf("BuildSingboxOutbound failed: %v", err)
	}

	if ob["type"] != "tuic" {
		t.Errorf("Expected type tuic, got %v", ob["type"])
	}
	if ob["tag"] != "hk-tuic" {
		t.Errorf("Expected tag hk-tuic, got %v", ob["tag"])
	}
	if ob["uuid"] != "test-token" {
		t.Errorf("Expected uuid test-token, got %v", ob["uuid"])
	}
}

func TestBuildSingboxOutboundDoesNotEmitTCPTransport(t *testing.T) {
	node := &model.Node{
		Tag:           "vless-tcp",
		Protocol:      "vless",
		ServerAddress: "1.2.3.4",
		ServerPort:    443,
		Security:      "reality",
		SNI:           strPtr("example.com"),
		PublicKey:     strPtr("public-key"),
		ShortID:       strPtr("short-id"),
		Fingerprint:   strPtr("chrome"),
		TransportType: "tcp",
		IsActive:      true,
	}
	user := &model.User{Token: "test-token"}

	ob, err := BuildSingboxOutbound(node, user)
	if err != nil {
		t.Fatalf("BuildSingboxOutbound failed: %v", err)
	}
	if _, exists := ob["transport"]; exists {
		t.Fatalf("sing-box outbound must not contain a tcp transport: %v", ob["transport"])
	}
}

func TestBuildSingboxOutboundDoesNotEmitUnsupportedTransport(t *testing.T) {
	node := &model.Node{
		Tag:           "vless-invalid-transport",
		Protocol:      "vless",
		ServerAddress: "1.2.3.4",
		ServerPort:    443,
		Security:      "reality",
		SNI:           strPtr("example.com"),
		PublicKey:     strPtr("public-key"),
		ShortID:       strPtr("short-id"),
		Fingerprint:   strPtr("chrome"),
		TransportType: "kcp",
		IsActive:      true,
	}

	ob, err := BuildSingboxOutbound(node, &model.User{Token: "test-token"})
	if err != nil {
		t.Fatalf("BuildSingboxOutbound failed: %v", err)
	}
	if _, exists := ob["transport"]; exists {
		t.Fatalf("sing-box outbound must not contain an unsupported transport: %v", ob["transport"])
	}
}

func TestVlessRealityValidation(t *testing.T) {
	// 缺少 reality 依赖字段测试
	node := &model.Node{
		Tag:           "vless-bad",
		Protocol:      "vless",
		ServerAddress: "1.2.3.4",
		ServerPort:    443,
		Security:      "reality",
		IsActive:      true,
	}
	user := &model.User{Token: "test-token"}

	_, err := BuildSingboxOutbound(node, user)
	if err == nil {
		t.Fatalf("Expected validation error for missing reality keys, got nil")
	}
}

func TestBuildSingboxEndpointOutbound_MultipleRoutesSameNode(t *testing.T) {
	node := &model.Node{
		Tag:           "entry-node-default-tag",
		NodeName:      "HK-Reality-Inbound",
		Protocol:      "vless",
		ServerAddress: "hk.example.com",
		ServerPort:    443,
		Security:      "reality",
		SNI:           strPtr("example.com"),
		PublicKey:     strPtr("99BZ0JCnaSB55YEQYOCV66GhKTiK2ZGMPR3b6D_Q3wo"),
		ShortID:       strPtr("1a91"),
		Fingerprint:   strPtr("chrome"),
		Flow:          strPtr("xtls-rprx-vision"),
		IsActive:      true,
	}

	epDirect := &model.Endpoint{
		Node:      node,
		Tag:       "🇭🇰 香港 01 - 直连",
		UUID:      "11111111-1111-1111-1111-111111111111",
		IsActive:  true,
		SortOrder: 0,
	}

	epWarp := &model.Endpoint{
		Node:      node,
		Tag:       "🇺🇸 美国 02 - WARP 出口",
		UUID:      "22222222-2222-2222-2222-222222222222",
		IsActive:  true,
		SortOrder: 1,
	}

	ob1, err := BuildSingboxEndpointOutbound(epDirect)
	if err != nil {
		t.Fatalf("BuildSingboxEndpointOutbound epDirect failed: %v", err)
	}
	if ob1["tag"] != "🇭🇰 香港 01 - 直连" {
		t.Errorf("Expected tag '🇭🇰 香港 01 - 直连', got '%v'", ob1["tag"])
	}
	if ob1["uuid"] != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("Expected uuid 11111111-1111-1111-1111-111111111111, got '%v'", ob1["uuid"])
	}

	ob2, err := BuildSingboxEndpointOutbound(epWarp)
	if err != nil {
		t.Fatalf("BuildSingboxEndpointOutbound epWarp failed: %v", err)
	}
	if ob2["tag"] != "🇺🇸 美国 02 - WARP 出口" {
		t.Errorf("Expected tag '🇺🇸 美国 02 - WARP 出口', got '%v'", ob2["tag"])
	}
	if ob2["uuid"] != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("Expected uuid 22222222-2222-2222-2222-222222222222, got '%v'", ob2["uuid"])
	}

	// Verify that server address and port are from the shared node
	if ob1["server"] != "hk.example.com" || ob2["server"] != "hk.example.com" {
		t.Errorf("Expected server hk.example.com")
	}
}

func TestGenerateSingboxConfig_IndependentNodeExcludedFromURLTest(t *testing.T) {
	node1 := &model.Node{
		ID: 1, Tag: "node-1", Protocol: "vless", ServerAddress: "one.example.com", ServerPort: 443,
		Security: "reality", SNI: strPtr("one.example.com"), PublicKey: strPtr("public-key-1"),
		ShortID: strPtr("0001"), Fingerprint: strPtr("chrome"), IsActive: true,
	}
	node2 := &model.Node{
		ID: 2, Tag: "node-2", Protocol: "vless", ServerAddress: "two.example.com", ServerPort: 443,
		Security: "reality", SNI: strPtr("two.example.com"), PublicKey: strPtr("public-key-2"),
		ShortID: strPtr("0002"), Fingerprint: strPtr("chrome"), IsActive: true,
	}

	independentIDs := "[2]"
	cfg := map[string]interface{}{
		"outbounds": []interface{}{
			map[string]interface{}{"type": "selector", "tag": "Proxy", "outbounds": []interface{}{}},
			map[string]interface{}{"type": "urltest", "tag": "urltest", "outbounds": []interface{}{}},
		},
	}
	result, err := GenerateSingboxConfigFromTemplate(cfg, []*model.Endpoint{
		{NodeID: &node1.ID, Node: node1, Tag: node1.Tag, UUID: "uuid-1", IsActive: true},
		{NodeID: &node2.ID, Node: node2, Tag: node2.Tag, UUID: "uuid-2", IsActive: true},
	}, &model.User{IndependentNodeIDs: &independentIDs})
	if err != nil {
		t.Fatalf("GenerateSingboxConfigFromTemplate failed: %v", err)
	}

	ob, ok := result["outbounds"].([]interface{})
	if !ok || len(ob) != 4 {
		t.Fatalf("unexpected outbounds: %#v", result["outbounds"])
	}
	selector := ob[0].(map[string]interface{})["outbounds"]
	urltest := ob[1].(map[string]interface{})["outbounds"]
	if got, want := mustJSON(selector), `["node-1","node-2"]`; got != want {
		t.Errorf("selector outbounds = %s, want %s", got, want)
	}
	if got, want := mustJSON(urltest), `["node-1"]`; got != want {
		t.Errorf("urltest outbounds = %s, want %s", got, want)
	}
}

func mustJSON(value interface{}) string {
	b, _ := json.Marshal(value)
	return string(b)
}

func strPtr(s string) *string {
	return &s
}
