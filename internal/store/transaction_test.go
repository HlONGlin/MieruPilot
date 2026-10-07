package store

import (
	"os"
	"path/filepath"
	"testing"

	"merit/internal/model"
)

func TestReadSnapshotsCannotModifyStoredNodes(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "data.json"))
	node := &model.Node{ID: "n", APIKey: "key", Name: "original", Ports: []*model.Port{{ID: "p", Password: "secret"}}}
	if err := s.AddNode(node); err != nil {
		t.Fatal(err)
	}
	node.Name = "caller mutation"
	read := s.Node("n")
	read.Name, read.Ports[0].Password = "mutation", "mutation"
	s.Nodes()[0].Name = "list mutation"
	s.NodeByAPIKey("key").Name = "key mutation"
	actual := s.Node("n")
	if actual.Name != "original" || actual.Ports[0].Password != "secret" {
		t.Fatalf("internal node escaped: %+v", actual)
	}
}

func TestNodeMutationsRollBackOnDiskFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s, _ := Open(path)
	if err := s.AddNode(&model.Node{ID: "n", Name: "original", APIKey: "key"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".tmp", 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update("n", func(n *model.Node) error { n.Name = "new"; return nil }); err == nil {
		t.Fatal("expected update failure")
	}
	if err := s.Delete("n"); err == nil {
		t.Fatal("expected delete failure")
	}
	if err := s.AddNode(&model.Node{ID: "other"}); err == nil {
		t.Fatal("expected add failure")
	}
	if s.Node("n").Name != "original" || s.Node("other") != nil || len(s.Nodes()) != 1 {
		t.Fatal("failed operation changed memory")
	}
	reopened, err := Open(path)
	if err != nil || reopened.Node("n").Name != "original" {
		t.Fatal("failed operation changed disk")
	}
}
