package api

import (
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/kevindiu/rclone-manager/internal/config"
)

func TestTrustedProxyClientIPAndSecureRequest(t *testing.T) {
	srv := &Server{
		cfg: &config.Config{
			TrustedProxies: []netip.Prefix{
				netip.MustParsePrefix("127.0.0.1/32"),
				netip.MustParsePrefix("172.16.0.0/12"),
			},
		},
	}

	// Untrusted client cannot spoof X-Forwarded-For or X-Forwarded-Proto.
	untrusted := httptest.NewRequest("POST", "/api/auth/login", nil)
	untrusted.RemoteAddr = "198.51.100.25:54321"
	untrusted.Header.Set("X-Forwarded-For", "203.0.113.9")
	untrusted.Header.Set("X-Forwarded-Proto", "https")
	if got := srv.clientIP(untrusted); got != "198.51.100.25" {
		t.Fatalf("untrusted proxy spoofed clientIP: got %q", got)
	}
	if srv.isSecureRequest(untrusted) {
		t.Fatal("untrusted proxy spoofed X-Forwarded-Proto")
	}

	// Trusted proxy forwards real client IP (skipping right-most trusted hop) and HTTPS proto.
	trusted := httptest.NewRequest("POST", "/api/auth/login", nil)
	trusted.RemoteAddr = "172.18.0.2:41234"
	trusted.Header.Set("X-Forwarded-For", "203.0.113.42, 127.0.0.1")
	trusted.Header.Set("X-Forwarded-Proto", "https, http")
	if got := srv.clientIP(trusted); got != "203.0.113.42" {
		t.Fatalf("expected 203.0.113.42 from XFF, got %q", got)
	}
	if !srv.isSecureRequest(trusted) {
		t.Fatal("expected trusted proxy X-Forwarded-Proto=https to be secure")
	}

	// Without TrustedProxies configured, preserve legacy X-Forwarded-Proto behavior and use RemoteAddr for IP.
	legacy := &Server{cfg: &config.Config{}}
	req := httptest.NewRequest("POST", "/api/auth/login", nil)
	req.RemoteAddr = "10.0.0.5:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.77")
	req.Header.Set("X-Forwarded-Proto", "https")
	if got := legacy.clientIP(req); got != "10.0.0.5" {
		t.Fatalf("expected RemoteAddr host without TrustedProxies, got %q", got)
	}
	if !legacy.isSecureRequest(req) {
		t.Fatal("expected legacy X-Forwarded-Proto=https to remain secure")
	}
}
