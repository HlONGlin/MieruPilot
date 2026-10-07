package netcheck

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"merit/internal/model"
)

func TestCheckSOCKS5EgressWithAuthentication(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go serveSOCKS5TestProxy(listener, t)
	_, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)

	ip, latency, err := CheckSOCKS5Egress(model.EgressProxy{Host: "127.0.0.1", Port: port, Username: "test-user", Password: "test-pass"})
	if err != nil {
		t.Fatal(err)
	}
	if ip != "203.0.113.9" {
		t.Fatalf("exit IP = %q", ip)
	}
	if latency < 0 {
		t.Fatalf("latency must not be negative: %d", latency)
	}
}

func serveSOCKS5TestProxy(listener net.Listener, t *testing.T) {
	t.Helper()
	conn, err := listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	var header [2]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return
	}
	_, _ = conn.Write([]byte{5, 2})
	var authHeader [2]byte
	if _, err := io.ReadFull(conn, authHeader[:]); err != nil {
		return
	}
	user := make([]byte, int(authHeader[1]))
	if _, err := io.ReadFull(conn, user); err != nil {
		return
	}
	var passLen [1]byte
	if _, err := io.ReadFull(conn, passLen[:]); err != nil {
		return
	}
	pass := make([]byte, int(passLen[0]))
	if _, err := io.ReadFull(conn, pass); err != nil {
		return
	}
	if string(user) != "test-user" || string(pass) != "test-pass" {
		_, _ = conn.Write([]byte{1, 1})
		return
	}
	_, _ = conn.Write([]byte{1, 0})
	var request [5]byte
	if _, err := io.ReadFull(conn, request[:]); err != nil {
		return
	}
	if request[0] != 5 || request[1] != 1 || request[3] != 3 {
		return
	}
	domain := make([]byte, int(request[4]))
	if _, err := io.ReadFull(conn, domain); err != nil {
		return
	}
	var destPort [2]byte
	if _, err := io.ReadFull(conn, destPort[:]); err != nil {
		return
	}
	if string(domain) != "api.ipify.org" || binary.BigEndian.Uint16(destPort[:]) != 80 {
		return
	}
	_, _ = conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0x1f, 0x90})
	reader := bufio.NewReader(conn)
	if _, err := http.ReadRequest(reader); err != nil {
		return
	}
	_, _ = io.WriteString(conn, "HTTP/1.1 200 OK\r\nContent-Length: 12\r\nConnection: close\r\n\r\n203.0.113.9\n")
}
