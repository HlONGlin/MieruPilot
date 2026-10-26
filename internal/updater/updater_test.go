package updater

import "testing"

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
}
