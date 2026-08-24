package core

import (
	"net/http"

	"YT-GO/internal/netpolicy"
)

type outboundNetworkPolicy interface {
	ConfigureTransport(*http.Transport)
	CheckRedirect(*http.Request, []*http.Request) error
}

func newOutboundNetworkPolicy() outboundNetworkPolicy {
	return netpolicy.New(nil)
}

func (s *Service) networkPolicy() outboundNetworkPolicy {
	if s != nil && s.outboundPolicy != nil {
		return s.outboundPolicy
	}
	return newOutboundNetworkPolicy()
}

// EnableRestrictedNetworking applies public-network-only DNS, dial, and
// redirect checks to the Go media clients. Web server construction enables it;
// desktop mode keeps explicit localhost proxy support.
func (s *Service) EnableRestrictedNetworking() {
	s.restrictedNet.Store(true)
}

func (s *Service) configureOutboundTransport(transport *http.Transport) func(*http.Request, []*http.Request) error {
	if s == nil || !s.restrictedNet.Load() {
		return nil
	}
	policy := s.networkPolicy()
	policy.ConfigureTransport(transport)
	return policy.CheckRedirect
}
