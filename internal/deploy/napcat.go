package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultNapcatWebUIPort      = 6099
	DefaultNapcatInstallScript  = "https://raw.githubusercontent.com/NapNeko/napcat-linux-installer/refs/heads/main/install.sh"
	NapcatMirrorMoeyyScript     = "https://github.moeyy.xyz/https://raw.githubusercontent.com/NapNeko/napcat-linux-installer/refs/heads/main/install.sh"
	NapcatMirrorJiashuScript    = "https://jiashu.1win.eu.org/https://raw.githubusercontent.com/NapNeko/napcat-linux-installer/refs/heads/main/install.sh"
	napcatScriptFilename        = "install-napcat.sh"
	napcatRunnerFilename        = "start-napcat.sh"
	napcatLauncherSOName        = "libnapcat_launcher.so"
	napcatQQBinary              = "/opt/QQ/qq"
	napcatInstallScriptTimeout  = 20 * time.Minute
	napcatDownloadScriptTimeout = 60 * time.Second
)

var napcatURLRegex = regexp.MustCompile(`https?://[^\s"'<>]+`)

// NapcatDeployer 负责 Napcat 安装目录规划与部署。
type NapcatDeployer struct {
	baseDir string
}

// NapcatDeployOptions 描述 Napcat 部署需要的参数。
type NapcatDeployOptions struct {
	ScriptURL         string
	RawScriptCommand  string
	WebUIPort         int
	QQDebURL          string
	InstallTimeoutSec int
}

// NewNapcatDeployer 创建 Napcat 部署器。
func NewNapcatDeployer(dataDir string) *NapcatDeployer {
	return &NapcatDeployer{baseDir: filepath.Join(dataDir, "Napcat")}
}

// TargetDir 计算指定服务注册名对应的安装目录。
func (d *NapcatDeployer) TargetDir(registryName string) string {
	return filepath.Join(d.baseDir, registryName)
}

// DeployFromScript 从脚本 URL 部署 Napcat，并返回托管执行入口和最终使用 URL。
func (d *NapcatDeployer) DeployFromScript(registryName string, opts NapcatDeployOptions, logf func(string, ...any)) (string, string, error) {
	targetDir := d.TargetDir(registryName)
	if _, err := os.Stat(targetDir); err == nil {
		return "", "", fmt.Errorf("target directory already exists: %s", targetDir)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", "", err
	}
	cleanupOnFail := true
	defer func() {
		if cleanupOnFail {
			_ = os.RemoveAll(targetDir)
		}
	}()

	scriptURL := strings.TrimSpace(opts.ScriptURL)
	if scriptURL == "" {
		parsed, err := extractFirstHTTPURL(opts.RawScriptCommand)
		if err != nil {
			return "", "", err
		}
		scriptURL = parsed
	}
	if err := validateScriptURL(scriptURL); err != nil {
		return "", "", err
	}

	if logf != nil {
		logf("deploy: target dir %s", targetDir)
		logf("deploy: downloading install script %s", scriptURL)
	}
	scriptPath := filepath.Join(targetDir, napcatScriptFilename)
	if err := downloadTextFile(scriptURL, scriptPath); err != nil {
		return "", "", err
	}
	if err := os.Chmod(scriptPath, 0o755); err != nil {
		return "", "", err
	}

	if logf != nil {
		logf("deploy: running install script")
	}
	if err := runNapcatInstallScript(targetDir, scriptPath, opts.QQDebURL, opts.InstallTimeoutSec, logf); err != nil {
		return "", "", err
	}

	if opts.WebUIPort <= 0 {
		opts.WebUIPort = DefaultNapcatWebUIPort
	}
	if err := UpdateNapcatWebUIPort(targetDir, opts.WebUIPort); err != nil {
		if logf != nil {
			logf("deploy: warning unable to set webui port: %v", err)
		}
	}

	execPath, err := EnsureNapcatRunner(targetDir, logf)
	if err != nil {
		return "", "", err
	}
	cleanupOnFail = false
	return execPath, scriptURL, nil
}

// EnsureNapcatRunner 启动前刷新 Napcat 启动脚本，保证运行入口可控。
func EnsureNapcatRunner(installDir string, logf func(string, ...any)) (string, error) {
	base := strings.TrimSpace(installDir)
	if base == "" {
		return "", errors.New("napcat install dir is empty")
	}
	if _, err := os.Stat(base); err != nil {
		return "", err
	}
	if logf != nil {
		logf("runtime: refreshing napcat launcher")
	}
	return writeNapcatRunner(base)
}

// ReadNapcatWebUIPort 读取 webui.json 的当前端口。
func ReadNapcatWebUIPort(installDir string) (int, error) {
	cfgPath, err := ensureNapcatWebUIConfig(installDir)
	if err != nil {
		return 0, err
	}
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return 0, err
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return 0, err
	}
	port := napcatToInt(root["port"])
	if port < 1 || port > 65535 {
		return 0, errors.New("invalid webui port in config")
	}
	return port, nil
}

// UpdateNapcatWebUIPort 修改 webui.json 中的 WebUI 端口配置。
func UpdateNapcatWebUIPort(installDir string, port int) error {
	if port < 1 || port > 65535 {
		return errors.New("webui port must be in 1-65535")
	}
	cfgPath, err := ensureNapcatWebUIConfig(installDir)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return err
	}
	root["port"] = port
	if host, _ := root["host"].(string); strings.TrimSpace(host) == "" {
		root["host"] = "::"
	}
	if token, _ := root["token"].(string); strings.TrimSpace(token) == "" {
		root["token"] = "napcat"
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, out, 0o644)
}

func extractFirstHTTPURL(raw string) (string, error) {
	match := napcatURLRegex.FindString(strings.TrimSpace(raw))
	if match == "" {
		return "", errors.New("script url is required")
	}
	return match, nil
}

func validateScriptURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return errors.New("invalid script url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("script url must start with http or https")
	}
	if strings.TrimSpace(u.Host) == "" {
		return errors.New("script url host is empty")
	}
	if len(raw) > 2048 {
		return errors.New("script url is too long")
	}
	return nil
}

func downloadTextFile(rawURL string, targetPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), napcatDownloadScriptTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "LDM-NapcatInstaller/1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download script failed: %s", resp.Status)
	}
	f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 2*1024*1024)); err != nil {
		return err
	}
	return nil
}

func runNapcatInstallScript(targetDir string, scriptPath string, qqDebURL string, timeoutSec int, logf func(string, ...any)) error {
	if runtime.GOOS != "linux" {
		return errors.New("napcat deploy is only supported on linux")
	}
	if _, err := exec.LookPath("apt-get"); err != nil {
		return errors.New("napcat deploy requires apt-get")
	}
	timeout := napcatInstallScriptTimeout
	if timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", scriptPath)
	cmd.Dir = targetDir
	env := os.Environ()
	if strings.TrimSpace(qqDebURL) != "" {
		env = append(env, "LDM_NAPCAT_QQ_DEB_URL="+strings.TrimSpace(qqDebURL))
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if logf != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = "(no output)"
		}
		logf("deploy: install output %s", msg)
	}
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("napcat install script timeout after %s", timeout)
	}
	if err != nil {
		return fmt.Errorf("napcat install script failed: %w", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, napcatLauncherSOName)); err != nil {
		return errors.New("libnapcat_launcher.so not found after installation")
	}
	return nil
}

func writeNapcatRunner(installDir string) (string, error) {
	runner := filepath.Join(installDir, napcatRunnerFilename)
	content := fmt.Sprintf(`#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

if [ ! -f "$SCRIPT_DIR/%s" ]; then
  echo "napcat launcher not found: $SCRIPT_DIR/%s"
  exit 1
fi

if [ ! -x "%s" ]; then
  echo "qq executable not found: %s"
  exit 1
fi

if ! command -v Xvfb >/dev/null 2>&1; then
  echo "Xvfb not found. Please install xvfb first."
  exit 1
fi

if ! pgrep -f "Xvfb :1" >/dev/null 2>&1; then
  Xvfb :1 -screen 0 1x1x8 +extension GLX +render >/dev/null 2>&1 &
  sleep 1
fi

export DISPLAY=:1
if [ -n "${WEBSEAL_SERVICE_PORT:-}" ]; then
  export NAPCAT_WEBUI_PREFERRED_PORT="$WEBSEAL_SERVICE_PORT"
fi
exec env LD_PRELOAD="$SCRIPT_DIR/%s" %s --no-sandbox "$@"
`, napcatLauncherSOName, napcatLauncherSOName, napcatQQBinary, napcatQQBinary, napcatLauncherSOName, napcatQQBinary)
	if err := os.WriteFile(runner, []byte(content), 0o755); err != nil {
		return "", err
	}
	return runner, nil
}

func ensureNapcatWebUIConfig(installDir string) (string, error) {
	base := strings.TrimSpace(installDir)
	if base == "" {
		return "", errors.New("napcat install dir is empty")
	}
	candidates := []string{
		filepath.Join(base, "napcat", "config", "webui.json"),
		filepath.Join(base, "config", "webui.json"),
		filepath.Join(base, "webui.json"),
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
		if strings.EqualFold(d.Name(), "webui.json") && strings.Contains(filepath.ToSlash(pathNow), "/config/") {
			found = pathNow
			return errFound
		}
		return nil
	})
	if err != nil && !errors.Is(err, errFound) {
		return "", err
	}
	if found != "" {
		return found, nil
	}

	cfgPath := filepath.Join(base, "napcat", "config", "webui.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return "", err
	}
	defaultConfig := map[string]any{
		"host":         "::",
		"port":         DefaultNapcatWebUIPort,
		"token":        "napcat",
		"loginRate":    10,
		"disableWebUI": false,
	}
	raw, _ := json.MarshalIndent(defaultConfig, "", "  ")
	if err := os.WriteFile(cfgPath, raw, 0o644); err != nil {
		return "", err
	}
	return cfgPath, nil
}

func napcatToInt(v any) int {
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
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(n))
		return i
	default:
		return 0
	}
}
