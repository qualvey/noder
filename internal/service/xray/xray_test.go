package xray

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"noder/internal/config"
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
	if len(cfg["outbounds"].([]interface{})) != 2 {
		t.Fatalf("expected template outbound plus generated outbound, got %#v", cfg["outbounds"])
	}

	duplicate := testXrayNode()
	duplicate.Tag = "direct"
	if _, err := GenerateXrayConfig([]*model.Node{duplicate}, testXrayUser()); err == nil {
		t.Error("expected duplicate tag error")
	}
}

func TestXrayTemplateIsValidJSON(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "templates", "xray.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid template JSON: %v", err)
	}
}
