package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFindAssetForCurrentPlatform(t *testing.T) {
	r := &Release{Assets: []Asset{{Name: "merit-manager-windows-amd64.exe"}, {Name: "merit-manager-linux-amd64"}}}
	asset := r.FindAsset("manager")
	if asset == nil {
		t.Skip("测试平台没有对应的固定资产")
	}
	if asset.Name == "" {
		t.Fatal("asset name is empty")
	}
}

func TestIsNewer(t *testing.T) {
	if !isNewer("v1.3.0", "v1.2.0") {
		t.Fatal("expected newer release")
	}
	if isNewer("v1.2.0", "v1.2.0") {
		t.Fatal("same release is not newer")
	}
	if !isNewer("v1.10.0", "v1.9.0") {
		t.Fatal("semantic version comparison should compare numeric components")
	}
	if isNewer("v1.5.0", "v1.10.0") {
		t.Fatal("older major/minor versions must not be treated as newer")
	}
	if isNewer("latest", "v1.0.0") {
		t.Fatal("invalid version strings must not be treated as newer")
	}
}

func TestLatestReleaseRejectsNonJSONGatewayResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusGatewayTimeout)
		_, _ = w.Write([]byte("<!doctype html><title>Gateway time-out</title>"))
	}))
	defer server.Close()

	oldURL := releaseAPIURL
	releaseAPIURL = server.URL
	defer func() { releaseAPIURL = oldURL }()
	resetReleaseCache()

	_, err := LatestRelease()
	if err == nil {
		t.Fatal("expected gateway error")
	}
	if got := err.Error(); got != "GitHub API returned status 504" {
		t.Fatalf("expected concise status error, got %q", got)
	}
}

func TestLatestReleaseRequestTimesOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.5.0"}`))
	}))
	defer server.Close()

	oldURL := releaseAPIURL
	oldTimeout := releaseRequestTimeout
	releaseAPIURL = server.URL
	releaseRequestTimeout = 20 * time.Millisecond
	defer func() {
		releaseAPIURL = oldURL
		releaseRequestTimeout = oldTimeout
	}()
	resetReleaseCache()

	_, err := LatestRelease()
	if err == nil {
		t.Fatal("expected request timeout")
	}
}

func TestLatestReleaseCachesSuccessfulResponse(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Release{TagName: "v1.5.0", PublishedAt: time.Now()})
	}))
	defer server.Close()

	oldURL := releaseAPIURL
	releaseAPIURL = server.URL
	defer func() { releaseAPIURL = oldURL }()
	resetReleaseCache()

	for i := 0; i < 2; i++ {
		release, err := LatestRelease()
		if err != nil || release.TagName != "v1.5.0" {
			t.Fatalf("unexpected latest release response: release=%+v err=%v", release, err)
		}
	}
	if requests != 1 {
		t.Fatalf("expected cached second request, got %d upstream requests", requests)
	}
}
