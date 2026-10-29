package template

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"noder/internal/model"
)

func testXrayTemplateEndpoint() *model.Endpoint {
	sni := "www.example.com"
	publicKey := "99BZ0JCnaSB55YEQYOCV66GhKTiK2ZGMPR3b6D_Q3wo"
	shortID := "1a91"
	fingerprint := "chrome"
	flow := "xtls-rprx-vision"
	return &model.Endpoint{
		Tag:      "xray-node",
		UUID:     "11111111-1111-4111-8111-111111111111",
		IsActive: true,
		Node: &model.Node{
			Tag:           "xray-node",
			Protocol:      "vless",
			ServerAddress: "example.com",
			ServerPort:    443,
			Security:      "reality",
			SNI:           &sni,
			PublicKey:     &publicKey,
			ShortID:       &shortID,
			Fingerprint:   &fingerprint,
			Flow:          &flow,
			IsActive:      true,
		},
	}
}

func TestDetectTemplateType(t *testing.T) {
	if got := DetectTemplateType([]byte(`{"outbounds":[{"type":"vless"}]}`)); got != TemplateTypeSing {
		t.Fatalf("sing-box template detected as %q", got)
	}
	if got := DetectTemplateType([]byte(`{"outbounds":[{"protocol":"vless"}]}`)); got != TemplateTypeXray {
		t.Fatalf("xray template detected as %q", got)
	}
	if got := DetectTemplateType([]byte("upstream_url: https://example.com/xray\nxray_bin: ./xray.exe")); got != TemplateTypeXray {
		t.Fatalf("xray launcher template detected as %q", got)
	}
}

func TestXrayTemplateContext(t *testing.T) {
	user := &model.User{Name: "Xray User", Token: "xray-token", UUID: stringPtr("11111111-1111-4111-8111-111111111111")}
	ctx := BuildTemplateContextFromEndpointsForTargetWithBaseURL(user, []*model.Endpoint{testXrayTemplateEndpoint()}, false, TemplateTypeXray, "https://proxy.example.test/base/")
	if !strings.Contains(ctx["xray_outbounds_json"], `"protocol": "vless"`) {
		t.Fatalf("xray outbound placeholder was not generated: %s", ctx["xray_outbounds_json"])
	}
	if !strings.Contains(ctx["xray_config_json"], `"inbounds"`) {
		t.Fatalf("full xray config placeholder was not generated: %s", ctx["xray_config_json"])
	}
	if got, want := ctx["xray_config_url"], "https://proxy.example.test/base/xray"; got != want {
		t.Fatalf("xray launcher URL = %q, want %q", got, want)
	}
}

func TestRenderXrayLauncherRendersCompanionConfigYAML(t *testing.T) {
	var source bytes.Buffer
	writer := zip.NewWriter(&source)
	jsonEntry, err := writer.Create("config.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jsonEntry.Write([]byte(`{{xray_config_json}}`)); err != nil {
		t.Fatal(err)
	}
	yamlEntry, err := writer.Create("config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := yamlEntry.Write([]byte("upstream_url: \"{{xray_config_url}}\"\ntoken: \"{{token}}\"\nxray_bin: ./xray.exe\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	user := &model.User{Name: "Xray User", Token: "xray-token", UUID: stringPtr("11111111-1111-4111-8111-111111111111")}
	rendered, err := RenderZipForUserWithProgressFromEndpointsForTargetAndBaseURL(source.Bytes(), stringPtr("config.json"), user, []*model.Endpoint{testXrayTemplateEndpoint()}, nil, TemplateTypeXray, "https://proxy.example.test", nil)
	if err != nil {
		t.Fatalf("rendering Xray launcher ZIP failed: %v", err)
	}
	resultZip, err := zip.NewReader(bytes.NewReader(rendered), int64(len(rendered)))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range resultZip.File {
		if entry.Name != "config.yaml" {
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		got := string(data)
		if !strings.Contains(got, `upstream_url: "https://proxy.example.test/xray"`) || !strings.Contains(got, `token: "xray-token"`) {
			t.Fatalf("Xray launcher config was not rendered: %s", got)
		}
		return
	}
	t.Fatal("rendered ZIP did not contain config.yaml")
}

func TestRenderXrayZipUsesUserConfig(t *testing.T) {
	var source bytes.Buffer
	writer := zip.NewWriter(&source)
	entry, err := writer.Create("config.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(`{{xray_config_json}}`)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	user := &model.User{Name: "Xray User", Token: "xray-token", UUID: stringPtr("11111111-1111-4111-8111-111111111111")}
	rendered, err := RenderZipForUserWithProgressFromEndpointsForTarget(source.Bytes(), nil, user, []*model.Endpoint{testXrayTemplateEndpoint()}, nil, TemplateTypeXray, nil)
	if err != nil {
		t.Fatalf("rendering Xray ZIP failed: %v", err)
	}
	resultZip, err := zip.NewReader(bytes.NewReader(rendered), int64(len(rendered)))
	if err != nil {
		t.Fatal(err)
	}
	configFile := resultZip.File[0]
	configReader, err := configFile.Open()
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]interface{}
	if err := json.NewDecoder(configReader).Decode(&config); err != nil {
		_ = configReader.Close()
		t.Fatal(err)
	}
	_ = configReader.Close()
	if _, ok := config["inbounds"]; !ok {
		t.Fatalf("rendered Xray config has no inbounds: %#v", config)
	}
}

func stringPtr(value string) *string { return &value }

func TestRenderTemplateText(t *testing.T) {
	user := &model.User{
		Name:  "TestUser",
		Token: "my-token-1234",
	}

	raw := "Hello {{name}}, your token is {{token}}"
	rendered := RenderTemplateText(raw, user, nil, nil)
	expected := "Hello TestUser, your token is my-token-1234"
	if rendered != expected {
		t.Errorf("Expected '%s', got '%s'", expected, rendered)
	}

	// 键名感知替换测试
	keyedRaw := `token: "00000000-0000-0000-0000-000000000000"`
	renderedKeyed := RenderTemplateText(keyedRaw, user, nil, nil)
	if !strings.Contains(renderedKeyed, "my-token-1234") {
		t.Errorf("Keyed replacement failed: %s", renderedKeyed)
	}
}

func TestRenderZipForUser(t *testing.T) {
	// 创建内存 ZIP
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	w1, _ := zw.Create("config.json")
	_, _ = w1.Write([]byte(`{"token": "{{token}}", "user": "{{name}}"}`))

	w2, _ := zw.Create("readme.txt")
	_, _ = w2.Write([]byte(`Fixed readme file`))

	_ = zw.Close()

	user := &model.User{
		Name:  "Alice",
		Token: "alice-token",
	}

	renderedZip, err := RenderZipForUser(buf.Bytes(), nil, user, nil, nil)
	if err != nil {
		t.Fatalf("RenderZipForUser failed: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(renderedZip), int64(len(renderedZip)))
	if err != nil {
		t.Fatalf("Read rendered zip failed: %v", err)
	}

	for _, f := range zr.File {
		rc, _ := f.Open()
		data := make([]byte, f.UncompressedSize64)
		_, _ = rc.Read(data)
		_ = rc.Close()

		if f.Name == "config.json" {
			if !strings.Contains(string(data), "alice-token") {
				t.Errorf("ZIP config.json not rendered with user token: %s", string(data))
			}
		} else if f.Name == "readme.txt" {
			if string(data) != "Fixed readme file" {
				t.Errorf("Non-template file altered: %s", string(data))
			}
		}
	}
}

func TestRenderZipForUserToRendersOnlySelectedFile(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	config, _ := zw.Create("folder/config.json")
	_, _ = config.Write([]byte(`{"token":"{{token}}"}`))
	readme, _ := zw.Create("readme.txt")
	_, _ = readme.Write([]byte("unrelated content"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	user := &model.User{Token: "streamed-token"}
	templateName := "config.json"
	var output bytes.Buffer
	if err := RenderZipForUserTo(buf.Bytes(), &templateName, user, nil, nil, &output); err != nil {
		t.Fatalf("RenderZipForUserTo failed: %v", err)
	}
	if got := output.String(); got != `{"token":"streamed-token"}` {
		t.Fatalf("unexpected rendered output: %s", got)
	}
}

func TestRenderZipForUserPreservesUntouchedStoredEntry(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	config, _ := zw.Create("config.json")
	_, _ = config.Write([]byte(`{"token":"{{token}}"}`))
	readmeHeader := &zip.FileHeader{Name: "readme.txt", Method: zip.Store}
	readme, err := zw.CreateHeader(readmeHeader)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = readme.Write([]byte("static"))
	compressed, err := zw.Create("docs/help.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = compressed.Write([]byte("compressed static content"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	user := &model.User{Token: "zip-token"}
	templateName := "config.json"
	var progress []uint64
	var totalProgress uint64
	rendered, err := RenderZipForUserWithProgress(buf.Bytes(), &templateName, user, nil, nil, func(processed, total uint64) {
		progress = append(progress, processed)
		totalProgress = total
		if processed > total {
			t.Errorf("progress exceeds total: %d > %d", processed, total)
		}
	})
	if err != nil {
		t.Fatalf("RenderZipForUser failed: %v", err)
	}
	if len(progress) < 3 || progress[len(progress)-1] != totalProgress {
		t.Fatalf("expected byte and entry progress through the complete ZIP, got %v of %d", progress, totalProgress)
	}
	for i := 1; i < len(progress); i++ {
		if progress[i] < progress[i-1] {
			t.Fatalf("progress moved backwards: %v", progress)
		}
	}
	zr, err := zip.NewReader(bytes.NewReader(rendered), int64(len(rendered)))
	if err != nil {
		t.Fatalf("read rendered archive: %v", err)
	}
	if len(zr.File) != 3 {
		t.Fatalf("expected 3 ZIP entries, got %d", len(zr.File))
	}
	for _, f := range zr.File {
		if f.Name == "readme.txt" || f.Name == "docs/help.txt" {
			expectedMethod := uint16(zip.Deflate)
			if f.Name == "readme.txt" {
				expectedMethod = zip.Store
			}
			if f.Method != expectedMethod {
				t.Errorf("expected untouched entry %s to retain method %d, got %d", f.Name, expectedMethod, f.Method)
			}
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			contents, err := io.ReadAll(rc)
			_ = rc.Close()
			expectedContents := "static"
			if f.Name == "docs/help.txt" {
				expectedContents = "compressed static content"
			}
			if err != nil || string(contents) != expectedContents {
				t.Errorf("unexpected readme content %q, error %v", contents, err)
			}
		}
	}
}
