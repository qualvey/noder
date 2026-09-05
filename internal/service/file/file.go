package file

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"noder/internal/config"
	"noder/internal/contract"
	"noder/internal/model"
)

func ComputeSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func EnsureFilesDir() error {
	return os.MkdirAll(config.FilesDir, 0o755)
}

func StoredFilePath(dist *model.DistFile) string {
	ext := filepath.Ext(dist.OriginalName)
	if ext == "" {
		if dist.FileType == "apk" {
			ext = ".apk"
		} else if dist.FileType == "zip" {
			ext = ".zip"
		} else {
			ext = ".txt"
		}
	}
	return filepath.Join(config.FilesDir, fmt.Sprintf("%d%s", dist.ID, ext))
}

func SaveFileContent(dist *model.DistFile, content []byte) error {
	if err := EnsureFilesDir(); err != nil {
		return err
	}
	path := StoredFilePath(dist)
	return os.WriteFile(path, content, 0o644)
}

func ReadFileContent(dist *model.DistFile) ([]byte, error) {
	path := StoredFilePath(dist)
	return os.ReadFile(path)
}

func DeleteStoredFile(dist *model.DistFile) error {
	path := StoredFilePath(dist)
	_ = os.Remove(path)
	return nil
}

func ValidateRemoteURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return contract.NewBadRequest("远程链接仅支持 http/https 协议")
	}
	return nil
}

func FetchRemoteFile(rawURL string) ([]byte, error) {
	if err := ValidateRemoteURL(rawURL); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, contract.NewBadRequest(fmt.Sprintf("远程拉取失败: %v", err))
	}
	req.Header.Set("User-Agent", "noder-sub-server")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &contract.APIError{StatusCode: http.StatusBadGateway, Detail: fmt.Sprintf("远程拉取失败: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &contract.APIError{StatusCode: http.StatusBadGateway, Detail: fmt.Sprintf("远程拉取失败，状态码: %d", resp.StatusCode)}
	}

	content, err := io.ReadAll(io.LimitReader(resp.Body, config.MaxFileSize+1024))
	if err != nil {
		return nil, &contract.APIError{StatusCode: http.StatusBadGateway, Detail: fmt.Sprintf("读取远程文件失败: %v", err)}
	}

	if int64(len(content)) > config.MaxFileSize {
		return nil, &contract.APIError{StatusCode: http.StatusRequestEntityTooLarge, Detail: fmt.Sprintf("远程文件过大，最大支持 %dMB", config.MaxFileSize/(1024*1024))}
	}

	return content, nil
}

func ExtractFileNameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err == nil && u.Path != "" {
		name := filepath.Base(u.Path)
		if name != "" && name != "/" && name != "." {
			return name
		}
	}
	return "remote_download"
}

func DetectFileType(name string) string {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".apk") {
		return "apk"
	}
	if strings.HasSuffix(lower, ".zip") {
		return "zip"
	}
	return "text"
}
