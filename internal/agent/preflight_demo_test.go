package agent

import (
	"testing"

	"merit/internal/model"
)

// This test demonstrates the preflight failure output for port occupancy scenarios
func TestPreflightCheck_Demo_PortOccupancyFailures(t *testing.T) {
	t.Run("NewPortOccupiedByUnknownProcess", func(t *testing.T) {
		mock := newMockChecker()
		mock.commands["systemctl"] = true
		mock.commands["dpkg"] = true
		mock.commandOut["systemctl is-system-running"] = "running"
		// Port occupied but occupier name unknown
		mock.portsInUse["23456:tcp"] = ""
		mock.portPIDs["23456:tcp"] = 999

		pc := mock.toPreflightChecker()

		port := &model.Port{
			Port:      23456,
			Protocol:  "TCP",
			ProxyType: model.ProxyTypeMieru,
		}

		result := pc.CheckForPort(port, "merit-mita-23456")
		if result == nil {
			t.Error("Expected port check to fail")
			return
		}

		t.Logf("CheckName: %s", result.CheckName)
		t.Logf("Pass: %v", result.Pass)
		t.Logf("Error: %s", result.Error)
		t.Logf("Hint: %s", result.Hint)
	})

	t.Run("ExistingPortOccupiedByOtherAfterRestart", func(t *testing.T) {
		mock := newMockChecker()
		mock.commands["systemctl"] = true
		mock.commands["dpkg"] = true
		mock.commandOut["systemctl is-system-running"] = "running"
		// Port occupied by nginx (simulates another service took the port after restart)
		mock.portsInUse["23456:tcp"] = "nginx"
		mock.portPIDs["23456:tcp"] = 888
		// Unit exists but has different PID (stopped or crashed)
		mock.units["merit-mita-23456"] = true
		mock.unitPIDs["merit-mita-23456"] = 1234

		pc := mock.toPreflightChecker()

		port := &model.Port{
			Port:      23456,
			Protocol:  "TCP",
			ProxyType: model.ProxyTypeMieru,
		}

		result := pc.CheckForPort(port, "merit-mita-23456")
		if result == nil {
			t.Error("Expected port check to fail")
			return
		}

		t.Logf("CheckName: %s", result.CheckName)
		t.Logf("Pass: %v", result.Pass)
		t.Logf("Error: %s", result.Error)
		t.Logf("Hint: %s", result.Hint)
	})
}
