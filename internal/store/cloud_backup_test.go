package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCloudBackupCredentialsPersistenceAndRestore(t *testing.T) {
	p := filepath.Join(t.TempDir(), "merit.json")
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	c := CloudBackupConfig{URL: "https://cloud.example/备份", Username: "cloud-user", Password: "cloud-secret"}
	if err := s.SaveCloudBackup(c, false); err != nil {
		t.Fatal(err)
	}
	c = s.CloudBackup()
	if c.URL != "https://cloud.example/%E5%A4%87%E4%BB%BD/" {
		t.Fatalf("noncanonical URL: %q", c.URL)
	}
	c.Password = ""
	if err := s.SaveCloudBackup(c, false); err != nil {
		t.Fatal(err)
	}
	if s.CloudBackup().Password != "cloud-secret" {
		t.Fatal("blank password discarded saved credential")
	}
	s, err = Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if s.CloudBackup().Password != "cloud-secret" {
		t.Fatal("credentials lost after restart")
	}
	raw, err := s.Export()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("cloudBackup")) || bytes.Contains(raw, []byte("cloud-secret")) {
		t.Fatal("cloud credentials leaked into export")
	}
	if err := s.Restore([]byte(`{"nodes":[],"cloudBackup":{"url":"https://evil.example/","password":"injected"}}`)); err != nil {
		t.Fatal(err)
	}
	if s.CloudBackup().Password != "cloud-secret" || s.CloudBackup().URL != c.URL {
		t.Fatal("node restore replaced cloud credentials")
	}
	if err := s.SaveCloudBackup(c, true); err != nil {
		t.Fatal(err)
	}
	if s.CloudBackup().Password != "" {
		t.Fatal("explicit clear did not clear password")
	}
}

func TestCloudBackupConfigValidationAndRollback(t *testing.T) {
	p := filepath.Join(t.TempDir(), "merit.json")
	s, _ := Open(p)
	for _, address := range []string{"", "ftp://host/", "https://u:p@host/", "https://host/?token=secret", "https://host/#fragment", "https://host/a/../b", "http://host/", "https://host/a%5cb"} {
		if err := s.SaveCloudBackup(CloudBackupConfig{URL: address}, false); err == nil {
			t.Fatalf("accepted invalid URL %q", address)
		}
	}
	c := CloudBackupConfig{URL: "https://cloud.example/backup/", Username: "user", Password: "saved-secret"}
	if err := s.SaveCloudBackup(c, false); err != nil {
		t.Fatal(err)
	}
	changed := c
	changed.URL = "https://other.example/"
	changed.Password = ""
	if err := s.SaveCloudBackup(changed, false); err == nil {
		t.Fatal("silently sent old credential to new origin")
	}
	if err := os.Mkdir(p+".tmp", 0o700); err != nil {
		t.Fatal(err)
	}
	changed.Password = "new-secret"
	if err := s.SaveCloudBackup(changed, false); err == nil {
		t.Fatal("expected failed disk save")
	}
	if s.CloudBackup() != c {
		t.Fatal("failed save changed in-memory config")
	}
}
