#!/usr/bin/env bash
set -euo pipefail

# Replaced by build scripts.
MANIFEST_URL="__MANIFEST_URL__"
BIN_URL_AMD64="__BIN_URL_AMD64__"
BIN_URL_ARM64="__BIN_URL_ARM64__"

DEFAULT_INSTALL_DIR="/LDManager"
SERVICE_NAME="ldmanager"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
BIN_NAME="ldmanager"

INSTALL_DIR="$DEFAULT_INSTALL_DIR"
BIN_PATH="${INSTALL_DIR}/${BIN_NAME}"
CONFIG_PATH="${INSTALL_DIR}/config.yaml"

if [ -t 1 ]; then
  C_RESET='\033[0m'
  C_BOLD='\033[1m'
  C_BLUE='\033[38;5;75m'
  C_CYAN='\033[38;5;81m'
  C_GREEN='\033[38;5;114m'
  C_YELLOW='\033[38;5;221m'
  C_RED='\033[38;5;203m'
  C_GRAY='\033[38;5;246m'
else
  C_RESET=''
  C_BOLD=''
  C_BLUE=''
  C_CYAN=''
  C_GREEN=''
  C_YELLOW=''
  C_RED=''
  C_GRAY=''
fi

log() { printf '%b[ldm]%b %s\n' "$C_CYAN" "$C_RESET" "$1"; }
ok() { printf '%b[ok]%b %s\n' "$C_GREEN" "$C_RESET" "$1"; }
warn() { printf '%b[warn]%b %s\n' "$C_YELLOW" "$C_RESET" "$1"; }
err() { printf '%b[error]%b %s\n' "$C_RED" "$C_RESET" "$1" >&2; }
fail() { err "$1"; exit 1; }

need_cmd() { command -v "$1" >/dev/null 2>&1 || fail "missing command: $1"; }

run_root() {
  if [ "${EUID:-$(id -u)}" -eq 0 ]; then
    "$@"
  else
    need_cmd sudo
    sudo "$@"
  fi
}

run_root_sh() {
  local script="$1"
  if [ "${EUID:-$(id -u)}" -eq 0 ]; then
    bash -lc "$script"
  else
    need_cmd sudo
    sudo bash -lc "$script"
  fi
}

prompt_line() {
  local prompt="$1"
  local default_value="${2:-}"
  local answer=""

  if [ -r /dev/tty ]; then
    if [ -n "$default_value" ]; then
      printf '%b%s%b (default / 默认: %s): ' "$C_BLUE" "$prompt" "$C_RESET" "$default_value" > /dev/tty
    else
      printf '%b%s%b: ' "$C_BLUE" "$prompt" "$C_RESET" > /dev/tty
    fi
    IFS= read -r answer < /dev/tty || true
  fi

  [ -n "$answer" ] || answer="$default_value"
  printf '%s' "$answer"
}

confirm_yes_no() {
  local prompt="$1"
  local default_yes="${2:-yes}"
  local suffix="(Y/n)"
  local answer=""

  [ "$default_yes" = "yes" ] || suffix="(y/N)"

  if [ -r /dev/tty ]; then
    printf '%b%s %s%b: ' "$C_BLUE" "$prompt" "$suffix" "$C_RESET" > /dev/tty
    IFS= read -r answer < /dev/tty || true
  fi

  answer="$(printf '%s' "$answer" | tr '[:upper:]' '[:lower:]')"
  if [ -z "$answer" ]; then
    [ "$default_yes" = "yes" ] && return 0 || return 1
  fi

  case "$answer" in
    y|yes) return 0 ;;
    n|no) return 1 ;;
    *) return 1 ;;
  esac
}

detect_arch() {
  local machine
  machine="$(uname -m | tr '[:upper:]' '[:lower:]')"
  case "$machine" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) fail "unsupported architecture: $machine" ;;
  esac
}

binary_url_for_arch() {
  case "$1" in
    amd64) echo "$BIN_URL_AMD64" ;;
    arm64) echo "$BIN_URL_ARM64" ;;
    *) fail "unsupported architecture: $1" ;;
  esac
}

download_file() {
  local url="$1"
  local out="$2"

  if command -v curl >/dev/null 2>&1; then
    curl -fL --retry 3 --connect-timeout 20 -o "$out" "$url"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -O "$out" "$url"
    return
  fi
  fail "curl or wget is required"
}

ensure_port() {
  case "$1" in
    ''|*[!0-9]*) fail "port must be a number" ;;
  esac

  [ "$1" -ge 1 ] && [ "$1" -le 65535 ] || fail "port must be in 1-65535"
}

service_installed() {
  run_root test -f "$SERVICE_FILE"
}

current_install_dir() {
  if service_installed; then
    run_root_sh "grep -E '^[[:space:]]*WorkingDirectory=' '$SERVICE_FILE' | tail -n1 | sed -E 's/^[^=]+=//' || true"
  fi
}

use_current_install_dir_if_any() {
  local dir
  dir="$(current_install_dir || true)"
  [ -n "$dir" ] && INSTALL_DIR="$dir"
  BIN_PATH="${INSTALL_DIR}/${BIN_NAME}"
  CONFIG_PATH="${INSTALL_DIR}/config.yaml"
}

normalize_version() {
  printf '%s' "$1" | sed -n 's/.*\([0-9][0-9]*\(\.[0-9][0-9]*\)*\).*/\1/p' | head -n1
}

current_binary_version() {
  use_current_install_dir_if_any
  if run_root test -x "$BIN_PATH"; then
    run_root_sh "'$BIN_PATH' --version 2>/dev/null | head -n1 || true"
  fi
}

extract_latest_version_from_manifest() {
  [ -n "$MANIFEST_URL" ] || return 0

  local tmp
  local version
  tmp="$(mktemp)"
  if download_file "$MANIFEST_URL" "$tmp" 2>/dev/null; then
    version="$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$tmp" | head -n1)"
    rm -f "$tmp"
    printf '%s' "$version"
  else
    rm -f "$tmp"
  fi
}

version_is_newer() {
  local local_v
  local remote_v
  local_v="$(normalize_version "$1")"
  remote_v="$(normalize_version "$2")"

  [ -n "$local_v" ] && [ -n "$remote_v" ] || return 1
  [ "$local_v" = "$remote_v" ] && return 1
  [ "$(printf '%s\n%s\n' "$local_v" "$remote_v" | sort -V | tail -n1)" = "$remote_v" ]
}

service_active_state() {
  if ! service_installed; then
    echo "not-installed"
  else
    run_root systemctl is-active "$SERVICE_NAME" 2>/dev/null || true
  fi
}

service_enable_state() {
  if ! service_installed; then
    echo "not-installed"
  else
    run_root systemctl is-enabled "$SERVICE_NAME" 2>/dev/null || true
  fi
}

read_current_port() {
  use_current_install_dir_if_any
  if run_root test -f "$CONFIG_PATH"; then
    run_root_sh "grep -E '^[[:space:]]*listen_port:' '$CONFIG_PATH' | tail -n1 | sed -E 's/.*:[[:space:]]*//' || true"
  else
    echo "3210"
  fi
}

allow_ufw_port() {
  local port="$1"
  if ! command -v ufw >/dev/null 2>&1; then
    warn "ufw not found / 未找到 ufw, skip firewall rule"
    return
  fi
  run_root ufw allow "${port}/tcp" || warn "ufw allow failed"
}

write_default_config() {
  local port="$1"
  run_root mkdir -p "$INSTALL_DIR"
  run_root_sh "cat > '$CONFIG_PATH' <<EOF
listen_host: 0.0.0.0
listen_port: ${port}
base_path: /
trust_proxy_headers: false
disable_https_warning: false
log_retention_count: 10
log_retention_days: 30
log_max_mb: 2048
file_manager:
  enabled: true
  upload_max_mb: 2048
login_protect:
  enabled: true
  max_attempts: 20
  window_seconds: 600
  block_seconds: 600
metrics_refresh_seconds: 2
session_cookie_name: sealpanel_session
session_ttl_hours: 48
session_secret: ""
EOF"
}

replace_listen_port() {
  local port="$1"
  run_root_sh "
set -e
if [ ! -f '$CONFIG_PATH' ]; then exit 1; fi
if grep -Eq '^[[:space:]]*listen_port:' '$CONFIG_PATH'; then
  sed -Ei 's|^[[:space:]]*listen_port:.*$|listen_port: ${port}|' '$CONFIG_PATH'
else
  printf '\nlisten_port: %s\n' '${port}' >> '$CONFIG_PATH'
fi
if grep -Eq '^[[:space:]]*listen_host:' '$CONFIG_PATH'; then
  sed -Ei 's|^[[:space:]]*listen_host:.*$|listen_host: 0.0.0.0|' '$CONFIG_PATH'
else
  printf 'listen_host: 0.0.0.0\n' >> '$CONFIG_PATH'
fi
"
}

install_binary() {
  local arch="$1"
  local url
  local tmp

  url="$(binary_url_for_arch "$arch")"
  [ -n "$url" ] || fail "binary url for $arch is empty"
  tmp="$(mktemp /tmp/ldmanager-bin.XXXXXX)"

  log "downloading ${arch} binary: $url"
  download_file "$url" "$tmp"
  chmod +x "$tmp"

  run_root mkdir -p "$INSTALL_DIR"
  run_root install -m 0755 "$tmp" "$BIN_PATH"
  rm -f "$tmp"
}

write_service_file() {
  run_root_sh "cat > '$SERVICE_FILE' <<EOF
[Unit]
Description=Lorana Dice Manager Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=${INSTALL_DIR}
ExecStart=${BIN_PATH} -config ${CONFIG_PATH}
Restart=on-failure
RestartSec=3
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF"
}

install_cli_links() {
  run_root ln -sf "$BIN_PATH" /usr/local/bin/ldmanager
  if [ -e /usr/local/bin/ldm ] && [ ! -L /usr/local/bin/ldm ]; then
    warn "existing /usr/local/bin/ldm found / 已存在同名命令, 保留原命令, 请使用 ldmanager"
  else
    run_root ln -sf "$BIN_PATH" /usr/local/bin/ldm
  fi
}

show_runtime_summary() {
  use_current_install_dir_if_any

  local arch
  local active
  local enabled
  local local_v
  local remote_v
  local port
  local update_tip

  arch="$(detect_arch)"
  active="$(service_active_state)"
  enabled="$(service_enable_state)"
  local_v="$(current_binary_version || true)"
  remote_v="$(extract_latest_version_from_manifest || true)"
  port="$(read_current_port || true)"

  [ -n "$local_v" ] || local_v="(not installed)"
  [ -n "$remote_v" ] || remote_v="(unknown)"
  [ -n "$port" ] || port="-"

  update_tip=""
  if version_is_newer "$local_v" "$remote_v"; then
    update_tip="${C_YELLOW}update available / 可更新: ${remote_v}${C_RESET}"
  fi

  printf "\n%bLorana's Dice Manager Installer / 安装器%b\n" "$C_BOLD$C_CYAN" "$C_RESET"
  printf '%b------------------------------------------------------------%b\n' "$C_GRAY" "$C_RESET"
  printf '  arch / 架构            : %s\n' "$arch"
  printf '  install dir / 安装目录 : %s\n' "$INSTALL_DIR"
  printf '  service / 服务名       : %s\n' "$SERVICE_NAME"
  printf '  active / 运行状态      : %s\n' "$active"
  printf '  enabled / 开机自启     : %s\n' "$enabled"
  printf '  port / 监听端口        : %s\n' "$port"
  printf '  local ver / 当前版本   : %s\n' "$local_v"
  printf '  remote ver / 最新版本  : %s\n' "$remote_v"
  [ -n "$update_tip" ] && printf '  notice                : %b\n' "$update_tip"
  printf '%b------------------------------------------------------------%b\n\n' "$C_GRAY" "$C_RESET"
}

service_ctl() {
  run_root systemctl daemon-reload
  run_root systemctl "$1" "$SERVICE_NAME"
}

enable_boot() {
  run_root systemctl daemon-reload
  run_root systemctl enable "$SERVICE_NAME"
}

disable_boot() {
  run_root systemctl daemon-reload
  run_root systemctl disable "$SERVICE_NAME"
}

show_status() {
  run_root systemctl status "$SERVICE_NAME" --no-pager || true
}

show_logs() {
  local mode
  mode="$(prompt_line 'Logs mode / 日志模式: 1=tail 最近200行 2=follow 实时跟随' '1')"
  if [ "$mode" = "2" ]; then
    run_root journalctl -u "$SERVICE_NAME" -f -o cat
  else
    run_root journalctl -u "$SERVICE_NAME" -n 200 --no-pager -o cat
  fi
}

ensure_installed() {
  service_installed || fail "ldmanager is not installed yet / 尚未安装"
}

install_flow() {
  local arch
  local port
  local dir

  confirm_yes_no 'Run install/repair now? / 现在执行安装或修复?' yes || fail 'cancelled / 已取消'

  arch="$(detect_arch)"
  dir="$(prompt_line 'Install directory / 安装目录' "$DEFAULT_INSTALL_DIR")"
  [ -n "$dir" ] || fail 'install dir cannot be empty / 安装目录不能为空'

  INSTALL_DIR="$dir"
  BIN_PATH="${INSTALL_DIR}/${BIN_NAME}"
  CONFIG_PATH="${INSTALL_DIR}/config.yaml"

  port="$(prompt_line 'Listen port / 监听端口' '3210')"
  ensure_port "$port"

  install_binary "$arch"
  if ! run_root test -f "$CONFIG_PATH"; then
    write_default_config "$port"
  else
    replace_listen_port "$port"
  fi

  write_service_file
  install_cli_links
  allow_ufw_port "$port"

  run_root systemctl daemon-reload
  run_root systemctl enable "$SERVICE_NAME"
  run_root systemctl restart "$SERVICE_NAME"

  ok "installed / 安装完成. web ui: http://<host>:${port}/"
}

update_flow() {
  ensure_installed
  use_current_install_dir_if_any

  install_binary "$(detect_arch)"
  install_cli_links
  run_root systemctl daemon-reload
  run_root systemctl restart "$SERVICE_NAME"

  ok "binary updated and service restarted / 已更新并重启服务"
}

port_flow() {
  ensure_installed
  use_current_install_dir_if_any

  local current
  local port

  current="$(read_current_port || true)"
  [ -n "$current" ] || current='3210'

  port="$(prompt_line 'New listen port / 新监听端口' "$current")"
  ensure_port "$port"

  replace_listen_port "$port"
  allow_ufw_port "$port"
  run_root systemctl restart "$SERVICE_NAME"

  ok "listen port updated / 端口已更新: $port"
}

relocate_flow() {
  ensure_installed

  local old_dir
  local new_dir
  local mode

  old_dir="$(current_install_dir || true)"
  [ -n "$old_dir" ] || old_dir="$DEFAULT_INSTALL_DIR"

  new_dir="$(prompt_line 'New install directory / 新安装目录' "$DEFAULT_INSTALL_DIR")"
  [ -n "$new_dir" ] || fail 'new install dir cannot be empty / 新安装目录不能为空'

  mode="$(prompt_line 'Move existing files? / 是否迁移旧文件? 1=yes 2=no' '1')"

  run_root mkdir -p "$new_dir"
  if [ "$mode" = '1' ] && [ "$new_dir" != "$old_dir" ]; then
    if command -v rsync >/dev/null 2>&1; then
      run_root rsync -a --delete "$old_dir/" "$new_dir/"
    else
      run_root cp -a "$old_dir/." "$new_dir/"
    fi
  fi

  INSTALL_DIR="$new_dir"
  BIN_PATH="${INSTALL_DIR}/${BIN_NAME}"
  CONFIG_PATH="${INSTALL_DIR}/config.yaml"

  if ! run_root test -x "$BIN_PATH"; then
    install_binary "$(detect_arch)"
  fi
  if ! run_root test -f "$CONFIG_PATH"; then
    write_default_config '3210'
  fi

  write_service_file
  install_cli_links
  run_root systemctl daemon-reload
  run_root systemctl restart "$SERVICE_NAME"

  ok "install directory switched / 安装目录已切换: $new_dir"
}

show_menu() {
  cat <<'EOF'
[Install / Update 安装与更新]
  1) Install / Repair 安装或修复
  2) Update binary 更新程序
  3) Change listen port 修改监听端口
  4) Change install directory 修改安装目录

[Service Control 服务控制]
  5) Start service 启动服务
  6) Stop service 停止服务
  7) Restart service 重启服务
  8) Enable auto-start 启用开机自启
  9) Disable auto-start 禁用开机自启

[Diagnostics 诊断]
 10) Service status 查看状态
 11) Service logs 查看日志
 12) Refresh summary 刷新摘要

  0) Exit 退出
EOF
}

main() {
  need_cmd grep
  need_cmd sed

  while true; do
    show_runtime_summary
    show_menu
    action="$(prompt_line 'Select action / 请选择操作' '0')"
    case "$action" in
      1) install_flow ;;
      2) update_flow ;;
      3) port_flow ;;
      4) relocate_flow ;;
      5) ensure_installed; service_ctl start; ok 'started / 已启动' ;;
      6) ensure_installed; service_ctl stop; ok 'stopped / 已停止' ;;
      7) ensure_installed; service_ctl restart; ok 'restarted / 已重启' ;;
      8) ensure_installed; enable_boot; ok 'auto-start enabled / 已启用开机自启' ;;
      9) ensure_installed; disable_boot; ok 'auto-start disabled / 已禁用开机自启' ;;
      10) ensure_installed; show_status ;;
      11) ensure_installed; show_logs ;;
      12) : ;;
      0) exit 0 ;;
      *) warn 'invalid action / 无效选项' ;;
    esac

    if [ -r /dev/tty ]; then
      printf '\nPress Enter to continue... / 回车继续...' > /dev/tty
      IFS= read -r _ < /dev/tty || true
    fi
  done
}

main "$@"
