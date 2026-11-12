package file

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"noder/internal/config"
	"noder/internal/contract"
	"noder/internal/model"
)

func TestComputeSHA256(t *testing.T) {
	data := []byte("hello noder subscription")
	expectedHash := sha256.Sum256(data)
	expectedStr := hex.EncodeToString(expectedHash[:])

	if got := ComputeSHA256(data); got != expectedStr {
		t.Fatalf("ComputeSHA256 = %s, want %s", got, expectedStr)
	}
}

func TestGenerateStoredName(t *testing.T) {
	name1 := GenerateStoredName("client.zip")
	if !strings.HasSuffix(name1, "_client.zip") {
		t.Fatalf("expected suffix _client.zip, got %s", name1)
	}

	name2 := GenerateStoredName("")
	if !strings.HasSuffix(name2, "_file.bin") {
		t.Fatalf("expected suffix _file.bin, got %s", name2)
	}
}

func TestStoredFilePath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "noder-file-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	config.FilesDir = tmpDir

	// Case 1: StoredName explicitly set
	dist1 := &model.DistFile{
		StoredName: "custom_uuid_file.zip",
	}
	if p := StoredFilePath(dist1); p != filepath.Join(tmpDir, "custom_uuid_file.zip") {
		t.Fatalf("unexpected stored path: %s", p)
	}

	// Case 2: OriginalName extension fallback
	dist2 := &model.DistFile{
		ID:           10,
		OriginalName: "app.apk",
	}
	if p := StoredFilePath(dist2); p != filepath.Join(tmpDir, "10.apk") {
		t.Fatalf("unexpected path for originalName: %s", p)
	}

	// Case 3: FileType fallback (zip, apk, text)
	distZip := &model.DistFile{ID: 11, FileType: "zip"}
	if p := StoredFilePath(distZip); p != filepath.Join(tmpDir, "11.zip") {
		t.Fatalf("unexpected zip fallback path: %s", p)
	}

	distText := &model.DistFile{ID: 12, FileType: "text"}
	if p := StoredFilePath(distText); p != filepath.Join(tmpDir, "12.txt") {
		t.Fatalf("unexpected text fallback path: %s", p)
	}
}

func TestSaveReadDeleteFileContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "noder-file-io-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	config.FilesDir = tmpDir

	dist := &model.DistFile{
		StoredName: "test_payload.txt",
	}
	content := []byte("payload content test")

	if err := SaveFileContent(dist, content); err != nil {
		t.Fatalf("SaveFileContent failed: %v", err)
	}

	readBack, err := ReadFileContent(dist)
	if err != nil {
		t.Fatalf("ReadFileContent failed: %v", err)
	}
	if string(readBack) != string(content) {
		t.Fatalf("expected %q, got %q", content, readBack)
	}

	if err := DeleteStoredFile(dist); err != nil {
		t.Fatalf("DeleteStoredFile failed: %v", err)
	}

	if _, err := ReadFileContent(dist); !os.IsNotExist(err) {
		t.Fatalf("expected file to be deleted, got err: %v", err)
	}
}

func TestValidateRemoteURL(t *testing.T) {
	if err := ValidateRemoteURL("https://example.com/dist.zip"); err != nil {
		t.Fatalf("valid https url failed: %v", err)
	}
	if err := ValidateRemoteURL("http://example.com/dist.apk"); err != nil {
		t.Fatalf("valid http url failed: %v", err)
	}

	if err := ValidateRemoteURL("ftp://example.com/dist.zip"); err == nil {
		t.Fatal("expected ftp url to be rejected")
	}
	if err := ValidateRemoteURL("file:///etc/passwd"); err == nil {
		t.Fatal("expected file:// url to be rejected")
	}
	if err := ValidateRemoteURL("not-a-url"); err == nil {
		t.Fatal("expected invalid url to be rejected")
	}
}

func TestDetectFileType(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"sing-box.apk", "apk"},
		{"SING-BOX.APK", "apk"},
		{"config.zip", "zip"},
		{"CONFIG.ZIP", "zip"},
		{"readme.txt", "text"},
		{"info.json", "text"},
		{"binary.bin", "text"},
	}

	for _, tt := range tests {
		if got := DetectFileType(tt.name); got != tt.expected {
			t.Errorf("DetectFileType(%q) = %q, want %q", tt.name, got, tt.expected)
		}
	}
}

func TestExtractFileNameFromURL(t *testing.T) {
	if got := ExtractFileNameFromURL("https://cdn.example.com/files/v1.0.0/client.apk?token=xyz"); got != "client.apk" {
		t.Fatalf("expected client.apk, got %s", got)
	}
	if got := ExtractFileNameFromURL("https://cdn.example.com/"); got != "remote_download" {
		t.Fatalf("expected remote_download, got %s", got)
	}
	if got := ExtractFileNameFromURL("invalid:url:%%"); got != "remote_download" {
		t.Fatalf("expected remote_download on error, got %s", got)
	}
}

func TestIsRemoteCacheExpired(t *testing.T) {
	// Empty or nil
	if !IsRemoteCacheExpired(&model.DistFile{CachedAt: nil}) {
		t.Fatal("nil cached_at should be expired")
	}
	empty := ""
	if !IsRemoteCacheExpired(&model.DistFile{CachedAt: &empty}) {
		t.Fatal("empty cached_at should be expired")
	}

	// Recent time (1 hour ago)
	recent := time.Now().Add(-1 * time.Hour).Format("2006-01-02 15:04:05")
	if IsRemoteCacheExpired(&model.DistFile{CachedAt: &recent}) {
		t.Fatal("recent cached_at should not be expired")
	}

	// Expired time (25 hours ago)
	expired := time.Now().Add(-25 * time.Hour).Format("2006-01-02 15:04:05")
	if !IsRemoteCacheExpired(&model.DistFile{CachedAt: &expired}) {
		t.Fatal("25h old cached_at should be expired")
	}

	// RFC3339 format support
	rfcRecent := time.Now().Add(-30 * time.Minute).Format(time.RFC3339)
	if IsRemoteCacheExpired(&model.DistFile{CachedAt: &rfcRecent}) {
		t.Fatal("RFC3339 recent cached_at should not be expired")
	}
}

func TestFetchRemoteFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("mock content"))
		case "/notfound":
			w.WriteHeader(http.StatusNotFound)
		case "/large":
			w.WriteHeader(http.StatusOK)
			// Return large payload exceeding MaxFileSize
			buf := make([]byte, 2048)
			_, _ = w.Write(buf)
		}
	}))
	defer server.Close()

	// 1. Success fetch
	content, err := FetchRemoteFile(server.URL + "/ok")
	if err != nil {
		t.Fatalf("FetchRemoteFile failed: %v", err)
	}
	if string(content) != "mock content" {
		t.Fatalf("expected 'mock content', got %q", string(content))
	}

	// 2. 404 fetch error
	_, err = FetchRemoteFile(server.URL + "/notfound")
	if err == nil {
		t.Fatal("expected error on 404 remote fetch")
	}
	apiErr, ok := err.(*contract.APIError)
	if !ok || apiErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected BadGateway, got: %v", err)
	}

	// 3. Oversized check
	origMax := config.MaxFileSize
	defer func() { config.MaxFileSize = origMax }()
	config.MaxFileSize = 100 // 100 bytes limit

	_, err = FetchRemoteFile(server.URL + "/large")
	if err == nil {
		t.Fatal("expected error for oversized file")
	}
	apiErr2, ok := err.(*contract.APIError)
	if !ok || apiErr2.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected RequestEntityTooLarge, got: %v", err)
	}
}
