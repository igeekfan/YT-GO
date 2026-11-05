package core

import "testing"

func TestCompareVersion(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
		want  int
	}{
		{name: "newer patch", left: "1.2.3", right: "1.2.2", want: 1},
		{name: "older minor", left: "1.1.9", right: "1.2.0", want: -1},
		{name: "missing patch is zero", left: "1.2", right: "1.2.0", want: 0},
		{name: "equal", left: "2.0.0", right: "2.0.0", want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := compareVersion(test.left, test.right); got != test.want {
				t.Fatalf("compareVersion(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
			}
		})
	}
}

func TestSelectUpdateAssetPrefersInstallablePackage(t *testing.T) {
	assets := []githubReleaseAsset{
		{Name: "YT-GO_1.2.0_linux_amd64_webkit2_41.deb", BrowserDownloadURL: "https://github.com/example/webkit.deb"},
		{Name: "YT-GO_1.2.0_linux_amd64.deb", BrowserDownloadURL: "https://github.com/example/linux.deb"},
		{Name: "YT-GO_1.2.0_x86_64.AppImage", BrowserDownloadURL: "https://github.com/example/linux.AppImage"},
	}

	asset, ok := selectUpdateAsset("linux", "amd64", assets)
	if !ok {
		t.Fatal("expected a Linux update asset")
	}
	if asset.Name != "YT-GO_1.2.0_x86_64.AppImage" {
		t.Fatalf("selected asset %q, want AppImage", asset.Name)
	}
}

func TestSelectUpdateAssetPrefersWindowsInstaller(t *testing.T) {
	assets := []githubReleaseAsset{
		{Name: "YTGO_Portable_1.2.0_windows_x64.zip", BrowserDownloadURL: "https://github.com/example/portable.zip"},
		{Name: "YTGO_Setup_1.2.0_windows_x64.exe", BrowserDownloadURL: "https://github.com/example/setup.exe"},
	}

	asset, ok := selectUpdateAsset("windows", "amd64", assets)
	if !ok {
		t.Fatal("expected a Windows update asset")
	}
	if asset.Name != "YTGO_Setup_1.2.0_windows_x64.exe" {
		t.Fatalf("selected asset %q, want installer", asset.Name)
	}
}

func TestUpdateDigest(t *testing.T) {
	valid := "sha256:454f2a1fdcd79311f2d03e6ecd6c1c2192e2a9b8cdaed3e68da65f6aa2ea98c2"
	value, ok := updateDigest(valid)
	if !ok || value == "" {
		t.Fatalf("updateDigest(%q) = %q, %v, want a valid digest", valid, value, ok)
	}

	for _, digest := range []string{"", "md5:454f2a1f", "sha256:not-hex"} {
		if _, ok := updateDigest(digest); ok {
			t.Fatalf("updateDigest(%q) accepted an invalid digest", digest)
		}
	}
}

func TestIsAllowedUpdateURL(t *testing.T) {
	for _, rawURL := range []string{
		"https://github.com/igeekfan/YT-GO/releases/download/v1.2.0/update.exe",
		"https://release-assets.githubusercontent.com/update.exe",
	} {
		if !isAllowedUpdateURL(rawURL) {
			t.Fatalf("isAllowedUpdateURL(%q) = false, want true", rawURL)
		}
	}

	for _, rawURL := range []string{
		"http://github.com/igeekfan/YT-GO/update.exe",
		"https://example.com/update.exe",
	} {
		if isAllowedUpdateURL(rawURL) {
			t.Fatalf("isAllowedUpdateURL(%q) = true, want false", rawURL)
		}
	}
}
