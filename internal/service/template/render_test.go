package template

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"noder/internal/model"
)

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

func TestRenderXrayLauncherConfigReplacesExampleValues(t *testing.T) {
	var source bytes.Buffer
	writer := zip.NewWriter(&source)
	config, err := writer.Create("xray-launcher/config.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.Write([]byte(`{{xray_config_json}}`)); err != nil {
		t.Fatal(err)
	}
	launcher, err := writer.Create("xray-launcher/config.yml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := launcher.Write([]byte("upstream_url: \"https://example.com/xray\"\ntoken: \"replace-with-user-token\"\nconfig_file: \"config.json\"\nxray_bin: \"./xray.exe\"\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	user := &model.User{Token: "real-user-token"}
	rendered, err := RenderZipForUserWithProgressFromEndpointsForTargetAndBaseURL(source.Bytes(), stringPtr("config.json"), user, nil, nil, TemplateTypeXray, "http://localhost:8000", nil)
	if err != nil {
		t.Fatalf("rendering Xray launcher ZIP failed: %v", err)
	}
	result, err := zip.NewReader(bytes.NewReader(rendered), int64(len(rendered)))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range result.File {
		if entry.Name != "xray-launcher/config.yml" {
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
		if !strings.Contains(got, `upstream_url: "http://localhost:8000/xray"`) {
			t.Fatalf("upstream URL was not rendered: %s", got)
		}
		if !strings.Contains(got, `token: "real-user-token"`) {
			t.Fatalf("launcher token was not rendered: %s", got)
		}
		return
	}
	t.Fatal("rendered ZIP did not contain launcher config.yml")
}

func TestRenderSingboxLauncherConfigUsesSingboxURL(t *testing.T) {
	var source bytes.Buffer
	writer := zip.NewWriter(&source)
	config, err := writer.Create("sing-launcher/config.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.Write([]byte(`{{singbox_config_json}}`)); err != nil {
		t.Fatal(err)
	}
	launcher, err := writer.Create("sing-launcher/config.yml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := launcher.Write([]byte("upstream_url: \"https://example.com/xray\"\ntoken: \"replace-with-user-token\"\nconfig_file: \"config.json\"\nsing_box_bin: \"./sing-box.exe\"\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	user := &model.User{Token: "real-user-token"}
	detected, err := DetectTemplateTypeFromZip(source.Bytes(), "config.json")
	if err != nil {
		t.Fatal(err)
	}
	if detected != TemplateTypeSing {
		t.Fatalf("expected sing-box pipeline, got %q", detected)
	}
	rendered, err := RenderZipForUserWithProgressFromEndpointsForTargetAndBaseURL(source.Bytes(), stringPtr("config.json"), user, nil, nil, detected, "http://localhost:8000", nil)
	if err != nil {
		t.Fatalf("rendering sing-box launcher ZIP failed: %v", err)
	}
	result, err := zip.NewReader(bytes.NewReader(rendered), int64(len(rendered)))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range result.File {
		if entry.Name != "sing-launcher/config.yml" {
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
		if !strings.Contains(got, `upstream_url: "http://localhost:8000/sub"`) {
			t.Fatalf("sing-box launcher used the wrong upstream URL: %s", got)
		}
		if strings.Contains(got, "/xray") {
			t.Fatalf("sing-box launcher still contains an Xray URL: %s", got)
		}
		return
	}
	t.Fatal("rendered ZIP did not contain sing-launcher config.yml")
}

func TestDetectTemplateTypePrefersExplicitSingboxMarker(t *testing.T) {
	data := []byte("upstream_url: \"https://example.com/xray\"\nsing_box_bin: \"./sing-box.exe\"\n")
	if got := DetectTemplateType(data); got != TemplateTypeSing {
		t.Fatalf("expected sing-box pipeline, got %q", got)
	}
}

func TestDetectTemplateTypeFromZipChecksLauncherWhenConfigIsPlaceholder(t *testing.T) {
	var source bytes.Buffer
	writer := zip.NewWriter(&source)
	config, err := writer.Create("xray-launcher/config.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.Write([]byte(`{{xray_config_json}}`)); err != nil {
		t.Fatal(err)
	}
	launcher, err := writer.Create("xray-launcher/config.yml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := launcher.Write([]byte("upstream_url: \"https://example.com/xray\"\nxray_bin: \"./xray.exe\"\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := DetectTemplateTypeFromZip(source.Bytes(), "config.json")
	if err != nil {
		t.Fatal(err)
	}
	if got != TemplateTypeXray {
		t.Fatalf("expected Xray pipeline from launcher metadata, got %q", got)
	}
}

func TestTargetTemplateContextsAreIsolated(t *testing.T) {
	sni := "example.com"
	user := &model.User{Token: "token"}
	endpoints := []*model.Endpoint{{
		Node: &model.Node{
			Protocol:      "vless",
			Tag:           "node",
			ServerAddress: "127.0.0.1",
			ServerPort:    443,
			Security:      "tls",
			SNI:           &sni,
			IsActive:      true,
		},
		Tag:      "node",
		UUID:     "00000000-0000-0000-0000-000000000001",
		IsActive: true,
	}}

	singboxContext := BuildTemplateContextFromEndpointsForTargetWithBaseURL(user, endpoints, false, TemplateTypeSing, "https://example.com")
	if singboxContext["singbox_config_url"] != "https://example.com/sub" {
		t.Fatalf("unexpected sing-box URL: %q", singboxContext["singbox_config_url"])
	}
	if _, ok := singboxContext["xray_config_url"]; ok {
		t.Fatal("sing-box context leaked the Xray URL")
	}
	if _, ok := singboxContext["xray_config_json"]; ok {
		t.Fatal("sing-box context leaked the Xray config")
	}

	xrayContext := BuildTemplateContextFromEndpointsForTargetWithBaseURL(user, endpoints, false, TemplateTypeXray, "https://example.com")
	if xrayContext["xray_config_url"] != "https://example.com/xray" {
		t.Fatalf("unexpected Xray URL: %q", xrayContext["xray_config_url"])
	}
	if _, ok := xrayContext["singbox_config_url"]; ok {
		t.Fatal("Xray context leaked the sing-box URL")
	}
	if _, ok := xrayContext["singbox_config_json"]; ok {
		t.Fatal("Xray context leaked the sing-box config")
	}
}

func stringPtr(value string) *string { return &value }

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
