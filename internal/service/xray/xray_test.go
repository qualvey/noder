package xray

import (
	"os"
	"path/filepath"
	"testing"

	"noder/internal/config"
	"noder/internal/jsonutil"
	"noder/internal/model"
)

func testXrayUser() *model.User {
	uuid := "11111111-1111-4111-8111-111111111111"
	return &model.User{UUID: &uuid}
}

func testXrayNode() *model.Node {
	sni := "www.example.com"
	publicKey := "public-key"
	shortID := "01234567"
	fingerprint := "chrome"
	flow := "xtls-rprx-vision"
	return &model.Node{
		Tag:           "vless-01",
		Protocol:      "vless",
		ServerAddress: "example.com",
		ServerPort:    443,
		Security:      "reality",
		SNI:           &sni,
		PublicKey:     &publicKey,
		ShortID:       &shortID,
		Fingerprint:   &fingerprint,
		Flow:          &flow,
		IsActive:      true,
	}
}

func TestBuildXrayOutbound(t *testing.T) {
	outbound, err := BuildXrayOutbound(testXrayNode(), testXrayUser())
	if err != nil {
		t.Fatalf("BuildXrayOutbound failed: %v", err)
	}

	if outbound["protocol"] != "vless" || outbound["tag"] != "vless-01" {
		t.Fatalf("unexpected outbound identity: %#v", outbound)
	}
	settings := outbound["settings"].(map[string]interface{})
	if settings["id"] != "11111111-1111-4111-8111-111111111111" || settings["encryption"] != "none" {
		t.Errorf("unexpected VLESS settings: %#v", settings)
	}
	stream := outbound["streamSettings"].(map[string]interface{})
	if stream["method"] != "raw" || stream["security"] != "reality" {
		t.Errorf("unexpected stream settings: %#v", stream)
	}
	reality := stream["realitySettings"].(map[string]interface{})
	if reality["serverName"] != "www.example.com" || reality["password"] != "public-key" || reality["shortId"] != "01234567" {
		t.Errorf("unexpected Reality settings: %#v", reality)
	}
}

func TestBuildXrayXHTTPOutbound(t *testing.T) {
	sni := "test.ryugo.org"
	path := "/api/v1/data"
	flow := ""
	node := &model.Node{
		Tag:           "xhttp",
		Protocol:      "vless",
		ServerAddress: "test.ryugo.org",
		ServerPort:    443,
		Security:      "tls",
		SNI:           &sni,
		TransportType: "xhttp",
		Path:          &path,
		Flow:          &flow,
		IsActive:      true,
	}

	outbound, err := BuildXrayOutbound(node, testXrayUser())
	if err != nil {
		t.Fatalf("BuildXrayOutbound xhttp failed: %v", err)
	}
	settings := outbound["settings"].(map[string]interface{})
	vnext := settings["vnext"].([]interface{})[0].(map[string]interface{})
	users := vnext["users"].([]interface{})[0].(map[string]interface{})
	if vnext["address"] != "test.ryugo.org" || vnext["port"] != 443 || users["flow"] != "" {
		t.Fatalf("unexpected xhttp vnext settings: %#v", settings)
	}
	stream := outbound["streamSettings"].(map[string]interface{})
	if stream["network"] != "xhttp" || stream["security"] != "tls" {
		t.Fatalf("unexpected xhttp stream settings: %#v", stream)
	}
	tls := stream["tlsSettings"].(map[string]interface{})
	if tls["serverName"] != "test.ryugo.org" || tls["allowInsecure"] != false {
		t.Fatalf("unexpected xhttp TLS settings: %#v", tls)
	}
	xhttp := stream["xhttpSettings"].(map[string]interface{})
	if xhttp["path"] != "/api/v1/data" || xhttp["mode"] != "auto" {
		t.Fatalf("unexpected xhttp settings: %#v", xhttp)
	}
}

func TestBuildXrayXHTTPRejectsReality(t *testing.T) {
	node := testXrayNode()
	node.TransportType = "xhttp"
	if _, err := BuildXrayOutbound(node, testXrayUser()); err == nil {
		t.Fatal("expected xhttp + Reality to be rejected")
	}
}

func TestGenerateXrayConfigPreservesTemplateAndRejectsDuplicateTag(t *testing.T) {
	tmp := t.TempDir()
	config.XrayTemplatePath = filepath.Join(tmp, "xray.json")
	data := []byte(`{"log":{"loglevel":"warning"},"outbounds":[{"protocol":"freedom","tag":"direct"}]}`)
	if err := os.WriteFile(config.XrayTemplatePath, data, 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := GenerateXrayConfig([]*model.Node{testXrayNode()}, testXrayUser())
	if err != nil {
		t.Fatalf("GenerateXrayConfig failed: %v", err)
	}
	if cfg["log"] == nil {
		t.Error("template field log was not preserved")
	}
	outbounds := cfg["outbounds"].([]interface{})
	if len(outbounds) != 2 {
		t.Fatalf("expected generated outbound plus template outbound, got %#v", outbounds)
	}
	if outbounds[0].(map[string]interface{})["tag"] != "vless-01" {
		t.Fatalf("expected user nodes prepended in priority order, got %#v", outbounds)
	}
	if outbounds[1].(map[string]interface{})["tag"] != "direct" {
		t.Fatalf("expected template outbound preserved after user nodes, got %#v", outbounds)
	}

	duplicate := testXrayNode()
	duplicate.Tag = "direct"
	if _, err := GenerateXrayConfig([]*model.Node{duplicate}, testXrayUser()); err == nil {
		t.Error("expected duplicate tag error")
	}
}

func TestGenerateXrayConfigSkipsUnsupportedEndpoints(t *testing.T) {
	tuic := &model.Node{
		Tag:           "tuic-01",
		Protocol:      "tuic",
		ServerAddress: "tuic.example.com",
		ServerPort:    443,
		IsActive:      true,
	}
	vless := testXrayNode()

	cfg, err := GenerateXrayConfigFromTemplate(
		map[string]interface{}{"outbounds": []interface{}{}},
		[]*model.Endpoint{
			{Node: tuic, Tag: "tuic-01", UUID: "tuic-uuid", IsActive: true},
			{Node: vless, Tag: "vless-01", UUID: "11111111-1111-4111-8111-111111111111", IsActive: true},
		},
		testXrayUser(),
	)
	if err != nil {
		t.Fatalf("mixed Xray endpoints should render: %v", err)
	}

	outbounds := cfg["outbounds"].([]interface{})
	if len(outbounds) != 1 {
		t.Fatalf("expected only the VLESS outbound, got %#v", outbounds)
	}
	if outbounds[0].(map[string]interface{})["tag"] != "vless-01" {
		t.Fatalf("unexpected rendered outbound: %#v", outbounds[0])
	}
}

func TestGenerateXrayConfigPrependsUserNodesAndRewritesDNSProxy(t *testing.T) {
	first := testXrayNode()
	first.Tag = "node-a"
	second := testXrayNode()
	second.Tag = "node-b"

	cfg, err := GenerateXrayConfigFromTemplate(
		map[string]interface{}{
			"outbounds": []interface{}{map[string]interface{}{"protocol": "freedom", "tag": "direct"}},
			"routing": map[string]interface{}{
				"rules": []interface{}{
					map[string]interface{}{"inboundTag": []interface{}{"dns-proxy"}, "outboundTag": "Proxy"},
					map[string]interface{}{"inboundTag": []interface{}{"dns-direct"}, "outboundTag": "direct"},
				},
			},
		},
		[]*model.Endpoint{
			{Node: first, Tag: "node-a", UUID: "11111111-1111-4111-8111-111111111111", IsActive: true},
			{Node: second, Tag: "node-b", UUID: "11111111-1111-4111-8111-111111111111", IsActive: true},
		},
		testXrayUser(),
	)
	if err != nil {
		t.Fatalf("GenerateXrayConfigFromTemplate failed: %v", err)
	}

	outbounds := cfg["outbounds"].([]interface{})
	gotTags := []string{
		outbounds[0].(map[string]interface{})["tag"].(string),
		outbounds[1].(map[string]interface{})["tag"].(string),
		outbounds[2].(map[string]interface{})["tag"].(string),
	}
	wantTags := []string{"node-a", "node-b", "direct"}
	for i := range wantTags {
		if gotTags[i] != wantTags[i] {
			t.Fatalf("unexpected outbound order: got %#v want %#v", gotTags, wantTags)
		}
	}

	rules := cfg["routing"].(map[string]interface{})["rules"].([]interface{})
	if rules[0].(map[string]interface{})["outboundTag"] != "node-a" {
		t.Fatalf("dns-proxy rule should point to first node, got %#v", rules[0])
	}
	if rules[1].(map[string]interface{})["outboundTag"] != "direct" {
		t.Fatalf("other routing rules should be preserved, got %#v", rules[1])
	}
}

func TestXrayTemplateSupportsComments(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "templates", "xray.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]interface{}
	if err := jsonutil.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid template JSON: %v", err)
	}
}
