package ssrf

import (
	"context"
	"testing"
)

func TestValidateOutboundURLRejectsUnsafeTargets(t *testing.T) {
	tests := []string{
		"file:///tmp/data",
		"http://localhost/test",
		"http://127.0.0.1/test",
		"http://10.0.0.1/test",
		"http://100.64.0.1/test",
		"http://169.254.169.254/latest/meta-data",
		"http://[::1]/test",
		"http://example/test",
		"http://bad_host.example/test",
	}
	for _, rawURL := range tests {
		t.Run(rawURL, func(t *testing.T) {
			if err := ValidateOutboundURL(context.Background(), rawURL); err == nil {
				t.Fatal("expected URL to be rejected")
			}
			if IsSafeOutboundURL(rawURL) {
				t.Fatal("IsSafeOutboundURL returned true for rejected URL")
			}
		})
	}
}

func TestValidateOutboundURLAllowsPublicTargetsAndProxySchemes(t *testing.T) {
	tests := []string{
		"https://1.1.1.1/generate_204",
		"http://8.8.8.8:8080",
		"socks5://user:pass@8.8.4.4:1080",
	}
	for _, rawURL := range tests {
		t.Run(rawURL, func(t *testing.T) {
			if err := ValidateOutboundURL(context.Background(), rawURL); err != nil {
				t.Fatalf("expected URL to be accepted: %v", err)
			}
			if !IsSafeOutboundURL(rawURL) {
				t.Fatal("IsSafeOutboundURL returned false for accepted URL")
			}
		})
	}
}

func TestValidateOutboundURLHonorsSchemeAllowlist(t *testing.T) {
	if err := ValidateOutboundURL(context.Background(), "http://8.8.8.8/test", "https"); err == nil {
		t.Fatal("expected HTTP URL to be rejected by HTTPS-only allowlist")
	}
	if err := ValidateOutboundURL(context.Background(), "https://8.8.8.8/test", "https"); err != nil {
		t.Fatalf("expected HTTPS URL to be accepted: %v", err)
	}
}
