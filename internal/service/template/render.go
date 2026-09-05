package template

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"noder/internal/model"
	"noder/internal/service/mihomo"
	"noder/internal/service/singbox"
)

const MihomoPlaceholder = "{{mihomo_proxies_yaml}}"

func BuildTemplateContext(user *model.User, nodes []*model.Node, includeMihomo bool) map[string]string {
	var activeNodes []*model.Node
	for _, n := range nodes {
		if n.IsActive {
			activeNodes = append(activeNodes, n)
		}
	}

	var nodeMeta []map[string]interface{}
	for _, n := range activeNodes {
		nodeMeta = append(nodeMeta, map[string]interface{}{
			"node_name": n.NodeName,
			"protocol":  n.Protocol,
			"server":    n.ServerAddress,
			"server_port": n.ServerPort,
		})
	}

	var outbounds []map[string]interface{}
	for _, n := range activeNodes {
		if ob, err := singbox.BuildSingboxOutbound(n, user); err == nil {
			outbounds = append(outbounds, ob)
		}
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
	if user.UUID != nil {
		uuidStr = *user.UUID
	}
	pwdStr := ""
	if user.Password != nil {
		pwdStr = *user.Password
	}

	ctx := map[string]string{
		"uuid":            uuidStr,
		"password":        pwdStr,
		"token":           user.Token,
		"name":            user.Name,
		"user_name":       user.Name,
		"node_list_yaml":  yamlDump(nodeMeta),
		"node_list_json":  jsonIndent(nodeMeta),
		"outbounds_yaml":  yamlDump(outbounds),
		"outbounds_json":  jsonIndent(outbounds),
	}

	if includeMihomo {
		if mhBytes, err := mihomo.BuildMihomoConfigYAML(activeNodes, user); err == nil {
			var mhObj map[string]interface{}
			_ = yaml.Unmarshal(mhBytes, &mhObj)
			if proxies, ok := mhObj["proxies"]; ok {
				ctx["mihomo_proxies_yaml"] = yamlDump(proxies)
			}
		}
	}

	return ctx
}

var keyedPattern = regexp.MustCompile(`(?i)(["']?[\w.\-]*?(?:token|uuid|password)["']?\s*[:=]\s*)(["']?)([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})(["']?)`)

func RenderTemplateText(text string, user *model.User, nodes []*model.Node, knownTokens []string) string {
	ctx := BuildTemplateContext(user, nodes, strings.Contains(text, MihomoPlaceholder))

	for k, v := range ctx {
		text = strings.ReplaceAll(text, "{{"+k+"}}", v)
	}

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
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, err
	}

	targetName := ""
	if templateName != nil && *templateName != "" {
		targetName = filepath.Base(*templateName)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, err
		}

		currName := filepath.Base(f.Name)
		// 如果未指定 templateName，或者匹配到目标文件
		if targetName == "" || currName == targetName {
			// 尝试 utf8 解码并替换
			rendered := RenderTemplateText(string(data), user, nodes, knownTokens)
			data = []byte(rendered)
		}

		header := f.FileHeader
		w, err := zw.CreateHeader(&header)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
