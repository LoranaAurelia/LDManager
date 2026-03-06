package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"web-sealdice/internal/config"
	"web-sealdice/internal/services"
)

// panelSettingsRequest 表示面板配置保存请求。
type panelSettingsRequest struct {
	Mode   string `json:"mode"`
	Raw    string `json:"raw"`
	Config struct {
		ListenHost                string `json:"listen_host"`
		ListenPort                int    `json:"listen_port"`
		BasePath                  string `json:"base_path"`
		TrustProxyHeaders         bool   `json:"trust_proxy_headers"`
		DisableHTTPSWarning       bool   `json:"disable_https_warning"`
		LogRetentionCount         int    `json:"log_retention_count"`
		LogRetentionDays          int    `json:"log_retention_days"`
		LogMaxMB                  int    `json:"log_max_mb"`
		FileManagerEnabled        *bool  `json:"file_manager_enabled"`
		FileUploadMaxMB           *int   `json:"file_upload_max_mb"`
		LoginProtectEnabled       *bool  `json:"login_protect_enabled"`
		LoginProtectMaxAttempts   *int   `json:"login_protect_max_attempts"`
		LoginProtectWindowSeconds *int   `json:"login_protect_window_seconds"`
		LoginProtectBlockSeconds  *int   `json:"login_protect_block_seconds"`
		MetricsRefreshSec         int    `json:"metrics_refresh_seconds"`
		SessionTTLHours           int    `json:"session_ttl_hours"`
	} `json:"config"`
}

// handlePanelSettings 处理面板配置查询与保存。
func (s *Server) handlePanelSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handlePanelSettingsGet(w)
	case http.MethodPost:
		s.handlePanelSettingsPost(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
	}
}

// handlePanelLogsClear 清空面板与托管服务历史日志目录。
func (s *Server) handlePanelLogsClear(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	root := filepath.Join(s.cfg.DataDir, "logs")
	if err := os.RemoveAll(root); err != nil {
		writeJSON(w, http.StatusInternalServerError, jsonMessage{Message: err.Error()})
		return
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, jsonMessage{Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, jsonMessage{Message: "logs cleared"})
}

// handlePanelSettingsGet 返回当前配置（原始文本 + 结构化对象）。
func (s *Server) handlePanelSettingsGet(w http.ResponseWriter) {
	raw, err := os.ReadFile(s.configPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, jsonMessage{Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"config_path": s.configPath,
		"raw":         string(raw),
		"config":      s.cfg,
	})
}

// handlePanelSettingsPost 保存新配置并在可热更新项上即时生效。
func (s *Server) handlePanelSettingsPost(w http.ResponseWriter, r *http.Request) {
	body, err := decodePanelSettingsRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}

	next, err := buildNextPanelConfig(s.cfg, body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}

	if requiresStoppedServicesForConfigChange(s.cfg, next) {
		running, err := s.hasRunningServices()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, jsonMessage{Message: err.Error()})
			return
		}
		if running {
			writeJSON(w, http.StatusConflict, jsonMessage{Message: "please stop all services before changing data_dir"})
			return
		}
	}

	if err := config.Save(s.configPath, next); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}
	s.applyRuntimeConfig(next)

	raw, _ := os.ReadFile(s.configPath)
	writeJSON(w, http.StatusOK, map[string]any{
		"message":     "panel settings saved",
		"config_path": s.configPath,
		"raw":         string(raw),
		"config":      s.cfg,
	})
}

// decodePanelSettingsRequest 解析并校验请求体 JSON。
func decodePanelSettingsRequest(r *http.Request) (panelSettingsRequest, error) {
	var body panelSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return panelSettingsRequest{}, errors.New("invalid json body")
	}
	return body, nil
}

// buildNextPanelConfig 按 mode 组装目标配置。
func buildNextPanelConfig(current config.Config, body panelSettingsRequest) (config.Config, error) {
	next := current
	mode := strings.ToLower(strings.TrimSpace(body.Mode))

	if mode == "raw" {
		raw := strings.TrimSpace(body.Raw)
		if raw == "" {
			return config.Config{}, errors.New("raw config cannot be empty")
		}
		if err := yaml.Unmarshal([]byte(raw), &next); err != nil {
			return config.Config{}, err
		}
		if strings.TrimSpace(next.SessionSecret) == "" {
			next.SessionSecret = current.SessionSecret
		}
		return next, nil
	}

	next.ListenHost = strings.TrimSpace(body.Config.ListenHost)
	next.ListenPort = body.Config.ListenPort
	next.BasePath = strings.TrimSpace(body.Config.BasePath)
	next.TrustProxyHeaders = body.Config.TrustProxyHeaders
	next.DisableHTTPSWarn = body.Config.DisableHTTPSWarning
	next.LogRetentionCount = body.Config.LogRetentionCount
	next.LogRetentionDays = body.Config.LogRetentionDays
	next.LogMaxMB = body.Config.LogMaxMB
	if body.Config.FileManagerEnabled != nil {
		next.FileManager.Enabled = *body.Config.FileManagerEnabled
	}
	if body.Config.FileUploadMaxMB != nil {
		next.FileManager.UploadMaxMB = *body.Config.FileUploadMaxMB
	}
	if body.Config.LoginProtectEnabled != nil {
		next.LoginProtect.Enabled = *body.Config.LoginProtectEnabled
	}
	if body.Config.LoginProtectMaxAttempts != nil {
		next.LoginProtect.MaxAttempts = *body.Config.LoginProtectMaxAttempts
	}
	if body.Config.LoginProtectWindowSeconds != nil {
		next.LoginProtect.WindowSeconds = *body.Config.LoginProtectWindowSeconds
	}
	if body.Config.LoginProtectBlockSeconds != nil {
		next.LoginProtect.BlockSeconds = *body.Config.LoginProtectBlockSeconds
	}
	next.MetricsRefreshSec = body.Config.MetricsRefreshSec
	next.SessionTTLHours = body.Config.SessionTTLHours

	return next, nil
}

// requiresStoppedServicesForConfigChange 判断本次配置修改是否要求先停服。
func requiresStoppedServicesForConfigChange(current config.Config, next config.Config) bool {
	return strings.TrimSpace(current.DataDir) != strings.TrimSpace(next.DataDir)
}

// hasRunningServices 返回当前是否存在运行中的托管服务。
func (s *Server) hasRunningServices() (bool, error) {
	items, err := s.serviceStore.List()
	if err != nil {
		return false, err
	}
	for _, item := range items {
		if item.Status == services.StatusRunning {
			return true, nil
		}
	}
	return false, nil
}
