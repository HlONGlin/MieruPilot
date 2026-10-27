package manager

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"merit/internal/clash"
	"merit/internal/model"
	"merit/internal/proxy"
	"merit/internal/proxy/shadowtls"
)

// clientConfig exports a complete client configuration, including both layers
// of ShadowTLS. A transport URI alone cannot represent that protocol.
func clientConfig(n *model.Node, p *model.Port) (map[string]any, error) {
	host := strings.TrimSpace(n.Address)
	if host == "" {
		host = strings.TrimSpace(n.Domain)
	}
	if host == "" {
		host = strings.TrimSpace(n.SeenIP)
	}
	host = strings.Trim(host, "[]")
	if host == "" {
		return nil, fmt.Errorf("节点地址为空")
	}
	typ := model.NormalizeProxyType(p.ProxyType)
	if typ == model.ProxyTypeNaive {
		u, err := url.Parse(proxy.GenerateShareLink(n, p))
		if err != nil {
			return nil, err
		}
		u.Scheme = p.EffectiveNaiveTransport()
		u.Fragment = ""
		return map[string]any{"listen": "socks://127.0.0.1:10808", "proxy": u.String()}, nil
	}
	out := map[string]any{"type": typ, "tag": "proxy", "server": host, "server_port": p.Port}
	outbounds := []any{out}
	switch typ {
	case model.ProxyTypeAnyTLS:
		out["password"] = p.Password
		tls := map[string]any{"enabled": true, "insecure": true, "utls": map[string]any{"enabled": true, "fingerprint": p.EffectiveTLSFingerprint()}}
		if p.Domain != "" {
			tls["server_name"] = p.Domain
		}
		out["tls"] = tls
	case model.ProxyTypeShadowTLS:
		if p.BackendPassword == "" {
			return nil, fmt.Errorf("缺少后端 Shadowsocks 密码，请编辑信息后重新同步")
		}
		method := p.BackendCipher
		if method == "" {
			method = clash.DefaultShadowTLSBackendCipher
		}
		out["type"] = "shadowsocks"
		out["method"] = method
		out["password"] = p.BackendPassword
		out["detour"] = "shadowtls-transport"
		outbounds = append(outbounds, map[string]any{"type": "shadowtls", "tag": "shadowtls-transport", "server": host, "server_port": p.Port, "version": 3, "password": p.Password, "tls": map[string]any{"enabled": true, "server_name": shadowtls.HandshakeHost(shadowtls.Handshake(p)), "utls": map[string]any{"enabled": true, "fingerprint": "chrome"}}})
	default:
		return nil, fmt.Errorf("Mieru 请使用 mierus 链接或 mihomo YAML")
	}
	return map[string]any{"log": map[string]any{"level": "warn"}, "inbounds": []any{map[string]any{"type": "mixed", "tag": "mixed-in", "listen": "127.0.0.1", "listen_port": 10808}}, "outbounds": outbounds, "route": map[string]any{"final": "proxy"}}, nil
}

func (s *Server) handlePortClientConfig(w http.ResponseWriter, r *http.Request) {
	n := s.store.Node(r.PathValue("id"))
	if n == nil {
		writeJSON(w, 404, map[string]any{"error": "节点不存在"})
		return
	}
	p := n.FindPort(r.PathValue("pid"))
	if p == nil {
		writeJSON(w, 404, map[string]any{"error": "端口不存在"})
		return
	}
	cfg, err := clientConfig(n, p)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=merit-%s-%d.json", model.NormalizeProxyType(p.ProxyType), p.Port))
	writeJSON(w, http.StatusOK, cfg)
}
