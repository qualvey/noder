package singbox

import (
	"encoding/json"
	"os"

	"noder/internal/config"
	"noder/internal/contract"
	"noder/internal/model"
)

func BuildSingboxOutbound(node *model.Node, user *model.User) (map[string]interface{}, error) {
	proto := node.Protocol
	if proto == "" {
		proto = "vless"
	}

	coreInfo, err := contract.GetCore("singbox")
	if err != nil {
		return nil, err
	}
	if err := contract.AssertProtocolSupported(coreInfo, proto); err != nil {
		return nil, err
	}

	tag := node.Tag
	if tag == "" {
		tag = node.NodeName
	}

	userUUID := user.Token
	if user.UUID != nil && *user.UUID != "" {
		userUUID = *user.UUID
	}
	userPassword := user.Token
	if user.Password != nil && *user.Password != "" {
		userPassword = *user.Password
	}

	// 校验全量契约
	merged := map[string]interface{}{
		"tag":            tag,
		"node_name":      node.NodeName,
		"protocol":       proto,
		"server_address": node.ServerAddress,
		"server_port":    node.ServerPort,
		"security":       node.Security,
		"uuid":           userUUID,
		"password":       userPassword,
	}
	if node.SNI != nil {
		merged["sni"] = *node.SNI
	}
	if node.PublicKey != nil {
		merged["public_key"] = *node.PublicKey
	}
	if node.ShortID != nil {
		merged["short_id"] = *node.ShortID
	}
	if node.Fingerprint != nil {
		merged["fingerprint"] = *node.Fingerprint
	}
	if node.Flow != nil {
		merged["flow"] = *node.Flow
	}
	if node.CongestionControl != nil {
		merged["congestion_control"] = *node.CongestionControl
	}

	if err := contract.ValidateNodeContract(merged, proto, false); err != nil {
		return nil, err
	}

	outbound := map[string]interface{}{
		"type":        proto,
		"tag":         tag,
		"server":      node.ServerAddress,
		"server_port": node.ServerPort,
	}

	switch proto {
	case "tuic":
		outbound["uuid"] = userUUID
		outbound["password"] = userPassword
		cc := "bbr"
		if node.CongestionControl != nil && *node.CongestionControl != "" {
			cc = *node.CongestionControl
		}
		outbound["congestion_control"] = cc
		outbound["zero_rtt_handshake"] = false
		tlsConfig := map[string]interface{}{
			"enabled": true,
			"alpn":    []string{"h3"},
		}
		if node.SNI != nil && *node.SNI != "" {
			tlsConfig["server_name"] = *node.SNI
		}
		outbound["tls"] = tlsConfig

	case "vless":
		outbound["uuid"] = userUUID
		flow := "xtls-rprx-vision"
		if node.Flow != nil && *node.Flow != "" {
			flow = *node.Flow
		}
		outbound["flow"] = flow

		if node.Security == "tls" || node.Security == "reality" {
			fp := "chrome"
			if node.Fingerprint != nil && *node.Fingerprint != "" {
				fp = *node.Fingerprint
			}
			tlsConfig := map[string]interface{}{
				"enabled": true,
				"utls": map[string]interface{}{
					"enabled":     true,
					"fingerprint": fp,
				},
			}
			if node.SNI != nil && *node.SNI != "" {
				tlsConfig["server_name"] = *node.SNI
			}
			if node.Security == "reality" {
				pk := ""
				if node.PublicKey != nil {
					pk = *node.PublicKey
				}
				sid := ""
				if node.ShortID != nil {
					sid = *node.ShortID
				}
				tlsConfig["reality"] = map[string]interface{}{
					"enabled":    true,
					"public_key": pk,
					"short_id":   sid,
				}
			}
			outbound["tls"] = tlsConfig
		}

	case "anytls":
		outbound["password"] = userPassword
		if node.Security == "tls" || node.Security == "reality" {
			tlsConfig := map[string]interface{}{
				"enabled": true,
			}
			if node.SNI != nil && *node.SNI != "" {
				tlsConfig["server_name"] = *node.SNI
			}
			if node.Security == "reality" {
				pk := ""
				if node.PublicKey != nil {
					pk = *node.PublicKey
				}
				sid := ""
				if node.ShortID != nil {
					sid = *node.ShortID
				}
				tlsConfig["reality"] = map[string]interface{}{
					"enabled":    true,
					"public_key": pk,
					"short_id":   sid,
				}
			}
			outbound["tls"] = tlsConfig
		}
	}

	if node.TransportType != "" && node.TransportType != "direct" {
		trans := map[string]interface{}{
			"type": node.TransportType,
		}
		if node.Path != nil && *node.Path != "" {
			trans["path"] = *node.Path
		}
		outbound["transport"] = trans
	}

	return outbound, nil
}

func LoadSingboxTemplate() map[string]interface{} {
	if data, err := os.ReadFile(config.SingBoxTemplatePath); err == nil && len(data) > 0 {
		var res map[string]interface{}
		if err := json.Unmarshal(data, &res); err == nil {
			return res
		}
	}
	return map[string]interface{}{
		"log": map[string]interface{}{"level": "info", "timestamp": true},
		"outbounds": []interface{}{
			map[string]interface{}{"type": "direct", "tag": "direct"},
			map[string]interface{}{"tag": "Proxy", "type": "selector", "outbounds": []interface{}{"urltest"}},
			map[string]interface{}{"tag": "urltest", "type": "urltest", "outbounds": []interface{}{}},
		},
	}
}

func GenerateSingboxConfig(nodes []*model.Node, user *model.User) (map[string]interface{}, error) {
	var activeNodes []*model.Node
	for _, n := range nodes {
		if n.IsActive {
			activeNodes = append(activeNodes, n)
		}
	}

	var nodeOutbounds []map[string]interface{}
	var nodeTags []string
	for _, n := range activeNodes {
		ob, err := BuildSingboxOutbound(n, user)
		if err != nil {
			return nil, err
		}
		nodeOutbounds = append(nodeOutbounds, ob)
		nodeTags = append(nodeTags, n.Tag)
	}

	cfg := LoadSingboxTemplate()

	var rawOutbounds []interface{}
	if obs, ok := cfg["outbounds"].([]interface{}); ok {
		rawOutbounds = obs
	}

	// 1. 注入 selector 与 urltest
	for _, raw := range rawOutbounds {
		if ob, ok := raw.(map[string]interface{}); ok {
			tag, _ := ob["tag"].(string)
			obType, _ := ob["type"].(string)

			if tag == "Proxy" || obType == "selector" {
				existingList, _ := ob["outbounds"].([]interface{})
				seen := make(map[string]bool)
				for _, e := range existingList {
					if s, ok := e.(string); ok {
						seen[s] = true
					}
				}
				for _, nt := range nodeTags {
					if !seen[nt] {
						existingList = append(existingList, nt)
						seen[nt] = true
					}
				}
				ob["outbounds"] = existingList
			} else if tag == "urltest" || obType == "urltest" {
				var tagInterfaces []interface{}
				for _, nt := range nodeTags {
					tagInterfaces = append(tagInterfaces, nt)
				}
				ob["outbounds"] = tagInterfaces
			}
		}
	}

	// 2. 节点放入 outbounds 末尾
	for _, nob := range nodeOutbounds {
		rawOutbounds = append(rawOutbounds, nob)
	}
	cfg["outbounds"] = rawOutbounds

	// 3. 应用 config_override
	if user.ConfigOverride != nil && *user.ConfigOverride != "" {
		var override map[string]interface{}
		if err := json.Unmarshal([]byte(*user.ConfigOverride), &override); err == nil {
			for k, v := range override {
				if config.AllowedOverrideKeys[k] {
					cfg[k] = v
				}
			}
		}
	}

	return cfg, nil
}
