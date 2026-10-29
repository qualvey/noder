package template

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"hash/crc32"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"noder/internal/logging"
	"noder/internal/model"
	"noder/internal/service/mihomo"
	"noder/internal/service/singbox"
	"noder/internal/service/xray"
)

const MihomoPlaceholder = "{{mihomo_proxies_yaml}}"

const (
	TemplateTypeAuto = "auto"
	TemplateTypeSing = "sing-box"
	TemplateTypeXray = "xray"
)

// NormalizeTemplateType accepts the values used by the admin UI and legacy
// spellings. Empty values are treated as auto for old DistFile rows.
func NormalizeTemplateType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case TemplateTypeXray:
		return TemplateTypeXray
	case TemplateTypeSing, "singbox", "sing_box":
		return TemplateTypeSing
	case "", TemplateTypeAuto:
		return TemplateTypeAuto
	default:
		return ""
	}
}

// DetectTemplateType inspects the selected configuration member. Xray uses
// protocol-based inbound/outbound objects, while Sing-Box uses type-based
// objects. Custom placeholder-only templates intentionally return empty.
func DetectTemplateType(data []byte) string {
	var value interface{}
	if json.Unmarshal(data, &value) == nil {
		if detected := detectTemplateTypeValue(value); detected != "" {
			return detected
		}
	}
	var yamlValue interface{}
	if yaml.Unmarshal(data, &yamlValue) == nil {
		if detected := detectTemplateTypeValue(yamlValue); detected != "" {
			return detected
		}
	}
	// Launcher archives often contain a YAML control file instead of a core
	// config. These stable keys let auto-detection classify those packages too.
	lower := strings.ToLower(string(data))
	if strings.Contains(lower, "xray_bin") || strings.Contains(lower, "xray.exe") || strings.Contains(lower, "/xray") {
		return TemplateTypeXray
	}
	if strings.Contains(lower, "sing_box_bin") || strings.Contains(lower, "sing-box.exe") || strings.Contains(lower, "/sub") {
		return TemplateTypeSing
	}
	return ""
}

// DetectTemplateTypeFromZip reads the selected ZIP member without modifying
// the archive. templateName may be empty, in which case the first text-like
// config member is inspected.
func DetectTemplateTypeFromZip(zipData []byte, templateName string) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return "", err
	}
	target := filepath.Base(templateName)
	for _, entry := range zr.File {
		if target == "" {
			lower := strings.ToLower(entry.Name)
			if !strings.HasSuffix(lower, ".json") && !strings.HasSuffix(lower, ".yaml") && !strings.HasSuffix(lower, ".yml") {
				continue
			}
		}
		if target != "" && filepath.Base(entry.Name) != target && entry.Name != templateName {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			return "", err
		}
		data, readErr := io.ReadAll(reader)
		_ = reader.Close()
		if readErr != nil {
			return "", readErr
		}
		return DetectTemplateType(data), nil
	}
	return "", zip.ErrFormat
}

func detectTemplateTypeValue(value interface{}) string {
	root, ok := value.(map[string]interface{})
	if !ok {
		return ""
	}
	for _, section := range []string{"outbounds", "inbounds"} {
		items, ok := root[section].([]interface{})
		if !ok {
			continue
		}
		for _, item := range items {
			object, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if _, ok := object["protocol"]; ok {
				return TemplateTypeXray
			}
			if _, ok := object["type"]; ok {
				return TemplateTypeSing
			}
		}
	}
	return ""
}

func BuildTemplateContextFromEndpoints(user *model.User, endpoints []*model.Endpoint, includeMihomo bool) map[string]string {
	return BuildTemplateContextFromEndpointsForTarget(user, endpoints, includeMihomo, TemplateTypeSing)
}

func BuildTemplateContextFromEndpointsForTarget(user *model.User, endpoints []*model.Endpoint, includeMihomo bool, templateType string) map[string]string {
	return BuildTemplateContextFromEndpointsForTargetWithBaseURL(user, endpoints, includeMihomo, templateType, "")
}

// BuildTemplateContextFromEndpointsForTargetWithBaseURL adds launcher
// subscription URLs to the regular per-user template context. The base URL is
// supplied by the request handler so reverse-proxy and configured public URLs
// are respected.
func BuildTemplateContextFromEndpointsForTargetWithBaseURL(user *model.User, endpoints []*model.Endpoint, includeMihomo bool, templateType string, baseURL string) map[string]string {
	templateType = NormalizeTemplateType(templateType)
	if templateType == TemplateTypeAuto || templateType == "" {
		templateType = TemplateTypeSing
	}

	var activeEndpoints []*model.Endpoint
	for _, ep := range endpoints {
		if ep.IsActive && ep.Node != nil && ep.Node.IsActive &&
			(templateType != TemplateTypeXray || ep.Node.Protocol == "vless") {
			activeEndpoints = append(activeEndpoints, ep)
		}
	}

	var nodeMeta []map[string]interface{}
	for _, ep := range activeEndpoints {
		nodeMeta = append(nodeMeta, map[string]interface{}{
			"tag":         ep.Tag,
			"node_name":   ep.Node.NodeName,
			"protocol":    ep.Node.Protocol,
			"server":      ep.Node.ServerAddress,
			"server_port": ep.Node.ServerPort,
		})
	}

	var singboxOutbounds []map[string]interface{}
	for _, ep := range activeEndpoints {
		if ob, err := singbox.BuildSingboxEndpointOutbound(ep); err == nil {
			singboxOutbounds = append(singboxOutbounds, ob)
		}
	}
	var xrayOutbounds []map[string]interface{}
	for _, ep := range activeEndpoints {
		if ob, err := xray.BuildXrayEndpointOutbound(ep); err == nil {
			xrayOutbounds = append(xrayOutbounds, ob)
		}
	}
	outbounds := singboxOutbounds
	if templateType == TemplateTypeXray {
		outbounds = xrayOutbounds
	}

	yamlDump := func(obj interface{}) string {
		out, err := yaml.Marshal(obj)
		if err != nil {
			return ""
		}
		return strings.TrimRight(string(out), "\n")
	}

	jsonIndent := func(obj interface{}) string {
		out, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			return ""
		}
		return string(out)
	}

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

	ctx := map[string]string{
		"uuid":                   uuidStr,
		"password":               pwdStr,
		"token":                  user.Token,
		"name":                   user.Name,
		"user_name":              user.Name,
		"node_list_yaml":         yamlDump(nodeMeta),
		"node_list_json":         jsonIndent(nodeMeta),
		"outbounds_yaml":         yamlDump(outbounds),
		"outbounds_json":         jsonIndent(outbounds),
		"singbox_outbounds_yaml": yamlDump(singboxOutbounds),
		"singbox_outbounds_json": jsonIndent(singboxOutbounds),
		"xray_outbounds_yaml":    yamlDump(xrayOutbounds),
		"xray_outbounds_json":    jsonIndent(xrayOutbounds),
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL != "" {
		ctx["singbox_config_url"] = baseURL + "/sub"
		ctx["xray_config_url"] = baseURL + "/xray"
	}
	if templateType != TemplateTypeXray {
		if singboxConfig, err := singbox.GenerateSingboxConfigFromEndpoints(activeEndpoints, user); err == nil {
			ctx["singbox_config_json"] = jsonIndent(singboxConfig)
		} else {
			logging.L().Warn("failed to build sing-box template context", "error", err, "active_endpoints", len(activeEndpoints))
		}
	}
	if templateType == TemplateTypeXray {
		if xrayConfig, err := xray.GenerateXrayConfigFromEndpoints(activeEndpoints, user); err == nil {
			ctx["xray_config_json"] = jsonIndent(xrayConfig)
		} else {
			logging.L().Warn("failed to build Xray template context", "error", err, "active_endpoints", len(activeEndpoints))
		}
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

	logging.L().Debug("template context built",
		"template_type", templateType,
		"active_endpoints", len(activeEndpoints),
		"singbox_outbounds", len(singboxOutbounds),
		"xray_outbounds", len(xrayOutbounds),
		"base_url_present", baseURL != "",
		"singbox_config_present", ctx["singbox_config_json"] != "",
		"xray_config_present", ctx["xray_config_json"] != "",
	)

	return ctx
}

func BuildTemplateContext(user *model.User, nodes []*model.Node, includeMihomo bool) map[string]string {
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
	return BuildTemplateContextFromEndpoints(user, endpoints, includeMihomo)
}

var keyedPattern = regexp.MustCompile(`(?i)(["']?[\w.\-]*?(?:token|uuid|password)["']?\s*[:=]\s*)(["']?)([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})(["']?)`)
var placeholderPattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)

func readAllWithProgress(r io.Reader, onRead func(uint64)) ([]byte, error) {
	var data bytes.Buffer
	chunk := make([]byte, 32*1024)
	var read uint64
	for {
		n, err := r.Read(chunk)
		if n > 0 {
			if _, writeErr := data.Write(chunk[:n]); writeErr != nil {
				return nil, writeErr
			}
			read += uint64(n)
			if onRead != nil {
				onRead(read)
			}
		}
		if err == io.EOF {
			return data.Bytes(), nil
		}
		if err != nil {
			return nil, err
		}
	}
}

type progressReader struct {
	reader io.Reader
	read   uint64
	onRead func(uint64)
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.read += uint64(n)
		if r.onRead != nil {
			r.onRead(r.read)
		}
	}
	return n, err
}

func zipEntryProgressWeight(f *zip.File, rendered bool) uint64 {
	weight := f.CompressedSize64
	if rendered {
		// Template data is decompressed and then rendered, so count both passes.
		weight = f.UncompressedSize64 * 2
	}
	if weight == 0 {
		return 1
	}
	return weight
}

func RenderTemplateText(text string, user *model.User, nodes []*model.Node, knownTokens []string) string {
	return RenderTemplateTextWithContext(text, user, BuildTemplateContext(user, nodes, strings.Contains(text, MihomoPlaceholder)), knownTokens)
}

func RenderTemplateTextFromEndpoints(text string, user *model.User, endpoints []*model.Endpoint, knownTokens []string) string {
	return RenderTemplateTextWithContext(text, user, BuildTemplateContextFromEndpoints(user, endpoints, strings.Contains(text, MihomoPlaceholder)), knownTokens)
}

func RenderTemplateTextWithContext(text string, user *model.User, ctx map[string]string, knownTokens []string) string {

	for k, v := range ctx {
		text = strings.ReplaceAll(text, "{{"+k+"}}", v)
	}
	// Accept the common spaced form as well, e.g. {{ xray_config_url }}.
	text = placeholderPattern.ReplaceAllStringFunc(text, func(m string) string {
		sub := placeholderPattern.FindStringSubmatch(m)
		if len(sub) != 2 {
			return m
		}
		if value, ok := ctx[sub[1]]; ok {
			return value
		}
		return m
	})

	// 键名感知替换
	text = keyedPattern.ReplaceAllStringFunc(text, func(m string) string {
		sub := keyedPattern.FindStringSubmatch(m)
		if len(sub) == 5 {
			keyPart := sub[1]
			qL := sub[2]
			qR := sub[4]

			cleanKey := strings.ToLower(strings.Trim(strings.TrimRight(strings.TrimSpace(keyPart), ":="), "\"'"))
			if strings.HasSuffix(cleanKey, "token") {
				return keyPart + qL + user.Token + qR
			}
			if strings.HasSuffix(cleanKey, "uuid") {
				u := ""
				if user.UUID != nil {
					u = *user.UUID
				}
				return keyPart + qL + u + qR
			}
			if strings.HasSuffix(cleanKey, "password") {
				p := ""
				if user.Password != nil {
					p = *user.Password
				}
				return keyPart + qL + p + qR
			}
		}
		return m
	})

	// 兜底已知 token 替换
	for _, tok := range knownTokens {
		if tok != "" && strings.Contains(text, tok) {
			text = strings.ReplaceAll(text, tok, user.Token)
		}
	}

	return text
}

func RenderZipForUser(zipData []byte, templateName *string, user *model.User, nodes []*model.Node, knownTokens []string) ([]byte, error) {
	return RenderZipForUserWithProgress(zipData, templateName, user, nodes, knownTokens, nil)
}

func RenderZipForUserWithProgress(zipData []byte, templateName *string, user *model.User, nodes []*model.Node, knownTokens []string, onProgress func(processed, total uint64)) ([]byte, error) {
	return renderZipForUserWithProgress(zipData, templateName, user, knownTokens, TemplateTypeAuto, func(data []byte) map[string]string {
		return BuildTemplateContext(user, nodes, strings.Contains(string(data), MihomoPlaceholder))
	}, onProgress)
}

// RenderZipForUserWithProgressFromEndpoints renders a user ZIP using endpoint
// records as the source of truth, including each endpoint's is_active switch.
func RenderZipForUserWithProgressFromEndpoints(zipData []byte, templateName *string, user *model.User, endpoints []*model.Endpoint, knownTokens []string, onProgress func(processed, total uint64)) ([]byte, error) {
	return RenderZipForUserWithProgressFromEndpointsForTarget(zipData, templateName, user, endpoints, knownTokens, TemplateTypeSing, onProgress)
}

func RenderZipForUserWithProgressFromEndpointsForTarget(zipData []byte, templateName *string, user *model.User, endpoints []*model.Endpoint, knownTokens []string, templateType string, onProgress func(processed, total uint64)) ([]byte, error) {
	return RenderZipForUserWithProgressFromEndpointsForTargetAndBaseURL(zipData, templateName, user, endpoints, knownTokens, templateType, "", onProgress)
}

// RenderZipForUserWithProgressFromEndpointsForTargetAndBaseURL renders the
// selected template and, for launcher archives, the companion config.yaml.
// Xray launcher archives commonly use config.json as their detected template
// while config.yaml controls where the launcher downloads that JSON.
func RenderZipForUserWithProgressFromEndpointsForTargetAndBaseURL(zipData []byte, templateName *string, user *model.User, endpoints []*model.Endpoint, knownTokens []string, templateType string, baseURL string, onProgress func(processed, total uint64)) ([]byte, error) {
	return renderZipForUserWithProgress(zipData, templateName, user, knownTokens, templateType, func(data []byte) map[string]string {
		return BuildTemplateContextFromEndpointsForTargetWithBaseURL(user, endpoints, strings.Contains(string(data), MihomoPlaceholder), templateType, baseURL)
	}, onProgress)
}

func renderZipForUserWithProgress(zipData []byte, templateName *string, user *model.User, knownTokens []string, templateType string, buildContext func([]byte) map[string]string, onProgress func(processed, total uint64)) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, err
	}
	targetName := ""
	if templateName != nil && *templateName != "" {
		targetName = filepath.Base(*templateName)
	}
	launcherEntries := make(map[string]bool)
	for _, f := range zr.File {
		if !isLauncherEntry(f.Name) {
			continue
		}
		rc, openErr := f.Open()
		if openErr != nil {
			return nil, openErr
		}
		data, readErr := io.ReadAll(rc)
		_ = rc.Close()
		if readErr != nil {
			return nil, readErr
		}
		if shouldRenderLauncherEntry(f.Name, data, templateType) {
			launcherEntries[f.Name] = true
			logging.L().Debug("launcher companion template selected",
				"entry", f.Name,
				"template_type", templateType,
				"target", targetName,
			)
		}
	}
	logging.L().Debug("zip template rendering started",
		"template_type", templateType,
		"target", targetName,
		"zip_entries", len(zr.File),
		"launcher_entries", len(launcherEntries),
	)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	var totalWeight uint64
	for _, f := range zr.File {
		rendered := targetName == "" || filepath.Base(f.Name) == targetName || launcherEntries[f.Name]
		totalWeight += zipEntryProgressWeight(f, rendered)
	}
	var processedWeight uint64
	for _, f := range zr.File {
		renderedEntry := targetName == "" || filepath.Base(f.Name) == targetName || launcherEntries[f.Name]
		weight := zipEntryProgressWeight(f, renderedEntry)
		if !renderedEntry {
			raw, err := f.OpenRaw()
			if err != nil {
				return nil, err
			}
			header := f.FileHeader
			entryWriter, err := zw.CreateRaw(&header)
			if err != nil {
				return nil, err
			}
			reader := &progressReader{reader: raw, onRead: func(read uint64) {
				if onProgress != nil {
					progress := processedWeight + read
					if progress > totalWeight {
						progress = totalWeight
					}
					onProgress(progress, totalWeight)
				}
			}}
			if _, err := io.CopyBuffer(entryWriter, reader, make([]byte, 32*1024)); err != nil {
				return nil, err
			}
		} else {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			data, err := readAllWithProgress(rc, func(read uint64) {
				if onProgress == nil {
					return
				}
				processed := processedWeight + read
				if processed > totalWeight {
					processed = totalWeight
				}
				onProgress(processed, totalWeight)
			})
			_ = rc.Close()
			if err != nil {
				return nil, err
			}
			ctx := buildContext(data)
			renderedText := RenderTemplateTextWithContext(string(data), user, ctx, knownTokens)
			rendered := []byte(renderedText)
			logging.L().Debug("zip template entry rendered",
				"entry", f.Name,
				"template_type", templateType,
				"selected", filepath.Base(f.Name) == targetName,
				"launcher_companion", launcherEntries[f.Name],
				"bytes_before", len(data),
				"bytes_after", len(rendered),
				"changed", string(data) != renderedText,
				"placeholders_before", strings.Count(string(data), "{{"),
				"placeholders_after", strings.Count(renderedText, "{{"),
			)

			header := f.FileHeader
			header.Method = zip.Store
			header.Flags &^= 0x08
			header.CRC32 = crc32.ChecksumIEEE(rendered)
			header.CompressedSize = uint32(len(rendered))
			header.UncompressedSize = uint32(len(rendered))
			header.CompressedSize64 = uint64(len(rendered))
			header.UncompressedSize64 = uint64(len(rendered))
			w, err := zw.CreateHeader(&header)
			if err != nil {
				return nil, err
			}
			if _, err := w.Write(rendered); err != nil {
				return nil, err
			}
			logging.L().Debug("zip template entry written",
				"entry", f.Name,
				"template_type", templateType,
				"bytes_written", len(rendered),
			)
		}
		processedWeight += weight
		if onProgress != nil {
			onProgress(processedWeight, totalWeight)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if onProgress != nil && totalWeight == 0 {
		onProgress(0, 0)
	}
	logging.L().Debug("zip template rendering completed",
		"template_type", templateType,
		"target", targetName,
		"output_bytes", buf.Len(),
	)
	return buf.Bytes(), nil
}

func isLauncherEntry(name string) bool {
	base := strings.ToLower(filepath.Base(name))
	return base == "config.yaml" || base == "config.yml" || base == "config.json"
}

func shouldRenderLauncherEntry(name string, data []byte, templateType string) bool {
	text := strings.ToLower(string(data))
	base := strings.ToLower(filepath.Base(name))
	if (base == "config.yaml" || base == "config.yml") && strings.Contains(text, "upstream_url:") {
		return strings.Contains(text, "xray_bin:") ||
			strings.Contains(text, "sing_box_bin:") ||
			templateType == TemplateTypeXray ||
			templateType == TemplateTypeSing ||
			strings.Contains(text, "{{xray_config_url}}") ||
			strings.Contains(text, "{{singbox_config_url}}")
	}
	if base != "config.json" {
		return false
	}
	for _, placeholder := range []string{
		"{{xray_config_json}}",
		"{{xray_outbounds_json}}",
		"{{singbox_config_json}}",
		"{{singbox_outbounds_json}}",
	} {
		if strings.Contains(text, placeholder) {
			return true
		}
	}
	return false
}

// RenderZipForUserTo renders only the selected ZIP member and writes its bytes to dst.
// This avoids recompressing every file when the personalized content is a single config.
func RenderZipForUserTo(zipData []byte, templateName *string, user *model.User, nodes []*model.Node, knownTokens []string, dst io.Writer) error {
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return err
	}

	targetName := ""
	if templateName != nil && *templateName != "" {
		targetName = filepath.Base(*templateName)
	}

	var target *zip.File
	for _, f := range zr.File {
		currName := filepath.Base(f.Name)
		if targetName != "" && currName != targetName {
			continue
		}
		target = f
		break
	}
	if target == nil {
		return zip.ErrFormat
	}

	rc, err := target.Open()
	if err != nil {
		return err
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return err
	}

	ctx := BuildTemplateContext(user, nodes, strings.Contains(string(data), MihomoPlaceholder))
	_, err = io.WriteString(dst, RenderTemplateTextWithContext(string(data), user, ctx, knownTokens))
	return err
}
