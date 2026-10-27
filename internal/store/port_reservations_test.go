package store

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"merit/internal/model"
)

func TestRestoreRejectsManagedBackendConflictBeforeMutation(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddNode(&model.Node{ID: "original", APIKey: "old"}); err != nil {
		t.Fatal(err)
	}
	n := &model.Node{ID: "replacement", APIKey: "new", Ports: []*model.Port{
		{ID: "shadow", Port: 443, Protocol: model.ProtocolTCP, ProxyType: model.ProxyTypeShadowTLS, BackendManaged: true, BackendAddr: "127.0.0.1:50000"},
		{ID: "conflict", Port: 50000, Protocol: model.ProtocolTCP, ProxyType: model.ProxyTypeAnyTLS},
	}}
	raw, _ := json.Marshal(map[string]any{"nodes": []*model.Node{n}})
	if err := s.Restore(raw); err == nil {
		t.Fatal("conflicting backup accepted")
	}
	if s.Node("original") == nil || s.Node("replacement") != nil {
		t.Fatal("failed restore mutated data")
	}
	n.Ports[1].ProxyType = model.ProxyTypeMieru
	n.Ports[1].Protocol = model.ProtocolUDP
	raw, _ = json.Marshal(map[string]any{"nodes": []*model.Node{n}})
	if err := s.Restore(raw); err != nil {
		t.Fatalf("separate UDP and TCP sockets should coexist: %v", err)
	}
}
