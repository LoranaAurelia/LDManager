package deploy

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultSealdiceLinuxAMD64URL = "https://d1.sealdice.com/sealdice-core_1.5.1_linux_amd64.tar.gz"

// SealdiceDeployer 负责 Sealdice 安装目录规划与部署。
type SealdiceDeployer struct {
	baseDir string
}

// NewSealdiceDeployer 创建 Sealdice 部署器。
func NewSealdiceDeployer(dataDir string) *SealdiceDeployer {
	return &SealdiceDeployer{
		baseDir: filepath.Join(dataDir, "Sealdice"),
	}
}

// TargetDir 计算指定服务注册名对应的安装目录。
func (d *SealdiceDeployer) TargetDir(registryName string) string {
	return filepath.Join(d.baseDir, registryName)
}

// DeployFromURL 从远端下载 Sealdice 压缩包并完成解压部署。
func (d *SealdiceDeployer) DeployFromURL(registryName string, url string, logf func(string, ...any)) (string, error) {
	if strings.TrimSpace(url) == "" {
		url = DefaultSealdiceLinuxAMD64URL
	}

	targetDir := d.TargetDir(registryName)
	if _, err := os.Stat(targetDir); err == nil {
		return "", fmt.Errorf("target directory already exists: %s", targetDir)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}
	if logf != nil {
		logf("deploy: target dir %s", targetDir)
		logf("deploy: downloading %s", url)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}

	if logf != nil {
		logf("deploy: extracting package")
	}
	if err := extractTarGz(resp.Body, targetDir); err != nil {
		return "", err
	}
	execPath := filepath.Join(targetDir, "sealdice-core")
	_ = os.Chmod(execPath, 0o755)
	if logf != nil {
		logf("deploy: executable %s", execPath)
	}
	return execPath, nil
}

// DeployFromReader 从上传流读取压缩包并完成解压部署。
func (d *SealdiceDeployer) DeployFromReader(registryName string, reader io.Reader, logf func(string, ...any)) (string, error) {
	targetDir := d.TargetDir(registryName)
	if _, err := os.Stat(targetDir); err == nil {
		return "", fmt.Errorf("target directory already exists: %s", targetDir)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}
	if logf != nil {
		logf("deploy: target dir %s", targetDir)
		logf("deploy: extracting uploaded package")
	}

	if err := extractTarGz(reader, targetDir); err != nil {
		return "", err
	}
	execPath := filepath.Join(targetDir, "sealdice-core")
	_ = os.Chmod(execPath, 0o755)
	if logf != nil {
		logf("deploy: executable %s", execPath)
	}
	return execPath, nil
}

// extractTarGz 安全解压 tar.gz 到目标目录，包含路径校验。
func extractTarGz(src io.Reader, dst string) error {
	gz, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	cleanDst := filepath.Clean(dst)

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		name := strings.TrimSpace(hdr.Name)
		if name == "" {
			continue
		}

		relative := stripFirstPathComponent(name)
		if relative == "" {
			continue
		}

		targetPath := filepath.Join(cleanDst, relative)
		targetPath = filepath.Clean(targetPath)
		if !strings.HasPrefix(targetPath, cleanDst+string(os.PathSeparator)) && targetPath != cleanDst {
			return fmt.Errorf("invalid archive path: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		default:
			// Skip symlink/device/special entries for safety.
		}
	}
	return nil
}

// stripFirstPathComponent 去掉压缩包顶层目录，避免多包一层目录嵌套。
func stripFirstPathComponent(name string) string {
	clean := strings.Trim(filepath.ToSlash(name), "/")
	if clean == "" || clean == "." {
		return ""
	}
	parts := strings.Split(clean, "/")
	if len(parts) <= 1 {
		return parts[0]
	}
	return strings.Join(parts[1:], string(os.PathSeparator))
}
