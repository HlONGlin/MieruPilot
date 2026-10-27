package naive

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"merit/internal/model"
)

func TestDownloadURLOnlySupportsAmd64(t *testing.T) {
	asset, name, err := DownloadURL("amd64")
	if err != nil {
		t.Fatalf("DownloadURL(amd64) error: %v", err)
	}
	if name != ArchiveName {
		t.Errorf("DownloadURL(amd64) name = %q, want %q", name, ArchiveName)
	}
	if !strings.Contains(asset, DefaultVersion) || !strings.Contains(asset, ArchiveName) {
		t.Errorf("DownloadURL(amd64) = %q, want it to contain version and asset name", asset)
	}
	if !strings.HasPrefix(asset, "https://github.com/klzgrad/forwardproxy/releases/download/") {
		t.Errorf("DownloadURL(amd64) = %q, want the forwardproxy release URL", asset)
	}

	for _, arch := range []string{"arm64", "arm", "386", "riscv64"} {
		if _, _, err := DownloadURL(arch); err == nil {
			t.Errorf("DownloadURL(%s) unexpectedly succeeded; upstream only ships linux/amd64", arch)
		}
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		port    *model.Port
		wantErr bool
	}{
		{
			name: "auto acme on 443 valid",
			port: &model.Port{Port: 443, Domain: "example.com", AutoACME: true},
		},
		{
			name:    "auto acme on non 443 invalid",
			port:    &model.Port{Port: 8443, Domain: "example.com", AutoACME: true},
			wantErr: true,
		},
		{
			name: "manual certificate valid",
			port: &model.Port{Port: 8443, Domain: "example.com", TLSCert: "cert", TLSKey: "key"},
		},
		{
			name:    "missing domain invalid",
			port:    &model.Port{Port: 443, AutoACME: true},
			wantErr: true,
		},
		{
			name:    "missing certificate invalid",
			port:    &model.Port{Port: 443, Domain: "example.com"},
			wantErr: true,
		},
		{
			name:    "invalid port invalid",
			port:    &model.Port{Port: 70000, Domain: "example.com", TLSCert: "c", TLSKey: "k"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateConfigUsesAuthCredentials(t *testing.T) {
	port := &model.Port{
		ID:       "p1",
		Port:     8443,
		Domain:   "example.com",
		Username: "user",
		Password: `p"a\ss`,
		TLSCert:  "cert",
		TLSKey:   "key",
	}

	raw, err := GenerateConfig(port)
	if err != nil {
		t.Fatalf("GenerateConfig() error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("GenerateConfig() produced invalid JSON: %v\n%s", err, raw)
	}
	if strings.Contains(raw, "auth_user_deprecated") || strings.Contains(raw, "auth_pass_deprecated") {
		t.Errorf("config still uses deprecated auth fields:\n%s", raw)
	}

	handler := handlerOf(t, parsed)
	if got := handler["handler"]; got != "forward_proxy" {
		t.Errorf("handler = %v, want forward_proxy", got)
	}

	credentials, ok := handler["auth_credentials"].([]any)
	if !ok || len(credentials) != 1 {
		t.Fatalf("auth_credentials = %v, want a single element array", handler["auth_credentials"])
	}
	token := base64.StdEncoding.EncodeToString([]byte(port.Username + ":" + port.Password))
	want := base64.StdEncoding.EncodeToString([]byte(token))
	if credentials[0] != want {
		t.Errorf("auth_credentials[0] = %v, want %q (JSON encoding of Basic token)", credentials[0], want)
	}
	// Decode exactly as forwardproxy does, rather than only checking JSON text.
	handlerJSON, err := json.Marshal(handler)
	if err != nil {
		t.Fatal(err)
	}
	var upstream struct {
		AuthCredentials [][]byte `json:"auth_credentials"`
	}
	if err := json.Unmarshal(handlerJSON, &upstream); err != nil {
		t.Fatal(err)
	}
	if len(upstream.AuthCredentials) != 1 || string(upstream.AuthCredentials[0]) != token {
		t.Fatalf("upstream credentials do not match Basic authorization token: %q", upstream.AuthCredentials)
	}

	// The manual certificate has to be referenced, otherwise Caddy falls back to
	// issuing a self-signed certificate and the client sees a TLS error.
	certFile, keyFile := CertPath(port.ID)
	loaded := loadFilesOf(t, parsed)
	if loaded["certificate"] != certFile {
		t.Errorf("certificate = %v, want %q", loaded["certificate"], certFile)
	}
	if loaded["key"] != keyFile {
		t.Errorf("key = %v, want %q", loaded["key"], keyFile)
	}
}

func TestGenerateConfigAutoACME(t *testing.T) {
	port := &model.Port{
		ID:       "p2",
		Port:     443,
		Domain:   "example.com",
		Username: "user",
		Password: "pass",
		AutoACME: true,
	}

	raw, err := GenerateConfig(port)
	if err != nil {
		t.Fatalf("GenerateConfig() error: %v", err)
	}
	if strings.Contains(raw, `""`) {
		t.Errorf("config contains an empty string entry:\n%s", raw)
	}
	if !strings.Contains(raw, "automate") {
		t.Errorf("auto ACME config has no automate entry:\n%s", raw)
	}
	if strings.Contains(raw, "load_files") {
		t.Errorf("auto ACME config should not load files:\n%s", raw)
	}
	if !strings.Contains(raw, `"disable_redirects": true`) {
		t.Fatal("per-port instances must not share the implicit HTTP redirect listener")
	}

	// Verify production CA is used by default (no explicit CA in issuer)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("config is not valid JSON: %v", err)
	}
	issuer := getACMEIssuer(t, parsed)
	if _, hasCA := issuer["ca"]; hasCA {
		t.Errorf("production ACME should not specify explicit CA, got: %v", issuer)
	}
}

func TestGenerateConfigACMEStaging(t *testing.T) {
	port := &model.Port{
		ID:          "p3",
		Port:        443,
		Domain:      "example.com",
		Username:    "user",
		Password:    "pass",
		AutoACME:    true,
		ACMEStaging: true,
	}

	raw, err := GenerateConfig(port)
	if err != nil {
		t.Fatalf("GenerateConfig() error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("config is not valid JSON: %v", err)
	}

	issuer := getACMEIssuer(t, parsed)
	ca, ok := issuer["ca"].(string)
	if !ok {
		t.Fatalf("staging ACME issuer has no CA field: %v", issuer)
	}
	want := "https://acme-staging-v02.api.letsencrypt.org/directory"
	if ca != want {
		t.Errorf("staging CA = %q, want %q", ca, want)
	}
}

// getACMEIssuer extracts the ACME issuer config from a parsed Caddy JSON config.
func getACMEIssuer(t *testing.T, parsed map[string]any) map[string]any {
	t.Helper()

	apps, ok := parsed["apps"].(map[string]any)
	if !ok {
		t.Fatalf("config has no apps map")
	}
	tlsApp, ok := apps["tls"].(map[string]any)
	if !ok {
		t.Fatalf("config has no tls app")
	}
	automation, ok := tlsApp["automation"].(map[string]any)
	if !ok {
		t.Fatalf("tls app has no automation")
	}
	policies, ok := automation["policies"].([]any)
	if !ok || len(policies) == 0 {
		t.Fatalf("automation has no policies")
	}
	policy, ok := policies[0].(map[string]any)
	if !ok {
		t.Fatalf("policy is not an object")
	}
	issuers, ok := policy["issuers"].([]any)
	if !ok || len(issuers) == 0 {
		t.Fatalf("policy has no issuers")
	}
	issuer, ok := issuers[0].(map[string]any)
	if !ok {
		t.Fatalf("issuer is not an object")
	}
	return issuer
}


// loadFilesOf returns the single load_files entry of a config.
func loadFilesOf(t *testing.T, parsed map[string]any) map[string]any {
	t.Helper()

	apps, ok := parsed["apps"].(map[string]any)
	if !ok {
		t.Fatalf("config has no apps map")
	}
	tlsApp, ok := apps["tls"].(map[string]any)
	if !ok {
		t.Fatalf("config has no tls app")
	}
	certificates, ok := tlsApp["certificates"].(map[string]any)
	if !ok {
		t.Fatalf("tls app has no certificates")
	}
	loadFiles, ok := certificates["load_files"].([]any)
	if !ok || len(loadFiles) != 1 {
		t.Fatalf("load_files = %v, want a single entry", certificates["load_files"])
	}
	entry, ok := loadFiles[0].(map[string]any)
	if !ok {
		t.Fatalf("load_files entry is not an object")
	}
	return entry
}

func handlerOf(t *testing.T, parsed map[string]any) map[string]any {
	t.Helper()

	apps, ok := parsed["apps"].(map[string]any)
	if !ok {
		t.Fatalf("config has no apps map")
	}
	httpApp, ok := apps["http"].(map[string]any)
	if !ok {
		t.Fatalf("config has no http app")
	}
	servers, ok := httpApp["servers"].(map[string]any)
	if !ok {
		t.Fatalf("http app has no servers")
	}
	server, ok := servers["naive"].(map[string]any)
	if !ok {
		t.Fatalf("no naive server declared")
	}
	routes, ok := server["routes"].([]any)
	if !ok || len(routes) == 0 {
		t.Fatalf("naive server has no routes")
	}
	route, ok := routes[0].(map[string]any)
	if !ok {
		t.Fatalf("route is not an object")
	}
	handles, ok := route["handle"].([]any)
	if !ok || len(handles) == 0 {
		t.Fatalf("route has no handlers")
	}
	handler, ok := handles[0].(map[string]any)
	if !ok {
		t.Fatalf("handler is not an object")
	}
	return handler
}

func TestSimpleLinkEscapesCredentials(t *testing.T) {
	node := &model.Node{Address: "203.0.113.10"}
	port := &model.Port{
		Port:     443,
		Domain:   "example.com",
		Username: "us er",
		Password: "p@ss:w/rd?#&=",
		Label:    "我的节点",
	}

	link := SimpleLink(node, port)
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("SimpleLink() = %q is not a valid URL: %v", link, err)
	}
	if parsed.Scheme != "naive+https" {
		t.Errorf("scheme = %q, want naive+https", parsed.Scheme)
	}
	if parsed.Host != "example.com:443" {
		t.Errorf("host = %q, want example.com:443", parsed.Host)
	}
	if parsed.User == nil {
		t.Fatalf("link %q has no userinfo", link)
	}
	if user := parsed.User.Username(); user != port.Username {
		t.Errorf("username = %q, want %q", user, port.Username)
	}
	if pass, _ := parsed.User.Password(); pass != port.Password {
		t.Errorf("password = %q, want %q", pass, port.Password)
	}
	if strings.Contains(link, "p@ss:w/rd") {
		t.Errorf("link %q contains unescaped credentials", link)
	}
}

func TestSimpleLinkFallsBackToNodeHost(t *testing.T) {
	node := &model.Node{Address: "203.0.113.10"}
	port := &model.Port{Port: 8443, Username: "u", Password: "p"}

	link := SimpleLink(node, port)
	if !strings.Contains(link, "203.0.113.10:8443") {
		t.Errorf("SimpleLink() = %q, want it to use the node address", link)
	}
}

func TestSimpleLinkWrapsIPv6(t *testing.T) {
	node := &model.Node{Address: "2001:db8::1"}
	port := &model.Port{Port: 8443, Username: "u", Password: "p"}

	link := SimpleLink(node, port)
	if !strings.Contains(link, "[2001:db8::1]:8443") {
		t.Errorf("SimpleLink() = %q, want a bracketed IPv6 host", link)
	}
}
