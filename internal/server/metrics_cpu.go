package server

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"web-sealdice/internal/services"
)

// cpuSampleSnapshot 记录一次 CPU 采样快照，用于计算区间占用率。
type cpuSampleSnapshot struct {
	capturedAt time.Time
	hostIdle   uint64
	hostTotal  uint64
	procTotal  uint64
}

// sampleCPUUsage 实现该函数对应的业务逻辑。
func (s *Server) sampleCPUUsage() (float64, float64) {
	if runtime.GOOS != "linux" {
		return 0, 0
	}

	items, _ := s.serviceStore.List()
	pids := make([]int, 0, len(items)+1)
	pids = append(pids, os.Getpid())
	for _, item := range items {
		if item.Status != services.StatusRunning || item.PID <= 0 {
			continue
		}
		pids = append(pids, item.PID)
	}

	current, ok := readCPUSnapshot(pids)
	if !ok {
		return 0, 0
	}

	s.metricsMu.Lock()
	prev := s.cpuSample
	s.cpuSample = current
	s.metricsMu.Unlock()

	if prev.capturedAt.IsZero() {
		time.Sleep(220 * time.Millisecond)
		second, ok := readCPUSnapshot(pids)
		if !ok {
			return 0, 0
		}
		s.metricsMu.Lock()
		s.cpuSample = second
		s.metricsMu.Unlock()
		return cpuPercentBetween(current, second)
	}

	return cpuPercentBetween(prev, current)
}

// cpuPercentBetween 实现该函数对应的业务逻辑。
func cpuPercentBetween(prev cpuSampleSnapshot, curr cpuSampleSnapshot) (float64, float64) {
	hostDelta := curr.hostTotal - prev.hostTotal
	idleDelta := curr.hostIdle - prev.hostIdle
	if hostDelta == 0 || curr.hostTotal < prev.hostTotal || curr.hostIdle < prev.hostIdle || curr.procTotal < prev.procTotal {
		return 0, 0
	}

	hostUsed := float64(hostDelta-idleDelta) / float64(hostDelta) * 100
	procUsed := float64(curr.procTotal-prev.procTotal) / float64(hostDelta) * 100
	return clampPercent(hostUsed), clampPercent(procUsed)
}

// readCPUSnapshot 读取并返回相关数据。
func readCPUSnapshot(pids []int) (cpuSampleSnapshot, bool) {
	idle, total, ok := readHostCPUTotals()
	if !ok {
		return cpuSampleSnapshot{}, false
	}

	var procTotal uint64
	for _, pid := range pids {
		if pid <= 0 {
			continue
		}
		procTotal += readProcessCPUJiffies(pid)
	}

	return cpuSampleSnapshot{
		capturedAt: time.Now(),
		hostIdle:   idle,
		hostTotal:  total,
		procTotal:  procTotal,
	}, true
}

// readHostCPUTotals 读取并返回相关数据。
func readHostCPUTotals() (uint64, uint64, bool) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0, 0, false
		}

		var total uint64
		var idle uint64
		for idx, field := range fields[1:] {
			v, err := strconv.ParseUint(field, 10, 64)
			if err != nil {
				return 0, 0, false
			}
			total += v
			if idx == 3 || idx == 4 {
				idle += v
			}
		}
		return idle, total, true
	}

	return 0, 0, false
}

// readProcessCPUJiffies 读取并返回相关数据。
func readProcessCPUJiffies(pid int) uint64 {
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0
	}

	data := string(raw)
	end := strings.LastIndexByte(data, ')')
	if end < 0 || end+2 >= len(data) {
		return 0
	}

	fields := strings.Fields(data[end+2:])
	if len(fields) < 15 {
		return 0
	}

	utime, err := strconv.ParseUint(fields[11], 10, 64)
	if err != nil {
		return 0
	}
	stime, err := strconv.ParseUint(fields[12], 10, 64)
	if err != nil {
		return 0
	}
	return utime + stime
}

// clampPercent 实现该函数对应的业务逻辑。
func clampPercent(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
