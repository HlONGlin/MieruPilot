package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"merit/internal/model"
	"merit/internal/proxy/anytls"
	"merit/internal/proxy/naive"
	"merit/internal/proxy/shadowsocks"
	"merit/internal/proxy/shadowtls"
)

func TestExecutableInstalledRequiresRegularExecutableAtExactPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server")
	if executableInstalled(path) || executableInstalled(dir) {
		t.Fatal("missing files and directories are not executables")
	}
	if err := os.WriteFile(path, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if executableInstalled(path) {
		t.Fatal("empty file accepted")
	}
	if err := os.WriteFile(path, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !executableInstalled(path) {
		t.Fatal("nonempty executable rejected")
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o644); err != nil {
			t.Fatal(err)
		}
		if executableInstalled(path) {
			t.Fatal("nonexecutable file accepted")
		}
	}
}

func TestNaiveInstalledRequiresForwardProxyModule(t *testing.T) {
	path := filepath.Join(t.TempDir(), "caddy-naive")
	if err := os.WriteFile(path, []byte("fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		modules     string
		fails, want bool
	}{
		{modules: "http.handlers.reverse_proxy\n"},
		{modules: "http.handlers.forward_proxy_extra\n"},
		{modules: "http.handlers.forward_proxy\n", fails: true},
		{modules: "http.handlers.forward_proxy\nStandard modules: 1", want: true},
	} {
		a := &Agent{runCommand: func(name string, args ...string) (string, error) {
			if name != path || len(args) != 1 || args[0] != "list-modules" {
				t.Fatalf("wrong module command: %s %v", name, args)
			}
			if tc.fails {
				return tc.modules, fmt.Errorf("not executable")
			}
			return tc.modules, nil
		}}
		if got := a.naiveInstalledAt(path); got != tc.want {
			t.Fatalf("modules %q: got %v want %v", tc.modules, got, tc.want)
		}
	}
}

func TestManagedInstallDetectionIgnoresPATH(t *testing.T) {
	dir := t.TempDir()
	a := &Agent{}
	for _, tc := range []struct {
		name, path string
		installed  func() bool
	}{
		{naive.BinaryName, naive.BinaryPath, a.naiveInstalled},
		{anytls.ServerBinaryName, anytls.ServerBinaryPath, a.anyTLSInstalled},
		{shadowtls.BinaryName, shadowtls.BinaryPath, a.shadowTLSInstalled},
		{shadowsocks.BinaryName, shadowsocks.BinaryPath, shadowsocks.IsInstalled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(tc.path); !os.IsNotExist(err) {
				t.Skip("managed path already exists on test host")
			}
			name := tc.name
			if runtime.GOOS == "windows" {
				name += ".cmd"
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte("placeholder"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir)
			if tc.installed() {
				t.Fatalf("PATH binary incorrectly satisfied managed path %s", tc.path)
			}
		})
	}
}

func TestMieruInstallFailureStillDispatchesIndependentProxy(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	a := &Agent{}
	_, results, err := a.applyConfig(&model.DesiredConfig{Partial: true, Ports: []*model.Port{
		{ID: "mita", Port: 23456, Protocol: model.ProtocolTCP, ProxyType: model.ProxyTypeMieru, Username: "u", Password: "p"},
		{ID: "independent", Port: 23457, Protocol: model.ProtocolTCP, ProxyType: "unsupported"},
	}})
	if err == nil {
		t.Fatal("failed deployment was acknowledged")
	}
	byID := map[string]model.PortInstanceResult{}
	for _, result := range results {
		byID[result.PortID] = result
	}
	for _, id := range []string{"mita", "independent"} {
		result, exists := byID[id]
		if !exists || result.OK || result.Error == "" {
			t.Fatalf("missing explicit failure result for %s: %+v", id, results)
		}
	}
}

func TestMieruApplyFailureReportsEveryPortAndContinues(t *testing.T) {
	dir := t.TempDir()
	name := "mita"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("placeholder"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	a := &Agent{unitDir: t.TempDir(), mitaDir: t.TempDir()}
	a.runCommand = func(name string, args ...string) (string, error) {
		if name == "systemctl" && strings.Join(args, " ") == "daemon-reload" {
			return "", fmt.Errorf("reload failed")
		}
		return "", nil
	}
	_, results, err := a.applyConfig(&model.DesiredConfig{Partial: true, Ports: []*model.Port{
		{ID: "one", Port: 23456, Protocol: model.ProtocolTCP, ProxyType: model.ProxyTypeMieru, Username: "u", Password: "p"},
		{ID: "two", Port: 23457, Protocol: model.ProtocolTCP, ProxyType: model.ProxyTypeMieru, Username: "v", Password: "q"},
		{ID: "other", Port: 23458, ProxyType: "unsupported"},
	}})
	if err == nil || len(results) != 3 {
		t.Fatalf("incomplete failed sync: %+v %v", results, err)
	}
	for _, result := range results {
		if result.OK || result.Error == "" || result.PortID == "" {
			t.Fatalf("incomplete failure: %+v", result)
		}
	}
}
