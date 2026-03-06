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
	"sort"
	"strings"
	"time"
)

const (
	LagrangeBaseURL           = "https://misc.cn.xuetao.host/pd/AirisuTek/Sealdice%E7%9B%B8%E5%85%B3/Lagrange%E4%B8%80%E9%94%AE%E5%8C%85/Lagrange%E4%B8%8B%E8%BD%BD"
	LagrangeOldSubdirEncoded  = "%E5%A5%BD%E5%8F%8B%E6%B2%A1%E9%97%AE%E9%A2%98%E7%9A%84%E7%89%88%E6%9C%ACFeb_13_ddda0a6"
	DefaultLagrangeSignServer = "https://cf-sign.xuetao.host/44343"
)

// LagrangeDeployer 负责 Lagrange 安装目录规划与自动部署流程。
type LagrangeDeployer struct {
	baseDir string
}

// LagrangeDeployOptions 描述部署阶段需要的版本、签名与端口参数。
type LagrangeDeployOptions struct {
	Version         string
	SignServerURL   string
	DownloadPrefix  string
	DownloadURL     string
	EnableForwardWS bool
	ForwardWSPort   int
	ForwardWSHost   string
	EnableReverseWS bool
	ReverseWSPort   int
	ReverseWSHost   string
	ReverseWSSuffix string
	EnableHTTP      bool
	HTTPPort        int
	HTTPHost        string
}

// LagrangeConfigState 表示 appsettings.json 中与 OneBot 相关的可编辑配置视图。
type LagrangeConfigState struct {
	SignServerURL   string `json:"sign_server_url"`
	EnableForwardWS bool   `json:"enable_forward_ws"`
	ForwardWSPort   int    `json:"forward_ws_port"`
	EnableReverseWS bool   `json:"enable_reverse_ws"`
	ReverseWSPort   int    `json:"reverse_ws_port"`
	EnableHTTP      bool   `json:"enable_http"`
	HTTPPort        int    `json:"http_port"`
}

// NewLagrangeDeployer 创建 Lagrange 部署器。
func NewLagrangeDeployer(dataDir string) *LagrangeDeployer {
	return &LagrangeDeployer{
		baseDir: filepath.Join(dataDir, "Lagrange"),
	}
}

// TargetDir 计算指定服务注册名对应的安装目录。
func (d *LagrangeDeployer) TargetDir(registryName string) string {
	return filepath.Join(d.baseDir, registryName)
}

// DeployFromAuto 下载并解压 Lagrange 发布包，写入配置后返回可执行路径。
func (d *LagrangeDeployer) DeployFromAuto(registryName string, opts LagrangeDeployOptions, logf func(string, ...any)) (string, error) {
	targetDir := d.TargetDir(registryName)
	if _, err := os.Stat(targetDir); err == nil {
		return "", fmt.Errorf("target directory already exists: %s", targetDir)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}
	if logf != nil {
		logf("deploy: target dir %s", targetDir)
	}

	url := strings.TrimSpace(opts.DownloadURL)
	if url == "" {
		zipName, err := lagrangeArchiveName()
		if err != nil {
			return "", err
		}
		url = lagrangeDownloadURL(zipName, opts.Version, opts.DownloadPrefix)
	}
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
	if err := extractZip(resp.Body, targetDir); err != nil {
		return "", err
	}
	if logf != nil {
		logf("deploy: writing appsettings.json")
	}
	if err := writeLagrangeConfig(targetDir, opts); err != nil {
		return "", err
	}

	execPath, err := findLagrangeExecutable(targetDir)
	if err != nil {
		return "", err
	}
	if logf != nil {
		logf("deploy: executable %s", execPath)
	}
	return execPath, nil
}

// DeployFromReader 从上传流解压 Lagrange 包并写入配置。
func (d *LagrangeDeployer) DeployFromReader(registryName string, opts LagrangeDeployOptions, reader io.Reader, logf func(string, ...any)) (string, error) {
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
	if err := extractZip(reader, targetDir); err != nil {
		return "", err
	}
	if logf != nil {
		logf("deploy: writing appsettings.json")
	}
	if err := writeLagrangeConfig(targetDir, opts); err != nil {
		return "", err
	}
	execPath, err := findLagrangeExecutable(targetDir)
	if err != nil {
		return "", err
	}
	if logf != nil {
		logf("deploy: executable %s", execPath)
	}
	return execPath, nil
}

// lagrangeArchiveName 根据当前 CPU 架构选择对应的 Lagrange 压缩包名。
func lagrangeArchiveName() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "Linux_x64_Lagrange.zip", nil
	case "arm64":
		return "Linux_arm64_Lagrange.zip", nil
	case "arm", "armv6l", "armv7l":
		return "Linux_arm_Lagrange.zip", nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}
}

// lagrangeDownloadURL 按版本策略组装最终下载地址。
func lagrangeDownloadURL(zipName string, version string, prefix string) string {
	base := strings.TrimSpace(prefix)
	if base == "" {
		base = LagrangeBaseURL
	}
	base = strings.TrimRight(base, "/")

	if strings.EqualFold(strings.TrimSpace(version), "old") {
		return base + "/" + LagrangeOldSubdirEncoded + "/" + zipName
	}
	return base + "/" + zipName
}

// writeLagrangeConfig 生成并写入 appsettings.json，包含签名地址与实现列表。
func writeLagrangeConfig(targetDir string, opts LagrangeDeployOptions) error {
	signURL := strings.TrimSpace(opts.SignServerURL)
	if signURL == "" {
		signURL = DefaultLagrangeSignServer
	}

	impls := make([]map[string]any, 0, 3)
	if opts.EnableForwardWS {
		if opts.ForwardWSPort < 1 || opts.ForwardWSPort > 65535 {
			return errors.New("forward ws port must be in 1-65535")
		}
		host := strings.TrimSpace(opts.ForwardWSHost)
		if host == "" {
			host = "127.0.0.1"
		}
		impls = append(impls, map[string]any{
			"Type":              "ForwardWebSocket",
			"Host":              host,
			"Port":              opts.ForwardWSPort,
			"HeartBeatInterval": 5000,
			"HeartBeatEnable":   true,
			"AccessToken":       "",
		})
	}
	if opts.EnableReverseWS {
		if opts.ReverseWSPort < 1 || opts.ReverseWSPort > 65535 {
			return errors.New("reverse ws port must be in 1-65535")
		}
		host := strings.TrimSpace(opts.ReverseWSHost)
		if host == "" {
			host = "127.0.0.1"
		}
		suffix := strings.TrimSpace(opts.ReverseWSSuffix)
		if suffix == "" {
			suffix = "/ws"
		}
		impls = append(impls, map[string]any{
			"Type":              "ReverseWebSocket",
			"Host":              host,
			"Port":              opts.ReverseWSPort,
			"Suffix":            suffix,
			"ReconnectInterval": 5000,
			"HeartBeatInterval": 5000,
			"AccessToken":       "",
		})
	}
	if opts.EnableHTTP {
		if opts.HTTPPort < 1 || opts.HTTPPort > 65535 {
			return errors.New("http port must be in 1-65535")
		}
		host := strings.TrimSpace(opts.HTTPHost)
		if host == "" {
			host = "127.0.0.1"
		}
		impls = append(impls, map[string]any{
			"Type":        "Http",
			"Host":        host,
			"Port":        opts.HTTPPort,
			"AccessToken": "",
		})
	}
	if len(impls) == 0 {
		return errors.New("at least one implementation must be enabled")
	}

	root := map[string]any{
		"$schema": "https://raw.githubusercontent.com/LagrangeDev/Lagrange.Core/master/Lagrange.OneBot/Resources/appsettings_schema.json",
		"Logging": map[string]any{
			"LogLevel": map[string]any{
				"Default": "Information",
			},
		},
		"SignServerUrl":      signURL,
		"SignProxyUrl":       "",
		"MusicSignServerUrl": "https://ss.xingzhige.com/music_card/card",
		"Account": map[string]any{
			"Uin":              0,
			"Password":         "",
			"Protocol":         "Linux",
			"AutoReconnect":    true,
			"GetOptimumServer": true,
		},
		"Message": map[string]any{
			"IgnoreSelf": true,
			"StringPost": false,
		},
		"QrCode": map[string]any{
			"ConsoleCompatibilityMode": false,
		},
		"Implementations": impls,
	}
	raw, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(targetDir, "appsettings.json"), raw, 0o644)
}

// extractZip 安全解压 zip 到目标目录，包含路径穿越防护。
func extractZip(src io.Reader, dst string) error {
	tempFile, err := os.CreateTemp("", "lagrange-*.zip")
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

// findLagrangeExecutable 在解压目录中定位可执行文件并设置执行权限。
func findLagrangeExecutable(targetDir string) (string, error) {
	candidates := make([]string, 0, 4)
	preferred := []string{
		filepath.Join(targetDir, "Lagrange.OneBot"),
		filepath.Join(targetDir, "Lagrange"),
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
		if strings.HasPrefix(name, "lagrange") && !strings.HasSuffix(name, ".dll") {
			candidates = append(candidates, pathNow)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(candidates)
	if len(candidates) == 0 {
		return "", errors.New("lagrange executable not found after extraction")
	}
	_ = os.Chmod(candidates[0], 0o755)
	return candidates[0], nil
}

// UpdateLagrangeForwardWSPort 修改 ForwardWebSocket 端口并回写配置文件。
func UpdateLagrangeForwardWSPort(installDir string, port int) error {
	if port < 1 || port > 65535 {
		return errors.New("forward ws port must be in 1-65535")
	}
	configPath := filepath.Join(strings.TrimSpace(installDir), "appsettings.json")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return err
	}

	impls, _ := root["Implementations"].([]any)
	if len(impls) == 0 {
		impls = []any{
			map[string]any{
				"Type":              "ForwardWebSocket",
				"Host":              "127.0.0.1",
				"Port":              float64(port),
				"HeartBeatInterval": float64(5000),
				"HeartBeatEnable":   true,
				"AccessToken":       "",
			},
		}
		root["Implementations"] = impls
	} else {
		updated := false
		for _, item := range impls {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			tp, _ := m["Type"].(string)
			if tp == "ForwardWebSocket" {
				m["Host"] = "127.0.0.1"
				m["Port"] = float64(port)
				updated = true
				break
			}
		}
		if !updated {
			impls = append(impls, map[string]any{
				"Type":              "ForwardWebSocket",
				"Host":              "127.0.0.1",
				"Port":              float64(port),
				"HeartBeatInterval": float64(5000),
				"HeartBeatEnable":   true,
				"AccessToken":       "",
			})
			root["Implementations"] = impls
		}
	}

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, out, 0o644)
}

// ReadLagrangeConfig 从 appsettings.json 读取当前可视化配置状态。
func ReadLagrangeConfig(installDir string) (LagrangeConfigState, error) {
	configPath := filepath.Join(strings.TrimSpace(installDir), "appsettings.json")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return LagrangeConfigState{}, err
	}

	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return LagrangeConfigState{}, err
	}

	state := LagrangeConfigState{
		SignServerURL: DefaultLagrangeSignServer,
	}
	if v, ok := root["SignServerUrl"].(string); ok && strings.TrimSpace(v) != "" {
		state.SignServerURL = v
	}

	impls, _ := root["Implementations"].([]any)
	for _, item := range impls {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		tp, _ := m["Type"].(string)
		switch tp {
		case "ForwardWebSocket":
			state.EnableForwardWS = true
			state.ForwardWSPort = toInt(m["Port"])
		case "ReverseWebSocket":
			state.EnableReverseWS = true
			state.ReverseWSPort = toInt(m["Port"])
		case "Http":
			state.EnableHTTP = true
			state.HTTPPort = toInt(m["Port"])
		}
	}
	return state, nil
}

// UpdateLagrangeConfig 按给定状态重写 Implementations 节点。
func UpdateLagrangeConfig(installDir string, state LagrangeConfigState) error {
	if !state.EnableForwardWS {
		return errors.New("forward ws must be enabled")
	}
	if state.ForwardWSPort < 1 || state.ForwardWSPort > 65535 {
		return errors.New("forward ws port must be in 1-65535")
	}
	if state.EnableReverseWS && (state.ReverseWSPort < 1 || state.ReverseWSPort > 65535) {
		return errors.New("reverse ws port must be in 1-65535")
	}
	if state.EnableHTTP && (state.HTTPPort < 1 || state.HTTPPort > 65535) {
		return errors.New("http port must be in 1-65535")
	}
	if strings.TrimSpace(state.SignServerURL) == "" {
		state.SignServerURL = DefaultLagrangeSignServer
	}

	configPath := filepath.Join(strings.TrimSpace(installDir), "appsettings.json")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return err
	}

	root["SignServerUrl"] = strings.TrimSpace(state.SignServerURL)
	impls := make([]any, 0, 3)
	impls = append(impls, map[string]any{
		"Type":              "ForwardWebSocket",
		"Host":              "127.0.0.1",
		"Port":              state.ForwardWSPort,
		"HeartBeatInterval": 5000,
		"HeartBeatEnable":   true,
		"AccessToken":       "",
	})
	if state.EnableReverseWS {
		impls = append(impls, map[string]any{
			"Type":              "ReverseWebSocket",
			"Host":              "127.0.0.1",
			"Port":              state.ReverseWSPort,
			"Suffix":            "/ws",
			"ReconnectInterval": 5000,
			"HeartBeatInterval": 5000,
			"AccessToken":       "",
		})
	}
	if state.EnableHTTP {
		impls = append(impls, map[string]any{
			"Type":        "Http",
			"Host":        "127.0.0.1",
			"Port":        state.HTTPPort,
			"AccessToken": "",
		})
	}
	root["Implementations"] = impls

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, out, 0o644)
}

// toInt 将 JSON 反序列化后的 number 值安全转换为 int。
func toInt(v any) int {
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
