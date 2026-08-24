package netpolicy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
)

type staticResolver map[string][]net.IPAddr

func (r staticResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	addresses, ok := r[host]
	if !ok {
		return nil, errors.New("host not found")
	}
	return addresses, nil
}

func TestValidateURLRejectsUnsafeTargets(t *testing.T) {
	policy := New(staticResolver{
		"public.example": {{IP: net.ParseIP("93.184.216.34")}},
		"mixed.example": {
			{IP: net.ParseIP("93.184.216.34")},
			{IP: net.ParseIP("127.0.0.1")},
		},
	})

	tests := []string{
		"file:///etc/passwd",
		"http://user:secret@public.example/video",
		"http://localhost/video",
		"http://169.254.169.254/latest/meta-data",
		"http://[::1]/video",
		"http://[64:ff9b::7f00:1]/video",
		"http://mixed.example/video",
	}
	for _, rawURL := range tests {
		t.Run(rawURL, func(t *testing.T) {
			if _, err := policy.ValidateURL(context.Background(), rawURL); err == nil {
				t.Fatalf("ValidateURL(%q) unexpectedly succeeded", rawURL)
			}
		})
	}

	if _, err := policy.ValidateURL(context.Background(), "https://public.example/video"); err != nil {
		t.Fatalf("public URL rejected: %v", err)
	}
}

func TestDialContextPinsValidatedAddress(t *testing.T) {
	policy := New(staticResolver{
		"public.example": {{IP: net.ParseIP("93.184.216.34")}},
	})
	var dialledAddress string
	policy.dialContext = func(_ context.Context, _, address string) (net.Conn, error) {
		dialledAddress = address
		return nil, errors.New("stop after recording address")
	}

	_, err := policy.DialContext(context.Background(), "tcp", "public.example:443")
	if err == nil {
		t.Fatal("expected injected dial error")
	}
	if dialledAddress != "93.184.216.34:443" {
		t.Fatalf("dialled %q, want pinned public IP", dialledAddress)
	}
}

func TestDialContextRejectsReboundDNSAnswer(t *testing.T) {
	policy := New(staticResolver{
		"rebound.example": {{IP: net.ParseIP("10.0.0.7")}},
	})
	called := false
	policy.dialContext = func(context.Context, string, string) (net.Conn, error) {
		called = true
		return nil, errors.New("must not dial")
	}

	if _, err := policy.DialContext(context.Background(), "tcp", "rebound.example:80"); err == nil {
		t.Fatal("private rebound address unexpectedly accepted")
	}
	if called {
		t.Fatal("dialer was called for a private rebound address")
	}
}

func TestCheckRedirectRejectsPrivateTargetAndLongChain(t *testing.T) {
	policy := New(staticResolver{
		"public.example": {{IP: net.ParseIP("93.184.216.34")}},
	})
	privateRequest, err := http.NewRequest(http.MethodGet, "http://127.0.0.1/admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := policy.CheckRedirect(privateRequest, nil); err == nil {
		t.Fatal("private redirect unexpectedly accepted")
	}

	publicRequest, err := http.NewRequest(http.MethodGet, "https://public.example/video", nil)
	if err != nil {
		t.Fatal(err)
	}
	via := make([]*http.Request, defaultMaxRedirects)
	if err := policy.CheckRedirect(publicRequest, via); err == nil {
		t.Fatal("long redirect chain unexpectedly accepted")
	}
}

func TestBlockedSpecialUseRanges(t *testing.T) {
	blocked := []string{
		"100.64.0.1",
		"192.0.2.1",
		"198.18.0.1",
		"203.0.113.1",
		"64:ff9b::7f00:1",
		"2001:db8::1",
		"2002:0a00:0001::",
		"fec0::1",
	}
	for _, value := range blocked {
		if !IsBlockedIP(net.ParseIP(value)) {
			t.Errorf("%s should be blocked", value)
		}
	}
	if IsBlockedIP(net.ParseIP("93.184.216.34")) {
		t.Fatal("public address was blocked")
	}
}

func TestAmbiguousNumericHosts(t *testing.T) {
	for _, host := range []string{"127.1", "0177.0.0.1", "0x7f000001", "0x7f.0.0.1", "2130706433"} {
		if !isAmbiguousNumericHost(host) {
			t.Errorf("%q should be rejected as an ambiguous numeric host", host)
		}
	}
	for _, host := range []string{"127.0.0.1", "93.184.216.34", "123.example", "face.example"} {
		if isAmbiguousNumericHost(host) {
			t.Errorf("%q should not be classified as an ambiguous numeric host", host)
		}
	}
}
