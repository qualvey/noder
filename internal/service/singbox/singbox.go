package singbox

import (
	"errors"
	"encoding/json"
	"log"
	"os"

	"noder/internal/config"
	"noder/internal/contract"
	"noder/internal/model"
)

func BuildSingboxEndpointOutbound(ep *model.Endpoint) (map[string]interface{}, error) {
	if ep == nil || ep.Node == nil {
		return nil, errors.New("endpoint and node are required")
	}
	node := ep.Node
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

	tag := ep.Tag
	if tag == "" {
		tag = node.Tag
		if tag == "" {
			tag = node.NodeName
		}
	}

	userUUID := ep.UUID
	userPassword := ""
	if ep.Password != nil {
		userPassword = *ep.Password
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

func BuildSingboxOutbound(node *model.Node, user *model.User) (map[string]interface{}, error) {
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
	return BuildSingboxEndpointOutbound(&model.Endpoint{
		Node:     node,
		Tag:      tag,
		UUID:     userUUID,
		Password: &userPassword,
	})
}

func LoadSingboxTemplate() (map[string]interface{}, error) {
	data, err := os.ReadFile(config.SingBoxTemplatePath)
	if err != nil {
		log.Printf("Failed to read singbox template: %v", err)
		return nil, err
	}
	if len(data) == 0 {
		log.Printf("Singbox template file is empty")
		return nil, errors.New("empty singbox template")
	}
	var res map[string]interface{}
	if err := json.Unmarshal(data, &res); err != nil {
		log.Printf("Failed to unmarshal singbox template: %v", err)
		return nil, err
	}
	return res, nil
}

func GenerateSingboxConfigFromEndpoints(endpoints []*model.Endpoint, user *model.User) (map[string]interface{}, error) {
	var activeEndpoints []*model.Endpoint
	for _, ep := range endpoints {
		if ep.IsActive && ep.Node != nil && ep.Node.IsActive {
			activeEndpoints = append(activeEndpoints, ep)
		}
	}

	var nodeOutbounds []map[string]interface{}
	var nodeTags []string
	for _, ep := range activeEndpoints {
		ob, err := BuildSingboxEndpointOutbound(ep)
		if err != nil {
			return nil, err
		}
		nodeOutbounds = append(nodeOutbounds, ob)
		nodeTags = append(nodeTags, ep.Tag)
	}

	cfg, err := LoadSingboxTemplate()
	if err != nil {
		return nil, err
	}

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
	if user != nil && user.ConfigOverride != nil && *user.ConfigOverride != "" {
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

func GenerateSingboxConfig(nodes []*model.Node, user *model.User) (map[string]interface{}, error) {
	var endpoints []*model.Endpoint
	userUUID := user.Token
	if user.UUID != nil && *user.UUID != "" {
		userUUID = *user.UUID
	}
	var pwd *string
	if user.Password != nil && *user.Password != "" {
		pwd = user.Password
	} else if user.Token != "" {
		pwd = &user.Token
	}

	for i, n := range nodes {
		tag := n.Tag
		if tag == "" {
			tag = n.NodeName
		}
		endpoints = append(endpoints, &model.Endpoint{
			Node:      n,
			Tag:       tag,
			UUID:      userUUID,
			Password:  pwd,
			IsActive:  n.IsActive,
			SortOrder: i,
		})
	}

	return GenerateSingboxConfigFromEndpoints(endpoints, user)
}
