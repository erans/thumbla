# Thumbla Security & Correctness Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close all Critical/High/Medium findings from the adversarial review — SSRF bypasses, image-decode OOM, Redis cache panic/growth, silent wrong output, and medium correctness gaps — without adding features.

**Architecture:** Introduce a shared SSRF-safe HTTP client with dial-time IP validation (used by `fetchers/http.go` and `manipulators/paste.go`); add pre-decode dimension checks and uniform `LimitReader` across fetchers; make the cache interface typed (`GetBytes`/`SetBytes`) with TTL and DEL-on-trim; move manipulator parameter validation onto the `Manipulator` interface; and fix the remaining medium items in place.

**Tech Stack:** Go 1.25.1, Fiber v2, go-redis/v9, oksvg, bild, kolesa-team/go-webp. Tests use the stdlib `testing` package (no testify in the suite — match existing style with `t.Errorf`/`t.Fatalf`).

## Global Constraints

- Go module path: `github.com/erans/thumbla`.
- All new tests use stdlib `testing` only (no testify) — match existing `t.Errorf`/`t.Fatalf` style.
- `config.GetConfig()` is the global config accessor; defaults live in `config/config.go` getter methods.
- `disableSSRFProtection` is a **test-only** flag; production config must never set it. New SSRF tests must NOT set it.
- The `Manipulator` interface is consumed by `handlers/imagehandler.go` via `manipulators.GetManipulatorByName`.
- Commit frequently (after each task, at minimum). One concern per commit.
- Never leave the build broken: run `go build ./...` before each commit.

---

## File Structure

- **Create:** `httpclient/ssrf.go` — shared SSRF-safe `*http.Client` factory + IP/port/path validation helpers.
- **Create:** `httpclient/ssrf_test.go` — tests for IP/port/path validation and redirect/rebind defense (without disabling protection).
- **Modify:** `fetchers/http.go` — use shared client; drop inline SSRF checks; keep `restrictHosts`/`restrictPaths` (delegated to client).
- **Modify:** `fetchers/http_test.go` — add SSRF-on tests (redirect to 127.0.0.1, private IP, `..` path).
- **Modify:** `manipulators/paste.go` — fetch `img` via shared SSRF client + `LimitReader`.
- **Modify:** `handlers/imagehandler.go` — `DecodeConfig` before decode; SVG clamp + nil-deref; `params` length-check (400); CacheControl base path; read output params from `c.Locals`.
- **Modify:** `fetchers/s3.go`, `fetchers/gs.go`, `fetchers/ds.go`, `fetchers/r2.go` — `defer Close` + `LimitReader` + size check.
- **Modify:** `cache/cache.go` — add `GetBytes`/`SetBytes` to the `Cache` interface.
- **Modify:** `cache/dummycache.go`, `cache/inmemory.go`, `cache/redis.go` — implement `GetBytes`/`SetBytes`; Redis TTL + DEL-on-trim.
- **Modify:** `config/config.go` — `yaml:"password"` fix; add `Cache.Redis.TTL` field + getter; add `TrustedProxies` config.
- **Modify:** `manipulators/facecrop.go` — typed cache round-trip via `GetBytes`/`SetBytes`; two-value assertion.
- **Modify:** `manipulators/manipulator.go` — add `Validate(params map[string]string) error` to the interface.
- **Modify:** each `manipulators/*.go` — implement `Validate`.
- **Modify:** `handlers/imagehandler.go` — call per-manipulator `Validate` in `parseManipulators`; remove `validateManipulatorParameter` numeric map.
- **Modify:** `manipulators/output.go` — write params via `c.Locals`; fix `image/web` typo; encoder allowlist.
- **Modify:** `thumbla.go` — register `recover` before logger; configure trusted proxies for rate limiter.

---

## Task 1: Shared SSRF-safe HTTP client (`httpclient/` package)

The foundation for the SSRF fixes. Other tasks depend on this package existing.

**Files:**
- Create: `httpclient/ssrf.go`
- Create: `httpclient/ssrf_test.go`

**Interfaces:**
- Consumes: `github.com/erans/thumbla/config` (`config.GetConfig().GetMaxImageSizeBytes()`, `GetHTTPTimeout()`).
- Produces: `httpclient.NewClient(opts Options) *http.Client` where:

```go
type Options struct {
    TimeoutSeconds        int
    MaxBytes              int64
    RestrictHosts         []string
    RestrictPaths         []string
    DisableSSRFProtection bool // test-only
}
```

- Produces: `httpclient.IsPrivateOrLocalIP(ip net.IP) bool`, `httpclient.IsDangerousPort(port string) bool` (relocated from `fetchers/http.go`), and `httpclient.ValidateURL(u *url.URL, opts Options) error` — the URL-level host/path/port/IP check that `CheckRedirect` calls on every redirect **and that callers MUST call on the initial request URL** (the `*http.Client` has no hook to validate the first URL's host/path before dial; `DialContext` only sees host:port and only enforces IP/port). `CheckRedirect` delegates to `ValidateURL`, so testing `ValidateURL` directly covers the redirect-validation logic.

- [ ] **Step 1: Write the failing test for IP validation**

Create `httpclient/ssrf_test.go`:

```go
package httpclient

import (
	"net"
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./httpclient/ -run TestIsPrivateOrLocalIP -v`
Expected: FAIL — package not found / functions undefined.

- [ ] **Step 3: Write the SSRF client implementation**

Create `httpclient/ssrf.go`:

```go
// Package httpclient provides a shared SSRF-safe HTTP client used by all
// HTTP-fetching code in thumbla. It validates the resolved IP at connect time
// (closing the DNS-rebinding TOCTOU) and re-validates host/path/port on every
// redirect (closing the redirect-following bypass). Callers MUST also call
// ValidateURL on the initial request URL before issuing it, to enforce
// RestrictHosts/RestrictPaths on the first request (CheckRedirect only runs
// on redirects).
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

// ValidateURL runs host/path/port/IP checks against a URL. It is the
// single source of truth for URL-level SSRF validation: CheckRedirect calls
// it on every redirect target, and callers MUST call it on the initial
// request URL before issuing the request (the *http.Client has no hook to
// validate the first URL's host/path before dial; DialContext only sees
// host:port and only enforces IP/port). Returns nil if the URL is allowed.
func ValidateURL(u *url.URL, opts Options) error {
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
			if err := ValidateURL(req.URL, opts); err != nil {
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./httpclient/ -run TestIsPrivateOrLocalIP -v`
Expected: PASS.

- [ ] **Step 5: Write failing tests for redirect validation and URL validation**

Update `httpclient/ssrf_test.go` so it contains the following additional tests. Merge `net/http` and `net/url` into the existing import block; do **not** add a second `package` declaration or a second import block.

```go
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
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./httpclient/ -run "TestNewClient_CheckRedirect_BlocksLoopback|TestValidateURL" -v`
Expected: PASS — `CheckRedirect` rejects the loopback redirect target; `ValidateURL` rejects private IPs, dangerous ports, disallowed hosts, and `..` path traversal, and allows valid hosts/paths.

- [ ] **Step 7: Commit**

```bash
git add httpclient/ssrf.go httpclient/ssrf_test.go
go build ./...
git commit -m "feat(httpclient): add shared SSRF-safe HTTP client with dial-time validation"
```

---

## Task 2: Wire `fetchers/http.go` to the shared SSRF client

Remove the inline SSRF/IP/port checks (now in the shared client) and the inline `http.Client` construction. Keep `restrictHosts`/`restrictPaths` by passing them to the shared client.

**Files:**
- Modify: `fetchers/http.go` (entire `Fetch` method; delete `isPrivateOrLocalIP`/`isDangerousPort` which moved to `httpclient`)
- Modify: `fetchers/http_test.go` (add SSRF-on test; keep existing tests working)

**Interfaces:**
- Consumes: `httpclient.NewClient`, `httpclient.Options`, and `httpclient.ValidateURL` for the initial request URL.
- Produces: unchanged `HTTPFetcher.Fetch` signature.

- [ ] **Step 1: Write integration tests asserting the HTTP fetcher enforces initial-URL validation**

Append to `fetchers/http_test.go`:

```go
func TestHTTPFetcher_SSRF_BlocksLoopbackInitialURL(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{HTTPTimeout: 5, MaxImageSizeBytes: 10 * 1024 * 1024}}
	config.SetConfig(cfg)

	fetcher := NewHTTPFetcher(map[string]interface{}{
		"name": "testHTTP",
		"type": "http",
		// NOTE: disableSSRFProtection intentionally NOT set.
	})

	var ctx *fiber.Ctx
	_, _, err := fetcher.Fetch(ctx, "http://127.0.0.1:6379/test.jpg")
	if err == nil {
		t.Fatal("expected SSRF error blocking loopback initial URL, got none")
	}
}

func TestHTTPFetcher_RestrictPathsBlocksDotDotTraversal(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{HTTPTimeout: 5, MaxImageSizeBytes: 10 * 1024 * 1024}}
	config.SetConfig(cfg)

	fetcher := NewHTTPFetcher(map[string]interface{}{
		"name":          "testHTTP",
		"type":          "http",
		"restrictPaths": []interface{}{"/allowed/"},
	})

	var ctx *fiber.Ctx
	_, _, err := fetcher.Fetch(ctx, "http://8.8.8.8/allowed/../secret/test.jpg")
	if err == nil {
		t.Fatal("expected restrictPaths to reject /allowed/../secret traversal, got none")
	}
}
```

These are integration/regression tests for the fetcher wiring. The redirect-specific validation path is covered in Task 1 by `httpclient.TestNewClient_CheckRedirect_BlocksLoopback`, which directly exercises `client.CheckRedirect` without the false-pass of a local `httptest` server.

- [ ] **Step 2: Run tests to capture current behavior**

Run: `go test ./fetchers/ -run "TestHTTPFetcher_SSRF_BlocksLoopbackInitialURL|TestHTTPFetcher_RestrictPathsBlocksDotDotTraversal" -v`
Expected: the loopback test may already PASS under the pre-refactor inline SSRF check; the `..` path traversal test demonstrates the gap this task must close (current code only raw-prefix checks paths and can pass for the wrong reason by failing later on network). Keep both as regression guards.

- [ ] **Step 3: Refactor `fetchers/http.go` to use the shared client**

Edit `fetchers/http.go`:

1. Add import `"github.com/erans/thumbla/httpclient"`.
2. Remove the `isPrivateOrLocalIP` and `isDangerousPort` functions (moved to `httpclient`). Remove now-unused imports (`"net"` if unused; keep `"net/url"` — still used for `url.Parse`; remove `"time"` if now unused).
3. In `Fetch`, delete the inline SSRF block (the `if !fetcher.DisableSSRFProtection { ... }` host/port/LookupIP block), the inline `restrictHosts`/`restrictPaths` blocks, and the `client := &http.Client{...}` line. Replace with:

```go
	// Build shared SSRF-safe options once so the initial URL and redirect targets
	// are validated with identical rules.
	clientOpts := httpclient.Options{
		TimeoutSeconds:        config.GetConfig().GetHTTPTimeout(),
		MaxBytes:              config.GetConfig().GetMaxImageSizeBytes(),
		RestrictHosts:         fetcher.RestrictHosts,
		RestrictPaths:         fetcher.RestrictPaths,
		DisableSSRFProtection: fetcher.DisableSSRFProtection,
	}
	client := httpclient.NewClient(clientOpts)

	// Validate the *initial* URL up front. CheckRedirect only runs on redirects;
	// DialContext only sees host:port and cannot enforce RestrictHosts or
	// RestrictPaths for the first request.
	parsedURL, err := url.Parse(fetchURL)
	if err != nil {
		return nil, "", fmt.Errorf("invalid URL: %w", err)
	}
	if err := httpclient.ValidateURL(parsedURL, clientOpts); err != nil {
		return nil, "", err
	}
```

Keep the existing request construction (`http.NewRequestWithContext`), `client.Do`, decompression, and `LimitReader` read unchanged. The `restrictPaths` `path.Clean` fix now lives in `httpclient.ValidateURL`.

- [ ] **Step 4: Run all fetcher tests**

Run: `go test ./fetchers/ -v`
Expected: PASS — including the new SSRF test and the existing `disableSSRFProtection: true` tests.

- [ ] **Step 5: Commit**

```bash
git add fetchers/http.go fetchers/http_test.go
go build ./...
git commit -m "fix(fetchers/http): use shared SSRF-safe client; close redirect/rebind bypass"
```

---

## Task 3: Fix `manipulators/paste.go` SSRF

The paste manipulator does a raw `http.Get` on a user-controlled URL with no SSRF protection, no timeout, and no size limit. Route it through the shared client and bound the body.

**Files:**
- Modify: `manipulators/paste.go`
- Create: `manipulators/paste_test.go`

**Interfaces:**
- Consumes: `httpclient.NewClient`, `config.GetConfig()`.

- [ ] **Step 1: Write the failing test asserting paste blocks an internal URL**

Create `manipulators/paste_test.go`:

```go
package manipulators

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/erans/thumbla/config"
	"github.com/gofiber/fiber/v2"
)

func TestPasteManipulator_BlocksInternalURL(t *testing.T) {
	config.SetConfig(&config.Config{})
	cfg := &config.Config{}
	config.SetConfig(cfg)
	m := NewPasteManipulator(cfg)

	base := image.NewRGBA(image.Rect(0, 0, 10, 10))
	base.Set(0, 0, color.RGBA{255, 0, 0, 255})

	var c *fiber.Ctx
	_, err := m.Execute(c, map[string]string{"img": "http://127.0.0.1:6379/x"}, base)
	if err == nil {
		t.Fatal("expected SSRF error for internal paste img URL, got none")
	}
	if !strings.Contains(err.Error(), "private/local") && !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected SSRF guard error, got: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./manipulators/ -run TestPasteManipulator_BlocksInternalURL -v`
Expected: FAIL — `http.Get` does not apply the SSRF guard; it returns a non-guard error or hangs rather than the SSRF guard error.

- [ ] **Step 3: Fix `paste.go` to use the shared client + LimitReader**

Edit `manipulators/paste.go`. Update the import block to:

```go
import (
	"fmt"
	"image"
	"image/draw"
	"io"

	"github.com/erans/thumbla/config"
	"github.com/erans/thumbla/httpclient"
	"github.com/gofiber/fiber/v2"
)
```

(`"net/http"` is no longer referenced — `client.Get` is a method on the `*http.Client` returned by `httpclient.NewClient`, and the `*http.Response` type is inferred, so the `net/http` import would be unused and must be dropped.)

In `Execute`, replace:

```go
		response, err := http.Get(imgUrl)
		if err != nil {
			return nil, err
		}

		defer response.Body.Close()
		pastedImg, _, err := image.Decode(response.Body)
		if err != nil {
			return nil, err
		}
```

with:

```go
		client := httpclient.NewClient(httpclient.Options{
			TimeoutSeconds: config.GetConfig().GetHTTPTimeout(),
			MaxBytes:       config.GetConfig().GetMaxImageSizeBytes(),
		})
		response, err := client.Get(imgUrl)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()

		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, fmt.Errorf("paste: fetch returned HTTP %d", response.StatusCode)
		}

		limited := io.LimitReader(response.Body, config.GetConfig().GetMaxImageSizeBytes())
		pastedImg, _, err := image.Decode(limited)
		if err != nil {
			return nil, err
		}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./manipulators/ -run TestPasteManipulator_BlocksInternalURL -v`
Expected: PASS — the shared client blocks the loopback address at dial time.

- [ ] **Step 5: Commit**

```bash
git add manipulators/paste.go manipulators/paste_test.go
go build ./...
git commit -m "fix(manipulators/paste): route img fetch through SSRF-safe client; bound body size"
```

---

## Task 4: Image-decode memory safety + SVG/blank nil-panic fix (`handlers/imagehandler.go`)

Add `*DecodeConfig` before `*Decode` for raster formats; clamp SVG `ViewBox` and check `err` before allocating; length-check `params` for `.svg`/`_blank` and return 400.

**Files:**
- Modify: `handlers/imagehandler.go` (`loadImage`, `HandleImage`)
- Modify: `handlers/imagehandler_test.go` (add cases)

**Interfaces:**
- Consumes: `config.GetConfig().GetMaxImageDimension()`.

- [ ] **Step 1: Write the failing test for oversized-image rejection**

Append to `handlers/imagehandler_test.go`:

```go
func TestHandleImage_RejectsOversizedPNG(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use a small max dimension so a 100x100 PNG exceeds it.
	cfg := &config.Config{
		Fetchers: []map[string]interface{}{
			{"name": "local", "type": "local", "path": os.TempDir()},
		},
		Paths: []config.PathConfig{{Path: "/test", FetcherName: "local"}},
		Server: config.ServerConfig{MaxImageDimension: 50, MaxImageSizeBytes: 50 * 1024 * 1024},
	}
	config.SetConfig(cfg)
	fetchers.InitFetchers(cfg)
	manipulators.InitManipulators(cfg)

	pngData, _ := createTestImage(100, 100, "png")
	os.WriteFile(filepath.Join(os.TempDir(), "big.png"), pngData, 0644)
	defer os.Remove(filepath.Join(os.TempDir(), "big.png"))

	app := fiber.New()
	app.Get("/test/:url/*", HandleImage)

	req := httptest.NewRequest("GET", "/test/big.png/output:f=png", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Errorf("expected 500 for oversized image, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run test to verify current behavior**

Run: `go test ./handlers/ -run TestHandleImage_RejectsOversizedPNG -v`
Expected: currently PASSes (post-decode check catches it) — this test gates the pre-decode behavior so it stays green after the change.

- [ ] **Step 3: Add `DecodeConfig` pre-check in `loadImage`**

Edit `handlers/imagehandler.go`. Add a helper after the imports:

```go
// checkImageDimensions reads format config from body and rejects images
// exceeding maxImageDimension before the full pixel buffer is allocated.
// DecodeConfig consumes bytes, so this function reads the already-size-limited
// body into memory once and returns a fresh reader over the full data.
func checkImageDimensions(contentType string, body io.Reader) (io.Reader, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	maxDim := config.GetConfig().GetMaxImageDimension()
	configReader := bytes.NewReader(data)

	var w, h int
	var cfgErr error
	switch contentType {
	case "image/jpeg", "image/jpg":
		var c image.Config
		c, cfgErr = jpeg.DecodeConfig(configReader)
		w, h = c.Width, c.Height
	case "image/png":
		var c image.Config
		c, cfgErr = png.DecodeConfig(configReader)
		w, h = c.Width, c.Height
	case "image/webp":
		var c image.Config
		c, cfgErr = webp.DecodeConfig(configReader)
		w, h = c.Width, c.Height
	case "image/gif":
		var c image.Config
		c, cfgErr = gif.DecodeConfig(configReader)
		w, h = c.Width, c.Height
	default:
		return bytes.NewReader(data), nil
	}
	if cfgErr != nil {
		return nil, fmt.Errorf("failed to read image config: %w", cfgErr)
	}
	if w > maxDim || h > maxDim || w <= 0 || h <= 0 {
		return nil, fmt.Errorf("image dimensions (%dx%d) exceed maximum allowed size (%dx%d)",
			w, h, maxDim, maxDim)
	}
	return bytes.NewReader(data), nil
}
```

Then in `loadImage`, before the raster `if`-chain, call it (skipping SVG):

```go
	if contentType != "image/svg+xml" {
		body, err = checkImageDimensions(contentType, body)
		if err != nil {
			return nil, err
		}
	}
```

(`loadImage` already declares `err`; ensure `bytes` is imported.)

- [ ] **Step 4: Fix the SVG branch (clamp + nil-deref)**

In `loadImage`, replace:

```go
	} else if contentType == "image/svg+xml" {
		var svgImg *oksvg.SvgIcon
		svgImg, err = oksvg.ReadIconStream(body)
		w := int(svgImg.ViewBox.W)
		h := int(svgImg.ViewBox.H)
```

with:

```go
	} else if contentType == "image/svg+xml" {
		var svgImg *oksvg.SvgIcon
		svgImg, err = oksvg.ReadIconStream(body)
		if err != nil {
			return nil, fmt.Errorf("failed to parse SVG: %w", err)
		}
		w := int(svgImg.ViewBox.W)
		h := int(svgImg.ViewBox.H)

		maxDim := config.GetConfig().GetMaxImageDimension()
		if w <= 0 || h <= 0 {
			return nil, fmt.Errorf("invalid SVG dimensions: %dx%d", w, h)
		}
		if w > maxDim {
			w = maxDim
		}
		if h > maxDim {
			h = maxDim
		}
```

Keep the rest of the SVG block (rescale by alternativeWidth/Height, `svgImg.SetTarget`, `image.NewRGBA64` — now clamped).

- [ ] **Step 5: Fix the `.svg`/`_blank` nil-panic in `HandleImage`**

In `HandleImage`, replace:

```go
	imageURL, params = getFileParams(imageURL)

	if strings.HasSuffix(strings.ToLower(imageURL), ".svg") {
		alternateWidth, _ = strconv.Atoi(params[0])
		alternateHeight, _ = strconv.Atoi(params[1])
	} else if imageURL == "_blank" {
		if params[0] == "rgba" {
			alternateWidth, _ = strconv.Atoi(params[1])
			alternateHeight, _ = strconv.Atoi(params[2])
```

with length-checked access:

```go
	imageURL, params = getFileParams(imageURL)

	if strings.HasSuffix(strings.ToLower(imageURL), ".svg") {
		if len(params) < 2 {
			return c.Status(fiber.StatusBadRequest).SendString("SVG requests require |width,height params")
		}
		alternateWidth, _ = strconv.Atoi(params[0])
		alternateHeight, _ = strconv.Atoi(params[1])
	} else if imageURL == "_blank" {
		if len(params) < 1 || params[0] != "rgba" {
			return c.Status(fiber.StatusBadRequest).SendString("blank image requires |rgba,width,height params")
		}
		if len(params) < 3 {
			return c.Status(fiber.StatusBadRequest).SendString("blank image requires |rgba,width,height params")
		}
		alternateWidth, _ = strconv.Atoi(params[1])
		alternateHeight, _ = strconv.Atoi(params[2])
```

(Keep the existing `img := image.NewRGBA(...)` / `png.Encode(buf, img)` block that follows.)

- [ ] **Step 6: Add the panic test and run**

Append to `handlers/imagehandler_test.go`:

```go
func TestHandleImage_SVGWithoutParamsReturns400(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	app := fiber.New()
	app.Get("/test/:url/*", HandleImage)

	req := httptest.NewRequest("GET", "/test/something.svg", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("expected 400 for svg without params, got %d", resp.StatusCode)
	}
}
```

Run: `go test ./handlers/ -run "TestHandleImage_RejectsOversizedPNG|TestHandleImage_SVGWithoutParamsReturns400" -v`
Expected: PASS — oversized PNG rejected before decode; SVG without params returns 400 instead of panicking.

- [ ] **Step 7: Commit**

```bash
git add handlers/imagehandler.go handlers/imagehandler_test.go
go build ./...
git commit -m "fix(handlers): pre-decode dimension checks; clamp SVG; 400 on malformed svg/blank params"
```

---

## Task 5: Fetcher body limits + close (S3/GS/DS/R2)

Each cloud fetcher reads bodies without a size limit and several leak the body (no `Close`). Apply `io.LimitReader` and `defer Close`, and have the handler close streaming bodies.

**Files:**
- Modify: `fetchers/s3.go`, `fetchers/gs.go`, `fetchers/ds.go`, `fetchers/r2.go`
- Modify: `handlers/imagehandler.go` (close `imageBody` when it's an `io.ReadCloser`)

**Interfaces:**
- Consumes: `config.GetConfig().GetMaxImageSizeBytes()`.

- [ ] **Step 1: Fix `fetchers/s3.go`**

Edit the `Fetch` tail. Replace:

```go
	// TODO: Currently we fetch the image to the memory. Consider adding protection to limit the max size

	log.Printf("Content Length: %d    Content-Type: %s", output.ContentLength, aws.ToString(output.ContentType))

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, output.Body)
	if err != nil {
		return nil, "", err
	}

	return bytes.NewReader(buf.Bytes()), aws.ToString(output.ContentType), nil
```

with:

```go
	defer output.Body.Close()

	maxSize := config.GetConfig().GetMaxImageSizeBytes()
	contentLength := int64(0)
	if output.ContentLength != nil {
		contentLength = *output.ContentLength
		if contentLength > maxSize {
			return nil, "", fmt.Errorf("object size (%d bytes) exceeds maximum (%d bytes)", contentLength, maxSize)
		}
	}

	log.Printf("Content Length: %d    Content-Type: %s", contentLength, aws.ToString(output.ContentType))

	buf := new(bytes.Buffer)
	n, err := io.Copy(buf, io.LimitReader(output.Body, maxSize+1))
	if err != nil {
		return nil, "", err
	}
	if n > maxSize {
		return nil, "", fmt.Errorf("object size exceeds maximum (%d bytes)", maxSize)
	}

	return bytes.NewReader(buf.Bytes()), aws.ToString(output.ContentType), nil
```

Add import `"github.com/erans/thumbla/config"`.

- [ ] **Step 2: Fix `fetchers/gs.go`**

Replace:

```go
	var reader *storage.Reader
	if reader, err = obj.NewReader(ctx); err != nil {
		return nil, "", err
	}
	defer reader.Close()

	var buf []byte
	if buf, err = io.ReadAll(reader); err != nil {
		return nil, "", err
	}

	return bytes.NewReader(buf), contentType, nil
```

with:

```go
	var reader *storage.Reader
	if reader, err = obj.NewReader(ctx); err != nil {
		return nil, "", err
	}
	defer reader.Close()

	maxSize := config.GetConfig().GetMaxImageSizeBytes()
	if objAttrs.Size > maxSize {
		return nil, "", fmt.Errorf("object size (%d bytes) exceeds maximum (%d bytes)", objAttrs.Size, maxSize)
	}
	buf, err := io.ReadAll(io.LimitReader(reader, maxSize+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(buf)) > maxSize {
		return nil, "", fmt.Errorf("object size exceeds maximum (%d bytes)", maxSize)
	}

	return bytes.NewReader(buf), contentType, nil
```

Add import `"github.com/erans/thumbla/config"` (`"fmt"` is already imported in `gs.go`).

- [ ] **Step 3: Fix `fetchers/ds.go`**

DS currently returns the raw body. Buffer it with `LimitReader(maxSize+1)`, reject overflow, and close the underlying object body inside the fetcher. Replace:

```go
	return output.Body, aws.ToString(output.ContentType), nil
```

with:

```go
	defer output.Body.Close()

	maxSize := config.GetConfig().GetMaxImageSizeBytes()
	contentLength := int64(0)
	if output.ContentLength != nil {
		contentLength = *output.ContentLength
		if contentLength > maxSize {
			return nil, "", fmt.Errorf("object size (%d bytes) exceeds maximum (%d bytes)", contentLength, maxSize)
		}
	}

	buf := new(bytes.Buffer)
	n, err := io.Copy(buf, io.LimitReader(output.Body, maxSize+1))
	if err != nil {
		return nil, "", err
	}
	if n > maxSize {
		return nil, "", fmt.Errorf("object size exceeds maximum (%d bytes)", maxSize)
	}

	return bytes.NewReader(buf.Bytes()), aws.ToString(output.ContentType), nil
```

Add imports `"bytes"` and `"github.com/erans/thumbla/config"`.

- [ ] **Step 4: Fix `fetchers/r2.go`**

Same as DS: buffer with `LimitReader(maxSize+1)`, reject overflow, and close the underlying object body inside the fetcher. Replace:

```go
	return output.Body, aws.ToString(output.ContentType), nil
```

with:

```go
	defer output.Body.Close()

	maxSize := config.GetConfig().GetMaxImageSizeBytes()
	contentLength := int64(0)
	if output.ContentLength != nil {
		contentLength = *output.ContentLength
		if contentLength > maxSize {
			return nil, "", fmt.Errorf("object size (%d bytes) exceeds maximum (%d bytes)", contentLength, maxSize)
		}
	}

	buf := new(bytes.Buffer)
	n, err := io.Copy(buf, io.LimitReader(output.Body, maxSize+1))
	if err != nil {
		return nil, "", err
	}
	if n > maxSize {
		return nil, "", fmt.Errorf("object size exceeds maximum (%d bytes)", maxSize)
	}

	return bytes.NewReader(buf.Bytes()), aws.ToString(output.ContentType), nil
```

Add imports `"bytes"` and `"github.com/erans/thumbla/config"`.

- [ ] **Step 5: Close streaming bodies in the handler**

In `handlers/imagehandler.go` `HandleImage`, after the fetch block and before `loadImage`, add:

```go
	if closer, ok := imageBody.(io.Closer); ok {
		defer closer.Close()
	}
```

(Place it just after the `if useFetchers { ... }` block and before the `imageBody == nil` check.)

- [ ] **Step 6: Build and run handler/fetcher tests**

Run: `go build ./... && go test ./handlers/ ./fetchers/ -v`
Expected: PASS (DS/R2/S3/GS changes are not exercised without cloud creds, but must compile; local-fetcher tests still pass).

- [ ] **Step 7: Commit**

```bash
git add fetchers/s3.go fetchers/gs.go fetchers/ds.go fetchers/r2.go handlers/imagehandler.go
go build ./...
git commit -m "fix(fetchers): LimitReader + defer Close on S3/GS/DS/R2 bodies; handler closes streams"
```

---

## Task 6: Typed cache (`GetBytes`/`SetBytes`) + TTL + DEL-on-trim + Redis password tag

Make the cache interface typed so `facecrop` can round-trip `[]image.Rectangle` without panicking under Redis; add a real TTL; delete data keys on LRU trim; fix the Redis password YAML tag.

**Files:**
- Modify: `cache/cache.go` (interface)
- Modify: `cache/dummycache.go`, `cache/inmemory.go`, `cache/redis.go`
- Modify: `config/config.go` (password tag; `Cache.Redis.TTL` field + getter)
- Modify: `manipulators/facecrop.go` (typed round-trip + two-value assertion)
- Create: `cache/redis_test.go` (using `miniredis`)

**Interfaces:**
- Produces: `Cache.GetBytes(key string) ([]byte, bool)`, `Cache.SetBytes(key string, value []byte)` added to the `Cache` interface. Existing `Get`/`Set`/`Contains` remain for backward compatibility.

- [ ] **Step 1: Add a miniredis dev dependency**

Run:

```bash
go get github.com/alicebob/miniredis/v2
go mod tidy
```

- [ ] **Step 2: Add the TTL config field and fix the password YAML tag**

Edit `config/config.go`. In the `Redis` struct change `Password string \`yaml:"string"\`` to:

```go
			Password   string `yaml:"password"`
```

and add a TTL field:

```go
			MaxLRUSize int    `yaml:"maxLRUSize"`
			TTL        int    `yaml:"ttl"` // TTL in seconds; 0 = use default
```

Add a getter:

```go
// GetRedisTTL returns the Redis cache TTL in seconds with a default fallback.
func (cfg *Config) GetRedisTTL() int {
	if cfg.Cache.Redis.TTL <= 0 {
		return 3600 // Default 1 hour
	}
	return cfg.Cache.Redis.TTL
}
```

- [ ] **Step 3: Write the failing tests for typed Redis round-trip**

Create `cache/redis_test.go`:

```go
package cache

import (
	"bytes"
	"encoding/gob"
	"image"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/erans/thumbla/config"
)

func TestRedisCache_SetGetBytes_RoundTrip(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	cfg := &config.Config{}
	cfg.Cache.Redis.Address = mr.Addr()
	cfg.Cache.Redis.TTL = 60
	cfg.Cache.Redis.MaxLRUSize = 100
	config.SetConfig(cfg)

	c := NewRedisCache(cfg)

	rects := []image.Rectangle{image.Rect(0, 0, 10, 10), image.Rect(5, 5, 20, 20)}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(rects); err != nil {
		t.Fatal(err)
	}
	c.SetBytes("face-key", buf.Bytes())

	got, ok := c.GetBytes("face-key")
	if !ok {
		t.Fatal("expected cache hit")
	}
	var out []image.Rectangle
	if err := gob.NewDecoder(bytes.NewReader(got)).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 2 || out[0] != rects[0] || out[1] != rects[1] {
		t.Fatalf("round-trip mismatch: %+v", out)
	}
}

func TestRedisCache_TTLApplied(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	cfg := &config.Config{}
	cfg.Cache.Redis.Address = mr.Addr()
	cfg.Cache.Redis.TTL = 60
	config.SetConfig(cfg)

	c := NewRedisCache(cfg)
	c.SetBytes("k", []byte("v"))
	ttl := mr.TTL("k")
	if ttl <= 0 {
		t.Fatal("expected a positive TTL on the key")
	}
}

func TestRedisCache_TrimsDeleteDataKeys(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	cfg := &config.Config{}
	cfg.Cache.Redis.Address = mr.Addr()
	cfg.Cache.Redis.TTL = 60
	cfg.Cache.Redis.MaxLRUSize = 1 // tiny to force eviction
	config.SetConfig(cfg)

	c := NewRedisCache(cfg)
	c.SetBytes("k1", []byte("v1"))
	c.SetBytes("k2", []byte("v2")) // should evict k1

	if c.Contains("k1") {
		t.Fatal("expected k1 to be evicted (data key deleted)")
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./cache/ -run TestRedisCache -v`
Expected: FAIL — `GetBytes`/`SetBytes` not defined.

- [ ] **Step 5: Extend the `Cache` interface**

Edit `cache/cache.go`:

```go
type Cache interface {
	Contains(key string) bool
	Get(key string) interface{}
	Set(key string, value interface{})
	GetBytes(key string) ([]byte, bool)
	SetBytes(key string, value []byte)
	Clear()
}
```

- [ ] **Step 6: Implement `GetBytes`/`SetBytes` in dummy and inmemory**

Edit `cache/dummycache.go` — add:

```go
func (m *DummyCache) GetBytes(key string) ([]byte, bool) { return nil, false }
func (m *DummyCache) SetBytes(key string, value []byte)   {}
```

Edit `cache/inmemory.go` — add:

```go
func (m *InMemoryCache) GetBytes(key string) ([]byte, bool) {
	if v, ok := m.Cache.Get(key); ok {
		if b, ok := v.([]byte); ok {
			return b, true
		}
	}
	return nil, false
}

func (m *InMemoryCache) SetBytes(key string, value []byte) {
	m.Cache.Add(key, value)
}
```

- [ ] **Step 7: Implement `GetBytes`/`SetBytes` + TTL + DEL-on-trim in `cache/redis.go`**

Edit `cache/redis.go`. Add import `"github.com/erans/thumbla/config"`. Add methods and refactor `Set`:

```go
// GetBytes returns the raw bytes stored for key.
func (r *RedisCache) GetBytes(key string) ([]byte, bool) {
	val, err := r.client.Get(r.ctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	r.client.ZIncrBy(r.ctx, "cache_access", 1, key)
	return val, true
}

// SetBytes stores raw bytes with the configured TTL.
func (r *RedisCache) SetBytes(key string, value []byte) {
	ttl := time.Duration(config.GetConfig().GetRedisTTL()) * time.Second
	r.client.Set(r.ctx, key, value, ttl)
	r.client.ZAdd(r.ctx, "cache_access", redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: key,
	})
	r.enforceMaxSize()
}

// Set marshals value to JSON and delegates to SetBytes.
func (r *RedisCache) Set(key string, value interface{}) {
	jsonVal, err := json.Marshal(value)
	if err != nil {
		return
	}
	r.SetBytes(key, jsonVal)
}

// enforceMaxSize trims the LRU set and deletes the corresponding data keys.
func (r *RedisCache) enforceMaxSize() {
	maxSize := r.maxLRUSize
	if maxSize <= 0 {
		maxSize = 10000
	}
	count, err := r.client.ZCard(r.ctx, "cache_access").Result()
	if err != nil || count <= int64(maxSize) {
		return
	}
	toRemove := count - int64(maxSize)
	members, err := r.client.ZPopMin(r.ctx, "cache_access", toRemove).Result()
	if err != nil {
		return
	}
	keys := make([]string, 0, len(members))
	for _, z := range members {
		if s, ok := z.Member.(string); ok {
			keys = append(keys, s)
		}
	}
	if len(keys) > 0 {
		r.client.Del(r.ctx, keys...)
	}
}
```

Remove the old `Set` body (replaced above) and the old inline trim block in the previous `Set`.

- [ ] **Step 8: Fix `facecrop.go` typed round-trip + two-value assertion**

Edit `manipulators/facecrop.go`. Add imports `"bytes"`, `"encoding/gob"`. Replace the cache block:

```go
		if !useCache || !cache.GetCache().Contains(cacheKey) {
			faces, err = detector.Detect(c, m.Cfg, params, img)
			if err != nil {
				log.Printf("Error: %v", err)
			}

			if useCache {
				cache.GetCache().Set(cacheKey, faces)
			}
		} else {
			faces = cache.GetCache().Get(cacheKey).([]image.Rectangle)
			log.Printf("Found faces cache")
		}
```

with a typed round-trip that avoids the `Contains`→`Get` TOCTOU:

```go
		if !useCache {
			faces, err = detector.Detect(c, m.Cfg, params, img)
			if err != nil {
				log.Printf("Error: %v", err)
			}
		} else if raw, ok := cache.GetCache().GetBytes(cacheKey); ok {
			var cached []image.Rectangle
			if gerr := gob.NewDecoder(bytes.NewReader(raw)).Decode(&cached); gerr == nil {
				faces = cached
				log.Printf("Found faces cache")
			} else {
				faces, err = detector.Detect(c, m.Cfg, params, img)
				if err != nil {
					log.Printf("Error: %v", err)
				}
			}
		} else {
			faces, err = detector.Detect(c, m.Cfg, params, img)
			if err != nil {
				log.Printf("Error: %v", err)
			}
			var buf bytes.Buffer
			if werr := gob.NewEncoder(&buf).Encode(faces); werr == nil {
				cache.GetCache().SetBytes(cacheKey, buf.Bytes())
			}
		}
```

- [ ] **Step 9: Run cache tests**

Run: `go test ./cache/ -v`
Expected: PASS — round-trip, TTL, and trim-DEL all pass.

- [ ] **Step 10: Commit**

```bash
git add cache/ config/config.go manipulators/facecrop.go go.mod go.sum
go build ./...
git commit -m "fix(cache): typed GetBytes/SetBytes; Redis TTL; DEL-on-trim; password YAML tag; facecrop round-trip"
```

---

## Task 7: Output manipulator Locals + encoder allowlist + `image/web` typo

Output quality/lossless/exact/encoder must travel via `c.Locals` (not request headers), unknown encoders rejected, and `image/web` → `image/webp`.

**Files:**
- Modify: `manipulators/output.go`
- Modify: `handlers/imagehandler.go` (`writeImageToResponse` reads `c.Locals`)
- Modify: `manipulators/output_test.go` (extend)

**Interfaces:**
- Produces: Locals keys `"X-Quality"`, `"X-Lossless"`, `"X-Exact"`, `"X-Encoder"` written via `c.Locals(...)`.

- [ ] **Step 1: Write the failing test asserting quality is honored**

Append to `manipulators/output_test.go` (read the existing file first to match style and imports):

```go
func TestOutputManipulator_QualityRecordedInLocals(t *testing.T) {
	cfg := &config.Config{}
	m := NewOutputManipulator(cfg)
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		_, err := m.Execute(c, map[string]string{"f": "jpg", "q": "45"}, img)
		if err != nil {
			return err
		}
		if c.Locals("X-Quality") != "45" {
			t.Errorf("expected X-Quality local = 45, got %v", c.Locals("X-Quality"))
		}
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/", nil)
	app.Test(req)
}
```

Ensure the test file imports `"net/http/httptest"` and `"github.com/gofiber/fiber/v2"` if not already present.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./manipulators/ -run TestOutputManipulator_QualityRecordedInLocals -v`
Expected: FAIL — current code uses `c.Set` (response header), not `c.Locals`.

- [ ] **Step 3: Fix `manipulators/output.go` to use Locals + allowlist + typo**

Edit `manipulators/output.go`. Change every `c.Set("X-Quality", val)` → `c.Locals("X-Quality", val)`, `c.Set("X-Lossless", val)` → `c.Locals("X-Lossless", val)`, `c.Set("X-Exact", val)` → `c.Locals("X-Exact", val)`. Keep `c.Set("Content-Type", contentType)` (it's a real response header). Fix `if contentType == "image/web"` → `if contentType == "image/webp"`. Replace the encoder block:

```go
			if val, ok := params["e"]; ok {
				if c != nil {
				logger := middleware.GetLoggerFromContext(c)
				logger.Debug().Str("encoder", val).Msg("Setting encoder")
			}
				if val == "guetzli" {
					if c != nil {
						c.Set("X-Encoder", val)
					}
				}
			}
```

with:

```go
			if val, ok := params["e"]; ok {
				if c != nil {
					logger := middleware.GetLoggerFromContext(c)
					logger.Debug().Str("encoder", val).Msg("Setting encoder")
					c.Locals("X-Encoder", val)
				}
			}
```

- [ ] **Step 4: Fix `writeImageToResponse` to read from Locals + encoder allowlist**

Edit `handlers/imagehandler.go`. Add a helper:

```go
func localString(c *fiber.Ctx, key string) string {
	if v, ok := c.Locals(key).(string); ok {
		return v
	}
	return ""
}
```

Replace the body of `writeImageToResponse` with:

```go
func writeImageToResponse(c *fiber.Ctx, contentType string, img image.Image) error {
	if contentType == "image/jpeg" || contentType == "image/jpg" {
		quality := 90
		if q := localString(c, "X-Quality"); q != "" {
			if n, err := strconv.Atoi(q); err == nil {
				quality = n
			}
		}
		enc := localString(c, "X-Encoder")
		if enc == "" {
			enc = "jpeg"
		}
		if enc != "jpeg" && enc != "guetzli" {
			return fmt.Errorf("unsupported encoder: %s", enc)
		}
		if enc == "jpeg" {
			jpeg.Encode(c.Response().BodyWriter(), img, &jpeg.Options{Quality: quality})
		}
		// guetzli is not implemented; no empty 200 body.
	} else if contentType == "image/png" {
		png.Encode(c.Response().BodyWriter(), img)
	} else if contentType == "image/webp" {
		quality := 100.0
		if q := localString(c, "X-Quality"); q != "" {
			if f, err := strconv.ParseFloat(q, 32); err == nil {
				quality = f
			}
		}
		options, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, float32(quality))
		if err != nil {
			return fmt.Errorf("failed to create WebP encoder options: %w", err)
		}
		if temp := localString(c, "X-Lossless"); temp == "1" || temp == "true" {
			options.Lossless = true
		}
		if temp := localString(c, "X-Exact"); temp == "1" || temp == "true" {
			options.Exact = 1
		}
		kolesawebp.Encode(c.Response().BodyWriter(), img, options)
	} else {
		return fmt.Errorf("write image to response failed. Unknown content type '%s'", contentType)
	}
	return nil
}
```

- [ ] **Step 5: Run output tests + handler tests**

Run: `go test ./manipulators/ -run TestOutput -v && go test ./handlers/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add manipulators/output.go handlers/imagehandler.go manipulators/output_test.go
go build ./...
git commit -m "fix(output): pass params via Locals; encoder allowlist; fix image/webp typo"
```

---

## Task 8: Per-manipulator `Validate` + remove global numeric map

Add `Validate(params map[string]string) error` to the `Manipulator` interface, implement it per manipulator, and replace the global `validateManipulatorParameter` numeric map in `parseManipulators`. This fixes the silent chain-drop on `r=lanczos` (resize), `r=1` (rotate), and `r=x|y|x|y` (crop).

**Files:**
- Modify: `manipulators/manipulator.go` (interface + shared helper)
- Modify: every `manipulators/*.go` (add `Validate`)
- Modify: `handlers/imagehandler.go` (`parseManipulators`; delete `validateManipulatorParameter`'s numeric map)
- Modify: `handlers/imagehandler_test.go` (end-to-end `resize:r=lanczos`)

**Interfaces:**
- Produces: `Manipulator.Validate(params map[string]string) error` and `manipulators.ValidateNumericInt(name, value string, min, max int) error`.

- [ ] **Step 1: Write the failing end-to-end test**

Append to `handlers/imagehandler_test.go`:

```go
func TestHandleImage_ResizeWithLanczosFilter(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	app := fiber.New()
	app.Get("/test/:url/*", HandleImage)

	req := httptest.NewRequest("GET", "/test/test.jpg/resize:w=50,r=lanczos/output:f=jpg", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for resize:r=lanczos, got %d (chain was dropped)", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	cfg, derr := jpeg.DecodeConfig(bytes.NewReader(body))
	if derr != nil {
		t.Fatalf("decode response: %v", derr)
	}
	if cfg.Width != 50 {
		t.Errorf("expected resized width 50, got %d (manipulator chain dropped)", cfg.Width)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./handlers/ -run TestHandleImage_ResizeWithLanczosFilter -v`
Expected: FAIL — `r=lanczos` fails the numeric validator and drops the chain; the response is the original 100-wide image, so `cfg.Width != 50`.

- [ ] **Step 3: Add `Validate` to the `Manipulator` interface and a shared helper**

Edit `manipulators/manipulator.go`:

```go
type Manipulator interface {
	Execute(c *fiber.Ctx, params map[string]string, img image.Image) (image.Image, error)
	Validate(params map[string]string) error
}
```

Add the helper and imports (`"fmt"`, `"strconv"`):

```go
// ValidateNumericInt parses a param as an int and checks bounds.
func ValidateNumericInt(name, value string, min, max int) error {
	v, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("parameter %s requires integer value, got: %s", name, value)
	}
	if v < min || v > max {
		return fmt.Errorf("parameter %s value %d outside [%d, %d]", name, v, min, max)
	}
	return nil
}
```

- [ ] **Step 4: Implement `Validate` per manipulator**

Add a `Validate` method to each manipulator. Ensure each file imports `"fmt"` (and `"strconv"` where used directly).

- **resize.go** — `r` is a string filter name; `w`/`h` numeric in [1, 20000]:

```go
func (manipulator *ResizeManipulator) Validate(params map[string]string) error {
	if v, ok := params["r"]; ok {
		if _, exists := resamplingFilters[v]; !exists {
			return fmt.Errorf("unsupported resampling filter: %s", v)
		}
	}
	if v, ok := params["w"]; ok {
		if err := ValidateNumericInt("w", v, 1, 20000); err != nil {
			return err
		}
	}
	if v, ok := params["h"]; ok {
		if err := ValidateNumericInt("h", v, 1, 20000); err != nil {
			return err
		}
	}
	return nil
}
```

- **fit.go** — same as resize (imports `"fmt"`):

```go
func (manipulator *FitManipulator) Validate(params map[string]string) error {
	if v, ok := params["r"]; ok {
		if _, exists := resamplingFilters[v]; !exists {
			return fmt.Errorf("unsupported resampling filter: %s", v)
		}
	}
	if v, ok := params["w"]; ok {
		if err := ValidateNumericInt("w", v, 1, 20000); err != nil {
			return err
		}
	}
	if v, ok := params["h"]; ok {
		if err := ValidateNumericInt("h", v, 1, 20000); err != nil {
			return err
		}
	}
	return nil
}
```

- **rotate.go** — `a` in [-360, 360]; `r` is `0`/`1` (imports `"fmt"`, `"strconv"`):

```go
func (manipulator *RotateManipulator) Validate(params map[string]string) error {
	if v, ok := params["a"]; ok {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("parameter a requires numeric value, got: %s", v)
		}
		if f < -360 || f > 360 {
			return fmt.Errorf("parameter a value %g outside [-360, 360]", f)
		}
	}
	if v, ok := params["r"]; ok {
		if v != "0" && v != "1" {
			return fmt.Errorf("parameter r must be 0 or 1, got: %s", v)
		}
	}
	return nil
}
```

- **crop.go** — `x`,`y` in [0, 20000]; `w`,`h` in [1, 20000]; `r` is a legacy rectangle string:

```go
func (manipulator *CropManipulator) Validate(params map[string]string) error {
	for _, name := range []string{"x", "y"} {
		if v, ok := params[name]; ok {
			if err := ValidateNumericInt(name, v, 0, 20000); err != nil {
				return err
			}
		}
	}
	for _, name := range []string{"w", "h"} {
		if v, ok := params[name]; ok {
			if err := ValidateNumericInt(name, v, 1, 20000); err != nil {
				return err
			}
		}
	}
	return nil
}
```

- **brightness.go** and **contrast.go** — `v` numeric in [-255, 255]:

```go
func (manipulator *BrightnessManipulator) Validate(params map[string]string) error {
	if v, ok := params["v"]; ok {
		return ValidateNumericInt("v", v, -255, 255)
	}
	return nil
}
```

(Same body for `ContrastManipulator`.)

- **shearh.go** and **shearv.go** — `a` numeric float in [-360, 360] (imports `"fmt"`, `"strconv"`):

```go
func (manipulator *ShearHorizontalManipulator) Validate(params map[string]string) error {
	if v, ok := params["a"]; ok {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("parameter a requires numeric value, got: %s", v)
		}
		if f < -360 || f > 360 {
			return fmt.Errorf("parameter a value %g outside [-360, 360]", f)
		}
	}
	return nil
}
```

(Same body for `ShearVerticalManipulator`.)

- **fliph.go / flipv.go / paste.go / facecrop.go** — minimal:

```go
func (manipulator *FlipHorizontalManipulator) Validate(params map[string]string) error { return nil }
```

(Same for `FlipVerticalManipulator`, `PasteManipulator`, `FaceCropManipulator`.)

- **output.go** — validate `f` allowlist, `q` in [1, 100], `lossless`/`progressive` booleans (imports `"fmt"`, `"strings"`, `"strconv"`):

```go
func (manipulator *OutputManipulator) Validate(params map[string]string) error {
	if val, ok := params["f"]; ok {
		if _, ok := formatContentTypeMapping[strings.ToLower(val)]; !ok {
			return fmt.Errorf("unsupported format: %s", val)
		}
	}
	if v, ok := params["q"]; ok {
		if err := ValidateNumericInt("q", v, 1, 100); err != nil {
			return err
		}
	}
	for _, b := range []string{"lossless", "progressive"} {
		if v, ok := params[b]; ok {
			if v != "true" && v != "false" && v != "1" && v != "0" {
				return fmt.Errorf("parameter %s requires boolean value, got: %s", b, v)
			}
		}
	}
	return nil
}
```

- [ ] **Step 5: Replace global validation in `parseManipulators`**

Edit `handlers/imagehandler.go`. In the param loop, replace:

```go
			if manipulatorParamName != "" {
				// Validate parameter values to prevent attacks
				if err := validateManipulatorParameter(manipulatorParamName, manipulatorParamValue); err != nil {
					logger.Warn().
						Str("param", manipulatorParamName).
						Str("value", manipulatorParamValue).
						Err(err).
						Msg("Invalid manipulator parameter")
					return nil
				}
				manipulatorParams[manipulatorParamName] = manipulatorParamValue
			}
```

with:

```go
			if manipulatorParamName != "" {
				manipulatorParams[manipulatorParamName] = manipulatorParamValue
			}
```

Then, after the manipulator-name/params are parsed (where `result[k] = &manipulatorAction{...}` is set), validate per-manipulator:

```go
		if manipulatorName != "" {
			m := manipulators.GetManipulatorByName(manipulatorName)
			if m != nil {
				if verr := m.Validate(manipulatorParams); verr != nil {
					logger.Warn().
						Str("manipulator", manipulatorName).
						Err(verr).
						Msg("Invalid manipulator parameters")
					return nil
				}
			}
			result[k] = &manipulatorAction{manipulatorName, manipulatorParams}
		}
```

Delete the `validateManipulatorParameter` function (no longer called). Remove now-unused imports if any (e.g. `net/url` is still used for `url.QueryUnescape` — keep it).

- [ ] **Step 6: Run the end-to-end test and the full manipulator suite**

Run: `go test ./handlers/ -run TestHandleImage_ResizeWithLanczosFilter -v && go test ./manipulators/ -v`
Expected: PASS — `r=lanczos` no longer dropped; existing manipulator tests still pass (they call `Execute` directly; the new `Validate` method must not break them).

- [ ] **Step 7: Commit**

```bash
git add manipulators/ handlers/imagehandler.go
go build ./...
git commit -m "fix(manipulators): per-manipulator Validate; drop global numeric map; fix r-filter chain-drop"
```

---

## Task 9: Medium items — middleware ordering, CacheControl path, rate-limiter trusted proxies

The remaining medium findings: register `recover` before the logger; pass the computed base path to `GetPathConfigByPath`; configure Fiber trusted proxies so `c.IP()` for rate-limiting is not spoofable.

**Files:**
- Modify: `thumbla.go` (middleware order; trusted proxy config)
- Modify: `handlers/imagehandler.go` (CacheControl uses base path)
- Modify: `config/config.go` (TrustedProxies config block)
- Modify: `config/config_test.go` (assert Redis password loads; assert TrustedProxies default)

**Interfaces:**
- Produces: `Config.Server.TrustedProxies` (struct: `Enabled bool; Proxies []string; ProxyHeader string`) + getter.

- [ ] **Step 1: Write the failing test asserting Redis password loads from YAML**

Append to `config/config_test.go` (inside the existing `package config`, reusing the temp-file pattern from `TestLoadConfig`):

```go
func TestLoadConfig_RedisPassword(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "thumbla_redis_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configContent := `
cache:
  active: true
  provider: "redis"
  redis:
    host: "localhost:6379"
    password: "s3cret"
    ttl: 120
`
	configFile := filepath.Join(tempDir, "test-config.yml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Cache.Redis.Password != "s3cret" {
		t.Errorf("expected password 's3cret', got %q", cfg.Cache.Redis.Password)
	}
	if cfg.GetRedisTTL() != 120 {
		t.Errorf("expected TTL 120, got %d", cfg.GetRedisTTL())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/ -run TestLoadConfig_RedisPassword -v`
Expected: FAIL — the old `yaml:"string"` tag drops `password`, so `Password` is empty. (After Task 6 Step 2 this may already PASS; if so, confirm and keep it as a regression guard.)

- [ ] **Step 3: Fix middleware ordering in `thumbla.go`**

Edit `thumbla.go`. Swap the order so `recover` is registered **before** the logger:

```go
	app.Use(recover.New())
	app.Use(middleware.New())
```

- [ ] **Step 4: Fix CacheControl to use the base path in `HandleImage`**

Edit `handlers/imagehandler.go`. In `HandleImage`, replace:

```go
	pathConfig := config.GetConfig().GetPathConfigByPath(c.Route().Path)
```

with (move it next to where `path` is computed, reusing the base path):

```go
	rawRequestPath := c.Route().Path
	path := rawRequestPath[0:strings.Index(rawRequestPath, "/:url")]
	pathConfig := config.GetConfig().GetPathConfigByPath(path)
```

Delete the original `pathConfig := ...` line and the later duplicate `rawRequestPath := c.Route().Path` / `path := ...` lines (consolidate them).

- [ ] **Step 5: Add TrustedProxies config + Fiber configuration**

Edit `config/config.go`. Add a struct and field:

```go
type TrustedProxiesConfig struct {
	Enabled     bool     `yaml:"enabled"`
	Proxies     []string `yaml:"proxies"`      // trusted proxy CIDRs/hostnames
	ProxyHeader string   `yaml:"proxyHeader"` // e.g. "X-Forwarded-For"
}
```

Add to `ServerConfig`:

```go
	TrustedProxies TrustedProxiesConfig `yaml:"trustedProxies"`
```

Add a getter:

```go
// IsTrustedProxiesEnabled returns whether trusted-proxy handling is enabled.
func (cfg *Config) IsTrustedProxiesEnabled() bool {
	return cfg.Server.TrustedProxies.Enabled
}
```

Edit `thumbla.go`. Configure Fiber's trusted-proxy check via `fiber.Config` when enabled. The v2 defaults are already safe: with `EnableTrustedProxyCheck=false` (the default), `c.IP()` returns the TCP peer (`fasthttp.RemoteIP`), which is unspoofable — so the rate limiter is correct out of the box when deployed direct-to-internet. Only when deployed behind a known proxy should the operator enable it. Change the `app := fiber.New(fiber.Config{...})` call to include the trusted-proxy fields:

```go
	fiberCfg := fiber.Config{
		BodyLimit:    int(cfg.GetMaxRequestSize()),
		ReadTimeout:  time.Duration(cfg.GetReadTimeout()) * time.Second,
		WriteTimeout: time.Duration(cfg.GetWriteTimeout()) * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			logger := middleware.GetLoggerFromContext(c)
			logger.Error().Err(err).Msg("Request error")
			return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
		},
	}
	if cfg.IsTrustedProxiesEnabled() {
		fiberCfg.EnableTrustedProxyCheck = true
		fiberCfg.TrustedProxies = cfg.Server.TrustedProxies.Proxies
		fiberCfg.ProxyHeader = cfg.Server.TrustedProxies.ProxyHeader
		fiberCfg.EnableIPValidation = true
	}
	app := fiber.New(fiberCfg)
```

Exact field names (verified against `gofiber/fiber/v2@v2.52.9/app.go`): `EnableTrustedProxyCheck bool`, `TrustedProxies []string`, `ProxyHeader string`, `EnableIPValidation bool`. When disabled (default), `c.IP()` returns `fasthttp.RemoteIP()` and ignores `X-Forwarded-For`, so spoofing is impossible.

- [ ] **Step 6: Run config + handler tests and build**

Run: `go build ./... && go test ./config/ ./handlers/ -v`
Expected: PASS — Redis password loads; CacheControl still applies for `/test` (non-trailing-slash) paths.

- [ ] **Step 7: Commit**

```bash
git add thumbla.go handlers/imagehandler.go config/config.go config/config_test.go
go build ./...
git commit -m "fix: recover-before-logger; CacheControl base path; trusted-proxy config for rate limiter; redis password tag"
```

---

## Task 10: Full verification

**Files:** none (verification only).

- [ ] **Step 1: Run the entire test suite**

Run: `go build ./... && go test ./... -v`
Expected: PASS — all packages.

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`
Expected: no issues.

- [ ] **Step 3: Confirm SSRF tests run without disabling protection**

Run: `go test ./fetchers/ ./httpclient/ ./manipulators/ -run "SSRF|BlocksRedirect|BlocksInternalURL|IsPrivateOrLocalIP" -v`
Expected: PASS — every SSRF test passes with `disableSSRFProtection` unset.

- [ ] **Step 4: Confirm no `disableSSRFProtection` in production paths**

Run: `grep -rn "disableSSRFProtection" --include=*.go | grep -v _test.go | grep -v httpclient/ssrf.go`
Expected: only the `HTTPFetcher` config field and its wiring (no hardcoded `true` in production code).

- [ ] **Step 5: Final commit (if any fixups)**

```bash
git add -A
git commit -m "chore: final verification of hardening pass" || echo "nothing to commit"
```

---

## Done

All Critical/High/Medium findings from the adversarial review are closed: shared SSRF-safe client (redirect + DNS-rebind defense, used by HTTP fetcher and paste); pre-decode dimension checks and uniform `LimitReader`; typed cache with TTL and DEL-on-trim; per-manipulator validation (no more `r`-filter chain-drop); output params via Locals with encoder allowlist; and the medium items (middleware order, CacheControl base path, trusted proxies, Redis password tag).