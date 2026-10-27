package model

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// ValidatePortReservations reserves managed TCP backends even while disabled,
// so re-enabling a port cannot introduce an already persisted bind conflict.
func ValidatePortReservations(n *Node) error {
	for _, backend := range n.Ports {
		if backend == nil || NormalizeProxyType(backend.ProxyType) != ProxyTypeShadowTLS || !backend.BackendManaged {
			continue
		}
		host, portText, err := net.SplitHostPort(strings.TrimSpace(backend.BackendAddr))
		ip := net.ParseIP(host)
		port, parseErr := strconv.Atoi(portText)
		if err != nil || ip == nil || !ip.IsLoopback() || parseErr != nil || port < 1 || port > 65535 {
			return fmt.Errorf("ShadowTLS 托管后端地址无效: %s", backend.BackendAddr)
		}
		for _, other := range n.Ports {
			if other == nil {
				continue
			}
			udp := NormalizeProxyType(other.ProxyType) == ProxyTypeMieru && strings.EqualFold(other.Protocol, ProtocolUDP)
			if !udp && other.Port == port {
				return fmt.Errorf("TCP 端口 %d 已保留给 ShadowTLS 托管后端 %s", port, backend.ID)
			}
			if other == backend {
				continue
			}
			h, p, splitErr := net.SplitHostPort(strings.TrimSpace(other.BackendAddr))
			otherPort, _ := strconv.Atoi(p)
			if otherIP := net.ParseIP(h); splitErr == nil && otherIP != nil && otherIP.IsLoopback() && otherPort == port {
				return fmt.Errorf("ShadowTLS 后端 TCP 端口 %d 被重复分配", port)
			}
		}
	}
	return nil
}
