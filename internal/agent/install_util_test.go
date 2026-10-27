package agent

import (
	"archive/zip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMirrorURL(t *testing.T) {
	tests := []struct {
		name   string
		mirror string
		raw    string
		want   string
	}{
		{
			name:   "no mirror keeps url",
			mirror: "",
			raw:    "https://github.com/x/y/releases/download/v1/a.tar.gz",
			want:   "https://github.com/x/y/releases/download/v1/a.tar.gz",
		},
		{
			name:   "blank mirror is ignored",
			mirror: "   ",
			raw:    "https://github.com/x/y",
			want:   "https://github.com/x/y",
		},
		{
			name:   "mirror keeps original scheme",
			mirror: "https://ghfast.top/",
			raw:    "https://github.com/x/y",
			want:   "https://ghfast.top/https://github.com/x/y",
		},
		{
			name:   "mirror without trailing slash",
			mirror: "https://ghfast.top",
			raw:    "https://github.com/x/y",
			want:   "https://ghfast.top/https://github.com/x/y",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mirrorURL(tt.mirror, tt.raw); got != tt.want {
				t.Errorf("mirrorURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSystemdEscapeArg(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{name: "plain token unquoted", arg: "server", want: "server"},
		{name: "path with slashes unquoted", arg: "/usr/local/bin/shadow-tls", want: "/usr/local/bin/shadow-tls"},
		{name: "address unquoted", arg: "0.0.0.0:8388", want: "0.0.0.0:8388"},
		{name: "space forces quotes", arg: "pass word", want: `"pass word"`},
		{name: "empty arg quoted", arg: "", want: `""`},
		{name: "dollar is doubled", arg: "$ecret", want: `"$$ecret"`},
		{name: "percent is doubled", arg: "100%", want: `"100%%"`},
		{name: "double quote escaped", arg: `a"b`, want: `"a\"b"`},
		{name: "backslash escaped", arg: `a\b`, want: `"a\\b"`},
		{name: "semicolon quoted", arg: "a;b", want: `"a;b"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := systemdEscapeArg(tt.arg); got != tt.want {
				t.Errorf("systemdEscapeArg(%q) = %q, want %q", tt.arg, got, tt.want)
			}
		})
	}
}

func TestSystemdExecStartEscapesEveryArgument(t *testing.T) {
	got := systemdExecStart("/usr/local/bin/shadow-tls", "--v3", "server", "--password", "a b")
	want := `/usr/local/bin/shadow-tls --v3 server --password "a b"`
	if got != want {
		t.Errorf("systemdExecStart() = %q, want %q", got, want)
	}
}

func TestSafePortID(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "abc123", want: "abc123"},
		{in: "port-1_a", want: "port-1_a"},
		{in: "us er/id", want: "us-er-id"},
		{in: "../etc/passwd", want: "etc-passwd"},
		{in: "", want: "unknown"},
		{in: "---", want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := safePortID(tt.in); got != tt.want {
				t.Errorf("safePortID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseProxyUnitName(t *testing.T) {
	tests := []struct {
		unit      string
		wantType  string
		wantID    string
		wantMatch bool
	}{
		{unit: "naive-proxy-p1", wantType: "naive", wantID: "p1", wantMatch: true},
		{unit: "anytls-p2", wantType: "anytls", wantID: "p2", wantMatch: true},
		{unit: "shadowtls-p3", wantType: "shadowtls", wantID: "p3", wantMatch: true},
		{unit: "merit-mita-8388", wantMatch: false},
		{unit: "sshd", wantMatch: false},
	}

	for _, tt := range tests {
		t.Run(tt.unit, func(t *testing.T) {
			typ, id, ok := parseProxyUnitName(tt.unit)
			if ok != tt.wantMatch {
				t.Fatalf("parseProxyUnitName(%q) matched = %v, want %v", tt.unit, ok, tt.wantMatch)
			}
			if !tt.wantMatch {
				return
			}
			if typ != tt.wantType || id != tt.wantID {
				t.Errorf("parseProxyUnitName(%q) = (%q, %q), want (%q, %q)", tt.unit, typ, id, tt.wantType, tt.wantID)
			}
		})
	}
}

func TestExtractZipRejectsEscapingPath(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "evil.zip")

	f, err := os.Create(archive)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("../escaped")
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	if _, err := w.Write([]byte("x")); err != nil {
		t.Fatalf("write entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	if err := extractZip(archive, filepath.Join(dir, "out")); err == nil {
		t.Fatal("extractZip() accepted an archive with an escaping path")
	}
}

func TestExtractArchiveAndFindBinary(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "anytls.zip")

	f, err := os.Create(archive)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(f)
	// Release archives keep binaries inside a folder next to a readme.
	entries := map[string]string{
		"readme.md":         "# anytls",
		"bin/anytls-server": "server-binary",
	}
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	out := filepath.Join(dir, "out")
	if err := extractArchive(archive, out); err != nil {
		t.Fatalf("extractArchive() error: %v", err)
	}

	found, err := findBinary(out, "anytls-server")
	if err != nil {
		t.Fatalf("findBinary() error: %v", err)
	}
	if filepath.Base(found) != "anytls-server" {
		t.Errorf("findBinary() = %q, want anytls-server", found)
	}

	if _, err := findBinary(out, "does-not-exist"); err == nil {
		t.Error("findBinary() did not fail for a missing binary")
	}
}

func TestExtractArchiveRejectsUnknownFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shadow-tls-x86_64-unknown-linux-musl")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := extractArchive(path, dir); err == nil {
		t.Error("extractArchive() accepted a bare binary, which must be installed directly")
	}
}

func TestInstallExecutable(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src-binary")
	if err := os.WriteFile(src, []byte("#!/bin/sh\necho ok\n"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dst := filepath.Join(dir, "bin", "installed")

	if err := installExecutable(src, dst); err != nil {
		t.Fatalf("installExecutable() error: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(data) != "#!/bin/sh\necho ok\n" {
		t.Errorf("installed content = %q", string(data))
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat dst: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o755 {
		t.Errorf("installed mode = %v, want 0755", info.Mode().Perm())
	}

	// A second install must overwrite in place through a temporary file.
	if err := os.WriteFile(src, []byte("v2"), 0o644); err != nil {
		t.Fatalf("rewrite src: %v", err)
	}
	if err := installExecutable(src, dst); err != nil {
		t.Fatalf("second installExecutable() error: %v", err)
	}
	data, err = os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst again: %v", err)
	}
	if string(data) != "v2" {
		t.Errorf("installed content after reinstall = %q, want v2", string(data))
	}

	// No temporary files may be left behind.
	entries, err := os.ReadDir(filepath.Dir(dst))
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".merit-install-") {
			t.Errorf("temporary file %q was not cleaned up", entry.Name())
		}
	}
}
