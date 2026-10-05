package manager

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestFirstSetupAndLogin(t *testing.T) {
	dataPath := filepath.Join(t.TempDir(), "merit.json")
	s, err := New(Config{DataPath: dataPath, AgentDir: t.TempDir(), PanelPath: "/panel/test"})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	getMe := func() map[string]any {
		res, err := http.Get(ts.URL + "/panel/test/api/me")
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var got map[string]any
		if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := getMe(); got["setup"] != true {
		t.Fatalf("expected setup=true, got %#v", got)
	}

	body := strings.NewReader(`{"username":"admin","password":"password"}`)
	res, err := http.Post(ts.URL+"/panel/test/api/setup", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("setup status = %d", res.StatusCode)
	}
	if got := getMe(); got["setup"] != false {
		t.Fatalf("expected setup=false, got %#v", got)
	}
}
