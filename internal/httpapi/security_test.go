package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type sequenceResolver struct {
	calls atomic.Int32
}

func (r *sequenceResolver) LookupIPAddr(_ context.Context, _ string) ([]net.IPAddr, error) {
	if r.calls.Add(1) == 1 {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
	}
	return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
}

type mapResolver struct {
	addresses map[string][]net.IPAddr
	err       error
}

func (r mapResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.addresses[host], nil
}

func TestURLPolicyAllowsPublicVideoPlatformURL(t *testing.T) {
	policy := newURLPolicy(mapResolver{addresses: map[string][]net.IPAddr{
		"www.youtube.com": {{IP: net.ParseIP("8.8.8.8")}},
	}})

	got, err := policy.validate(context.Background(), "https://www.youtube.com/watch?v=abc")
	if err != nil {
		t.Fatalf("expected public URL to pass: %v", err)
	}
	if got != "https://www.youtube.com/watch?v=abc" {
		t.Fatalf("unexpected normalized URL: %q", got)
	}
}

func TestURLPolicyNormalizesSchemeAndAllowsPublicIPv6(t *testing.T) {
	policy := newURLPolicy(mapResolver{})
	got, err := policy.validate(context.Background(), "HTTPS://[2606:4700:4700::1111]/video")
	if err != nil {
		t.Fatalf("expected public IPv6 URL to pass: %v", err)
	}
	if got != "https://[2606:4700:4700::1111]/video" {
		t.Fatalf("unexpected normalized URL: %q", got)
	}
}

func TestURLPolicyBlocksPrivateLiteralAndDNSAnswers(t *testing.T) {
	policy := newURLPolicy(mapResolver{addresses: map[string][]net.IPAddr{
		"mixed.example": {
			{IP: net.ParseIP("8.8.8.8")},
			{IP: net.ParseIP("10.0.0.8")},
		},
	}})

	for _, rawURL := range []string{
		"http://127.0.0.1/admin",
		"http://[::1]/admin",
		"http://[::ffff:127.0.0.1]/admin",
		"http://[fc00::1]/admin",
		"http://[fe80::1]/admin",
		"http://[64:ff9b::7f00:1]/admin",
		"http://[2002:0a00:0001::]/admin",
		"http://169.254.169.254/latest/meta-data",
		"http://100.64.0.1/video",
		"http://192.0.2.1/video",
		"http://mixed.example/video",
		"http://metadata.google.internal/computeMetadata/v1",
	} {
		if _, err := policy.validate(context.Background(), rawURL); err == nil {
			t.Errorf("expected %q to be rejected", rawURL)
		}
	}
}

func TestURLPolicyRevalidatesDNSOnEveryBoundaryCheck(t *testing.T) {
	resolver := &sequenceResolver{}
	policy := newURLPolicy(resolver)
	if _, err := policy.validate(context.Background(), "https://rebind.example/video"); err != nil {
		t.Fatalf("first public answer should pass: %v", err)
	}
	if _, err := policy.validate(context.Background(), "https://rebind.example/video"); err == nil {
		t.Fatal("second private answer should be rejected")
	}
}

func TestURLPolicyRejectsUnsafeURLShapes(t *testing.T) {
	policy := newURLPolicy(mapResolver{})
	for _, rawURL := range []string{
		"file:///etc/passwd",
		"http:///missing-host",
		"https://user:password@example.com/video",
	} {
		if _, err := policy.validate(context.Background(), rawURL); err == nil {
			t.Errorf("expected %q to be rejected", rawURL)
		}
	}
}

func TestURLPolicyChecksRedirectTargets(t *testing.T) {
	policy := newURLPolicy(mapResolver{})
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/internal", nil)
	if err := policy.checkRedirect(req, nil); err == nil {
		t.Fatal("expected redirect to loopback to be rejected")
	}

	via := make([]*http.Request, maxRedirects)
	req = httptest.NewRequest(http.MethodGet, "https://example.com/video", nil)
	if err := policy.checkRedirect(req, via); err == nil {
		t.Fatal("expected redirect limit to be enforced")
	}
}

func TestDownloadPolicyConfinesDirectoriesToRoot(t *testing.T) {
	base := t.TempDir()
	policy, err := newDownloadPolicy(filepath.Join(base, "downloads"))
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}
	defer policy.close()

	inside, err := policy.ensureDir(filepath.Join("shows", "season-1"))
	if err != nil {
		t.Fatalf("create safe subdirectory: %v", err)
	}
	if !pathWithinRoot(policy.path, inside) {
		t.Fatalf("expected %q to remain beneath %q", inside, policy.path)
	}

	for _, candidate := range []string{
		filepath.Join("..", "outside"),
		filepath.Join(base, "outside"),
	} {
		if _, err := policy.ensureDir(candidate); err == nil {
			t.Errorf("expected %q to be rejected", candidate)
		}
	}
}

func TestDownloadPolicyRejectsFilesystemRoot(t *testing.T) {
	volumeRoot := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	if _, err := newDownloadPolicy(volumeRoot); err == nil {
		t.Fatalf("expected filesystem root %q to be rejected", volumeRoot)
	}
}

func TestDownloadPolicyRejectsEscapingSymlink(t *testing.T) {
	base := t.TempDir()
	rootPath := filepath.Join(base, "downloads")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(outside, 0o750); err != nil {
		t.Fatal(err)
	}
	policy, err := newDownloadPolicy(rootPath)
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}
	defer policy.close()
	if err := os.Symlink(outside, filepath.Join(rootPath, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if _, err := policy.existingDir("escape"); err == nil {
		t.Fatal("expected symlink escaping the root to be rejected")
	}
}

func TestValidateFilenameTemplate(t *testing.T) {
	for _, value := range []string{"", "%(title)s.%(ext)s", "%(uploader)s - %(title)s.%(ext)s"} {
		if err := validateFilenameTemplate(value); err != nil {
			t.Errorf("expected %q to be accepted: %v", value, err)
		}
	}
	for _, value := range []string{"../video.%(ext)s", `..\video.%(ext)s`, "/tmp/video.%(ext)s", ".."} {
		if err := validateFilenameTemplate(value); err == nil {
			t.Errorf("expected %q to be rejected", value)
		}
	}
	for _, value := range []string{".", "bad\nname.%(ext)s", "\n%(title)s.%(ext)s", " %(title)s.%(ext)s", strings.Repeat("a", 513)} {
		if err := validateFilenameTemplate(value); err == nil {
			t.Errorf("expected %q to be rejected", value)
		}
	}
}

func TestValidateListenAddressRequiresAuthOffLoopback(t *testing.T) {
	if err := ValidateListenAddress("127.0.0.1:8080", ""); err != nil {
		t.Fatalf("loopback listener should be allowed: %v", err)
	}
	if err := ValidateListenAddress("[::1]:8080", ""); err != nil {
		t.Fatalf("IPv6 loopback listener should be allowed: %v", err)
	}
	if err := ValidateListenAddress(":8080", ""); err == nil {
		t.Fatal("wildcard listener without auth should be rejected")
	}
	if err := ValidateListenAddress("0.0.0.0:8080", "strong-token"); err != nil {
		t.Fatalf("authenticated public listener should be allowed: %v", err)
	}
}

func TestNewHTTPServerAppliesRequestLimits(t *testing.T) {
	server := NewHTTPServer("127.0.0.1:0", http.NotFoundHandler())
	if server.ReadHeaderTimeout != 5*time.Second || server.ReadTimeout != 30*time.Second || server.IdleTimeout != 90*time.Second {
		t.Fatalf("unexpected HTTP timeouts: %+v", server)
	}
	if server.WriteTimeout != 0 {
		t.Fatalf("write timeout must remain disabled for SSE, got %s", server.WriteTimeout)
	}
	if server.MaxHeaderBytes != maxHeaderBytes {
		t.Fatalf("expected max header bytes %d, got %d", maxHeaderBytes, server.MaxHeaderBytes)
	}
}

func TestHandlerRejectsOversizedJSONBody(t *testing.T) {
	server := newTestServer(t)
	body := `{"lang":"` + strings.Repeat("a", maxJSONBodyBytes) + `"}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/lang", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, recorder.Code)
	}
}

func TestBrowseDirectoryAllowsRootAndHidesEscapes(t *testing.T) {
	server := newTestServer(t)
	if err := os.Mkdir(filepath.Join(server.DownloadRoot(), "Videos"), 0o750); err != nil {
		t.Fatal(err)
	}
	requestBody, _ := json.Marshal(map[string]string{"path": server.DownloadRoot()})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/settings/browse-dir", strings.NewReader(string(requestBody)))

	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	var response struct {
		Path   string   `json:"path"`
		Parent string   `json:"parent"`
		Dirs   []string `json:"dirs"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Path != server.DownloadRoot() || response.Parent != "" {
		t.Fatalf("unexpected root response: %+v", response)
	}
	if len(response.Dirs) != 1 || response.Dirs[0] != "Videos" {
		t.Fatalf("unexpected directories: %v", response.Dirs)
	}
}

func TestDownloadRequestRejectsEscapingOutputAndTemplate(t *testing.T) {
	server := newTestServer(t)
	outside := filepath.Join(filepath.Dir(server.DownloadRoot()), "outside")

	requests := []map[string]any{
		{"url": "https://video.example/watch/1", "outputDir": outside, "quality": "best"},
		{"url": "https://video.example/watch/1", "outputDir": server.DownloadRoot(), "quality": "best", "options": map[string]any{"filenameTemplate": "../escape.%(ext)s"}},
	}
	for _, body := range requests {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/downloads", strings.NewReader(string(encoded)))
		server.Handler().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
		}
	}
}

func TestURLPolicyReportsResolverFailure(t *testing.T) {
	policy := newURLPolicy(mapResolver{err: errors.New("resolver unavailable")})
	if _, err := policy.validate(context.Background(), "https://video.example/watch/1"); err == nil {
		t.Fatal("expected resolver failure")
	}
}

func TestURLPolicyValidatesProxyEndpoint(t *testing.T) {
	policy := newURLPolicy(mapResolver{addresses: map[string][]net.IPAddr{
		"proxy.example": {{IP: net.ParseIP("8.8.8.8")}},
		"private.example": {
			{IP: net.ParseIP("8.8.8.8")},
			{IP: net.ParseIP("10.0.0.2")},
		},
	}})
	for _, value := range []string{
		"",
		"http://user:password@proxy.example:8080",
		"https://proxy.example:8443",
		"socks5h://proxy.example:1080",
	} {
		if err := policy.validateProxy(context.Background(), value); err != nil {
			t.Errorf("expected proxy %q to pass: %v", value, err)
		}
	}
	for _, value := range []string{
		"file:///tmp/socket",
		"http://127.0.0.1:8080",
		"socks5://[::1]:1080",
		"http://private.example:8080",
		"http:///missing-host",
	} {
		if err := policy.validateProxy(context.Background(), value); err == nil {
			t.Errorf("expected proxy %q to be rejected", value)
		}
	}
}
