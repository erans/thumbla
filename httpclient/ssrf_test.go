package httpclient

import (
	"net"
	"net/http"
	"net/url"
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
		{"0.0.0.0", true},         // IPv4 unspecified
		{"224.0.0.1", true},       // IPv4 multicast
		{"::", true},              // IPv6 unspecified
		{"::1", true},             // IPv6 loopback
		{"fe80::1", true},         // IPv6 link-local
		{"fd00::1", true},         // IPv6 ULA
		{"fc00::1", true},         // IPv6 ULA
		{"8.8.8.8", false},        // public
		{"1.1.1.1", false},        // public
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			got := IsPrivateOrLocalIP(net.ParseIP(tt.ip))
			if got != tt.private {
				t.Errorf("IsPrivateOrLocalIP(%s) = %v, want %v", tt.ip, got, tt.private)
			}
		})
	}

	if !IsPrivateOrLocalIP(nil) {
		t.Error("IsPrivateOrLocalIP(nil) = false, want true")
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

// TestNewClient_CheckRedirect_BlocksLoopback exercises the redirect policy
// directly (no network): CheckRedirect must reject a redirect target whose
// host resolves to a loopback/private IP. Calling client.CheckRedirect
// directly avoids the false-pass where the *initial* dial to a 127.0.0.1
// test server is blocked before any redirect occurs.
func TestNewClient_CheckRedirect_BlocksLoopback(t *testing.T) {
	client := NewClient(Options{TimeoutSeconds: 5})
	req, err := http.NewRequest("GET", "http://127.0.0.1:6379/", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	prev, err := http.NewRequest("GET", "http://8.8.8.8/", nil)
	if err != nil {
		t.Fatalf("new prev request: %v", err)
	}
	if err := client.CheckRedirect(req, []*http.Request{prev}); err == nil {
		t.Fatal("expected CheckRedirect to reject loopback redirect target, got nil")
	}
}

// TestValidateURL covers the URL-level validation logic that CheckRedirect
// delegates to and that callers must run on the initial request URL. The tests
// use public IP literals (8.8.8.8 / 1.1.1.1) instead of hostnames so they are
// hermetic and do not need DisableSSRFProtection.
func TestValidateURL(t *testing.T) {
	mustURL := func(s string) *url.URL {
		u, err := url.Parse(s)
		if err != nil {
			panic(err)
		}
		return u
	}
	tests := []struct {
		name    string
		u       string
		opts    Options
		wantErr bool
	}{
		{"loopback ip", "http://127.0.0.1/x", Options{}, true},
		{"metadata ip", "http://169.254.169.254/x", Options{}, true},
		{"dangerous port", "http://8.8.8.8:6379/x", Options{}, true},
		{"host not allowed", "http://8.8.8.8/x", Options{RestrictHosts: []string{"1.1.1.1"}}, true},
		{"host allowed", "http://8.8.8.8/x", Options{RestrictHosts: []string{"8.8.8.8"}}, false},
		{"path traversal blocked", "http://8.8.8.8/allowed/../secret/x", Options{RestrictPaths: []string{"/allowed/"}}, true},
		{"path allowed", "http://8.8.8.8/allowed/x", Options{RestrictPaths: []string{"/allowed/"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(mustURL(tt.u), tt.opts)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateURL(%s) = nil, want error", tt.u)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateURL(%s) = %v, want nil", tt.u, err)
			}
		})
	}
}
