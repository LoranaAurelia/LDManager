package server

import (
	"bufio"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"web-sealdice/internal/services"
)

// handleMetricsOverview 处理对应 HTTP 请求。
func (s *Server) handleMetricsOverview(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"system":      s.collectSystemMetrics(),
		"application": s.collectApplicationMetrics(),
	})
}

// collectSystemMetrics 收集并汇总运行指标。
func (s *Server) collectSystemMetrics() map[string]any {
	totalMem, availMem := readLinuxMemInfo()
	swapTotal, swapFree := readLinuxSwapInfo()
	load1, load5, load15 := readLinuxLoadAvg()
	diskTotal, diskFree := readLinuxDiskUsage("/")
	hostCPU, panelCPU := s.sampleCPUUsage()
	return map[string]any{
		"goos":                    runtime.GOOS,
		"goarch":                  runtime.GOARCH,
		"cpu_model":               readLinuxCPUModel(),
		"cpu_cores":               runtime.NumCPU(),
		"cpu_host_percent":        hostCPU,
		"cpu_panel_percent":       panelCPU,
		"metrics_refresh_seconds": s.cfg.MetricsRefreshSec,
		"mem_total_bytes":         totalMem,
		"mem_available_bytes":     availMem,
		"swap_total_bytes":        swapTotal,
		"swap_free_bytes":         swapFree,
		"load_1":                  load1,
		"load_5":                  load5,
		"load_15":                 load15,
		"disk_mount":              "/",
		"disk_total_bytes":        diskTotal,
		"disk_free_bytes":         diskFree,
		"disk_used_bytes":         maxInt64(diskTotal-diskFree, 0),
	}
}

// readLinuxSwapInfo 读取并返回相关数据。
func readLinuxSwapInfo() (int64, int64) {
	if runtime.GOOS != "linux" {
		return 0, 0
	}
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer func() { _ = f.Close() }()

	var total int64
	var free int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "SwapTotal:") {
			total = parseMemInfoLine(line)
		}
		if strings.HasPrefix(line, "SwapFree:") {
			free = parseMemInfoLine(line)
		}
	}
	return total, free
}

// collectApplicationMetrics 收集并汇总运行指标。
func (s *Server) collectApplicationMetrics() map[string]any {
	items, _ := s.serviceStore.List()
	selfRSS := readProcessRSSBytes(os.Getpid())
	totalMem, availMem := readLinuxMemInfo()
	var servicesRSS int64
	var running int
	for _, item := range items {
		if item.Status != services.StatusRunning || item.PID <= 0 {
			continue
		}
		running++
		servicesRSS += readProcessRSSBytes(item.PID)
	}
	dataSize := dirSizeBytes(s.cfg.DataDir)
	return map[string]any{
		"service_count":         len(items),
		"running_service_count": running,
		"panel_rss_bytes":       selfRSS,
		"services_rss_bytes":    servicesRSS,
		"total_rss_bytes":       selfRSS + servicesRSS,
		"other_used_bytes":      maxInt64((totalMem-availMem)-(selfRSS+servicesRSS), 0),
		"free_bytes":            maxInt64(availMem, 0),
		"data_size_bytes":       dataSize,
	}
}

// collectServiceMetrics 收集并汇总运行指标。
func (s *Server) collectServiceMetrics(item services.Service) map[string]any {
	installSize := dirSizeBytes(strings.TrimSpace(item.InstallDir))
	if installSize == 0 {
		installSize = dirSizeBytes(strings.TrimSpace(item.WorkDir))
	}
	logSize := fileSizeBytes(strings.TrimSpace(item.LogPath))
	return map[string]any{
		"id":                 item.ID,
		"type":               item.Type,
		"status":             item.Status,
		"pid":                item.PID,
		"rss_bytes":          readProcessRSSBytes(item.PID),
		"install_size_bytes": installSize,
		"log_size_bytes":     logSize,
	}
}

// readLinuxMemInfo 读取并返回相关数据。
func readLinuxMemInfo() (int64, int64) {
	if runtime.GOOS != "linux" {
		return 0, 0
	}
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer func() { _ = f.Close() }()

	var total int64
	var avail int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			total = parseMemInfoLine(line)
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			avail = parseMemInfoLine(line)
		}
	}
	return total, avail
}

// parseMemInfoLine 解析输入并转换为结构化结果。
func parseMemInfoLine(line string) int64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	v, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return 0
	}
	return v * 1024
}

// readLinuxLoadAvg 读取并返回相关数据。
func readLinuxLoadAvg() (float64, float64, float64) {
	if runtime.GOOS != "linux" {
		return 0, 0, 0
	}
	raw, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Fields(string(raw))
	if len(fields) < 3 {
		return 0, 0, 0
	}
	a, _ := strconv.ParseFloat(fields[0], 64)
	b, _ := strconv.ParseFloat(fields[1], 64)
	c, _ := strconv.ParseFloat(fields[2], 64)
	return a, b, c
}

// readLinuxCPUModel 读取并返回相关数据。
func readLinuxCPUModel() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	raw, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "model name") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return ""
		}
		return strings.TrimSpace(parts[1])
	}
	return ""
}

// readProcessRSSBytes 读取并返回相关数据。
func readProcessRSSBytes(pid int) int64 {
	if pid <= 0 || runtime.GOOS != "linux" {
		return 0
	}
	f, err := os.Open(filepath.Join("/proc", strconv.Itoa(pid), "status"))
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		return parseMemInfoLine(line)
	}
	return 0
}

// dirSizeBytes 实现该函数对应的业务逻辑。
func dirSizeBytes(root string) int64 {
	root = strings.TrimSpace(root)
	if root == "" {
		return 0
	}
	var total int64
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

// fileSizeBytes 实现该函数对应的业务逻辑。
func fileSizeBytes(path string) int64 {
	path = strings.TrimSpace(path)
	if path == "" {
		return 0
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return 0
	}
	return info.Size()
}

// maxInt64 实现该函数对应的业务逻辑。
func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
