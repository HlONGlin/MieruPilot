package naive

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"merit/internal/model"
)

// Run with NAIVE_TEST_CADDY pointing to the upstream patched Caddy release.
// Only test configuration allows the loopback target and uses a self-signed cert.
func TestUpstreamCONNECTAuthentication(t *testing.T) {
	binary := os.Getenv("NAIVE_TEST_CADDY")
	if binary == "" {
		t.Skip("set NAIVE_TEST_CADDY to run the upstream integration test")
	}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "authenticated-connect-ok")
	}))
	defer target.Close()
	targetAddress := target.Listener.Addr().String()
	token := base64.StdEncoding.EncodeToString([]byte("integration-user:integration-password"))

	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacyEncoding=%v", legacy), func(t *testing.T) {
			dir := t.TempDir()
			fixture := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
			certificate := fixture.TLS.Certificates[0]
			fixture.Close()
			key, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
			if err != nil {
				t.Fatal(err)
			}
			certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
			if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0o600); err != nil {
				t.Fatal(err)
			}
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			address := listener.Addr().String()
			port := listener.Addr().(*net.TCPAddr).Port
			_ = listener.Close()
			raw, err := GenerateConfig(&model.Port{ID: "integration", Port: port, Domain: "example.com", Username: "integration-user", Password: "integration-password", TLSCert: "fixture", TLSKey: "fixture"})
			if err != nil {
				t.Fatal(err)
			}
			var cfg map[string]any
			if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
				t.Fatal(err)
			}
			cfg["admin"].(map[string]any)["config"] = map[string]any{"persist": false}
			loaded := loadFilesOf(t, cfg)
			loaded["certificate"], loaded["key"] = certFile, keyFile
			handler := handlerOf(t, cfg)
			handler["acl"] = []any{map[string]any{"subjects": []string{"127.0.0.1"}, "allow": true}}
			if legacy {
				handler["auth_credentials"] = []string{token}
			}
			server := cfg["apps"].(map[string]any)["http"].(map[string]any)["servers"].(map[string]any)["naive"].(map[string]any)
			server["listen"] = []string{address}
			server["automatic_https"] = map[string]any{"disable": true}
			route := server["routes"].([]any)[0].(map[string]any)
			route["handle"] = append(route["handle"].([]any), map[string]any{"handler": "static_response", "status_code": 404})
			configFile := filepath.Join(dir, "caddy.json")
			encoded, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(configFile, encoded, 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(binary, "run", "--config", configFile)
			cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+filepath.Join(dir, "config"), "XDG_DATA_HOME="+filepath.Join(dir, "data"))
			var logs bytes.Buffer
			cmd.Stdout, cmd.Stderr = &logs, &logs
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				if t.Failed() {
					t.Log(logs.String())
				}
			})
			tlsConfig := &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"http/1.1"}} // Local fixture certificate only.
			ready := false
			for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
				conn, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", address, tlsConfig)
				if err == nil {
					_ = conn.Close()
					ready = true
					break
				}
				time.Sleep(50 * time.Millisecond)
			}
			if !ready {
				t.Fatal("Caddy did not start")
			}
			for _, credential := range []string{"", "wrong", token} {
				conn, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", address, tlsConfig)
				if err != nil {
					t.Fatal(err)
				}
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				header := ""
				if credential != "" {
					header = "Proxy-Authorization: Basic " + credential + "\r\n"
				}
				_, err = fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n%s\r\n", targetAddress, targetAddress, header)
				if err != nil {
					_ = conn.Close()
					t.Fatal(err)
				}
				reader := bufio.NewReader(conn)
				response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
				if err != nil {
					_ = conn.Close()
					t.Fatal(err)
				}
				want := 404
				if !legacy && credential == token {
					want = 200
				}
				if response.StatusCode != want {
					_ = conn.Close()
					t.Fatalf("CONNECT status = %d, want %d", response.StatusCode, want)
				}
				if want == 200 {
					_, _ = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", targetAddress)
					forwarded, err := http.ReadResponse(reader, &http.Request{Method: http.MethodGet})
					if err != nil {
						_ = conn.Close()
						t.Fatal(err)
					}
					body, err := io.ReadAll(forwarded.Body)
					_ = forwarded.Body.Close()
					if err != nil || string(body) != "authenticated-connect-ok" {
						_ = conn.Close()
						t.Fatalf("forwarded response = %s: %v", strconv.Quote(string(body)), err)
					}
				}
				_ = conn.Close()
			}
		})
	}
}
