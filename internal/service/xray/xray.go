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

func BuildXrayOutbound(node *model.Node, user *model.User) (map[string]interface{}, error) {
	if node == nil || user == nil {
		return nil, errors.New("node and user are required")
	}
	if node.Protocol != "vless" {
		return nil, contract.NewBadRequest(fmt.Sprintf("Xray only supports vless, got '%s'", node.Protocol))
	}
	if node.Security != "reality" {
		return nil, contract.NewBadRequest("Xray VLESS outbound requires reality security")
	}
	if node.ServerAddress == "" || node.ServerPort <= 0 {
		return nil, contract.NewBadRequest("Xray VLESS outbound requires server address and port")
	}
	if user.UUID == nil || *user.UUID == "" {
		return nil, contract.NewBadRequest("Xray VLESS outbound requires user UUID")
	}
	if node.SNI == nil || *node.SNI == "" || node.PublicKey == nil || *node.PublicKey == "" ||
		node.ShortID == nil || *node.ShortID == "" || node.Fingerprint == nil || *node.Fingerprint == "" {
		return nil, contract.NewBadRequest("Xray VLESS Reality outbound requires sni, public_key, short_id and fingerprint")
	}

	tag := node.Tag
	if tag == "" {
		tag = node.NodeName
	}
	if tag == "" {
		return nil, contract.NewBadRequest("Xray VLESS outbound requires node tag")
	}

	settings := map[string]interface{}{
		"address":    node.ServerAddress,
		"port":       node.ServerPort,
		"id":         *user.UUID,
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

func GenerateXrayConfig(nodes []*model.Node, user *model.User) (map[string]interface{}, error) {
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

	for _, node := range nodes {
		if !node.IsActive {
			continue
		}
		outbound, err := BuildXrayOutbound(node, user)
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
