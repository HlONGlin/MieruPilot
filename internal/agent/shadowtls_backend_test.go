package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"merit/internal/model"
)

// backendTestAgent returns an agent whose systemctl is simulated and whose host
// checks (binary install, port probe) are replaced, so no network is touched.
func backendTestAgent(t *testing.T) (*Agent, *[]string, map[string]bool) {
	t.Helper()
	a := &Agent{unitDir: t.TempDir()}
	active := map[string]bool{}
	var commands []string
	a.runCommand = func(name string, args ...string) (string, error) {
		commands = append(commands, name+" "+strings.Join(args, " "))
		unit := ""
		if len(args) > 0 {
			unit = strings.TrimSuffix(args[len(args)-1], ".service")
		}
		switch args[0] {
		case "is-active":
			if active[unit] {
				return "active", nil
			}
			return "inactive", fmt.Errorf("inactive")
		case "start", "restart":
			active[unit] = true
		case "disable":
			delete(active, unit)
		}
		return "", nil
	}
	a.ensureBackendBinary = func() error { return nil }
	a.backendPortFree = func(string) bool { return true }
	return a, &commands, active
}

func managedPort() *model.Port {
	return &model.Port{
		ID: "p1", Port: 443, ProxyType: model.ProxyTypeShadowTLS, Enabled: true,
		Password: "front-secret", HandshakeAddr: "cloudflare.com:443",
		BackendAddr: "127.0.0.1:50000", BackendPassword: "back-secret",
		BackendCipher: "aes-256-gcm", BackendManaged: true,
	}
}

func readUnit(t *testing.T, a *Agent, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(a.systemdDir(), name+".service"))
	if err != nil {
		t.Fatalf("unit %s missing: %v", name, err)
	}
	return string(raw)
}

func TestDeployManagedBackendUsesManagerValues(t *testing.T) {
	a, _, active := backendTestAgent(t)
	port := managedPort()
	if err := a.deployManagedBackend(port, "p1"); err != nil {
		t.Fatal(err)
	}
	unit := readUnit(t, a, "shadowsocks-backend-p1")
	for _, want := range []string{"-s 127.0.0.1:50000", "-m aes-256-gcm", "-k back-secret", "#MeritPort=50000"} {
		if !strings.Contains(unit, want) {
			t.Errorf("backend unit lacks %q:\n%s", want, unit)
		}
	}
	if !active["shadowsocks-backend-p1"] {
		t.Error("backend was not started")
	}
	// The agent must not invent or alter credentials.
	if port.BackendPassword != "back-secret" || port.BackendAddr != "127.0.0.1:50000" {
		t.Errorf("agent modified the Manager's backend settings: %+v", port)
	}
}

func TestDeployManagedBackendIsIdempotent(t *testing.T) {
	a, commands, _ := backendTestAgent(t)
	port := managedPort()
	if err := a.deployManagedBackend(port, "p1"); err != nil {
		t.Fatal(err)
	}
	*commands = nil
	if err := a.deployManagedBackend(port, "p1"); err != nil {
		t.Fatal(err)
	}
	for _, c := range *commands {
		if strings.Contains(c, "restart") || strings.Contains(c, "daemon-reload") {
			t.Fatalf("unchanged backend was restarted: %v", *commands)
		}
	}
	*commands = nil
	port.BackendPassword = "rotated"
	if err := a.deployManagedBackend(port, "p1"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(*commands, "\n"), "systemctl restart shadowsocks-backend-p1.service") {
		t.Fatalf("changed password did not restart the backend: %v", *commands)
	}
}

func TestDeployManagedBackendRejectsUnsafeAndBusy(t *testing.T) {
	a, _, _ := backendTestAgent(t)
	public := managedPort()
	public.BackendAddr = "0.0.0.0:50000"
	if err := a.deployManagedBackend(public, "p1"); err == nil {
		t.Fatal("a backend listening on a public address was deployed")
	}

	a.backendPortFree = func(string) bool { return false }
	if err := a.deployManagedBackend(managedPort(), "p2"); err == nil {
		t.Fatal("a busy backend port was accepted")
	}
	if _, err := os.Stat(filepath.Join(a.systemdDir(), "shadowsocks-backend-p2.service")); err == nil {
		t.Fatal("unit was written even though the port is taken")
	}
}

func TestShadowTLSUnitOrdersAfterBackend(t *testing.T) {
	a, _, _ := backendTestAgent(t)
	err := a.writeProxyUnitAfter("shadowtls-p1", "ShadowTLS", 443, "/usr/local/bin/shadow-tls --v3 server", nil, "",
		[]string{backendServiceName("p1") + ".service"})
	if err != nil {
		t.Fatal(err)
	}
	unit := readUnit(t, a, "shadowtls-p1")
	for _, want := range []string{
		"After=network-online.target shadowsocks-backend-p1.service",
		"Wants=network-online.target shadowsocks-backend-p1.service",
	} {
		if !strings.Contains(unit, want) {
			t.Errorf("frontend unit lacks %q:\n%s", want, unit)
		}
	}
	if strings.Contains(unit, "Requires=") {
		t.Error("a hard Requires would stop the listener whenever the backend restarts")
	}
}

func TestRemoveManagedBackend(t *testing.T) {
	a, commands, active := backendTestAgent(t)
	if err := a.removeManagedBackend("none"); err != nil || len(*commands) != 0 {
		t.Fatalf("removing a missing backend must be a no-op: %v %v", err, *commands)
	}
	if err := a.deployManagedBackend(managedPort(), "p1"); err != nil {
		t.Fatal(err)
	}
	if err := a.removeManagedBackend("p1"); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", ".applied"} {
		if _, err := os.Stat(filepath.Join(a.systemdDir(), "shadowsocks-backend-p1.service"+suffix)); err == nil {
			t.Errorf("backend file %q was left behind", suffix)
		}
	}
	if active["shadowsocks-backend-p1"] {
		t.Error("backend is still running")
	}
}

func TestStaleBackendIsCleanedUp(t *testing.T) {
	a, _, active := backendTestAgent(t)
	for _, id := range []string{"keep", "gone"} {
		p := managedPort()
		p.ID = id
		if id == "gone" {
			p.BackendAddr = "127.0.0.1:50001"
		}
		if err := a.deployManagedBackend(p, id); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &model.DesiredConfig{Partial: false, Ports: []*model.Port{
		{ID: "keep", Port: 443, ProxyType: model.ProxyTypeShadowTLS, Enabled: true},
	}}
	if _, err := a.removeStaleProxyInstances(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.systemdDir(), "shadowsocks-backend-gone.service")); err == nil {
		t.Error("orphaned backend survived the cleanup")
	}
	if _, err := os.Stat(filepath.Join(a.systemdDir(), "shadowsocks-backend-keep.service")); err != nil {
		t.Error("backend of a live port was removed")
	}
	if active["shadowsocks-backend-gone"] || !active["shadowsocks-backend-keep"] {
		t.Errorf("wrong services running: %v", active)
	}
}

func TestBackendUnitsAreNotProxyInstances(t *testing.T) {
	if _, _, ok := parseProxyUnitName("shadowsocks-backend-p1"); ok {
		t.Fatal("a backend unit must not be reported as a proxy instance")
	}
	if id, ok := parseBackendUnitName("shadowsocks-backend-p1"); !ok || id != "p1" {
		t.Fatalf("parseBackendUnitName() = %q, %v", id, ok)
	}
	if _, ok := parseBackendUnitName("shadowtls-p1"); ok {
		t.Fatal("a frontend unit was taken for a backend")
	}
}
