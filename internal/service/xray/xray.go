package xray

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"noder/internal/config"
	"noder/internal/contract"
	"noder/internal/model"
)

func LoadXrayTemplate() (map[string]interface{}, error) {
	data, err := os.ReadFile(config.XrayTemplatePath)
	if err != nil {
		log.Printf("Failed to read xray template: %v", err)
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("empty xray template")
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
	if node.Protocol != "vless" {
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
		"protocol": "vless",
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

func GenerateXrayConfigFromEndpoints(endpoints []*model.Endpoint, user *model.User) (map[string]interface{}, error) {
	cfg, err := LoadXrayTemplate()
	if err != nil {
		return nil, err
	}

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

func GenerateXrayConfig(nodes []*model.Node, user *model.User) (map[string]interface{}, error) {
	var endpoints []*model.Endpoint
	userUUID := user.Token
	if user.UUID != nil && *user.UUID != "" {
		userUUID = *user.UUID
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
			IsActive:  n.IsActive,
			SortOrder: i,
		})
	}
	return GenerateXrayConfigFromEndpoints(endpoints, user)
}
