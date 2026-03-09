package services

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"web-sealdice/internal/deploy"
)

type runningProcess struct {
	cmd           *exec.Cmd
	startedAt     time.Time
	stopMu        sync.RWMutex
	stopRequested bool
}

// MarkStopRequested 标记进程已收到停止请求。
func (p *runningProcess) MarkStopRequested() {
	p.stopMu.Lock()
	defer p.stopMu.Unlock()
	p.stopRequested = true
}

// StopRequested 返回当前是否已标记停止。
func (p *runningProcess) StopRequested() bool {
	p.stopMu.RLock()
	defer p.stopMu.RUnlock()
	return p.stopRequested
}

type Manager struct {
	store             *Store
	dataDir           string
	logRetentionCount int
	logRetentionDays  int
	logMaxBytes       int64

	mu      sync.RWMutex
	running map[string]*runningProcess
}

// NewManager 创建运行时管理器。
func NewManager(store *Store, dataDir string, logRetentionCount int, logRetentionDays int, logMaxMB int) *Manager {
	maxBytes := int64(logMaxMB) * 1024 * 1024
	return &Manager{
		store:             store,
		dataDir:           strings.TrimSpace(dataDir),
		logRetentionCount: logRetentionCount,
		logRetentionDays:  logRetentionDays,
		logMaxBytes:       maxBytes,
		running:           make(map[string]*runningProcess),
	}
}

func (m *Manager) Start(id string) (Service, error) {
	item, err := m.store.Get(id)
	if err != nil {
		return Service{}, err
	}

	if item.Type == "LuckyLilliaBot" {
		refreshedExec, refreshErr := deploy.EnsureLLBotLauncher(item.InstallDir, nil)
		if refreshErr != nil {
			return Service{}, refreshErr
		}
		if strings.TrimSpace(refreshedExec) != "" {
			item.ExecPath = refreshedExec
			_ = m.store.Update(item)
		}
	}
	if item.Type == "Napcat" {
		refreshedExec, refreshErr := deploy.EnsureNapcatRunner(item.InstallDir, nil)
		if refreshErr != nil {
			return Service{}, refreshErr
		}
		if strings.TrimSpace(refreshedExec) != "" {
			item.ExecPath = refreshedExec
			_ = m.store.Update(item)
		}
	}

	m.mu.RLock()
	_, exists := m.running[id]
	m.mu.RUnlock()
	if exists {
		return Service{}, errors.New("service is already running")
	}

	workDir := item.WorkDir
	execPath := item.ExecPath
	if workDir == "" && execPath != "" {
		workDir = filepath.Dir(execPath)
	}
	if workDir == "" {
		workDir = "."
	}

	if absWorkDir, err := filepath.Abs(workDir); err == nil {
		workDir = absWorkDir
	}

	if execPath == "" {
		return Service{}, errors.New("exec path is empty")
	}
	if !filepath.IsAbs(execPath) {
		if strings.Contains(execPath, string(os.PathSeparator)) {
			if absExec, err := filepath.Abs(execPath); err == nil {
				execPath = absExec
			}
		} else {
			execPath = filepath.Join(workDir, execPath)
		}
	}
	if _, err := os.Stat(execPath); err != nil {
		return Service{}, fmt.Errorf("exec file not found: %s", execPath)
	}
	if item.ExecPath != execPath || item.WorkDir != workDir {
		item.ExecPath = execPath
		item.WorkDir = workDir
		_ = m.store.Update(item)
	}

	if item.Type == "Sealdice" {
		normalized := normalizeSealdiceArgs(item.Args)
		if !equalArgs(normalized, item.Args) {
			item.Args = normalized
			_ = m.store.Update(item)
		}
	}

	logPath, runDir, err := PrepareServiceRunLog(m.dataDir, id)
	if err != nil {
		return Service{}, err
	}
	item.LogPath = logPath
	logFile, err := os.OpenFile(item.LogPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return Service{}, err
	}

	cmd := exec.Command(execPath, item.Args...)
	cmd.Dir = workDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = buildEnv(item.Env)
	if item.Type == "LuckyLilliaBot" {
		cmd.Env = append(cmd.Env,
			"WEBSEAL_SERVICE_ID="+id,
			"WEBSEAL_SERVICE_TYPE=LuckyLilliaBot",
			"WEBSEAL_INSTALL_DIR="+item.InstallDir,
			fmt.Sprintf("WEBSEAL_SERVICE_PORT=%d", item.Port),
		)
	}
	if item.Type == "Napcat" {
		cmd.Env = append(cmd.Env,
			"WEBSEAL_SERVICE_ID="+id,
			"WEBSEAL_SERVICE_TYPE=Napcat",
			"WEBSEAL_INSTALL_DIR="+item.InstallDir,
			fmt.Sprintf("WEBSEAL_SERVICE_PORT=%d", item.Port),
		)
	}
	setProcessGroup(cmd)

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		item.LastError = err.Error()
		_ = m.store.Update(item)
		log.Printf("runtime: start failed id=%s exec=%s err=%v", id, execPath, err)
		return Service{}, err
	}

	item.Status = StatusRunning
	item.PID = cmd.Process.Pid
	item.LastError = ""
	item.LastStartAt = time.Now().UTC().Format(time.RFC3339)
	if err := m.store.Update(item); err != nil {
		_ = logFile.Close()
		_ = cmd.Process.Kill()
		return Service{}, err
	}

	m.mu.Lock()
	m.running[id] = &runningProcess{cmd: cmd, startedAt: time.Now()}
	m.mu.Unlock()
	log.Printf("runtime: started id=%s pid=%d exec=%s", id, item.PID, execPath)

	go m.wait(id, cmd, logFile, runDir)
	return item, nil
}

func (m *Manager) Stop(id string) (Service, error) {
	return m.stopInternal(id, false)
}

func (m *Manager) ForceStop(id string) (Service, error) {
	return m.stopInternal(id, true)
}

func (m *Manager) stopInternal(id string, force bool) (Service, error) {
	m.mu.RLock()
	proc, exists := m.running[id]
	m.mu.RUnlock()
	if !exists {
		item, err := m.store.Get(id)
		if err != nil {
			return Service{}, err
		}
		item.Status = StatusStopped
		item.PID = 0
		_ = m.store.Update(item)
		log.Printf("runtime: stop noop id=%s already stopped", id)
		return item, nil
	}

	if force {
		proc.MarkStopRequested()
		_ = signalProcessGroup(proc.cmd.Process, syscall.SIGKILL)
	} else {
		proc.MarkStopRequested()
		if err := signalProcessGroup(proc.cmd.Process, syscall.SIGINT); err != nil {
			_ = signalProcessGroup(proc.cmd.Process, syscall.SIGKILL)
		}
	}

	item, err := m.store.Get(id)
	if err != nil {
		return Service{}, err
	}
	item.Status = StatusStopped
	item.PID = 0
	if err := m.store.Update(item); err != nil {
		return Service{}, err
	}
	if item.Type == "LuckyLilliaBot" {
		cleanupLLBotProcesses(item.InstallDir, item.Port, proc.cmd.Process.Pid, id)
	}
	log.Printf("runtime: stopped id=%s force=%t", id, force)
	return item, nil
}

// Restart 先停止后拉起同一服务。
func (m *Manager) Restart(id string) (Service, error) {
	if _, err := m.stopInternal(id, false); err != nil {
		return Service{}, err
	}
	time.Sleep(250 * time.Millisecond)
	return m.Start(id)
}

func (m *Manager) wait(id string, cmd *exec.Cmd, logFile *os.File, runDir string) {
	err := cmd.Wait()
	_ = logFile.Close()

	// 归档策略优先读取服务级配置；未设置时回退到面板全局配置。
	archiveKeep := m.logRetentionCount
	archiveDays := m.logRetentionDays
	archiveMaxBytes := m.logMaxBytes
	if current, getErr := m.store.Get(id); getErr == nil {
		if current.LogPolicy.RetentionCount > 0 {
			archiveKeep = current.LogPolicy.RetentionCount
		}
		if current.LogPolicy.RetentionDays > 0 {
			archiveDays = current.LogPolicy.RetentionDays
		}
		if current.LogPolicy.MaxMB > 0 {
			archiveMaxBytes = int64(current.LogPolicy.MaxMB) * 1024 * 1024
		}
	}
	if archiveErr := ArchiveServiceRunLog(runDir, archiveKeep, archiveDays, archiveMaxBytes); archiveErr != nil {
		log.Printf("runtime: archive service log failed id=%s err=%v", id, archiveErr)
	}

	m.mu.Lock()
	proc := m.running[id]
	delete(m.running, id)
	m.mu.Unlock()

	startedAt := time.Time{}
	stopRequested := false
	if proc != nil {
		startedAt = proc.startedAt
		stopRequested = proc.StopRequested()
	}
	runDuration := time.Duration(0)
	if !startedAt.IsZero() {
		runDuration = time.Since(startedAt)
	}

	item, getErr := m.store.Get(id)
	if getErr == nil {
		item.Status = StatusStopped
		item.PID = 0
		item.LastExitAt = time.Now().UTC().Format(time.RFC3339)
		shouldRestart := false
		if err != nil {
			item.LastError = fmt.Sprintf("process exited: %v", err)
			log.Printf("runtime: exited with error id=%s err=%v", id, err)
		} else {
			log.Printf("runtime: exited id=%s", id)
			item.LastError = ""
		}

		if !stopRequested && item.Restart.Enabled {
			const crashWindow = 15 * time.Second
			if runDuration > 0 && runDuration < crashWindow {
				item.Restart.ConsecutiveCrash++
				log.Printf("runtime: crash detected id=%s consecutive=%d", id, item.Restart.ConsecutiveCrash)
			} else {
				item.Restart.ConsecutiveCrash = 0
			}
			if item.Restart.MaxCrashCount <= 0 || item.Restart.ConsecutiveCrash < item.Restart.MaxCrashCount {
				shouldRestart = true
			} else {
				item.LastError = fmt.Sprintf("auto restart stopped after %d consecutive crashes", item.Restart.ConsecutiveCrash)
				log.Printf("runtime: auto restart disabled by crash limit id=%s consecutive=%d", id, item.Restart.ConsecutiveCrash)
			}
		} else if stopRequested {
			item.Restart.ConsecutiveCrash = 0
		}
		_ = m.store.Update(item)
		if shouldRestart {
			delay := time.Duration(item.Restart.DelaySeconds) * time.Second
			if delay < 0 {
				delay = 0
			}
			go func(serviceID string, restartDelay time.Duration) {
				if restartDelay > 0 {
					time.Sleep(restartDelay)
				}
				current, err := m.store.Get(serviceID)
				if err != nil {
					return
				}
				if current.Status == StatusRunning || !current.Restart.Enabled {
					return
				}
				log.Printf("runtime: auto restarting id=%s delay=%s", serviceID, restartDelay)
				if _, err := m.Start(serviceID); err != nil {
					log.Printf("runtime: auto restart failed id=%s err=%v", serviceID, err)
				}
			}(id, delay)
		}
	}
}

// buildEnv 基于当前进程环境叠加服务自定义变量。
func buildEnv(extra map[string]string) []string {
	env := os.Environ()
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}

func (m *Manager) StartAutoServices() error {
	items, err := m.store.List()
	if err != nil {
		return err
	}

	for _, item := range items {
		if !item.AutoStart {
			continue
		}
		log.Printf("runtime: auto-start id=%s", item.ID)
		_, _ = m.Start(item.ID)
	}
	return nil
}

func (m *Manager) StopAllGraceful(timeout time.Duration) {
	m.mu.RLock()
	ids := make([]string, 0, len(m.running))
	for id := range m.running {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	if len(ids) == 0 {
		return
	}

	log.Printf("runtime: graceful stop begin count=%d", len(ids))
	for _, id := range ids {
		_, err := m.stopInternal(id, false)
		if err != nil {
			log.Printf("runtime: graceful stop failed id=%s err=%v", id, err)
		}
	}

	deadline := time.Now().Add(timeout)
	for {
		m.mu.RLock()
		remaining := len(m.running)
		m.mu.RUnlock()
		if remaining == 0 {
			log.Printf("runtime: graceful stop complete")
			return
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(120 * time.Millisecond)
	}

	m.mu.RLock()
	forceIDs := make([]string, 0, len(m.running))
	for id := range m.running {
		forceIDs = append(forceIDs, id)
	}
	m.mu.RUnlock()
	for _, id := range forceIDs {
		_, err := m.stopInternal(id, true)
		if err != nil {
			log.Printf("runtime: force stop on shutdown failed id=%s err=%v", id, err)
		}
	}
}

func normalizeSealdiceArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}
	out := make([]string, len(args))
	copy(out, args)
	for i, arg := range out {
		if strings.HasPrefix(arg, "--address:") {
			out[i] = "--address=" + strings.TrimPrefix(arg, "--address:")
		}
	}
	return out
}

// equalArgs 判断两个参数切片是否完全一致。
func equalArgs(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
