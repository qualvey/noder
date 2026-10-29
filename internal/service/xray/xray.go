package xray

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"noder/internal/config"
	"noder/internal/contract"
	"noder/internal/model"
)

// DefaultXrayTemplateJSON is intentionally a small, client-side configuration.
// Its empty outbounds array is populated with the user's active VLESS nodes,
// making the first active node Xray's default outbound.
const DefaultXrayTemplateJSON = `{
  "log": {"loglevel": "warning"},
  "inbounds": [
    {
      "listen": "127.0.0.1",
      "port": 1080,
      "protocol": "socks",
      "settings": {"auth": "noauth", "udp": true},
      "tag": "socks-in"
    },
	{
      "protocol": "tun",
      "settings": {
        "name": "utun10",
        "desc": "Wintun",
        "mtu": 1500,
        "autoSystemRoutingTable": ["0.0.0.0/0", "::/0"],
        "autoOutboundsInterface": "auto"
      }
    }
  ],
  "outbounds": [],
  "routing": {"domainStrategy": "AsIs", "rules": []}
}`

func LoadXrayTemplate() (map[string]interface{}, error) {
	data, err := os.ReadFile(config.XrayTemplatePath)
	if err != nil || len(data) == 0 {
		var cfg map[string]interface{}
		_ = json.Unmarshal([]byte(DefaultXrayTemplateJSON), &cfg)
		return cfg, nil
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("Failed to unmarshal xray template: %v", err)
		return nil, err
	}
	return cfg, nil
}

func BuildXrayEndpointOutbound(ep *model.Endpoint) (map[string]interface{}, error) {
	if ep == nil || ep.Node == nil {
		return nil, errors.New("endpoint and node are required")
	}
	node := ep.Node
	proto := strings.ToLower(strings.TrimSpace(node.Protocol))
	if proto != "vless" {
		return nil, contract.NewBadRequest(fmt.Sprintf("Xray only supports vless, got '%s'", node.Protocol))
	}
	if node.Security != "reality" {
		return nil, contract.NewBadRequest("Xray VLESS outbound requires reality security")
	}
	if node.ServerAddress == "" || node.ServerPort <= 0 {
		return nil, contract.NewBadRequest("Xray VLESS outbound requires server address and port")
	}
	if ep.UUID == "" {
		return nil, contract.NewBadRequest("Xray VLESS outbound requires user UUID")
	}
	if node.SNI == nil || *node.SNI == "" || node.PublicKey == nil || *node.PublicKey == "" ||
		node.ShortID == nil || *node.ShortID == "" || node.Fingerprint == nil || *node.Fingerprint == "" {
		return nil, contract.NewBadRequest("Xray VLESS Reality outbound requires sni, public_key, short_id and fingerprint")
	}

	tag := ep.Tag
	if tag == "" {
		tag = node.Tag
		if tag == "" {
			tag = node.NodeName
		}
	}
	if tag == "" {
		return nil, contract.NewBadRequest("Xray VLESS outbound requires node tag")
	}

	merged := map[string]interface{}{
		"tag":            tag,
		"node_name":      node.NodeName,
		"protocol":       proto,
		"server_address": node.ServerAddress,
		"server_port":    node.ServerPort,
		"security":       node.Security,
		"uuid":           ep.UUID,
		"sni":            *node.SNI,
		"public_key":     *node.PublicKey,
		"short_id":       *node.ShortID,
		"fingerprint":    *node.Fingerprint,
	}
	if node.Flow != nil {
		merged["flow"] = *node.Flow
	}
	if err := contract.ValidateNodeContract(merged, proto, false); err != nil {
		return nil, err
	}

	settings := map[string]interface{}{
		"address":    node.ServerAddress,
		"port":       node.ServerPort,
		"id":         ep.UUID,
		"encryption": "none",
	}
	if node.Flow != nil && *node.Flow != "" {
		settings["flow"] = *node.Flow
	}

	return map[string]interface{}{
		"protocol": proto,
		"tag":      tag,
		"settings": settings,
		"streamSettings": map[string]interface{}{
			"method":   "raw",
			"security": "reality",
			"realitySettings": map[string]interface{}{
				"serverName":  *node.SNI,
				"fingerprint": *node.Fingerprint,
				"password":    *node.PublicKey,
				"shortId":     *node.ShortID,
			},
		},
	}, nil
}

func BuildXrayOutbound(node *model.Node, user *model.User) (map[string]interface{}, error) {
	if node == nil || user == nil {
		return nil, errors.New("node and user are required")
	}
	tag := node.Tag
	if tag == "" {
		tag = node.NodeName
	}
	userUUID := user.Token
	if user.UUID != nil && *user.UUID != "" {
		userUUID = *user.UUID
	}
	return BuildXrayEndpointOutbound(&model.Endpoint{
		Node: node,
		Tag:  tag,
		UUID: userUUID,
	})
}

func GenerateXrayConfigFromTemplate(cfg map[string]interface{}, endpoints []*model.Endpoint, user *model.User) (map[string]interface{}, error) {
	var outbounds []interface{}
	seenTags := make(map[string]bool)
	if raw, ok := cfg["outbounds"].([]interface{}); ok {
		outbounds = append(outbounds, raw...)
		for _, rawOutbound := range raw {
			if outbound, ok := rawOutbound.(map[string]interface{}); ok {
				if tag, ok := outbound["tag"].(string); ok && tag != "" {
					seenTags[tag] = true
				}
			}
		}
	}

	for _, ep := range endpoints {
		if ep == nil {
			continue
		}
		if !ep.IsActive || ep.Node == nil || !ep.Node.IsActive {
			continue
		}
		outbound, err := BuildXrayEndpointOutbound(ep)
		if err != nil {
			return nil, err
		}
		tag := outbound["tag"].(string)
		if seenTags[tag] {
			return nil, contract.NewBadRequest(fmt.Sprintf("duplicate Xray outbound tag: %s", tag))
		}
		seenTags[tag] = true
		outbounds = append(outbounds, outbound)
	}

	cfg["outbounds"] = outbounds
	return cfg, nil
}

func GenerateXrayConfigFromEndpoints(endpoints []*model.Endpoint, user *model.User) (map[string]interface{}, error) {
	cfg, err := LoadXrayTemplate()
	if err != nil {
		return nil, err
	}
	return GenerateXrayConfigFromTemplate(cfg, endpoints, user)
}

func GenerateXrayConfig(nodes []*model.Node, user *model.User) (map[string]interface{}, error) {
	if user == nil {
		return nil, errors.New("user is required")
	}
	var endpoints []*model.Endpoint
	userUUID := user.Token
	if user.UUID != nil && *user.UUID != "" {
		userUUID = *user.UUID
	}
	for i, n := range nodes {
		if n == nil {
			continue
		}
		tag := n.Tag
		if tag == "" {
			tag = n.NodeName
		}
		endpoints = append(endpoints, &model.Endpoint{
			Node:      n,
			Tag:       tag,
			UUID:      userUUID,
			IsActive:  n.IsActive,
			SortOrder: i,
		})
	}
	return GenerateXrayConfigFromEndpoints(endpoints, user)
}
