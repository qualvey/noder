package mihomo

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"noder/internal/model"
)

func strPtr(s string) *string {
	return &s
}

func TestBuildMihomoEndpointProxy_NilCheck(t *testing.T) {
	if _, err := BuildMihomoEndpointProxy(nil); err == nil {
		t.Fatal("expected error when endpoint is nil")
	}
	if _, err := BuildMihomoEndpointProxy(&model.Endpoint{Node: nil}); err == nil {
		t.Fatal("expected error when endpoint node is nil")
	}
}

func TestBuildMihomoEndpointProxy_XHTTPRejected(t *testing.T) {
	ep := &model.Endpoint{
		Node: &model.Node{
			Tag:           "vless-xhttp",
			Protocol:      "vless",
			ServerAddress: "example.com",
			ServerPort:    443,
			Security:      "tls",
			SNI:           strPtr("example.com"),
			TransportType: "xhttp",
		},
		UUID: "00000000-0000-0000-0000-000000000001",
	}
	_, err := BuildMihomoEndpointProxy(ep)
	if err == nil {
		t.Fatal("expected error for xhttp transport in mihomo")
	}
	if !strings.Contains(err.Error(), "xhttp") {
		t.Fatalf("expected xhttp error message, got: %v", err)
	}
}

func TestBuildMihomoEndpointProxy_VlessReality(t *testing.T) {
	ep := &model.Endpoint{
		Tag: "tokyo-vless",
		Node: &model.Node{
			Tag:           "node-tag",
			NodeName:      "Tokyo Node",
			Protocol:      "vless",
			ServerAddress: "198.51.100.10",
			ServerPort:    8443,
			Security:      "reality",
			SNI:           strPtr("gateway.example.com"),
			PublicKey:     strPtr("test-public-key-base64"),
			ShortID:       strPtr("abcdef12"),
			Fingerprint:   strPtr("chrome"),
			Flow:          strPtr("xtls-rprx-vision"),
		},
		UUID: "c8e2358b-9e45-4202-b2a6-068305fce9c0",
	}

	proxy, err := BuildMihomoEndpointProxy(ep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if proxy["name"] != "tokyo-vless" {
		t.Errorf("expected name tokyo-vless, got %v", proxy["name"])
	}
	if proxy["type"] != "vless" {
		t.Errorf("expected type vless, got %v", proxy["type"])
	}
	if proxy["server"] != "198.51.100.10" || proxy["port"] != 8443 {
		t.Errorf("unexpected server/port: %v:%v", proxy["server"], proxy["port"])
	}
	if proxy["uuid"] != "c8e2358b-9e45-4202-b2a6-068305fce9c0" {
		t.Errorf("unexpected uuid: %v", proxy["uuid"])
	}
	if proxy["network"] != "tcp" || proxy["udp"] != true || proxy["tls"] != true {
		t.Errorf("unexpected network flags: network=%v, udp=%v, tls=%v", proxy["network"], proxy["udp"], proxy["tls"])
	}
	if proxy["servername"] != "gateway.example.com" {
		t.Errorf("expected servername gateway.example.com, got %v", proxy["servername"])
	}
	if proxy["flow"] != "xtls-rprx-vision" {
		t.Errorf("expected flow xtls-rprx-vision, got %v", proxy["flow"])
	}
	realityOpts, ok := proxy["reality-opts"].(map[string]interface{})
	if !ok || realityOpts["public-key"] != "test-public-key-base64" || realityOpts["short-id"] != "abcdef12" {
		t.Errorf("unexpected reality-opts: %v", proxy["reality-opts"])
	}
	if proxy["client-fingerprint"] != "chrome" {
		t.Errorf("expected client-fingerprint chrome, got %v", proxy["client-fingerprint"])
	}
}

func TestBuildMihomoEndpointProxy_Tuic(t *testing.T) {
	pwd := "tuic-secret-pass"
	ep := &model.Endpoint{
		Tag: "tuic-hk",
		Node: &model.Node{
			Tag:               "node-tuic",
			Protocol:          "tuic",
			ServerAddress:     "198.51.100.20",
			ServerPort:        443,
			Security:          "tls",
			SNI:               strPtr("tuic.example.com"),
			CongestionControl: strPtr("cubic"),
		},
		UUID:     "d9e2358b-9e45-4202-b2a6-068305fce9c1",
		Password: &pwd,
	}

	proxy, err := BuildMihomoEndpointProxy(ep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if proxy["type"] != "tuic" {
		t.Errorf("expected type tuic, got %v", proxy["type"])
	}
	if proxy["uuid"] != "d9e2358b-9e45-4202-b2a6-068305fce9c1" {
		t.Errorf("unexpected uuid: %v", proxy["uuid"])
	}
	if proxy["password"] != pwd {
		t.Errorf("unexpected password: %v", proxy["password"])
	}
	if proxy["congestion-controller"] != "cubic" {
		t.Errorf("expected congestion-controller cubic, got %v", proxy["congestion-controller"])
	}
	if proxy["sni"] != "tuic.example.com" {
		t.Errorf("expected sni tuic.example.com, got %v", proxy["sni"])
	}
	if proxy["reduce-rtt"] != true {
		t.Errorf("expected reduce-rtt true, got %v", proxy["reduce-rtt"])
	}
	alpn, ok := proxy["alpn"].([]string)
	if !ok || len(alpn) != 1 || alpn[0] != "h3" {
		t.Errorf("expected alpn [h3], got %v", proxy["alpn"])
	}
}

func TestBuildMihomoProxy_UserFallback(t *testing.T) {
	node := &model.Node{
		Tag:           "fallback-node",
		Protocol:      "vless",
		ServerAddress: "198.51.100.30",
		ServerPort:    443,
		Security:      "tls",
		SNI:           strPtr("sni.example.com"),
	}
	user := &model.User{
		Token: "user-token-fallback",
	}

	proxy, err := BuildMihomoProxy(node, user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proxy["uuid"] != "user-token-fallback" {
		t.Errorf("expected uuid to fallback to user token, got %v", proxy["uuid"])
	}
	if proxy["name"] != "fallback-node" {
		t.Errorf("expected name fallback-node, got %v", proxy["name"])
	}
}

func TestGenerateMihomoConfigFromEndpoints_FilteringAndGroups(t *testing.T) {
	pwd := "pass123"
	ep1 := &model.Endpoint{
		Tag:      "ep-active-1",
		IsActive: true,
		UUID:     "uuid-1",
		Password: &pwd,
		Node: &model.Node{
			Tag:           "node-active-1",
			Protocol:      "vless",
			ServerAddress: "1.1.1.1",
			ServerPort:    443,
			Security:      "tls",
			SNI:           strPtr("s1.com"),
			IsActive:      true,
		},
	}
	epInactiveEp := &model.Endpoint{
		Tag:      "ep-inactive",
		IsActive: false,
		UUID:     "uuid-2",
		Node: &model.Node{
			Tag:           "node-2",
			Protocol:      "vless",
			ServerAddress: "2.2.2.2",
			ServerPort:    443,
			Security:      "tls",
			SNI:           strPtr("s2.com"),
			IsActive:      true,
		},
	}
	epInactiveNode := &model.Endpoint{
		Tag:      "ep-inactive-node",
		IsActive: true,
		UUID:     "uuid-3",
		Node: &model.Node{
			Tag:           "node-3",
			Protocol:      "vless",
			ServerAddress: "3.3.3.3",
			ServerPort:    443,
			Security:      "tls",
			SNI:           strPtr("s3.com"),
			IsActive:      false,
		},
	}

	cfg, err := GenerateMihomoConfigFromEndpoints([]*model.Endpoint{ep1, epInactiveEp, epInactiveNode}, &model.User{Token: "test"})
	if err != nil {
		t.Fatalf("GenerateMihomoConfigFromEndpoints failed: %v", err)
	}

	proxies, ok := cfg["proxies"].([]interface{})
	if !ok {
		t.Fatalf("missing or invalid proxies list in config")
	}
	if len(proxies) != 1 {
		t.Fatalf("expected exactly 1 active proxy, got %d", len(proxies))
	}
	pMap := proxies[0].(map[string]interface{})
	if pMap["name"] != "ep-active-1" {
		t.Fatalf("expected ep-active-1, got %v", pMap["name"])
	}

	// Verify proxy groups
	groups, ok := cfg["proxy-groups"].([]interface{})
	if !ok || len(groups) == 0 {
		t.Fatalf("missing proxy-groups")
	}

	foundProxyGroup := false
	foundAutoGroup := false
	for _, gRaw := range groups {
		g := gRaw.(map[string]interface{})
		name := g["name"].(string)
		members := g["proxies"].([]interface{})
		if name == "Proxy" {
			foundProxyGroup = true
			if len(members) < 2 || members[1] != "ep-active-1" {
				t.Errorf("expected 'ep-active-1' in Proxy group, got: %v", members)
			}
		}
		if name == "auto" {
			foundAutoGroup = true
			if len(members) != 1 || members[0] != "ep-active-1" {
				t.Errorf("expected 'ep-active-1' in auto group, got: %v", members)
			}
		}
	}
	if !foundProxyGroup || !foundAutoGroup {
		t.Errorf("expected both 'Proxy' and 'auto' groups, foundProxy=%v, foundAuto=%v", foundProxyGroup, foundAutoGroup)
	}
}

func TestBuildMihomoConfigYAML(t *testing.T) {
	node := &model.Node{
		Tag:           "yaml-node",
		Protocol:      "vless",
		ServerAddress: "1.2.3.4",
		ServerPort:    443,
		Security:      "tls",
		SNI:           strPtr("yaml.example.com"),
		IsActive:      true,
	}
	user := &model.User{
		Token: "yaml-user-token",
	}

	yamlBytes, err := BuildMihomoConfigYAML([]*model.Node{node}, user)
	if err != nil {
		t.Fatalf("BuildMihomoConfigYAML failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := yaml.Unmarshal(yamlBytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal generated yaml: %v", err)
	}
	if parsed["mixed-port"] != 7890 {
		t.Errorf("expected mixed-port 7890, got %v", parsed["mixed-port"])
	}
	proxies, ok := parsed["proxies"].([]interface{})
	if !ok || len(proxies) != 1 {
		t.Fatalf("expected 1 proxy in yaml, got %d", len(proxies))
	}
}
