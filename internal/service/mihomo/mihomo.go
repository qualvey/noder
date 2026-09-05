package mihomo

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"noder/internal/config"
	"noder/internal/contract"
	"noder/internal/model"
)

var NodeProxyTypes = map[string]bool{
	"vless": true, "tuic": true, "anytls": true, "vmess": true, "ss": true, "ssr": true,
	"trojan": true, "hysteria": true, "hysteria2": true, "wireguard": true, "snell": true,
	"ssh": true, "shadowtls": true,
}

func BuildMihomoProxy(node *model.Node, user *model.User) (map[string]interface{}, error) {
	proto := strings.ToLower(node.Protocol)
	if proto == "" {
		proto = "vless"
	}

	coreInfo, err := contract.GetCore("mihomo")
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

	proxy := map[string]interface{}{
		"name":   tag,
		"type":   proto,
		"server": node.ServerAddress,
		"port":   node.ServerPort,
	}

	if proto == "vless" {
		proxy["uuid"] = userUUID
		proxy["network"] = "tcp"
		proxy["udp"] = true
		proxy["tls"] = true

		sn := node.ServerAddress
		if node.SNI != nil && *node.SNI != "" {
			sn = *node.SNI
		}
		proxy["servername"] = sn

		flow := "xtls-rprx-vision"
		if node.Flow != nil && *node.Flow != "" {
			flow = *node.Flow
		}
		proxy["flow"] = flow

		if node.Security == "reality" {
			pk := ""
			if node.PublicKey != nil {
				pk = *node.PublicKey
			}
			sid := ""
			if node.ShortID != nil {
				sid = *node.ShortID
			}
			proxy["reality-opts"] = map[string]interface{}{
				"public-key": pk,
				"short-id":   sid,
			}
		}

		fp := "chrome"
		if node.Fingerprint != nil && *node.Fingerprint != "" {
			fp = *node.Fingerprint
		}
		proxy["client-fingerprint"] = fp

	} else if proto == "tuic" {
		proxy["uuid"] = userUUID
		proxy["password"] = userPassword
		proxy["alpn"] = []string{"h3"}
		cc := "bbr"
		if node.CongestionControl != nil && *node.CongestionControl != "" {
			cc = *node.CongestionControl
		}
		proxy["congestion-controller"] = cc
		if node.SNI != nil && *node.SNI != "" {
			proxy["sni"] = *node.SNI
		}
		proxy["reduce-rtt"] = true
	}

	return proxy, nil
}

func LoadMihomoTemplate() map[string]interface{} {
	if data, err := os.ReadFile(config.MihomoTemplatePath); err == nil && len(data) > 0 {
		var res map[string]interface{}
		if err := yaml.Unmarshal(data, &res); err == nil {
			return res
		}
	}
	return map[string]interface{}{
		"mixed-port": 7890,
		"log-level":  "info",
		"mode":       "rule",
		"proxies":    []interface{}{},
		"proxy-groups": []interface{}{
			map[string]interface{}{"name": "Proxy", "type": "select", "proxies": []interface{}{"auto"}},
			map[string]interface{}{
				"name": "auto", "type": "url-test", "proxies": []interface{}{},
				"url": "http://www.gstatic.com/generate_204", "interval": 300,
			},
		},
		"rules": []interface{}{"MATCH,Proxy"},
	}
}

func GenerateMihomoConfig(nodes []*model.Node, user *model.User) (map[string]interface{}, error) {
	cfg := LoadMihomoTemplate()

	var tplProxies []interface{}
	if raw, ok := cfg["proxies"].([]interface{}); ok {
		tplProxies = raw
	}

	var specialProxies []map[string]interface{}
	for _, raw := range tplProxies {
		if p, ok := raw.(map[string]interface{}); ok {
			pType, _ := p["type"].(string)
			if !NodeProxyTypes[pType] {
				specialProxies = append(specialProxies, p)
			}
		}
	}

	var realProxies []map[string]interface{}
	var proxyNames []string
	for _, n := range nodes {
		if n.IsActive {
			p, err := BuildMihomoProxy(n, user)
			if err != nil {
				return nil, err
			}
			realProxies = append(realProxies, p)
			name, _ := p["name"].(string)
			proxyNames = append(proxyNames, name)
		}
	}

	// 1. proxies: specialProxies + realProxies
	var mergedProxies []interface{}
	for _, p := range specialProxies {
		mergedProxies = append(mergedProxies, p)
	}
	for _, p := range realProxies {
		mergedProxies = append(mergedProxies, p)
	}
	cfg["proxies"] = mergedProxies

	// 2. proxy-groups
	if groups, ok := cfg["proxy-groups"].([]interface{}); ok {
		groupNames := make(map[string]bool)
		for _, raw := range groups {
			if g, ok := raw.(map[string]interface{}); ok {
				name, _ := g["name"].(string)
				groupNames[name] = true
			}
		}

		realNameSet := make(map[string]bool)
		for _, name := range proxyNames {
			realNameSet[name] = true
		}

		for _, raw := range groups {
			if g, ok := raw.(map[string]interface{}); ok {
				gname, _ := g["name"].(string)
				gtype, _ := g["type"].(string)

				var members []string
				if rawMembers, ok := g["proxies"].([]interface{}); ok {
					for _, m := range rawMembers {
						if str, ok := m.(string); ok {
							if groupNames[str] || realNameSet[str] {
								members = append(members, str)
							}
						}
					}
				}

				if gname == "Proxy" || gtype == "select" {
					memSet := make(map[string]bool)
					for _, m := range members {
						memSet[m] = true
					}
					for _, n := range proxyNames {
						if !memSet[n] && n != gname {
							members = append(members, n)
							memSet[n] = true
						}
					}
				} else if gtype == "url-test" || gtype == "fallback" || gtype == "load-balance" {
					members = append([]string(nil), proxyNames...)
				}

				var memberInterfaces []interface{}
				for _, m := range members {
					memberInterfaces = append(memberInterfaces, m)
				}
				g["proxies"] = memberInterfaces
			}
		}
	}

	return cfg, nil
}

func BuildMihomoConfigYAML(nodes []*model.Node, user *model.User) ([]byte, error) {
	cfg, err := GenerateMihomoConfig(nodes, user)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(cfg)
}
