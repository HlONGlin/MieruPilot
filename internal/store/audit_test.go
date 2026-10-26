package store

import (
	"path/filepath"
	"testing"
	"time"

	"merit/internal/model"
)

func TestAuditPersistsAcrossOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "merit.json")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	entry := model.AuditEntry{Time: time.Now().UTC(), Actor: "admin", Action: "新增节点", Target: "edge-a"}
	if err := st.AddAudit(entry); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	entries := reopened.Audit(10)
	if len(entries) != 1 || entries[0] != entry {
		t.Fatalf("audit entry did not persist: %+v", entries)
	}
}
