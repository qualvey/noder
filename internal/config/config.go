package config

import (
	"os"
	"path/filepath"
)

var (
	BaseDir              string
	DBPath               string
	SingBoxTemplatePath  string
	XrayTemplatePath     string
	MihomoTemplatePath   string
	StaticDir            string
	FilesDir             string
	AdminSecretToken     string
	MallServiceSecret    string
	AllowedProtocols     = map[string]bool{"tuic": true, "vless": true, "anytls": true, "socks5": true}
	AllowedOverrideKeys  = map[string]bool{"route": true, "dns": true}
	AllowedFileTypes     = map[string]bool{"apk": true, "zip": true, "text": true}
	MaxFileSize          int64 = 100 * 1024 * 1024 // 100MB
	RemoteCacheTTL       int64 = 86400             // 1 天 (秒)
)

func Init(baseDir string) {
	if baseDir == "" {
		cwd, err := os.Getwd()
		if err == nil {
			BaseDir = cwd
		} else {
			BaseDir = "."
		}
	} else {
		BaseDir = baseDir
	}

	DBPath = filepath.Join(BaseDir, "data.db")
	SingBoxTemplatePath = filepath.Join(BaseDir, "templates", "sing-box.json")
	XrayTemplatePath = filepath.Join(BaseDir, "templates", "xray.json")
	MihomoTemplatePath = filepath.Join(BaseDir, "templates", "mihomo.yml")
	StaticDir = filepath.Join(BaseDir, "static")
	FilesDir = filepath.Join(BaseDir, "data", "files")

	AdminSecretToken = os.Getenv("ADMIN_SECRET_TOKEN")
	if AdminSecretToken == "" {
		AdminSecretToken = "admin-secret"
	}

	MallServiceSecret = os.Getenv("MALL_SERVICE_SECRET")
	if MallServiceSecret == "" {
		MallServiceSecret = os.Getenv("SERVICE_SECRET_TOKEN")
	}
	if MallServiceSecret == "" {
		MallServiceSecret = "service-secret"
	}
}

func init() {
	Init("")
}
