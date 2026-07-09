// Package httpclient provides a shared SSRF-safe HTTP client used by all
// HTTP-fetching code in thumbla. It validates the resolved IP at connect time
// (closing the DNS-rebinding TOCTOU) and re-validates host/path/port on every
// redirect (closing the redirect-following bypass).
package httpclient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/erans/thumbla/config"
)

// Options configures a SSRF-safe HTTP client.
type Options struct {
	TimeoutSeconds        int
	MaxBytes              int64
	RestrictHosts         []string
	RestrictPaths         []string
	DisableSSRFProtection bool // test-only: bypasses IP/port checks
}

// IsPrivateOrLocalIP reports whether ip is loopback, private, or link-local.
func IsPrivateOrLocalIP(ip net.IP) bool {
	privateRanges := []net.IPNet{
		{IP: net.IPv4(10, 0, 0, 0), Mask: net.IPv4Mask(255, 0, 0, 0)},
		{IP: net.IPv4(172, 16, 0, 0), Mask: net.IPv4Mask(255, 240, 0, 0)},
		{IP: net.IPv4(192, 168, 0, 0), Mask: net.IPv4Mask(255, 255, 0, 0)},
		{IP: net.IPv4(127, 0, 0, 0), Mask: net.IPv4Mask(255, 0, 0, 0)},
		{IP: net.IPv4(169, 254, 0, 0), Mask: net.IPv4Mask(255, 255, 0, 0)},
	}
	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

// IsDangerousPort reports whether port is commonly an internal service.
func IsDangerousPort(port string) bool {
	dangerousPorts := map[string]bool{
		"22": true, "23": true, "25": true, "53": true, "110": true,
		"143": true, "993": true, "995": true, "1433": true, "3306": true,
		"5432": true, "6379": true, "11211": true, "27017": true,
	}
	return dangerousPorts[port]
}

// validateURL runs host/path/port checks against a URL. It is called for the
// initial request and for every redirect target.
func validateURL(u *url.URL, opts Options) error {
	host := u.Hostname()
	port := u.Port()

	if !opts.DisableSSRFProtection {
		if port != "" && IsDangerousPort(port) {
			return fmt.Errorf("access to port %s is not allowed for security reasons", port)
		}
		ips, err := net.LookupIP(host)
		if err != nil {
			return fmt.Errorf("failed to resolve hostname %s: %w", host, err)
		}
		for _, ip := range ips {
			if IsPrivateOrLocalIP(ip) {
				return fmt.Errorf("access to private/local IP %s (resolved from %s) is not allowed", ip.String(), host)
			}
		}
	}

	if len(opts.RestrictHosts) > 0 {
		allowed := false
		for _, h := range opts.RestrictHosts {
			if u.Host == h || strings.HasSuffix(u.Host, "."+h) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("host %s is not in allowed hosts list", u.Host)
		}
	}

	if len(opts.RestrictPaths) > 0 {
		// path.Clean collapses ".." so /allowed/../secret cannot bypass a prefix check.
		cleaned := path.Clean(u.Path)
		allowed := false
		for _, p := range opts.RestrictPaths {
			if strings.HasPrefix(cleaned, p) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("path %s is not in allowed paths list", u.Path)
		}
	}
	return nil
}

// NewClient returns an http.Client that validates IPs at dial time and
// re-validates host/path/port on every redirect.
func NewClient(opts Options) *http.Client {
	timeout := time.Duration(opts.TimeoutSeconds) * time.Second
	if opts.TimeoutSeconds <= 0 {
		timeout = 30 * time.Second
	}

	dialer := &net.Dialer{Timeout: 30 * time.Second}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if opts.DisableSSRFProtection {
				return dialer.DialContext(ctx, network, addr)
			}
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if IsDangerousPort(port) {
				return nil, fmt.Errorf("access to port %s is not allowed for security reasons", port)
			}
			ips, err := net.LookupIP(host)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve hostname %s: %w", host, err)
			}
			for _, ip := range ips {
				if IsPrivateOrLocalIP(ip) {
					return nil, fmt.Errorf("access to private/local IP %s is not allowed", ip.String())
				}
			}
			// Dial the first resolved IP directly to pin the resolution (no re-resolution).
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		},
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if err := validateURL(req.URL, opts); err != nil {
				return err
			}
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}
}

// MaxBytes returns the configured max image size, falling back to the global config.
func MaxBytes() int64 {
	return config.GetConfig().GetMaxImageSizeBytes()
}
