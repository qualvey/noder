package xray

import (
	"errors"
	"fmt"
	"log"
	"os"

	"noder/internal/config"
	"noder/internal/contract"
	"noder/internal/jsonutil"
	"noder/internal/model"
)

const DefaultXrayTemplateJSON = `{"outbounds": []}`

func LoadXrayTemplate() (map[string]interface{}, error) {
	data, err := os.ReadFile(config.XrayTemplatePath)
	if err != nil || len(data) == 0 {
		var cfg map[string]interface{}
		_ = jsonutil.Unmarshal([]byte(DefaultXrayTemplateJSON), &cfg)
		return cfg, nil
	}
	var cfg map[string]interface{}
	if err := jsonutil.Unmarshal(data, &cfg); err != nil {
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
	if node.ServerAddress == "" || node.ServerPort <= 0 {
		return nil, contract.NewBadRequest("Xray VLESS outbound requires server address and port")
	}
	if ep.UUID == "" {
		return nil, contract.NewBadRequest("Xray VLESS outbound requires user UUID")
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

	transportType := contract.NormalizeTransportType(node.TransportType)
	if transportType == "xhttp" {
		if node.Security != "tls" {
			return nil, contract.NewBadRequest("Xray xhttp outbound requires TLS security and does not support Reality")
		}
		if node.SNI == nil || *node.SNI == "" {
			return nil, contract.NewBadRequest("Xray xhttp outbound requires reverse-proxy SNI")
		}
		flow := ""
		if node.Flow != nil {
			flow = *node.Flow
		}
		path := "/"
		if node.Path != nil && *node.Path != "" {
			path = *node.Path
		}
		return map[string]interface{}{
			"tag":      tag,
			"protocol": "vless",
			"settings": map[string]interface{}{
				"vnext": []interface{}{map[string]interface{}{
					"address": node.ServerAddress,
					"port":    node.ServerPort,
					"users": []interface{}{map[string]interface{}{
						"id":         ep.UUID,
						"encryption": "none",
						"flow":       flow,
					}},
				}},
			},
			"streamSettings": map[string]interface{}{
				"network":  "xhttp",
				"security": "tls",
				"sockopt": map[string]interface{}{
					"mark": 1024,
				},
				"tlsSettings": map[string]interface{}{
					"serverName":    *node.SNI,
					"allowInsecure": false,
				},
				"xhttpSettings": map[string]interface{}{
					"path": path,
					"mode": "auto",
					"extra": map[string]interface{}{
						"xPaddingBytes": "100-1000",
					},
				},
			},
		}, nil
	}

	if node.Security != "reality" && node.Security != "tls" {
		return nil, contract.NewBadRequest("Xray VLESS outbound requires TLS or Reality security")
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

	if node.Security == "tls" {
		stream := map[string]interface{}{
			"method":   "raw",
			"security": "tls",
			"tlsSettings": map[string]interface{}{
				"allowInsecure": false,
			},
		}
		if node.SNI != nil && *node.SNI != "" {
			stream["tlsSettings"].(map[string]interface{})["serverName"] = *node.SNI
		}
		return map[string]interface{}{
			"protocol":       "vless",
			"tag":            tag,
			"settings":       settings,
			"streamSettings": stream,
		}, nil
	}

	if node.SNI == nil || *node.SNI == "" || node.PublicKey == nil || *node.PublicKey == "" ||
		node.ShortID == nil || *node.ShortID == "" || node.Fingerprint == nil || *node.Fingerprint == "" {
		return nil, contract.NewBadRequest("Xray VLESS Reality outbound requires sni, public_key, short_id and fingerprint")
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
		if !ep.IsActive || ep.Node == nil || !ep.Node.IsActive || ep.Node.Protocol != "vless" {
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
