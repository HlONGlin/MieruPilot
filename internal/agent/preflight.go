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
	// Unit and process identity checks
	unitExists    func(string) bool
	getUnitPID    func(string) (int, error)
	getPIDByPort  func(int, string) (int, error)
}

// NewPreflightChecker creates a checker with real implementations.
func NewPreflightChecker() *PreflightChecker {
	return &PreflightChecker{
		commandExists: commandExists,
		commandRun:    runCommand,
		getArch:       func() string { return runtime.GOARCH },
		getUID:        os.Getuid,
		portInUse:     checkPortInUse,
		unitExists:    unitFileExists,
		getUnitPID:    getUnitMainPID,
		getPIDByPort:  getPIDOccupyingPort,
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
// unitName is the systemd unit name (without .service suffix) for this port.
// Returns nil if all checks pass, otherwise returns a PreflightResult with the first failure.
func (pc *PreflightChecker) CheckForPort(port *model.Port, unitName string) *PreflightResult {
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

	// Check 6: Port availability
	// 占用判定矩阵：
	// - 未占用 → 通过
	// - 占用者是该端口自己的进程（通过 unit MainPID 匹配）→ 通过
	// - 占用者是其他任何进程（包括另一个 UnifiedProxy 端口、同名进程）→ 阻断
	// 身份排除已解决重启误判，剩下的陌生占用必然导致绑定失败，preflight 应提前阻断
	if result := pc.checkPortAvailability(port, unitName); result != nil {
		return result
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

// checkPortAvailability verifies the port is not occupied by other processes.
// Occupancy by the port's own unit (checked via MainPID) is allowed.
func (pc *PreflightChecker) checkPortAvailability(port *model.Port, unitName string) *PreflightResult {
	protocol := strings.ToLower(port.Protocol)
	if protocol != "tcp" && protocol != "udp" {
		protocol = "tcp"
	}

	inUse, occupierInfo := pc.portInUse(port.Port, protocol)
	if !inUse {
		return nil
	}

	// Port is occupied, check if it's by the unit's own process
	occupyingPID, err := pc.getPIDByPort(port.Port, protocol)
	if err != nil {
		// Cannot determine occupier identity, treat as foreign process
		return &PreflightResult{
			CheckName: "port_availability",
			Pass:      false,
			Error:     fmt.Sprintf("端口 %d (%s) 已被占用", port.Port, protocol),
			Hint:      fmt.Sprintf("占用者信息: %s\n无法确认占用者身份，请释放端口或选择其他端口", occupierInfo),
		}
	}

	// Check if the occupying process belongs to this port's unit
	if unitName != "" && pc.unitExists(unitName) {
		unitPID, err := pc.getUnitPID(unitName)
		if err == nil && unitPID == occupyingPID {
			// Port is occupied by its own unit's process, this is expected
			return nil
		}
	}

	// Occupied by another process (including another UnifiedProxy port)
	return &PreflightResult{
		CheckName: "port_availability",
		Pass:      false,
		Error:     fmt.Sprintf("端口 %d (%s) 已被其他进程占用", port.Port, protocol),
		Hint:      fmt.Sprintf("占用进程 PID: %d\n详情: %s\n请释放端口或选择其他端口", occupyingPID, occupierInfo),
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

// unitFileExists checks if a systemd unit file exists.
func unitFileExists(unitName string) bool {
	unitPath := fmt.Sprintf("/etc/systemd/system/%s.service", unitName)
	_, err := os.Stat(unitPath)
	return err == nil
}

// getUnitMainPID retrieves the MainPID of a systemd unit.
func getUnitMainPID(unitName string) (int, error) {
	output, err := runCommand("systemctl", "show", "--property=MainPID", unitName)
	if err != nil {
		return 0, fmt.Errorf("failed to query unit MainPID: %w", err)
	}

	// Parse output: MainPID=12345
	output = strings.TrimSpace(output)
	if !strings.HasPrefix(output, "MainPID=") {
		return 0, fmt.Errorf("unexpected output format: %s", output)
	}

	pidStr := strings.TrimPrefix(output, "MainPID=")
	var pid int
	_, err = fmt.Sscanf(pidStr, "%d", &pid)
	if err != nil {
		return 0, fmt.Errorf("failed to parse PID: %w", err)
	}

	if pid == 0 {
		return 0, fmt.Errorf("unit has no main process")
	}

	return pid, nil
}

// getPIDOccupyingPort finds the PID of the process occupying a port.
func getPIDOccupyingPort(port int, protocol string) (int, error) {
	var cmd string
	var args []string

	if commandExists("ss") {
		if protocol == "tcp" {
			args = []string{"-tlnp"}
		} else {
			args = []string{"-ulnp"}
		}
		cmd = "ss"
	} else if commandExists("netstat") {
		if protocol == "tcp" {
			args = []string{"-tlnp"}
		} else {
			args = []string{"-ulnp"}
		}
		cmd = "netstat"
	} else {
		return 0, fmt.Errorf("neither ss nor netstat available")
	}

	output, err := runCommand(cmd, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to run %s: %w", cmd, err)
	}

	portStr := fmt.Sprintf(":%d ", port)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, portStr) {
			// Extract PID from the line
			// ss format: ... users:(("process",pid=123,fd=4))
			// netstat format: ... 123/program
			if strings.Contains(line, "pid=") {
				// ss format
				start := strings.Index(line, "pid=")
				if start != -1 {
					start += 4
					end := start
					for end < len(line) && line[end] >= '0' && line[end] <= '9' {
						end++
					}
					if end > start {
						pidStr := line[start:end]
						var pid int
						_, err := fmt.Sscanf(pidStr, "%d", &pid)
						if err == nil {
							return pid, nil
						}
					}
				}
			} else {
				// netstat format
				fields := strings.Fields(line)
				for _, field := range fields {
					if strings.Contains(field, "/") {
						parts := strings.Split(field, "/")
						if len(parts) == 2 {
							var pid int
							_, err := fmt.Sscanf(parts[0], "%d", &pid)
							if err == nil {
								return pid, nil
							}
						}
					}
				}
			}
		}
	}

	return 0, fmt.Errorf("no process found occupying port %d", port)
}
