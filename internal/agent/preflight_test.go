package agent

import (
	"fmt"
	"strings"
	"testing"

	"merit/internal/model"
)

// Mock implementations for testing
type mockChecker struct {
	commands     map[string]bool
	commandOut   map[string]string
	arch         string
	uid          int
	portsInUse   map[string]string // "port:protocol" -> "occupier"
}

func newMockChecker() *mockChecker {
	return &mockChecker{
		commands:   make(map[string]bool),
		commandOut: make(map[string]string),
		arch:       "amd64",
		uid:        0,
		portsInUse: make(map[string]string),
	}
}

func (m *mockChecker) toPreflightChecker() *PreflightChecker {
	return &PreflightChecker{
		commandExists: func(cmd string) bool {
			return m.commands[cmd]
		},
		commandRun: func(cmd string, args ...string) (string, error) {
			key := cmd + " " + strings.Join(args, " ")
			if out, ok := m.commandOut[key]; ok {
				return out, nil
			}
			return "", nil
		},
		getArch: func() string {
			return m.arch
		},
		getUID: func() int {
			return m.uid
		},
		portInUse: func(port int, protocol string) (bool, string) {
			key := fmt.Sprintf("%d:%s", port, protocol)
			if occupier, ok := m.portsInUse[key]; ok {
				return true, occupier
			}
			return false, ""
		},
	}
}

func TestPreflightCheck_AllPass(t *testing.T) {
	mock := newMockChecker()
	mock.commands["systemctl"] = true
	mock.commands["dpkg"] = true
	mock.commands["tar"] = true
	mock.commands["unxz"] = true
	mock.commandOut["systemctl is-system-running"] = "running"

	pc := mock.toPreflightChecker()

	port := &model.Port{
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeMieru,
	}

	result := pc.CheckForPort(port, true)
	if result != nil {
		t.Errorf("Expected all checks to pass, got: %v", result.Error)
	}
}

func TestPreflightCheck_NoRootPermission(t *testing.T) {
	mock := newMockChecker()
	mock.uid = 1000 // non-root

	pc := mock.toPreflightChecker()

	port := &model.Port{
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeMieru,
	}

	result := pc.CheckForPort(port, false)
	if result == nil {
		t.Error("Expected root permission check to fail")
		return
	}

	if result.CheckName != "root_permission" {
		t.Errorf("Expected check name 'root_permission', got '%s'", result.CheckName)
	}
	if result.Pass {
		t.Error("Expected Pass to be false")
	}
	if !strings.Contains(result.Error, "root") {
		t.Errorf("Expected error to mention root, got: %s", result.Error)
	}
}

func TestPreflightCheck_UnsupportedArch(t *testing.T) {
	mock := newMockChecker()
	mock.arch = "armv7"
	mock.commands["systemctl"] = true
	mock.commandOut["systemctl is-system-running"] = "running"

	pc := mock.toPreflightChecker()

	port := &model.Port{
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeMieru,
	}

	result := pc.CheckForPort(port, false)
	if result == nil {
		t.Error("Expected architecture check to fail")
		return
	}

	if result.CheckName != "architecture" {
		t.Errorf("Expected check name 'architecture', got '%s'", result.CheckName)
	}
	if !strings.Contains(result.Error, "armv7") {
		t.Errorf("Expected error to mention armv7, got: %s", result.Error)
	}
}

func TestPreflightCheck_NoSystemd(t *testing.T) {
	mock := newMockChecker()
	// systemctl command doesn't exist

	pc := mock.toPreflightChecker()

	port := &model.Port{
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeMieru,
	}

	result := pc.CheckForPort(port, false)
	if result == nil {
		t.Error("Expected systemd check to fail")
		return
	}

	if result.CheckName != "systemd" {
		t.Errorf("Expected check name 'systemd', got '%s'", result.CheckName)
	}
	if !strings.Contains(result.Error, "systemctl") {
		t.Errorf("Expected error to mention systemctl, got: %s", result.Error)
	}
}

func TestPreflightCheck_SystemdNotRunning(t *testing.T) {
	mock := newMockChecker()
	mock.commands["systemctl"] = true
	mock.commands["dpkg"] = true // Add package manager to pass that check
	mock.commandOut["systemctl is-system-running"] = "offline"

	pc := mock.toPreflightChecker()

	port := &model.Port{
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeMieru,
	}

	result := pc.CheckForPort(port, false)
	if result == nil {
		t.Error("Expected systemd running check to fail")
		return
	}

	if result.CheckName != "systemd" {
		t.Errorf("Expected check name 'systemd', got '%s'", result.CheckName)
	}
}

func TestPreflightCheck_MissingPackageManager(t *testing.T) {
	mock := newMockChecker()
	mock.commands["systemctl"] = true
	mock.commandOut["systemctl is-system-running"] = "running"
	// No dpkg or rpm

	pc := mock.toPreflightChecker()

	port := &model.Port{
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeMieru,
	}

	result := pc.CheckForPort(port, false)
	if result == nil {
		t.Error("Expected package manager check to fail")
		return
	}

	if result.CheckName != "package_manager" {
		t.Errorf("Expected check name 'package_manager', got '%s'", result.CheckName)
	}
	if !strings.Contains(result.Error, "包管理器") {
		t.Errorf("Expected error to mention package manager, got: %s", result.Error)
	}
}

func TestPreflightCheck_MissingTar(t *testing.T) {
	mock := newMockChecker()
	mock.commands["systemctl"] = true
	mock.commandOut["systemctl is-system-running"] = "running"
	// No tar command

	pc := mock.toPreflightChecker()

	port := &model.Port{
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeNaive,
	}

	result := pc.CheckForPort(port, false)
	if result == nil {
		t.Error("Expected tar check to fail")
		return
	}

	if result.CheckName != "tar" {
		t.Errorf("Expected check name 'tar', got '%s'", result.CheckName)
	}
	if !strings.Contains(result.Hint, "Debian/Ubuntu") {
		t.Errorf("Expected hint to contain install commands, got: %s", result.Hint)
	}
}

func TestPreflightCheck_MissingXZ(t *testing.T) {
	mock := newMockChecker()
	mock.commands["systemctl"] = true
	mock.commands["tar"] = true
	mock.commandOut["systemctl is-system-running"] = "running"
	mock.commandOut["tar --help"] = "Options:\n  -c  create\n  -x  extract"
	// No unxz and tar doesn't support xz

	pc := mock.toPreflightChecker()

	port := &model.Port{
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeNaive,
	}

	result := pc.CheckForPort(port, false)
	if result == nil {
		t.Error("Expected xz check to fail")
		return
	}

	if result.CheckName != "xz" {
		t.Errorf("Expected check name 'xz', got '%s'", result.CheckName)
	}
	if !strings.Contains(result.Hint, "xz-utils") {
		t.Errorf("Expected hint to contain xz-utils, got: %s", result.Hint)
	}
}

func TestPreflightCheck_XZBuiltInTar(t *testing.T) {
	mock := newMockChecker()
	mock.commands["systemctl"] = true
	mock.commands["tar"] = true
	mock.commandOut["systemctl is-system-running"] = "running"
	mock.commandOut["tar --help"] = "Options:\n  --xz  use xz compression"
	// No unxz but tar has xz support

	pc := mock.toPreflightChecker()

	port := &model.Port{
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeNaive,
	}

	result := pc.CheckForPort(port, false)
	if result != nil {
		t.Errorf("Expected xz check to pass with tar built-in support, got: %v", result.Error)
	}
}

func TestPreflightCheck_PortOccupied(t *testing.T) {
	mock := newMockChecker()
	mock.commands["systemctl"] = true
	mock.commands["dpkg"] = true
	mock.commandOut["systemctl is-system-running"] = "running"
	mock.portsInUse["23456:tcp"] = "nginx"

	pc := mock.toPreflightChecker()

	port := &model.Port{
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeMieru,
	}

	// New port should check and fail
	result := pc.CheckForPort(port, true)
	if result == nil {
		t.Error("Expected port check to fail for occupied port")
		return
	}

	if result.CheckName != "port_availability" {
		t.Errorf("Expected check name 'port_availability', got '%s'", result.CheckName)
	}
	if !strings.Contains(result.Error, "23456") {
		t.Errorf("Expected error to mention port number, got: %s", result.Error)
	}
	if !strings.Contains(result.Hint, "nginx") {
		t.Errorf("Expected hint to mention occupier, got: %s", result.Hint)
	}
}

func TestPreflightCheck_PortOccupiedBySelf(t *testing.T) {
	mock := newMockChecker()
	mock.commands["systemctl"] = true
	mock.commands["dpkg"] = true
	mock.commandOut["systemctl is-system-running"] = "running"
	mock.portsInUse["23456:tcp"] = "mita"

	pc := mock.toPreflightChecker()

	port := &model.Port{
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeMieru,
	}

	// Occupied by self service, should pass
	result := pc.CheckForPort(port, true)
	if result != nil {
		t.Errorf("Expected port check to pass when occupied by self service, got: %v", result.Error)
	}
}

func TestPreflightCheck_ExistingPortNotChecked(t *testing.T) {
	mock := newMockChecker()
	mock.commands["systemctl"] = true
	mock.commands["dpkg"] = true
	mock.commandOut["systemctl is-system-running"] = "running"
	mock.portsInUse["23456:tcp"] = "nginx"

	pc := mock.toPreflightChecker()

	port := &model.Port{
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeMieru,
	}

	// Existing port (isNewPort=false) should not check port availability
	result := pc.CheckForPort(port, false)
	if result != nil {
		t.Errorf("Expected no port check for existing port, got: %v", result.Error)
	}
}

func TestPreflightCheck_NaiveNeedsTarXZ(t *testing.T) {
	mock := newMockChecker()
	mock.commands["systemctl"] = true
	mock.commands["tar"] = true
	mock.commands["unxz"] = true
	mock.commandOut["systemctl is-system-running"] = "running"
	// NaïveProxy doesn't need package manager

	pc := mock.toPreflightChecker()

	port := &model.Port{
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeNaive,
	}

	result := pc.CheckForPort(port, false)
	if result != nil {
		t.Errorf("Expected all checks to pass for NaïveProxy, got: %v", result.Error)
	}
}

func TestPreflightCheck_ShadowTLSManagedBackend(t *testing.T) {
	mock := newMockChecker()
	mock.commands["systemctl"] = true
	mock.commands["tar"] = true
	mock.commands["unxz"] = true
	mock.commandOut["systemctl is-system-running"] = "running"

	pc := mock.toPreflightChecker()

	port := &model.Port{
		Port:           23456,
		Protocol:       "TCP",
		ProxyType:      model.ProxyTypeShadowTLS,
		BackendManaged: true, // Managed backend needs tar/xz
	}

	result := pc.CheckForPort(port, false)
	if result != nil {
		t.Errorf("Expected all checks to pass for ShadowTLS with managed backend, got: %v", result.Error)
	}
}

func TestPreflightCheck_ShadowTLSExternalBackend(t *testing.T) {
	mock := newMockChecker()
	mock.commands["systemctl"] = true
	mock.commandOut["systemctl is-system-running"] = "running"
	// External backend doesn't need tar/xz

	pc := mock.toPreflightChecker()

	port := &model.Port{
		Port:           23456,
		Protocol:       "TCP",
		ProxyType:      model.ProxyTypeShadowTLS,
		BackendManaged: false,
	}

	result := pc.CheckForPort(port, false)
	if result != nil {
		t.Errorf("Expected all checks to pass for ShadowTLS with external backend, got: %v", result.Error)
	}
}
