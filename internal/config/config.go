package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type fileConfig struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
		Output string `yaml:"output"`
	} `yaml:"logging"`
	Security struct {
		AdminSecretToken  string `yaml:"admin_secret_token"`
		MallServiceSecret string `yaml:"mall_service_secret"`
	} `yaml:"security"`
}

var (
	BaseDir             string
	DBPath              string
	SingBoxTemplatePath string
	XrayTemplatePath    string
	MihomoTemplatePath  string
	StaticDir           string
	FilesDir            string
	ConfigPath          string
	ServerHost          string
	ServerPort          int
	LogLevel            string
	LogFormat           string
	LogOutput           string
	AdminSecretToken    string
	MallServiceSecret   string
	AllowedProtocols          = map[string]bool{"tuic": true, "vless": true, "anytls": true, "socks5": true}
	AllowedOverrideKeys       = map[string]bool{"route": true, "dns": true}
	AllowedFileTypes          = map[string]bool{"apk": true, "zip": true, "text": true}
	MaxFileSize         int64 = 100 * 1024 * 1024 // 100MB
	RemoteCacheTTL      int64 = 86400             // 1 天 (秒)
)

func setDefaults(baseDir string) {
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
	ConfigPath = filepath.Join(BaseDir, "config.yaml")
	ServerHost = "0.0.0.0"
	ServerPort = 8000
	LogLevel = "info"
	LogFormat = "text"
	LogOutput = "stderr"

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

// Load loads the application configuration. The precedence is:
// defaults < config.yaml < environment variables.
func Load(baseDir string) error {
	setDefaults(baseDir)

	if path := strings.TrimSpace(os.Getenv("NODER_CONFIG_FILE")); path != "" {
		ConfigPath = path
	}

	data, err := os.ReadFile(ConfigPath)
	if err == nil {
		var fileCfg fileConfig
		if err := yaml.Unmarshal(data, &fileCfg); err != nil {
			return fmt.Errorf("parse config file %s: %w", ConfigPath, err)
		}
		if fileCfg.Server.Host != "" {
			ServerHost = fileCfg.Server.Host
		}
		if fileCfg.Server.Port != 0 {
			ServerPort = fileCfg.Server.Port
		}
		if fileCfg.Logging.Level != "" {
			LogLevel = fileCfg.Logging.Level
		}
		if fileCfg.Logging.Format != "" {
			LogFormat = fileCfg.Logging.Format
		}
		if fileCfg.Logging.Output != "" {
			LogOutput = fileCfg.Logging.Output
		}
		if fileCfg.Security.AdminSecretToken != "" {
			AdminSecretToken = fileCfg.Security.AdminSecretToken
		}
		if fileCfg.Security.MallServiceSecret != "" {
			MallServiceSecret = fileCfg.Security.MallServiceSecret
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read config file %s: %w", ConfigPath, err)
	}

	if value := strings.TrimSpace(os.Getenv("HOST")); value != "" {
		ServerHost = value
	}
	if value := strings.TrimSpace(os.Getenv("PORT")); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid PORT %q: %w", value, err)
		}
		ServerPort = port
	}
	if value := strings.TrimSpace(os.Getenv("LOG_LEVEL")); value != "" {
		LogLevel = value
	}
	if value := strings.TrimSpace(os.Getenv("LOG_FORMAT")); value != "" {
		LogFormat = value
	}
	if value := strings.TrimSpace(os.Getenv("LOG_OUTPUT")); value != "" {
		LogOutput = value
	}
	if value := os.Getenv("ADMIN_SECRET_TOKEN"); value != "" {
		AdminSecretToken = value
	}
	if value := os.Getenv("MALL_SERVICE_SECRET"); value != "" {
		MallServiceSecret = value
	} else if value := os.Getenv("SERVICE_SECRET_TOKEN"); value != "" {
		MallServiceSecret = value
	}

	if ServerHost == "" {
		return fmt.Errorf("server host must not be empty")
	}
	if ServerPort < 1 || ServerPort > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535, got %d", ServerPort)
	}
	return nil
}

// Init is kept for callers and tests that use the original initialization API.
// The server uses Load so configuration errors can be reported cleanly.
func Init(baseDir string) {
	if err := Load(baseDir); err != nil {
		setDefaults(baseDir)
	}
}

func init() {
	setDefaults("")
}
