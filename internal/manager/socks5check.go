package manager

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"merit/internal/model"
)

const socks5TestTarget = "api.ipify.org:80"

func checkSOCKS5Egress(proxy model.EgressProxy) (string, int64, error) {
	started := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(proxy.Host, strconv.Itoa(proxy.Port)), 5*time.Second)
	if err != nil {
		return "", 0, fmt.Errorf("无法连接 SOCKS5 落地机: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	methods := []byte{0}
	if proxy.Username != "" || proxy.Password != "" {
		methods = []byte{0, 2}
	}
	if _, err := conn.Write(append([]byte{5, byte(len(methods))}, methods...)); err != nil {
		return "", 0, fmt.Errorf("SOCKS5 方法协商写入失败: %w", err)
	}
	var selection [2]byte
	if _, err := io.ReadFull(conn, selection[:]); err != nil {
		return "", 0, fmt.Errorf("SOCKS5 方法协商读取失败: %w", err)
	}
	if selection[0] != 5 || selection[1] == 0xff {
		return "", 0, errors.New("SOCKS5 落地机不接受支持的认证方式")
	}
	switch selection[1] {
	case 0:
	case 2:
		if proxy.Username == "" && proxy.Password == "" {
			return "", 0, errors.New("落地机要求用户名密码认证，但尚未配置凭据")
		}
		if len(proxy.Username) > 255 || len(proxy.Password) > 255 {
			return "", 0, errors.New("SOCKS5 用户名或密码超过协议长度限制")
		}
		auth := []byte{1, byte(len(proxy.Username))}
		auth = append(auth, proxy.Username...)
		auth = append(auth, byte(len(proxy.Password)))
		auth = append(auth, proxy.Password...)
		if _, err := conn.Write(auth); err != nil {
			return "", 0, fmt.Errorf("SOCKS5 认证请求失败: %w", err)
		}
		var authReply [2]byte
		if _, err := io.ReadFull(conn, authReply[:]); err != nil {
			return "", 0, fmt.Errorf("SOCKS5 认证响应失败: %w", err)
		}
		if authReply[0] != 1 || authReply[1] != 0 {
			return "", 0, errors.New("SOCKS5 用户名或密码认证失败")
		}
	default:
		return "", 0, fmt.Errorf("SOCKS5 选择了不支持的认证方式: %d", selection[1])
	}

	if _, err := conn.Write([]byte{5, 1, 0, 3, byte(len("api.ipify.org"))}); err != nil {
		return "", 0, fmt.Errorf("SOCKS5 CONNECT 请求失败: %w", err)
	}
	if _, err := io.WriteString(conn, "api.ipify.org"); err != nil {
		return "", 0, fmt.Errorf("SOCKS5 CONNECT 域名写入失败: %w", err)
	}
	if _, err := conn.Write([]byte{0, 80}); err != nil {
		return "", 0, fmt.Errorf("SOCKS5 CONNECT 端口写入失败: %w", err)
	}
	var reply [4]byte
	if _, err := io.ReadFull(conn, reply[:]); err != nil {
		return "", 0, fmt.Errorf("SOCKS5 CONNECT 响应失败: %w", err)
	}
	if reply[0] != 5 || reply[1] != 0 {
		return "", 0, fmt.Errorf("SOCKS5 目标连接失败，响应代码 %d", reply[1])
	}
	if err := discardSOCKS5Address(conn, reply[3]); err != nil {
		return "", 0, fmt.Errorf("读取 SOCKS5 CONNECT 地址失败: %w", err)
	}

	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: api.ipify.org\r\nConnection: close\r\nUser-Agent: merit-egress-check\r\n\r\n"); err != nil {
		return "", 0, fmt.Errorf("通过落地机发送出口检测请求失败: %w", err)
	}
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, &http.Request{Method: http.MethodGet})
	if err != nil {
		return "", 0, fmt.Errorf("出口检测 HTTP 响应失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return "", 0, fmt.Errorf("读取出口 IP 失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("出口检测服务返回 HTTP %d", resp.StatusCode)
	}
	ip := strings.TrimSpace(string(body))
	if ip == "" || strings.ContainsAny(ip, "\r\n ") {
		return "", 0, errors.New("出口检测未返回有效 IP")
	}
	return ip, time.Since(started).Milliseconds(), nil
}

func discardSOCKS5Address(conn net.Conn, atyp byte) error {
	switch atyp {
	case 1:
		_, err := io.CopyN(io.Discard, conn, 4+2)
		return err
	case 4:
		_, err := io.CopyN(io.Discard, conn, 16+2)
		return err
	case 3:
		var length [1]byte
		if _, err := io.ReadFull(conn, length[:]); err != nil {
			return err
		}
		_, err := io.CopyN(io.Discard, conn, int64(length[0])+2)
		return err
	default:
		return fmt.Errorf("未知地址类型 %d", atyp)
	}
}
