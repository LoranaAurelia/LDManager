package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 定义面板配置文件结构。
// 该结构会映射到 YAML（config.yaml）并可直接序列化到前端。
type Config struct {
	ListenHost        string `yaml:"listen_host" json:"listen_host"`
	ListenPort        int    `yaml:"listen_port" json:"listen_port"`
	BasePath          string `yaml:"base_path" json:"base_path"`
	DataDir           string `yaml:"-" json:"-"`
	AuthDir           string `yaml:"-" json:"-"`
	TrustProxyHeaders bool   `yaml:"trust_proxy_headers" json:"trust_proxy_headers"`
	DisableHTTPSWarn  bool   `yaml:"disable_https_warning" json:"disable_https_warning"`
	LogRetentionCount int    `yaml:"log_retention_count" json:"log_retention_count"`
	LogRetentionDays  int    `yaml:"log_retention_days" json:"log_retention_days"`
	LogMaxMB          int    `yaml:"log_max_mb" json:"log_max_mb"`
	FileManager       struct {
		Enabled     bool `yaml:"enabled" json:"enabled"`
		UploadMaxMB int  `yaml:"upload_max_mb" json:"upload_max_mb"`
	} `yaml:"file_manager" json:"file_manager"`
	LoginProtect struct {
		Enabled       bool `yaml:"enabled" json:"enabled"`
		MaxAttempts   int  `yaml:"max_attempts" json:"max_attempts"`
		WindowSeconds int  `yaml:"window_seconds" json:"window_seconds"`
		BlockSeconds  int  `yaml:"block_seconds" json:"block_seconds"`
	} `yaml:"login_protect" json:"login_protect"`
	MetricsRefreshSec int    `yaml:"metrics_refresh_seconds" json:"metrics_refresh_seconds"`
	SessionCookieName string `yaml:"session_cookie_name" json:"session_cookie_name"`
	SessionTTLHours   int    `yaml:"session_ttl_hours" json:"session_ttl_hours"`
	SessionSecret     string `yaml:"session_secret" json:"session_secret"`
}

// Default 返回默认配置。
// 这些值用于首次安装和旧配置缺项时的兜底。
func Default() Config {
	return Config{
		ListenHost:        "0.0.0.0",
		ListenPort:        3210,
		BasePath:          "/",
		DataDir:           "./data",
		AuthDir:           "./auth",
		TrustProxyHeaders: false,
		LogRetentionCount: 10,
		LogRetentionDays:  30,
		LogMaxMB:          2048,
		FileManager: struct {
			Enabled     bool `yaml:"enabled" json:"enabled"`
			UploadMaxMB int  `yaml:"upload_max_mb" json:"upload_max_mb"`
		}{
			Enabled:     true,
			UploadMaxMB: 2048,
		},
		LoginProtect: struct {
			Enabled       bool `yaml:"enabled" json:"enabled"`
			MaxAttempts   int  `yaml:"max_attempts" json:"max_attempts"`
			WindowSeconds int  `yaml:"window_seconds" json:"window_seconds"`
			BlockSeconds  int  `yaml:"block_seconds" json:"block_seconds"`
		}{
			Enabled:       true,
			MaxAttempts:   20,
			WindowSeconds: 600,
			BlockSeconds:  600,
		},
		MetricsRefreshSec: 2,
		SessionCookieName: "sealpanel_session",
		SessionTTLHours:   48,
		SessionSecret:     mustNewSecret(),
	}
}

// ListenAddr 返回 net/http 可直接使用的监听地址（host:port）。
func (c Config) ListenAddr() string {
	return fmt.Sprintf("%s:%d", c.ListenHost, c.ListenPort)
}

// PasswordHashPath 返回密码哈希文件路径。
func (c Config) PasswordHashPath() string {
	return filepath.Join(c.AuthDir, "password.hash")
}

// LoadOrCreate 读取配置；若配置不存在则写入默认配置并返回。
// 第二个返回值 created 表示是否在本次调用中新建了配置文件。
func LoadOrCreate(path string) (Config, bool, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		cfg := Default()
		if err := writeConfig(path, cfg); err != nil {
			return Config{}, false, err
		}
		return cfg, true, nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, false, err
	}

	cfg := Default()
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, false, err
	}

	secretWasEmpty := strings.TrimSpace(cfg.SessionSecret) == ""
	cfg.normalize()
	changed := false
	if secretWasEmpty {
		cfg.SessionSecret = mustNewSecret()
		changed = true
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, false, err
	}
	if changed {
		if err := writeConfig(path, cfg); err != nil {
			return Config{}, false, err
		}
	}
	return cfg, false, nil
}

// Save 将配置写回磁盘。
func Save(path string, cfg Config) error {
	return writeConfig(path, cfg)
}

// writeConfig 先规范化并校验，再写入 YAML。
func writeConfig(path string, cfg Config) error {
	cfg.normalize()
	if err := cfg.Validate(); err != nil {
		return err
	}

	content, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, content, 0o644)
}

// normalize 修复空值/非法值，保证配置可运行。
func (c *Config) normalize() {
	if c.ListenHost == "" {
		c.ListenHost = "0.0.0.0"
	}
	if c.ListenPort == 0 {
		c.ListenPort = 3210
	}
	if c.DataDir == "" {
		c.DataDir = "./data"
	}
	if c.AuthDir == "" {
		c.AuthDir = "./auth"
	}
	if c.LogRetentionCount <= 0 {
		c.LogRetentionCount = 10
	}
	if c.LogRetentionDays <= 0 {
		c.LogRetentionDays = 30
	}
	if c.LogMaxMB <= 0 {
		c.LogMaxMB = 2048
	}
	if !c.FileManager.Enabled && c.FileManager.UploadMaxMB <= 0 {
		c.FileManager.Enabled = true
	}
	if c.FileManager.UploadMaxMB <= 0 {
		c.FileManager.UploadMaxMB = 2048
	}
	if c.LoginProtect.MaxAttempts <= 0 {
		c.LoginProtect.MaxAttempts = 20
	}
	if c.LoginProtect.WindowSeconds <= 0 {
		c.LoginProtect.WindowSeconds = 600
	}
	if c.LoginProtect.BlockSeconds <= 0 {
		c.LoginProtect.BlockSeconds = 600
	}
	if c.MetricsRefreshSec <= 0 {
		c.MetricsRefreshSec = 2
	}
	if c.SessionCookieName == "" {
		c.SessionCookieName = "sealpanel_session"
	}
	if c.SessionTTLHours <= 0 {
		c.SessionTTLHours = 48
	}

	base := strings.TrimSpace(c.BasePath)
	if base == "" {
		base = "/"
	}
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	if len(base) > 1 {
		base = strings.TrimRight(base, "/")
	}
	c.BasePath = base
}

// Validate 校验配置值是否合法。
func (c Config) Validate() error {
	if c.ListenPort < 1 || c.ListenPort > 65535 {
		return fmt.Errorf("invalid listen_port: %d", c.ListenPort)
	}
	if !strings.HasPrefix(c.BasePath, "/") {
		return fmt.Errorf("base_path must start with '/': %s", c.BasePath)
	}
	if c.SessionTTLHours < 1 || c.SessionTTLHours > 24*365 {
		return fmt.Errorf("invalid session_ttl_hours: %d", c.SessionTTLHours)
	}
	if c.MetricsRefreshSec < 1 || c.MetricsRefreshSec > 3600 {
		return fmt.Errorf("invalid metrics_refresh_seconds: %d", c.MetricsRefreshSec)
	}
	if c.LogRetentionCount < 1 || c.LogRetentionCount > 365 {
		return fmt.Errorf("invalid log_retention_count: %d", c.LogRetentionCount)
	}
	if c.LogRetentionDays < 1 || c.LogRetentionDays > 3650 {
		return fmt.Errorf("invalid log_retention_days: %d", c.LogRetentionDays)
	}
	if c.LogMaxMB < 16 || c.LogMaxMB > 1024*1024 {
		return fmt.Errorf("invalid log_max_mb: %d", c.LogMaxMB)
	}
	if c.FileManager.UploadMaxMB < 1 || c.FileManager.UploadMaxMB > 4096 {
		return fmt.Errorf("invalid file_manager.upload_max_mb: %d", c.FileManager.UploadMaxMB)
	}
	if c.LoginProtect.MaxAttempts < 1 || c.LoginProtect.MaxAttempts > 10000 {
		return fmt.Errorf("invalid login_protect.max_attempts: %d", c.LoginProtect.MaxAttempts)
	}
	if c.LoginProtect.WindowSeconds < 1 || c.LoginProtect.WindowSeconds > 86400 {
		return fmt.Errorf("invalid login_protect.window_seconds: %d", c.LoginProtect.WindowSeconds)
	}
	if c.LoginProtect.BlockSeconds < 1 || c.LoginProtect.BlockSeconds > 86400 {
		return fmt.Errorf("invalid login_protect.block_seconds: %d", c.LoginProtect.BlockSeconds)
	}
	if strings.TrimSpace(c.SessionSecret) == "" {
		return errors.New("session_secret cannot be empty")
	}
	return nil
}

// SessionTTL 返回会话时长（小时）。
// 兼容历史非法值：小于等于 0 时回退为默认 48 小时。
func (c Config) SessionTTL() int {
	if c.SessionTTLHours <= 0 {
		return 48
	}
	return c.SessionTTLHours
}

// mustNewSecret 生成会话签名密钥。
func mustNewSecret() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "change_me_session_secret"
	}
	return hex.EncodeToString(buf)
}
