package services

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Store 负责服务元数据的持久化读写。
// 当前实现使用 JSON 文件存储在 <dataDir>/services 目录。
type Store struct {
	dir string
	mu  sync.RWMutex
}

// NewStore 创建服务存储对象。
func NewStore(dataDir string) *Store {
	return &Store{
		dir: filepath.Join(dataDir, "services"),
	}
}

// Create 创建并持久化一条服务记录。
// 注意：这里只保存元数据，不会启动进程。
func (s *Store) Create(req CreateServiceRequest, logsDir string) (Service, error) {
	if err := validateCreateRequest(req); err != nil {
		return Service{}, err
	}

	id := strings.TrimSpace(req.ID)
	if id == "" {
		var err error
		id, err = generateID()
		if err != nil {
			return Service{}, err
		}
	}

	item := Service{
		ID:          id,
		Name:        strings.TrimSpace(req.Name),
		DisplayName: strings.TrimSpace(req.DisplayName),
		Type:        strings.TrimSpace(req.Type),
		WorkDir:     strings.TrimSpace(req.WorkDir),
		InstallDir:  strings.TrimSpace(req.InstallDir),
		ExecPath:    strings.TrimSpace(req.ExecPath),
		Args:        req.Args,
		Env:         req.Env,
		Port:        req.Port,
		AutoStart:   req.AutoStart,
		OpenPathURL: "",
		Restart:     normalizeRestartPolicy(req.Restart),
		DeployMeta:  req.DeployMeta,
		Status:      StatusStopped,
		PID:         0,
		LogPath:     filepath.Join(logsDir, id+".log"),
	}
	if item.DisplayName == "" {
		item.DisplayName = item.Name
	}
	if item.Name == "" {
		item.Name = item.ID
	}
	item.MarkCreated()

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return Service{}, err
	}
	if _, err := os.Stat(s.pathFor(id)); err == nil {
		return Service{}, fmt.Errorf("service id already exists: %s", id)
	}
	if err := writeServiceFile(s.pathFor(id), item); err != nil {
		return Service{}, err
	}
	return item, nil
}

// Get 按 ID 读取服务记录。
func (s *Store) Get(id string) (Service, error) {
	if strings.TrimSpace(id) == "" {
		return Service{}, errors.New("service id is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return readServiceFile(s.pathFor(id))
}

// List 读取全部服务并按创建时间倒序返回。
func (s *Store) List() ([]Service, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Service{}, nil
	}
	if err != nil {
		return nil, err
	}

	services := make([]Service, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		item, err := readServiceFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		services = append(services, item)
	}

	sort.Slice(services, func(i, j int) bool {
		return services[i].CreatedAt > services[j].CreatedAt
	})
	return services, nil
}

// Update 更新服务记录并刷新 UpdatedAt。
func (s *Store) Update(item Service) error {
	if strings.TrimSpace(item.ID) == "" {
		return errors.New("service id is required")
	}
	item.Touch()

	s.mu.Lock()
	defer s.mu.Unlock()
	return writeServiceFile(s.pathFor(item.ID), item)
}

// Delete 删除指定服务记录文件。
func (s *Store) Delete(id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("service id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.Remove(s.pathFor(id))
}

// pathFor 返回服务记录对应的 JSON 文件路径。
func (s *Store) pathFor(id string) string {
	return filepath.Join(s.dir, id+".json")
}

// validateCreateRequest 校验创建请求参数。
func validateCreateRequest(req CreateServiceRequest) error {
	if strings.TrimSpace(req.Type) == "" {
		return errors.New("type is required")
	}
	if strings.TrimSpace(req.ID) != "" {
		if err := ValidateRegistryName(req.ID); err != nil {
			return err
		}
	}
	if strings.TrimSpace(req.ExecPath) == "" {
		return errors.New("exec_path is required")
	}
	if req.Port < 0 || req.Port > 65535 {
		return errors.New("invalid port")
	}
	if err := validateRestartPolicy(req.Restart); err != nil {
		return err
	}
	return nil
}

// validateRestartPolicy 约束自动重启参数范围，避免异常配置。
func validateRestartPolicy(p RestartPolicy) error {
	if p.DelaySeconds < 0 || p.DelaySeconds > 86400 {
		return errors.New("invalid restart delay_seconds")
	}
	if p.MaxCrashCount < 0 || p.MaxCrashCount > 1000 {
		return errors.New("invalid restart max_crash_count")
	}
	return nil
}

// normalizeRestartPolicy 修复负值，保证运行时可以直接使用。
func normalizeRestartPolicy(p RestartPolicy) RestartPolicy {
	if p.DelaySeconds < 0 {
		p.DelaySeconds = 0
	}
	if p.MaxCrashCount < 0 {
		p.MaxCrashCount = 0
	}
	return p
}

// NormalizeService 对读取到的服务对象进行轻量规范化。
func NormalizeService(item *Service) {
	if item == nil {
		return
	}
	item.Restart = normalizeRestartPolicy(item.Restart)
}

// generateID 生成 16 位十六进制随机 ID。
func generateID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// readServiceFile 从 JSON 文件读取服务记录。
func readServiceFile(path string) (Service, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Service{}, err
	}
	var item Service
	if err := json.Unmarshal(raw, &item); err != nil {
		return Service{}, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	NormalizeService(&item)
	return item, nil
}

// writeServiceFile 将服务记录写入 JSON 文件。
func writeServiceFile(path string, item Service) error {
	raw, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
