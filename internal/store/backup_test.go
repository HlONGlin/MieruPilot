package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"merit/internal/model"
)

func TestTasksSurviveRestartAndBackupExcludesThem(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s, _ := Open(path)
	if err := s.SetTasks("node", map[string]*model.Task{"sync": {ID: "sync", Kind: model.TaskSync}}); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.PendingTasks()["node"]["sync"].Kind != model.TaskSync {
		t.Fatal("task did not survive restart")
	}
	raw, err := s.Export()
	if err != nil {
		t.Fatal(err)
	}
	var backup map[string]any
	_ = json.Unmarshal(raw, &backup)
	if _, exists := backup["tasks"]; exists {
		t.Fatal("transient tasks leaked into backup")
	}
}

func TestRestorePreservesAdministratorAndPathAndRejectsInvalidBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s, _ := Open(path)
	if err := s.SetAdmin("current", "password"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnsurePanelPath("/panel/current"); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"nodes":[{"id":"n1","apiKey":"k1","ports":[{"id":"p1","port":1234,"protocol":"TCP","enabled":true}]}],"settings":{"panelPath":"/panel/old","subToken":"restored"}}`)
	if err := s.Restore(raw); err != nil {
		t.Fatal(err)
	}
	if s.Admin().Username != "current" || s.data.Settings.PanelPath != "/panel/current" || s.SubToken() != "restored" {
		t.Fatal("restore changed protected identity or lost subscription token")
	}
	if len(s.Nodes()) != 1 {
		t.Fatal("restore failed to install nodes")
	}
	if err := s.Restore([]byte(`{"nodes":[null]}`)); err == nil {
		t.Fatal("accepted invalid backup")
	}
	if len(s.Nodes()) != 1 {
		t.Fatal("invalid restore modified data")
	}
	files, _ := filepath.Glob(path + ".before-restore-*.json")
	if len(files) != 1 {
		t.Fatal("missing pre-restore backup")
	}
	if _, err := os.Stat(files[0]); err != nil {
		t.Fatal(err)
	}
}

func TestSubscriptionRotationPreservesPanelPathAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s, _ := Open(path)
	if _, err := s.EnsurePanelPath("/panel/stable"); err != nil {
		t.Fatal(err)
	}
	first := s.SubToken()
	second, err := s.RotateSubToken()
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("token did not rotate")
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := reopened.EnsurePanelPath("")
	if err != nil || actual != "/panel/stable" {
		t.Fatalf("panel path lost after token rotation: %q %v", actual, err)
	}
}

func TestTaskSaveFailureRollsBackInMemoryQueue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s, _ := Open(path)
	old := map[string]*model.Task{"old": {ID: "old", Kind: model.TaskSync}}
	if err := s.SetTasks("node", old); err != nil {
		t.Fatal(err)
	}
	// An existing directory at the temp-file path forces the atomic save to fail.
	if err := os.Mkdir(path+".tmp", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTasks("node", map[string]*model.Task{"new": {ID: "new"}}); err == nil {
		t.Fatal("expected write failure")
	}
	got := s.PendingTasks()["node"]
	if len(got) != 1 || got["old"] == nil {
		t.Fatal("failed save changed in-memory queue")
	}
}
