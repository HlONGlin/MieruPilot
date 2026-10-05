#!/usr/bin/env bash

set -u

REPO_RAW="https://raw.githubusercontent.com/HlONGlin/MieruPilot/master/dist"
INSTALL_DIR="/opt/merit"
DATA_DIR="/var/lib/merit"
SERVICE_FILE="/etc/systemd/system/merit-manager.service"

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
        curl -fL --retry 3 --connect-timeout 10 "$url" -o "$output" || die "Download failed: $url"
    elif command -v wget >/dev/null 2>&1; then
        wget -O "$output" "$url" || die "Download failed: $url"
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

install_manager() {
    require_root
    require_systemd

    arch="$(detect_arch)"
    mkdir -p "$INSTALL_DIR" "$DATA_DIR"

    log "Downloading merit binaries for $arch..."
    download "$REPO_RAW/merit-manager-linux-$arch" "$INSTALL_DIR/merit-manager"
    download "$REPO_RAW/merit-agent-linux-$arch" "$INSTALL_DIR/merit-agent"
    chmod 0755 "$INSTALL_DIR/merit-manager" "$INSTALL_DIR/merit-agent"

    port="3000"
    printf 'Manager port [3000]: '
    read -r input_port
    [ -n "$input_port" ] && port="$input_port"
    case "$port" in
        ''|*[!0-9]*) die "Port must be a number." ;;
    esac

    cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=merit Manager
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/merit-manager --addr :$port --agent-dir $INSTALL_DIR --data $DATA_DIR/merit.json
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable --now merit-manager.service
    log "Installed and started. Open http://<server-ip>:$port"
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
0. 退出
====================================
EOF
    printf '请选择 [0-4]: '
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
        0) exit 0 ;;
        *) log "无效选择。" ;;
    esac
    printf '\n按回车键继续...'
    read -r _
done
