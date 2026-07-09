package httpclient

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsPrivateOrLocalIP(t *testing.T) {
	tests := []struct {
		ip      string
		private bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true}, // cloud metadata
		{"::1", true},            // IPv6 loopback
		{"fe80::1", true},        // IPv6 link-local
		{"8.8.8.8", false},       // public
		{"1.1.1.1", false},       // public
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			got := IsPrivateOrLocalIP(net.ParseIP(tt.ip))
			if got != tt.private {
				t.Errorf("IsPrivateOrLocalIP(%s) = %v, want %v", tt.ip, got, tt.private)
			}
		})
	}
}

func TestIsDangerousPort(t *testing.T) {
	dangerous := []string{"22", "6379", "27017", "11211"}
	safe := []string{"80", "443", "8080", ""}
	for _, p := range dangerous {
		if !IsDangerousPort(p) {
			t.Errorf("IsDangerousPort(%s) = false, want true", p)
		}
	}
	for _, p := range safe {
		if IsDangerousPort(p) {
			t.Errorf("IsDangerousPort(%s) = true, want false", p)
		}
	}
}

// A redirect to a loopback address must be blocked even though the initial
// host (the test server) is reachable and public-ish on the dial path.
func TestNewClient_BlocksRedirectToLoopback(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "http://127.0.0.1:6379/")
		w.WriteHeader(http.StatusFound)
	}))
	defer target.Close()

	client := NewClient(Options{TimeoutSeconds: 5, DisableSSRFProtection: false})
	resp, err := client.Get(target.URL)
	if err == nil {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected error blocking redirect to loopback, got status %d body=%q", resp.StatusCode, string(body))
	}
}
