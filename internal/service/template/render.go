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

	"noder/internal/model"
)

const MihomoPlaceholder = "{{mihomo_proxies_yaml}}"

const (
	TemplateTypeAuto = "auto"
	TemplateTypeSing = "sing-box"
	TemplateTypeXray = "xray"
)

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
	lower := strings.ToLower(string(data))
	// Explicit pipeline markers take precedence over example URLs. A sing-box
	// launcher may still contain an old example URL ending in /xray.
	if strings.Contains(lower, "sing_box_bin") || strings.Contains(lower, "sing-box.exe") ||
		strings.Contains(lower, "{{singbox_config_") || strings.Contains(lower, "{{singbox_outbounds_") {
		return TemplateTypeSing
	}
	if strings.Contains(lower, "xray_bin") || strings.Contains(lower, "xray.exe") ||
		strings.Contains(lower, "{{xray_config_") || strings.Contains(lower, "{{xray_outbounds_") {
		return TemplateTypeXray
	}
	if strings.Contains(lower, "/xray") {
		return TemplateTypeXray
	}
	if strings.Contains(lower, "/sub") {
		return TemplateTypeSing
	}
	return ""
}

func DetectTemplateTypeFromZip(zipData []byte, templateName string) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return "", err
	}
	target := filepath.Base(templateName)
	var targetEntry *zip.File
	var candidates []*zip.File
	foundTarget := target == ""
	for _, entry := range zr.File {
		lower := strings.ToLower(entry.Name)
		isConfig := strings.HasSuffix(lower, ".json") || strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")
		if !isConfig {
			continue
		}
		if target != "" && (filepath.Base(entry.Name) == target || entry.Name == templateName) {
			if targetEntry == nil {
				targetEntry = entry
			}
			foundTarget = true
			continue
		}
		if target == "" {
			candidates = append(candidates, entry)
		} else {
			// The selected config may be a placeholder. Launcher metadata in a
			// sibling config.yml is still part of the target declaration.
			candidates = append(candidates, entry)
		}
	}
	if !foundTarget {
		return "", zip.ErrFormat
	}
	if targetEntry != nil {
		candidates = append([]*zip.File{targetEntry}, candidates...)
	}
	if len(candidates) == 0 {
		return "", zip.ErrFormat
	}
	for _, entry := range candidates {
		rc, err := entry.Open()
		if err != nil {
			return "", err
		}
		data, readErr := io.ReadAll(rc)
		_ = rc.Close()
		if readErr != nil {
			return "", readErr
		}
		if detected := DetectTemplateType(data); detected != "" {
			return detected, nil
		}
	}
	return "", nil
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

var keyedPattern = regexp.MustCompile(`(?i)(["']?[\w.\-]*?(?:token|uuid|password)["']?\s*[:=]\s*)(["']?)([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})(["']?)`)
var placeholderPattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)

func isLauncherEntry(name string) bool {
	base := strings.ToLower(filepath.Base(name))
	return base == "config.yaml" || base == "config.yml" || base == "config.json"
}

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
	text = placeholderPattern.ReplaceAllStringFunc(text, func(m string) string {
		sub := placeholderPattern.FindStringSubmatch(m)
		if len(sub) == 2 {
			if value, ok := ctx[sub[1]]; ok {
				return value
			}
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
	pipeline := singboxPipeline()
	return renderZipForUserWithProgress(zipData, templateName, user, knownTokens, func(data []byte) map[string]string {
		return BuildTemplateContext(user, nodes, strings.Contains(string(data), MihomoPlaceholder))
	}, pipeline, onProgress)
}

// RenderZipForUserWithProgressFromEndpoints renders a user ZIP using endpoint
// records as the source of truth, including each endpoint's is_active switch.
func RenderZipForUserWithProgressFromEndpoints(zipData []byte, templateName *string, user *model.User, endpoints []*model.Endpoint, knownTokens []string, onProgress func(processed, total uint64)) ([]byte, error) {
	return RenderZipForUserWithProgressFromEndpointsForTargetAndBaseURL(zipData, templateName, user, endpoints, knownTokens, TemplateTypeSing, "", onProgress)
}

func RenderZipForUserWithProgressFromEndpointsForTarget(zipData []byte, templateName *string, user *model.User, endpoints []*model.Endpoint, knownTokens []string, templateType string, onProgress func(processed, total uint64)) ([]byte, error) {
	return RenderZipForUserWithProgressFromEndpointsForTargetAndBaseURL(zipData, templateName, user, endpoints, knownTokens, templateType, "", onProgress)
}

func RenderZipForUserWithProgressFromEndpointsForTargetAndBaseURL(zipData []byte, templateName *string, user *model.User, endpoints []*model.Endpoint, knownTokens []string, templateType string, baseURL string, onProgress func(processed, total uint64)) ([]byte, error) {
	pipeline := pipelineForType(templateType)
	return renderZipForUserWithProgress(zipData, templateName, user, knownTokens, func(data []byte) map[string]string {
		return pipeline.buildContext(user, endpoints, strings.Contains(string(data), MihomoPlaceholder), baseURL)
	}, pipeline, onProgress)
}

func renderZipForUserWithProgress(zipData []byte, templateName *string, user *model.User, knownTokens []string, buildContext func([]byte) map[string]string, pipeline templatePipeline, onProgress func(processed, total uint64)) ([]byte, error) {
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
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(rc)
		_ = rc.Close()
		if readErr != nil {
			return nil, readErr
		}
		if pipeline.shouldRenderLauncherEntry(f.Name, data) {
			launcherEntries[f.Name] = true
		}
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	var totalWeight uint64
	for _, f := range zr.File {
		totalWeight += zipEntryProgressWeight(f, targetName == "" || filepath.Base(f.Name) == targetName || launcherEntries[f.Name])
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
			if launcherEntries[f.Name] && (strings.EqualFold(filepath.Ext(f.Name), ".yaml") || strings.EqualFold(filepath.Ext(f.Name), ".yml")) {
				renderedText = pipeline.renderLauncherConfig(renderedText, user, ctx)
			}
			rendered := []byte(renderedText)

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
	return buf.Bytes(), nil
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
