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
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	user := &model.User{Token: "zip-token"}
	templateName := "config.json"
	rendered, err := RenderZipForUser(buf.Bytes(), &templateName, user, nil, nil)
	if err != nil {
		t.Fatalf("RenderZipForUser failed: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(rendered), int64(len(rendered)))
	if err != nil {
		t.Fatalf("read rendered archive: %v", err)
	}
	if len(zr.File) != 2 {
		t.Fatalf("expected 2 ZIP entries, got %d", len(zr.File))
	}
	for _, f := range zr.File {
		if f.Name == "readme.txt" {
			if f.Method != zip.Store {
				t.Errorf("expected untouched stored entry to remain Store, got method %d", f.Method)
			}
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			contents, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil || string(contents) != "static" {
				t.Errorf("unexpected readme content %q, error %v", contents, err)
			}
		}
	}
}
