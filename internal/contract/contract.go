package contract

import (
	"fmt"
	"net/http"
	"strings"
)

// APIError 自定义 HTTP 异常错误，包含状态码与错误详情
type APIError struct {
	StatusCode int
	Detail     string
}

func (e *APIError) Error() string {
	return e.Detail
}

func NewBadRequest(detail string) *APIError {
	return &APIError{StatusCode: http.StatusBadRequest, Detail: detail}
}

type NodeContract struct {
	Protocol  string
	Required  map[string]bool
	Optional  map[string]bool
	Deps      map[string]map[string][]string
	Fixed     map[string]string
	Enum      map[string][]string
	Conflicts [][4]string
}

var NodeOwnedFields = map[string]bool{
	"tag": true, "node_name": true, "server_address": true, "server_port": true,
	"security": true, "sni": true, "method": true, "transport_type": true,
	"path": true, "public_key": true, "short_id": true, "fingerprint": true,
	"flow": true, "remark": true, "congestion_control": true, "country": true,
}

var (
	UtlsFingerprints       = []string{"chrome", "firefox", "edge", "safari", "360", "qq", "ios", "android", "random", "randomized"}
	VlessFlows             = []string{"", "xtls-rprx-vision"}
	TuicCongestionControls = []string{"bbr", "cubic", "new_reno"}
	// Transport types shared by the node form. "xhttp" is intentionally only
	// rendered by Xray; sing-box and Mihomo reject it explicitly.
	TransportTypes = []string{"http", "ws", "quic", "grpc", "httpupgrade", "xhttp"}
)

// NormalizeTransportType returns a supported transport type or "direct" so
// invalid/legacy values cannot leak into generated sing-box configurations.
func NormalizeTransportType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "direct" {
		return value
	}
	for _, supported := range TransportTypes {
		if value == supported {
			return value
		}
	}
	return "direct"
}

var ProtocolContracts = map[string]*NodeContract{
	"tuic": {
		Protocol: "tuic",
		Required: map[string]bool{"tag": true, "server_address": true, "server_port": true, "uuid": true, "password": true},
		Optional: map[string]bool{"congestion_control": true},
		Fixed:    map[string]string{"security": "tls"},
		Enum:     map[string][]string{"congestion_control": TuicCongestionControls},
	},
	"vless": {
		Protocol: "vless",
		Required: map[string]bool{"tag": true, "server_address": true, "server_port": true, "uuid": true},
		Optional: map[string]bool{"node_name": true, "flow": true, "transport_type": true, "path": true},
		Deps: map[string]map[string][]string{
			"security": {
				"reality": {"public_key", "short_id", "sni", "fingerprint"},
				"tls":     {},
			},
		},
		Enum: map[string][]string{
			"flow":           VlessFlows,
			"fingerprint":    UtlsFingerprints,
			"security":       {"tls", "reality"},
			"transport_type": append([]string{"direct"}, TransportTypes...),
		},
	},
	"anytls": {
		Protocol: "anytls",
		Required: map[string]bool{"server_address": true, "server_port": true},
		Optional: map[string]bool{"node_name": true, "method": true, "sni": true, "transport_type": true, "path": true, "remark": true},
		Deps: map[string]map[string][]string{
			"security": {
				"tls":     {},
				"reality": {"public_key", "short_id"},
			},
		},
	},
	"socks5": {
		Protocol: "socks5",
		Required: map[string]bool{"server_address": true, "server_port": true},
		Optional: map[string]bool{"tag": true, "node_name": true, "country": true, "remark": true, "sort_order": true},
	},
}

func GetContract(protocol string) (*NodeContract, error) {
	proto := strings.ToLower(strings.TrimSpace(protocol))
	contract, ok := ProtocolContracts[proto]
	if !ok {
		return nil, NewBadRequest(fmt.Sprintf("Protocol '%s' is invalid. Allowed protocols: anytls, tuic, vless, socks5", protocol))
	}
	return contract, nil
}

func isEmpty(v interface{}) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val) == ""
	case *string:
		return val == nil || strings.TrimSpace(*val) == ""
	default:
		return false
	}
}

func getString(values map[string]interface{}, key string) (string, bool) {
	v, ok := values[key]
	if !ok || v == nil {
		return "", false
	}
	switch val := v.(type) {
	case string:
		return val, true
	case *string:
		if val == nil {
			return "", false
		}
		return *val, true
	default:
		return fmt.Sprintf("%v", val), true
	}
}

func ValidateNodeContract(values map[string]interface{}, protocol string, nodeLevel bool) error {
	contract, err := GetContract(protocol)
	if err != nil {
		return err
	}

	// 1. 必填字段
	var missing []string
	for field := range contract.Required {
		if nodeLevel && !NodeOwnedFields[field] {
			continue
		}
		val, exists := values[field]
		if !exists || isEmpty(val) {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		return NewBadRequest(fmt.Sprintf("[%s] 缺少必填字段: %s", protocol, strings.Join(missing, ", ")))
	}

	// xhttp is an Xray-only VLESS transport. It cannot be combined with
	// Reality and needs the reverse-proxy hostname in SNI.
	transport, _ := getString(values, "transport_type")
	if strings.EqualFold(strings.TrimSpace(transport), "xhttp") {
		if strings.ToLower(strings.TrimSpace(protocol)) != "vless" {
			return NewBadRequest("xhttp 仅支持 VLESS 协议")
		}
		security, _ := getString(values, "security")
		if strings.ToLower(strings.TrimSpace(security)) != "tls" {
			return NewBadRequest("xhttp 仅支持与 TLS 结合，不支持 Reality")
		}
		sni, _ := getString(values, "sni")
		if strings.TrimSpace(sni) == "" {
			return NewBadRequest("xhttp 必须填写反代域名 SNI")
		}
	}

	// 2. 固定取值
	for field, mustBe := range contract.Fixed {
		valStr, exists := getString(values, field)
		if !exists || strings.ToLower(valStr) != mustBe {
			var currVal interface{}
			if exists {
				currVal = valStr
			}
			return NewBadRequest(fmt.Sprintf("[%s] 字段 '%s' 必须为 '%s'（当前: %v）", protocol, field, mustBe, currVal))
		}
	}

	// 3. 条件依赖
	for field, valueMap := range contract.Deps {
		valStr, exists := getString(values, field)
		if exists {
			if depList, hasDep := valueMap[valStr]; hasDep {
				var missingDeps []string
				for _, depField := range depList {
					depVal, depExists := values[depField]
					if !depExists || isEmpty(depVal) {
						missingDeps = append(missingDeps, depField)
					}
				}
				if len(missingDeps) > 0 {
					return NewBadRequest(fmt.Sprintf("[%s] 字段 '%s=%s' 时，必须同时提供: %s", protocol, field, valStr, strings.Join(missingDeps, ", ")))
				}
			}
		}
	}

	// 4. 枚举取值
	for field, allowed := range contract.Enum {
		valStr, exists := getString(values, field)
		if exists {
			allowedMap := make(map[string]bool)
			for _, item := range allowed {
				allowedMap[item] = true
			}
			if !allowedMap[valStr] {
				return NewBadRequest(fmt.Sprintf("[%s] 字段 '%s' 取值无效: '%s'，允许: %s", protocol, field, valStr, strings.Join(allowed, ", ")))
			}
		}
	}

	return nil
}

type CoreInfo struct {
	Key                string
	Name               string
	SupportedProtocols map[string]bool
	ContentType        string
}

var CoreRegistry = map[string]*CoreInfo{
	"singbox": {
		Key:                "singbox",
		Name:               "Sing-Box",
		SupportedProtocols: map[string]bool{"tuic": true, "vless": true, "anytls": true},
		ContentType:        "application/json",
	},
	"mihomo": {
		Key:                "mihomo",
		Name:               "Mihomo",
		SupportedProtocols: map[string]bool{"vless": true, "tuic": true},
		ContentType:        "text/yaml",
	},
	"xray": {
		Key:                "xray",
		Name:               "Xray",
		SupportedProtocols: map[string]bool{"vless": true},
		ContentType:        "application/json",
	},
}

func GetCore(core string) (*CoreInfo, error) {
	key := strings.ToLower(strings.TrimSpace(core))
	if key == "" {
		key = "singbox"
	}
	info, ok := CoreRegistry[key]
	if !ok {
		return nil, NewBadRequest(fmt.Sprintf("Unsupported core '%s'. Available cores: mihomo, singbox, xray", core))
	}
	return info, nil
}

func AssertProtocolSupported(core *CoreInfo, protocol string) error {
	proto := strings.ToLower(strings.TrimSpace(protocol))
	if !core.SupportedProtocols[proto] {
		var protos []string
		for p := range core.SupportedProtocols {
			protos = append(protos, p)
		}
		return NewBadRequest(fmt.Sprintf("核心 %s 不支持协议 '%s'（该核心支持的协议: %s）。请修改或移除该节点。", core.Name, protocol, strings.Join(protos, ", ")))
	}
	return nil
}
