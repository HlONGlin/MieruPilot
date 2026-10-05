#!/usr/bin/env bash

set -u

REPO_RAW="https://github.com/HlONGlin/MieruPilot/raw/main/dist"
INSTALL_DIR="/opt/merit"
DATA_DIR="/var/lib/merit"
SERVICE_FILE="/etc/systemd/system/merit-manager.service"
PANEL_PATH_FILE="$DATA_DIR/panel-path"

log() {
    printf '\n%s\n' "$1"
}

die() {
    printf 'Error: %s\n' "$1" >&2
    exit 1
}

require_root() {
    [ "$(id -u)" -eq 0 ] || die "Please run this script as root."
}

require_systemd() {
    command -v systemctl >/dev/null 2>&1 || die "systemd/systemctl is required."
}

download() {
    url="$1"
    output="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fL --retry 3 --connect-timeout 10 "$url" -o "$output"
    elif command -v wget >/dev/null 2>&1; then
        wget -O "$output" "$url"
    else
        die "curl or wget is required. Install it first."
    fi
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) printf 'amd64' ;;
        aarch64|arm64) printf 'arm64' ;;
        *) die "Unsupported architecture: $(uname -m)" ;;
    esac
}

detect_host() {
    host=""
    if command -v curl >/dev/null 2>&1; then
        host="$(curl -4 -fsS --max-time 5 https://api.ipify.org 2>/dev/null || true)"
    fi
    [ -n "$host" ] || host="$(hostname -I 2>/dev/null | awk '{print $1}')"
    [ -n "$host" ] || host="<服务器IP>"
    printf '%s' "$host"
}

ensure_panel_path() {
    if [ -s "$PANEL_PATH_FILE" ]; then
        tr -d '\r\n' < "$PANEL_PATH_FILE"
        return
    fi
    panel_path="/panel/$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')"
    printf '%s\n' "$panel_path" > "$PANEL_PATH_FILE"
    chmod 0600 "$PANEL_PATH_FILE"
    printf '%s' "$panel_path"
}

install_manager() {
    require_root
    require_systemd

    arch="$(detect_arch)"
    mkdir -p "$INSTALL_DIR" "$DATA_DIR"

    was_running=0
    if systemctl is-active --quiet merit-manager.service; then
        was_running=1
        log "Stopping merit Manager before update..."
        systemctl stop merit-manager.service || die "Failed to stop merit Manager."
    fi

    log "Downloading merit binaries for $arch..."
    manager_tmp="$(mktemp "$INSTALL_DIR/.merit-manager.XXXXXX")"
    agent_tmp="$(mktemp "$INSTALL_DIR/.merit-agent.XXXXXX")"
    if ! download "$REPO_RAW/merit-manager-linux-$arch" "$manager_tmp" || \
       ! download "$REPO_RAW/merit-agent-linux-$arch" "$agent_tmp"; then
        rm -f "$manager_tmp" "$agent_tmp"
        if [ "$was_running" -eq 1 ]; then
            systemctl start merit-manager.service || true
        fi
        die "Download failed. Existing installation was kept."
    fi
    chmod 0755 "$manager_tmp" "$agent_tmp"
    mv -f "$manager_tmp" "$INSTALL_DIR/merit-manager"
    mv -f "$agent_tmp" "$INSTALL_DIR/merit-agent"

    port="3000"
    if [ -f "$SERVICE_FILE" ]; then
        current_port="$(sed -n 's/.*--addr :\([0-9][0-9]*\).*/\1/p' "$SERVICE_FILE" | head -n 1)"
        [ -n "$current_port" ] && port="$current_port"
    fi
    printf 'Manager port [%s]: ' "$port"
    read -r input_port
    [ -n "$input_port" ] && port="$input_port"
    case "$port" in
        ''|*[!0-9]*) die "Port must be a number." ;;
    esac

    panel_path="$(ensure_panel_path)"

    cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=merit Manager
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/merit-manager --addr :$port --agent-dir $INSTALL_DIR --data $DATA_DIR/merit.json --panel-path $panel_path
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable --now merit-manager.service
    host="$(detect_host)"
    log "安装完成，管理面板地址："
    printf 'http://%s:%s%s\n' "$host" "$port" "$panel_path"
    log "请复制上面的完整地址，首次打开后设置管理员账号和密码。"
}

start_manager() {
    require_root
    require_systemd
    systemctl start merit-manager.service
    systemctl --no-pager --full status merit-manager.service
}

stop_manager() {
    require_root
    require_systemd
    systemctl stop merit-manager.service
    log "merit Manager stopped."
}

reset_admin() {
    require_root
    require_systemd
    systemctl stop merit-manager.service 2>/dev/null || true
    "$INSTALL_DIR/merit-manager" --data "$DATA_DIR/merit.json" --reset-admin
    systemctl start merit-manager.service
    log "管理员账号已重置，请使用新账号登录。"
}

uninstall_manager() {
    require_root
    require_systemd
    systemctl disable --now merit-manager.service 2>/dev/null || true
    rm -f "$SERVICE_FILE"
    systemctl daemon-reload
    rm -rf "$INSTALL_DIR" "$DATA_DIR"
    log "merit Manager uninstalled."
}

show_menu() {
    clear 2>/dev/null || true
    cat <<'EOF'
====================================
       merit Manager 一键管理工具
====================================
1. 安装或更新 Manager
2. 启动 Manager
3. 停止 Manager
4. 卸载 Manager
5. 重置管理员账号（保留节点数据）
0. 退出
====================================
EOF
    printf '请选择 [0-5]: '
}

require_root

while true; do
    show_menu
    read -r choice
    case "$choice" in
        1) install_manager ;;
        2) start_manager ;;
        3) stop_manager ;;
        4) uninstall_manager ;;
        5) reset_admin ;;
        0) exit 0 ;;
        *) log "无效选择。" ;;
    esac
    printf '\n按回车键继续...'
    read -r _
done
