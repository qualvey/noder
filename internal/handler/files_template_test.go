package handler

import (
	"archive/zip"
	"bytes"
	"testing"
)

func makeTemplateZip(t *testing.T, name, content string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestResolveZipTemplateMetadataDetectsXray(t *testing.T) {
	data := makeTemplateZip(t, "config.json", `{"inbounds":[{"protocol":"socks"}],"outbounds":[{"protocol":"vless"}]}`)
	name, target, err := resolveZipTemplateMetadata(data, "", "auto")
	if err != nil {
		t.Fatalf("resolveZipTemplateMetadata failed: %v", err)
	}
	if name != "config.json" || target != "xray" {
		t.Fatalf("unexpected template metadata: name=%q target=%q", name, target)
	}
}

func TestResolveZipTemplateMetadataRejectsMismatchedType(t *testing.T) {
	data := makeTemplateZip(t, "config.json", `{"outbounds":[{"protocol":"vless"}]}`)
	if _, _, err := resolveZipTemplateMetadata(data, "config.json", "sing-box"); err == nil {
		t.Fatal("expected an explicit Sing-Box/Xray template mismatch to fail")
	}
}
