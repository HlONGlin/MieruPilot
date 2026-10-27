package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"merit/internal/model"
)

func TestLastMieruInstanceIsRemoved(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(fmt.Sprintf("partial=%v", partial), func(t *testing.T) {
			a := &Agent{unitDir: t.TempDir(), mitaDir: t.TempDir()}
			unit := filepath.Join(a.unitDir, "merit-mita-23456.service")
			paths := []string{unit}
			for _, suffix := range []string{".json", ".json.applied", ".json.previous"} {
				paths = append(paths, filepath.Join(a.mitaDir, "23456"+suffix))
			}
			for _, path := range paths {
				if err := os.WriteFile(path, []byte("old config"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var commands []string
			a.runCommand = func(name string, args ...string) (string, error) {
				commands = append(commands, name+" "+strings.Join(args, " "))
				return "", nil
			}
			_, results, err := a.applyConfig(&model.DesiredConfig{Partial: partial,
				DisabledPorts: []model.PortInstanceTarget{{ID: "last", Port: 23456}},
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range paths {
				_, err := os.Stat(path)
				if partial && err != nil || !partial && !os.IsNotExist(err) {
					t.Fatalf("unexpected cleanup for %s: %v", path, err)
				}
			}
			if partial {
				if len(commands) != 0 {
					t.Fatalf("partial sync touched another instance: %v", commands)
				}
			} else {
				if !strings.Contains(strings.Join(commands, "\n"), "systemctl disable --now merit-mita-23456.service") {
					t.Fatalf("instance was not stopped: %v", commands)
				}
				if len(results) != 1 || results[0].PortID != "last" || !results[0].OK || results[0].Running {
					t.Fatalf("invalid stop result: %+v", results)
				}
			}
		})
	}
}

func TestProxyFailureFailsTheSync(t *testing.T) {
	a := &Agent{}
	_, results, err := a.applyConfig(&model.DesiredConfig{Partial: true, Ports: []*model.Port{
		{ID: "bad", Port: 23456, ProxyType: "unsupported", Enabled: true},
	}})
	if err == nil || len(results) != 1 || results[0].OK || results[0].Error == "" {
		t.Fatalf("failed deployment was acknowledged: %+v, %v", results, err)
	}
}

func TestProxyUnitAppliesChangesAndRetriesFailures(t *testing.T) {
	a := &Agent{unitDir: t.TempDir()}
	active := false
	failRestart := false
	var commands []string
	a.runCommand = func(name string, args ...string) (string, error) {
		command := name + " " + strings.Join(args, " ")
		commands = append(commands, command)
		if args[0] == "is-active" {
			if active {
				return "active", nil
			}
			return "inactive", fmt.Errorf("inactive")
		}
		if args[0] == "start" || args[0] == "restart" {
			if failRestart {
				return "startup failed", fmt.Errorf("startup failed")
			}
			active = true
		}
		return "", nil
	}
	apply := func(password, certState string) error {
		return a.writeProxyUnit("anytls-test", "test", 23456,
			"/server -p "+password, nil, certState)
	}
	wantCommand := func(fragment string, want bool) {
		t.Helper()
		if strings.Contains(strings.Join(commands, "\n"), fragment) != want {
			t.Fatalf("command %q presence != %v: %v", fragment, want, commands)
		}
	}
	if err := apply("old-secret", "cert1"); err != nil {
		t.Fatal(err)
	}
	wantCommand("systemctl start ", true)
	commands = nil
	if err := apply("old-secret", "cert1"); err != nil {
		t.Fatal(err)
	}
	wantCommand("systemctl restart ", false)
	wantCommand("systemctl start ", false)
	wantCommand("systemctl daemon-reload", false)
	commands = nil
	if err := apply("old-secret", "cert2"); err != nil {
		t.Fatal(err)
	}
	wantCommand("systemctl restart ", true)
	marker := filepath.Join(a.unitDir, "anytls-test.service.applied")
	previous, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	failRestart = true
	if err := apply("new-secret", "cert2"); err == nil {
		t.Fatal("startup failure was ignored")
	}
	after, _ := os.ReadFile(marker)
	if string(previous) != string(after) {
		t.Fatal("failed config was marked applied")
	}
	failRestart = false
	commands = nil
	if err := apply("new-secret", "cert2"); err != nil {
		t.Fatal(err)
	}
	wantCommand("systemctl restart ", true)
	wantCommand("systemctl daemon-reload", true)
	active = false
	commands = nil
	if err := apply("new-secret", "cert2"); err != nil {
		t.Fatal(err)
	}
	wantCommand("systemctl start ", true)
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(a.unitDir, "anytls-test.service"))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("unit does not protect passwords: %v, %v", info, err)
		}
	}
}
