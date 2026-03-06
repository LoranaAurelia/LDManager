package bootstrap

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

	"web-sealdice/internal/config"
)

// RunLogInfo 描述本次运行日志目录及运行/访问日志文件路径。
type RunLogInfo struct {
	RootDir    string
	RunDir     string
	RuntimeLog string
	AccessLog  string
}

// PrepareRunLogs 实现该函数对应的业务逻辑。
func PrepareRunLogs(cfg config.Config) (RunLogInfo, error) {
	root := filepath.Join(cfg.DataDir, "logs", "runs")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return RunLogInfo{}, err
	}

	if err := compressRunDirs(root); err != nil {
		return RunLogInfo{}, err
	}
	if err := trimRunArchives(root, cfg.LogRetentionCount, cfg.LogRetentionDays, int64(cfg.LogMaxMB)*1024*1024); err != nil {
		return RunLogInfo{}, err
	}

	runName := time.Now().Format("20060102-150405")
	runDir := filepath.Join(root, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return RunLogInfo{}, err
	}

	return RunLogInfo{
		RootDir:    root,
		RunDir:     runDir,
		RuntimeLog: filepath.Join(runDir, "runtime.log"),
		AccessLog:  filepath.Join(runDir, "access.log"),
	}, nil
}

// compressRunDirs 压缩目标内容并生成归档。
func compressRunDirs(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		srcDir := filepath.Join(root, entry.Name())
		dstTarGz := srcDir + ".tar.gz"
		if _, err := os.Stat(dstTarGz); err == nil {
			continue
		}
		if err := compressDirToTarGz(srcDir, dstTarGz); err != nil {
			return err
		}
		if err := os.RemoveAll(srcDir); err != nil {
			return err
		}
	}
	return nil
}

// trimRunArchives 实现该函数对应的业务逻辑。
func trimRunArchives(root string, keep int, days int, maxBytes int64) error {
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

	// archive 记录归档文件路径与时间，用于排序和清理。
	type archive struct {
		path    string
		modTime time.Time
	}
	archives := make([]archive, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".gz" || filepath.Ext(strings.TrimSuffix(entry.Name(), ".gz")) != ".tar" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
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

// compressDirToTarGz 压缩目标内容并生成归档。
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

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(filepath.Dir(srcDir), path)
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
			return fmt.Errorf("copy file to tar: %w", copyErr)
		}
		if closeErr != nil {
			return closeErr
		}
		return nil
	})
}
