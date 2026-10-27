package manager

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"merit/internal/model"
	"merit/internal/store"
)

type fakeCloudDAV struct {
	mu           sync.Mutex
	files        map[string][]byte
	directory    bool
	putStatus    int
	corruptReads bool
	server       *httptest.Server
}

func newFakeCloudDAV(t *testing.T) *fakeCloudDAV {
	f := &fakeCloudDAV{files: map[string][]byte{}}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		u, p, ok := r.BasicAuth()
		if !ok || u != "cloud-user" || p != "cloud-password" {
			w.WriteHeader(401)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/备份/") {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/备份/")
		if r.Method == "MKCOL" && name == "" {
			f.directory = true
			w.WriteHeader(201)
			return
		}
		if !f.directory {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case "PROPFIND":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(207)
			io.WriteString(w, `<d:multistatus xmlns:d="DAV:">`)
			if r.Header.Get("Depth") == "1" {
				names := []string{}
				for name := range f.files {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					href := (&url.URL{Path: "/备份/" + name}).String()
					fmt.Fprintf(w, `<d:response><d:href>%s</d:href><d:propstat><d:prop><d:getcontentlength>%d</d:getcontentlength><d:getlastmodified>Mon, 26 Oct 2026 12:00:00 GMT</d:getlastmodified><d:resourcetype/></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`, href, len(f.files[name]))
				}
			}
			io.WriteString(w, `</d:multistatus>`)
		case "PUT":
			if f.putStatus != 0 {
				w.WriteHeader(f.putStatus)
				return
			}
			if _, exists := f.files[name]; exists && r.Header.Get("If-None-Match") == "*" {
				w.WriteHeader(412)
				return
			}
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(400)
				return
			}
			f.files[name] = raw
			w.WriteHeader(201)
		case "GET":
			raw, ok := f.files[name]
			if !ok {
				http.NotFound(w, r)
				return
			}
			if f.corruptReads {
				raw = []byte("corrupted response")
			}
			w.Write(raw)
		case "DELETE":
			delete(f.files, name)
			w.WriteHeader(204)
		default:
			w.WriteHeader(405)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func newCloudManager(t *testing.T, f *fakeCloudDAV) (*Server, *httptest.Server, string) {
	t.Helper()
	s, err := New(Config{DataPath: filepath.Join(t.TempDir(), "merit.json"), PanelPath: "/panel/test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.SetAdmin("current-admin", "current-password"); err != nil {
		t.Fatal(err)
	}
	if err := s.store.SaveCloudBackup(store.CloudBackupConfig{URL: f.server.URL + "/备份/", Username: "cloud-user", Password: "cloud-password", AllowHTTP: true}, false); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return s, ts, s.newSession()
}

func cloudAPICall(t *testing.T, ts *httptest.Server, session, method, endpoint, body string, status int) []byte {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+"/panel/test/api/backup/webdav"+endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if session != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: session})
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != status {
		t.Fatalf("%s %s: status=%d want=%d body=%s", method, endpoint, res.StatusCode, status, raw)
	}
	return raw
}

func TestCloudBackupUploadListAndRestore(t *testing.T) {
	f := newFakeCloudDAV(t)
	s, ts, session := newCloudManager(t, f)
	n := &model.Node{ID: "n1", APIKey: "node-key", Name: "before", Ports: []*model.Port{{ID: "p1", Port: 1234, Protocol: model.ProtocolTCP, Enabled: true, Username: "node-user", Password: "node-secret"}}}
	if err := s.store.AddNode(n); err != nil {
		t.Fatal(err)
	}
	cloudAPICall(t, ts, session, "POST", "/test", "", 200)
	f.mu.Lock()
	if len(f.files) != 0 || !f.directory {
		t.Fatal("connection probe not cleaned up")
	}
	f.mu.Unlock()
	cloudAPICall(t, ts, session, "POST", "/upload", "", 200)
	cloudAPICall(t, ts, session, "POST", "/upload", "", 200)
	raw := cloudAPICall(t, ts, session, "GET", "/files", "", 200)
	var files []cloudBackupFile
	if err := json.Unmarshal(raw, &files); err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].Name <= files[1].Name || files[0].Modified == "" {
		t.Fatalf("invalid listing: %s", raw)
	}
	f.mu.Lock()
	backup := append([]byte(nil), f.files[files[0].Name]...)
	f.mu.Unlock()
	if !bytes.Contains(backup, []byte("node-secret")) || bytes.Contains(backup, []byte("cloudBackup")) || bytes.Contains(backup, []byte("cloud-password")) {
		t.Fatal("wrong backup content or cloud credentials leaked")
	}
	if _, err := s.store.Update("n1", func(n *model.Node) error { n.Name = "after"; return nil }); err != nil {
		t.Fatal(err)
	}
	cloudAPICall(t, ts, session, "POST", "/restore", fmt.Sprintf(`{"name":%q}`, files[0].Name), 200)
	if s.store.Node("n1").Name != "before" || !s.store.VerifyAdmin("current-admin", "current-password") || s.store.CloudBackup().Password != "cloud-password" {
		t.Fatal("restore lost configuration or protected identity")
	}
	if len(s.store.PendingTasks()["n1"]) != 1 {
		t.Fatal("restored node not resynced")
	}
	paths, _ := filepath.Glob(s.cfg.DataPath + ".before-restore-*.json")
	if len(paths) != 1 {
		t.Fatal("missing local pre-restore backup")
	}
}

func TestCloudBackupAdminOnlyAndSettingsRedaction(t *testing.T) {
	f := newFakeCloudDAV(t)
	_, ts, session := newCloudManager(t, f)
	for _, req := range []struct{ method, path string }{{"GET", ""}, {"PUT", ""}, {"GET", "/files"}, {"POST", "/upload"}, {"POST", "/restore"}, {"POST", "/test"}} {
		cloudAPICall(t, ts, "", req.method, req.path, "{}", 401)
	}
	raw := cloudAPICall(t, ts, session, "GET", "", "", 200)
	if bytes.Contains(raw, []byte("cloud-password")) || bytes.Contains(raw, []byte(`"password"`)) {
		t.Fatal("password leaked in settings")
	}
	if !bytes.Contains(raw, []byte(`"hasPassword":true`)) {
		t.Fatal("password flag missing")
	}
	cloudAPICall(t, ts, session, "PUT", "", fmt.Sprintf(`{"url":%q,"username":"cloud-user","password":"","allowHTTP":true}`, f.server.URL+"/备份/"), 200)
	cloudAPICall(t, ts, session, "POST", "/test", "", 200)
	res, err := ts.Client().Get(ts.URL + "/dav/")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("obsolete DAV server still exposed: %d", res.StatusCode)
	}
}

func TestCloudBackupRejectsFailuresWithoutChangingNodes(t *testing.T) {
	f := newFakeCloudDAV(t)
	s, ts, session := newCloudManager(t, f)
	if err := s.store.AddNode(&model.Node{ID: "keep", Name: "keep", APIKey: "keep-key"}); err != nil {
		t.Fatal(err)
	}
	name, _ := cloudUniqueName("merit-backup-", ".json")
	f.mu.Lock()
	f.directory = true
	f.files[name] = []byte(`{"nodes":[null]}`)
	f.mu.Unlock()
	cloudAPICall(t, ts, session, "POST", "/restore", fmt.Sprintf(`{"name":%q}`, name), 400)
	if s.store.Node("keep") == nil {
		t.Fatal("bad restore changed nodes")
	}
	f.mu.Lock()
	f.files[name] = []byte(`{"nodes":[{"id":"x","apiKey":"x","egress":{"proxies":[{"id":"bad","protocol":"BAD","host":"x","port":1}]}}]}`)
	f.mu.Unlock()
	cloudAPICall(t, ts, session, "POST", "/restore", fmt.Sprintf(`{"name":%q}`, name), 400)
	f.mu.Lock()
	f.files[name] = bytes.Repeat([]byte("x"), maxCloudBackup+1)
	f.mu.Unlock()
	cloudAPICall(t, ts, session, "POST", "/restore", fmt.Sprintf(`{"name":%q}`, name), 400)
	cloudAPICall(t, ts, session, "POST", "/restore", `{"name":"../../merit.json"}`, 400)
	if s.store.Node("keep") == nil {
		t.Fatal("invalid restores changed nodes")
	}
	f.mu.Lock()
	f.putStatus = 507
	f.mu.Unlock()
	cloudAPICall(t, ts, session, "POST", "/upload", "", 502)
	f.mu.Lock()
	f.putStatus = 0
	f.corruptReads = true
	f.mu.Unlock()
	raw := cloudAPICall(t, ts, session, "POST", "/upload", "", 502)
	if !bytes.Contains(raw, []byte("校验失败")) {
		t.Fatalf("corrupt remote backup reported success: %s", raw)
	}
	cloudAPICall(t, ts, session, "POST", "/test", "", 502)
	f.mu.Lock()
	defer f.mu.Unlock()
	for name := range f.files {
		if strings.HasPrefix(name, ".merit-probe-") {
			t.Fatal("failed probe not cleaned up")
		}
	}
}

func TestCloudBackupRejectsRedirectsAndConcurrentOperations(t *testing.T) {
	f := newFakeCloudDAV(t)
	s, ts, session := newCloudManager(t, f)
	s.cloudMu.Lock()
	cloudAPICall(t, ts, session, "POST", "/upload", "", 409)
	s.cloudMu.Unlock()
	var redirected bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected = true; w.WriteHeader(200) }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", target.URL+"/")
		w.WriteHeader(307)
	}))
	defer redirect.Close()
	if err := s.store.SaveCloudBackup(store.CloudBackupConfig{URL: redirect.URL + "/", Username: "cloud-user", Password: "cloud-password", AllowHTTP: true}, false); err != nil {
		t.Fatal(err)
	}
	raw := cloudAPICall(t, ts, session, "POST", "/upload", "", 502)
	if redirected || !bytes.Contains(raw, []byte("重定向")) {
		t.Fatal("credentials or backup followed a redirect")
	}
}

func TestCloudBackupDAVListFiltersUntrustedHrefs(t *testing.T) {
	name, _ := cloudUniqueName("merit-backup-", ".json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(207)
		io.WriteString(w, `<d:multistatus xmlns:d="DAV:">`)
		for _, href := range []string{"/backups/" + name, "https://evil.example/backups/" + name, "/outside/" + name, "/backups/nested/" + name, "/backups/other.json"} {
			var escaped bytes.Buffer
			xml.EscapeText(&escaped, []byte(href))
			fmt.Fprintf(w, `<d:response><d:href>%s</d:href><d:propstat><d:prop><d:getcontentlength>123</d:getcontentlength></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`, escaped.String())
		}
		io.WriteString(w, `</d:multistatus>`)
	}))
	defer server.Close()
	d, err := newCloudDAV(store.CloudBackupConfig{URL: server.URL + "/backups/", AllowHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	files, err := d.list(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name != name {
		t.Fatalf("accepted untrusted href: %+v", files)
	}
}
