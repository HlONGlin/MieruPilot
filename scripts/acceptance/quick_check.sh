#!/usr/bin/env bash
# scripts/acceptance/quick_check.sh
# 快速验收脚本 - 5 分钟内验证所有协议
#
# 使用方法:
#   1. 复制 test_config.env.example 为 test_config.env
#   2. 填写实际的节点信息和凭据
#   3. 运行: ./scripts/acceptance/quick_check.sh

set -euo pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 加载配置
CONFIG_FILE="$(dirname "$0")/test_config.env"
if [[ ! -f "$CONFIG_FILE" ]]; then
    echo -e "${RED}❌ 配置文件不存在: $CONFIG_FILE${NC}"
    echo -e "${YELLOW}请复制 test_config.env.example 并填写实际配置${NC}"
    exit 1
fi

source "$CONFIG_FILE"

# 验证必需配置
if [[ -z "${NODE_IP:-}" ]]; then
    echo -e "${RED}❌ NODE_IP 未配置${NC}"
    exit 1
fi

# 统计
PASS=0
FAIL=0
SKIP=0

# 辅助函数
print_test() {
    echo -e "${BLUE}🧪 测试: $1${NC}"
}

print_pass() {
    echo -e "${GREEN}✅ $1${NC}"
    ((PASS++))
}

print_fail() {
    echo -e "${RED}❌ $1${NC}"
    ((FAIL++))
}

print_skip() {
    echo -e "${YELLOW}⏭  $1${NC}"
    ((SKIP++))
}

check_port_tcp() {
    local host=$1
    local port=$2
    timeout 3 bash -c "cat < /dev/null > /dev/tcp/$host/$port" 2>/dev/null
}

check_port_udp() {
    local host=$1
    local port=$2
    # UDP 探测需要 nc (netcat)
    if command -v nc >/dev/null 2>&1; then
        echo -n "test" | nc -u -w2 "$host" "$port" >/dev/null 2>&1
    else
        return 2  # nc 不可用
    fi
}

# 开始测试
echo ""
echo "========================================="
echo "  UnifiedProxy 快速验收"
echo "========================================="
echo ""
echo "目标节点: $NODE_IP"
echo "测试时间: $(date '+%Y-%m-%d %H:%M:%S')"
echo ""

# ===================================
# 1. Mieru TCP 端口测试
# ===================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
print_test "Mieru TCP 端口连通性"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ -n "${MIERU_TCP_PORTS:-}" ]]; then
    IFS=',' read -ra PORTS <<< "$MIERU_TCP_PORTS"
    for port in "${PORTS[@]}"; do
        port=$(echo "$port" | xargs)  # trim
        if check_port_tcp "$NODE_IP" "$port"; then
            print_pass "Mieru TCP:$port 端口开放"
        else
            print_fail "Mieru TCP:$port 端口不可达"
        fi
    done
else
    print_skip "Mieru TCP 端口未配置 (MIERU_TCP_PORTS)"
fi

echo ""

# ===================================
# 2. Mieru UDP 端口测试
# ===================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
print_test "Mieru UDP 端口连通性"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ -n "${MIERU_UDP_PORTS:-}" ]]; then
    IFS=',' read -ra PORTS <<< "$MIERU_UDP_PORTS"
    for port in "${PORTS[@]}"; do
        port=$(echo "$port" | xargs)
        result=$(check_port_udp "$NODE_IP" "$port"; echo $?)
        if [[ $result -eq 0 ]]; then
            print_pass "Mieru UDP:$port 端口开放"
        elif [[ $result -eq 2 ]]; then
            print_skip "Mieru UDP:$port (nc 命令不可用,跳过 UDP 测试)"
        else
            print_fail "Mieru UDP:$port 端口不可达"
        fi
    done
else
    print_skip "Mieru UDP 端口未配置 (MIERU_UDP_PORTS)"
fi

echo ""

# ===================================
# 3. AnyTLS 测试
# ===================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
print_test "AnyTLS 连通性和认证"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ -n "${ANYTLS_PORT:-}" ]]; then
    # 端口连通性
    if check_port_tcp "$NODE_IP" "$ANYTLS_PORT"; then
        print_pass "AnyTLS:$ANYTLS_PORT 端口开放"

        # TODO: 添加实际的代理转发测试
        # 需要 anytls 客户端或支持 anytls:// 协议的工具
        if [[ -n "${ANYTLS_PASSWORD:-}" ]]; then
            echo "  ℹ️  认证测试需要 AnyTLS 客户端,已跳过"
        fi
    else
        print_fail "AnyTLS:$ANYTLS_PORT 端口不可达"
    fi
else
    print_skip "AnyTLS 端口未配置 (ANYTLS_PORT)"
fi

echo ""

# ===================================
# 4. ShadowTLS 测试
# ===================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
print_test "ShadowTLS v3 连通性"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ -n "${SHADOWTLS_PORT:-}" ]]; then
    if check_port_tcp "$NODE_IP" "$SHADOWTLS_PORT"; then
        print_pass "ShadowTLS:$SHADOWTLS_PORT 端口开放"

        # TODO: 添加 SS 后端测试
        echo "  ℹ️  完整链路测试需要 Shadowsocks 客户端,已跳过"
    else
        print_fail "ShadowTLS:$SHADOWTLS_PORT 端口不可达"
    fi
else
    print_skip "ShadowTLS 端口未配置 (SHADOWTLS_PORT)"
fi

echo ""

# ===================================
# 5. NaïveProxy 测试
# ===================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
print_test "NaïveProxy 连通性和认证"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ -n "${NAIVE_PORT:-443}" ]]; then
    NAIVE_PORT="${NAIVE_PORT:-443}"

    # 端口连通性
    if check_port_tcp "$NODE_IP" "$NAIVE_PORT"; then
        print_pass "NaïveProxy:$NAIVE_PORT 端口开放"

        # TLS 握手测试
        if command -v openssl >/dev/null 2>&1; then
            if echo | timeout 3 openssl s_client -connect "$NODE_IP:$NAIVE_PORT" -servername "${NAIVE_DOMAIN:-$NODE_IP}" 2>/dev/null | grep -q "BEGIN CERTIFICATE"; then
                print_pass "NaïveProxy TLS 握手成功"

                # 检查证书有效期
                cert_expiry=$(echo | timeout 3 openssl s_client -connect "$NODE_IP:$NAIVE_PORT" -servername "${NAIVE_DOMAIN:-$NODE_IP}" 2>/dev/null | openssl x509 -noout -enddate 2>/dev/null | cut -d= -f2)
                if [[ -n "$cert_expiry" ]]; then
                    echo "  ℹ️  证书到期时间: $cert_expiry"
                fi
            else
                print_fail "NaïveProxy TLS 握手失败"
            fi
        else
            print_skip "openssl 命令不可用,跳过 TLS 测试"
        fi

        # TODO: 添加 HTTPS CONNECT 认证测试
        # 需要 NaïveProxy 客户端或支持 HTTPS proxy 的工具
        if [[ -n "${NAIVE_USER:-}" ]] && [[ -n "${NAIVE_PASS:-}" ]]; then
            echo "  ℹ️  认证测试需要 NaïveProxy 客户端,已跳过"
        fi
    else
        print_fail "NaïveProxy:$NAIVE_PORT 端口不可达"
    fi
else
    print_skip "NaïveProxy 端口未配置 (NAIVE_PORT)"
fi

echo ""

# ===================================
# 6. Manager API 测试
# ===================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
print_test "Manager API 健康检查"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ -n "${MANAGER_URL:-}" ]]; then
    # 健康检查端点
    if curl -sS --connect-timeout 3 --max-time 5 "${MANAGER_URL}/api/health" >/dev/null 2>&1; then
        print_pass "Manager API 可达"
    else
        print_fail "Manager API 不可达"
    fi

    # 订阅生成测试
    if [[ -n "${SUBSCRIPTION_TOKEN:-}" ]]; then
        subscription_url="${MANAGER_URL}/api/clash/subscribe?token=${SUBSCRIPTION_TOKEN}"

        if curl -sS --connect-timeout 3 --max-time 5 "$subscription_url" | grep -q "proxies:"; then
            print_pass "Clash 订阅生成成功"

            # 统计代理数量
            proxy_count=$(curl -sS "$subscription_url" | grep -c "^  - name:" || true)
            echo "  ℹ️  订阅包含 $proxy_count 个代理节点"
        else
            print_fail "Clash 订阅生成失败"
        fi
    else
        print_skip "订阅令牌未配置 (SUBSCRIPTION_TOKEN)"
    fi
else
    print_skip "Manager URL 未配置 (MANAGER_URL)"
fi

echo ""

# ===================================
# 汇总
# ===================================
echo "========================================="
echo "  测试结果汇总"
echo "========================================="
echo ""
echo -e "✅ 通过: ${GREEN}$PASS${NC}"
echo -e "❌ 失败: ${RED}$FAIL${NC}"
echo -e "⏭  跳过: ${YELLOW}$SKIP${NC}"
echo ""

if [[ $FAIL -eq 0 ]]; then
    echo -e "${GREEN}🎉 所有测试通过!${NC}"
    exit 0
else
    echo -e "${RED}⚠️  存在 $FAIL 项失败,请检查配置和服务状态${NC}"
    echo ""
    echo "排查建议:"
    echo "  1. 检查防火墙规则: ufw status / firewall-cmd --list-ports"
    echo "  2. 检查服务状态: systemctl status merit-agent"
    echo "  3. 查看日志: journalctl -u merit-agent -f"
    echo "  4. 验证端口监听: ss -tlnp | grep <port>"
    exit 1
fi
