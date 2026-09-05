package template

import (
	"archive/zip"
	"bytes"
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
