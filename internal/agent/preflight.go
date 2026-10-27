package agent

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"merit/internal/model"
)

// PreflightChecker defines the interface for system checks.
type PreflightChecker struct {
	// Command execution hooks for testing
	commandExists func(string) bool
	commandRun    func(string, ...string) (string, error)
	getArch       func() string
	getUID        func() int
	portInUse     func(int, string) (bool, string)
}

// NewPreflightChecker creates a checker with real implementations.
func NewPreflightChecker() *PreflightChecker {
	return &PreflightChecker{
		commandExists: commandExists,
		commandRun:    runCommand,
		getArch:       func() string { return runtime.GOARCH },
		getUID:        os.Getuid,
		portInUse:     checkPortInUse,
	}
}

// PreflightResult represents the result of a single check.
type PreflightResult struct {
	CheckName string
	Pass      bool
	Error     string
	Hint      string
}

// CheckForPort runs preflight checks for a specific port configuration.
// It only checks dependencies required by the port's protocol type.
// Returns nil if all checks pass, otherwise returns a PreflightResult with the first failure.
func (pc *PreflightChecker) CheckForPort(port *model.Port, isNewPort bool) *PreflightResult {
	// Check 1: Root permission (all protocols need this)
	if pc.getUID() != 0 {
		return &PreflightResult{
			CheckName: "root_permission",
			Pass:      false,
			Error:     "Agent 需要 root 权限运行",
			Hint:      "请使用 sudo 或以 root 用户执行",
		}
	}

	// Check 2: Architecture (all protocols)
	arch := pc.getArch()
	if arch != "amd64" && arch != "arm64" {
		return &PreflightResult{
			CheckName: "architecture",
			Pass:      false,
			Error:     fmt.Sprintf("不支持的架构: %s", arch),
			Hint:      "仅支持 amd64 和 arm64 架构",
		}
	}

	// Check 3: systemd availability (all protocols)
	if result := pc.checkSystemd(); result != nil {
		return result
	}

	proxyType := model.NormalizeProxyType(port.ProxyType)

	// Check 4: Package manager for Mieru
	if proxyType == model.ProxyTypeMieru {
		if result := pc.checkPackageManager(); result != nil {
			return result
		}
	}

	// Check 5: tar and xz for NaïveProxy and shadowsocks backend
	if proxyType == model.ProxyTypeNaive ||
	   (proxyType == model.ProxyTypeShadowTLS && port.BackendManaged) {
		if result := pc.checkTarXZ(); result != nil {
			return result
		}
	}

	// Check 6: Port availability (only for new ports)
	// "修改端口号到一个新号码"按新增算，由调用方通过 isNewPort 参数指定
	if isNewPort {
		if result := pc.checkPortAvailability(port); result != nil {
			return result
		}
	}

	return nil
}

// checkSystemd verifies systemd is available and running.
func (pc *PreflightChecker) checkSystemd() *PreflightResult {
	if !pc.commandExists("systemctl") {
		return &PreflightResult{
			CheckName: "systemd",
			Pass:      false,
			Error:     "systemctl 命令不存在",
			Hint:      "需要 systemd 初始化系统",
		}
	}

	// Verify systemd is actually running (not just installed)
	output, _ := pc.commandRun("systemctl", "is-system-running")
	// systemd states: initializing, starting, running, degraded, maintenance, stopping, offline, unknown
	// Accept running and degraded (degraded means some units failed but system is operational)
	validStates := []string{"running", "degraded"}
	for _, state := range validStates {
		if strings.Contains(output, state) {
			return nil
		}
	}

	return &PreflightResult{
		CheckName: "systemd",
		Pass:      false,
		Error:     "systemd 未运行",
		Hint:      "当前系统未使用 systemd 初始化",
	}
}

// checkPackageManager verifies dpkg or rpm is available (for Mieru).
func (pc *PreflightChecker) checkPackageManager() *PreflightResult {
	hasDpkg := pc.commandExists("dpkg")
	hasRpm := pc.commandExists("rpm")

	if !hasDpkg && !hasRpm {
		return &PreflightResult{
			CheckName: "package_manager",
			Pass:      false,
			Error:     "缺少包管理器",
			Hint:      "Mieru 需要 dpkg (Debian/Ubuntu) 或 rpm (RHEL/CentOS) 包管理器",
		}
	}

	return nil
}

// checkTarXZ verifies tar and xz decompression capability.
func (pc *PreflightChecker) checkTarXZ() *PreflightResult {
	if !pc.commandExists("tar") {
		return &PreflightResult{
			CheckName: "tar",
			Pass:      false,
			Error:     "缺少依赖: tar",
			Hint: `安装方法:
  Debian/Ubuntu: apt-get install tar
  RHEL/CentOS/Rocky: yum install tar
  Alpine: apk add tar`,
		}
	}

	// Check xz support: either unxz command exists OR tar supports --xz
	hasUnxz := pc.commandExists("unxz")
	if hasUnxz {
		return nil
	}

	// Test if tar has built-in xz support
	_, err := pc.commandRun("tar", "--help")
	if err == nil {
		output, _ := pc.commandRun("tar", "--help")
		if strings.Contains(output, "--xz") || strings.Contains(output, "xz") {
			return nil
		}
	}

	return &PreflightResult{
		CheckName: "xz",
		Pass:      false,
		Error:     "缺少依赖: xz 解压能力",
		Hint: `安装方法:
  Debian/Ubuntu: apt-get install xz-utils
  RHEL/CentOS/Rocky: yum install xz
  Alpine: apk add xz`,
	}
}

// checkPortAvailability verifies the port is not occupied.
func (pc *PreflightChecker) checkPortAvailability(port *model.Port) *PreflightResult {
	protocol := strings.ToLower(port.Protocol)
	if protocol != "tcp" && protocol != "udp" {
		protocol = "tcp"
	}

	inUse, occupier := pc.portInUse(port.Port, protocol)
	if !inUse {
		return nil
	}

	// 占用者是该端口自身实例/自身服务时不算冲突
	// 检查占用者是否为 mita, caddy-naive, anytls-server, shadow-tls, ssserver
	lowerOccupier := strings.ToLower(occupier)
	selfServices := []string{"mita", "caddy", "anytls", "shadow-tls", "ssserver"}
	for _, svc := range selfServices {
		if strings.Contains(lowerOccupier, svc) {
			// 自身服务占用，不算冲突
			return nil
		}
	}

	return &PreflightResult{
		CheckName: "port_availability",
		Pass:      false,
		Error:     fmt.Sprintf("端口 %d (%s) 已被占用", port.Port, protocol),
		Hint:      fmt.Sprintf("占用进程: %s\n请释放端口或选择其他端口", occupier),
	}
}

// Helper functions for real implementations

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func runCommand(cmd string, args ...string) (string, error) {
	out, err := exec.Command(cmd, args...).CombinedOutput()
	return string(out), err
}

func checkPortInUse(port int, protocol string) (bool, string) {
	// Use 'ss' if available, fallback to 'netstat'
	var cmd string
	var args []string

	if commandExists("ss") {
		// ss -tulnp | grep :PORT
		if protocol == "tcp" {
			args = []string{"-tlnp"}
		} else {
			args = []string{"-ulnp"}
		}
		cmd = "ss"
	} else if commandExists("netstat") {
		// netstat -tulnp | grep :PORT
		if protocol == "tcp" {
			args = []string{"-tlnp"}
		} else {
			args = []string{"-ulnp"}
		}
		cmd = "netstat"
	} else {
		// No tool available, assume not in use
		return false, ""
	}

	output, err := runCommand(cmd, args...)
	if err != nil {
		return false, ""
	}

	// Parse output to find the port
	portStr := fmt.Sprintf(":%d ", port)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, portStr) {
			// Extract process name from the line
			// ss format: ... users:(("process",pid=123,fd=4))
			// netstat format: ... PID/Program
			if strings.Contains(line, "users:((") {
				// ss format
				start := strings.Index(line, "users:((\"")
				if start != -1 {
					start += 9
					end := strings.Index(line[start:], "\"")
					if end != -1 {
						return true, line[start : start+end]
					}
				}
			} else {
				// netstat format or fallback
				fields := strings.Fields(line)
				for i, field := range fields {
					if strings.Contains(field, "/") && i > 0 {
						parts := strings.Split(field, "/")
						if len(parts) == 2 {
							return true, parts[1]
						}
					}
				}
			}
			return true, "unknown"
		}
	}

	return false, ""
}
