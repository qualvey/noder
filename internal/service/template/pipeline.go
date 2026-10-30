package template

import (
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v3"
	"noder/internal/model"
	"noder/internal/service/mihomo"
	"noder/internal/service/singbox"
	"noder/internal/service/xray"
)

// A template pipeline owns all target-specific behavior. The ZIP walker stays
// shared, but a sing-box render never builds or consumes Xray values (and vice
// versa).
type templatePipeline struct {
	buildContext              func(*model.User, []*model.Endpoint, bool, string) map[string]string
	shouldRenderLauncherEntry func(string, []byte) bool
	renderLauncherConfig      func(string, *model.User, map[string]string) string
}

func singboxPipeline() templatePipeline {
	return templatePipeline{
		buildContext:              buildSingboxTemplateContext,
		shouldRenderLauncherEntry: shouldRenderSingboxLauncherEntry,
		renderLauncherConfig:      RenderSingboxLauncherConfigText,
	}
}

func xrayPipeline() templatePipeline {
	return templatePipeline{
		buildContext:              buildXrayTemplateContext,
		shouldRenderLauncherEntry: shouldRenderXrayLauncherEntry,
		renderLauncherConfig:      RenderXrayLauncherConfigText,
	}
}

func pipelineForType(templateType string) templatePipeline {
	if NormalizeTemplateType(templateType) == TemplateTypeXray {
		return xrayPipeline()
	}
	return singboxPipeline()
}

func yamlDump(value interface{}) string {
	data, err := yaml.Marshal(value)
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(data), "\n")
}

func jsonIndent(value interface{}) string {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return ""
	}
	return string(data)
}

func activeEndpointsForSingbox(endpoints []*model.Endpoint) []*model.Endpoint {
	var active []*model.Endpoint
	for _, ep := range endpoints {
		if ep != nil && ep.IsActive && ep.Node != nil && ep.Node.IsActive {
			active = append(active, ep)
		}
	}
	return active
}

func activeEndpointsForXray(endpoints []*model.Endpoint) []*model.Endpoint {
	var active []*model.Endpoint
	for _, ep := range endpoints {
		if ep != nil && ep.IsActive && ep.Node != nil && ep.Node.IsActive && ep.Node.Protocol == "vless" {
			active = append(active, ep)
		}
	}
	return active
}

func commonTemplateContext(user *model.User, activeEndpoints []*model.Endpoint, includeMihomo bool) map[string]string {
	uuidStr := ""
	if user.UUID != nil && *user.UUID != "" {
		uuidStr = *user.UUID
	} else if len(activeEndpoints) > 0 {
		uuidStr = activeEndpoints[0].UUID
	}
	pwdStr := ""
	if user.Password != nil && *user.Password != "" {
		pwdStr = *user.Password
	} else if len(activeEndpoints) > 0 && activeEndpoints[0].Password != nil {
		pwdStr = *activeEndpoints[0].Password
	}

	nodeMeta := make([]map[string]interface{}, 0, len(activeEndpoints))
	for _, ep := range activeEndpoints {
		nodeMeta = append(nodeMeta, map[string]interface{}{
			"tag":         ep.Tag,
			"node_name":   ep.Node.NodeName,
			"protocol":    ep.Node.Protocol,
			"server":      ep.Node.ServerAddress,
			"server_port": ep.Node.ServerPort,
		})
	}

	ctx := map[string]string{
		"uuid":           uuidStr,
		"password":       pwdStr,
		"token":          user.Token,
		"name":           user.Name,
		"user_name":      user.Name,
		"node_list_yaml": yamlDump(nodeMeta),
		"node_list_json": jsonIndent(nodeMeta),
	}
	if includeMihomo {
		if mhBytes, err := mihomo.BuildMihomoConfigYAMLFromEndpoints(activeEndpoints, user); err == nil {
			var mhObj map[string]interface{}
			_ = yaml.Unmarshal(mhBytes, &mhObj)
			if proxies, ok := mhObj["proxies"]; ok {
				ctx["mihomo_proxies_yaml"] = yamlDump(proxies)
			}
		}
	}
	return ctx
}

func addBaseURL(ctx map[string]string, baseURL, key, suffix string) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL != "" {
		ctx[key] = baseURL + suffix
	}
}

func buildSingboxTemplateContext(user *model.User, endpoints []*model.Endpoint, includeMihomo bool, baseURL string) map[string]string {
	activeEndpoints := activeEndpointsForSingbox(endpoints)
	ctx := commonTemplateContext(user, activeEndpoints, includeMihomo)

	var outbounds []map[string]interface{}
	for _, ep := range activeEndpoints {
		if outbound, err := singbox.BuildSingboxEndpointOutbound(ep); err == nil {
			outbounds = append(outbounds, outbound)
		}
	}
	ctx["outbounds_yaml"] = yamlDump(outbounds)
	ctx["outbounds_json"] = jsonIndent(outbounds)
	ctx["singbox_outbounds_yaml"] = yamlDump(outbounds)
	ctx["singbox_outbounds_json"] = jsonIndent(outbounds)
	addBaseURL(ctx, baseURL, "singbox_config_url", "/sub")
	if config, err := singbox.GenerateSingboxConfigFromEndpoints(activeEndpoints, user); err == nil {
		ctx["singbox_config_json"] = jsonIndent(config)
	}
	return ctx
}

func buildXrayTemplateContext(user *model.User, endpoints []*model.Endpoint, includeMihomo bool, baseURL string) map[string]string {
	activeEndpoints := activeEndpointsForXray(endpoints)
	ctx := commonTemplateContext(user, activeEndpoints, includeMihomo)

	var outbounds []map[string]interface{}
	for _, ep := range activeEndpoints {
		if outbound, err := xray.BuildXrayEndpointOutbound(ep); err == nil {
			outbounds = append(outbounds, outbound)
		}
	}
	ctx["outbounds_yaml"] = yamlDump(outbounds)
	ctx["outbounds_json"] = jsonIndent(outbounds)
	ctx["xray_outbounds_yaml"] = yamlDump(outbounds)
	ctx["xray_outbounds_json"] = jsonIndent(outbounds)
	addBaseURL(ctx, baseURL, "xray_config_url", "/xray")
	if config, err := xray.GenerateXrayConfigFromEndpoints(activeEndpoints, user); err == nil {
		ctx["xray_config_json"] = jsonIndent(config)
	}
	return ctx
}

func BuildTemplateContextFromEndpoints(user *model.User, endpoints []*model.Endpoint, includeMihomo bool) map[string]string {
	return buildSingboxTemplateContext(user, endpoints, includeMihomo, "")
}

func BuildTemplateContextFromEndpointsForTarget(user *model.User, endpoints []*model.Endpoint, includeMihomo bool, templateType string) map[string]string {
	return pipelineForType(templateType).buildContext(user, endpoints, includeMihomo, "")
}

func BuildTemplateContextFromEndpointsForTargetWithBaseURL(user *model.User, endpoints []*model.Endpoint, includeMihomo bool, templateType string, baseURL string) map[string]string {
	return pipelineForType(templateType).buildContext(user, endpoints, includeMihomo, baseURL)
}

func BuildTemplateContext(user *model.User, nodes []*model.Node, includeMihomo bool) map[string]string {
	var endpoints []*model.Endpoint
	userUUID := user.Token
	if user.UUID != nil && *user.UUID != "" {
		userUUID = *user.UUID
	}
	var password *string
	if user.Password != nil && *user.Password != "" {
		password = user.Password
	} else if user.Token != "" {
		password = &user.Token
	}
	for i, node := range nodes {
		tag := node.Tag
		if tag == "" {
			tag = node.NodeName
		}
		endpoints = append(endpoints, &model.Endpoint{
			Node:      node,
			Tag:       tag,
			UUID:      userUUID,
			Password:  password,
			IsActive:  node.IsActive,
			SortOrder: i,
		})
	}
	return BuildTemplateContextFromEndpoints(user, endpoints, includeMihomo)
}

func renderLauncherConfigText(text string, user *model.User, configURL string) string {
	if user == nil {
		return text
	}
	lines := strings.SplitAfter(text, "\n")
	for i, line := range lines {
		ending := ""
		body := line
		if strings.HasSuffix(body, "\n") {
			ending = "\n"
			body = strings.TrimSuffix(body, "\n")
		}
		if strings.HasSuffix(body, "\r") {
			ending = "\r" + ending
			body = strings.TrimSuffix(body, "\r")
		}
		colon := strings.IndexByte(body, ':')
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(body[:colon])
		var replacement string
		switch key {
		case "upstream_url":
			replacement = configURL
		case "token":
			replacement = user.Token
		default:
			continue
		}
		if replacement == "" {
			continue
		}
		value := strings.TrimSpace(body[colon+1:])
		suffix := ""
		if comment := strings.IndexByte(value, '#'); comment >= 0 {
			suffix = value[comment:]
			value = strings.TrimSpace(value[:comment])
		}
		quote := ""
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			quote = value[:1]
		}
		spaces := body[colon+1:]
		spaces = spaces[:len(spaces)-len(strings.TrimLeft(spaces, " \t"))]
		lines[i] = body[:colon+1] + spaces + quote + replacement + quote
		if suffix != "" {
			lines[i] += " " + suffix
		}
		lines[i] += ending
	}
	return strings.Join(lines, "")
}

func RenderSingboxLauncherConfigText(text string, user *model.User, ctx map[string]string) string {
	return renderLauncherConfigText(text, user, ctx["singbox_config_url"])
}

func RenderXrayLauncherConfigText(text string, user *model.User, ctx map[string]string) string {
	return renderLauncherConfigText(text, user, ctx["xray_config_url"])
}

func shouldRenderSingboxLauncherEntry(name string, data []byte) bool {
	text := strings.ToLower(string(data))
	base := strings.ToLower(name)
	if strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml") {
		return strings.Contains(text, "upstream_url:") &&
			(strings.Contains(text, "sing_box_bin:") || strings.Contains(text, "{{singbox_config_url}}"))
	}
	if strings.HasSuffix(base, ".json") {
		return strings.Contains(text, "{{singbox_config_json}}") || strings.Contains(text, "{{singbox_outbounds_json}}")
	}
	return false
}

func shouldRenderXrayLauncherEntry(name string, data []byte) bool {
	text := strings.ToLower(string(data))
	base := strings.ToLower(name)
	if strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml") {
		return strings.Contains(text, "upstream_url:") &&
			(strings.Contains(text, "xray_bin:") || strings.Contains(text, "{{xray_config_url}}"))
	}
	if strings.HasSuffix(base, ".json") {
		return strings.Contains(text, "{{xray_config_json}}") || strings.Contains(text, "{{xray_outbounds_json}}")
	}
	return false
}
