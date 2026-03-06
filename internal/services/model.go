package services

import (
	"encoding/json"
	"time"
)

const (
	StatusStopped = "stopped"
	StatusRunning = "running"
)

// Service 描述一个被面板托管的服务实例及其运行状态。
type Service struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Type         string            `json:"type"`
	WorkDir      string            `json:"work_dir"`
	InstallDir   string            `json:"install_dir"`
	ExecPath     string            `json:"exec_path"`
	Args         []string          `json:"args"`
	Env          map[string]string `json:"env"`
	Port         int               `json:"port"`
	AutoStart    bool              `json:"auto_start"`
	OpenPathURL  string            `json:"open_path_url"`
	Restart      RestartPolicy     `json:"restart"`
	LogPolicy    LogPolicy         `json:"log_policy"`
	Status       string            `json:"status"`
	PID          int               `json:"pid"`
	LogPath      string            `json:"log_path"`
	LastError    string            `json:"last_error"`
	LastExitAt   string            `json:"last_exit_at"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
	LastStartAt  string            `json:"last_start_at"`
	ProjectAlias string            `json:"project_alias"`
	DeployMeta   DeployMeta        `json:"deploy_meta"`
}

// RestartPolicy 定义服务自动重启策略。
type RestartPolicy struct {
	Enabled          bool `json:"enabled"`
	DelaySeconds     int  `json:"delay_seconds"`
	MaxCrashCount    int  `json:"max_crash_count"`
	ConsecutiveCrash int  `json:"consecutive_crash"`
}

// CreateServiceRequest 定义创建服务时需要的请求参数。
type CreateServiceRequest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Type       string            `json:"type"`
	WorkDir    string            `json:"work_dir"`
	InstallDir string            `json:"install_dir"`
	ExecPath   string            `json:"exec_path"`
	Args       []string          `json:"args"`
	Env        map[string]string `json:"env"`
	Port       int               `json:"port"`
	AutoStart  bool              `json:"auto_start"`
	Restart    RestartPolicy     `json:"restart"`
	LogPolicy  LogPolicy         `json:"log_policy"`
	DeployMeta DeployMeta        `json:"deploy_meta"`
}

// LogPolicy 定义单服务日志归档策略。
// 字段为 0 时表示“继承面板全局配置”。
type LogPolicy struct {
	RetentionCount int `json:"retention_count"`
	RetentionDays  int `json:"retention_days"`
	MaxMB          int `json:"max_mb"`
}

// DeployMeta 以 kind+payload 方式保存部署相关元数据。
type DeployMeta struct {
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

// MarkCreated 实现该函数对应的业务逻辑。
func (s *Service) MarkCreated() {
	now := time.Now().UTC().Format(time.RFC3339)
	s.CreatedAt = now
	s.UpdatedAt = now
	if s.Status == "" {
		s.Status = StatusStopped
	}
}

// Touch 将输入转换为目标类型。
func (s *Service) Touch() {
	s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}
