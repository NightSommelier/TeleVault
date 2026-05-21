package clientip

import (
	"net"
	"net/http"
	"strings"
)

type Resolver struct {
	trusted []*net.IPNet
}

func New(trustedCIDRs []string) (*Resolver, error) {
	resolver := &Resolver{}
	for _, raw := range trustedCIDRs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		_, network, err := net.ParseCIDR(raw)
		if err != nil {
			return nil, err
		}
		resolver.trusted = append(resolver.trusted, network)
	}
	return resolver, nil
}

func (r *Resolver) ClientIP(req *http.Request) string {
	if req == nil {
		return "unknown"
	}

	remoteIP := parseRemoteIP(req.RemoteAddr)
	if remoteIP == nil {
		return fallbackRemote(req.RemoteAddr)
	}
	if !r.isTrusted(remoteIP) {
		return remoteIP.String()
	}

	if forwarded := strings.TrimSpace(req.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			candidate := net.ParseIP(strings.TrimSpace(parts[i]))
			if candidate == nil {
				continue
			}
			if !r.isTrusted(candidate) {
				return candidate.String()
			}
		}
	}

	if realIP := net.ParseIP(strings.TrimSpace(req.Header.Get("X-Real-IP"))); realIP != nil {
		return realIP.String()
	}
	return remoteIP.String()
}

func (r *Resolver) isTrusted(ip net.IP) bool {
	if ip == nil || len(r.trusted) == 0 {
		return false
	}
	for _, network := range r.trusted {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func parseRemoteIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil && host != "" {
		return net.ParseIP(host)
	}
	return net.ParseIP(remoteAddr)
}

func fallbackRemote(remoteAddr string) string {
	if remoteAddr == "" {
		return "unknown"
	}
	return remoteAddr
}
