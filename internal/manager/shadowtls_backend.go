package manager

import (
	"fmt"
	"strings"

	"merit/internal/mieru"
	"merit/internal/model"
	"merit/internal/proxy/shadowsocks"
)

// finalizeShadowTLSBackend decides whether a ShadowTLS port uses a managed or a
// manual backend and fills in everything a managed backend needs.
//
// A port is managed when no backend address is given. The Manager then owns the
// loopback address, the password and the cipher, so the node and the client
// subscriptions are always generated from the same values and a re-sync never
// changes them. A manually entered address is kept as is and never deployed.
//
//   - wasManaged and prevAddr describe the stored port before this request.
//   - addrProvided tells whether the request carried a backend address at all.
//   - credsGiven tells whether the request carried a backend password.
func finalizeShadowTLSBackend(n *model.Node, p *model.Port, wasManaged bool, prevAddr string, addrProvided, credsGiven bool) error {
	if model.NormalizeProxyType(p.ProxyType) != model.ProxyTypeShadowTLS {
		p.BackendManaged = false
		return nil
	}

	addr := strings.TrimSpace(p.BackendAddr)
	var managed bool
	switch {
	case addrProvided && addr == "":
		managed = true
	case addrProvided:
		// The panel sends the stored address back unchanged on every edit.
		managed = wasManaged && addr == prevAddr
	default:
		managed = wasManaged || addr == ""
	}
	p.BackendManaged = managed
	if !managed {
		return nil
	}

	if !wasManaged {
		// Credentials of a hand-run server must not leak into a managed one.
		p.BackendAddr = ""
		if !credsGiven {
			p.BackendPassword = ""
			p.BackendCipher = ""
		}
	}
	if strings.TrimSpace(p.BackendAddr) == "" {
		allocated, err := allocateBackendAddr(n, p)
		if err != nil {
			return err
		}
		p.BackendAddr = allocated
	}
	if strings.TrimSpace(p.BackendPassword) == "" {
		p.BackendPassword = mieru.GeneratePassword()
	}
	if strings.TrimSpace(p.BackendCipher) == "" {
		p.BackendCipher = shadowsocks.DefaultCipher
	}
	return shadowsocks.Validate(p.BackendAddr, p.BackendPassword, p.BackendCipher)
}

// allocateBackendAddr picks the lowest loopback port that no other port of the
// node uses, neither as a listen port nor as a backend address.
func allocateBackendAddr(n *model.Node, self *model.Port) (string, error) {
	used := map[int]bool{self.Port: true}
	for _, other := range n.Ports {
		if other == self {
			continue
		}
		used[other.Port] = true
		if port, err := shadowsocks.LoopbackPort(other.BackendAddr); err == nil {
			used[port] = true
		}
	}
	for port := shadowsocks.BackendPortMin; port <= shadowsocks.BackendPortMax; port++ {
		if !used[port] {
			return fmt.Sprintf("127.0.0.1:%d", port), nil
		}
	}
	return "", fmt.Errorf("没有可用的后端端口：%d-%d 已全部被该节点占用", shadowsocks.BackendPortMin, shadowsocks.BackendPortMax)
}
