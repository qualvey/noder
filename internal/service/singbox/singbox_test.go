package singbox

import (
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
