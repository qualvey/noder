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
	origPath := config.XrayTemplatePath
	config.XrayTemplatePath = filepath.Join(tmp, "xray.json")
	t.Cleanup(func() {
		config.XrayTemplatePath = origPath
	})
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

func TestGenerateXrayConfigReconcilesInvalidOutboundTags(t *testing.T) {
	primaryNode := testXrayNode()
	primaryNode.Tag = "primary-proxy"
	secondaryNode := testXrayNode()
	secondaryNode.Tag = "secondary-proxy"

	template := map[string]interface{}{
		"outbounds": []interface{}{
			map[string]interface{}{"protocol": "freedom", "tag": "direct"},
			map[string]interface{}{"protocol": "blackhole", "tag": "block"},
			map[string]interface{}{"protocol": "dns", "tag": "dns-out"},
		},
		"routing": map[string]interface{}{
			"rules": []interface{}{
				// 1. dns-proxy rule with invalid placeholder tag "usp" -> must become primary-proxy
				map[string]interface{}{
					"type":        "field",
					"inboundTag":  []interface{}{"dns-proxy"},
					"outboundTag": "usp",
				},
				// 2. General rule with non-existent tag "usp" -> must become primary-proxy
				map[string]interface{}{
					"type":        "field",
					"network":     "tcp,udp",
					"outboundTag": "usp",
				},
				// 3. General rule with placeholder "proxy" -> must become primary-proxy
				map[string]interface{}{
					"type":        "field",
					"domain":      []interface{}{"geosite:google"},
					"outboundTag": "proxy",
				},
				// 4. DNS port rule with valid template outbound "dns-out" -> must be untouched
				map[string]interface{}{
					"type":        "field",
					"port":        "53",
					"outboundTag": "dns-out",
				},
				// 5. Domestic domain rule with valid template outbound "direct" -> must be untouched
				map[string]interface{}{
					"type":        "field",
					"domain":      []interface{}{"geosite:cn"},
					"outboundTag": "direct",
				},
				// 6. Block rule with valid template outbound "block" -> must be untouched
				map[string]interface{}{
					"type":        "field",
					"protocol":    []interface{}{"bittorrent"},
					"outboundTag": "block",
				},
				// 7. Explicit rule pointing to valid secondary user node -> must be untouched
				map[string]interface{}{
					"type":        "field",
					"domain":      []interface{}{"domain:custom.org"},
					"outboundTag": "secondary-proxy",
				},
				// 8. Rule without outboundTag (balancerTag) -> must not be modified
				map[string]interface{}{
					"type":        "field",
					"inboundTag":  []interface{}{"tun"},
					"balancerTag": "all-balancers",
				},
				// 9. Rule with empty outboundTag "" -> must become primary-proxy
				map[string]interface{}{
					"type":        "field",
					"domain":      []interface{}{"domain:empty-tag.org"},
					"outboundTag": "",
				},
				// 10. Rule with invalid non-string outboundTag -> must become primary-proxy
				map[string]interface{}{
					"type":        "field",
					"domain":      []interface{}{"domain:invalid-type.org"},
					"outboundTag": 12345,
				},
				// 11. Malformed non-map rule -> must be safely ignored without panic
				"malformed-rule-entry",
				// 12. Rule with whitespace around dns-proxy inbound tag -> must become primary-proxy
				map[string]interface{}{
					"type":        "field",
					"inboundTag":  "  dns-proxy  ",
					"outboundTag": "direct",
				},
			},
		},
	}

	endpoints := []*model.Endpoint{
		{Node: primaryNode, Tag: "primary-proxy", UUID: "11111111-1111-4111-8111-111111111111", IsActive: true},
		{Node: secondaryNode, Tag: "secondary-proxy", UUID: "11111111-1111-4111-8111-111111111111", IsActive: true},
	}

	cfg, err := GenerateXrayConfigFromTemplate(template, endpoints, testXrayUser())
	if err != nil {
		t.Fatalf("GenerateXrayConfigFromTemplate failed: %v", err)
	}

	// 1. Verify outbounds order: user nodes prepended at index 0 and 1, followed by template outbounds
	outbounds, ok := cfg["outbounds"].([]interface{})
	if !ok || len(outbounds) != 5 {
		t.Fatalf("expected 5 outbounds, got %#v", cfg["outbounds"])
	}
	expectedTags := []string{"primary-proxy", "secondary-proxy", "direct", "block", "dns-out"}
	for i, want := range expectedTags {
		got := outbounds[i].(map[string]interface{})["tag"].(string)
		if got != want {
			t.Errorf("outbound[%d] tag = %q, want %q", i, got, want)
		}
	}

	// 2. Verify rules reconciliation
	rules := cfg["routing"].(map[string]interface{})["rules"].([]interface{})
	if len(rules) != 12 {
		t.Fatalf("expected 12 rules, got %d", len(rules))
	}

	// Rule 0: dns-proxy -> primary-proxy
	if got := rules[0].(map[string]interface{})["outboundTag"]; got != "primary-proxy" {
		t.Errorf("rule 0 outboundTag = %v, want 'primary-proxy'", got)
	}
	// Rule 1: network tcp,udp -> primary-proxy
	if got := rules[1].(map[string]interface{})["outboundTag"]; got != "primary-proxy" {
		t.Errorf("rule 1 outboundTag = %v, want 'primary-proxy'", got)
	}
	// Rule 2: google proxy -> primary-proxy
	if got := rules[2].(map[string]interface{})["outboundTag"]; got != "primary-proxy" {
		t.Errorf("rule 2 outboundTag = %v, want 'primary-proxy'", got)
	}
	// Rule 3: dns-out preserved
	if got := rules[3].(map[string]interface{})["outboundTag"]; got != "dns-out" {
		t.Errorf("rule 3 outboundTag = %v, want 'dns-out'", got)
	}
	// Rule 4: direct preserved
	if got := rules[4].(map[string]interface{})["outboundTag"]; got != "direct" {
		t.Errorf("rule 4 outboundTag = %v, want 'direct'", got)
	}
	// Rule 5: block preserved
	if got := rules[5].(map[string]interface{})["outboundTag"]; got != "block" {
		t.Errorf("rule 5 outboundTag = %v, want 'block'", got)
	}
	// Rule 6: secondary-proxy preserved
	if got := rules[6].(map[string]interface{})["outboundTag"]; got != "secondary-proxy" {
		t.Errorf("rule 6 outboundTag = %v, want 'secondary-proxy'", got)
	}
	// Rule 7: balancerTag preserved, outboundTag still nil
	if got := rules[7].(map[string]interface{})["balancerTag"]; got != "all-balancers" {
		t.Errorf("rule 7 balancerTag = %v, want 'all-balancers'", got)
	}
	if got, exists := rules[7].(map[string]interface{})["outboundTag"]; exists {
		t.Errorf("rule 7 unexpected outboundTag = %v", got)
	}
	// Rule 8: empty outboundTag rewritten to primary-proxy
	if got := rules[8].(map[string]interface{})["outboundTag"]; got != "primary-proxy" {
		t.Errorf("rule 8 outboundTag = %v, want 'primary-proxy'", got)
	}
	// Rule 9: invalid type outboundTag rewritten to primary-proxy
	if got := rules[9].(map[string]interface{})["outboundTag"]; got != "primary-proxy" {
		t.Errorf("rule 9 outboundTag = %v, want 'primary-proxy'", got)
	}
	// Rule 10: malformed string remains string untouched
	if got := rules[10]; got != "malformed-rule-entry" {
		t.Errorf("rule 10 = %v, want 'malformed-rule-entry'", got)
	}
	// Rule 11: trimmed dns-proxy rewritten to primary-proxy
	if got := rules[11].(map[string]interface{})["outboundTag"]; got != "primary-proxy" {
		t.Errorf("rule 11 outboundTag = %v, want 'primary-proxy'", got)
	}
}

func TestGenerateXrayConfigReconcilesNoActiveEndpoints(t *testing.T) {
	template := map[string]interface{}{
		"outbounds": []interface{}{
			map[string]interface{}{"protocol": "freedom", "tag": "direct"},
		},
		"routing": map[string]interface{}{
			"rules": []interface{}{
				map[string]interface{}{
					"type":        "field",
					"inboundTag":  []interface{}{"dns-proxy"},
					"outboundTag": "usp",
				},
			},
		},
	}

	cfg, err := GenerateXrayConfigFromTemplate(template, nil, testXrayUser())
	if err != nil {
		t.Fatalf("GenerateXrayConfigFromTemplate failed: %v", err)
	}

	outbounds := cfg["outbounds"].([]interface{})
	if len(outbounds) != 1 || outbounds[0].(map[string]interface{})["tag"] != "direct" {
		t.Fatalf("expected only template outbound, got %#v", outbounds)
	}

	rules := cfg["routing"].(map[string]interface{})["rules"].([]interface{})
	if got := rules[0].(map[string]interface{})["outboundTag"]; got != "usp" {
		t.Errorf("rule outboundTag should remain unchanged when no user nodes exist, got %v", got)
	}
}

func TestGenerateXrayConfigSelectsFirstActiveEndpoint(t *testing.T) {
	inactiveEp := &model.Endpoint{
		Node:     testXrayNode(),
		Tag:      "inactive-ep",
		UUID:     "11111111-1111-4111-8111-111111111111",
		IsActive: false,
	}

	inactiveNode := testXrayNode()
	inactiveNode.IsActive = false
	inactiveNodeEp := &model.Endpoint{
		Node:     inactiveNode,
		Tag:      "inactive-node",
		UUID:     "11111111-1111-4111-8111-111111111111",
		IsActive: true,
	}

	tuicNode := &model.Node{
		Tag:           "tuic-node",
		Protocol:      "tuic",
		ServerAddress: "tuic.example.com",
		ServerPort:    443,
		IsActive:      true,
	}
	tuicEp := &model.Endpoint{
		Node:     tuicNode,
		Tag:      "tuic-node",
		UUID:     "11111111-1111-4111-8111-111111111111",
		IsActive: true,
	}

	activeNode := testXrayNode()
	activeNode.Tag = "active-vless"
	activeEp := &model.Endpoint{
		Node:     activeNode,
		Tag:      "active-vless",
		UUID:     "11111111-1111-4111-8111-111111111111",
		IsActive: true,
	}

	template := map[string]interface{}{
		"outbounds": []interface{}{
			map[string]interface{}{"protocol": "freedom", "tag": "direct"},
		},
		"routing": map[string]interface{}{
			"rules": []interface{}{
				map[string]interface{}{"inboundTag": "dns-proxy", "outboundTag": "usp"},
				map[string]interface{}{"domain": []interface{}{"geosite:google"}, "outboundTag": "proxy"},
			},
		},
	}

	cfg, err := GenerateXrayConfigFromTemplate(template, []*model.Endpoint{inactiveEp, inactiveNodeEp, tuicEp, activeEp}, testXrayUser())
	if err != nil {
		t.Fatalf("GenerateXrayConfigFromTemplate failed: %v", err)
	}

	outbounds := cfg["outbounds"].([]interface{})
	if len(outbounds) != 2 {
		t.Fatalf("expected 2 outbounds (1 active node + 1 template), got %d", len(outbounds))
	}
	if outbounds[0].(map[string]interface{})["tag"] != "active-vless" {
		t.Fatalf("expected outbounds[0] to be active-vless, got %v", outbounds[0])
	}

	rules := cfg["routing"].(map[string]interface{})["rules"].([]interface{})
	if rules[0].(map[string]interface{})["outboundTag"] != "active-vless" {
		t.Errorf("expected dns-proxy rule outboundTag to be active-vless, got %v", rules[0])
	}
	if rules[1].(map[string]interface{})["outboundTag"] != "active-vless" {
		t.Errorf("expected proxy rule outboundTag to be active-vless, got %v", rules[1])
	}
}

func TestGenerateXrayConfigWithProductionTemplate(t *testing.T) {
	origPath := config.XrayTemplatePath
	config.XrayTemplatePath = filepath.Join("..", "..", "..", "templates", "xray.json")
	t.Cleanup(func() {
		config.XrayTemplatePath = origPath
	})

	node := testXrayNode()
	node.Tag = "hongkong-01"
	endpoint := &model.Endpoint{
		Node:     node,
		Tag:      "hongkong-01",
		UUID:     "11111111-1111-4111-8111-111111111111",
		IsActive: true,
	}

	cfg, err := GenerateXrayConfigFromEndpoints([]*model.Endpoint{endpoint}, testXrayUser())
	if err != nil {
		t.Fatalf("GenerateXrayConfigFromEndpoints failed with production template: %v", err)
	}

	outbounds, ok := cfg["outbounds"].([]interface{})
	if !ok || len(outbounds) < 4 {
		t.Fatalf("expected at least 4 outbounds (hongkong-01 + direct + block + dns-out), got %d", len(outbounds))
	}

	// Verify outbounds[0] is user node
	if got := outbounds[0].(map[string]interface{})["tag"]; got != "hongkong-01" {
		t.Fatalf("expected outbounds[0] tag = 'hongkong-01', got %v", got)
	}

	// Verify template outbounds follow
	expectedTags := []string{"hongkong-01", "direct", "block", "dns-out"}
	for i, want := range expectedTags {
		got := outbounds[i].(map[string]interface{})["tag"].(string)
		if got != want {
			t.Errorf("outbound[%d] tag = %q, want %q", i, got, want)
		}
	}

	// Verify routing rules reconciliation
	rules := cfg["routing"].(map[string]interface{})["rules"].([]interface{})
	dnsProxyFound := false
	for _, raw := range rules {
		rule := raw.(map[string]interface{})
		if xrayRuleHasInboundTag(rule, "dns-proxy") {
			dnsProxyFound = true
			if rule["outboundTag"] != "hongkong-01" {
				t.Errorf("dns-proxy rule outboundTag = %v, want 'hongkong-01'", rule["outboundTag"])
			}
		}
		if xrayRuleHasInboundTag(rule, "dns-direct") {
			if rule["outboundTag"] != "direct" {
				t.Errorf("dns-direct rule outboundTag = %v, want 'direct'", rule["outboundTag"])
			}
		}
	}
	if !dnsProxyFound {
		t.Error("dns-proxy rule was not found in production template rules")
	}
}
