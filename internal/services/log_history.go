package services

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const serviceLogFileName = "console.log"

// ServiceLogHistoryItem 描述单条服务历史日志记录。
type ServiceLogHistoryItem struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	EndedAt   string `json:"ended_at"`
	Size      int64  `json:"size"`
	Archived  bool   `json:"archived"`
}

// PrepareServiceRunLog 为服务创建本次运行日志目录并返回日志文件路径。
func PrepareServiceRunLog(dataDir string, serviceID string) (string, string, error) {
	root := ServiceLogRoot(dataDir, serviceID)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", "", err
	}
	runName := time.Now().Format("20060102-150405")
	runDir := filepath.Join(root, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", "", err
	}
	return filepath.Join(runDir, serviceLogFileName), runDir, nil
}

// ServiceLogRoot 返回服务历史日志根目录：<data>/logs/services/<id>。
func ServiceLogRoot(dataDir string, serviceID string) string {
	return filepath.Join(dataDir, "logs", "services", SanitizeName(serviceID))
}

// ArchiveServiceRunLog 将单次运行日志目录压缩为 tar.gz 并清理历史归档。
func ArchiveServiceRunLog(runDir string, keep int, days int, maxBytes int64) error {
	runDir = filepath.Clean(strings.TrimSpace(runDir))
	if runDir == "" {
		return nil
	}
	info, err := os.Stat(runDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	dst := runDir + ".tar.gz"
	if _, err := os.Stat(dst); err == nil {
		return trimServiceLogArchives(filepath.Dir(runDir), keep, days, maxBytes)
	}
	if err := compressDirToTarGz(runDir, dst); err != nil {
		return err
	}
	_ = os.RemoveAll(runDir)
	return trimServiceLogArchives(filepath.Dir(runDir), keep, days, maxBytes)
}

// ListServiceLogHistory 列出服务历史日志目录与归档（按时间倒序）。
func ListServiceLogHistory(dataDir string, serviceID string) ([]ServiceLogHistoryItem, error) {
	root := ServiceLogRoot(dataDir, serviceID)
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return []ServiceLogHistoryItem{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]ServiceLogHistoryItem, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(root, name)
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		if entry.IsDir() {
			startAt := inferRunStartTime(name, info.ModTime())
			items = append(items, ServiceLogHistoryItem{
				Name:      name,
				CreatedAt: startAt.UTC().Format(time.RFC3339),
				EndedAt:   info.ModTime().UTC().Format(time.RFC3339),
				Size:      dirSize(path),
				Archived:  false,
			})
			continue
		}
		if !strings.HasSuffix(strings.ToLower(name), ".tar.gz") {
			continue
		}
		startAt := inferRunStartTime(strings.TrimSuffix(name, ".tar.gz"), info.ModTime())
		items = append(items, ServiceLogHistoryItem{
			Name:      name,
			CreatedAt: startAt.UTC().Format(time.RFC3339),
			EndedAt:   info.ModTime().UTC().Format(time.RFC3339),
			Size:      info.Size(),
			Archived:  true,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt > items[j].CreatedAt
	})
	return items, nil
}

// ReadServiceLogHistoryTail 读取指定历史日志（目录或 .tar.gz）的尾部内容。
func ReadServiceLogHistoryTail(dataDir string, serviceID string, name string, lines int) (string, error) {
	if lines <= 0 {
		lines = 300
	}
	base := ServiceLogRoot(dataDir, serviceID)
	targetAbs, err := resolvePathWithin(base, name)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(targetAbs)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return TailFile(filepath.Join(targetAbs, serviceLogFileName), lines)
	}
	if strings.HasSuffix(strings.ToLower(targetAbs), ".tar.gz") {
		raw, readErr := readTarGzEntry(targetAbs, filepath.Base(strings.TrimSuffix(targetAbs, ".tar.gz"))+"/"+serviceLogFileName)
		if readErr != nil {
			// 兼容部分打包结构，尝试直接读取 console.log。
			raw, readErr = readTarGzEntry(targetAbs, serviceLogFileName)
			if readErr != nil {
				return "", readErr
			}
		}
		return tailText(string(raw), lines), nil
	}
	return "", fmt.Errorf("unsupported history log item: %s", name)
}

// DeleteServiceLogHistory 删除指定历史日志项（目录会话或 .tar.gz 归档）。
func DeleteServiceLogHistory(dataDir string, serviceID string, name string) error {
	base := ServiceLogRoot(dataDir, serviceID)
	targetAbs, err := resolvePathWithin(base, name)
	if err != nil {
		return err
	}
	info, err := os.Stat(targetAbs)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.RemoveAll(targetAbs)
	}
	return os.Remove(targetAbs)
}

// ClearServiceLogHistory 清空指定服务全部历史日志。
func ClearServiceLogHistory(dataDir string, serviceID string) error {
	root := ServiceLogRoot(dataDir, serviceID)
	if err := os.RemoveAll(root); err != nil {
		return err
	}
	return os.MkdirAll(root, 0o755)
}

func trimServiceLogArchives(root string, keep int, days int, maxBytes int64) error {
	if keep < 1 {
		keep = 1
	}
	if days < 1 {
		days = 1
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	type archive struct {
		path    string
		modTime time.Time
	}
	archives := make([]archive, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if !strings.HasSuffix(name, ".tar.gz") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		archives = append(archives, archive{
			path:    filepath.Join(root, entry.Name()),
			modTime: info.ModTime(),
		})
	}
	sort.Slice(archives, func(i, j int) bool {
		return archives[i].modTime.After(archives[j].modTime)
	})
	expireBefore := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	totalSize := int64(0)
	for _, item := range archives {
		info, infoErr := os.Stat(item.path)
		if infoErr == nil {
			totalSize += info.Size()
		}
	}
	for idx, old := range archives {
		info, infoErr := os.Stat(old.path)
		size := int64(0)
		if infoErr == nil {
			size = info.Size()
		}
		shouldDropByCount := idx >= keep
		shouldDropByDays := old.modTime.Before(expireBefore)
		shouldDropBySpace := maxBytes > 0 && totalSize > maxBytes
		if !shouldDropByCount && !shouldDropByDays && !shouldDropBySpace {
			continue
		}
		_ = os.Remove(old.path)
		totalSize -= size
	}
	return nil
}

func compressDirToTarGz(srcDir string, dstPath string) error {
	out, err := os.OpenFile(dstPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	gz, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()

	tw := tar.NewWriter(gz)
	defer func() { _ = tw.Close() }()

	base := filepath.Dir(srcDir)
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, in)
		closeErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func readTarGzEntry(path string, entryName string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	want := filepath.ToSlash(strings.TrimSpace(entryName))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		name := filepath.ToSlash(strings.TrimSpace(hdr.Name))
		if name != want && filepath.Base(name) != filepath.Base(want) {
			continue
		}
		return io.ReadAll(tr)
	}
	return nil, os.ErrNotExist
}

func tailText(content string, lines int) string {
	if lines <= 0 {
		return content
	}
	arr := strings.Split(content, "\n")
	for len(arr) > 0 && arr[len(arr)-1] == "" {
		arr = arr[:len(arr)-1]
	}
	if len(arr) <= lines {
		return strings.Join(arr, "\n")
	}
	return strings.Join(arr[len(arr)-lines:], "\n")
}

func dirSize(root string) int64 {
	var size int64
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		size += info.Size()
		return nil
	})
	return size
}

func inferRunStartTime(name string, fallback time.Time) time.Time {
	t, err := time.Parse("20060102-150405", strings.TrimSpace(name))
	if err != nil {
		return fallback
	}
	return t
}

func resolvePathWithin(root string, rel string) (string, error) {
	cleanRoot := filepath.Clean(strings.TrimSpace(root))
	if cleanRoot == "" {
		return "", fmt.Errorf("invalid root")
	}
	cleanRel := filepath.Clean(strings.TrimSpace(rel))
	if cleanRel == "." || cleanRel == "" {
		return "", fmt.Errorf("invalid path")
	}
	candidate := filepath.Join(cleanRoot, cleanRel)
	absRoot, err := filepath.Abs(cleanRoot)
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	rootWithSep := absRoot + string(os.PathSeparator)
	if absPath != absRoot && !strings.HasPrefix(absPath, rootWithSep) {
		return "", fmt.Errorf("path escapes root")
	}
	return absPath, nil
}
