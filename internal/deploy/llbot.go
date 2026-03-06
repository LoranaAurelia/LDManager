package deploy

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	llbotRepoBase        = "https://github.com/LLOneBot/LuckyLilliaBot"
	llbotLatestAssetBase = llbotRepoBase + "/releases/latest/download"
	llbotReleaseBase     = llbotRepoBase + "/releases/download"
)

// LLBotDeployer 负责 LuckyLilliaBot 的安装目录规划与部署流程。
type LLBotDeployer struct {
	baseDir string
}

// NewLLBotDeployer 创建 LLBot 部署器。
func NewLLBotDeployer(dataDir string) *LLBotDeployer {
	return &LLBotDeployer{
		baseDir: filepath.Join(dataDir, "LLBot"),
	}
}

// TargetDir 计算指定服务注册名对应的安装目录。
func (d *LLBotDeployer) TargetDir(registryName string) string {
	return filepath.Join(d.baseDir, registryName)
}

// EnsureLLBotLauncher 启动前刷新 Linux 启动脚本，避免脚本漂移导致启动失败。
func EnsureLLBotLauncher(installDir string, logf func(string, ...any)) (string, error) {
	execPath, err := findLLBotExecutable(installDir)
	if err != nil {
		return "", err
	}
	if runtime.GOOS != "linux" {
		return execPath, nil
	}
	if logf != nil {
		logf("runtime: refreshing llbot launcher")
	}
	return writeLLBotHeadlessScript(installDir, execPath)
}

// DeployFromAuto 下载并解压 LLBot 发布包，修复权限并返回启动入口。
func (d *LLBotDeployer) DeployFromAuto(registryName string, version string, logf func(string, ...any)) (string, error) {
	targetDir := d.TargetDir(registryName)
	if _, err := os.Stat(targetDir); err == nil {
		return "", fmt.Errorf("target directory already exists: %s", targetDir)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}
	cleanupOnFail := true
	defer func() {
		if cleanupOnFail {
			_ = os.RemoveAll(targetDir)
		}
	}()
	if logf != nil {
		logf("deploy: target dir %s", targetDir)
	}

	asset, err := llbotAssetName()
	if err != nil {
		return "", err
	}
	url := llbotDownloadURL(asset, version)
	if logf != nil {
		logf("deploy: downloading %s", url)
	}

	client := &http.Client{Timeout: 10 * time.Minute}
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
	if err := extractZipFromReader(resp.Body, targetDir); err != nil {
		return "", err
	}
	if err := fixLLBotPermissions(targetDir, logf); err != nil {
		return "", err
	}

	execPath, err := findLLBotExecutable(targetDir)
	if err != nil {
		return "", err
	}
	if logf != nil {
		logf("deploy: executable %s", execPath)
	}

	if runtime.GOOS == "linux" {
		if logf != nil {
			logf("deploy: writing headless launcher")
		}
		wrapper, err := writeLLBotHeadlessScript(targetDir, execPath)
		if err != nil {
			return "", err
		}
		cleanupOnFail = false
		return wrapper, nil
	}

	cleanupOnFail = false
	return execPath, nil
}

// DeployFromURL 从指定 URL 下载 LLBot 压缩包并部署。
func (d *LLBotDeployer) DeployFromURL(registryName string, rawURL string, logf func(string, ...any)) (string, error) {
	targetDir := d.TargetDir(registryName)
	if _, err := os.Stat(targetDir); err == nil {
		return "", fmt.Errorf("target directory already exists: %s", targetDir)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}
	cleanupOnFail := true
	defer func() {
		if cleanupOnFail {
			_ = os.RemoveAll(targetDir)
		}
	}()

	url := strings.TrimSpace(rawURL)
	if url == "" {
		return "", errors.New("download url is required")
	}
	if logf != nil {
		logf("deploy: target dir %s", targetDir)
		logf("deploy: downloading %s", url)
	}

	client := &http.Client{Timeout: 10 * time.Minute}
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
	if err := extractZipFromReader(resp.Body, targetDir); err != nil {
		return "", err
	}
	if err := fixLLBotPermissions(targetDir, logf); err != nil {
		return "", err
	}
	execPath, err := findLLBotExecutable(targetDir)
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "linux" {
		wrapper, err := writeLLBotHeadlessScript(targetDir, execPath)
		if err != nil {
			return "", err
		}
		cleanupOnFail = false
		return wrapper, nil
	}
	cleanupOnFail = false
	return execPath, nil
}

// DeployFromReader 从上传流读取 LLBot 压缩包并完成部署。
func (d *LLBotDeployer) DeployFromReader(registryName string, reader io.Reader, logf func(string, ...any)) (string, error) {
	targetDir := d.TargetDir(registryName)
	if _, err := os.Stat(targetDir); err == nil {
		return "", fmt.Errorf("target directory already exists: %s", targetDir)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}
	cleanupOnFail := true
	defer func() {
		if cleanupOnFail {
			_ = os.RemoveAll(targetDir)
		}
	}()
	if logf != nil {
		logf("deploy: target dir %s", targetDir)
		logf("deploy: extracting uploaded package")
	}

	if err := extractZipFromReader(reader, targetDir); err != nil {
		return "", err
	}
	if err := fixLLBotPermissions(targetDir, logf); err != nil {
		return "", err
	}

	execPath, err := findLLBotExecutable(targetDir)
	if err != nil {
		return "", err
	}
	if logf != nil {
		logf("deploy: executable %s", execPath)
	}

	if runtime.GOOS == "linux" {
		if logf != nil {
			logf("deploy: writing headless launcher")
		}
		wrapper, err := writeLLBotHeadlessScript(targetDir, execPath)
		if err != nil {
			return "", err
		}
		cleanupOnFail = false
		return wrapper, nil
	}

	cleanupOnFail = false
	return execPath, nil
}

// UpdateLLBotWebUIPort 修改 default_config.json 中的 WebUI 端口配置。
func UpdateLLBotWebUIPort(installDir string, port int) error {
	if port < 1 || port > 65535 {
		return errors.New("webui port must be in 1-65535")
	}
	configPath, err := findLLBotConfigPath(installDir)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return err
	}

	webui, ok := root["webui"].(map[string]any)
	if !ok {
		webui = map[string]any{}
		root["webui"] = webui
	}
	webui["enable"] = true
	webui["host"] = "0.0.0.0"
	webui["port"] = port

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, out, 0o644)
}

// ReadLLBotWebUIPort 读取 default_config.json 的当前 WebUI 端口。
func ReadLLBotWebUIPort(installDir string) (int, error) {
	configPath, err := findLLBotConfigPath(installDir)
	if err != nil {
		return 0, err
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return 0, err
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return 0, err
	}
	webui, ok := root["webui"].(map[string]any)
	if !ok {
		return 0, errors.New("webui section not found")
	}
	port := llbotToInt(webui["port"])
	if port < 1 || port > 65535 {
		return 0, errors.New("invalid webui port in config")
	}
	return port, nil
}

// findLLBotConfigPath 在安装目录中定位 default_config.json。
func findLLBotConfigPath(installDir string) (string, error) {
	base := strings.TrimSpace(installDir)
	if base == "" {
		return "", errors.New("llbot install dir is empty")
	}
	candidates := []string{
		filepath.Join(base, "bin", "llbot", "default_config.json"),
		filepath.Join(base, "default_config.json"),
	}
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, nil
		}
	}

	var found string
	var errFound = errors.New("found")
	err := filepath.WalkDir(base, func(pathNow string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), "default_config.json") && strings.Contains(filepath.ToSlash(pathNow), "/bin/llbot/") {
			found = pathNow
			return errFound
		}
		return nil
	})
	if err != nil && !errors.Is(err, errFound) {
		return "", err
	}
	if found == "" {
		return "", errors.New("default_config.json not found under bin/llbot")
	}
	return found, nil
}

// llbotToInt 将 JSON number 兼容转换为 int。
func llbotToInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float32:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

// llbotAssetName 根据当前 OS/ARCH 选择对应发行包名称。
func llbotAssetName() (string, error) {
	switch runtime.GOOS {
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "LLBot-CLI-linux-x64.zip", nil
		case "arm64":
			return "LLBot-CLI-linux-arm64.zip", nil
		default:
			return "", fmt.Errorf("LuckyLilliaBot unsupported arch: %s", runtime.GOARCH)
		}
	case "windows":
		if runtime.GOARCH != "amd64" {
			return "", fmt.Errorf("LuckyLilliaBot unsupported arch: %s", runtime.GOARCH)
		}
		return "LLBot-CLI-win-x64.zip", nil
	default:
		return "", fmt.Errorf("LuckyLilliaBot unsupported os: %s", runtime.GOOS)
	}
}

// llbotDownloadURL 按版本策略拼接 LLBot 下载地址。
func llbotDownloadURL(asset string, version string) string {
	v := strings.TrimSpace(version)
	if v == "" || strings.EqualFold(v, "latest") {
		return llbotLatestAssetBase + "/" + asset
	}
	return llbotReleaseBase + "/" + v + "/" + asset
}

// extractZipFromReader 将下载流落盘后安全解压 zip。
func extractZipFromReader(src io.Reader, dst string) error {
	tempFile, err := os.CreateTemp("", "llbot-*.zip")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer func() { _ = os.Remove(tempPath) }()

	if _, err := io.Copy(tempFile, src); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}

	zr, err := zip.OpenReader(tempPath)
	if err != nil {
		return err
	}
	defer func() { _ = zr.Close() }()

	cleanDst := filepath.Clean(dst)
	for _, f := range zr.File {
		name := strings.Trim(filepath.ToSlash(f.Name), "/")
		if name == "" {
			continue
		}
		target := filepath.Join(cleanDst, filepath.FromSlash(name))
		target = filepath.Clean(target)
		if target != cleanDst && !strings.HasPrefix(target, cleanDst+string(os.PathSeparator)) {
			return errors.New("invalid archive entry path")
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode())
		if err != nil {
			_ = in.Close()
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = in.Close()
			_ = out.Close()
			return err
		}
		_ = in.Close()
		_ = out.Close()
	}
	return nil
}

// findLLBotExecutable 在安装目录定位主执行入口。
func findLLBotExecutable(targetDir string) (string, error) {
	candidates := make([]string, 0, 6)
	preferred := []string{
		filepath.Join(targetDir, "llbot"),
		filepath.Join(targetDir, "LLBot"),
		filepath.Join(targetDir, "bin", "llbot"),
		filepath.Join(targetDir, "bin", "LLBot"),
		filepath.Join(targetDir, "llbot.exe"),
		filepath.Join(targetDir, "LLBot.exe"),
	}
	for _, p := range preferred {
		info, err := os.Stat(p)
		if err == nil && !info.IsDir() {
			_ = os.Chmod(p, 0o755)
			return p, nil
		}
	}

	err := filepath.WalkDir(targetDir, func(pathNow string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		if name == "llbot" || name == "llbot.exe" {
			candidates = append(candidates, pathNow)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", errors.New("llbot executable not found after extraction")
	}
	_ = os.Chmod(candidates[0], 0o755)
	return candidates[0], nil
}

// fixLLBotPermissions 为关键二进制和脚本设置可执行权限。
func fixLLBotPermissions(targetDir string, logf func(string, ...any)) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	candidates := []string{
		filepath.Join(targetDir, "llbot"),
		filepath.Join(targetDir, "LLBot"),
		filepath.Join(targetDir, "bin", "llbot", "node"),
		filepath.Join(targetDir, "bin", "pmhq", "pmhq"),
	}
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			_ = os.Chmod(p, 0o755)
		}
	}

	pmhq := filepath.Join(targetDir, "bin", "pmhq", "pmhq")
	if info, err := os.Stat(pmhq); err == nil && !info.IsDir() {
		if os.Geteuid() == 0 {
			_ = os.Chown(pmhq, 0, 0)
			_ = os.Chmod(pmhq, 0o4755)
			if logf != nil {
				logf("deploy: pmhq setuid root applied")
			}
		} else if logf != nil {
			logf("deploy: pmhq requires root; run as root to apply setuid")
		}
	}
	return nil
}

// writeLLBotHeadlessScript 生成 headless 启动脚本并注入运行参数。
func writeLLBotHeadlessScript(targetDir string, execPath string) (string, error) {
	scriptPath := filepath.Join(targetDir, "start-headless.sh")
	relPath := execPath
	if targetDir != "" {
		if rel, err := filepath.Rel(targetDir, execPath); err == nil && !strings.HasPrefix(rel, "..") {
			relPath = filepath.ToSlash(rel)
		}
	}
	useScriptDir := true
	if filepath.IsAbs(relPath) {
		useScriptDir = false
	} else if !strings.HasPrefix(relPath, "./") {
		relPath = "./" + relPath
	}
	cliLine := fmt.Sprintf("LLBOT_CLI_BIN=\"%s\"", relPath)
	if useScriptDir {
		cliLine = fmt.Sprintf("LLBOT_CLI_BIN=\"$SCRIPT_DIR/%s\"", relPath)
	}
	content := fmt.Sprintf(`#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
export PATH=$PATH:/usr/bin:/usr/local/bin
export PMHQ_HEADLESS=true
export ELECTRON_DISABLE_GPU=1
export LIBGL_ALWAYS_SOFTWARE=1

%s
if [ ! -x "$LLBOT_CLI_BIN" ]; then
  echo "llbot executable not found: $LLBOT_CLI_BIN"
  exit 1
fi

if ! command -v xvfb-run >/dev/null 2>&1; then
  echo "xvfb-run not found. Please install dependencies first."
  exit 1
fi

IM_ENV="XMODIFIERS=@im=fcitx"
if [[ "${XDG_SESSION_TYPE:-}" == "wayland" || -n "${WAYLAND_DISPLAY:-}" ]]; then
  EXTRA_FLAGS="--enable-features=UseOzonePlatform --ozone-platform=wayland --enable-wayland-ime"
else
  IM_ENV="GTK_IM_MODULE=fcitx QT_IM_MODULE=fcitx $IM_ENV SDL_IM_MODULE=fcitx GLFW_IM_MODULE=ibus"
fi

exec env $IM_ENV xvfb-run -a "$LLBOT_CLI_BIN" "$@"
`, cliLine)

	if err := os.WriteFile(scriptPath, []byte(content), 0o755); err != nil {
		return "", err
	}
	return scriptPath, nil
}
