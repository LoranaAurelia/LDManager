//go:build linux

package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// cleanupLLBotProcesses 实现该函数对应的业务逻辑。
func cleanupLLBotProcesses(installDir string, port int, pid int, serviceID string) {
	if strings.TrimSpace(installDir) == "" {
		installDir = ""
	}
	cleanDir := ""
	if installDir != "" {
		cleanDir = filepath.Clean(installDir)
		if cleanDir == "." || cleanDir == "/" {
			cleanDir = ""
		}
	}
	if cleanDir != "" {
		killByInstallDir(cleanDir)
	}
	if port > 0 && port <= 65535 {
		killByListeningPort(port)
	}
	if serviceID != "" {
		killByEnvMatch("WEBSEAL_SERVICE_ID", serviceID)
	}
	if cleanDir != "" {
		killByEnvMatch("WEBSEAL_INSTALL_DIR", cleanDir)
	}
	if pid > 1 {
		killByProcessTree(pid)
	}
}

// killByInstallDir 实现该函数对应的业务逻辑。
func killByInstallDir(installDir string) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 1 {
			continue
		}
		if matchesInstallDir(pid, installDir) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}

// matchesInstallDir 实现该函数对应的业务逻辑。
func matchesInstallDir(pid int, installDir string) bool {
	prefix := installDir + string(os.PathSeparator)
	exe, _ := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if exe != "" && (exe == installDir || strings.HasPrefix(exe, prefix)) {
		return true
	}
	cwd, _ := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
	if cwd != "" && (cwd == installDir || strings.HasPrefix(cwd, prefix)) {
		return true
	}
	cmdline, _ := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if len(cmdline) > 0 && strings.Contains(string(cmdline), installDir) {
		return true
	}
	return false
}

// killByListeningPort 实现该函数对应的业务逻辑。
func killByListeningPort(port int) {
	inodes := make(map[string]struct{})
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		_ = collectListeningInodes(path, port, inodes)
	}
	if len(inodes) == 0 {
		return
	}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 1 {
			continue
		}
		if pidOwnsInode(pid, inodes) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}

// killByProcessTree 实现该函数对应的业务逻辑。
func killByProcessTree(rootPID int) {
	descendants := collectDescendants(rootPID)
	for _, pid := range descendants {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	_ = syscall.Kill(rootPID, syscall.SIGKILL)
}

// collectDescendants 收集并汇总运行指标。
func collectDescendants(rootPID int) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	children := make(map[int][]int)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 1 {
			continue
		}
		ppid, ok := readPPid(pid)
		if !ok {
			continue
		}
		children[ppid] = append(children[ppid], pid)
	}

	var out []int
	queue := []int{rootPID}
	seen := map[int]struct{}{rootPID: {}}
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		for _, child := range children[pid] {
			if _, ok := seen[child]; ok {
				continue
			}
			seen[child] = struct{}{}
			out = append(out, child)
			queue = append(queue, child)
		}
	}
	return out
}

// readPPid 读取并返回相关数据。
func readPPid(pid int) (int, bool) {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "PPid:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, false
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil {
			return 0, false
		}
		return ppid, true
	}
	return 0, false
}

// killByEnvMatch 实现该函数对应的业务逻辑。
func killByEnvMatch(key string, value string) {
	if key == "" || value == "" {
		return
	}
	target := key + "=" + value
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 1 {
			continue
		}
		raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
		if err != nil || len(raw) == 0 {
			continue
		}
		if strings.Contains(string(raw), target) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}

// collectListeningInodes 收集并汇总运行指标。
func collectListeningInodes(path string, port int, out map[string]struct{}) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "sl") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		local := fields[1]
		state := fields[3]
		if state != "0A" {
			continue
		}
		hostPort := strings.Split(local, ":")
		if len(hostPort) != 2 {
			continue
		}
		p, err := strconv.ParseInt(hostPort[1], 16, 32)
		if err != nil || int(p) != port {
			continue
		}
		inode := fields[9]
		if inode != "" {
			out[inode] = struct{}{}
		}
	}
	return nil
}

// pidOwnsInode 实现该函数对应的业务逻辑。
func pidOwnsInode(pid int, inodes map[string]struct{}) bool {
	fdDir := fmt.Sprintf("/proc/%d/fd", pid)
	entries, err := os.ReadDir(fdDir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		link, err := os.Readlink(filepath.Join(fdDir, entry.Name()))
		if err != nil {
			continue
		}
		if strings.HasPrefix(link, "socket:[") && strings.HasSuffix(link, "]") {
			inode := strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]")
			if _, ok := inodes[inode]; ok {
				return true
			}
		}
	}
	return false
}
