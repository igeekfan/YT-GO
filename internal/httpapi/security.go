package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"YT-GO/internal/netpolicy"
)

const (
	maxJSONBodyBytes = 1 << 20
	maxHeaderBytes   = 64 << 10
	maxRedirects     = 10
)

type ipResolver = netpolicy.Resolver

type urlPolicy struct {
	policy *netpolicy.Policy
}

func newURLPolicy(resolver ipResolver) *urlPolicy {
	return &urlPolicy{policy: netpolicy.New(resolver)}
}

// validate checks the initial user-provided URL before it reaches core. This
// protects literal and currently-resolved private addresses, but cannot pin DNS
// or inspect redirects performed inside the external yt-dlp process. Web-mode
// deployments still need an egress policy that blocks private and metadata
// networks for complete SSRF containment.
func (p *urlPolicy) validate(ctx context.Context, rawURL string) (string, error) {
	rawURL = extractURLFromText(rawURL)
	parsed, err := p.policy.ValidateURL(ctx, rawURL)
	if err != nil {
		return "", err
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	return parsed.String(), nil
}

// checkRedirect can be installed on Go HTTP clients used for media requests.
// yt-dlp performs its own networking and is outside the reach of this callback.
func (p *urlPolicy) checkRedirect(req *http.Request, via []*http.Request) error {
	return p.policy.CheckRedirect(req, via)
}

func (p *urlPolicy) validateProxy(ctx context.Context, rawProxy string) error {
	rawProxy = strings.TrimSpace(rawProxy)
	if rawProxy == "" {
		return nil
	}
	parsed, err := url.Parse(rawProxy)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "socks5", "socks5h":
	default:
		return fmt.Errorf("proxy scheme must be http, https, socks5, or socks5h")
	}
	if parsed.Hostname() == "" {
		return fmt.Errorf("proxy host is required")
	}
	// Validate only the proxy network endpoint. Credentials are allowed for an
	// authenticated proxy but are intentionally removed before URL validation.
	endpoint := *parsed
	endpoint.Scheme = "http"
	endpoint.User = nil
	endpoint.Path = ""
	endpoint.RawPath = ""
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	if _, err := p.policy.ValidateURL(ctx, endpoint.String()); err != nil {
		return fmt.Errorf("proxy endpoint is not allowed: %w", err)
	}
	return nil
}

type downloadPolicy struct {
	path string
	root *os.Root
}

func newDownloadPolicy(path string) (*downloadPolicy, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("web download root is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve web download root: %w", err)
	}
	absolute = filepath.Clean(absolute)
	volumeRoot := filepath.Clean(filepath.VolumeName(absolute) + string(filepath.Separator))
	if samePath(absolute, volumeRoot) {
		return nil, fmt.Errorf("web download root must not be a filesystem root")
	}
	if err := os.MkdirAll(absolute, 0o750); err != nil {
		return nil, fmt.Errorf("create web download root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, fmt.Errorf("resolve web download root symlinks: %w", err)
	}
	root, err := os.OpenRoot(resolved)
	if err != nil {
		return nil, fmt.Errorf("open web download root: %w", err)
	}
	return &downloadPolicy{path: resolved, root: root}, nil
}

func (p *downloadPolicy) close() error {
	if p == nil || p.root == nil {
		return nil
	}
	return p.root.Close()
}

func (p *downloadPolicy) ensureDir(input string) (string, error) {
	relative, err := p.relative(input)
	if err != nil {
		return "", err
	}
	if err := p.root.MkdirAll(relative, 0o750); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}
	return p.existingDirRelative(relative)
}

func (p *downloadPolicy) existingDir(input string) (string, error) {
	relative, err := p.relative(input)
	if err != nil {
		return "", err
	}
	return p.existingDirRelative(relative)
}

func (p *downloadPolicy) existingDirRelative(relative string) (string, error) {
	child, err := p.root.OpenRoot(relative)
	if err != nil {
		return "", fmt.Errorf("output directory is outside the web download root or does not exist: %w", err)
	}
	defer child.Close()
	info, err := child.Stat(".")
	if err != nil {
		return "", fmt.Errorf("stat output directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("output path is not a directory")
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(p.path, relative))
	if err != nil {
		return "", fmt.Errorf("resolve output directory: %w", err)
	}
	if !pathWithinRoot(p.path, resolved) {
		return "", fmt.Errorf("output directory is outside the web download root")
	}
	return resolved, nil
}

func (p *downloadPolicy) readDir(input string) (path string, parent string, names []string, err error) {
	relative, err := p.relative(input)
	if err != nil {
		return "", "", nil, err
	}
	resolved, err := p.existingDirRelative(relative)
	if err != nil {
		return "", "", nil, err
	}
	entries, err := fs.ReadDir(p.root.FS(), relative)
	if err != nil {
		return "", "", nil, fmt.Errorf("read output directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			names = append(names, entry.Name())
		}
	}
	if relative != "." {
		parentRelative := filepath.Dir(relative)
		parent = filepath.Join(p.path, parentRelative)
	}
	return resolved, parent, names, nil
}

func (p *downloadPolicy) openFile(input string) (*os.File, os.FileInfo, error) {
	relative, err := p.relative(input)
	if err != nil {
		return nil, nil, err
	}
	file, err := p.root.Open(relative)
	if err != nil {
		return nil, nil, fmt.Errorf("open download file: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, fmt.Errorf("stat download file: %w", err)
	}
	if !info.Mode().IsRegular() {
		file.Close()
		return nil, nil, fmt.Errorf("download path is not a regular file")
	}
	return file, info, nil
}

func validateCookieFile(dataDir, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}
	if strings.TrimSpace(dataDir) == "" {
		return "", fmt.Errorf("data directory is not configured")
	}
	cookiesDir := filepath.Join(dataDir, "cookies")
	if err := os.MkdirAll(cookiesDir, 0o700); err != nil {
		return "", fmt.Errorf("create cookies directory: %w", err)
	}
	if err := os.Chmod(cookiesDir, 0o700); err != nil {
		return "", fmt.Errorf("secure cookies directory: %w", err)
	}
	policy, err := newDownloadPolicy(cookiesDir)
	if err != nil {
		return "", fmt.Errorf("open cookies directory: %w", err)
	}
	defer policy.close()
	relative, err := policy.relative(input)
	if err != nil {
		return "", fmt.Errorf("cookies file must remain in the server cookies directory: %w", err)
	}
	file, info, err := policy.openFile(input)
	if err != nil {
		return "", fmt.Errorf("invalid cookies file: %w", err)
	}
	file.Close()
	if filepath.Separator != '\\' && info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("cookies file permissions must not allow group or other access")
	}
	lexicalPath := filepath.Join(policy.path, relative)
	resolvedPath, err := filepath.EvalSymlinks(lexicalPath)
	if err != nil {
		return "", fmt.Errorf("resolve cookies file: %w", err)
	}
	if !samePath(lexicalPath, resolvedPath) {
		return "", fmt.Errorf("cookies file must not be a symbolic link")
	}
	return resolvedPath, nil
}

func (p *downloadPolicy) relative(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return ".", nil
	}
	var relative string
	if filepath.IsAbs(input) {
		value, err := filepath.Rel(p.path, filepath.Clean(input))
		if err != nil {
			return "", fmt.Errorf("resolve output directory: %w", err)
		}
		relative = value
	} else {
		relative = filepath.Clean(input)
	}
	if relative == "." {
		return relative, nil
	}
	if !filepath.IsLocal(relative) {
		return "", fmt.Errorf("path is outside the web download root")
	}
	return relative, nil
}

func pathWithinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative == "." || filepath.IsLocal(relative)
}

func validateFilenameTemplate(value string) error {
	if value == "" {
		return nil
	}
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("filename template must not have leading or trailing whitespace")
	}
	if len(value) > 512 {
		return fmt.Errorf("filename template is too long")
	}
	if value == "." || value == ".." || filepath.IsAbs(value) || filepath.VolumeName(value) != "" {
		return fmt.Errorf("filename template must be a file name, not a path")
	}
	if strings.ContainsAny(value, `/\\`) {
		return fmt.Errorf("filename template must not contain path separators")
	}
	for _, char := range value {
		if char == 0 || char < 0x20 || char == 0x7f {
			return fmt.Errorf("filename template contains control characters")
		}
	}
	return nil
}

func ValidateListenAddress(address, authToken string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid YTGO_WEB_ADDR %q: %w", address, err)
	}
	if strings.TrimSpace(authToken) != "" {
		return nil
	}
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return nil
	}
	return errors.New("YTGO_AUTH_TOKEN is required when YTGO_WEB_ADDR listens on a non-loopback interface")
}

// NewHTTPServer applies conservative request-side limits. WriteTimeout remains
// disabled because SSE streams and large file responses are intentionally
// long-lived; handlers and reverse proxies should apply route-specific write
// deadlines where appropriate.
func NewHTTPServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    maxHeaderBytes,
	}
}
