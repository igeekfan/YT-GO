package netpolicy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultLookupTimeout = 5 * time.Second
	defaultDialTimeout   = 10 * time.Second
	defaultMaxRedirects  = 10
)

// Resolver is the DNS capability required by Policy.
type Resolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type dialContextFunc func(context.Context, string, string) (net.Conn, error)

// Policy rejects URLs and connections that can reach local, private, metadata,
// or reserved networks. Hostnames are resolved again immediately before dial
// and the validated IP is dialled directly, closing the DNS rebinding window
// for Go HTTP clients that use DialContext.
type Policy struct {
	resolver      Resolver
	dialContext   dialContextFunc
	lookupTimeout time.Duration
	maxRedirects  int
}

// New creates a network policy backed by the system resolver and a bounded
// TCP dialer. A custom resolver is useful for deterministic tests.
func New(resolver Resolver) *Policy {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	dialer := &net.Dialer{Timeout: defaultDialTimeout, KeepAlive: 30 * time.Second}
	return &Policy{
		resolver:      resolver,
		dialContext:   dialer.DialContext,
		lookupTimeout: defaultLookupTimeout,
		maxRedirects:  defaultMaxRedirects,
	}
}

// ValidateURL verifies the URL scheme, authority, and every currently resolved
// address. Callers should also install DialContext on their transport so a later
// DNS answer is checked and pinned at connection time.
func (p *Policy) ValidateURL(ctx context.Context, rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return nil, fmt.Errorf("URL scheme must be http or https, got %q", parsed.Scheme)
	}
	if parsed.User != nil {
		return nil, errors.New("URL credentials are not allowed")
	}
	host := normalizeHostname(parsed.Hostname())
	if host == "" {
		return nil, errors.New("URL host is required")
	}
	if strings.Contains(host, "%") {
		return nil, errors.New("IPv6 zone identifiers are not allowed")
	}
	if isAmbiguousNumericHost(host) {
		return nil, errors.New("ambiguous numeric URL hosts are not allowed")
	}
	if isMetadataHostname(host) {
		return nil, errors.New("URL host is not allowed")
	}
	if _, err := p.resolvePublicIPs(ctx, host, ""); err != nil {
		return nil, err
	}
	return parsed, nil
}

// DialContext resolves, validates, and pins the selected address for an HTTP
// transport. It must be used in addition to ValidateURL/CheckRedirect because
// DNS can change between request validation and connection establishment.
func (p *Policy) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid network address %q: %w", address, err)
	}
	host = normalizeHostname(host)
	if host == "" || strings.Contains(host, "%") {
		return nil, fmt.Errorf("network host %q is not allowed", host)
	}

	addresses, err := p.resolvePublicIPs(ctx, host, network)
	if err != nil {
		return nil, err
	}
	var dialErrors []error
	for _, ip := range addresses {
		connection, dialErr := p.dialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return connection, nil
		}
		dialErrors = append(dialErrors, dialErr)
		if ctx.Err() != nil {
			break
		}
	}
	return nil, fmt.Errorf("dial public address for %q: %w", host, errors.Join(dialErrors...))
}

// CheckRedirect validates every redirect target and caps redirect chains.
func (p *Policy) CheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= p.maxRedirects {
		return fmt.Errorf("stopped after %d redirects", p.maxRedirects)
	}
	_, err := p.ValidateURL(req.Context(), req.URL.String())
	return err
}

// ConfigureTransport installs the rebinding-safe dial function on transport.
func (p *Policy) ConfigureTransport(transport *http.Transport) {
	transport.DialContext = p.DialContext
}

func (p *Policy) resolvePublicIPs(ctx context.Context, host, network string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		if IsBlockedIP(ip) {
			return nil, errors.New("URL resolves to a private or reserved address")
		}
		if !matchesNetwork(ip, network) {
			return nil, fmt.Errorf("address %q is incompatible with network %q", host, network)
		}
		return []net.IP{ip}, nil
	}

	lookupCtx := ctx
	cancel := func() {}
	if p.lookupTimeout > 0 {
		lookupCtx, cancel = context.WithTimeout(ctx, p.lookupTimeout)
	}
	defer cancel()
	resolved, err := p.resolver.LookupIPAddr(lookupCtx, host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve URL host: %w", err)
	}
	if len(resolved) == 0 {
		return nil, errors.New("URL host did not resolve to an address")
	}

	addresses := make([]net.IP, 0, len(resolved))
	for _, item := range resolved {
		if item.Zone != "" || IsBlockedIP(item.IP) {
			return nil, errors.New("URL resolves to a private or reserved address")
		}
		if matchesNetwork(item.IP, network) {
			addresses = append(addresses, item.IP)
		}
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("URL host has no address compatible with network %q", network)
	}
	return addresses, nil
}

func matchesNetwork(ip net.IP, network string) bool {
	switch network {
	case "tcp4", "udp4", "ip4":
		return ip.To4() != nil
	case "tcp6", "udp6", "ip6":
		return ip.To4() == nil
	default:
		return true
	}
}

func normalizeHostname(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

// isAmbiguousNumericHost rejects legacy IPv4 spellings such as 127.1,
// 0177.0.0.1, 0x7f000001, and 2130706433. Different URL stacks disagree on
// how these values are interpreted, so validating them through DNS while a
// child process later treats them as loopback would reopen an SSRF bypass.
func isAmbiguousNumericHost(host string) bool {
	if net.ParseIP(host) != nil {
		return false
	}
	allDecimalOrDot := true
	for _, char := range host {
		if (char < '0' || char > '9') && char != '.' {
			allDecimalOrDot = false
			break
		}
	}
	if allDecimalOrDot {
		return true
	}
	for _, component := range strings.Split(host, ".") {
		if strings.HasPrefix(component, "0x") || strings.HasPrefix(component, "0X") {
			return true
		}
	}
	return false
}

func isMetadataHostname(host string) bool {
	switch host {
	case "localhost", "localhost.localdomain", "metadata", "metadata.google", "metadata.google.internal", "instance-data", "kubernetes.default", "kubernetes.default.svc":
		return true
	}
	return strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".localdomain") ||
		strings.HasSuffix(host, ".internal")
}

// IsBlockedIP reports whether ip is non-public or belongs to a special-use
// range that must never be reached from user-controlled media URLs.
func IsBlockedIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	for _, network := range blockedNetworks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

var blockedNetworks = mustParseNetworks(
	"0.0.0.0/8",
	"100.64.0.0/10",
	"169.254.0.0/16",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"192.88.99.0/24",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"240.0.0.0/4",
	"64:ff9b::/96",
	"64:ff9b:1::/48",
	"100::/64",
	"2001::/23",
	"2001:db8::/32",
	"2002::/16",
	"3fff::/20",
	"fec0::/10",
)

func mustParseNetworks(values ...string) []*net.IPNet {
	networks := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			panic(err)
		}
		networks = append(networks, network)
	}
	return networks
}
