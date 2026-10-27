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
	units        map[string]bool   // unit name -> exists
	unitPIDs     map[string]int    // unit name -> MainPID
	portPIDs     map[string]int    // "port:protocol" -> PID
}

func newMockChecker() *mockChecker {
	return &mockChecker{
		commands:   make(map[string]bool),
		commandOut: make(map[string]string),
		arch:       "amd64",
		uid:        0,
		portsInUse: make(map[string]string),
		units:      make(map[string]bool),
		unitPIDs:   make(map[string]int),
		portPIDs:   make(map[string]int),
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
		unitExists: func(unitName string) bool {
			return m.units[unitName]
		},
		getUnitPID: func(unitName string) (int, error) {
			if pid, ok := m.unitPIDs[unitName]; ok {
				return pid, nil
			}
			return 0, fmt.Errorf("unit %s has no MainPID", unitName)
		},
		getPIDByPort: func(port int, protocol string) (int, error) {
			key := fmt.Sprintf("%d:%s", port, protocol)
			if pid, ok := m.portPIDs[key]; ok {
				return pid, nil
			}
			return 0, fmt.Errorf("no PID found for port %d", port)
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

	result := pc.CheckForPort(port, "merit-mita-23456")
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

	result := pc.CheckForPort(port, "merit-mita-23456")
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

	result := pc.CheckForPort(port, "merit-mita-23456")
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

	result := pc.CheckForPort(port, "merit-mita-23456")
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

	result := pc.CheckForPort(port, "merit-mita-23456")
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

	result := pc.CheckForPort(port, "merit-mita-23456")
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
		ID:        "naive-1",
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeNaive,
	}

	result := pc.CheckForPort(port, "naive-proxy-naive-1")
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
		ID:        "naive-1",
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeNaive,
	}

	result := pc.CheckForPort(port, "naive-proxy-naive-1")
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
		ID:        "naive-1",
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeNaive,
	}

	result := pc.CheckForPort(port, "naive-proxy-naive-1")
	if result != nil {
		t.Errorf("Expected xz check to pass with tar built-in support, got: %v", result.Error)
	}
}

// TestPreflightCheck_NewPortOccupiedByOther tests new port blocked by unrelated process
func TestPreflightCheck_NewPortOccupiedByOther(t *testing.T) {
	mock := newMockChecker()
	mock.commands["systemctl"] = true
	mock.commands["dpkg"] = true
	mock.commandOut["systemctl is-system-running"] = "running"
	// Port occupied by nginx (PID 999)
	mock.portsInUse["23456:tcp"] = "nginx"
	mock.portPIDs["23456:tcp"] = 999

	pc := mock.toPreflightChecker()

	port := &model.Port{
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeMieru,
	}

	// New port: unit doesn't exist yet
	result := pc.CheckForPort(port, "merit-mita-23456")
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

// TestPreflightCheck_ExistingPortOccupiedBySelf tests existing port occupied by its own process
func TestPreflightCheck_ExistingPortOccupiedBySelf(t *testing.T) {
	mock := newMockChecker()
	mock.commands["systemctl"] = true
	mock.commands["dpkg"] = true
	mock.commandOut["systemctl is-system-running"] = "running"
	// Port occupied by mita (PID 1234)
	mock.portsInUse["23456:tcp"] = "mita"
	mock.portPIDs["23456:tcp"] = 1234
	// Unit exists and has same PID
	mock.units["merit-mita-23456"] = true
	mock.unitPIDs["merit-mita-23456"] = 1234

	pc := mock.toPreflightChecker()

	port := &model.Port{
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeMieru,
	}

	// Existing port occupied by self - should pass
	result := pc.CheckForPort(port, "merit-mita-23456")
	if result != nil {
		t.Errorf("Expected port check to pass when occupied by self service, got: %v", result.Error)
	}
}

// TestPreflightCheck_ExistingPortOccupiedByOther tests existing port occupied by unrelated process
func TestPreflightCheck_ExistingPortOccupiedByOther(t *testing.T) {
	mock := newMockChecker()
	mock.commands["systemctl"] = true
	mock.commands["dpkg"] = true
	mock.commandOut["systemctl is-system-running"] = "running"
	// Port occupied by nginx (PID 999)
	mock.portsInUse["23456:tcp"] = "nginx"
	mock.portPIDs["23456:tcp"] = 999
	// Unit exists but has different PID
	mock.units["merit-mita-23456"] = true
	mock.unitPIDs["merit-mita-23456"] = 1234

	pc := mock.toPreflightChecker()

	port := &model.Port{
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeMieru,
	}

	// Existing port occupied by OTHER process - should block
	result := pc.CheckForPort(port, "merit-mita-23456")
	if result == nil {
		t.Error("Expected port check to fail when existing port occupied by other process")
		return
	}

	if result.CheckName != "port_availability" {
		t.Errorf("Expected check name 'port_availability', got '%s'", result.CheckName)
	}
	if !strings.Contains(result.Hint, "nginx") {
		t.Errorf("Expected hint to mention occupier nginx, got: %s", result.Hint)
	}
}

// TestPreflightCheck_NewPortOccupiedByAnotherUnifiedProxyPort tests new port blocked by another UP port
func TestPreflightCheck_NewPortOccupiedByAnotherUnifiedProxyPort(t *testing.T) {
	mock := newMockChecker()
	mock.commands["systemctl"] = true
	mock.commands["dpkg"] = true
	mock.commandOut["systemctl is-system-running"] = "running"
	// Port 23456 occupied by merit-mita-9999 (PID 1234)
	mock.portsInUse["23456:tcp"] = "mita"
	mock.portPIDs["23456:tcp"] = 1234
	// Another unit exists
	mock.units["merit-mita-9999"] = true
	mock.unitPIDs["merit-mita-9999"] = 1234

	pc := mock.toPreflightChecker()

	port := &model.Port{
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeMieru,
	}

	// New port (merit-mita-23456 doesn't exist) but occupied by another UP port - should block
	result := pc.CheckForPort(port, "merit-mita-23456")
	if result == nil {
		t.Error("Expected port check to fail when occupied by another UnifiedProxy port")
		return
	}

	if result.CheckName != "port_availability" {
		t.Errorf("Expected check name 'port_availability', got '%s'", result.CheckName)
	}
}

// TestPreflightCheck_PortChangedNumber tests port ID unchanged but listen port changed
func TestPreflightCheck_PortChangedNumber(t *testing.T) {
	mock := newMockChecker()
	mock.commands["systemctl"] = true
	mock.commands["tar"] = true
	mock.commands["unxz"] = true
	mock.commandOut["systemctl is-system-running"] = "running"
	// Old port 9999 occupied by naive-1 (PID 1234)
	mock.portsInUse["9999:tcp"] = "naive"
	mock.portPIDs["9999:tcp"] = 1234
	mock.units["naive-proxy-naive-1"] = true
	mock.unitPIDs["naive-proxy-naive-1"] = 1234
	// New port 23456 occupied by nginx (PID 999)
	mock.portsInUse["23456:tcp"] = "nginx"
	mock.portPIDs["23456:tcp"] = 999

	pc := mock.toPreflightChecker()

	port := &model.Port{
		ID:        "naive-1",
		Port:      23456, // Changed from 9999 to 23456
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeNaive,
	}

	// Port number changed: per decision matrix "监听号变了=按新增处理"
	// New target port occupied by nginx - should block
	result := pc.CheckForPort(port, "naive-proxy-naive-1")
	if result == nil {
		t.Error("Expected port check to fail when target port occupied")
		return
	}

	if result.CheckName != "port_availability" {
		t.Errorf("Expected check name 'port_availability', got '%s'", result.CheckName)
	}
	if !strings.Contains(result.Hint, "nginx") {
		t.Errorf("Expected hint to mention nginx, got: %s", result.Hint)
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
		ID:        "naive-1",
		Port:      23456,
		Protocol:  "TCP",
		ProxyType: model.ProxyTypeNaive,
	}

	result := pc.CheckForPort(port, "naive-proxy-naive-1")
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
		ID:             "shadowtls-1",
		Port:           23456,
		Protocol:       "TCP",
		ProxyType:      model.ProxyTypeShadowTLS,
		BackendManaged: true, // Managed backend needs tar/xz
	}

	result := pc.CheckForPort(port, "shadowtls-shadowtls-1")
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
		ID:             "shadowtls-1",
		Port:           23456,
		Protocol:       "TCP",
		ProxyType:      model.ProxyTypeShadowTLS,
		BackendManaged: false,
	}

	result := pc.CheckForPort(port, "shadowtls-shadowtls-1")
	if result != nil {
		t.Errorf("Expected all checks to pass for ShadowTLS with external backend, got: %v", result.Error)
	}
}
