package clientip

import (
	"net/http"
	"testing"
)

func TestClientIPWithoutTrustedProxyUsesRemoteAddr(t *testing.T) {
	resolver, err := New(nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	req := &http.Request{RemoteAddr: "198.51.100.10:1234"}
	if got := resolver.ClientIP(req); got != "198.51.100.10" {
		t.Fatalf("ClientIP() = %q, want %q", got, "198.51.100.10")
	}
}

func TestClientIPWithTrustedProxyUsesForwardedFor(t *testing.T) {
	resolver, err := New([]string{"172.20.0.0/16"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "172.20.0.9:3456"
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 172.20.0.9")
	if got := resolver.ClientIP(req); got != "203.0.113.5" {
		t.Fatalf("ClientIP() = %q, want %q", got, "203.0.113.5")
	}
}

func TestClientIPWithTrustedProxyFallsBackToRealIP(t *testing.T) {
	resolver, err := New([]string{"172.20.0.0/16"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "172.20.0.9:3456"
	req.Header.Set("X-Real-IP", "203.0.113.8")
	if got := resolver.ClientIP(req); got != "203.0.113.8" {
		t.Fatalf("ClientIP() = %q, want %q", got, "203.0.113.8")
	}
}
