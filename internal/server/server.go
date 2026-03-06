package server

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"web-sealdice/internal/auth"
	"web-sealdice/internal/bootstrap"
	"web-sealdice/internal/config"
	"web-sealdice/internal/deploy"
	"web-sealdice/internal/services"
	"web-sealdice/internal/update"
	buildver "web-sealdice/internal/version"
)

//go:embed static/*
var staticFS embed.FS

// Server 聚合配置、认证、服务存储/运行时与部署器，是后端核心入口。
type Server struct {
	cfg              config.Config
	configPath       string
	passwordStore    *auth.PasswordStore
	sessions         *auth.SessionManager
	serviceStore     *services.Store
	serviceMgr       *services.Manager
	sealdiceDeployer *deploy.SealdiceDeployer
	lagrangeDeployer *deploy.LagrangeDeployer
	llbotDeployer    *deploy.LLBotDeployer
	loginProtector   *loginProtector
	handler          http.Handler
	metricsMu        sync.Mutex
	cpuSample        cpuSampleSnapshot
}

// jsonMessage 是统一的简单消息响应结构。
type jsonMessage struct {
	Message string `json:"message"`
}

// deployLogger 用于记录部署阶段日志，并供前端实时拉取。
type deployLogger struct {
	Name string
	file *os.File
}

// sealdiceDeployMeta 记录 Sealdice 部署来源、下载地址与端口。
type sealdiceDeployMeta struct {
	Source string `json:"source"`
	URL    string `json:"url"`
	Auto   bool   `json:"auto"`
	Port   int    `json:"port"`
}

// lagrangeDeployMeta 记录 Lagrange 部署版本、签名地址与端口配置。
type lagrangeDeployMeta struct {
	Source          string `json:"source"`
	Version         string `json:"version"`
	SignServerURL   string `json:"sign_server_url"`
	DownloadPrefix  string `json:"download_prefix"`
	DownloadURL     string `json:"download_url"`
	EnableForwardWS bool   `json:"enable_forward_ws"`
	ForwardWSPort   int    `json:"forward_ws_port"`
	EnableReverseWS bool   `json:"enable_reverse_ws"`
	ReverseWSPort   int    `json:"reverse_ws_port"`
	EnableHTTP      bool   `json:"enable_http"`
	HTTPPort        int    `json:"http_port"`
}

// llbotDeployMeta 记录 LuckyLilliaBot 部署版本与 WebUI 端口。
type llbotDeployMeta struct {
	Source  string `json:"source"`
	Version string `json:"version"`
	Port    int    `json:"port"`
}

// sealdiceInstaller 抽象 Sealdice 的安装动作（自动下载/上传包）。
type sealdiceInstaller func(registryName string) (string, error)

// llbotInstaller 抽象 LuckyLilliaBot 的安装动作（自动下载/上传包）。
type llbotInstaller func(registryName string) (string, error)

// deployLogFunc 抽象部署日志输出回调。
type deployLogFunc func(string, ...any)

// Printf 将部署日志写入文件并附带 RFC3339 时间戳。
func (l *deployLogger) Printf(format string, args ...any) {
	if l == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(l.file, "[%s] %s\n", time.Now().Format(time.RFC3339), msg)
	_ = l.file.Sync()
}

// Close 关闭部署日志文件句柄。
func (l *deployLogger) Close() {
	if l == nil || l.file == nil {
		return
	}
	_ = l.file.Close()
}

// New 构建 Server 实例并初始化路由、存储、运行时与自动拉起。
func New(cfg config.Config, configPath string) (*Server, error) {
	store := services.NewStore(cfg.DataDir)
	s := &Server{
		cfg:              cfg,
		configPath:       configPath,
		passwordStore:    auth.NewPasswordStore(cfg.PasswordHashPath()),
		sessions:         auth.NewSessionManager(cfg.SessionCookieName, cfg.BasePath, cfg.TrustProxyHeaders, time.Duration(cfg.SessionTTL())*time.Hour, cfg.SessionSecret),
		serviceStore:     store,
		serviceMgr:       services.NewManager(store, cfg.DataDir, cfg.LogRetentionCount, cfg.LogRetentionDays, cfg.LogMaxMB),
		sealdiceDeployer: deploy.NewSealdiceDeployer(cfg.DataDir),
		lagrangeDeployer: deploy.NewLagrangeDeployer(cfg.DataDir),
		llbotDeployer:    deploy.NewLLBotDeployer(cfg.DataDir),
		loginProtector:   newLoginProtector(cfg),
	}

	h, err := s.buildHandler()
	if err != nil {
		return nil, err
	}
	s.handler = h
	_ = s.serviceMgr.StartAutoServices()
	return s, nil
}

// Handler 返回当前 HTTP 根处理器。
func (s *Server) Handler() http.Handler {
	return s.handler
}

// Shutdown 先优雅停止托管服务，再交给外层关闭 HTTP Server。
func (s *Server) Shutdown() {
	s.serviceMgr.StopAllGraceful(10 * time.Second)
}

// applyRuntimeConfig 将新配置同步到运行期对象，必要时重建 DataDir 相关依赖。
func (s *Server) applyRuntimeConfig(next config.Config) {
	prev := s.cfg
	s.cfg = next

	s.passwordStore = auth.NewPasswordStore(next.PasswordHashPath())
	s.sessions = auth.NewSessionManager(
		next.SessionCookieName,
		next.BasePath,
		next.TrustProxyHeaders,
		time.Duration(next.SessionTTL())*time.Hour,
		next.SessionSecret,
	)
	s.loginProtector = newLoginProtector(next)

	if strings.TrimSpace(prev.DataDir) == strings.TrimSpace(next.DataDir) {
		return
	}

	store := services.NewStore(next.DataDir)
	s.serviceStore = store
	s.serviceMgr = services.NewManager(store, next.DataDir, next.LogRetentionCount, next.LogRetentionDays, next.LogMaxMB)
	s.sealdiceDeployer = deploy.NewSealdiceDeployer(next.DataDir)
	s.lagrangeDeployer = deploy.NewLagrangeDeployer(next.DataDir)
	s.llbotDeployer = deploy.NewLLBotDeployer(next.DataDir)
}

// buildHandler 组装 API、服务代理、静态资源与 SPA 回退路由。
func (s *Server) buildHandler() (http.Handler, error) {
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}
	indexHTML, err := fs.ReadFile(staticSub, "index.html")
	if err != nil {
		return nil, err
	}
	staticHandler := http.FileServer(http.FS(staticSub))

	mux := http.NewServeMux()
	mux.HandleFunc("/api/bootstrap/status", s.handleBootstrapStatus)
	mux.HandleFunc("/api/update/check", s.handleUpdateCheck)
	mux.HandleFunc("/api/update/apply", s.handleUpdateApply)
	mux.HandleFunc("/api/panel/settings", s.handlePanelSettings)
	mux.HandleFunc("/api/panel/logs/clear", s.handlePanelLogsClear)
	mux.HandleFunc("/api/auth/init", s.handleInitPassword)
	mux.HandleFunc("/api/auth/login", s.handleLogin)
	mux.HandleFunc("/api/auth/logout", s.handleLogout)
	mux.HandleFunc("/api/me", s.handleMe)
	mux.HandleFunc("/api/metrics/overview", s.handleMetricsOverview)
	mux.HandleFunc("/api/services", s.handleServices)
	mux.HandleFunc("/api/services/", s.handleServiceByID)
	mux.HandleFunc("/api/ports/check", s.handlePortCheck)
	mux.HandleFunc("/api/deploy/sealdice/auto", s.handleDeploySealdiceAuto)
	mux.HandleFunc("/api/deploy/sealdice/upload", s.handleDeploySealdiceUpload)
	mux.HandleFunc("/api/deploy/lagrange/auto", s.handleDeployLagrangeAuto)
	mux.HandleFunc("/api/deploy/lagrange/upload", s.handleDeployLagrangeUpload)
	mux.HandleFunc("/api/deploy/lagrange/versions", s.handleLagrangeVersions)
	mux.HandleFunc("/api/deploy/llbot/auto", s.handleDeployLLBotAuto)
	mux.HandleFunc("/api/deploy/llbot/upload", s.handleDeployLLBotUpload)
	mux.HandleFunc("/api/deploy/logs/", s.handleDeployLogs)
	mux.HandleFunc("/api/deploy/lagrange/signinfo", s.handleLagrangeSignInfo)
	mux.HandleFunc("/api/deploy/lagrange/sign-probe", s.handleLagrangeSignProbe)
	mux.HandleFunc("/api/llbot/qq/status", s.handleLLBotQQStatus)
	mux.HandleFunc("/api/llbot/qq/install", s.handleLLBotQQInstall)
	mux.HandleFunc("/service/", s.handleServiceProxy)
	mux.Handle("/", s.wrapServiceEntry(staticHandler, indexHTML))
	securedMux := s.withSecurityHeaders(mux)

	if s.cfg.BasePath == "/" {
		return securedMux, nil
	}

	baseMux := http.NewServeMux()
	baseMux.Handle(s.cfg.BasePath+"/", http.StripPrefix(s.cfg.BasePath, securedMux))
	baseMux.HandleFunc(s.cfg.BasePath, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, s.cfg.BasePath+"/", http.StatusTemporaryRedirect)
	})
	return baseMux, nil
}

// newDeployLogger 为指定服务创建独立部署日志文件。
func (s *Server) newDeployLogger(serviceType string, registry string) *deployLogger {
	name := strings.TrimSpace(registry)
	if name == "" {
		name = "unknown"
	}
	safe := services.SanitizeName(name)
	if safe == "" {
		safe = "unknown"
	}
	dir := filepath.Join(s.cfg.DataDir, "logs", "deploy")
	_ = os.MkdirAll(dir, 0o755)
	stamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.log", safe, stamp)
	full := filepath.Join(dir, filename)
	f, err := os.OpenFile(full, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("deploy: unable to open log file: %v", err)
		nullFile, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0o644)
		return &deployLogger{Name: filename, file: nullFile}
	}
	logger := &deployLogger{Name: filename, file: f}
	logger.Printf("deploy: start type=%s registry=%s", serviceType, registry)
	return logger
}

// wrapServiceEntry 处理 SPA 深链与静态资源分发的冲突。
func (s *Server) wrapServiceEntry(staticHandler http.Handler, indexHTML []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/service/") {
			staticHandler.ServeHTTP(w, r)
			return
		}
		clean := strings.Trim(strings.TrimSpace(path), "/")
		if clean == "" {
			s.serveAppIndex(w, indexHTML)
			return
		}
		parts := strings.Split(clean, "/")
		if len(parts) >= 1 && (parts[0] == "dashboard" || parts[0] == "settings") {
			s.serveAppIndex(w, indexHTML)
			return
		}
		if parts[0] == "services" {
			if len(parts) == 1 {
				s.serveAppIndex(w, indexHTML)
				return
			}
			serviceID := parts[1]
			if strings.Contains(serviceID, ".") {
				staticHandler.ServeHTTP(w, r)
				return
			}
			if _, err := s.serviceStore.Get(serviceID); err == nil {
				if len(parts) == 2 || (len(parts) == 3 && isKnownServiceTab(parts[2])) {
					s.serveAppIndex(w, indexHTML)
					return
				}
			} else {
				target := "/services"
				if s.cfg.BasePath != "/" {
					target = s.cfg.BasePath + target
				}
				http.Redirect(w, r, target, http.StatusTemporaryRedirect)
				return
			}
			staticHandler.ServeHTTP(w, r)
			return
		}
		if len(parts) != 1 {
			staticHandler.ServeHTTP(w, r)
			return
		}
		first := parts[0]
		if strings.Contains(first, ".") {
			staticHandler.ServeHTTP(w, r)
			return
		}
		if _, err := s.serviceStore.Get(first); err != nil {
			staticHandler.ServeHTTP(w, r)
			return
		}
		target := "/services/" + url.PathEscape(first) + "/overview"
		if s.cfg.BasePath != "/" {
			target = s.cfg.BasePath + target
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	})
}

// serveAppIndex 输出前端入口 HTML。
func (s *Server) serveAppIndex(w http.ResponseWriter, indexHTML []byte) {
	output := indexHTML
	if s.cfg.BasePath != "/" {
		baseHref := s.cfg.BasePath
		if !strings.HasSuffix(baseHref, "/") {
			baseHref += "/"
		}
		output = bytes.Replace(
			indexHTML,
			[]byte(`<base href="/">`),
			[]byte(`<base href="`+baseHref+`">`),
			1,
		)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(output)
}

// isKnownServiceTab 判断 URL 片段是否属于服务子页面标签。
func isKnownServiceTab(tab string) bool {
	switch tab {
	case "overview", "logs", "files":
		return true
	default:
		return false
	}
}

// lagrangeConfigEqual 比较两份 Lagrange 可视化配置是否等价。
func lagrangeConfigEqual(a, b deploy.LagrangeConfigState) bool {
	return a.EnableForwardWS == b.EnableForwardWS &&
		a.ForwardWSPort == b.ForwardWSPort &&
		a.EnableReverseWS == b.EnableReverseWS &&
		a.ReverseWSPort == b.ReverseWSPort &&
		a.EnableHTTP == b.EnableHTTP &&
		a.HTTPPort == b.HTTPPort &&
		strings.TrimSpace(a.SignServerURL) == strings.TrimSpace(b.SignServerURL)
}

// findServiceUsingPort 在已登记服务中查找端口占用者。
func (s *Server) findServiceUsingPort(port int, excludeID string) (*services.Service, error) {
	items, err := s.serviceStore.List()
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == excludeID {
			continue
		}
		if items[i].Port == port {
			cp := items[i]
			return &cp, nil
		}
	}
	return nil, nil
}

// checkPortAvailable 先查托管服务，再查系统监听端口，判断端口是否可用。
func (s *Server) checkPortAvailable(port int, excludeID string) (bool, string, *services.Service, error) {
	if port < 1 || port > 65535 {
		return false, "invalid port", nil, nil
	}
	owner, err := s.findServiceUsingPort(port, excludeID)
	if err != nil {
		return false, "", nil, err
	}
	if owner != nil {
		name := strings.TrimSpace(owner.DisplayName)
		if name == "" {
			name = owner.ID
		}
		return false, fmt.Sprintf("port %d is used by %s", port, name), owner, nil
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false, fmt.Sprintf("port %d is occupied", port), nil, nil
	}
	_ = ln.Close()
	return true, "", nil, nil
}

// handlePortCheck 提供前端端口实时校验接口。
func (s *Server) handlePortCheck(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	// reqBody 鐎规矮绠熺拠銉δ侀崸妞惧▏閻劎娈戦弫鐗堝祦缂佹挻鐎妴?
	type reqBody struct {
		Port      int    `json:"port"`
		ServiceID string `json:"service_id"`
	}
	var body reqBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid json body"})
		return
	}

	ok, message, owner, err := s.checkPortAvailable(body.Port, strings.TrimSpace(body.ServiceID))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, jsonMessage{Message: err.Error()})
		return
	}
	resp := map[string]any{
		"valid":     body.Port >= 1 && body.Port <= 65535,
		"available": ok,
		"message":   message,
	}
	if owner != nil {
		resp["service_id"] = owner.ID
		resp["display_name"] = owner.DisplayName
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleBootstrapStatus 返回初始化状态（是否已设置密码等）。
func (s *Server) handleBootstrapStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"needs_password_setup": !s.passwordStore.Exists(),
		"version":              buildver.Display(),
		"version_raw":          buildver.Version,
		"commit":               buildver.Commit,
		"build_time":           buildver.BuildTime,
		"channel":              buildver.Channel,
		"dirty":                buildver.Dirty,
	})
}

// handleUpdateCheck 使用内嵌更新源配置拉取默认 manifest，并比较本地版本与远端版本。
func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}

	result, err := update.Check(r.Context(), buildver.Version, runtime.GOARCH)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, jsonMessage{Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleUpdateApply 下载新版可执行并替换当前二进制文件（Linux）。
// 说明：替换完成后当前进程不会自动退出，需由用户手动重启服务以加载新版本。
func (s *Server) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	if runtime.GOOS != "linux" {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "direct update is only supported on linux"})
		return
	}

	result, err := update.Check(r.Context(), buildver.Version, runtime.GOARCH)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, jsonMessage{Message: err.Error()})
		return
	}
	if !result.HasUpdate {
		writeJSON(w, http.StatusOK, map[string]any{
			"updated": false,
			"message": "already up to date",
		})
		return
	}
	downloadURL := strings.TrimSpace(result.DownloadURL)
	if downloadURL == "" {
		writeJSON(w, http.StatusBadGateway, jsonMessage{Message: "update download url is empty"})
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, jsonMessage{Message: err.Error()})
		return
	}
	if err := applyBinaryUpdate(r.Context(), exePath, downloadURL); err != nil {
		writeJSON(w, http.StatusBadGateway, jsonMessage{Message: err.Error()})
		return
	}
	log.Printf("update: applied new binary from %s version=%s", downloadURL, result.RemoteVersion)
	writeJSON(w, http.StatusOK, map[string]any{
		"updated":          true,
		"message":          "update applied, restart panel service to take effect",
		"remote_version":   result.RemoteVersion,
		"restart_required": true,
	})
}

// handleInitPassword 首次初始化密码。
func (s *Server) handleInitPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	if s.passwordStore.Exists() {
		writeJSON(w, http.StatusConflict, jsonMessage{Message: "password already initialized"})
		return
	}

	password, err := readPasswordFromBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}

	if err := s.passwordStore.Set(password); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, jsonMessage{Message: "password initialized"})
}

// handleLogin 校验密码并签发会话 Cookie。
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	if !s.passwordStore.Exists() {
		writeJSON(w, http.StatusPreconditionFailed, jsonMessage{Message: "password is not initialized"})
		return
	}
	if ok, retryAfter := s.loginProtector.Allow(r); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"message":     "too many login attempts, please try later",
			"retry_after": retryAfter,
		})
		return
	}

	password, err := readPasswordFromBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}

	ok, err := s.passwordStore.Verify(password)
	if err != nil {
		log.Printf("auth: password verify failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, jsonMessage{Message: "failed to verify password"})
		return
	}
	if !ok {
		retryAfter := s.loginProtector.RecordFailure(r)
		log.Printf("auth: login failed from %s", clientAddressForLog(r, s.cfg.TrustProxyHeaders))
		writeJSON(w, http.StatusUnauthorized, jsonMessage{Message: "invalid credentials"})
		if retryAfter > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		}
		return
	}

	if err := s.sessions.Create(w, r); err != nil {
		log.Printf("auth: session create failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, jsonMessage{Message: "failed to create session"})
		return
	}
	s.loginProtector.RecordSuccess(r)
	log.Printf("auth: login success from %s", clientAddressForLog(r, s.cfg.TrustProxyHeaders))
	writeJSON(w, http.StatusOK, jsonMessage{Message: "login succeeded"})
}

// handleLogout 清除会话 Cookie。
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	s.sessions.Destroy(w, r)
	log.Printf("auth: logout from %s", clientAddressForLog(r, s.cfg.TrustProxyHeaders))
	writeJSON(w, http.StatusOK, jsonMessage{Message: "logged out"})
}

// handleMe 返回当前认证状态。
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	if !s.sessions.IsAuthenticated(r) {
		writeJSON(w, http.StatusUnauthorized, jsonMessage{Message: "not authenticated"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated":         true,
		"base_path":             s.cfg.BasePath,
		"is_secure":             isRequestSecure(r, s.cfg.TrustProxyHeaders),
		"disable_https_warning": s.cfg.DisableHTTPSWarn,
		"projects":              []string{"Sealdice", "Napcat", "Lagrange", "LuckyLilliaBot"},
	})
}

// handleServices 处理服务列表读取与新建。
func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	items, err := s.serviceStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, jsonMessage{Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": items})
}

// handleDeploySealdiceAuto 处理 Sealdice 自动下载部署请求。
func (s *Server) handleDeploySealdiceAuto(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}

	// reqBody 鐎规矮绠熺拠銉δ侀崸妞惧▏閻劎娈戦弫鐗堝祦缂佹挻鐎妴?
	type reqBody struct {
		Source       string `json:"source"`
		URL          string `json:"url"`
		RegistryName string `json:"registry_name"`
		DisplayName  string `json:"display_name"`
		Port         int    `json:"port"`
		AutoStart    bool   `json:"auto_start"`
		Restart      struct {
			Enabled       bool `json:"enabled"`
			DelaySeconds  int  `json:"delay_seconds"`
			MaxCrashCount int  `json:"max_crash_count"`
		} `json:"restart"`
	}
	var body reqBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid json body"})
		return
	}

	source := strings.ToLower(strings.TrimSpace(body.Source))
	if source == "" {
		source = "auto"
	}
	if source != "auto" && source != "url" {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid source"})
		return
	}
	resolvedURL := strings.TrimSpace(body.URL)
	if source == "auto" {
		release, err := update.FetchSealdiceLatest(r.Context(), runtime.GOARCH)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, jsonMessage{Message: err.Error()})
			return
		}
		resolvedURL = release.DownloadURL
	}
	if strings.TrimSpace(resolvedURL) == "" {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "download url is empty"})
		return
	}

	deployLog := s.newDeployLogger("Sealdice", body.RegistryName)
	defer deployLog.Close()
	deployLog.Printf("deploy: sealdice auto request registry=%s port=%d auto_start=%t source=%s url=%s", body.RegistryName, body.Port, body.AutoStart, source, resolvedURL)

	restartPolicy := toRestartPolicy(
		body.Restart.Enabled,
		body.Restart.DelaySeconds,
		body.Restart.MaxCrashCount,
	)
	item, err := s.deploySealdice(
		body.RegistryName,
		body.DisplayName,
		body.Port,
		body.AutoStart,
		restartPolicy,
		source,
		resolvedURL,
		func(name string) (string, error) {
			return s.sealdiceDeployer.DeployFromURL(name, resolvedURL, deployLog.Printf)
		},
	)
	if err != nil {
		log.Printf("deploy: sealdice auto failed registry=%s err=%v", body.RegistryName, err)
		deployLog.Printf("deploy: failed err=%v", err)
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"message":    err.Error(),
			"deploy_log": deployLog.Name,
		})
		return
	}
	log.Printf("deploy: sealdice auto success registry=%s port=%d", body.RegistryName, body.Port)
	deployLog.Printf("deploy: success id=%s", item.ID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"service":    item,
		"deploy_log": deployLog.Name,
	})
}

// handleDeploySealdiceUpload 处理 Sealdice 上传压缩包部署请求。
func (s *Server) handleDeploySealdiceUpload(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	if err := r.ParseMultipartForm(1024 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid multipart form"})
		return
	}

	registryName := strings.TrimSpace(r.FormValue("registry_name"))
	displayName := strings.TrimSpace(r.FormValue("display_name"))
	port, err := strconv.Atoi(strings.TrimSpace(r.FormValue("port")))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid port"})
		return
	}
	autoStart := parseBool(r.FormValue("auto_start"))
	restartEnabled := parseBool(r.FormValue("restart_enabled"))
	restartDelay, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("restart_delay_seconds")))
	restartMaxCrash, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("restart_max_crash_count")))

	file, _, err := r.FormFile("package")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "package file is required"})
		return
	}
	defer func() { _ = file.Close() }()
	tmpPath, cachePath, err := s.saveUploadForRebuild("Sealdice", registryName, file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}
	defer func() { _ = os.Remove(tmpPath) }()

	deployLog := s.newDeployLogger("Sealdice", registryName)
	defer deployLog.Close()
	deployLog.Printf("deploy: sealdice upload request registry=%s port=%d auto_start=%t", registryName, port, autoStart)
	deployLog.Printf("deploy: upload cache %s", cachePath)

	restartPolicy := toRestartPolicy(restartEnabled, restartDelay, restartMaxCrash)
	item, err := s.deploySealdice(
		registryName,
		displayName,
		port,
		autoStart,
		restartPolicy,
		"upload",
		"",
		func(name string) (string, error) {
			f, openErr := os.Open(tmpPath)
			if openErr != nil {
				return "", openErr
			}
			defer func() { _ = f.Close() }()
			return s.sealdiceDeployer.DeployFromReader(name, f, deployLog.Printf)
		},
	)
	if err != nil {
		log.Printf("deploy: sealdice upload failed registry=%s err=%v", registryName, err)
		deployLog.Printf("deploy: failed err=%v", err)
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"message":    err.Error(),
			"deploy_log": deployLog.Name,
		})
		return
	}
	log.Printf("deploy: sealdice upload success registry=%s port=%d", registryName, port)
	deployLog.Printf("deploy: success id=%s", item.ID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"service":    item,
		"deploy_log": deployLog.Name,
	})
}

// handleDeployLagrangeAuto 处理 Lagrange 自动下载部署请求。
func (s *Server) handleDeployLagrangeAuto(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}

	// reqBody 鐎规矮绠熺拠銉δ侀崸妞惧▏閻劎娈戦弫鐗堝祦缂佹挻鐎妴?
	type reqBody struct {
		Source          string `json:"source"`
		RegistryName    string `json:"registry_name"`
		DisplayName     string `json:"display_name"`
		Port            int    `json:"port"`
		AutoStart       bool   `json:"auto_start"`
		Version         string `json:"version"`
		DownloadPrefix  string `json:"download_prefix"`
		DownloadURL     string `json:"download_url"`
		SignServer      string `json:"sign_server_url"`
		EnableForwardWS bool   `json:"enable_forward_ws"`
		ForwardWSPort   int    `json:"forward_ws_port"`
		EnableReverseWS bool   `json:"enable_reverse_ws"`
		ReverseWSPort   int    `json:"reverse_ws_port"`
		EnableHTTP      bool   `json:"enable_http"`
		HTTPPort        int    `json:"http_port"`
		Restart         struct {
			Enabled       bool `json:"enabled"`
			DelaySeconds  int  `json:"delay_seconds"`
			MaxCrashCount int  `json:"max_crash_count"`
		} `json:"restart"`
	}
	var body reqBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid json body"})
		return
	}

	if body.ForwardWSPort == 0 && body.Port > 0 {
		body.ForwardWSPort = body.Port
	}
	if body.ForwardWSPort == 0 {
		body.ForwardWSPort = 18080
	}
	if body.ReverseWSPort == 0 {
		body.ReverseWSPort = 18081
	}
	if body.HTTPPort == 0 {
		body.HTTPPort = 18082
	}
	if !body.EnableForwardWS {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "forward ws must be enabled"})
		return
	}
	source := strings.ToLower(strings.TrimSpace(body.Source))
	if source == "" {
		source = "auto"
	}
	if source != "auto" && source != "url" {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid source"})
		return
	}

	resolvedURL := strings.TrimSpace(body.DownloadURL)
	resolvedVersion := strings.TrimSpace(body.Version)
	if source == "auto" {
		pick, err := update.ResolveLagrangeDownloadURL(r.Context(), runtime.GOARCH, resolvedVersion)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, jsonMessage{Message: err.Error()})
			return
		}
		resolvedURL = pick.DownloadURL
		if strings.TrimSpace(resolvedVersion) == "" {
			resolvedVersion = pick.Key
		}
	}
	if strings.TrimSpace(resolvedURL) == "" {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "download url is empty"})
		return
	}

	deployLog := s.newDeployLogger("Lagrange", body.RegistryName)
	defer deployLog.Close()
	deployLog.Printf("deploy: lagrange request registry=%s auto_start=%t source=%s version=%s url=%s", body.RegistryName, body.AutoStart, source, resolvedVersion, resolvedURL)

	opts := deploy.LagrangeDeployOptions{
		Version:         resolvedVersion,
		SignServerURL:   body.SignServer,
		DownloadPrefix:  body.DownloadPrefix,
		DownloadURL:     resolvedURL,
		EnableForwardWS: body.EnableForwardWS,
		ForwardWSPort:   body.ForwardWSPort,
		ForwardWSHost:   "127.0.0.1",
		EnableReverseWS: body.EnableReverseWS,
		ReverseWSPort:   body.ReverseWSPort,
		ReverseWSHost:   "127.0.0.1",
		ReverseWSSuffix: "/ws",
		EnableHTTP:      body.EnableHTTP,
		HTTPPort:        body.HTTPPort,
		HTTPHost:        "127.0.0.1",
	}

	restartPolicy := toRestartPolicy(
		body.Restart.Enabled,
		body.Restart.DelaySeconds,
		body.Restart.MaxCrashCount,
	)
	item, err := s.deployLagrange(
		body.RegistryName,
		body.DisplayName,
		body.AutoStart,
		restartPolicy,
		opts,
		source,
		deployLog.Printf,
	)
	if err != nil {
		log.Printf("deploy: lagrange auto failed registry=%s err=%v", body.RegistryName, err)
		deployLog.Printf("deploy: failed err=%v", err)
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"message":    err.Error(),
			"deploy_log": deployLog.Name,
		})
		return
	}
	log.Printf("deploy: lagrange auto success registry=%s", body.RegistryName)
	deployLog.Printf("deploy: success id=%s", item.ID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"service":    item,
		"deploy_log": deployLog.Name,
	})
}

// handleDeployLagrangeUpload 处理 Lagrange 上传压缩包部署请求。
func (s *Server) handleDeployLagrangeUpload(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	if err := r.ParseMultipartForm(1024 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid multipart form"})
		return
	}

	registryName := strings.TrimSpace(r.FormValue("registry_name"))
	displayName := strings.TrimSpace(r.FormValue("display_name"))
	autoStart := parseBool(r.FormValue("auto_start"))
	version := strings.TrimSpace(r.FormValue("version"))
	restartEnabled := parseBool(r.FormValue("restart_enabled"))
	restartDelay, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("restart_delay_seconds")))
	restartMaxCrash, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("restart_max_crash_count")))

	forwardPort, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("forward_ws_port")))
	reversePort, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("reverse_ws_port")))
	httpPort, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("http_port")))
	enableReverse := parseBool(r.FormValue("enable_reverse_ws"))
	enableHTTP := parseBool(r.FormValue("enable_http"))
	signServer := strings.TrimSpace(r.FormValue("sign_server_url"))
	if forwardPort == 0 {
		forwardPort = 18080
	}
	if reversePort == 0 {
		reversePort = 18081
	}
	if httpPort == 0 {
		httpPort = 18082
	}

	file, _, err := r.FormFile("package")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "package file is required"})
		return
	}
	defer func() { _ = file.Close() }()
	tmpPath, cachePath, err := s.saveUploadForRebuild("Lagrange", registryName, file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}
	defer func() { _ = os.Remove(tmpPath) }()

	deployLog := s.newDeployLogger("Lagrange", registryName)
	defer deployLog.Close()
	deployLog.Printf("deploy: lagrange upload request registry=%s auto_start=%t version=%s", registryName, autoStart, version)
	deployLog.Printf("deploy: upload cache %s", cachePath)

	opts := deploy.LagrangeDeployOptions{
		Version:         version,
		SignServerURL:   signServer,
		EnableForwardWS: true,
		ForwardWSPort:   forwardPort,
		ForwardWSHost:   "127.0.0.1",
		EnableReverseWS: enableReverse,
		ReverseWSPort:   reversePort,
		ReverseWSHost:   "127.0.0.1",
		ReverseWSSuffix: "/ws",
		EnableHTTP:      enableHTTP,
		HTTPPort:        httpPort,
		HTTPHost:        "127.0.0.1",
	}

	restartPolicy := toRestartPolicy(restartEnabled, restartDelay, restartMaxCrash)
	item, err := s.deployLagrangeWithInstaller(
		registryName,
		displayName,
		autoStart,
		restartPolicy,
		opts,
		"upload",
		deployLog.Printf,
		func(name string) (string, error) {
			f, openErr := os.Open(tmpPath)
			if openErr != nil {
				return "", openErr
			}
			defer func() { _ = f.Close() }()
			return s.lagrangeDeployer.DeployFromReader(name, opts, f, deployLog.Printf)
		},
	)
	if err != nil {
		log.Printf("deploy: lagrange upload failed registry=%s err=%v", registryName, err)
		deployLog.Printf("deploy: failed err=%v", err)
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"message":    err.Error(),
			"deploy_log": deployLog.Name,
		})
		return
	}
	log.Printf("deploy: lagrange upload success registry=%s", registryName)
	deployLog.Printf("deploy: success id=%s", item.ID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"service":    item,
		"deploy_log": deployLog.Name,
	})
}

// handleDeployLLBotAuto 处理 LuckyLilliaBot 自动下载部署请求。
func (s *Server) handleDeployLLBotAuto(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}

	// reqBody 鐎规矮绠熺拠銉δ侀崸妞惧▏閻劎娈戦弫鐗堝祦缂佹挻鐎妴?
	type reqBody struct {
		Source       string `json:"source"`
		URL          string `json:"url"`
		RegistryName string `json:"registry_name"`
		DisplayName  string `json:"display_name"`
		Port         int    `json:"port"`
		AutoStart    bool   `json:"auto_start"`
		Version      string `json:"version"`
		Restart      struct {
			Enabled       bool `json:"enabled"`
			DelaySeconds  int  `json:"delay_seconds"`
			MaxCrashCount int  `json:"max_crash_count"`
		} `json:"restart"`
	}
	var body reqBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid json body"})
		return
	}
	if body.Port == 0 {
		body.Port = 3212
	}
	source := strings.ToLower(strings.TrimSpace(body.Source))
	if source == "" {
		source = "auto"
	}
	if source != "auto" && source != "url" {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid source"})
		return
	}
	if runtime.GOOS == "linux" && !bootstrap.HasLinuxQQ() {
		writeJSON(w, http.StatusConflict, jsonMessage{Message: "QQ is not installed, install QQ first"})
		return
	}

	deployLog := s.newDeployLogger("LuckyLilliaBot", body.RegistryName)
	defer deployLog.Close()
	deployLog.Printf("deploy: llbot request registry=%s port=%d auto_start=%t source=%s version=%s", body.RegistryName, body.Port, body.AutoStart, source, body.Version)

	restartPolicy := toRestartPolicy(
		body.Restart.Enabled,
		body.Restart.DelaySeconds,
		body.Restart.MaxCrashCount,
	)
	var item services.Service
	var err error
	if source == "url" {
		url := strings.TrimSpace(body.URL)
		if url == "" {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "download url is required"})
			return
		}
		item, err = s.deployLLBotWithInstaller(
			body.RegistryName,
			body.DisplayName,
			body.Port,
			body.AutoStart,
			restartPolicy,
			body.Version,
			source,
			deployLog.Printf,
			func(name string) (string, error) {
				return s.llbotDeployer.DeployFromURL(name, url, deployLog.Printf)
			},
		)
	} else {
		item, err = s.deployLLBot(
			body.RegistryName,
			body.DisplayName,
			body.Port,
			body.AutoStart,
			restartPolicy,
			body.Version,
			source,
			deployLog.Printf,
		)
	}
	if err != nil {
		log.Printf("deploy: llbot auto failed registry=%s err=%v", body.RegistryName, err)
		deployLog.Printf("deploy: failed err=%v", err)
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"message":    err.Error(),
			"deploy_log": deployLog.Name,
		})
		return
	}
	log.Printf("deploy: llbot auto success registry=%s port=%d", body.RegistryName, body.Port)
	deployLog.Printf("deploy: success id=%s", item.ID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"service":    item,
		"deploy_log": deployLog.Name,
	})
}

// handleDeployLLBotUpload 处理 LuckyLilliaBot 上传压缩包部署请求。
func (s *Server) handleDeployLLBotUpload(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	if runtime.GOOS == "linux" && !bootstrap.HasLinuxQQ() {
		writeJSON(w, http.StatusConflict, jsonMessage{Message: "QQ is not installed, install QQ first"})
		return
	}
	if err := r.ParseMultipartForm(1024 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid multipart form"})
		return
	}

	registryName := strings.TrimSpace(r.FormValue("registry_name"))
	displayName := strings.TrimSpace(r.FormValue("display_name"))
	port, err := strconv.Atoi(strings.TrimSpace(r.FormValue("port")))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid port"})
		return
	}
	autoStart := parseBool(r.FormValue("auto_start"))
	restartEnabled := parseBool(r.FormValue("restart_enabled"))
	restartDelay, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("restart_delay_seconds")))
	restartMaxCrash, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("restart_max_crash_count")))

	file, _, err := r.FormFile("package")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "package file is required"})
		return
	}
	defer func() { _ = file.Close() }()
	tmpPath, cachePath, err := s.saveUploadForRebuild("LuckyLilliaBot", registryName, file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}
	defer func() { _ = os.Remove(tmpPath) }()

	deployLog := s.newDeployLogger("LuckyLilliaBot", registryName)
	defer deployLog.Close()
	deployLog.Printf("deploy: llbot upload request registry=%s port=%d auto_start=%t", registryName, port, autoStart)
	deployLog.Printf("deploy: upload cache %s", cachePath)

	restartPolicy := toRestartPolicy(restartEnabled, restartDelay, restartMaxCrash)
	item, err := s.deployLLBotWithInstaller(
		registryName,
		displayName,
		port,
		autoStart,
		restartPolicy,
		"latest",
		"upload",
		deployLog.Printf,
		func(name string) (string, error) {
			f, openErr := os.Open(tmpPath)
			if openErr != nil {
				return "", openErr
			}
			defer func() { _ = f.Close() }()
			return s.llbotDeployer.DeployFromReader(name, f, deployLog.Printf)
		},
	)
	if err != nil {
		log.Printf("deploy: llbot upload failed registry=%s err=%v", registryName, err)
		deployLog.Printf("deploy: failed err=%v", err)
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"message":    err.Error(),
			"deploy_log": deployLog.Name,
		})
		return
	}
	log.Printf("deploy: llbot upload success registry=%s port=%d", registryName, port)
	deployLog.Printf("deploy: success id=%s", item.ID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"service":    item,
		"deploy_log": deployLog.Name,
	})
}

// handleLLBotQQStatus 返回当前 Linux QQ 安装状态。
func (s *Server) handleLLBotQQStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"supported": runtime.GOOS == "linux",
		"installed": bootstrap.HasLinuxQQ(),
		"path":      "/opt/QQ/qq",
		"default":   bootstrap.DefaultQQDebURL,
	})
}

// handleLLBotQQInstall 执行 Linux QQ 安装/覆盖安装流程。
func (s *Server) handleLLBotQQInstall(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	// reqBody 鐎规矮绠熺拠銉δ侀崸妞惧▏閻劎娈戦弫鐗堝祦缂佹挻鐎妴?
	type reqBody struct {
		URL string `json:"url"`
	}
	var body reqBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid json body"})
		return
	}
	deployLog := s.newDeployLogger("QQ", "linuxqq")
	defer deployLog.Close()
	deployLog.Printf("qq: install request")
	if err := bootstrap.InstallLinuxQQ(body.URL, deployLog.Printf); err != nil {
		deployLog.Printf("qq: install failed err=%v", err)
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"message":    err.Error(),
			"deploy_log": deployLog.Name,
		})
		return
	}
	deployLog.Printf("qq: install success")
	writeJSON(w, http.StatusOK, map[string]any{
		"message":    "QQ installed",
		"deploy_log": deployLog.Name,
	})
}

// handleLagrangeSignProbe 测试签名节点可达性并返回延迟。
func (s *Server) handleLagrangeSignProbe(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}

	// reqBody 鐎规矮绠熺拠銉δ侀崸妞惧▏閻劎娈戦弫鐗堝祦缂佹挻鐎妴?
	type reqBody struct {
		URL string `json:"url"`
	}
	var body reqBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid json body"})
		return
	}
	base := strings.TrimRight(strings.TrimSpace(body.URL), "/")
	if base == "" {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "url is required"})
		return
	}

	// attempt 表示一次 /ping 请求检测结果。
	type attempt struct {
		Index      int    `json:"index"`
		DurationMS int64  `json:"duration_ms"`
		HTTPCode   int    `json:"http_code"`
		BodyBytes  int    `json:"body_bytes"`
		OK         bool   `json:"ok"`
		Error      string `json:"error,omitempty"`
	}

	client := &http.Client{Timeout: 15 * time.Second}
	results := make([]attempt, 0, 1)
	start := time.Now()
	res := attempt{Index: 1}
	req, _ := http.NewRequest(http.MethodGet, base+"/ping", nil)
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := client.Do(req)
	res.DurationMS = time.Since(start).Milliseconds()
	if err != nil {
		res.OK = false
		res.Error = err.Error()
	} else {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		_ = resp.Body.Close()
		res.HTTPCode = resp.StatusCode
		res.BodyBytes = len(raw)
		// 规则：仅检测 /ping 是否返回 HTTP 200。
		res.OK = resp.StatusCode == http.StatusOK
	}
	results = append(results, res)

	writeJSON(w, http.StatusOK, map[string]any{
		"url":      base,
		"ok":       res.OK,
		"times":    len(results),
		"min_ms":   res.DurationMS,
		"max_ms":   res.DurationMS,
		"avg_ms":   res.DurationMS,
		"attempts": results,
	})
}

// handleLagrangeSignInfo 获取签名节点列表并透传给前端。
func (s *Server) handleLagrangeSignInfo(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get("https://d1.sealdice.com/sealsign/signinfo.json")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, jsonMessage{Message: "failed to fetch sign info"})
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeJSON(w, http.StatusBadGateway, jsonMessage{Message: "failed to fetch sign info"})
		return
	}

	var body any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadGateway, jsonMessage{Message: "invalid sign info payload"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": body})
}

// handleLagrangeVersions 返回当前架构可用的 Lagrange 版本列表（来自 manifest 源）。
func (s *Server) handleLagrangeVersions(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	items, err := update.FetchLagrangeVersions(r.Context(), runtime.GOARCH)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, jsonMessage{Message: err.Error()})
		return
	}
	stable := ""
	for _, item := range items {
		if !item.IsLatest {
			stable = item.Key
			break
		}
	}
	if stable == "" && len(items) > 0 {
		stable = items[0].Key
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":          items,
		"default_stable": stable,
	})
}

// deploySealdice 统一落地 Sealdice 部署并注册服务记录。
func (s *Server) deploySealdice(
	registryName string,
	displayName string,
	port int,
	autoStart bool,
	restart services.RestartPolicy,
	source string,
	url string,
	installer sealdiceInstaller,
) (services.Service, error) {
	if err := services.ValidateRegistryName(registryName); err != nil {
		return services.Service{}, err
	}
	if port < 1 || port > 65535 {
		return services.Service{}, errors.New("port must be in 1-65535")
	}
	if ok, message, _, err := s.checkPortAvailable(port, ""); err != nil {
		return services.Service{}, err
	} else if !ok {
		return services.Service{}, errors.New(message)
	}
	if displayName == "" {
		displayName = registryName
	}
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" {
		source = "auto"
	}

	execPath, err := installer(registryName)
	if err != nil {
		return services.Service{}, err
	}
	bootstrap.TryAllowUFWPort(port, nil)

	meta := sealdiceDeployMeta{
		Source: source,
		URL:    strings.TrimSpace(url),
		Auto:   strings.EqualFold(strings.TrimSpace(source), "auto"),
		Port:   port,
	}
	rawMeta, _ := json.Marshal(meta)

	item, err := s.serviceStore.Create(services.CreateServiceRequest{
		ID:          registryName,
		Name:        registryName,
		DisplayName: displayName,
		Type:        "Sealdice",
		WorkDir:     filepath.Dir(execPath),
		InstallDir:  s.sealdiceDeployer.TargetDir(registryName),
		ExecPath:    execPath,
		Args:        []string{"--address=0.0.0.0:" + strconv.Itoa(port)},
		Port:        port,
		AutoStart:   autoStart,
		Restart:     restart,
		Env:         map[string]string{},
		DeployMeta:  services.DeployMeta{Kind: "Sealdice", Payload: rawMeta},
	}, filepath.Join(s.cfg.DataDir, "logs"))
	if err != nil {
		return services.Service{}, err
	}

	if autoStart {
		_, _ = s.serviceMgr.Start(item.ID)
		item, _ = s.serviceStore.Get(item.ID)
	}
	return item, nil
}

// deployLagrange 统一落地 Lagrange 部署并注册服务记录。
func (s *Server) deployLagrange(
	registryName string,
	displayName string,
	autoStart bool,
	restart services.RestartPolicy,
	opts deploy.LagrangeDeployOptions,
	source string,
	logf deployLogFunc,
) (services.Service, error) {
	return s.deployLagrangeWithInstaller(
		registryName,
		displayName,
		autoStart,
		restart,
		opts,
		source,
		logf,
		func(name string) (string, error) {
			return s.lagrangeDeployer.DeployFromAuto(name, opts, logf)
		},
	)
}

func (s *Server) deployLagrangeWithInstaller(
	registryName string,
	displayName string,
	autoStart bool,
	restart services.RestartPolicy,
	opts deploy.LagrangeDeployOptions,
	source string,
	logf deployLogFunc,
	installer llbotInstaller,
) (services.Service, error) {
	if err := services.ValidateRegistryName(registryName); err != nil {
		return services.Service{}, err
	}
	if displayName == "" {
		displayName = registryName
	}
	if opts.EnableForwardWS {
		if ok, message, _, err := s.checkPortAvailable(opts.ForwardWSPort, ""); err != nil {
			return services.Service{}, err
		} else if !ok {
			return services.Service{}, errors.New(message)
		}
	}
	if opts.EnableReverseWS {
		if ok, message, _, err := s.checkPortAvailable(opts.ReverseWSPort, ""); err != nil {
			return services.Service{}, err
		} else if !ok {
			return services.Service{}, errors.New(message)
		}
	}
	if opts.EnableHTTP {
		if ok, message, _, err := s.checkPortAvailable(opts.HTTPPort, ""); err != nil {
			return services.Service{}, err
		} else if !ok {
			return services.Service{}, errors.New(message)
		}
	}

	execPath, err := installer(registryName)
	if err != nil {
		return services.Service{}, err
	}
	mainPort := 0
	if opts.EnableForwardWS {
		mainPort = opts.ForwardWSPort
	} else if opts.EnableReverseWS {
		mainPort = opts.ReverseWSPort
	} else if opts.EnableHTTP {
		mainPort = opts.HTTPPort
	}

	if strings.TrimSpace(source) == "" {
		source = "auto"
	}
	meta := lagrangeDeployMeta{
		Source:          strings.ToLower(strings.TrimSpace(source)),
		Version:         opts.Version,
		SignServerURL:   opts.SignServerURL,
		DownloadPrefix:  opts.DownloadPrefix,
		DownloadURL:     strings.TrimSpace(opts.DownloadURL),
		EnableForwardWS: opts.EnableForwardWS,
		ForwardWSPort:   opts.ForwardWSPort,
		EnableReverseWS: opts.EnableReverseWS,
		ReverseWSPort:   opts.ReverseWSPort,
		EnableHTTP:      opts.EnableHTTP,
		HTTPPort:        opts.HTTPPort,
	}
	rawMeta, _ := json.Marshal(meta)

	item, err := s.serviceStore.Create(services.CreateServiceRequest{
		ID:          registryName,
		Name:        registryName,
		DisplayName: displayName,
		Type:        "Lagrange",
		WorkDir:     filepath.Dir(execPath),
		InstallDir:  s.lagrangeDeployer.TargetDir(registryName),
		ExecPath:    execPath,
		Args:        []string{},
		Port:        mainPort,
		AutoStart:   autoStart,
		Restart:     restart,
		Env:         map[string]string{},
		DeployMeta:  services.DeployMeta{Kind: "Lagrange", Payload: rawMeta},
	}, filepath.Join(s.cfg.DataDir, "logs"))
	if err != nil {
		return services.Service{}, err
	}

	if autoStart {
		_, _ = s.serviceMgr.Start(item.ID)
		item, _ = s.serviceStore.Get(item.ID)
	}
	return item, nil
}

// deployLLBot 统一落地 LuckyLilliaBot 部署并注册服务记录。
func (s *Server) deployLLBot(
	registryName string,
	displayName string,
	port int,
	autoStart bool,
	restart services.RestartPolicy,
	version string,
	source string,
	logf deployLogFunc,
) (services.Service, error) {
	return s.deployLLBotWithInstaller(
		registryName,
		displayName,
		port,
		autoStart,
		restart,
		version,
		source,
		logf,
		func(name string) (string, error) {
			return s.llbotDeployer.DeployFromAuto(name, version, logf)
		},
	)
}

func (s *Server) deployLLBotWithInstaller(
	registryName string,
	displayName string,
	port int,
	autoStart bool,
	restart services.RestartPolicy,
	version string,
	source string,
	logf deployLogFunc,
	installer llbotInstaller,
) (services.Service, error) {
	if err := services.ValidateRegistryName(registryName); err != nil {
		return services.Service{}, err
	}
	if port < 1 || port > 65535 {
		return services.Service{}, errors.New("port must be in 1-65535")
	}
	if ok, message, _, err := s.checkPortAvailable(port, ""); err != nil {
		return services.Service{}, err
	} else if !ok {
		return services.Service{}, errors.New(message)
	}
	if displayName == "" {
		displayName = registryName
	}

	execPath, err := installer(registryName)
	if err != nil {
		return services.Service{}, err
	}
	if err := deploy.UpdateLLBotWebUIPort(s.llbotDeployer.TargetDir(registryName), port); err != nil {
		return services.Service{}, err
	}
	bootstrap.TryAllowUFWPort(port, logf)

	if strings.TrimSpace(source) == "" {
		source = "auto"
	}
	meta := llbotDeployMeta{
		Source:  strings.ToLower(strings.TrimSpace(source)),
		Version: version,
		Port:    port,
	}
	rawMeta, _ := json.Marshal(meta)

	item, err := s.serviceStore.Create(services.CreateServiceRequest{
		ID:          registryName,
		Name:        registryName,
		DisplayName: displayName,
		Type:        "LuckyLilliaBot",
		WorkDir:     filepath.Dir(execPath),
		InstallDir:  s.llbotDeployer.TargetDir(registryName),
		ExecPath:    execPath,
		Args:        []string{},
		Port:        port,
		AutoStart:   autoStart,
		Restart:     restart,
		Env:         map[string]string{},
		DeployMeta:  services.DeployMeta{Kind: "LuckyLilliaBot", Payload: rawMeta},
	}, filepath.Join(s.cfg.DataDir, "logs"))
	if err != nil {
		return services.Service{}, err
	}

	if autoStart {
		_, _ = s.serviceMgr.Start(item.ID)
		item, _ = s.serviceStore.Get(item.ID)
	}
	return item, nil
}

// handleServiceByID 是服务子路由总入口。
// handleServiceByID 是服务子路由总入口，负责 action 分发。
func (s *Server) handleServiceByID(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}

	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/services/"), "/")
	if raw == "" {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "service id is required"})
		return
	}

	parts := strings.Split(raw, "/")
	id := parts[0]
	if len(parts) == 1 {
		s.handleServiceGet(w, r, id)
		return
	}

	action := parts[1]
	switch action {
	case "start", "stop", "force-stop", "restart":
		s.handleServiceLifecycle(w, r, id, action)
	case "port":
		s.handleServicePort(w, r, id)
	case "config":
		s.handleLagrangeConfig(w, r, id)
	case "settings":
		s.handleServiceSettings(w, r, id)
	case "logs":
		s.handleServiceLogs(w, r, id, parts[2:])
	case "metrics":
		s.handleServiceMetrics(w, r, id)
	case "files":
		s.handleServiceFiles(w, r, id, parts[2:])
	case "rebuild":
		s.handleServiceRebuild(w, r, id)
	case "rebuild-info":
		s.handleServiceRebuildInfo(w, r, id)
	case "delete":
		s.handleServiceDelete(w, r, id)
	case "qrcode":
		s.handleServiceQRCode(w, r, id)
	default:
		writeJSON(w, http.StatusNotFound, jsonMessage{Message: "unknown action"})
	}
}

// handleServiceGet 读取单个服务详情。
// 路径：GET /api/services/{id}
func (s *Server) handleServiceGet(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	item, err := s.serviceStore.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, jsonMessage{Message: "service not found"})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// handleServiceLifecycle 处理服务进程生命周期动作。
// 支持动作：start / stop / force-stop / restart。
func (s *Server) handleServiceLifecycle(w http.ResponseWriter, r *http.Request, id string, action string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}

	var (
		item services.Service
		err  error
	)
	switch action {
	case "start":
		item, err = s.serviceMgr.Start(id)
	case "stop":
		item, err = s.serviceMgr.Stop(id)
	case "force-stop":
		item, err = s.serviceMgr.ForceStop(id)
	case "restart":
		item, err = s.serviceMgr.Restart(id)
	}
	if err != nil {
		log.Printf("service: %s failed id=%s err=%v", action, id, err)
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}
	log.Printf("service: %s success id=%s pid=%d", action, id, item.PID)
	writeJSON(w, http.StatusOK, item)
}

// handleServicePort 修改服务主端口。
// 约束：服务必须为停止状态，且端口必须可用。
func (s *Server) handleServicePort(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	type reqBody struct {
		Port int `json:"port"`
	}
	var body reqBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid json body"})
		return
	}
	if body.Port < 1 || body.Port > 65535 {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid port"})
		return
	}

	item, err := s.serviceStore.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, jsonMessage{Message: "service not found"})
		return
	}
	if body.Port == item.Port {
		writeJSON(w, http.StatusOK, item)
		return
	}
	if item.Status == services.StatusRunning {
		writeJSON(w, http.StatusConflict, jsonMessage{Message: "service must be stopped before updating port"})
		return
	}
	if ok, message, _, err := s.checkPortAvailable(body.Port, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, jsonMessage{Message: err.Error()})
		return
	} else if !ok {
		writeJSON(w, http.StatusConflict, jsonMessage{Message: message})
		return
	}

	item.Port = body.Port
	switch item.Type {
	case "Sealdice":
		item.Args = []string{"--address=0.0.0.0:" + strconv.Itoa(body.Port)}
		updateSealdiceMetaPort(&item, body.Port)
		bootstrap.TryAllowUFWPort(body.Port, nil)
	case "Lagrange":
		if err := deploy.UpdateLagrangeForwardWSPort(item.InstallDir, body.Port); err != nil {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
			return
		}
		updateLagrangeMetaPorts(&item, body.Port, 0, 0)
	case "LuckyLilliaBot":
		if err := deploy.UpdateLLBotWebUIPort(item.InstallDir, body.Port); err != nil {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
			return
		}
		updateLLBotMetaPort(&item, body.Port)
		bootstrap.TryAllowUFWPort(body.Port, nil)
	}

	if err := s.serviceStore.Update(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, jsonMessage{Message: err.Error()})
		return
	}
	log.Printf("service: port updated id=%s port=%d", id, body.Port)
	writeJSON(w, http.StatusOK, item)
}

// handleLagrangeConfig 读取或更新 Lagrange 配置。
// 更新时会执行端口冲突检查，并要求服务处于停止状态。
func (s *Server) handleLagrangeConfig(w http.ResponseWriter, r *http.Request, id string) {
	item, err := s.serviceStore.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, jsonMessage{Message: "service not found"})
		return
	}
	if item.Type != "Lagrange" {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "config action is only supported for Lagrange"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		state, err := deploy.ReadLagrangeConfig(item.InstallDir)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, state)
	case http.MethodPost:
		var body deploy.LagrangeConfigState
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid json body"})
			return
		}
		current, err := deploy.ReadLagrangeConfig(item.InstallDir)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
			return
		}
		if lagrangeConfigEqual(current, body) {
			writeJSON(w, http.StatusOK, item)
			return
		}
		if item.Status == services.StatusRunning {
			writeJSON(w, http.StatusConflict, jsonMessage{Message: "service must be stopped before updating config"})
			return
		}
		if body.EnableForwardWS {
			if ok, message, _, err := s.checkPortAvailable(body.ForwardWSPort, id); err != nil {
				writeJSON(w, http.StatusInternalServerError, jsonMessage{Message: err.Error()})
				return
			} else if !ok {
				writeJSON(w, http.StatusConflict, jsonMessage{Message: message})
				return
			}
		}
		if body.EnableReverseWS {
			if ok, message, _, err := s.checkPortAvailable(body.ReverseWSPort, id); err != nil {
				writeJSON(w, http.StatusInternalServerError, jsonMessage{Message: err.Error()})
				return
			} else if !ok {
				writeJSON(w, http.StatusConflict, jsonMessage{Message: message})
				return
			}
		}
		if body.EnableHTTP {
			if ok, message, _, err := s.checkPortAvailable(body.HTTPPort, id); err != nil {
				writeJSON(w, http.StatusInternalServerError, jsonMessage{Message: err.Error()})
				return
			} else if !ok {
				writeJSON(w, http.StatusConflict, jsonMessage{Message: message})
				return
			}
		}
		if err := deploy.UpdateLagrangeConfig(item.InstallDir, body); err != nil {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
			return
		}
		item.Port = body.ForwardWSPort
		updateLagrangeMetaConfig(&item, body)
		if err := s.serviceStore.Update(item); err != nil {
			writeJSON(w, http.StatusInternalServerError, jsonMessage{Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, item)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
	}
}

// handleServiceSettings 读写服务通用设置。
// 目前包含显示名、自启、访问地址和自动重启策略。
func (s *Server) handleServiceSettings(w http.ResponseWriter, r *http.Request, id string) {
	item, err := s.serviceStore.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, jsonMessage{Message: "service not found"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"display_name":  item.DisplayName,
			"auto_start":    item.AutoStart,
			"open_path_url": item.OpenPathURL,
			"log_policy": map[string]any{
				"retention_count": item.LogPolicy.RetentionCount,
				"retention_days":  item.LogPolicy.RetentionDays,
				"max_mb":          item.LogPolicy.MaxMB,
			},
			"restart": map[string]any{
				"enabled":           item.Restart.Enabled,
				"delay_seconds":     item.Restart.DelaySeconds,
				"max_crash_count":   item.Restart.MaxCrashCount,
				"consecutive_crash": item.Restart.ConsecutiveCrash,
			},
		})
	case http.MethodPost:
		type reqBody struct {
			DisplayName string `json:"display_name"`
			AutoStart   bool   `json:"auto_start"`
			OpenPathURL string `json:"open_path_url"`
			Restart     struct {
				Enabled       bool `json:"enabled"`
				DelaySeconds  int  `json:"delay_seconds"`
				MaxCrashCount int  `json:"max_crash_count"`
			} `json:"restart"`
			LogPolicy struct {
				RetentionCount int `json:"retention_count"`
				RetentionDays  int `json:"retention_days"`
				MaxMB          int `json:"max_mb"`
			} `json:"log_policy"`
		}
		var body reqBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid json body"})
			return
		}
		item.DisplayName = strings.TrimSpace(body.DisplayName)
		item.AutoStart = body.AutoStart
		item.OpenPathURL = strings.TrimSpace(body.OpenPathURL)
		item.Restart = toRestartPolicy(body.Restart.Enabled, body.Restart.DelaySeconds, body.Restart.MaxCrashCount)
		if body.LogPolicy.RetentionCount < 0 || body.LogPolicy.RetentionCount > 365 {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid log_policy.retention_count"})
			return
		}
		if body.LogPolicy.RetentionDays < 0 || body.LogPolicy.RetentionDays > 3650 {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid log_policy.retention_days"})
			return
		}
		if body.LogPolicy.MaxMB < 0 || body.LogPolicy.MaxMB > 1024*1024 {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid log_policy.max_mb"})
			return
		}
		item.LogPolicy = services.LogPolicy{
			RetentionCount: body.LogPolicy.RetentionCount,
			RetentionDays:  body.LogPolicy.RetentionDays,
			MaxMB:          body.LogPolicy.MaxMB,
		}
		if err := s.serviceStore.Update(item); err != nil {
			writeJSON(w, http.StatusInternalServerError, jsonMessage{Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, item)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
	}
}

// handleServiceLogs 返回服务日志尾部内容，默认 300 行。
func (s *Server) handleServiceLogs(w http.ResponseWriter, r *http.Request, id string, tail []string) {
	if r.Method != http.MethodGet && r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	item, err := s.serviceStore.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, jsonMessage{Message: "service not found"})
		return
	}
	if len(tail) > 0 && tail[0] == "history" {
		s.handleServiceLogHistory(w, r, item, tail[1:])
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	lines := 300
	if raw := strings.TrimSpace(r.URL.Query().Get("lines")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr == nil && parsed > 0 {
			lines = parsed
		}
	}
	logText, err := services.TailFile(item.LogPath, lines)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"service_id": id,
		"log_path":   item.LogPath,
		"content":    logText,
	})
}

func (s *Server) handleServiceLogHistory(w http.ResponseWriter, r *http.Request, item services.Service, tail []string) {
	if r.Method == http.MethodDelete {
		if len(tail) == 0 {
			if clearErr := services.ClearServiceLogHistory(s.cfg.DataDir, item.ID); clearErr != nil {
				writeJSON(w, http.StatusBadRequest, jsonMessage{Message: clearErr.Error()})
				return
			}
			writeJSON(w, http.StatusOK, jsonMessage{Message: "history logs cleared"})
			return
		}
		name, err := url.PathUnescape(strings.Join(tail, "/"))
		if err != nil || strings.TrimSpace(name) == "" {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid history log name"})
			return
		}
		if delErr := services.DeleteServiceLogHistory(s.cfg.DataDir, item.ID, name); delErr != nil {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: delErr.Error()})
			return
		}
		writeJSON(w, http.StatusOK, jsonMessage{Message: "history log deleted"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	if len(tail) == 0 {
		items, err := services.ListServiceLogHistory(s.cfg.DataDir, item.ID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"service_id": item.ID,
			"items":      items,
		})
		return
	}
	name, err := url.PathUnescape(strings.Join(tail, "/"))
	if err != nil || strings.TrimSpace(name) == "" {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid history log name"})
		return
	}
	lines := 300
	if raw := strings.TrimSpace(r.URL.Query().Get("lines")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr == nil && parsed > 0 {
			lines = parsed
		}
	}
	content, readErr := services.ReadServiceLogHistoryTail(s.cfg.DataDir, item.ID, name, lines)
	if readErr != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: readErr.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"service_id": item.ID,
		"name":       name,
		"content":    content,
	})
}

// handleServiceMetrics 返回单服务资源占用快照。
func (s *Server) handleServiceMetrics(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	item, err := s.serviceStore.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, jsonMessage{Message: "service not found"})
		return
	}
	writeJSON(w, http.StatusOK, s.collectServiceMetrics(item))
}

// handleServiceRebuild 触发服务重建流程。
func (s *Server) handleServiceRebuild(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	type reqBody struct {
		Mode string `json:"mode"`
	}
	var body reqBody
	_ = json.NewDecoder(r.Body).Decode(&body)
	item, err := s.rebuildService(id, body.Mode)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// handleServiceRebuildInfo 返回重建来源信息与可用模式。
func (s *Server) handleServiceRebuildInfo(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	item, err := s.serviceStore.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, jsonMessage{Message: "service not found"})
		return
	}
	source := "auto"
	switch item.Type {
	case "Sealdice":
		if meta, ok := readDeployMeta[sealdiceDeployMeta](item.DeployMeta, "Sealdice"); ok && strings.TrimSpace(meta.Source) != "" {
			source = strings.ToLower(strings.TrimSpace(meta.Source))
		}
	case "Lagrange":
		if meta, ok := readDeployMeta[lagrangeDeployMeta](item.DeployMeta, "Lagrange"); ok && strings.TrimSpace(meta.Source) != "" {
			source = strings.ToLower(strings.TrimSpace(meta.Source))
		}
	case "LuckyLilliaBot":
		if meta, ok := readDeployMeta[llbotDeployMeta](item.DeployMeta, "LuckyLilliaBot"); ok && strings.TrimSpace(meta.Source) != "" {
			source = strings.ToLower(strings.TrimSpace(meta.Source))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"type":          item.Type,
		"source":        source,
		"default_mode":  source,
		"upload_cached": fileExists(s.rebuildCachePath(item.Type, item.ID)),
		"auto_enabled":  true,
	})
}

// handleServiceDelete 删除服务记录与安装目录。
func (s *Server) handleServiceDelete(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	if err := s.deleteService(id); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, jsonMessage{Message: "service deleted"})
}

// handleServiceQRCode 返回 Lagrange 登录二维码图片。
func (s *Server) handleServiceQRCode(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}
	item, err := s.serviceStore.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, jsonMessage{Message: "service not found"})
		return
	}
	if item.Type != "Lagrange" {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "qrcode is only supported for Lagrange"})
		return
	}
	qrPath := filepath.Join(strings.TrimSpace(item.InstallDir), "qr-0.png")
	if _, err := filepath.Abs(qrPath); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid qrcode path"})
		return
	}
	if _, err := os.Stat(qrPath); err != nil {
		writeJSON(w, http.StatusNotFound, jsonMessage{Message: "qr-0.png not found"})
		return
	}
	http.ServeFile(w, r, qrPath)
}

// handleServiceProxy 反代到具体服务 WebUI/接口。
func (s *Server) handleServiceProxy(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}

	raw := strings.TrimPrefix(r.URL.Path, "/service/")
	raw = strings.Trim(raw, "/")
	if raw == "" {
		writeJSON(w, http.StatusNotFound, jsonMessage{Message: "service not found"})
		return
	}

	parts := strings.SplitN(raw, "/", 2)
	id := parts[0]
	restPath := "/"
	if len(parts) == 2 {
		restPath = "/" + parts[1]
	}
	s.proxyToService(w, r, id, restPath)
}

// handleDeployLogs 读取指定部署日志的尾部内容。
func (s *Server) handleDeployLogs(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, jsonMessage{Message: "method not allowed"})
		return
	}

	name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/deploy/logs/"), "/")
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid log name"})
		return
	}

	lines := 400
	if raw := strings.TrimSpace(r.URL.Query().Get("lines")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			lines = parsed
		}
	}

	dir := filepath.Join(s.cfg.DataDir, "logs", "deploy")
	full := filepath.Clean(filepath.Join(dir, name))
	cleanDir := filepath.Clean(dir)
	if full != cleanDir && !strings.HasPrefix(full, cleanDir+string(os.PathSeparator)) {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "invalid log path"})
		return
	}

	content, err := services.TailFile(full, lines)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"log":     name,
		"content": content,
	})
}

// rebuildService 按 DeployMeta 重新下载/解压并重建服务。
func (s *Server) rebuildService(id string, mode string) (map[string]any, error) {
	item, err := s.serviceStore.Get(id)
	if err != nil {
		return nil, err
	}

	if item.Status == services.StatusRunning {
		_, _ = s.serviceMgr.Stop(id)
	}

	deployLog := s.newDeployLogger(item.Type, item.ID)
	defer deployLog.Close()

	deployLog.Printf("deploy: rebuild start")
	if item.InstallDir == "" {
		return nil, errors.New("install_dir is empty")
	}
	if err := os.RemoveAll(item.InstallDir); err != nil {
		return nil, err
	}
	deployLog.Printf("deploy: cleared %s", item.InstallDir)

	switch item.Type {
	case "Sealdice":
		meta, ok := readDeployMeta[sealdiceDeployMeta](item.DeployMeta, "Sealdice")
		if !ok {
			return nil, errors.New("deploy meta missing for sealdice")
		}
		source := strings.ToLower(strings.TrimSpace(meta.Source))
		if source == "" {
			source = "auto"
		}
		rebuildMode := strings.ToLower(strings.TrimSpace(mode))
		if rebuildMode == "" {
			rebuildMode = source
		}
		var execPath string
		if rebuildMode == "upload" {
			f, err := os.Open(s.rebuildCachePath(item.Type, item.ID))
			if err != nil {
				return nil, errors.New("uploaded package cache not found, choose auto-download rebuild")
			}
			defer func() { _ = f.Close() }()
			execPath, err = s.sealdiceDeployer.DeployFromReader(item.ID, f, deployLog.Printf)
			if err != nil {
				return nil, err
			}
		} else {
			var err error
			downloadURL := strings.TrimSpace(meta.URL)
			if strings.EqualFold(source, "auto") {
				release, resolveErr := update.FetchSealdiceLatest(context.Background(), runtime.GOARCH)
				if resolveErr != nil {
					return nil, resolveErr
				}
				downloadURL = strings.TrimSpace(release.DownloadURL)
			}
			execPath, err = s.sealdiceDeployer.DeployFromURL(item.ID, downloadURL, deployLog.Printf)
			if err != nil {
				return nil, err
			}
		}
		item.ExecPath = execPath
		item.WorkDir = filepath.Dir(execPath)
		item.Port = meta.Port
		item.Args = []string{"--address=0.0.0.0:" + strconv.Itoa(meta.Port)}
	case "Lagrange":
		meta, ok := readDeployMeta[lagrangeDeployMeta](item.DeployMeta, "Lagrange")
		if !ok {
			return nil, errors.New("deploy meta missing for lagrange")
		}
		source := strings.ToLower(strings.TrimSpace(meta.Source))
		if source == "" {
			source = "auto"
		}
		rebuildMode := strings.ToLower(strings.TrimSpace(mode))
		if rebuildMode == "" {
			rebuildMode = source
		}
		opts := deploy.LagrangeDeployOptions{
			Version:         meta.Version,
			SignServerURL:   meta.SignServerURL,
			DownloadPrefix:  meta.DownloadPrefix,
			DownloadURL:     meta.DownloadURL,
			EnableForwardWS: meta.EnableForwardWS,
			ForwardWSPort:   meta.ForwardWSPort,
			ForwardWSHost:   "127.0.0.1",
			EnableReverseWS: meta.EnableReverseWS,
			ReverseWSPort:   meta.ReverseWSPort,
			ReverseWSHost:   "127.0.0.1",
			ReverseWSSuffix: "/ws",
			EnableHTTP:      meta.EnableHTTP,
			HTTPPort:        meta.HTTPPort,
			HTTPHost:        "127.0.0.1",
		}
		var (
			execPath string
			err      error
		)
		if rebuildMode == "upload" {
			f, openErr := os.Open(s.rebuildCachePath(item.Type, item.ID))
			if openErr != nil {
				return nil, errors.New("uploaded package cache not found, choose auto-download rebuild")
			}
			defer func() { _ = f.Close() }()
			execPath, err = s.lagrangeDeployer.DeployFromReader(item.ID, opts, f, deployLog.Printf)
		} else {
			if strings.EqualFold(source, "auto") {
				pick, resolveErr := update.ResolveLagrangeDownloadURL(context.Background(), runtime.GOARCH, meta.Version)
				if resolveErr != nil {
					return nil, resolveErr
				}
				opts.DownloadURL = pick.DownloadURL
			}
			execPath, err = s.lagrangeDeployer.DeployFromAuto(item.ID, opts, deployLog.Printf)
		}
		if err != nil {
			return nil, err
		}
		item.ExecPath = execPath
		item.WorkDir = filepath.Dir(execPath)
		item.Port = meta.ForwardWSPort
	case "LuckyLilliaBot":
		meta, ok := readDeployMeta[llbotDeployMeta](item.DeployMeta, "LuckyLilliaBot")
		if !ok {
			// Backfill from existing config for legacy services.
			port := item.Port
			if port < 1 || port > 65535 {
				p, err := deploy.ReadLLBotWebUIPort(item.InstallDir)
				if err != nil {
					deployLog.Printf("deploy: llbot meta missing and config read failed: %v", err)
					return nil, errors.New("deploy meta missing for llbot")
				}
				port = p
			}
			meta = llbotDeployMeta{Source: "auto", Version: "latest", Port: port}
			rawMeta, _ := json.Marshal(meta)
			item.DeployMeta = services.DeployMeta{Kind: "LuckyLilliaBot", Payload: rawMeta}
			_ = s.serviceStore.Update(item)
		}
		source := strings.ToLower(strings.TrimSpace(meta.Source))
		if source == "" {
			source = "auto"
		}
		rebuildMode := strings.ToLower(strings.TrimSpace(mode))
		if rebuildMode == "" {
			rebuildMode = source
		}
		var (
			execPath string
			err      error
		)
		if rebuildMode == "upload" {
			f, openErr := os.Open(s.rebuildCachePath(item.Type, item.ID))
			if openErr != nil {
				return nil, errors.New("uploaded package cache not found, choose auto-download rebuild")
			}
			defer func() { _ = f.Close() }()
			execPath, err = s.llbotDeployer.DeployFromReader(item.ID, f, deployLog.Printf)
		} else {
			execPath, err = s.llbotDeployer.DeployFromAuto(item.ID, meta.Version, deployLog.Printf)
		}
		if err != nil {
			return nil, err
		}
		if err := deploy.UpdateLLBotWebUIPort(s.llbotDeployer.TargetDir(item.ID), meta.Port); err != nil {
			return nil, err
		}
		item.ExecPath = execPath
		item.WorkDir = filepath.Dir(execPath)
		item.Port = meta.Port
	default:
		return nil, errors.New("unsupported service type")
	}

	item.Status = services.StatusStopped
	item.PID = 0
	item.LastError = ""
	item.LastExitAt = ""
	if err := s.serviceStore.Update(item); err != nil {
		return nil, err
	}
	deployLog.Printf("deploy: rebuild complete")

	if item.AutoStart {
		_, _ = s.serviceMgr.Start(item.ID)
		item, _ = s.serviceStore.Get(item.ID)
	}
	return map[string]any{"service": item, "deploy_log": deployLog.Name}, nil
}

// deleteService 删除服务记录、安装目录及相关资源。
func (s *Server) deleteService(id string) error {
	item, err := s.serviceStore.Get(id)
	if err != nil {
		return err
	}
	if item.Status == services.StatusRunning {
		_, _ = s.serviceMgr.ForceStop(id)
	}
	if item.InstallDir != "" {
		_ = os.RemoveAll(item.InstallDir)
	}
	_ = os.RemoveAll(services.ServiceLogRoot(s.cfg.DataDir, item.ID))
	if item.Type == "Sealdice" || item.Type == "LuckyLilliaBot" {
		bootstrap.TryDeleteUFWPort(item.Port, nil)
	}
	return s.serviceStore.Delete(id)
}

func readDeployMeta[T any](meta services.DeployMeta, expect string) (T, bool) {
	var zero T
	if strings.TrimSpace(meta.Kind) == "" || strings.TrimSpace(meta.Kind) != expect {
		return zero, false
	}
	if len(meta.Payload) == 0 {
		return zero, false
	}
	if err := json.Unmarshal(meta.Payload, &zero); err != nil {
		return zero, false
	}
	return zero, true
}

// toRestartPolicy 将前端配置转换为内部重启策略结构。
func toRestartPolicy(enabled bool, delaySeconds int, maxCrashCount int) services.RestartPolicy {
	if delaySeconds < 0 {
		delaySeconds = 0
	}
	if maxCrashCount < 0 {
		maxCrashCount = 0
	}
	return services.RestartPolicy{
		Enabled:       enabled,
		DelaySeconds:  delaySeconds,
		MaxCrashCount: maxCrashCount,
	}
}

// updateSealdiceMetaPort 同步更新 Sealdice DeployMeta 端口。
func updateSealdiceMetaPort(item *services.Service, port int) {
	meta, ok := readDeployMeta[sealdiceDeployMeta](item.DeployMeta, "Sealdice")
	if !ok {
		return
	}
	meta.Port = port
	raw, _ := json.Marshal(meta)
	item.DeployMeta = services.DeployMeta{Kind: "Sealdice", Payload: raw}
}

// updateLagrangeMetaPorts 同步更新 Lagrange DeployMeta 端口字段。
func updateLagrangeMetaPorts(item *services.Service, forward int, reverse int, httpPort int) {
	meta, ok := readDeployMeta[lagrangeDeployMeta](item.DeployMeta, "Lagrange")
	if !ok {
		return
	}
	if forward > 0 {
		meta.ForwardWSPort = forward
	}
	if reverse > 0 {
		meta.ReverseWSPort = reverse
	}
	if httpPort > 0 {
		meta.HTTPPort = httpPort
	}
	raw, _ := json.Marshal(meta)
	item.DeployMeta = services.DeployMeta{Kind: "Lagrange", Payload: raw}
}

// updateLagrangeMetaConfig 同步更新 Lagrange DeployMeta 完整配置。
func updateLagrangeMetaConfig(item *services.Service, state deploy.LagrangeConfigState) {
	meta, ok := readDeployMeta[lagrangeDeployMeta](item.DeployMeta, "Lagrange")
	if !ok {
		return
	}
	meta.SignServerURL = strings.TrimSpace(state.SignServerURL)
	meta.EnableForwardWS = true
	meta.ForwardWSPort = state.ForwardWSPort
	meta.EnableReverseWS = state.EnableReverseWS
	meta.ReverseWSPort = state.ReverseWSPort
	meta.EnableHTTP = state.EnableHTTP
	meta.HTTPPort = state.HTTPPort
	raw, _ := json.Marshal(meta)
	item.DeployMeta = services.DeployMeta{Kind: "Lagrange", Payload: raw}
}

// updateLLBotMetaPort 同步更新 LLBot DeployMeta 端口。
func updateLLBotMetaPort(item *services.Service, port int) {
	meta, ok := readDeployMeta[llbotDeployMeta](item.DeployMeta, "LuckyLilliaBot")
	if !ok {
		return
	}
	meta.Port = port
	raw, _ := json.Marshal(meta)
	item.DeployMeta = services.DeployMeta{Kind: "LuckyLilliaBot", Payload: raw}
}

// proxyToService 将请求转发到目标服务的本地监听端口。
func (s *Server) proxyToService(w http.ResponseWriter, r *http.Request, id string, restPath string) {
	item, err := s.serviceStore.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, jsonMessage{Message: "service not found"})
		return
	}
	if item.Port < 1 || item.Port > 65535 {
		writeJSON(w, http.StatusBadRequest, jsonMessage{Message: "service port is invalid"})
		return
	}

	target, err := url.Parse("http://127.0.0.1:" + strconv.Itoa(item.Port))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, jsonMessage{Message: "proxy target parse failed"})
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Rewrite = func(pr *httputil.ProxyRequest) {
		pr.SetURL(target)
		pr.Out.URL.Path = restPath
		pr.Out.Host = target.Host
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, e error) {
		log.Printf("proxy: id=%s target=%s err=%v", id, target.String(), e)
		writeJSON(rw, http.StatusBadGateway, jsonMessage{Message: "service is not reachable"})
	}
	proxy.ServeHTTP(w, r)
}

// parseBool 解析常见布尔字符串表示。
func parseBool(raw string) bool {
	value := strings.ToLower(strings.TrimSpace(raw))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (s *Server) rebuildCachePath(serviceType, id string) string {
	safeType := strings.ToLower(strings.TrimSpace(serviceType))
	if safeType == "" {
		safeType = "unknown"
	}
	safeID := services.SanitizeName(strings.TrimSpace(id))
	if safeID == "" {
		safeID = "unknown"
	}
	return filepath.Join(s.cfg.DataDir, "rebuild-cache", safeType, safeID, "package.bin")
}

func (s *Server) saveUploadForRebuild(serviceType, registry string, src io.Reader) (string, string, error) {
	tmp, err := os.CreateTemp("", "sealpanel-upload-*.bin")
	if err != nil {
		return "", "", err
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, src); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return "", "", err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", "", err
	}
	cachePath := s.rebuildCachePath(serviceType, registry)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		_ = os.Remove(tmpPath)
		return "", "", err
	}
	if err := copyLocalFile(tmpPath, cachePath); err != nil {
		_ = os.Remove(tmpPath)
		return "", "", err
	}
	return tmpPath, cachePath, nil
}

func copyLocalFile(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = dst.Close() }()
	_, err = io.Copy(dst, src)
	return err
}

func applyBinaryUpdate(ctx context.Context, exePath string, downloadURL string) error {
	client := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "LDM-Updater/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download update failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("download update http status: %d", resp.StatusCode)
	}

	dir := filepath.Dir(exePath)
	tmpFile, err := os.CreateTemp(dir, ".ldm-update-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := io.Copy(tmpFile, io.LimitReader(resp.Body, 512*1024*1024)); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return err
	}

	backup := exePath + ".old"
	_ = os.Remove(backup)
	if err := os.Rename(exePath, backup); err != nil {
		return fmt.Errorf("backup current binary failed: %w", err)
	}
	if err := os.Rename(tmpPath, exePath); err != nil {
		_ = os.Rename(backup, exePath)
		return fmt.Errorf("replace binary failed: %w", err)
	}
	_ = os.Remove(backup)
	return nil
}

// requireAuth 校验会话，未登录时返回 401。
func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if !s.passwordStore.Exists() {
		writeJSON(w, http.StatusPreconditionFailed, jsonMessage{Message: "password is not initialized"})
		return false
	}
	if !s.sessions.IsAuthenticated(r) {
		writeJSON(w, http.StatusUnauthorized, jsonMessage{Message: "not authenticated"})
		return false
	}
	return true
}

// readPasswordFromBody 从请求体解析 password 字段并做基础校验。
func readPasswordFromBody(r *http.Request) (string, error) {
	// payload 鐎规矮绠熺拠銉δ侀崸妞惧▏閻劎娈戦弫鐗堝祦缂佹挻鐎妴?
	type payload struct {
		Password string `json:"password"`
	}
	var body payload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return "", errors.New("invalid json body")
	}
	pwd := strings.TrimSpace(body.Password)
	if pwd == "" {
		return "", errors.New("password is required")
	}
	return pwd, nil
}

// writeJSON 统一输出 JSON 响应并设置状态码。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
